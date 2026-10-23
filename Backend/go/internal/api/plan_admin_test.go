package api

import "testing"

func planBody(mod map[string]any) map[string]any {
	b := map[string]any{"description": "Todo para crecer", "price_month": 899, "price_year": 8990, "max_staff": 8, "max_branches": 1,
		"pos": true, "deposits": true, "deposits_mp": true, "terminal": true, "support": "prioritario"}
	for k, v := range mod {
		b[k] = v
	}
	return b
}

func publicPlan(t *testing.T, f *pbFx, id string) map[string]any {
	t.Helper()
	r := f.a.do(t, "GET", "/api/plans", "", nil)
	var list []any
	if err := jsonUnmarshal(r.Raw, &list); err != nil {
		t.Fatal(err)
	}
	for _, p := range list {
		if m := p.(map[string]any); m["id"] == id {
			return m
		}
	}
	t.Fatalf("plan %s no encontrado", id)
	return nil
}

func TestPlatformEditsPlansAndEverythingUsesTheNewValues(t *testing.T) {
	f := newPB(t)
	if publicPlan(t, f, "crecimiento")["price_month"] != float64(700) {
		t.Fatal("sin ajustes, el plan usa lo definido en el código")
	}
	// solo el administrador de plataforma edita
	for name, tok := range map[string]string{"soporte": f.support, "dueño del negocio": f.admin, "sin sesión": ""} {
		if r := f.a.do(t, "PUT", "/api/platform/plans/crecimiento", tok, planBody(nil)); r.Status == 204 {
			t.Fatalf("%s no debe poder editar planes", name)
		}
		if r := f.a.do(t, "DELETE", "/api/platform/plans/crecimiento", tok, nil); r.Status == 204 {
			t.Fatalf("%s no debe poder restablecer planes", name)
		}
	}
	for name, mod := range map[string]map[string]any{
		"precio negativo": {"price_month": -1}, "precio enorme": {"price_year": 99999999}, "sin sucursales": {"max_branches": 0},
		"personas 0": {"max_staff": 0}, "soporte inventado": {"support": "vip"}, "descripción larga": {"description": string(make([]byte, 200))},
	} {
		if r := f.a.do(t, "PUT", "/api/platform/plans/crecimiento", f.plat, planBody(mod)); r.Status != 400 {
			t.Fatalf("%s: esperaba 400 y fue %d", name, r.Status)
		}
	}
	if r := f.a.do(t, "PUT", "/api/platform/plans/inventado", f.plat, planBody(nil)); r.Status != 404 {
		t.Fatalf("plan inexistente: %d", r.Status)
	}
	if r := f.a.do(t, "PUT", "/api/platform/plans/crecimiento", f.plat, planBody(nil)); r.Status != 204 {
		t.Fatalf("editar: %d %v", r.Status, r.Body)
	}
	// la página de inicio, la suscripción del negocio y el precio del cobro usan lo nuevo
	if p := publicPlan(t, f, "crecimiento"); p["price_month"] != float64(899) || p["max_staff"] != float64(8) || p["name"] != "Crecimiento" {
		t.Fatalf("plan público: %v", p)
	}
	bill := f.a.do(t, "GET", "/api/billing", f.admin, nil)
	if bill.Body["plan"].(map[string]any)["price_month"] != float64(899) {
		t.Fatalf("la suscripción del negocio debe mostrar el precio nuevo: %v", bill.Body["plan"])
	}
	listed := f.a.do(t, "GET", "/api/platform/plans", f.plat, nil)
	if listed.Status != 200 || len(listed.Raw) == 0 {
		t.Fatalf("lista de plataforma: %d", listed.Status)
	}
	// los límites nuevos se aplican de inmediato: sin cobros, el negocio ya no puede usar la caja
	f.a.do(t, "PUT", "/api/platform/plans/crecimiento", f.plat, planBody(map[string]any{"pos": false, "max_staff": nil}))
	if r := f.a.do(t, "GET", "/api/sales", f.admin, nil); r.Status != 403 || r.Body["code"] != "plan_feature" {
		t.Fatalf("el plan sin cobros bloquea ventas: %d %v", r.Status, r.Body)
	}
	if publicPlan(t, f, "crecimiento")["max_staff"] != nil {
		t.Fatal("sin límite de personas se guarda como vacío")
	}
	// restablecer vuelve a lo del código
	if r := f.a.do(t, "DELETE", "/api/platform/plans/crecimiento", f.plat, nil); r.Status != 204 {
		t.Fatalf("restablecer: %d", r.Status)
	}
	if p := publicPlan(t, f, "crecimiento"); p["price_month"] != float64(700) || p["pos"] != true {
		t.Fatalf("tras restablecer: %v", p)
	}
	if r := f.a.do(t, "GET", "/api/sales", f.admin, nil); r.Status != 200 {
		t.Fatalf("de nuevo con cobros: %d", r.Status)
	}
}

func TestPlatformUsageMetricsDoNotExposeClientData(t *testing.T) {
	f := newPB(t)
	ov := f.a.do(t, "GET", "/api/platform/overview", f.plat, nil)
	u, ok := ov.Body["usage_30d"].(map[string]any)
	if !ok || u["appointments"] == nil || u["online_bookings"] == nil || u["sales"] == nil || u["active_businesses"] == nil {
		t.Fatalf("uso en 30 días: %v", ov.Body["usage_30d"])
	}
	for _, tok := range []string{f.plat, f.support} {
		d := f.a.do(t, "GET", path("/api/platform/businesses/", f.biz), tok, nil)
		m, ok := d.Body["metrics"].(map[string]any)
		if !ok || m["appointments_30d"] == nil || m["sales_30d"] == nil {
			t.Fatalf("métricas del negocio: %v", d.Body["metrics"])
		}
	}
	if r := f.a.do(t, "GET", "/api/platform/overview", f.admin, nil); r.Status == 200 {
		t.Fatal("el dueño de un negocio no ve el resumen de la plataforma")
	}
}
