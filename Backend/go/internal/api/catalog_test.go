package api

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func id(r resp) int64 { return int64(r.Body["id"].(float64)) }

func path(parts ...any) string {
	var b strings.Builder
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			b.WriteString(v)
		case int64:
			b.WriteString(strconv.FormatInt(v, 10))
		}
	}
	return b.String()
}

func list[T any](t *testing.T, a *API, url, token string) []T {
	t.Helper()
	r := a.do(t, "GET", url, token, nil)
	if r.Status != 200 {
		t.Fatalf("GET %s: %d", url, r.Status)
	}
	var out []T
	if err := json.Unmarshal(r.Raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func login(t *testing.T, a *API, email string) string {
	return a.doLogin(t, email, "supersecreta1").Body["token"].(string)
}

func TestServicesRolesAndValidation(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "N", "admin@x.com")
	a.addMember(t, admin, "staff@x.com", "staff")
	staff := login(t, a, "staff@x.com")

	good := map[string]any{"name": "Corte", "duration_min": 30, "buffer_min": 5, "price_cents": 15000}
	if r := a.do(t, "POST", "/api/services", staff, good); r.Status != 403 {
		t.Fatalf("staff creando servicio: esperaba 403, obtuvo %d", r.Status)
	}
	for i, bad := range []map[string]any{
		{"name": "", "duration_min": 30, "price_cents": 100},
		{"name": "X", "duration_min": 0, "price_cents": 100},
		{"name": "X", "duration_min": 30, "price_cents": -1},
		{"name": "X", "duration_min": 30, "buffer_min": -5, "price_cents": 100},
	} {
		if r := a.do(t, "POST", "/api/services", admin, bad); r.Status != 400 {
			t.Errorf("inválido %d: esperaba 400, obtuvo %d", i, r.Status)
		}
	}
	sid := id(a.do(t, "POST", "/api/services", admin, good))

	// el equipo puede leer el catálogo
	if got := list[service](t, a, "/api/services", staff); len(got) != 1 || got[0].PriceCents != 15000 {
		t.Fatalf("lista: %+v", got)
	}
	// cambio parcial y desactivar
	if r := a.do(t, "PATCH", path("/api/services/", sid), admin, map[string]any{"price_cents": 18000, "active": false}); r.Status != 204 {
		t.Fatalf("patch: %d", r.Status)
	}
	got := list[service](t, a, "/api/services", admin)[0]
	if got.PriceCents != 18000 || got.Active || got.Name != "Corte" || got.DurationMin != 30 {
		t.Fatalf("patch parcial dañó otros campos: %+v", got)
	}
}

func TestServiceStaffAssignment(t *testing.T) {
	a := newTestAPI(t)
	adminA := a.mustRegister(t, "A", "a@x.com")
	adminB := a.mustRegister(t, "B", "b@x.com")
	own := a.addMember(t, adminA, "staff@x.com", "staff")
	foreign := a.addMember(t, adminB, "staffb@x.com", "staff")
	sid := id(a.do(t, "POST", "/api/services", adminA, map[string]any{"name": "Corte", "duration_min": 30, "price_cents": 100}))

	url := path("/api/services/", sid, "/staff")
	if r := a.do(t, "PUT", url, adminA, map[string]any{"user_ids": []int64{own, own}}); r.Status != 204 {
		t.Fatalf("asignar propio (con duplicado): %d", r.Status)
	}
	if got := list[service](t, a, "/api/services", adminA)[0]; len(got.StaffIDs) != 1 || got.StaffIDs[0] != own {
		t.Fatalf("staff_ids: %v", got.StaffIDs)
	}
	if r := a.do(t, "PUT", url, adminA, map[string]any{"user_ids": []int64{own, foreign}}); r.Status != 400 {
		t.Fatalf("asignar persona de otro negocio: esperaba 400, obtuvo %d", r.Status)
	}
	if got := list[service](t, a, "/api/services", adminA)[0]; len(got.StaffIDs) != 1 {
		t.Fatal("la asignación inválida modificó los datos")
	}
	if r := a.do(t, "PUT", url, adminB, map[string]any{"user_ids": []int64{foreign}}); r.Status != 404 {
		t.Fatalf("B tocando servicio de A: esperaba 404, obtuvo %d", r.Status)
	}
	if r := a.do(t, "PUT", url, adminA, map[string]any{"user_ids": []int64{}}); r.Status != 204 {
		t.Fatalf("vaciar lista: %d", r.Status)
	}
}

func TestCatalogIsolationBetweenBusinesses(t *testing.T) {
	a := newTestAPI(t)
	adminA := a.mustRegister(t, "A", "a@x.com")
	adminB := a.mustRegister(t, "B", "b@x.com")

	svc := id(a.do(t, "POST", "/api/services", adminA, map[string]any{"name": "Corte", "duration_min": 30, "price_cents": 100}))
	prod := id(a.do(t, "POST", "/api/products", adminA, map[string]any{"name": "Cera", "price_cents": 100, "stock": 3}))
	var branchA int64
	for _, b := range list[branch](t, a, "/api/branches", adminA) {
		branchA = b.ID
	}

	for _, url := range []string{"/api/services", "/api/products"} {
		if r := a.do(t, "GET", url, adminB, nil); string(r.Raw) != "[]\n" {
			t.Errorf("B ve datos de A en %s: %s", url, r.Raw)
		}
	}
	for _, c := range []struct{ url string }{
		{path("/api/services/", svc)}, {path("/api/products/", prod)}, {path("/api/branches/", branchA)},
	} {
		if r := a.do(t, "PATCH", c.url, adminB, map[string]any{"active": false}); r.Status != 404 {
			t.Errorf("B modificando %s: esperaba 404, obtuvo %d", c.url, r.Status)
		}
	}
	if !list[service](t, a, "/api/services", adminA)[0].Active || !list[product](t, a, "/api/products", adminA)[0].Active {
		t.Fatal("datos de A fueron modificados por B")
	}
}

func TestProductsValidation(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "N", "a@x.com")
	for i, bad := range []map[string]any{
		{"name": "X", "price_cents": 100, "stock": -1},
		{"name": "", "price_cents": 100},
		{"name": "X", "price_cents": -5},
	} {
		if r := a.do(t, "POST", "/api/products", admin, bad); r.Status != 400 {
			t.Errorf("inválido %d: esperaba 400, obtuvo %d", i, r.Status)
		}
	}
	pid := id(a.do(t, "POST", "/api/products", admin, map[string]any{"name": "Cera", "price_cents": 100, "cost_cents": 40, "stock": 5}))
	// la existencia solo cambia por el libro de inventario: el PATCH la ignora
	a.do(t, "PATCH", path("/api/products/", pid), admin, map[string]any{"stock": 99, "price_cents": 120})
	got := list[product](t, a, "/api/products", admin)[0]
	if got.Stock != 5 || got.PriceCents != 120 || got.Name != "Cera" || got.CostCents == nil || *got.CostCents != 40 {
		t.Fatalf("%+v", got)
	}
	if r := a.do(t, "PATCH", path("/api/products/", pid), admin, map[string]any{"cost_cents": -1}); r.Status != 400 {
		t.Fatalf("costo negativo: %d", r.Status)
	}
}

func TestClientsSearchAndIsolation(t *testing.T) {
	a := newTestAPI(t)
	adminA := a.mustRegister(t, "A", "a@x.com")
	adminB := a.mustRegister(t, "B", "b@x.com")
	a.addMember(t, adminA, "recep@x.com", "reception")
	recep := login(t, a, "recep@x.com")

	// recepción puede dar de alta clientes
	ana := id(a.do(t, "POST", "/api/clients", recep, map[string]any{"name": "Ana López", "phone": "6641112222", "birthday": "1990-05-17"}))
	a.do(t, "POST", "/api/clients", recep, map[string]any{"name": "Beto Ruiz", "email": "BETO@mail.com"})

	if got := list[client](t, a, "/api/clients?q=lópez", adminA); len(got) != 1 || got[0].ID != ana {
		t.Fatalf("búsqueda por nombre: %+v", got)
	}
	if got := list[client](t, a, "/api/clients?q=111222", adminA); len(got) != 1 {
		t.Fatalf("búsqueda por teléfono: %+v", got)
	}
	if got := list[client](t, a, "/api/clients?q=beto@", adminA); len(got) != 1 || got[0].Email != "beto@mail.com" {
		t.Fatalf("búsqueda por correo: %+v", got)
	}
	if got := list[client](t, a, "/api/clients?q=%25", adminA); len(got) != 0 {
		t.Fatalf("el comodín %% no debe coincidir con todo: %+v", got)
	}
	if got := list[client](t, a, "/api/clients", adminA); len(got) != 2 {
		t.Fatalf("lista completa: %d", len(got))
	}
	if r := a.do(t, "GET", path("/api/clients/", ana), adminA, nil); r.Body["birthday"] != "1990-05-17" {
		t.Fatalf("cumpleaños: %v", r.Body)
	}

	// aislamiento
	if got := list[client](t, a, "/api/clients", adminB); len(got) != 0 {
		t.Fatalf("B ve clientes de A: %+v", got)
	}
	if r := a.do(t, "GET", path("/api/clients/", ana), adminB, nil); r.Status != 404 {
		t.Fatalf("B leyendo cliente de A: %d", r.Status)
	}
	if r := a.do(t, "PATCH", path("/api/clients/", ana), adminB, map[string]any{"name": "Hackeada"}); r.Status != 404 {
		t.Fatalf("B editando cliente de A: %d", r.Status)
	}

	// validaciones
	for i, bad := range []map[string]any{
		{"name": ""}, {"name": "X", "email": "mal"}, {"name": "X", "birthday": "2999-01-01"}, {"name": "X", "birthday": "17/05/1990"},
	} {
		if r := a.do(t, "POST", "/api/clients", adminA, bad); r.Status != 400 {
			t.Errorf("inválido %d: esperaba 400, obtuvo %d", i, r.Status)
		}
	}
	if r := a.do(t, "PATCH", path("/api/clients/", ana), adminA, map[string]any{"name": "Ana L.", "phone": "1"}); r.Status != 204 {
		t.Fatalf("editar: %d", r.Status)
	}
}

func TestClientNotesPermissionsAndIsolation(t *testing.T) {
	a := newTestAPI(t)
	adminA := a.mustRegister(t, "A", "a@x.com")
	adminB := a.mustRegister(t, "B", "b@x.com")
	a.addMember(t, adminA, "recep@x.com", "reception")
	a.addMember(t, adminA, "caja@x.com", "cashier")
	a.addMember(t, adminA, "staff@x.com", "staff")
	cid := id(a.do(t, "POST", "/api/clients", adminA, map[string]any{"name": "Ana"}))
	url := path("/api/clients/", cid, "/notes")

	// recepción y caja no ven ni escriben notas
	for _, email := range []string{"recep@x.com", "caja@x.com"} {
		tok := login(t, a, email)
		if r := a.do(t, "GET", url, tok, nil); r.Status != 403 {
			t.Errorf("%s leyendo notas: esperaba 403, obtuvo %d", email, r.Status)
		}
		if r := a.do(t, "POST", url, tok, map[string]any{"body": "x"}); r.Status != 403 {
			t.Errorf("%s escribiendo notas: esperaba 403, obtuvo %d", email, r.Status)
		}
	}
	staff := login(t, a, "staff@x.com")
	if r := a.do(t, "POST", url, staff, map[string]any{"body": "   "}); r.Status != 400 {
		t.Fatalf("nota vacía: %d", r.Status)
	}
	a.do(t, "POST", url, adminA, map[string]any{"body": "Alérgica al amoníaco"})
	a.do(t, "POST", url, staff, map[string]any{"body": "Fade bajo, tijera arriba"})
	notes := list[note](t, a, url, adminA)
	if len(notes) != 2 || notes[0].Body != "Fade bajo, tijera arriba" || notes[0].Author != "Miembro" {
		t.Fatalf("notas (más reciente primero): %+v", notes)
	}

	// otro negocio
	if r := a.do(t, "POST", url, adminB, map[string]any{"body": "intruso"}); r.Status != 404 {
		t.Fatalf("B escribiendo nota en cliente de A: %d", r.Status)
	}
	if got := list[note](t, a, url, adminB); len(got) != 0 {
		t.Fatalf("B lee notas de A: %+v", got)
	}
}

func TestBranches(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "N", "a@x.com")
	first := list[branch](t, a, "/api/branches", admin)[0]
	a.DB.Exec(t.Context(), `UPDATE businesses SET plan = 'pro'`) // varias sucursales son del plan Pro

	if r := a.do(t, "PATCH", path("/api/branches/", first.ID), admin, map[string]any{"active": false}); r.Status != 400 {
		t.Fatalf("desactivar la única sucursal: esperaba 400, obtuvo %d", r.Status)
	}
	second := id(a.do(t, "POST", "/api/branches", admin, map[string]any{"name": "Centro"}))
	if r := a.do(t, "PATCH", path("/api/branches/", first.ID), admin, map[string]any{"active": false}); r.Status != 204 {
		t.Fatalf("desactivar con otra activa: %d", r.Status)
	}
	if r := a.do(t, "PATCH", path("/api/branches/", second), admin, map[string]any{"active": false}); r.Status != 400 {
		t.Fatalf("desactivar la última activa: %d", r.Status)
	}
}

func TestWorkingHours(t *testing.T) {
	a := newTestAPI(t)
	adminA := a.mustRegister(t, "A", "a@x.com")
	adminB := a.mustRegister(t, "B", "b@x.com")
	staff := a.addMember(t, adminA, "staff@x.com", "staff")
	branchA := list[branch](t, a, "/api/branches", adminA)[0].ID
	branchB := list[branch](t, a, "/api/branches", adminB)[0].ID
	url := path("/api/team/", staff, "/hours")

	block := func(b int64, day int, s, e string) map[string]any {
		return map[string]any{"branch_id": b, "weekday": day, "start": s, "end": e}
	}
	set := func(token string, blocks ...map[string]any) resp {
		return a.do(t, "PUT", url, token, map[string]any{"blocks": blocks})
	}

	if r := set(adminA, block(branchA, 1, "09:00", "14:00"), block(branchA, 1, "16:00", "20:00"), block(branchA, 2, "10:00", "18:00")); r.Status != 204 {
		t.Fatalf("horario válido: %d %v", r.Status, r.Body)
	}
	got := list[hourBlock](t, a, url, adminA)
	if len(got) != 3 || got[0].Start != "09:00" || got[1].End != "20:00" {
		t.Fatalf("horario guardado: %+v", got)
	}

	for name, blocks := range map[string][]map[string]any{
		"encimado":       {block(branchA, 1, "09:00", "14:00"), block(branchA, 1, "13:00", "15:00")},
		"fin antes":      {block(branchA, 1, "14:00", "09:00")},
		"hora inválida":  {block(branchA, 1, "9am", "14:00")},
		"día inválido":   {block(branchA, 7, "09:00", "14:00")},
		"sucursal ajena": {block(branchB, 1, "09:00", "14:00")},
	} {
		if r := set(adminA, blocks...); r.Status != 400 {
			t.Errorf("%s: esperaba 400, obtuvo %d", name, r.Status)
		}
	}
	if len(list[hourBlock](t, a, url, adminA)) != 3 {
		t.Fatal("un horario inválido modificó el guardado")
	}
	if r := set(adminB, block(branchB, 1, "09:00", "14:00")); r.Status != 404 {
		t.Fatalf("B fijando horario a persona de A: %d", r.Status)
	}
	if got := list[hourBlock](t, a, url, adminB); len(got) != 0 {
		t.Fatalf("B lee horario de A: %+v", got)
	}
	if r := set(adminA); r.Status != 204 || len(list[hourBlock](t, a, url, adminA)) != 0 {
		t.Fatal("no se pudo vaciar el horario")
	}
}

func mustJSON(t *testing.T, raw []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}
