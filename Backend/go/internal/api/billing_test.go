package api

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func tp(t time.Time) *time.Time { return &t }

func TestBillingStateMatrix(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	for _, c := range []struct {
		name          string
		status        string
		trial, period *time.Time
		want          string
		usable        bool
	}{
		{"pendiente", "pending", nil, nil, "pending", false},
		{"suspendido", "suspended", tp(now.Add(day)), nil, "suspended", false},
		{"prueba vigente", "trial", tp(now.Add(day)), nil, "trialing", true},
		{"prueba justo ahora", "trial", tp(now), nil, "trial_expired", false},
		{"prueba vencida", "trial", tp(now.Add(-day)), nil, "trial_expired", false},
		{"prueba sin fecha", "trial", nil, nil, "trial_expired", false},
		{"activo sin vencimiento (cortesía)", "active", nil, nil, "active", true},
		{"activo vigente", "active", nil, tp(now.Add(day)), "active", true},
		{"venció hoy: aún en gracia", "active", nil, tp(now.Add(-time.Hour)), "active", true},
		{"venció hace 2 días 23 h: gracia", "active", nil, tp(now.Add(-3*day + time.Hour)), "active", true},
		{"venció hace 3 días y 1 h: atrasado", "active", nil, tp(now.Add(-3*day - time.Hour)), "past_due", false},
	} {
		got := BillingState(c.status, c.trial, c.period, now)
		if got != c.want || BillingUsable(got) != c.usable {
			t.Errorf("%s: %s (usable %v), esperaba %s (%v)", c.name, got, BillingUsable(got), c.want, c.usable)
		}
	}
}

func TestSQLAndGoAgreeOnState(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	cases := []struct {
		status        string
		trial, period *time.Time
	}{
		{"pending", nil, nil}, {"suspended", nil, nil}, {"trial", tp(now.Add(day)), nil}, {"trial", tp(now.Add(-day)), nil},
		{"active", nil, nil}, {"active", nil, tp(now.Add(day))}, {"active", nil, tp(now.Add(-day))}, {"active", nil, tp(now.Add(-4 * day))},
	}
	for i, c := range cases {
		var id int64
		a.DB.QueryRow(ctx, `INSERT INTO businesses (name, slug, kind, status, trial_ends_at, current_period_end) VALUES ('n', $1, 'barberia', $2, $3, $4) RETURNING id`,
			fmt.Sprintf("n%d", i), c.status, c.trial, c.period).Scan(&id)
		var sqlState string
		var usable bool
		a.DB.QueryRow(ctx, `SELECT `+stateSQL("$2")+`, `+usableSQL("b.", "$2")+` FROM businesses b WHERE b.id = $1`, id, now).Scan(&sqlState, &usable)
		goState := BillingState(c.status, c.trial, c.period, now)
		if sqlState != goState || usable != BillingUsable(goState) {
			t.Errorf("caso %d: SQL=%s/%v Go=%s/%v", i, sqlState, usable, goState, BillingUsable(goState))
		}
	}
}

func TestLapsedTrialLocksTenantButKeepsAccountAndBilling(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "Barbería", "a@x.com")
	plat := newPlatform(t, a)
	if r := a.do(t, "GET", "/api/services", admin, nil); r.Status != 200 {
		t.Fatalf("en prueba: %d", r.Status)
	}
	a.Now = func() time.Time { return time.Now().Add(15 * 24 * time.Hour) } // la prueba de 14 días terminó
	r := a.do(t, "GET", "/api/services", admin, nil)
	if r.Status != 403 || r.Body["code"] != "subscription_required" {
		t.Fatalf("prueba vencida: %d %v", r.Status, r.Body)
	}
	me := a.do(t, "GET", "/api/me", admin, nil)
	biz := me.Body["business"].(map[string]any)
	if me.Status != 200 || biz["billing_state"] != "trial_expired" || biz["plan"] != "crecimiento" {
		t.Fatalf("/api/me debe seguir sirviendo con el estado: %d %v", me.Status, me.Body)
	}
	if r := a.do(t, "POST", "/api/auth/logout-all", admin, nil); r.Status != 204 {
		t.Fatalf("la cuenta sigue pudiendo cerrar sesión: %d", r.Status)
	}
	admin = a.doLogin(t, "a@x.com", "supersecreta1").Body["token"].(string)
	// la plataforma registra un pago y todo vuelve
	var id int64
	a.DB.QueryRow(context.Background(), `SELECT id FROM businesses`).Scan(&id)
	if p := a.do(t, "POST", path("/api/platform/businesses/", id, "/payments"), plat, map[string]any{"amount_cents": 70000, "months": 1, "method": "transfer"}); p.Status != 201 {
		t.Fatalf("pago: %d %v", p.Status, p.Body)
	}
	if r := a.do(t, "GET", "/api/services", admin, nil); r.Status != 200 {
		t.Fatalf("tras pagar: %d", r.Status)
	}
}

func TestPastDueGraceAndLock(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "Barbería", "a@x.com")
	end := time.Now().Add(10 * 24 * time.Hour)
	a.DB.Exec(context.Background(), `UPDATE businesses SET status = 'active', current_period_end = $1`, end)
	at := func(d time.Duration) { a.Now = func() time.Time { return end.Add(d) } }
	at(-time.Hour)
	if r := a.do(t, "GET", "/api/services", admin, nil); r.Status != 200 {
		t.Fatalf("vigente: %d", r.Status)
	}
	at(2*24*time.Hour + 23*time.Hour)
	if r := a.do(t, "GET", "/api/services", admin, nil); r.Status != 200 {
		t.Fatalf("en gracia (3 días): %d", r.Status)
	}
	at(3*24*time.Hour + time.Hour)
	if r := a.do(t, "GET", "/api/services", admin, nil); r.Status != 403 || r.Body["code"] != "subscription_required" {
		t.Fatalf("pasada la gracia: %d %v", r.Status, r.Body)
	}
}

func TestLapsedBusinessStopsServingBookingPage(t *testing.T) {
	o := newOnline(t)
	if o.a.do(t, "GET", "/api/public/"+o.slug, "", nil).Status != 200 {
		t.Fatal("negocio de pago sirve su página")
	}
	o.a.DB.Exec(context.Background(), `UPDATE businesses SET status = 'active', current_period_end = $1`, o.a.now().Add(-5*24*time.Hour))
	if r := o.a.do(t, "GET", "/api/public/"+o.slug, "", nil); r.Status != 404 {
		t.Fatalf("pago atrasado: %d", r.Status)
	}
	if r := o.book(t, at(monday, "10:00"), nil); r.Status != 404 {
		t.Fatalf("reservar con pago atrasado: %d", r.Status)
	}
	// los derechos de privacidad de los clientes siguen disponibles aunque el negocio no pague
	if r := o.a.do(t, "GET", "/api/public/"+o.slug+"/privacy", "", nil); r.Status != 200 {
		t.Fatalf("aviso de privacidad: %d", r.Status)
	}
}

func TestPlanLimitsStaffBranchesAndFeatures(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "Barbería", "a@x.com")
	a.DB.Exec(context.Background(), `UPDATE businesses SET plan = 'basico'`)
	mk := func(email string, bookable bool) resp {
		return a.do(t, "POST", "/api/team", admin, map[string]any{"name": "P", "email": email, "password": "supersecreta1", "role": "staff", "bookable": bookable})
	}
	if r := mk("p1@x.com", true); r.Status != 201 {
		t.Fatalf("primera persona: %d %v", r.Status, r.Body)
	}
	if r := mk("p2@x.com", true); r.Status != 201 {
		t.Fatalf("segunda persona: %d", r.Status)
	}
	if r := mk("p3@x.com", true); r.Status != 403 || r.Body["code"] != "plan_limit" {
		t.Fatalf("tercera persona en Esencial (máx. 2): %d %v", r.Status, r.Body)
	}
	if r := mk("recep@x.com", false); r.Status != 201 {
		t.Fatalf("quien no atiende no cuenta: %d", r.Status)
	}
	// activar a alguien existente en la agenda también cuenta
	var rid int64
	a.DB.QueryRow(context.Background(), `SELECT id FROM users WHERE email = 'recep@x.com'`).Scan(&rid)
	if r := a.do(t, "PATCH", path("/api/team/", rid), admin, map[string]any{"bookable": true}); r.Status != 403 {
		t.Fatalf("pasar a la agenda con el plan lleno: %d", r.Status)
	}
	// liberar un lugar
	var p1 int64
	a.DB.QueryRow(context.Background(), `SELECT id FROM users WHERE email = 'p1@x.com'`).Scan(&p1)
	a.do(t, "PATCH", path("/api/team/", p1), admin, map[string]any{"active": false})
	if r := a.do(t, "PATCH", path("/api/team/", rid), admin, map[string]any{"bookable": true}); r.Status != 204 {
		t.Fatalf("con lugar libre: %d", r.Status)
	}
	if r := a.do(t, "PATCH", path("/api/team/", p1), admin, map[string]any{"active": true}); r.Status != 403 {
		t.Fatalf("reactivar con el plan lleno: %d", r.Status)
	}
	// sucursales: Básico permite 1 (la que se crea al registrarse)
	if r := a.do(t, "POST", "/api/branches", admin, map[string]any{"name": "Sucursal 2"}); r.Status != 403 || r.Body["code"] != "plan_limit" {
		t.Fatalf("segunda sucursal en Básico: %d %v", r.Status, r.Body)
	}
	// Crecimiento incluye todo pero sigue en una sucursal: las sucursales son del plan Pro
	a.DB.Exec(context.Background(), `UPDATE businesses SET plan = 'crecimiento'`)
	if r := a.do(t, "POST", "/api/branches", admin, map[string]any{"name": "Sucursal 2"}); r.Status != 403 || r.Body["code"] != "plan_limit" {
		t.Fatalf("segunda sucursal en Crecimiento: %d %v", r.Status, r.Body)
	}
	a.DB.Exec(context.Background(), `UPDATE businesses SET plan = 'pro'`)
	for i := 2; i <= 10; i++ {
		if r := a.do(t, "POST", "/api/branches", admin, map[string]any{"name": fmt.Sprintf("Sucursal %d", i)}); r.Status != 201 {
			t.Fatalf("Pro permite 10 (sucursal %d): %d", i, r.Status)
		}
	}
	if r := a.do(t, "POST", "/api/branches", admin, map[string]any{"name": "Sucursal 11"}); r.Status != 403 {
		t.Fatalf("la 11: %d", r.Status)
	}
}

func TestDepositAndTerminalNeedPlanFeature(t *testing.T) {
	d := newDeposit(t)
	// un plan sin anticipos (la plataforma puede quitarlos de cualquiera)
	d.a.DB.Exec(context.Background(), `INSERT INTO plan_settings (plan_id, price_month, price_year, max_staff, max_branches, pos, deposits, deposits_mp, terminal, support) VALUES ('individual', 150, 1500, 1, 1, false, false, false, false, 'correo')`)
	d.a.DB.Exec(context.Background(), `UPDATE businesses SET plan = 'individual'`)
	r := d.a.do(t, "PUT", "/api/booking/settings", d.admin, map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24, "deposit_percent": 30})
	if r.Status != 403 || r.Body["code"] != "plan_feature" {
		t.Fatalf("anticipo en Individual: %d %v", r.Status, r.Body)
	}
	// un negocio que bajó de plan con un anticipo ya configurado deja de pedirlo
	d.a.DB.Exec(context.Background(), `UPDATE businesses SET plan = 'crecimiento'`)
	d.settings(t, 30)
	d.a.DB.Exec(context.Background(), `UPDATE businesses SET plan = 'basico'`)
	token := d.mustBook(t, at(monday, "10:00"), nil)
	if d.apptStatus(t, token) != "scheduled" {
		t.Fatal("sin el plan, la reserva no pide anticipo")
	}
	if rr := d.a.do(t, "PUT", "/api/payments/mercadopago/terminals", d.admin, map[string]any{"branch_id": d.branch, "terminal_id": "T1"}); rr.Status != 403 || rr.Body["code"] != "plan_feature" {
		t.Fatalf("terminal en Esencial: %d %v", rr.Status, rr.Body)
	}
	st := d.a.do(t, "GET", path("/api/payments/point/status?branch_id=", d.branch), d.admin, nil)
	if st.Body["code"] != "plan_feature" || st.Body["ok"] != false {
		t.Fatalf("estado de la terminal: %v", st.Body)
	}
}

func TestBasicPlanIsAppointmentsOnlyAndGrowthIncludesCobros(t *testing.T) {
	a := newTestAPI(t)
	admin := a.mustRegister(t, "Barbería", "dueno@x.com")
	a.addMember(t, admin, "caja@x.com", "cashier")
	caja := login(t, a, "caja@x.com")
	cobros := []struct{ method, url string }{
		{"GET", "/api/cash/current?branch_id=1"}, {"POST", "/api/cash/open"}, {"GET", "/api/sales"}, {"POST", "/api/sales"},
		{"GET", "/api/packages"}, {"POST", "/api/packages"}, {"GET", "/api/reports/sales"}, {"GET", "/api/reports/commissions"},
		{"GET", "/api/commission-rules"}, {"POST", "/api/products"},
	}
	// un negocio nuevo empieza en Crecimiento: todo incluido
	me := a.do(t, "GET", "/api/me", admin, nil).Body["business"].(map[string]any)
	if f := me["plan_features"].(map[string]any); me["plan"] != "crecimiento" || f["pos"] != true || f["deposits"] != true {
		t.Fatalf("plan inicial: %v %v", me["plan"], f)
	}
	if r := a.do(t, "GET", "/api/cash/current?branch_id=1", caja, nil); r.Status == 403 && r.Body["code"] == "plan_feature" {
		t.Fatal("Crecimiento incluye caja")
	}
	a.DB.Exec(context.Background(), `UPDATE businesses SET plan = 'basico'`)
	f := a.do(t, "GET", "/api/me", admin, nil).Body["business"].(map[string]any)["plan_features"].(map[string]any)
	if f["pos"] != false || f["deposits"] != true || f["deposits_mp"] != false || f["terminal"] != false {
		t.Fatalf("Básico no incluye caja ni Mercado Pago, sí anticipo por transferencia: %v", f)
	}
	for i, c := range cobros {
		who := map[string]string{"administrador": admin}
		if i < 5 { // las primeras rutas también son de caja; el resto, solo de administración
			who["cajero"] = caja
		}
		for name, tok := range who {
			if r := a.do(t, c.method, c.url, tok, map[string]any{}); r.Status != 403 || r.Body["code"] != "plan_feature" {
				t.Errorf("%s en %s %s con Básico: %d %v", name, c.method, c.url, r.Status, r.Body)
			}
		}
	}
	// lo que sí incluye: catálogo de servicios, equipo, clientes y citas
	for _, u := range []string{"/api/services", "/api/clients", "/api/team", "/api/branches"} {
		if r := a.do(t, "GET", u, admin, nil); r.Status != 200 {
			t.Errorf("Básico debe permitir %s: %d", u, r.Status)
		}
	}
	if r := a.do(t, "POST", "/api/services", admin, map[string]any{"name": "Corte", "duration_min": 30, "price_cents": 15000}); r.Status != 201 {
		t.Errorf("Básico crea servicios: %d %v", r.Status, r.Body)
	}
	a.DB.Exec(context.Background(), `UPDATE businesses SET plan = 'crecimiento'`)
	if r := a.do(t, "GET", "/api/sales", admin, nil); r.Status != 200 {
		t.Errorf("Crecimiento incluye ventas: %d", r.Status)
	}
}
