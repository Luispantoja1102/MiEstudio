package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"miestudio/backend-go/internal/auth"
)

var (
	nonSlug = regexp.MustCompile(`[^a-z0-9]+`)
	kinds   = map[string]bool{"barberia": true, "estetica": true, "spa": true, "unas": true}
)

func slugify(s string) string {
	// quita acentos comunes del español antes de limpiar
	s = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n").Replace(strings.ToLower(s))
	s = strings.Trim(nonSlug.ReplaceAllString(s, "-"), "-")
	if s == "" {
		return "negocio"
	}
	return s
}

func validPassword(p string) bool { return len(p) >= 8 && len(p) <= 72 } // 72 = límite de bcrypt

func validEmail(e string) bool {
	return len(e) <= 254 && strings.Contains(e, "@") && !strings.ContainsAny(e, " \t\n")
}

func validName(s string) bool { n := utf8.RuneCountInString(s); return n >= 1 && n <= 120 }

// allow aplica el límite de intentos; responde 429 y devuelve false si se excede.
func (a *API) allow(w http.ResponseWriter, key string, max int, window time.Duration) bool {
	if a.Limiter == nil || a.Limiter.Allow(key, max, window) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeErr(w, http.StatusTooManyRequests, "demasiados intentos, espera un momento")
	return false
}

func pgConstraint(err error) (code, name string) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code, pg.ConstraintName
	}
	return "", ""
}

type registerReq struct {
	BusinessName  string `json:"business_name"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Username      string `json:"username"` // opcional: si falta se genera a partir del correo
	Phone         string `json:"phone"`    // opcional: queda en la primera sucursal
	Password      string `json:"password"`
	AcceptTerms   bool   `json:"accept_terms"` // términos y aviso de privacidad de la plataforma
	EmailCode     string `json:"email_code"`   // código de 6 dígitos que llegó al correo (si el servidor lo exige)
	Plan          string `json:"plan"`         // plan que pide al registrarse; mientras se aprueba, prueba el Básico
	requestedPlan string // lo fija el registro público; el alta que hace la plataforma no lo usa
}

var errEmailTaken = errors.New("correo ya registrado")
var errUsernameTaken = errors.New("usuario ya registrado")

// lowerTrim limpia un correo o un usuario: sin espacios alrededor y en minúsculas.
func lowerTrim(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// userConflict traduce el choque de un correo o usuario repetido en la tabla de personas; devuelve nil si el error es otro.
func userConflict(err error) error {
	code, constraint := pgConstraint(err)
	switch {
	case code != "23505":
		return nil
	case constraint == "users_email_key":
		return errEmailTaken
	case constraint == "users_username_key":
		return errUsernameTaken
	}
	return nil
}

// writeUserConflict responde 409 si el error es un correo o usuario repetido; devuelve false si es otra cosa.
func writeUserConflict(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, errEmailTaken):
		writeErr(w, http.StatusConflict, "ese correo ya está registrado")
	case errors.Is(err, errUsernameTaken):
		writeErr(w, http.StatusConflict, "ese usuario ya está en uso")
	default:
		return false
	}
	return true
}

// El usuario se guarda en minúsculas: letras, números, punto, guion y guion bajo (nunca "@", para no confundirse con un correo).
var usernameRe = regexp.MustCompile(`^[a-z0-9._-]{3,30}$`)

const usernameHelp = "el usuario debe tener de 3 a 30 caracteres: letras, números, punto, guion o guion bajo"

func validUsername(s string) bool { return usernameRe.MatchString(s) }

var nonUsername = regexp.MustCompile(`[^a-z0-9._-]+`)

// uniqueUsername propone un usuario libre a partir de una base (normalmente la parte local de un correo).
func (a *API) uniqueUsername(ctx context.Context, q queryer, base string) string {
	base = nonUsername.ReplaceAllString(strings.ToLower(base), "")
	if len(base) < 3 {
		base += "user"
	}
	if len(base) > 24 {
		base = base[:24]
	}
	cand := base
	for n := 2; n < 1000; n++ {
		var taken bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE username = $1)`, cand).Scan(&taken); err != nil || !taken {
			return cand
		}
		cand = base + strconv.Itoa(n)
	}
	b := make([]byte, 3)
	rand.Read(b)
	return base + hex.EncodeToString(b)
}

// emailLocal es la parte del correo antes de la @.
func emailLocal(email string) string {
	local, _, _ := strings.Cut(email, "@")
	return local
}

// registrationMode devuelve "open", "approval" o "closed" (por defecto, abierto).
func (a *API) registrationMode(ctx context.Context) string {
	var v string
	if err := a.DB.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = 'registration_mode'`).Scan(&v); err != nil {
		return "open"
	}
	return v
}

func (a *API) registrationInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"mode": a.registrationMode(r.Context()), "email_verification": a.signupVerifyRequired()})
}

// validateRegister limpia y valida los datos de alta de un negocio.
func validateRegister(req *registerReq) string {
	req.BusinessName = strings.TrimSpace(req.BusinessName)
	req.Name = strings.TrimSpace(req.Name)
	req.Email = lowerTrim(req.Email)
	req.Username = lowerTrim(req.Username)
	req.Phone = strings.TrimSpace(req.Phone)
	switch {
	case !validName(req.BusinessName) || !validName(req.Name):
		return "el nombre del negocio y el nombre de la persona son obligatorios"
	case !kinds[req.Kind]:
		return "giro inválido"
	case !validEmail(req.Email):
		return "correo inválido"
	case req.Username != "" && !validUsername(req.Username):
		return usernameHelp
	case len(req.Phone) > 30:
		return "el teléfono es demasiado largo"
	case !validPassword(req.Password):
		return "la contraseña debe tener de 8 a 72 caracteres"
	}
	return ""
}

// register crea el negocio, su primera sucursal y el usuario administrador.
// El estado inicial depende del modo de registro que fije la plataforma.
func (a *API) register(w http.ResponseWriter, r *http.Request) {
	if !a.allow(w, "register:"+clientIP(r), 10, time.Hour) {
		return
	}
	mode := a.registrationMode(r.Context())
	if mode == "closed" {
		writeErr(w, http.StatusForbidden, "el registro está cerrado por ahora")
		return
	}
	var req registerReq
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if msg := validateRegister(&req); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if !req.AcceptTerms { // el alta que hace la plataforma por alguien más no pasa por aquí
		writeErr(w, http.StatusBadRequest, "debes aceptar los términos y el aviso de privacidad")
		return
	}
	// sin plan elegido se pide el Básico; uno inexistente se rechaza
	if req.Plan == "" {
		req.Plan = "basico"
	}
	if !a.planExists(r.Context(), req.Plan) {
		writeErr(w, http.StatusBadRequest, "el plan elegido no existe")
		return
	}
	req.requestedPlan = req.Plan
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		serverErr(w)
		return
	}
	if fail := a.consumeSignupCode(r.Context(), req.Email, req.EmailCode); fail != nil {
		writeFail(w, fail)
		return
	}
	status := "trial"
	if mode == "approval" {
		status = "pending"
	}
	bizID, userID, err := a.registerBusiness(r.Context(), req, hash, status)
	switch {
	case writeUserConflict(w, err):
	case err != nil:
		serverErr(w)
	default:
		a.logActivity(r.Context(), 0, bizID, "business_registered", map[string]any{"status": status, "email": req.Email})
		a.welcomeEmail(req.Email, req.Name, req.BusinessName, status == "pending")
		a.signupNotice(r.Context(), req, bizID, status)
		token, _ := a.startSession(w, r, userID)
		writeJSON(w, http.StatusCreated, map[string]any{"token": token, "status": status})
	}
}

// registerBusiness inserta el negocio con un identificador único, reintentando con sufijo si ya existe.
func (a *API) registerBusiness(ctx context.Context, req registerReq, hash, status string) (bizID, userID int64, err error) {
	if req.Username == "" {
		req.Username = a.uniqueUsername(ctx, a.DB, emailLocal(req.Email))
	}
	base := slugify(req.BusinessName)
	for attempt := 0; attempt < 5; attempt++ {
		slug := base
		if attempt > 0 { // el nombre ya existe: se agrega un sufijo corto
			b := make([]byte, 2)
			rand.Read(b)
			slug = base + "-" + hex.EncodeToString(b)
		}
		bizID, userID, err = a.createBusiness(ctx, req, slug, hash, status)
		if err == nil {
			return bizID, userID, nil
		}
		if conflict := userConflict(err); conflict != nil {
			return 0, 0, conflict
		}
		if code, constraint := pgConstraint(err); code == "23505" && constraint == "businesses_slug_key" {
			continue
		}
		return 0, 0, err
	}
	return 0, 0, errors.New("no se pudo asignar un identificador único al negocio")
}

func (a *API) createBusiness(ctx context.Context, req registerReq, slug, hash, status string) (bizID, userID int64, err error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx,
		`INSERT INTO businesses (name, slug, kind, status, trial_ends_at, plan, requested_plan)
		 VALUES ($1, $2, $3, $4, CASE WHEN $4 = 'trial' THEN now() + interval '14 days' END,
		   CASE WHEN $5 = '' THEN 'crecimiento' ELSE 'basico' END, NULLIF($5, '')) RETURNING id`,
		req.BusinessName, slug, req.Kind, status, req.requestedPlan).Scan(&bizID); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO branches (business_id, name, phone) VALUES ($1, 'Principal', $2)`, bizID, req.Phone); err != nil {
		return 0, 0, err
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (business_id, email, username, password_hash, name, role, terms_version, terms_accepted_at)
		 VALUES ($1, $2, $3, $4, $5, 'admin', CASE WHEN $6 THEN $7 END, CASE WHEN $6 THEN now() END) RETURNING id`,
		bizID, req.Email, req.Username, hash, req.Name, req.AcceptTerms, NoticeVersion).Scan(&userID); err != nil {
		return 0, 0, err
	}
	// el primer administrador es el titular del negocio
	if _, err := tx.Exec(ctx, `UPDATE businesses SET owner_user_id = $2 WHERE id = $1`, bizID, userID); err != nil {
		return 0, 0, err
	}
	return bizID, userID, tx.Commit(ctx)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	// Se entra con el usuario o con el correo en el mismo campo ("identifier"); "email" se sigue aceptando por compatibilidad.
	var req struct{ Identifier, Email, Password string }
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	who := strings.TrimSpace(req.Identifier)
	if who == "" {
		who = strings.TrimSpace(req.Email)
	}
	who = strings.ToLower(who) // mayúsculas y minúsculas dan igual, tanto en el usuario como en el correo
	// el inicio de sesión no se bloquea por intentos (decisión del dueño): un fallo siempre responde 401 con el mismo mensaje

	var id int64
	var hash string
	var active, twoFactor bool
	// un usuario nunca lleva "@", así que no puede coincidir con el correo de otra persona
	err := a.DB.QueryRow(r.Context(),
		`SELECT id, password_hash, active, totp_secret_enc IS NOT NULL FROM users WHERE lower(email) = $1 OR username = $1 LIMIT 1`, who).Scan(&id, &hash, &active, &twoFactor)
	// mismo error si el usuario o correo no existe, está inactivo o la contraseña es incorrecta
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!active || !auth.CheckPassword(hash, req.Password))) {
		writeErr(w, http.StatusUnauthorized, "credenciales incorrectas")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	if twoFactor { // la contraseña sola no abre la sesión
		challenge, _ := auth.SignPurpose(a.Secret, purpose2FA, id, challengeTTL)
		writeJSON(w, http.StatusOK, map[string]any{"needs_2fa": true, "challenge": challenge})
		return
	}
	token, _ := a.startSession(w, r, id)
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

// startSession firma el token con la versión de sesión vigente y lo deja en la cookie HttpOnly.
// El token también se devuelve en el cuerpo para herramientas que no usan cookies; la interfaz web no lo guarda.
func (a *API) startSession(w http.ResponseWriter, r *http.Request, userID int64) (string, error) {
	var sv int
	if err := a.DB.QueryRow(r.Context(), `SELECT session_version FROM users WHERE id = $1`, userID).Scan(&sv); err != nil {
		return "", err
	}
	token, err := auth.Sign(a.Secret, userID, sv)
	if err != nil {
		return "", err
	}
	auth.SetCookie(w, token, a.secureCookie(r))
	return token, nil
}

// logout cierra esta sesión en el navegador.
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearCookie(w, a.secureCookie(r))
	w.WriteHeader(http.StatusNoContent)
}

// logoutAll cierra todas las sesiones de la persona, en todos sus dispositivos.
func (a *API) logoutAll(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if _, err := a.DB.Exec(r.Context(), `UPDATE users SET session_version = session_version + 1 WHERE id = $1`, p.UserID); err != nil {
		serverErr(w)
		return
	}
	auth.ClearCookie(w, a.secureCookie(r))
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var name, email, username string
	var bizName, kind, status, tz, plan *string
	var trialEnds, periodEnd, setupDone *time.Time
	err := a.DB.QueryRow(r.Context(),
		`SELECT u.name, u.email, u.username, b.name, b.kind, b.status, b.trial_ends_at, b.timezone, b.plan, b.current_period_end, b.setup_completed_at
		 FROM users u LEFT JOIN businesses b ON b.id = u.business_id WHERE u.id = $1`, p.UserID).
		Scan(&name, &email, &username, &bizName, &kind, &status, &trialEnds, &tz, &plan, &periodEnd, &setupDone)
	if err != nil {
		serverErr(w)
		return
	}
	var twoFactor bool
	a.DB.QueryRow(r.Context(), `SELECT totp_secret_enc IS NOT NULL FROM users WHERE id = $1`, p.UserID).Scan(&twoFactor)
	out := map[string]any{"id": p.UserID, "name": name, "email": email, "username": username, "role": p.Role, "business": nil, "two_factor": twoFactor}
	if p.BusinessID != 0 {
		isOwner := p.Role == "admin" && a.isOwner(r.Context(), p.BusinessID, p.UserID)
		out["business"] = map[string]any{
			"id": p.BusinessID, "name": bizName, "kind": kind, "status": status, "trial_ends_at": trialEnds, "timezone": tz,
			"plan": plan, "billing_state": p.BillingState, "current_period_end": periodEnd,
			"is_owner":      isOwner,
			"plan_features": a.planFeatures(r.Context(), p.BusinessID),
			// el asistente de inicio lo ve solo el titular, y solo mientras no lo termine u omita
			"setup_pending": isOwner && setupDone == nil,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) changePassword(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if !a.allow(w, "password:"+clientIP(r), 10, 15*time.Minute) {
		return
	}
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !decode(w, r, &req) || !validPassword(req.New) {
		writeErr(w, http.StatusBadRequest, "la nueva contraseña debe tener de 8 a 72 caracteres")
		return
	}
	var hash string
	if err := a.DB.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id = $1`, p.UserID).Scan(&hash); err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	if !auth.CheckPassword(hash, req.Current) {
		writeErr(w, http.StatusUnauthorized, "la contraseña actual es incorrecta")
		return
	}
	newHash, err := auth.HashPassword(req.New)
	if err == nil {
		_, err = a.DB.Exec(r.Context(), `UPDATE users SET password_hash = $1 WHERE id = $2`, newHash, p.UserID)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	// la clave cambió: todas las sesiones anteriores quedaron cerradas; esta se renueva para no sacar a quien la cambió
	if _, err := a.startSession(w, r, p.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
