package api

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"miestudio/backend-go/internal/auth"
)

// Finanzas de la plataforma: lo cobrado por suscripciones, lo que cuesta operar y la ganancia de cada mes.
// Los meses se cuentan en la hora de Ciudad de México, igual que «cobrado este mes» del resumen.

const financeTZ = "America/Mexico_City"

// lastPaymentJoin une cada negocio con su último pago que no sea cortesía: de ahí sale lo que paga al mes.
const lastPaymentJoin = `JOIN LATERAL (SELECT amount_cents, months FROM payments p WHERE p.business_id = b.id AND p.method <> 'courtesy'
	   ORDER BY p.created_at DESC, p.id DESC LIMIT 1) last ON true`

// mrrCents es el ingreso mensual recurrente: lo que cada negocio activo y de pago paga al mes según su último pago.
func (a *API) mrrCents(ctx context.Context, now time.Time) int {
	var mrr int
	a.DB.QueryRow(ctx, `SELECT COALESCE(sum(last.amount_cents / last.months), 0) FROM businesses b `+lastPaymentJoin+`
	  WHERE (`+stateSQL("$1")+`) = 'active' AND b.current_period_end IS NOT NULL`, now).Scan(&mrr)
	return mrr
}

type financeMonth struct {
	Month         string `json:"month"` // AAAA-MM
	RevenueCents  int    `json:"revenue_cents"`
	ExpensesCents int    `json:"expenses_cents"`
	ProfitCents   int    `json:"profit_cents"`
}

type financePlan struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Clients     int    `json:"clients"`
	PriceCents  int    `json:"price_cents"`
	AmountCents int    `json:"amount_cents"`
}

type financePayer struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Plan        string `json:"plan"`
	Interval    string `json:"interval"`     // month | year
	AmountCents int    `json:"amount_cents"` // la parte de un mes
}

type expenseRow struct {
	ID          int64  `json:"id"`
	Label       string `json:"label"`
	AmountCents int    `json:"amount_cents"`
	Note        string `json:"note"`
	Day         string `json:"day"`
}

// platformFinance: GET /api/platform/finance?month=AAAA-MM (por defecto, el mes actual). Solo administradores.
func (a *API) platformFinance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := a.now()
	loc, err := time.LoadLocation(financeTZ)
	if err != nil {
		loc = time.UTC
	}
	start := time.Date(now.In(loc).Year(), now.In(loc).Month(), 1, 0, 0, 0, 0, loc)
	if q := strings.TrimSpace(r.URL.Query().Get("month")); q != "" {
		start, err = time.ParseInLocation("2006-01", q, loc)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "mes inválido")
			return
		}
	}
	ms := start.Format("2006-01-02")

	var revenue, manual int
	a.DB.QueryRow(ctx, `SELECT COALESCE(sum(amount_cents), 0), COALESCE(sum(amount_cents) FILTER (WHERE method IN ('transfer', 'cash', 'card')), 0)
	  FROM payments WHERE method <> 'courtesy'
	   AND created_at >= ($1::text::timestamp AT TIME ZONE $2) AND created_at < (($1::text::timestamp + interval '1 month') AT TIME ZONE $2)`, ms, financeTZ).Scan(&revenue, &manual)

	clients := map[string]int{"active": 0, "trialing": 0, "past_due": 0, "suspended": 0}
	if rows, err := a.DB.Query(ctx, `SELECT st, count(*) FROM (SELECT `+stateSQL("$1")+` AS st FROM businesses b) x GROUP BY 1`, now); err == nil {
		for rows.Next() {
			var st string
			var n int
			if rows.Scan(&st, &n) == nil {
				clients[st] = n
			}
		}
		rows.Close()
	}

	// los últimos seis meses, contando el elegido
	trend := []financeMonth{}
	if rows, err := a.DB.Query(ctx, `SELECT to_char(m, 'YYYY-MM'),
	    COALESCE((SELECT sum(p.amount_cents) FROM payments p WHERE p.method <> 'courtesy'
	       AND p.created_at >= (m AT TIME ZONE $2) AND p.created_at < ((m + interval '1 month') AT TIME ZONE $2)), 0),
	    COALESCE((SELECT sum(e.amount_cents) FROM platform_expenses e WHERE e.day >= m::date AND e.day < (m + interval '1 month')::date), 0)
	  FROM generate_series($1::text::timestamp - interval '5 months', $1::text::timestamp, interval '1 month') AS m ORDER BY 1`, ms, financeTZ); err == nil {
		for rows.Next() {
			var x financeMonth
			if rows.Scan(&x.Month, &x.RevenueCents, &x.ExpensesCents) == nil {
				x.ProfitCents = x.RevenueCents - x.ExpensesCents
				trend = append(trend, x)
			}
		}
		rows.Close()
	}

	// lo esperado por plan y quién lo paga: negocios activos con su último pago, a razón de un mes
	byPlan := map[string][2]int{}
	if rows, err := a.DB.Query(ctx, `SELECT b.plan, count(*), COALESCE(sum(last.amount_cents / last.months), 0) FROM businesses b `+lastPaymentJoin+`
	  WHERE (`+stateSQL("$1")+`) = 'active' AND b.current_period_end IS NOT NULL GROUP BY b.plan`, now); err == nil {
		for rows.Next() {
			var id string
			var v [2]int
			if rows.Scan(&id, &v[0], &v[1]) == nil {
				byPlan[id] = v
			}
		}
		rows.Close()
	}
	plans := a.plans(ctx)
	planName := map[string]string{}
	planOut := make([]financePlan, 0, len(plans))
	for _, p := range plans {
		planName[p.ID] = p.Name
		planOut = append(planOut, financePlan{ID: p.ID, Name: p.Name, Clients: byPlan[p.ID][0], PriceCents: p.PriceCents(1), AmountCents: byPlan[p.ID][1]})
	}

	payers := []financePayer{}
	if rows, err := a.DB.Query(ctx, `SELECT b.id, b.name, b.plan, last.months, last.amount_cents / last.months FROM businesses b `+lastPaymentJoin+`
	  WHERE (`+stateSQL("$1")+`) = 'active' AND b.current_period_end IS NOT NULL ORDER BY 5 DESC, b.name LIMIT 100`, now); err == nil {
		for rows.Next() {
			var x financePayer
			var plan string
			var months int
			if rows.Scan(&x.ID, &x.Name, &plan, &months, &x.AmountCents) == nil {
				x.Plan = planName[plan]
				if x.Plan == "" {
					x.Plan = plan
				}
				x.Interval = "month"
				if months >= 12 {
					x.Interval = "year"
				}
				payers = append(payers, x)
			}
		}
		rows.Close()
	}

	expenses := []expenseRow{}
	expensesTotal := 0
	if rows, err := a.DB.Query(ctx, `SELECT id, label, amount_cents, note, day::text FROM platform_expenses
	  WHERE day >= $1::text::date AND day < ($1::text::date + interval '1 month')::date ORDER BY day DESC, id DESC`, ms); err == nil {
		for rows.Next() {
			var x expenseRow
			if rows.Scan(&x.ID, &x.Label, &x.AmountCents, &x.Note, &x.Day) == nil {
				expenses = append(expenses, x)
				expensesTotal += x.AmountCents
			}
		}
		rows.Close()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"month": start.Format("2006-01"), "revenue_cents": revenue, "manual_cents": manual, "expected_cents": a.mrrCents(ctx, now),
		"expenses_cents": expensesTotal, "profit_cents": revenue - expensesTotal,
		"clients": clients, "trend": trend, "by_plan": planOut, "payers": payers, "expenses": expenses,
	})
}

// platformAddExpense: POST /api/platform/expenses. Solo administradores.
func (a *API) platformAddExpense(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label       string `json:"label"`
		AmountCents int    `json:"amount_cents"`
		Note        string `json:"note"`
		Day         string `json:"day"` // AAAA-MM-DD; por defecto, hoy
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Label, req.Note, req.Day = strings.TrimSpace(req.Label), strings.TrimSpace(req.Note), strings.TrimSpace(req.Day)
	switch {
	case utf8.RuneCountInString(req.Label) < 2 || utf8.RuneCountInString(req.Label) > 80:
		writeErr(w, http.StatusBadRequest, "escribe el concepto del gasto (2 a 80 caracteres)")
		return
	case req.AmountCents <= 0 || req.AmountCents > 100_000_000:
		writeErr(w, http.StatusBadRequest, "el monto debe ser mayor que cero")
		return
	case utf8.RuneCountInString(req.Note) > 200:
		writeErr(w, http.StatusBadRequest, "la nota es demasiado larga")
		return
	}
	var day *string
	if req.Day != "" {
		if _, err := time.Parse("2006-01-02", req.Day); err != nil {
			writeErr(w, http.StatusBadRequest, "fecha inválida")
			return
		}
		day = &req.Day
	}
	var id int64
	if err := a.DB.QueryRow(r.Context(),
		`INSERT INTO platform_expenses (label, amount_cents, note, day, created_by)
		 VALUES ($1, $2, $3, COALESCE($4::text::date, (now() AT TIME ZONE $6)::date), $5) RETURNING id`,
		req.Label, req.AmountCents, req.Note, day, auth.FromContext(r.Context()).UserID, financeTZ).Scan(&id); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), actor(r), 0, "platform_expense_added", map[string]any{"label": req.Label, "amount_cents": req.AmountCents})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// platformDeleteExpense: DELETE /api/platform/expenses/{id}. Solo administradores.
func (a *API) platformDeleteExpense(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var label string
	if err := a.DB.QueryRow(r.Context(), `DELETE FROM platform_expenses WHERE id = $1 RETURNING label`, id).Scan(&label); err != nil {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	a.logActivity(r.Context(), actor(r), 0, "platform_expense_removed", map[string]any{"label": label})
	w.WriteHeader(http.StatusNoContent)
}
