package api

import (
	"context"
	"strings"
	"testing"
)

func regWithPlan(t *testing.T, a *API, email, plan string) resp {
	t.Helper()
	body := map[string]any{"business_name": "Barbería Sur", "kind": "barberia", "name": "Dueño", "email": email, "password": "supersecreta1", "accept_terms": true}
	if plan != "" {
		body["plan"] = plan
	}
	return a.do(t, "POST", "/api/auth/register", "", body)
}

func planRow(a *API, email string) (plan, requested string) {
	a.DB.QueryRow(context.Background(), `SELECT b.plan, COALESCE(b.requested_plan, '') FROM businesses b JOIN users u ON u.business_id = b.id WHERE lower(u.email) = $1`, email).Scan(&plan, &requested)
	return
}

func TestRegistrationStartsOnBasicAndKeepsTheRequestedPlan(t *testing.T) {
	a := newTestAPI(t)
	fm := &fakeMailer{}
	a.Mailer, a.AppURL, a.PlatformNoticeTo, a.SignupVerifyOff = fm, "https://app.test", "equipo@plataforma.test", true

	if r := regWithPlan(t, a, "pro@x.com", "pro"); r.Status != 201 {
		t.Fatalf("registro con plan Pro: %d %v", r.Status, r.Body)
	}
	if plan, req := planRow(a, "pro@x.com"); plan != "basico" || req != "pro" {
		t.Fatalf("debe probar Básico y pedir Pro: %s / %s", plan, req)
	}
	// sin elegir nada se pide el Básico
	if r := regWithPlan(t, a, "nada@x.com", ""); r.Status != 201 {
		t.Fatalf("registro sin plan: %d", r.Status)
	}
	if plan, req := planRow(a, "nada@x.com"); plan != "basico" || req != "basico" {
		t.Fatalf("sin plan: %s / %s", plan, req)
	}
	// un plan que no existe no pasa
	if r := regWithPlan(t, a, "raro@x.com", "gratis"); r.Status != 400 {
		t.Fatalf("plan inexistente: %d", r.Status)
	}

	// el aviso al equipo dice qué prueba y qué pidió
	waitMail(t, fm, 4)
	var notice string
	for _, m := range fm.all() {
		if m.To == "equipo@plataforma.test" && strings.Contains(m.Subject, "Barbería Sur") && strings.Contains(m.Text, "pro@x.com") {
			notice = m.Text
		}
	}
	for _, want := range []string{"Plan activo en la prueba: Básico · $350 al mes", "Plan solicitado: Pro · $1200 al mes", "Activar plan solicitado"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("falta %q en el aviso:\n%s", want, notice)
		}
	}
}

func TestPlatformSeesTheRequestedPlanAndActivatingItClearsIt(t *testing.T) {
	a := newTestAPI(t)
	a.SignupVerifyOff = true
	plat := newPlatform(t, a)
	regWithPlan(t, a, "p@x.com", "crecimiento")
	var id int64
	a.DB.QueryRow(context.Background(), `SELECT business_id FROM users WHERE email = 'p@x.com'`).Scan(&id)

	biz := func() map[string]any {
		return a.do(t, "GET", path("/api/platform/businesses/", id), plat, nil).Body["business"].(map[string]any)
	}
	if b := biz(); b["plan"] != "basico" || b["requested_plan"] != "crecimiento" || b["requested_plan_name"] != "Crecimiento" {
		t.Fatalf("ficha: %v", b)
	}
	// otro plan distinto del pedido no borra la solicitud
	if r := a.do(t, "PATCH", path("/api/platform/businesses/", id), plat, map[string]any{"plan": "pro"}); r.Status != 204 {
		t.Fatalf("cambiar a otro plan: %d", r.Status)
	}
	if b := biz(); b["plan"] != "pro" || b["requested_plan"] != "crecimiento" {
		t.Fatalf("la solicitud sigue pendiente: %v", b)
	}
	// activar el solicitado la cierra
	if r := a.do(t, "PATCH", path("/api/platform/businesses/", id), plat, map[string]any{"plan": "crecimiento"}); r.Status != 204 {
		t.Fatalf("activar el solicitado: %d", r.Status)
	}
	if b := biz(); b["plan"] != "crecimiento" || b["requested_plan"] != "" {
		t.Fatalf("aprobado: %v", b)
	}
}
