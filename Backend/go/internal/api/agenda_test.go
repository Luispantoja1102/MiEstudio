package api

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Lunes 9 de nov de 2026 (zona America/Tijuana, UTC-8 en noviembre).
const (
	monday  = "2026-11-09"
	tuesday = "2026-11-10"
)

type fixture struct {
	a                           *API
	admin                       string
	branch, marco, ana, svc, cl int64
}

func at(day, clock string) string { return day + "T" + clock + ":00-08:00" }

// newFixture crea un negocio con una sucursal, dos profesionales (Marco con horario lunes 09:00–14:00),
// un servicio de 30 min con 10 min de preparación y un cliente. El reloj queda fijo antes del lunes.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	a := newTestAPI(t)
	a.Now = func() time.Time { return time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC) }
	admin := a.mustRegister(t, "Barbería", "admin@x.com")
	// las pruebas mueven el reloj por semanas: el negocio es de pago (sin vencimiento) para no depender de la prueba de 14 días
	a.DB.Exec(context.Background(), `UPDATE businesses SET status = 'active'`)
	f := &fixture{a: a, admin: admin}
	f.branch = list[branch](t, a, "/api/branches", admin)[0].ID

	mk := func(name, email string) int64 {
		r := a.do(t, "POST", "/api/team", admin, map[string]any{
			"name": name, "email": email, "password": "supersecreta1", "role": "staff", "bookable": true})
		if r.Status != 201 {
			t.Fatalf("crear %s: %d %v", name, r.Status, r.Body)
		}
		return id(r)
	}
	f.marco, f.ana = mk("Marco", "marco@x.com"), mk("Ana", "ana@x.com")
	f.setHours(t, f.marco, 1, "09:00", "14:00")
	f.svc = id(a.do(t, "POST", "/api/services", admin, map[string]any{"name": "Corte", "duration_min": 30, "buffer_min": 10, "price_cents": 15000}))
	f.cl = id(a.do(t, "POST", "/api/clients", admin, map[string]any{"name": "Cliente"}))
	return f
}

func (f *fixture) setHours(t *testing.T, user int64, weekday int, start, end string) {
	t.Helper()
	r := f.a.do(t, "PUT", path("/api/team/", user, "/hours"), f.admin, map[string]any{
		"blocks": []map[string]any{{"branch_id": f.branch, "weekday": weekday, "start": start, "end": end}}})
	if r.Status != 204 {
		t.Fatalf("horario: %d %v", r.Status, r.Body)
	}
}

func (f *fixture) book(t *testing.T, token string, staff int64, start string) resp {
	return f.a.do(t, "POST", "/api/appointments", token, map[string]any{
		"client_id": f.cl, "service_id": f.svc, "staff_id": staff, "branch_id": f.branch, "starts_at": start})
}

func (f *fixture) mustBook(t *testing.T, staff int64, start string) int64 {
	t.Helper()
	r := f.book(t, f.admin, staff, start)
	if r.Status != 201 {
		t.Fatalf("agendar %s: %d %v", start, r.Status, r.Body)
	}
	return id(r)
}

type slotsResp struct {
	Staff []struct {
		StaffID int64    `json:"staff_id"`
		Starts  []string `json:"starts"`
	} `json:"staff"`
}

func (f *fixture) slots(t *testing.T, token, day string) slotsResp {
	t.Helper()
	url := fmt.Sprintf("/api/agenda/slots?service_id=%d&branch_id=%d&date=%s", f.svc, f.branch, day)
	r := f.a.do(t, "GET", url, token, nil)
	if r.Status != 200 {
		t.Fatalf("slots: %d %v", r.Status, r.Body)
	}
	var out slotsResp
	mustJSON(t, r.Raw, &out)
	return out
}

func (f *fixture) marcoStarts(t *testing.T, day string) []string {
	for _, s := range f.slots(t, f.admin, day).Staff {
		if s.StaffID == f.marco {
			return s.Starts
		}
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func TestSlotsEndpoint(t *testing.T) {
	f := newFixture(t)
	starts := f.marcoStarts(t, monday)
	if len(starts) != 19 || starts[0] != at(monday, "09:00") || starts[18] != at(monday, "13:30") {
		t.Fatalf("slots de Marco: %d %v", len(starts), starts)
	}
	// Ana no tiene horario: no aparece
	for _, s := range f.slots(t, f.admin, monday).Staff {
		if s.StaffID == f.ana {
			t.Fatal("Ana no tiene horario y no debería tener huecos")
		}
	}
	// martes: nadie trabaja
	if got := f.slots(t, f.admin, tuesday); len(got.Staff) != 0 {
		t.Fatalf("martes: %+v", got)
	}
	// con una cita a las 09:00 (30 min + 10 de preparación) el siguiente hueco es 09:45
	f.mustBook(t, f.marco, at(monday, "09:00"))
	starts = f.marcoStarts(t, monday)
	if starts[0] != at(monday, "09:45") {
		t.Fatalf("tras la cita, primer hueco: %s", starts[0])
	}
}

func TestSlotsValidation(t *testing.T) {
	f := newFixture(t)
	base := fmt.Sprintf("/api/agenda/slots?service_id=%d&branch_id=%d", f.svc, f.branch)
	if r := f.a.do(t, "GET", base+"&date=09/11/2026", f.admin, nil); r.Status != 400 {
		t.Errorf("fecha inválida: %d", r.Status)
	}
	if r := f.a.do(t, "GET", "/api/agenda/slots?date="+monday, f.admin, nil); r.Status != 400 {
		t.Errorf("sin servicio ni sucursal: %d", r.Status)
	}
	if r := f.a.do(t, "GET", fmt.Sprintf("/api/agenda/slots?service_id=99999&branch_id=%d&date=%s", f.branch, monday), f.admin, nil); r.Status != 404 {
		t.Errorf("servicio inexistente: %d", r.Status)
	}
}

func TestCreateAppointmentRules(t *testing.T) {
	f := newFixture(t)
	a := f.a

	check := func(name, start string, want int) {
		t.Helper()
		if r := f.book(t, f.admin, f.marco, start); r.Status != want {
			t.Errorf("%s (%s): esperaba %d, obtuvo %d %v", name, start, want, r.Status, r.Body)
		}
	}
	check("día sin horario", at(tuesday, "10:00"), 400)
	check("antes de abrir", at(monday, "08:30"), 400)
	check("termina después de cerrar", at(monday, "13:45"), 400)
	check("justo al cierre", at(monday, "13:30"), 201)
	check("fecha inválida", "mañana", 400)
	check("primera", at(monday, "09:00"), 201)
	check("encima de la anterior", at(monday, "09:15"), 409)
	check("dentro de la preparación", at(monday, "09:30"), 409)
	check("justo tras la preparación", at(monday, "09:40"), 201)

	// profesional sin horario ese día, no asignada, no en agenda, inactiva
	if r := f.book(t, f.admin, f.ana, at(monday, "10:00")); r.Status != 400 {
		t.Errorf("Ana sin horario: %d", r.Status)
	}
	other := a.mustRegister(t, "Otro", "otro@x.com")
	foreignStaff := a.addMember(t, other, "fs@x.com", "staff")
	if r := f.book(t, f.admin, foreignStaff, at(monday, "10:00")); r.Status != 400 {
		t.Errorf("profesional de otro negocio: %d", r.Status)
	}
	noBook := a.addMember(t, f.admin, "nobook@x.com", "staff")
	if r := f.book(t, f.admin, noBook, at(monday, "10:00")); r.Status != 400 {
		t.Errorf("persona fuera de la agenda: %d", r.Status)
	}
	// servicio asignado solo a Ana: Marco ya no puede
	a.do(t, "PUT", path("/api/services/", f.svc, "/staff"), f.admin, map[string]any{"user_ids": []int64{f.ana}})
	if r := f.book(t, f.admin, f.marco, at(monday, "11:00")); r.Status != 400 {
		t.Errorf("Marco no está asignado al servicio: %d", r.Status)
	}
	a.do(t, "PUT", path("/api/services/", f.svc, "/staff"), f.admin, map[string]any{"user_ids": []int64{}})
	// servicio inactivo
	a.do(t, "PATCH", path("/api/services/", f.svc), f.admin, map[string]any{"active": false})
	if r := f.book(t, f.admin, f.marco, at(monday, "11:00")); r.Status != 400 {
		t.Errorf("servicio inactivo: %d", r.Status)
	}
}

func TestAppointmentTenantIsolation(t *testing.T) {
	f := newFixture(t)
	a := f.a
	apptID := f.mustBook(t, f.marco, at(monday, "09:00"))
	other := a.mustRegister(t, "Otro", "otro@x.com")

	// otro negocio no puede usar cliente/servicio/sucursal de A al agendar
	if r := a.do(t, "POST", "/api/appointments", other, map[string]any{
		"client_id": f.cl, "service_id": f.svc, "staff_id": f.marco, "branch_id": f.branch, "starts_at": at(monday, "10:00")}); r.Status != 400 {
		t.Errorf("agendar con datos de otro negocio: %d", r.Status)
	}
	if r := a.do(t, "PATCH", path("/api/appointments/", apptID), other, map[string]any{"status": "cancelled"}); r.Status != 404 {
		t.Errorf("cancelar cita ajena: %d", r.Status)
	}
	if r := a.do(t, "POST", path("/api/appointments/", apptID, "/move"), other, map[string]any{"starts_at": at(monday, "11:00")}); r.Status != 404 {
		t.Errorf("mover cita ajena: %d", r.Status)
	}
	otherBranch := list[branch](t, a, "/api/branches", other)[0].ID
	if r := a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s", f.branch, monday), other, nil); r.Status != 404 {
		t.Errorf("agenda con sucursal ajena: %d", r.Status)
	}
	r := a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s", otherBranch, monday), other, nil)
	if r.Status != 200 || len(r.Body["appointments"].([]any)) != 0 || len(r.Body["staff"].([]any)) != 0 {
		t.Errorf("agenda de B ve datos de A: %v", r.Body)
	}
	// y la cita sigue intacta
	if f.a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s", f.branch, monday), f.admin, nil).Body["appointments"].([]any)[0].(map[string]any)["status"] != "scheduled" {
		t.Fatal("la cita de A cambió")
	}
}

func TestAppointmentLifecycle(t *testing.T) {
	f := newFixture(t)
	a := f.a
	first := f.mustBook(t, f.marco, at(monday, "09:00"))

	// completar bloquea cambios posteriores pero la cita sigue ocupando el horario
	if r := a.do(t, "PATCH", path("/api/appointments/", first), f.admin, map[string]any{"status": "completed"}); r.Status != 204 {
		t.Fatalf("completar: %d", r.Status)
	}
	if r := a.do(t, "PATCH", path("/api/appointments/", first), f.admin, map[string]any{"status": "cancelled"}); r.Status != 409 {
		t.Fatalf("cancelar una completada: %d", r.Status)
	}
	if contains(f.marcoStarts(t, monday), at(monday, "09:00")) {
		t.Fatal("una cita completada sigue ocupando el horario")
	}

	// cancelar libera el horario
	second := f.mustBook(t, f.marco, at(monday, "10:00"))
	if r := a.do(t, "PATCH", path("/api/appointments/", second), f.admin, map[string]any{"status": "cancelled"}); r.Status != 204 {
		t.Fatalf("cancelar: %d", r.Status)
	}
	if !contains(f.marcoStarts(t, monday), at(monday, "10:00")) {
		t.Fatal("cancelar debe liberar el horario")
	}
	f.mustBook(t, f.marco, at(monday, "10:00")) // se puede volver a agendar
	// no_show también libera
	third := f.mustBook(t, f.marco, at(monday, "11:00"))
	a.do(t, "PATCH", path("/api/appointments/", third), f.admin, map[string]any{"status": "no_show"})
	if !contains(f.marcoStarts(t, monday), at(monday, "11:00")) {
		t.Fatal("no_show debe liberar el horario")
	}
	// notas y estados inválidos
	fourth := f.mustBook(t, f.marco, at(monday, "12:00"))
	if r := a.do(t, "PATCH", path("/api/appointments/", fourth), f.admin, map[string]any{"notes": "Trae foto de referencia"}); r.Status != 204 {
		t.Fatalf("notas: %d", r.Status)
	}
	for _, body := range []map[string]any{{"status": "scheduled"}, {"status": "inventado"}, {}} {
		if r := a.do(t, "PATCH", path("/api/appointments/", fourth), f.admin, body); r.Status != 400 {
			t.Errorf("%v: esperaba 400, obtuvo %d", body, r.Status)
		}
	}
}

func TestMoveAppointment(t *testing.T) {
	f := newFixture(t)
	a := f.a
	f.setHours(t, f.ana, 1, "09:00", "12:00")
	first := f.mustBook(t, f.marco, at(monday, "09:00"))
	f.mustBook(t, f.marco, at(monday, "10:00"))
	move := func(id int64, body map[string]any) resp {
		return a.do(t, "POST", path("/api/appointments/", id, "/move"), f.admin, body)
	}

	if r := move(first, map[string]any{"starts_at": at(monday, "10:15")}); r.Status != 409 {
		t.Errorf("mover encima de otra: %d", r.Status)
	}
	if r := move(first, map[string]any{"starts_at": at(monday, "15:00")}); r.Status != 400 {
		t.Errorf("mover fuera de horario: %d", r.Status)
	}
	if r := move(first, map[string]any{"starts_at": at(monday, "09:15")}); r.Status != 204 {
		t.Errorf("mover sobre sí misma (09:00 → 09:15): %d", r.Status)
	}
	if r := move(first, map[string]any{"starts_at": at(monday, "11:00")}); r.Status != 204 {
		t.Errorf("mover a un hueco libre: %d", r.Status)
	}
	if !contains(f.marcoStarts(t, monday), at(monday, "09:00")) {
		t.Error("el horario anterior debe quedar libre")
	}
	// cambiar de profesional: Ana trabaja hasta las 12:00
	if r := move(first, map[string]any{"starts_at": at(monday, "09:00"), "staff_id": f.ana}); r.Status != 204 {
		t.Errorf("pasar a Ana: %d", r.Status)
	}
	if r := move(first, map[string]any{"starts_at": at(monday, "13:00"), "staff_id": f.ana}); r.Status != 400 {
		t.Errorf("Ana no trabaja a las 13:00: %d", r.Status)
	}
	if r := move(first, map[string]any{"starts_at": "ayer"}); r.Status != 400 {
		t.Errorf("fecha inválida: %d", r.Status)
	}
	// completada: no se reprograma
	a.do(t, "PATCH", path("/api/appointments/", first), f.admin, map[string]any{"status": "completed"})
	if r := move(first, map[string]any{"starts_at": at(monday, "09:30")}); r.Status != 409 {
		t.Errorf("mover una completada: %d", r.Status)
	}
}

func TestBlocks(t *testing.T) {
	f := newFixture(t)
	a := f.a
	marcoTok := login(t, a, "marco@x.com")
	anaTok := login(t, a, "ana@x.com")
	block := func(token string, user int64, start, end string) resp {
		return a.do(t, "POST", "/api/blocks", token, map[string]any{"user_id": user, "starts_at": start, "ends_at": end, "reason": "Comida"})
	}

	r := block(f.admin, f.marco, at(monday, "10:00"), at(monday, "11:00"))
	if r.Status != 201 {
		t.Fatalf("crear bloqueo: %d %v", r.Status, r.Body)
	}
	blockID := id(r)
	starts := f.marcoStarts(t, monday)
	if contains(starts, at(monday, "10:15")) || contains(starts, at(monday, "09:45")) || !contains(starts, at(monday, "09:15")) || !contains(starts, at(monday, "11:00")) {
		t.Fatalf("el bloqueo no se refleja bien en los huecos: %v", starts)
	}
	if r := f.book(t, f.admin, f.marco, at(monday, "10:00")); r.Status != 409 {
		t.Errorf("agendar sobre un bloqueo: %d", r.Status)
	}
	// no se bloquea sobre citas agendadas
	f.mustBook(t, f.marco, at(monday, "12:00"))
	if r := block(f.admin, f.marco, at(monday, "12:15"), at(monday, "13:00")); r.Status != 409 {
		t.Errorf("bloquear sobre una cita: %d", r.Status)
	}
	// permisos: el profesional solo toca lo suyo
	if r := block(anaTok, f.marco, at(monday, "13:00"), at(monday, "13:30")); r.Status != 403 {
		t.Errorf("Ana bloqueando a Marco: %d", r.Status)
	}
	if r := block(marcoTok, f.marco, at(monday, "13:00"), at(monday, "13:30")); r.Status != 201 {
		t.Errorf("Marco bloqueando lo suyo: %d", r.Status)
	}
	if r := a.do(t, "DELETE", path("/api/blocks/", blockID), anaTok, nil); r.Status != 404 {
		t.Errorf("Ana quitando bloqueo de Marco: %d", r.Status)
	}
	if r := block(f.admin, f.marco, at(monday, "11:00"), at(monday, "10:00")); r.Status != 400 {
		t.Errorf("fin antes del inicio: %d", r.Status)
	}
	// otro negocio
	other := a.mustRegister(t, "Otro", "otro@x.com")
	if r := a.do(t, "DELETE", path("/api/blocks/", blockID), other, nil); r.Status != 404 {
		t.Errorf("otro negocio quitando bloqueo: %d", r.Status)
	}
	if r := block(other, f.marco, at(monday, "14:00"), at(monday, "15:00")); r.Status != 400 {
		t.Errorf("otro negocio bloqueando a Marco: %d", r.Status)
	}
	// quitar el bloqueo libera los huecos
	if r := a.do(t, "DELETE", path("/api/blocks/", blockID), f.admin, nil); r.Status != 204 {
		t.Fatalf("quitar bloqueo: %d", r.Status)
	}
	if !contains(f.marcoStarts(t, monday), at(monday, "10:15")) {
		t.Fatal("al quitar el bloqueo, el horario debe liberarse")
	}
}

func TestAgendaDayRolesAndTimezone(t *testing.T) {
	f := newFixture(t)
	a := f.a
	a.addMember(t, f.admin, "caja@x.com", "cashier")
	a.addMember(t, f.admin, "recep@x.com", "reception")
	cashier, recep := login(t, a, "caja@x.com"), login(t, a, "recep@x.com")

	// cita 22:30–23:00 hora local = 06:30 UTC del día siguiente
	f.setHours(t, f.marco, 1, "20:00", "23:00")
	f.mustBook(t, f.marco, at(monday, "22:30"))
	agenda := func(token, day string) resp {
		return a.do(t, "GET", fmt.Sprintf("/api/agenda?branch_id=%d&date=%s", f.branch, day), token, nil)
	}

	r := agenda(f.admin, monday)
	appts := r.Body["appointments"].([]any)
	if r.Status != 200 || len(appts) != 1 {
		t.Fatalf("lunes: %d %v", r.Status, r.Body)
	}
	first := appts[0].(map[string]any)
	if first["client_name"] != "Cliente" || first["service_name"] != "Corte" || first["staff_name"] != "Marco" {
		t.Fatalf("la cita debe traer nombres: %v", first)
	}
	if got := len(agenda(f.admin, tuesday).Body["appointments"].([]any)); got != 0 {
		t.Fatalf("la cita de las 22:30 locales no debe aparecer el martes (aparecen %d)", got)
	}
	// horario de trabajo del día en la respuesta
	var marcoHours []any
	for _, s := range r.Body["staff"].([]any) {
		if s.(map[string]any)["name"] == "Marco" {
			marcoHours = s.(map[string]any)["hours"].([]any)
		}
	}
	if len(marcoHours) != 1 || marcoHours[0].(map[string]any)["start"] != "20:00" {
		t.Fatalf("horarios de Marco: %v", marcoHours)
	}

	// caja puede ver pero no agendar; recepción sí agenda
	if r := agenda(cashier, monday); r.Status != 200 {
		t.Errorf("caja leyendo la agenda: %d", r.Status)
	}
	if r := f.book(t, cashier, f.marco, at(monday, "21:00")); r.Status != 403 {
		t.Errorf("caja agendando: %d", r.Status)
	}
	if r := f.book(t, recep, f.marco, at(monday, "21:00")); r.Status != 201 {
		t.Errorf("recepción agendando: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/blocks", cashier, map[string]any{"user_id": f.marco, "starts_at": at(monday, "20:00"), "ends_at": at(monday, "20:30")}); r.Status != 403 {
		t.Errorf("caja bloqueando: %d", r.Status)
	}
	var n int
	a.DB.QueryRow(context.Background(), `SELECT count(*) FROM appointments`).Scan(&n)
	if n != 2 {
		t.Errorf("citas guardadas: %d", n)
	}
}
