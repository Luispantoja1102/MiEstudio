package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"miestudio/backend-go/internal/ratelimit"
)

func TestAddBusinessDays(t *testing.T) {
	mon := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		start time.Time
		n     int
		want  string
	}{
		{mon, 1, "2026-11-03"}, {mon, 5, "2026-11-09"}, {mon, 20, "2026-11-30"},
		{time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC), 1, "2026-11-09"},  // viernes -> lunes
		{time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC), 1, "2026-11-09"},  // sábado -> lunes
		{time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC), 20, "2026-12-04"}, // domingo
	} {
		if got := addBusinessDays(c.start, c.n).Format("2006-01-02"); got != c.want {
			t.Errorf("%s + %d = %s, esperaba %s", c.start.Format("2006-01-02"), c.n, got, c.want)
		}
	}
}

func TestLegalInfoAndTermsAcceptanceAreRecorded(t *testing.T) {
	a := newTestAPI(t)
	a.Operator = Operator{Name: "MiEstudio SA de CV", Address: "Calle 1", Email: "privacidad@miestudio.mx"}
	r := a.do(t, "GET", "/api/legal", "", nil)
	op, _ := r.Body["operator"].(map[string]any)
	if r.Status != 200 || r.Body["version"] != NoticeVersion || op["email"] != "privacidad@miestudio.mx" {
		t.Fatalf("legal: %d %v", r.Status, r.Body)
	}
	a.mustRegister(t, "Barbería", "a@x.com")
	var ver *string
	var at *time.Time
	a.DB.QueryRow(context.Background(), `SELECT terms_version, terms_accepted_at FROM users WHERE email = 'a@x.com'`).Scan(&ver, &at)
	if ver == nil || *ver != NoticeVersion || at == nil {
		t.Fatalf("debe registrarse qué versión aceptó y cuándo: %v %v", ver, at)
	}
}

func TestBookingStoresNoticeVersion(t *testing.T) {
	o := newOnline(t)
	o.mustBook(t, at(monday, "10:00"), nil)
	var v *string
	o.a.DB.QueryRow(context.Background(), `SELECT privacy_version FROM appointments WHERE source = 'online'`).Scan(&v)
	if v == nil || *v != ClientNoticeVersion {
		t.Fatalf("versión del aviso: %v", v)
	}
}

type arcoFx struct {
	*onlineFixture
	fm *fakeMailer
}

func newArco(t *testing.T) *arcoFx {
	o := newOnline(t)
	fm := &fakeMailer{}
	o.a.Mailer, o.a.AppURL = fm, "https://app.test"
	return &arcoFx{o, fm}
}

func (f *arcoFx) send(t *testing.T, mod map[string]any) resp {
	body := map[string]any{"kind": "access", "name": "Laura Pérez", "email": "laura@x.com", "phone": "", "detail": "Quiero saber qué datos tienen de mí", "accept_privacy": true}
	for k, v := range mod {
		body[k] = v
	}
	return f.a.do(t, "POST", "/api/public/"+f.slug+"/arco", "", body)
}

func TestArcoRequestFlow(t *testing.T) {
	f := newArco(t)
	r := f.send(t, nil)
	if r.Status != 201 || r.Body["days"] != float64(20) {
		t.Fatalf("enviar: %d %v", r.Status, r.Body)
	}
	token := r.Body["token"].(string)
	// vence a los 20 días hábiles de la fecha fija del reloj (lunes 2 de nov de 2026)
	var due time.Time
	f.a.DB.QueryRow(context.Background(), `SELECT due_at FROM arco_requests`).Scan(&due)
	if due.Format("2006-01-02") != "2026-11-30" {
		t.Fatalf("fecha límite: %s", due.Format("2006-01-02"))
	}
	waitMail(t, f.fm, 2) // solicitante y administrador
	to := map[string]bool{}
	for _, m := range f.fm.sent {
		to[m.To] = true
	}
	if !to["laura@x.com"] || !to["admin@x.com"] {
		t.Fatalf("correos a: %v", to)
	}
	// estado público: sin datos personales
	s := f.a.do(t, "GET", "/api/public-arco/"+token, "", nil)
	if s.Status != 200 || s.Body["status"] != "received" || s.Body["kind"] != "Acceso" {
		t.Fatalf("estado: %d %v", s.Status, s.Body)
	}
	for _, leak := range []string{"laura@x.com", "Laura", "Quiero saber"} {
		if strings.Contains(string(s.Raw), leak) {
			t.Fatalf("el estado público revela %q", leak)
		}
	}
	for _, bad := range []string{"x", strings.Repeat("a", 32), "%27%20OR"} {
		if r := f.a.do(t, "GET", "/api/public-arco/"+bad, "", nil); r.Status != 404 {
			t.Errorf("%q: %d", bad, r.Status)
		}
	}
}

func TestArcoValidationAndAvailability(t *testing.T) {
	f := newArco(t)
	for name, mod := range map[string]map[string]any{
		"tipo inválido": {"kind": "borrar-todo"}, "sin nombre": {"name": " "}, "sin contacto": {"email": "", "phone": ""},
		"correo malo": {"email": "nope"}, "teléfono malo": {"email": "", "phone": "12"}, "detalle corto": {"detail": "hola"},
		"sin aceptar": {"accept_privacy": false},
	} {
		if r := f.send(t, mod); r.Status != 400 {
			t.Errorf("%s: %d %v", name, r.Status, r.Body)
		}
	}
	if r := f.a.do(t, "POST", "/api/public/no-existe/arco", "", map[string]any{"kind": "access"}); r.Status != 404 {
		t.Errorf("negocio desconocido: %d", r.Status)
	}
	// existe aunque la reserva esté apagada: el negocio tiene clientes de todos modos
	f.a.do(t, "PUT", "/api/booking/settings", f.admin, map[string]any{"enabled": false, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24, "show_prices": true})
	if r := f.send(t, nil); r.Status != 201 {
		t.Errorf("con la reserva apagada: %d", r.Status)
	}
	f.a.DB.Exec(context.Background(), `UPDATE businesses SET status = 'suspended'`)
	if r := f.send(t, nil); r.Status != 404 {
		t.Errorf("negocio suspendido: %d", r.Status)
	}
}

func TestArcoPanelDeadlinesRolesAndTenancy(t *testing.T) {
	f := newArco(t)
	f.send(t, map[string]any{"kind": "cancel"})
	list := f.a.do(t, "GET", "/api/arco", f.admin, nil)
	if list.Status != 200 {
		t.Fatal(list.Status)
	}
	items := listOf(t, f.a, "/api/arco", f.admin)
	if len(items) != 1 || items[0]["status"] != "received" || items[0]["overdue"] != false || items[0]["days_left"].(float64) < 27 {
		t.Fatalf("listado: %v", items)
	}
	// vencida: 20 días hábiles = 28 naturales
	f.setNow(time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC))
	if items = listOf(t, f.a, "/api/arco", f.admin); items[0]["overdue"] != true {
		t.Fatalf("debe marcarse vencida: %v", items[0])
	}
	id := path(int64(items[0]["id"].(float64)))
	// resolver exige explicar
	if r := f.a.do(t, "PATCH", "/api/arco/"+id, f.admin, map[string]any{"status": "resolved"}); r.Status != 400 {
		t.Fatalf("resolver sin explicación: %d", r.Status)
	}
	if r := f.a.do(t, "PATCH", "/api/arco/"+id, f.admin, map[string]any{"status": "bogus"}); r.Status != 400 {
		t.Fatalf("estado inválido: %d", r.Status)
	}
	if r := f.a.do(t, "PATCH", "/api/arco/"+id, f.admin, map[string]any{"status": "in_progress"}); r.Status != 204 {
		t.Fatalf("en proceso: %d", r.Status)
	}
	if r := f.a.do(t, "PATCH", "/api/arco/"+id, f.admin, map[string]any{"status": "resolved", "resolution": "Se eliminaron tus datos el 2 de diciembre."}); r.Status != 204 {
		t.Fatalf("resolver: %d", r.Status)
	}
	items = listOf(t, f.a, "/api/arco", f.admin)
	if items[0]["status"] != "resolved" || items[0]["overdue"] != false || items[0]["resolved_at"] == nil {
		t.Fatalf("resuelta: %v", items[0])
	}
	// el solicitante ve la respuesta
	var token string
	f.a.DB.QueryRow(context.Background(), `SELECT public_token FROM arco_requests`).Scan(&token)
	if s := f.a.do(t, "GET", "/api/public-arco/"+token, "", nil); s.Body["resolution"] != "Se eliminaron tus datos el 2 de diciembre." || s.Body["status"] != "resolved" {
		t.Fatalf("estado público: %v", s.Body)
	}
	// un cliente de otro negocio no puede vincularse; otro negocio no ve ni toca la solicitud
	other := f.a.mustRegister(t, "Otra", "otra@x.com")
	oc := f.a.do(t, "POST", "/api/clients", other, map[string]any{"name": "Ajeno"})
	if r := f.a.do(t, "PATCH", "/api/arco/"+id, f.admin, map[string]any{"client_id": float64(id2(oc))}); r.Status != 400 {
		t.Fatalf("cliente de otro negocio: %d", r.Status)
	}
	if n := len(listOf(t, f.a, "/api/arco", other)); n != 0 {
		t.Fatalf("otro negocio ve %d solicitudes", n)
	}
	if r := f.a.do(t, "PATCH", "/api/arco/"+id, other, map[string]any{"status": "rejected", "resolution": "no corresponde"}); r.Status != 404 {
		t.Fatalf("otro negocio modifica: %d", r.Status)
	}
	// solo administradores
	f.a.addMember(t, f.admin, "recep@x.com", "reception")
	rec := f.a.doLogin(t, "recep@x.com", "supersecreta1").Body["token"].(string)
	if r := f.a.do(t, "GET", "/api/arco", rec, nil); r.Status != 403 {
		t.Fatalf("recepción: %d", r.Status)
	}
}

func id2(r resp) int64 { return int64(r.Body["id"].(float64)) }

func listOf(t *testing.T, a *API, p, token string) []map[string]any {
	t.Helper()
	r := a.do(t, "GET", p, token, nil)
	var out []map[string]any
	if err := jsonUnmarshal(r.Raw, &out); err != nil {
		t.Fatalf("%s: %v %s", p, err, r.Raw)
	}
	return out
}

func TestArcoIsRateLimited(t *testing.T) {
	f := newArco(t)
	f.a.Limiter = newLimiter()
	codes := []int{}
	for i := 0; i < 7; i++ {
		codes = append(codes, f.send(t, nil).Status)
	}
	if codes[4] != 201 || codes[5] != 429 {
		t.Fatalf("máximo 5 por hora: %v", codes)
	}
}

func TestExportClientHasEverythingAndIsScoped(t *testing.T) {
	o := newOnline(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	var cid int64
	o.a.DB.QueryRow(context.Background(), `SELECT client_id FROM appointments WHERE public_token = $1`, token).Scan(&cid)
	o.a.do(t, "POST", path("/api/clients/", cid, "/notes"), o.admin, map[string]any{"body": "Alergia al tinte"})
	r := o.a.do(t, "GET", path("/api/clients/", cid, "/export"), o.admin, nil)
	if r.Status != 200 {
		t.Fatalf("exportar: %d", r.Status)
	}
	raw := string(r.Raw)
	for _, want := range []string{"Laura Pérez", "laura@x.com", "Alergia al tinte", "Corte", "privacy_version"} {
		if !strings.Contains(raw, want) {
			t.Errorf("falta %q en la exportación", want)
		}
	}
	other := o.a.mustRegister(t, "Otra", "otra@x.com")
	if r := o.a.do(t, "GET", path("/api/clients/", cid, "/export"), other, nil); r.Status != 404 {
		t.Fatalf("otro negocio: %d", r.Status)
	}
	o.a.addMember(t, o.admin, "recep@x.com", "reception")
	rec := o.a.doLogin(t, "recep@x.com", "supersecreta1").Body["token"].(string)
	if r := o.a.do(t, "GET", path("/api/clients/", cid, "/export"), rec, nil); r.Status != 403 {
		t.Fatalf("recepción: %d", r.Status)
	}
}

func TestEraseClientAnonymizesButKeepsSales(t *testing.T) {
	d := newDeposit(t)
	d.settings(t, 0) // sin anticipo
	d.openCashAdmin(t)
	token := d.mustBook(t, at(monday, "10:00"), nil)
	appt := d.apptID(t, token)
	var cid int64
	d.a.DB.QueryRow(context.Background(), `SELECT client_id FROM appointments WHERE id = $1`, appt).Scan(&cid)
	d.a.do(t, "POST", path("/api/clients/", cid, "/notes"), d.admin, map[string]any{"body": "Nota privada"})
	d.process(t) // sale la confirmación y queda en la bandeja

	// con una cita por venir no se puede
	if r := d.a.do(t, "POST", path("/api/clients/", cid, "/erase"), d.admin, nil); r.Status != 409 || r.Body["code"] != "client_has_appointments" {
		t.Fatalf("cita pendiente: %d %v", r.Status, r.Body)
	}
	// se atiende y se cobra
	if r := d.sale(t, appt, []any{cash(15000)}, nil); r.Status != 201 {
		t.Fatalf("cobrar: %d %v", r.Status, r.Body)
	}
	if r := d.a.do(t, "POST", path("/api/clients/", cid, "/erase"), d.admin, nil); r.Status != 204 {
		t.Fatalf("borrar: %d", r.Status)
	}
	ctx := context.Background()
	var name, phone, email string
	var anon *time.Time
	var consent bool
	d.a.DB.QueryRow(ctx, `SELECT name, phone, email, anonymized_at, reminders_consent FROM clients WHERE id = $1`, cid).Scan(&name, &phone, &email, &anon, &consent)
	if name != "Cliente eliminado" || phone != "" || email != "" || anon == nil || consent {
		t.Fatalf("cliente: %q %q %q %v", name, phone, email, anon)
	}
	var notes, notifs, sales int
	var tok *string
	var privAt *time.Time
	d.a.DB.QueryRow(ctx, `SELECT count(*) FROM client_notes WHERE client_id = $1`, cid).Scan(&notes)
	d.a.DB.QueryRow(ctx, `SELECT count(*) FROM notifications`).Scan(&notifs)
	d.a.DB.QueryRow(ctx, `SELECT count(*) FROM sales WHERE client_id = $1 AND status = 'paid'`, cid).Scan(&sales)
	d.a.DB.QueryRow(ctx, `SELECT public_token, privacy_accepted_at FROM appointments WHERE id = $1`, appt).Scan(&tok, &privAt)
	if notes != 0 || notifs != 0 || sales != 1 || tok != nil || privAt != nil {
		t.Fatalf("notas=%d correos=%d ventas=%d token=%v aceptación=%v", notes, notifs, sales, tok, privAt)
	}
	if r := d.a.do(t, "GET", "/api/public-appointments/"+token, "", nil); r.Status != 404 {
		t.Fatalf("el enlace de la cita ya no debe servir: %d", r.Status)
	}
	// repetir es inofensivo; otro negocio no puede
	if r := d.a.do(t, "POST", path("/api/clients/", cid, "/erase"), d.admin, nil); r.Status != 204 {
		t.Fatalf("repetir: %d", r.Status)
	}
	other := d.a.mustRegister(t, "Otra", "otra@x.com")
	if r := d.a.do(t, "POST", path("/api/clients/", cid, "/erase"), other, nil); r.Status != 404 {
		t.Fatalf("otro negocio: %d", r.Status)
	}
}

func TestPublicPrivacyPage(t *testing.T) {
	o := newOnline(t)
	o.a.Operator = Operator{Name: "MiEstudio SA"}
	o.a.do(t, "PUT", "/api/booking/settings", o.admin, map[string]any{"enabled": false, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24, "show_prices": true, "privacy_email": "Privacidad@Barberia.mx"})
	r := o.a.do(t, "GET", "/api/public/"+o.slug+"/privacy", "", nil)
	if r.Status != 200 || r.Body["privacy_email"] != "privacidad@barberia.mx" || r.Body["business"] == "" || r.Body["days"] != float64(20) {
		t.Fatalf("aviso: %d %v", r.Status, r.Body)
	}
	if r := o.a.do(t, "GET", "/api/public/no-existe/privacy", "", nil); r.Status != 404 {
		t.Fatalf("desconocido: %d", r.Status)
	}
	if r := o.a.do(t, "PUT", "/api/booking/settings", o.admin, map[string]any{"enabled": false, "min_notice_hours": 2, "max_days_ahead": 30, "cancel_min_hours": 2, "reminder_hours": 24, "show_prices": true, "privacy_email": "no-es-correo"}); r.Status != 400 {
		t.Fatalf("correo inválido: %d", r.Status)
	}
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

func newLimiter() *ratelimit.Limiter { return ratelimit.New() }
