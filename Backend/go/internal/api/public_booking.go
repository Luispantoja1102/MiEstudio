package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Reserva en línea: páginas públicas sin sesión. Todo lo que se devuelve es lo que cualquier visitante puede ver;
// una dirección desconocida, apagada o de un negocio suspendido responde igual (404) para no revelar nada.

var tokenShape = regexp.MustCompile(`^[A-Za-z0-9_-]{32}$`)

func newPublicToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b) // 32 caracteres
}

type pubBiz struct {
	ID       int64
	Name     string
	Kind     string
	Slug     string
	Loc      *time.Location
	Settings bookingSettings
}

// publicBusiness busca el negocio de una dirección pública. nil si no existe, no tiene la reserva encendida o no está en uso.
func (a *API) publicBusiness(ctx context.Context, slug string) (*pubBiz, error) {
	if slug == "" || len(slug) > 100 {
		return nil, nil
	}
	b := &pubBiz{}
	var tz string
	err := a.DB.QueryRow(ctx,
		`SELECT b.id, b.name, b.kind, b.slug, b.timezone FROM businesses b
		 JOIN booking_settings bs ON bs.business_id = b.id
		 WHERE b.slug = $1 AND `+usableSQL("b.", "$2")+` AND bs.enabled`, slug, a.now()).Scan(&b.ID, &b.Name, &b.Kind, &b.Slug, &tz)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.Loc = a.businessLoc(ctx, b.ID)
	if b.Settings, err = loadBookingSettings(ctx, a.DB, b.ID); err != nil {
		return nil, err
	}
	return b, nil
}

func notFound(w http.ResponseWriter) { writeErr(w, http.StatusNotFound, "no encontrado") }

// depositAmount es la parte del precio que se pide por adelantado (Mercado Pago no cobra menos de un peso).
func depositAmount(price, percent int) int {
	if price <= 0 || percent <= 0 {
		return 0
	}
	cents := (price*percent + 50) / 100
	if cents < 100 {
		cents = min(100, price)
	}
	return cents
}

// depositFor es el anticipo de un servicio para esta reserva (0 = sin anticipo).
func (a *API) depositFor(ctx context.Context, b *pubBiz, price int) int {
	if len(a.depositMethods(ctx, b)) == 0 {
		return 0
	}
	return depositAmount(price, b.Settings.DepositPercent)
}

// normalizePhone deja solo dígitos (de 10 a 15). Devuelve "" si no parece un teléfono.
func normalizePhone(raw string) string {
	var d strings.Builder
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			d.WriteRune(r)
		case r == '+', r == ' ', r == '-', r == '(', r == ')', r == '.':
		default:
			return ""
		}
	}
	if n := d.Len(); n < 10 || n > 15 {
		return ""
	}
	return d.String()
}

// phoneKey son los últimos 10 dígitos: identifica al mismo número escrito con o sin lada.
func phoneKey(digits string) string {
	if len(digits) > 10 {
		return digits[len(digits)-10:]
	}
	return digits
}

type publicService struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	DurationMin  int    `json:"duration_min"`
	PriceCents   *int   `json:"price_cents,omitempty"`
	DepositCents int    `json:"deposit_cents,omitempty"`
}

// publicInfo devuelve lo que la página de reservas necesita para empezar.
func (a *API) publicInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	b, err := a.publicBusiness(ctx, r.PathValue("slug"))
	if err != nil {
		serverErr(w)
		return
	}
	if b == nil {
		notFound(w)
		return
	}
	a.sweepHolds(ctx)

	brows, err := a.DB.Query(ctx, `SELECT id, name, address, phone FROM branches WHERE business_id = $1 AND active ORDER BY id`, b.ID)
	if err != nil {
		serverErr(w)
		return
	}
	branches := []branch{}
	for brows.Next() {
		var x branch
		if err := brows.Scan(&x.ID, &x.Name, &x.Address, &x.Phone); err != nil {
			brows.Close()
			serverErr(w)
			return
		}
		x.Active = true
		branches = append(branches, x)
	}
	brows.Close()

	srows, err := a.DB.Query(ctx,
		`SELECT id, name, duration_min, price_cents FROM services WHERE business_id = $1 AND active AND online_bookable ORDER BY name`, b.ID)
	if err != nil {
		serverErr(w)
		return
	}
	services := []publicService{}
	for srows.Next() {
		var s publicService
		var price int
		if err := srows.Scan(&s.ID, &s.Name, &s.DurationMin, &price); err != nil {
			srows.Close()
			serverErr(w)
			return
		}
		if b.Settings.ShowPrices {
			s.PriceCents = &price
		}
		s.DepositCents = a.depositFor(ctx, b, price)
		services = append(services, s)
	}
	srows.Close()

	// personas que se pueden elegir: en agenda, activas y con horario en alguna sucursal
	prows, err := a.DB.Query(ctx,
		`SELECT u.id, u.name FROM users u WHERE u.business_id = $1 AND u.active AND u.bookable
		   AND EXISTS (SELECT 1 FROM working_hours h WHERE h.user_id = u.id) ORDER BY u.name`, b.ID)
	if err != nil {
		serverErr(w)
		return
	}
	type person struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	staff := []person{}
	for prows.Next() {
		var x person
		if err := prows.Scan(&x.ID, &x.Name); err != nil {
			prows.Close()
			serverErr(w)
			return
		}
		staff = append(staff, x)
	}
	prows.Close()

	writeJSON(w, http.StatusOK, map[string]any{
		"business":         map[string]any{"name": b.Name, "kind": b.Kind, "timezone": b.Loc.String(), "welcome_text": b.Settings.WelcomeText},
		"branches":         branches,
		"services":         services,
		"staff":            staff,
		"min_notice_hours": b.Settings.MinNoticeHours,
		"max_days_ahead":   b.Settings.MaxDaysAhead,
		"cancel_min_hours": b.Settings.CancelMinHours,
		"deposit_percent":  b.Settings.DepositPercent,
		"deposit_methods":  a.depositMethods(ctx, b), // mp y/o transfer
		"reminders":        b.Settings.ReminderHours > 0 && a.mailEnabled(),
		"whatsapp":         b.Settings.ReminderHours > 0 && b.Settings.WhatsAppReminders && a.waEnabled(),
		"waitlist":         a.mailEnabled(), // la lista de espera avisa por correo
		"theme":            b.Settings.PublicTheme,
		"today":            a.now().In(b.Loc).Format(dateLayout),
	})
}

// onlineService devuelve el servicio si se ofrece en línea.
func (a *API) onlineService(ctx context.Context, bizID, id int64) (*svcInfo, int, string, error) {
	var dur, buf, price int
	var name string
	err := a.DB.QueryRow(ctx,
		`SELECT duration_min, buffer_min, price_cents, name FROM services WHERE id = $1 AND business_id = $2 AND active AND online_bookable`,
		id, bizID).Scan(&dur, &buf, &price, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, "", nil
	}
	return &svcInfo{time.Duration(dur) * time.Minute, time.Duration(buf) * time.Minute}, price, name, err
}

// withinHorizon dice si un día cae dentro de lo que se puede reservar.
func (b *pubBiz) withinHorizon(day, now time.Time) bool {
	today := now.In(b.Loc)
	first := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, b.Loc)
	last := first.AddDate(0, 0, b.Settings.MaxDaysAhead)
	return !day.Before(first) && day.Before(last.AddDate(0, 0, 1))
}

func (a *API) publicSlots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "pub-slots:"+clientIP(r), 120, time.Minute) {
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
	q := r.URL.Query()
	serviceID, _ := strconv.ParseInt(q.Get("service_id"), 10, 64)
	branchID, _ := strconv.ParseInt(q.Get("branch_id"), 10, 64)
	onlyStaff, _ := strconv.ParseInt(q.Get("staff_id"), 10, 64)
	day, err := time.ParseInLocation(dateLayout, q.Get("date"), b.Loc)
	if err != nil || serviceID <= 0 || branchID <= 0 {
		writeErr(w, http.StatusBadRequest, "indica el servicio, la sucursal y la fecha")
		return
	}
	now := a.now()
	if !b.withinHorizon(day, now) {
		writeJSON(w, http.StatusOK, map[string]any{"date": q.Get("date"), "timezone": b.Loc.String(), "staff": []staffSlots{}})
		return
	}
	a.sweepHolds(ctx)
	svc, _, _, err := a.onlineService(ctx, b.ID, serviceID)
	if err != nil {
		serverErr(w)
		return
	}
	okBranch, err := activeBranch(ctx, a.DB, b.ID, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	if svc == nil || !okBranch {
		notFound(w)
		return
	}
	earliest := now.Add(time.Duration(b.Settings.MinNoticeHours) * time.Hour)
	out, err := a.computeSlotsTaken(ctx, b.ID, svc, serviceID, branchID, onlyStaff, day, b.Loc, earliest, true)
	if err == nil {
		out, err = a.withoutOthersHolds(ctx, out, svc, q.Get("holder")) // lo que otra visita tiene apartado se ve ocupado
	}
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": q.Get("date"), "timezone": b.Loc.String(), "staff": out})
}

type bookingReq struct {
	ServiceID        int64  `json:"service_id"`
	BranchID         int64  `json:"branch_id"`
	StaffID          int64  `json:"staff_id"` // 0 = cualquiera
	StartsAt         string `json:"starts_at"`
	Name             string `json:"name"`
	Phone            string `json:"phone"`
	Email            string `json:"email"`
	Notes            string `json:"notes"`
	AcceptPrivacy    bool   `json:"accept_privacy"`
	ConsentReminders bool   `json:"consent_reminders"`
	ConsentMarketing bool   `json:"consent_marketing"` // promociones y felicitación de cumpleaños
	ConsentWhatsApp  bool   `json:"consent_whatsapp"`  // recordatorio por WhatsApp
	Holder           string `json:"holder"`            // identificador de quien apartó el horario (opcional)
	PaymentMethod    string `json:"payment_method"`    // cómo paga el anticipo: mp | transfer
}

// findOrCreateClient identifica a quien reserva por teléfono (o correo) sin revelar nada de lo que ya hay guardado.
func findOrCreateClient(ctx context.Context, tx pgx.Tx, bizID int64, name, digits, email string, consent bool) (int64, error) {
	key := phoneKey(digits)
	var id int64
	err := tx.QueryRow(ctx,
		`SELECT id FROM clients WHERE business_id = $1
		   AND (right(regexp_replace(phone, '\D', '', 'g'), 10) = $2 OR ($3 <> '' AND lower(email) = $3))
		 ORDER BY (right(regexp_replace(phone, '\D', '', 'g'), 10) = $2) DESC, id LIMIT 1`, bizID, key, email).Scan(&id)
	switch {
	case err == nil:
		// no se cambia lo que ya tenía; solo se completa el correo y se registra el consentimiento
		_, err = tx.Exec(ctx,
			`UPDATE clients SET email = CASE WHEN email = '' THEN $2 ELSE email END,
			   reminders_consent = reminders_consent OR $3 WHERE id = $1`, id, email, consent)
		return id, err
	case !errors.Is(err, pgx.ErrNoRows):
		return 0, err
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO clients (business_id, name, phone, email, reminders_consent) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		bizID, name, digits, email, consent).Scan(&id)
	return id, err
}

// pickStaff elige a quién atiende: la persona pedida o, si da igual, la que tenga menos citas ese día.
func (a *API) pickStaff(ctx context.Context, bizID int64, offers []staffSlots, startISO string, wanted int64, day time.Time) (int64, bool, error) {
	var candidates []int64
	for _, s := range offers {
		for _, st := range s.Starts {
			if st == startISO && (wanted == 0 || wanted == s.StaffID) {
				candidates = append(candidates, s.StaffID)
			}
		}
	}
	switch len(candidates) {
	case 0:
		return 0, false, nil
	case 1:
		return candidates[0], true, nil
	}
	var best int64
	bestN := -1
	for _, id := range candidates {
		var n int
		if err := a.DB.QueryRow(ctx,
			`SELECT count(*) FROM appointments WHERE business_id = $1 AND staff_id = $2 AND status IN ('scheduled', 'completed', 'pending_payment')
			   AND starts_at >= $3 AND starts_at < $4`, bizID, id, day, day.AddDate(0, 0, 1)).Scan(&n); err != nil {
			return 0, false, err
		}
		if bestN < 0 || n < bestN {
			best, bestN = id, n
		}
	}
	return best, true, nil
}

func (a *API) publicBook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "pub-book:"+clientIP(r), 10, time.Hour) {
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
	var req bookingReq
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Notes = strings.TrimSpace(req.Notes)
	digits := normalizePhone(strings.TrimSpace(req.Phone))
	start, tErr := time.Parse(time.RFC3339, req.StartsAt)
	switch nameLen := utf8.RuneCountInString(req.Name); {
	case nameLen < 2 || nameLen > 120:
		writeErr(w, http.StatusBadRequest, "escribe tu nombre")
		return
	case digits == "":
		writeErr(w, http.StatusBadRequest, "escribe un teléfono válido de 10 dígitos")
		return
	case req.Email != "" && !validEmail(req.Email):
		writeErr(w, http.StatusBadRequest, "el correo no es válido")
		return
	case utf8.RuneCountInString(req.Notes) > 300:
		writeErr(w, http.StatusBadRequest, "las notas son demasiado largas")
		return
	case !req.AcceptPrivacy:
		writeErr(w, http.StatusBadRequest, "debes aceptar el aviso de privacidad para reservar")
		return
	case tErr != nil:
		writeErr(w, http.StatusBadRequest, "horario inválido")
		return
	case (req.ConsentReminders || req.ConsentMarketing) && req.Email == "":
		writeErr(w, http.StatusBadRequest, "para recibir mensajes necesitamos tu correo")
		return
	}
	if !a.allow(w, "pub-book-phone:"+phoneKey(digits), 5, time.Hour) {
		return
	}
	a.sweepHolds(ctx)

	svc, price, svcName, err := a.onlineService(ctx, b.ID, req.ServiceID)
	if err != nil {
		serverErr(w)
		return
	}
	okBranch, err := activeBranch(ctx, a.DB, b.ID, req.BranchID)
	if err != nil {
		serverErr(w)
		return
	}
	if svc == nil || !okBranch {
		writeErr(w, http.StatusBadRequest, "servicio o sucursal no disponibles")
		return
	}

	// el horario debe ser uno de los que se ofrecen (respeta horario, preparación, bloqueos y aviso mínimo)
	now := a.now()
	ls := start.In(b.Loc)
	day := time.Date(ls.Year(), ls.Month(), ls.Day(), 0, 0, 0, 0, b.Loc)
	if !b.withinHorizon(day, now) {
		writeErr(w, http.StatusConflict, "ese horario ya no está disponible")
		return
	}
	earliest := now.Add(time.Duration(b.Settings.MinNoticeHours) * time.Hour)
	offers, err := a.computeSlots(ctx, b.ID, svc, req.ServiceID, req.BranchID, req.StaffID, day, b.Loc, earliest)
	if err == nil {
		offers, err = a.withoutOthersHolds(ctx, offers, svc, req.Holder)
	}
	if err != nil {
		serverErr(w)
		return
	}
	staffID, ok, err := a.pickStaff(ctx, b.ID, offers, start.Format(time.RFC3339), req.StaffID, day)
	if err != nil {
		serverErr(w)
		return
	}
	if !ok {
		writeErr(w, http.StatusConflict, "ese horario ya no está disponible")
		return
	}

	deposit := a.depositFor(ctx, b, price)
	method := ""
	if deposit > 0 {
		methods := a.depositMethods(ctx, b)
		switch {
		case req.PaymentMethod == "" && len(methods) == 1:
			method = methods[0]
		case slices.Contains(methods, req.PaymentMethod):
			method = req.PaymentMethod
		default:
			writeErr(w, http.StatusBadRequest, "elige cómo pagar el anticipo")
			return
		}
	}
	res, ferr := a.createOnlineAppointment(ctx, b, req, digits, start, staffID, deposit, method, svcName, svc)
	if ferr != nil {
		if !writeFail(w, ferr) {
			serverErr(w)
		}
		return
	}
	if validHolder(req.Holder) {
		a.DB.Exec(ctx, `DELETE FROM booking_holds WHERE holder = $1`, req.Holder) // ya es cita: se suelta el apartado
	}
	writeJSON(w, http.StatusCreated, res)
}

type bookingResult struct {
	Token        string `json:"token"`
	Status       string `json:"status"`
	StartsAt     string `json:"starts_at"`
	EndsAt       string `json:"ends_at"`
	ManagePath   string `json:"manage_path"`
	DepositCents int    `json:"deposit_cents,omitempty"`
	PayURL       string `json:"pay_url,omitempty"`
	PayMethod    string `json:"pay_method,omitempty"` // transfer: se paga por fuera y se sube el comprobante en la página de la cita
}

const maxActiveBookingsPerPhone = 3

// createOnlineAppointment guarda la cita de una reserva en línea.
func (a *API) createOnlineAppointment(ctx context.Context, b *pubBiz, req bookingReq, digits string, start time.Time, staffID int64, deposit int, method, svcName string, svc *svcInfo) (*bookingResult, error) {
	end := start.Add(svc.Duration)
	token := newPublicToken()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	clientID, err := findOrCreateClient(ctx, tx, b.ID, req.Name, digits, req.Email, req.ConsentReminders)
	if err != nil {
		return nil, err
	}
	if req.ConsentWhatsApp {
		if _, err := tx.Exec(ctx, `UPDATE clients SET whatsapp_consent = true WHERE id = $1`, clientID); err != nil {
			return nil, err
		}
	}
	if req.ConsentMarketing && req.Email != "" {
		if _, err := tx.Exec(ctx, `UPDATE clients SET marketing_consent = true WHERE id = $1 AND email <> ''`, clientID); err != nil {
			return nil, err
		}
	}
	var active int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM appointments WHERE business_id = $1 AND client_id = $2
		   AND status IN ('scheduled', 'pending_payment') AND starts_at > $3`, b.ID, clientID, a.now()).Scan(&active); err != nil {
		return nil, err
	}
	if active >= maxActiveBookingsPerPhone {
		return nil, fail(http.StatusConflict, "ya tienes varias citas pendientes; cancela alguna o contáctanos")
	}
	status, holdUntil := "scheduled", (*time.Time)(nil)
	if deposit > 0 {
		hold := holdMinutes * time.Minute
		if method == "transfer" {
			hold = time.Duration(b.Settings.TransferHoldHours) * time.Hour // da tiempo de transferir y mandar el comprobante
		}
		h := a.now().Add(hold)
		status, holdUntil = "pending_payment", &h
	}
	var apptID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO appointments (business_id, branch_id, client_id, staff_id, service_id, starts_at, ends_at, blocked_until, notes,
		   source, public_token, reminders_consent, privacy_accepted_at, created_at, status, hold_expires_at, privacy_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'online', $10, $11, $12, $12, $13, $14, $15) RETURNING id`,
		b.ID, req.BranchID, clientID, staffID, req.ServiceID, start, end, end.Add(svc.Buffer), req.Notes, token, req.ConsentReminders, a.now(),
		status, holdUntil, ClientNoticeVersion).Scan(&apptID)
	if isExclusionViolation(err) {
		return nil, fail(http.StatusConflict, "ese horario ya no está disponible")
	}
	if err != nil {
		return nil, err
	}
	if deposit > 0 {
		// la referencia lleva el id del anticipo; se fija en cuanto se conoce
		var depID int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO deposits (business_id, appointment_id, amount_cents, external_ref, created_at, method) VALUES ($1, $2, $3, 'tmp-' || $4, $5, $6) RETURNING id`,
			b.ID, apptID, deposit, token, a.now(), method).Scan(&depID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE deposits SET external_ref = 'mie-dep:' || id WHERE id = $1`, depID); err != nil {
			return nil, err
		}
	} else if err := a.enqueueMail(ctx, tx, apptID, "booking", false); err != nil {
		return nil, err // con anticipo, la confirmación sale cuando se paga
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	res := &bookingResult{Token: token, Status: status, StartsAt: start.UTC().Format(time.RFC3339), EndsAt: end.UTC().Format(time.RFC3339),
		ManagePath: "/cita/" + token}
	switch {
	case deposit > 0 && method == "transfer":
		res.DepositCents, res.PayMethod = deposit, "transfer"
		a.inboxAppt(ctx, apptID, "booking_transfer", "Reserva por transferencia: falta el comprobante")
	case deposit > 0:
		payURL, err := a.startDeposit(ctx, b, apptID, token, deposit, svcName, *holdUntil)
		if err != nil {
			return nil, err
		}
		res.DepositCents, res.PayURL = deposit, payURL
	default:
		a.inboxAppt(ctx, apptID, "booking", "Nueva reserva en línea")
	}
	return res, nil
}

// ---------- ver y cancelar con el enlace privado ----------

type manageRow struct {
	ID, BizID       int64
	Status          string
	Starts, Ends    time.Time
	Business, Slug  string
	Branch, Address string
	Service, Staff  string
	ClientName      string
	Confirmed       bool
}

// byToken busca una cita por su enlace privado. Un token mal formado no toca la base de datos.
func (a *API) byToken(ctx context.Context, token string) (*manageRow, error) {
	if !tokenShape.MatchString(token) {
		return nil, nil
	}
	m := &manageRow{}
	err := a.DB.QueryRow(ctx,
		`SELECT ap.id, ap.business_id, ap.status, ap.starts_at, ap.ends_at, b.name, b.slug, br.name, br.address, s.name, u.name, c.name, ap.confirmed_at IS NOT NULL
		   FROM appointments ap JOIN businesses b ON b.id = ap.business_id JOIN branches br ON br.id = ap.branch_id
		   JOIN services s ON s.id = ap.service_id JOIN users u ON u.id = ap.staff_id JOIN clients c ON c.id = ap.client_id
		  WHERE ap.public_token = $1`, token).Scan(&m.ID, &m.BizID, &m.Status, &m.Starts, &m.Ends, &m.Business, &m.Slug, &m.Branch, &m.Address, &m.Service, &m.Staff, &m.ClientName, &m.Confirmed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

func (m *manageRow) canCancel(now time.Time, cancelMinHours int) bool {
	switch m.Status {
	case "pending_payment":
		return true
	case "scheduled":
		return now.Before(m.Starts.Add(-time.Duration(cancelMinHours) * time.Hour))
	}
	return false
}

func (a *API) publicAppointment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "pub-token:"+clientIP(r), 60, time.Minute) {
		return
	}
	a.sweepHolds(ctx)
	m, err := a.byToken(ctx, r.PathValue("token"))
	if err != nil {
		serverErr(w)
		return
	}
	if m == nil {
		notFound(w)
		return
	}
	dep, err := loadDeposit(ctx, a.DB, "appointment_id = $1", m.ID)
	if err != nil {
		serverErr(w)
		return
	}
	if dep != nil && dep.Method != "transfer" && m.Status == "pending_payment" {
		// el cliente suele volver de Mercado Pago a esta página: se confirma el pago sin esperar al aviso automático
		if _, err := a.settleDeposit(ctx, dep.ID); err == nil {
			if m, err = a.byToken(ctx, r.PathValue("token")); err != nil || m == nil {
				serverErr(w)
				return
			}
			dep, _ = loadDeposit(ctx, a.DB, "appointment_id = $1", m.ID)
		}
	}
	st, err := loadBookingSettings(ctx, a.DB, m.BizID)
	if err != nil {
		serverErr(w)
		return
	}
	loc := a.businessLoc(ctx, m.BizID)
	now := a.now()
	first := strings.Fields(m.ClientName)
	name := ""
	if len(first) > 0 {
		name = first[0]
	}
	out := map[string]any{
		"status":           m.Status,
		"starts_at":        m.Starts.UTC().Format(time.RFC3339),
		"ends_at":          m.Ends.UTC().Format(time.RFC3339),
		"timezone":         loc.String(),
		"business":         m.Business,
		"slug":             m.Slug,
		"branch":           m.Branch,
		"address":          m.Address,
		"service":          m.Service,
		"staff":            m.Staff,
		"client_first":     name,
		"can_cancel":       m.canCancel(now, st.CancelMinHours),
		"confirmed":        m.Confirmed,
		"cancel_min_hours": st.CancelMinHours,
		"theme":            st.PublicTheme,
		"cancel_deadline":  m.Starts.Add(-time.Duration(st.CancelMinHours) * time.Hour).UTC().Format(time.RFC3339),
	}
	if dep != nil {
		d := map[string]any{"amount_cents": dep.Amount, "status": dep.Status, "method": dep.Method}
		if dep.Method == "transfer" && dep.Status == "pending" && m.Status == "pending_payment" {
			t := transferDetails(st, m.ID)
			t["proof_uploaded"] = dep.ProofAt != nil
			t["reject_reason"] = dep.RejectReason
			d["transfer"] = t
			var hold *time.Time
			a.DB.QueryRow(ctx, `SELECT hold_expires_at FROM appointments WHERE id = $1`, m.ID).Scan(&hold)
			if hold != nil {
				d["hold_expires_at"] = hold.UTC().Format(time.RFC3339)
			}
		} else if dep.Status == "pending" && m.Status == "pending_payment" {
			var payURL string
			var hold *time.Time
			a.DB.QueryRow(ctx, `SELECT d.pay_url, ap.hold_expires_at FROM deposits d JOIN appointments ap ON ap.id = d.appointment_id WHERE d.id = $1`, dep.ID).Scan(&payURL, &hold)
			d["pay_url"] = payURL
			if hold != nil {
				d["hold_expires_at"] = hold.UTC().Format(time.RFC3339)
			}
		}
		out["deposit"] = d
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) publicCancel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "pub-token:"+clientIP(r), 60, time.Minute) {
		return
	}
	a.sweepHolds(ctx)
	m, err := a.byToken(ctx, r.PathValue("token"))
	if err != nil {
		serverErr(w)
		return
	}
	if m == nil {
		notFound(w)
		return
	}
	st, err := loadBookingSettings(ctx, a.DB, m.BizID)
	if err != nil {
		serverErr(w)
		return
	}
	if m.Status == "cancelled" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "cancelled"})
		return
	}
	if !m.canCancel(a.now(), st.CancelMinHours) {
		writeErr(w, http.StatusConflict, "ya no es posible cancelar en línea; contacta al negocio")
		return
	}
	// dentro del plazo el anticipo se devuelve; si Mercado Pago no puede, la cita no se cancela
	if err := a.refundDeposit(ctx, m.ID); err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}
	tag, err := a.DB.Exec(ctx, `UPDATE appointments SET status = 'cancelled' WHERE id = $1 AND status IN ('scheduled', 'pending_payment')`, m.ID)
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusConflict, "la cita ya cambió; recarga la página")
		return
	}
	a.DB.Exec(ctx, `UPDATE deposits SET status = 'expired' WHERE appointment_id = $1 AND status = 'pending'`, m.ID)
	a.notify(ctx, m.ID, "cancellation", false)
	a.inboxAppt(ctx, m.ID, "cancelled", "Cita cancelada por el cliente")
	writeJSON(w, http.StatusOK, map[string]any{"status": "cancelled"})
}

// publicRemindersOff retira el consentimiento de recordatorios de quien tenga este enlace (en todas sus citas futuras).
func (a *API) publicRemindersOff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "pub-token:"+clientIP(r), 60, time.Minute) {
		return
	}
	m, err := a.byToken(ctx, r.PathValue("token"))
	if err != nil {
		serverErr(w)
		return
	}
	if m == nil {
		notFound(w)
		return
	}
	if _, err := a.DB.Exec(ctx,
		`WITH c AS (UPDATE clients SET reminders_consent = false, whatsapp_consent = false WHERE id = (SELECT client_id FROM appointments WHERE id = $1) RETURNING id)
		 UPDATE appointments SET reminders_consent = false WHERE client_id IN (SELECT id FROM c) AND starts_at > $2`, m.ID, a.now()); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reminders": false})
}

// publicConfirm es POST /api/public-appointments/{token}/confirm: el cliente avisa que sí va.
func (a *API) publicConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "pub-token:"+clientIP(r), 60, time.Minute) {
		return
	}
	m, err := a.byToken(ctx, r.PathValue("token"))
	if err != nil {
		serverErr(w)
		return
	}
	if m == nil {
		notFound(w)
		return
	}
	if m.Status != "scheduled" || !a.now().Before(m.Starts) {
		writeErr(w, http.StatusConflict, "esta cita ya no se puede confirmar")
		return
	}
	tag, err := a.DB.Exec(ctx, `UPDATE appointments SET confirmed_at = now() WHERE id = $1 AND confirmed_at IS NULL`, m.ID)
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() > 0 {
		a.inboxAppt(ctx, m.ID, "confirmed", "El cliente confirmó su asistencia")
	}
	w.WriteHeader(http.StatusNoContent)
}

// publicMonth es GET /api/public/{slug}/month: los días de un mes con al menos un horario libre, para marcarlos en el calendario.
// Una sola consulta por mes; no dice por quién ni para qué están ocupados los demás.
func (a *API) publicMonth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "pub-month:"+clientIP(r), 60, time.Minute) {
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
	q := r.URL.Query()
	serviceID, _ := strconv.ParseInt(q.Get("service_id"), 10, 64)
	branchID, _ := strconv.ParseInt(q.Get("branch_id"), 10, 64)
	onlyStaff, _ := strconv.ParseInt(q.Get("staff_id"), 10, 64)
	first, err := time.ParseInLocation("2006-01", q.Get("month"), b.Loc)
	if err != nil || serviceID <= 0 || branchID <= 0 {
		writeErr(w, http.StatusBadRequest, "indica el servicio, la sucursal y el mes (AAAA-MM)")
		return
	}
	a.sweepHolds(ctx)
	svc, _, _, err := a.onlineService(ctx, b.ID, serviceID)
	if err != nil {
		serverErr(w)
		return
	}
	okBranch, err := activeBranch(ctx, a.DB, b.ID, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	if svc == nil || !okBranch {
		notFound(w)
		return
	}
	now := a.now()
	earliest := now.Add(time.Duration(b.Settings.MinNoticeHours) * time.Hour)
	days := []string{}
	for d := first; d.Month() == first.Month(); d = d.AddDate(0, 0, 1) {
		if !b.withinHorizon(d, now) {
			continue
		}
		offers, err := a.computeSlots(ctx, b.ID, svc, serviceID, branchID, onlyStaff, d, b.Loc, earliest)
		if err == nil {
			offers, err = a.withoutOthersHolds(ctx, offers, svc, q.Get("holder"))
		}
		if err != nil {
			serverErr(w)
			return
		}
		for _, o := range offers {
			if len(o.Starts) > 0 {
				days = append(days, d.Format(dateLayout))
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"month": q.Get("month"), "days": days})
}
