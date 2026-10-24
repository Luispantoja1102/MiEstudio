package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type boFx struct {
	*pbFx
	mp *fakeMP
	fm *fakeMailer
}

func newBO(t *testing.T) *boFx {
	t.Helper()
	f := newPB(t)
	b := &boFx{pbFx: f, mp: newFakeMP(t), fm: &fakeMailer{}}
	f.a.EncKey = bytes.Repeat([]byte{7}, 32)
	f.a.MPBase = b.mp.srv.URL
	f.a.PlatformMPToken = mpToken
	f.a.AppURL, f.a.PublicURL = "https://app.test", "https://api.test"
	f.a.Mailer = b.fm
	f.a.PlatformNoticeTo = "cobros@plataforma.test"
	return b
}

func (b *boFx) checkout(t *testing.T, plan string, months int) resp {
	return b.a.do(t, "POST", "/api/billing/checkout", b.admin, map[string]any{"plan": plan, "months": months})
}

func (b *boFx) lapse() {
	b.a.DB.Exec(context.Background(), `UPDATE businesses SET status = 'trial', trial_ends_at = $1`, b.a.now().Add(-time.Hour))
}

func (b *boFx) ref(t *testing.T, id float64) string {
	var ref string
	b.a.DB.QueryRow(context.Background(), `SELECT external_ref FROM subscription_checkouts WHERE id = $1`, int64(id)).Scan(&ref)
	return ref
}

func TestBillingPageWorksWhileLockedAndIsAdminOnly(t *testing.T) {
	b := newBO(t)
	b.lapse()
	if r := b.a.do(t, "GET", "/api/services", b.admin, nil); r.Status != 403 {
		t.Fatalf("bloqueado: %d", r.Status)
	}
	r := b.a.do(t, "GET", "/api/billing", b.admin, nil)
	if r.Status != 200 || r.Body["billing_state"] != "trial_expired" || r.Body["online"] != true || len(r.Body["plans"].([]any)) != 4 {
		t.Fatalf("billing: %d %v", r.Status, r.Body)
	}
	b.a.DB.Exec(context.Background(), `UPDATE businesses SET status = 'active', current_period_end = NULL`)
	b.a.addMember(t, b.admin, "recep@x.com", "reception")
	b.lapse()
	rec := b.a.doLogin(t, "recep@x.com", "supersecreta1").Body["token"].(string)
	if r := b.a.do(t, "GET", "/api/billing", rec, nil); r.Status != 403 {
		t.Fatalf("recepción: %d", r.Status)
	}
	if r := b.a.do(t, "GET", "/api/billing", b.plat, nil); r.Status != 403 {
		t.Fatalf("plataforma: %d", r.Status)
	}
	if r := b.a.do(t, "GET", "/api/billing", "", nil); r.Status != 401 {
		t.Fatalf("anónimo: %d", r.Status)
	}
}

func TestCheckoutUsesServerPriceAndPlatformAccount(t *testing.T) {
	b := newBO(t)
	b.lapse()
	r := b.checkout(t, "crecimiento", 1)
	if r.Status != 201 || r.Body["amount_cents"] != float64(70000) || r.Body["pay_url"] != "https://mp.test/pay/1" {
		t.Fatalf("checkout: %d %v", r.Status, r.Body)
	}
	pref := b.mp.prefs[0]
	if pref["items"].([]any)[0].(map[string]any)["unit_price"] != float64(700) || pref["external_reference"] != b.ref(t, r.Body["id"].(float64)) ||
		!strings.HasPrefix(pref["external_reference"].(string), "mie-sub:") || pref["notification_url"] != "https://api.test/api/webhooks/mercadopago-platform" ||
		pref["back_urls"].(map[string]any)["success"] != fmt.Sprintf("https://app.test/panel/suscripcion?checkout=%d", int(r.Body["id"].(float64))) {
		t.Fatalf("preferencia: %v", pref)
	}
	if r := b.checkout(t, "pro", 12); r.Body["amount_cents"] != float64(1200000) {
		t.Fatalf("anual: %v", r.Body)
	}
	for name, body := range map[string]map[string]any{"plan": {"plan": "gratis", "months": 1}, "meses": {"plan": "pro", "months": 6}, "vacío": {}} {
		if rr := b.a.do(t, "POST", "/api/billing/checkout", b.admin, body); rr.Status != 400 {
			t.Errorf("%s: %d", name, rr.Status)
		}
	}
	// el precio no se puede mandar desde el navegador
	b.a.do(t, "POST", "/api/billing/checkout", b.admin, map[string]any{"plan": "basico", "months": 1, "amount_cents": 1})
	var amount int
	b.a.DB.QueryRow(context.Background(), `SELECT amount_cents FROM subscription_checkouts ORDER BY id DESC LIMIT 1`).Scan(&amount)
	if amount != 35000 {
		t.Fatalf("monto guardado: %d", amount)
	}
	b.mp.prefFail = true
	if rr := b.checkout(t, "basico", 1); rr.Status != 502 {
		t.Fatalf("falla de Mercado Pago: %d", rr.Status)
	}
	b.a.PlatformMPToken = ""
	if rr := b.checkout(t, "basico", 1); rr.Status != 503 || rr.Body["code"] != "billing_offline" {
		t.Fatalf("sin cuenta de plataforma: %d %v", rr.Status, rr.Body)
	}
}

func TestPaymentUnlocksBusinessOnceAndSendsReceipts(t *testing.T) {
	b := newBO(t)
	b.lapse()
	c := b.checkout(t, "crecimiento", 1)
	id := c.Body["id"].(float64)
	ref := b.ref(t, id)
	// monto distinto, sin aprobar o de otra referencia: nada
	b.mp.pay(ref, 100, "approved")
	b.mp.pay(ref, 700, "pending")
	b.mp.pay("mie-sub:999", 700, "approved")
	if g := b.a.do(t, "GET", path("/api/billing/checkouts/", int64(id)), b.admin, nil); g.Body["status"] != "pending" {
		t.Fatalf("no debe acreditarse: %v", g.Body)
	}
	if r := b.a.do(t, "GET", "/api/services", b.admin, nil); r.Status != 403 {
		t.Fatal("sigue bloqueado")
	}
	pid := b.mp.pay(ref, 700, "approved")
	g := b.a.do(t, "GET", path("/api/billing/checkouts/", int64(id)), b.admin, nil)
	if g.Body["status"] != "paid" {
		t.Fatalf("pagado: %v", g.Body)
	}
	if r := b.a.do(t, "GET", "/api/services", b.admin, nil); r.Status != 200 {
		t.Fatalf("desbloqueado: %d", r.Status)
	}
	status, plan, end, _ := b.row(t)
	if status != "active" || plan != "crecimiento" || end == nil || !end.After(b.a.now().AddDate(0, 0, 27)) {
		t.Fatalf("negocio: %s %s %v", status, plan, end)
	}
	var n int
	var method, extRef string
	b.a.DB.QueryRow(context.Background(), `SELECT count(*), max(method), max(external_ref) FROM payments`).Scan(&n, &method, &extRef)
	if n != 1 || method != "mercadopago" || extRef != pid {
		t.Fatalf("pago registrado: %d %s %s", n, method, extRef)
	}
	// consultar otra vez no vuelve a acreditar
	b.a.do(t, "GET", path("/api/billing/checkouts/", int64(id)), b.admin, nil)
	b.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM payments`).Scan(&n)
	if n != 1 {
		t.Fatalf("doble acreditación: %d", n)
	}
	waitMail(t, b.fm, 2)
	to := map[string]bool{}
	for _, m := range b.fm.all() {
		to[m.To] = true
	}
	if !to["cobros@plataforma.test"] || !to["norte@x.com"] {
		t.Fatalf("correos a: %v", to)
	}
}

func TestPlatformWebhookSettlesAndVerifiesSignature(t *testing.T) {
	b := newBO(t)
	b.a.PlatformWebhookSecret = "secreto"
	b.lapse()
	c := b.checkout(t, "basico", 1)
	ref := b.ref(t, c.Body["id"].(float64))
	pid := b.mp.pay(ref, 350, "approved")
	send := func(url, sig string) int {
		req := httptest.NewRequest("POST", url, strings.NewReader(`{}`))
		req.Header.Set("x-signature", sig)
		req.Header.Set("x-request-id", "r1")
		rec := httptest.NewRecorder()
		b.a.Routes().ServeHTTP(rec, req)
		return rec.Code
	}
	url := "/api/webhooks/mercadopago-platform?type=payment&data.id=" + pid
	if send(url, "") != 401 || send(url, "ts=1,v1=00") != 401 {
		t.Fatal("sin firma válida no debe procesarse")
	}
	if _, _, _, _ = b.row(t); func() bool { s, _, _, _ := b.row(t); return s == "active" }() {
		t.Fatal("no debía acreditarse")
	}
	sig := signWebhook("secreto", pid, "r1", "1700000000")
	if send(url, "ts=1700000000,v1="+sig) != 200 {
		t.Fatal("firma válida")
	}
	if s, _, _, _ := b.row(t); s != "active" {
		t.Fatalf("estado tras el aviso: %s", s)
	}
	// avisos que no son de pago, o de otra referencia, no hacen nada
	b.a.PlatformWebhookSecret = ""
	if send("/api/webhooks/mercadopago-platform?type=merchant_order&data.id=1", "") != 200 {
		t.Fatal("otro tipo")
	}
	other := b.mp.pay("mie-dep:5", 10, "approved")
	if send("/api/webhooks/mercadopago-platform?type=payment&data.id="+other, "") != 200 {
		t.Fatal("referencia de anticipo")
	}
}

func TestDuplicatePaymentIsRefundedAndOtherBusinessCannotSeeCheckout(t *testing.T) {
	b := newBO(t)
	b.lapse()
	c := b.checkout(t, "basico", 1)
	id := c.Body["id"].(float64)
	ref := b.ref(t, id)
	p1 := b.mp.pay(ref, 350, "approved")
	p2 := b.mp.pay(ref, 350, "approved")
	b.a.do(t, "GET", path("/api/billing/checkouts/", int64(id)), b.admin, nil)
	if b.mp.refunds(p1)+b.mp.refunds(p2) != 1 {
		t.Fatalf("debe devolverse exactamente uno: %d %d", b.mp.refunds(p1), b.mp.refunds(p2))
	}
	other := b.a.mustRegister(t, "Otra", "otra@x.com")
	if r := b.a.do(t, "GET", path("/api/billing/checkouts/", int64(id)), other, nil); r.Status != 404 {
		t.Fatalf("otro negocio: %d", r.Status)
	}
}

func TestConcurrentSettleCreditsOnce(t *testing.T) {
	b := newBO(t)
	b.lapse()
	c := b.checkout(t, "crecimiento", 1)
	id := int64(c.Body["id"].(float64))
	b.mp.pay(b.ref(t, c.Body["id"].(float64)), 700, "approved")
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); b.a.settleCheckout(context.Background(), id) }()
	}
	wg.Wait()
	var n int
	b.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM payments`).Scan(&n)
	_, _, end, _ := b.row(t)
	if n != 1 || end == nil || end.After(b.a.now().AddDate(0, 1, 1)) {
		t.Fatalf("pagos=%d fin=%v", n, end)
	}
}

func TestBillingNoticesAreSentOncePerPeriod(t *testing.T) {
	b := newBO(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	b.a.Now = func() time.Time { return now }
	count := func(sub string) int {
		time.Sleep(120 * time.Millisecond)
		n := 0
		for _, m := range b.fm.all() {
			if strings.Contains(m.Subject, sub) && m.To == "norte@x.com" {
				n++
			}
		}
		return n
	}
	set := func(status string, trial, period *time.Time) {
		b.a.DB.Exec(ctx, `UPDATE businesses SET status = $1, trial_ends_at = $2, current_period_end = $3`, status, trial, period)
	}
	day := 24 * time.Hour
	set("trial", tp(now.Add(10*day)), nil)
	b.a.billingPass(ctx)
	if count("") != 0 {
		t.Fatal("con 10 días de prueba no se avisa")
	}
	set("trial", tp(now.Add(2*day)), nil)
	b.a.billingPass(ctx)
	b.a.billingPass(ctx)
	if count("termina pronto") != 1 {
		t.Fatalf("aviso de fin de prueba: %d", count("termina pronto"))
	}
	set("trial", tp(now.Add(-day)), nil)
	b.a.billingPass(ctx)
	b.a.billingPass(ctx)
	if count("prueba de Barbella terminó") != 1 {
		t.Fatalf("prueba vencida: %d", count("terminó"))
	}
	set("active", nil, tp(now.Add(4*day)))
	b.a.billingPass(ctx)
	b.a.billingPass(ctx)
	if count("vence pronto") != 1 {
		t.Fatalf("renovación próxima: %d", count("vence pronto"))
	}
	set("active", nil, tp(now.Add(-5*day)))
	b.a.billingPass(ctx)
	b.a.billingPass(ctx)
	if count("venció") != 1 {
		t.Fatalf("periodo vencido: %d", count("venció"))
	}
	// renovar abre otro periodo: el siguiente aviso sí sale
	set("active", nil, tp(now.Add(35*day)))
	b.a.Now = func() time.Time { return now.Add(32 * day) }
	b.a.billingPass(ctx)
	if count("vence pronto") != 2 {
		t.Fatalf("aviso del nuevo periodo: %d", count("vence pronto"))
	}
	// suspendidos y pendientes no reciben avisos de cobro
	before := b.fm.count()
	set("suspended", nil, nil)
	b.a.billingPass(ctx)
	time.Sleep(100 * time.Millisecond)
	if b.fm.count() != before {
		t.Fatal("un negocio suspendido no recibe avisos de cobro")
	}
}

func TestCourtesyPaymentSendsNoNotice(t *testing.T) {
	b := newBO(t)
	b.pay(t, map[string]any{"amount_cents": 0, "months": 1, "method": "courtesy"})
	time.Sleep(150 * time.Millisecond)
	if b.fm.count() != 0 {
		t.Fatalf("una cortesía no avisa: %d", b.fm.count())
	}
	b.pay(t, map[string]any{"amount_cents": 70000, "months": 1, "method": "transfer"})
	waitMail(t, b.fm, 2)
}

func signWebhook(secret, dataID, requestID, ts string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "id:%s;request-id:%s;ts:%s;", strings.ToLower(dataID), requestID, ts)
	return hex.EncodeToString(mac.Sum(nil))
}

// subjectsTo junta los asuntos de los correos que recibió una dirección.
func subjectsTo(fm *fakeMailer, to string) []string {
	var out []string
	for _, m := range fm.all() {
		if m.To == to {
			out = append(out, m.Subject)
		}
	}
	return out
}

func TestRegistrationSendsWelcomeAndTeamNotice(t *testing.T) {
	a := newTestAPI(t)
	fm := &fakeMailer{}
	a.Mailer, a.AppURL, a.PlatformNoticeTo = fm, "https://app.test", "equipo@plataforma.test"
	a.mustRegister(t, "Barbería Norte", "norte@x.com")
	waitMail(t, fm, 2)

	var welcome, notice *mailMsg
	for _, m := range fm.all() {
		m := m
		switch m.To {
		case "norte@x.com":
			welcome = &mailMsg{m.Subject, m.Text, m.ReplyTo}
		case "equipo@plataforma.test":
			notice = &mailMsg{m.Subject, m.Text, m.ReplyTo}
		}
	}
	if welcome == nil || welcome.subject != "Bienvenido a Barbella" || !strings.Contains(welcome.text, "https://app.test/login") || !strings.Contains(welcome.text, "Barbería Norte") {
		t.Fatalf("bienvenida: %+v", welcome)
	}
	if notice == nil || notice.subject != "Nuevo registro · Barbería Norte" || notice.replyTo != "norte@x.com" ||
		!strings.Contains(notice.text, "https://app.test/plataforma/negocios/") || !strings.Contains(notice.text, "norte@x.com") {
		t.Fatalf("aviso al equipo: %+v", notice)
	}
}

type mailMsg struct{ subject, text, replyTo string }

func TestTeamPaymentNoticeSaysFirstPaymentThenRenewal(t *testing.T) {
	b := newBO(t)
	b.pay(t, map[string]any{"amount_cents": 70000, "months": 1, "plan": "crecimiento", "method": "transfer"})
	waitMail(t, b.fm, 2)
	b.pay(t, map[string]any{"amount_cents": 70000, "months": 1, "plan": "crecimiento", "method": "transfer"})
	waitMail(t, b.fm, 4)
	got := subjectsTo(b.fm, "cobros@plataforma.test")
	if len(got) != 2 || !strings.HasPrefix(got[0], "Primer pago · Barbería Norte") || !strings.HasPrefix(got[1], "Pago mensual · Barbería Norte") {
		t.Fatalf("avisos de pago al equipo: %v", got)
	}
	b.pay(t, map[string]any{"amount_cents": 700000, "months": 12, "plan": "crecimiento", "method": "transfer"})
	waitMail(t, b.fm, 6)
	if got := subjectsTo(b.fm, "cobros@plataforma.test"); len(got) != 3 || !strings.HasPrefix(got[2], "Pago anual") {
		t.Fatalf("pago anual: %v", got)
	}
}

func TestMissedPaymentNoticeGoesToTeamOncePerEvent(t *testing.T) {
	b := newBO(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	b.a.Now = func() time.Time { return now }
	team := func() int {
		time.Sleep(150 * time.Millisecond)
		n := 0
		for _, s := range subjectsTo(b.fm, "cobros@plataforma.test") {
			if strings.HasPrefix(s, "No pagó") {
				n++
			}
		}
		return n
	}
	day := 24 * time.Hour
	// una prueba que terminó hace un día sin pagar: avisa una sola vez
	b.a.DB.Exec(ctx, `UPDATE businesses SET status = 'trial', trial_ends_at = $1, current_period_end = NULL`, now.Add(-day))
	b.a.billingPass(ctx)
	b.a.billingPass(ctx)
	if n := team(); n != 1 {
		t.Fatalf("prueba terminada sin pagar: %d avisos", n)
	}
	// historia vieja (hace 10 días): no se avisa, para no repasar todo el pasado
	b.a.DB.Exec(ctx, `UPDATE businesses SET trial_ends_at = $1`, now.Add(-10*day))
	b.a.billingPass(ctx)
	if n := team(); n != 1 {
		t.Fatalf("una prueba vieja no debe avisar: %d", n)
	}
	// un periodo pagado que terminó y ya pasó su gracia: otro evento, otro aviso
	b.a.DB.Exec(ctx, `UPDATE businesses SET status = 'active', trial_ends_at = NULL, current_period_end = $1`, now.Add(-(pastDueGraceDays*day + time.Hour)))
	b.a.billingPass(ctx)
	b.a.billingPass(ctx)
	if n := team(); n != 2 {
		t.Fatalf("periodo vencido sin renovar: %d avisos", n)
	}
}
