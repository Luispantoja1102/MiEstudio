package api

import (
	"context"
	"strings"
	"testing"
	"time"
)

func (o *onlineFixture) block(t *testing.T, from, to string) int64 {
	t.Helper()
	r := o.a.do(t, "POST", "/api/blocks", o.admin, map[string]any{"user_id": o.marco, "starts_at": at(monday, from), "ends_at": at(monday, to), "reason": "Cierre"})
	if r.Status != 201 {
		t.Fatalf("bloqueo: %d %v", r.Status, r.Body)
	}
	return id(r)
}

func (o *onlineFixture) join(t *testing.T, mod map[string]any) resp {
	t.Helper()
	body := map[string]any{"service_id": o.svc, "branch_id": o.branch, "date": monday, "name": "Laura Pérez", "phone": "664 123 4567",
		"email": "laura@x.com", "accept_privacy": true}
	for k, v := range mod {
		body[k] = v
	}
	return o.a.do(t, "POST", "/api/public/"+o.slug+"/waitlist", "", body)
}

func (o *onlineFixture) waitStatus(t *testing.T) []string {
	rows, err := o.a.DB.Query(context.Background(), `SELECT status FROM waitlist ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		out = append(out, s)
	}
	return out
}

func TestWaitlistOffersFreedSlotsInsideTheWindow(t *testing.T) {
	o, fm := newMailing(t)
	block := o.block(t, "09:00", "14:00")
	r := o.join(t, map[string]any{"from": "10:00", "to": "12:00"})
	if r.Status != 201 {
		t.Fatalf("anotarse: %d %v", r.Status, r.Body)
	}
	token := r.Body["token"].(string)
	// sin horarios libres no se avisa
	if o.process(t) != 0 || fm.count() != 0 {
		t.Fatal("no había lugar: no debía avisarse")
	}
	// se quita el bloqueo: ahora sí
	if r := o.a.do(t, "DELETE", path("/api/blocks/", block), o.admin, nil); r.Status != 204 {
		t.Fatalf("quitar bloqueo: %d", r.Status)
	}
	if o.process(t) != 1 {
		t.Fatalf("debía salir un aviso, salieron %d", fm.count())
	}
	m := fm.last()
	if m.To != "laura@x.com" || !strings.Contains(m.Subject, "Se liberó") {
		t.Fatalf("%+v", m)
	}
	for _, want := range []string{"lunes 9 de noviembre a las 10:00", "https://app.test/reservar/" + o.slug + "?service=", "date=" + monday, "https://app.test/espera/" + token} {
		if !strings.Contains(m.Text, want) {
			t.Fatalf("falta %q en:\n%s", want, m.Text)
		}
	}
	if strings.Contains(m.Text, "a las 09:") || strings.Contains(m.Text, "12:") || strings.Contains(m.Text, "13:") {
		t.Fatalf("ofreció horarios fuera de la franja:\n%s", m.Text)
	}
	if got := o.waitStatus(t); len(got) != 1 || got[0] != "notified" {
		t.Fatalf("estado: %v", got)
	}
	// un solo aviso por solicitud
	if o.process(t) != 0 || fm.count() != 1 {
		t.Fatal("no debe repetirse")
	}
	// al reservar, la solicitud se cierra
	o.mustBook(t, at(monday, "10:00"), map[string]any{"consent_reminders": false})
	o.process(t)
	if got := o.waitStatus(t); got[0] != "booked" {
		t.Fatalf("debía quedar reservada: %v", got)
	}
}

func TestWaitlistNoticesAreCappedByFreeSlots(t *testing.T) {
	o, _ := newMailing(t)
	o.block(t, "09:00", "13:30") // queda poco al final de la tarde
	free := len(o.marcoStarts(t, monday))
	if free < 1 || free > 2 {
		t.Fatalf("la prueba supone 1 o 2 horarios libres, hay %d", free)
	}
	for i, name := range []string{"Uno", "Dos", "Tres", "Cuatro"} {
		r := o.join(t, map[string]any{"name": name + " Pérez", "phone": "66400000" + string(rune('0'+i)) + "0", "email": strings.ToLower(name) + "@x.com"})
		if r.Status != 201 {
			t.Fatalf("anotar %s: %d %v", name, r.Status, r.Body)
		}
	}
	if sent := o.process(t); sent != free {
		t.Fatalf("avisos: %d, horarios libres: %d", sent, free)
	}
	// el orden de llegada manda: se avisó a los primeros
	got := o.waitStatus(t)
	for i, s := range got {
		want := "waiting"
		if i < free {
			want = "notified"
		}
		if s != want {
			t.Fatalf("estado %d: %s, esperado %s (%v)", i, s, want, got)
		}
	}
	// y los avisos recientes siguen ocupando esos lugares
	if o.process(t) != 0 {
		t.Fatal("no debe avisarse a más gente por los mismos horarios")
	}
}

func TestWaitlistExpiresAndSkipsCancelledOrErased(t *testing.T) {
	o, fm := newMailing(t)
	o.block(t, "09:00", "14:00")
	o.join(t, map[string]any{"name": "Cancelada", "phone": "6640000011", "email": "c@x.com"})
	o.join(t, map[string]any{"name": "Borrada", "phone": "6640000022", "email": "b@x.com"})
	o.join(t, map[string]any{"name": "Vence", "phone": "6640000033", "email": "v@x.com"})

	// una persona sale con su enlace
	var tok string
	o.a.DB.QueryRow(context.Background(), `SELECT token FROM waitlist ORDER BY id LIMIT 1`).Scan(&tok)
	if r := o.a.do(t, "POST", "/api/public-waitlist/"+tok+"/leave", "", nil); r.Status != 204 {
		t.Fatalf("salir: %d", r.Status)
	}
	if r := o.a.do(t, "POST", "/api/public-waitlist/"+tok+"/leave", "", nil); r.Status != 204 {
		t.Fatalf("salir otra vez es idempotente: %d", r.Status)
	}
	if r := o.a.do(t, "GET", "/api/public-waitlist/"+tok, "", nil); r.Status != 200 || r.Body["status"] != "cancelled" {
		t.Fatalf("info: %d %v", r.Status, r.Body)
	}
	if r := o.a.do(t, "POST", "/api/public-waitlist/"+strings.Repeat("a", 32)+"/leave", "", nil); r.Status != 404 {
		t.Fatalf("token desconocido: %d", r.Status)
	}
	if r := o.a.do(t, "GET", "/api/public-waitlist/corto", "", nil); r.Status != 404 {
		t.Fatalf("token mal formado: %d", r.Status)
	}

	// al borrar los datos de una persona no se le avisa
	var erased int64
	o.a.DB.QueryRow(context.Background(), `SELECT client_id FROM waitlist WHERE status = 'waiting' ORDER BY id LIMIT 1`).Scan(&erased)
	if r := o.a.do(t, "POST", path("/api/clients/", erased, "/erase"), o.admin, map[string]any{}); r.Status >= 300 {
		t.Fatalf("borrar datos: %d %v", r.Status, r.Body)
	}
	o.process(t)
	if got := o.waitStatus(t); got[0] != "cancelled" || got[1] != "cancelled" || got[2] != "waiting" {
		t.Fatalf("estados: %v", got)
	}

	// pasado el día, vence
	o.setNow(time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC))
	o.process(t)
	if got := o.waitStatus(t); got[2] != "expired" {
		t.Fatalf("debía vencer: %v", got)
	}
	if fm.count() != 0 {
		t.Fatalf("no debió salir ningún correo (todo seguía bloqueado): %d", fm.count())
	}
}

func TestWaitlistNoticeIsSkippedIfPersonLeavesBeforeSending(t *testing.T) {
	o, fm := newMailing(t)
	block := o.block(t, "09:00", "14:00")
	r := o.join(t, nil)
	tok := r.Body["token"].(string)
	o.a.do(t, "DELETE", path("/api/blocks/", block), o.admin, nil)
	if err := o.a.waitlistPass(context.Background()); err != nil { // encola sin enviar
		t.Fatal(err)
	}
	o.a.do(t, "POST", "/api/public-waitlist/"+tok+"/leave", "", nil)
	if o.process(t) != 0 || fm.count() != 0 {
		t.Fatalf("salió un aviso a quien ya se había dado de baja")
	}
}

func TestWaitlistWithoutMailStillExpiresButNeverNotifies(t *testing.T) {
	o := newOnline(t) // sin correo
	o.join(t, nil)
	if _, err := o.a.ProcessNotifications(context.Background()); err != nil {
		t.Fatal(err)
	}
	o.setNow(time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC))
	o.a.ProcessNotifications(context.Background())
	if got := o.waitStatus(t); got[0] != "expired" {
		t.Fatalf("%v", got)
	}
}

func TestPublicWaitlistValidation(t *testing.T) {
	o, _ := newMailing(t)
	bad := map[string]map[string]any{
		"sin privacidad":     {"accept_privacy": false},
		"sin correo":         {"email": ""},
		"correo malo":        {"email": "x"},
		"sin teléfono":       {"phone": ""},
		"sin nombre":         {"name": ""},
		"día pasado":         {"date": "2026-10-30"},
		"muy lejano":         {"date": "2027-03-01"},
		"servicio ajeno":     {"service_id": 99999},
		"franja al revés":    {"from": "12:00", "to": "10:00"},
		"franja a medias":    {"from": "12:00"},
		"franja inválida":    {"from": "ab", "to": "cd"},
		"día inválido":       {"date": "mañana"},
		"persona no atiende": {"staff_id": o.ana},
	}
	// Ana no atiende Corte: se restringe el servicio a Marco
	o.a.do(t, "PUT", path("/api/services/", o.svc, "/staff"), o.admin, map[string]any{"user_ids": []int64{o.marco}})
	for name, mod := range bad {
		if r := o.join(t, mod); r.Status != 400 {
			t.Errorf("%s: %d %v", name, r.Status, r.Body)
		}
	}
	// un servicio que no se ofrece en línea no se acepta en la página pública
	o.a.do(t, "PATCH", path("/api/services/", o.svc), o.admin, map[string]any{"online_bookable": false})
	if r := o.join(t, nil); r.Status != 400 {
		t.Errorf("servicio no en línea: %d", r.Status)
	}
	o.a.do(t, "PATCH", path("/api/services/", o.svc), o.admin, map[string]any{"online_bookable": true})

	if r := o.join(t, nil); r.Status != 201 {
		t.Fatalf("válida: %d %v", r.Status, r.Body)
	}
	if r := o.join(t, nil); r.Status != 409 {
		t.Errorf("duplicada: %d", r.Status)
	}
	// máximo de listas abiertas por persona
	svc2 := id(o.a.do(t, "POST", "/api/services", o.admin, map[string]any{"name": "Barba", "duration_min": 20, "price_cents": 8000, "online_bookable": true}))
	o.a.do(t, "PATCH", path("/api/services/", svc2), o.admin, map[string]any{"online_bookable": true})
	for i, d := range []string{"2026-11-10", "2026-11-11"} {
		if r := o.join(t, map[string]any{"service_id": svc2, "date": d}); r.Status != 201 {
			t.Fatalf("lista %d: %d %v", i, r.Status, r.Body)
		}
	}
	if r := o.join(t, map[string]any{"service_id": svc2, "date": "2026-11-12"}); r.Status != 409 {
		t.Errorf("la cuarta lista debe rechazarse: %d %v", r.Status, r.Body)
	}
}

func TestPanelWaitlist(t *testing.T) {
	o, _ := newMailing(t)
	a := o.a
	a.addMember(t, o.admin, "recep@x.com", "reception")
	a.addMember(t, o.admin, "caja@x.com", "cashier")
	recep, caja := login(t, a, "recep@x.com"), login(t, a, "caja@x.com")
	noMail := id(a.do(t, "POST", "/api/clients", o.admin, map[string]any{"name": "Sin correo", "phone": "6641112222"}))
	withMail := id(a.do(t, "POST", "/api/clients", o.admin, map[string]any{"name": "Con correo", "phone": "6643334444", "email": "con@x.com"}))
	body := func(c int64) map[string]any {
		return map[string]any{"client_id": c, "service_id": o.svc, "branch_id": o.branch, "date": monday, "from": "10:00", "to": "12:00"}
	}
	if r := a.do(t, "POST", "/api/waitlist", recep, body(noMail)); r.Status != 400 {
		t.Errorf("sin correo: %d %v", r.Status, r.Body)
	}
	if r := a.do(t, "POST", "/api/waitlist", caja, body(withMail)); r.Status != 403 {
		t.Errorf("caja no maneja la lista: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/waitlist", recep, body(withMail)); r.Status != 201 {
		t.Fatalf("crear: %d %v", r.Status, r.Body)
	}
	if r := a.do(t, "POST", "/api/waitlist", recep, body(withMail)); r.Status != 409 {
		t.Errorf("duplicada: %d", r.Status)
	}
	other := a.mustRegister(t, "Otro", "otro@x.com")
	if r := a.do(t, "POST", "/api/waitlist", other, body(withMail)); r.Status != 400 {
		t.Errorf("cliente y servicio ajenos: %d", r.Status)
	}
	list := a.do(t, "GET", "/api/waitlist", o.admin, nil).List()
	if len(list) != 1 || list[0]["client_name"] != "Con correo" || list[0]["from"] != "10:00" || list[0]["to"] != "12:00" || list[0]["status"] != "waiting" || list[0]["source"] != "panel" {
		t.Fatalf("lista: %v", list)
	}
	if r := a.do(t, "GET", "/api/waitlist", other, nil); len(r.List()) != 0 {
		t.Errorf("lista ajena visible: %v", r.Body)
	}
	wid := int64(list[0]["id"].(float64))
	if r := a.do(t, "DELETE", path("/api/waitlist/", wid), other, nil); r.Status != 404 {
		t.Errorf("borrar ajena: %d", r.Status)
	}
	if r := a.do(t, "DELETE", path("/api/waitlist/", wid), recep, nil); r.Status != 204 {
		t.Errorf("borrar: %d", r.Status)
	}
	if r := a.do(t, "DELETE", path("/api/waitlist/", wid), recep, nil); r.Status != 404 {
		t.Errorf("borrar dos veces: %d", r.Status)
	}
	if r := a.do(t, "GET", "/api/waitlist", o.admin, nil); len(r.List()) != 0 {
		t.Errorf("debía quedar vacía: %v", r.Body)
	}
}

func TestWaitlistOnlyOffersSlotsOfTheRequestedPerson(t *testing.T) {
	o, fm := newMailing(t)
	// Ana no tiene horario ese día; Marco sí. Quien pidió a Ana no recibe el lugar de Marco.
	if r := o.join(t, map[string]any{"staff_id": o.ana}); r.Status != 201 {
		t.Fatalf("anotarse con Ana: %d %v", r.Status, r.Body)
	}
	if o.process(t) != 0 || fm.count() != 0 {
		t.Fatal("se avisó con un horario de otra persona")
	}
	if r := o.join(t, map[string]any{"staff_id": o.marco, "name": "Otra Persona", "phone": "6640009999", "email": "otra@x.com"}); r.Status != 201 {
		t.Fatalf("anotarse con Marco: %d %v", r.Status, r.Body)
	}
	if o.process(t) != 1 || fm.last().To != "otra@x.com" {
		t.Fatalf("debía avisarse solo a quien pidió a Marco")
	}
}
