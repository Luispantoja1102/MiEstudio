package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"testing"
)

type posFx struct {
	*fixture
	barba, cera           int64 // servicio y producto
	cashier, recep, staff string
}

// newPOS arma un negocio con dos profesionales (Marco 30 %, Ana 40 %), dos servicios, un producto
// (5 piezas, costo $50, precio $120) y usuarios de caja, recepción y profesional.
func newPOS(t *testing.T) *posFx {
	t.Helper()
	f := newFixture(t) // svc = Corte $150 (30 min, 10 de preparación)
	a := f.a
	f.must(t, a.do(t, "PATCH", path("/api/team/", f.marco), f.admin, map[string]any{"commission_pct": 30}))
	f.must(t, a.do(t, "PATCH", path("/api/team/", f.ana), f.admin, map[string]any{"commission_pct": 40}))
	p := &posFx{fixture: f}
	p.barba = id(a.do(t, "POST", "/api/services", f.admin, map[string]any{"name": "Barba", "duration_min": 20, "price_cents": 8000}))
	p.cera = id(a.do(t, "POST", "/api/products", f.admin, map[string]any{"name": "Cera", "price_cents": 12000, "cost_cents": 5000, "stock": 5}))
	a.addMember(t, f.admin, "caja@x.com", "cashier")
	a.addMember(t, f.admin, "recep@x.com", "reception")
	p.cashier, p.recep, p.staff = login(t, a, "caja@x.com"), login(t, a, "recep@x.com"), login(t, a, "marco@x.com")
	return p
}

func (f *fixture) must(t *testing.T, r resp) {
	t.Helper()
	if r.Status >= 300 {
		t.Fatalf("petición falló: %d %v", r.Status, r.Body)
	}
}

func (p *posFx) openCash(t *testing.T, token string, opening int) {
	t.Helper()
	p.must(t, p.a.do(t, "POST", "/api/cash/open", token, map[string]any{"branch_id": p.branch, "opening_cents": opening}))
}

func (p *posFx) sale(t *testing.T, token string, body map[string]any) resp {
	t.Helper()
	if _, ok := body["branch_id"]; !ok {
		body["branch_id"] = p.branch
	}
	return p.a.do(t, "POST", "/api/sales", token, body)
}

func (p *posFx) mustSale(t *testing.T, token string, body map[string]any) *saleOut {
	t.Helper()
	r := p.sale(t, token, body)
	if r.Status != http.StatusCreated {
		t.Fatalf("venta: %d %v", r.Status, r.Body)
	}
	var s saleOut
	mustJSON(t, r.Raw, &s)
	return &s
}

func svcLine(svc int64, staff int64) map[string]any {
	return map[string]any{"service_id": svc, "qty": 1, "staff_id": staff}
}
func prdLine(prd int64, qty int) map[string]any { return map[string]any{"product_id": prd, "qty": qty} }
func cash(amount int) map[string]any            { return map[string]any{"method": "cash", "amount_cents": amount} }

func (p *posFx) cashSummary(t *testing.T) *cashSummary {
	t.Helper()
	r := p.a.do(t, "GET", fmt.Sprintf("/api/cash/current?branch_id=%d", p.branch), p.admin, nil)
	var out struct {
		Session *cashSummary `json:"session"`
	}
	mustJSON(t, r.Raw, &out)
	return out.Session
}

func (p *posFx) stock(t *testing.T) int {
	return list[product](t, p.a, "/api/products", p.admin)[0].Stock
}

func TestSaleRequiresOpenCash(t *testing.T) {
	p := newPOS(t)
	body := map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}}
	r := p.sale(t, p.admin, body)
	if r.Status != 409 || r.Body["code"] != "cash_closed" {
		t.Fatalf("sin caja: %d %v", r.Status, r.Body)
	}
	p.openCash(t, p.admin, 0)
	if r := p.sale(t, p.admin, body); r.Status != 201 {
		t.Fatalf("con caja: %d %v", r.Status, r.Body)
	}
}

// Caso con cuentas hechas a mano:
//
//	corte $150 (Marco, 30 %) + barba $80 (Marco, regla 50 %) + 2 ceras $240 (sin persona) = $470
//	descuento del ticket $47 (10 %): se reparte 15, 8 y 24 pesos  → bases $135, $72 y $216
//	comisiones: 30 % de 135 = $40.50  ·  50 % de 72 = $36.00  ·  cera = $0
//	total $423 + propina $30 = $453: efectivo $300 (entrega $500, cambio $200) + tarjeta $153
func TestSaleTotalsCommissionAndChange(t *testing.T) {
	p := newPOS(t)
	a := p.a
	p.must(t, a.do(t, "POST", "/api/commission-rules", p.admin, map[string]any{"service_id": p.barba, "user_id": p.marco, "percent": 50}))
	p.openCash(t, p.admin, 50000)

	s := p.mustSale(t, p.cashier, map[string]any{
		"lines":          []any{svcLine(p.svc, p.marco), svcLine(p.barba, p.marco), prdLine(p.cera, 2)},
		"discount_cents": 4700,
		"tip_cents":      3000,
		"payments": []any{
			map[string]any{"method": "cash", "amount_cents": 30000, "received_cents": 50000},
			map[string]any{"method": "card", "amount_cents": 15300},
		},
	})
	if s.Folio != 1 || s.SubtotalCents != 47000 || s.DiscountCents != 4700 || s.TotalCents != 42300 || s.TipCents != 3000 || s.PaidCents != 45300 || s.ChangeCents != 20000 {
		t.Fatalf("totales: %+v", s)
	}
	want := []struct{ total, base, comm int }{{15000, 13500, 4050}, {8000, 7200, 3600}, {24000, 21600, 0}}
	for i, w := range want {
		it := s.Items[i]
		if it.TotalCents != w.total || it.CommissionCents != w.comm {
			t.Errorf("partida %d (%s): total %d comisión %d, esperaba %d y %d", i, it.Name, it.TotalCents, it.CommissionCents, w.total, w.comm)
		}
	}
	if s.Items[0].CommissionPct != 30 || s.Items[1].CommissionPct != 50 || s.Items[2].CommissionPct != 0 {
		t.Errorf("porcentajes: %v %v %v", s.Items[0].CommissionPct, s.Items[1].CommissionPct, s.Items[2].CommissionPct)
	}
	if s.TipStaffID == nil || *s.TipStaffID != p.marco {
		t.Errorf("la propina debe ir a Marco (único profesional de los servicios): %v", s.TipStaffID)
	}
	// la comisión de lo vendido queda guardada: cambiar la regla después no altera el historial
	a.do(t, "PATCH", path("/api/commission-rules/", int64(1)), p.admin, map[string]any{"percent": 5})
	a.do(t, "PATCH", path("/api/team/", p.marco), p.admin, map[string]any{"commission_pct": 99})
	got := p.a.do(t, "GET", path("/api/sales/", s.ID), p.admin, nil)
	var again saleOut
	mustJSON(t, got.Raw, &again)
	if again.Items[0].CommissionCents != 4050 || again.Items[1].CommissionCents != 3600 {
		t.Errorf("el historial cambió con las reglas: %+v", again.Items)
	}

	// caja: apertura $500 + efectivo $300 (ya sin el cambio) = $800
	c := p.cashSummary(t)
	if c.ExpectedCents != 80000 || c.CashInCents != 30000 || c.Sales != 1 || c.SalesCents != 42300 || c.TipCents != 3000 {
		t.Fatalf("caja: %+v", c)
	}
	bm := map[string]int{}
	for _, m := range c.ByMethod {
		bm[m.Method] = m.AmountCents
	}
	if bm["cash"] != 30000 || bm["card"] != 15300 {
		t.Errorf("por método: %v", bm)
	}
}

func TestSaleValidation(t *testing.T) {
	p := newPOS(t)
	a := p.a
	p.openCash(t, p.admin, 0)
	other := a.mustRegister(t, "Otro", "otro@x.com")
	otherClient := id(a.do(t, "POST", "/api/clients", other, map[string]any{"name": "Ajeno"}))
	otherProduct := id(a.do(t, "POST", "/api/products", other, map[string]any{"name": "Ajeno", "price_cents": 100, "stock": 5}))
	foreignStaff := a.addMember(t, other, "fs@x.com", "staff")
	var otherSvc int64 = id(a.do(t, "POST", "/api/services", other, map[string]any{"name": "Ajeno", "duration_min": 30, "price_cents": 100}))

	exact := []any{cash(15000)}
	cases := []struct {
		name string
		body map[string]any
		tok  string
		want int
	}{
		{"sin partidas", map[string]any{"lines": []any{}, "payments": exact}, p.admin, 400},
		{"cantidad cero", map[string]any{"lines": []any{map[string]any{"service_id": p.svc, "qty": 0}}, "payments": exact}, p.admin, 400},
		{"servicio y producto a la vez", map[string]any{"lines": []any{map[string]any{"service_id": p.svc, "product_id": p.cera, "qty": 1}}, "payments": exact}, p.admin, 400},
		{"ni servicio ni producto", map[string]any{"lines": []any{map[string]any{"qty": 1}}, "payments": exact}, p.admin, 400},
		{"pagos que no cubren", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(14999)}}, p.admin, 400},
		{"pagos que se pasan", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15001)}}, p.admin, 400},
		{"sin pagos", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{}}, p.admin, 400},
		{"descuento mayor al subtotal", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "discount_cents": 15001, "payments": exact}, p.admin, 400},
		{"descuento de partida mayor", map[string]any{"lines": []any{map[string]any{"service_id": p.svc, "qty": 1, "discount_cents": 15001}}, "payments": exact}, p.admin, 400},
		{"descuento negativo", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "discount_cents": -1, "payments": exact}, p.admin, 400},
		{"propina negativa", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "tip_cents": -1, "payments": exact}, p.admin, 400},
		{"método inválido", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{map[string]any{"method": "bitcoin", "amount_cents": 15000}}}, p.admin, 400},
		{"efectivo recibido menor", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{map[string]any{"method": "cash", "amount_cents": 15000, "received_cents": 14000}}}, p.admin, 400},
		{"tarjeta con monto recibido", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{map[string]any{"method": "card", "amount_cents": 15000, "received_cents": 15000}}}, p.admin, 400},
		{"terminal sin cobro", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{map[string]any{"method": "mp_point", "amount_cents": 15000}}}, p.admin, 400},
		{"servicio de otro negocio", map[string]any{"lines": []any{svcLine(otherSvc, p.marco)}, "payments": []any{cash(100)}}, p.admin, 400},
		{"producto de otro negocio", map[string]any{"lines": []any{prdLine(otherProduct, 1)}, "payments": []any{cash(100)}}, p.admin, 400},
		{"cliente de otro negocio", map[string]any{"client_id": otherClient, "lines": []any{svcLine(p.svc, p.marco)}, "payments": exact}, p.admin, 400},
		{"persona de otro negocio", map[string]any{"lines": []any{svcLine(p.svc, foreignStaff)}, "payments": exact}, p.admin, 400},
		{"propina a persona de otro negocio", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "tip_cents": 100, "tip_staff_id": foreignStaff, "payments": []any{cash(15100)}}, p.admin, 400},
		{"cajero cambia el precio", map[string]any{"lines": []any{map[string]any{"service_id": p.svc, "qty": 1, "staff_id": p.marco, "unit_price_cents": 1}}, "payments": []any{cash(1)}}, p.cashier, 403},
		{"profesional no cobra", map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": exact}, p.staff, 403},
		{"sucursal inválida", map[string]any{"branch_id": 99999, "lines": []any{svcLine(p.svc, p.marco)}, "payments": exact}, p.admin, 400},
	}
	for _, c := range cases {
		if r := p.sale(t, c.tok, c.body); r.Status != c.want {
			t.Errorf("%s: esperaba %d, obtuvo %d %v", c.name, c.want, r.Status, r.Body)
		}
	}
	// el administrador sí puede cambiar un precio; queda guardado
	s := p.mustSale(t, p.admin, map[string]any{
		"lines": []any{map[string]any{"service_id": p.svc, "qty": 1, "staff_id": p.marco, "unit_price_cents": 10000}}, "payments": []any{cash(10000)}})
	if s.TotalCents != 10000 || s.Items[0].UnitPriceCents != 10000 {
		t.Errorf("precio cambiado: %+v", s)
	}
	// nada de lo rechazado dejó ventas ni consumió folios
	var n, maxFolio int
	a.DB.QueryRow(context.Background(), `SELECT count(*), COALESCE(max(folio), 0) FROM sales`).Scan(&n, &maxFolio)
	if n != 1 || maxFolio != 1 {
		t.Errorf("ventas %d, último folio %d (esperaba 1 y 1)", n, maxFolio)
	}
}

func TestStockOnSaleAndVoid(t *testing.T) {
	p := newPOS(t)
	a := p.a
	p.openCash(t, p.admin, 0)

	s := p.mustSale(t, p.admin, map[string]any{"lines": []any{prdLine(p.cera, 2)}, "payments": []any{cash(24000)}})
	if p.stock(t) != 3 {
		t.Fatalf("existencia tras vender 2: %d", p.stock(t))
	}
	// más de lo que hay: se rechaza completo y no consume folio
	if r := p.sale(t, p.admin, map[string]any{"lines": []any{prdLine(p.cera, 4)}, "payments": []any{cash(48000)}}); r.Status != 409 {
		t.Fatalf("sin existencia: %d %v", r.Status, r.Body)
	}
	if p.stock(t) != 3 {
		t.Fatal("la venta rechazada cambió la existencia")
	}
	s2 := p.mustSale(t, p.admin, map[string]any{"lines": []any{prdLine(p.cera, 1)}, "payments": []any{cash(12000)}})
	if s2.Folio != 2 {
		t.Fatalf("el folio debe seguir sin huecos, fue %d", s2.Folio)
	}

	// cancelar devuelve la existencia, solo una vez
	if r := a.do(t, "POST", path("/api/sales/", s.ID, "/void"), p.admin, map[string]any{"reason": "Error de captura"}); r.Status != 200 {
		t.Fatalf("cancelar: %d %v", r.Status, r.Body)
	}
	if p.stock(t) != 4 {
		t.Fatalf("existencia tras cancelar: %d", p.stock(t))
	}
	if r := a.do(t, "POST", path("/api/sales/", s.ID, "/void"), p.admin, map[string]any{"reason": "otra vez"}); r.Status != 409 {
		t.Fatalf("doble cancelación: %d", r.Status)
	}
	if p.stock(t) != 4 {
		t.Fatal("la doble cancelación devolvió existencia dos veces")
	}
	// el libro cuadra: cada saldo es el anterior más el movimiento
	moves := list[stockMovement](t, a, path("/api/products/", p.cera, "/movements"), p.admin)
	sort.Slice(moves, func(i, j int) bool { return moves[i].ID < moves[j].ID })
	bal := 0
	for i, m := range moves {
		bal += m.Delta
		if m.Balance != bal {
			t.Errorf("movimiento %d (%s): saldo %d, esperaba %d", i, m.Reason, m.Balance, bal)
		}
	}
	if len(moves) != 4 || moves[0].Reason != "initial" || moves[3].Reason != "void" {
		t.Errorf("libro: %+v", moves)
	}
	// motivo obligatorio
	if r := a.do(t, "POST", path("/api/sales/", s2.ID, "/void"), p.admin, map[string]any{"reason": " "}); r.Status != 400 {
		t.Errorf("cancelar sin motivo: %d", r.Status)
	}
}

func TestConcurrentSalesOfLastUnit(t *testing.T) {
	p := newPOS(t)
	p.a.do(t, "POST", path("/api/products/", p.cera, "/stock"), p.admin, map[string]any{"delta": -4, "reason": "adjustment"}) // queda 1
	p.openCash(t, p.admin, 0)

	var wg sync.WaitGroup
	statuses := make([]int, 8)
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			statuses[i] = p.sale(t, p.admin, map[string]any{"lines": []any{prdLine(p.cera, 1)}, "payments": []any{cash(12000)}}).Status
		}(i)
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, s := range statuses {
		switch s {
		case 201:
			ok++
		case 409:
			conflict++
		}
	}
	if ok != 1 || conflict != 7 || p.stock(t) != 0 {
		t.Fatalf("ventas ok=%d conflicto=%d existencia=%d (esperaba 1, 7 y 0): %v", ok, conflict, p.stock(t), statuses)
	}
}

func TestFoliosUniqueUnderConcurrency(t *testing.T) {
	p := newPOS(t)
	p.openCash(t, p.admin, 0)
	const n = 12
	var wg sync.WaitGroup
	folios := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := p.sale(t, p.admin, map[string]any{"lines": []any{svcLine(p.barba, p.marco)}, "payments": []any{cash(8000)}})
			if r.Status == 201 {
				folios[i] = int(r.Body["folio"].(float64))
			}
		}(i)
	}
	wg.Wait()
	sort.Ints(folios)
	for i, f := range folios {
		if f != i+1 {
			t.Fatalf("folios esperados 1..%d sin huecos ni repetidos: %v", n, folios)
		}
	}
}

func TestIdempotentSale(t *testing.T) {
	p := newPOS(t)
	p.openCash(t, p.admin, 0)
	body := func() map[string]any {
		return map[string]any{"client_token": "tok-123", "lines": []any{prdLine(p.cera, 1)}, "payments": []any{cash(12000)}}
	}
	first := p.sale(t, p.admin, body())
	second := p.sale(t, p.admin, body()) // doble clic o reintento de red
	if first.Status != 201 || second.Status != 200 || first.Body["id"] != second.Body["id"] {
		t.Fatalf("repetición: %d/%d %v %v", first.Status, second.Status, first.Body["id"], second.Body["id"])
	}
	var n int
	p.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM sales`).Scan(&n)
	if n != 1 || p.stock(t) != 4 {
		t.Fatalf("ventas %d, existencia %d (esperaba 1 y 4)", n, p.stock(t))
	}
	// el mismo token en otro negocio es otra venta distinta
	other := p.a.mustRegister(t, "Otro", "otro@x.com")
	otherBranch := list[branch](t, p.a, "/api/branches", other)[0].ID
	p.must(t, p.a.do(t, "POST", "/api/cash/open", other, map[string]any{"branch_id": otherBranch, "opening_cents": 0}))
	otherSvc := id(p.a.do(t, "POST", "/api/services", other, map[string]any{"name": "Corte", "duration_min": 30, "price_cents": 5000}))
	r := p.a.do(t, "POST", "/api/sales", other, map[string]any{
		"branch_id": otherBranch, "client_token": "tok-123",
		"lines": []any{map[string]any{"service_id": otherSvc, "qty": 1}}, "payments": []any{cash(5000)}})
	if r.Status != 201 || r.Body["id"] == first.Body["id"] || r.Body["total_cents"].(float64) != 5000 {
		t.Fatalf("el token de otro negocio no debe devolver la venta de A: %d %v", r.Status, r.Body)
	}
}

func TestSaleFromAppointment(t *testing.T) {
	p := newPOS(t)
	a := p.a
	p.openCash(t, p.admin, 0)
	appt := p.mustBook(t, p.marco, at(monday, "10:00"))

	// sin indicar persona toma la de la cita; la cita queda completada
	s := p.mustSale(t, p.cashier, map[string]any{
		"appointment_id": appt,
		"lines":          []any{map[string]any{"service_id": p.svc, "qty": 1}},
		"payments":       []any{cash(15000)},
	})
	if s.Items[0].StaffID == nil || *s.Items[0].StaffID != p.marco || s.ClientName != "Cliente" || s.AppointmentID == nil {
		t.Fatalf("la venta debe tomar persona y cliente de la cita: %+v", s)
	}
	var st string
	a.DB.QueryRow(context.Background(), `SELECT status FROM appointments WHERE id = $1`, appt).Scan(&st)
	if st != "completed" {
		t.Fatalf("estado de la cita: %s", st)
	}
	// no se cobra dos veces la misma cita
	if r := p.sale(t, p.admin, map[string]any{"appointment_id": appt, "lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}}); r.Status != 409 {
		t.Fatalf("segundo cobro: %d", r.Status)
	}
	// al cancelar la venta, la cita se puede volver a cobrar
	a.do(t, "POST", path("/api/sales/", s.ID, "/void"), p.admin, map[string]any{"reason": "Cobro duplicado"})
	if r := p.sale(t, p.admin, map[string]any{"appointment_id": appt, "lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}}); r.Status != 201 {
		t.Fatalf("recobro tras cancelar: %d %v", r.Status, r.Body)
	}
	// una cita cancelada o de otra sucursal no se cobra
	cancelled := p.mustBook(t, p.marco, at(monday, "11:00"))
	a.do(t, "PATCH", path("/api/appointments/", cancelled), p.admin, map[string]any{"status": "cancelled"})
	if r := p.sale(t, p.admin, map[string]any{"appointment_id": cancelled, "lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}}); r.Status != 409 {
		t.Fatalf("cita cancelada: %d", r.Status)
	}
	if r := p.sale(t, p.admin, map[string]any{"appointment_id": 99999, "lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}}); r.Status != 400 {
		t.Fatalf("cita inexistente: %d", r.Status)
	}
}

func TestCashSessionMath(t *testing.T) {
	p := newPOS(t)
	a := p.a
	p.openCash(t, p.admin, 50000)
	if r := a.do(t, "POST", "/api/cash/open", p.admin, map[string]any{"branch_id": p.branch, "opening_cents": 1}); r.Status != 409 {
		t.Fatalf("dos cajas abiertas: %d", r.Status)
	}
	p.mustSale(t, p.cashier, map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}})
	p.mustSale(t, p.cashier, map[string]any{"lines": []any{svcLine(p.barba, p.marco)}, "payments": []any{map[string]any{"method": "card", "amount_cents": 8000}}})
	mv := func(kind string, amount int, concept string) int {
		return a.do(t, "POST", "/api/cash/movements", p.cashier, map[string]any{"branch_id": p.branch, "kind": kind, "amount_cents": amount, "concept": concept}).Status
	}
	if got := mv("out", 2000, "Compra de toallas"); got != 204 {
		t.Fatalf("salida: %d", got)
	}
	if got := mv("in", 500, "Cambio"); got != 204 {
		t.Fatalf("entrada: %d", got)
	}
	// $500 + $150 (solo el efectivo; la tarjeta no entra) − $20 + $5 = $635
	if c := p.cashSummary(t); c.ExpectedCents != 63500 || len(c.Movements) != 2 {
		t.Fatalf("esperado de la caja: %+v", c)
	}
	if got := mv("out", 63501, "Retiro"); got != 409 {
		t.Fatalf("salida mayor al efectivo: %d", got)
	}
	for _, bad := range []struct {
		kind    string
		amount  int
		concept string
	}{{"otro", 100, "x"}, {"out", 0, "xx"}, {"out", -5, "xx"}, {"out", 100, ""}} {
		if got := mv(bad.kind, bad.amount, bad.concept); got != 400 {
			t.Errorf("movimiento inválido %+v: %d", bad, got)
		}
	}

	// cierre: contó $625, faltan $10; el esperado queda congelado
	r := a.do(t, "POST", "/api/cash/close", p.cashier, map[string]any{"branch_id": p.branch, "counted_cents": 62500, "note": "Faltante"})
	var closed cashSummary
	mustJSON(t, r.Raw, &closed)
	if r.Status != 200 || closed.ExpectedCents != 63500 || closed.DiffCents == nil || *closed.DiffCents != -1000 || closed.ClosedAt == nil {
		t.Fatalf("cierre: %d %+v", r.Status, closed)
	}
	if r := a.do(t, "POST", "/api/cash/close", p.cashier, map[string]any{"branch_id": p.branch, "counted_cents": 1}); r.Status != 409 {
		t.Fatalf("cerrar dos veces: %d", r.Status)
	}
	if got := mv("out", 100, "Gasto"); got != 409 {
		t.Fatalf("movimiento con caja cerrada: %d", got)
	}
	// con la caja cerrada no se cobra, y una nueva caja empieza de cero
	if r := p.sale(t, p.admin, map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}}); r.Status != 409 {
		t.Fatalf("cobrar con caja cerrada: %d", r.Status)
	}
	p.openCash(t, p.admin, 10000)
	if c := p.cashSummary(t); c.ExpectedCents != 10000 || c.Sales != 0 {
		t.Fatalf("la caja nueva debe empezar limpia: %+v", c)
	}
	hist := list[cashSummary](t, a, fmt.Sprintf("/api/cash/sessions?branch_id=%d", p.branch), p.admin)
	if len(hist) != 2 || hist[1].DiffCents == nil || *hist[1].DiffCents != -1000 {
		t.Fatalf("historial: %+v", hist)
	}
}

func TestVoidCashAcrossSessions(t *testing.T) {
	p := newPOS(t)
	a := p.a
	void := func(id int64) resp {
		return a.do(t, "POST", path("/api/sales/", id, "/void"), p.admin, map[string]any{"reason": "Devolución al cliente"})
	}

	// venta de la caja abierta: al cancelar simplemente deja de contar, sin movimiento
	p.openCash(t, p.admin, 10000)
	same := p.mustSale(t, p.admin, map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}})
	void(same.ID)
	if c := p.cashSummary(t); c.ExpectedCents != 10000 || len(c.Movements) != 0 {
		t.Fatalf("cancelar en la misma caja: %+v", c)
	}

	// venta en efectivo, caja cerrada; cancelar sin caja abierta no se puede
	old := p.mustSale(t, p.admin, map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}})
	a.do(t, "POST", "/api/cash/close", p.admin, map[string]any{"branch_id": p.branch, "counted_cents": 25000})
	if r := void(old.ID); r.Status != 409 {
		t.Fatalf("cancelar sin caja abierta: %d %v", r.Status, r.Body)
	}
	var st string
	a.DB.QueryRow(context.Background(), `SELECT status FROM sales WHERE id = $1`, old.ID).Scan(&st)
	if st != "paid" {
		t.Fatal("la venta no debió cancelarse")
	}
	// con otra caja abierta, el efectivo sale de ella; la caja ya cerrada no cambia
	p.openCash(t, p.admin, 20000)
	if r := void(old.ID); r.Status != 200 {
		t.Fatalf("cancelar con caja nueva: %d %v", r.Status, r.Body)
	}
	c := p.cashSummary(t)
	if c.ExpectedCents != 5000 || len(c.Movements) != 1 || c.Movements[0].Kind != "out" || c.Movements[0].AmountCents != 15000 {
		t.Fatalf("la devolución debe salir de la caja abierta: %+v", c)
	}
	hist := list[cashSummary](t, a, fmt.Sprintf("/api/cash/sessions?branch_id=%d", p.branch), p.admin)
	if hist[1].ExpectedCents != 25000 {
		t.Fatalf("el esperado de la caja cerrada debe seguir congelado en $250, es %d", hist[1].ExpectedCents)
	}
}

func TestSalesPermissionsAndVisibility(t *testing.T) {
	p := newPOS(t)
	a := p.a
	p.openCash(t, p.recep, 1000) // recepción también maneja la caja
	body := func() map[string]any {
		return map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}}
	}
	s := p.mustSale(t, p.recep, body())

	for name, tok := range map[string]string{"cajero": p.cashier, "profesional": p.staff} {
		r := a.do(t, "POST", path("/api/sales/", s.ID, "/void"), tok, map[string]any{"reason": "no debería"})
		if r.Status != 403 {
			t.Errorf("%s cancelando: %d", name, r.Status)
		}
	}
	for _, c := range []struct{ method, url string }{
		{"GET", "/api/sales"}, {"GET", path("/api/sales/", s.ID)}, {"GET", fmt.Sprintf("/api/cash/current?branch_id=%d", p.branch)},
	} {
		if r := a.do(t, c.method, c.url, p.staff, nil); r.Status != 403 {
			t.Errorf("profesional en %s %s: %d", c.method, c.url, r.Status)
		}
	}
	for _, c := range []struct{ method, url string }{
		{"POST", path("/api/products/", p.cera, "/stock")}, {"GET", path("/api/products/", p.cera, "/movements")},
		{"GET", "/api/commission-rules"}, {"POST", "/api/commission-rules"},
	} {
		if r := a.do(t, c.method, c.url, p.cashier, map[string]any{}); r.Status != 403 {
			t.Errorf("cajero en %s %s: %d", c.method, c.url, r.Status)
		}
	}
	// el costo del producto solo lo ve el administrador
	if got := list[product](t, a, "/api/products", p.cashier)[0]; got.CostCents != nil {
		t.Errorf("el cajero no debe ver el costo: %v", *got.CostCents)
	}
	if got := list[product](t, a, "/api/products", p.admin)[0]; got.CostCents == nil || *got.CostCents != 5000 {
		t.Errorf("el administrador debe ver el costo: %+v", got)
	}
}

func TestSalesTenantIsolation(t *testing.T) {
	p := newPOS(t)
	a := p.a
	p.openCash(t, p.admin, 0)
	s := p.mustSale(t, p.admin, map[string]any{"lines": []any{prdLine(p.cera, 1)}, "payments": []any{cash(12000)}})
	other := a.mustRegister(t, "Otro", "otro@x.com")

	if r := a.do(t, "GET", path("/api/sales/", s.ID), other, nil); r.Status != 404 {
		t.Errorf("ver venta ajena: %d", r.Status)
	}
	if r := a.do(t, "POST", path("/api/sales/", s.ID, "/void"), other, map[string]any{"reason": "intruso"}); r.Status != 404 {
		t.Errorf("cancelar venta ajena: %d", r.Status)
	}
	if got := list[saleRow](t, a, "/api/sales", other); len(got) != 0 {
		t.Errorf("la lista de B ve ventas de A: %+v", got)
	}
	if r := a.do(t, "POST", path("/api/products/", p.cera, "/stock"), other, map[string]any{"delta": 5, "reason": "purchase"}); r.Status != 404 {
		t.Errorf("ajustar producto ajeno: %d", r.Status)
	}
	if got := list[stockMovement](t, a, path("/api/products/", p.cera, "/movements"), other); len(got) != 0 {
		t.Errorf("B ve movimientos de A: %+v", got)
	}
	if r := a.do(t, "GET", fmt.Sprintf("/api/cash/current?branch_id=%d", p.branch), other, nil); r.Body["session"] != nil {
		t.Errorf("B ve la caja de A: %v", r.Body)
	}
	if r := a.do(t, "POST", "/api/cash/open", other, map[string]any{"branch_id": p.branch, "opening_cents": 0}); r.Status != 404 {
		t.Errorf("abrir caja en sucursal ajena: %d", r.Status)
	}
	if p.stock(t) != 4 {
		t.Error("la existencia de A cambió")
	}
}

func TestStockLedgerAdjustments(t *testing.T) {
	p := newPOS(t)
	a := p.a
	adjust := func(delta int, reason string) resp {
		return a.do(t, "POST", path("/api/products/", p.cera, "/stock"), p.admin, map[string]any{"delta": delta, "reason": reason, "note": "n"})
	}
	if r := adjust(10, "purchase"); r.Status != 200 || r.Body["stock"].(float64) != 15 {
		t.Fatalf("compra: %v", r)
	}
	if r := adjust(-3, "loss"); r.Status != 200 || r.Body["stock"].(float64) != 12 {
		t.Fatalf("pérdida: %v", r)
	}
	for name, r := range map[string]resp{
		"salida mayor a la existencia": adjust(-13, "adjustment"),
		"compra negativa":              adjust(-1, "purchase"),
		"pérdida positiva":             adjust(1, "loss"),
		"cantidad cero":                adjust(0, "adjustment"),
		"motivo inválido":              adjust(1, "venta"),
	} {
		if r.Status == 200 {
			t.Errorf("%s debió rechazarse", name)
		}
	}
	if p.stock(t) != 12 {
		t.Fatalf("existencia: %d", p.stock(t))
	}
}

func TestCommissionRulesAndPriority(t *testing.T) {
	p := newPOS(t)
	a := p.a
	other := a.mustRegister(t, "Otro", "otro@x.com")
	otherSvc := id(a.do(t, "POST", "/api/services", other, map[string]any{"name": "X", "duration_min": 30, "price_cents": 100}))

	for name, body := range map[string]map[string]any{
		"sin artículo":        {"percent": 10},
		"servicio y producto": {"service_id": p.svc, "product_id": p.cera, "percent": 10},
		"porcentaje excedido": {"service_id": p.svc, "percent": 101},
		"artículo de otro":    {"service_id": otherSvc, "percent": 10},
		"persona de otro":     {"service_id": p.svc, "user_id": 99999, "percent": 10},
	} {
		if r := a.do(t, "POST", "/api/commission-rules", p.admin, body); r.Status != 400 {
			t.Errorf("%s: esperaba 400, obtuvo %d", name, r.Status)
		}
	}
	ruleAll := id(a.do(t, "POST", "/api/commission-rules", p.admin, map[string]any{"service_id": p.svc, "percent": 20}))
	p.openCash(t, p.admin, 0)
	sell := func(staff int64) *saleOut {
		return p.mustSale(t, p.admin, map[string]any{"lines": []any{svcLine(p.svc, staff)}, "payments": []any{cash(15000)}})
	}
	if got := sell(p.marco).Items[0].CommissionPct; got != 20 {
		t.Errorf("la regla del artículo (20) gana a la general de Marco (30): %v", got)
	}
	a.do(t, "POST", "/api/commission-rules", p.admin, map[string]any{"service_id": p.svc, "user_id": p.ana, "percent": 60})
	if got := sell(p.ana).Items[0].CommissionPct; got != 60 {
		t.Errorf("persona + artículo debe ganar: %v", got)
	}
	if got := sell(p.marco).Items[0].CommissionPct; got != 20 {
		t.Errorf("la regla de Ana no aplica a Marco: %v", got)
	}
	a.do(t, "PATCH", path("/api/commission-rules/", ruleAll), p.admin, map[string]any{"active": false})
	if got := sell(p.marco).Items[0].CommissionPct; got != 30 {
		t.Errorf("sin regla vuelve la comisión general: %v", got)
	}
	// producto: sin regla no hay comisión aunque la persona tenga general
	s := p.mustSale(t, p.admin, map[string]any{"lines": []any{map[string]any{"product_id": p.cera, "qty": 1, "staff_id": p.marco}}, "payments": []any{cash(12000)}})
	if s.Items[0].CommissionCents != 0 {
		t.Errorf("producto sin regla: %d", s.Items[0].CommissionCents)
	}
}

func TestTipRouting(t *testing.T) {
	p := newPOS(t)
	p.openCash(t, p.admin, 0)
	tipOf := func(body map[string]any) *int64 { return p.mustSale(t, p.admin, body).TipStaffID }

	if got := tipOf(map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "tip_cents": 1000, "payments": []any{cash(16000)}}); got == nil || *got != p.marco {
		t.Errorf("una sola persona: %v", got)
	}
	if got := tipOf(map[string]any{"lines": []any{svcLine(p.svc, p.marco), svcLine(p.barba, p.ana)}, "tip_cents": 1000, "payments": []any{cash(24000)}}); got != nil {
		t.Errorf("varias personas sin elegir: debe quedar sin asignar, fue %v", *got)
	}
	if got := tipOf(map[string]any{"lines": []any{svcLine(p.svc, p.marco), svcLine(p.barba, p.ana)}, "tip_cents": 1000, "tip_staff_id": p.ana, "payments": []any{cash(24000)}}); got == nil || *got != p.ana {
		t.Errorf("propina elegida: %v", got)
	}
	if got := tipOf(map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "tip_staff_id": p.ana, "payments": []any{cash(15000)}}); got != nil {
		t.Errorf("sin propina no hay persona de propina: %v", *got)
	}
}

func TestSalesListFilters(t *testing.T) {
	p := newPOS(t)
	a := p.a
	p.openCash(t, p.admin, 0)
	p.mustSale(t, p.admin, map[string]any{"client_id": p.cl, "lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}})
	two := p.mustSale(t, p.admin, map[string]any{"lines": []any{svcLine(p.barba, p.marco)}, "payments": []any{map[string]any{"method": "card", "amount_cents": 8000}}})
	a.do(t, "POST", path("/api/sales/", two.ID, "/void"), p.admin, map[string]any{"reason": "Prueba"})

	n := func(url string) int { return len(list[saleRow](t, a, url, p.admin)) }
	// el reloj de pruebas es fijo (2 nov 00:00 UTC): en la zona del negocio todavía es 1 de noviembre
	today := a.now().In(tijuana(t)).Format("2006-01-02")
	if n("/api/sales") != 2 || n("/api/sales?from="+today+"&to="+today) != 2 {
		t.Fatalf("hoy: %d", n("/api/sales"))
	}
	if n("/api/sales?from=2026-11-03") != 0 || n("/api/sales?from=2026-10-01&to=2026-11-30") != 2 {
		t.Error("filtro por fechas")
	}
	if n("/api/sales?q=1") != 1 || n("/api/sales?q=cliente") != 1 || n("/api/sales?q=zzz") != 0 {
		t.Error("búsqueda por folio o cliente")
	}
	rows := list[saleRow](t, a, "/api/sales", p.admin)
	if rows[0].Folio != 2 || rows[0].Status != "void" || len(rows[0].Methods) != 1 || rows[0].Methods[0] != "card" {
		t.Errorf("orden o campos: %+v", rows[0])
	}
	for _, bad := range []string{"/api/sales?from=ayer", "/api/sales?from=2026-11-05&to=2026-11-01", "/api/sales?from=2020-01-01&to=2026-11-01"} {
		if r := a.do(t, "GET", bad, p.admin, nil); r.Status != 400 {
			t.Errorf("%s: esperaba 400, obtuvo %d", bad, r.Status)
		}
	}
}
