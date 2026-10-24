package api

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"miestudio/backend-go/internal/ratelimit"
)

var resetLink = regexp.MustCompile(`/restablecer\?token=([A-Za-z0-9_-]+)`)

func waitMail(t *testing.T, fm *fakeMailer, n int) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if fm.count() >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("esperaba %d correos, hay %d", n, fm.count())
}

func newReset(t *testing.T) (*API, *fakeMailer) {
	a := newTestAPI(t)
	fm := &fakeMailer{}
	a.mustRegister(t, "Barbería", "a@x.com") // se registra antes de instalar el correo de prueba: la bienvenida no cuenta aquí
	a.Mailer, a.AppURL = fm, "https://app.test"
	return a, fm
}

func tokenOf(t *testing.T, fm *fakeMailer) string {
	t.Helper()
	m := resetLink.FindStringSubmatch(fm.last().Text)
	if m == nil {
		t.Fatalf("el correo no trae enlace:\n%s", fm.last().Text)
	}
	return m[1]
}

func TestForgotIsIdenticalForKnownAndUnknownAccounts(t *testing.T) {
	a, fm := newReset(t)
	known := a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "a@x.com"})
	unknown := a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "nadie@x.com"})
	if known.Status != 200 || unknown.Status != 200 || string(known.Raw) != string(unknown.Raw) {
		t.Fatalf("las respuestas deben ser idénticas: %d %s / %d %s", known.Status, known.Raw, unknown.Status, unknown.Raw)
	}
	waitMail(t, fm, 1)
	time.Sleep(100 * time.Millisecond)
	if fm.count() != 1 || fm.last().To != "a@x.com" {
		t.Fatalf("solo la cuenta real recibe correo: %d", fm.count())
	}
	if !strings.Contains(fm.last().Text, "https://app.test/restablecer?token=") {
		t.Fatal("falta el enlace")
	}
	if r := a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "no-es-correo"}); r.Status != 400 {
		t.Fatalf("correo inválido: %d", r.Status)
	}
}

func TestResetFullFlowClosesSessionsAndIsSingleUse(t *testing.T) {
	a, fm := newReset(t)
	cookie := loginCookie(t, a, "a@x.com", "supersecreta1")
	a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "a@x.com"})
	waitMail(t, fm, 1)
	token := tokenOf(t, fm)

	if r := a.do(t, "POST", "/api/auth/reset", "", map[string]any{"token": token, "password": "corta"}); r.Status != 400 {
		t.Fatalf("clave débil: %d", r.Status)
	}
	// la clave débil no gasta el enlace
	if r := a.do(t, "POST", "/api/auth/reset", "", map[string]any{"token": token, "password": "nuevaclave123"}); r.Status != 204 {
		t.Fatalf("restablecer: %d %v", r.Status, r.Body)
	}
	if r := a.doLogin(t, "a@x.com", "nuevaclave123"); r.Status != 200 {
		t.Fatalf("entrar con la clave nueva: %d", r.Status)
	}
	if r := a.doLogin(t, "a@x.com", "supersecreta1"); r.Status == 200 {
		t.Fatal("la clave vieja no debe servir")
	}
	if rec, _ := a.doCookie(t, "GET", "/api/me", cookie, false, nil); rec.Code != 401 {
		t.Fatalf("las sesiones abiertas deben cerrarse: %d", rec.Code)
	}
	// un solo uso
	if r := a.do(t, "POST", "/api/auth/reset", "", map[string]any{"token": token, "password": "otraclave12345"}); r.Status != 400 || r.Body["code"] != "reset_invalid" {
		t.Fatalf("reuso: %d %v", r.Status, r.Body)
	}
}

func TestResetTokenExpiresAfterAnHour(t *testing.T) {
	a, fm := newReset(t)
	a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "a@x.com"})
	waitMail(t, fm, 1)
	token := tokenOf(t, fm)
	base := a.now()
	a.Now = func() time.Time { return base.Add(61 * time.Minute) }
	if r := a.do(t, "POST", "/api/auth/reset", "", map[string]any{"token": token, "password": "nuevaclave123"}); r.Status != 400 {
		t.Fatalf("vencido: %d", r.Status)
	}
	a.Now = func() time.Time { return base.Add(59 * time.Minute) }
	if r := a.do(t, "POST", "/api/auth/reset", "", map[string]any{"token": token, "password": "nuevaclave123"}); r.Status != 204 {
		t.Fatalf("a los 59 min aún vale: %d", r.Status)
	}
}

func TestNewRequestInvalidatesPreviousLinkAndTokensAreHashed(t *testing.T) {
	a, fm := newReset(t)
	a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "a@x.com"})
	waitMail(t, fm, 1)
	first := tokenOf(t, fm)
	a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "a@x.com"})
	waitMail(t, fm, 2)
	second := tokenOf(t, fm)
	if first == second {
		t.Fatal("cada solicitud genera un token distinto")
	}
	if r := a.do(t, "POST", "/api/auth/reset", "", map[string]any{"token": first, "password": "nuevaclave123"}); r.Status != 400 {
		t.Fatalf("el enlace anterior debe quedar anulado: %d", r.Status)
	}
	var stored string
	a.DB.QueryRow(context.Background(), `SELECT token_hash FROM password_resets`).Scan(&stored)
	if stored == second || strings.Contains(stored, second) || stored != hashToken(second) {
		t.Fatalf("en la base solo debe estar el hash: %q", stored)
	}
	if r := a.do(t, "POST", "/api/auth/reset", "", map[string]any{"token": second, "password": "nuevaclave123"}); r.Status != 204 {
		t.Fatalf("el nuevo enlace sirve: %d", r.Status)
	}
}

func TestPlatformAdminCannotUseEmailReset(t *testing.T) {
	a, fm := newReset(t)
	if _, err := a.UpsertPlatformAdmin(context.Background(), "Super", "super@p.com", "ClaveLarga123"); err != nil {
		t.Fatal(err)
	}
	a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "super@p.com"})
	time.Sleep(150 * time.Millisecond)
	if fm.count() != 0 {
		t.Fatal("al administrador de plataforma no se le envía enlace")
	}
}

func TestInactiveAccountGetsNoResetAndTokenDiesWithDeactivation(t *testing.T) {
	a, fm := newReset(t)
	a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "a@x.com"})
	waitMail(t, fm, 1)
	token := tokenOf(t, fm)
	a.DB.Exec(context.Background(), `UPDATE users SET active = false WHERE email = 'a@x.com'`)
	if r := a.do(t, "POST", "/api/auth/reset", "", map[string]any{"token": token, "password": "nuevaclave123"}); r.Status != 400 {
		t.Fatalf("una cuenta dada de baja no se reactiva con el enlace: %d", r.Status)
	}
	before := fm.count()
	a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "a@x.com"})
	time.Sleep(150 * time.Millisecond)
	if fm.count() != before {
		t.Fatal("una cuenta inactiva no recibe enlace")
	}
}

func TestForgotWithoutMailAndRateLimits(t *testing.T) {
	a := newTestAPI(t)
	a.mustRegister(t, "Barbería", "a@x.com")
	if r := a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "a@x.com"}); r.Status != 503 || r.Body["code"] != "mail_disabled" {
		t.Fatalf("sin correo configurado: %d %v", r.Status, r.Body)
	}
	a.Mailer, a.AppURL = &fakeMailer{}, "https://app.test"
	a.Limiter = ratelimit.New()
	codes := []int{}
	for i := 0; i < 5; i++ {
		codes = append(codes, a.do(t, "POST", "/api/auth/forgot", "", map[string]any{"email": "a@x.com"}).Status)
	}
	if codes[2] != 200 || codes[3] != 429 {
		t.Fatalf("máximo 3 por correo y hora: %v", codes)
	}
}
