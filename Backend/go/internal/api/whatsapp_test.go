package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type waMsg struct {
	To, Template, Lang string
	Params             []string
}

type fakeWA struct {
	mu   sync.Mutex
	sent []waMsg
	fail error
	off  bool
}

func (f *fakeWA) Enabled() bool              { return !f.off }
func (f *fakeWA) Reminder() (string, string) { return "recordatorio_cita", "es_MX" }
func (f *fakeWA) count() int                 { f.mu.Lock(); defer f.mu.Unlock(); return len(f.sent) }
func (f *fakeWA) last() waMsg                { f.mu.Lock(); defer f.mu.Unlock(); return f.sent[len(f.sent)-1] }
func (f *fakeWA) SendTemplate(_ context.Context, to, name, lang string, params []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	f.sent = append(f.sent, waMsg{to, name, lang, params})
	return nil
}

// newWhatsApp arma un negocio con correo y WhatsApp y los recordatorios por WhatsApp encendidos.
func newWhatsApp(t *testing.T) (*onlineFixture, *fakeMailer, *fakeWA) {
	t.Helper()
	o, fm := newMailing(t)
	wa := &fakeWA{}
	o.a.WhatsApp = wa
	o.setWA(t, true, 204)
	return o, fm, wa
}

func (o *onlineFixture) setWA(t *testing.T, on bool, want int) {
	t.Helper()
	r := o.a.do(t, "PUT", "/api/booking/settings", o.admin, map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2,
		"deposit_percent": 0, "reminder_hours": 24, "show_prices": true, "welcome_text": "Hola", "whatsapp_reminders": on})
	if r.Status != want {
		t.Fatalf("ajustes: %d %v", r.Status, r.Body)
	}
}

func remindWindow(o *onlineFixture) { o.setNow(time.Date(2026, 11, 8, 22, 0, 0, 0, time.UTC)) }

func TestWhatsAppReminderGoesToConsentingClientOnly(t *testing.T) {
	o, fm, wa := newWhatsApp(t)
	o.mustBook(t, at(monday, "10:00"), map[string]any{"consent_whatsapp": true, "consent_reminders": true})
	o.mustBook(t, at(monday, "11:00"), map[string]any{"name": "Sin Whats", "phone": "6649998888", "email": "sw@x.com", "consent_whatsapp": false, "consent_reminders": false})
	o.process(t) // confirmaciones
	if wa.count() != 0 {
		t.Fatal("antes de la ventana no se manda")
	}
	remindWindow(o)
	o.process(t)
	if wa.count() != 1 {
		t.Fatalf("debía salir un solo WhatsApp, salieron %d", wa.count())
	}
	m := wa.last()
	if m.To != "526641234567" || m.Template != "recordatorio_cita" || m.Lang != "es_MX" {
		t.Fatalf("%+v", m)
	}
	if len(m.Params) != 4 || m.Params[0] != "Laura" || m.Params[1] != "Corte" || !strings.Contains(m.Params[2], "lunes 9 de noviembre a las 10:00") || m.Params[3] != "Barbería" {
		t.Fatalf("parámetros: %v", m.Params)
	}
	// el correo y el WhatsApp son independientes: Laura recibe ambos recordatorios
	var mails int
	for _, x := range fm.all() {
		if strings.Contains(x.Subject, "Recordatorio") {
			mails++
		}
	}
	if mails != 1 {
		t.Fatalf("recordatorios por correo: %d", mails)
	}
	// ni siquiera se encola nada para quien no aceptó WhatsApp
	var queued int
	o.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE channel = 'whatsapp'`).Scan(&queued)
	if queued != 1 {
		t.Fatalf("WhatsApp en la bandeja: %d", queued)
	}
	// una sola vez
	o.setNow(time.Date(2026, 11, 9, 2, 0, 0, 0, time.UTC))
	o.process(t)
	if wa.count() != 1 {
		t.Fatal("no debe repetirse")
	}
}

func TestEmailAndWhatsAppRemindersDoNotBlockEachOther(t *testing.T) {
	o, fm, wa := newWhatsApp(t)
	o.mustBook(t, at(monday, "10:00"), map[string]any{"consent_whatsapp": true, "consent_reminders": true})
	o.process(t)
	remindWindow(o)
	// el de WhatsApp se encola primero: el de correo debe salir igual
	if err := o.a.enqueueWhatsAppReminders(context.Background()); err != nil {
		t.Fatal(err)
	}
	o.process(t)
	mails := 0
	for _, x := range fm.all() {
		if strings.Contains(x.Subject, "Recordatorio") {
			mails++
		}
	}
	if mails != 1 || wa.count() != 1 {
		t.Fatalf("correos=%d whatsapp=%d", mails, wa.count())
	}
}

func TestWhatsAppRespectsBusinessSwitchAndConfiguration(t *testing.T) {
	// interruptor del negocio apagado
	o, _, wa := newWhatsApp(t)
	o.mustBook(t, at(monday, "10:00"), map[string]any{"consent_whatsapp": true})
	o.setWA(t, false, 204)
	remindWindow(o)
	o.process(t)
	if wa.count() != 0 {
		t.Fatal("el negocio lo tenía apagado")
	}
	var queued int
	o.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE channel = 'whatsapp'`).Scan(&queued)
	if queued != 0 {
		t.Fatalf("con el interruptor apagado no debe encolarse: %d", queued)
	}
	// sin credenciales en el servidor no se puede encender, y lo encolado espera
	o2, _, wa2 := newWhatsApp(t)
	o2.mustBook(t, at(monday, "10:00"), map[string]any{"consent_whatsapp": true})
	remindWindow(o2)
	wa2.off = true
	o2.process(t)
	if wa2.count() != 0 {
		t.Fatal("sin configuración no se manda")
	}
	o2.a.WhatsApp = nil
	o2.setWA(t, true, 409)
	wa2.off = false
	o2.a.WhatsApp = wa2
	o2.process(t)
	if wa2.count() != 1 {
		t.Fatalf("al configurarse debía salir lo pendiente: %d", wa2.count())
	}
}

func TestWhatsAppQueuedReminderSkippedAfterOptOutOrChange(t *testing.T) {
	o, _, wa := newWhatsApp(t)
	token := o.mustBook(t, at(monday, "10:00"), map[string]any{"consent_whatsapp": true})
	remindWindow(o)
	if err := o.a.enqueueWhatsAppReminders(context.Background()); err != nil {
		t.Fatal(err)
	}
	// la persona se da de baja antes de que salga
	if r := o.a.do(t, "POST", "/api/public-appointments/"+token+"/reminders-off", "", nil); r.Status != 200 {
		t.Fatalf("baja: %d", r.Status)
	}
	o.process(t)
	if wa.count() != 0 {
		t.Fatal("salió un WhatsApp tras la baja")
	}
	var skipped int
	o.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE channel = 'whatsapp' AND status = 'skipped'`).Scan(&skipped)
	if skipped != 1 {
		t.Fatalf("debía quedar omitido: %d", skipped)
	}

	// quien aceptó recordatorios por correo pero retira WhatsApp (por ejemplo respondiendo BAJA) tampoco lo recibe
	o3, _, wa3 := newWhatsApp(t)
	o3.mustBook(t, at(monday, "10:00"), map[string]any{"consent_whatsapp": true, "consent_reminders": true})
	remindWindow(o3)
	o3.a.enqueueWhatsAppReminders(context.Background())
	o3.a.DB.Exec(context.Background(), `UPDATE clients SET whatsapp_consent = false`)
	o3.process(t)
	if wa3.count() != 0 {
		t.Fatal("salió un WhatsApp tras retirar el consentimiento de WhatsApp")
	}

	// si cambia el teléfono del cliente, el aviso encolado ya no es para esa persona
	o2, _, wa2 := newWhatsApp(t)
	o2.mustBook(t, at(monday, "10:00"), map[string]any{"consent_whatsapp": true})
	remindWindow(o2)
	o2.a.enqueueWhatsAppReminders(context.Background())
	o2.a.DB.Exec(context.Background(), `UPDATE clients SET phone = '6640000000'`)
	o2.process(t)
	if wa2.count() != 0 {
		t.Fatal("salió a un número que ya no es del cliente")
	}
}

func TestWhatsAppFailureRetriesThenGivesUp(t *testing.T) {
	o, _, wa := newWhatsApp(t)
	o.mustBook(t, at(monday, "10:00"), map[string]any{"consent_whatsapp": true})
	remindWindow(o)
	wa.fail = errors.New("whatsapp: 132001 plantilla inexistente")
	o.process(t)
	var status, lastErr string
	var attempts int
	q := `SELECT status, attempts, last_error FROM notifications WHERE channel = 'whatsapp'`
	o.a.DB.QueryRow(context.Background(), q).Scan(&status, &attempts, &lastErr)
	if status != "pending" || attempts != 1 || !strings.Contains(lastErr, "132001") {
		t.Fatalf("tras el primer fallo: %s %d %q", status, attempts, lastErr)
	}
	// reintenta más tarde y, si sigue fallando, se rinde
	for i := 0; i < 3; i++ {
		o.setNow(o.a.now().Add(time.Hour))
		o.process(t)
	}
	o.a.DB.QueryRow(context.Background(), q).Scan(&status, &attempts, &lastErr)
	if status != "failed" || attempts != notifMaxAttempts {
		t.Fatalf("tras agotar intentos: %s %d", status, attempts)
	}
	// y si se arregla, una nueva cita sí sale
	wa.fail = nil
}

func TestClientWhatsAppConsentNeedsAPhone(t *testing.T) {
	o, _, _ := newWhatsApp(t)
	if r := o.a.do(t, "POST", "/api/clients", o.admin, map[string]any{"name": "Sin tel", "whatsapp_consent": true}); r.Status != 400 {
		t.Fatalf("sin teléfono: %d %v", r.Status, r.Body)
	}
	r := o.a.do(t, "POST", "/api/clients", o.admin, map[string]any{"name": "Con tel", "phone": "664 555 1212", "whatsapp_consent": true})
	if r.Status != 201 {
		t.Fatalf("con teléfono: %d %v", r.Status, r.Body)
	}
	c := id(r)
	if got := o.a.do(t, "GET", path("/api/clients/", c), o.admin, nil).Body; got["whatsapp_consent"] != true {
		t.Fatalf("no se guardó: %v", got)
	}
	// quitar el teléfono revoca el consentimiento de WhatsApp
	if r := o.a.do(t, "PATCH", path("/api/clients/", c), o.admin, map[string]any{"name": "Con tel", "phone": "", "whatsapp_consent": true}); r.Status != 400 {
		t.Fatalf("PATCH sin teléfono: %d", r.Status)
	}
	if r := o.a.do(t, "PATCH", path("/api/clients/", c), o.admin, map[string]any{"name": "Con tel", "phone": ""}); r.Status != 204 {
		t.Fatalf("PATCH: %d", r.Status)
	}
	if got := o.a.do(t, "GET", path("/api/clients/", c), o.admin, nil).Body; got["whatsapp_consent"] != false {
		t.Fatalf("debía revocarse: %v", got)
	}
}

func TestPublicInfoAdvertisesWhatsAppOnlyWhenAvailable(t *testing.T) {
	o, _, wa := newWhatsApp(t)
	info := func() any { return o.a.do(t, "GET", "/api/public/"+o.slug, "", nil).Body["whatsapp"] }
	if info() != true {
		t.Fatalf("debía ofrecerse: %v", info())
	}
	wa.off = true
	if info() != false {
		t.Fatal("sin configuración no debe ofrecerse")
	}
	wa.off = false
	o.setWA(t, false, 204)
	if info() != false {
		t.Fatal("con el interruptor apagado no debe ofrecerse")
	}
}

func signWA(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func (o *onlineFixture) waHook(t *testing.T, body, sig string) int {
	req := httptest.NewRequest("POST", "/api/webhooks/whatsapp", bytes.NewReader([]byte(body)))
	if sig != "" {
		req.Header.Set("X-Hub-Signature-256", sig)
	}
	rec := httptest.NewRecorder()
	o.a.Routes().ServeHTTP(rec, req)
	return rec.Code
}

func TestWhatsAppWebhookVerifyAndOptOut(t *testing.T) {
	o, _, _ := newWhatsApp(t)
	o.a.WAVerifyToken, o.a.WAAppSecret = "verif", "secreto"
	// verificación del webhook
	get := func(q string) (int, string) {
		rec := httptest.NewRecorder()
		o.a.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/api/webhooks/whatsapp?"+q, nil))
		return rec.Code, rec.Body.String()
	}
	if code, body := get("hub.mode=subscribe&hub.verify_token=verif&hub.challenge=12345"); code != http.StatusOK || body != "12345" {
		t.Fatalf("verificación: %d %q", code, body)
	}
	if code, _ := get("hub.mode=subscribe&hub.verify_token=otro&hub.challenge=1"); code != http.StatusForbidden {
		t.Fatalf("token incorrecto: %d", code)
	}
	if code, _ := get("hub.mode=subscribe&hub.challenge=1"); code != http.StatusForbidden {
		t.Fatalf("sin token: %d", code)
	}

	o.mustBook(t, at(monday, "10:00"), map[string]any{"consent_whatsapp": true})
	other := o.newClient(t, "Otra", "o@x.com", map[string]any{"phone": "6647770000", "whatsapp_consent": true})
	consent := func(phone string) bool {
		var v bool
		o.a.DB.QueryRow(context.Background(), `SELECT whatsapp_consent FROM clients WHERE right(regexp_replace(phone, '\D', '', 'g'), 10) = $1`, phone).Scan(&v)
		return v
	}
	msg := func(from, text string) string {
		return `{"entry":[{"changes":[{"value":{"messages":[{"from":"` + from + `","type":"text","text":{"body":"` + text + `"}}]}}]}]}`
	}
	// firma inválida o ausente: se ignora
	b := msg("526641234567", "BAJA")
	if code := o.waHook(t, b, "sha256=00"); code != http.StatusForbidden {
		t.Fatalf("firma mala: %d", code)
	}
	if code := o.waHook(t, b, ""); code != http.StatusForbidden {
		t.Fatalf("sin firma: %d", code)
	}
	if !consent("6641234567") {
		t.Fatal("un aviso sin firma válida no debe revocar nada")
	}
	// un mensaje cualquiera no revoca
	hi := msg("526641234567", "Gracias, ahí estaré")
	if code := o.waHook(t, hi, signWA("secreto", []byte(hi))); code != http.StatusOK || !consent("6641234567") {
		t.Fatalf("mensaje normal: %d", code)
	}
	// BAJA revoca solo a quien lo escribió
	if code := o.waHook(t, b, signWA("secreto", []byte(b))); code != http.StatusOK {
		t.Fatalf("baja: %d", code)
	}
	if consent("6641234567") || !consent("6647770000") {
		t.Fatalf("la baja debía afectar solo a ese número (otra=%d)", other)
	}
	// sin secreto configurado el webhook no existe
	o.a.WAAppSecret = ""
	if code := o.waHook(t, b, signWA("", []byte(b))); code != http.StatusNotFound {
		t.Fatalf("sin secreto: %d", code)
	}
}

func TestOptOutWords(t *testing.T) {
	for _, s := range []string{"BAJA", " baja ", "Stop", "STOP", "detener", "no más", "Baja por favor", "¡Cancelar!"} {
		if !isOptOut(s) {
			t.Errorf("%q debía ser baja", s)
		}
	}
	for _, s := range []string{"", "hola", "gracias", "no puedo ir", "bajar la cita", "estoy en la parada"} {
		if isOptOut(s) {
			t.Errorf("%q no debía ser baja", s)
		}
	}
}
