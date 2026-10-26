package api

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func sampleMailCtx() *mailCtx {
	loc, _ := time.LoadLocation("America/Tijuana")
	return &mailCtx{
		ApptID: 7, Business: "Barbería; Demo, SA", Branch: "Centro", Address: "Av. Revolución 123", Phone: "6641234567",
		Service: "Corte", Staff: "Marco", Loc: loc,
		Starts: time.Date(2026, 11, 9, 10, 0, 0, 0, loc), Ends: time.Date(2026, 11, 9, 10, 30, 0, 0, loc),
	}
}

func TestICSFileHasStableUIDAndEscapes(t *testing.T) {
	m := sampleMailCtx()
	ics := m.icsFile(false, 1000, "https://app.test/cita/abc")
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n", "METHOD:PUBLISH\r\n", "UID:cita-7@barbella\r\n", "SEQUENCE:1000\r\n",
		"DTSTART:20261109T180000Z\r\n", "DTEND:20261109T183000Z\r\n", // 10:00 de Tijuana (UTC-8) = 18:00 UTC
		`SUMMARY:Corte · Barbería\; Demo\, SA`, "STATUS:CONFIRMED\r\n", "URL:https://app.test/cita/abc\r\n", "END:VCALENDAR\r\n",
	} {
		if !strings.Contains(ics, want) {
			t.Fatalf("falta %q en:\n%s", want, ics)
		}
	}
	// un segundo archivo de la misma cita conserva el UID y sube el SEQUENCE: reemplaza al anterior
	cancel := m.icsFile(true, 2000, "")
	for _, want := range []string{"METHOD:CANCEL\r\n", "UID:cita-7@barbella\r\n", "SEQUENCE:2000\r\n", "STATUS:CANCELLED\r\n"} {
		if !strings.Contains(cancel, want) {
			t.Fatalf("la cancelación necesita %q:\n%s", want, cancel)
		}
	}
	for _, line := range strings.Split(ics, "\r\n") {
		if len(line) > 75 {
			t.Fatalf("línea sin partir (%d): %q", len(line), line)
		}
	}
}

func TestICSWithoutEndAssumesHalfHour(t *testing.T) {
	m := sampleMailCtx()
	m.Ends = time.Time{}
	if !strings.Contains(m.icsFile(false, 1, ""), "DTEND:20261109T183000Z") {
		t.Fatal("sin fin se supone media hora")
	}
}

func TestGoogleCalendarURL(t *testing.T) {
	u := sampleMailCtx().googleCalendarURL()
	for _, want := range []string{"https://calendar.google.com/calendar/render?", "action=TEMPLATE", "dates=20261109T180000Z%2F20261109T183000Z", "text=Corte"} {
		if !strings.Contains(u, want) {
			t.Fatalf("falta %q en %s", want, u)
		}
	}
}

func TestConfirmationMailOffersCalendar(t *testing.T) {
	text, html := func() (string, string) {
		_, t, h := renderMail("booking", false, sampleMailCtx(), "https://app.test")
		return t, h
	}()
	for _, body := range []string{text, html} {
		if !strings.Contains(body, "calendar.google.com") || !strings.Contains(body, "cita.ics") {
			t.Fatalf("el correo debe ofrecer el calendario:\n%s", body)
		}
	}
	_, cancelText, _ := renderMail("cancellation", false, sampleMailCtx(), "https://app.test")
	if strings.Contains(cancelText, "calendar.google.com") {
		t.Fatalf("una cancelación no ofrece agregar al calendario:\n%s", cancelText)
	}
}

func TestMailCarriesICSAttachment(t *testing.T) {
	o, fm := newMailing(t)
	token := o.mustBook(t, at(monday, "10:00"), nil)
	o.process(t)
	m := fm.last()
	if len(m.Attachments) != 1 || m.Attachments[0].Name != "cita.ics" || !strings.Contains(m.Attachments[0].ContentType, "method=PUBLISH") {
		t.Fatalf("adjuntos: %+v", m.Attachments)
	}
	if body := string(m.Attachments[0].Data); !strings.Contains(body, "SUMMARY:Corte") || !strings.Contains(body, "URL:https://app.test/cita/"+token) {
		t.Fatalf("contenido del .ics:\n%s", body)
	}
	// cancelar manda un .ics de cancelación con el mismo UID
	if r := o.a.do(t, "POST", "/api/public-appointments/"+token+"/cancel", "", nil); r.Status != 204 && r.Status != 200 {
		t.Fatalf("cancelar: %d %v", r.Status, r.Body)
	}
	o.process(t)
	last := fm.last()
	if !strings.Contains(last.Subject, "cancelada") || len(last.Attachments) != 1 || !strings.Contains(last.Attachments[0].ContentType, "method=CANCEL") {
		t.Fatalf("correo de cancelación: %q %+v", last.Subject, last.Attachments)
	}
}

func TestCalendarFeedLifecycleAndPrivacy(t *testing.T) {
	f := newFixture(t)
	a := f.a
	marco := login(t, a, "marco@x.com")
	appt := f.mustBook(t, f.marco, at(monday, "10:00"))

	// apagado de origen
	if r := a.do(t, "GET", "/api/me/calendar", marco, nil); r.Body["enabled"] != false {
		t.Fatalf("debe empezar apagado: %v", r.Body)
	}
	r := a.do(t, "POST", "/api/me/calendar", marco, map[string]any{"action": "enable"})
	path, _ := r.Body["path"].(string)
	if r.Status != 200 || r.Body["enabled"] != true || !strings.HasPrefix(path, "/api/calendar/") || !strings.HasSuffix(path, ".ics") {
		t.Fatalf("activar: %d %v", r.Status, r.Body)
	}
	// activar de nuevo conserva la dirección
	if again := a.do(t, "POST", "/api/me/calendar", marco, map[string]any{"action": "enable"}); again.Body["path"] != path {
		t.Fatalf("enable no debe cambiar la dirección: %v", again.Body["path"])
	}

	// el calendario es público (lo lee Google) pero sin nombres de clientes
	feed := a.do(t, "GET", path, "", nil)
	body := string(feed.Raw)
	if feed.Status != 200 || !strings.Contains(body, "UID:agenda-"+strconv.FormatInt(appt, 10)+"@barbella") || !strings.Contains(body, "SUMMARY:Cita · Corte") {
		t.Fatalf("calendario: %d\n%s", feed.Status, body)
	}
	if strings.Contains(body, "Cliente") {
		t.Fatalf("el calendario no debe llevar nombres por omisión:\n%s", body)
	}
	// con nombres encendidos sí
	a.do(t, "POST", "/api/me/calendar", marco, map[string]any{"action": "enable", "show_names": true})
	if body := string(a.do(t, "GET", path, "", nil).Raw); !strings.Contains(body, "Cliente") {
		t.Fatalf("con nombres debía incluir al cliente:\n%s", body)
	}
	// la cita de otra persona no aparece
	f.setHours(t, f.ana, 1, "09:00", "14:00")
	f.mustBook(t, f.ana, at(monday, "10:00"))
	if body := string(a.do(t, "GET", path, "", nil).Raw); strings.Count(body, "BEGIN:VEVENT") != 1 {
		t.Fatalf("solo debe haber la cita de Marco:\n%s", body)
	}

	// cambiar la dirección corta la anterior; apagar también
	rot := a.do(t, "POST", "/api/me/calendar", marco, map[string]any{"action": "rotate"})
	if a.do(t, "GET", path, "", nil).Status != 404 {
		t.Fatal("la dirección anterior debe dejar de servir")
	}
	newPath, _ := rot.Body["path"].(string)
	if a.do(t, "GET", newPath, "", nil).Status != 200 {
		t.Fatal("la nueva dirección debe servir")
	}
	a.do(t, "POST", "/api/me/calendar", marco, map[string]any{"action": "disable"})
	if a.do(t, "GET", newPath, "", nil).Status != 404 {
		t.Fatal("apagado debe dar 404")
	}
	for _, bad := range []string{"/api/calendar/corto.ics", "/api/calendar/" + strings.Repeat("a", 32), "/api/calendar/" + strings.Repeat("a", 32) + ".ics"} {
		if a.do(t, "GET", bad, "", nil).Status != 404 {
			t.Fatalf("%s debe dar 404", bad)
		}
	}
	if r := a.do(t, "POST", "/api/me/calendar", marco, map[string]any{"action": "otra"}); r.Status != 400 {
		t.Fatalf("acción inválida: %d", r.Status)
	}
	if r := a.do(t, "GET", "/api/me/calendar", "", nil); r.Status != 401 {
		t.Fatalf("sin sesión: %d", r.Status)
	}
}
