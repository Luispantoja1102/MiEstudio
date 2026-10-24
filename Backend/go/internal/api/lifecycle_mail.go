package api

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"miestudio/backend-go/internal/mail"
)

// Correos del ciclo de vida de una cuenta: la bienvenida a quien se registra y los avisos al equipo de la plataforma
// (cada alta, cada pago y quien no pagó). Todos salen en segundo plano y solo si el servidor tiene correo.

var kindNames = map[string]string{"barberia": "Barbería", "estetica": "Estética / salón", "spa": "Spa", "unas": "Uñas"}

// ---------- bienvenida ----------

func mailButton(href, label string) string {
	return fmt.Sprintf(`<p><a href="%s" style="display:inline-block;padding:10px 18px;border-radius:8px;background:#b8402f;color:#fff;text-decoration:none">%s</a></p>`,
		html.EscapeString(href), html.EscapeString(label))
}

func mailLayout(title, body string) string {
	return `<div style="font-family:system-ui,sans-serif;max-width:520px;margin:0 auto;padding:24px;color:#1f2a28"><h2 style="margin:0 0 12px">` +
		html.EscapeString(title) + `</h2>` + body + `<p style="font-size:13px;color:#5b6864">— Barbella</p></div>`
}

// welcomeEmail le da la bienvenida a quien acaba de crear su negocio. Si el registro espera aprobación, lo dice.
func (a *API) welcomeEmail(to, name, business string, pending bool) {
	hello := "Hola"
	if first := strings.Fields(name); len(first) > 0 {
		hello += " " + first[0]
	}
	link := a.AppURL + "/login"
	var text, body string
	if pending {
		text = fmt.Sprintf("%s,\n\nTu negocio «%s» ya quedó registrado. Revisamos cada registro: te avisaremos por correo cuando esté aprobado y puedas entrar.\n\n— Barbella\n", hello, business)
		body = "<p>" + html.EscapeString(hello) + ",</p><p>Tu negocio <b>" + html.EscapeString(business) + "</b> ya quedó registrado. Revisamos cada registro: te avisaremos por correo cuando esté aprobado y puedas entrar.</p>"
	} else {
		text = fmt.Sprintf("%s,\n\nTu negocio «%s» ya está creado y tienes %d días de prueba gratis. Entra aquí para configurarlo: %s\n\n— Barbella\n", hello, business, TrialDays, link)
		body = "<p>" + html.EscapeString(hello) + ",</p><p>Tu negocio <b>" + html.EscapeString(business) + "</b> ya está creado y tienes <b>" + fmt.Sprint(TrialDays) +
			" días de prueba gratis</b>. Te ayudamos a configurarlo en unos minutos: giro, horario y equipo.</p>" + mailButton(link, "Entrar a Barbella")
	}
	a.sendBackground(mail.Message{To: to, Subject: "Bienvenido a Barbella", Text: text, HTML: mailLayout("Bienvenido a Barbella", body)}, "bienvenida")
}

// ---------- avisos al equipo de la plataforma ----------

// sendTeam manda a quien atiende la plataforma (PLATFORM_NOTICE_EMAIL o los administradores) una tabla de datos con el enlace al negocio.
// replyTo, si se da, hace que al responder el correo la respuesta llegue a la persona.
func (a *API) sendTeam(ctx context.Context, subject string, lines [][2]string, footer string, bizID int64, replyTo string) {
	if !a.mailEnabled() {
		return
	}
	link := a.AppURL + "/plataforma/negocios/" + itoa(bizID)
	text, rows := "", ""
	for _, l := range lines {
		if l[1] == "" {
			continue
		}
		text += l[0] + ": " + l[1] + "\n"
		rows += fmt.Sprintf(`<tr><td style="padding:4px 12px 4px 0;color:#5d6e7d">%s</td><td style="padding:4px 0">%s</td></tr>`, html.EscapeString(l[0]), html.EscapeString(l[1]))
	}
	text += "\n" + footer + "\n" + link + "\n"
	body := `<table style="font:14px system-ui,sans-serif">` + rows + `</table><p>` + html.EscapeString(footer) + ` <a href="` + html.EscapeString(link) + `">Ver el negocio en la plataforma</a>.</p>`
	for _, to := range a.teamRecipients(ctx) {
		a.sendBackground(mail.Message{To: to, Subject: subject, Text: text, HTML: body, ReplyTo: replyTo}, "aviso al equipo")
	}
}

// signupNotice avisa al equipo de que alguien creó su negocio.
func (a *API) signupNotice(ctx context.Context, req registerReq, bizID int64, status string) {
	state := fmt.Sprintf("En prueba de %d días", TrialDays)
	if status == "pending" {
		state = "Espera tu aprobación"
	}
	kind := kindNames[req.Kind]
	if kind == "" {
		kind = req.Kind
	}
	a.sendTeam(ctx, "Nuevo registro · "+req.BusinessName,
		[][2]string{{"Negocio", req.BusinessName}, {"Giro", kind}, {"Nombre", req.Name}, {"Correo", req.Email}, {"Teléfono", req.Phone}, {"Usuario", req.Username}, {"Estado", state}},
		"Nuevo registro desde el sitio.", bizID, req.Email)
}

// ---------- historia de pagos ----------

// paidHistory es lo que un negocio había pagado: cuántos pagos, cuántos meses y desde cuándo.
type paidHistory struct {
	Payments int
	Months   int
	Since    *time.Time
}

// paidBefore cuenta los pagos de un negocio (sin cortesías), sin contar el pago excludeID.
func (a *API) paidBefore(ctx context.Context, bizID, excludeID int64) paidHistory {
	var h paidHistory
	a.DB.QueryRow(ctx, `SELECT count(*), COALESCE(sum(months), 0), min(created_at) FROM payments WHERE business_id = $1 AND amount_cents > 0 AND method <> 'courtesy' AND id <> $2`,
		bizID, excludeID).Scan(&h.Payments, &h.Months, &h.Since)
	return h
}

// tenureLabel dice cuánto lleva alguien como cliente, en años y meses; vacío antes del primer mes completo.
func tenureLabel(since, now time.Time) string {
	m := (now.Year()-since.Year())*12 + int(now.Month()) - int(since.Month())
	if now.Day() < since.Day() {
		m--
	}
	if m < 1 {
		return ""
	}
	var parts []string
	if y := m / 12; y > 0 {
		parts = append(parts, plural(y, "año", "años"))
	}
	if mo := m % 12; mo > 0 {
		parts = append(parts, plural(mo, "mes", "meses"))
	}
	return strings.Join(parts, " y ")
}

func tenureLine(h paidHistory, now time.Time) string {
	if h.Since == nil {
		return "Nunca ha pagado"
	}
	if l := tenureLabel(*h.Since, now); l != "" {
		return "Lleva " + l + " con nosotros"
	}
	return "Cliente nuevo (menos de 1 mes)"
}

// teamPaymentNotice avisa al equipo de un pago: el primero del negocio, una renovación mensual o un pago anual.
func (a *API) teamPaymentNotice(ctx context.Context, bizID int64, p paymentRow, planName string) {
	var name, owner string
	a.DB.QueryRow(ctx, `SELECT b.name, COALESCE((SELECT u.email FROM users u WHERE u.business_id = b.id AND u.role = 'admin' ORDER BY u.id LIMIT 1), '') FROM businesses b WHERE b.id = $1`, bizID).Scan(&name, &owner)
	before := a.paidBefore(ctx, bizID, p.ID)
	kind := "Pago mensual"
	switch {
	case before.Payments == 0:
		kind = "Primer pago"
	case p.Months >= 12:
		kind = "Pago anual"
	}
	h, now := before, a.now()
	if before.Payments == 0 {
		h.Since = &now // el primer pago empieza la cuenta ahora
	}
	a.sendTeam(ctx, kind+" · "+name+" · "+planName,
		[][2]string{
			{"Negocio", name}, {"Correo", owner}, {"Plan que paga", planName}, {"Monto", moneyES(p.AmountCents)}, {"Meses que pagó", itoa(int64(p.Months))},
			{"Meses pagados en total", itoa(int64(before.Months + p.Months))}, {"Pagado hasta", p.PeriodEnd.Format("2006-01-02")}, {"Antigüedad", tenureLine(h, now)},
		}, "Pago recibido.", bizID, "")
}

// missedPaymentNotice avisa al equipo de un pago que no llegó, una sola vez por evento (kind + key).
func (a *API) missedPaymentNotice(ctx context.Context, b bizRow, kind, key, reason string) {
	if !a.claimNotice(ctx, b.ID, kind, key) {
		return
	}
	h, now := a.paidBefore(ctx, b.ID, 0), a.now()
	a.sendTeam(ctx, "No pagó · "+b.Name+" · "+b.PlanName,
		[][2]string{
			{"Negocio", b.Name}, {"Correo", b.OwnerEmail}, {"Plan", b.PlanName}, {"Qué pasó", reason},
			{"Meses pagados en total", itoa(int64(h.Months))}, {"Antigüedad", tenureLine(h, now)},
		}, "No llegó el pago.", b.ID, "")
}

// missedPaymentPass busca, entre los negocios de la lista, los que no pagaron en los últimos días: una prueba que terminó sin pagar
// y un periodo pagado que terminó (con sus días de gracia) sin renovar. Solo mira los últimos 3 días, para que la primera vez
// no avise de toda la historia.
func (a *API) missedPaymentPass(ctx context.Context, list []bizRow, now time.Time) {
	recent := 3 * 24 * time.Hour
	for _, b := range list {
		switch b.BillingState {
		case "trial_expired":
			if b.TrialEndsAt != nil && now.Sub(*b.TrialEndsAt) < recent {
				a.missedPaymentNotice(ctx, b, "team_trial_ended", b.TrialEndsAt.Format("2006-01-02"), "Terminó su prueba el "+b.TrialEndsAt.Format("2006-01-02")+" sin pagar")
			}
		case "past_due":
			if b.CurrentPeriodEnd != nil && now.Sub(b.CurrentPeriodEnd.Add(pastDueGraceDays*24*time.Hour)) < recent {
				a.missedPaymentNotice(ctx, b, "team_period_ended", b.CurrentPeriodEnd.Format("2006-01-02"),
					"Terminó su periodo pagado el "+b.CurrentPeriodEnd.Format("2006-01-02")+" y no hubo un nuevo pago (ya pasaron los "+itoa(pastDueGraceDays)+" días de gracia)")
			}
		}
	}
}
