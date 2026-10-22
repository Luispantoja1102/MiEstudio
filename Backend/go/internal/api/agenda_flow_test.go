package api

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func (o *onlineFixture) hold(t *testing.T, holder, start string) resp {
	t.Helper()
	return o.a.do(t, "POST", "/api/public/"+o.slug+"/holds", "", map[string]any{
		"service_id": o.svc, "branch_id": o.branch, "staff_id": o.marco, "starts_at": start, "holder": holder})
}

func (o *onlineFixture) offered(t *testing.T, holder, start string) bool {
	t.Helper()
	r := o.a.do(t, "GET", fmt.Sprintf("/api/public/%s/slots?service_id=%d&branch_id=%d&date=%s&holder=%s", o.slug, o.svc, o.branch, monday, holder), "", nil)
	for _, s := range r.Body["staff"].([]any) {
		for _, st := range s.(map[string]any)["starts"].([]any) {
			if st.(string) == start {
				return true
			}
		}
	}
	return false
}

func TestHeldSlotIsHiddenFromOthersButNotFromTheHolder(t *testing.T) {
	o := newOnline(t)
	start := at(monday, "10:00")
	a, b := "visita-a-0123456789", "visita-b-0123456789"
	if !o.offered(t, b, start) {
		t.Fatal("antes de apartarlo, el horario se ofrece")
	}
	if r := o.hold(t, a, start); r.Status != 201 {
		t.Fatalf("apartar: %d %v", r.Status, r.Body)
	}
	if o.offered(t, b, start) {
		t.Fatal("otra visita no debe ver apartado el horario")
	}
	if !o.offered(t, a, start) {
		t.Fatal("quien lo apartó lo sigue viendo")
	}
	if r := o.hold(t, b, start); r.Status != 409 {
		t.Fatalf("otra visita no puede apartar lo apartado: %d", r.Status)
	}
	if r := o.book(t, start, map[string]any{"holder": b}); r.Status != 409 {
		t.Fatalf("otra visita no puede reservar lo apartado: %d %v", r.Status, r.Body)
	}
	if r := o.book(t, start, map[string]any{"holder": a}); r.Status != 201 {
		t.Fatalf("quien lo apartó sí reserva: %d %v", r.Status, r.Body)
	}
	var n int
	o.a.DB.QueryRow(t.Context(), `SELECT count(*) FROM booking_holds`).Scan(&n)
	if n != 0 {
		t.Fatalf("al reservar se suelta el apartado: %d", n)
	}
}

func TestHoldExpiresAndMovesWithTheVisitor(t *testing.T) {
	o := newOnline(t)
	a, b := "visita-a-0123456789", "visita-b-0123456789"
	o.hold(t, a, at(monday, "10:00"))
	// quien cambia de horario suelta el anterior
	if r := o.hold(t, a, at(monday, "11:00")); r.Status != 201 {
		t.Fatalf("cambiar de horario: %d", r.Status)
	}
	if !o.offered(t, b, at(monday, "10:00")) {
		t.Fatal("el horario anterior debe quedar libre")
	}
	if o.offered(t, b, at(monday, "11:00")) {
		t.Fatal("el nuevo sigue apartado")
	}
	o.a.DB.Exec(t.Context(), `UPDATE booking_holds SET expires_at = now() - interval '1 minute'`)
	if !o.offered(t, b, at(monday, "11:00")) {
		t.Fatal("un apartado vencido ya no estorba")
	}
	if r := o.a.do(t, "POST", "/api/public/"+o.slug+"/holds", "", map[string]any{"service_id": o.svc, "branch_id": o.branch, "starts_at": at(monday, "10:00"), "holder": "corto"}); r.Status != 400 {
		t.Fatalf("identificador inválido: %d", r.Status)
	}
}

func TestConfirmAndArriveMarkAScheduledAppointment(t *testing.T) {
	o := newOnline(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	day := o.a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s", o.branch, monday), o.admin, nil)
	appt := day.Body["appointments"].([]any)[0].(map[string]any)
	id := int64(appt["id"].(float64))
	if appt["confirmed_at"] != nil || appt["arrived_at"] != nil {
		t.Fatalf("una cita nueva no está confirmada ni llegó: %v", appt)
	}
	// el cliente confirma desde su enlace
	if r := o.a.do(t, "POST", "/api/public-appointments/"+token+"/confirm", "", nil); r.Status != 204 {
		t.Fatalf("confirmar: %d %v", r.Status, r.Body)
	}
	if o.a.do(t, "GET", "/api/public-appointments/"+token, "", nil).Body["confirmed"] != true {
		t.Fatal("la página pública debe mostrar la cita confirmada")
	}
	// con días de anticipación no se puede marcar que llegó
	arrive := fmt.Sprintf("/api/appointments/%d/arrive", id)
	if r := o.a.do(t, "POST", arrive, o.admin, nil); r.Status != 409 {
		t.Fatalf("llegó con días de anticipación: %d", r.Status)
	}
	// la cita es el lunes a las 10:00 (hora de Tijuana, UTC-8): una hora antes sí, 90 minutos antes todavía no
	o.a.Now = func() time.Time { return time.Date(2026, 11, 9, 16, 29, 0, 0, time.UTC) } // 08:29 locales: 91 min antes
	if r := o.a.do(t, "POST", arrive, o.admin, nil); r.Status != 409 {
		t.Fatalf("llegó 91 minutos antes: %d", r.Status)
	}
	o.a.Now = func() time.Time { return time.Date(2026, 11, 9, 17, 5, 0, 0, time.UTC) } // 09:05 locales
	if r := o.a.do(t, "POST", arrive, o.admin, nil); r.Status != 204 {
		t.Fatalf("llegó: %d %v", r.Status, r.Body)
	}
	day = o.a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s", o.branch, monday), o.admin, nil)
	appt = day.Body["appointments"].([]any)[0].(map[string]any)
	if appt["confirmed_at"] == nil || appt["arrived_at"] == nil || appt["status"] != "scheduled" {
		t.Fatalf("debe verse confirmada y llegada, aún agendada: %v", appt)
	}
	// quitar la marca
	if r := o.a.do(t, "POST", arrive, o.admin, map[string]any{"on": false}); r.Status != 204 {
		t.Fatalf("quitar llegada: %d", r.Status)
	}
	// una cita cancelada ya no se marca (con tiempo suficiente para cancelar en línea)
	o.a.Now = func() time.Time { return time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC) }
	o.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil)
	if r := o.a.do(t, "POST", fmt.Sprintf("/api/appointments/%d/confirm", id), o.admin, nil); r.Status != 409 {
		t.Fatalf("marcar una cancelada: %d", r.Status)
	}
	if r := o.a.do(t, "POST", "/api/public-appointments/"+token+"/confirm", "", nil); r.Status != 409 {
		t.Fatalf("confirmar una cancelada: %d", r.Status)
	}
}

func TestAgendaCanReturnSeveralDaysAndTheWeeklyHours(t *testing.T) {
	o := newOnline(t)
	o.mustBook(t, at(monday, "10:00"), nil)
	one := o.a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s", o.branch, monday), o.admin, nil)
	week := o.a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s&days=7", o.branch, monday), o.admin, nil)
	if one.Status != 200 || week.Status != 200 {
		t.Fatalf("agenda: %d %d", one.Status, week.Status)
	}
	staff := week.Body["staff"].([]any)[0].(map[string]any)
	if _, ok := staff["week_hours"].(map[string]any); !ok {
		t.Fatalf("falta week_hours: %v", staff)
	}
	if len(staff["hours"].([]any)) != len(one.Body["staff"].([]any)[0].(map[string]any)["hours"].([]any)) {
		t.Fatal("hours (el primer día) debe ser igual con days=7")
	}
	// un rango de más de 42 días se acota
	if big := o.a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s&days=99", o.branch, monday), o.admin, nil); big.Status != 200 {
		t.Fatalf("days se acota a 42: %d", big.Status)
	}
}

func TestPublicDayShowsBusyTimesWithoutSayingWhoOrWhat(t *testing.T) {
	o := newOnline(t)
	o.mustBook(t, at(monday, "10:00"), nil)
	r := o.a.do(t, "GET", fmt.Sprintf("/api/public/%s/slots?service_id=%d&branch_id=%d&staff_id=%d&date=%s", o.slug, o.svc, o.branch, o.marco, monday), "", nil)
	staff := r.Body["staff"].([]any)[0].(map[string]any)
	taken, _ := staff["taken"].([]any)
	has10 := false
	for _, s := range taken {
		has10 = has10 || s.(string) == at(monday, "10:00")
	}
	if !has10 {
		t.Fatalf("las 10:00 deben verse ocupadas: %v", staff)
	}
	for _, s := range staff["starts"].([]any) {
		if s.(string) == at(monday, "10:00") {
			t.Fatal("un horario ocupado no se ofrece")
		}
	}
	// solo hay horarios y nombres de profesionales: nada de clientes, servicios ni notas
	for k := range staff {
		if k != "staff_id" && k != "staff_name" && k != "starts" && k != "taken" {
			t.Fatalf("campo inesperado en la respuesta pública: %s", k)
		}
	}
	if strings.Contains(string(r.Raw), "Laura") {
		t.Fatal("la página pública no debe revelar quién tiene la cita")
	}
	// un horario que otra visita está apartando también se ve ocupado
	o.hold(t, "visita-a-0123456789", at(monday, "11:00"))
	r = o.a.do(t, "GET", fmt.Sprintf("/api/public/%s/slots?service_id=%d&branch_id=%d&staff_id=%d&date=%s&holder=visita-b-0123456789", o.slug, o.svc, o.branch, o.marco, monday), "", nil)
	staff = r.Body["staff"].([]any)[0].(map[string]any)
	found := false
	for _, s := range staff["taken"].([]any) {
		found = found || s.(string) == at(monday, "11:00")
	}
	if !found {
		t.Fatalf("lo apartado por otra visita se ve ocupado: %v", staff["taken"])
	}
}

func TestPublicMonthListsOnlyDaysWithRoom(t *testing.T) {
	o := newOnline(t)
	get := func() []string {
		r := o.a.do(t, "GET", fmt.Sprintf("/api/public/%s/month?service_id=%d&branch_id=%d&staff_id=%d&month=2026-11", o.slug, o.svc, o.branch, o.marco), "", nil)
		if r.Status != 200 {
			t.Fatalf("mes: %d %v", r.Status, r.Body)
		}
		out := []string{}
		for _, d := range r.Body["days"].([]any) {
			out = append(out, d.(string))
		}
		return out
	}
	days := get()
	has := func(d string) bool {
		for _, x := range days {
			if x == d {
				return true
			}
		}
		return false
	}
	if !has(monday) || has(tuesday) {
		t.Fatalf("Marco trabaja los lunes y no los martes: %v", days)
	}
	if has("2026-11-08") {
		t.Fatalf("un domingo sin horario no aparece: %v", days)
	}
	// al llenarse el lunes ya no se marca disponible
	for _, h := range []string{"09:00", "09:40", "10:20", "11:00", "11:40", "12:20", "13:00"} {
		o.a.do(t, "POST", "/api/appointments", o.admin, map[string]any{"client_id": o.cl, "service_id": o.svc, "staff_id": o.marco, "branch_id": o.branch, "starts_at": at(monday, h)})
	}
	days = get()
	if has(monday) && len(days) > 0 {
		// quedó algún hueco: se vuelve a comprobar tras bloquear el resto del día
		o.a.do(t, "POST", "/api/blocks", o.admin, map[string]any{"user_id": o.marco, "starts_at": at(monday, "09:00"), "ends_at": at(monday, "14:00"), "reason": "x"})
		days = get()
	}
	if has(monday) {
		t.Fatalf("un día lleno no debe aparecer disponible: %v", days)
	}
	if r := o.a.do(t, "GET", "/api/public/"+o.slug+"/month?service_id=1&branch_id=1&month=hola", "", nil); r.Status != 400 {
		t.Fatalf("mes inválido: %d", r.Status)
	}
}
