package api

import (
	"errors"
	"regexp"
	"testing"
	"time"
)

var sixDigits = regexp.MustCompile(`\b(\d{6})\b`)

func newSignup(t *testing.T) (*API, *fakeMailer) {
	t.Helper()
	a := newTestAPI(t)
	fm := &fakeMailer{}
	a.Mailer = fm
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	a.Now = func() time.Time { return now }
	return a, fm
}

func advance(a *API, d time.Duration) { t0 := a.now(); a.Now = func() time.Time { return t0.Add(d) } }

func signupBody(email, code string) map[string]any {
	b := map[string]any{"business_name": "Barbería", "kind": "barberia", "name": "Dueño", "email": email, "password": "supersecreta1", "accept_terms": true}
	if code != "" {
		b["email_code"] = code
	}
	return b
}

func codeSent(t *testing.T, fm *fakeMailer) string {
	t.Helper()
	m := sixDigits.FindStringSubmatch(fm.last().Text)
	if m == nil {
		t.Fatalf("el correo no trae código:\n%s", fm.last().Text)
	}
	return m[1]
}

func TestSignupWithoutMailerDoesNotAskForCode(t *testing.T) {
	a := newTestAPI(t)
	if r := a.do(t, "GET", "/api/auth/registration", "", nil); r.Body["email_verification"] != false {
		t.Fatalf("sin correo no se exige verificación: %v", r.Body)
	}
	a.mustRegister(t, "Barbería", "a@x.com")
}

func TestSignupAsksForTheEmailCodeAndSpendsIt(t *testing.T) {
	a, fm := newSignup(t)
	if r := a.do(t, "GET", "/api/auth/registration", "", nil); r.Body["email_verification"] != true {
		t.Fatalf("con correo se exige verificación: %v", r.Body)
	}
	// sin código no se crea la cuenta
	if r := a.do(t, "POST", "/api/auth/register", "", signupBody("nuevo@x.com", "")); r.Status != 400 || r.Body["code"] != "email_code_required" {
		t.Fatalf("sin código: %d %v", r.Status, r.Body)
	}
	if r := a.do(t, "POST", "/api/auth/register", "", signupBody("nuevo@x.com", "123456")); r.Status != 400 || r.Body["code"] != "email_code_expired" {
		t.Fatalf("código que nunca se pidió: %d %v", r.Status, r.Body)
	}
	if r := a.do(t, "POST", "/api/auth/register/code", "", map[string]any{"email": "no-es-correo"}); r.Status != 400 {
		t.Fatalf("correo inválido: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/auth/register/code", "", map[string]any{"email": "Nuevo@X.com"}); r.Status != 200 || r.Body["required"] != true {
		t.Fatalf("pedir código: %d %v", r.Status, r.Body)
	}
	code := codeSent(t, fm)
	if fm.last().To != "nuevo@x.com" {
		t.Fatalf("el código va al correo escrito: %s", fm.last().To)
	}
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	if r := a.do(t, "POST", "/api/auth/register", "", signupBody("nuevo@x.com", wrong)); r.Status != 400 || r.Body["code"] != "email_code_wrong" {
		t.Fatalf("código equivocado: %d %v", r.Status, r.Body)
	}
	// el código de otro correo no sirve
	a.do(t, "POST", "/api/auth/register/code", "", map[string]any{"email": "otro@x.com"})
	if r := a.do(t, "POST", "/api/auth/register", "", signupBody("nuevo@x.com", codeSent(t, fm))); r.Status != 400 {
		t.Fatalf("código de otro correo: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/auth/register", "", signupBody("nuevo@x.com", code)); r.Status != 201 {
		t.Fatalf("con el código correcto: %d %v", r.Status, r.Body)
	}
	// se gasta: no vale una segunda vez
	if r := a.do(t, "POST", "/api/auth/register", "", signupBody("nuevo@x.com", code)); r.Status != 400 || r.Body["code"] != "email_code_expired" {
		t.Fatalf("reusar el código: %d %v", r.Status, r.Body)
	}
	// y ya no se manda código a un correo registrado
	advance(a, 2*time.Minute)
	if r := a.do(t, "POST", "/api/auth/register/code", "", map[string]any{"email": "nuevo@x.com"}); r.Status != 409 {
		t.Fatalf("correo ya registrado: %d", r.Status)
	}
}

func TestSignupCodeCooldownHourlyCapExpiryAndTries(t *testing.T) {
	a, fm := newSignup(t)
	ask := func() resp { return a.do(t, "POST", "/api/auth/register/code", "", map[string]any{"email": "z@x.com"}) }
	if r := ask(); r.Status != 200 {
		t.Fatalf("primer código: %d", r.Status)
	}
	if r := ask(); r.Status != 429 {
		t.Fatalf("de inmediato debe esperar: %d", r.Status)
	}
	// tope por hora: 5 envíos
	for i := 2; i <= 5; i++ {
		advance(a, time.Duration(i)*70*time.Second)
		if r := ask(); r.Status != 200 {
			t.Fatalf("envío %d: %d %v", i, r.Status, r.Body)
		}
	}
	advance(a, 6*70*time.Second)
	if r := ask(); r.Status != 429 {
		t.Fatalf("el sexto envío en una hora: %d", r.Status)
	}
	// pasada la hora vuelve a poder
	advance(a, 2*time.Hour)
	if r := ask(); r.Status != 200 {
		t.Fatalf("tras una hora: %d", r.Status)
	}
	code := codeSent(t, fm)
	// vence a los 10 minutos
	advance(a, 11*time.Minute)
	if r := a.do(t, "POST", "/api/auth/register", "", signupBody("z@x.com", code)); r.Body["code"] != "email_code_expired" {
		t.Fatalf("código vencido: %d %v", r.Status, r.Body)
	}
	// 5 intentos malos lo anulan aunque después se escriba bien
	advance(a, 20*time.Minute)
	ask()
	good := codeSent(t, fm)
	bad := "000000"
	if good == bad {
		bad = "111111"
	}
	for i := 0; i < 5; i++ {
		a.do(t, "POST", "/api/auth/register", "", signupBody("z@x.com", bad))
	}
	if r := a.do(t, "POST", "/api/auth/register", "", signupBody("z@x.com", good)); r.Status != 400 {
		t.Fatalf("tras 5 intentos malos el código ya no sirve: %d", r.Status)
	}
}

func TestSignupCodeSendFailureAndOptOut(t *testing.T) {
	a, fm := newSignup(t)
	fm.fail = errors.New("smtp caído")
	if r := a.do(t, "POST", "/api/auth/register/code", "", map[string]any{"email": "f@x.com"}); r.Status != 502 {
		t.Fatalf("falla el envío: %d", r.Status)
	}
	fm.fail = nil
	// la falla no deja un código a medias ni bloquea el siguiente intento
	if r := a.do(t, "POST", "/api/auth/register/code", "", map[string]any{"email": "f@x.com"}); r.Status != 200 {
		t.Fatalf("reintento tras falla: %d", r.Status)
	}
	// SIGNUP_VERIFY=off: se registra sin código aunque haya correo
	a.SignupVerifyOff = true
	if r := a.do(t, "POST", "/api/auth/register", "", signupBody("sin-codigo@x.com", "")); r.Status != 201 {
		t.Fatalf("verificación apagada: %d %v", r.Status, r.Body)
	}
	if r := a.do(t, "GET", "/api/auth/registration", "", nil); r.Body["email_verification"] != false {
		t.Fatalf("apagada: %v", r.Body)
	}
}
