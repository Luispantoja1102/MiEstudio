package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"miestudio/backend-go/internal/ratelimit"
)

type onlineFixture struct {
	*fixture
	slug string
}

// newOnline deja la reserva en línea encendida (aviso mínimo 2 h, 30 días, cancelar hasta 2 h antes) y a Marco con horario los lunes.
func newOnline(t *testing.T) *onlineFixture {
	t.Helper()
	f := newFixture(t)
	r := f.a.do(t, "PUT", "/api/booking/settings", f.admin, map[string]any{
		"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2,
		"deposit_percent": 0, "reminder_hours": 24, "show_prices": true, "welcome_text": "Hola"})
	if r.Status != 204 {
		t.Fatalf("settings: %d %v", r.Status, r.Body)
	}
	o := &onlineFixture{fixture: f}
	if err := f.a.DB.QueryRow(context.Background(), `SELECT slug FROM businesses`).Scan(&o.slug); err != nil {
		t.Fatal(err)
	}
	return o
}

func (o *onlineFixture) book(t *testing.T, start string, mod map[string]any) resp {
	t.Helper()
	body := map[string]any{"service_id": o.svc, "branch_id": o.branch, "staff_id": o.marco, "starts_at": start,
		"name": "Laura Pérez", "phone": "664 123 4567", "email": "laura@x.com", "accept_privacy": true, "consent_reminders": true}
	for k, v := range mod {
		body[k] = v
	}
	return o.a.do(t, "POST", "/api/public/"+o.slug+"/bookings", "", body)
}

func (o *onlineFixture) mustBook(t *testing.T, start string, mod map[string]any) string {
	t.Helper()
	r := o.book(t, start, mod)
	if r.Status != 201 {
		t.Fatalf("reservar %s: %d %v", start, r.Status, r.Body)
	}
	return r.Body["token"].(string)
}

func TestPublicHiddenWhenOffUnknownOrSuspended(t *testing.T) {
	o := newOnline(t)
	pub := func(slug string) int { return o.a.do(t, "GET", "/api/public/"+slug, "", nil).Status }
	if pub(o.slug) != 200 {
		t.Fatal("encendida debe responder 200")
	}
	if pub("no-existe") != 404 {
		t.Fatal("desconocida debe ser 404")
	}
	o.a.do(t, "PUT", "/api/booking/settings", o.admin, map[string]any{"enabled": false, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24, "show_prices": true})
	if pub(o.slug) != 404 {
		t.Fatal("apagada debe ser 404")
	}
	o.a.do(t, "PUT", "/api/booking/settings", o.admin, map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24, "show_prices": true})
	o.a.DB.Exec(context.Background(), `UPDATE businesses SET status = 'suspended'`)
	if pub(o.slug) != 404 {
		t.Fatal("suspendida debe ser 404")
	}
	if s := o.a.do(t, "GET", "/api/public/"+o.slug+"/slots?service_id=1&branch_id=1&date="+monday, "", nil).Status; s != 404 {
		t.Fatalf("slots de suspendida: %d", s)
	}
	if s := o.book(t, at(monday, "10:00"), nil).Status; s != 404 {
		t.Fatalf("reserva en suspendida: %d", s)
	}
}

func TestPublicInfoExposesOnlyWhatIsPublic(t *testing.T) {
	o := newOnline(t)
	r := o.a.do(t, "GET", "/api/public/"+o.slug, "", nil)
	if r.Status != 200 {
		t.Fatalf("%d", r.Status)
	}
	raw := string(r.Raw)
	for _, leak := range []string{"admin@x.com", "marco@x.com", "business_id", "password", "Cliente"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("la página pública revela %q: %s", leak, raw)
		}
	}
	if len(r.Body["services"].([]any)) != 1 || len(r.Body["staff"].([]any)) != 1 {
		t.Fatalf("servicios/personas: %s", raw)
	}
	// un servicio apagado para línea no aparece ni se puede reservar
	if s := o.a.do(t, "PATCH", path("/api/services/", o.svc), o.admin, map[string]any{"online_bookable": false}).Status; s != 204 {
		t.Fatalf("patch %d", s)
	}
	r = o.a.do(t, "GET", "/api/public/"+o.slug, "", nil)
	if len(r.Body["services"].([]any)) != 0 {
		t.Fatalf("no debe listarse: %s", r.Raw)
	}
	if s := o.book(t, at(monday, "10:00"), nil).Status; s != 400 {
		t.Fatalf("reservar servicio no disponible: %d", s)
	}
	// precios ocultos
	o.a.do(t, "PATCH", path("/api/services/", o.svc), o.admin, map[string]any{"online_bookable": true})
	o.a.do(t, "PUT", "/api/booking/settings", o.admin, map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24, "show_prices": false})
	r = o.a.do(t, "GET", "/api/public/"+o.slug, "", nil)
	if strings.Contains(string(r.Raw), "price_cents") {
		t.Fatalf("precios deben estar ocultos: %s", r.Raw)
	}
}

func slotsOf(t *testing.T, o *onlineFixture, day string) []string {
	t.Helper()
	r := o.a.do(t, "GET", path("/api/public/", o.slug, "/slots?service_id=", o.svc, "&branch_id=", o.branch, "&date=", day), "", nil)
	if r.Status != 200 {
		t.Fatalf("slots %d %v", r.Status, r.Body)
	}
	var out []string
	for _, s := range r.Body["staff"].([]any) {
		for _, x := range s.(map[string]any)["starts"].([]any) {
			out = append(out, x.(string))
		}
	}
	return out
}

func TestPublicSlotsRespectNoticeHorizonAndBookings(t *testing.T) {
	o := newOnline(t)
	if got := slotsOf(t, o, monday); len(got) == 0 {
		t.Fatal("el lunes debe tener horarios")
	}
	// aviso mínimo: el reloj a las 08:00 locales del lunes (16:00Z) y aviso 2 h -> nada antes de las 10:00
	o.a.Now = func() time.Time { return time.Date(2026, 11, 9, 16, 0, 0, 0, time.UTC) }
	for _, s := range slotsOf(t, o, monday) {
		if tm, _ := time.Parse(time.RFC3339, s); tm.Before(time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)) {
			t.Fatalf("horario %s dentro del aviso mínimo", s)
		}
	}
	// horizonte: 30 días desde el lunes 9 -> el 9 de dic es el último día; el 10 de dic no
	if got := slotsOf(t, o, "2026-12-10"); len(got) != 0 {
		t.Fatalf("fuera del horizonte: %v", got)
	}
	// un horario reservado desaparece de la oferta (y con él la preparación)
	o.a.Now = func() time.Time { return time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC) }
	o.mustBook(t, at(monday, "10:00"), nil)
	for _, s := range slotsOf(t, o, monday) {
		if strings.Contains(s, "T10:00") || strings.Contains(s, "T18:00:00Z") {
			t.Fatalf("horario reservado sigue ofrecido: %s", s)
		}
	}
}

func TestPublicBookValidation(t *testing.T) {
	o := newOnline(t)
	cases := map[string]map[string]any{
		"sin aviso de privacidad": {"accept_privacy": false},
		"teléfono corto":          {"phone": "123"},
		"teléfono con letras":     {"phone": "66412345ab"},
		"nombre vacío":            {"name": " "},
		"correo inválido":         {"email": "nope"},
		"recordatorio sin correo": {"email": "", "consent_reminders": true},
		"horario inválido":        {"starts_at": "mañana"},
		"servicio ajeno":          {"service_id": 99999},
	}
	for name, mod := range cases {
		start := at(monday, "10:00")
		if r := o.book(t, start, mod); r.Status != 400 {
			t.Errorf("%s: esperaba 400, fue %d %v", name, r.Status, r.Body)
		}
	}
	// horarios que no se ofrecen: fuera de horario, no alineado, pasado, otra persona sin horario
	for name, start := range map[string]string{
		"fuera de horario": at(monday, "20:00"),
		"no alineado":      at(monday, "10:07"),
		"pasado":           at("2026-11-01", "10:00"),
		"domingo":          at("2026-11-15", "10:00"),
	} {
		if r := o.book(t, start, nil); r.Status != 409 {
			t.Errorf("%s: esperaba 409, fue %d %v", name, r.Status, r.Body)
		}
	}
	if r := o.book(t, at(monday, "10:00"), map[string]any{"staff_id": o.ana}); r.Status != 409 {
		t.Errorf("persona sin horario: %d", r.Status)
	}
}

func TestPublicBookingCreatesOnlineAppointmentAndDedupesClient(t *testing.T) {
	o := newOnline(t)
	o.mustBook(t, at(monday, "10:00"), nil)
	// mismo número escrito distinto (con lada) = misma persona
	o.mustBook(t, at(monday, "11:00"), map[string]any{"phone": "+52 (664) 123-4567", "name": "Laura P."})
	var clients, appts int
	o.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM clients WHERE name LIKE 'Laura%'`).Scan(&clients)
	o.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM appointments WHERE source = 'online' AND status = 'scheduled' AND privacy_accepted_at IS NOT NULL AND reminders_consent`).Scan(&appts)
	if clients != 1 || appts != 2 {
		t.Fatalf("clientes=%d citas=%d", clients, appts)
	}
	// el cliente del negocio con ese teléfono no cambia de nombre
	var name string
	o.a.DB.QueryRow(context.Background(), `SELECT name FROM clients WHERE name LIKE 'Laura%'`).Scan(&name)
	if name != "Laura Pérez" {
		t.Fatalf("no debe sobrescribirse el nombre: %q", name)
	}
	// aparece en la agenda interna
	r := o.a.do(t, "GET", path("/api/agenda?branch_id=", o.branch, "&date=", monday), o.admin, nil)
	if r.Status != 200 || !strings.Contains(string(r.Raw), "Laura") {
		t.Fatalf("agenda %d %s", r.Status, r.Raw)
	}
}

func TestPublicBookingAnyStaffBalancesAndConflicts(t *testing.T) {
	o := newOnline(t)
	o.setHours(t, o.ana, 1, "09:00", "14:00")
	// Marco ya tiene una cita a las 09:00 -> "cualquiera" a las 10:00 debe ir con Ana
	o.mustBook(t, at(monday, "09:00"), map[string]any{"staff_id": o.marco, "phone": "6641110001"})
	o.mustBook(t, at(monday, "10:00"), map[string]any{"staff_id": 0, "phone": "6641110002"})
	var staff int64
	if err := o.a.DB.QueryRow(context.Background(), `SELECT staff_id FROM appointments WHERE starts_at = $1`, time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)).Scan(&staff); err != nil {
		t.Fatal(err)
	}
	if staff != o.ana {
		t.Fatalf("debía asignarse a Ana (menos carga), fue %d", staff)
	}
	// mismo horario con la misma persona ya no está
	if r := o.book(t, at(monday, "09:00"), map[string]any{"staff_id": o.marco, "phone": "6641110003"}); r.Status != 409 {
		t.Fatalf("doble reserva: %d", r.Status)
	}
	// ya no hay nadie a las 09:00 para "cualquiera"? Ana está libre
	if r := o.book(t, at(monday, "09:00"), map[string]any{"staff_id": 0, "phone": "6641110004"}); r.Status != 201 {
		t.Fatalf("Ana libre: %d", r.Status)
	}
	if r := o.book(t, at(monday, "09:00"), map[string]any{"staff_id": 0, "phone": "6641110005"}); r.Status != 409 {
		t.Fatalf("sin nadie libre: %d", r.Status)
	}
}

func TestPublicBookingRespectsInternalBlocksAndBuffer(t *testing.T) {
	o := newOnline(t)
	o.mustBook(t, at(monday, "10:00"), nil) // ocupa 10:00-10:30 + 10 min de preparación
	if r := o.book(t, at(monday, "10:30"), map[string]any{"phone": "6641110002"}); r.Status != 409 {
		t.Fatalf("la preparación debe bloquear 10:30: %d", r.Status)
	}
	if r := o.book(t, at(monday, "10:45"), map[string]any{"phone": "6641110002"}); r.Status != 201 {
		t.Fatalf("10:45 debe estar libre: %d", r.Status)
	}
}

func TestPublicBookingPhoneCapAndRateLimits(t *testing.T) {
	o := newOnline(t)
	for i, h := range []string{"09:00", "10:00", "11:00"} {
		_ = i
		o.mustBook(t, at(monday, h), nil)
	}
	if r := o.book(t, at(monday, "12:00"), nil); r.Status != 409 {
		t.Fatalf("tope por teléfono: %d %v", r.Status, r.Body)
	}
	// otro teléfono sí puede
	if r := o.book(t, at(monday, "12:00"), map[string]any{"phone": "6649990000", "email": "otro@x.com"}); r.Status != 201 {
		t.Fatalf("otro teléfono: %d", r.Status)
	}

	o.a.Limiter = ratelimit.New()
	n429 := 0
	for i := 0; i < 8; i++ {
		if o.book(t, at(monday, "13:00"), map[string]any{"phone": "66411100" + string(rune('0'+i)) + "0"}).Status == 429 {
			n429++
		}
	}
	// 5 por teléfono no se alcanza con teléfonos distintos; por IP el tope es 10/h
	if n429 != 0 {
		t.Fatalf("no debía limitar aún: %d", n429)
	}
	got := 0
	for i := 0; i < 5; i++ {
		if o.book(t, at(monday, "13:00"), map[string]any{"phone": "6648880000"}).Status == 429 {
			got++
		}
	}
	if got == 0 {
		t.Fatal("el límite por teléfono (5/h) debía activarse")
	}
}

func TestPublicManageAndCancelWindow(t *testing.T) {
	o := newOnline(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	g := o.a.do(t, "GET", "/api/public-appointments/"+token, "", nil)
	if g.Status != 200 || g.Body["can_cancel"] != true || g.Body["client_first"] != "Laura" || g.Body["service"] != "Corte" {
		t.Fatalf("ver: %d %v", g.Status, g.Body)
	}
	raw := string(g.Raw)
	for _, leak := range []string{"664", "laura@x.com", "Pérez"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("el enlace revela %q", leak)
		}
	}
	// dentro de la ventana (menos de 2 h antes) ya no se puede cancelar
	o.a.Now = func() time.Time { return time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC) } // 09:00 local; cita 10:00 -> 1 h
	if r := o.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil); r.Status != 409 {
		t.Fatalf("fuera de plazo: %d", r.Status)
	}
	if g := o.a.do(t, "GET", "/api/public-appointments/"+token, "", nil); g.Body["can_cancel"] != false {
		t.Fatalf("can_cancel: %v", g.Body)
	}
	o.a.Now = func() time.Time { return time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC) }
	if r := o.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil); r.Status != 200 {
		t.Fatalf("cancelar: %d %v", r.Status, r.Body)
	}
	// idempotente y libera el horario
	if r := o.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil); r.Status != 200 {
		t.Fatalf("repetir: %d", r.Status)
	}
	if r := o.book(t, at(monday, "10:00"), map[string]any{"phone": "6641110002"}); r.Status != 201 {
		t.Fatalf("horario liberado: %d", r.Status)
	}
}

func TestPublicTokenShapeAndIsolation(t *testing.T) {
	o := newOnline(t)
	t1 := o.mustBook(t, at(monday, "10:00"), nil)
	if len(t1) != 32 {
		t.Fatalf("token de %d caracteres", len(t1))
	}
	for _, bad := range []string{"x", strings.Repeat("a", 32), strings.Repeat("a", 33), "%27%20OR%201=1--"} {
		if r := o.a.do(t, "GET", "/api/public-appointments/"+bad, "", nil); r.Status != 404 {
			t.Errorf("%q: %d", bad, r.Status)
		}
		if r := o.a.do(t, "POST", "/api/public-appointments/"+bad+"/cancel", "", nil); r.Status != 404 {
			t.Errorf("cancel %q: %d", bad, r.Status)
		}
	}
	t2 := o.mustBook(t, at(monday, "11:00"), nil)
	if t1 == t2 {
		t.Fatal("tokens repetidos")
	}
	// las citas internas no tienen enlace público
	id := o.mustBook2(t)
	_ = id
}

// mustBook2 agenda por el panel y comprueba que no queda un token público.
func (o *onlineFixture) mustBook2(t *testing.T) int64 {
	id := o.fixture.mustBook(t, o.marco, at(monday, "12:00"))
	var tok *string
	o.a.DB.QueryRow(context.Background(), `SELECT public_token FROM appointments WHERE id = $1`, id).Scan(&tok)
	if tok != nil {
		t.Fatalf("cita interna con token: %v", *tok)
	}
	return id
}

func TestPublicTenantIsolation(t *testing.T) {
	o := newOnline(t)
	other := o.a.mustRegister(t, "Otra Barbería", "otra@x.com")
	var slug2 string
	o.a.DB.QueryRow(context.Background(), `SELECT slug FROM businesses WHERE name = 'Otra Barbería'`).Scan(&slug2)
	o.a.do(t, "PUT", "/api/booking/settings", other, map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24, "show_prices": true})
	// un servicio del primer negocio no se puede reservar en la dirección del segundo
	r := o.a.do(t, "POST", "/api/public/"+slug2+"/bookings", "", map[string]any{
		"service_id": o.svc, "branch_id": o.branch, "staff_id": o.marco, "starts_at": at(monday, "10:00"),
		"name": "Laura Pérez", "phone": "6641234567", "accept_privacy": true})
	if r.Status != 400 {
		t.Fatalf("cruce entre negocios: %d %v", r.Status, r.Body)
	}
}

func TestBookingSettingsValidationAndRoles(t *testing.T) {
	o := newOnline(t)
	bad := []map[string]any{
		{"enabled": true, "min_notice_hours": -1, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24},
		{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 0, "cancel_min_hours": 2, "reminder_hours": 24},
		{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 500},
		{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24, "deposit_percent": 101},
	}
	for i, b := range bad {
		if r := o.a.do(t, "PUT", "/api/booking/settings", o.admin, b); r.Status != 400 {
			t.Errorf("caso %d: %d %v", i, r.Status, r.Body)
		}
	}
	// anticipo sin Mercado Pago conectado -> 409 mp_required
	r := o.a.do(t, "PUT", "/api/booking/settings", o.admin, map[string]any{"enabled": true, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24, "deposit_percent": 30})
	if r.Status != 409 || r.Body["code"] != "mp_required" {
		t.Fatalf("anticipo sin MP: %d %v", r.Status, r.Body)
	}
	// solo el administrador
	o.a.addMember(t, o.admin, "recep@x.com", "reception")
	staff := o.a.doLogin(t, "recep@x.com", "supersecreta1").Body["token"].(string)
	if r := o.a.do(t, "GET", "/api/booking/settings", staff, nil); r.Status != 403 {
		t.Fatalf("recepción: %d", r.Status)
	}
	if r := o.a.do(t, "GET", "/api/booking/settings", "", nil); r.Status != 401 {
		t.Fatalf("anónimo: %d", r.Status)
	}
	g := o.a.do(t, "GET", "/api/booking/settings", o.admin, nil)
	if g.Status != 200 || g.Body["public_path"] != "/reservar/"+o.slug {
		t.Fatalf("get: %d %v", g.Status, g.Body)
	}
}

func TestSweepHoldsReleasesExpiredPending(t *testing.T) {
	o := newOnline(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	ctx := context.Background()
	o.a.DB.Exec(ctx, `UPDATE appointments SET status = 'pending_payment', hold_expires_at = $1 WHERE public_token = $2`, o.a.now().Add(15*time.Minute), token)
	// vigente: sigue bloqueando
	if r := o.book(t, at(monday, "10:00"), map[string]any{"phone": "6641110002"}); r.Status != 409 {
		t.Fatalf("reserva vigente debe bloquear: %d", r.Status)
	}
	// vencida: se libera
	o.a.Now = func() time.Time { return time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC) }
	if r := o.book(t, at(monday, "10:00"), map[string]any{"phone": "6641110002"}); r.Status != 201 {
		t.Fatalf("reserva vencida debe liberarse: %d %v", r.Status, r.Body)
	}
	var st string
	o.a.DB.QueryRow(ctx, `SELECT status FROM appointments WHERE public_token = $1`, token).Scan(&st)
	if st != "cancelled" {
		t.Fatalf("estado %s", st)
	}
}
