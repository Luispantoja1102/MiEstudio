package api

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

type pkgFx struct {
	*posFx
	pkg int64 // 5 cortes por $600 (Corte vale $150)
}

func newPackages(t *testing.T) *pkgFx {
	t.Helper()
	p := &pkgFx{posFx: newPOS(t)}
	r := p.a.do(t, "POST", "/api/packages", p.admin, map[string]any{"name": "5 cortes", "service_id": p.svc, "sessions": 5, "price_cents": 60000, "valid_days": 90})
	p.must(t, r)
	p.pkg = id(r)
	p.openCash(t, p.admin, 0)
	return p
}

func pkgLine(pkg int64) map[string]any { return map[string]any{"package_id": pkg, "qty": 1} }
func pkgPay(amount int, cp int64) map[string]any {
	return map[string]any{"method": "package", "amount_cents": amount, "client_package_id": cp}
}

// buy vende el paquete al cliente y devuelve (venta, id del saldo).
func (p *pkgFx) buy(t *testing.T, client int64) (*saleOut, int64) {
	t.Helper()
	s := p.mustSale(t, p.admin, map[string]any{"client_id": client, "lines": []any{pkgLine(p.pkg)}, "payments": []any{cash(60000)}})
	var cp int64
	p.a.DB.QueryRow(context.Background(), `SELECT id FROM client_packages WHERE sale_id = $1`, s.ID).Scan(&cp)
	if cp == 0 {
		t.Fatal("la venta debe crear el saldo del cliente")
	}
	return s, cp
}

func (p *pkgFx) used(t *testing.T, cp int64) int {
	var n int
	p.a.DB.QueryRow(context.Background(), `SELECT sessions_used FROM client_packages WHERE id = $1`, cp).Scan(&n)
	return n
}

func (p *pkgFx) redeem(t *testing.T, client, cp int64, amount int) resp {
	return p.sale(t, p.admin, map[string]any{"client_id": client, "lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{pkgPay(amount, cp)}})
}

func TestPackageCatalogRules(t *testing.T) {
	p := newPackages(t)
	bad := []map[string]any{
		{"name": "", "service_id": p.svc, "sessions": 5, "price_cents": 100},
		{"name": "x", "service_id": p.svc, "sessions": 1, "price_cents": 100},
		{"name": "x", "service_id": p.svc, "sessions": 101, "price_cents": 100},
		{"name": "x", "service_id": p.svc, "sessions": 5, "price_cents": -1},
		{"name": "x", "service_id": p.svc, "sessions": 5, "price_cents": 100, "valid_days": 0},
		{"name": "x", "service_id": 999999, "sessions": 5, "price_cents": 100},
	}
	for i, b := range bad {
		if r := p.a.do(t, "POST", "/api/packages", p.admin, b); r.Status != 400 {
			t.Errorf("caso %d: %d %v", i, r.Status, r.Body)
		}
	}
	// otro negocio no puede usar mis servicios
	other := p.a.mustRegister(t, "Otro", "otro@x.com")
	if r := p.a.do(t, "POST", "/api/packages", other, map[string]any{"name": "x", "service_id": p.svc, "sessions": 5, "price_cents": 100}); r.Status != 400 {
		t.Errorf("servicio ajeno: %d", r.Status)
	}
	if r := p.a.do(t, "GET", "/api/packages", other, nil); r.Status != 200 || len(r.List()) != 0 {
		t.Errorf("catálogo ajeno: %v", r.Body)
	}
	if r := p.a.do(t, "POST", "/api/packages", p.cashier, map[string]any{"name": "x", "service_id": p.svc, "sessions": 5, "price_cents": 100}); r.Status != 403 {
		t.Errorf("caja no crea: %d", r.Status)
	}
	if r := p.a.do(t, "GET", "/api/packages", p.recep, nil); r.Status != 200 || len(r.List()) != 1 {
		t.Errorf("recepción ve el catálogo: %v", r.Body)
	}
	if r := p.a.do(t, "GET", "/api/packages", p.staff, nil); r.Status != 403 {
		t.Errorf("profesional no cobra: %d", r.Status)
	}
	// desactivar lo oculta de quien cobra y se puede quitar la vigencia
	p.must(t, p.a.do(t, "PATCH", path("/api/packages/", p.pkg), p.admin, map[string]any{"active": false, "valid_days": 0, "price_cents": 55000}))
	if r := p.a.do(t, "GET", "/api/packages", p.admin, nil); len(r.List()) != 0 {
		t.Errorf("inactivo visible: %v", r.Body)
	}
	all := p.a.do(t, "GET", "/api/packages?all=1", p.admin, nil).List()
	if len(all) != 1 || all[0]["valid_days"] != nil || all[0]["price_cents"] != float64(55000) {
		t.Errorf("edición: %v", all)
	}
	if r := p.sale(t, p.admin, map[string]any{"client_id": p.cl, "lines": []any{pkgLine(p.pkg)}, "payments": []any{cash(55000)}}); r.Status != 400 {
		t.Errorf("vender inactivo: %d %v", r.Status, r.Body)
	}
}

func TestSellPackageNeedsClientAndGrantsBalance(t *testing.T) {
	p := newPackages(t)
	if r := p.sale(t, p.admin, map[string]any{"lines": []any{pkgLine(p.pkg)}, "payments": []any{cash(60000)}}); r.Status != 400 {
		t.Fatalf("sin cliente: %d %v", r.Status, r.Body)
	}
	if r := p.sale(t, p.admin, map[string]any{"client_id": p.cl, "lines": []any{map[string]any{"package_id": p.pkg, "qty": 2}}, "payments": []any{cash(120000)}}); r.Status != 400 {
		t.Fatalf("dos paquetes en una partida: %d", r.Status)
	}
	if r := p.sale(t, p.admin, map[string]any{"client_id": p.cl, "lines": []any{map[string]any{"package_id": p.pkg, "service_id": p.svc, "qty": 1}}, "payments": []any{cash(75000)}}); r.Status != 400 {
		t.Fatalf("partida doble: %d", r.Status)
	}
	if r := p.sale(t, p.admin, map[string]any{"client_id": p.cl, "lines": []any{map[string]any{"package_id": p.pkg, "qty": 1, "staff_id": p.marco}}, "payments": []any{cash(60000)}}); r.Status != 400 {
		t.Fatalf("paquete con persona: %d", r.Status)
	}
	s, cp := p.buy(t, p.cl)
	if len(s.Items) != 1 || s.Items[0].Kind != "package" || s.Items[0].CommissionCents != 0 || s.Items[0].TotalCents != 60000 {
		t.Fatalf("partida: %+v", s.Items)
	}
	list := p.a.do(t, "GET", path("/api/clients/", p.cl, "/packages"), p.recep, nil).List()
	if len(list) != 1 || list[0]["remaining"] != float64(5) || list[0]["status"] != "active" || list[0]["id"] != float64(cp) {
		t.Fatalf("saldo: %v", list)
	}
	// vigencia de 90 días desde la venta (reloj de pruebas: 2026-11-02)
	var exp time.Time
	p.a.DB.QueryRow(context.Background(), `SELECT expires_at FROM client_packages WHERE id = $1`, cp).Scan(&exp)
	if want := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 90); !exp.Equal(want) {
		t.Fatalf("vence %v, esperado %v", exp, want)
	}
	// el saldo es propio del cliente y del negocio
	other := p.a.mustRegister(t, "Otro", "otro@x.com")
	if r := p.a.do(t, "GET", path("/api/clients/", p.cl, "/packages"), other, nil); len(r.List()) != 0 {
		t.Fatalf("saldo ajeno visible: %v", r.Body)
	}
	// cambiar el catálogo no altera lo ya vendido
	p.must(t, p.a.do(t, "PATCH", path("/api/packages/", p.pkg), p.admin, map[string]any{"name": "Otro nombre"}))
	var name string
	p.a.DB.QueryRow(context.Background(), `SELECT name FROM client_packages WHERE id = $1`, cp).Scan(&name)
	if name != "5 cortes" {
		t.Fatalf("el saldo cambió de nombre: %s", name)
	}
	// el efectivo del paquete entra a la caja
	cs := p.a.do(t, "GET", path("/api/cash/current?branch_id=", p.branch), p.admin, nil)
	if sess, _ := cs.Body["session"].(map[string]any); sess["expected_cents"] != float64(60000) {
		t.Fatalf("caja: %v", cs.Body)
	}
}

func TestRedeemPackageSession(t *testing.T) {
	p := newPackages(t)
	_, cp := p.buy(t, p.cl)

	r := p.redeem(t, p.cl, cp, 15000)
	if r.Status != 201 {
		t.Fatalf("canje: %d %v", r.Status, r.Body)
	}
	if p.used(t, cp) != 1 {
		t.Fatalf("usadas: %d", p.used(t, cp))
	}
	var s saleOut
	mustJSON(t, r.Raw, &s)
	// la comisión de la sesión se paga al profesional (30 % de $150)
	if len(s.Payments) != 1 || s.Payments[0].Method != "package" || s.Items[0].CommissionCents != 4500 {
		t.Fatalf("venta: %+v", s)
	}
	// la caja no recibe dinero de un canje
	cs := p.a.do(t, "GET", path("/api/cash/current?branch_id=", p.branch), p.admin, nil)
	if sess, _ := cs.Body["session"].(map[string]any); sess["expected_cents"] != float64(60000) {
		t.Fatalf("caja: %v", cs.Body)
	}
	// el reporte no cuenta dos veces el dinero: $600 del paquete, $0 del canje
	rep := p.a.do(t, "GET", "/api/reports/sales?from=2026-11-01&to=2026-11-30&branch_id="+itoa(p.branch), p.admin, nil)
	if rep.Body["collected_cents"] != float64(60000) {
		t.Fatalf("cobrado: %v", rep.Body["collected_cents"])
	}
	found := false
	for _, m := range rep.Body["by_method"].([]any) {
		if m.(map[string]any)["method"] == "package" && m.(map[string]any)["amount_cents"] == float64(15000) {
			found = true
		}
	}
	if !found {
		t.Fatalf("por método: %v", rep.Body["by_method"])
	}

	// el importe debe ser el de la sesión
	if r := p.redeem(t, p.cl, cp, 14000); r.Status != 400 {
		t.Errorf("importe distinto: %d %v", r.Status, r.Body)
	}
	// sin cliente, o con otro cliente, no se canjea
	if r := p.sale(t, p.admin, map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{pkgPay(15000, cp)}}); r.Status != 400 {
		t.Errorf("sin cliente: %d", r.Status)
	}
	other := id(p.a.do(t, "POST", "/api/clients", p.admin, map[string]any{"name": "Otra"}))
	if r := p.redeem(t, other, cp, 15000); r.Status != 400 {
		t.Errorf("otro cliente: %d %v", r.Status, r.Body)
	}
	// solo cubre el servicio del paquete
	if r := p.sale(t, p.admin, map[string]any{"client_id": p.cl, "lines": []any{svcLine(p.barba, p.marco)}, "payments": []any{pkgPay(8000, cp)}}); r.Status != 400 {
		t.Errorf("otro servicio: %d %v", r.Status, r.Body)
	}
	// una sesión por unidad: dos canjes para una sola unidad no se permiten
	if r := p.sale(t, p.admin, map[string]any{"client_id": p.cl, "lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{pkgPay(7500, cp), pkgPay(7500, cp)}}); r.Status != 400 {
		t.Errorf("doble canje: %d %v", r.Status, r.Body)
	}
	// ni usar el mismo canje para pagar otro servicio que cuesta lo mismo
	barba15 := map[string]any{"service_id": p.barba, "qty": 1, "staff_id": p.ana, "unit_price_cents": 15000}
	if r := p.sale(t, p.admin, map[string]any{"client_id": p.cl, "lines": []any{svcLine(p.svc, p.marco), barba15}, "payments": []any{pkgPay(15000, cp), pkgPay(15000, cp)}}); r.Status != 400 {
		t.Errorf("canje sobre otro servicio: %d %v", r.Status, r.Body)
	}
	if p.used(t, cp) != 1 {
		t.Fatalf("los rechazos no deben consumir sesiones: %d", p.used(t, cp))
	}
	// dos unidades, dos sesiones, más una parte en efectivo de otra partida
	r = p.sale(t, p.admin, map[string]any{"client_id": p.cl,
		"lines":    []any{map[string]any{"service_id": p.svc, "qty": 2, "staff_id": p.marco}, svcLine(p.barba, p.ana)},
		"payments": []any{pkgPay(15000, cp), pkgPay(15000, cp), cash(8000)}})
	if r.Status != 201 || p.used(t, cp) != 3 {
		t.Fatalf("canje doble: %d %v usadas=%d", r.Status, r.Body, p.used(t, cp))
	}
}

func TestRedeemPackageExhaustedExpiredAndVoid(t *testing.T) {
	p := newPackages(t)
	_, cp := p.buy(t, p.cl)
	p.a.DB.Exec(context.Background(), `UPDATE client_packages SET sessions_used = 4 WHERE id = $1`, cp)
	if r := p.redeem(t, p.cl, cp, 15000); r.Status != 201 {
		t.Fatalf("última sesión: %d %v", r.Status, r.Body)
	}
	if r := p.redeem(t, p.cl, cp, 15000); r.Status != 409 {
		t.Fatalf("agotado: %d %v", r.Status, r.Body)
	}
	if list := p.a.do(t, "GET", path("/api/clients/", p.cl, "/packages"), p.admin, nil).List(); list[0]["status"] != "exhausted" || list[0]["remaining"] != float64(0) {
		t.Fatalf("estado: %v", list)
	}

	// vencido
	_, cp2 := p.buy(t, p.cl)
	p.a.DB.Exec(context.Background(), `UPDATE client_packages SET expires_at = '2026-11-01T00:00:00Z' WHERE id = $1`, cp2)
	if r := p.redeem(t, p.cl, cp2, 15000); r.Status != 409 {
		t.Fatalf("vencido: %d %v", r.Status, r.Body)
	}
	got := map[float64]any{}
	for _, c := range p.a.do(t, "GET", path("/api/clients/", p.cl, "/packages"), p.admin, nil).List() {
		got[c["id"].(float64)] = c["status"]
	}
	if got[float64(cp)] != "exhausted" || got[float64(cp2)] != "expired" {
		t.Fatalf("estados: %v", got)
	}
	// uno vigente aparece antes que los agotados y vencidos
	_, cp3 := p.buy(t, p.cl)
	if list := p.a.do(t, "GET", path("/api/clients/", p.cl, "/packages"), p.admin, nil).List(); list[0]["id"] != float64(cp3) || list[0]["status"] != "active" {
		t.Fatalf("orden: %v", list)
	}
}

func TestVoidRestoresSessionsAndGuardsPackageSale(t *testing.T) {
	p := newPackages(t)
	bought, cp := p.buy(t, p.cl)
	red := p.redeem(t, p.cl, cp, 15000)
	if red.Status != 201 {
		t.Fatalf("canje: %d %v", red.Status, red.Body)
	}
	redID := id(red)

	// con una sesión usada no se puede cancelar la venta del paquete, y nada cambia
	if r := p.a.do(t, "POST", path("/api/sales/", bought.ID, "/void"), p.admin, map[string]any{"reason": "error de captura"}); r.Status != 409 {
		t.Fatalf("cancelar con sesiones usadas: %d %v", r.Status, r.Body)
	}
	var st string
	p.a.DB.QueryRow(context.Background(), `SELECT status FROM sales WHERE id = $1`, bought.ID).Scan(&st)
	if st != "paid" {
		t.Fatalf("la venta no debía cancelarse: %s", st)
	}

	// cancelar el canje devuelve la sesión
	if r := p.a.do(t, "POST", path("/api/sales/", redID, "/void"), p.admin, map[string]any{"reason": "error de captura"}); r.Status != 200 {
		t.Fatalf("cancelar canje: %d %v", r.Status, r.Body)
	}
	if p.used(t, cp) != 0 {
		t.Fatalf("la sesión no volvió: %d", p.used(t, cp))
	}

	// ahora sí: sin sesiones usadas se cancela y el saldo se anula
	if r := p.a.do(t, "POST", path("/api/sales/", bought.ID, "/void"), p.admin, map[string]any{"reason": "error de captura"}); r.Status != 200 {
		t.Fatalf("cancelar venta del paquete: %d %v", r.Status, r.Body)
	}
	if r := p.redeem(t, p.cl, cp, 15000); r.Status != 409 {
		t.Fatalf("canjear un paquete anulado: %d %v", r.Status, r.Body)
	}
	if list := p.a.do(t, "GET", path("/api/clients/", p.cl, "/packages"), p.admin, nil).List(); list[0]["status"] != "void" {
		t.Fatalf("estado: %v", list)
	}
}

func TestLastSessionRedeemedOnlyOnceConcurrently(t *testing.T) {
	p := newPackages(t)
	_, cp := p.buy(t, p.cl)
	p.a.DB.Exec(context.Background(), `UPDATE client_packages SET sessions_used = 4 WHERE id = $1`, cp)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := p.redeem(t, p.cl, cp, 15000)
			if r.Status != 201 && r.Status != 409 {
				t.Errorf("respuesta inesperada: %d %v", r.Status, r.Body)
			}
			if r.Status == 201 {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 || p.used(t, cp) != 5 {
		t.Fatalf("canjes aceptados: %d, usadas: %d", ok, p.used(t, cp))
	}
}

// List interpreta el cuerpo crudo como un arreglo de objetos.
func (r resp) List() []map[string]any {
	var out []map[string]any
	json.Unmarshal(r.Raw, &out)
	return out
}
