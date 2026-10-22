package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"miestudio/backend-go/internal/auth"
)

// La campana: avisos dentro de la plataforma para quien administra el negocio (administración y recepción).

// inboxAppt agrega un aviso sobre una cita; el cuerpo resume quién, qué y cuándo, y el enlace abre la cita en la agenda.
// Un fallo se registra pero no deshace nada: el aviso es un extra de la operación.
func (a *API) inboxAppt(ctx context.Context, apptID int64, kind, title string) {
	var bizID int64
	var client, service, when, day string
	err := a.DB.QueryRow(ctx,
		`SELECT ap.business_id, c.name, s.name, to_char(ap.starts_at AT TIME ZONE b.timezone, 'DD/MM HH24:MI'), to_char(ap.starts_at AT TIME ZONE b.timezone, 'YYYY-MM-DD')
		   FROM appointments ap JOIN clients c ON c.id = ap.client_id JOIN services s ON s.id = ap.service_id JOIN businesses b ON b.id = ap.business_id
		  WHERE ap.id = $1`, apptID).Scan(&bizID, &client, &service, &when, &day)
	if err != nil {
		log.Printf("aviso: cita %d: %v", apptID, err)
		return
	}
	if _, err := a.DB.Exec(ctx,
		`INSERT INTO inbox (business_id, kind, title, body, link, appointment_id) VALUES ($1, $2, $3, $4, $5, $6)`,
		bizID, kind, title, fmt.Sprintf("%s · %s · %s", client, service, when), fmt.Sprintf("/panel/agenda?cita=%d&date=%s", apptID, day), apptID); err != nil {
		log.Printf("aviso: %v", err)
	}
}

// getInbox devuelve los últimos avisos, cuántos no se han visto y cuántos comprobantes esperan revisión.
func (a *API) getInbox(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	var seen int64
	if err := a.DB.QueryRow(ctx, `SELECT inbox_seen_id FROM users WHERE id = $1`, p.UserID).Scan(&seen); err != nil {
		serverErr(w)
		return
	}
	rows, err := a.DB.Query(ctx,
		`SELECT id, kind, title, body, link, created_at FROM inbox WHERE business_id = $1 ORDER BY id DESC LIMIT 30`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	type item struct {
		ID        int64  `json:"id"`
		Kind      string `json:"kind"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		Link      string `json:"link"`
		CreatedAt string `json:"created_at"`
		Unread    bool   `json:"unread"`
	}
	items := []item{}
	unread := 0
	for rows.Next() {
		var it item
		var created time.Time
		if err := rows.Scan(&it.ID, &it.Kind, &it.Title, &it.Body, &it.Link, &created); err != nil {
			serverErr(w)
			return
		}
		it.CreatedAt = created.UTC().Format("2006-01-02T15:04:05Z")
		it.Unread = it.ID > seen
		if it.Unread {
			unread++
		}
		items = append(items, it)
	}
	var review int
	a.DB.QueryRow(ctx,
		`SELECT count(*) FROM deposits d JOIN appointments ap ON ap.id = d.appointment_id
		  WHERE d.business_id = $1 AND d.method = 'transfer' AND d.status = 'pending' AND d.proof_at IS NOT NULL AND ap.status = 'pending_payment'`, p.BusinessID).Scan(&review)
	writeJSON(w, http.StatusOK, map[string]any{"unread": unread, "items": items, "to_review": review})
}

// readInbox marca todo lo que hay como visto por esta persona.
func (a *API) readInbox(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if _, err := a.DB.Exec(r.Context(),
		`UPDATE users SET inbox_seen_id = COALESCE((SELECT max(id) FROM inbox WHERE business_id = $2), 0) WHERE id = $1`, p.UserID, p.BusinessID); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
