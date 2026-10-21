package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Anticipos de la reserva en línea (Mercado Pago Checkout Pro).
//
// La cita se aparta como «pending_payment» durante holdMinutes. Si el pago se aprueba pasa a «scheduled»;
// si vence, el horario se libera. El dinero nunca se da por cobrado por lo que diga el navegador del cliente:
// siempre se le pregunta a Mercado Pago (por el aviso automático, al volver el cliente a su enlace o al
// vencer el apartado), y solo cuenta un pago aprobado por el monto exacto del anticipo.
// Un pago que llega tarde (apartado vencido, cita cancelada) se devuelve íntegro.

const (
	holdMinutes    = 15
	holdGraceAfter = time.Hour // si no se puede consultar a Mercado Pago, un apartado vencido se libera pasada esta gracia
)

var depRef = regexp.MustCompile(`^mie-dep:([0-9]{1,18})$`)

// flexID acepta el id de un pago como número o texto.
type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	*f = flexID(s)
	return nil
}

type depositRow struct {
	ID, BizID, ApptID int64
	Amount            int
	Status, Ref       string
	PaymentID         string
}

func loadDeposit(ctx context.Context, q queryer, where string, arg any) (*depositRow, error) {
	d := &depositRow{}
	var pid *string
	err := q.QueryRow(ctx,
		`SELECT id, business_id, appointment_id, amount_cents, status, external_ref, mp_payment_id FROM deposits WHERE `+where, arg).
		Scan(&d.ID, &d.BizID, &d.ApptID, &d.Amount, &d.Status, &d.Ref, &pid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if pid != nil {
		d.PaymentID = *pid
	}
	return d, err
}

func (a *API) webhookURL(bizID int64) string {
	if a.PublicURL == "" {
		return ""
	}
	return a.PublicURL + "/api/webhooks/mercadopago?biz=" + strconv.FormatInt(bizID, 10)
}

// startDeposit crea el enlace de pago del anticipo. Si Mercado Pago falla, la reserva se deshace
// (el horario no debe quedar apartado sin forma de pagar).
func (a *API) startDeposit(ctx context.Context, b *pubBiz, apptID int64, apptToken string, amount int, svcName string, holdUntil time.Time) (string, error) {
	dep, err := loadDeposit(ctx, a.DB, "appointment_id = $1", apptID)
	if err != nil || dep == nil {
		return "", errors.New("anticipo no encontrado")
	}
	undo := func() {
		a.DB.Exec(ctx, `UPDATE appointments SET status = 'cancelled' WHERE id = $1 AND status = 'pending_payment'`, apptID)
		a.DB.Exec(ctx, `UPDATE deposits SET status = 'expired' WHERE id = $1 AND status = 'pending'`, dep.ID)
	}
	token, err := a.mpToken(ctx, b.ID)
	if err != nil {
		undo()
		return "", err
	}
	back := a.AppURL + "/cita/" + apptToken
	pref := map[string]any{
		"items": []map[string]any{{
			"title": truncate("Anticipo: "+svcName, 120), "quantity": 1, "currency_id": "MXN", "unit_price": float64(amount) / 100,
		}},
		"external_reference": dep.Ref,
		"back_urls":          map[string]string{"success": back, "failure": back, "pending": back},
		"expires":            true,
		"expiration_date_to": holdUntil.UTC().Format("2006-01-02T15:04:05.000Z"),
	}
	if strings.HasPrefix(a.AppURL, "https://") {
		pref["auto_return"] = "approved" // Mercado Pago solo lo acepta con direcciones https
	}
	if u := a.webhookURL(b.ID); u != "" {
		pref["notification_url"] = u
	}
	var out struct {
		ID        string `json:"id"`
		InitPoint string `json:"init_point"`
	}
	if err := a.mpCall(ctx, token, http.MethodPost, "/checkout/preferences", nil, pref, &out); err != nil || out.InitPoint == "" {
		undo()
		if err == nil {
			err = errors.New("Mercado Pago no devolvió el enlace de pago")
		}
		return "", providerFail(err)
	}
	if _, err := a.DB.Exec(ctx, `UPDATE deposits SET mp_preference_id = $2, pay_url = $3 WHERE id = $1`, dep.ID, out.ID, out.InitPoint); err != nil {
		undo()
		return "", err
	}
	return out.InitPoint, nil
}

type mpPayment struct {
	ID     flexID    `json:"id"`
	Status string    `json:"status"`
	Amount flexFloat `json:"transaction_amount"`
}

func (p mpPayment) approvedFor(amount int) bool {
	return strings.EqualFold(p.Status, "approved") && int(float64(p.Amount)*100+0.5) == amount
}

// refundPayment devuelve un pago completo. «Ya reembolsado» cuenta como éxito.
var alreadyRefunded = regexp.MustCompile(`(?i)already.*refund|ya.*reembols|fully refunded|total.*refund`)

func (a *API) refundPayment(ctx context.Context, token string, id flexID) error {
	err := a.mpCall(ctx, token, http.MethodPost, "/v1/payments/"+url.PathEscape(string(id))+"/refunds", nil, map[string]any{}, nil)
	var me *mpError
	if err != nil && !(errors.As(err, &me) && alreadyRefunded.MatchString(me.Msg)) {
		return err
	}
	return nil
}

// settleDeposit pregunta a Mercado Pago por los pagos del anticipo y actualiza la cita. Es idempotente y
// seguro ante llamadas simultáneas. Devuelve el estado del anticipo al terminar.
func (a *API) settleDeposit(ctx context.Context, depID int64) (string, error) {
	d, err := loadDeposit(ctx, a.DB, "id = $1", depID)
	if err != nil || d == nil {
		return "", err
	}
	if d.Status != "pending" && d.Status != "expired" {
		return d.Status, nil
	}
	token, err := a.mpToken(ctx, d.BizID)
	if err != nil {
		return d.Status, err
	}
	var res struct {
		Results []mpPayment `json:"results"`
	}
	if err := a.mpCall(ctx, token, http.MethodGet, "/v1/payments/search?sort=date_created&criteria=desc&external_reference="+url.QueryEscape(d.Ref), nil, nil, &res); err != nil {
		return d.Status, providerFail(err)
	}
	var paid []mpPayment
	for _, p := range res.Results {
		if p.approvedFor(d.Amount) {
			paid = append(paid, p)
		}
	}
	if len(paid) == 0 {
		return d.Status, nil
	}

	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return d.Status, err
	}
	defer tx.Rollback(ctx)
	var depStatus, apptStatus string
	if err := tx.QueryRow(ctx,
		`SELECT d.status, ap.status FROM deposits d JOIN appointments ap ON ap.id = d.appointment_id WHERE d.id = $1 FOR UPDATE OF d, ap`, d.ID).
		Scan(&depStatus, &apptStatus); err != nil {
		return d.Status, err
	}
	extra := paid
	if depStatus == "pending" && apptStatus == "pending_payment" {
		_, err = tx.Exec(ctx, `UPDATE deposits SET status = 'paid', mp_payment_id = $2, paid_at = $3 WHERE id = $1`, d.ID, string(paid[0].ID), a.now())
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE appointments SET status = 'scheduled', hold_expires_at = NULL WHERE id = $1`, d.ApptID)
		}
		if err != nil {
			return d.Status, err
		}
		if err := a.enqueueMail(ctx, tx, d.ApptID, "booking", false); err != nil {
			return d.Status, err
		}
		if err := tx.Commit(ctx); err != nil {
			return d.Status, err
		}
		a.refundExtras(ctx, token, paid[1:])
		return "paid", nil
	} else if depStatus == "paid" || depStatus == "applied" || depStatus == "kept" || depStatus == "refunded" {
		return depStatus, nil // otro proceso ya lo resolvió
	}
	tx.Rollback(ctx)

	// pago tardío o cita ya cancelada: devolver el dinero
	for _, p := range extra {
		if err := a.refundPayment(ctx, token, p.ID); err != nil {
			log.Printf("anticipo %d: no se pudo devolver el pago %s: %v", d.ID, p.ID, err)
			return d.Status, providerFail(err)
		}
	}
	a.DB.Exec(ctx, `UPDATE deposits SET status = 'refunded', refunded_at = $2, mp_payment_id = $3 WHERE id = $1 AND status IN ('pending', 'expired')`,
		d.ID, a.now(), string(extra[0].ID))
	a.DB.Exec(ctx, `UPDATE appointments SET status = 'cancelled' WHERE id = $1 AND status = 'pending_payment'`, d.ApptID)
	return "refunded", nil
}

func (a *API) refundExtras(ctx context.Context, token string, extra []mpPayment) {
	for _, p := range extra {
		if err := a.refundPayment(ctx, token, p.ID); err != nil {
			log.Printf("pago duplicado %s: no se pudo devolver: %v", p.ID, err)
		}
	}
}

// refundDeposit devuelve un anticipo pagado y no usado. Sin anticipo pagado no hace nada.
func (a *API) refundDeposit(ctx context.Context, apptID int64) error {
	d, err := loadDeposit(ctx, a.DB, "appointment_id = $1", apptID)
	if err != nil {
		return err
	}
	if d == nil || d.Status != "paid" {
		return nil
	}
	token, err := a.mpToken(ctx, d.BizID)
	if err != nil {
		return err
	}
	if err := a.refundPayment(ctx, token, flexID(d.PaymentID)); err != nil {
		return providerFail(err)
	}
	_, err = a.DB.Exec(ctx, `UPDATE deposits SET status = 'refunded', refunded_at = $2 WHERE id = $1 AND status = 'paid'`, d.ID, a.now())
	return err
}

// keepDeposit: una inasistencia se queda con el anticipo.
func (a *API) keepDeposit(ctx context.Context, apptID int64) {
	a.DB.Exec(ctx, `UPDATE deposits SET status = 'kept' WHERE appointment_id = $1 AND status = 'paid'`, apptID)
}

// sweepHolds libera los horarios apartados cuyo plazo venció, pero antes confirma con Mercado Pago que no
// se hayan pagado (un aviso perdido no debe dejar a alguien con dinero cobrado y sin cita).
func (a *API) sweepHolds(ctx context.Context) {
	now := a.now()
	rows, err := a.DB.Query(ctx,
		`SELECT ap.id, ap.hold_expires_at, d.id FROM appointments ap LEFT JOIN deposits d ON d.appointment_id = ap.id
		  WHERE ap.status = 'pending_payment' AND ap.hold_expires_at < $1 ORDER BY ap.hold_expires_at LIMIT 10`, now)
	if err != nil {
		return
	}
	type hold struct {
		appt  int64
		until time.Time
		dep   *int64
	}
	var holds []hold
	for rows.Next() {
		var h hold
		if rows.Scan(&h.appt, &h.until, &h.dep) == nil {
			holds = append(holds, h)
		}
	}
	rows.Close()
	for _, h := range holds {
		if h.dep != nil {
			st, err := a.settleDeposit(ctx, *h.dep)
			if st == "paid" || st == "refunded" {
				continue
			}
			if err != nil && now.Before(h.until.Add(holdGraceAfter)) {
				continue // no se pudo consultar: se reintenta en el siguiente barrido
			}
		}
		a.DB.Exec(ctx, `UPDATE appointments SET status = 'cancelled' WHERE id = $1 AND status = 'pending_payment'`, h.appt)
		if h.dep != nil {
			a.DB.Exec(ctx, `UPDATE deposits SET status = 'expired' WHERE id = $1 AND status = 'pending'`, *h.dep)
		}
	}
}

// recheckRecentlyExpired revisa los anticipos vencidos hace poco: un pago puede llegar después del apartado
// (el cliente tardó en la pantalla de Mercado Pago) y hay que devolverlo.
func (a *API) recheckRecentlyExpired(ctx context.Context) {
	rows, err := a.DB.Query(ctx,
		`SELECT id FROM deposits WHERE status = 'expired' AND created_at > $1 ORDER BY id DESC LIMIT 20`, a.now().Add(-2*time.Hour))
	if err != nil {
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if _, err := a.settleDeposit(ctx, id); err != nil {
			log.Printf("anticipo %d: %v", id, err)
		}
	}
}

// ---------- aviso automático de Mercado Pago ----------

// verifyWebhookSignature valida el encabezado x-signature (https://www.mercadopago.com.mx/developers/es/docs/your-integrations/notifications/webhooks).
func verifyWebhookSignature(secret, header, requestID, dataID string) bool {
	var ts, v1 string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "ts":
			ts = kv[1]
		case "v1":
			v1 = kv[1]
		}
	}
	if ts == "" || v1 == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "id:%s;request-id:%s;ts:%s;", strings.ToLower(dataID), requestID, ts)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(strings.ToLower(v1)))
}

func (a *API) mpWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "mp-webhook:"+clientIP(r), 300, time.Minute) {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	var payload struct {
		Type string `json:"type"`
		Data struct {
			ID flexID `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(body, &payload)
	q := r.URL.Query()
	topic := firstNonEmpty(payload.Type, q.Get("type"), q.Get("topic"))
	payID := string(payload.Data.ID)
	if v := q.Get("data.id"); v != "" {
		payID = v
	} else if payID == "" {
		payID = q.Get("id")
	}
	if a.MPWebhookSecret != "" && !verifyWebhookSignature(a.MPWebhookSecret, r.Header.Get("x-signature"), r.Header.Get("x-request-id"), payID) {
		writeErr(w, http.StatusUnauthorized, "firma inválida")
		return
	}
	bizID, _ := strconv.ParseInt(q.Get("biz"), 10, 64)
	if topic != "payment" || payID == "" || bizID <= 0 {
		w.WriteHeader(http.StatusOK) // otros avisos no nos interesan
		return
	}
	token, err := a.mpToken(ctx, bizID)
	if err != nil {
		w.WriteHeader(http.StatusOK) // negocio sin cuenta conectada: nada que hacer
		return
	}
	var pay struct {
		Ref string `json:"external_reference"`
	}
	if err := a.mpCall(ctx, token, http.MethodGet, "/v1/payments/"+url.PathEscape(payID), nil, nil, &pay); err != nil {
		var me *mpError
		if errors.As(err, &me) && me.Status == 404 {
			w.WriteHeader(http.StatusOK)
			return
		}
		serverErr(w) // error transitorio: Mercado Pago reintenta
		return
	}
	m := depRef.FindStringSubmatch(pay.Ref)
	if m == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	depID, _ := strconv.ParseInt(m[1], 10, 64)
	d, err := loadDeposit(ctx, a.DB, "id = $1", depID)
	if err != nil {
		serverErr(w)
		return
	}
	if d == nil || d.BizID != bizID { // el aviso de un negocio no puede tocar anticipos de otro
		w.WriteHeader(http.StatusOK)
		return
	}
	if _, err := a.settleDeposit(ctx, d.ID); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// refundAppliedDeposit devuelve el anticipo que una venta había descontado, al cancelarse la venta.
// Es idempotente: si ya se devolvió no hace nada.
func (a *API) refundAppliedDeposit(ctx context.Context, bizID, saleID int64) error {
	d, err := loadDeposit(ctx, a.DB, "sale_id = $1 AND business_id = "+strconv.FormatInt(bizID, 10), saleID)
	if err != nil || d == nil || d.Status != "applied" {
		return err
	}
	token, err := a.mpToken(ctx, d.BizID)
	if err != nil {
		return err
	}
	if err := a.refundPayment(ctx, token, flexID(d.PaymentID)); err != nil {
		return providerFail(err)
	}
	_, err = a.DB.Exec(ctx, `UPDATE deposits SET status = 'refunded', refunded_at = $2 WHERE id = $1 AND status = 'applied'`, d.ID, a.now())
	return err
}
