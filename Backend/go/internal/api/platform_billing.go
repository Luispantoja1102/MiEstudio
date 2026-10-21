package api

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

func planExists(id string) bool { _, ok := planByID(id); return ok }

// ---------- pagos ----------

type paymentRow struct {
	ID          int64     `json:"id"`
	BusinessID  int64     `json:"business_id"`
	Business    string    `json:"business,omitempty"`
	AmountCents int       `json:"amount_cents"`
	Method      string    `json:"method"`
	Plan        string    `json:"plan"`
	Months      int       `json:"months"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Note        string    `json:"note"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *API) paymentsQuery(ctx context.Context, where string, args ...any) []paymentRow {
	rows, err := a.DB.Query(ctx,
		`SELECT p.id, p.business_id, b.name, p.amount_cents, p.method, p.plan, p.months, p.period_start, p.period_end, p.note,
		        COALESCE(u.name, 'Mercado Pago'), p.created_at
		   FROM payments p JOIN businesses b ON b.id = p.business_id LEFT JOIN users u ON u.id = p.created_by `+where, args...)
	out := []paymentRow{}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var x paymentRow
		if rows.Scan(&x.ID, &x.BusinessID, &x.Business, &x.AmountCents, &x.Method, &x.Plan, &x.Months, &x.PeriodStart, &x.PeriodEnd, &x.Note, &x.CreatedBy, &x.CreatedAt) == nil {
			out = append(out, x)
		}
	}
	return out
}

func (a *API) paymentsOf(ctx context.Context, bizID int64, limit int) []paymentRow {
	return a.paymentsQuery(ctx, `WHERE p.business_id = $1 ORDER BY p.created_at DESC, p.id DESC LIMIT $2`, bizID, limit)
}

// extendSubscription acredita un periodo pagado: lo suma al vigente si aún no vence y deja el negocio activo.
// Es la única vía por la que se paga una suscripción, tanto a mano como por Mercado Pago.
func (a *API) extendSubscription(ctx context.Context, q execQueryer, bizID int64, plan string, months, amountCents int, method, note, externalRef string, by *int64) (paymentRow, error) {
	now := a.now()
	var end *time.Time
	var status, currentPlan string
	if err := q.QueryRow(ctx, `SELECT current_period_end, status, plan FROM businesses WHERE id = $1 FOR UPDATE`, bizID).Scan(&end, &status, &currentPlan); err != nil {
		return paymentRow{}, err
	}
	if plan == "" {
		plan = currentPlan
	}
	start := now
	if status == "active" && end != nil && end.After(now) {
		start = *end // renovar antes de vencer no pierde días
	}
	newEnd := start.AddDate(0, months, 0)
	var ref *string
	if externalRef != "" {
		ref = &externalRef
	}
	var x paymentRow
	err := q.QueryRow(ctx,
		`INSERT INTO payments (business_id, amount_cents, method, plan, months, period_start, period_end, note, external_ref, created_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		bizID, amountCents, method, plan, months, start, newEnd, note, ref, by, now).Scan(&x.ID)
	if err != nil {
		return paymentRow{}, err
	}
	if _, err := q.Exec(ctx,
		`UPDATE businesses SET status = 'active', plan = $2, current_period_end = $3, suspended_reason = '' WHERE id = $1`, bizID, plan, newEnd); err != nil {
		return paymentRow{}, err
	}
	x.BusinessID, x.AmountCents, x.Method, x.Plan, x.Months, x.PeriodStart, x.PeriodEnd, x.Note, x.CreatedAt = bizID, amountCents, method, plan, months, start, newEnd, note, now
	return x, nil
}

var paymentMethods = map[string]bool{"transfer": true, "cash": true, "card": true, "courtesy": true}

// platformRecordPayment registra un pago hecho fuera de la plataforma (transferencia, efectivo) o una cortesía.
func (a *API) platformRecordPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		AmountCents int    `json:"amount_cents"`
		Months      int    `json:"months"`
		Plan        string `json:"plan"`
		Method      string `json:"method"`
		Note        string `json:"note"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	switch {
	case req.Months < 1 || req.Months > 24:
		writeErr(w, http.StatusBadRequest, "los meses deben ser de 1 a 24")
		return
	case !paymentMethods[req.Method]:
		writeErr(w, http.StatusBadRequest, "método inválido (transfer, cash, card o courtesy)")
		return
	case req.Method == "courtesy" && req.AmountCents != 0, req.Method != "courtesy" && (req.AmountCents <= 0 || req.AmountCents > 100_000_000):
		writeErr(w, http.StatusBadRequest, "el monto debe ser mayor que cero (una cortesía va en cero)")
		return
	case req.Plan != "" && !planExists(req.Plan):
		writeErr(w, http.StatusBadRequest, "plan inválido")
		return
	case len(req.Note) > 300:
		writeErr(w, http.StatusBadRequest, "la nota es demasiado larga")
		return
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	by := actor(r)
	p, err := a.extendSubscription(ctx, tx, id, req.Plan, req.Months, req.AmountCents, req.Method, req.Note, "", &by)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil || tx.Commit(ctx) != nil {
		serverErr(w)
		return
	}
	a.logActivity(ctx, by, id, "payment_recorded", map[string]any{"amount_cents": p.AmountCents, "months": p.Months, "plan": p.Plan, "method": p.Method})
	a.notifyPayment(ctx, id, p)
	writeJSON(w, http.StatusCreated, p)
}

// platformPayments lista los últimos pagos de todos los negocios (solo administradores de plataforma).
func (a *API) platformPayments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.paymentsQuery(r.Context(), `ORDER BY p.created_at DESC, p.id DESC LIMIT 200`))
}

// ---------- planes ----------

func (a *API) platformPlans(w http.ResponseWriter, r *http.Request) {
	type planOut struct {
		Plan
		Businesses int `json:"businesses"` // negocios en uso (prueba o de pago)
		Paying     int `json:"paying"`
	}
	now := a.now()
	counts := map[string][2]int{}
	rows, err := a.DB.Query(r.Context(), `SELECT b.plan, `+stateSQL("$1")+` AS st, count(*) FROM businesses b GROUP BY 1, 2`, now)
	if err == nil {
		for rows.Next() {
			var pl, st string
			var n int
			if rows.Scan(&pl, &st, &n) == nil {
				c := counts[pl]
				if st == "trialing" || st == "active" {
					c[0] += n
				}
				if st == "active" {
					c[1] += n
				}
				counts[pl] = c
			}
		}
		rows.Close()
	}
	out := make([]planOut, 0, len(planCatalog))
	for _, p := range planCatalog {
		out = append(out, planOut{Plan: p, Businesses: counts[p.ID][0], Paying: counts[p.ID][1]})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- resumen ----------

type attentionItem struct {
	Kind       string     `json:"kind"`
	BusinessID int64      `json:"business_id"`
	Name       string     `json:"name"`
	Title      string     `json:"title"`
	Detail     string     `json:"detail"`
	At         *time.Time `json:"at"`
}

// platformOverview: cuántos negocios hay en cada estado, ingresos, y lo que pide atención.
func (a *API) platformOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := a.now()
	isAdmin := auth.FromContext(ctx).Role == auth.RolePlatformAdmin

	rows, err := a.DB.Query(ctx, bizSelect+` ORDER BY b.created_at DESC, b.id DESC`)
	if err != nil {
		serverErr(w)
		return
	}
	var all []bizRow
	for rows.Next() {
		b, err := scanBiz(rows, now)
		if err != nil {
			rows.Close()
			serverErr(w)
			return
		}
		all = append(all, b)
	}
	rows.Close()

	counts := map[string]int{"total": len(all), "pending": 0, "trialing": 0, "trial_expired": 0, "active": 0, "past_due": 0, "suspended": 0}
	signups := 0
	attention := []attentionItem{}
	for _, b := range all {
		counts[b.BillingState]++
		if now.Sub(b.CreatedAt) < 30*24*time.Hour {
			signups++
		}
		switch b.BillingState {
		case "pending":
			attention = append(attention, attentionItem{"pending", b.ID, b.Name, "Espera tu aprobación", "Se registró y espera revisión", &b.CreatedAt})
		case "past_due":
			attention = append(attention, attentionItem{"past_due", b.ID, b.Name, "Pago atrasado", "Plan " + b.PlanName, b.CurrentPeriodEnd})
		case "trial_expired":
			attention = append(attention, attentionItem{"trial_expired", b.ID, b.Name, "La prueba terminó", "Sin suscripción activa", b.TrialEndsAt})
		case "trialing":
			if b.TrialDaysLeft != nil && *b.TrialDaysLeft <= 3 {
				attention = append(attention, attentionItem{"trial_ending", b.ID, b.Name, "La prueba termina pronto", plural(*b.TrialDaysLeft, "día restante", "días restantes"), b.TrialEndsAt})
			}
		}
	}
	order := map[string]int{"pending": 0, "past_due": 1, "trial_expired": 2, "trial_ending": 3}
	sort.SliceStable(attention, func(i, j int) bool { return order[attention[i].Kind] < order[attention[j].Kind] })
	recent := all
	if len(recent) > 5 {
		recent = recent[:5]
	}

	var users, clients int
	a.DB.QueryRow(ctx, `SELECT (SELECT count(*) FROM users WHERE business_id IS NOT NULL), (SELECT count(*) FROM clients)`).Scan(&users, &clients)
	out := map[string]any{
		// claves que ya existían
		"businesses": map[string]int{"pending": counts["pending"], "trial": counts["trialing"] + counts["trial_expired"], "active": counts["active"] + counts["past_due"], "suspended": counts["suspended"]},
		"total":      counts["total"], "users": users, "clients": clients, "signups_30d": signups, "registration_mode": a.registrationMode(ctx),
		// estados de cobro
		"counts": counts, "attention": attention, "recent": recent,
	}
	if isAdmin {
		var mrr, paid int
		// ingreso mensual recurrente: lo que cada negocio activo y de pago paga al mes según su último pago
		a.DB.QueryRow(ctx, `SELECT COALESCE(sum(last.amount_cents / last.months), 0) FROM businesses b
		   JOIN LATERAL (SELECT amount_cents, months FROM payments p WHERE p.business_id = b.id AND p.method <> 'courtesy' ORDER BY p.created_at DESC, p.id DESC LIMIT 1) last ON true
		  WHERE (`+stateSQL("$1")+`) = 'active' AND b.current_period_end IS NOT NULL`, now).Scan(&mrr)
		a.DB.QueryRow(ctx, `SELECT COALESCE(sum(amount_cents), 0) FROM payments WHERE method <> 'courtesy'
		   AND created_at >= date_trunc('month', $1::timestamptz AT TIME ZONE 'America/Mexico_City') AT TIME ZONE 'America/Mexico_City'`, now).Scan(&paid)
		out["mrr_cents"], out["paid_this_month_cents"] = mrr, paid
	}
	writeJSON(w, http.StatusOK, out)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return itoa(int64(n)) + " " + many
}

// ---------- salud ----------

func (a *API) platformHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := a.now()
	started := time.Now()
	dbOK := a.DB.Ping(ctx) == nil
	latency := time.Since(started).Milliseconds()

	var migrations int
	var lastMigration string
	a.DB.QueryRow(ctx, `SELECT count(*), COALESCE(max(name), '') FROM schema_migrations`).Scan(&migrations, &lastMigration)
	var pending, failed, stale int
	a.DB.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status = 'pending'), count(*) FILTER (WHERE status = 'failed' AND created_at > $1 - interval '24 hours'),
	    count(*) FILTER (WHERE status = 'pending' AND send_after < $1 - interval '15 minutes') FROM notifications`, now).Scan(&pending, &failed, &stale)
	var holds int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM appointments WHERE status = 'pending_payment' AND hold_expires_at < $1`, now).Scan(&holds)
	var mpBusinesses int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM mp_accounts`).Scan(&mpBusinesses)

	checks := []map[string]any{
		{"key": "database", "label": "Base de datos", "ok": dbOK, "detail": itoa(latency) + " ms"},
		{"key": "migrations", "label": "Migraciones", "ok": migrations > 0, "detail": itoa(int64(migrations)) + " aplicadas · última " + lastMigration},
		{"key": "mail", "label": "Correo (SMTP)", "ok": a.mailEnabled(), "detail": ternary(a.mailEnabled(), "configurado", "sin configurar: no salen confirmaciones, recordatorios ni recuperación de contraseña")},
		{"key": "outbox", "label": "Bandeja de correos", "ok": stale == 0 && failed == 0, "detail": itoa(int64(pending)) + " pendientes · " + itoa(int64(stale)) + " atrasados · " + itoa(int64(failed)) + " fallidos (24 h)"},
		{"key": "holds", "label": "Horarios apartados vencidos", "ok": holds == 0, "detail": itoa(int64(holds)) + " por liberar"},
		{"key": "mp_oauth", "label": "Conexión de Mercado Pago (OAuth)", "ok": a.oauthEnabled(), "detail": ternary(a.oauthEnabled(), itoa(int64(mpBusinesses))+" negocios conectados", "MP_CLIENT_ID/SECRET sin configurar: los negocios solo pueden pegar su token")},
		{"key": "mp_platform", "label": "Cobro de suscripciones", "ok": a.PlatformMPToken != "", "detail": ternary(a.PlatformMPToken != "", "MPP_ACCESS_TOKEN configurado", "sin MPP_ACCESS_TOKEN: los pagos de suscripción se registran a mano")},
		{"key": "webhook", "label": "Avisos de Mercado Pago", "ok": a.PublicURL != "", "detail": ternary(a.PublicURL != "", a.PublicURL, "sin API_PUBLIC_URL: los pagos se confirman al volver el cliente, no al instante")},
		{"key": "app_url", "label": "Dirección pública", "ok": !strings.Contains(a.AppURL, "localhost"), "detail": a.AppURL},
		{"key": "production", "label": "Modo producción", "ok": a.Production, "detail": ternary(a.Production, "APP_ENV=production", "modo desarrollo")},
	}
	allOK := true
	for _, c := range checks {
		if c["ok"] == false && (c["key"] == "database" || c["key"] == "outbox" || c["key"] == "holds") {
			allOK = false
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": allOK, "checked_at": now, "checks": checks})
}

func ternary(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}
