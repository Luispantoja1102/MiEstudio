package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

var errInsufficientStock = errors.New("existencia insuficiente")

// applyStock cambia la existencia de un producto y deja el movimiento en el libro.
// Bloquea la fila del producto, así dos ventas simultáneas no pueden vender la misma pieza.
func applyStock(ctx context.Context, tx pgx.Tx, bizID, productID int64, delta int, reason string, saleID *int64, note string, userID int64) (int, error) {
	var stock int
	err := tx.QueryRow(ctx, `SELECT stock FROM products WHERE id = $1 AND business_id = $2 FOR UPDATE`, productID, bizID).Scan(&stock)
	if err != nil {
		return 0, err
	}
	if stock+delta < 0 {
		return stock, errInsufficientStock
	}
	balance := stock + delta
	if _, err := tx.Exec(ctx, `UPDATE products SET stock = $2 WHERE id = $1`, productID, balance); err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO stock_movements (business_id, product_id, delta, reason, sale_id, note, balance, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, 0))`, bizID, productID, delta, reason, saleID, note, balance, userID)
	return balance, err
}

// adjustStock registra una compra, una pérdida o un ajuste de existencia.
func (a *API) adjustStock(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Delta  int    `json:"delta"`
		Reason string `json:"reason"`
		Note   string `json:"note"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	switch {
	case req.Delta == 0 || req.Delta > 1_000_000 || req.Delta < -1_000_000:
		writeErr(w, http.StatusBadRequest, "la cantidad debe ser distinta de cero")
		return
	case req.Reason != "purchase" && req.Reason != "adjustment" && req.Reason != "loss":
		writeErr(w, http.StatusBadRequest, "motivo inválido (purchase, adjustment o loss)")
		return
	case req.Reason == "purchase" && req.Delta < 0, req.Reason == "loss" && req.Delta > 0:
		writeErr(w, http.StatusBadRequest, "el signo de la cantidad no corresponde al motivo")
		return
	case len(req.Note) > 300:
		writeErr(w, http.StatusBadRequest, "nota demasiado larga")
		return
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	balance, err := applyStock(ctx, tx, p.BusinessID, id, req.Delta, req.Reason, nil, req.Note, p.UserID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	case errors.Is(err, errInsufficientStock):
		writeErr(w, http.StatusConflict, "no hay suficientes piezas para esa salida")
		return
	case err != nil:
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stock": balance})
}

type stockMovement struct {
	ID        int64     `json:"id"`
	Delta     int       `json:"delta"`
	Reason    string    `json:"reason"`
	Balance   int       `json:"balance"`
	Note      string    `json:"note"`
	SaleID    *int64    `json:"sale_id"`
	By        string    `json:"by"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *API) listStockMovements(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(),
		`SELECT m.id, m.delta, m.reason, m.balance, m.note, m.sale_id, COALESCE(u.name, ''), m.created_at
		 FROM stock_movements m LEFT JOIN users u ON u.id = m.created_by
		 WHERE m.product_id = $1 AND m.business_id = $2 ORDER BY m.created_at DESC, m.id DESC LIMIT 100`, id, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []stockMovement{}
	for rows.Next() {
		var m stockMovement
		if err := rows.Scan(&m.ID, &m.Delta, &m.Reason, &m.Balance, &m.Note, &m.SaleID, &m.By, &m.CreatedAt); err != nil {
			serverErr(w)
			return
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
}
