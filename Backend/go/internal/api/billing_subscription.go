package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"miestudio/backend-go/internal/auth"
)

// Suscripción con cobro automático mensual (preapproval de Mercado Pago, cuenta de la plataforma, MPP_ACCESS_TOKEN).
// El negocio autoriza su tarjeta una sola vez; Mercado Pago cobra cada mes y avisa de cada cargo. Igual que con los pagos
// por periodo, nada se da por bueno por lo que diga el navegador ni un aviso: cada cargo se consulta en Mercado Pago, debe
// estar aprobado por el monto exacto del plan, y se acredita una sola vez (el id del pago es único en `payments`).
// El primer cargo cae cuando termina la prueba o el periodo ya pagado, para no cobrar dos veces.

var depMandateRef = "mie-mandate:"

type mandateRow struct {
	ID, BizID int64
	Plan      string
	Amount    int
	Status    string
	MPID      string
	InitPoint string
}

func loadMandate(ctx context.Context, q queryer, where string, args ...any) (*mandateRow, error) {
	m := &mandateRow{}
	var mpid *string
	err := q.QueryRow(ctx, `SELECT id, business_id, plan, amount_cents, status, mp_preapproval_id, init_point FROM subscription_mandates WHERE `+where, args...).
		Scan(&m.ID, &m.BizID, &m.Plan, &m.Amount, &m.Status, &mpid, &m.InitPoint)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if mpid != nil {
		m.MPID = *mpid
	}
	return m, err
}

// liveMandate: la suscripción vigente del negocio (la más reciente que no esté cancelada).
func (a *API) liveMandate(ctx context.Context, bizID int64) *mandateRow {
	m, _ := loadMandate(ctx, a.DB, `business_id = $1 AND status <> 'cancelled' ORDER BY created_at DESC, id DESC LIMIT 1`, bizID)
	return m
}

func (a *API) hasAuthorizedMandate(ctx context.Context, bizID int64) bool {
	var n int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM subscription_mandates WHERE business_id = $1 AND status = 'authorized'`, bizID).Scan(&n)
	return n > 0
}

func mandateJSON(m *mandateRow) map[string]any {
	if m == nil {
		return nil
	}
	return map[string]any{"id": m.ID, "status": m.Status, "plan": m.Plan, "amount_cents": m.Amount}
}

// cancelAtMP cancela la suscripción en Mercado Pago. Que ya no exista (404) cuenta como cancelada.
func (a *API) cancelAtMP(ctx context.Context, m *mandateRow) error {
	if m.MPID == "" {
		return nil
	}
	err := a.mpCall(ctx, a.PlatformMPToken, http.MethodPut, "/preapproval/"+url.PathEscape(m.MPID), nil, map[string]any{"status": "cancelled"}, nil)
	var me *mpError
	if err != nil && !(errors.As(err, &me) && me.Status == 404) {
		return err
	}
	return nil
}

// startSubscription: el administrador elige un plan y se le manda a autorizar el cobro mensual en Mercado Pago.
func (a *API) startSubscription(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	if !a.onlineBillingEnabled() {
		writeFail(w, &httpFail{http.StatusServiceUnavailable, "El pago en línea no está habilitado. Pide a la plataforma que registre tu pago.", "billing_offline"})
		return
	}
	if !a.allow(w, "subscribe:"+itoa(p.BusinessID), 10, time.Hour) {
		return
	}
	var req struct {
		Plan       string `json:"plan"`
		PayerEmail string `json:"payer_email"` // opcional: el correo de la cuenta de Mercado Pago con la que se va a pagar
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	plan, ok := a.planByID(ctx, req.Plan)
	if !ok {
		writeErr(w, http.StatusBadRequest, "elige un plan")
		return
	}
	req.PayerEmail = strings.ToLower(strings.TrimSpace(req.PayerEmail))
	if req.PayerEmail != "" && !validEmail(req.PayerEmail) {
		writeErr(w, http.StatusBadRequest, "el correo de tu cuenta de Mercado Pago no es válido")
		return
	}
	var status string
	var trialEnds, periodEnd *time.Time
	if err := a.DB.QueryRow(ctx, `SELECT status, trial_ends_at, current_period_end FROM businesses WHERE id = $1`, p.BusinessID).Scan(&status, &trialEnds, &periodEnd); err != nil {
		serverErr(w)
		return
	}
	if status == "pending" || status == "suspended" {
		writeErr(w, http.StatusConflict, "tu cuenta no puede suscribirse en línea ahora; contáctanos")
		return
	}
	email := req.PayerEmail
	if email == "" {
		if err := a.DB.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, p.UserID).Scan(&email); err != nil {
			serverErr(w)
			return
		}
	}

	// Una sola suscripción por negocio: la anterior se cancela primero (si no se pudiera cancelar, no se crea otra: no se cobra doble).
	rows, err := a.DB.Query(ctx, `SELECT id, business_id, plan, amount_cents, status, coalesce(mp_preapproval_id, ''), init_point FROM subscription_mandates WHERE business_id = $1 AND status <> 'cancelled'`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	var old []*mandateRow
	for rows.Next() {
		m := &mandateRow{}
		if rows.Scan(&m.ID, &m.BizID, &m.Plan, &m.Amount, &m.Status, &m.MPID, &m.InitPoint) == nil {
			old = append(old, m)
		}
	}
	rows.Close()
	for _, m := range old {
		if err := a.cancelAtMP(ctx, m); err != nil {
			writeFail(w, providerFail(err))
			return
		}
		a.DB.Exec(ctx, `UPDATE subscription_mandates SET status = 'cancelled', updated_at = $2 WHERE id = $1`, m.ID, a.now())
	}

	amount := plan.PriceCents(1) // el precio sale del servidor, nunca del navegador
	var id int64
	if err := a.DB.QueryRow(ctx,
		`INSERT INTO subscription_mandates (business_id, plan, amount_cents, payer_email, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $6) RETURNING id`, p.BusinessID, plan.ID, amount, email, p.UserID, a.now()).Scan(&id); err != nil {
		serverErr(w)
		return
	}

	// Primer cargo: al terminar la prueba o el periodo vigente; si no queda nada por esperar, al autorizar.
	now := a.now()
	first := now.Add(10 * time.Minute)
	if status == "trial" && trialEnds != nil && trialEnds.After(first) {
		first = *trialEnds
	}
	if status == "active" && periodEnd != nil && periodEnd.After(first) {
		first = *periodEnd
	}
	recurring := map[string]any{
		"frequency": 1, "frequency_type": "months", "transaction_amount": float64(amount) / 100, "currency_id": "MXN",
	}
	if first.After(now.Add(10 * time.Minute)) {
		recurring["start_date"] = first.UTC().Format("2006-01-02T15:04:05.000-07:00")
	}
	ref := depMandateRef + strconv.FormatInt(id, 10)
	body := map[string]any{
		"reason":             firstRunes(fmt.Sprintf("Barbella %s · suscripción mensual", plan.Name), 60), // Mercado Pago rechaza más de 60 caracteres
		"external_reference": ref,
		"payer_email":        email,
		"back_url":           a.AppURL + "/panel/suscripcion?subscription=" + strconv.FormatInt(id, 10),
		"auto_recurring":     recurring,
		"status":             "pending",
	}
	if a.PublicURL != "" {
		body["notification_url"] = a.PublicURL + "/api/webhooks/mercadopago-platform"
	}
	var out struct {
		ID        string `json:"id"`
		InitPoint string `json:"init_point"`
	}
	if err := a.mpCall(ctx, a.PlatformMPToken, http.MethodPost, "/preapproval", nil, body, &out); err != nil || out.ID == "" || out.InitPoint == "" {
		a.DB.Exec(ctx, `UPDATE subscription_mandates SET status = 'cancelled', updated_at = $2 WHERE id = $1`, id, a.now())
		if err == nil {
			err = errors.New("Mercado Pago no devolvió el enlace de la suscripción")
		}
		writeFail(w, providerFail(err))
		return
	}
	if _, err := a.DB.Exec(ctx, `UPDATE subscription_mandates SET mp_preapproval_id = $2, init_point = $3, updated_at = $4 WHERE id = $1`, id, out.ID, out.InitPoint, a.now()); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "pay_url": out.InitPoint, "amount_cents": amount, "first_charge": first})
}

// syncMandate pregunta a Mercado Pago el estado de la suscripción y lo copia (authorized, paused, cancelled o pending).
func (a *API) syncMandate(ctx context.Context, m *mandateRow) (string, error) {
	if m.MPID == "" || !a.onlineBillingEnabled() {
		return m.Status, nil
	}
	var pre struct {
		Status string `json:"status"`
	}
	if err := a.mpCall(ctx, a.PlatformMPToken, http.MethodGet, "/preapproval/"+url.PathEscape(m.MPID), nil, nil, &pre); err != nil {
		return m.Status, providerFail(err)
	}
	st := strings.ToLower(pre.Status)
	switch st {
	case "authorized", "paused", "cancelled", "pending":
	default:
		return m.Status, nil
	}
	if st != m.Status {
		if _, err := a.DB.Exec(ctx, `UPDATE subscription_mandates SET status = $2, updated_at = $3 WHERE id = $1`, m.ID, st, a.now()); err != nil {
			return m.Status, err
		}
		a.logActivity(ctx, 0, m.BizID, "subscription_"+st, map[string]any{"plan": m.Plan, "amount_cents": m.Amount})
	}
	return st, nil
}

// settleMandatePayment acredita un mes por un cargo de la suscripción, una sola vez y solo si está aprobado por el monto exacto.
func (a *API) settleMandatePayment(ctx context.Context, m *mandateRow, payID string) (bool, error) {
	var pay struct {
		mpPayment
		Ref string `json:"external_reference"`
	}
	if err := a.mpCall(ctx, a.PlatformMPToken, http.MethodGet, "/v1/payments/"+url.PathEscape(payID), nil, nil, &pay); err != nil {
		var me *mpError
		if errors.As(err, &me) && me.Status == 404 {
			return false, nil
		}
		return false, err
	}
	if pay.Ref != depMandateRef+strconv.FormatInt(m.ID, 10) || !pay.approvedFor(m.Amount) {
		return false, nil
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	paid, err := a.extendSubscription(ctx, tx, m.BizID, m.Plan, 1, m.Amount, "mercadopago", "Suscripción mensual", payID, nil)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return false, nil // este cargo ya estaba acreditado (aviso repetido)
		}
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE subscription_mandates SET status = CASE WHEN status = 'pending' THEN 'authorized' ELSE status END, updated_at = $2 WHERE id = $1`, m.ID, a.now()); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	a.logActivity(ctx, 0, m.BizID, "payment_recorded", map[string]any{"amount_cents": m.Amount, "months": 1, "plan": m.Plan, "method": "mercadopago", "subscription": true})
	a.notifyPayment(ctx, m.BizID, paid)
	return true, nil
}

// getSubscription: la página lo consulta al volver de Mercado Pago.
func (a *API) getSubscription(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	m, err := loadMandate(r.Context(), a.DB, "id = $1 AND business_id = $2", id, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	if m == nil {
		notFound(w)
		return
	}
	if m.Status == "pending" {
		if st, err := a.syncMandate(r.Context(), m); err == nil {
			m.Status = st
		}
	}
	writeJSON(w, http.StatusOK, mandateJSON(m))
}

// cancelSubscription: deja de cobrar. El negocio conserva el acceso hasta el fin del periodo ya pagado.
func (a *API) cancelSubscription(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	m := a.liveMandate(ctx, p.BusinessID)
	if m == nil {
		writeErr(w, http.StatusNotFound, "no tienes una suscripción con cobro automático")
		return
	}
	if err := a.cancelAtMP(ctx, m); err != nil {
		writeFail(w, providerFail(err))
		return
	}
	if _, err := a.DB.Exec(ctx, `UPDATE subscription_mandates SET status = 'cancelled', updated_at = $2 WHERE id = $1`, m.ID, a.now()); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(ctx, p.UserID, p.BusinessID, "subscription_cancelled", map[string]any{"plan": m.Plan})
	writeJSON(w, http.StatusOK, map[string]any{"status": "cancelled"})
}

// subscriptionWebhook atiende los avisos de la suscripción. Devuelve true si el aviso era de este tipo (ya respondió).
func (a *API) subscriptionWebhook(w http.ResponseWriter, r *http.Request, topic, id string) bool {
	ctx := r.Context()
	switch topic {
	case "subscription_preapproval":
		if id == "" {
			w.WriteHeader(http.StatusOK)
			return true
		}
		m, err := loadMandate(ctx, a.DB, "mp_preapproval_id = $1", id)
		if err != nil {
			serverErr(w)
			return true
		}
		if m != nil {
			if _, err := a.syncMandate(ctx, m); err != nil {
				serverErr(w)
				return true
			}
		}
		w.WriteHeader(http.StatusOK)
		return true
	case "subscription_authorized_payment":
		var ap struct {
			PreapprovalID flexID `json:"preapproval_id"`
			Payment       struct {
				ID flexID `json:"id"`
			} `json:"payment"`
		}
		if id == "" {
			w.WriteHeader(http.StatusOK)
			return true
		}
		if err := a.mpCall(ctx, a.PlatformMPToken, http.MethodGet, "/authorized_payments/"+url.PathEscape(id), nil, nil, &ap); err != nil {
			var me *mpError
			if errors.As(err, &me) && me.Status == 404 {
				w.WriteHeader(http.StatusOK)
				return true
			}
			serverErr(w)
			return true
		}
		m, err := loadMandate(ctx, a.DB, "mp_preapproval_id = $1", string(ap.PreapprovalID))
		if err != nil {
			serverErr(w)
			return true
		}
		if m != nil && ap.Payment.ID != "" {
			if _, err := a.settleMandatePayment(ctx, m, string(ap.Payment.ID)); err != nil {
				log.Printf("suscripción %d: no se pudo acreditar el cargo %s: %v", m.ID, ap.Payment.ID, err)
				serverErr(w)
				return true
			}
		}
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

// firstRunes recorta un texto a n caracteres (no bytes).
func firstRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
