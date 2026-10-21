package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type depFx struct {
	*onlineFixture
	mp *fakeMP
	fm *fakeMailer
}

// newDeposit: reserva en línea con Mercado Pago conectado (falso) y anticipo del 30 % ($45 de un corte de $150).
func newDeposit(t *testing.T) *depFx {
	t.Helper()
	o := newOnline(t)
	d := &depFx{onlineFixture: o, mp: newFakeMP(t), fm: &fakeMailer{}}
	o.a.EncKey = bytes.Repeat([]byte{7}, 32)
	o.a.MPBase = d.mp.srv.URL
	o.a.AppURL = "https://app.test"
	o.a.PublicURL = "https://api.test"
	o.a.Mailer = d.fm
	if r := o.a.do(t, "PUT", "/api/payments/mercadopago/token", o.admin, map[string]any{"access_token": mpToken}); r.Status != 204 {
		t.Fatalf("conectar MP: %d %v", r.Status, r.Body)
	}
	d.settings(t, 30)
	return d
}

func (d *depFx) settings(t *testing.T, percent int) {
	t.Helper()
	r := d.a.do(t, "PUT", "/api/booking/settings", d.admin, map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30,
		"cancel_min_hours": 2, "reminder_hours": 24, "show_prices": true, "deposit_percent": percent})
	if r.Status != 204 {
		t.Fatalf("ajustes: %d %v", r.Status, r.Body)
	}
}

func (d *depFx) ref(t *testing.T, token string) string {
	t.Helper()
	var ref string
	if err := d.a.DB.QueryRow(context.Background(),
		`SELECT d.external_ref FROM deposits d JOIN appointments a ON a.id = d.appointment_id WHERE a.public_token = $1`, token).Scan(&ref); err != nil {
		t.Fatal(err)
	}
	return ref
}

func (d *depFx) apptStatus(t *testing.T, token string) string {
	t.Helper()
	var s string
	d.a.DB.QueryRow(context.Background(), `SELECT status FROM appointments WHERE public_token = $1`, token).Scan(&s)
	return s
}

func (d *depFx) depStatus(t *testing.T, token string) string {
	t.Helper()
	var s string
	d.a.DB.QueryRow(context.Background(), `SELECT d.status FROM deposits d JOIN appointments a ON a.id = d.appointment_id WHERE a.public_token = $1`, token).Scan(&s)
	return s
}

func (d *depFx) view(t *testing.T, token string) resp {
	return d.a.do(t, "GET", "/api/public-appointments/"+token, "", nil)
}

func (d *depFx) bookDeposit(t *testing.T, start string, mod map[string]any) (string, resp) {
	t.Helper()
	r := d.book(t, start, mod)
	if r.Status != 201 {
		t.Fatalf("reservar con anticipo: %d %v", r.Status, r.Body)
	}
	return r.Body["token"].(string), r
}

func TestDepositBookingHoldsSlotAndCreatesPreference(t *testing.T) {
	d := newDeposit(t)
	token, r := d.bookDeposit(t, at(monday, "10:00"), nil)
	if r.Body["status"] != "pending_payment" || r.Body["deposit_cents"] != float64(4500) || r.Body["pay_url"] != "https://mp.test/pay/1" {
		t.Fatalf("respuesta: %v", r.Body)
	}
	if len(d.mp.prefs) != 1 {
		t.Fatalf("preferencias: %d", len(d.mp.prefs))
	}
	p := d.mp.prefs[0]
	item := p["items"].([]any)[0].(map[string]any)
	if item["unit_price"] != float64(45) || item["currency_id"] != "MXN" || item["quantity"] != float64(1) {
		t.Fatalf("item: %v", item)
	}
	if p["external_reference"] != d.ref(t, token) || !strings.HasPrefix(p["external_reference"].(string), "mie-dep:") {
		t.Fatalf("referencia: %v", p["external_reference"])
	}
	if p["auto_return"] != "approved" || p["back_urls"].(map[string]any)["success"] != "https://app.test/cita/"+token {
		t.Fatalf("retorno: %v", p)
	}
	if !strings.HasPrefix(p["notification_url"].(string), "https://api.test/api/webhooks/mercadopago?biz=") {
		t.Fatalf("aviso: %v", p["notification_url"])
	}
	if p["expires"] != true {
		t.Fatal("el enlace debe vencer junto con el apartado")
	}
	// el horario queda apartado y no hay confirmación todavía
	if r := d.book(t, at(monday, "10:00"), map[string]any{"phone": "6641110002", "email": "otro@x.com"}); r.Status != 409 {
		t.Fatalf("horario apartado: %d", r.Status)
	}
	d.process(t)
	if d.fm.count() != 0 {
		t.Fatal("no se confirma antes del pago")
	}
	v := d.view(t, token)
	dep := v.Body["deposit"].(map[string]any)
	if v.Body["status"] != "pending_payment" || dep["pay_url"] != "https://mp.test/pay/1" || dep["status"] != "pending" {
		t.Fatalf("ver: %v", v.Body)
	}
}

func TestDepositPaidConfirmsAppointment(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	d.mp.pay(d.ref(t, token), 45, "approved")
	v := d.view(t, token) // al volver de Mercado Pago
	if v.Body["status"] != "scheduled" || v.Body["deposit"].(map[string]any)["status"] != "paid" {
		t.Fatalf("tras pagar: %v", v.Body)
	}
	if _, has := v.Body["deposit"].(map[string]any)["pay_url"]; has {
		t.Fatal("ya pagado: no debe ofrecerse el enlace de pago")
	}
	d.process(t)
	if d.fm.count() != 1 || !strings.Contains(d.fm.last().Subject, "confirmada") {
		t.Fatalf("correo de confirmación: %d", d.fm.count())
	}
	// ver otra vez no cambia nada ni duplica correos
	d.view(t, token)
	d.process(t)
	if d.fm.count() != 1 {
		t.Fatal("confirmación duplicada")
	}
	var paidAt *time.Time
	d.a.DB.QueryRow(context.Background(), `SELECT paid_at FROM deposits`).Scan(&paidAt)
	if paidAt == nil {
		t.Fatal("falta paid_at")
	}
}

func TestDepositIgnoresWrongOrUnapprovedPayments(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	ref := d.ref(t, token)
	d.mp.pay(ref, 30, "approved")    // monto distinto
	d.mp.pay(ref, 45, "pending")     // sin aprobar
	d.mp.pay(ref, 45, "rejected")    // rechazado
	d.mp.pay(ref, 45.01, "approved") // casi
	d.mp.pay("mie-dep:99999", 45, "approved")
	if v := d.view(t, token); v.Body["status"] != "pending_payment" {
		t.Fatalf("no debe confirmarse: %v", v.Body)
	}
}

func TestDepositWebhook(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	pid := d.mp.pay(d.ref(t, token), 45, "approved")
	biz := fmt.Sprint(d.bizID(t))
	hook := func(path string, body string, hdr map[string]string) int {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		d.a.Routes().ServeHTTP(rec, req)
		return rec.Code
	}
	// otros avisos o negocios ajenos no tocan nada
	if hook("/api/webhooks/mercadopago?biz="+biz, `{"type":"merchant_order","data":{"id":"1"}}`, nil) != 200 || d.apptStatus(t, token) != "pending_payment" {
		t.Fatal("un aviso de otro tipo debe ignorarse")
	}
	if hook("/api/webhooks/mercadopago?biz=99999", `{"type":"payment","data":{"id":"`+pid+`"}}`, nil) != 200 || d.apptStatus(t, token) != "pending_payment" {
		t.Fatal("un negocio ajeno no puede liquidar este anticipo")
	}
	if hook("/api/webhooks/mercadopago?biz="+biz, `{"type":"payment","data":{"id":"`+pid+`"}}`, nil) != 200 || d.apptStatus(t, token) != "scheduled" {
		t.Fatalf("el aviso debía confirmar la cita: %s", d.apptStatus(t, token))
	}
	// repetir el mismo aviso es inofensivo
	if hook("/api/webhooks/mercadopago?biz="+biz+"&type=payment&data.id="+pid, ``, nil) != 200 || d.apptStatus(t, token) != "scheduled" {
		t.Fatal("aviso repetido")
	}
	if d.mp.refunds(pid) != 0 {
		t.Fatal("el pago confirmado no se devuelve")
	}
}

func (d *depFx) bizID(t *testing.T) int64 {
	var id int64
	d.a.DB.QueryRow(context.Background(), `SELECT id FROM businesses`).Scan(&id)
	return id
}

func TestDepositWebhookSignature(t *testing.T) {
	d := newDeposit(t)
	d.a.MPWebhookSecret = "secreto-de-prueba"
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	pid := d.mp.pay(d.ref(t, token), 45, "approved")
	biz := fmt.Sprint(d.bizID(t))
	url := "/api/webhooks/mercadopago?biz=" + biz + "&data.id=" + pid + "&type=payment"
	send := func(sig string) int {
		req := httptest.NewRequest("POST", url, strings.NewReader(`{}`))
		req.Header.Set("x-signature", sig)
		req.Header.Set("x-request-id", "req-1")
		rec := httptest.NewRecorder()
		d.a.Routes().ServeHTTP(rec, req)
		return rec.Code
	}
	if send("") != http.StatusUnauthorized || send("ts=1,v1=deadbeef") != http.StatusUnauthorized || d.apptStatus(t, token) != "pending_payment" {
		t.Fatal("sin firma válida no debe procesarse")
	}
	mac := hmac.New(sha256.New, []byte("secreto-de-prueba"))
	fmt.Fprintf(mac, "id:%s;request-id:req-1;ts:1700000000;", pid)
	if send("ts=1700000000,v1="+hex.EncodeToString(mac.Sum(nil))) != 200 || d.apptStatus(t, token) != "scheduled" {
		t.Fatal("con firma válida debe confirmarse")
	}
}

func TestVerifyWebhookSignatureUnit(t *testing.T) {
	mac := hmac.New(sha256.New, []byte("k"))
	mac.Write([]byte("id:abc;request-id:r;ts:5;"))
	good := hex.EncodeToString(mac.Sum(nil))
	if !verifyWebhookSignature("k", "ts=5,v1="+good, "r", "ABC") { // el id va en minúsculas
		t.Fatal("firma buena rechazada")
	}
	for _, bad := range []string{"", "ts=5", "v1=" + good, "ts=6,v1=" + good, "ts=5,v1=" + good[:10]} {
		if verifyWebhookSignature("k", bad, "r", "abc") {
			t.Fatalf("firma mala aceptada: %q", bad)
		}
	}
	if verifyWebhookSignature("otra", "ts=5,v1="+good, "r", "abc") {
		t.Fatal("clave distinta aceptada")
	}
}

func TestHoldExpiryReleasesSlotAndMarksDeposit(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	d.setNow(d.a.now().Add(20 * time.Minute))
	if r := d.book(t, at(monday, "10:00"), map[string]any{"phone": "6641110002", "email": "otro@x.com"}); r.Status != 201 {
		t.Fatalf("el apartado vencido debe liberar el horario: %d %v", r.Status, r.Body)
	}
	if d.apptStatus(t, token) != "cancelled" || d.depStatus(t, token) != "expired" {
		t.Fatalf("estado: %s / %s", d.apptStatus(t, token), d.depStatus(t, token))
	}
}

func TestHoldExpiryHonoursPaymentWhoseWebhookWasLost(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	d.mp.pay(d.ref(t, token), 45, "approved") // pagó, pero nadie nos avisó
	d.setNow(d.a.now().Add(20 * time.Minute))
	d.process(t) // el barrido consulta a Mercado Pago antes de liberar
	if d.apptStatus(t, token) != "scheduled" || d.depStatus(t, token) != "paid" {
		t.Fatalf("debía confirmarse: %s / %s", d.apptStatus(t, token), d.depStatus(t, token))
	}
}

func TestLatePaymentAfterExpiryIsRefunded(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	d.setNow(d.a.now().Add(20 * time.Minute))
	d.process(t)
	if d.depStatus(t, token) != "expired" {
		t.Fatalf("debía vencer: %s", d.depStatus(t, token))
	}
	pid := d.mp.pay(d.ref(t, token), 45, "approved") // pagó con el enlace ya vencido
	d.process(t)                                     // la revisión de vencidos recientes lo detecta
	if d.depStatus(t, token) != "refunded" || d.mp.refunds(pid) != 1 || d.apptStatus(t, token) != "cancelled" {
		t.Fatalf("pago tardío: %s / refunds=%d / %s", d.depStatus(t, token), d.mp.refunds(pid), d.apptStatus(t, token))
	}
	d.process(t) // no se devuelve dos veces
	if d.mp.refunds(pid) != 1 {
		t.Fatal("devolución duplicada")
	}
}

func TestExpiryKeepsHoldWhileMPUnreachableThenReleases(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	d.mp.searchFail = true
	d.setNow(d.a.now().Add(20 * time.Minute))
	d.process(t)
	if d.apptStatus(t, token) != "pending_payment" {
		t.Fatal("sin poder consultar, el apartado no se libera de inmediato")
	}
	d.setNow(d.a.now().Add(90 * time.Minute))
	d.process(t)
	if d.apptStatus(t, token) != "cancelled" {
		t.Fatal("pasada la gracia debe liberarse")
	}
}

func TestPreferenceFailureUndoesBooking(t *testing.T) {
	d := newDeposit(t)
	d.mp.prefFail = true
	r := d.book(t, at(monday, "10:00"), nil)
	if r.Status != 502 {
		t.Fatalf("esperaba 502, fue %d %v", r.Status, r.Body)
	}
	d.mp.prefFail = false
	if r := d.book(t, at(monday, "10:00"), nil); r.Status != 201 {
		t.Fatalf("el horario debe quedar libre: %d %v", r.Status, r.Body)
	}
}

func TestNoDepositWhenMPNotConnected(t *testing.T) {
	d := newDeposit(t)
	d.a.DB.Exec(context.Background(), `DELETE FROM mp_accounts`)
	token := d.mustBook(t, at(monday, "10:00"), nil)
	if d.apptStatus(t, token) != "scheduled" {
		t.Fatal("sin Mercado Pago conectado no se pide anticipo")
	}
}

func TestClientCancelRefundsPaidDeposit(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	pid := d.mp.pay(d.ref(t, token), 45, "approved")
	d.view(t, token)

	// Mercado Pago no puede devolver: la cita sigue en pie
	d.mp.payRefundFail = "cannot refund"
	if r := d.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil); r.Status != 502 {
		t.Fatalf("falla de devolución: %d %v", r.Status, r.Body)
	}
	if d.apptStatus(t, token) != "scheduled" || d.depStatus(t, token) != "paid" {
		t.Fatal("no debe cancelarse si no se pudo devolver")
	}
	d.mp.payRefundFail = ""
	if r := d.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil); r.Status != 200 {
		t.Fatalf("cancelar: %d %v", r.Status, r.Body)
	}
	if d.apptStatus(t, token) != "cancelled" || d.depStatus(t, token) != "refunded" || d.mp.refunds(pid) != 1 {
		t.Fatalf("estado: %s / %s / %d", d.apptStatus(t, token), d.depStatus(t, token), d.mp.refunds(pid))
	}
}

func TestClientCannotCancelInsideWindowAndKeepsDeposit(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	pid := d.mp.pay(d.ref(t, token), 45, "approved")
	d.view(t, token)
	d.setNow(time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC)) // 1 h antes
	if r := d.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil); r.Status != 409 {
		t.Fatalf("%d", r.Status)
	}
	if d.mp.refunds(pid) != 0 || d.depStatus(t, token) != "paid" {
		t.Fatal("fuera de plazo no se devuelve")
	}
}

func TestClientCancelPendingPaymentFreesSlotWithoutRefund(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	if r := d.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil); r.Status != 200 {
		t.Fatal(r.Status)
	}
	if d.depStatus(t, token) != "expired" {
		t.Fatalf("anticipo: %s", d.depStatus(t, token))
	}
	// si aun así paga con el enlace, se devuelve
	pid := d.mp.pay(d.ref(t, token), 45, "approved")
	d.process(t)
	if d.mp.refunds(pid) != 1 || d.depStatus(t, token) != "refunded" {
		t.Fatalf("pago a una cita cancelada: %d %s", d.mp.refunds(pid), d.depStatus(t, token))
	}
}

func TestStaffCancelRefundsAndNoShowKeeps(t *testing.T) {
	d := newDeposit(t)
	t1, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	p1 := d.mp.pay(d.ref(t, t1), 45, "approved")
	d.view(t, t1)
	t2, _ := d.bookDeposit(t, at(monday, "11:00"), map[string]any{"phone": "6641110002", "email": "b@x.com"})
	p2 := d.mp.pay(d.ref(t, t2), 45, "approved")
	d.view(t, t2)
	idOf := func(tok string) int64 {
		var id int64
		d.a.DB.QueryRow(context.Background(), `SELECT id FROM appointments WHERE public_token = $1`, tok).Scan(&id)
		return id
	}
	d.mp.payRefundFail = "nope"
	if r := d.a.do(t, "PATCH", path("/api/appointments/", idOf(t1)), d.admin, map[string]any{"status": "cancelled"}); r.Status != 502 {
		t.Fatalf("falla de devolución: %d", r.Status)
	}
	if d.apptStatus(t, t1) != "scheduled" {
		t.Fatal("la cita no debe cancelarse si no se devolvió el anticipo")
	}
	d.mp.payRefundFail = ""
	if r := d.a.do(t, "PATCH", path("/api/appointments/", idOf(t1)), d.admin, map[string]any{"status": "cancelled"}); r.Status != 204 {
		t.Fatalf("cancelar: %d", r.Status)
	}
	if d.depStatus(t, t1) != "refunded" || d.mp.refunds(p1) != 1 {
		t.Fatal("el negocio que cancela devuelve el anticipo")
	}
	if r := d.a.do(t, "PATCH", path("/api/appointments/", idOf(t2)), d.admin, map[string]any{"status": "no_show"}); r.Status != 204 {
		t.Fatal(r.Status)
	}
	if d.depStatus(t, t2) != "kept" || d.mp.refunds(p2) != 0 {
		t.Fatalf("inasistencia: %s", d.depStatus(t, t2))
	}
}

func TestDuplicatePaymentIsRefunded(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	a := d.mp.pay(d.ref(t, token), 45, "approved")
	b := d.mp.pay(d.ref(t, token), 45, "approved")
	d.view(t, token)
	if d.mp.refunds(a)+d.mp.refunds(b) != 1 {
		t.Fatalf("debe devolverse exactamente uno de los dos: %d %d", d.mp.refunds(a), d.mp.refunds(b))
	}
	if d.apptStatus(t, token) != "scheduled" {
		t.Fatal("la cita queda confirmada")
	}
}

func TestDepositAmountRules(t *testing.T) {
	for _, c := range []struct{ price, pct, want int }{
		{15000, 30, 4500}, {15050, 30, 4515}, {15001, 50, 7501}, // redondea al centavo
		{150, 10, 100}, // mínimo un peso
		{80, 50, 80},   // nunca más que el precio
		{0, 30, 0}, {15000, 0, 0},
	} {
		if got := depositAmount(c.price, c.pct); got != c.want {
			t.Errorf("depositAmount(%d,%d)=%d, esperaba %d", c.price, c.pct, got, c.want)
		}
	}
}

func TestSettleIsSafeUnderConcurrency(t *testing.T) {
	d := newDeposit(t)
	token, _ := d.bookDeposit(t, at(monday, "10:00"), nil)
	d.mp.pay(d.ref(t, token), 45, "approved")
	var depID int64
	d.a.DB.QueryRow(context.Background(), `SELECT id FROM deposits`).Scan(&depID)
	done := make(chan string, 6)
	for i := 0; i < 6; i++ {
		go func() { st, _ := d.a.settleDeposit(context.Background(), depID); done <- st }()
	}
	for i := 0; i < 6; i++ {
		<-done
	}
	var n int
	d.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE kind = 'booking'`).Scan(&n)
	if n != 1 || d.apptStatus(t, token) != "scheduled" || d.depStatus(t, token) != "paid" {
		t.Fatalf("concurrencia: correos=%d %s %s", n, d.apptStatus(t, token), d.depStatus(t, token))
	}
}
