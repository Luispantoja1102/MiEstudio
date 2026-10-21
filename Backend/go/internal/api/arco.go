package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
	"miestudio/backend-go/internal/mail"
)

// Derechos ARCO (acceso, rectificación, cancelación y oposición). Un cliente los pide desde la página pública del
// negocio; el negocio los atiende en su panel. La ley da 20 días hábiles para responder: cada solicitud lleva su fecha
// límite y el panel avisa de las que se acercan. (Los días festivos no se descuentan.)

var arcoKinds = map[string]string{"access": "Acceso", "rectify": "Rectificación", "cancel": "Cancelación", "oppose": "Oposición"}

// addBusinessDays suma n días hábiles (lunes a viernes) a t.
func addBusinessDays(t time.Time, n int) time.Time {
	for n > 0 {
		t = t.AddDate(0, 0, 1)
		if wd := t.Weekday(); wd != time.Saturday && wd != time.Sunday {
			n--
		}
	}
	return t
}

const arcoDays = 20

func (a *API) publicArco(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "arco-ip:"+clientIP(r), 5, time.Hour) {
		return
	}
	// la página de derechos existe aunque la reserva en línea esté apagada: el negocio puede tener clientes aun así
	var bizID int64
	var bizName, privacyEmail string
	err := a.DB.QueryRow(ctx,
		`SELECT b.id, b.name, COALESCE(bs.privacy_email, '') FROM businesses b LEFT JOIN booking_settings bs ON bs.business_id = b.id
		  WHERE b.slug = $1 AND b.status IN ('trial', 'active')`, r.PathValue("slug")).Scan(&bizID, &bizName, &privacyEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	var req struct {
		Kind    string `json:"kind"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Detail  string `json:"detail"`
		Consent bool   `json:"accept_privacy"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name, req.Email, req.Detail = strings.TrimSpace(req.Name), strings.ToLower(strings.TrimSpace(req.Email)), strings.TrimSpace(req.Detail)
	digits := ""
	if strings.TrimSpace(req.Phone) != "" {
		if digits = normalizePhone(req.Phone); digits == "" {
			writeErr(w, http.StatusBadRequest, "el teléfono no es válido")
			return
		}
	}
	switch {
	case arcoKinds[req.Kind] == "":
		writeErr(w, http.StatusBadRequest, "elige el derecho que quieres ejercer")
	case utf8.RuneCountInString(req.Name) < 2 || utf8.RuneCountInString(req.Name) > 120:
		writeErr(w, http.StatusBadRequest, "escribe tu nombre")
	case req.Email == "" && digits == "":
		writeErr(w, http.StatusBadRequest, "déjanos un correo o un teléfono para responderte")
	case req.Email != "" && !validEmail(req.Email):
		writeErr(w, http.StatusBadRequest, "el correo no es válido")
	case utf8.RuneCountInString(req.Detail) < 5 || utf8.RuneCountInString(req.Detail) > 1500:
		writeErr(w, http.StatusBadRequest, "cuéntanos qué necesitas (de 5 a 1500 caracteres)")
	case !req.Consent:
		writeErr(w, http.StatusBadRequest, "acepta el aviso de privacidad para enviar tu solicitud")
	default:
		raw := make([]byte, 24)
		rand.Read(raw)
		token := base64.RawURLEncoding.EncodeToString(raw)
		now := a.now()
		if _, err := a.DB.Exec(ctx,
			`INSERT INTO arco_requests (business_id, kind, requester, email, phone, detail, public_token, due_at, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, bizID, req.Kind, req.Name, req.Email, digits, req.Detail, token, addBusinessDays(now, arcoDays), now); err != nil {
			serverErr(w)
			return
		}
		a.notifyArco(ctx, bizID, bizName, req.Kind, req.Name, req.Email, token)
		writeJSON(w, http.StatusCreated, map[string]any{"token": token, "status_path": "/arco/estado/" + token, "days": arcoDays})
	}
}

// notifyArco avisa al solicitante (con su enlace de seguimiento) y a los administradores del negocio.
func (a *API) notifyArco(ctx context.Context, bizID int64, bizName, kind, requester, email, token string) {
	if !a.mailEnabled() {
		return
	}
	if email != "" {
		link := a.AppURL + "/arco/estado/" + token
		a.sendBackground(mail.Message{
			To:      email,
			Subject: "Recibimos tu solicitud de privacidad · " + bizName,
			Text:    fmt.Sprintf("Hola %s,\n\n%s recibió tu solicitud de %s. Te responderá en un máximo de %d días hábiles.\n\nPuedes ver su estado aquí:\n%s\n", requester, bizName, strings.ToLower(arcoKinds[kind]), arcoDays, link),
			HTML: fmt.Sprintf(`<p>Hola %s,</p><p>%s recibió tu solicitud de <b>%s</b>. Te responderá en un máximo de %d días hábiles.</p><p><a href="%s">Ver el estado de mi solicitud</a></p>`,
				html.EscapeString(requester), html.EscapeString(bizName), html.EscapeString(strings.ToLower(arcoKinds[kind])), arcoDays, html.EscapeString(link)),
		}, "arco-solicitante")
	}
	rows, err := a.DB.Query(ctx, `SELECT email FROM users WHERE business_id = $1 AND role = 'admin' AND active`, bizID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var to string
		if rows.Scan(&to) == nil {
			a.sendBackground(mail.Message{
				To:      to,
				Subject: "Nueva solicitud de privacidad (" + arcoKinds[kind] + ")",
				Text:    fmt.Sprintf("%s envió una solicitud de %s. Tienes %d días hábiles para responder. Atiéndela en tu panel, sección Privacidad:\n%s/panel/privacidad\n", requester, strings.ToLower(arcoKinds[kind]), arcoDays, a.AppURL),
			}, "arco-negocio")
		}
	}
}

var arcoStatusLabel = map[string]string{"received": "Recibida", "in_progress": "En proceso", "resolved": "Resuelta", "rejected": "Rechazada"}

// publicArcoStatus: quien tiene el enlace ve el estado de su solicitud (sin sus datos).
func (a *API) publicArcoStatus(w http.ResponseWriter, r *http.Request) {
	if !a.allow(w, "pub-token:"+clientIP(r), 60, time.Minute) {
		return
	}
	token := r.PathValue("token")
	if !tokenShape.MatchString(token) {
		notFound(w)
		return
	}
	var kind, status, resolution, biz string
	var due, created time.Time
	var resolved *time.Time
	err := a.DB.QueryRow(r.Context(),
		`SELECT q.kind, q.status, q.resolution, b.name, q.due_at, q.created_at, q.resolved_at
		   FROM arco_requests q JOIN businesses b ON b.id = q.business_id WHERE q.public_token = $1`, token).
		Scan(&kind, &status, &resolution, &biz, &due, &created, &resolved)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"business": biz, "kind": arcoKinds[kind], "status": status, "status_label": arcoStatusLabel[status],
		"resolution": resolution, "created_at": created, "due_at": due, "resolved_at": resolved})
}

// ---------- panel del negocio ----------

type arcoRow struct {
	ID         int64      `json:"id"`
	Kind       string     `json:"kind"`
	KindLabel  string     `json:"kind_label"`
	Requester  string     `json:"requester"`
	Email      string     `json:"email"`
	Phone      string     `json:"phone"`
	Detail     string     `json:"detail"`
	Status     string     `json:"status"`
	Resolution string     `json:"resolution"`
	ClientID   *int64     `json:"client_id"`
	DueAt      time.Time  `json:"due_at"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
	Overdue    bool       `json:"overdue"`
	DaysLeft   int        `json:"days_left"`
}

func (a *API) listArco(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := a.DB.Query(r.Context(),
		`SELECT id, kind, requester, email, phone, detail, status, resolution, client_id, due_at, created_at, resolved_at
		   FROM arco_requests WHERE business_id = $1
		  ORDER BY (status IN ('resolved', 'rejected')), due_at LIMIT 200`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	now := a.now()
	out := []arcoRow{}
	for rows.Next() {
		var x arcoRow
		if err := rows.Scan(&x.ID, &x.Kind, &x.Requester, &x.Email, &x.Phone, &x.Detail, &x.Status, &x.Resolution, &x.ClientID, &x.DueAt, &x.CreatedAt, &x.ResolvedAt); err != nil {
			serverErr(w)
			return
		}
		x.KindLabel = arcoKinds[x.Kind]
		open := x.Status == "received" || x.Status == "in_progress"
		x.Overdue = open && now.After(x.DueAt)
		if open {
			x.DaysLeft = int(x.DueAt.Sub(now).Hours() / 24)
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) updateArco(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Status     *string `json:"status"`
		Resolution *string `json:"resolution"`
		ClientID   *int64  `json:"client_id"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if req.Status != nil && arcoStatusLabel[*req.Status] == "" {
		writeErr(w, http.StatusBadRequest, "estado inválido")
		return
	}
	if req.Resolution != nil && utf8.RuneCountInString(*req.Resolution) > 2000 {
		writeErr(w, http.StatusBadRequest, "la respuesta es demasiado larga")
		return
	}
	if req.Status != nil && (*req.Status == "resolved" || *req.Status == "rejected") && (req.Resolution == nil || utf8.RuneCountInString(strings.TrimSpace(*req.Resolution)) < 3) {
		writeErr(w, http.StatusBadRequest, "explica cómo se resolvió o por qué se rechaza")
		return
	}
	if req.ClientID != nil {
		var okClient bool
		a.DB.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM clients WHERE id = $1 AND business_id = $2)`, *req.ClientID, p.BusinessID).Scan(&okClient)
		if !okClient {
			writeErr(w, http.StatusBadRequest, "cliente inválido")
			return
		}
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE arco_requests SET status = COALESCE($3, status), resolution = COALESCE($4, resolution), client_id = COALESCE($5, client_id),
		   resolved_at = CASE WHEN COALESCE($3, status) IN ('resolved', 'rejected') THEN COALESCE(resolved_at, $6) ELSE NULL END
		 WHERE id = $1 AND business_id = $2`, id, p.BusinessID, req.Status, req.Resolution, req.ClientID, a.now())
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	a.logActivity(r.Context(), p.UserID, p.BusinessID, "arco_updated", map[string]any{"id": id, "status": req.Status})
	w.WriteHeader(http.StatusNoContent)
}

// ---------- exportar y borrar los datos de un cliente ----------

// exportClient arma todo lo que el negocio guarda de un cliente (derecho de acceso), listo para entregarse.
func (a *API) exportClient(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var c struct {
		Name, Phone, Email  string
		Birthday            *time.Time
		Consent, Promos, WA bool
		Created             time.Time
		Anonymized          *time.Time
	}
	err := a.DB.QueryRow(ctx, `SELECT name, phone, email, birthday, reminders_consent, marketing_consent, whatsapp_consent, created_at, anonymized_at FROM clients WHERE id = $1 AND business_id = $2`, id, p.BusinessID).
		Scan(&c.Name, &c.Phone, &c.Email, &c.Birthday, &c.Consent, &c.Promos, &c.WA, &c.Created, &c.Anonymized)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	collect := func(query string, args ...any) []map[string]any {
		rows, err := a.DB.Query(ctx, query, args...)
		if err != nil {
			return nil
		}
		defer rows.Close()
		fields := rows.FieldDescriptions()
		out := []map[string]any{}
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				return out
			}
			m := map[string]any{}
			for i, f := range fields {
				m[f.Name] = vals[i]
			}
			out = append(out, m)
		}
		return out
	}
	data := map[string]any{
		"exported_at": a.now(),
		"client":      map[string]any{"name": c.Name, "phone": c.Phone, "email": c.Email, "birthday": c.Birthday, "accepts_reminders": c.Consent, "accepts_promotions": c.Promos, "accepts_whatsapp": c.WA, "created_at": c.Created},
		"notes":       collect(`SELECT n.body AS note, n.created_at FROM client_notes n WHERE n.client_id = $1 ORDER BY n.created_at`, id),
		"appointments": collect(`SELECT ap.starts_at, ap.status, s.name AS service, u.name AS professional, ap.notes, ap.source, ap.privacy_accepted_at, ap.privacy_version
		   FROM appointments ap JOIN services s ON s.id = ap.service_id JOIN users u ON u.id = ap.staff_id WHERE ap.client_id = $1 ORDER BY ap.starts_at`, id),
		"surveys": collect(`SELECT sv.score, sv.comment, sv.answered_at FROM surveys sv WHERE sv.client_id = $1 AND sv.answered_at IS NOT NULL ORDER BY sv.answered_at`, id),
		"sales": collect(`SELECT s.folio, s.created_at, s.status, s.total_cents, s.tip_cents,
		     (SELECT string_agg(i.name || ' x' || i.qty, ', ') FROM sale_items i WHERE i.sale_id = s.id) AS items
		   FROM sales s WHERE s.client_id = $1 ORDER BY s.created_at`, id),
	}
	a.logActivity(ctx, p.UserID, p.BusinessID, "client_exported", map[string]any{"client_id": id})
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="datos-cliente-%d.json"`, id))
	writeJSON(w, http.StatusOK, data)
}

// eraseClient anonimiza al cliente (derecho de cancelación). Las ventas se conservan, porque el negocio debe guardar
// su contabilidad, pero ya no llevan a una persona. No se hace si tiene citas por venir: primero hay que atenderlas o cancelarlas.
func (a *API) eraseClient(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	var already *time.Time
	if err := tx.QueryRow(ctx, `SELECT anonymized_at FROM clients WHERE id = $1 AND business_id = $2 FOR UPDATE`, id, p.BusinessID).Scan(&already); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(w)
			return
		}
		serverErr(w)
		return
	}
	if already != nil {
		w.WriteHeader(http.StatusNoContent) // ya estaba borrado
		return
	}
	var pending int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM appointments WHERE client_id = $1 AND status IN ('scheduled', 'pending_payment') AND ends_at > $2`, id, a.now()).Scan(&pending); err != nil {
		serverErr(w)
		return
	}
	if pending > 0 {
		writeFail(w, &httpFail{http.StatusConflict, fmt.Sprintf("El cliente tiene %d cita(s) por venir. Atiéndelas o cancélalas primero.", pending), "client_has_appointments"})
		return
	}
	for _, q := range []string{
		`UPDATE clients SET name = 'Cliente eliminado', phone = '', email = '', birthday = NULL, reminders_consent = false, marketing_consent = false, whatsapp_consent = false, unsub_token = NULL, anonymized_at = $2 WHERE id = $1`,
		`DELETE FROM client_notes WHERE client_id = $1`,
		`UPDATE appointments SET notes = '', public_token = NULL, reminders_consent = false, privacy_accepted_at = NULL WHERE client_id = $1`,
		`DELETE FROM notifications WHERE appointment_id IN (SELECT id FROM appointments WHERE client_id = $1)`,
		`DELETE FROM notifications WHERE client_id = $1`,
		`UPDATE surveys SET comment = '' WHERE client_id = $1`,
	} {
		var args []any
		if strings.Contains(q, "$2") {
			args = []any{id, a.now()}
		} else {
			args = []any{id}
		}
		if _, err := tx.Exec(ctx, q, args...); err != nil {
			serverErr(w)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(ctx, p.UserID, p.BusinessID, "client_erased", map[string]any{"client_id": id})
	w.WriteHeader(http.StatusNoContent)
}

// publicPrivacy: lo que muestra la página del aviso de privacidad de un negocio (existe aunque la reserva esté apagada).
func (a *API) publicPrivacy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var id int64
	var name, email, pubTheme string
	err := a.DB.QueryRow(ctx,
		`SELECT b.id, b.name, COALESCE(bs.privacy_email, ''), COALESCE(bs.public_theme, 'auto') FROM businesses b LEFT JOIN booking_settings bs ON bs.business_id = b.id
		  WHERE b.slug = $1 AND b.status IN ('trial', 'active')`, r.PathValue("slug")).Scan(&id, &name, &email, &pubTheme)
	if errors.Is(err, pgx.ErrNoRows) {
		notFound(w)
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	type place struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		Phone   string `json:"phone"`
	}
	places := []place{}
	if rows, err := a.DB.Query(ctx, `SELECT name, address, phone FROM branches WHERE business_id = $1 AND active ORDER BY id`, id); err == nil {
		defer rows.Close()
		for rows.Next() {
			var p place
			if rows.Scan(&p.Name, &p.Address, &p.Phone) == nil {
				places = append(places, p)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"business": name, "privacy_email": email, "branches": places, "version": ClientNoticeVersion, "days": arcoDays,
		"platform": a.Operator.Name, "theme": pubTheme})
}
