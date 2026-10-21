package api

import (
	"context"
	"testing"
)

func TestBusinessProfileEditAndValidation(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "Barbería Norte", "a@x.com")
	other := a.mustRegister(t, "Otra", "otra@x.com")
	g := a.do(t, "GET", "/api/business", admin, nil)
	if g.Status != 200 || g.Body["name"] != "Barbería Norte" || g.Body["slug"] == "" || g.Body["timezone"] == "" {
		t.Fatalf("leer: %d %v", g.Status, g.Body)
	}
	slug := g.Body["slug"]
	ok := map[string]any{"name": "  Barbería Sur  ", "kind": "spa", "timezone": "America/Mexico_City"}
	if r := a.do(t, "PUT", "/api/business", admin, ok); r.Status != 204 {
		t.Fatalf("guardar: %d %v", r.Status, r.Body)
	}
	g = a.do(t, "GET", "/api/business", admin, nil)
	if g.Body["name"] != "Barbería Sur" || g.Body["kind"] != "spa" || g.Body["timezone"] != "America/Mexico_City" || g.Body["slug"] != slug {
		t.Fatalf("tras guardar: %v", g.Body)
	}
	for name, body := range map[string]map[string]any{
		"sin nombre": {"name": " ", "kind": "spa", "timezone": "America/Tijuana"}, "giro raro": {"name": "N", "kind": "x", "timezone": "America/Tijuana"},
		"zona inválida": {"name": "N", "kind": "spa", "timezone": "Marte/Olimpo"}, "zona vacía": {"name": "N", "kind": "spa", "timezone": ""}, "zona Local": {"name": "N", "kind": "spa", "timezone": "Local"},
	} {
		if r := a.do(t, "PUT", "/api/business", admin, body); r.Status != 400 {
			t.Errorf("%s: %d", name, r.Status)
		}
	}
	// el otro negocio no se toca
	var n string
	a.DB.QueryRow(context.Background(), `SELECT name FROM businesses WHERE slug <> $1`, slug).Scan(&n)
	if n != "Otra" {
		t.Fatalf("otro negocio: %q", n)
	}
	_ = other
	// solo administradores
	a.addMember(t, admin, "recep@x.com", "reception")
	rec := a.doLogin(t, "recep@x.com", "supersecreta1").Body["token"].(string)
	if r := a.do(t, "PUT", "/api/business", rec, ok); r.Status != 403 {
		t.Fatalf("recepción: %d", r.Status)
	}
	if r := a.do(t, "GET", "/api/business", "", nil); r.Status != 401 {
		t.Fatalf("anónimo: %d", r.Status)
	}
	// la zona horaria cambia cómo se interpreta la agenda
	if loc := a.businessLoc(context.Background(), 1); loc.String() != "America/Mexico_City" && loc.String() != "America/Tijuana" {
		t.Fatalf("zona: %s", loc)
	}
}
