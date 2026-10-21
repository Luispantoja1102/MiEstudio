package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"miestudio/backend-go/internal/auth"
)

// doCookie hace una petición como lo haría el navegador: con la cookie y, si se pide, el encabezado anti-CSRF.
func (a *API) doCookie(t *testing.T, method, path string, cookie *http.Cookie, csrf bool, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if csrf {
		req.Header.Set(auth.CSRFHeader, auth.CSRFValue)
	}
	rec := httptest.NewRecorder()
	a.Routes().ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	return nil
}

func loginCookie(t *testing.T, a *API, email, pass string) *http.Cookie {
	t.Helper()
	rec, _ := a.doCookie(t, "POST", "/api/auth/login", nil, true, map[string]any{"email": email, "password": pass})
	c := sessionCookie(rec)
	if rec.Code != 200 || c == nil {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	return c
}

func TestLoginSetsHardenedCookie(t *testing.T) {
	a := newTestAPI(t)
	a.mustRegister(t, "Barbería", "a@x.com")
	rec, _ := a.doCookie(t, "POST", "/api/auth/login", nil, true, map[string]any{"email": "a@x.com", "password": "supersecreta1"})
	c := sessionCookie(rec)
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != auth.SessionHours*3600 {
		t.Fatalf("cookie: %+v", c)
	}
	if c.Secure {
		t.Fatal("en http local no debe ser Secure")
	}
	a.Production = true
	rec, _ = a.doCookie(t, "POST", "/api/auth/login", nil, true, map[string]any{"email": "a@x.com", "password": "supersecreta1"})
	if c := sessionCookie(rec); c == nil || !c.Secure {
		t.Fatal("en producción la cookie debe ser Secure")
	}
}

func TestCookieAuthNeedsCSRFHeaderOnWrites(t *testing.T) {
	a := newTestAPI(t)
	a.mustRegister(t, "Barbería", "a@x.com")
	c := loginCookie(t, a, "a@x.com", "supersecreta1")
	if rec, _ := a.doCookie(t, "GET", "/api/me", c, false, nil); rec.Code != 200 {
		t.Fatalf("lectura con cookie: %d", rec.Code)
	}
	// una escritura sin el encabezado (como la de un formulario de otro sitio) se rechaza
	rec, body := a.doCookie(t, "POST", "/api/services", c, false, map[string]any{"name": "Corte", "duration_min": 30, "price_cents": 100})
	if rec.Code != 403 || body["code"] != "csrf" {
		t.Fatalf("sin encabezado: %d %v", rec.Code, body)
	}
	if rec, _ := a.doCookie(t, "POST", "/api/services", c, true, map[string]any{"name": "Corte", "duration_min": 30, "price_cents": 100}); rec.Code != 201 {
		t.Fatalf("con encabezado: %d", rec.Code)
	}
	// con Bearer (herramientas) no se exige: no hay cookie que un tercero pueda aprovechar
	tok := a.doLogin(t, "a@x.com", "supersecreta1").Body["token"].(string)
	if r := a.do(t, "POST", "/api/services", tok, map[string]any{"name": "Barba", "duration_min": 20, "price_cents": 100}); r.Status != 201 {
		t.Fatalf("bearer: %d", r.Status)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	a := newTestAPI(t)
	a.mustRegister(t, "Barbería", "a@x.com")
	rec, _ := a.doCookie(t, "POST", "/api/auth/logout", nil, true, nil)
	c := sessionCookie(rec)
	if rec.Code != 204 || c == nil || c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("logout: %d %+v", rec.Code, c)
	}
}

func TestPasswordChangeClosesOtherSessionsButKeepsThisOne(t *testing.T) {
	a := newTestAPI(t)
	a.mustRegister(t, "Barbería", "a@x.com")
	phone := loginCookie(t, a, "a@x.com", "supersecreta1")
	laptop := loginCookie(t, a, "a@x.com", "supersecreta1")
	rec, _ := a.doCookie(t, "POST", "/api/auth/password", laptop, true, map[string]any{"current": "supersecreta1", "new": "otraclave12345"})
	if rec.Code != 204 {
		t.Fatalf("cambiar clave: %d", rec.Code)
	}
	if rec, _ := a.doCookie(t, "GET", "/api/me", phone, false, nil); rec.Code != 401 {
		t.Fatalf("la sesión del otro dispositivo debe cerrarse: %d", rec.Code)
	}
	if rec, _ := a.doCookie(t, "GET", "/api/me", laptop, false, nil); rec.Code != 401 {
		t.Fatalf("la cookie vieja de este dispositivo también quedó invalidada: %d", rec.Code)
	}
	renewed := sessionCookie(rec)
	if renewed == nil {
		t.Fatal("el cambio debe renovar la cookie de quien lo hizo")
	}
	if rec, _ := a.doCookie(t, "GET", "/api/me", renewed, false, nil); rec.Code != 200 {
		t.Fatalf("la cookie renovada debe servir: %d", rec.Code)
	}
}

func TestRoleChangeAndDeactivationCloseSessions(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "Barbería", "a@x.com")
	mid := a.addMember(t, admin, "m@x.com", "reception")
	c := loginCookie(t, a, "m@x.com", "supersecreta1")
	if rec, _ := a.doCookie(t, "GET", "/api/me", c, false, nil); rec.Code != 200 {
		t.Fatal("sesión inicial")
	}
	if r := a.do(t, "PATCH", path("/api/team/", mid), admin, map[string]any{"role": "cashier"}); r.Status >= 300 {
		t.Fatalf("cambiar rol: %d %v", r.Status, r.Body)
	}
	if rec, _ := a.doCookie(t, "GET", "/api/me", c, false, nil); rec.Code != 401 {
		t.Fatalf("cambiar el rol cierra la sesión: %d", rec.Code)
	}
	c = loginCookie(t, a, "m@x.com", "supersecreta1")
	a.do(t, "PATCH", path("/api/team/", mid), admin, map[string]any{"active": false})
	if rec, _ := a.doCookie(t, "GET", "/api/me", c, false, nil); rec.Code != 401 {
		t.Fatalf("la baja cierra la sesión: %d", rec.Code)
	}
}

func TestLogoutAllInvalidatesEverySession(t *testing.T) {
	a := newTestAPI(t)
	a.mustRegister(t, "Barbería", "a@x.com")
	one := loginCookie(t, a, "a@x.com", "supersecreta1")
	two := loginCookie(t, a, "a@x.com", "supersecreta1")
	if rec, _ := a.doCookie(t, "POST", "/api/auth/logout-all", one, true, nil); rec.Code != 204 {
		t.Fatalf("logout-all: %d", rec.Code)
	}
	for _, c := range []*http.Cookie{one, two} {
		if rec, _ := a.doCookie(t, "GET", "/api/me", c, false, nil); rec.Code != 401 {
			t.Fatalf("sesión viva tras cerrar todas: %d", rec.Code)
		}
	}
	// entrar de nuevo funciona
	loginCookie(t, a, "a@x.com", "supersecreta1")
}

func TestLegacyTokenWithoutVersionStillWorksUntilFirstBump(t *testing.T) {
	a := newTestAPI(t)
	a.mustRegister(t, "Barbería", "a@x.com")
	tok, _ := auth.Sign(a.Secret, 1, 0)
	if r := a.do(t, "GET", "/api/me", tok, nil); r.Status != 200 {
		t.Fatalf("versión 0: %d", r.Status)
	}
	if r := a.do(t, "GET", "/api/me", strings.Replace(tok, "a", "b", 1), nil); r.Status == 200 {
		t.Fatal("un token alterado no debe servir")
	}
	badVersion, _ := auth.Sign(a.Secret, 1, 5)
	if r := a.do(t, "GET", "/api/me", badVersion, nil); r.Status != 401 {
		t.Fatalf("versión que no corresponde: %d", r.Status)
	}
}
