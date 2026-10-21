package api

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

// Mensajes a clientes: cumpleaños, reactivación y encuesta de satisfacción.
//
// Las promociones (cumpleaños y reactivación) solo van a quien las aceptó (clients.marketing_consent); la encuesta, a quien aceptó
// recordatorios o promociones. Cada correo trae un enlace para darse de baja de todo lo opcional. Todo sale por la bandeja de
// correos con una clave de duplicado, así que revisar cada minuto nunca manda dos veces lo mismo.
// La encuesta invita a dejar una reseña en Google a todas las personas por igual: filtrar a quién se le pide (solo a quien
// calificó bien) va contra las reglas de Google y desvirtúa las opiniones.

const (
	engagementFromHour = 9 // no se manda nada antes de esta hora local del negocio
	surveyWindow       = 7 * 24 * time.Hour
	surveyPerClient    = 7 * 24 * time.Hour // una encuesta por cliente por semana
	winbackMaxAge      = 365 * 24 * time.Hour
)

type engagementSettings struct {
	BirthdayEnabled  bool   `json:"birthday_enabled"`
	BirthdayMessage  string `json:"birthday_message"`
	WinbackEnabled   bool   `json:"winback_enabled"`
	WinbackDays      int    `json:"winback_days"`
	WinbackMessage   string `json:"winback_message"`
	SurveyEnabled    bool   `json:"survey_enabled"`
	SurveyDelayHours int    `json:"survey_delay_hours"`
	ReviewURL        string `json:"review_url"`
}

func loadEngagement(ctx context.Context, q queryer, bizID int64) (engagementSettings, error) {
	s := engagementSettings{WinbackDays: 60, SurveyDelayHours: 24}
	err := q.QueryRow(ctx,
		`SELECT birthday_enabled, birthday_message, winback_enabled, winback_days, winback_message, survey_enabled, survey_delay_hours, review_url
		   FROM engagement_settings WHERE business_id = $1`, bizID).
		Scan(&s.BirthdayEnabled, &s.BirthdayMessage, &s.WinbackEnabled, &s.WinbackDays, &s.WinbackMessage, &s.SurveyEnabled, &s.SurveyDelayHours, &s.ReviewURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, nil
	}
	return s, err
}

func (a *API) getEngagement(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	s, err := loadEngagement(r.Context(), a.DB, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	var promos, askable int
	a.DB.QueryRow(r.Context(), `SELECT count(*) FILTER (WHERE marketing_consent), count(*) FILTER (WHERE marketing_consent OR reminders_consent)
	   FROM clients WHERE business_id = $1 AND email <> '' AND anonymized_at IS NULL`, p.BusinessID).Scan(&promos, &askable)
	writeJSON(w, http.StatusOK, map[string]any{"settings": s, "mail_enabled": a.mailEnabled(), "promo_clients": promos, "survey_clients": askable})
}

func (a *API) putEngagement(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var s engagementSettings
	if !decode(w, r, &s) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	s.BirthdayMessage, s.WinbackMessage = strings.TrimSpace(s.BirthdayMessage), strings.TrimSpace(s.WinbackMessage)
	s.ReviewURL = strings.TrimSpace(s.ReviewURL)
	switch {
	case utf8.RuneCountInString(s.BirthdayMessage) > 500 || utf8.RuneCountInString(s.WinbackMessage) > 500:
		writeErr(w, http.StatusBadRequest, "los mensajes pueden tener hasta 500 caracteres")
		return
	case s.WinbackDays < 14 || s.WinbackDays > 365:
		writeErr(w, http.StatusBadRequest, "los días sin venir deben estar entre 14 y 365")
		return
	case s.SurveyDelayHours < 1 || s.SurveyDelayHours > 168:
		writeErr(w, http.StatusBadRequest, "la encuesta se envía entre 1 y 168 horas después de la cita")
		return
	}
	if s.ReviewURL != "" {
		u, err := url.Parse(s.ReviewURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || len(s.ReviewURL) > 400 {
			writeErr(w, http.StatusBadRequest, "el enlace de reseñas debe ser una dirección https válida")
			return
		}
	}
	if _, err := a.DB.Exec(r.Context(),
		`INSERT INTO engagement_settings (business_id, birthday_enabled, birthday_message, winback_enabled, winback_days, winback_message, survey_enabled, survey_delay_hours, review_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (business_id) DO UPDATE SET birthday_enabled = $2, birthday_message = $3, winback_enabled = $4, winback_days = $5,
		   winback_message = $6, survey_enabled = $7, survey_delay_hours = $8, review_url = $9`,
		p.BusinessID, s.BirthdayEnabled, s.BirthdayMessage, s.WinbackEnabled, s.WinbackDays, s.WinbackMessage, s.SurveyEnabled, s.SurveyDelayHours, s.ReviewURL); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listSurveys: promedio y últimas opiniones contestadas.
func (a *API) listSurveys(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	var sent, answered int
	var avg *float64
	if err := a.DB.QueryRow(ctx, `SELECT count(*), count(answered_at), avg(score)::float8 FROM surveys WHERE business_id = $1`, p.BusinessID).Scan(&sent, &answered, &avg); err != nil {
		serverErr(w)
		return
	}
	rows, err := a.DB.Query(ctx,
		`SELECT sv.score, sv.comment, sv.answered_at, c.name, s.name FROM surveys sv
		   JOIN clients c ON c.id = sv.client_id JOIN appointments ap ON ap.id = sv.appointment_id JOIN services s ON s.id = ap.service_id
		  WHERE sv.business_id = $1 AND sv.answered_at IS NOT NULL ORDER BY sv.answered_at DESC LIMIT 50`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	type item struct {
		Score      int       `json:"score"`
		Comment    string    `json:"comment"`
		AnsweredAt time.Time `json:"answered_at"`
		Client     string    `json:"client"`
		Service    string    `json:"service"`
	}
	items := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.Score, &it.Comment, &it.AnsweredAt, &it.Client, &it.Service); err != nil {
			serverErr(w)
			return
		}
		items = append(items, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": sent, "answered": answered, "average": avg, "items": items})
}

// ---------- páginas públicas: encuesta y baja ----------

func (a *API) publicSurveyInfo(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !tokenShape.MatchString(token) {
		notFound(w)
		return
	}
	var out struct {
		Business  string `json:"business"`
		Service   string `json:"service"`
		Answered  bool   `json:"answered"`
		ReviewURL string `json:"review_url"`
	}
	var bizID int64
	err := a.DB.QueryRow(r.Context(),
		`SELECT b.id, b.name, s.name, sv.answered_at IS NOT NULL FROM surveys sv JOIN businesses b ON b.id = sv.business_id
		   JOIN appointments ap ON ap.id = sv.appointment_id JOIN services s ON s.id = ap.service_id WHERE sv.token = $1`, token).
		Scan(&bizID, &out.Business, &out.Service, &out.Answered)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	es, err := loadEngagement(r.Context(), a.DB, bizID)
	if err != nil {
		serverErr(w)
		return
	}
	out.ReviewURL = es.ReviewURL
	writeJSON(w, http.StatusOK, out)
}

func (a *API) publicSurveyAnswer(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !tokenShape.MatchString(token) {
		notFound(w)
		return
	}
	if !a.allow(w, "pub-survey:"+clientIP(r), 30, time.Hour) {
		return
	}
	var req struct {
		Score   int    `json:"score"`
		Comment string `json:"comment"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Comment = strings.TrimSpace(req.Comment)
	if req.Score < 1 || req.Score > 5 || utf8.RuneCountInString(req.Comment) > 1000 {
		writeErr(w, http.StatusBadRequest, "elige una calificación del 1 al 5 (el comentario, hasta 1000 caracteres)")
		return
	}
	var bizID int64
	err := a.DB.QueryRow(r.Context(),
		`UPDATE surveys SET score = $2, comment = $3, answered_at = $4 WHERE token = $1 AND answered_at IS NULL RETURNING business_id`,
		token, req.Score, req.Comment, a.now()).Scan(&bizID)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		a.DB.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM surveys WHERE token = $1)`, token).Scan(&exists)
		if !exists {
			notFound(w)
			return
		}
		writeErr(w, http.StatusConflict, "ya contestaste esta encuesta, ¡gracias!")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	es, _ := loadEngagement(r.Context(), a.DB, bizID)
	writeJSON(w, http.StatusOK, map[string]any{"review_url": es.ReviewURL})
}

func (a *API) publicUnsubInfo(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !tokenShape.MatchString(token) {
		notFound(w)
		return
	}
	var business string
	var subscribed bool
	err := a.DB.QueryRow(r.Context(),
		`SELECT b.name, (c.marketing_consent OR c.reminders_consent OR c.whatsapp_consent) FROM clients c JOIN businesses b ON b.id = c.business_id WHERE c.unsub_token = $1`, token).Scan(&business, &subscribed)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"business": business, "subscribed": subscribed})
}

// publicUnsub retira el consentimiento de todo lo opcional. Es idempotente.
func (a *API) publicUnsub(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !tokenShape.MatchString(token) {
		notFound(w)
		return
	}
	if !a.allow(w, "pub-unsub:"+clientIP(r), 30, time.Hour) {
		return
	}
	tag, err := a.DB.Exec(r.Context(), `UPDATE clients SET marketing_consent = false, reminders_consent = false, whatsapp_consent = false WHERE unsub_token = $1`, token)
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		notFound(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- envío ----------

// unsubToken devuelve (y crea si falta) el enlace de baja del cliente.
func unsubToken(ctx context.Context, q execQueryer, clientID int64) (string, error) {
	var t string
	err := q.QueryRow(ctx, `UPDATE clients SET unsub_token = COALESCE(unsub_token, $2) WHERE id = $1 RETURNING unsub_token`, clientID, newPublicToken()).Scan(&t)
	return t, err
}

type engRow struct {
	ClientID, BizID             int64
	Name, Email, Business, Slug string
	Message, Extra, Key         string
	Online                      bool
	ApptID                      int64
	ServiceName                 string
}

func firstName(n string) string {
	if f := strings.Fields(n); len(f) > 0 {
		return f[0]
	}
	return n
}

// paragraphs pasa un texto libre a HTML escapado, respetando los saltos de línea.
func paragraphs(s string) string {
	parts := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, p := range parts {
		parts[i] = html.EscapeString(p)
	}
	return strings.Join(parts, "<br>")
}

type engMail struct {
	Subject, Lead, Body string // Body: texto libre del negocio (opcional)
	CTA, CTALink        string
	Extra               string // HTML ya armado, p. ej. los botones de la encuesta
	ExtraText           string
}

// sendEngagement deja el correo en la bandeja. pre (opcional) corre en la misma transacción y puede vetar el envío.
func (a *API) sendEngagement(ctx context.Context, row engRow, kind string, m engMail, now time.Time, pre func(pgx.Tx) (bool, error)) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if pre != nil {
		if ok, err := pre(tx); err != nil || !ok {
			return err
		}
	}
	tok, err := unsubToken(ctx, tx, row.ClientID)
	if err != nil {
		return err
	}
	unsub := ""
	if a.AppURL != "" {
		unsub = a.AppURL + "/baja/" + tok
	}
	esc := html.EscapeString
	var t strings.Builder
	fmt.Fprintf(&t, "Hola %s,\n\n%s\n", firstName(row.Name), m.Lead)
	if m.Body != "" {
		fmt.Fprintf(&t, "\n%s\n", m.Body)
	}
	if m.ExtraText != "" {
		fmt.Fprintf(&t, "\n%s\n", m.ExtraText)
	}
	if m.CTALink != "" {
		fmt.Fprintf(&t, "\n%s: %s\n", m.CTA, m.CTALink)
	}
	if unsub != "" {
		fmt.Fprintf(&t, "\nSi ya no quieres recibir estos mensajes: %s\n", unsub)
	}
	fmt.Fprintf(&t, "\n— %s\n", row.Business)

	var h strings.Builder
	h.WriteString(`<div style="font-family:system-ui,-apple-system,Segoe UI,sans-serif;max-width:520px;margin:0 auto;padding:24px;color:#1f2a28">`)
	fmt.Fprintf(&h, `<h2 style="margin:0 0 12px;font-size:20px">%s</h2><p style="margin:0 0 16px;line-height:1.5">Hola %s, %s</p>`, esc(row.Business), esc(firstName(row.Name)), esc(lowerFirst(m.Lead)))
	if m.Body != "" {
		fmt.Fprintf(&h, `<p style="margin:0 0 16px;line-height:1.5">%s</p>`, paragraphs(m.Body))
	}
	h.WriteString(m.Extra)
	if m.CTALink != "" {
		fmt.Fprintf(&h, `<p style="margin:20px 0 0"><a href="%s" style="display:inline-block;padding:10px 18px;border-radius:8px;background:#1c2f5c;color:#fff;text-decoration:none">%s</a></p>`, esc(m.CTALink), esc(m.CTA))
	}
	if unsub != "" {
		fmt.Fprintf(&h, `<p style="margin:24px 0 0;font-size:12px;color:#5b6864"><a href="%s" style="color:#5b6864">Dejar de recibir estos mensajes</a></p>`, esc(unsub))
	}
	h.WriteString(`</div>`)

	tag, err := tx.Exec(ctx,
		`INSERT INTO notifications (business_id, client_id, kind, to_addr, subject, body_text, body_html, send_after, dedupe_key)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT DO NOTHING`,
		row.BizID, row.ClientID, kind, row.Email, m.Subject, t.String(), h.String(), now, row.Key)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	return tx.Commit(ctx)
}

// engagementPass crea los cumpleaños, reactivaciones y encuestas que ya tocan. Requiere correo configurado.
func (a *API) engagementPass(ctx context.Context) error {
	if !a.mailEnabled() {
		return nil
	}
	now := a.now()
	if err := a.birthdayPass(ctx, now); err != nil {
		return err
	}
	if err := a.winbackPass(ctx, now); err != nil {
		return err
	}
	return a.surveyPass(ctx, now)
}

func (a *API) engRows(ctx context.Context, query string, args ...any) ([]engRow, error) {
	rows, err := a.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []engRow
	for rows.Next() {
		var r engRow
		if err := rows.Scan(&r.ClientID, &r.BizID, &r.Name, &r.Email, &r.Business, &r.Slug, &r.Message, &r.Extra, &r.Key, &r.Online, &r.ApptID, &r.ServiceName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *API) bookURL(row engRow) string {
	if !row.Online || a.AppURL == "" {
		return ""
	}
	return a.AppURL + "/reservar/" + row.Slug
}

func (a *API) birthdayPass(ctx context.Context, now time.Time) error {
	rows, err := a.engRows(ctx,
		`SELECT c.id, b.id, c.name, c.email, b.name, b.slug, es.birthday_message, '',
		        'birthday:' || c.id || ':' || to_char(($1::timestamptz AT TIME ZONE b.timezone), 'YYYY'),
		        COALESCE(bs.enabled, false), 0, ''
		   FROM clients c JOIN businesses b ON b.id = c.business_id JOIN engagement_settings es ON es.business_id = b.id
		   LEFT JOIN booking_settings bs ON bs.business_id = b.id
		  WHERE es.birthday_enabled AND `+usableSQL("b.", "$1")+` AND c.marketing_consent AND c.email <> '' AND c.anonymized_at IS NULL
		    AND c.birthday IS NOT NULL AND to_char(c.birthday, 'MM-DD') = to_char(($1::timestamptz AT TIME ZONE b.timezone), 'MM-DD')
		    AND extract(hour FROM ($1::timestamptz AT TIME ZONE b.timezone)) >= $2
		    AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.business_id = b.id
		                      AND n.dedupe_key = 'birthday:' || c.id || ':' || to_char(($1::timestamptz AT TIME ZONE b.timezone), 'YYYY'))
		  ORDER BY c.id LIMIT 200`, now, engagementFromHour)
	if err != nil {
		return err
	}
	for _, r := range rows {
		m := engMail{
			Subject: "¡Feliz cumpleaños! · " + r.Business,
			Lead:    fmt.Sprintf("En %s queremos desearte un feliz cumpleaños.", r.Business),
			Body:    r.Message,
			CTA:     "Reservar mi cita", CTALink: a.bookURL(r),
		}
		if err := a.sendEngagement(ctx, r, "birthday", m, now, nil); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) winbackPass(ctx context.Context, now time.Time) error {
	rows, err := a.engRows(ctx,
		`SELECT c.id, b.id, c.name, c.email, b.name, b.slug, es.winback_message, '',
		        'winback:' || c.id || ':' || to_char(lv.visit AT TIME ZONE 'UTC', 'YYYY-MM-DD'),
		        COALESCE(bs.enabled, false), 0, ''
		   FROM clients c JOIN businesses b ON b.id = c.business_id JOIN engagement_settings es ON es.business_id = b.id
		   LEFT JOIN booking_settings bs ON bs.business_id = b.id
		   CROSS JOIN LATERAL (SELECT max(t) AS visit FROM (
		          SELECT ap.starts_at AS t FROM appointments ap WHERE ap.client_id = c.id AND ap.status = 'completed'
		          UNION ALL SELECT s.created_at FROM sales s WHERE s.client_id = c.id AND s.status = 'paid') x) lv
		  WHERE es.winback_enabled AND `+usableSQL("b.", "$1")+` AND c.marketing_consent AND c.email <> '' AND c.anonymized_at IS NULL
		    AND lv.visit IS NOT NULL AND lv.visit <= $1::timestamptz - es.winback_days * interval '1 day'
		    AND lv.visit > $1::timestamptz - $3::interval
		    AND extract(hour FROM ($1::timestamptz AT TIME ZONE b.timezone)) >= $2
		    AND NOT EXISTS (SELECT 1 FROM appointments ap WHERE ap.client_id = c.id AND ap.status IN ('scheduled', 'pending_payment') AND ap.starts_at > $1::timestamptz)
		    AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.business_id = b.id
		                      AND n.dedupe_key = 'winback:' || c.id || ':' || to_char(lv.visit AT TIME ZONE 'UTC', 'YYYY-MM-DD'))
		  ORDER BY c.id LIMIT 200`, now, engagementFromHour, fmt.Sprintf("%d hours", int(winbackMaxAge.Hours())))
	if err != nil {
		return err
	}
	for _, r := range rows {
		m := engMail{
			Subject: "Te extrañamos · " + r.Business,
			Lead:    fmt.Sprintf("Hace tiempo que no te vemos en %s y nos encantaría atenderte de nuevo.", r.Business),
			Body:    r.Message,
			CTA:     "Reservar mi cita", CTALink: a.bookURL(r),
		}
		if err := a.sendEngagement(ctx, r, "winback", m, now, nil); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) surveyPass(ctx context.Context, now time.Time) error {
	rows, err := a.engRows(ctx,
		`SELECT c.id, b.id, c.name, c.email, b.name, b.slug, '', '', 'survey:' || ap.id, false, ap.id, s.name
		   FROM appointments ap JOIN clients c ON c.id = ap.client_id JOIN businesses b ON b.id = ap.business_id
		   JOIN services s ON s.id = ap.service_id JOIN engagement_settings es ON es.business_id = b.id
		  WHERE es.survey_enabled AND `+usableSQL("b.", "$1")+` AND ap.status = 'completed'
		    AND ap.ends_at + es.survey_delay_hours * interval '1 hour' <= $1::timestamptz AND ap.ends_at > $1::timestamptz - $3::interval
		    AND (c.reminders_consent OR c.marketing_consent) AND c.email <> '' AND c.anonymized_at IS NULL
		    AND extract(hour FROM ($1::timestamptz AT TIME ZONE b.timezone)) >= $2
		    AND NOT EXISTS (SELECT 1 FROM surveys sv WHERE sv.appointment_id = ap.id)
		    AND NOT EXISTS (SELECT 1 FROM surveys sv WHERE sv.client_id = c.id AND sv.created_at > $1::timestamptz - $4::interval)
		  ORDER BY ap.ends_at LIMIT 200`, now, engagementFromHour, fmt.Sprintf("%d hours", int(surveyWindow.Hours())), fmt.Sprintf("%d hours", int(surveyPerClient.Hours())))
	if err != nil {
		return err
	}
	for _, r := range rows {
		token := newPublicToken()
		link := ""
		if a.AppURL != "" {
			link = a.AppURL + "/encuesta/" + token
		}
		var extra strings.Builder
		extra.WriteString(`<p style="margin:0 0 8px">`)
		for i := 1; i <= 5; i++ {
			if link != "" {
				fmt.Fprintf(&extra, `<a href="%s?s=%d" style="display:inline-block;width:40px;height:40px;line-height:40px;margin:0 4px 4px 0;text-align:center;border-radius:8px;border:1px solid #1c2f5c;color:#1c2f5c;text-decoration:none;font-weight:600">%d</a>`, html.EscapeString(link), i, i)
			}
		}
		extra.WriteString(`</p><p style="margin:0;font-size:13px;color:#5b6864">1 = muy mal · 5 = excelente</p>`)
		m := engMail{
			Subject: "¿Cómo te fue en " + r.Business + "?",
			Lead:    fmt.Sprintf("Gracias por visitarnos. ¿Cómo calificarías tu cita de %s?", r.ServiceName),
			Extra:   extra.String(),
			CTA:     "Contestar la encuesta", CTALink: link,
			ExtraText: "Responde del 1 (muy mal) al 5 (excelente) en el enlace; toma menos de un minuto.",
		}
		if link == "" {
			m.Extra = ""
		}
		// la encuesta y su correo nacen juntos; una persona con dos citas elegibles recibe solo una por semana
		create := func(tx pgx.Tx) (bool, error) {
			tag, err := tx.Exec(ctx,
				`INSERT INTO surveys (business_id, appointment_id, client_id, token, created_at)
				 SELECT $1::bigint, $2::bigint, $3::bigint, $4::text, $5::timestamptz
				  WHERE NOT EXISTS (SELECT 1 FROM surveys WHERE client_id = $3::bigint AND created_at > $5::timestamptz - $6::text::interval)`,
				r.BizID, r.ApptID, r.ClientID, token, now, fmt.Sprintf("%d hours", int(surveyPerClient.Hours())))
			return tag.RowsAffected() == 1, err
		}
		if err := a.sendEngagement(ctx, r, "survey", m, now, create); err != nil {
			return err
		}
	}
	return nil
}
