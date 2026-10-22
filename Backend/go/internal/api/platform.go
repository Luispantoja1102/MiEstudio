package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

var statuses = map[string]bool{"pending": true, "trial": true, "active": true, "suspended": true}

// ---------- negocios ----------

type bizRow struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	TrialEndsAt *time.Time `json:"trial_ends_at"`
	CreatedAt   time.Time  `json:"created_at"`
	Users       int        `json:"users"`
	Clients     int        `json:"clients"`
	OwnerEmail  string     `json:"owner_email"`

	Plan             string     `json:"plan"`
	PlanName         string     `json:"plan_name"`
	BillingState     string     `json:"billing_state"` // pending, trialing, trial_expired, active, past_due o suspended
	CurrentPeriodEnd *time.Time `json:"current_period_end"`
	SuspendedReason  string     `json:"suspended_reason"`
	TrialDaysLeft    *int       `json:"trial_days_left"`
	LastPayment      *time.Time `json:"last_payment_at"`
}

// stateSQL calcula el estado de cobro en SQL (gemelo de BillingState). nowArg es el parámetro con el momento actual.
func stateSQL(nowArg string) string {
	return `CASE b.status WHEN 'pending' THEN 'pending' WHEN 'suspended' THEN 'suspended'
	  WHEN 'trial' THEN CASE WHEN b.trial_ends_at > ` + nowArg + ` THEN 'trialing' ELSE 'trial_expired' END
	  ELSE CASE WHEN b.current_period_end IS NOT NULL AND b.current_period_end + interval '3 days' < ` + nowArg + ` THEN 'past_due' ELSE 'active' END END`
}

// bizSelect no lleva parámetros: cada consulta añade su WHERE.
const bizSelect = `SELECT b.id, b.name, b.slug, b.kind, b.status, b.trial_ends_at, b.created_at,
	(SELECT count(*) FROM users u WHERE u.business_id = b.id),
	(SELECT count(*) FROM clients c WHERE c.business_id = b.id),
	COALESCE((SELECT u.email FROM users u WHERE u.business_id = b.id AND u.role = 'admin' ORDER BY u.id LIMIT 1), ''),
	b.plan, b.current_period_end, b.suspended_reason,
	(SELECT max(p.created_at) FROM payments p WHERE p.business_id = b.id AND p.method <> 'courtesy')
	FROM businesses b`

func scanBiz(row pgx.Row, now time.Time) (bizRow, error) {
	var b bizRow
	err := row.Scan(&b.ID, &b.Name, &b.Slug, &b.Kind, &b.Status, &b.TrialEndsAt, &b.CreatedAt, &b.Users, &b.Clients, &b.OwnerEmail,
		&b.Plan, &b.CurrentPeriodEnd, &b.SuspendedReason, &b.LastPayment)
	if err != nil {
		return b, err
	}
	if pl, ok := defaultPlan(b.Plan); ok {
		b.PlanName = pl.Name
	}
	b.BillingState = BillingState(b.Status, b.TrialEndsAt, b.CurrentPeriodEnd, now)
	if b.Status == "trial" && b.TrialEndsAt != nil {
		d := int(b.TrialEndsAt.Sub(now).Hours()/24 + 0.999)
		if d < 0 {
			d = 0
		}
		b.TrialDaysLeft = &d
	}
	return b, nil
}

func (a *API) platformListBusinesses(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	state := r.URL.Query().Get("state")
	plan := r.URL.Query().Get("plan")
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	now := a.now()
	rows, err := a.DB.Query(r.Context(),
		bizSelect+` WHERE ($2 = '' OR b.status = $2) AND ($3 = '' OR (`+stateSQL("$1")+`) = $3) AND ($4 = '' OR b.plan = $4)
		  AND ($5 = '' OR strpos(lower(b.name), lower($5)) > 0 OR strpos(b.slug, lower($5)) > 0
		       OR EXISTS (SELECT 1 FROM users u WHERE u.business_id = b.id AND strpos(u.email, lower($5)) > 0))
		 ORDER BY b.created_at DESC, b.id DESC LIMIT 200`, now, status, state, plan, q)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []bizRow{}
	for rows.Next() {
		b, err := scanBiz(rows, now)
		if err != nil {
			serverErr(w)
			return
		}
		out = append(out, b)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) platformGetBusiness(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	b, err := scanBiz(a.DB.QueryRow(r.Context(), bizSelect+` WHERE b.id = $1`, id), a.now())
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	rows, err := a.DB.Query(r.Context(),
		`SELECT id, name, email, role, bookable, commission_pct::float8, active FROM users WHERE business_id = $1 ORDER BY id`, id)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	team := []member{}
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.Role, &m.Bookable, &m.CommissionPct, &m.Active); err != nil {
			serverErr(w)
			return
		}
		team = append(team, m)
	}
	out := map[string]any{"business": b, "team": team, "usage": a.usageOf(r.Context(), a.DB, id), "metrics": a.businessMetrics(r.Context(), id)}
	if pl, ok := a.planByID(r.Context(), b.Plan); ok {
		out["limits"] = pl
	}
	// el dinero solo lo ve quien administra la plataforma
	if auth.FromContext(r.Context()).Role == auth.RolePlatformAdmin {
		out["payments"] = a.paymentsOf(r.Context(), id, 50)
	}
	writeJSON(w, http.StatusOK, out)
}

// platformCreateBusiness da de alta un negocio manualmente (sin importar el modo de registro).
func (a *API) platformCreateBusiness(w http.ResponseWriter, r *http.Request) {
	var req struct {
		registerReq
		Status string `json:"status"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if msg := validateRegister(&req.registerReq); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if !statuses[req.Status] {
		writeErr(w, http.StatusBadRequest, "estado inválido")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		serverErr(w)
		return
	}
	bizID, _, err := a.registerBusiness(r.Context(), req.registerReq, hash, req.Status)
	switch {
	case errors.Is(err, errEmailTaken):
		writeErr(w, http.StatusConflict, "ese correo ya está registrado")
	case err != nil:
		serverErr(w)
	default:
		a.logActivity(r.Context(), actor(r), bizID, "business_created_by_platform", map[string]any{"status": req.Status, "email": req.Email})
		writeJSON(w, http.StatusCreated, map[string]any{"id": bizID})
	}
}

func (a *API) platformUpdateBusiness(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Status          *string `json:"status"`
		TrialEndsAt     *string `json:"trial_ends_at"`      // YYYY-MM-DD
		PeriodEnd       *string `json:"current_period_end"` // YYYY-MM-DD: fin del periodo pagado
		Plan            *string `json:"plan"`
		SuspendedReason *string `json:"suspended_reason"`
		Name            *string `json:"name"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	var trialEnds *time.Time
	switch {
	case req.Status != nil && !statuses[*req.Status]:
		writeErr(w, http.StatusBadRequest, "estado inválido")
		return
	case req.Name != nil && !validName(strings.TrimSpace(*req.Name)):
		writeErr(w, http.StatusBadRequest, "el nombre es obligatorio")
		return
	case req.Plan != nil && !a.planExists(r.Context(), *req.Plan):
		writeErr(w, http.StatusBadRequest, "plan inválido")
		return
	case req.SuspendedReason != nil && len(*req.SuspendedReason) > 300:
		writeErr(w, http.StatusBadRequest, "el motivo es demasiado largo")
		return
	}
	var periodEnd *time.Time
	if req.PeriodEnd != nil {
		t, err := time.Parse(dateLayout, *req.PeriodEnd)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "fecha inválida (AAAA-MM-DD)")
			return
		}
		t = t.Add(23*time.Hour + 59*time.Minute)
		periodEnd = &t
	}
	if req.TrialEndsAt != nil {
		t, err := time.Parse(dateLayout, *req.TrialEndsAt)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "fecha inválida (AAAA-MM-DD)")
			return
		}
		t = t.Add(23*time.Hour + 59*time.Minute) // vence al final de ese día
		trialEnds = &t
	}
	if req.Name != nil {
		t := strings.TrimSpace(*req.Name)
		req.Name = &t
	}

	ctx := r.Context()
	var before string
	if err := a.DB.QueryRow(ctx, `SELECT status FROM businesses WHERE id = $1`, id).Scan(&before); errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	} else if err != nil {
		serverErr(w)
		return
	}
	// al aprobar un negocio pendiente, arranca su prueba de 14 días
	startTrial := req.Status != nil && *req.Status == "trial" && before == "pending" && trialEnds == nil
	_, err := a.DB.Exec(ctx,
		`UPDATE businesses SET status = COALESCE($2, status), name = COALESCE($3, name),
		   trial_ends_at = CASE WHEN $5 THEN $9::timestamptz + interval '14 days' ELSE COALESCE($4, trial_ends_at) END,
		   plan = COALESCE($6, plan), current_period_end = COALESCE($7, current_period_end),
		   suspended_reason = CASE WHEN COALESCE($2, status) = 'suspended' THEN COALESCE($8, suspended_reason) ELSE '' END
		 WHERE id = $1`, id, req.Status, req.Name, trialEnds, startTrial, req.Plan, periodEnd, req.SuspendedReason, a.now())
	if err != nil {
		serverErr(w)
		return
	}
	detail := map[string]any{"from": before}
	if req.Status != nil {
		detail["to"] = *req.Status
	}
	if req.TrialEndsAt != nil {
		detail["trial_ends_at"] = *req.TrialEndsAt
	}
	if req.Plan != nil {
		detail["plan"] = *req.Plan
	}
	if req.PeriodEnd != nil {
		detail["current_period_end"] = *req.PeriodEnd
	}
	if req.SuspendedReason != nil {
		detail["reason"] = *req.SuspendedReason
	}
	a.logActivity(ctx, actor(r), id, "business_updated", detail)
	w.WriteHeader(http.StatusNoContent)
}

// ---------- usuarios de negocios ----------

// platformResetPassword fija una contraseña nueva a una persona de cualquier negocio.
func (a *API) platformResetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct{ Password string }
	if !decode(w, r, &req) || !validPassword(req.Password) {
		writeErr(w, http.StatusBadRequest, "la contraseña debe tener de 8 a 72 caracteres")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		serverErr(w)
		return
	}
	var bizID int64
	err = a.DB.QueryRow(r.Context(),
		`UPDATE users SET password_hash = $2 WHERE id = $1 AND business_id IS NOT NULL RETURNING business_id`, id, hash).Scan(&bizID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), actor(r), bizID, "user_password_reset", map[string]any{"user_id": id})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) platformUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Active *bool `json:"active"`
	}
	if !decode(w, r, &req) || req.Active == nil {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	var bizID int64
	err := a.DB.QueryRow(r.Context(),
		`UPDATE users SET active = $2 WHERE id = $1 AND business_id IS NOT NULL RETURNING business_id`, id, *req.Active).Scan(&bizID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), actor(r), bizID, "user_active_changed", map[string]any{"user_id": id, "active": *req.Active})
	w.WriteHeader(http.StatusNoContent)
}

// ---------- equipo de plataforma ----------

type platformMember struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Active    bool   `json:"active"`
	TwoFactor bool   `json:"two_factor"`
}

func (a *API) platformListTeam(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(r.Context(),
		`SELECT id, name, email, role, active, totp_secret_enc IS NOT NULL FROM users WHERE business_id IS NULL ORDER BY id`)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []platformMember{}
	for rows.Next() {
		var m platformMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.Role, &m.Active, &m.TwoFactor); err != nil {
			serverErr(w)
			return
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
}

func validPlatformRole(r string) bool {
	return r == auth.RolePlatformAdmin || r == auth.RolePlatformSupport
}

func (a *API) platformCreateTeam(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name, Email, Password, Role string }
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	switch {
	case !validName(req.Name):
		writeErr(w, http.StatusBadRequest, "el nombre es obligatorio")
		return
	case !validEmail(req.Email):
		writeErr(w, http.StatusBadRequest, "correo inválido")
		return
	case !validPassword(req.Password):
		writeErr(w, http.StatusBadRequest, "la contraseña debe tener de 8 a 72 caracteres")
		return
	case !validPlatformRole(req.Role):
		writeErr(w, http.StatusBadRequest, "rol inválido")
		return
	}
	id, err := a.insertPlatformUser(r.Context(), req.Name, req.Email, req.Password, req.Role)
	if errors.Is(err, errEmailTaken) {
		writeErr(w, http.StatusConflict, "ese correo ya está registrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), actor(r), 0, "platform_user_created", map[string]any{"user_id": id, "role": req.Role})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) insertPlatformUser(ctx context.Context, name, email, password, role string) (int64, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return 0, err
	}
	var id int64
	err = a.DB.QueryRow(ctx,
		`INSERT INTO users (business_id, email, password_hash, name, role) VALUES (NULL, $1, $2, $3, $4) RETURNING id`,
		email, hash, name, role).Scan(&id)
	if code, constraint := pgConstraint(err); code == "23505" && constraint == "users_email_key" {
		return 0, errEmailTaken
	}
	return id, err
}

func (a *API) platformUpdateTeam(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Role   *string `json:"role"`
		Active *bool   `json:"active"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	switch {
	case req.Role != nil && !validPlatformRole(*req.Role):
		writeErr(w, http.StatusBadRequest, "rol inválido")
		return
	case id == actor(r) && (req.Role != nil || (req.Active != nil && !*req.Active)):
		writeErr(w, http.StatusBadRequest, "no puedes cambiar tu propio rol ni desactivarte")
		return
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE users SET role = COALESCE($2, role), active = COALESCE($3, active) WHERE id = $1 AND business_id IS NULL`,
		id, req.Role, req.Active)
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	a.logActivity(r.Context(), actor(r), 0, "platform_user_updated", map[string]any{"user_id": id})
	w.WriteHeader(http.StatusNoContent)
}

// ---------- ajustes ----------

func (a *API) platformGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"registration_mode": a.registrationMode(r.Context())})
}

func (a *API) platformPutSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RegistrationMode string `json:"registration_mode"`
	}
	if !decode(w, r, &req) || (req.RegistrationMode != "open" && req.RegistrationMode != "approval" && req.RegistrationMode != "closed") {
		writeErr(w, http.StatusBadRequest, "modo de registro inválido (open, approval o closed)")
		return
	}
	if _, err := a.DB.Exec(r.Context(),
		`INSERT INTO platform_settings (key, value) VALUES ('registration_mode', $1)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, req.RegistrationMode); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), actor(r), 0, "settings_changed", map[string]any{"registration_mode": req.RegistrationMode})
	w.WriteHeader(http.StatusNoContent)
}

// businessMetrics son los conteos de uso de un negocio en los últimos 30 días y su última actividad (sin datos de clientes).
func (a *API) businessMetrics(ctx context.Context, id int64) map[string]any {
	since := a.now().AddDate(0, 0, -30)
	var appts, online, salesN int
	var lastAppt, lastSale *time.Time
	a.DB.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM appointments WHERE business_id = $1 AND created_at >= $2 AND status <> 'cancelled'),
		(SELECT count(*) FROM appointments WHERE business_id = $1 AND created_at >= $2 AND source = 'online'),
		(SELECT count(*) FROM sales WHERE business_id = $1 AND created_at >= $2 AND status = 'paid'),
		(SELECT max(created_at) FROM appointments WHERE business_id = $1),
		(SELECT max(created_at) FROM sales WHERE business_id = $1)`, id, since).Scan(&appts, &online, &salesN, &lastAppt, &lastSale)
	return map[string]any{"appointments_30d": appts, "online_bookings_30d": online, "sales_30d": salesN, "last_appointment_at": lastAppt, "last_sale_at": lastSale}
}
