package api

import (
	"net/http"
	"strings"
	"time"

	"miestudio/backend-go/internal/auth"
)

// Datos del negocio: nombre, giro y zona horaria. La dirección pública (identificador) no cambia aquí:
// cambiarla rompería los enlaces que el negocio ya compartió.

func (a *API) getBusiness(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var name, slug, kind, tz string
	if err := a.DB.QueryRow(r.Context(), `SELECT name, slug, kind, timezone FROM businesses WHERE id = $1`, p.BusinessID).Scan(&name, &slug, &kind, &tz); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "slug": slug, "kind": kind, "timezone": tz})
}

func (a *API) putBusiness(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Name     string `json:"name"`
		Kind     string `json:"kind"`
		Timezone string `json:"timezone"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name, req.Timezone = strings.TrimSpace(req.Name), strings.TrimSpace(req.Timezone)
	if !validName(req.Name) {
		writeErr(w, http.StatusBadRequest, "el nombre del negocio es obligatorio")
		return
	}
	if !kinds[req.Kind] {
		writeErr(w, http.StatusBadRequest, "giro inválido")
		return
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil || req.Timezone == "" || req.Timezone == "Local" {
		writeErr(w, http.StatusBadRequest, "zona horaria inválida")
		return
	}
	var before string
	a.DB.QueryRow(r.Context(), `SELECT timezone FROM businesses WHERE id = $1`, p.BusinessID).Scan(&before)
	if _, err := a.DB.Exec(r.Context(), `UPDATE businesses SET name = $2, kind = $3, timezone = $4 WHERE id = $1`, p.BusinessID, req.Name, req.Kind, req.Timezone); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), p.UserID, p.BusinessID, "business_profile_changed", map[string]any{"kind": req.Kind, "timezone_changed": before != req.Timezone})
	w.WriteHeader(http.StatusNoContent)
}

// completeSetup marca terminado (o omitido) el asistente de inicio.
func (a *API) completeSetup(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if _, err := a.DB.Exec(r.Context(), `UPDATE businesses SET setup_completed_at = COALESCE(setup_completed_at, now()) WHERE id = $1`, p.BusinessID); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
