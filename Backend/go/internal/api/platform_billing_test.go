package api

import (
	"context"
	"testing"
	"time"
)

type pbFx struct {
	a       *API
	plat    string
	support string
	biz     int64
	admin   string
}

func newPB(t *testing.T) *pbFx {
	t.Helper()
	a := newTestAPI(t)
	f := &pbFx{a: a, plat: newPlatform(t, a)}
	if r := a.do(t, "POST", "/api/platform/team", f.plat, map[string]any{"name": "Soporte", "email": "soporte@p.com", "password": "ClaveLarga123", "role": "platform_support"}); r.Status != 201 {
		t.Fatalf("soporte: %d %v", r.Status, r.Body)
	}
	f.support = a.doLogin(t, "soporte@p.com", "ClaveLarga123").Body["token"].(string)
	f.admin = a.mustRegister(t, "Barbería Norte", "norte@x.com")
	a.DB.QueryRow(context.Background(), `SELECT id FROM businesses`).Scan(&f.biz)
	return f
}

func (f *pbFx) pay(t *testing.T, body map[string]any) resp {
	return f.a.do(t, "POST", path("/api/platform/businesses/", f.biz, "/payments"), f.plat, body)
}

func (f *pbFx) row(t *testing.T) (status, plan string, end *time.Time, reason string) {
	f.a.DB.QueryRow(context.Background(), `SELECT status, plan, current_period_end, suspended_reason FROM businesses WHERE id = $1`, f.biz).Scan(&status, &plan, &end, &reason)
	return
}

func TestRecordPaymentExtendsAndActivates(t *testing.T) {
	f := newPB(t)
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	f.a.Now = func() time.Time { return now }
	r := f.pay(t, map[string]any{"amount_cents": 79900, "months": 1, "plan": "crecimiento", "method": "transfer", "note": "SPEI 123"})
	if r.Status != 201 {
		t.Fatalf("pago: %d %v", r.Status, r.Body)
	}
	status, plan, end, _ := f.row(t)
	if status != "active" || plan != "crecimiento" || end == nil || !end.Equal(now.AddDate(0, 1, 0)) {
		t.Fatalf("negocio: %s %s %v", status, plan, end)
	}
	// renovar antes de vencer suma al periodo vigente (no pierde días)
	f.a.Now = func() time.Time { return now.AddDate(0, 0, 10) }
	f.pay(t, map[string]any{"amount_cents": 79900, "months": 1, "method": "transfer"})
	_, _, end2, _ := f.row(t)
	if !end2.Equal(now.AddDate(0, 2, 0)) {
		t.Fatalf("debía sumarse al periodo vigente: %v", end2)
	}
	// después de vencer el periodo empieza hoy
	late := now.AddDate(0, 5, 0)
	f.a.Now = func() time.Time { return late }
	f.pay(t, map[string]any{"amount_cents": 79900, "months": 1, "method": "cash"})
	_, _, end3, _ := f.row(t)
	if !end3.Equal(late.AddDate(0, 1, 0)) {
		t.Fatalf("tras vencer empieza hoy: %v", end3)
	}
	// un plan distinto en el pago cambia el plan
	f.pay(t, map[string]any{"amount_cents": 1499000, "months": 12, "plan": "pro", "method": "transfer"})
	if _, plan, _, _ := f.row(t); plan != "pro" {
		t.Fatalf("plan: %s", plan)
	}
}

func TestPaymentReactivatesSuspendedAndClearsReason(t *testing.T) {
	f := newPB(t)
	if r := f.a.do(t, "PATCH", path("/api/platform/businesses/", f.biz), f.plat, map[string]any{"status": "suspended", "suspended_reason": "Pago atrasado 30 días"}); r.Status != 204 {
		t.Fatalf("suspender: %d", r.Status)
	}
	if _, _, _, reason := f.row(t); reason != "Pago atrasado 30 días" {
		t.Fatalf("motivo: %q", reason)
	}
	if r := f.a.do(t, "GET", "/api/services", f.admin, nil); r.Status != 403 || r.Body["code"] != "business_suspended" {
		t.Fatalf("suspendido: %d %v", r.Status, r.Body)
	}
	f.pay(t, map[string]any{"amount_cents": 79900, "months": 1, "method": "transfer"})
	if status, _, _, reason := f.row(t); status != "active" || reason != "" {
		t.Fatalf("el pago reactiva: %s %q", status, reason)
	}
	// reactivar a mano también limpia el motivo
	f.a.do(t, "PATCH", path("/api/platform/businesses/", f.biz), f.plat, map[string]any{"status": "suspended", "suspended_reason": "x"})
	f.a.do(t, "PATCH", path("/api/platform/businesses/", f.biz), f.plat, map[string]any{"status": "active"})
	if _, _, _, reason := f.row(t); reason != "" {
		t.Fatalf("motivo tras reactivar: %q", reason)
	}
}

func TestRecordPaymentValidationAndPermissions(t *testing.T) {
	f := newPB(t)
	for name, body := range map[string]map[string]any{
		"sin meses": {"amount_cents": 100, "months": 0, "method": "transfer"}, "25 meses": {"amount_cents": 100, "months": 25, "method": "transfer"},
		"método raro": {"amount_cents": 100, "months": 1, "method": "bitcoin"}, "monto cero": {"amount_cents": 0, "months": 1, "method": "transfer"},
		"monto negativo": {"amount_cents": -5, "months": 1, "method": "cash"}, "cortesía con monto": {"amount_cents": 100, "months": 1, "method": "courtesy"},
		"plan inválido": {"amount_cents": 100, "months": 1, "method": "cash", "plan": "gratis"}, "mercadopago manual": {"amount_cents": 100, "months": 1, "method": "mercadopago"},
	} {
		if r := f.pay(t, body); r.Status != 400 {
			t.Errorf("%s: %d", name, r.Status)
		}
	}
	if r := f.pay(t, map[string]any{"amount_cents": 0, "months": 2, "method": "courtesy"}); r.Status != 201 {
		t.Errorf("cortesía en cero: %d", r.Status)
	}
	// solo el administrador de plataforma; soporte y negocios no
	for name, tok := range map[string]string{"soporte": f.support, "negocio": f.admin} {
		if r := f.a.do(t, "POST", path("/api/platform/businesses/", f.biz, "/payments"), tok, map[string]any{"amount_cents": 100, "months": 1, "method": "cash"}); r.Status != 403 {
			t.Errorf("%s registrando pago: %d", name, r.Status)
		}
	}
	if r := f.a.do(t, "POST", "/api/platform/businesses/99999/payments", f.plat, map[string]any{"amount_cents": 100, "months": 1, "method": "cash"}); r.Status != 404 {
		t.Errorf("negocio inexistente: %d", r.Status)
	}
}

func TestMoneyIsAdminOnly(t *testing.T) {
	f := newPB(t)
	f.pay(t, map[string]any{"amount_cents": 79900, "months": 1, "method": "transfer"})
	det := f.a.do(t, "GET", path("/api/platform/businesses/", f.biz), f.plat, nil)
	if det.Status != 200 || len(det.Body["payments"].([]any)) != 1 || det.Body["usage"] == nil || det.Body["limits"] == nil {
		t.Fatalf("detalle del admin: %v", det.Body)
	}
	sup := f.a.do(t, "GET", path("/api/platform/businesses/", f.biz), f.support, nil)
	if sup.Status != 200 || sup.Body["payments"] != nil {
		t.Fatalf("soporte no ve pagos: %v", sup.Body["payments"])
	}
	if r := f.a.do(t, "GET", "/api/platform/payments", f.support, nil); r.Status != 403 {
		t.Fatalf("lista global de pagos para soporte: %d", r.Status)
	}
	if r := f.a.do(t, "GET", "/api/platform/payments", f.plat, nil); r.Status != 200 {
		t.Fatalf("lista global: %d", r.Status)
	}
	ov := f.a.do(t, "GET", "/api/platform/overview", f.support, nil)
	if _, has := ov.Body["mrr_cents"]; has {
		t.Fatal("el resumen de soporte no debe traer dinero")
	}
}

func TestOverviewCountsMRRAndAttention(t *testing.T) {
	f := newPB(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	f.a.Now = func() time.Time { return now }
	mk := func(name, status string, trial, period *time.Time) int64 {
		var id int64
		f.a.DB.QueryRow(ctx, `INSERT INTO businesses (name, slug, kind, status, trial_ends_at, current_period_end, created_at) VALUES ($1, $2, 'barberia', $3, $4, $5, $6) RETURNING id`,
			name, name, status, trial, period, now.Add(-48*time.Hour)).Scan(&id)
		return id
	}
	day := 24 * time.Hour
	mk("pend", "pending", nil, nil)
	mk("prueba-fin", "trial", tp(now.Add(2*day)), nil)
	mk("prueba-larga", "trial", tp(now.Add(10*day)), nil)
	mk("prueba-vencida", "trial", tp(now.Add(-day)), nil)
	mk("atrasado", "active", nil, tp(now.Add(-5*day)))
	mk("suspendido", "suspended", nil, nil)
	paid := mk("pagando", "active", nil, tp(now.Add(20*day)))
	annual := mk("anual", "active", nil, tp(now.Add(200*day)))
	mk("cortesia", "active", nil, nil)
	f.a.DB.Exec(ctx, `UPDATE businesses SET status = 'active', current_period_end = $2 WHERE id = $1`, f.biz, now.Add(20*day))
	ins := func(id int64, cents, months int, method string, created time.Time) {
		f.a.DB.Exec(ctx, `INSERT INTO payments (business_id, amount_cents, method, plan, months, period_start, period_end, created_at) VALUES ($1, $2, $3, 'crecimiento', $4, $5, $5, $5)`, id, cents, method, months, created)
	}
	ins(paid, 79900, 1, "transfer", now.Add(-2*day))
	ins(annual, 799000, 12, "transfer", now.Add(-3*day))
	ins(f.biz, 0, 1, "courtesy", now.Add(-day))      // una cortesía no suma
	ins(paid, 0, 1, "courtesy", now.Add(-time.Hour)) // ni borra lo que ya pagaba: el último pago de verdad es el que cuenta
	ins(paid, 50000, 1, "transfer", now.AddDate(0, -2, 0))

	ov := f.a.do(t, "GET", "/api/platform/overview", f.plat, nil)
	c := ov.Body["counts"].(map[string]any)
	want := map[string]float64{"pending": 1, "trialing": 2, "trial_expired": 1, "past_due": 1, "suspended": 1, "active": 4}
	for k, v := range want {
		if c[k] != v {
			t.Errorf("conteo %s: %v, esperaba %v", k, c[k], v)
		}
	}
	// MRR: $799 + $7990/12 (= $665.83) = $1,464.83; la cortesía y los pagos viejos de otro negocio no cuentan
	if got := ov.Body["mrr_cents"].(float64); got != 79900+66583 {
		t.Errorf("MRR: %v", got)
	}
	// cobrado en noviembre: 799.00 + 7,990.00 (la cortesía en cero y el pago de septiembre no)
	if got := ov.Body["paid_this_month_cents"].(float64); got != 79900+799000 {
		t.Errorf("cobrado este mes: %v", got)
	}
	var kinds []string
	for _, x := range ov.Body["attention"].([]any) {
		kinds = append(kinds, x.(map[string]any)["kind"].(string))
	}
	wantKinds := []string{"pending", "past_due", "trial_expired", "trial_ending"}
	if len(kinds) != 4 {
		t.Fatalf("atención: %v", kinds)
	}
	for i, k := range wantKinds {
		if kinds[i] != k {
			t.Fatalf("orden de atención: %v", kinds)
		}
	}
	if len(ov.Body["recent"].([]any)) != 5 {
		t.Errorf("recientes: %d", len(ov.Body["recent"].([]any)))
	}
	// claves de compatibilidad
	if b := ov.Body["businesses"].(map[string]any); b["suspended"] != float64(1) || ov.Body["total"] != float64(10) {
		t.Errorf("compat: %v total=%v", b, ov.Body["total"])
	}
}

func TestListFiltersByStateAndPlan(t *testing.T) {
	f := newPB(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	f.a.Now = func() time.Time { return now }
	f.a.DB.Exec(ctx, `UPDATE businesses SET status = 'active', current_period_end = $1, plan = 'pro'`, now.Add(-10*24*time.Hour))
	f.a.DB.Exec(ctx, `INSERT INTO businesses (name, slug, kind, status, trial_ends_at, plan) VALUES ('Otro', 'otro', 'spa', 'trial', $1, 'basico')`, now.Add(5*24*time.Hour))
	count := func(q string) int {
		r := f.a.do(t, "GET", "/api/platform/businesses?"+q, f.plat, nil)
		var out []any
		jsonUnmarshal(r.Raw, &out)
		return len(out)
	}
	for q, want := range map[string]int{"state=past_due": 1, "state=trialing": 1, "state=active": 0, "plan=pro": 1, "plan=basico": 1, "plan=crecimiento": 0, "state=past_due&plan=basico": 0, "": 2} {
		if got := count(q); got != want {
			t.Errorf("filtro %q: %d, esperaba %d", q, got, want)
		}
	}
}

func TestPlansEndpointAndPatchPlanAndPeriod(t *testing.T) {
	f := newPB(t)
	r := f.a.do(t, "GET", "/api/platform/plans", f.support, nil)
	var plans []map[string]any
	jsonUnmarshal(r.Raw, &plans)
	if r.Status != 200 || len(plans) != 3 || plans[0]["id"] != "basico" || plans[1]["businesses"] != float64(1) {
		t.Fatalf("planes: %d %v", r.Status, plans)
	}
	if r := f.a.do(t, "PATCH", path("/api/platform/businesses/", f.biz), f.plat, map[string]any{"plan": "gratis"}); r.Status != 400 {
		t.Fatalf("plan inválido: %d", r.Status)
	}
	if r := f.a.do(t, "PATCH", path("/api/platform/businesses/", f.biz), f.plat, map[string]any{"plan": "pro", "status": "active", "current_period_end": "2027-01-31"}); r.Status != 204 {
		t.Fatalf("cambiar plan y periodo: %d", r.Status)
	}
	if status, plan, end, _ := f.row(t); status != "active" || plan != "pro" || end == nil || end.Format("2006-01-02") != "2027-01-31" {
		t.Fatalf("resultado: %s %s %v", status, plan, end)
	}
	if r := f.a.do(t, "PATCH", path("/api/platform/businesses/", f.biz), f.support, map[string]any{"plan": "basico"}); r.Status != 403 {
		t.Fatalf("soporte no cambia planes: %d", r.Status)
	}
}

func TestHealthEndpoint(t *testing.T) {
	f := newPB(t)
	r := f.a.do(t, "GET", "/api/platform/health", f.plat, nil)
	if r.Status != 200 || r.Body["ok"] != true {
		t.Fatalf("salud: %d %v", r.Status, r.Body)
	}
	keys := map[string]bool{}
	for _, c := range r.Body["checks"].([]any) {
		keys[c.(map[string]any)["key"].(string)] = true
	}
	for _, k := range []string{"database", "migrations", "mail", "outbox", "holds", "mp_oauth", "mp_platform", "webhook", "app_url", "production"} {
		if !keys[k] {
			t.Errorf("falta la revisión %q", k)
		}
	}
	if r := f.a.do(t, "GET", "/api/platform/health", f.support, nil); r.Status != 403 {
		t.Fatalf("soporte: %d", r.Status)
	}
	if r := f.a.do(t, "GET", "/api/platform/health", f.admin, nil); r.Status != 403 {
		t.Fatalf("negocio: %d", r.Status)
	}
}
