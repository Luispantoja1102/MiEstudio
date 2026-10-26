package api

import (
	"fmt"
	"testing"
)

func TestCashReportForPrinting(t *testing.T) {
	p := newPOS(t)
	a := p.a
	p.openCash(t, p.admin, 10000)
	paid := p.mustSale(t, p.cashier, map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}})
	voided := p.mustSale(t, p.cashier, map[string]any{"lines": []any{svcLine(p.barba, p.marco)}, "payments": []any{cash(8000)}})
	p.must(t, a.do(t, "POST", fmt.Sprintf("/api/sales/%d/void", voided.ID), p.admin, map[string]any{"reason": "se equivocó"}))
	p.must(t, a.do(t, "POST", "/api/cash/movements", p.admin, map[string]any{"branch_id": p.branch, "kind": "out", "amount_cents": 2000, "concept": "Toallas"}))

	// corte parcial con la caja abierta: lo esperado ya cuenta lo vendido y el retiro
	cur := p.cashSummary(t)
	r := a.do(t, "GET", fmt.Sprintf("/api/cash/sessions/%d", cur.ID), p.cashier, nil)
	if r.Status != 200 || r.Body["closed_at"] != nil || r.Body["expected_cents"] != float64(10000+15000-2000) {
		t.Fatalf("corte parcial: %d %v", r.Status, r.Body)
	}

	closed := a.do(t, "POST", "/api/cash/close", p.admin, map[string]any{"branch_id": p.branch, "counted_cents": 22500, "note": "faltan 5 pesos"})
	p.must(t, closed)
	r = a.do(t, "GET", fmt.Sprintf("/api/cash/sessions/%d", cur.ID), p.cashier, nil)
	b := r.Body
	if b["business_name"] == "" || b["branch_name"] == "" || b["diff_cents"] != float64(-500) || b["counted_cents"] != float64(22500) ||
		b["voided_count"] != float64(1) || b["voided_cents"] != float64(8000) || b["sales"] != float64(1) {
		t.Fatalf("corte: %v", b)
	}
	tickets, _ := b["tickets"].([]any)
	if len(tickets) != 2 {
		t.Fatalf("tickets del turno: %v", b["tickets"])
	}
	first := tickets[0].(map[string]any)
	if first["folio"] != float64(paid.Folio) || first["status"] != "paid" {
		t.Fatalf("primer ticket: %v", first)
	}
	if moves, _ := b["movements"].([]any); len(moves) != 1 {
		t.Fatalf("movimientos: %v", b["movements"])
	}

	// otro negocio no puede verlo; sin sesión tampoco
	other := a.mustRegister(t, "Otro", "otro@x.com")
	if r := a.do(t, "GET", fmt.Sprintf("/api/cash/sessions/%d", cur.ID), other, nil); r.Status != 404 {
		t.Fatalf("corte ajeno: %d", r.Status)
	}
	if r := a.do(t, "GET", fmt.Sprintf("/api/cash/sessions/%d", cur.ID), "", nil); r.Status != 401 {
		t.Fatalf("sin sesión: %d", r.Status)
	}
	if r := a.do(t, "GET", "/api/cash/sessions/abc", p.admin, nil); r.Status != 400 {
		t.Fatalf("id inválido: %d", r.Status)
	}
}
