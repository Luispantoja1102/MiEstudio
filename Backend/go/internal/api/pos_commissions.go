package api

import (
	"context"
	"math"
	"net/http"

	"miestudio/backend-go/internal/auth"
)

// commissionRule: regla por servicio o producto, opcionalmente para una persona.
type commissionRule struct {
	UserID    *int64
	ServiceID *int64
	ProductID *int64
	Percent   float64
}

func eqPtr(a, b *int64) bool { return a != nil && b != nil && *a == *b }

// pickPercent devuelve la comisión de una partida.
// Prioridad: regla persona + artículo > regla del artículo > comisión general de la persona (solo servicios).
// Sin persona no hay comisión.
func pickPercent(rules []commissionRule, staffID, serviceID, productID *int64, staffDefault float64) float64 {
	if staffID == nil {
		return 0
	}
	best, bestRank := 0.0, 0
	for _, r := range rules {
		if !(eqPtr(r.ServiceID, serviceID) || eqPtr(r.ProductID, productID)) {
			continue
		}
		rank := 0
		switch {
		case r.UserID == nil:
			rank = 1
		case *r.UserID == *staffID:
			rank = 2
		}
		if rank > bestRank {
			best, bestRank = r.Percent, rank
		}
	}
	if bestRank > 0 {
		return best
	}
	if serviceID != nil {
		return staffDefault
	}
	return 0
}

// commissionOf calcula la comisión en centavos sobre una base, con redondeo al centavo más cercano.
func commissionOf(baseCents int, percent float64) int {
	bp := int64(math.Round(percent * 100)) // centésimas de punto porcentual
	return int((int64(baseCents)*bp + 5000) / 10000)
}

// allocateDiscount reparte un descuento del ticket entre las partidas, proporcional a su importe.
// La suma de lo repartido es exactamente el descuento (redondeo acumulado).
func allocateDiscount(parts []int, discount int) []int {
	total := 0
	for _, p := range parts {
		total += p
	}
	out := make([]int, len(parts))
	if total == 0 || discount == 0 {
		return out
	}
	cum, prev := 0, 0
	for i, p := range parts {
		cum += p
		cur := int((int64(discount)*int64(cum) + int64(total)/2) / int64(total))
		out[i] = cur - prev
		prev = cur
	}
	return out
}

func (a *API) loadRules(ctx context.Context, q queryer, bizID int64) ([]commissionRule, error) {
	rows, err := q.Query(ctx, `SELECT user_id, service_id, product_id, percent::float8 FROM commission_rules WHERE business_id = $1 AND active`, bizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []commissionRule
	for rows.Next() {
		var r commissionRule
		if err := rows.Scan(&r.UserID, &r.ServiceID, &r.ProductID, &r.Percent); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------- reglas (administrador) ----------

type ruleOut struct {
	ID        int64   `json:"id"`
	UserID    *int64  `json:"user_id"`
	UserName  string  `json:"user_name"`
	ServiceID *int64  `json:"service_id"`
	ProductID *int64  `json:"product_id"`
	ItemName  string  `json:"item_name"`
	Percent   float64 `json:"percent"`
	Active    bool    `json:"active"`
}

func (a *API) listRules(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := a.DB.Query(r.Context(),
		`SELECT r.id, r.user_id, COALESCE(u.name, ''), r.service_id, r.product_id, COALESCE(s.name, pr.name, ''), r.percent::float8, r.active
		 FROM commission_rules r LEFT JOIN users u ON u.id = r.user_id
		 LEFT JOIN services s ON s.id = r.service_id LEFT JOIN products pr ON pr.id = r.product_id
		 WHERE r.business_id = $1 ORDER BY r.id`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []ruleOut{}
	for rows.Next() {
		var x ruleOut
		if err := rows.Scan(&x.ID, &x.UserID, &x.UserName, &x.ServiceID, &x.ProductID, &x.ItemName, &x.Percent, &x.Active); err != nil {
			serverErr(w)
			return
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createRule(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		UserID    *int64  `json:"user_id"`
		ServiceID *int64  `json:"service_id"`
		ProductID *int64  `json:"product_id"`
		Percent   float64 `json:"percent"`
	}
	if !decode(w, r, &req) || (req.ServiceID == nil) == (req.ProductID == nil) || req.Percent < 0 || req.Percent > 100 {
		writeErr(w, http.StatusBadRequest, "indica un servicio o un producto, y un porcentaje de 0 a 100")
		return
	}
	// todo lo referenciado debe ser del negocio
	var okItem, okUser bool
	var err error
	if req.ServiceID != nil {
		err = a.DB.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM services WHERE id = $1 AND business_id = $2)`, *req.ServiceID, p.BusinessID).Scan(&okItem)
	} else {
		err = a.DB.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM products WHERE id = $1 AND business_id = $2)`, *req.ProductID, p.BusinessID).Scan(&okItem)
	}
	if err == nil {
		okUser = true
		if req.UserID != nil {
			err = a.DB.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND business_id = $2)`, *req.UserID, p.BusinessID).Scan(&okUser)
		}
	}
	if err != nil {
		serverErr(w)
		return
	}
	if !okItem || !okUser {
		writeErr(w, http.StatusBadRequest, "el artículo o la persona no pertenecen a tu negocio")
		return
	}
	var id int64
	if err := a.DB.QueryRow(r.Context(),
		`INSERT INTO commission_rules (business_id, user_id, service_id, product_id, percent) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		p.BusinessID, req.UserID, req.ServiceID, req.ProductID, req.Percent).Scan(&id); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) updateRule(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Percent *float64 `json:"percent"`
		Active  *bool    `json:"active"`
	}
	if !decode(w, r, &req) || (req.Percent != nil && (*req.Percent < 0 || *req.Percent > 100)) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE commission_rules SET percent = COALESCE($3, percent), active = COALESCE($4, active) WHERE id = $1 AND business_id = $2`,
		id, p.BusinessID, req.Percent, req.Active)
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
