package api

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

// Lista de espera. Quien no encuentra horario deja su servicio, día y (opcional) franja; cuando se libera un
// lugar —una cancelación, un bloqueo que se quita, un horario nuevo— se le avisa por correo. El aviso no
// aparta el horario: el primero que lo reserva se lo queda. Para que no se avise a veinte personas por un solo
// lugar, solo se avisa a tantas como horarios libres haya (descontando a quienes ya avisamos hace poco).

const (
	maxWaitDaysAhead     = 120
	maxOpenWaitPerPerson = 3
	waitNoticeCooldown   = 12 * time.Hour // un aviso reciente sigue «ocupando» un horario libre
	waitlistBatch        = 200
)

type waitIn struct {
	ClientID  int64  `json:"client_id"`
	BranchID  int64  `json:"branch_id"`
	ServiceID int64  `json:"service_id"`
	StaffID   int64  `json:"staff_id"` // 0 = con cualquiera
	Date      string `json:"date"`
	From      string `json:"from"` // «HH:MM», opcional
	To        string `json:"to"`
}

// window convierte la franja en minutos desde medianoche; ambos nulos si no hay franja.
func (in waitIn) window() (from, to *int, msg string) {
	if in.From == "" && in.To == "" {
		return nil, nil, ""
	}
	fh, fm, ok1 := parseClock(in.From)
	th, tm, ok2 := parseClock(in.To)
	if !ok1 || !ok2 {
		return nil, nil, "la franja debe ir de una hora a otra (HH:MM)"
	}
	f, t := fh*60+fm, th*60+tm
	if t == 0 {
		t = 1440
	}
	if f >= t {
		return nil, nil, "la franja debe terminar después de empezar"
	}
	return &f, &t, ""
}

type waitOut struct {
	ID          int64     `json:"id"`
	ClientID    int64     `json:"client_id"`
	ClientName  string    `json:"client_name"`
	ClientPhone string    `json:"client_phone"`
	ServiceName string    `json:"service_name"`
	StaffName   string    `json:"staff_name"`
	BranchID    int64     `json:"branch_id"`
	BranchName  string    `json:"branch_name"`
	Day         string    `json:"day"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Status      string    `json:"status"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
}

func clockOf(min *int) string {
	if min == nil {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", *min/60%24, *min%60)
}

func (a *API) listWaitlist(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := a.DB.Query(r.Context(),
		`SELECT w.id, w.client_id, c.name, c.phone, s.name, COALESCE(u.name, ''), w.branch_id, br.name, to_char(w.day, 'YYYY-MM-DD'),
		        w.from_min, w.to_min, w.status, w.source, w.created_at
		   FROM waitlist w JOIN clients c ON c.id = w.client_id JOIN services s ON s.id = w.service_id
		   JOIN branches br ON br.id = w.branch_id LEFT JOIN users u ON u.id = w.staff_id
		  WHERE w.business_id = $1 AND w.status IN ('waiting', 'notified')
		  ORDER BY w.day, w.created_at, w.id LIMIT 500`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []waitOut{}
	for rows.Next() {
		var x waitOut
		var from, to *int
		if err := rows.Scan(&x.ID, &x.ClientID, &x.ClientName, &x.ClientPhone, &x.ServiceName, &x.StaffName, &x.BranchID, &x.BranchName, &x.Day, &from, &to, &x.Status, &x.Source, &x.CreatedAt); err != nil {
			serverErr(w)
			return
		}
		x.From, x.To = clockOf(from), clockOf(to)
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, out)
}

// addWaitlist guarda la solicitud ya validada. Devuelve el token público de la solicitud.
func (a *API) addWaitlist(ctx context.Context, tx pgx.Tx, bizID, clientID int64, in waitIn, day time.Time, source string) (string, error) {
	from, to, _ := in.window()
	var staff *int64
	if in.StaffID > 0 {
		staff = &in.StaffID
	}
	var dup bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM waitlist WHERE business_id = $1 AND client_id = $2 AND service_id = $3 AND day = $4 AND status IN ('waiting', 'notified'))`,
		bizID, clientID, in.ServiceID, day.Format(dateLayout)).Scan(&dup); err != nil {
		return "", err
	}
	if dup {
		return "", fail(http.StatusConflict, "ya estás en la lista de espera de ese servicio y día")
	}
	var open int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM waitlist WHERE business_id = $1 AND client_id = $2 AND status IN ('waiting', 'notified')`, bizID, clientID).Scan(&open); err != nil {
		return "", err
	}
	if open >= maxOpenWaitPerPerson {
		return "", fail(http.StatusConflict, fmt.Sprintf("solo puedes estar en %d listas de espera a la vez", maxOpenWaitPerPerson))
	}
	token := newPublicToken()
	_, err := tx.Exec(ctx,
		`INSERT INTO waitlist (business_id, branch_id, service_id, staff_id, client_id, day, from_min, to_min, token, source, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		bizID, in.BranchID, in.ServiceID, staff, clientID, day.Format(dateLayout), from, to, token, source, a.now())
	return token, err
}

// checkWait valida servicio, sucursal, profesional, día y franja. online exige que el servicio se ofrezca en línea.
func (a *API) checkWait(ctx context.Context, bizID int64, in waitIn, loc *time.Location, online bool, horizon int) (time.Time, *httpFail) {
	day, err := time.ParseInLocation(dateLayout, in.Date, loc)
	if err != nil || in.ServiceID <= 0 || in.BranchID <= 0 {
		return day, fail(http.StatusBadRequest, "indica el servicio, la sucursal y el día")
	}
	if _, _, msg := in.window(); msg != "" {
		return day, fail(http.StatusBadRequest, msg)
	}
	now := a.now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if day.Before(today) || day.After(today.AddDate(0, 0, horizon)) {
		return day, fail(http.StatusBadRequest, "ese día no está disponible para la lista de espera")
	}
	var ok bool
	if err := a.DB.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM services WHERE id = $1 AND business_id = $2 AND active AND (online_bookable OR NOT $3))`,
		in.ServiceID, bizID, online).Scan(&ok); err != nil {
		return day, fail(http.StatusInternalServerError, "error")
	}
	branchOK, err := activeBranch(ctx, a.DB, bizID, in.BranchID)
	if err != nil {
		return day, fail(http.StatusInternalServerError, "error")
	}
	if !ok || !branchOK {
		return day, fail(http.StatusBadRequest, "servicio o sucursal no disponibles")
	}
	if in.StaffID > 0 {
		can, err := a.canPerform(ctx, bizID, in.ServiceID, in.StaffID)
		if err != nil {
			return day, fail(http.StatusInternalServerError, "error")
		}
		if !can {
			return day, fail(http.StatusBadRequest, "esa persona no atiende este servicio")
		}
	}
	return day, nil
}

func (a *API) createWaitlist(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	var in waitIn
	if !decode(w, r, &in) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	loc := a.businessLoc(ctx, p.BusinessID)
	day, f := a.checkWait(ctx, p.BusinessID, in, loc, false, maxWaitDaysAhead)
	if f != nil {
		writeFail(w, f)
		return
	}
	var email string
	var gone bool
	err := a.DB.QueryRow(ctx, `SELECT email, anonymized_at IS NOT NULL FROM clients WHERE id = $1 AND business_id = $2`, in.ClientID, p.BusinessID).Scan(&email, &gone)
	if errors.Is(err, pgx.ErrNoRows) || gone {
		writeErr(w, http.StatusBadRequest, "cliente inválido")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	if email == "" {
		writeErr(w, http.StatusBadRequest, "el cliente necesita un correo para avisarle cuando haya lugar")
		return
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := a.addWaitlist(ctx, tx, p.BusinessID, in.ClientID, in, day, "panel"); err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (a *API) deleteWaitlist(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE waitlist SET status = 'cancelled' WHERE id = $1 AND business_id = $2 AND status IN ('waiting', 'notified')`, id, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		notFound(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// publicWaitlist: la persona se anota desde la página de reservas.
func (a *API) publicWaitlist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "pub-wait:"+clientIP(r), 10, time.Hour) {
		return
	}
	b, err := a.publicBusiness(ctx, r.PathValue("slug"))
	if err != nil {
		serverErr(w)
		return
	}
	if b == nil {
		notFound(w)
		return
	}
	var req struct {
		waitIn
		Name          string `json:"name"`
		Phone         string `json:"phone"`
		Email         string `json:"email"`
		AcceptPrivacy bool   `json:"accept_privacy"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	digits := normalizePhone(strings.TrimSpace(req.Phone))
	switch nameLen := utf8.RuneCountInString(req.Name); {
	case nameLen < 2 || nameLen > 120:
		writeErr(w, http.StatusBadRequest, "escribe tu nombre")
		return
	case digits == "":
		writeErr(w, http.StatusBadRequest, "escribe un teléfono válido de 10 dígitos")
		return
	case !validEmail(req.Email):
		writeErr(w, http.StatusBadRequest, "escribe tu correo: por ahí te avisamos cuando haya lugar")
		return
	case !req.AcceptPrivacy:
		writeErr(w, http.StatusBadRequest, "debes aceptar el aviso de privacidad")
		return
	}
	if !a.allow(w, "pub-wait-phone:"+phoneKey(digits), 5, time.Hour) {
		return
	}
	day, f := a.checkWait(ctx, b.ID, req.waitIn, b.Loc, true, b.Settings.MaxDaysAhead)
	if f != nil {
		writeFail(w, f)
		return
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	clientID, err := findOrCreateClient(ctx, tx, b.ID, req.Name, digits, req.Email, false)
	if err != nil {
		serverErr(w)
		return
	}
	token, err := a.addWaitlist(ctx, tx, b.ID, clientID, req.waitIn, day, "online")
	if err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "mail": a.mailEnabled()})
}

// publicWaitlistInfo / publicWaitlistLeave: la persona ve su solicitud y sale de la lista con el enlace del correo.
func (a *API) publicWaitlistInfo(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !tokenShape.MatchString(token) {
		notFound(w)
		return
	}
	var out struct {
		Business string `json:"business"`
		Service  string `json:"service"`
		Day      string `json:"day"`
		Status   string `json:"status"`
	}
	err := a.DB.QueryRow(r.Context(),
		`SELECT b.name, s.name, to_char(w.day, 'YYYY-MM-DD'), w.status FROM waitlist w
		   JOIN businesses b ON b.id = w.business_id JOIN services s ON s.id = w.service_id WHERE w.token = $1`, token).
		Scan(&out.Business, &out.Service, &out.Day, &out.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) publicWaitlistLeave(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !tokenShape.MatchString(token) {
		notFound(w)
		return
	}
	tag, err := a.DB.Exec(r.Context(), `UPDATE waitlist SET status = 'cancelled' WHERE token = $1 AND status IN ('waiting', 'notified')`, token)
	if err != nil {
		serverErr(w)
		return
	}
	var exists bool
	if tag.RowsAffected() == 0 {
		if err := a.DB.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM waitlist WHERE token = $1)`, token).Scan(&exists); err != nil {
			serverErr(w)
			return
		}
		if !exists {
			notFound(w)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- aviso cuando se libera un lugar ----------

type waitCand struct {
	ID, BizID, BranchID, ServiceID, ClientID int64
	StaffID                                  *int64
	Day                                      string
	From, To                                 *int
	Email, ClientName, Token                 string
	Business, Slug, ServiceName              string
	TZ                                       string
	DurMin, BufMin                           int
	LinkOnline                               bool
	MinNotice                                int
}

// waitlistPass pone al día la lista: cierra lo vencido o ya reservado y avisa a quien le toca.
func (a *API) waitlistPass(ctx context.Context) error {
	now := a.now()
	// vencidas: el día ya pasó en la zona del negocio
	if _, err := a.DB.Exec(ctx,
		`UPDATE waitlist w SET status = 'expired' FROM businesses b
		  WHERE b.id = w.business_id AND w.status IN ('waiting', 'notified') AND w.day < ($1::timestamptz AT TIME ZONE b.timezone)::date`, now); err != nil {
		return err
	}
	// reservadas: la persona ya tiene cita de ese servicio ese día
	if _, err := a.DB.Exec(ctx,
		`UPDATE waitlist w SET status = 'booked' FROM businesses b
		  WHERE b.id = w.business_id AND w.status IN ('waiting', 'notified')
		    AND EXISTS (SELECT 1 FROM appointments ap WHERE ap.business_id = w.business_id AND ap.client_id = w.client_id
		                  AND ap.service_id = w.service_id AND ap.status IN ('scheduled', 'completed', 'pending_payment')
		                  AND (ap.starts_at AT TIME ZONE b.timezone)::date = w.day)`); err != nil {
		return err
	}
	// datos borrados: sin correo no hay a quién avisar
	if _, err := a.DB.Exec(ctx,
		`UPDATE waitlist w SET status = 'cancelled' FROM clients c
		  WHERE c.id = w.client_id AND w.status IN ('waiting', 'notified') AND (c.anonymized_at IS NOT NULL OR c.email = '')`); err != nil {
		return err
	}
	if !a.mailEnabled() {
		return nil
	}

	rows, err := a.DB.Query(ctx,
		`SELECT w.id, w.business_id, w.branch_id, w.service_id, w.client_id, w.staff_id, to_char(w.day, 'YYYY-MM-DD'), w.from_min, w.to_min,
		        c.email, c.name, w.token, b.name, b.slug, s.name, b.timezone, s.duration_min, s.buffer_min,
		        COALESCE(bs.enabled, false) AND s.online_bookable, COALESCE(bs.min_notice_hours, 0)
		   FROM waitlist w JOIN businesses b ON b.id = w.business_id JOIN clients c ON c.id = w.client_id
		   JOIN services s ON s.id = w.service_id JOIN branches br ON br.id = w.branch_id
		   LEFT JOIN booking_settings bs ON bs.business_id = w.business_id
		  WHERE w.status = 'waiting' AND `+usableSQL("b.", "$1")+` AND s.active AND br.active AND c.email <> ''
		  ORDER BY w.created_at, w.id LIMIT $2`, now, waitlistBatch)
	if err != nil {
		return err
	}
	var cands []waitCand
	for rows.Next() {
		var c waitCand
		if err := rows.Scan(&c.ID, &c.BizID, &c.BranchID, &c.ServiceID, &c.ClientID, &c.StaffID, &c.Day, &c.From, &c.To,
			&c.Email, &c.ClientName, &c.Token, &c.Business, &c.Slug, &c.ServiceName, &c.TZ, &c.DurMin, &c.BufMin, &c.LinkOnline, &c.MinNotice); err != nil {
			rows.Close()
			return err
		}
		cands = append(cands, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	type group struct {
		biz, branch, service int64
		day                  string
	}
	type groupInfo struct {
		slots    []staffSlots
		capacity int
	}
	groups := map[group]*groupInfo{}
	for _, c := range cands {
		g := group{c.BizID, c.BranchID, c.ServiceID, c.Day}
		gi := groups[g]
		if gi == nil {
			loc, err := time.LoadLocation(c.TZ)
			if err != nil {
				loc = time.UTC
			}
			day, err := time.ParseInLocation(dateLayout, c.Day, loc)
			if err != nil {
				continue
			}
			svc := &svcInfo{Duration: time.Duration(c.DurMin) * time.Minute, Buffer: time.Duration(c.BufMin) * time.Minute}
			earliest := now.Add(time.Duration(c.MinNotice) * time.Hour)
			slots, err := a.computeSlots(ctx, c.BizID, svc, c.ServiceID, c.BranchID, 0, day, loc, earliest)
			if err != nil {
				return err
			}
			distinct := map[string]bool{}
			for _, s := range slots {
				for _, t := range s.Starts {
					distinct[t] = true
				}
			}
			var recent int
			if err := a.DB.QueryRow(ctx,
				`SELECT count(*) FROM waitlist WHERE business_id = $1 AND branch_id = $2 AND service_id = $3 AND day = $4 AND status = 'notified' AND notified_at > $5`,
				c.BizID, c.BranchID, c.ServiceID, c.Day, now.Add(-waitNoticeCooldown)).Scan(&recent); err != nil {
				return err
			}
			gi = &groupInfo{slots: slots, capacity: len(distinct) - recent}
			groups[g] = gi
		}
		if gi.capacity <= 0 {
			continue
		}
		loc, err := time.LoadLocation(c.TZ)
		if err != nil {
			loc = time.UTC
		}
		times := matchingStarts(gi.slots, c, loc)
		if len(times) == 0 {
			continue
		}
		if err := a.offerWaitlist(ctx, c, times, loc, now); err != nil {
			return err
		}
		gi.capacity--
	}
	return nil
}

// matchingStarts deja los horarios libres que le sirven a la solicitud (profesional y franja).
func matchingStarts(slots []staffSlots, c waitCand, loc *time.Location) []time.Time {
	seen := map[string]bool{}
	var out []time.Time
	for _, s := range slots {
		if c.StaffID != nil && s.StaffID != *c.StaffID {
			continue
		}
		for _, iso := range s.Starts {
			t, err := time.Parse(time.RFC3339, iso)
			if err != nil || seen[iso] {
				continue
			}
			l := t.In(loc)
			m := l.Hour()*60 + l.Minute()
			if c.From != nil && (m < *c.From || m >= *c.To) {
				continue
			}
			seen[iso] = true
			out = append(out, t)
		}
	}
	sortTimes(out)
	return out
}

func sortTimes(ts []time.Time) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j].Before(ts[j-1]); j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}

func (a *API) offerWaitlist(ctx context.Context, c waitCand, times []time.Time, loc *time.Location, now time.Time) error {
	shown := times
	if len(shown) > 4 {
		shown = shown[:4]
	}
	first := strings.Fields(c.ClientName)
	name := c.ClientName
	if len(first) > 0 {
		name = first[0]
	}
	var list, htmlList strings.Builder
	for i, t := range shown {
		l := t.In(loc)
		if i == 0 {
			list.WriteString(whenES(t, loc))
		} else {
			fmt.Fprintf(&list, ", %02d:%02d", l.Hour(), l.Minute())
		}
	}
	htmlList.WriteString(html.EscapeString(list.String()))
	if len(times) > len(shown) {
		list.WriteString(" y más")
		htmlList.WriteString(" y más")
	}
	leave := ""
	if a.AppURL != "" {
		leave = a.AppURL + "/espera/" + c.Token
	}
	book := ""
	if c.LinkOnline && a.AppURL != "" {
		book = fmt.Sprintf("%s/reservar/%s?service=%d&branch=%d&date=%s", a.AppURL, c.Slug, c.ServiceID, c.BranchID, c.Day)
	}
	subject := "Se liberó un horario · " + c.Business
	var t strings.Builder
	fmt.Fprintf(&t, "Hola %s,\n\nSe liberó un lugar para %s: %s.\n\n", name, c.ServiceName, list.String())
	if book != "" {
		fmt.Fprintf(&t, "Resérvalo aquí (el primero que lo toma, se lo queda):\n%s\n\n", book)
	} else {
		fmt.Fprintf(&t, "Comunícate con %s para tomarlo; el primero que lo pide, se lo queda.\n\n", c.Business)
	}
	if leave != "" {
		fmt.Fprintf(&t, "Si ya no lo necesitas, sal de la lista de espera: %s\n", leave)
	}
	fmt.Fprintf(&t, "\n— %s\n", c.Business)

	esc := html.EscapeString
	var h strings.Builder
	h.WriteString(`<div style="font-family:system-ui,-apple-system,Segoe UI,sans-serif;max-width:520px;margin:0 auto;padding:24px;color:#1f2a28">`)
	fmt.Fprintf(&h, `<h2 style="margin:0 0 12px;font-size:20px">%s</h2><p style="margin:0 0 16px;line-height:1.5">Hola %s, se liberó un lugar para %s: %s.</p>`, esc(c.Business), esc(name), esc(c.ServiceName), htmlList.String())
	if book != "" {
		fmt.Fprintf(&h, `<p style="margin:20px 0 0"><a href="%s" style="display:inline-block;padding:10px 18px;border-radius:8px;background:#4b2a5e;color:#fff;text-decoration:none">Reservar ahora</a></p><p style="margin:8px 0 0;font-size:13px;color:#5b6864">El primero que lo toma, se lo queda.</p>`, esc(book))
	} else {
		fmt.Fprintf(&h, `<p style="margin:16px 0 0;line-height:1.5">Comunícate con %s para tomarlo; el primero que lo pide, se lo queda.</p>`, esc(c.Business))
	}
	if leave != "" {
		fmt.Fprintf(&h, `<p style="margin:20px 0 0;font-size:12px;color:#5b6864"><a href="%s" style="color:#5b6864">Salir de la lista de espera</a></p>`, esc(leave))
	}
	h.WriteString(`</div>`)

	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE waitlist SET status = 'notified', notified_at = $2 WHERE id = $1 AND status = 'waiting'`, c.ID, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		if _, err := tx.Exec(ctx,
			`INSERT INTO notifications (business_id, client_id, waitlist_id, kind, to_addr, subject, body_text, body_html, send_after, dedupe_key)
			 VALUES ($1, $2, $3, 'waitlist', $4, $5, $6, $7, $8, $9) ON CONFLICT DO NOTHING`,
			c.BizID, c.ClientID, c.ID, c.Email, subject, t.String(), h.String(), now, fmt.Sprintf("waitlist:%d", c.ID)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
