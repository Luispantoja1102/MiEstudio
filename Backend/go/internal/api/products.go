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
	MinStock   int    `json:"min_stock"` // por debajo (o igual) es «poco stock»; 0 = sin aviso
	Barcode    string `json:"barcode"`
	Active     bool   `json:"active"`
}

// validStock revisa la existencia mínima y el código de barras (único por negocio; vacío = sin código).
func validStock(min int, barcode string) string {
	switch {
	case min < 0 || min > 1_000_000:
		return "existencia mínima inválida"
	case len(barcode) > 64 || strings.ContainsAny(barcode, " \t\r\n"):
		return "el código de barras no puede llevar espacios ni pasar de 64 caracteres"
	}
	return ""
}

// writeBarcodeTaken responde 409 si el error es el código repetido; devuelve si lo era.
func writeBarcodeTaken(w http.ResponseWriter, err error) bool {
	if code, name := pgConstraint(err); code == "23505" && name == "products_barcode_key" {
		writeErr(w, http.StatusConflict, "ese código de barras ya lo tiene otro producto")
		return true
	}
	return false
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
		`SELECT id, name, price_cents, cost_cents, stock, min_stock, barcode, active FROM products WHERE business_id = $1 ORDER BY name`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []product{}
	for rows.Next() {
		var x product
		var cost int
		if err := rows.Scan(&x.ID, &x.Name, &x.PriceCents, &cost, &x.Stock, &x.MinStock, &x.Barcode, &x.Active); err != nil {
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
		MinStock   int    `json:"min_stock"`
		Barcode    string `json:"barcode"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Barcode = strings.TrimSpace(req.Barcode)
	if msg := validStock(req.MinStock, req.Barcode); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
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
		`INSERT INTO products (business_id, name, price_cents, cost_cents, stock, min_stock, barcode) VALUES ($1, $2, $3, $4, 0, $5, $6) RETURNING id`,
		p.BusinessID, req.Name, req.PriceCents, req.CostCents, req.MinStock, req.Barcode).Scan(&id); err != nil {
		if writeBarcodeTaken(w, err) {
			return
		}
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
		MinStock   *int    `json:"min_stock"`
		Barcode    *string `json:"barcode"`
		Active     *bool   `json:"active"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if req.Barcode != nil {
		t := strings.TrimSpace(*req.Barcode)
		req.Barcode = &t
	}
	min, code := 0, ""
	if req.MinStock != nil {
		min = *req.MinStock
	}
	if req.Barcode != nil {
		code = *req.Barcode
	}
	if msg := validStock(min, code); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
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
		   cost_cents = COALESCE($5, cost_cents), active = COALESCE($6, active),
		   min_stock = COALESCE($7, min_stock), barcode = COALESCE($8, barcode)
		 WHERE id = $1 AND business_id = $2`,
		id, p.BusinessID, req.Name, req.PriceCents, req.CostCents, req.Active, req.MinStock, req.Barcode)
	if err != nil {
		if writeBarcodeTaken(w, err) {
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
