package api

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

// now de las pruebas: 2026-11-02 00:00 UTC = 1 de noviembre, 16:00 en Tijuana.
var nov1 = time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)

func (o *onlineFixture) engage(t *testing.T, body map[string]any) {
	t.Helper()
	base := map[string]any{"birthday_enabled": false, "birthday_message": "", "winback_enabled": false, "winback_days": 60, "winback_message": "",
		"survey_enabled": false, "survey_delay_hours": 24, "review_url": ""}
	for k, v := range body {
		base[k] = v
	}
	if r := o.a.do(t, "PUT", "/api/engagement", o.admin, base); r.Status != 204 {
		t.Fatalf("ajustes de mensajes: %d %v", r.Status, r.Body)
	}
}

func (o *onlineFixture) newClient(t *testing.T, name, email string, extra map[string]any) int64 {
	t.Helper()
	body := map[string]any{"name": name, "phone": fmt.Sprintf("664%07d", time.Now().UnixNano()%10000000), "email": email}
	for k, v := range extra {
		body[k] = v
	}
	r := o.a.do(t, "POST", "/api/clients", o.admin, body)
	if r.Status != 201 {
		t.Fatalf("cliente %s: %d %v", name, r.Status, r.Body)
	}
	return id(r)
}

var visitSeq int

// visit inserta una cita ya registrada en la base (el API no agenda en el pasado). Cada llamada se corre unos
// minutos hacia atrás para que dos visitas con la misma hora no se encimen con el mismo profesional.
func (o *onlineFixture) visit(t *testing.T, client int64, start time.Time, status string) int64 {
	t.Helper()
	visitSeq++
	start = start.Add(-time.Duration(visitSeq) * 31 * time.Minute)
	var apptID int64
	err := o.a.DB.QueryRow(context.Background(),
		`INSERT INTO appointments (business_id, branch_id, client_id, staff_id, service_id, starts_at, ends_at, blocked_until, status)
		 SELECT business_id, $1::bigint, $2::bigint, $3::bigint, $4::bigint, $5::timestamptz, $5::timestamptz + interval '30 minutes',
		        $5::timestamptz + interval '30 minutes', $6::text FROM clients WHERE id = $2::bigint RETURNING id`,
		o.branch, client, o.marco, o.svc, start, status).Scan(&apptID)
	if err != nil {
		t.Fatal(err)
	}
	return apptID
}

func (o *onlineFixture) subjects(fm *fakeMailer) []string {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	var out []string
	for _, m := range fm.all() {
		out = append(out, m.Subject)
	}
	return out
}

var unsubRe = regexp.MustCompile(`https://app\.test/baja/([A-Za-z0-9_-]{32})`)

func TestEngagementSettingsValidationAndRoles(t *testing.T) {
	o, _ := newMailing(t)
	got := o.a.do(t, "GET", "/api/engagement", o.admin, nil)
	if got.Status != 200 || got.Body["settings"].(map[string]any)["winback_days"] != float64(60) || got.Body["mail_enabled"] != true {
		t.Fatalf("valores iniciales: %v", got.Body)
	}
	good := map[string]any{"birthday_enabled": true, "birthday_message": "10 % de descuento", "winback_enabled": true, "winback_days": 45, "winback_message": "",
		"survey_enabled": true, "survey_delay_hours": 12, "review_url": "https://g.page/r/abc/review"}
	bad := map[string]map[string]any{
		"url http":       {"review_url": "http://g.page/r/abc"},
		"url sin host":   {"review_url": "https://"},
		"url basura":     {"review_url": "javascript:alert(1)"},
		"días bajos":     {"winback_days": 13},
		"días altos":     {"winback_days": 366},
		"encuesta 0 h":   {"survey_delay_hours": 0},
		"encuesta 169 h": {"survey_delay_hours": 169},
		"mensaje largo":  {"birthday_message": strings.Repeat("a", 501)},
	}
	for name, mod := range bad {
		body := map[string]any{}
		for k, v := range good {
			body[k] = v
		}
		for k, v := range mod {
			body[k] = v
		}
		if r := o.a.do(t, "PUT", "/api/engagement", o.admin, body); r.Status != 400 {
			t.Errorf("%s: %d", name, r.Status)
		}
	}
	if r := o.a.do(t, "PUT", "/api/engagement", o.admin, good); r.Status != 204 {
		t.Fatalf("guardar: %d", r.Status)
	}
	s := o.a.do(t, "GET", "/api/engagement", o.admin, nil).Body["settings"].(map[string]any)
	if s["winback_days"] != float64(45) || s["review_url"] != "https://g.page/r/abc/review" || s["birthday_message"] != "10 % de descuento" {
		t.Fatalf("no se guardó: %v", s)
	}
	o.a.addMember(t, o.admin, "recep@x.com", "reception")
	recep := login(t, o.a, "recep@x.com")
	for _, p := range []string{"/api/engagement", "/api/surveys"} {
		if r := o.a.do(t, "GET", p, recep, nil); r.Status != 403 {
			t.Errorf("%s para recepción: %d", p, r.Status)
		}
	}
	if r := o.a.do(t, "PUT", "/api/engagement", recep, good); r.Status != 403 {
		t.Errorf("PUT para recepción: %d", r.Status)
	}
}

func TestBirthdayEmailOnlyToConsentingClientsOnTheirDay(t *testing.T) {
	o, fm := newMailing(t)
	o.engage(t, map[string]any{"birthday_enabled": true, "birthday_message": "Un 10 % <b>de regalo</b>"})
	yes := o.newClient(t, "Ana Cumple", "ana@x.com", map[string]any{"birthday": "1990-11-01", "marketing_consent": true})
	o.newClient(t, "Sin permiso", "sp@x.com", map[string]any{"birthday": "1990-11-01"})
	o.newClient(t, "Otro día", "od@x.com", map[string]any{"birthday": "1990-11-02", "marketing_consent": true})
	o.newClient(t, "Sin correo", "", map[string]any{"birthday": "1990-11-01"})
	_ = yes

	// antes de las 9 de la mañana (hora local) no se manda
	o.setNow(time.Date(2026, 11, 1, 14, 0, 0, 0, time.UTC)) // 06:00 en Tijuana
	if o.process(t) != 0 {
		t.Fatal("no debe mandarse de madrugada")
	}
	o.setNow(nov1)
	if o.process(t) != 1 {
		t.Fatalf("debía salir solo el de Ana, salieron %v", o.subjects(fm))
	}
	m := fm.last()
	if m.To != "ana@x.com" || !strings.Contains(m.Subject, "Feliz cumpleaños") || !strings.Contains(m.Text, "Un 10 % <b>de regalo</b>") {
		t.Fatalf("%+v", m)
	}
	if !strings.Contains(m.HTML, "&lt;b&gt;de regalo") || strings.Contains(m.HTML, "<b>de regalo") {
		t.Fatalf("el mensaje del negocio debe escaparse en el HTML:\n%s", m.HTML)
	}
	if !unsubRe.MatchString(m.Text) || !strings.Contains(m.Text, "https://app.test/reservar/") {
		t.Fatalf("falta el enlace de baja o de reserva:\n%s", m.Text)
	}
	if o.process(t) != 0 {
		t.Fatal("no debe repetirse el mismo año")
	}
	// ni siquiera se encola nada para quien no aceptó promociones, no cumple hoy o no tiene correo
	var queued int
	o.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE kind = 'birthday'`).Scan(&queued)
	if queued != 1 {
		t.Fatalf("correos de cumpleaños en la bandeja: %d", queued)
	}
}

func TestBirthdayDisabledOrMailOffSendsNothing(t *testing.T) {
	o, fm := newMailing(t)
	o.newClient(t, "Ana", "ana@x.com", map[string]any{"birthday": "1990-11-01", "marketing_consent": true})
	if o.process(t) != 0 { // sin activar
		t.Fatal("estaba desactivado")
	}
	o.engage(t, map[string]any{"birthday_enabled": true})
	fm.off = true
	if o.process(t) != 0 {
		t.Fatal("sin correo no se manda")
	}
	fm.off = false
	if o.process(t) != 1 {
		t.Fatal("al volver el correo debía salir")
	}
}

func TestWinbackSendsOncePerLapsedVisit(t *testing.T) {
	o, fm := newMailing(t)
	o.engage(t, map[string]any{"winback_enabled": true, "winback_days": 60, "winback_message": "Vuelve"})
	lapsed := o.newClient(t, "Lapsed", "l@x.com", map[string]any{"marketing_consent": true})
	recent := o.newClient(t, "Reciente", "r@x.com", map[string]any{"marketing_consent": true})
	ancient := o.newClient(t, "Antigua", "a@x.com", map[string]any{"marketing_consent": true})
	nopermit := o.newClient(t, "Sin permiso", "n@x.com", nil)
	booked := o.newClient(t, "Ya agendada", "b@x.com", map[string]any{"marketing_consent": true})
	noshow := o.newClient(t, "Faltista", "f@x.com", map[string]any{"marketing_consent": true})
	day := 24 * time.Hour
	o.visit(t, lapsed, nov1.Add(-90*day), "completed")
	o.visit(t, recent, nov1.Add(-30*day), "completed")
	o.visit(t, ancient, nov1.Add(-400*day), "completed")
	o.visit(t, nopermit, nov1.Add(-90*day), "completed")
	o.visit(t, booked, nov1.Add(-90*day), "completed")
	o.visit(t, booked, nov1.Add(5*day), "scheduled")
	o.visit(t, noshow, nov1.Add(-90*day), "no_show")
	o.setNow(nov1)

	if o.process(t) != 1 {
		t.Fatalf("solo debía salir el de quien lleva 90 días sin venir: %v", o.subjects(fm))
	}
	m := fm.last()
	if m.To != "l@x.com" || !strings.Contains(m.Subject, "Te extrañamos") || !strings.Contains(m.Text, "Vuelve") {
		t.Fatalf("%+v", m)
	}
	if o.process(t) != 0 {
		t.Fatal("no debe repetirse por la misma visita")
	}
	// una venta pagada en mostrador también cuenta como visita
	var biz int64
	o.a.DB.QueryRow(context.Background(), `SELECT business_id FROM clients WHERE id = $1`, recent).Scan(&biz)
	o.setNow(nov1.Add(40 * day)) // ahora «Reciente» lleva 70 días sin venir
	before := fm.count()
	o.process(t)
	var to []string
	for _, m := range fm.all()[before:] {
		to = append(to, m.To)
	}
	// «Reciente» ya cumplió el plazo; «Ya agendada» también (su cita futura ya pasó sin atenderse). Nadie repite.
	if len(to) != 2 || !(contains(to, "r@x.com") && contains(to, "b@x.com")) {
		t.Fatalf("destinatarios: %v", to)
	}
}

func TestSurveyFlowAndReviewLink(t *testing.T) {
	o, fm := newMailing(t)
	o.engage(t, map[string]any{"survey_enabled": true, "survey_delay_hours": 24, "review_url": "https://g.page/r/abc/review"})
	c := o.newClient(t, "Laura", "laura@x.com", map[string]any{"reminders_consent": true})
	noConsent := o.newClient(t, "Sin permiso", "sp@x.com", nil)
	tooSoon := o.newClient(t, "Muy pronto", "mp@x.com", map[string]any{"reminders_consent": true})
	tooOld := o.newClient(t, "Muy vieja", "mv@x.com", map[string]any{"reminders_consent": true})
	notDone := o.newClient(t, "Sin atender", "sa@x.com", map[string]any{"reminders_consent": true})
	h := time.Hour
	o.visit(t, c, nov1.Add(-48*h), "completed")
	o.visit(t, c, nov1.Add(-30*h), "completed") // segunda cita elegible: no genera otra encuesta
	o.visit(t, noConsent, nov1.Add(-48*h), "completed")
	o.visit(t, tooSoon, nov1.Add(-2*h), "completed")
	o.visit(t, tooOld, nov1.Add(-10*24*h), "completed")
	o.visit(t, notDone, nov1.Add(-48*h), "no_show")
	o.setNow(nov1)

	if o.process(t) != 1 {
		t.Fatalf("debía salir una sola encuesta: %v", o.subjects(fm))
	}
	m := fm.last()
	tokRe := regexp.MustCompile(`https://app\.test/encuesta/([A-Za-z0-9_-]{32})`)
	mm := tokRe.FindStringSubmatch(m.Text)
	if m.To != "laura@x.com" || mm == nil || !strings.Contains(m.Text, "Corte") || !unsubRe.MatchString(m.Text) {
		t.Fatalf("%+v", m)
	}
	token := mm[1]
	if !strings.Contains(m.HTML, "/encuesta/"+token+"?s=5") {
		t.Fatalf("faltan los botones de calificación:\n%s", m.HTML)
	}
	if o.process(t) != 0 {
		t.Fatal("no debe repetirse")
	}

	// el enlace por sí solo no califica: hay que contestar con POST
	info := o.a.do(t, "GET", "/api/public-survey/"+token, "", nil)
	if info.Status != 200 || info.Body["answered"] != false || info.Body["review_url"] != "https://g.page/r/abc/review" {
		t.Fatalf("info: %d %v", info.Status, info.Body)
	}
	for _, bad := range []map[string]any{{"score": 0}, {"score": 6}, {"score": 3, "comment": strings.Repeat("a", 1001)}} {
		if r := o.a.do(t, "POST", "/api/public-survey/"+token, "", bad); r.Status != 400 {
			t.Errorf("%v: %d", bad, r.Status)
		}
	}
	// la reseña se ofrece a todos por igual, sin importar la calificación
	r := o.a.do(t, "POST", "/api/public-survey/"+token, "", map[string]any{"score": 2, "comment": "Tardaron mucho"})
	if r.Status != 200 || r.Body["review_url"] != "https://g.page/r/abc/review" {
		t.Fatalf("contestar: %d %v", r.Status, r.Body)
	}
	if r := o.a.do(t, "POST", "/api/public-survey/"+token, "", map[string]any{"score": 5}); r.Status != 409 {
		t.Errorf("segunda respuesta: %d", r.Status)
	}
	if r := o.a.do(t, "POST", "/api/public-survey/"+strings.Repeat("z", 32), "", map[string]any{"score": 5}); r.Status != 404 {
		t.Errorf("token desconocido: %d", r.Status)
	}
	if r := o.a.do(t, "GET", "/api/public-survey/corto", "", nil); r.Status != 404 {
		t.Errorf("token mal formado: %d", r.Status)
	}

	list := o.a.do(t, "GET", "/api/surveys", o.admin, nil)
	items := list.Body["items"].([]any)
	if list.Body["sent"] != float64(1) || list.Body["answered"] != float64(1) || list.Body["average"] != float64(2) || len(items) != 1 ||
		items[0].(map[string]any)["comment"] != "Tardaron mucho" || items[0].(map[string]any)["client"] != "Laura" {
		t.Fatalf("opiniones: %v", list.Body)
	}
	other := o.a.mustRegister(t, "Otro", "otro@x.com")
	if r := o.a.do(t, "GET", "/api/surveys", other, nil); r.Body["sent"] != float64(0) {
		t.Errorf("opiniones ajenas visibles: %v", r.Body)
	}
}

func TestUnsubscribeStopsPromosAndQueuedMail(t *testing.T) {
	o, fm := newMailing(t)
	o.engage(t, map[string]any{"birthday_enabled": true})
	c := o.newClient(t, "Ana", "ana@x.com", map[string]any{"birthday": "1990-11-01", "marketing_consent": true, "reminders_consent": true})
	// se encola pero aún no sale
	if err := o.a.engagementPass(context.Background()); err != nil {
		t.Fatal(err)
	}
	var tok string
	o.a.DB.QueryRow(context.Background(), `SELECT unsub_token FROM clients WHERE id = $1`, c).Scan(&tok)
	if tok == "" {
		t.Fatal("el cliente debe tener enlace de baja")
	}
	info := o.a.do(t, "GET", "/api/public-unsub/"+tok, "", nil)
	if info.Status != 200 || info.Body["subscribed"] != true {
		t.Fatalf("info: %d %v", info.Status, info.Body)
	}
	if r := o.a.do(t, "POST", "/api/public-unsub/"+tok, "", nil); r.Status != 204 {
		t.Fatalf("baja: %d", r.Status)
	}
	if r := o.a.do(t, "POST", "/api/public-unsub/"+tok, "", nil); r.Status != 204 {
		t.Fatalf("baja repetida: %d", r.Status)
	}
	if o.process(t) != 0 || fm.count() != 0 {
		t.Fatal("salió un correo promocional a alguien que se dio de baja")
	}
	got := o.a.do(t, "GET", path("/api/clients/", c), o.admin, nil).Body
	if got["marketing_consent"] != false || got["reminders_consent"] != false {
		t.Fatalf("consentimientos: %v", got)
	}
	if r := o.a.do(t, "POST", "/api/public-unsub/"+strings.Repeat("q", 32), "", nil); r.Status != 404 {
		t.Errorf("token desconocido: %d", r.Status)
	}
	if r := o.a.do(t, "GET", "/api/public-unsub/x", "", nil); r.Status != 404 {
		t.Errorf("token mal formado: %d", r.Status)
	}
}

func TestMarketingConsentComesFromBookingAndNeedsEmail(t *testing.T) {
	o, _ := newMailing(t)
	if r := o.book(t, at(monday, "10:00"), map[string]any{"email": "", "consent_marketing": true, "consent_reminders": false}); r.Status != 400 {
		t.Fatalf("promociones sin correo: %d %v", r.Status, r.Body)
	}
	o.mustBook(t, at(monday, "10:00"), map[string]any{"consent_marketing": true})
	var promos bool
	o.a.DB.QueryRow(context.Background(), `SELECT marketing_consent FROM clients WHERE email = 'laura@x.com'`).Scan(&promos)
	if !promos {
		t.Fatal("la casilla de promociones debía guardarse")
	}
	// una reserva sin casilla no revoca lo aceptado antes
	o.mustBook(t, at(monday, "11:00"), map[string]any{"consent_marketing": false})
	o.a.DB.QueryRow(context.Background(), `SELECT marketing_consent FROM clients WHERE email = 'laura@x.com'`).Scan(&promos)
	if !promos {
		t.Fatal("reservar sin la casilla no debe revocar el consentimiento")
	}
	// en el panel también se exige el correo
	if r := o.a.do(t, "POST", "/api/clients", o.admin, map[string]any{"name": "Sin correo", "phone": "6640001234", "marketing_consent": true}); r.Status != 400 {
		t.Errorf("panel sin correo: %d", r.Status)
	}
}

func TestErasingAClientDropsSurveyCommentsAndQueuedMessages(t *testing.T) {
	o, _ := newMailing(t)
	o.engage(t, map[string]any{"survey_enabled": true})
	c := o.newClient(t, "Laura", "laura@x.com", map[string]any{"reminders_consent": true})
	o.visit(t, c, nov1.Add(-48*time.Hour), "completed")
	o.setNow(nov1)
	if err := o.a.engagementPass(context.Background()); err != nil {
		t.Fatal(err)
	}
	var tok string
	o.a.DB.QueryRow(context.Background(), `SELECT token FROM surveys`).Scan(&tok)
	o.a.do(t, "POST", "/api/public-survey/"+tok, "", map[string]any{"score": 3, "comment": "Mi teléfono es 6641234567"})
	if r := o.a.do(t, "POST", path("/api/clients/", c, "/erase"), o.admin, map[string]any{}); r.Status >= 300 {
		t.Fatalf("borrar: %d %v", r.Status, r.Body)
	}
	var comment string
	var queued int
	o.a.DB.QueryRow(context.Background(), `SELECT comment FROM surveys`).Scan(&comment)
	o.a.DB.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE client_id = $1`, c).Scan(&queued)
	if comment != "" || queued != 0 {
		t.Fatalf("comentario=%q, correos en cola=%d", comment, queued)
	}
}
