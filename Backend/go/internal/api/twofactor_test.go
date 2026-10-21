package api

import (
	"bytes"
	"context"
	"testing"
	"time"

	"miestudio/backend-go/internal/auth"
	"miestudio/backend-go/internal/ratelimit"
)

type tfFx struct {
	a      *API
	admin  string // token Bearer
	secret string
	codes  []string
}

func curStep() int64 { return time.Now().Unix() / 30 }

func (f *tfFx) code(t *testing.T, step int64) string {
	c, err := totpCode(f.secret, step)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// newTF registra un negocio y le activa los dos pasos.
func newTF(t *testing.T) *tfFx {
	t.Helper()
	a := newTestAPI(t)
	a.EncKey = bytes.Repeat([]byte{3}, 32)
	f := &tfFx{a: a, admin: a.mustRegister(t, "Barbería", "a@x.com")}
	s := a.do(t, "POST", "/api/me/2fa/setup", f.admin, nil)
	if s.Status != 200 {
		t.Fatalf("setup: %d %v", s.Status, s.Body)
	}
	f.secret = s.Body["secret"].(string)
	e := a.do(t, "POST", "/api/me/2fa/enable", f.admin, map[string]any{"code": f.code(t, curStep())})
	if e.Status != 200 {
		t.Fatalf("enable: %d %v", e.Status, e.Body)
	}
	for _, c := range e.Body["recovery_codes"].([]any) {
		f.codes = append(f.codes, c.(string))
	}
	return f
}

func (f *tfFx) challenge(t *testing.T) string {
	r := f.a.doLogin(t, "a@x.com", "supersecreta1")
	if r.Status != 200 || r.Body["needs_2fa"] != true || r.Body["token"] != nil {
		t.Fatalf("login con 2FA: %d %v", r.Status, r.Body)
	}
	return r.Body["challenge"].(string)
}

func TestEnrollmentFlowAndSecretIsEncrypted(t *testing.T) {
	a := newTestAPI(t)
	a.EncKey = bytes.Repeat([]byte{3}, 32)
	admin := a.mustRegister(t, "Barbería", "a@x.com")
	if r := a.do(t, "POST", "/api/me/2fa/enable", admin, map[string]any{"code": "123456"}); r.Status != 409 {
		t.Fatalf("activar sin generar el QR: %d", r.Status)
	}
	s := a.do(t, "POST", "/api/me/2fa/setup", admin, nil)
	secret := s.Body["secret"].(string)
	if url, _ := s.Body["otpauth_url"].(string); len(url) < 20 || url[:15] != "otpauth://totp/" {
		t.Fatalf("otpauth: %v", s.Body["otpauth_url"])
	}
	if r := a.do(t, "POST", "/api/me/2fa/enable", admin, map[string]any{"code": "000000"}); r.Status != 400 {
		t.Fatalf("código incorrecto: %d", r.Status)
	}
	good, _ := totpCode(secret, curStep())
	e := a.do(t, "POST", "/api/me/2fa/enable", admin, map[string]any{"code": good})
	if e.Status != 200 || len(e.Body["recovery_codes"].([]any)) != 8 {
		t.Fatalf("activar: %d %v", e.Status, e.Body)
	}
	st := a.do(t, "GET", "/api/me/2fa", admin, nil)
	if st.Body["enabled"] != true || st.Body["recovery_left"] != float64(8) {
		t.Fatalf("estado: %v", st.Body)
	}
	if r := a.do(t, "POST", "/api/me/2fa/setup", admin, nil); r.Status != 409 {
		t.Fatalf("no se puede generar otro con el primero activo: %d", r.Status)
	}
	var enc []byte
	a.DB.QueryRow(context.Background(), `SELECT totp_secret_enc FROM users`).Scan(&enc)
	if len(enc) == 0 || bytes.Contains(enc, []byte(secret)) {
		t.Fatal("el secreto no debe estar en claro")
	}
	// los códigos de respaldo tampoco
	var h string
	a.DB.QueryRow(context.Background(), `SELECT code_hash FROM totp_recovery LIMIT 1`).Scan(&h)
	for _, c := range e.Body["recovery_codes"].([]any) {
		if h == c.(string) {
			t.Fatal("respaldo en claro")
		}
	}
	if me := a.do(t, "GET", "/api/me", admin, nil); me.Body["two_factor"] != true {
		t.Fatal("/api/me debe indicar dos pasos")
	}
}

func TestLoginNeedsSecondStepAndChallengeIsNotASession(t *testing.T) {
	f := newTF(t)
	ch := f.challenge(t)
	if r := f.a.do(t, "GET", "/api/me", ch, nil); r.Status != 401 {
		t.Fatalf("el reto no abre sesión: %d", r.Status)
	}
	if rec, _ := f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": ch, "code": "000000"}); rec.Code != 401 || sessionCookie(rec) != nil {
		t.Fatalf("código malo: %d", rec.Code)
	}
	rec, body := f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": ch, "code": f.code(t, curStep()+1)})
	c := sessionCookie(rec)
	if rec.Code != 200 || c == nil || body["token"] == nil {
		t.Fatalf("segundo paso: %d %v", rec.Code, body)
	}
	if rec, _ := f.a.doCookie(t, "GET", "/api/me", c, false, nil); rec.Code != 200 {
		t.Fatalf("sesión: %d", rec.Code)
	}
	if rec, _ := f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": "basura", "code": "123456"}); rec.Code != 401 {
		t.Fatalf("reto inválido: %d", rec.Code)
	}
}

func TestSessionTokenIsNotAChallenge(t *testing.T) {
	f := newTF(t)
	session, _ := auth.Sign(f.a.Secret, 1, 0)
	// con un código válido y fresco: lo único que lo detiene es que el token no es un reto
	if rec, _ := f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": session, "code": f.code(t, curStep()+1)}); rec.Code != 401 {
		t.Fatalf("un token de sesión no es un reto: %d", rec.Code)
	}
}

func TestTOTPCodeCannotBeReplayed(t *testing.T) {
	f := newTF(t)
	code := f.code(t, curStep()+1)
	rec, _ := f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": f.challenge(t), "code": code})
	if rec.Code != 200 {
		t.Fatalf("primer uso: %d", rec.Code)
	}
	rec, _ = f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": f.challenge(t), "code": code})
	if rec.Code != 401 {
		t.Fatalf("un código ya usado no sirve de nuevo: %d", rec.Code)
	}
	// ni uno de un paso anterior al último aceptado
	rec, _ = f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": f.challenge(t), "code": f.code(t, curStep())})
	if rec.Code != 401 {
		t.Fatalf("paso anterior: %d", rec.Code)
	}
}

func TestRecoveryCodeWorksOnce(t *testing.T) {
	f := newTF(t)
	rc := f.codes[0]
	rec, body := f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": f.challenge(t), "code": rc})
	if rec.Code != 200 || body["recovery_left"] != float64(7) {
		t.Fatalf("respaldo: %d %v", rec.Code, body)
	}
	if rec, _ := f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": f.challenge(t), "code": rc}); rec.Code != 401 {
		t.Fatalf("un respaldo se usa una sola vez: %d", rec.Code)
	}
	// se acepta con mayúsculas y sin guion
	other := f.codes[1]
	messy := other[:4] + other[5:]
	if rec, _ := f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": f.challenge(t), "code": messy}); rec.Code != 200 {
		t.Fatalf("formato flexible: %d", rec.Code)
	}
}

func TestSecondStepIsRateLimited(t *testing.T) {
	f := newTF(t)
	f.a.Limiter = ratelimit.New()
	ch := f.challenge(t)
	last := 0
	for i := 0; i < 8; i++ {
		rec, _ := f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": ch, "code": "000000"})
		last = rec.Code
	}
	if last != 429 {
		t.Fatalf("tras varios intentos debe limitar: %d", last)
	}
	// incluso con el código correcto
	if rec, _ := f.a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": ch, "code": f.code(t, curStep()+1)}); rec.Code != 429 {
		t.Fatalf("limitado aun con código bueno: %d", rec.Code)
	}
}

func TestDisableAndRegenerateNeedPasswordAndCode(t *testing.T) {
	f := newTF(t)
	a := f.a
	if r := a.do(t, "POST", "/api/me/2fa/disable", f.admin, map[string]any{"password": "mal", "code": f.code(t, curStep()+1)}); r.Status != 401 {
		t.Fatalf("clave mala: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/me/2fa/disable", f.admin, map[string]any{"password": "supersecreta1", "code": "000000"}); r.Status != 401 {
		t.Fatalf("código malo: %d", r.Status)
	}
	// regenerar invalida los anteriores
	old := f.codes[0]
	r := a.do(t, "POST", "/api/me/2fa/recovery", f.admin, map[string]any{"password": "supersecreta1", "code": f.code(t, curStep()+1)})
	if r.Status != 200 || len(r.Body["recovery_codes"].([]any)) != 8 {
		t.Fatalf("regenerar: %d %v", r.Status, r.Body)
	}
	if rec, _ := a.doCookie(t, "POST", "/api/auth/login/2fa", nil, true, map[string]any{"challenge": f.challenge(t), "code": old}); rec.Code != 401 {
		t.Fatalf("respaldo viejo: %d", rec.Code)
	}
	// desactivar con un código de respaldo nuevo
	fresh := r.Body["recovery_codes"].([]any)[0].(string)
	if d := a.do(t, "POST", "/api/me/2fa/disable", f.admin, map[string]any{"password": "supersecreta1", "code": fresh}); d.Status != 204 {
		t.Fatalf("desactivar: %d %v", d.Status, d.Body)
	}
	if l := a.doLogin(t, "a@x.com", "supersecreta1"); l.Status != 200 || l.Body["token"] == nil {
		t.Fatalf("sin dos pasos vuelve el login normal: %v", l.Body)
	}
	var n int
	a.DB.QueryRow(context.Background(), `SELECT count(*) FROM totp_recovery`).Scan(&n)
	if n != 0 {
		t.Fatal("deben borrarse los respaldos")
	}
}

func TestTeamAndPlatformResetTwoFactor(t *testing.T) {
	a := newTestAPI(t)
	a.EncKey = bytes.Repeat([]byte{3}, 32)
	admin := a.mustRegister(t, "Barbería", "a@x.com")
	other := a.mustRegister(t, "Otra", "o@x.com")
	mid := a.addMember(t, admin, "m@x.com", "reception")
	mtoken := a.doLogin(t, "m@x.com", "supersecreta1").Body["token"].(string)
	s := a.do(t, "POST", "/api/me/2fa/setup", mtoken, nil)
	secret := s.Body["secret"].(string)
	c, _ := totpCode(secret, curStep())
	if e := a.do(t, "POST", "/api/me/2fa/enable", mtoken, map[string]any{"code": c}); e.Status != 200 {
		t.Fatal(e.Status, e.Body)
	}
	if r := a.doLogin(t, "m@x.com", "supersecreta1"); r.Body["needs_2fa"] != true {
		t.Fatal("el miembro tiene dos pasos")
	}
	// otro negocio no puede tocarlo
	if r := a.do(t, "POST", path("/api/team/", mid, "/2fa/reset"), other, nil); r.Status != 404 {
		t.Fatalf("otro negocio: %d", r.Status)
	}
	// un miembro sin rol de administrador tampoco
	if r := a.do(t, "POST", path("/api/team/", mid, "/2fa/reset"), mtoken, nil); r.Status != 403 {
		t.Fatalf("sin permiso: %d", r.Status)
	}
	if r := a.do(t, "POST", path("/api/team/", mid, "/2fa/reset"), admin, nil); r.Status != 204 {
		t.Fatalf("reset: %d", r.Status)
	}
	if r := a.doLogin(t, "m@x.com", "supersecreta1"); r.Body["needs_2fa"] == true || r.Body["token"] == nil {
		t.Fatalf("tras el reset entra normal: %v", r.Body)
	}
	// plataforma
	a.UpsertPlatformAdmin(context.Background(), "Super", "super@p.com", "ClaveLarga123")
	ptoken := a.doLogin(t, "super@p.com", "ClaveLarga123").Body["token"].(string)
	if r := a.do(t, "POST", path("/api/platform/users/", mid, "/2fa-reset"), admin, nil); r.Status != 403 {
		t.Fatalf("un negocio no usa la ruta de plataforma: %d", r.Status)
	}
	if r := a.do(t, "POST", path("/api/platform/users/", mid, "/2fa-reset"), ptoken, nil); r.Status != 204 {
		t.Fatalf("plataforma: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/platform/users/99999/2fa-reset", ptoken, nil); r.Status != 404 {
		t.Fatalf("inexistente: %d", r.Status)
	}
}

func TestSetupNeedsEncryptionKey(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "Barbería", "a@x.com")
	a.EncKey = nil
	if r := a.do(t, "POST", "/api/me/2fa/setup", admin, nil); r.Status != 503 {
		t.Fatalf("sin clave de cifrado: %d", r.Status)
	}
}
