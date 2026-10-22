package api

import (
	"fmt"
	"testing"
	"time"
)

type reportView struct {
	Sales          int           `json:"sales"`
	SubtotalCents  int           `json:"subtotal_cents"`
	DiscountCents  int           `json:"discount_cents"`
	TotalCents     int           `json:"total_cents"`
	TipCents       int           `json:"tip_cents"`
	CollectedCents int           `json:"collected_cents"`
	CostCents      int           `json:"cost_cents"`
	ByMethod       []methodTotal `json:"by_method"`
	ByStaff        []staffReport `json:"by_staff"`
	TopItems       []itemReport  `json:"top_items"`
	ByDay          []dayReport   `json:"by_day"`
}

type commissionsView struct {
	Staff []staffReport    `json:"staff"`
	Lines []commissionLine `json:"lines"`
}

func (p *posFx) clock(t *testing.T, rfc string) {
	t.Helper()
	at, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		t.Fatal(err)
	}
	p.a.Now = func() time.Time { return at }
}

// Ventas del reporte (hora de Tijuana, UTC-7 hasta el 1 nov y UTC-8 después):
//
//	V1  1 nov 10:00  corte Marco $150 + cera $120 (costo $50) · propina $10 a Marco · efectivo $280
//	V2  1 nov 23:30  barba Ana $80 · propina $5 a Ana · tarjeta $85   (en UTC ya es 2 nov)
//	V3  2 nov 12:00  corte Marco $150 · transferencia · CANCELADA (no cuenta)
//	V4  3 nov 10:00  corte Marco $150 · efectivo
func seedReportSales(t *testing.T, p *posFx) {
	t.Helper()
	p.openCash(t, p.admin, 0)
	p.clock(t, "2026-11-01T10:00:00-07:00")
	p.mustSale(t, p.admin, map[string]any{"lines": []any{svcLine(p.svc, p.marco), prdLine(p.cera, 1)}, "tip_cents": 1000, "payments": []any{cash(28000)}})
	p.clock(t, "2026-11-01T23:30:00-07:00")
	p.mustSale(t, p.admin, map[string]any{"lines": []any{svcLine(p.barba, p.ana)}, "tip_cents": 500, "tip_staff_id": p.ana,
		"payments": []any{map[string]any{"method": "card", "amount_cents": 8500}}})
	p.clock(t, "2026-11-02T12:00:00-08:00")
	v3 := p.mustSale(t, p.admin, map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{map[string]any{"method": "transfer", "amount_cents": 15000}}})
	p.must(t, p.a.do(t, "POST", path("/api/sales/", v3.ID, "/void"), p.admin, map[string]any{"reason": "Prueba de reporte"}))
	p.clock(t, "2026-11-03T10:00:00-08:00")
	p.mustSale(t, p.admin, map[string]any{"lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}})
}

func report(t *testing.T, p *posFx, token, query string) reportView {
	t.Helper()
	r := p.a.do(t, "GET", "/api/reports/sales?"+query, token, nil)
	if r.Status != 200 {
		t.Fatalf("reporte %s: %d %v", query, r.Status, r.Body)
	}
	var v reportView
	mustJSON(t, r.Raw, &v)
	return v
}

func TestSalesReportTotals(t *testing.T) {
	p := newPOS(t)
	seedReportSales(t, p)

	// 1 nov: V1 + V2
	v := report(t, p, p.admin, "from=2026-11-01&to=2026-11-01")
	if v.Sales != 2 || v.SubtotalCents != 35000 || v.DiscountCents != 0 || v.TotalCents != 35000 || v.TipCents != 1500 || v.CollectedCents != 36500 || v.CostCents != 5000 {
		t.Fatalf("totales del 1 nov: %+v", v)
	}
	bm := map[string]methodTotal{}
	for _, m := range v.ByMethod {
		bm[m.Method] = m
	}
	if bm["cash"].AmountCents != 28000 || bm["card"].AmountCents != 8500 || len(bm) != 2 {
		t.Errorf("por método (la venta cancelada no cuenta): %+v", v.ByMethod)
	}
	// una venta a las 23:30 locales cuenta en SU día local aunque en UTC sea el siguiente
	if len(v.ByDay) != 1 || v.ByDay[0].Date != "2026-11-01" || v.ByDay[0].Sales != 2 || v.ByDay[0].TotalCents != 35000 {
		t.Errorf("por día: %+v", v.ByDay)
	}
	// por persona: Marco (corte $150, comisión 30 % = $45, propina $10) y Ana (barba $80, 40 % = $32, propina $5)
	if len(v.ByStaff) != 2 || v.ByStaff[0].Name != "Marco" || v.ByStaff[0].RevenueCents != 15000 || v.ByStaff[0].CommissionCents != 4500 || v.ByStaff[0].TipCents != 1000 || v.ByStaff[0].Items != 1 ||
		v.ByStaff[1].Name != "Ana" || v.ByStaff[1].RevenueCents != 8000 || v.ByStaff[1].CommissionCents != 3200 || v.ByStaff[1].TipCents != 500 {
		t.Errorf("por persona: %+v", v.ByStaff)
	}
	if len(v.TopItems) != 3 || v.TopItems[0].Name != "Corte" || v.TopItems[0].TotalCents != 15000 || v.TopItems[1].Name != "Cera" || v.TopItems[1].Kind != "product" {
		t.Errorf("más vendido: %+v", v.TopItems)
	}

	// del 1 al 3: V1 + V2 + V4 (V3 cancelada)
	v = report(t, p, p.admin, "from=2026-11-01&to=2026-11-03")
	if v.Sales != 3 || v.TotalCents != 50000 || len(v.ByDay) != 2 || v.ByDay[1].Date != "2026-11-03" || v.ByDay[1].TotalCents != 15000 {
		t.Errorf("1 al 3 nov: %+v", v)
	}
	// un día sin ventas
	if v = report(t, p, p.admin, "from=2026-11-02&to=2026-11-02"); v.Sales != 0 || v.TotalCents != 0 || len(v.ByStaff) != 0 || len(v.ByDay) != 0 {
		t.Errorf("2 nov (solo hay una venta cancelada): %+v", v)
	}
	// otra sucursal
	p.a.DB.Exec(t.Context(), `UPDATE businesses SET plan = 'pro'`) // varias sucursales son del plan Pro
	branch2 := id(p.a.do(t, "POST", "/api/branches", p.admin, map[string]any{"name": "Centro"}))
	if v = report(t, p, p.admin, fmt.Sprintf("from=2026-11-01&to=2026-11-03&branch_id=%d", branch2)); v.Sales != 0 {
		t.Errorf("filtro por sucursal: %+v", v)
	}
}

func TestSalesReportWithDiscounts(t *testing.T) {
	p := newPOS(t)
	p.openCash(t, p.admin, 0)
	p.clock(t, "2026-11-01T10:00:00-07:00")
	// corte $150 + barba $80 = $230, descuento $23 → total $207
	p.mustSale(t, p.admin, map[string]any{"discount_cents": 2300, "lines": []any{svcLine(p.svc, p.marco), svcLine(p.barba, p.marco)}, "payments": []any{cash(20700)}})
	v := report(t, p, p.admin, "from=2026-11-01&to=2026-11-01")
	if v.SubtotalCents != 23000 || v.DiscountCents != 2300 || v.TotalCents != 20700 {
		t.Fatalf("%+v", v)
	}
	// lo atribuido a la persona suma exactamente lo cobrado (sin perder centavos por el reparto)
	if len(v.ByStaff) != 1 || v.ByStaff[0].RevenueCents != 20700 {
		t.Fatalf("ingreso de la persona: %+v", v.ByStaff)
	}
}

func TestReportsPermissions(t *testing.T) {
	p := newPOS(t)
	seedReportSales(t, p)
	for _, c := range []struct {
		name, tok, url string
		want           int
	}{
		{"cajero ve ventas", p.cashier, "/api/reports/sales", 403},
		{"recepción ve ventas", p.recep, "/api/reports/sales", 403},
		{"profesional ve ventas", p.staff, "/api/reports/sales", 403},
		{"cajero ve comisiones", p.cashier, "/api/reports/commissions", 403},
		{"recepción ve comisiones", p.recep, "/api/reports/commissions", 403},
		{"sin sesión", "", "/api/reports/sales", 401},
		{"fechas inválidas", p.admin, "/api/reports/sales?from=ayer", 400},
		{"rango invertido", p.admin, "/api/reports/commissions?from=2026-11-05&to=2026-11-01", 400},
	} {
		if r := p.a.do(t, "GET", c.url, c.tok, nil); r.Status != c.want {
			t.Errorf("%s: esperaba %d, obtuvo %d", c.name, c.want, r.Status)
		}
	}
	other := p.a.mustRegister(t, "Otro", "otro@x.com")
	if v := report(t, p, other, "from=2026-11-01&to=2026-11-03"); v.Sales != 0 || v.TotalCents != 0 || len(v.ByStaff) != 0 {
		t.Errorf("otro negocio ve datos de A: %+v", v)
	}
}

func commissions(t *testing.T, p *posFx, token, query string) commissionsView {
	t.Helper()
	r := p.a.do(t, "GET", "/api/reports/commissions?"+query, token, nil)
	if r.Status != 200 {
		t.Fatalf("comisiones %s: %d %v", query, r.Status, r.Body)
	}
	var v commissionsView
	mustJSON(t, r.Raw, &v)
	return v
}

func TestCommissionsReportScopes(t *testing.T) {
	p := newPOS(t)
	seedReportSales(t, p)
	period := "from=2026-11-01&to=2026-11-03"

	// el administrador ve a todos, sin detalle si no pide a una persona
	all := commissions(t, p, p.admin, period)
	if len(all.Staff) != 2 || len(all.Lines) != 0 {
		t.Fatalf("administrador: %+v", all)
	}
	got := map[string]staffReport{}
	for _, s := range all.Staff {
		got[s.Name] = s
	}
	// Marco: V1 corte ($45) + V4 corte ($45) = $90 sobre $300, propina $10 · Ana: $32 sobre $80, propina $5
	if m := got["Marco"]; m.Items != 2 || m.RevenueCents != 30000 || m.CommissionCents != 9000 || m.TipCents != 1000 {
		t.Errorf("Marco: %+v", m)
	}
	if a := got["Ana"]; a.Items != 1 || a.RevenueCents != 8000 || a.CommissionCents != 3200 || a.TipCents != 500 {
		t.Errorf("Ana: %+v", a)
	}
	// detalle de una persona: dos partidas de Marco (la cancelada no aparece), de la más reciente a la más antigua
	d := commissions(t, p, p.admin, period+fmt.Sprintf("&staff_id=%d", p.marco))
	if len(d.Staff) != 1 || len(d.Lines) != 2 || d.Lines[0].Folio != 4 || d.Lines[1].Folio != 1 || d.Lines[0].Pct != 30 || d.Lines[0].CommissionCents != 4500 {
		t.Errorf("detalle de Marco: %+v", d)
	}
	// periodo más corto
	if one := commissions(t, p, p.admin, "from=2026-11-01&to=2026-11-01"); len(one.Staff) != 2 {
		t.Errorf("1 nov: %+v", one.Staff)
	}
	if one := commissions(t, p, p.admin, "from=2026-11-03&to=2026-11-03"); len(one.Staff) != 1 || one.Staff[0].Name != "Marco" {
		t.Errorf("3 nov: %+v", one.Staff)
	}

	// cada profesional solo ve lo suyo, aunque pida a otra persona
	mine := commissions(t, p, p.staff, period+fmt.Sprintf("&staff_id=%d", p.ana))
	if len(mine.Staff) != 1 || mine.Staff[0].Name != "Marco" || len(mine.Lines) != 2 {
		t.Errorf("Marco pidiendo lo de Ana: %+v", mine)
	}
	anaTok := login(t, p.a, "ana@x.com")
	hers := commissions(t, p, anaTok, period)
	if len(hers.Staff) != 1 || hers.Staff[0].Name != "Ana" || len(hers.Lines) != 1 || hers.Lines[0].CommissionCents != 3200 {
		t.Errorf("Ana: %+v", hers)
	}
}
