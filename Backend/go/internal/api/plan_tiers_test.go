package api

import (
	"context"
	"testing"
)

func setPlan(a *API, plan string) {
	a.DB.Exec(context.Background(), `UPDATE businesses SET plan = $1`, plan)
}

func publicMethods(t *testing.T, d *depFx) []any {
	t.Helper()
	m, _ := d.a.do(t, "GET", "/api/public/"+d.slug, "", nil).Body["deposit_methods"].([]any)
	return m
}

// Básico cobra anticipo solo por transferencia; Mercado Pago es de Crecimiento en adelante.
func TestBasicoDepositsAreTransferOnly(t *testing.T) {
	d := newDeposit(t) // Mercado Pago conectado y anticipo del 30 %
	body := func(mod map[string]any) map[string]any {
		b := map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24,
			"show_prices": true, "deposit_percent": 30}
		for k, v := range mod {
			b[k] = v
		}
		return b
	}
	transfer := map[string]any{"transfer_enabled": true, "transfer_bank": "BBVA", "transfer_holder": "Barbería Don Luis",
		"transfer_clabe": "012345678901234568", "transfer_whatsapp": "664 123 4567", "transfer_hold_hours": 12}

	// Crecimiento: los dos métodos
	if m := publicMethods(t, d); len(m) != 1 || m[0] != "mp" {
		t.Fatalf("Crecimiento con solo Mercado Pago: %v", m)
	}
	if r := d.a.do(t, "PUT", "/api/booking/settings", d.admin, body(transfer)); r.Status != 204 {
		t.Fatalf("Crecimiento con transferencia: %d %v", r.Status, r.Body)
	}
	if m := publicMethods(t, d); len(m) != 2 {
		t.Fatalf("Crecimiento debe ofrecer mp y transfer: %v", m)
	}

	// Básico: aunque Mercado Pago siga conectado, solo transferencia
	setPlan(d.a, "basico")
	if m := publicMethods(t, d); len(m) != 1 || m[0] != "transfer" {
		t.Fatalf("Básico debe ofrecer solo transferencia: %v", m)
	}
	if r := d.book(t, at(monday, "10:00"), map[string]any{"payment_method": "mp"}); r.Status != 400 {
		t.Fatalf("pagar con Mercado Pago en Básico: %d %v", r.Status, r.Body)
	}
	if r := d.book(t, at(monday, "10:00"), nil); r.Status != 201 || r.Body["pay_method"] != "transfer" || r.Body["pay_url"] != nil {
		t.Fatalf("reserva por transferencia en Básico: %d %v", r.Status, r.Body)
	}
	// guardar ajustes con anticipo y transferencias activas es válido
	if r := d.a.do(t, "PUT", "/api/booking/settings", d.admin, body(transfer)); r.Status != 204 {
		t.Fatalf("Básico con transferencia: %d %v", r.Status, r.Body)
	}
	// con Mercado Pago conectado pero sin transferencias, Básico no puede cobrar anticipo
	r := d.a.do(t, "PUT", "/api/booking/settings", d.admin, body(map[string]any{"transfer_enabled": false}))
	if r.Status != 409 || r.Body["code"] != "transfer_required" {
		t.Fatalf("Básico sin transferencias: %d %v", r.Status, r.Body)
	}
	// y la cita creada desde la agenda con anticipo sigue siendo por transferencia
	setPlan(d.a, "crecimiento")
	if m := publicMethods(t, d); len(m) != 2 {
		t.Fatalf("al volver a Crecimiento regresan los dos métodos: %v", m)
	}
}

// Individual: una persona y sin anticipos.
func TestIndividualPlan(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "Barbería", "a@x.com")
	setPlan(a, "individual")
	mk := func(email string) resp {
		return a.do(t, "POST", "/api/team", admin, map[string]any{"name": "P", "email": email, "password": "supersecreta1", "role": "staff", "bookable": true})
	}
	if r := mk("p1@x.com"); r.Status != 201 {
		t.Fatalf("primera persona: %d %v", r.Status, r.Body)
	}
	if r := mk("p2@x.com"); r.Status != 403 || r.Body["code"] != "plan_limit" {
		t.Fatalf("segunda persona en Individual (máx. 1): %d %v", r.Status, r.Body)
	}
	if r := a.do(t, "POST", "/api/branches", admin, map[string]any{"name": "Sucursal 2"}); r.Status != 403 || r.Body["code"] != "plan_limit" {
		t.Fatalf("segunda sucursal: %d %v", r.Status, r.Body)
	}
	// sin anticipos de ningún tipo
	r := a.do(t, "PUT", "/api/booking/settings", admin, map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2,
		"reminder_hours": 24, "show_prices": true, "deposit_percent": 30, "transfer_enabled": true, "transfer_bank": "BBVA", "transfer_holder": "Luis",
		"transfer_clabe": "012345678901234568", "transfer_whatsapp": "664 123 4567", "transfer_hold_hours": 12})
	if r.Status != 403 || r.Body["code"] != "plan_feature" {
		t.Fatalf("anticipo en Individual: %d %v", r.Status, r.Body)
	}
	// ni caja ni ventas
	if r := a.do(t, "GET", "/api/cash/current", admin, nil); r.Status != 403 {
		t.Fatalf("caja en Individual: %d", r.Status)
	}
	// el catálogo público lo incluye con su precio
	var list []map[string]any
	jsonUnmarshal(a.do(t, "GET", "/api/plans", "", nil).Raw, &list)
	if len(list) != 4 || list[0]["id"] != "individual" || list[0]["price_month"] != float64(150) || list[1]["price_month"] != float64(350) ||
		list[2]["price_month"] != float64(700) || list[3]["price_month"] != float64(1200) {
		t.Fatalf("catálogo: %v", list)
	}
	if list[1]["deposits"] != true || list[1]["deposits_mp"] != false || list[2]["deposits_mp"] != true {
		t.Fatalf("anticipos por plan: %v", list)
	}
}
