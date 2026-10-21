package api

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"miestudio/backend-go/internal/auth"
)

type staffReport struct {
	StaffID         int64  `json:"staff_id"`
	Name            string `json:"name"`
	Items           int    `json:"items"`
	RevenueCents    int    `json:"revenue_cents"` // lo cobrado por sus partidas, ya con descuentos
	CommissionCents int    `json:"commission_cents"`
	TipCents        int    `json:"tip_cents"`
}

type itemReport struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Qty        int    `json:"qty"`
	TotalCents int    `json:"total_cents"`
}

type dayReport struct {
	Date       string `json:"date"`
	Sales      int    `json:"sales"`
	TotalCents int    `json:"total_cents"`
}

// salesReport resume las ventas pagadas de un periodo (las canceladas no cuentan).
func (a *API) salesReport(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	from, to, fromS, toS, ok := a.dateRange(ctx, p.BusinessID, r.URL.Query())
	if !ok {
		writeErr(w, http.StatusBadRequest, "rango de fechas inválido (AAAA-MM-DD, hasta un año)")
		return
	}
	branchID, _ := strconv.ParseInt(r.URL.Query().Get("branch_id"), 10, 64)
	loc := a.businessLoc(ctx, p.BusinessID)

	var out struct {
		From           string        `json:"from"`
		To             string        `json:"to"`
		Sales          int           `json:"sales"`
		SubtotalCents  int           `json:"subtotal_cents"`
		DiscountCents  int           `json:"discount_cents"`
		TotalCents     int           `json:"total_cents"` // vendido, sin propinas
		TipCents       int           `json:"tip_cents"`
		CollectedCents int           `json:"collected_cents"` // total + propinas
		CostCents      int           `json:"cost_cents"`      // costo de los productos vendidos
		ByMethod       []methodTotal `json:"by_method"`
		ByStaff        []staffReport `json:"by_staff"`
		TopItems       []itemReport  `json:"top_items"`
		ByDay          []dayReport   `json:"by_day"`
	}
	out.From, out.To = fromS, toS
	out.ByMethod, out.ByStaff, out.TopItems, out.ByDay = []methodTotal{}, []staffReport{}, []itemReport{}, []dayReport{}

	const where = `s.business_id = $1 AND s.status = 'paid' AND s.created_at >= $2 AND s.created_at < $3 AND ($4 = 0 OR s.branch_id = $4)`

	if err := a.DB.QueryRow(ctx,
		`SELECT count(*), COALESCE(sum(s.subtotal_cents), 0), COALESCE(sum(s.discount_cents), 0), COALESCE(sum(s.total_cents), 0), COALESCE(sum(s.tip_cents), 0)
		 FROM sales s WHERE `+where, p.BusinessID, from, to, branchID).
		Scan(&out.Sales, &out.SubtotalCents, &out.DiscountCents, &out.TotalCents, &out.TipCents); err != nil {
		serverErr(w)
		return
	}
	out.CollectedCents = out.TotalCents + out.TipCents
	if err := a.DB.QueryRow(ctx,
		`SELECT COALESCE(sum(i.unit_cost_cents * i.qty), 0) FROM sale_items i JOIN sales s ON s.id = i.sale_id
		 WHERE i.product_id IS NOT NULL AND `+where, p.BusinessID, from, to, branchID).Scan(&out.CostCents); err != nil {
		serverErr(w)
		return
	}

	rows, err := a.DB.Query(ctx,
		`SELECT sp.method, sum(sp.amount_cents), count(*) FROM sale_payments sp JOIN sales s ON s.id = sp.sale_id
		 WHERE `+where+` GROUP BY sp.method ORDER BY sp.method`, p.BusinessID, from, to, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	for rows.Next() {
		var m methodTotal
		if err := rows.Scan(&m.Method, &m.AmountCents, &m.Count); err != nil {
			rows.Close()
			serverErr(w)
			return
		}
		out.ByMethod = append(out.ByMethod, m)
	}
	rows.Close()

	staff := map[int64]*staffReport{}
	get := func(id int64, name string) *staffReport {
		if staff[id] == nil {
			staff[id] = &staffReport{StaffID: id, Name: name}
		}
		return staff[id]
	}
	srows, err := a.DB.Query(ctx,
		`SELECT i.staff_id, u.name, count(*), sum(i.commission_base_cents), sum(i.commission_cents)
		 FROM sale_items i JOIN sales s ON s.id = i.sale_id JOIN users u ON u.id = i.staff_id
		 WHERE i.staff_id IS NOT NULL AND `+where+` GROUP BY i.staff_id, u.name`, p.BusinessID, from, to, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	for srows.Next() {
		var id int64
		var name string
		var n, rev, com int
		if err := srows.Scan(&id, &name, &n, &rev, &com); err != nil {
			srows.Close()
			serverErr(w)
			return
		}
		x := get(id, name)
		x.Items, x.RevenueCents, x.CommissionCents = n, rev, com
	}
	srows.Close()
	trows, err := a.DB.Query(ctx,
		`SELECT s.tip_staff_id, u.name, sum(s.tip_cents) FROM sales s JOIN users u ON u.id = s.tip_staff_id
		 WHERE s.tip_staff_id IS NOT NULL AND `+where+` GROUP BY s.tip_staff_id, u.name`, p.BusinessID, from, to, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	for trows.Next() {
		var id int64
		var name string
		var tip int
		if err := trows.Scan(&id, &name, &tip); err != nil {
			trows.Close()
			serverErr(w)
			return
		}
		get(id, name).TipCents = tip
	}
	trows.Close()
	for _, x := range staff {
		out.ByStaff = append(out.ByStaff, *x)
	}
	sort.Slice(out.ByStaff, func(i, j int) bool {
		if out.ByStaff[i].RevenueCents != out.ByStaff[j].RevenueCents {
			return out.ByStaff[i].RevenueCents > out.ByStaff[j].RevenueCents
		}
		return out.ByStaff[i].Name < out.ByStaff[j].Name
	})

	irows, err := a.DB.Query(ctx,
		`SELECT i.name, CASE WHEN i.service_id IS NOT NULL THEN 'service' ELSE 'product' END, sum(i.qty), sum(i.total_cents)
		 FROM sale_items i JOIN sales s ON s.id = i.sale_id WHERE `+where+`
		 GROUP BY i.name, 2 ORDER BY sum(i.total_cents) DESC, i.name LIMIT 10`, p.BusinessID, from, to, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	for irows.Next() {
		var it itemReport
		if err := irows.Scan(&it.Name, &it.Kind, &it.Qty, &it.TotalCents); err != nil {
			irows.Close()
			serverErr(w)
			return
		}
		out.TopItems = append(out.TopItems, it)
	}
	irows.Close()

	drows, err := a.DB.Query(ctx,
		`SELECT to_char(s.created_at AT TIME ZONE $5, 'YYYY-MM-DD') AS d, count(*), sum(s.total_cents)
		 FROM sales s WHERE `+where+` GROUP BY d ORDER BY d`, p.BusinessID, from, to, branchID, loc.String())
	if err != nil {
		serverErr(w)
		return
	}
	defer drows.Close()
	for drows.Next() {
		var d dayReport
		if err := drows.Scan(&d.Date, &d.Sales, &d.TotalCents); err != nil {
			serverErr(w)
			return
		}
		out.ByDay = append(out.ByDay, d)
	}
	writeJSON(w, http.StatusOK, out)
}

type commissionLine struct {
	SaleID          int64     `json:"sale_id"`
	Folio           int       `json:"folio"`
	CreatedAt       time.Time `json:"created_at"`
	Item            string    `json:"item"`
	Qty             int       `json:"qty"`
	BaseCents       int       `json:"base_cents"`
	Pct             float64   `json:"pct"`
	CommissionCents int       `json:"commission_cents"`
}

// commissionsReport: el administrador ve a todas las personas; un profesional, solo lo suyo.
// Con staff_id (o siendo profesional) incluye el detalle partida por partida.
func (a *API) commissionsReport(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	from, to, fromS, toS, ok := a.dateRange(ctx, p.BusinessID, r.URL.Query())
	if !ok {
		writeErr(w, http.StatusBadRequest, "rango de fechas inválido (AAAA-MM-DD, hasta un año)")
		return
	}
	staffID, _ := strconv.ParseInt(r.URL.Query().Get("staff_id"), 10, 64)
	if p.Role != "admin" {
		staffID = p.UserID // un profesional no ve lo de los demás
	}

	rows, err := a.DB.Query(ctx,
		`SELECT i.staff_id, u.name, count(*), COALESCE(sum(i.commission_base_cents), 0), COALESCE(sum(i.commission_cents), 0)
		 FROM sale_items i JOIN sales s ON s.id = i.sale_id JOIN users u ON u.id = i.staff_id
		 WHERE s.business_id = $1 AND s.status = 'paid' AND s.created_at >= $2 AND s.created_at < $3
		   AND i.staff_id IS NOT NULL AND ($4 = 0 OR i.staff_id = $4)
		 GROUP BY i.staff_id, u.name ORDER BY u.name`, p.BusinessID, from, to, staffID)
	if err != nil {
		serverErr(w)
		return
	}
	people := []staffReport{}
	for rows.Next() {
		var x staffReport
		if err := rows.Scan(&x.StaffID, &x.Name, &x.Items, &x.RevenueCents, &x.CommissionCents); err != nil {
			rows.Close()
			serverErr(w)
			return
		}
		people = append(people, x)
	}
	rows.Close()
	// propinas del periodo
	for i := range people {
		if err := a.DB.QueryRow(ctx,
			`SELECT COALESCE(sum(tip_cents), 0) FROM sales WHERE business_id = $1 AND status = 'paid' AND tip_staff_id = $2 AND created_at >= $3 AND created_at < $4`,
			p.BusinessID, people[i].StaffID, from, to).Scan(&people[i].TipCents); err != nil {
			serverErr(w)
			return
		}
	}

	lines := []commissionLine{}
	if staffID != 0 {
		lrows, err := a.DB.Query(ctx,
			`SELECT s.id, s.folio, s.created_at, i.name, i.qty, i.commission_base_cents, i.commission_pct::float8, i.commission_cents
			 FROM sale_items i JOIN sales s ON s.id = i.sale_id
			 WHERE s.business_id = $1 AND s.status = 'paid' AND s.created_at >= $2 AND s.created_at < $3 AND i.staff_id = $4
			 ORDER BY s.created_at DESC, i.id DESC LIMIT 500`, p.BusinessID, from, to, staffID)
		if err != nil {
			serverErr(w)
			return
		}
		defer lrows.Close()
		for lrows.Next() {
			var l commissionLine
			if err := lrows.Scan(&l.SaleID, &l.Folio, &l.CreatedAt, &l.Item, &l.Qty, &l.BaseCents, &l.Pct, &l.CommissionCents); err != nil {
				serverErr(w)
				return
			}
			lines = append(lines, l)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"from": fromS, "to": toS, "staff": people, "lines": lines})
}
