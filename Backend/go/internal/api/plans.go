package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"miestudio/backend-go/internal/auth"
)

const (
	TrialDays        = 14
	pastDueGraceDays = 3 // un periodo pagado vencido sigue funcionando estos días
)

// Plan es lo que incluye cada suscripción. Los precios son de lista, en pesos; el año cobra 10 meses.
// nil en un tope = sin límite.
type Plan struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceMonth  int    `json:"price_month"`
	PriceYear   int    `json:"price_year"`
	MaxStaff    *int   `json:"max_staff"`    // personas activas que atienden (aparecen en la agenda)
	MaxBranches int    `json:"max_branches"` // sucursales activas
	POS         bool   `json:"pos"`          // cobros: caja, ventas, paquetes, inventario, comisiones y reportes
	Deposits    bool   `json:"deposits"`     // anticipos en la reserva en línea
	Terminal    bool   `json:"terminal"`     // cobro con terminal de Mercado Pago
	Support     string `json:"support"`
}

func ptr(n int) *int { return &n }

var planCatalog = []Plan{
	{ID: "basico", Name: "Básico", Description: "Solo agenda y citas, sin cobros", PriceMonth: 399, PriceYear: 3990,
		MaxStaff: ptr(2), MaxBranches: 1, POS: false, Deposits: false, Terminal: false, Support: "correo"},
	{ID: "crecimiento", Name: "Crecimiento", Description: "Todo incluido: citas, caja, ventas, anticipos y más", PriceMonth: 799, PriceYear: 7990,
		MaxStaff: ptr(6), MaxBranches: 1, POS: true, Deposits: true, Terminal: true, Support: "prioritario"},
	{ID: "pro", Name: "Pro", Description: "Todo, con varias sucursales y equipos grandes", PriceMonth: 1499, PriceYear: 14990,
		MaxStaff: nil, MaxBranches: 10, POS: true, Deposits: true, Terminal: true, Support: "dedicado"},
}

func planByID(id string) (Plan, bool) {
	for _, p := range planCatalog {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}

// PriceCents es lo que cuesta el plan por 1 o 12 meses, en centavos.
func (p Plan) PriceCents(months int) int {
	if months == 12 {
		return p.PriceYear * 100
	}
	return p.PriceMonth * 100
}

// BillingState es el estado efectivo de un negocio, que también cuenta las fechas que pasan:
// pending, trialing, trial_expired, active, past_due o suspended.
func BillingState(status string, trialEnds, periodEnd *time.Time, now time.Time) string {
	switch status {
	case "pending":
		return "pending"
	case "suspended":
		return "suspended"
	case "trial":
		if trialEnds != nil && trialEnds.After(now) {
			return "trialing"
		}
		return "trial_expired"
	default: // active
		if periodEnd != nil && periodEnd.Add(pastDueGraceDays*24*time.Hour).Before(now) {
			return "past_due"
		}
		return "active"
	}
}

// BillingUsable dice si el negocio puede usar el producto ahora.
func BillingUsable(state string) bool { return state == "trialing" || state == "active" }

// usableSQL es la versión en SQL de BillingUsable: la prueba sigue o el periodo pagado (con su gracia) no ha terminado.
// alias es el de businesses con su punto ("b."); nowArg es el parámetro ($1…) con el momento actual.
func usableSQL(alias, nowArg string) string {
	return `((` + alias + `status = 'trial' AND ` + alias + `trial_ends_at > ` + nowArg + `)
		OR (` + alias + `status = 'active' AND (` + alias + `current_period_end IS NULL OR ` + alias + `current_period_end + interval '3 days' >= ` + nowArg + `)))`
}

// planOf lee el plan de un negocio.
func (a *API) planOf(ctx context.Context, q queryer, bizID int64) Plan {
	var id string
	q.QueryRow(ctx, `SELECT plan FROM businesses WHERE id = $1`, bizID).Scan(&id)
	if p, ok := planByID(id); ok {
		return p
	}
	return planCatalog[1]
}

// Uso actual frente a los topes del plan.
type planUsage struct {
	Staff    int `json:"staff"`
	Branches int `json:"branches"`
	Users    int `json:"users"`
	Clients  int `json:"clients"`
}

func (a *API) usageOf(ctx context.Context, q queryer, bizID int64) planUsage {
	var u planUsage
	q.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM users WHERE business_id = $1 AND active AND bookable),
		(SELECT count(*) FROM branches WHERE business_id = $1 AND active),
		(SELECT count(*) FROM users WHERE business_id = $1 AND active),
		(SELECT count(*) FROM clients WHERE business_id = $1 AND anonymized_at IS NULL)`, bizID).Scan(&u.Staff, &u.Branches, &u.Users, &u.Clients)
	return u
}

func planLimit(code, msg string) error {
	return &httpFail{403, msg, code}
}

var errPlanPOS = &httpFail{403, "Tu plan no incluye cobros (caja, ventas, inventario y reportes). Cambia a Crecimiento o Pro para usarlos.", "plan_feature"}
var errPlanTerminal = &httpFail{403, "Tu plan no incluye el cobro con terminal. Cambia de plan para usarlo.", "plan_feature"}
var errPlanDeposits = &httpFail{403, "Tu plan no incluye anticipos en la reserva en línea. Cambia de plan para usarlos.", "plan_feature"}

// checkStaffRoom exige que quepa una persona más en la agenda según el plan.
func (a *API) checkStaffRoom(ctx context.Context, bizID int64) error {
	plan := a.planOf(ctx, a.DB, bizID)
	if plan.MaxStaff == nil {
		return nil
	}
	if a.usageOf(ctx, a.DB, bizID).Staff >= *plan.MaxStaff {
		return planLimit("plan_limit", fmt.Sprintf("Tu plan %s permite hasta %d personas en la agenda. Desactiva a alguien o cambia de plan.", plan.Name, *plan.MaxStaff))
	}
	return nil
}

// checkBranchRoom exige que quepa una sucursal más según el plan.
func (a *API) checkBranchRoom(ctx context.Context, bizID int64) error {
	plan := a.planOf(ctx, a.DB, bizID)
	if a.usageOf(ctx, a.DB, bizID).Branches >= plan.MaxBranches {
		return planLimit("plan_limit", fmt.Sprintf("Tu plan %s permite hasta %d sucursal(es). Cambia de plan para agregar más.", plan.Name, plan.MaxBranches))
	}
	return nil
}

// posOnly deja pasar solo a los negocios cuyo plan incluye cobros (caja, ventas, paquetes, inventario, comisiones, reportes).
func (a *API) posOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := auth.FromContext(r.Context())
		if !a.planOf(r.Context(), a.DB, p.BusinessID).POS {
			writeFail(w, errPlanPOS)
			return
		}
		h(w, r)
	}
}

// planFeatures es lo que la interfaz necesita saber del plan para mostrar u ocultar secciones.
func (a *API) planFeatures(ctx context.Context, bizID int64) map[string]any {
	pl := a.planOf(ctx, a.DB, bizID)
	return map[string]any{"pos": pl.POS, "deposits": pl.Deposits, "terminal": pl.Terminal, "max_branches": pl.MaxBranches, "max_staff": pl.MaxStaff, "name": pl.Name}
}

// publicPlans es GET /api/plans: el catálogo de planes para la página de inicio, sin datos de ningún negocio.
func (a *API) publicPlans(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, planCatalog)
}
