package api

import (
	"fmt"
	"testing"
)

func categoryOf(t *testing.T, a *API, token string, svc int64) *int64 {
	t.Helper()
	for _, s := range list[service](t, a, "/api/services", token) {
		if s.ID == svc {
			return s.CategoryID
		}
	}
	t.Fatalf("servicio %d no encontrado", svc)
	return nil
}

func TestServiceCategories(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "N", "a@x.com")
	other := a.mustRegister(t, "Otro", "b@x.com")
	a.addMember(t, admin, "caja@x.com", "cashier")
	cashier := login(t, a, "caja@x.com")

	// solo el administrador escribe; cualquiera del negocio lee
	if r := a.do(t, "POST", "/api/service-categories", cashier, map[string]any{"name": "Cabello"}); r.Status != 403 {
		t.Fatalf("caja crea categoría: %d", r.Status)
	}
	for i, bad := range []string{"", "   ", string(make([]rune, 41))} {
		if r := a.do(t, "POST", "/api/service-categories", admin, map[string]any{"name": bad}); r.Status != 400 {
			t.Errorf("nombre inválido %d: %d", i, r.Status)
		}
	}
	hair := id(a.do(t, "POST", "/api/service-categories", admin, map[string]any{"name": " Cabello "}))
	beard := id(a.do(t, "POST", "/api/service-categories", admin, map[string]any{"name": "Barba"}))
	if r := a.do(t, "POST", "/api/service-categories", admin, map[string]any{"name": "cabello"}); r.Status != 409 {
		t.Fatalf("nombre repetido sin importar mayúsculas: %d", r.Status)
	}
	// en otro negocio el mismo nombre sí se puede
	a.must2(t, a.do(t, "POST", "/api/service-categories", other, map[string]any{"name": "Cabello"}))
	otherCat := list[serviceCategory](t, a, "/api/service-categories", other)[0].ID

	// servicios con categoría: al crear, al editar y al quitarla con 0
	cut := id(a.do(t, "POST", "/api/services", admin, map[string]any{"name": "Corte", "duration_min": 30, "price_cents": 15000, "category_id": hair}))
	trim := id(a.do(t, "POST", "/api/services", admin, map[string]any{"name": "Perfilado", "duration_min": 20, "price_cents": 8000}))
	if c := categoryOf(t, a, cashier, cut); c == nil || *c != hair {
		t.Fatalf("categoría del corte: %v", c)
	}
	if c := categoryOf(t, a, cashier, trim); c != nil {
		t.Fatalf("sin categoría: %v", *c)
	}
	a.must2(t, a.do(t, "PATCH", path("/api/services/", trim), admin, map[string]any{"category_id": beard}))
	a.must2(t, a.do(t, "PATCH", path("/api/services/", trim), admin, map[string]any{"price_cents": 9000})) // no toca la categoría
	if c := categoryOf(t, a, admin, trim); c == nil || *c != beard {
		t.Fatalf("editar otro campo no debe quitar la categoría: %v", c)
	}
	// una categoría de otro negocio no vale
	if r := a.do(t, "PATCH", path("/api/services/", trim), admin, map[string]any{"category_id": otherCat}); r.Status != 400 {
		t.Fatalf("categoría ajena: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/services", admin, map[string]any{"name": "X", "duration_min": 30, "price_cents": 100, "category_id": otherCat}); r.Status != 400 {
		t.Fatalf("crear con categoría ajena: %d", r.Status)
	}

	// renombrar y contar
	a.must2(t, a.do(t, "PATCH", path("/api/service-categories/", hair), admin, map[string]any{"name": "Corte de cabello"}))
	if r := a.do(t, "PATCH", path("/api/service-categories/", hair), other, map[string]any{"name": "Hackeo"}); r.Status != 404 {
		t.Fatalf("renombrar ajena: %d", r.Status)
	}
	cats := list[serviceCategory](t, a, "/api/service-categories", cashier)
	if len(cats) != 2 || cats[0].Name != "Barba" || cats[0].Count != 1 || cats[1].Name != "Corte de cabello" || cats[1].Count != 1 {
		t.Fatalf("categorías: %+v", cats)
	}

	// eliminar moviendo los servicios a otra categoría
	if r := a.do(t, "DELETE", fmt.Sprintf("/api/service-categories/%d?move_to=%d", hair, otherCat), admin, nil); r.Status != 400 {
		t.Fatalf("mover a categoría ajena: %d", r.Status)
	}
	if r := a.do(t, "DELETE", fmt.Sprintf("/api/service-categories/%d", hair), other, nil); r.Status != 404 {
		t.Fatalf("eliminar ajena: %d", r.Status)
	}
	a.must2(t, a.do(t, "DELETE", fmt.Sprintf("/api/service-categories/%d?move_to=%d", hair, beard), admin, nil))
	if c := categoryOf(t, a, admin, cut); c == nil || *c != beard {
		t.Fatalf("el corte debía pasar a Barba: %v", c)
	}
	// eliminar sin destino deja los servicios sin categoría
	a.must2(t, a.do(t, "DELETE", path("/api/service-categories/", beard), admin, nil))
	if c := categoryOf(t, a, admin, cut); c != nil {
		t.Fatalf("debía quedar sin categoría: %v", *c)
	}
	if got := list[serviceCategory](t, a, "/api/service-categories", admin); len(got) != 0 {
		t.Fatalf("quedaron categorías: %+v", got)
	}
}
