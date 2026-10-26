package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"miestudio/backend-go/internal/auth"
)

// Calendario privado. Cada persona de la agenda puede activarlo y obtener una dirección a la que suscribirse desde
// Google Calendar (u otro calendario): sus citas aparecen ahí. Es de un solo sentido: Barbella publica y nunca lee el
// calendario de nadie. La dirección lleva un secreto (quien la tenga puede leer el calendario) que se puede cambiar en
// cualquier momento, y eso corta la anterior. Por omisión los eventos no llevan el nombre del cliente.

const (
	feedPast   = 30 * 24 * time.Hour
	feedFuture = 365 * 24 * time.Hour
)

func (a *API) feedState(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var tok *string
	var names bool
	if err := a.DB.QueryRow(r.Context(), `SELECT calendar_token, calendar_show_names FROM users WHERE id = $1 AND business_id = $2`, p.UserID, p.BusinessID).Scan(&tok, &names); err != nil {
		serverErr(w)
		return
	}
	out := map[string]any{"enabled": tok != nil, "show_names": names, "path": ""}
	if tok != nil {
		out["path"] = "/api/calendar/" + *tok + ".ics"
	}
	writeJSON(w, http.StatusOK, out)
}

// feedUpdate: {"action": "enable" | "rotate" | "disable", "show_names": bool}
func (a *API) feedUpdate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Action    string `json:"action"`
		ShowNames bool   `json:"show_names"`
	}
	if !decode(w, r, &in) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	p := auth.FromContext(r.Context())
	var err error
	switch in.Action {
	case "enable": // conserva la dirección actual si ya hay una
		_, err = a.DB.Exec(r.Context(), `UPDATE users SET calendar_token = COALESCE(calendar_token, $3), calendar_show_names = $4 WHERE id = $1 AND business_id = $2`,
			p.UserID, p.BusinessID, newPublicToken(), in.ShowNames)
	case "rotate":
		_, err = a.DB.Exec(r.Context(), `UPDATE users SET calendar_token = $3, calendar_show_names = $4 WHERE id = $1 AND business_id = $2`,
			p.UserID, p.BusinessID, newPublicToken(), in.ShowNames)
	case "disable":
		_, err = a.DB.Exec(r.Context(), `UPDATE users SET calendar_token = NULL WHERE id = $1 AND business_id = $2`, p.UserID, p.BusinessID)
	default:
		writeErr(w, http.StatusBadRequest, "acción no válida")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	a.feedState(w, r)
}

// publicFeed es GET /api/calendar/{token}.ics: el calendario de una persona. Una dirección desconocida, apagada o de un
// negocio suspendido responde igual (404) para no revelar nada.
func (a *API) publicFeed(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !a.allow(w, "cal-read:"+ip, 120, 10*time.Minute) {
		return
	}
	tok, ok := strings.CutSuffix(r.PathValue("file"), ".ics")
	notFound := func() { writeErr(w, http.StatusNotFound, "este calendario no existe") }
	if !ok || !tokenShape.MatchString(tok) {
		notFound()
		return
	}
	ctx := r.Context()
	now := a.now()
	var userID, bizID int64
	var bizName string
	var names bool
	err := a.DB.QueryRow(ctx,
		`SELECT u.id, u.business_id, b.name, u.calendar_show_names
		   FROM users u JOIN businesses b ON b.id = u.business_id
		  WHERE u.calendar_token = $1 AND u.active AND `+usableSQL("b.", "$2"), tok, now).Scan(&userID, &bizID, &bizName, &names)
	if err != nil {
		notFound()
		return
	}
	rows, err := a.DB.Query(ctx,
		`SELECT ap.id, ap.starts_at, ap.ends_at, s.name, c.name, c.phone, br.name, br.address
		   FROM appointments ap JOIN services s ON s.id = ap.service_id JOIN clients c ON c.id = ap.client_id JOIN branches br ON br.id = ap.branch_id
		  WHERE ap.business_id = $1 AND ap.staff_id = $2 AND ap.status IN ('scheduled', 'pending_payment', 'completed')
		    AND ap.starts_at BETWEEN $3 AND $4 ORDER BY ap.starts_at LIMIT 3000`, bizID, userID, now.Add(-feedPast), now.Add(feedFuture))
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	var events [][]string
	for rows.Next() {
		var id int64
		var start, end time.Time
		var service, client, phone, branch, address string
		if err := rows.Scan(&id, &start, &end, &service, &client, &phone, &branch, &address); err != nil {
			serverErr(w)
			return
		}
		summary, notes := "Cita · "+service, "Servicio: "+service
		if names {
			summary = service + " · " + client
			notes += "\nCliente: " + client
			if phone != "" {
				notes += "\nTeléfono: " + phone
			}
		}
		where := branch
		if address != "" {
			where += ", " + address
		}
		events = append(events, icsEvent("agenda-"+strconv.FormatInt(id, 10)+"@barbella", 0, start, end, summary, where, notes, "CONFIRMED", "", ""))
	}
	if rows.Err() != nil {
		serverErr(w)
		return
	}
	header := []string{"X-WR-CALNAME:" + icsText("Citas · "+bizName), "X-WR-CALDESC:" + icsText("Tus citas de "+bizName+" en Barbella"), "REFRESH-INTERVAL;VALUE=DURATION:PT1H", "X-PUBLISHED-TTL:PT1H"}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write([]byte(icsCalendar("PUBLISH", header, events...)))
}
