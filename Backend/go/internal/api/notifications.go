package api

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"miestudio/backend-go/internal/mail"
)

// Correos de la reserva: confirmación, cambio de horario, cancelación y recordatorio.
// Todo pasa por una bandeja de salida (tabla notifications): la petición solo escribe una fila y un
// proceso aparte la envía con reintentos, así un fallo del servidor de correo nunca estropea una reserva.
// La confirmación, el cambio y la cancelación son parte del servicio y van al correo que dio el cliente;
// el recordatorio solo se manda si el cliente lo aceptó, y cada uno trae cómo darse de baja.

const (
	notifBatch       = 20
	notifMaxAttempts = 3
	notifClaim       = 10 * time.Minute // si el proceso muere a medio envío, la fila vuelve a intentarse pasado este tiempo
)

// execQueryer es un pool o una transacción con escritura.
type execQueryer interface {
	queryer
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (a *API) mailEnabled() bool { return a.Mailer != nil && a.Mailer.Enabled() }

// notifBackoff es la espera tras el n-ésimo intento fallido: 5 y 20 minutos.
func notifBackoff(attempts int) time.Duration {
	if attempts <= 1 {
		return 5 * time.Minute
	}
	return 20 * time.Minute
}

var (
	weekdaysES = [...]string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}
	monthsES   = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
)

// whenES: «lunes 9 de noviembre a las 10:00» en la zona del negocio.
func whenES(t time.Time, loc *time.Location) string {
	l := t.In(loc)
	return fmt.Sprintf("%s %d de %s a las %02d:%02d", weekdaysES[l.Weekday()], l.Day(), monthsES[l.Month()-1], l.Hour(), l.Minute())
}

type mailCtx struct {
	ApptID, BizID                     int64
	Status                            string
	Business, Branch, Address, Phone  string
	Service, Staff, ClientName, Email string
	Starts                            time.Time
	Loc                               *time.Location
	Token                             *string
	CancelMinHours                    int
	ReminderHours                     int
}

func loadMailCtx(ctx context.Context, q queryer, apptID int64) (*mailCtx, error) {
	m := &mailCtx{ApptID: apptID}
	var tz string
	err := q.QueryRow(ctx,
		`SELECT ap.business_id, ap.status, b.name, b.timezone, br.name, br.address, br.phone, s.name, u.name, c.name, c.email,
		        ap.starts_at, ap.public_token, COALESCE(bs.cancel_min_hours, 2), COALESCE(bs.reminder_hours, 0)
		   FROM appointments ap JOIN businesses b ON b.id = ap.business_id JOIN branches br ON br.id = ap.branch_id
		   JOIN services s ON s.id = ap.service_id JOIN users u ON u.id = ap.staff_id JOIN clients c ON c.id = ap.client_id
		   LEFT JOIN booking_settings bs ON bs.business_id = ap.business_id
		  WHERE ap.id = $1`, apptID).Scan(&m.BizID, &m.Status, &m.Business, &tz, &m.Branch, &m.Address, &m.Phone, &m.Service, &m.Staff,
		&m.ClientName, &m.Email, &m.Starts, &m.Token, &m.CancelMinHours, &m.ReminderHours)
	if err != nil {
		return nil, err
	}
	if m.Loc, err = time.LoadLocation(tz); err != nil {
		m.Loc = time.UTC
	}
	return m, nil
}

func (m *mailCtx) manageURL(appURL string) string {
	if m.Token == nil || appURL == "" {
		return ""
	}
	return appURL + "/cita/" + *m.Token
}

// renderMail arma asunto, texto y HTML. Todo dato del cliente o del negocio se escapa en el HTML.
func renderMail(kind string, moved bool, m *mailCtx, appURL string) (subject, text, htmlBody string) {
	first := m.ClientName
	if f := strings.Fields(first); len(f) > 0 {
		first = f[0]
	}
	when := whenES(m.Starts, m.Loc)
	var lead string
	switch {
	case kind == "cancellation":
		subject = "Cita cancelada · " + m.Business
		lead = fmt.Sprintf("Tu cita de %s del %s fue cancelada.", m.Service, when)
	case kind == "reminder":
		subject = "Recordatorio de tu cita · " + m.Business
		lead = fmt.Sprintf("Te recordamos tu cita de %s el %s.", m.Service, when)
	case moved:
		subject = "Tu cita cambió de horario · " + m.Business
		lead = fmt.Sprintf("Tu cita de %s se movió: ahora es el %s.", m.Service, when)
	default:
		subject = "Cita confirmada · " + m.Business
		lead = fmt.Sprintf("Tu cita de %s quedó agendada para el %s.", m.Service, when)
	}
	type row struct{ k, v string }
	rows := []row{{"Servicio", m.Service}, {"Cuándo", when}, {"Con", m.Staff}, {"Dónde", m.Branch}}
	if m.Address != "" {
		rows[3].v += " · " + m.Address
	}
	if m.Phone != "" {
		rows = append(rows, row{"Teléfono", m.Phone})
	}
	link := m.manageURL(appURL)

	var t strings.Builder
	fmt.Fprintf(&t, "Hola %s,\n\n%s\n\n", first, lead)
	if kind != "cancellation" {
		for _, r := range rows {
			fmt.Fprintf(&t, "%s: %s\n", r.k, r.v)
		}
		if link != "" {
			fmt.Fprintf(&t, "\nPara ver o cancelar tu cita (hasta %d h antes):\n%s\n", m.CancelMinHours, link)
		}
		if kind == "reminder" && link != "" {
			fmt.Fprintf(&t, "\nSi ya no quieres recibir recordatorios: %s?baja=1\n", link)
		}
	}
	fmt.Fprintf(&t, "\n— %s\n", m.Business)

	var h strings.Builder
	esc := html.EscapeString
	fmt.Fprintf(&h, `<div style="font-family:system-ui,-apple-system,Segoe UI,sans-serif;max-width:520px;margin:0 auto;padding:24px;color:#1f2a28">`)
	fmt.Fprintf(&h, `<h2 style="margin:0 0 12px;font-size:20px">%s</h2><p style="margin:0 0 16px;line-height:1.5">Hola %s, %s</p>`, esc(m.Business), esc(first), esc(lowerFirst(lead)))
	if kind != "cancellation" {
		h.WriteString(`<table style="border-collapse:collapse;width:100%;font-size:15px">`)
		for _, r := range rows {
			fmt.Fprintf(&h, `<tr><td style="padding:6px 12px 6px 0;color:#5b6864;white-space:nowrap">%s</td><td style="padding:6px 0">%s</td></tr>`, esc(r.k), esc(r.v))
		}
		h.WriteString(`</table>`)
		if link != "" {
			fmt.Fprintf(&h, `<p style="margin:20px 0 0"><a href="%s" style="display:inline-block;padding:10px 18px;border-radius:8px;background:#2f6f5e;color:#fff;text-decoration:none">Ver o cancelar mi cita</a></p>`, esc(link))
			fmt.Fprintf(&h, `<p style="margin:8px 0 0;font-size:13px;color:#5b6864">Puedes cancelar hasta %d h antes.</p>`, m.CancelMinHours)
		}
		if kind == "reminder" && link != "" {
			fmt.Fprintf(&h, `<p style="margin:20px 0 0;font-size:12px;color:#5b6864"><a href="%s?baja=1" style="color:#5b6864">Dejar de recibir recordatorios</a></p>`, esc(link))
		}
	}
	h.WriteString(`</div>`)
	return subject, t.String(), h.String()
}

// lowerFirst: «Tu cita…» -> «tu cita…» para seguir a «Hola Laura, ».
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] += 'a' - 'A'
	}
	return string(r)
}

// enqueueMail deja un correo en la bandeja. No hace nada si no hay correo configurado o la persona no dio el suyo.
// Un duplicado (misma cita, tipo y hora) se ignora, de modo que repetir la llamada es seguro.
func (a *API) enqueueMail(ctx context.Context, q execQueryer, apptID int64, kind string, moved bool) error {
	if !a.mailEnabled() {
		return nil
	}
	m, err := loadMailCtx(ctx, q, apptID)
	if err != nil {
		return err
	}
	if m.Email == "" {
		return nil
	}
	subject, text, htmlBody := renderMail(kind, moved, m, a.AppURL)
	_, err = q.Exec(ctx,
		`INSERT INTO notifications (business_id, appointment_id, kind, for_starts_at, to_addr, subject, body_text, body_html, send_after)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (appointment_id, kind, for_starts_at) DO NOTHING`,
		m.BizID, apptID, kind, m.Starts, m.Email, subject, text, htmlBody, a.now())
	return err
}

// notify lo usa quien ya terminó su operación: un fallo se registra pero no deshace nada.
func (a *API) notify(ctx context.Context, apptID int64, kind string, moved bool) {
	if err := a.enqueueMail(ctx, a.DB, apptID, kind, moved); err != nil {
		log.Printf("correo: no se pudo encolar (%s, cita %d): %v", kind, apptID, err)
	}
}

// enqueueReminders crea los recordatorios que ya tocan: citas agendadas dentro del plazo del negocio,
// con consentimiento y correo. No se recuerda lo que se reservó ya dentro del plazo (la confirmación cumple esa función).
func (a *API) enqueueReminders(ctx context.Context) error {
	now := a.now()
	rows, err := a.DB.Query(ctx,
		`SELECT ap.id FROM appointments ap
		   JOIN clients c ON c.id = ap.client_id
		   JOIN businesses b ON b.id = ap.business_id
		   JOIN booking_settings bs ON bs.business_id = ap.business_id
		  WHERE ap.status = 'scheduled' AND b.status IN ('trial', 'active') AND bs.reminder_hours > 0
		    AND (ap.reminders_consent OR c.reminders_consent) AND c.email <> ''
		    AND ap.starts_at > $1 AND ap.starts_at <= $1 + bs.reminder_hours * interval '1 hour'
		    AND ap.created_at <= ap.starts_at - bs.reminder_hours * interval '1 hour'
		    AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.appointment_id = ap.id AND n.kind = 'reminder' AND n.for_starts_at = ap.starts_at)
		  ORDER BY ap.starts_at LIMIT 200`, now)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := a.enqueueMail(ctx, a.DB, id, "reminder", false); err != nil {
			return err
		}
	}
	return nil
}

type queued struct {
	ID, ApptID              int64
	Kind, To, Subject, Text string
	HTML                    string
	ForStarts               time.Time
	Attempts                int
}

// claimNotifications toma los correos vencidos y los aparta unos minutos para que nadie más los envíe a la vez.
func (a *API) claimNotifications(ctx context.Context, now time.Time) ([]queued, error) {
	rows, err := a.DB.Query(ctx,
		`UPDATE notifications SET attempts = attempts + 1, send_after = $2
		  WHERE id IN (SELECT id FROM notifications WHERE status = 'pending' AND send_after <= $1 ORDER BY send_after LIMIT $3 FOR UPDATE SKIP LOCKED)
		  RETURNING id, appointment_id, kind, to_addr, subject, body_text, body_html, for_starts_at, attempts`, now, now.Add(notifClaim), notifBatch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []queued
	for rows.Next() {
		var q queued
		if err := rows.Scan(&q.ID, &q.ApptID, &q.Kind, &q.To, &q.Subject, &q.Text, &q.HTML, &q.ForStarts, &q.Attempts); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// stillRelevant: entre encolar y enviar la cita pudo cancelarse, moverse o retirarse el consentimiento.
func (a *API) stillRelevant(ctx context.Context, q queued, now time.Time) (bool, error) {
	var status string
	var starts time.Time
	var consent bool
	err := a.DB.QueryRow(ctx,
		`SELECT ap.status, ap.starts_at, (ap.reminders_consent OR c.reminders_consent)
		   FROM appointments ap JOIN clients c ON c.id = ap.client_id WHERE ap.id = $1`, q.ApptID).Scan(&status, &starts, &consent)
	if err != nil {
		return false, err
	}
	switch q.Kind {
	case "reminder":
		return status == "scheduled" && starts.Equal(q.ForStarts) && starts.After(now) && consent, nil
	case "booking":
		return status != "cancelled" && status != "pending_payment" && starts.After(now.Add(-time.Hour)), nil
	}
	return true, nil
}

// ProcessNotifications libera horarios vencidos, crea los recordatorios que tocan y envía la bandeja.
// Devuelve cuántos correos salieron.
func (a *API) ProcessNotifications(ctx context.Context) (int, error) {
	a.sweepHolds(ctx)
	if !a.mailEnabled() {
		return 0, nil
	}
	if err := a.enqueueReminders(ctx); err != nil {
		return 0, err
	}
	sent := 0
	for round := 0; round < 25; round++ { // acotado: una acumulación grande se atiende en varias vueltas
		now := a.now()
		batch, err := a.claimNotifications(ctx, now)
		if err != nil {
			return sent, err
		}
		for _, q := range batch {
			ok, err := a.stillRelevant(ctx, q, now)
			if err != nil {
				return sent, err
			}
			if !ok {
				a.DB.Exec(ctx, `UPDATE notifications SET status = 'skipped' WHERE id = $1`, q.ID)
				continue
			}
			sctx, cancel := context.WithTimeout(ctx, 45*time.Second)
			err = a.Mailer.Send(sctx, mail.Message{To: q.To, Subject: q.Subject, Text: q.Text, HTML: q.HTML})
			cancel()
			switch {
			case err == nil:
				sent++
				a.DB.Exec(ctx, `UPDATE notifications SET status = 'sent', sent_at = $2, last_error = '' WHERE id = $1`, q.ID, a.now())
			case q.Attempts >= notifMaxAttempts:
				log.Printf("correo: se agotaron los intentos (%s, cita %d): %v", q.Kind, q.ApptID, err)
				a.DB.Exec(ctx, `UPDATE notifications SET status = 'failed', last_error = $2 WHERE id = $1`, q.ID, truncate(err.Error(), 300))
			default:
				a.DB.Exec(ctx, `UPDATE notifications SET send_after = $2, last_error = $3 WHERE id = $1`, q.ID, a.now().Add(notifBackoff(q.Attempts)), truncate(err.Error(), 300))
			}
		}
		if len(batch) < notifBatch {
			break
		}
	}
	return sent, nil
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// RunNotifications procesa la bandeja cada minuto hasta que termine ctx.
func (a *API) RunNotifications(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		if _, err := a.ProcessNotifications(ctx); err != nil && ctx.Err() == nil {
			log.Printf("correo: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
