package api

import (
	"context"
	"testing"
)

func (d *depFx) openCashAdmin(t *testing.T) {
	t.Helper()
	if r := d.a.do(t, "POST", "/api/cash/open", d.admin, map[string]any{"branch_id": d.branch, "opening_cents": 10000}); r.Status >= 300 {
		t.Fatalf("abrir caja: %d %v", r.Status, r.Body)
	}
}

func (d *depFx) apptID(t *testing.T, token string) int64 {
	var id int64
	d.a.DB.QueryRow(context.Background(), `SELECT id FROM appointments WHERE public_token = $1`, token).Scan(&id)
	return id
}

// paidDeposit reserva con anticipo, lo paga y devuelve (token, id de la cita).
func (d *depFx) paidDeposit(t *testing.T, start string, mod map[string]any) (string, int64, string) {
	t.Helper()
	token, _ := d.bookDeposit(t, start, mod)
	pid := d.mp.pay(d.ref(t, token), 45, "approved")
	d.view(t, token)
	return token, d.apptID(t, token), pid
}

func (d *depFx) sale(t *testing.T, appt int64, payments []any, extra map[string]any) resp {
	body := map[string]any{"branch_id": d.branch, "appointment_id": appt,
		"lines": []any{map[string]any{"service_id": d.svc, "qty": 1, "staff_id": d.marco}}, "payments": payments}
	for k, v := range extra {
		body[k] = v
	}
	return d.a.do(t, "POST", "/api/sales", d.admin, body)
}

func dep(amount int) map[string]any {
	return map[string]any{"method": "deposit", "amount_cents": amount}
}

func TestSaleAppliesDepositAndCountsOnlyCashInDrawer(t *testing.T) {
	d := newDeposit(t)
	d.openCashAdmin(t)
	token, appt, _ := d.paidDeposit(t, at(monday, "10:00"), nil)
	r := d.sale(t, appt, []any{dep(4500), cash(10500)}, nil)
	if r.Status != 201 {
		t.Fatalf("cobrar: %d %v", r.Status, r.Body)
	}
	if d.depStatus(t, token) != "applied" || d.apptStatus(t, token) != "completed" {
		t.Fatalf("estado: %s / %s", d.depStatus(t, token), d.apptStatus(t, token))
	}
	var sid *int64
	d.a.DB.QueryRow(context.Background(), `SELECT sale_id FROM deposits`).Scan(&sid)
	if sid == nil || *sid != id(r) {
		t.Fatal("el anticipo debe quedar ligado a la venta")
	}
	// en la caja solo entra el efectivo
	cs := d.a.do(t, "GET", path("/api/cash/current?branch_id=", d.branch), d.admin, nil)
	if sess, _ := cs.Body["session"].(map[string]any); cs.Status != 200 || sess["expected_cents"] != float64(10000+10500) {
		t.Fatalf("caja: %v", cs.Body)
	}
	// el reporte lo muestra como forma de pago propia
	rep := d.a.do(t, "GET", "/api/reports/sales?from=2000-01-01&to=2100-01-01&branch_id="+itoa(d.branch), d.admin, nil)
	found := false
	for _, m := range rep.Body["by_method"].([]any) {
		if m.(map[string]any)["method"] == "deposit" && m.(map[string]any)["amount_cents"] == float64(4500) {
			found = true
		}
	}
	if !found {
		t.Fatalf("reporte por método: %v", rep.Body["by_method"])
	}
}


func TestSaleDepositValidation(t *testing.T) {
	d := newDeposit(t)
	d.openCashAdmin(t)
	_, appt, _ := d.paidDeposit(t, at(monday, "10:00"), nil)

	if r := d.sale(t, appt, []any{dep(4000), cash(11000)}, nil); r.Status != 400 {
		t.Errorf("monto distinto: %d", r.Status)
	}
	if r := d.sale(t, appt, []any{dep(4500), dep(4500), cash(6000)}, nil); r.Status != 400 {
		t.Errorf("dos anticipos: %d", r.Status)
	}
	if r := d.sale(t, appt, []any{cash(15000)}, nil); r.Status != 409 {
		t.Errorf("anticipo sin aplicar: %d %v", r.Status, r.Body)
	}
	// sin cita no hay anticipo que aplicar
	if r := d.a.do(t, "POST", "/api/sales", d.admin, map[string]any{"branch_id": d.branch,
		"lines": []any{map[string]any{"service_id": d.svc, "qty": 1, "staff_id": d.marco}}, "payments": []any{dep(4500), cash(10500)}}); r.Status != 400 {
		t.Errorf("sin cita: %d", r.Status)
	}
	// descuento tal que la cuenta es menor que el anticipo
	if r := d.sale(t, appt, []any{dep(4500)}, map[string]any{"discount_cents": 14000}); r.Status == 201 {
		t.Errorf("anticipo mayor que la cuenta debió rechazarse")
	}
	// cita interna sin anticipo
	other := d.fixture.mustBook(t, d.marco, at(monday, "12:00"))
	if r := d.sale(t, other, []any{dep(4500), cash(10500)}, nil); r.Status != 409 {
		t.Errorf("cita sin anticipo: %d", r.Status)
	}
	// nada de esto aplicó el anticipo
	var st string
	d.a.DB.QueryRow(context.Background(), `SELECT status FROM deposits`).Scan(&st)
	if st != "paid" {
		t.Fatalf("el anticipo no debía cambiar: %s", st)
	}
}

func TestSaleCannotUseUnpaidDeposit(t *testing.T) {
	d := newDeposit(t)
	d.openCashAdmin(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil) // sin pagar
	appt := d.apptID(t, token)
	if r := d.sale(t, appt, []any{dep(4500), cash(10500)}, nil); r.Status != 409 {
		t.Fatalf("la cita espera su anticipo: %d %v", r.Status, r.Body)
	}
}

func TestVoidSaleRefundsAppliedDeposit(t *testing.T) {
	d := newDeposit(t)
	d.openCashAdmin(t)
	token, appt, pid := d.paidDeposit(t, at(monday, "10:00"), nil)
	r := d.sale(t, appt, []any{dep(4500), cash(10500)}, nil)
	if r.Status != 201 {
		t.Fatal(r.Status, r.Body)
	}
	sid := id(r)
	d.mp.payRefundFail = "no se pudo"
	if v := d.a.do(t, "POST", path("/api/sales/", sid, "/void"), d.admin, map[string]any{"reason": "error de captura"}); v.Status != 502 {
		t.Fatalf("falla de devolución: %d %v", v.Status, v.Body)
	}
	var st string
	d.a.DB.QueryRow(context.Background(), `SELECT status FROM sales WHERE id = $1`, sid).Scan(&st)
	if st != "paid" || d.depStatus(t, token) != "applied" {
		t.Fatalf("no debe cancelarse la venta si no se devolvió el anticipo: %s / %s", st, d.depStatus(t, token))
	}
	d.mp.payRefundFail = ""
	if v := d.a.do(t, "POST", path("/api/sales/", sid, "/void"), d.admin, map[string]any{"reason": "error de captura"}); v.Status != 200 && v.Status != 204 {
		t.Fatalf("cancelar: %d %v", v.Status, v.Body)
	}
	if d.depStatus(t, token) != "refunded" || d.mp.refunds(pid) != 1 {
		t.Fatalf("anticipo: %s refunds=%d", d.depStatus(t, token), d.mp.refunds(pid))
	}
	// el anticipo devuelto no se puede volver a aplicar
	if r := d.sale(t, appt, []any{dep(4500), cash(10500)}, nil); r.Status != 409 {
		t.Fatalf("reaplicar: %d", r.Status)
	}
}

func TestAgendaExposesDepositAndSource(t *testing.T) {
	d := newDeposit(t)
	d.paidDeposit(t, at(monday, "10:00"), nil)
	d.fixture.mustBook(t, d.marco, at(monday, "12:00"))
	r := d.a.do(t, "GET", path("/api/agenda?branch_id=", d.branch, "&date=", monday), d.admin, nil)
	if r.Status != 200 {
		t.Fatal(r.Status)
	}
	seen := map[string]map[string]any{}
	for _, x := range r.Body["appointments"].([]any) {
		m := x.(map[string]any)
		seen[m["source"].(string)] = m
	}
	if seen["online"]["deposit_status"] != "paid" || seen["online"]["deposit_cents"] != float64(4500) {
		t.Fatalf("cita en línea: %v", seen["online"])
	}
	if _, has := seen["staff"]["deposit_cents"]; has {
		t.Fatal("la cita interna no debe mostrar anticipo")
	}
}
