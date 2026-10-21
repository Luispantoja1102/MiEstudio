package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"miestudio/backend-go/internal/db"
	"miestudio/backend-go/internal/ratelimit"
)

const testSecret = "0123456789abcdef0123456789abcdef"

// newTestAPI crea una base temporal con las migraciones aplicadas.
// Requiere TEST_DATABASE_URL apuntando a una base con permiso para crear bases.
func newTestAPI(t *testing.T) *API {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL no definido")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	suffix := make([]byte, 4)
	rand.Read(suffix)
	name := "miestudio_test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}

	cfg, _ := pgxpool.ParseConfig(adminURL)
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		admin.Close()
	})
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return &API{DB: pool, Secret: testSecret}
}

type resp struct {
	Status int
	Body   map[string]any
	Raw    []byte
}

func (a *API) do(t *testing.T, method, path, token string, body any) resp {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.Routes().ServeHTTP(rec, req)
	out := resp{Status: rec.Code, Raw: rec.Body.Bytes()}
	json.Unmarshal(rec.Body.Bytes(), &out.Body)
	return out
}

// mustRegister crea un negocio y devuelve el token de su administrador.
func (a *API) mustRegister(t *testing.T, biz, email string) string {
	t.Helper()
	r := a.do(t, "POST", "/api/auth/register", "", map[string]any{
		"business_name": biz, "kind": "barberia", "name": "Dueño", "email": email, "password": "supersecreta1", "accept_terms": true,
	})
	if r.Status != http.StatusCreated {
		t.Fatalf("register %s: %d %v", email, r.Status, r.Body)
	}
	return r.Body["token"].(string)
}

func (a *API) addMember(t *testing.T, adminToken, email, role string) int64 {
	t.Helper()
	r := a.do(t, "POST", "/api/team", adminToken, map[string]any{
		"name": "Miembro", "email": email, "password": "supersecreta1", "role": role,
	})
	if r.Status != http.StatusCreated {
		t.Fatalf("addMember: %d %v", r.Status, r.Body)
	}
	return int64(r.Body["id"].(float64))
}

func (a *API) doLogin(t *testing.T, email, pass string) resp {
	return a.do(t, "POST", "/api/auth/login", "", map[string]any{"email": email, "password": pass})
}

func TestRegisterLoginMe(t *testing.T) {
	a := newTestAPI(t)
	a.mustRegister(t, "Barbería Don Luis", "Luis@X.com")

	r := a.doLogin(t, "luis@x.com", "supersecreta1")
	if r.Status != 200 {
		t.Fatalf("login: %d", r.Status)
	}
	me := a.do(t, "GET", "/api/me", r.Body["token"].(string), nil)
	if me.Status != 200 || me.Body["role"] != "admin" || me.Body["email"] != "luis@x.com" {
		t.Fatalf("me: %d %v", me.Status, me.Body)
	}
}

func TestRegisterValidationAndConflicts(t *testing.T) {
	a := newTestAPI(t)
	a.mustRegister(t, "Mismo Nombre", "a@x.com")

	// correo repetido -> 409 con mensaje de correo
	r := a.do(t, "POST", "/api/auth/register", "", map[string]any{
		"business_name": "Otro", "kind": "spa", "name": "X", "email": "a@x.com", "password": "supersecreta1", "accept_terms": true})
	if r.Status != 409 {
		t.Fatalf("correo repetido: %d", r.Status)
	}
	// sin aceptar los términos no hay cuenta
	if r := a.do(t, "POST", "/api/auth/register", "", map[string]any{
		"business_name": "Sin Términos", "kind": "spa", "name": "X", "email": "t@x.com", "password": "supersecreta1"}); r.Status != 400 {
		t.Fatalf("sin aceptar términos: %d", r.Status)
	}
	// mismo nombre de negocio con otro correo -> permitido (slug con sufijo)
	a.mustRegister(t, "Mismo Nombre", "b@x.com")
	var n int
	a.DB.QueryRow(context.Background(), `SELECT count(DISTINCT slug) FROM businesses`).Scan(&n)
	if n != 2 {
		t.Fatalf("slugs distintos esperados 2, hay %d", n)
	}

	bad := []map[string]any{
		{"business_name": "N", "kind": "gimnasio", "name": "X", "email": "c@x.com", "password": "supersecreta1"},
		{"business_name": "N", "kind": "spa", "name": "X", "email": "sin-arroba", "password": "supersecreta1"},
		{"business_name": "N", "kind": "spa", "name": "X", "email": "d@x.com", "password": "corta"},
		{"business_name": "  ", "kind": "spa", "name": "X", "email": "e@x.com", "password": "supersecreta1"},
	}
	for i, b := range bad {
		if r := a.do(t, "POST", "/api/auth/register", "", b); r.Status != 400 {
			t.Errorf("caso inválido %d: esperaba 400, obtuvo %d", i, r.Status)
		}
	}
}

func TestLoginFailuresAreGeneric(t *testing.T) {
	a := newTestAPI(t)
	a.mustRegister(t, "N", "a@x.com")
	wrongPass := a.doLogin(t, "a@x.com", "incorrecta1")
	noUser := a.doLogin(t, "nadie@x.com", "incorrecta1")
	if wrongPass.Status != 401 || noUser.Status != 401 || wrongPass.Body["error"] != noUser.Body["error"] {
		t.Fatalf("errores distintos: %v / %v", wrongPass, noUser)
	}
}

func TestProtectedRoutesNeedValidToken(t *testing.T) {
	a := newTestAPI(t)
	for _, tok := range []string{"", "basura", "eyJhbGciOiJub25lIn0.e30."} {
		if r := a.do(t, "GET", "/api/me", tok, nil); r.Status != 401 {
			t.Errorf("token %q: esperaba 401, obtuvo %d", tok, r.Status)
		}
	}
}

func TestOnlyAdminManagesTeam(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "N", "admin@x.com")
	a.addMember(t, admin, "staff@x.com", "staff")
	a.addMember(t, admin, "caja@x.com", "cashier")

	staff := a.doLogin(t, "staff@x.com", "supersecreta1").Body["token"].(string)
	for _, c := range []struct{ method, path string }{{"GET", "/api/team"}, {"POST", "/api/team"}, {"PATCH", "/api/team/1"}} {
		if r := a.do(t, c.method, c.path, staff, map[string]any{}); r.Status != 403 {
			t.Errorf("%s %s como staff: esperaba 403, obtuvo %d", c.method, c.path, r.Status)
		}
	}
}

func TestTeamIsolatedBetweenBusinesses(t *testing.T) {
	a := newTestAPI(t)
	adminA := a.mustRegister(t, "Negocio A", "a@x.com")
	adminB := a.mustRegister(t, "Negocio B", "b@x.com")
	memberA := a.addMember(t, adminA, "staffa@x.com", "staff")

	// B no ve el equipo de A
	req := httptest.NewRequest("GET", "/api/team", nil)
	req.Header.Set("Authorization", "Bearer "+adminB)
	rec := httptest.NewRecorder()
	a.Routes().ServeHTTP(rec, req)
	var team []member
	json.Unmarshal(rec.Body.Bytes(), &team)
	if rec.Code != 200 || len(team) != 1 || team[0].Email != "b@x.com" {
		t.Fatalf("B debe ver solo a su propio equipo: %d %+v", rec.Code, team)
	}
	// B no puede modificar a un miembro de A (404, no 204)
	patch := a.do(t, "PATCH", "/api/team/"+itoa(memberA), adminB, map[string]any{"active": false})
	if patch.Status != 404 {
		t.Fatalf("B modificando a miembro de A: esperaba 404, obtuvo %d", patch.Status)
	}
	var active bool
	a.DB.QueryRow(context.Background(), `SELECT active FROM users WHERE id = $1`, memberA).Scan(&active)
	if !active {
		t.Fatal("el miembro de A fue desactivado por B")
	}
}

func TestDeactivatedAndDemotedTakeEffectImmediately(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "N", "admin@x.com")
	id := a.addMember(t, admin, "otro@x.com", "admin")
	tok := a.doLogin(t, "otro@x.com", "supersecreta1").Body["token"].(string)

	if r := a.do(t, "GET", "/api/team", tok, nil); r.Status != 200 {
		t.Fatal("como admin debería poder")
	}
	a.do(t, "PATCH", "/api/team/"+itoa(id), admin, map[string]any{"role": "reception"})
	// bajar de rol cierra la sesión (hay que volver a entrar) y con el rol nuevo ya no tiene permiso
	if r := a.do(t, "GET", "/api/team", tok, nil); r.Status != 401 {
		t.Fatalf("tras bajar de rol: esperaba 401 (sesión cerrada), obtuvo %d", r.Status)
	}
	tok = a.doLogin(t, "otro@x.com", "supersecreta1").Body["token"].(string)
	if r := a.do(t, "GET", "/api/team", tok, nil); r.Status != 403 {
		t.Fatalf("con el rol nuevo: esperaba 403, obtuvo %d", r.Status)
	}
	a.do(t, "PATCH", "/api/team/"+itoa(id), admin, map[string]any{"active": false})
	if r := a.do(t, "GET", "/api/me", tok, nil); r.Status != 401 {
		t.Fatalf("tras desactivar: esperaba 401, obtuvo %d", r.Status)
	}
	if r := a.doLogin(t, "otro@x.com", "supersecreta1"); r.Status != 401 {
		t.Fatalf("login de usuario inactivo: esperaba 401, obtuvo %d", r.Status)
	}
}

func TestAdminCannotLockThemselvesOut(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "N", "admin@x.com")
	var id int64
	a.DB.QueryRow(context.Background(), `SELECT id FROM users WHERE email = 'admin@x.com'`).Scan(&id)
	for _, body := range []map[string]any{{"active": false}, {"role": "staff"}} {
		if r := a.do(t, "PATCH", "/api/team/"+itoa(id), admin, body); r.Status != 400 {
			t.Errorf("%v: esperaba 400, obtuvo %d", body, r.Status)
		}
	}
}

func TestChangePassword(t *testing.T) {
	a := newTestAPI(t)
	tok := a.mustRegister(t, "N", "a@x.com")

	if r := a.do(t, "POST", "/api/auth/password", tok, map[string]any{"current": "mala", "new": "nuevaclave1"}); r.Status != 401 {
		t.Fatalf("actual incorrecta: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/auth/password", tok, map[string]any{"current": "supersecreta1", "new": "corta"}); r.Status != 400 {
		t.Fatalf("nueva corta: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/auth/password", tok, map[string]any{"current": "supersecreta1", "new": "nuevaclave1"}); r.Status != 204 {
		t.Fatalf("cambio válido: %d", r.Status)
	}
	if a.doLogin(t, "a@x.com", "supersecreta1").Status != 401 || a.doLogin(t, "a@x.com", "nuevaclave1").Status != 200 {
		t.Fatal("la contraseña no cambió como se esperaba")
	}
}

func TestLoginIsNeverLockedOut(t *testing.T) {
	a := newTestAPI(t)
	a.Limiter = ratelimit.New()
	a.mustRegister(t, "N", "a@x.com")
	for i := 0; i < 60; i++ {
		if r := a.doLogin(t, "a@x.com", "incorrecta1"); r.Status != 401 {
			t.Fatalf("intento %d: esperaba 401, obtuvo %d", i, r.Status)
		}
	}
	if r := a.doLogin(t, "a@x.com", "supersecreta1"); r.Status != 200 {
		t.Fatalf("con la contraseña correcta debe entrar siempre: %d", r.Status)
	}
}
