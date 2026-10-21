package api

import (
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
}

// register crea el negocio, su primera sucursal y el usuario administrador.
func (a *API) register(w http.ResponseWriter, r *http.Request) {
	if !a.allow(w, "register:"+clientIP(r), 10, time.Hour) {
		return
	}
	var req registerReq
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.BusinessName = strings.TrimSpace(req.BusinessName)
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	switch {
	case !validName(req.BusinessName) || !validName(req.Name):
		writeErr(w, http.StatusBadRequest, "el nombre del negocio y tu nombre son obligatorios")
		return
	case !kinds[req.Kind]:
		writeErr(w, http.StatusBadRequest, "giro inválido")
		return
	case !validEmail(req.Email):
		writeErr(w, http.StatusBadRequest, "correo inválido")
		return
	case !validPassword(req.Password):
		writeErr(w, http.StatusBadRequest, "la contraseña debe tener de 8 a 72 caracteres")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}

	base := slugify(req.BusinessName)
	for attempt := 0; attempt < 5; attempt++ {
		slug := base
		if attempt > 0 { // el nombre ya existe: se agrega un sufijo corto
			b := make([]byte, 2)
			rand.Read(b)
			slug = base + "-" + hex.EncodeToString(b)
		}
		userID, err := a.createBusiness(r, req, slug, hash)
		if err == nil {
			token, _ := auth.Sign(a.Secret, userID)
			writeJSON(w, http.StatusCreated, map[string]any{"token": token})
			return
		}
		code, constraint := pgConstraint(err)
		switch {
		case code == "23505" && constraint == "users_email_key":
			writeErr(w, http.StatusConflict, "ese correo ya está registrado")
			return
		case code == "23505" && constraint == "businesses_slug_key":
			continue
		default:
			writeErr(w, http.StatusInternalServerError, "error interno")
			return
		}
	}
	writeErr(w, http.StatusConflict, "no se pudo asignar un nombre único al negocio, intenta con otro")
}

func (a *API) createBusiness(r *http.Request, req registerReq, slug, hash string) (int64, error) {
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var bizID, userID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO businesses (name, slug, kind) VALUES ($1, $2, $3) RETURNING id`,
		req.BusinessName, slug, req.Kind).Scan(&bizID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO branches (business_id, name) VALUES ($1, 'Principal')`, bizID); err != nil {
		return 0, err
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (business_id, email, password_hash, name, role) VALUES ($1, $2, $3, $4, 'admin') RETURNING id`,
		bizID, req.Email, hash, req.Name).Scan(&userID); err != nil {
		return 0, err
	}
	return userID, tx.Commit(ctx)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !a.allow(w, "login-ip:"+clientIP(r), 30, time.Minute) ||
		!a.allow(w, "login-email:"+email, 10, 15*time.Minute) {
		return
	}

	var id int64
	var hash string
	var active bool
	err := a.DB.QueryRow(r.Context(),
		`SELECT id, password_hash, active FROM users WHERE email = $1`, email).Scan(&id, &hash, &active)
	// mismo error si el correo no existe, está inactivo o la contraseña es incorrecta
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!active || !auth.CheckPassword(hash, req.Password))) {
		writeErr(w, http.StatusUnauthorized, "credenciales incorrectas")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	token, _ := auth.Sign(a.Secret, id)
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var name, email, biz, kind string
	err := a.DB.QueryRow(r.Context(),
		`SELECT u.name, u.email, b.name, b.kind FROM users u JOIN businesses b ON b.id = u.business_id
		 WHERE u.id = $1`, p.UserID).Scan(&name, &email, &biz, &kind)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": p.UserID, "name": name, "email": email, "role": p.Role,
		"business": map[string]any{"id": p.BusinessID, "name": biz, "kind": kind},
	})
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
	w.WriteHeader(http.StatusNoContent)
}
