package api

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMergeAndFreeMinutes(t *testing.T) {
	d := func(h, m int) time.Time { return time.Date(2026, 11, 9, h, m, 0, 0, time.UTC) }
	merged := mergeSpans([]span{{d(12, 0), d(13, 0)}, {d(10, 0), d(11, 0)}, {d(10, 30), d(11, 30)}})
	if len(merged) != 2 || !merged[0].end.Equal(d(11, 30)) {
		t.Fatalf("bloqueos combinados: %+v", merged)
	}
	// 09:00-14:00 menos 10:00-11:30 y 12:00-13:00 = 5 h - 1.5 h - 1 h = 2.5 h
	if got := freeMinutes(d(9, 0), d(14, 0), merged); got != 150 {
		t.Fatalf("minutos libres: %d", got)
	}
	// un bloqueo que empieza antes del horario solo cuenta lo que lo cruza
	if got := freeMinutes(d(9, 0), d(10, 0), []span{{d(8, 0), d(9, 30)}}); got != 30 {
		t.Fatalf("bloqueo parcial: %d", got)
	}
}

func TestOccupancyReport(t *testing.T) {
	f := newFixture(t) // Marco trabaja los lunes de 9:00 a 14:00 (300 min); Corte dura 30 min
	a := f.a
	day := fmt.Sprintf("/api/reports/occupancy?from=%s&to=%s", monday, monday)

	r := a.do(t, "GET", day, f.admin, nil)
	if r.Status != 200 || r.Body["available_min"] != float64(300) || r.Body["booked_min"] != float64(0) || r.Body["occupancy_pct"] != float64(0) {
		t.Fatalf("sin citas: %d %v", r.Status, r.Body)
	}

	f.mustBook(t, f.marco, at(monday, "09:00"))
	second := f.mustBook(t, f.marco, at(monday, "10:00"))
	third := f.mustBook(t, f.marco, at(monday, "11:00"))
	a.DB.Exec(context.Background(), `UPDATE appointments SET status = 'no_show' WHERE id = $1`, second)
	a.DB.Exec(context.Background(), `UPDATE appointments SET status = 'cancelled' WHERE id = $1`, third)
	f.mustBook(t, f.marco, at(monday, "12:00"))

	r = a.do(t, "GET", day, f.admin, nil)
	b := r.Body
	if b["appointments"] != float64(2) || b["booked_min"] != float64(60) || b["occupancy_pct"] != float64(20) ||
		b["no_shows"] != float64(1) || b["cancelled"] != float64(1) || b["no_show_pct"].(float64) < 33 || b["no_show_pct"].(float64) > 34 {
		t.Fatalf("con citas: %v", b)
	}
	staff := b["staff"].([]any)
	if len(staff) != 1 || staff[0].(map[string]any)["name"] != "Marco" || staff[0].(map[string]any)["available_min"] != float64(300) {
		t.Fatalf("por persona: %v", b["staff"])
	}
	// el lunes (weekday 1) concentra todo; las 9 y las 12 son horas con citas
	if wd := b["by_weekday"].([]any)[1].(map[string]any); wd["available_min"] != float64(300) || wd["booked_min"] != float64(60) {
		t.Fatalf("por día: %v", wd)
	}
	if heat := b["heat"].([]any); len(heat) != 2 || heat[0].(map[string]any)["hour"] != float64(9) {
		t.Fatalf("horas pico: %v", b["heat"])
	}

	// un bloqueo (comida) reduce lo disponible
	a.must2(t, a.do(t, "POST", "/api/blocks", f.admin, map[string]any{"user_id": f.marco, "starts_at": at(monday, "13:00"), "ends_at": at(monday, "14:00"), "reason": "Comida"}))
	if r := a.do(t, "GET", day, f.admin, nil); r.Body["available_min"] != float64(240) {
		t.Fatalf("con bloqueo: %v", r.Body)
	}

	// rango inválido, solo administración y sin sesión
	if r := a.do(t, "GET", "/api/reports/occupancy?from=hoy", f.admin, nil); r.Status != 400 {
		t.Fatalf("rango inválido: %d", r.Status)
	}
	if r := a.do(t, "GET", day, login(t, a, "marco@x.com"), nil); r.Status != 403 {
		t.Fatalf("profesional: %d", r.Status)
	}
	if r := a.do(t, "GET", day, "", nil); r.Status != 401 {
		t.Fatalf("sin sesión: %d", r.Status)
	}
}

func TestRetentionReport(t *testing.T) {
	p := newPOS(t)
	a := p.a
	a.Now = func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) }
	p.openCash(t, p.admin, 0)

	client := func(name string) int64 {
		return id(a.do(t, "POST", "/api/clients", p.admin, map[string]any{"name": name}))
	}
	cA, cB, cC, cD := p.cl, client("B nuevo"), client("C perdido"), client("D una vez")
	visit := func(c int64, date string) {
		s := p.mustSale(t, p.admin, map[string]any{"client_id": c, "lines": []any{svcLine(p.svc, p.marco)}, "payments": []any{cash(15000)}})
		when, _ := time.Parse(time.RFC3339, date+"T18:00:00Z")
		if _, err := a.DB.Exec(context.Background(), `UPDATE sales SET created_at = $2 WHERE id = $1`, s.ID, when); err != nil {
			t.Fatal(err)
		}
	}
	visit(cA, "2026-09-15")
	visit(cA, "2026-10-10") // vuelve: retenido
	visit(cB, "2026-10-20") // nuevo
	visit(cC, "2026-06-01")
	visit(cC, "2026-08-25") // dos visitas y hace más de 60 días: en riesgo
	visit(cD, "2026-09-05") // una sola visita y no volvió

	r := a.do(t, "GET", "/api/reports/retention?from=2026-10-01&to=2026-10-31", p.admin, nil)
	b := r.Body
	want := map[string]float64{"active": 2, "new": 1, "returning": 1, "repeaters": 0, "prev_active": 2, "retained": 1, "retention_pct": 50, "one_visit": 2, "total_clients": 4}
	for k, v := range want {
		if b[k] != v {
			t.Errorf("%s: esperaba %v, obtuvo %v (%v)", k, v, b[k], b)
		}
	}
	if b["prev_from"] != "2026-08-31" || b["prev_to"] != "2026-09-30" {
		t.Errorf("periodo anterior: %v - %v", b["prev_from"], b["prev_to"])
	}
	lapsed := b["lapsed"].([]any)
	if len(lapsed) != 1 || lapsed[0].(map[string]any)["name"] != "C perdido" || lapsed[0].(map[string]any)["visits"] != float64(2) {
		t.Fatalf("en riesgo: %v", b["lapsed"])
	}
	top := b["top"].([]any)
	if len(top) != 2 {
		t.Fatalf("mejores clientes del periodo: %v", top)
	}

	// otro negocio no ve a estos clientes; un profesional no entra
	other := a.mustRegister(t, "Otro", "otro@x.com")
	if r := a.do(t, "GET", "/api/reports/retention?from=2026-10-01&to=2026-10-31", other, nil); r.Body["total_clients"] != float64(0) {
		t.Fatalf("aislamiento: %v", r.Body)
	}
	if r := a.do(t, "GET", "/api/reports/retention?from=2026-10-01&to=2026-10-31", p.staff, nil); r.Status != 403 {
		t.Fatalf("profesional: %d", r.Status)
	}
}
