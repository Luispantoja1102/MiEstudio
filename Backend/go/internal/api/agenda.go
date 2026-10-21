package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

func (a *API) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// businessLoc devuelve la zona horaria del negocio (UTC si no se puede cargar).
func (a *API) businessLoc(ctx context.Context, bizID int64) *time.Location {
	var tz string
	if err := a.DB.QueryRow(ctx, `SELECT timezone FROM businesses WHERE id = $1`, bizID).Scan(&tz); err == nil {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return time.UTC
}

// activeBranch comprueba que la sucursal sea del negocio y esté activa.
// Recibe el ejecutor (pool o transacción): dentro de una transacción debe usar la misma conexión,
// pedir otra al pool puede agotarlo y bloquear el servidor cuando hay muchas peticiones a la vez.
func activeBranch(ctx context.Context, q queryer, bizID, branchID int64) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM branches WHERE id = $1 AND business_id = $2 AND active)`, branchID, bizID).Scan(&ok)
	return ok, err
}

type svcInfo struct {
	Duration, Buffer time.Duration
}

// activeService devuelve duración y preparación de un servicio activo del negocio.
func (a *API) activeService(ctx context.Context, bizID, serviceID int64) (*svcInfo, error) {
	var dur, buf int
	err := a.DB.QueryRow(ctx, `SELECT duration_min, buffer_min FROM services WHERE id = $1 AND business_id = $2 AND active`, serviceID, bizID).Scan(&dur, &buf)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &svcInfo{time.Duration(dur) * time.Minute, time.Duration(buf) * time.Minute}, err
}

// canPerform dice si la persona puede atender el servicio: activa, en agenda y asignada
// (si el servicio no tiene asignaciones, cualquier persona en agenda puede hacerlo).
func (a *API) canPerform(ctx context.Context, bizID, serviceID, staffID int64) (bool, error) {
	var ok bool
	err := a.DB.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM users u WHERE u.id = $3 AND u.business_id = $1 AND u.active AND u.bookable
		   AND (NOT EXISTS (SELECT 1 FROM service_staff WHERE service_id = $2)
		        OR EXISTS (SELECT 1 FROM service_staff ss WHERE ss.service_id = $2 AND ss.user_id = u.id)))`,
		bizID, serviceID, staffID).Scan(&ok)
	return ok, err
}

func parseClock(s string) (h, m int, ok bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, false
	}
	return t.Hour(), t.Minute(), true
}

// ---------- horarios libres ----------

type staffSlots struct {
	StaffID   int64    `json:"staff_id"`
	StaffName string   `json:"staff_name"`
	Starts    []string `json:"starts"`
}

// computeSlots calcula los horarios libres de un servicio en un día, por persona.
// earliest es la primera hora que se puede ofrecer (ahora, o ahora más el aviso mínimo de la reserva en línea).
// onlyPublic limita a los servicios y personas que se ofrecen en línea.
func (a *API) computeSlots(ctx context.Context, bizID int64, svc *svcInfo, serviceID, branchID, onlyStaff int64, day time.Time, loc *time.Location, earliest time.Time) ([]staffSlots, error) {
	rows, err := a.DB.Query(ctx,
		`SELECT u.id, u.name FROM users u
		 WHERE u.business_id = $1 AND u.active AND u.bookable AND ($3 = 0 OR u.id = $3)
		   AND (NOT EXISTS (SELECT 1 FROM service_staff WHERE service_id = $2)
		        OR EXISTS (SELECT 1 FROM service_staff ss WHERE ss.service_id = $2 AND ss.user_id = u.id))
		 ORDER BY u.name`, bizID, serviceID, onlyStaff)
	if err != nil {
		return nil, err
	}
	type person struct {
		id   int64
		name string
	}
	var people []person
	ids := []int64{}
	for rows.Next() {
		var x person
		if err := rows.Scan(&x.id, &x.name); err != nil {
			rows.Close()
			return nil, err
		}
		people = append(people, x)
		ids = append(ids, x.id)
	}
	rows.Close()

	hours, err := a.workingRanges(ctx, ids, branchID, int(day.Weekday()))
	if err != nil {
		return nil, err
	}
	busy, err := a.busyIntervals(ctx, bizID, ids, day, day.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	out := []staffSlots{}
	for _, x := range people {
		free := freeSlots(loc, day, hours[x.id], svc.Duration, svc.Buffer, busy[x.id], earliest)
		if len(free) == 0 {
			continue
		}
		s := staffSlots{StaffID: x.id, StaffName: x.name, Starts: make([]string, len(free))}
		for i, t := range free {
			s.Starts[i] = t.Format(time.RFC3339)
		}
		out = append(out, s)
	}
	return out, nil
}

func (a *API) slots(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	q := r.URL.Query()
	serviceID, _ := strconv.ParseInt(q.Get("service_id"), 10, 64)
	branchID, _ := strconv.ParseInt(q.Get("branch_id"), 10, 64)
	onlyStaff, _ := strconv.ParseInt(q.Get("staff_id"), 10, 64)
	loc := a.businessLoc(ctx, p.BusinessID)
	day, err := time.ParseInLocation(dateLayout, q.Get("date"), loc)
	if err != nil || serviceID <= 0 || branchID <= 0 {
		writeErr(w, http.StatusBadRequest, "indica service_id, branch_id y date (AAAA-MM-DD)")
		return
	}
	svc, err := a.activeService(ctx, p.BusinessID, serviceID)
	if err != nil {
		serverErr(w)
		return
	}
	okBranch, err := activeBranch(ctx, a.DB, p.BusinessID, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	if svc == nil || !okBranch {
		writeErr(w, http.StatusNotFound, "servicio o sucursal no encontrados")
		return
	}
	out, err := a.computeSlots(ctx, p.BusinessID, svc, serviceID, branchID, onlyStaff, day, loc, a.now())
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": q.Get("date"), "timezone": loc.String(), "staff": out})
}

func (a *API) workingRanges(ctx context.Context, userIDs []int64, branchID int64, weekday int) (map[int64][]clockRange, error) {
	rows, err := a.DB.Query(ctx,
		`SELECT user_id, to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI') FROM working_hours
		 WHERE user_id = ANY($1) AND branch_id = $2 AND weekday = $3 ORDER BY start_time`, userIDs, branchID, weekday)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]clockRange{}
	for rows.Next() {
		var uid int64
		var s, e string
		if err := rows.Scan(&uid, &s, &e); err != nil {
			return nil, err
		}
		sh, sm, _ := parseClock(s)
		eh, em, _ := parseClock(e)
		out[uid] = append(out[uid], clockRange{sh, sm, eh, em})
	}
	return out, rows.Err()
}

// busyIntervals reúne citas (con su preparación) y bloqueos que tocan [from, to).
func (a *API) busyIntervals(ctx context.Context, bizID int64, userIDs []int64, from, to time.Time) (map[int64][]interval, error) {
	out := map[int64][]interval{}
	rows, err := a.DB.Query(ctx,
		`SELECT staff_id, starts_at, blocked_until FROM appointments
		 WHERE business_id = $1 AND staff_id = ANY($2) AND status IN ('scheduled', 'completed', 'pending_payment')
		   AND starts_at < $4 AND blocked_until > $3`, bizID, userIDs, from, to)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var uid int64
		var iv interval
		if err := rows.Scan(&uid, &iv.start, &iv.end); err != nil {
			rows.Close()
			return nil, err
		}
		out[uid] = append(out[uid], iv)
	}
	rows.Close()

	rows, err = a.DB.Query(ctx,
		`SELECT b.user_id, b.starts_at, b.ends_at FROM time_blocks b JOIN users u ON u.id = b.user_id
		 WHERE u.business_id = $1 AND b.user_id = ANY($2) AND b.starts_at < $4 AND b.ends_at > $3`, bizID, userIDs, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var uid int64
		var iv interval
		if err := rows.Scan(&uid, &iv.start, &iv.end); err != nil {
			return nil, err
		}
		out[uid] = append(out[uid], iv)
	}
	return out, rows.Err()
}

// ---------- vista del día ----------

type apptRow struct {
	ID          int64     `json:"id"`
	ClientID    int64     `json:"client_id"`
	ClientName  string    `json:"client_name"`
	ServiceID   int64     `json:"service_id"`
	ServiceName string    `json:"service_name"`
	PriceCents  int       `json:"price_cents"`
	StaffID     int64     `json:"staff_id"`
	StaffName   string    `json:"staff_name"`
	BranchID    int64     `json:"branch_id"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Status      string    `json:"status"`
	Notes       string    `json:"notes"`
}

type blockRow struct {
	ID       int64     `json:"id"`
	UserID   int64     `json:"user_id"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Reason   string    `json:"reason"`
}

// agendaDay devuelve, para un día y una sucursal, quién trabaja (con su horario), las citas y los bloqueos.
func (a *API) agendaDay(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	loc := a.businessLoc(ctx, p.BusinessID)
	branchID, _ := strconv.ParseInt(r.URL.Query().Get("branch_id"), 10, 64)
	day, err := time.ParseInLocation(dateLayout, r.URL.Query().Get("date"), loc)
	if err != nil || branchID <= 0 {
		writeErr(w, http.StatusBadRequest, "indica branch_id y date (AAAA-MM-DD)")
		return
	}
	ok, err := activeBranch(ctx, a.DB, p.BusinessID, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "sucursal no encontrada")
		return
	}
	next := day.AddDate(0, 0, 1)

	// personas en agenda (todas las activas; se marca su horario de ese día en esta sucursal)
	urows, err := a.DB.Query(ctx, `SELECT id, name FROM users WHERE business_id = $1 AND active AND bookable ORDER BY name`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	type staffOut struct {
		ID    int64               `json:"id"`
		Name  string              `json:"name"`
		Hours []map[string]string `json:"hours"`
	}
	staff := []staffOut{}
	ids := []int64{}
	for urows.Next() {
		var s staffOut
		if err := urows.Scan(&s.ID, &s.Name); err != nil {
			urows.Close()
			serverErr(w)
			return
		}
		s.Hours = []map[string]string{}
		staff = append(staff, s)
		ids = append(ids, s.ID)
	}
	urows.Close()

	hrows, err := a.DB.Query(ctx,
		`SELECT user_id, to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI') FROM working_hours
		 WHERE user_id = ANY($1) AND branch_id = $2 AND weekday = $3 ORDER BY start_time`, ids, branchID, int(day.Weekday()))
	if err != nil {
		serverErr(w)
		return
	}
	for hrows.Next() {
		var uid int64
		var s, e string
		if err := hrows.Scan(&uid, &s, &e); err != nil {
			hrows.Close()
			serverErr(w)
			return
		}
		for i := range staff {
			if staff[i].ID == uid {
				staff[i].Hours = append(staff[i].Hours, map[string]string{"start": s, "end": e})
			}
		}
	}
	hrows.Close()

	arows, err := a.DB.Query(ctx,
		`SELECT ap.id, ap.client_id, c.name, ap.service_id, s.name, s.price_cents, ap.staff_id, u.name,
		        ap.branch_id, ap.starts_at, ap.ends_at, ap.status, ap.notes
		 FROM appointments ap JOIN clients c ON c.id = ap.client_id JOIN services s ON s.id = ap.service_id
		 JOIN users u ON u.id = ap.staff_id
		 WHERE ap.business_id = $1 AND ap.branch_id = $2 AND ap.starts_at >= $3 AND ap.starts_at < $4
		 ORDER BY ap.starts_at, ap.id`, p.BusinessID, branchID, day, next)
	if err != nil {
		serverErr(w)
		return
	}
	appts := []apptRow{}
	for arows.Next() {
		var x apptRow
		if err := arows.Scan(&x.ID, &x.ClientID, &x.ClientName, &x.ServiceID, &x.ServiceName, &x.PriceCents, &x.StaffID, &x.StaffName,
			&x.BranchID, &x.StartsAt, &x.EndsAt, &x.Status, &x.Notes); err != nil {
			arows.Close()
			serverErr(w)
			return
		}
		appts = append(appts, x)
	}
	arows.Close()

	brows, err := a.DB.Query(ctx,
		`SELECT b.id, b.user_id, b.starts_at, b.ends_at, b.reason FROM time_blocks b JOIN users u ON u.id = b.user_id
		 WHERE u.business_id = $1 AND b.starts_at < $3 AND b.ends_at > $2 ORDER BY b.starts_at`, p.BusinessID, day, next)
	if err != nil {
		serverErr(w)
		return
	}
	defer brows.Close()
	blocks := []blockRow{}
	for brows.Next() {
		var x blockRow
		if err := brows.Scan(&x.ID, &x.UserID, &x.StartsAt, &x.EndsAt, &x.Reason); err != nil {
			serverErr(w)
			return
		}
		blocks = append(blocks, x)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date": r.URL.Query().Get("date"), "timezone": loc.String(), "staff": staff, "appointments": appts, "blocks": blocks,
	})
}

// ---------- validación de un horario para una cita ----------

type bookingProblem struct {
	Status int
	Msg    string
}

// checkSlot valida que [start, end) esté dentro del horario de la persona en la sucursal
// y que no choque con un bloqueo. El choque con otras citas lo garantiza la base de datos.
func (a *API) checkSlot(ctx context.Context, loc *time.Location, staffID, branchID int64, start, end time.Time) (*bookingProblem, error) {
	ls, le := start.In(loc), end.In(loc)
	if ls.YearDay() != le.YearDay() || ls.Year() != le.Year() {
		return &bookingProblem{http.StatusBadRequest, "la cita no puede cruzar la medianoche"}, nil
	}
	var inHours bool
	if err := a.DB.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM working_hours WHERE user_id = $1 AND branch_id = $2 AND weekday = $3
		   AND start_time <= $4::time AND end_time >= $5::time)`,
		staffID, branchID, int(ls.Weekday()), ls.Format("15:04:05"), le.Format("15:04:05")).Scan(&inHours); err != nil {
		return nil, err
	}
	if !inHours {
		return &bookingProblem{http.StatusBadRequest, "ese horario está fuera del horario de trabajo de la persona"}, nil
	}
	var blocked bool
	if err := a.DB.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM time_blocks WHERE user_id = $1 AND tstzrange(starts_at, ends_at) && tstzrange($2, $3))`,
		staffID, start, end).Scan(&blocked); err != nil {
		return nil, err
	}
	if blocked {
		return &bookingProblem{http.StatusConflict, "la persona tiene ese horario bloqueado"}, nil
	}
	return nil, nil
}

func isExclusionViolation(err error) bool {
	code, _ := pgConstraint(err)
	return code == "23P01"
}

// ---------- citas ----------

func (a *API) createAppointment(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	var req struct {
		ClientID  int64  `json:"client_id"`
		ServiceID int64  `json:"service_id"`
		StaffID   int64  `json:"staff_id"`
		BranchID  int64  `json:"branch_id"`
		StartsAt  string `json:"starts_at"`
		Notes     string `json:"notes"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	start, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil || len(req.Notes) > 1000 {
		writeErr(w, http.StatusBadRequest, "fecha u hora inválida (formato RFC 3339) o notas demasiado largas")
		return
	}
	svc, err := a.activeService(ctx, p.BusinessID, req.ServiceID)
	if err != nil {
		serverErr(w)
		return
	}
	var clientOK bool
	if err := a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM clients WHERE id = $1 AND business_id = $2)`, req.ClientID, p.BusinessID).Scan(&clientOK); err != nil {
		serverErr(w)
		return
	}
	branchOK, err := activeBranch(ctx, a.DB, p.BusinessID, req.BranchID)
	if err != nil {
		serverErr(w)
		return
	}
	staffOK := false
	if svc != nil {
		if staffOK, err = a.canPerform(ctx, p.BusinessID, req.ServiceID, req.StaffID); err != nil {
			serverErr(w)
			return
		}
	}
	if svc == nil || !clientOK || !branchOK || !staffOK {
		writeErr(w, http.StatusBadRequest, "cliente, servicio, profesional o sucursal inválidos")
		return
	}

	end := start.Add(svc.Duration)
	loc := a.businessLoc(ctx, p.BusinessID)
	if prob, err := a.checkSlot(ctx, loc, req.StaffID, req.BranchID, start, end); err != nil {
		serverErr(w)
		return
	} else if prob != nil {
		writeErr(w, prob.Status, prob.Msg)
		return
	}

	var id int64
	err = a.DB.QueryRow(ctx,
		`INSERT INTO appointments (business_id, branch_id, client_id, staff_id, service_id, starts_at, ends_at, blocked_until, notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		p.BusinessID, req.BranchID, req.ClientID, req.StaffID, req.ServiceID, start, end, end.Add(svc.Buffer), strings.TrimSpace(req.Notes)).Scan(&id)
	if isExclusionViolation(err) {
		writeErr(w, http.StatusConflict, "ese horario ya está ocupado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

var transitions = map[string]bool{"completed": true, "no_show": true, "cancelled": true}

// updateAppointment cambia el estado (solo desde «scheduled») o las notas.
func (a *API) updateAppointment(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Status *string `json:"status"`
		Notes  *string `json:"notes"`
	}
	if !decode(w, r, &req) || (req.Status == nil && req.Notes == nil) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if (req.Status != nil && !transitions[*req.Status]) || (req.Notes != nil && len(*req.Notes) > 1000) {
		writeErr(w, http.StatusBadRequest, "estado inválido o notas demasiado largas")
		return
	}
	ctx := r.Context()
	var current string
	err := a.DB.QueryRow(ctx, `SELECT status FROM appointments WHERE id = $1 AND business_id = $2`, id, p.BusinessID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	if req.Status != nil && current != "scheduled" {
		writeErr(w, http.StatusConflict, "solo se puede cambiar el estado de una cita agendada")
		return
	}
	if _, err := a.DB.Exec(ctx,
		`UPDATE appointments SET status = COALESCE($3, status), notes = COALESCE($4, notes) WHERE id = $1 AND business_id = $2`,
		id, p.BusinessID, req.Status, req.Notes); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// moveAppointment reprograma una cita agendada (otra hora y/o otro profesional).
func (a *API) moveAppointment(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		StartsAt string `json:"starts_at"`
		StaffID  *int64 `json:"staff_id"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	start, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "fecha u hora inválida (formato RFC 3339)")
		return
	}
	var status string
	var staffID, branchID, serviceID int64
	var oldStart, oldEnd, oldBlocked time.Time
	err = a.DB.QueryRow(ctx,
		`SELECT status, staff_id, branch_id, service_id, starts_at, ends_at, blocked_until FROM appointments WHERE id = $1 AND business_id = $2`,
		id, p.BusinessID).Scan(&status, &staffID, &branchID, &serviceID, &oldStart, &oldEnd, &oldBlocked)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	if status != "scheduled" {
		writeErr(w, http.StatusConflict, "solo se puede reprogramar una cita agendada")
		return
	}
	if req.StaffID != nil && *req.StaffID != staffID {
		staffID = *req.StaffID
		if okStaff, err := a.canPerform(ctx, p.BusinessID, serviceID, staffID); err != nil {
			serverErr(w)
			return
		} else if !okStaff {
			writeErr(w, http.StatusBadRequest, "esa persona no puede atender este servicio")
			return
		}
	}
	// conserva la duración y la preparación con las que se agendó
	end := start.Add(oldEnd.Sub(oldStart))
	blocked := end.Add(oldBlocked.Sub(oldEnd))
	loc := a.businessLoc(ctx, p.BusinessID)
	if prob, err := a.checkSlot(ctx, loc, staffID, branchID, start, end); err != nil {
		serverErr(w)
		return
	} else if prob != nil {
		writeErr(w, prob.Status, prob.Msg)
		return
	}
	_, err = a.DB.Exec(ctx,
		`UPDATE appointments SET starts_at = $3, ends_at = $4, blocked_until = $5, staff_id = $6 WHERE id = $1 AND business_id = $2`,
		id, p.BusinessID, start, end, blocked, staffID)
	if isExclusionViolation(err) {
		writeErr(w, http.StatusConflict, "ese horario ya está ocupado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- bloqueos de horario ----------

func (a *API) createBlock(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	var req struct {
		UserID   int64  `json:"user_id"`
		StartsAt string `json:"starts_at"`
		EndsAt   string `json:"ends_at"`
		Reason   string `json:"reason"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	start, err1 := time.Parse(time.RFC3339, req.StartsAt)
	end, err2 := time.Parse(time.RFC3339, req.EndsAt)
	if err1 != nil || err2 != nil || !end.After(start) || end.Sub(start) > 31*24*time.Hour || len(req.Reason) > 200 {
		writeErr(w, http.StatusBadRequest, "horario inválido: el fin debe ser después del inicio")
		return
	}
	// el profesional solo bloquea su propio horario
	if p.Role == "staff" && req.UserID != p.UserID {
		writeErr(w, http.StatusForbidden, "solo puedes bloquear tu propio horario")
		return
	}
	var userOK bool
	if err := a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND business_id = $2 AND active)`, req.UserID, p.BusinessID).Scan(&userOK); err != nil {
		serverErr(w)
		return
	}
	if !userOK {
		writeErr(w, http.StatusBadRequest, "persona inválida")
		return
	}
	var clash bool
	if err := a.DB.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM appointments WHERE staff_id = $1 AND status IN ('scheduled', 'pending_payment')
		   AND tstzrange(starts_at, blocked_until) && tstzrange($2, $3))`, req.UserID, start, end).Scan(&clash); err != nil {
		serverErr(w)
		return
	}
	if clash {
		writeErr(w, http.StatusConflict, "hay citas agendadas en ese horario: reprográmalas o cancélalas primero")
		return
	}
	var id int64
	if err := a.DB.QueryRow(ctx,
		`INSERT INTO time_blocks (user_id, starts_at, ends_at, reason) VALUES ($1, $2, $3, $4) RETURNING id`,
		req.UserID, start, end, strings.TrimSpace(req.Reason)).Scan(&id); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) deleteBlock(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	// el profesional solo quita sus propios bloqueos
	tag, err := a.DB.Exec(r.Context(),
		`DELETE FROM time_blocks b USING users u WHERE b.id = $1 AND u.id = b.user_id AND u.business_id = $2
		   AND ($3 <> 'staff' OR b.user_id = $4)`, id, p.BusinessID, p.Role, p.UserID)
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
