package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func (b *boFx) subscribe(t *testing.T, plan string) resp {
	return b.a.do(t, "POST", "/api/billing/subscribe", b.admin, map[string]any{"plan": plan})
}

// hook manda un aviso de Mercado Pago a la plataforma.
func (b *boFx) hook(t *testing.T, query string) int {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/webhooks/mercadopago-platform?"+query, strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	b.a.Routes().ServeHTTP(rec, req)
	return rec.Code
}

func (b *boFx) paymentsCount() (n int) {
	b.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM payments WHERE business_id = $1`, b.biz).Scan(&n)
	return
}

func TestSubscribeCreatesPreapprovalWithServerPrice(t *testing.T) {
	b := newBO(t)
	// el navegador manda un monto falso: se ignora, el precio sale del plan
	r := b.a.do(t, "POST", "/api/billing/subscribe", b.admin, map[string]any{"plan": "basico", "amount": 1, "months": 12})
	if r.Status != 201 || !strings.HasPrefix(r.Body["pay_url"].(string), "https://mp.test/subscribe/") || r.Body["amount_cents"].(float64) != 35000 {
		t.Fatalf("subscribe: %d %v", r.Status, r.Body)
	}
	pre := b.mp.pres["pre-1"]
	if pre == nil {
		t.Fatalf("no se creó el preapproval: %v", b.mp.calls)
	}
	rec, _ := pre.Body["auto_recurring"].(map[string]any)
	if rec["frequency"].(float64) != 1 || rec["frequency_type"] != "months" || rec["transaction_amount"].(float64) != 350 || rec["currency_id"] != "MXN" {
		t.Fatalf("auto_recurring: %v", rec)
	}
	if _, ok := rec["start_date"]; !ok {
		t.Fatalf("en prueba el primer cargo debe esperar al fin de la prueba: %v", rec)
	}
	if pre.Body["status"] != "pending" || !strings.HasPrefix(pre.Ref, "mie-mandate:") || pre.Body["notification_url"] != "https://api.test/api/webhooks/mercadopago-platform" || pre.Body["payer_email"] != "norte@x.com" {
		t.Fatalf("preapproval: %v", pre.Body)
	}
	if r := b.subscribe(t, "no-existe"); r.Status != 400 {
		t.Fatalf("plan desconocido: %d", r.Status)
	}
	if r := b.a.do(t, "GET", "/api/billing", b.admin, nil); r.Body["subscription"] == nil {
		t.Fatalf("billing debe mostrar la suscripción: %v", r.Body)
	}
}

func TestSubscribeStartsChargingNowWhenNothingToWaitFor(t *testing.T) {
	b := newBO(t)
	b.lapse() // prueba terminada: no hay nada que esperar
	if r := b.subscribe(t, "pro"); r.Status != 201 {
		t.Fatalf("subscribe: %d %v", r.Status, r.Body)
	}
	rec := b.mp.pres["pre-1"].Body["auto_recurring"].(map[string]any)
	if _, ok := rec["start_date"]; ok {
		t.Fatalf("sin prueba ni periodo vigente se cobra al autorizar: %v", rec)
	}
}

func TestSubscriptionChargeExtendsOnlyOnceAndChecksTheAmount(t *testing.T) {
	b := newBO(t)
	b.lapse()
	if r := b.subscribe(t, "basico"); r.Status != 201 {
		t.Fatalf("subscribe: %d", r.Status)
	}
	b.mp.authorize("pre-1")
	// un cargo con un monto distinto al del plan no acredita nada
	bad, _ := b.mp.charge("pre-1", 100, "approved")
	_ = bad
	authBad := "AP-" + bad
	if b.hook(t, "type=subscription_authorized_payment&data.id="+authBad) != 200 || b.paymentsCount() != 0 {
		t.Fatal("monto distinto: no debía acreditarse")
	}
	// uno rechazado, tampoco
	_, rej := b.mp.charge("pre-1", 350, "rejected")
	if b.hook(t, "type=subscription_authorized_payment&data.id=AP-"+rej) != 200 || b.paymentsCount() != 0 {
		t.Fatal("rechazado: no debía acreditarse")
	}
	// el bueno acredita un mes; repetirlo no suma otro
	_, ok := b.mp.charge("pre-1", 350, "approved")
	for i := 0; i < 3; i++ {
		if b.hook(t, "type=subscription_authorized_payment&data.id=AP-"+ok) != 200 {
			t.Fatal("aviso")
		}
	}
	st, plan, end, _ := b.row(t)
	if st != "active" || plan != "basico" || end == nil || b.paymentsCount() != 1 {
		t.Fatalf("tras el cargo: %s %s %v pagos=%d", st, plan, end, b.paymentsCount())
	}
	if !end.After(b.a.now().AddDate(0, 0, 27)) || end.After(b.a.now().AddDate(0, 0, 32)) {
		t.Fatalf("debe sumar un mes: %v", end)
	}
	// el mismo cargo por el aviso de «payment» tampoco duplica
	if b.hook(t, "type=payment&data.id="+ok) != 200 || b.paymentsCount() != 1 {
		t.Fatal("el aviso de pago repetido no debe duplicar")
	}
	// el siguiente mes llega otro cargo y se suma
	_, next := b.mp.charge("pre-1", 350, "approved")
	if b.hook(t, "type=payment&data.id="+next) != 200 || b.paymentsCount() != 2 {
		t.Fatalf("segundo cargo: pagos=%d", b.paymentsCount())
	}
	var status string
	b.a.DB.QueryRow(context.Background(), `SELECT status FROM subscription_mandates WHERE id = 1`).Scan(&status)
	if status != "authorized" {
		t.Fatalf("un cargo aprobado deja la suscripción autorizada: %s", status)
	}
}

func TestSubscriptionStatusFollowsMercadoPago(t *testing.T) {
	b := newBO(t)
	r := b.subscribe(t, "basico")
	id := int64(r.Body["id"].(float64))
	url := path("/api/billing/subscriptions/", id)
	if g := b.a.do(t, "GET", url, b.admin, nil); g.Body["status"] != "pending" {
		t.Fatalf("pendiente: %v", g.Body)
	}
	b.mp.authorize("pre-1")
	if g := b.a.do(t, "GET", url, b.admin, nil); g.Body["status"] != "authorized" {
		t.Fatalf("al volver de Mercado Pago debe verse autorizada: %v", g.Body)
	}
	// MP avisa que la pausó o canceló (por ejemplo, tarjeta rechazada varias veces)
	b.mp.pres["pre-1"].Status = "cancelled"
	if b.hook(t, "type=subscription_preapproval&data.id=pre-1") != 200 {
		t.Fatal("aviso")
	}
	var status string
	b.a.DB.QueryRow(context.Background(), `SELECT status FROM subscription_mandates WHERE id = $1`, id).Scan(&status)
	if status != "cancelled" {
		t.Fatalf("estado: %s", status)
	}
	// otro negocio no puede ver la suscripción
	other := b.a.mustRegister(t, "Spa Sur", "sur@x.com")
	if g := b.a.do(t, "GET", url, other, nil); g.Status != 404 {
		t.Fatalf("otro negocio: %d", g.Status)
	}
}

func TestCancelSubscriptionStopsChargingButKeepsPaidPeriod(t *testing.T) {
	b := newBO(t)
	b.lapse()
	b.subscribe(t, "basico")
	b.mp.authorize("pre-1")
	_, p1 := b.mp.charge("pre-1", 350, "approved")
	b.hook(t, "type=payment&data.id="+p1)
	if r := b.a.do(t, "POST", "/api/billing/subscription/cancel", b.admin, nil); r.Status != 200 {
		t.Fatalf("cancel: %d %v", r.Status, r.Body)
	}
	if b.mp.pres["pre-1"].Status != "cancelled" {
		t.Fatal("debe cancelarse en Mercado Pago")
	}
	if st, _, _, _ := b.row(t); st != "active" {
		t.Fatalf("conserva el periodo pagado: %s", st)
	}
	if r := b.a.do(t, "GET", "/api/billing", b.admin, nil); r.Body["subscription"] != nil {
		t.Fatalf("ya no hay suscripción: %v", r.Body["subscription"])
	}
	if r := b.a.do(t, "POST", "/api/billing/subscription/cancel", b.admin, nil); r.Status != 404 {
		t.Fatalf("cancelar sin suscripción: %d", r.Status)
	}
	// si Mercado Pago no pudo cancelar, no se da por cancelada
	b.subscribe(t, "basico")
	b.mp.authorize("pre-2")
	b.mp.preCancelFail = true
	if r := b.a.do(t, "POST", "/api/billing/subscription/cancel", b.admin, nil); r.Status != 502 {
		t.Fatalf("falla de MP: %d", r.Status)
	}
	if r := b.a.do(t, "GET", "/api/billing", b.admin, nil); r.Body["subscription"] == nil {
		t.Fatal("sigue vigente si MP no canceló")
	}
}

func TestChangingPlanReplacesThePreviousSubscription(t *testing.T) {
	b := newBO(t)
	b.subscribe(t, "basico")
	b.mp.authorize("pre-1")
	// si no se puede cancelar la anterior, no se crea otra (no se cobra doble)
	b.mp.preCancelFail = true
	if r := b.subscribe(t, "pro"); r.Status != 502 || len(b.mp.pres) != 1 {
		t.Fatalf("no debe crear otra: %d pres=%d", r.Status, len(b.mp.pres))
	}
	b.mp.preCancelFail = false
	if r := b.subscribe(t, "pro"); r.Status != 201 {
		t.Fatalf("cambio de plan: %d %v", r.Status, r.Body)
	}
	if b.mp.pres["pre-1"].Status != "cancelled" || b.mp.pres["pre-2"].Status != "pending" {
		t.Fatalf("estados: %s %s", b.mp.pres["pre-1"].Status, b.mp.pres["pre-2"].Status)
	}
	var live int
	b.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM subscription_mandates WHERE status <> 'cancelled'`).Scan(&live)
	if live != 1 {
		t.Fatalf("debe quedar una sola vigente: %d", live)
	}
}

func TestSubscribeNeedsMercadoPagoAndAdmin(t *testing.T) {
	b := newBO(t)
	b.a.PlatformMPToken = ""
	if r := b.subscribe(t, "basico"); r.Status != 503 {
		t.Fatalf("sin MP: %d", r.Status)
	}
	b.a.PlatformMPToken = mpToken
	b.mp.preFail = true
	if r := b.subscribe(t, "basico"); r.Status != 502 {
		t.Fatalf("MP rechaza: %d", r.Status)
	}
	var live int
	b.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM subscription_mandates WHERE status <> 'cancelled'`).Scan(&live)
	if live != 0 {
		t.Fatalf("una creación fallida no deja suscripción vigente: %d", live)
	}
	if r := b.a.do(t, "POST", "/api/billing/subscribe", b.plat, map[string]any{"plan": "basico"}); r.Status != 403 {
		t.Fatalf("la plataforma no suscribe negocios: %d", r.Status)
	}
}

func TestAutoRenewalNoticeIsSkippedWhileSubscribed(t *testing.T) {
	waitMail := func(b *boFx) int {
		for i := 0; i < 40; i++ {
			if n := b.fm.count(); n > 0 {
				return n
			}
			time.Sleep(25 * time.Millisecond)
		}
		return 0
	}
	// control: sin suscripción automática y a punto de vencer, sí se avisa
	c := newBO(t)
	c.a.DB.Exec(context.Background(), `UPDATE businesses SET status = 'active', current_period_end = $1`, c.a.now().Add(48*time.Hour))
	c.a.billingPass(context.Background())
	if waitMail(c) == 0 {
		t.Fatal("sin cobro automático debe avisar del vencimiento")
	}
	// con cobro automático autorizado, no
	b := newBO(t)
	b.a.DB.Exec(context.Background(), `UPDATE businesses SET status = 'active', current_period_end = $1`, b.a.now().Add(48*time.Hour))
	b.subscribe(t, "basico")
	b.a.DB.Exec(context.Background(), `UPDATE subscription_mandates SET status = 'authorized'`)
	b.a.billingPass(context.Background())
	time.Sleep(300 * time.Millisecond)
	if n := b.fm.count(); n != 0 {
		t.Fatalf("con cobro automático no se avisa de vencimiento: %d correos", n)
	}
}

func TestReasonNeverExceedsMercadoPagoLimit(t *testing.T) {
	if got := firstRunes(strings.Repeat("ñ", 80), 60); len([]rune(got)) != 60 {
		t.Fatalf("recorte: %d", len([]rune(got)))
	}
	if firstRunes("corto", 60) != "corto" {
		t.Fatal("un texto corto no cambia")
	}
}
