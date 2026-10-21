package api

import (
	"context"
	"testing"
)

const platPass = "plataforma-clave-1"

// newPlatform crea el administrador de plataforma y devuelve su token.
func newPlatform(t *testing.T, a *API) string {
	t.Helper()
	if err := a.BootstrapPlatformAdmin(context.Background(), "Super", "super@plataforma.com", platPass); err != nil {
		t.Fatal(err)
	}
	r := a.doLogin(t, "super@plataforma.com", platPass)
	if r.Status != 200 {
		t.Fatalf("login de plataforma: %d", r.Status)
	}
	return r.Body["token"].(string)
}

func setMode(t *testing.T, a *API, token, mode string) {
	t.Helper()
	if r := a.do(t, "PUT", "/api/platform/settings", token, map[string]any{"registration_mode": mode}); r.Status != 204 {
		t.Fatalf("modo %s: %d %v", mode, r.Status, r.Body)
	}
}

func TestBootstrapPlatformAdminIsIdempotent(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := a.BootstrapPlatformAdmin(ctx, "S", "s@p.com", platPass); err != nil {
			t.Fatal(err)
		}
	}
	// con otro correo no crea un segundo administrador
	if err := a.BootstrapPlatformAdmin(ctx, "Otro", "otro@p.com", platPass); err != nil {
		t.Fatal(err)
	}
	var n int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'platform_admin'`).Scan(&n)
	if n != 1 {
		t.Fatalf("esperaba 1 administrador de plataforma, hay %d", n)
	}
	if err := a.BootstrapPlatformAdmin(ctx, "S", "sin-arroba", "corta"); err == nil {
		t.Fatal("debe rechazar datos inválidos")
	}
	if err := a.BootstrapPlatformAdmin(ctx, "", "", ""); err != nil {
		t.Fatal("sin configuración no debe fallar")
	}
}

func TestPlatformAndTenantAreSeparated(t *testing.T) {
	a := newTestAPI(t)
	plat := newPlatform(t, a)
	biz := a.mustRegister(t, "Negocio", "a@x.com")

	me := a.do(t, "GET", "/api/me", plat, nil)
	if me.Body["role"] != "platform_admin" || me.Body["business"] != nil {
		t.Fatalf("me de plataforma: %v", me.Body)
	}
	// la plataforma no entra a rutas de negocio
	for _, url := range []string{"/api/clients", "/api/services", "/api/team", "/api/branches"} {
		if r := a.do(t, "GET", url, plat, nil); r.Status != 403 {
			t.Errorf("plataforma en %s: esperaba 403, obtuvo %d", url, r.Status)
		}
	}
	// el negocio no entra a rutas de plataforma
	for _, url := range []string{"/api/platform/stats", "/api/platform/businesses", "/api/platform/team", "/api/platform/settings", "/api/platform/activity"} {
		if r := a.do(t, "GET", url, biz, nil); r.Status != 403 {
			t.Errorf("negocio en %s: esperaba 403, obtuvo %d", url, r.Status)
		}
		if r := a.do(t, "GET", url, "", nil); r.Status != 401 {
			t.Errorf("sin sesión en %s: esperaba 401, obtuvo %d", url, r.Status)
		}
	}
}

func TestRegistrationModes(t *testing.T) {
	a := newTestAPI(t)
	plat := newPlatform(t, a)
	reg := func(email string) resp {
		return a.do(t, "POST", "/api/auth/register", "", map[string]any{
			"business_name": "N " + email, "kind": "spa", "name": "X", "email": email, "password": "supersecreta1"})
	}

	// abierto (por defecto): prueba inmediata
	if r := a.do(t, "GET", "/api/auth/registration", "", nil); r.Body["mode"] != "open" {
		t.Fatalf("modo por defecto: %v", r.Body)
	}
	open := reg("open@x.com")
	if open.Status != 201 || open.Body["status"] != "trial" {
		t.Fatalf("registro abierto: %d %v", open.Status, open.Body)
	}
	if r := a.do(t, "GET", "/api/services", open.Body["token"].(string), nil); r.Status != 200 {
		t.Fatalf("negocio en prueba debe poder trabajar: %d", r.Status)
	}

	// cerrado: nadie se registra solo
	setMode(t, a, plat, "closed")
	if r := reg("closed@x.com"); r.Status != 403 {
		t.Fatalf("registro cerrado: esperaba 403, obtuvo %d", r.Status)
	}
	if r := a.do(t, "GET", "/api/auth/registration", "", nil); r.Body["mode"] != "closed" {
		t.Fatalf("modo público: %v", r.Body)
	}

	// con aprobación: queda pendiente y bloqueado hasta que la plataforma lo apruebe
	setMode(t, a, plat, "approval")
	pend := reg("pend@x.com")
	if pend.Status != 201 || pend.Body["status"] != "pending" {
		t.Fatalf("registro con aprobación: %d %v", pend.Status, pend.Body)
	}
	tok := pend.Body["token"].(string)
	blocked := a.do(t, "GET", "/api/services", tok, nil)
	if blocked.Status != 403 || blocked.Body["code"] != "business_pending" {
		t.Fatalf("pendiente debe bloquear: %d %v", blocked.Status, blocked.Body)
	}
	// pero puede ver su estado
	me := a.do(t, "GET", "/api/me", tok, nil)
	if me.Status != 200 || me.Body["business"].(map[string]any)["status"] != "pending" {
		t.Fatalf("me pendiente: %v", me.Body)
	}

	var bizID int64
	a.DB.QueryRow(context.Background(), `SELECT business_id FROM users WHERE email = 'pend@x.com'`).Scan(&bizID)
	if r := a.do(t, "PATCH", path("/api/platform/businesses/", bizID), plat, map[string]any{"status": "trial"}); r.Status != 204 {
		t.Fatalf("aprobar: %d", r.Status)
	}
	if r := a.do(t, "GET", "/api/services", tok, nil); r.Status != 200 {
		t.Fatalf("aprobado debe poder trabajar: %d", r.Status)
	}
	var days float64
	a.DB.QueryRow(context.Background(), `SELECT extract(day FROM trial_ends_at - now()) FROM businesses WHERE id = $1`, bizID).Scan(&days)
	if days < 13 || days > 14 {
		t.Fatalf("al aprobar debe iniciar la prueba de 14 días, quedan %v", days)
	}
}

func TestSuspendAndReactivate(t *testing.T) {
	a := newTestAPI(t)
	plat := newPlatform(t, a)
	tok := a.mustRegister(t, "N", "a@x.com")
	var bizID int64
	a.DB.QueryRow(context.Background(), `SELECT business_id FROM users WHERE email = 'a@x.com'`).Scan(&bizID)
	url := path("/api/platform/businesses/", bizID)

	a.do(t, "PATCH", url, plat, map[string]any{"status": "suspended"})
	r := a.do(t, "GET", "/api/clients", tok, nil)
	if r.Status != 403 || r.Body["code"] != "business_suspended" {
		t.Fatalf("suspendido: %d %v", r.Status, r.Body)
	}
	a.do(t, "PATCH", url, plat, map[string]any{"status": "active"})
	if r := a.do(t, "GET", "/api/clients", tok, nil); r.Status != 200 {
		t.Fatalf("reactivado: %d", r.Status)
	}
	if r := a.do(t, "PATCH", url, plat, map[string]any{"status": "inventado"}); r.Status != 400 {
		t.Fatalf("estado inválido: %d", r.Status)
	}
	if r := a.do(t, "PATCH", path("/api/platform/businesses/", int64(99999)), plat, map[string]any{"status": "active"}); r.Status != 404 {
		t.Fatalf("negocio inexistente: %d", r.Status)
	}
}

func TestSupportIsReadOnly(t *testing.T) {
	a := newTestAPI(t)
	plat := newPlatform(t, a)
	a.mustRegister(t, "N", "a@x.com")
	if r := a.do(t, "POST", "/api/platform/team", plat, map[string]any{
		"name": "Soporte", "email": "soporte@p.com", "password": platPass, "role": "platform_support"}); r.Status != 201 {
		t.Fatalf("crear soporte: %d %v", r.Status, r.Body)
	}
	sup := a.doLogin(t, "soporte@p.com", platPass).Body["token"].(string)

	if got := list[bizRow](t, a, "/api/platform/businesses", sup); len(got) != 1 {
		t.Fatalf("soporte debe poder ver negocios: %+v", got)
	}
	var bizID int64
	a.DB.QueryRow(context.Background(), `SELECT id FROM businesses LIMIT 1`).Scan(&bizID)
	for _, c := range []struct {
		method, url string
		body        any
	}{
		{"PATCH", path("/api/platform/businesses/", bizID), map[string]any{"status": "suspended"}},
		{"POST", "/api/platform/businesses", map[string]any{}},
		{"PUT", "/api/platform/settings", map[string]any{"registration_mode": "closed"}},
		{"POST", "/api/platform/team", map[string]any{}},
		{"POST", path("/api/platform/users/", int64(1), "/password"), map[string]any{"password": platPass}},
	} {
		if r := a.do(t, c.method, c.url, sup, c.body); r.Status != 403 {
			t.Errorf("soporte %s %s: esperaba 403, obtuvo %d", c.method, c.url, r.Status)
		}
	}
}

func TestPlatformCreatesBusinessAndManagesUsers(t *testing.T) {
	a := newTestAPI(t)
	plat := newPlatform(t, a)
	setMode(t, a, plat, "closed") // aunque el registro esté cerrado, la plataforma puede dar de alta

	r := a.do(t, "POST", "/api/platform/businesses", plat, map[string]any{
		"business_name": "Spa Alta Manual", "kind": "spa", "name": "Dueña", "email": "duena@x.com", "password": "claveinicial1"})
	if r.Status != 201 {
		t.Fatalf("alta manual: %d %v", r.Status, r.Body)
	}
	bizID := id(r)
	tok := a.doLogin(t, "duena@x.com", "claveinicial1").Body["token"].(string)
	if r := a.do(t, "GET", "/api/services", tok, nil); r.Status != 200 {
		t.Fatalf("negocio dado de alta debe quedar activo: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/platform/businesses", plat, map[string]any{
		"business_name": "Otro", "kind": "spa", "name": "X", "email": "duena@x.com", "password": "claveinicial1"}); r.Status != 409 {
		t.Fatalf("correo repetido: %d", r.Status)
	}

	// detalle con equipo
	detail := a.do(t, "GET", path("/api/platform/businesses/", bizID), plat, nil)
	team := detail.Body["team"].([]any)
	if detail.Status != 200 || len(team) != 1 {
		t.Fatalf("detalle: %d %v", detail.Status, detail.Body)
	}
	userID := int64(team[0].(map[string]any)["id"].(float64))

	// restablecer contraseña
	if r := a.do(t, "POST", path("/api/platform/users/", userID, "/password"), plat, map[string]any{"password": "corta"}); r.Status != 400 {
		t.Fatalf("contraseña corta: %d", r.Status)
	}
	if r := a.do(t, "POST", path("/api/platform/users/", userID, "/password"), plat, map[string]any{"password": "claveNueva99"}); r.Status != 204 {
		t.Fatalf("reset: %d", r.Status)
	}
	if a.doLogin(t, "duena@x.com", "claveinicial1").Status != 401 || a.doLogin(t, "duena@x.com", "claveNueva99").Status != 200 {
		t.Fatal("la contraseña no cambió como se esperaba")
	}
	// desactivar a la persona: su sesión deja de valer
	a.do(t, "PATCH", path("/api/platform/users/", userID), plat, map[string]any{"active": false})
	if r := a.do(t, "GET", "/api/me", tok, nil); r.Status != 401 {
		t.Fatalf("persona desactivada: %d", r.Status)
	}
	// no se pueden tocar usuarios de plataforma por esta vía
	var platID int64
	a.DB.QueryRow(context.Background(), `SELECT id FROM users WHERE role = 'platform_admin'`).Scan(&platID)
	if r := a.do(t, "POST", path("/api/platform/users/", platID, "/password"), plat, map[string]any{"password": "claveNueva99"}); r.Status != 404 {
		t.Fatalf("usuario de plataforma por vía de negocios: %d", r.Status)
	}
}

func TestPlatformListFilters(t *testing.T) {
	a := newTestAPI(t)
	plat := newPlatform(t, a)
	a.mustRegister(t, "Barbería Norte", "norte@x.com")
	a.mustRegister(t, "Spa Sur", "sur@x.com")
	var bizID int64
	a.DB.QueryRow(context.Background(), `SELECT business_id FROM users WHERE email = 'sur@x.com'`).Scan(&bizID)
	a.do(t, "PATCH", path("/api/platform/businesses/", bizID), plat, map[string]any{"status": "suspended"})

	check := func(url string, want int) {
		t.Helper()
		if got := list[bizRow](t, a, url, plat); len(got) != want {
			t.Errorf("%s: esperaba %d, obtuvo %d", url, want, len(got))
		}
	}
	check("/api/platform/businesses", 2)
	check("/api/platform/businesses?status=suspended", 1)
	check("/api/platform/businesses?status=trial", 1)
	check("/api/platform/businesses?q=norte", 2-1)
	check("/api/platform/businesses?q=sur@x", 1) // por correo del dueño
	check("/api/platform/businesses?q=%25", 0)

	st := a.do(t, "GET", "/api/platform/stats", plat, nil)
	counts := st.Body["businesses"].(map[string]any)
	if st.Body["total"].(float64) != 2 || counts["suspended"].(float64) != 1 || counts["trial"].(float64) != 1 {
		t.Fatalf("stats: %v", st.Body)
	}
}

func TestPlatformTeamGuardsAndActivity(t *testing.T) {
	a := newTestAPI(t)
	plat := newPlatform(t, a)
	var self int64
	a.DB.QueryRow(context.Background(), `SELECT id FROM users WHERE role = 'platform_admin'`).Scan(&self)

	for _, body := range []map[string]any{{"role": "platform_support"}, {"active": false}} {
		if r := a.do(t, "PATCH", path("/api/platform/team/", self), plat, body); r.Status != 400 {
			t.Errorf("auto-cambio %v: esperaba 400, obtuvo %d", body, r.Status)
		}
	}
	if r := a.do(t, "POST", "/api/platform/team", plat, map[string]any{"name": "X", "email": "x@p.com", "password": platPass, "role": "admin"}); r.Status != 400 {
		t.Fatalf("rol de negocio en equipo de plataforma: %d", r.Status)
	}
	// la bitácora registra lo que hizo la plataforma y los registros de negocios
	a.mustRegister(t, "N", "a@x.com")
	setMode(t, a, plat, "approval")
	log := list[activityRow](t, a, "/api/platform/activity", plat)
	seen := map[string]bool{}
	for _, l := range log {
		seen[l.Action] = true
	}
	if !seen["business_registered"] || !seen["settings_changed"] {
		t.Fatalf("bitácora incompleta: %+v", seen)
	}
}

func TestUpsertPlatformAdmin(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()

	created, err := a.UpsertPlatformAdmin(ctx, "Yo", "Yo@Plataforma.com", "clave-de-prueba-1")
	if err != nil || !created {
		t.Fatalf("crear: %v %v", created, err)
	}
	if a.doLogin(t, "yo@plataforma.com", "clave-de-prueba-1").Status != 200 {
		t.Fatal("no puede entrar")
	}
	// existente: restablece la contraseña y reactiva
	a.DB.Exec(ctx, `UPDATE users SET active = false, role = 'platform_support' WHERE email = 'yo@plataforma.com'`)
	created, err = a.UpsertPlatformAdmin(ctx, "", "yo@plataforma.com", "otra-clave-nueva-2")
	if err != nil || created {
		t.Fatalf("restablecer: %v %v", created, err)
	}
	r := a.doLogin(t, "yo@plataforma.com", "otra-clave-nueva-2")
	if r.Status != 200 || a.doLogin(t, "yo@plataforma.com", "clave-de-prueba-1").Status != 401 {
		t.Fatal("la contraseña no se restableció")
	}
	if me := a.do(t, "GET", "/api/me", r.Body["token"].(string), nil); me.Body["role"] != "platform_admin" {
		t.Fatalf("debe quedar como administrador: %v", me.Body)
	}
	// no pisa a personas de un negocio
	a.mustRegister(t, "N", "negocio@x.com")
	if _, err := a.UpsertPlatformAdmin(ctx, "X", "negocio@x.com", "clave-de-prueba-1"); err == nil {
		t.Fatal("no debe tomar el correo de una persona de un negocio")
	}
	if a.doLogin(t, "negocio@x.com", "supersecreta1").Status != 200 {
		t.Fatal("la persona del negocio perdió su contraseña")
	}
	// validaciones y clave de ejemplo en producción
	if _, err := a.UpsertPlatformAdmin(ctx, "X", "sin-arroba", "clave-de-prueba-1"); err == nil {
		t.Fatal("correo inválido")
	}
	if _, err := a.UpsertPlatformAdmin(ctx, "X", "z@p.com", "corta"); err == nil {
		t.Fatal("contraseña corta")
	}
	a.Production = true
	if _, err := a.UpsertPlatformAdmin(ctx, "X", "z@p.com", ExampleAdminPassword); err == nil {
		t.Fatal("la clave de ejemplo debe rechazarse en producción")
	}
	if err := a.BootstrapPlatformAdmin(ctx, "X", "w@p.com", ExampleAdminPassword); err == nil {
		t.Fatal("el arranque debe rechazar la clave de ejemplo en producción")
	}
	a.Production = false
	if _, err := a.UpsertPlatformAdmin(ctx, "X", "z@p.com", ExampleAdminPassword); err != nil {
		t.Fatalf("en desarrollo la clave de ejemplo es válida: %v", err)
	}
}

func TestBootstrapDemoBusiness(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	if err := a.BootstrapDemoBusiness(ctx, "", "", "", "", ""); err != nil {
		t.Fatal(err) // sin datos: no hace nada
	}
	var n int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM businesses`).Scan(&n)
	if n != 0 {
		t.Fatal("sin variables no debe crear nada")
	}
	if err := a.BootstrapDemoBusiness(ctx, "Barbería Demo", "barberia", "Dueño", "demo@x.com", "NegocioDemo123"); err != nil {
		t.Fatal(err)
	}
	if r := a.doLogin(t, "demo@x.com", "NegocioDemo123"); r.Status != 200 {
		t.Fatalf("entrar al negocio de ejemplo: %d", r.Status)
	}
	var status string
	a.DB.QueryRow(ctx, `SELECT status FROM businesses`).Scan(&status)
	if status != "active" {
		t.Fatalf("estado %s", status)
	}
	// ya hay un negocio: no se crea otro aunque cambien los datos
	if err := a.BootstrapDemoBusiness(ctx, "Otro", "barberia", "X", "otro@x.com", "NegocioDemo123"); err != nil {
		t.Fatal(err)
	}
	a.DB.QueryRow(ctx, `SELECT count(*) FROM businesses`).Scan(&n)
	if n != 1 {
		t.Fatalf("negocios: %d", n)
	}
	// producción rechaza la clave de ejemplo; datos inválidos informan el motivo
	b := newTestAPI(t)
	b.Production = true
	if err := b.BootstrapDemoBusiness(ctx, "N", "barberia", "X", "a@x.com", ExampleDemoPassword); err == nil {
		t.Fatal("producción no acepta la clave de ejemplo")
	}
	if err := b.BootstrapDemoBusiness(ctx, "N", "giro-raro", "X", "a@x.com", "ClaveLarga123"); err == nil {
		t.Fatal("giro inválido debe informarse")
	}
}
