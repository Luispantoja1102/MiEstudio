package api

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"miestudio/backend-go/internal/auth"
)

// Precios y límites de los planes, editables por la plataforma. Lo que se guarda aquí manda sobre lo definido en el código:
// la página de inicio, la pantalla de suscripción de cada negocio y los cobros nuevos lo usan en cuanto se guarda.

type planEdit struct {
	Description string `json:"description"`
	PriceMonth  int    `json:"price_month"`
	PriceYear   int    `json:"price_year"`
	MaxStaff    *int   `json:"max_staff"` // sin valor = sin límite
	MaxBranches int    `json:"max_branches"`
	POS         bool   `json:"pos"`
	Deposits    bool   `json:"deposits"`
	Terminal    bool   `json:"terminal"`
	Support     string `json:"support"`
}

func (a *API) platformPutPlan(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := r.PathValue("id")
	if !a.planExists(r.Context(), id) {
		writeErr(w, http.StatusNotFound, "plan no encontrado")
		return
	}
	var req planEdit
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Description = strings.TrimSpace(req.Description)
	switch {
	case req.PriceMonth < 0 || req.PriceMonth > 1000000 || req.PriceYear < 0 || req.PriceYear > 10000000:
		writeErr(w, http.StatusBadRequest, "los precios deben estar entre $0 y $1,000,000")
		return
	case req.MaxStaff != nil && (*req.MaxStaff < 1 || *req.MaxStaff > 10000):
		writeErr(w, http.StatusBadRequest, "las personas en agenda deben ser de 1 a 10,000 (o sin límite)")
		return
	case req.MaxBranches < 1 || req.MaxBranches > 1000:
		writeErr(w, http.StatusBadRequest, "las sucursales deben ser de 1 a 1,000")
		return
	case req.Support != "correo" && req.Support != "prioritario" && req.Support != "dedicado":
		writeErr(w, http.StatusBadRequest, "el soporte debe ser correo, prioritario o dedicado")
		return
	case utf8.RuneCountInString(req.Description) > 120:
		writeErr(w, http.StatusBadRequest, "la descripción es demasiado larga (máximo 120 caracteres)")
		return
	}
	if _, err := a.DB.Exec(r.Context(),
		`INSERT INTO plan_settings (plan_id, description, price_month, price_year, max_staff, max_branches, pos, deposits, terminal, support, updated_at, updated_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT (plan_id) DO UPDATE SET description = EXCLUDED.description, price_month = EXCLUDED.price_month, price_year = EXCLUDED.price_year,
		   max_staff = EXCLUDED.max_staff, max_branches = EXCLUDED.max_branches, pos = EXCLUDED.pos, deposits = EXCLUDED.deposits,
		   terminal = EXCLUDED.terminal, support = EXCLUDED.support, updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by`,
		id, req.Description, req.PriceMonth, req.PriceYear, req.MaxStaff, req.MaxBranches, req.POS, req.Deposits, req.Terminal, req.Support, time.Now(), p.UserID); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), p.UserID, 0, "plan_changed", map[string]any{"plan": id, "price_month": req.PriceMonth, "price_year": req.PriceYear})
	w.WriteHeader(http.StatusNoContent)
}

// platformResetPlan devuelve un plan a lo definido en el código.
func (a *API) platformResetPlan(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := r.PathValue("id")
	if !a.planExists(r.Context(), id) {
		writeErr(w, http.StatusNotFound, "plan no encontrado")
		return
	}
	if _, err := a.DB.Exec(r.Context(), `DELETE FROM plan_settings WHERE plan_id = $1`, id); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), p.UserID, 0, "plan_reset", map[string]any{"plan": id})
	w.WriteHeader(http.StatusNoContent)
}
