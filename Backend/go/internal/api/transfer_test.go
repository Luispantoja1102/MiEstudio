package api

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// png mínimo: la firma basta para que se reconozca como imagen
var tinyPNG = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)

func (a *API) doFile(t *testing.T, path, token string, name string, content []byte) resp {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	fw.Write(content)
	mw.Close()
	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.Routes().ServeHTTP(rec, req)
	out := resp{Status: rec.Code, Raw: rec.Body.Bytes()}
	return out
}

// newTransfer: reserva en línea sin Mercado Pago, con anticipo del 30 % por transferencia ($45 de un corte de $150).
func newTransfer(t *testing.T) *onlineFixture {
	t.Helper()
	o := newOnline(t)
	o.transferSettings(t, 200, nil)
	return o
}

func (o *onlineFixture) transferSettings(t *testing.T, want int, mod map[string]any) resp {
	t.Helper()
	body := map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24,
		"show_prices": true, "deposit_percent": 30, "transfer_enabled": true, "transfer_bank": "BBVA", "transfer_holder": "Barbería Don Luis",
		"transfer_clabe": "012345678901234568", "transfer_whatsapp": "664 123 4567", "transfer_hold_hours": 12}
	for k, v := range mod {
		body[k] = v
	}
	r := o.a.do(t, "PUT", "/api/booking/settings", o.admin, body)
	if want == 200 && r.Status != 204 {
		t.Fatalf("ajustes de transferencia: %d %v", r.Status, r.Body)
	}
	if want != 200 && r.Status != want {
		t.Fatalf("ajustes: esperaba %d y fue %d %v", want, r.Status, r.Body)
	}
	return r
}

func (o *onlineFixture) apptID(t *testing.T, token string) int64 {
	t.Helper()
	var id int64
	if err := o.a.DB.QueryRow(t.Context(), `SELECT id FROM appointments WHERE public_token = $1`, token).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (o *onlineFixture) status(t *testing.T, token string) (appt, dep string) {
	t.Helper()
	o.a.DB.QueryRow(t.Context(), `SELECT ap.status, d.status FROM appointments ap JOIN deposits d ON d.appointment_id = ap.id WHERE ap.public_token = $1`, token).Scan(&appt, &dep)
	return
}

func TestTransferSettingsAreValidated(t *testing.T) {
	o := newOnline(t)
	o.transferSettings(t, 400, map[string]any{"transfer_clabe": "123"})
	o.transferSettings(t, 400, map[string]any{"transfer_bank": ""})
	o.transferSettings(t, 400, map[string]any{"transfer_whatsapp": "abc"})
	o.transferSettings(t, 400, map[string]any{"transfer_hold_hours": 999})
	// con una CLABE escrita con espacios se guardan solo los dígitos
	o.transferSettings(t, 200, map[string]any{"transfer_clabe": "0123 4567 8901 2345 68"})
	got := o.a.do(t, "GET", "/api/booking/settings", o.admin, nil).Body["settings"].(map[string]any)
	if got["transfer_clabe"] != "012345678901234568" {
		t.Fatalf("clabe: %v", got["transfer_clabe"])
	}
	// sin Mercado Pago ni transferencias no hay anticipo posible
	o.transferSettings(t, 409, map[string]any{"transfer_enabled": false})
}

// El titular puede dar CLABE, tarjeta o las dos; con ninguna no hay a dónde transferir.
func TestTransferCardAndClabeCombinations(t *testing.T) {
	o := newOnline(t)
	// un dígito verificador equivocado se rechaza: casi siempre es un error de captura
	o.transferSettings(t, 400, map[string]any{"transfer_clabe": "012345678901234567"})
	// ninguna de las dos
	o.transferSettings(t, 400, map[string]any{"transfer_clabe": "", "transfer_card": ""})
	// tarjeta inválida (no pasa Luhn) y con longitud incorrecta
	o.transferSettings(t, 400, map[string]any{"transfer_clabe": "", "transfer_card": "4111111111111112"})
	o.transferSettings(t, 400, map[string]any{"transfer_clabe": "", "transfer_card": "41111111111"})
	// solo tarjeta
	o.transferSettings(t, 200, map[string]any{"transfer_clabe": "", "transfer_card": "4111 1111 1111 1111"})
	got := o.a.do(t, "GET", "/api/booking/settings", o.admin, nil).Body["settings"].(map[string]any)
	if got["transfer_card"] != "4111111111111111" || got["transfer_clabe"] != "" {
		t.Fatalf("solo tarjeta: %v %v", got["transfer_card"], got["transfer_clabe"])
	}
	// una reserva por transferencia muestra la tarjeta y no una CLABE vacía
	token := o.mustBook(t, at(monday, "10:00"), nil)
	tr := o.a.do(t, "GET", "/api/public-appointments/"+token, "", nil).Body["deposit"].(map[string]any)["transfer"].(map[string]any)
	if tr["card"] != "4111111111111111" || tr["clabe"] != "" {
		t.Fatalf("datos de transferencia con solo tarjeta: %v", tr)
	}
	// las dos
	o.transferSettings(t, 200, map[string]any{"transfer_card": "4111111111111111"})
	info := o.a.do(t, "GET", "/api/transfer-info", o.admin, nil).Body
	if info["card"] != "4111111111111111" || info["clabe"] != "012345678901234568" {
		t.Fatalf("transfer-info: %v", info)
	}
	// 15 dígitos (American Express)
	o.transferSettings(t, 200, map[string]any{"transfer_clabe": "", "transfer_card": "378282246310005"})
}

func TestTransferBookingProofAndOwnerConfirmation(t *testing.T) {
	o := newTransfer(t)
	info := o.a.do(t, "GET", "/api/public/"+o.slug, "", nil)
	if m, _ := info.Body["deposit_methods"].([]any); len(m) != 1 || m[0] != "transfer" {
		t.Fatalf("métodos de anticipo: %v", info.Body["deposit_methods"])
	}
	r := o.book(t, at(monday, "10:00"), nil)
	if r.Status != 201 || r.Body["status"] != "pending_payment" || r.Body["pay_method"] != "transfer" || r.Body["pay_url"] != nil {
		t.Fatalf("reserva por transferencia: %d %v", r.Status, r.Body)
	}
	token := r.Body["token"].(string)
	id := o.apptID(t, token)

	// la cita pública muestra a dónde transferir y con qué concepto
	view := o.a.do(t, "GET", "/api/public-appointments/"+token, "", nil)
	dep := view.Body["deposit"].(map[string]any)
	tr := dep["transfer"].(map[string]any)
	if dep["method"] != "transfer" || tr["clabe"] != "012345678901234568" || tr["reference"] != fmt.Sprintf("CITA-%d", id) || tr["whatsapp"] != "526641234567" || tr["proof_uploaded"] != false {
		t.Fatalf("datos de transferencia: %v", dep)
	}
	if inbox := o.a.do(t, "GET", "/api/inbox", o.admin, nil); inbox.Body["unread"].(float64) != 1 {
		t.Fatalf("la reserva debe avisar en la campana: %v", inbox.Body)
	}

	// archivos que no sirven
	if r := o.a.doFile(t, "/api/public-appointments/"+token+"/proof", "", "nota.txt", []byte("hola")); r.Status != 415 {
		t.Fatalf("un texto no es comprobante: %d", r.Status)
	}
	if r := o.a.doFile(t, "/api/public-appointments/"+token+"/proof", "", "grande.png", append(tinyPNG, bytes.Repeat([]byte{1}, maxProofBytes)...)); r.Status != 413 {
		t.Fatalf("más de 5 MB: %d", r.Status)
	}
	if o.a.do(t, "POST", "/api/appointments/"+fmt.Sprint(id)+"/transfer/confirm", o.admin, nil).Status != 204 {
		// confirmar sin comprobante es posible (el titular lo vio en su banco): se prueba abajo con otra cita
		t.Fatal("el titular puede confirmar viendo su banco, sin comprobante")
	}
	if a, d := o.status(t, token); a != "scheduled" || d != "paid" {
		t.Fatalf("tras confirmar: %s / %s", a, d)
	}
}

func TestTransferProofReviewRejectAndConfirm(t *testing.T) {
	o := newTransfer(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	id := o.apptID(t, token)
	proofURL := "/api/public-appointments/" + token + "/proof"
	if r := o.a.doFile(t, proofURL, "", "comprobante.png", tinyPNG); r.Status != 204 {
		t.Fatalf("subir comprobante: %d %s", r.Status, r.Raw)
	}
	inbox := o.a.do(t, "GET", "/api/inbox", o.admin, nil)
	if inbox.Body["to_review"].(float64) != 1 || inbox.Body["items"].([]any)[0].(map[string]any)["kind"] != "proof" {
		t.Fatalf("la campana debe pedir revisar el comprobante: %v", inbox.Body)
	}
	link := inbox.Body["items"].([]any)[0].(map[string]any)["link"].(string)
	if !strings.HasPrefix(link, fmt.Sprintf("/panel/agenda?cita=%d&date=2026-11-09", id)) {
		t.Fatalf("enlace del aviso: %s", link)
	}

	// el archivo solo lo ve administración o recepción
	proof := fmt.Sprintf("/api/appointments/%d/proof", id)
	o.a.addMember(t, o.admin, "recep@x.com", "reception")
	o.a.addMember(t, o.admin, "barbero@x.com", "staff")
	recep, barbero := login(t, o.a, "recep@x.com"), login(t, o.a, "barbero@x.com")
	if g := o.a.do(t, "GET", proof, o.admin, nil); g.Status != 200 || !bytes.Equal(g.Raw, tinyPNG) {
		t.Fatalf("el titular ve el comprobante: %d", g.Status)
	}
	if o.a.do(t, "GET", proof, recep, nil).Status != 200 {
		t.Fatal("recepción ve el comprobante")
	}
	if o.a.do(t, "GET", proof, barbero, nil).Status != 403 {
		t.Fatal("el personal no ve comprobantes")
	}
	// recepción no confirma: lo hace el titular
	confirm := fmt.Sprintf("/api/appointments/%d/transfer/confirm", id)
	if r := o.a.do(t, "POST", confirm, recep, nil); r.Status != 403 {
		t.Fatalf("recepción no confirma pagos: %d", r.Status)
	}

	// rechazo: exige motivo, borra el comprobante y deja subir otro
	reject := fmt.Sprintf("/api/appointments/%d/transfer/reject", id)
	if r := o.a.do(t, "POST", reject, o.admin, map[string]any{"reason": ""}); r.Status != 400 {
		t.Fatalf("rechazo sin motivo: %d", r.Status)
	}
	if r := o.a.do(t, "POST", reject, o.admin, map[string]any{"reason": "El monto no coincide"}); r.Status != 204 {
		t.Fatalf("rechazar: %d %v", r.Status, r.Body)
	}
	tr := o.a.do(t, "GET", "/api/public-appointments/"+token, "", nil).Body["deposit"].(map[string]any)["transfer"].(map[string]any)
	if tr["proof_uploaded"] != false || tr["reject_reason"] != "El monto no coincide" {
		t.Fatalf("el cliente debe ver el motivo: %v", tr)
	}
	if o.a.do(t, "GET", proof, o.admin, nil).Status != 404 {
		t.Fatal("el comprobante rechazado se borra")
	}
	if r := o.a.doFile(t, proofURL, "", "otro.png", tinyPNG); r.Status != 204 {
		t.Fatalf("subir otro: %d", r.Status)
	}
	// la agenda marca que hay un comprobante por revisar
	day := o.a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s", o.branch, monday), o.admin, nil)
	row := day.Body["appointments"].([]any)[0].(map[string]any)
	if row["deposit_method"] != "transfer" || row["proof_at"] == nil || row["reject_reason"] != nil {
		t.Fatalf("fila de la agenda: %v", row)
	}
	if r := o.a.do(t, "POST", confirm, o.admin, nil); r.Status != 204 {
		t.Fatalf("confirmar: %d %v", r.Status, r.Body)
	}
	if a, d := o.status(t, token); a != "scheduled" || d != "paid" {
		t.Fatalf("tras confirmar: %s / %s", a, d)
	}
	if r := o.a.do(t, "POST", confirm, o.admin, nil); r.Status != 409 {
		t.Fatalf("confirmar dos veces: %d", r.Status)
	}
	if r := o.a.doFile(t, proofURL, "", "tarde.png", tinyPNG); r.Status != 409 {
		t.Fatalf("ya pagada no espera comprobante: %d", r.Status)
	}
}

func TestTransferHoldExpiryRespectsAProofUnderReview(t *testing.T) {
	o := newTransfer(t)
	sin := o.mustBook(t, at(monday, "10:00"), nil)
	con := o.mustBook(t, at(monday, "11:00"), map[string]any{"phone": "6641110002", "email": "otro@x.com"})
	o.a.doFile(t, "/api/public-appointments/"+con+"/proof", "", "c.png", tinyPNG)
	// pasan 13 h: el plazo sin comprobante (12 h) venció; el comprobante en revisión conserva su horario (48 h)
	o.a.Now = func() time.Time { return time.Date(2026, 11, 2, 13, 0, 0, 0, time.UTC) }
	o.a.sweepHolds(t.Context())
	if a, d := o.status(t, sin); a != "cancelled" || d != "expired" {
		t.Fatalf("sin comprobante: %s / %s", a, d)
	}
	if a, d := o.status(t, con); a != "pending_payment" || d != "pending" {
		t.Fatalf("con comprobante en revisión: %s / %s", a, d)
	}
	var kinds []string
	rows, _ := o.a.DB.Query(t.Context(), `SELECT kind FROM inbox ORDER BY id`)
	for rows.Next() {
		var k string
		rows.Scan(&k)
		kinds = append(kinds, k)
	}
	if !strings.Contains(strings.Join(kinds, ","), "expired") {
		t.Fatalf("la campana avisa del vencimiento: %v", kinds)
	}
}

func TestCancelledPaidTransferLeavesARefundForTheBusiness(t *testing.T) {
	o := newTransfer(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	id := o.apptID(t, token)
	o.a.do(t, "POST", fmt.Sprintf("/api/appointments/%d/transfer/confirm", id), o.admin, nil)
	if r := o.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil); r.Status != 200 {
		t.Fatalf("cancelar: %d %v", r.Status, r.Body)
	}
	var st string
	var due bool
	o.a.DB.QueryRow(t.Context(), `SELECT status, refund_due FROM deposits WHERE appointment_id = $1`, id).Scan(&st, &due)
	if st != "paid" || !due {
		t.Fatalf("una transferencia no se devuelve sola: %s %v", st, due)
	}
	day := o.a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s&days=1", o.branch, monday), o.admin, nil)
	_ = day
	refunded := fmt.Sprintf("/api/appointments/%d/transfer/refunded", id)
	if r := o.a.do(t, "POST", refunded, o.admin, nil); r.Status != 204 {
		t.Fatalf("marcar devuelto: %d", r.Status)
	}
	o.a.DB.QueryRow(t.Context(), `SELECT status, refund_due FROM deposits WHERE appointment_id = $1`, id).Scan(&st, &due)
	if st != "refunded" || due {
		t.Fatalf("tras devolver: %s %v", st, due)
	}
	if r := o.a.do(t, "POST", refunded, o.admin, nil); r.Status != 409 {
		t.Fatalf("devolver dos veces: %d", r.Status)
	}
}

func TestStaffCanAskForATransferDepositFromTheAgendaAndAttachTheProof(t *testing.T) {
	o := newTransfer(t)
	start := at(monday, "12:00")
	req := func(extra map[string]any) resp {
		b := map[string]any{"client_id": o.cl, "service_id": o.svc, "staff_id": o.marco, "branch_id": o.branch, "starts_at": start}
		for k, v := range extra {
			b[k] = v
		}
		return o.a.do(t, "POST", "/api/appointments", o.admin, b)
	}
	if r := req(map[string]any{"deposit_cents": 99}); r.Status != 400 {
		t.Fatalf("anticipo menor a $1: %d", r.Status)
	}
	if r := req(map[string]any{"deposit_cents": 99999}); r.Status != 400 {
		t.Fatalf("anticipo mayor al precio: %d", r.Status)
	}
	r := req(map[string]any{"deposit_cents": 5000})
	if r.Status != 201 || r.Body["status"] != "pending_payment" || r.Body["manage_path"] == nil {
		t.Fatalf("cita con anticipo: %d %v", r.Status, r.Body)
	}
	id := int64(r.Body["id"].(float64))
	// el horario queda apartado
	if r2 := o.a.do(t, "POST", "/api/appointments", o.admin, map[string]any{"client_id": o.cl, "service_id": o.svc, "staff_id": o.marco, "branch_id": o.branch, "starts_at": start}); r2.Status != 409 {
		t.Fatalf("el horario apartado no se puede duplicar: %d", r2.Status)
	}
	// el personal adjunta el comprobante que llegó por WhatsApp
	if r := o.a.doFile(t, fmt.Sprintf("/api/appointments/%d/proof", id), o.admin, "wa.png", tinyPNG); r.Status != 204 {
		t.Fatalf("adjuntar comprobante: %d %s", r.Status, r.Raw)
	}
	if r := o.a.do(t, "POST", fmt.Sprintf("/api/appointments/%d/transfer/confirm", id), o.admin, nil); r.Status != 204 {
		t.Fatalf("confirmar: %d", r.Status)
	}
	var st string
	o.a.DB.QueryRow(t.Context(), `SELECT status FROM appointments WHERE id = $1`, id).Scan(&st)
	if st != "scheduled" {
		t.Fatalf("estado final: %s", st)
	}
	// sin transferencias activadas no se puede pedir
	o.transferSettings(t, 200, map[string]any{"transfer_enabled": false, "deposit_percent": 0})
	if r := o.a.do(t, "POST", "/api/appointments", o.admin, map[string]any{"client_id": o.cl, "service_id": o.svc, "staff_id": o.marco, "branch_id": o.branch, "starts_at": at(monday, "13:00"), "deposit_cents": 5000}); r.Status != 409 {
		t.Fatalf("sin transferencias activas: %d", r.Status)
	}
}

func TestInboxIsPerPersonAndOnlyForAdministration(t *testing.T) {
	o := newOnline(t)
	o.mustBook(t, at(monday, "10:00"), nil) // sin anticipo: «Nueva reserva en línea»
	o.a.addMember(t, o.admin, "recep@x.com", "reception")
	o.a.addMember(t, o.admin, "barbero@x.com", "staff")
	recep, barbero := login(t, o.a, "recep@x.com"), login(t, o.a, "barbero@x.com")
	if o.a.do(t, "GET", "/api/inbox", barbero, nil).Status != 403 {
		t.Fatal("el personal no tiene campana de administración")
	}
	get := func(tok string) map[string]any { return o.a.do(t, "GET", "/api/inbox", tok, nil).Body }
	if get(o.admin)["unread"].(float64) != 1 || get(recep)["unread"].(float64) != 1 {
		t.Fatal("cada quien ve el aviso sin leer")
	}
	if r := o.a.do(t, "POST", "/api/inbox/read", o.admin, nil); r.Status != 204 {
		t.Fatalf("marcar leído: %d", r.Status)
	}
	if get(o.admin)["unread"].(float64) != 0 || get(recep)["unread"].(float64) != 1 {
		t.Fatal("leerlo no afecta a las demás personas")
	}
	// otro negocio no ve los avisos de este
	other := o.a.mustRegister(t, "Otro", "otro@x.com")
	if items := o.a.do(t, "GET", "/api/inbox", other, nil).Body["items"].([]any); len(items) != 0 {
		t.Fatalf("aislamiento entre negocios: %v", items)
	}
	o.a.do(t, "POST", "/api/public-appointments/"+o.latestToken(t)+"/cancel", "", nil)
	if get(o.admin)["unread"].(float64) != 1 {
		t.Fatal("la cancelación del cliente avisa")
	}
}

func (o *onlineFixture) latestToken(t *testing.T) string {
	t.Helper()
	var tok string
	o.a.DB.QueryRow(t.Context(), `SELECT public_token FROM appointments WHERE public_token IS NOT NULL ORDER BY id DESC LIMIT 1`).Scan(&tok)
	return tok
}

var _ = http.StatusOK

func httpGet(t *testing.T, a *API, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.Routes().ServeHTTP(rec, req)
	return rec
}
