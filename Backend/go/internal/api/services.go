package api

import (
	"net/http"
	"strings"

	"miestudio/backend-go/internal/auth"
)

type service struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	DurationMin int     `json:"duration_min"`
	BufferMin   int     `json:"buffer_min"`
	PriceCents  int     `json:"price_cents"`
	Active      bool    `json:"active"`
	Online      bool    `json:"online_bookable"`
	CategoryID  *int64  `json:"category_id"`
	StaffIDs    []int64 `json:"staff_ids"`
}

const maxPriceCents = 100_000_000

func validService(name string, duration, buffer, price int) string {
	switch {
	case !validName(name):
		return "el nombre es obligatorio"
	case duration < 5 || duration > 480:
		return "la duración debe estar entre 5 y 480 minutos"
	case buffer < 0 || buffer > 240:
		return "el tiempo de preparación debe estar entre 0 y 240 minutos"
	case price < 0 || price > maxPriceCents:
		return "precio inválido"
	}
	return ""
}

func (a *API) listServices(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := a.DB.Query(r.Context(),
		`SELECT s.id, s.name, s.duration_min, s.buffer_min, s.price_cents, s.active, s.online_bookable, s.category_id,
		        COALESCE(array_agg(ss.user_id ORDER BY ss.user_id) FILTER (WHERE ss.user_id IS NOT NULL), '{}')
		 FROM services s LEFT JOIN service_staff ss ON ss.service_id = s.id
		 WHERE s.business_id = $1 GROUP BY s.id ORDER BY s.name`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []service{}
	for rows.Next() {
		var s service
		if err := rows.Scan(&s.ID, &s.Name, &s.DurationMin, &s.BufferMin, &s.PriceCents, &s.Active, &s.Online, &s.CategoryID, &s.StaffIDs); err != nil {
			serverErr(w)
			return
		}
		out = append(out, s)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createService(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Name        string `json:"name"`
		DurationMin int    `json:"duration_min"`
		BufferMin   int    `json:"buffer_min"`
		PriceCents  int    `json:"price_cents"`
		CategoryID  *int64 `json:"category_id"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if msg := validService(req.Name, req.DurationMin, req.BufferMin, req.PriceCents); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if !a.categoryAllowed(w, r, p.BusinessID, req.CategoryID) {
		return
	}
	var id int64
	err := a.DB.QueryRow(r.Context(),
		`INSERT INTO services (business_id, name, duration_min, buffer_min, price_cents, category_id) VALUES ($1,$2,$3,$4,$5, NULLIF($6::bigint, 0)) RETURNING id`,
		p.BusinessID, req.Name, req.DurationMin, req.BufferMin, req.PriceCents, req.CategoryID).Scan(&id)
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) updateService(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name        *string `json:"name"`
		DurationMin *int    `json:"duration_min"`
		BufferMin   *int    `json:"buffer_min"`
		PriceCents  *int    `json:"price_cents"`
		Active      *bool   `json:"active"`
		Online      *bool   `json:"online_bookable"`
		CategoryID  *int64  `json:"category_id"` // ausente = sin cambio; 0 = quitar la categoría
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if !a.categoryAllowed(w, r, p.BusinessID, req.CategoryID) {
		return
	}
	if req.Name != nil {
		t := strings.TrimSpace(*req.Name)
		req.Name = &t
	}
	// valida solo los campos enviados, usando valores neutros para el resto
	name, dur, buf, price := "x", 30, 0, 0
	if req.Name != nil {
		name = *req.Name
	}
	if req.DurationMin != nil {
		dur = *req.DurationMin
	}
	if req.BufferMin != nil {
		buf = *req.BufferMin
	}
	if req.PriceCents != nil {
		price = *req.PriceCents
	}
	if msg := validService(name, dur, buf, price); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE services SET name = COALESCE($3, name), duration_min = COALESCE($4, duration_min),
		   buffer_min = COALESCE($5, buffer_min), price_cents = COALESCE($6, price_cents), active = COALESCE($7, active),
		   online_bookable = COALESCE($8, online_bookable),
		   category_id = CASE WHEN $9::bigint IS NULL THEN category_id WHEN $9::bigint = 0 THEN NULL ELSE $9::bigint END
		 WHERE id = $1 AND business_id = $2`,
		id, p.BusinessID, req.Name, req.DurationMin, req.BufferMin, req.PriceCents, req.Active, req.Online, req.CategoryID)
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// categoryAllowed revisa que la categoría (si se indica y no es 0) sea del negocio; si no, responde 400.
func (a *API) categoryAllowed(w http.ResponseWriter, r *http.Request, bizID int64, id *int64) bool {
	if id == nil || *id == 0 {
		return true
	}
	ok, err := a.categoryInBiz(r.Context(), a.DB, bizID, *id)
	if err != nil {
		serverErr(w)
		return false
	}
	if !ok {
		writeErr(w, http.StatusBadRequest, "esa categoría no existe")
		return false
	}
	return true
}

// setServiceStaff reemplaza la lista de profesionales que hacen el servicio.
func (a *API) setServiceStaff(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		UserIDs []int64 `json:"user_ids"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	ids := uniqueIDs(req.UserIDs)

	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM services WHERE id = $1 AND business_id = $2)`, id, p.BusinessID).Scan(&exists); err != nil {
		serverErr(w)
		return
	}
	if !exists {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	var inBiz int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE business_id = $1 AND id = ANY($2)`, p.BusinessID, ids).Scan(&inBiz); err != nil {
		serverErr(w)
		return
	}
	if inBiz != len(ids) {
		writeErr(w, http.StatusBadRequest, "hay personas que no pertenecen a tu negocio")
		return
	}
	if _, err := tx.Exec(ctx, `DELETE FROM service_staff WHERE service_id = $1`, id); err != nil {
		serverErr(w)
		return
	}
	if _, err := tx.Exec(ctx, `INSERT INTO service_staff (service_id, user_id) SELECT $1, unnest($2::bigint[])`, id, ids); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func uniqueIDs(in []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(in))
	for _, id := range in {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
