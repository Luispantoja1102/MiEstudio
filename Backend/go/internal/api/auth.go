package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"miestudio/backend-go/internal/auth"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
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
	var req registerReq
	if !decode(r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	slug := slugify(req.BusinessName)
	if slug == "" || req.Name == "" || !strings.Contains(req.Email, "@") || len(req.Password) < 8 || len(req.Password) > 72 {
		writeErr(w, http.StatusBadRequest, "datos incompletos o contraseña de 8 a 72 caracteres")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}

	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	defer tx.Rollback(r.Context())

	var bizID, userID int64
	err = tx.QueryRow(r.Context(),
		`INSERT INTO businesses (name, slug, kind) VALUES ($1, $2, $3) RETURNING id`,
		req.BusinessName, slug, req.Kind).Scan(&bizID)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO branches (business_id, name) VALUES ($1, 'Principal')`, bizID)
	}
	if err == nil {
		err = tx.QueryRow(r.Context(),
			`INSERT INTO users (business_id, email, password_hash, name, role) VALUES ($1, $2, $3, $4, 'admin') RETURNING id`,
			bizID, req.Email, hash, req.Name).Scan(&userID)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			writeErr(w, http.StatusConflict, "el correo o el nombre del negocio ya existe")
			return
		}
		if errors.As(err, &pg) && pg.Code == "23514" {
			writeErr(w, http.StatusBadRequest, "giro inválido")
			return
		}
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}

	token, _ := auth.Sign(a.Secret, userID, bizID, "admin")
	writeJSON(w, http.StatusCreated, map[string]any{"token": token})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Email, Password string }
	if !decode(r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	var id, bizID int64
	var hash, role string
	var active bool
	err := a.DB.QueryRow(r.Context(),
		`SELECT id, business_id, password_hash, role, active FROM users WHERE email = $1`,
		strings.ToLower(strings.TrimSpace(req.Email))).Scan(&id, &bizID, &hash, &role, &active)
	// mismo error si el correo no existe o la contraseña es incorrecta
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!active || !auth.CheckPassword(hash, req.Password))) {
		writeErr(w, http.StatusUnauthorized, "credenciales incorrectas")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	token, _ := auth.Sign(a.Secret, id, bizID, role)
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	var name, email, biz, kind string
	err := a.DB.QueryRow(r.Context(),
		`SELECT u.name, u.email, b.name, b.kind FROM users u JOIN businesses b ON b.id = u.business_id
		 WHERE u.id = $1 AND u.active`, c.UserID).Scan(&name, &email, &biz, &kind)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": c.UserID, "name": name, "email": email, "role": c.Role,
		"business": map[string]any{"id": c.BusinessID, "name": biz, "kind": kind},
	})
}
