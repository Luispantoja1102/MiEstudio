package api

import "testing"

func (o *onlineFixture) setTheme(t *testing.T, theme any, want int) {
	t.Helper()
	body := map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2,
		"deposit_percent": 0, "reminder_hours": 24, "show_prices": true, "welcome_text": "Hola"}
	if theme != nil {
		body["public_theme"] = theme
	}
	if r := o.a.do(t, "PUT", "/api/booking/settings", o.admin, body); r.Status != want {
		t.Fatalf("ajustes con tema %v: %d %v", theme, r.Status, r.Body)
	}
}

func TestPublicThemeIsChosenByTheBusiness(t *testing.T) {
	o := newOnline(t)
	// por omisión (y con clientes que no envían el campo) manda quien visita
	info := func() any { return o.a.do(t, "GET", "/api/public/"+o.slug, "", nil).Body["theme"] }
	if info() != "auto" {
		t.Fatalf("por omisión: %v", info())
	}
	o.setTheme(t, nil, 204)
	if info() != "auto" {
		t.Fatalf("sin campo: %v", info())
	}
	for _, bad := range []any{"neon", "DARK", 3} {
		if bad == 3 {
			continue // un número ni siquiera decodifica a texto
		}
		o.setTheme(t, bad, 400)
	}
	for _, th := range []string{"lino", "light", "ivory", "dark", "auto"} {
		o.setTheme(t, th, 204)
		if info() != th {
			t.Fatalf("tema %s: %v", th, info())
		}
		if got := o.a.do(t, "GET", "/api/booking/settings", o.admin, nil).Body["settings"].(map[string]any)["public_theme"]; got != th {
			t.Fatalf("el panel ve %v, esperado %s", got, th)
		}
	}
	// la cita y el aviso de privacidad usan el mismo tema
	o.setTheme(t, "dark", 204)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	if got := o.a.do(t, "GET", "/api/public-appointments/"+token, "", nil).Body["theme"]; got != "dark" {
		t.Fatalf("cita: %v", got)
	}
	if got := o.a.do(t, "GET", "/api/public/"+o.slug+"/privacy", "", nil).Body["theme"]; got != "dark" {
		t.Fatalf("aviso: %v", got)
	}
	// otro negocio no se ve afectado
	other := o.a.mustRegister(t, "Otro", "otro@x.com")
	if r := o.a.do(t, "GET", "/api/booking/settings", other, nil); r.Body["settings"].(map[string]any)["public_theme"] != "auto" {
		t.Fatalf("otro negocio: %v", r.Body)
	}
}

func TestOnlyTheAdministratorCanChoosePublicTheme(t *testing.T) {
	o := newOnline(t)
	o.setTheme(t, "lino", 204)
	o.a.addMember(t, o.admin, "recep@x.com", "reception")
	o.a.addMember(t, o.admin, "caja@x.com", "cashier")
	body := map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2,
		"deposit_percent": 0, "reminder_hours": 24, "show_prices": true, "welcome_text": "Hola", "public_theme": "dark"}
	for _, tok := range []string{login(t, o.a, "recep@x.com"), login(t, o.a, "caja@x.com"), login(t, o.a, "marco@x.com")} {
		if r := o.a.do(t, "PUT", "/api/booking/settings", tok, body); r.Status != 403 {
			t.Fatalf("un rol que no es administrador cambió el tema: %d %v", r.Status, r.Body)
		}
		if r := o.a.do(t, "GET", "/api/booking/settings", tok, nil); r.Status != 403 {
			t.Fatalf("un rol que no es administrador ve los ajustes: %d", r.Status)
		}
	}
	if got := o.a.do(t, "GET", "/api/public/"+o.slug, "", nil).Body["theme"]; got != "lino" {
		t.Fatalf("el tema cambió sin permiso: %v", got)
	}
	// ni sin sesión
	if r := o.a.do(t, "PUT", "/api/booking/settings", "", body); r.Status != 401 {
		t.Fatalf("sin sesión: %d", r.Status)
	}
}
