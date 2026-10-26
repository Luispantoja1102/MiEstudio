package api

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"miestudio/backend-go/internal/auth"
)

// Reportes de ocupación (qué tanto se llena la agenda) y de retención (qué clientes vuelven).

const (
	lapsedDays   = 60 // sin venir en este tiempo, con al menos dos visitas, un cliente está «en riesgo»
	lapsedListed = 20
	topListed    = 10
)

type span struct{ start, end time.Time }

// mergeSpans junta los intervalos que se encimen, para no descontar dos veces el mismo tiempo bloqueado.
func mergeSpans(in []span) []span {
	sort.Slice(in, func(i, j int) bool { return in[i].start.Before(in[j].start) })
	var out []span
	for _, s := range in {
		if n := len(out); n > 0 && !s.start.After(out[n-1].end) {
			if s.end.After(out[n-1].end) {
				out[n-1].end = s.end
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

// freeMinutes es el tiempo del intervalo [start, end) que no cae en los bloques (ya combinados).
func freeMinutes(start, end time.Time, blocks []span) int {
	total := end.Sub(start)
	for _, b := range blocks {
		s, e := b.start, b.end
		if s.Before(start) {
			s = start
		}
		if e.After(end) {
			e = end
		}
		if e.After(s) {
			total -= e.Sub(s)
		}
	}
	return int(total.Minutes())
}

type occStaff struct {
	StaffID      int64  `json:"staff_id"`
	Name         string `json:"name"`
	AvailableMin int    `json:"available_min"`
	BookedMin    int    `json:"booked_min"`
	Appointments int    `json:"appointments"`
	NoShows      int    `json:"no_shows"`
}

type occDay struct {
	Weekday      int `json:"weekday"` // 0 = domingo
	AvailableMin int `json:"available_min"`
	BookedMin    int `json:"booked_min"`
}

type occHeat struct {
	Weekday int `json:"weekday"`
	Hour    int `json:"hour"`
	Count   int `json:"count"`
}

// occupancyReport compara el tiempo que cada persona tiene disponible (su horario menos sus bloqueos) con el que
// tiene agendado. Cuenta las citas agendadas y las terminadas; las canceladas y las inasistencias se informan aparte.
func (a *API) occupancyReport(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	from, to, fromS, toS, ok := a.dateRange(ctx, p.BusinessID, r.URL.Query())
	if !ok {
		writeErr(w, http.StatusBadRequest, "rango de fechas inválido (AAAA-MM-DD, hasta un año)")
		return
	}
	branchID, _ := strconv.ParseInt(r.URL.Query().Get("branch_id"), 10, 64)
	loc := a.businessLoc(ctx, p.BusinessID)

	// horario semanal de cada persona (en minutos desde la medianoche)
	type hours struct{ weekday, start, end int }
	byStaff := map[int64][]hours{}
	names := map[int64]string{}
	var order []int64
	rows, err := a.DB.Query(ctx,
		`SELECT u.id, u.name, h.weekday, (EXTRACT(EPOCH FROM h.start_time) / 60)::int, (EXTRACT(EPOCH FROM h.end_time) / 60)::int
		   FROM working_hours h JOIN users u ON u.id = h.user_id
		  WHERE u.business_id = $1 AND u.active AND ($2 = 0 OR h.branch_id = $2) ORDER BY u.name, u.id, h.weekday, h.start_time`, p.BusinessID, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	for rows.Next() {
		var id int64
		var name string
		var h hours
		if err := rows.Scan(&id, &name, &h.weekday, &h.start, &h.end); err != nil {
			rows.Close()
			serverErr(w)
			return
		}
		if _, seen := names[id]; !seen {
			names[id] = name
			order = append(order, id)
		}
		byStaff[id] = append(byStaff[id], h)
	}
	rows.Close()

	// bloqueos de esa gente dentro del periodo
	blocks := map[int64][]span{}
	brows, err := a.DB.Query(ctx,
		`SELECT b.user_id, b.starts_at, b.ends_at FROM time_blocks b JOIN users u ON u.id = b.user_id
		  WHERE u.business_id = $1 AND b.ends_at > $2 AND b.starts_at < $3`, p.BusinessID, from, to)
	if err != nil {
		serverErr(w)
		return
	}
	for brows.Next() {
		var id int64
		var s span
		if err := brows.Scan(&id, &s.start, &s.end); err != nil {
			brows.Close()
			serverErr(w)
			return
		}
		blocks[id] = append(blocks[id], s)
	}
	brows.Close()
	for id := range blocks {
		blocks[id] = mergeSpans(blocks[id])
	}

	stats := map[int64]*occStaff{}
	for _, id := range order {
		stats[id] = &occStaff{StaffID: id, Name: names[id]}
	}
	var days [7]occDay
	for i := range days {
		days[i].Weekday = i
	}
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		wd := int(d.Weekday())
		for _, id := range order {
			for _, h := range byStaff[id] {
				if h.weekday != wd {
					continue
				}
				start := time.Date(d.Year(), d.Month(), d.Day(), 0, h.start, 0, 0, loc)
				end := time.Date(d.Year(), d.Month(), d.Day(), 0, h.end, 0, 0, loc)
				m := freeMinutes(start, end, blocks[id])
				stats[id].AvailableMin += m
				days[wd].AvailableMin += m
			}
		}
	}

	// citas del periodo
	arows, err := a.DB.Query(ctx,
		`SELECT ap.staff_id, ap.status, COALESCE((EXTRACT(EPOCH FROM (ap.ends_at - ap.starts_at)) / 60)::int, 0),
		        EXTRACT(DOW FROM ap.starts_at AT TIME ZONE $5)::int, EXTRACT(HOUR FROM ap.starts_at AT TIME ZONE $5)::int
		   FROM appointments ap
		  WHERE ap.business_id = $1 AND ap.starts_at >= $2 AND ap.starts_at < $3 AND ($4 = 0 OR ap.branch_id = $4)
		    AND ap.status IN ('scheduled', 'completed', 'no_show', 'cancelled')`, p.BusinessID, from, to, branchID, loc.String())
	if err != nil {
		serverErr(w)
		return
	}
	defer arows.Close()
	var out struct {
		From         string     `json:"from"`
		To           string     `json:"to"`
		AvailableMin int        `json:"available_min"`
		BookedMin    int        `json:"booked_min"`
		Occupancy    *float64   `json:"occupancy_pct"` // null si no hay horario cargado
		Appointments int        `json:"appointments"`  // agendadas y terminadas
		NoShows      int        `json:"no_shows"`
		Cancelled    int        `json:"cancelled"`
		NoShowRate   *float64   `json:"no_show_pct"`
		Staff        []occStaff `json:"staff"`
		ByWeekday    []occDay   `json:"by_weekday"`
		Heat         []occHeat  `json:"heat"`
	}
	out.From, out.To = fromS, toS
	heat := map[[2]int]int{}
	for arows.Next() {
		var staffID int64
		var status string
		var minutes, dow, hour int
		if err := arows.Scan(&staffID, &status, &minutes, &dow, &hour); err != nil {
			serverErr(w)
			return
		}
		st := stats[staffID]
		switch status {
		case "cancelled":
			out.Cancelled++
			continue
		case "no_show":
			out.NoShows++
			if st != nil {
				st.NoShows++
			}
			continue
		}
		out.Appointments++
		out.BookedMin += minutes
		days[dow].BookedMin += minutes
		heat[[2]int{dow, hour}]++
		if st != nil {
			st.BookedMin += minutes
			st.Appointments++
		}
	}
	if arows.Err() != nil {
		serverErr(w)
		return
	}
	out.Staff = make([]occStaff, 0, len(order))
	for _, id := range order {
		out.AvailableMin += stats[id].AvailableMin
		out.Staff = append(out.Staff, *stats[id])
	}
	out.ByWeekday = days[:]
	out.Heat = []occHeat{}
	for k, n := range heat {
		out.Heat = append(out.Heat, occHeat{Weekday: k[0], Hour: k[1], Count: n})
	}
	sort.Slice(out.Heat, func(i, j int) bool {
		if out.Heat[i].Weekday != out.Heat[j].Weekday {
			return out.Heat[i].Weekday < out.Heat[j].Weekday
		}
		return out.Heat[i].Hour < out.Heat[j].Hour
	})
	if out.AvailableMin > 0 {
		v := float64(out.BookedMin) / float64(out.AvailableMin) * 100
		out.Occupancy = &v
	}
	if all := out.Appointments + out.NoShows; all > 0 {
		v := float64(out.NoShows) / float64(all) * 100
		out.NoShowRate = &v
	}
	writeJSON(w, http.StatusOK, out)
}

type retClient struct {
	ClientID  int64  `json:"client_id"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	LastVisit string `json:"last_visit"`
	Visits    int    `json:"visits"`
	Spent     int    `json:"spent_cents"`
	DaysAgo   int    `json:"days_ago"`
}

// retentionReport cuenta los clientes por sus visitas, que son los días en que se les cobró algo. Un cliente que solo
// compra de mostrador (sin ficha) no entra.
func (a *API) retentionReport(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	_, _, fromS, toS, ok := a.dateRange(ctx, p.BusinessID, r.URL.Query())
	if !ok {
		writeErr(w, http.StatusBadRequest, "rango de fechas inválido (AAAA-MM-DD, hasta un año)")
		return
	}
	branchID, _ := strconv.ParseInt(r.URL.Query().Get("branch_id"), 10, 64)
	loc := a.businessLoc(ctx, p.BusinessID)
	f, _ := time.ParseInLocation(dateLayout, fromS, loc)
	t, _ := time.ParseInLocation(dateLayout, toS, loc)
	length := int(t.Sub(f).Hours()/24) + 1
	prevFrom := f.AddDate(0, 0, -length).Format(dateLayout)
	prevTo := f.AddDate(0, 0, -1).Format(dateLayout)
	today := a.now().In(loc)

	rows, err := a.DB.Query(ctx,
		`WITH v AS (
		   SELECT s.client_id, (s.created_at AT TIME ZONE $2)::date AS d, sum(s.total_cents + s.tip_cents) AS cents
		     FROM sales s WHERE s.business_id = $1 AND s.status = 'paid' AND s.client_id IS NOT NULL AND ($3 = 0 OR s.branch_id = $3)
		    GROUP BY 1, 2)
		 SELECT v.client_id, c.name, c.phone, min(v.d), max(v.d), count(*), COALESCE(sum(v.cents), 0),
		        count(*) FILTER (WHERE v.d BETWEEN $4::date AND $5::date), COALESCE(sum(v.cents) FILTER (WHERE v.d BETWEEN $4::date AND $5::date), 0),
		        count(*) FILTER (WHERE v.d BETWEEN $6::date AND $7::date)
		   FROM v JOIN clients c ON c.id = v.client_id AND c.business_id = $1 AND c.anonymized_at IS NULL
		  GROUP BY v.client_id, c.name, c.phone LIMIT 50000`,
		p.BusinessID, loc.String(), branchID, fromS, toS, prevFrom, prevTo)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()

	var out struct {
		From         string      `json:"from"`
		To           string      `json:"to"`
		PrevFrom     string      `json:"prev_from"`
		PrevTo       string      `json:"prev_to"`
		Active       int         `json:"active"`    // clientes con visita en el periodo
		New          int         `json:"new"`       // su primera visita fue en el periodo
		Returning    int         `json:"returning"` // ya habían venido antes
		Repeaters    int         `json:"repeaters"` // dos visitas o más en el periodo
		VisitsPer    *float64    `json:"visits_per_client"`
		PrevActive   int         `json:"prev_active"` // clientes del periodo anterior
		Retained     int         `json:"retained"`    // de esos, los que volvieron en este periodo
		RetentionPct *float64    `json:"retention_pct"`
		AvgGapDays   *float64    `json:"avg_gap_days"`  // días entre visitas, de quienes han venido más de una vez
		OneVisit     int         `json:"one_visit"`     // clientes con una sola visita en toda su historia
		Total        int         `json:"total_clients"` // clientes con al menos una visita
		Lapsed       []retClient `json:"lapsed"`
		Top          []retClient `json:"top"`
		LapsedDays   int         `json:"lapsed_days"`
	}
	out.From, out.To, out.PrevFrom, out.PrevTo, out.LapsedDays = fromS, toS, prevFrom, prevTo, lapsedDays
	out.Lapsed, out.Top = []retClient{}, []retClient{}
	var visitsInRange, gapClients int
	var gapSum float64
	var lapsed, top []retClient
	for rows.Next() {
		var c retClient
		var first, last time.Time
		var spentIn, visitsIn, visitsPrev int
		if err := rows.Scan(&c.ClientID, &c.Name, &c.Phone, &first, &last, &c.Visits, &c.Spent, &visitsIn, &spentIn, &visitsPrev); err != nil {
			serverErr(w)
			return
		}
		out.Total++
		if c.Visits == 1 {
			out.OneVisit++
		}
		if c.Visits > 1 {
			gapClients++
			gapSum += last.Sub(first).Hours() / 24 / float64(c.Visits-1)
		}
		if visitsPrev > 0 {
			out.PrevActive++
			if visitsIn > 0 {
				out.Retained++
			}
		}
		if visitsIn > 0 {
			out.Active++
			visitsInRange += visitsIn
			if visitsIn >= 2 {
				out.Repeaters++
			}
			if first.Format(dateLayout) >= fromS {
				out.New++
			}
			c.Spent = spentIn // en la lista de mejores clientes, lo gastado en el periodo
			top = append(top, c)
		}
		c.LastVisit = last.Format(dateLayout)
		c.DaysAgo = int(today.Sub(time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, loc)).Hours() / 24)
		if c.Visits >= 2 && c.DaysAgo > lapsedDays {
			lapsed = append(lapsed, c)
		}
	}
	if rows.Err() != nil {
		serverErr(w)
		return
	}
	out.Returning = out.Active - out.New
	if out.Active > 0 {
		v := float64(visitsInRange) / float64(out.Active)
		out.VisitsPer = &v
	}
	if out.PrevActive > 0 {
		v := float64(out.Retained) / float64(out.PrevActive) * 100
		out.RetentionPct = &v
	}
	if gapClients > 0 {
		v := gapSum / float64(gapClients)
		out.AvgGapDays = &v
	}
	// los que se fueron hace menos tiempo se pueden recuperar más fácil: van primero
	sort.Slice(lapsed, func(i, j int) bool { return lapsed[i].DaysAgo < lapsed[j].DaysAgo })
	if len(lapsed) > lapsedListed {
		lapsed = lapsed[:lapsedListed]
	}
	sort.Slice(top, func(i, j int) bool { return top[i].Spent > top[j].Spent })
	if len(top) > topListed {
		top = top[:topListed]
	}
	out.Lapsed, out.Top = append(out.Lapsed, lapsed...), append(out.Top, top...)
	writeJSON(w, http.StatusOK, out)
}
