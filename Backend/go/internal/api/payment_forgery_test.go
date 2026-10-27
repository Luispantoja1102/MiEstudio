package api

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// Intentos de falsificar un pago o de saltarse al titular: ninguno debe cambiar el estado de una cita ni de un anticipo.

func (o *onlineFixture) apptState(t *testing.T, id int64) (appt, dep string) {
	t.Helper()
	o.a.DB.QueryRow(t.Context(), `SELECT ap.status, d.status FROM appointments ap JOIN deposits d ON d.appointment_id = ap.id WHERE ap.id = $1`, id).Scan(&appt, &dep)
	return
}

func TestNobodyButTheOwnerCanTurnATransferIntoAPaidAppointment(t *testing.T) {
	o := newTransfer(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	id := o.apptID(t, token)
	confirm := fmt.Sprintf("/api/appointments/%d/transfer/confirm", id)
	reject := fmt.Sprintf("/api/appointments/%d/transfer/reject", id)
	refunded := fmt.Sprintf("/api/appointments/%d/transfer/refunded", id)
	proof := fmt.Sprintf("/api/appointments/%d/proof", id)

	o.a.addMember(t, o.admin, "recep@x.com", "reception")
	o.a.addMember(t, o.admin, "caja@x.com", "cashier")
	o.a.addMember(t, o.admin, "barbero@x.com", "staff")
	otherAdmin := o.a.mustRegister(t, "Otro negocio", "otro@x.com")

	// sin sesión, con un token inventado o con una sesión de otro negocio: nada cambia
	for _, c := range []struct{ name, tok string }{{"sin sesión", ""}, {"token falso", "no-es-un-token"}, {"administrador de otro negocio", otherAdmin},
		{"recepción", login(t, o.a, "recep@x.com")}, {"caja", login(t, o.a, "caja@x.com")}, {"personal", login(t, o.a, "barbero@x.com")}} {
		for _, path := range []string{confirm, refunded} {
			if r := o.a.do(t, "POST", path, c.tok, nil); r.Status == 204 {
				t.Fatalf("%s pudo usar %s", c.name, path)
			}
		}
		if r := o.a.do(t, "POST", reject, c.tok, map[string]any{"reason": "no me gusta"}); r.Status == 204 {
			t.Fatalf("%s pudo rechazar", c.name)
		}
	}
	if a, d := o.apptState(t, id); a != "pending_payment" || d != "pending" {
		t.Fatalf("tras los intentos la cita debe seguir esperando el anticipo: %s / %s", a, d)
	}

	// subir un comprobante no paga: solo lo deja a la vista del titular
	if r := o.a.doFile(t, "/api/public-appointments/"+token+"/proof", "", "c.png", tinyPNG); r.Status != 204 {
		t.Fatalf("subir: %d", r.Status)
	}
	if a, d := o.apptState(t, id); a != "pending_payment" || d != "pending" {
		t.Fatalf("un comprobante no confirma nada: %s / %s", a, d)
	}
	// el comprobante de un negocio no se ve desde otro, ni sin sesión, ni con un token de cita ajeno
	for _, tok := range []string{"", otherAdmin} {
		if r := o.a.do(t, "GET", proof, tok, nil); r.Status == 200 {
			t.Fatalf("comprobante visible para quien no debe (%q)", tok)
		}
	}
	if r := o.a.doFile(t, proof, otherAdmin, "x.png", tinyPNG); r.Status == 204 {
		t.Fatal("otro negocio no puede adjuntar comprobantes a una cita ajena")
	}
	if r := o.a.doFile(t, "/api/public-appointments/ABCDEFGHIJKLMNOPQRSTUVWXYZ012345/proof", "", "x.png", tinyPNG); r.Status != 404 {
		t.Fatalf("token inventado: %d", r.Status)
	}
	// la cita pendiente de anticipo no se puede cobrar en caja como si estuviera pagada
	if r := o.a.do(t, "POST", "/api/sales", o.admin, map[string]any{"branch_id": o.branch, "appointment_id": id,
		"lines": []any{map[string]any{"service_id": o.svc, "qty": 1, "staff_id": o.marco}}, "payments": []any{map[string]any{"method": "deposit", "amount_cents": 4500}}}); r.Status == 201 || r.Status == 200 {
		t.Fatalf("no se puede aplicar un anticipo que no está pagado: %d %v", r.Status, r.Body)
	}
	if a, d := o.apptState(t, id); a != "pending_payment" || d != "pending" {
		t.Fatalf("estado tras intentar cobrar: %s / %s", a, d)
	}
	// y la devolución solo existe tras un pago confirmado
	if r := o.a.do(t, "POST", refunded, o.admin, nil); r.Status != 409 {
		t.Fatalf("marcar devuelto algo que no se pagó: %d", r.Status)
	}
	// solo el titular lo confirma
	if r := o.a.do(t, "POST", confirm, o.admin, nil); r.Status != 204 {
		t.Fatalf("el titular sí confirma: %d", r.Status)
	}
}

func TestProofFilesAreValidatedAndServedSafely(t *testing.T) {
	o := newTransfer(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	id := o.apptID(t, token)
	url := "/api/public-appointments/" + token + "/proof"
	for name, content := range map[string][]byte{
		"html":      []byte("<html><script>alert(1)</script></html>"),
		"svg":       []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"exe":       append([]byte("MZ"), bytes.Repeat([]byte{0}, 100)...),
		"vacío":     {},
		"js":        []byte("alert(document.cookie)"),
		"texto png": []byte("esto no es una imagen.png"),
	} {
		if r := o.a.doFile(t, url, "", name, content); r.Status == 204 {
			t.Fatalf("el archivo %q no debe aceptarse como comprobante", name)
		}
	}
	if r := o.a.doFile(t, url, "", "../../etc/passwd.png", tinyPNG); r.Status != 204 {
		t.Fatalf("un nombre raro se guarda como texto, no como ruta: %d", r.Status)
	}
	var stored string
	o.a.DB.QueryRow(t.Context(), `SELECT content_type FROM deposit_proofs`).Scan(&stored)
	if stored != "image/png" {
		t.Fatalf("el tipo se decide por el contenido, no por el nombre: %s", stored)
	}
	// la imagen se sirve sin que el navegador la interprete como otra cosa
	req := o.a.do(t, "GET", fmt.Sprintf("/api/appointments/%d/proof", id), o.admin, nil)
	if req.Status != 200 {
		t.Fatalf("ver comprobante: %d", req.Status)
	}
	// un PDF se descarga en lugar de abrirse dentro de la plataforma
	pdf := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), 50)...)
	if r := o.a.doFile(t, url, "", "c.pdf", pdf); r.Status != 204 {
		t.Fatalf("pdf: %d", r.Status)
	}
	rec := httpGet(t, o.a, fmt.Sprintf("/api/appointments/%d/proof", id), o.admin)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") ||
		!strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment") || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("encabezados del comprobante: %v", rec.Header())
	}
}

func TestClientCannotChooseTheDepositAmountOrAnUnofferedMethod(t *testing.T) {
	o := newTransfer(t) // anticipo del 30 % de $150 = $45, solo por transferencia
	r := o.book(t, at(monday, "10:00"), map[string]any{"deposit_cents": 100, "amount_cents": 1, "status": "scheduled", "deposit_status": "paid", "payment_method": "transfer"})
	if r.Status != 201 {
		t.Fatalf("reserva: %d %v", r.Status, r.Body)
	}
	if r.Body["status"] != "pending_payment" || r.Body["deposit_cents"] != float64(4500) {
		t.Fatalf("el monto y el estado los decide el servidor: %v", r.Body)
	}
	token := r.Body["token"].(string)
	if a, d := o.status(t, token); a != "pending_payment" || d != "pending" {
		t.Fatalf("estado real: %s / %s", a, d)
	}
	// Mercado Pago no está conectado: no se puede pedir ese método
	if r := o.book(t, at(monday, "11:00"), map[string]any{"payment_method": "mp", "phone": "6641110002", "email": "b@x.com"}); r.Status != 400 {
		t.Fatalf("método no ofrecido: %d %v", r.Status, r.Body)
	}
	// con las transferencias apagadas tampoco existe ese camino
	o.transferSettings(t, 200, map[string]any{"transfer_enabled": false, "deposit_percent": 0})
	if r := o.book(t, at(monday, "12:00"), map[string]any{"payment_method": "transfer", "phone": "6641110003", "email": "c@x.com"}); r.Status != 201 || r.Body["status"] != "scheduled" || r.Body["pay_method"] != nil {
		t.Fatalf("sin anticipo configurado no se crea uno: %d %v", r.Status, r.Body)
	}
}

func TestConfirmingAnExpiredOrCancelledTransferIsRefused(t *testing.T) {
	o := newTransfer(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	id := o.apptID(t, token)
	o.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil)
	if r := o.a.do(t, "POST", fmt.Sprintf("/api/appointments/%d/transfer/confirm", id), o.admin, nil); r.Status != 409 {
		t.Fatalf("confirmar una cita cancelada: %d", r.Status)
	}
	if a, _ := o.apptState(t, id); a != "cancelled" {
		t.Fatalf("la cita cancelada no se reactiva: %s", a)
	}
	if r := o.a.doFile(t, "/api/public-appointments/"+token+"/proof", "", "c.png", tinyPNG); r.Status != 409 {
		t.Fatalf("subir comprobante a una cita cancelada: %d", r.Status)
	}
}

func TestABusinessCannotGrantItselfAPlanOrASubscription(t *testing.T) {
	a := newTestAPI(t)
	r := a.do(t, "POST", "/api/auth/register", "", map[string]any{
		"business_name": "Listo", "kind": "barberia", "name": "Dueño", "email": "listo@x.com", "password": "supersecreta1", "accept_terms": true,
		"plan": "pro", "status": "active", "current_period_end": "2099-01-01T00:00:00Z", "trial_ends_at": "2099-01-01T00:00:00Z"})
	if r.Status != 201 {
		t.Fatalf("registro: %d %v", r.Status, r.Body)
	}
	tok := r.Body["token"].(string)
	state := func() (plan, status string, end, trial *string) {
		a.DB.QueryRow(t.Context(), `SELECT plan, status, current_period_end::text, trial_ends_at::text FROM businesses`).Scan(&plan, &status, &end, &trial)
		return
	}
	plan, status, end, trial := state()
	if plan != "basico" || status != "trial" || end != nil || trial == nil || strings.HasPrefix(*trial, "2099") {
		t.Fatalf("el alta no concede plan ni vigencia: solo arranca en Básico y pide el plan: %s %s %v %v", plan, status, end, trial)
	}
	var requested string
	a.DB.QueryRow(t.Context(), `SELECT COALESCE(requested_plan, '') FROM businesses`).Scan(&requested)
	if requested != "pro" { // pedir el plan es solo una solicitud que la plataforma aprueba o no
		t.Fatalf("el plan pedido debe quedar como solicitud: %q", requested)
	}
	// ni desde los datos del negocio, ni con los endpoints de la plataforma
	a.do(t, "PUT", "/api/business", tok, map[string]any{"name": "Listo", "kind": "barberia", "timezone": "America/Mexico_City", "plan": "pro", "status": "active", "current_period_end": "2099-01-01T00:00:00Z"})
	var id int64
	a.DB.QueryRow(t.Context(), `SELECT id FROM businesses`).Scan(&id)
	for _, c := range []struct {
		method, path string
		body         map[string]any
	}{
		{"PATCH", fmt.Sprintf("/api/platform/businesses/%d", id), map[string]any{"plan": "pro", "status": "active"}},
		{"POST", fmt.Sprintf("/api/platform/businesses/%d/payments", id), map[string]any{"amount_cents": 1, "months": 24, "plan": "pro", "method": "courtesy"}},
		{"GET", "/api/platform/businesses", nil},
	} {
		if r := a.do(t, c.method, c.path, tok, c.body); r.Status == 200 || r.Status == 201 || r.Status == 204 {
			t.Fatalf("un negocio no usa %s %s: %d", c.method, c.path, r.Status)
		}
	}
	if p, s, e, _ := state(); p != "basico" || s != "trial" || e != nil {
		t.Fatalf("el negocio no debe cambiar su propio plan: %s %s %v", p, s, e)
	}
	// el cobro en línea de la suscripción no se acredita sin el pago real ni con un monto elegido por el cliente
	if r := a.do(t, "POST", "/api/billing/checkout", tok, map[string]any{"plan": "pro", "months": 12, "amount_cents": 100}); r.Status == 200 || r.Status == 201 {
		var n int
		a.DB.QueryRow(t.Context(), `SELECT amount_cents FROM subscription_checkouts`).Scan(&n)
		if n == 100 {
			t.Fatal("el monto del cobro lo decide el servidor")
		}
	}
	if p, s, e, _ := state(); p != "basico" || s != "trial" || e != nil {
		t.Fatalf("abrir un cobro no activa la suscripción: %s %s %v", p, s, e)
	}
}
