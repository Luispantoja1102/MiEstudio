package api

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func (p *posFx) stampsOf(t *testing.T, client int64) map[string]any {
	t.Helper()
	r := p.a.do(t, "GET", fmt.Sprintf("/api/clients/%d/loyalty", client), p.cashier, nil)
	if r.Status != 200 {
		t.Fatalf("tarjeta: %d %v", r.Status, r.Body)
	}
	return r.Body
}

func TestLoyaltyStampsRedeemAndCard(t *testing.T) {
	p := newPOS(t)
	a := p.a
	day := func(d string) {
		a.Now = func() time.Time { t0, _ := time.Parse(time.RFC3339, d+"T18:00:00Z"); return t0 }
	}
	day("2026-11-02")
	p.openCash(t, p.admin, 0)
	maria := id(a.do(t, "POST", "/api/clients", p.admin, map[string]any{"name": "María Pérez López", "phone": "6641112233"}))
	sell := func(client any, lines ...any) *saleOut {
		// cada línea cuesta lo que su servicio o producto: Corte $150, Barba $80, Cera $120
		total := 0
		for _, l := range lines {
			m := l.(map[string]any)
			switch {
			case m["product_id"] != nil:
				total += 12000 * m["qty"].(int)
			case m["service_id"] == p.barba:
				total += 8000
			default:
				total += 15000
			}
		}
		body := map[string]any{"lines": lines, "payments": []any{cash(total)}}
		if client != nil {
			body["client_id"] = client
		}
		return p.mustSale(t, p.cashier, body)
	}

	// apagada de origen: no da sellos, pero el cliente se consulta igual
	if r := a.do(t, "GET", "/api/loyalty", p.cashier, nil); r.Body["enabled"] != false || r.Body["visits_required"] != float64(10) {
		t.Fatalf("estado inicial: %v", r.Body)
	}
	sell(maria, svcLine(p.svc, p.marco))
	if p.stampsOf(t, maria)["stamps"] != float64(0) {
		t.Fatal("apagada no debe dar sellos")
	}

	// configuración: solo administración la cambia y se valida
	if r := a.do(t, "PUT", "/api/loyalty", p.cashier, map[string]any{"enabled": true, "visits_required": 2, "reward": "Corte gratis"}); r.Status != 403 {
		t.Fatalf("caja configura: %d", r.Status)
	}
	for i, bad := range []map[string]any{
		{"enabled": true, "visits_required": 1, "reward": "x"},
		{"enabled": true, "visits_required": 51, "reward": "x"},
		{"enabled": true, "visits_required": 5, "reward": "  "},
		{"enabled": true, "visits_required": 5, "reward": strings.Repeat("a", 81)},
	} {
		if r := a.do(t, "PUT", "/api/loyalty", p.admin, bad); r.Status != 400 {
			t.Errorf("inválido %d: %d", i, r.Status)
		}
	}
	p.must(t, a.do(t, "PUT", "/api/loyalty", p.admin, map[string]any{"enabled": true, "visits_required": 2, "reward": " Corte gratis "}))

	// un sello por visita con servicio: no cuentan el mostrador, los productos solos ni una segunda venta el mismo día
	sell(nil, svcLine(p.svc, p.marco))
	sell(maria, prdLine(p.cera, 1))
	s1 := sell(maria, svcLine(p.svc, p.marco))
	sell(maria, svcLine(p.barba, p.marco))
	if got := p.stampsOf(t, maria); got["stamps"] != float64(1) || got["ready"] != false || got["reward"] != "Corte gratis" {
		t.Fatalf("un sello por día: %v", got)
	}
	// cancelar la venta quita su sello
	p.must(t, a.do(t, "POST", fmt.Sprintf("/api/sales/%d/void", s1.ID), p.admin, map[string]any{"reason": "error"}))
	if got := p.stampsOf(t, maria)["stamps"]; got != float64(0) { // la otra venta de ese día no tenía sello: no lo repone
		t.Fatalf("tras cancelar debía quedar sin sellos: %v", got)
	}

	// canjear sin completar no se puede
	if r := a.do(t, "POST", fmt.Sprintf("/api/clients/%d/loyalty/redeem", maria), p.cashier, nil); r.Status != 409 {
		t.Fatalf("canje sin sellos: %d", r.Status)
	}
	day("2026-11-03")
	sell(maria, svcLine(p.svc, p.marco))
	day("2026-11-04")
	sell(maria, svcLine(p.svc, p.marco))
	card := p.stampsOf(t, maria)
	if card["ready"] != true || card["stamps"].(float64) < 2 {
		t.Fatalf("debía estar completa: %v", card)
	}
	before := card["stamps"].(float64)

	// enlace de la tarjeta virtual
	if r := a.do(t, "GET", "/api/public-card/"+strings.Repeat("a", 32), "", nil); r.Status != 404 {
		t.Fatalf("enlace inexistente: %d", r.Status)
	}
	r := a.do(t, "POST", fmt.Sprintf("/api/clients/%d/loyalty/link", maria), p.cashier, map[string]any{"action": "create"})
	path, _ := r.Body["path"].(string)
	if r.Status != 200 || !strings.HasPrefix(path, "/tarjeta/") {
		t.Fatalf("crear enlace: %d %v", r.Status, r.Body)
	}
	apiPath := "/api/public-card/" + strings.TrimPrefix(path, "/tarjeta/")
	pub := a.do(t, "GET", apiPath, "", nil)
	if pub.Status != 200 || pub.Body["name"] != "María" || pub.Body["ready"] != true || pub.Body["reward"] != "Corte gratis" || pub.Body["business"] == "" {
		t.Fatalf("tarjeta pública: %d %v", pub.Status, pub.Body)
	}
	if strings.Contains(string(pub.Raw), "Pérez") || strings.Contains(string(pub.Raw), "6641112233") {
		t.Fatalf("la tarjeta pública solo lleva el primer nombre: %s", pub.Raw)
	}
	if again := a.do(t, "POST", fmt.Sprintf("/api/clients/%d/loyalty/link", maria), p.cashier, map[string]any{"action": "create"}); again.Body["path"] != path {
		t.Fatal("crear de nuevo debe conservar el enlace")
	}

	// canje: usa los sellos que pide el premio y deja el resto
	red := a.do(t, "POST", fmt.Sprintf("/api/clients/%d/loyalty/redeem", maria), p.cashier, nil)
	if red.Status != 200 || red.Body["redeemed"] != float64(1) || red.Body["stamps"] != before-2 {
		t.Fatalf("canje: %d %v", red.Status, red.Body)
	}
	if again := a.do(t, "POST", fmt.Sprintf("/api/clients/%d/loyalty/redeem", maria), p.cashier, nil); again.Status != 409 {
		t.Fatalf("segundo canje seguido: %d", again.Status)
	}

	// otro negocio no ve ni canjea; cambiar el enlace corta el anterior; apagar la tarjeta también
	other := a.mustRegister(t, "Otro", "otro@x.com")
	if r := a.do(t, "GET", fmt.Sprintf("/api/clients/%d/loyalty", maria), other, nil); r.Status != 404 {
		t.Fatalf("tarjeta ajena: %d", r.Status)
	}
	if r := a.do(t, "POST", fmt.Sprintf("/api/clients/%d/loyalty/redeem", maria), other, nil); r.Status != 404 {
		t.Fatalf("canje ajeno: %d", r.Status)
	}
	rot := a.do(t, "POST", fmt.Sprintf("/api/clients/%d/loyalty/link", maria), p.admin, map[string]any{"action": "rotate"})
	if a.do(t, "GET", apiPath, "", nil).Status != 404 {
		t.Fatal("el enlace anterior debe dejar de servir")
	}
	newAPI := "/api/public-card/" + strings.TrimPrefix(rot.Body["path"].(string), "/tarjeta/")
	if a.do(t, "GET", newAPI, "", nil).Status != 200 {
		t.Fatal("el enlace nuevo debe servir")
	}
	p.must(t, a.do(t, "PUT", "/api/loyalty", p.admin, map[string]any{"enabled": false, "visits_required": 2, "reward": "Corte gratis"}))
	if a.do(t, "GET", newAPI, "", nil).Status != 404 {
		t.Fatal("con la tarjeta apagada el enlace no sirve")
	}
	p.must(t, a.do(t, "PUT", "/api/loyalty", p.admin, map[string]any{"enabled": true, "visits_required": 2, "reward": "Corte gratis"}))

	// borrar los datos del cliente corta su enlace y borra sus sellos
	p.must(t, a.do(t, "POST", fmt.Sprintf("/api/clients/%d/erase", maria), p.admin, nil))
	if a.do(t, "GET", newAPI, "", nil).Status != 404 {
		t.Fatal("cliente borrado: el enlace no debe servir")
	}
}
