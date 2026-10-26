package api

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Calendario. Los correos de una cita llevan un archivo .ics (se abre en Google Calendar, Outlook y Apple Calendar, y
// Gmail ofrece agregarlo) y un enlace que la agrega a Google Calendar. El evento conserva su UID y su SEQUENCE crece con
// cada cambio, así que una cita movida o cancelada actualiza la que ya está en el calendario.

const icsStamp = "20060102T150405Z"

// icsText escapa un valor para un campo de texto de iCalendar.
func icsText(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`, "\r", "").Replace(s)
}

// icsFold parte una línea a 75 octetos, como pide el formato.
func icsFold(line string) string {
	var b strings.Builder
	n := 0
	for _, r := range line {
		size := len(string(r))
		if n+size > 74 {
			b.WriteString("\r\n ")
			n = 1
		}
		b.WriteRune(r)
		n += size
	}
	return b.String()
}

// icsEvent arma las líneas de un VEVENT. status: CONFIRMED o CANCELLED; manage (opcional) es el enlace de la cita.
func icsEvent(uid string, seq int64, start, end time.Time, summary, where, notes, status, manage, alarm string) []string {
	lines := []string{
		"BEGIN:VEVENT",
		"UID:" + uid,
		"DTSTAMP:" + time.Now().UTC().Format(icsStamp),
		"SEQUENCE:" + fmt.Sprint(max(seq, 0)),
		"DTSTART:" + start.UTC().Format(icsStamp),
		"DTEND:" + end.UTC().Format(icsStamp),
		"SUMMARY:" + icsText(summary),
		"LOCATION:" + icsText(where),
		"DESCRIPTION:" + icsText(notes),
		"STATUS:" + status,
	}
	if manage != "" {
		lines = append(lines, "URL:"+manage)
	}
	if alarm != "" {
		lines = append(lines, "BEGIN:VALARM", "ACTION:DISPLAY", "DESCRIPTION:"+icsText(alarm), "TRIGGER:-PT2H", "END:VALARM")
	}
	return append(lines, "END:VEVENT")
}

// icsCalendar envuelve eventos en un VCALENDAR, ya con las líneas partidas y terminadas en CRLF.
func icsCalendar(method string, header []string, events ...[]string) string {
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Barbella//Citas//ES", "CALSCALE:GREGORIAN", "METHOD:" + method}
	lines = append(lines, header...)
	for _, e := range events {
		lines = append(lines, e...)
	}
	lines = append(lines, "END:VCALENDAR")
	for i, l := range lines {
		lines[i] = icsFold(l)
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

func (m *mailCtx) calendarTitle() string { return m.Service + " · " + m.Business }

func (m *mailCtx) calendarWhere() string {
	if m.Address != "" {
		return m.Branch + ", " + m.Address
	}
	return m.Branch
}

func (m *mailCtx) calendarNotes() string {
	lines := []string{"Servicio: " + m.Service, "Con: " + m.Staff, "Negocio: " + m.Business}
	if m.Phone != "" {
		lines = append(lines, "Teléfono: "+m.Phone)
	}
	return strings.Join(lines, "\n")
}

// calendarEnd es el fin de la cita; si falta (cita sin duración) se supone media hora.
func (m *mailCtx) calendarEnd() time.Time {
	if m.Ends.After(m.Starts) {
		return m.Ends
	}
	return m.Starts.Add(30 * time.Minute)
}

// icsFile es el .ics de la cita para el correo. seq debe crecer con cada correo de la misma cita.
func (m *mailCtx) icsFile(cancel bool, seq int64, manage string) string {
	status, method := "CONFIRMED", "PUBLISH"
	if cancel {
		status, method = "CANCELLED", "CANCEL"
	}
	return icsCalendar(method, nil, icsEvent(fmt.Sprintf("cita-%d@barbella", m.ApptID), seq, m.Starts, m.calendarEnd(),
		m.calendarTitle(), m.calendarWhere(), m.calendarNotes(), status, manage, "Tienes una cita en "+m.Business))
}

// googleCalendarURL abre Google Calendar con la cita lista para guardar.
func (m *mailCtx) googleCalendarURL() string {
	q := url.Values{}
	q.Set("action", "TEMPLATE")
	q.Set("text", m.calendarTitle())
	q.Set("dates", m.Starts.UTC().Format(icsStamp)+"/"+m.calendarEnd().UTC().Format(icsStamp))
	q.Set("details", m.calendarNotes())
	q.Set("location", m.calendarWhere())
	return "https://calendar.google.com/calendar/render?" + q.Encode()
}
