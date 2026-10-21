package api

import (
	"net/http"
	"strconv"
	"strings"

	"miestudio/backend-go/internal/auth"
)

var roles = map[string]bool{"admin": true, "staff": true, "reception": true, "cashier": true}

type member struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	Role          string  `json:"role"`
	Bookable      bool    `json:"bookable"`
	CommissionPct float64 `json:"commission_pct"`
	Active        bool    `json:"active"`
	TwoFactor     bool    `json:"two_factor"`
}

func (a *API) listTeam(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := a.DB.Query(r.Context(),
		`SELECT id, name, email, role, bookable, commission_pct::float8, active, totp_secret_enc IS NOT NULL
		 FROM users WHERE business_id = $1 ORDER BY name`, p.BusinessID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	defer rows.Close()
	out := []member{}
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.Role, &m.Bookable, &m.CommissionPct, &m.Active, &m.TwoFactor); err != nil {
			writeErr(w, http.StatusInternalServerError, "error interno")
			return
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createTeamMember(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Name          string  `json:"name"`
		Email         string  `json:"email"`
		Password      string  `json:"password"`
		Role          string  `json:"role"`
		Bookable      bool    `json:"bookable"`
		CommissionPct float64 `json:"commission_pct"`
	}
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
	case !roles[req.Role]:
		writeErr(w, http.StatusBadRequest, "rol inválido")
		return
	case req.CommissionPct < 0 || req.CommissionPct > 100:
		writeErr(w, http.StatusBadRequest, "la comisión debe estar entre 0 y 100")
		return
	}
	if req.Bookable {
		if err := a.checkStaffRoom(r.Context(), p.BusinessID); err != nil {
			writeFail(w, err)
			return
		}
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	var id int64
	err = a.DB.QueryRow(r.Context(),
		`INSERT INTO users (business_id, email, password_hash, name, role, bookable, commission_pct)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		p.BusinessID, req.Email, hash, req.Name, req.Role, req.Bookable, req.CommissionPct).Scan(&id)
	if code, _ := pgConstraint(err); code == "23505" {
		writeErr(w, http.StatusConflict, "ese correo ya está registrado")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// updateTeamMember cambia rol, estado, agenda o comisión. Solo dentro del propio negocio.
func (a *API) updateTeamMember(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "id inválido")
		return
	}
	var req struct {
		Role          *string  `json:"role"`
		Active        *bool    `json:"active"`
		Bookable      *bool    `json:"bookable"`
		CommissionPct *float64 `json:"commission_pct"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	switch {
	case req.Role != nil && !roles[*req.Role]:
		writeErr(w, http.StatusBadRequest, "rol inválido")
		return
	case req.CommissionPct != nil && (*req.CommissionPct < 0 || *req.CommissionPct > 100):
		writeErr(w, http.StatusBadRequest, "la comisión debe estar entre 0 y 100")
		return
	case id == p.UserID && (req.Role != nil || (req.Active != nil && !*req.Active)):
		// evita dejar el negocio sin administrador
		writeErr(w, http.StatusBadRequest, "no puedes cambiar tu propio rol ni desactivarte")
		return
	}
	// pasar a ser una persona activa en la agenda ocupa un lugar del plan
	var curActive, curBookable bool
	if err := a.DB.QueryRow(r.Context(), `SELECT active, bookable FROM users WHERE id = $1 AND business_id = $2`, id, p.BusinessID).Scan(&curActive, &curBookable); err == nil {
		newActive, newBookable := curActive, curBookable
		if req.Active != nil {
			newActive = *req.Active
		}
		if req.Bookable != nil {
			newBookable = *req.Bookable
		}
		if newActive && newBookable && !(curActive && curBookable) {
			if err := a.checkStaffRoom(r.Context(), p.BusinessID); err != nil {
				writeFail(w, err)
				return
			}
		}
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE users SET role = COALESCE($3, role), active = COALESCE($4, active),
		   bookable = COALESCE($5, bookable), commission_pct = COALESCE($6, commission_pct)
		 WHERE id = $1 AND business_id = $2`,
		id, p.BusinessID, req.Role, req.Active, req.Bookable, req.CommissionPct)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "error interno")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
