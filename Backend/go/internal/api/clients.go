package api

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

type client struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Phone     string     `json:"phone"`
	Email     string     `json:"email"`
	Reminders bool       `json:"reminders_consent"`
	Promos    bool       `json:"marketing_consent"`
	WhatsApp  bool       `json:"whatsapp_consent"`
	Birthday  *time.Time `json:"-"`
	BirthdayS *string    `json:"birthday"`
	CreatedAt time.Time  `json:"created_at"`
}

const dateLayout = "2006-01-02"

func (c *client) finish() {
	if c.Birthday != nil {
		s := c.Birthday.Format(dateLayout)
		c.BirthdayS = &s
	}
}

func validContact(name, phone, email string) string {
	switch {
	case !validName(name):
		return "el nombre es obligatorio"
	case len(phone) > 30:
		return "teléfono inválido"
	case email != "" && !validEmail(email):
		return "correo inválido"
	}
	return ""
}

// parseBirthday acepta "" (sin fecha) o YYYY-MM-DD no futura.
func parseBirthday(s string) (*time.Time, bool) {
	if s == "" {
		return nil, true
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil || t.After(time.Now()) || t.Year() < 1900 {
		return nil, false
	}
	return &t, true
}

const clientCols = `id, name, phone, email, reminders_consent, marketing_consent, whatsapp_consent, birthday, created_at`

func scanClient(row pgx.Row) (client, error) {
	var c client
	err := row.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Reminders, &c.Promos, &c.WhatsApp, &c.Birthday, &c.CreatedAt)
	c.finish()
	return c, err
}

func (a *API) listClients(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	rows, err := a.DB.Query(r.Context(),
		`SELECT `+clientCols+` FROM clients
		 WHERE business_id = $1 AND ($2 = '' OR strpos(lower(name), lower($2)) > 0
		   OR strpos(phone, $2) > 0 OR strpos(lower(email), lower($2)) > 0)
		 ORDER BY name LIMIT 100`, p.BusinessID, q)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []client{}
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			serverErr(w)
			return
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createClient(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Name, Phone, Email, Birthday string
		RemindersConsent             *bool `json:"reminders_consent"`
		MarketingConsent             *bool `json:"marketing_consent"`
		WhatsAppConsent              *bool `json:"whatsapp_consent"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name, req.Phone = strings.TrimSpace(req.Name), strings.TrimSpace(req.Phone)
	req.Email = lowerTrim(req.Email)
	if msg := validContact(req.Name, req.Phone, req.Email); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	bday, ok := parseBirthday(req.Birthday)
	if !ok {
		writeErr(w, http.StatusBadRequest, "fecha de nacimiento inválida (AAAA-MM-DD)")
		return
	}
	consent := req.RemindersConsent != nil && *req.RemindersConsent
	promos := req.MarketingConsent != nil && *req.MarketingConsent
	wa := req.WhatsAppConsent != nil && *req.WhatsAppConsent
	if wa && waNumber(req.Phone) == "" {
		writeErr(w, http.StatusBadRequest, "para WhatsApp se necesita un teléfono de 10 dígitos")
		return
	}
	if (consent || promos) && req.Email == "" {
		writeErr(w, http.StatusBadRequest, "para enviar mensajes se necesita el correo")
		return
	}
	var id int64
	if err := a.DB.QueryRow(r.Context(),
		`INSERT INTO clients (business_id, name, phone, email, birthday, reminders_consent, marketing_consent, whatsapp_consent) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		p.BusinessID, req.Name, req.Phone, req.Email, bday, consent, promos, wa).Scan(&id); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *API) getClient(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := scanClient(a.DB.QueryRow(r.Context(),
		`SELECT `+clientCols+` FROM clients WHERE id = $1 AND business_id = $2`, id, p.BusinessID))
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (a *API) updateClient(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name, Phone, Email, Birthday string
		RemindersConsent             *bool `json:"reminders_consent"`
		MarketingConsent             *bool `json:"marketing_consent"`
		WhatsAppConsent              *bool `json:"whatsapp_consent"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name, req.Phone = strings.TrimSpace(req.Name), strings.TrimSpace(req.Phone)
	req.Email = lowerTrim(req.Email)
	if msg := validContact(req.Name, req.Phone, req.Email); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	bday, ok := parseBirthday(req.Birthday)
	if !ok {
		writeErr(w, http.StatusBadRequest, "fecha de nacimiento inválida (AAAA-MM-DD)")
		return
	}
	if ((req.RemindersConsent != nil && *req.RemindersConsent) || (req.MarketingConsent != nil && *req.MarketingConsent)) && req.Email == "" {
		writeErr(w, http.StatusBadRequest, "para enviar mensajes se necesita el correo")
		return
	}
	if req.WhatsAppConsent != nil && *req.WhatsAppConsent && waNumber(req.Phone) == "" {
		writeErr(w, http.StatusBadRequest, "para WhatsApp se necesita un teléfono de 10 dígitos")
		return
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE clients SET name = $3, phone = $4, email = $5, birthday = $6, reminders_consent = COALESCE($7, reminders_consent) AND $5 <> '',
		   marketing_consent = COALESCE($8, marketing_consent) AND $5 <> '',
		   whatsapp_consent = COALESCE($9, whatsapp_consent) AND length(regexp_replace($4, '\D', '', 'g')) >= 10
		 WHERE id = $1 AND business_id = $2`,
		id, p.BusinessID, req.Name, req.Phone, req.Email, bday, req.RemindersConsent, req.MarketingConsent, req.WhatsAppConsent)
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type note struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *API) listNotes(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(),
		`SELECT n.id, n.body, COALESCE(u.name, ''), n.created_at
		 FROM client_notes n JOIN clients c ON c.id = n.client_id
		 LEFT JOIN users u ON u.id = n.author_id
		 WHERE n.client_id = $1 AND c.business_id = $2 ORDER BY n.created_at DESC, n.id DESC`, id, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []note{}
	for rows.Next() {
		var n note
		if err := rows.Scan(&n.ID, &n.Body, &n.Author, &n.CreatedAt); err != nil {
			serverErr(w)
			return
		}
		out = append(out, n)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createNote(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct{ Body string }
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if n := utf8.RuneCountInString(req.Body); n < 1 || n > 4000 {
		writeErr(w, http.StatusBadRequest, "la nota debe tener de 1 a 4000 caracteres")
		return
	}
	// solo inserta si el cliente es de este negocio
	var noteID int64
	err := a.DB.QueryRow(r.Context(),
		`INSERT INTO client_notes (client_id, author_id, body)
		 SELECT c.id, $3, $4 FROM clients c WHERE c.id = $1 AND c.business_id = $2 RETURNING id`,
		id, p.BusinessID, p.UserID, req.Body).Scan(&noteID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": noteID})
}
