package api

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
	"miestudio/backend-go/internal/mail"
)

// Suscripción: el administrador de un negocio ve su plan y paga con Mercado Pago (Checkout Pro, cuenta de la plataforma,
// MPP_ACCESS_TOKEN). Como con los anticipos, el pago nunca se da por bueno por lo que diga el navegador: se confirma
// con Mercado Pago (aviso, regreso a la página o consulta), por el monto exacto del plan y una sola vez.
// El cobro automático recurrente (preapproval) queda para después; hoy se paga cada periodo (1 o 12 meses).

var depSubRef = "mie-sub:"

func (a *API) onlineBillingEnabled() bool { return a.PlatformMPToken != "" }

type checkoutRow struct {
	ID, BizID int64
	Plan      string
	Months    int
	Amount    int
	Status    string
	Ref       string
	PayURL    string
}

func loadCheckout(ctx context.Context, q queryer, where string, args ...any) (*checkoutRow, error) {
	c := &checkoutRow{}
	err := q.QueryRow(ctx, `SELECT id, business_id, plan, months, amount_cents, status, external_ref, pay_url FROM subscription_checkouts WHERE `+where, args...).
		Scan(&c.ID, &c.BizID, &c.Plan, &c.Months, &c.Amount, &c.Status, &c.Ref, &c.PayURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

// billingAdmin: solo el administrador de un negocio (aun con la suscripción vencida: es justo lo que necesita).
func (a *API) billingAdmin(h http.HandlerFunc) http.Handler {
	return a.authed()(auth.RequireRole("admin")(h))
}

func (a *API) getBilling(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	var plan, status string
	var trialEnds, periodEnd *time.Time
	if err := a.DB.QueryRow(ctx, `SELECT plan, status, trial_ends_at, current_period_end FROM businesses WHERE id = $1`, p.BusinessID).Scan(&plan, &status, &trialEnds, &periodEnd); err != nil {
		serverErr(w)
		return
	}
	cur, _ := planByID(plan)
	writeJSON(w, http.StatusOK, map[string]any{
		"plan": cur, "billing_state": p.BillingState, "trial_ends_at": trialEnds, "current_period_end": periodEnd,
		"plans": planCatalog, "usage": a.usageOf(ctx, a.DB, p.BusinessID), "payments": a.paymentsOf(ctx, p.BusinessID, 24),
		"online": a.onlineBillingEnabled(), "grace_days": pastDueGraceDays,
	})
}

func (a *API) startCheckout(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	if !a.onlineBillingEnabled() {
		writeFail(w, &httpFail{http.StatusServiceUnavailable, "El pago en línea no está habilitado. Pide a la plataforma que registre tu pago.", "billing_offline"})
		return
	}
	if !a.allow(w, "checkout:"+itoa(p.BusinessID), 10, time.Hour) {
		return
	}
	var req struct {
		Plan   string `json:"plan"`
		Months int    `json:"months"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	plan, ok := planByID(req.Plan)
	if !ok || (req.Months != 1 && req.Months != 12) {
		writeErr(w, http.StatusBadRequest, "elige un plan y 1 o 12 meses")
		return
	}
	var status string
	a.DB.QueryRow(ctx, `SELECT status FROM businesses WHERE id = $1`, p.BusinessID).Scan(&status)
	if status == "pending" || status == "suspended" {
		writeErr(w, http.StatusConflict, "tu cuenta no puede pagar en línea ahora; contáctanos")
		return
	}
	amount := plan.PriceCents(req.Months) // el precio sale del servidor, nunca del navegador
	var id int64
	if err := a.DB.QueryRow(ctx,
		`INSERT INTO subscription_checkouts (business_id, plan, months, amount_cents, external_ref, created_by, created_at)
		 VALUES ($1, $2, $3, $4, 'tmp-' || gen_random_uuid(), $5, $6) RETURNING id`, p.BusinessID, plan.ID, req.Months, amount, p.UserID, a.now()).Scan(&id); err != nil {
		serverErr(w)
		return
	}
	ref := depSubRef + strconv.FormatInt(id, 10)
	back := a.AppURL + "/panel/suscripcion?checkout=" + strconv.FormatInt(id, 10)
	title := fmt.Sprintf("Barbella %s · %s", plan.Name, plural(req.Months, "mes", "meses"))
	pref := map[string]any{
		"items":              []map[string]any{{"title": title, "quantity": 1, "currency_id": "MXN", "unit_price": float64(amount) / 100}},
		"external_reference": ref,
		"back_urls":          map[string]string{"success": back, "failure": back, "pending": back},
	}
	if strings.HasPrefix(a.AppURL, "https://") {
		pref["auto_return"] = "approved"
	}
	if a.PublicURL != "" {
		pref["notification_url"] = a.PublicURL + "/api/webhooks/mercadopago-platform"
	}
	var out struct {
		ID        string `json:"id"`
		InitPoint string `json:"init_point"`
	}
	if err := a.mpCall(ctx, a.PlatformMPToken, http.MethodPost, "/checkout/preferences", nil, pref, &out); err != nil || out.InitPoint == "" {
		a.DB.Exec(ctx, `UPDATE subscription_checkouts SET status = 'expired', external_ref = $2 WHERE id = $1`, id, ref)
		if err == nil {
			err = errors.New("Mercado Pago no devolvió el enlace de pago")
		}
		writeFail(w, providerFail(err))
		return
	}
	if _, err := a.DB.Exec(ctx, `UPDATE subscription_checkouts SET external_ref = $2, pay_url = $3 WHERE id = $1`, id, ref, out.InitPoint); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "pay_url": out.InitPoint, "amount_cents": amount})
}

// settleCheckout pregunta a Mercado Pago por los pagos del cobro y acredita el periodo una sola vez.
func (a *API) settleCheckout(ctx context.Context, id int64) (string, error) {
	c, err := loadCheckout(ctx, a.DB, "id = $1", id)
	if err != nil || c == nil {
		return "", err
	}
	if c.Status == "paid" || !a.onlineBillingEnabled() {
		return c.Status, nil
	}
	var res struct {
		Results []mpPayment `json:"results"`
	}
	if err := a.mpCall(ctx, a.PlatformMPToken, http.MethodGet, "/v1/payments/search?sort=date_created&criteria=desc&external_reference="+url.QueryEscape(c.Ref), nil, nil, &res); err != nil {
		return c.Status, providerFail(err)
	}
	var paid []mpPayment
	for _, p := range res.Results {
		if p.approvedFor(c.Amount) {
			paid = append(paid, p)
		}
	}
	if len(paid) == 0 {
		return c.Status, nil
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return c.Status, err
	}
	defer tx.Rollback(ctx)
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM subscription_checkouts WHERE id = $1 FOR UPDATE`, id).Scan(&status); err != nil {
		return c.Status, err
	}
	if status == "paid" {
		return "paid", nil
	}
	pay, err := a.extendSubscription(ctx, tx, c.BizID, c.Plan, c.Months, c.Amount, "mercadopago", "Pago en línea", string(paid[0].ID), nil)
	if err != nil {
		return c.Status, err
	}
	if _, err := tx.Exec(ctx, `UPDATE subscription_checkouts SET status = 'paid', mp_payment_id = $2, paid_at = $3 WHERE id = $1`, id, string(paid[0].ID), a.now()); err != nil {
		return c.Status, err
	}
	if err := tx.Commit(ctx); err != nil {
		return c.Status, err
	}
	a.logActivity(ctx, 0, c.BizID, "payment_recorded", map[string]any{"amount_cents": c.Amount, "months": c.Months, "plan": c.Plan, "method": "mercadopago"})
	a.notifyPayment(ctx, c.BizID, pay)
	for _, extra := range paid[1:] { // un segundo pago por lo mismo se devuelve
		if err := a.refundPayment(ctx, a.PlatformMPToken, extra.ID); err != nil {
			log.Printf("suscripción %d: no se pudo devolver el pago duplicado %s: %v", id, extra.ID, err)
		}
	}
	return "paid", nil
}

// getCheckout: la página de suscripción lo consulta al volver de Mercado Pago.
func (a *API) getCheckout(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := loadCheckout(r.Context(), a.DB, "id = $1 AND business_id = $2", id, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	if c == nil {
		notFound(w)
		return
	}
	if c.Status == "pending" {
		if st, err := a.settleCheckout(r.Context(), c.ID); err == nil {
			c.Status = st
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": c.ID, "status": c.Status, "plan": c.Plan, "months": c.Months, "amount_cents": c.Amount})
}

// platformWebhook: aviso de Mercado Pago de un pago de suscripción.
func (a *API) platformMPWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "mpp-webhook:"+clientIP(r), 300, time.Minute) {
		return
	}
	topic, payID, ok := a.parseWebhook(w, r, a.PlatformWebhookSecret)
	if !ok {
		return
	}
	if topic != "payment" || payID == "" || !a.onlineBillingEnabled() {
		w.WriteHeader(http.StatusOK)
		return
	}
	var pay struct {
		Ref string `json:"external_reference"`
	}
	if err := a.mpCall(ctx, a.PlatformMPToken, http.MethodGet, "/v1/payments/"+url.PathEscape(payID), nil, nil, &pay); err != nil {
		var me *mpError
		if errors.As(err, &me) && me.Status == 404 {
			w.WriteHeader(http.StatusOK)
			return
		}
		serverErr(w)
		return
	}
	rest, found := strings.CutPrefix(pay.Ref, depSubRef)
	id, err := strconv.ParseInt(rest, 10, 64)
	if !found || err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	if _, err := a.settleCheckout(ctx, id); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ---------- avisos ----------

// teamRecipients: a quién avisa la plataforma de sus propios asuntos (pagos). PLATFORM_NOTICE_EMAIL, o los administradores de plataforma.
func (a *API) teamRecipients(ctx context.Context) []string {
	if a.PlatformNoticeTo != "" {
		return []string{a.PlatformNoticeTo}
	}
	rows, err := a.DB.Query(ctx, `SELECT email FROM users WHERE role = 'platform_admin' AND active`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var e string
		if rows.Scan(&e) == nil && strings.Contains(e, "@") && !strings.HasSuffix(e, ".local") {
			out = append(out, e)
		}
	}
	return out
}

func (a *API) businessAdmins(ctx context.Context, bizID int64) []string {
	rows, err := a.DB.Query(ctx, `SELECT email FROM users WHERE business_id = $1 AND role = 'admin' AND active`, bizID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var e string
		if rows.Scan(&e) == nil {
			out = append(out, e)
		}
	}
	return out
}

func moneyES(cents int) string { return "$" + cents2(cents) + " MXN" }

func cents2(n int) string { return cents(n) }

// notifyPayment avisa del pago al equipo de la plataforma y le da su recibo al negocio. Las cortesías no se avisan.
func (a *API) notifyPayment(ctx context.Context, bizID int64, p paymentRow) {
	if !a.mailEnabled() || p.Method == "courtesy" {
		return
	}
	var name string
	a.DB.QueryRow(ctx, `SELECT name FROM businesses WHERE id = $1`, bizID).Scan(&name)
	plan, _ := planByID(p.Plan)
	until := p.PeriodEnd.Format("2006-01-02")
	for _, to := range a.teamRecipients(ctx) {
		a.sendBackground(mail.Message{To: to, Subject: "Pago recibido: " + name,
			Text: fmt.Sprintf("%s pagó %s (plan %s, %s). Vigente hasta %s.\n", name, moneyES(p.AmountCents), plan.Name, plural(p.Months, "mes", "meses"), until)}, "aviso de pago a la plataforma")
	}
	for _, to := range a.businessAdmins(ctx, bizID) {
		a.sendBackground(mail.Message{To: to, Subject: "Recibimos tu pago · Barbella",
			Text: fmt.Sprintf("Gracias. Recibimos tu pago de %s por el plan %s (%s).\nTu suscripción está vigente hasta el %s.\n\n— Barbella\n", moneyES(p.AmountCents), plan.Name, plural(p.Months, "mes", "meses"), until),
			HTML: fmt.Sprintf(`<p>Gracias. Recibimos tu pago de <b>%s</b> por el plan <b>%s</b> (%s).</p><p>Tu suscripción está vigente hasta el <b>%s</b>.</p><p>— Barbella</p>`,
				html.EscapeString(moneyES(p.AmountCents)), html.EscapeString(plan.Name), plural(p.Months, "mes", "meses"), until)}, "recibo de pago")
	}
}

// claimNotice marca un aviso como enviado; devuelve false si ya se había enviado (así no se repite).
func (a *API) claimNotice(ctx context.Context, bizID int64, kind, key string) bool {
	tag, err := a.DB.Exec(ctx, `INSERT INTO billing_notices (business_id, kind, period_key, sent_at) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, bizID, kind, key, a.now())
	return err == nil && tag.RowsAffected() == 1
}

// billingPass avisa a los administradores de los negocios cuando su prueba o su periodo está por terminar o terminó.
// Cada aviso sale una sola vez por periodo.
func (a *API) billingPass(ctx context.Context) {
	if !a.mailEnabled() {
		return
	}
	now := a.now()
	rows, err := a.DB.Query(ctx, bizSelect+` WHERE b.status IN ('trial', 'active')`)
	if err != nil {
		return
	}
	var list []bizRow
	for rows.Next() {
		if b, err := scanBiz(rows, now); err == nil {
			list = append(list, b)
		}
	}
	rows.Close()
	link := a.AppURL + "/panel/suscripcion"
	for _, b := range list {
		var kind, key, subject, body string
		switch b.BillingState {
		case "trialing":
			if b.TrialDaysLeft != nil && *b.TrialDaysLeft <= 3 {
				kind, key = "trial_ending", b.TrialEndsAt.Format("2006-01-02")
				subject = "Tu prueba de Barbella termina pronto"
				body = fmt.Sprintf("Tu prueba gratis de %s termina el %s. Elige un plan para seguir usando Barbella sin interrupciones:\n%s\n", b.Name, b.TrialEndsAt.Format("2006-01-02"), link)
			}
		case "trial_expired":
			kind, key = "trial_expired", b.TrialEndsAt.Format("2006-01-02")
			subject = "Tu prueba de Barbella terminó"
			body = fmt.Sprintf("La prueba de %s terminó y tu cuenta está en pausa (tus datos están a salvo). Elige un plan para reactivarla:\n%s\n", b.Name, link)
		case "active":
			if b.CurrentPeriodEnd != nil && b.CurrentPeriodEnd.Sub(now) <= 5*24*time.Hour {
				kind, key = "renewal_soon", b.CurrentPeriodEnd.Format("2006-01-02")
				subject = "Tu suscripción de Barbella vence pronto"
				body = fmt.Sprintf("La suscripción de %s vence el %s. Renuévala para no perder el acceso:\n%s\n", b.Name, b.CurrentPeriodEnd.Format("2006-01-02"), link)
			}
		case "past_due":
			kind, key = "past_due", b.CurrentPeriodEnd.Format("2006-01-02")
			subject = "Tu suscripción de Barbella venció"
			body = fmt.Sprintf("La suscripción de %s venció el %s y tu cuenta está en pausa. Renueva para reactivarla:\n%s\n", b.Name, b.CurrentPeriodEnd.Format("2006-01-02"), link)
		}
		if kind == "" || !a.claimNotice(ctx, b.ID, kind, key) {
			continue
		}
		for _, to := range a.businessAdmins(ctx, b.ID) {
			a.sendBackground(mail.Message{To: to, Subject: subject, Text: body + "\n— Barbella\n"}, "aviso de cobro")
		}
	}
}
