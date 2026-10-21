package api

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"miestudio/backend-go/internal/mail"
)

type fakeMailer struct {
	mu   sync.Mutex
	sent []mail.Message
	fail error
	off  bool
}

func (f *fakeMailer) Enabled() bool { return !f.off }
func (f *fakeMailer) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	f.sent = append(f.sent, m)
	return nil
}
func (f *fakeMailer) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.sent) }
func (f *fakeMailer) last() mail.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent[len(f.sent)-1]
}

func newMailing(t *testing.T) (*onlineFixture, *fakeMailer) {
	o := newOnline(t)
	fm := &fakeMailer{}
	o.a.Mailer = fm
	o.a.AppURL = "https://app.test"
	return o, fm
}

func (o *onlineFixture) process(t *testing.T) int {
	t.Helper()
	n, err := o.a.ProcessNotifications(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func (o *onlineFixture) setNow(t time.Time) { o.a.Now = func() time.Time { return t } }

func TestBookingSendsConfirmationWithManageLink(t *testing.T) {
	o, fm := newMailing(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	if fm.count() != 0 {
		t.Fatal("la petición no debe enviar: solo encola")
	}
	if o.process(t) != 1 {
		t.Fatal("debía salir la confirmación")
	}
	m := fm.last()
	if m.To != "laura@x.com" || !strings.Contains(m.Subject, "confirmada") {
		t.Fatalf("%+v", m)
	}
	for _, want := range []string{"lunes 9 de noviembre a las 10:00", "https://app.test/cita/" + token, "Corte", "Marco"} {
		if !strings.Contains(m.Text, want) {
			t.Fatalf("falta %q en:\n%s", want, m.Text)
		}
	}
	// no se reenvía
	if o.process(t) != 0 || fm.count() != 1 {
		t.Fatal("no debe repetirse")
	}
}

func TestMailEscapesHTMLAndSkipsWithoutEmail(t *testing.T) {
	o, fm := newMailing(t)
	o.mustBook(t, at(monday, "10:00"), map[string]any{"name": "<b>Laura</b>", "consent_reminders": false})
	o.process(t)
	if h := fm.last().HTML; strings.Contains(h, "<b>Laura") || !strings.Contains(h, "&lt;b&gt;") {
		t.Fatalf("html sin escapar: %s", h)
	}
	o.mustBook(t, at(monday, "11:00"), map[string]any{"email": "", "consent_reminders": false, "phone": "6640001111"})
	before := fm.count()
	o.process(t)
	if fm.count() != before {
		t.Fatal("sin correo no se envía nada")
	}
}

func TestNoMailWhenSMTPNotConfigured(t *testing.T) {
	o := newOnline(t)
	o.mustBook(t, at(monday, "10:00"), nil)
	var n int
	o.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM notifications`).Scan(&n)
	if n != 0 {
		t.Fatal("sin SMTP no se encola")
	}
	o.a.Mailer = &fakeMailer{off: true}
	if got, _ := o.a.ProcessNotifications(context.Background()); got != 0 {
		t.Fatal("deshabilitado no envía")
	}
}

func TestRetriesWithBackoffThenFails(t *testing.T) {
	o, fm := newMailing(t)
	o.mustBook(t, at(monday, "10:00"), nil)
	fm.fail = errors.New("smtp caído")
	base := o.a.now()
	for i, adv := range []time.Duration{0, 6 * time.Minute, 30 * time.Minute} {
		o.setNow(base.Add(adv))
		o.process(t)
		var attempts int
		var status string
		o.a.DB.QueryRow(context.Background(), `SELECT attempts, status FROM notifications`).Scan(&attempts, &status)
		wantStatus := "pending"
		if i == 2 {
			wantStatus = "failed"
		}
		if attempts != i+1 || status != wantStatus {
			t.Fatalf("intento %d: attempts=%d status=%s", i+1, attempts, status)
		}
	}
	// antes del plazo de reintento no se vuelve a intentar
	o.setNow(base.Add(time.Hour))
	o.process(t)
	var attempts int
	o.a.DB.QueryRow(context.Background(), `SELECT attempts FROM notifications`).Scan(&attempts)
	if attempts != 3 {
		t.Fatalf("una fila fallida no se reintenta: %d", attempts)
	}
}

func TestRetryWaitsForBackoff(t *testing.T) {
	o, fm := newMailing(t)
	o.mustBook(t, at(monday, "10:00"), nil)
	fm.fail = errors.New("smtp caído")
	o.process(t)
	fm.fail = nil
	o.setNow(o.a.now().Add(2 * time.Minute))
	if o.process(t) != 0 {
		t.Fatal("aún no toca reintentar")
	}
	o.setNow(o.a.now().Add(5 * time.Minute))
	if o.process(t) != 1 {
		t.Fatal("ya tocaba reintentar")
	}
}

func TestReminderWindowConsentAndDedupe(t *testing.T) {
	o, fm := newMailing(t)
	// reservada con mucha anticipación, con consentimiento
	o.mustBook(t, at(monday, "10:00"), nil)
	o.process(t) // confirmación
	base := fm.count()

	// 30 h antes: fuera de la ventana de 24 h
	o.setNow(time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)) // domingo 04:00 local; cita lunes 10:00 local = 30 h
	o.process(t)
	if fm.count() != base {
		t.Fatal("aún no toca el recordatorio")
	}
	// 20 h antes: dentro
	o.setNow(time.Date(2026, 11, 8, 22, 0, 0, 0, time.UTC))
	o.process(t)
	if fm.count() != base+1 || !strings.Contains(fm.last().Subject, "Recordatorio") {
		t.Fatalf("faltó el recordatorio: %d", fm.count())
	}
	if !strings.Contains(fm.last().Text, "?baja=1") {
		t.Fatal("el recordatorio debe traer cómo darse de baja")
	}
	// una sola vez
	o.setNow(time.Date(2026, 11, 9, 2, 0, 0, 0, time.UTC))
	o.process(t)
	if fm.count() != base+1 {
		t.Fatal("el recordatorio no debe repetirse")
	}
}

func TestReminderNeedsConsentAndSkipsLateBookings(t *testing.T) {
	o, fm := newMailing(t)
	o.mustBook(t, at(monday, "10:00"), map[string]any{"consent_reminders": false})
	o.setNow(time.Date(2026, 11, 8, 22, 0, 0, 0, time.UTC))
	o.process(t) // la confirmación sale; el recordatorio no
	for _, m := range fm.sent {
		if strings.Contains(m.Subject, "Recordatorio") {
			t.Fatal("sin consentimiento no hay recordatorio")
		}
	}
	// reservada ya dentro de la ventana: la confirmación basta
	o2, fm2 := newMailing(t)
	o2.setNow(time.Date(2026, 11, 8, 22, 0, 0, 0, time.UTC))
	o2.mustBook(t, at(monday, "10:00"), nil)
	o2.process(t)
	if fm2.count() != 1 {
		t.Fatalf("solo la confirmación, hubo %d", fm2.count())
	}
}

func TestReminderSkippedIfCancelledOrMovedOrUnsubscribed(t *testing.T) {
	o, fm := newMailing(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	o.process(t)
	base := fm.count()
	o.setNow(time.Date(2026, 11, 8, 22, 0, 0, 0, time.UTC))
	// el recordatorio se encola pero antes de enviarlo la persona se da de baja
	if err := o.a.enqueueReminders(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := o.a.do(t, "POST", "/api/public-appointments/"+token+"/reminders-off", "", nil); r.Status != 200 {
		t.Fatalf("baja: %d", r.Status)
	}
	o.process(t)
	if fm.count() != base {
		t.Fatal("tras la baja no debe enviarse el recordatorio")
	}
	var skipped int
	o.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE kind = 'reminder' AND status = 'skipped'`).Scan(&skipped)
	if skipped != 1 {
		t.Fatalf("debía quedar omitido: %d", skipped)
	}
	// y no vuelve a crearse
	o.process(t)
	var n int
	o.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE kind = 'reminder'`).Scan(&n)
	if n != 1 {
		t.Fatalf("recordatorios=%d", n)
	}
}

func TestCancelAndMoveSendMail(t *testing.T) {
	o, fm := newMailing(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	o.process(t)
	if r := o.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil); r.Status != 200 {
		t.Fatal(r.Status)
	}
	o.process(t)
	if !strings.Contains(fm.last().Subject, "cancelada") {
		t.Fatalf("%s", fm.last().Subject)
	}
	// cita interna cancelada por el negocio con cliente con correo
	o.a.do(t, "PATCH", path("/api/clients/", o.cl), o.admin, map[string]any{"name": "Cliente", "email": "cliente@x.com"})
	idAp := o.fixture.mustBook(t, o.marco, at(monday, "12:00"))
	o.process(t)
	n := fm.count()
	if r := o.a.do(t, "PATCH", path("/api/appointments/", idAp), o.admin, map[string]any{"status": "cancelled"}); r.Status != 204 {
		t.Fatal(r.Status)
	}
	o.process(t)
	if fm.count() != n+1 || fm.last().To != "cliente@x.com" {
		t.Fatalf("la cancelación interna debe avisar: %d", fm.count())
	}
	// reprogramar avisa del nuevo horario
	t2 := o.mustBook(t, at(monday, "09:00"), map[string]any{"phone": "6640002222", "email": "otra@x.com"})
	_ = t2
	var apID int64
	o.a.DB.QueryRow(context.Background(), `SELECT id FROM appointments WHERE public_token = $1`, t2).Scan(&apID)
	o.process(t)
	n = fm.count()
	if r := o.a.do(t, "POST", path("/api/appointments/", apID, "/move"), o.admin, map[string]any{"starts_at": at(monday, "13:00")}); r.Status != 204 {
		t.Fatalf("mover %d %v", r.Status, r.Body)
	}
	o.process(t)
	if fm.count() != n+1 || !strings.Contains(fm.last().Subject, "cambió") || !strings.Contains(fm.last().Text, "a las 13:00") {
		t.Fatalf("aviso de cambio: %d %v", fm.count(), fm.last().Subject)
	}
}

func TestConfirmationNotSentForCancelledBeforeSend(t *testing.T) {
	o, fm := newMailing(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	o.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil)
	o.process(t)
	for _, m := range fm.sent {
		if strings.Contains(m.Subject, "confirmada") {
			t.Fatal("no se confirma una cita ya cancelada")
		}
	}
}

func TestClientRemindersConsentRules(t *testing.T) {
	f := newFixture(t)
	if r := f.a.do(t, "POST", "/api/clients", f.admin, map[string]any{"name": "Sin correo", "reminders_consent": true}); r.Status != 400 {
		t.Fatalf("consentimiento sin correo: %d", r.Status)
	}
	r := f.a.do(t, "POST", "/api/clients", f.admin, map[string]any{"name": "Con correo", "email": "c@x.com", "reminders_consent": true})
	if r.Status != 201 {
		t.Fatal(r.Status, r.Body)
	}
	cid := id(r)
	get := func() bool {
		c := f.a.do(t, "GET", path("/api/clients/", cid), f.admin, nil)
		return c.Body["reminders_consent"] == true
	}
	if !get() {
		t.Fatal("debía guardarse el consentimiento")
	}
	// editar sin mencionar el consentimiento lo conserva
	f.a.do(t, "PATCH", path("/api/clients/", cid), f.admin, map[string]any{"name": "Con correo", "email": "c@x.com"})
	if !get() {
		t.Fatal("no debe perderse al editar otros datos")
	}
	// quitar el correo retira el consentimiento
	f.a.do(t, "PATCH", path("/api/clients/", cid), f.admin, map[string]any{"name": "Con correo", "email": ""})
	if get() {
		t.Fatal("sin correo no puede haber recordatorios")
	}
}
