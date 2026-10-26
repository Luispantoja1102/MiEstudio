package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"miestudio/backend-go/internal/auth"
)

type serviceCategory struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"` // servicios con esta categoría
}

func validCategoryName(s string) bool { n := utf8.RuneCountInString(s); return n >= 1 && n <= 40 }

// writeCategoryTaken responde 409 si el error es un nombre repetido; devuelve si lo era.
func writeCategoryTaken(w http.ResponseWriter, err error) bool {
	if code, name := pgConstraint(err); code == "23505" && name == "service_categories_name_key" {
		writeErr(w, http.StatusConflict, "ya tienes una categoría con ese nombre")
		return true
	}
	return false
}

// categoryInBiz dice si la categoría existe en el negocio.
func (a *API) categoryInBiz(ctx context.Context, q queryer, bizID, id int64) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM service_categories WHERE id = $1 AND business_id = $2)`, id, bizID).Scan(&ok)
	return ok, err
}

func (a *API) listServiceCategories(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := a.DB.Query(r.Context(),
		`SELECT c.id, c.name, count(s.id) FROM service_categories c LEFT JOIN services s ON s.category_id = c.id
		 WHERE c.business_id = $1 GROUP BY c.id ORDER BY lower(c.name)`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []serviceCategory{}
	for rows.Next() {
		var c serviceCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Count); err != nil {
			serverErr(w)
			return
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createServiceCategory(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !validCategoryName(req.Name) {
		writeErr(w, http.StatusBadRequest, "el nombre de la categoría debe tener entre 1 y 40 caracteres")
		return
	}
	var id int64
	if err := a.DB.QueryRow(r.Context(), `INSERT INTO service_categories (business_id, name) VALUES ($1, $2) RETURNING id`, p.BusinessID, req.Name).Scan(&id); err != nil {
		if writeCategoryTaken(w, err) {
			return
		}
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) renameServiceCategory(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !validCategoryName(req.Name) {
		writeErr(w, http.StatusBadRequest, "el nombre de la categoría debe tener entre 1 y 40 caracteres")
		return
	}
	tag, err := a.DB.Exec(r.Context(), `UPDATE service_categories SET name = $3 WHERE id = $1 AND business_id = $2`, id, p.BusinessID, req.Name)
	if err != nil {
		if writeCategoryTaken(w, err) {
			return
		}
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteServiceCategory borra la categoría. Con ?move_to=ID sus servicios pasan a esa otra categoría;
// sin él quedan «sin categoría».
func (a *API) deleteServiceCategory(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var moveTo int64
	if v := r.URL.Query().Get("move_to"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 || n == id {
			writeErr(w, http.StatusBadRequest, "categoría de destino inválida")
			return
		}
		moveTo = n
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	if exists, err := a.categoryInBiz(ctx, tx, p.BusinessID, id); err != nil {
		serverErr(w)
		return
	} else if !exists {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if moveTo > 0 {
		if exists, err := a.categoryInBiz(ctx, tx, p.BusinessID, moveTo); err != nil {
			serverErr(w)
			return
		} else if !exists {
			writeErr(w, http.StatusBadRequest, "categoría de destino inválida")
			return
		}
		if _, err := tx.Exec(ctx, `UPDATE services SET category_id = $2 WHERE category_id = $1 AND business_id = $3`, id, moveTo, p.BusinessID); err != nil {
			serverErr(w)
			return
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM service_categories WHERE id = $1 AND business_id = $2`, id, p.BusinessID); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
