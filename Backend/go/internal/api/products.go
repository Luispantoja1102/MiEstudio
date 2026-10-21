package api

import (
	"net/http"
	"strings"

	"miestudio/backend-go/internal/auth"
)

type product struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	CostCents  *int   `json:"cost_cents,omitempty"` // solo lo ve el administrador
	Stock      int    `json:"stock"`
	Active     bool   `json:"active"`
}

func validProduct(name string, price, stock int) string {
	switch {
	case !validName(name):
		return "el nombre es obligatorio"
	case price < 0 || price > maxPriceCents:
		return "precio inválido"
	case stock < 0 || stock > 1_000_000:
		return "existencia inválida"
	}
	return ""
}

func (a *API) listProducts(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := a.DB.Query(r.Context(),
		`SELECT id, name, price_cents, cost_cents, stock, active FROM products WHERE business_id = $1 ORDER BY name`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []product{}
	for rows.Next() {
		var x product
		var cost int
		if err := rows.Scan(&x.ID, &x.Name, &x.PriceCents, &cost, &x.Stock, &x.Active); err != nil {
			serverErr(w)
			return
		}
		if p.Role == "admin" {
			x.CostCents = &cost
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createProduct(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Name       string `json:"name"`
		PriceCents int    `json:"price_cents"`
		CostCents  int    `json:"cost_cents"`
		Stock      int    `json:"stock"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if msg := validProduct(req.Name, req.PriceCents, req.Stock); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if req.CostCents < 0 || req.CostCents > maxPriceCents {
		writeErr(w, http.StatusBadRequest, "costo inválido")
		return
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	var id int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO products (business_id, name, price_cents, cost_cents, stock) VALUES ($1, $2, $3, $4, 0) RETURNING id`,
		p.BusinessID, req.Name, req.PriceCents, req.CostCents).Scan(&id); err != nil {
		serverErr(w)
		return
	}
	if req.Stock > 0 { // la existencia inicial entra por el libro de inventario
		if _, err := applyStock(ctx, tx, p.BusinessID, id, req.Stock, "initial", nil, "", p.UserID); err != nil {
			serverErr(w)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// updateProduct cambia nombre, precio, costo o estado. La existencia solo cambia por el libro de inventario.
func (a *API) updateProduct(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name       *string `json:"name"`
		PriceCents *int    `json:"price_cents"`
		CostCents  *int    `json:"cost_cents"`
		Active     *bool   `json:"active"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if req.Name != nil {
		t := strings.TrimSpace(*req.Name)
		req.Name = &t
	}
	name, price := "x", 0
	if req.Name != nil {
		name = *req.Name
	}
	if req.PriceCents != nil {
		price = *req.PriceCents
	}
	if msg := validProduct(name, price, 0); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if req.CostCents != nil && (*req.CostCents < 0 || *req.CostCents > maxPriceCents) {
		writeErr(w, http.StatusBadRequest, "costo inválido")
		return
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE products SET name = COALESCE($3, name), price_cents = COALESCE($4, price_cents),
		   cost_cents = COALESCE($5, cost_cents), active = COALESCE($6, active)
		 WHERE id = $1 AND business_id = $2`,
		id, p.BusinessID, req.Name, req.PriceCents, req.CostCents, req.Active)
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
