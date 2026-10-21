package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
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
	BusinessName string `json:"business_name"`
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	AcceptTerms  bool   `json:"accept_terms"` // términos y aviso de privacidad de la plataforma
}

var errEmailTaken = errors.New("correo ya registrado")

// registrationMode devuelve "open", "approval" o "closed" (por defecto, abierto).
func (a *API) registrationMode(ctx context.Context) string {
	var v string
	if err := a.DB.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = 'registration_mode'`).Scan(&v); err != nil {
		return "open"
	}
	return v
}

func (a *API) registrationInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"mode": a.registrationMode(r.Context())})
}

// validateRegister limpia y valida los datos de alta de un negocio.
func validateRegister(req *registerReq) string {
	req.BusinessName = strings.TrimSpace(req.BusinessName)
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	switch {
	case !validName(req.BusinessName) || !validName(req.Name):
		return "el nombre del negocio y el nombre de la persona son obligatorios"
	case !kinds[req.Kind]:
		return "giro inválido"
	case !validEmail(req.Email):
		return "correo inválido"
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
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		serverErr(w)
		return
	}
	status := "trial"
	if mode == "approval" {
		status = "pending"
	}
	bizID, userID, err := a.registerBusiness(r.Context(), req, hash, status)
	switch {
	case errors.Is(err, errEmailTaken):
		writeErr(w, http.StatusConflict, "ese correo ya está registrado")
	case err != nil:
		serverErr(w)
	default:
		a.logActivity(r.Context(), 0, bizID, "business_registered", map[string]any{"status": status, "email": req.Email})
		token, _ := a.startSession(w, r, userID)
		writeJSON(w, http.StatusCreated, map[string]any{"token": token, "status": status})
	}
}

// registerBusiness inserta el negocio con un identificador único, reintentando con sufijo si ya existe.
func (a *API) registerBusiness(ctx context.Context, req registerReq, hash, status string) (bizID, userID int64, err error) {
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
		code, constraint := pgConstraint(err)
		switch {
		case code == "23505" && constraint == "users_email_key":
			return 0, 0, errEmailTaken
		case code == "23505" && constraint == "businesses_slug_key":
			continue
		default:
			return 0, 0, err
		}
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
		`INSERT INTO businesses (name, slug, kind, status, trial_ends_at)
		 VALUES ($1, $2, $3, $4, CASE WHEN $4 = 'trial' THEN now() + interval '14 days' END) RETURNING id`,
		req.BusinessName, slug, req.Kind, status).Scan(&bizID); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO branches (business_id, name) VALUES ($1, 'Principal')`, bizID); err != nil {
		return 0, 0, err
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (business_id, email, password_hash, name, role, terms_version, terms_accepted_at)
		 VALUES ($1, $2, $3, $4, 'admin', CASE WHEN $5 THEN $6 END, CASE WHEN $5 THEN now() END) RETURNING id`,
		bizID, req.Email, hash, req.Name, req.AcceptTerms, NoticeVersion).Scan(&userID); err != nil {
		return 0, 0, err
	}
	return bizID, userID, tx.Commit(ctx)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	// el inicio de sesión no se bloquea por intentos (decisión del dueño): un fallo siempre responde 401 con el mismo mensaje

	var id int64
	var hash string
	var active, twoFactor bool
	err := a.DB.QueryRow(r.Context(),
		`SELECT id, password_hash, active, totp_secret_enc IS NOT NULL FROM users WHERE email = $1`, email).Scan(&id, &hash, &active, &twoFactor)
	// mismo error si el correo no existe, está inactivo o la contraseña es incorrecta
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
	var name, email string
	var bizName, kind, status, tz, plan *string
	var trialEnds, periodEnd *time.Time
	err := a.DB.QueryRow(r.Context(),
		`SELECT u.name, u.email, b.name, b.kind, b.status, b.trial_ends_at, b.timezone, b.plan, b.current_period_end
		 FROM users u LEFT JOIN businesses b ON b.id = u.business_id WHERE u.id = $1`, p.UserID).
		Scan(&name, &email, &bizName, &kind, &status, &trialEnds, &tz, &plan, &periodEnd)
	if err != nil {
		serverErr(w)
		return
	}
	var twoFactor bool
	a.DB.QueryRow(r.Context(), `SELECT totp_secret_enc IS NOT NULL FROM users WHERE id = $1`, p.UserID).Scan(&twoFactor)
	out := map[string]any{"id": p.UserID, "name": name, "email": email, "role": p.Role, "business": nil, "two_factor": twoFactor}
	if p.BusinessID != 0 {
		out["business"] = map[string]any{
			"id": p.BusinessID, "name": bizName, "kind": kind, "status": status, "trial_ends_at": trialEnds, "timezone": tz,
			"plan": plan, "billing_state": p.BillingState, "current_period_end": periodEnd,
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
