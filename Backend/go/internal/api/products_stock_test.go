package api

import "testing"

func TestProductMinStockAndBarcode(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "N", "a@x.com")
	other := a.mustRegister(t, "Otro", "b@x.com")

	// mínimo y código inválidos
	for i, bad := range []map[string]any{
		{"name": "X", "price_cents": 100, "min_stock": -1},
		{"name": "X", "price_cents": 100, "barcode": "con espacio"},
		{"name": "X", "price_cents": 100, "barcode": string(make([]byte, 65))},
	} {
		if r := a.do(t, "POST", "/api/products", admin, bad); r.Status != 400 {
			t.Errorf("inválido %d: esperaba 400, obtuvo %d", i, r.Status)
		}
	}

	pid := id(a.do(t, "POST", "/api/products", admin, map[string]any{"name": "Cera", "price_cents": 100, "stock": 5, "min_stock": 3, "barcode": " 750100 "}))
	got := list[product](t, a, "/api/products", admin)[0]
	if got.MinStock != 3 || got.Barcode != "750100" {
		t.Fatalf("mínimo y código: %+v", got)
	}

	// el código es único por negocio (en otro negocio sí se puede repetir) y vacío no cuenta
	if r := a.do(t, "POST", "/api/products", admin, map[string]any{"name": "Gel", "price_cents": 100, "barcode": "750100"}); r.Status != 409 {
		t.Fatalf("código repetido: %d", r.Status)
	}
	a.must2(t, a.do(t, "POST", "/api/products", other, map[string]any{"name": "Gel", "price_cents": 100, "barcode": "750100"}))
	a.must2(t, a.do(t, "POST", "/api/products", admin, map[string]any{"name": "Sin código A", "price_cents": 100}))
	a.must2(t, a.do(t, "POST", "/api/products", admin, map[string]any{"name": "Sin código B", "price_cents": 100}))

	// se edita sin tocar lo demás, y se quita el código con vacío
	a.must2(t, a.do(t, "PATCH", path("/api/products/", pid), admin, map[string]any{"min_stock": 8}))
	got = list[product](t, a, "/api/products", admin)[0]
	if got.MinStock != 8 || got.Barcode != "750100" || got.Name != "Cera" {
		t.Fatalf("editar mínimo: %+v", got)
	}
	a.must2(t, a.do(t, "PATCH", path("/api/products/", pid), admin, map[string]any{"barcode": ""}))
	if got = list[product](t, a, "/api/products", admin)[0]; got.Barcode != "" || got.MinStock != 8 {
		t.Fatalf("quitar código: %+v", got)
	}
	if r := a.do(t, "PATCH", path("/api/products/", pid), admin, map[string]any{"min_stock": -2}); r.Status != 400 {
		t.Fatalf("mínimo negativo: %d", r.Status)
	}
}

func (a *API) must2(t *testing.T, r resp) {
	t.Helper()
	if r.Status >= 300 {
		t.Fatalf("petición falló: %d %v", r.Status, r.Body)
	}
}
