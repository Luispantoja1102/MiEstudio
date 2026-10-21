package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// claimMPCharge reserva un cobro aprobado de la terminal para una venta. Cada cobro sirve una sola vez.
func claimMPCharge(ctx context.Context, tx pgx.Tx, bizID, branchID int64, chargeID string, amountCents int, saleID int64) error {
	var status string
	var amount int
	var branch int64
	var used *int64
	err := tx.QueryRow(ctx,
		`SELECT status, amount_cents, branch_id, sale_id FROM mp_charges WHERE id = $1 AND business_id = $2 FOR UPDATE`,
		chargeID, bizID).Scan(&status, &amount, &branch, &used)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(http.StatusBadRequest, "no encontramos ese cobro de la terminal")
	}
	if err != nil {
		return err
	}
	switch {
	case status != "approved":
		return fail(http.StatusConflict, "el pago con la terminal todavía no está aprobado")
	case used != nil:
		return fail(http.StatusConflict, "ese pago de la terminal ya se usó en otra venta")
	case branch != branchID:
		return fail(http.StatusBadRequest, "ese cobro es de otra sucursal")
	case amount != amountCents:
		return fail(http.StatusBadRequest, "el monto del pago no coincide con lo aprobado en la terminal ($"+cents(amount)+")")
	}
	_, err = tx.Exec(ctx, `UPDATE mp_charges SET sale_id = $2 WHERE id = $1`, chargeID, saleID)
	return err
}
