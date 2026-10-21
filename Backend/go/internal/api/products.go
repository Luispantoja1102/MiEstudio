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
		`SELECT id, name, price_cents, stock, active FROM products WHERE business_id = $1 ORDER BY name`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []product{}
	for rows.Next() {
		var x product
		if err := rows.Scan(&x.ID, &x.Name, &x.PriceCents, &x.Stock, &x.Active); err != nil {
			serverErr(w)
			return
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
	var id int64
	if err := a.DB.QueryRow(r.Context(),
		`INSERT INTO products (business_id, name, price_cents, stock) VALUES ($1,$2,$3,$4) RETURNING id`,
		p.BusinessID, req.Name, req.PriceCents, req.Stock).Scan(&id); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) updateProduct(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name       *string `json:"name"`
		PriceCents *int    `json:"price_cents"`
		Stock      *int    `json:"stock"`
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
	name, price, stock := "x", 0, 0
	if req.Name != nil {
		name = *req.Name
	}
	if req.PriceCents != nil {
		price = *req.PriceCents
	}
	if req.Stock != nil {
		stock = *req.Stock
	}
	if msg := validProduct(name, price, stock); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE products SET name = COALESCE($3, name), price_cents = COALESCE($4, price_cents),
		   stock = COALESCE($5, stock), active = COALESCE($6, active)
		 WHERE id = $1 AND business_id = $2`,
		id, p.BusinessID, req.Name, req.PriceCents, req.Stock, req.Active)
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
