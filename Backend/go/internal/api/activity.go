package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"miestudio/backend-go/internal/auth"
)

// logActivity guarda una acción en la bitácora. actorID o businessID en 0 significan "ninguno".
// Un fallo al registrar no debe romper la operación principal.
func (a *API) logActivity(ctx context.Context, actorID, businessID int64, action string, detail map[string]any) {
	if detail == nil {
		detail = map[string]any{}
	}
	raw, _ := json.Marshal(detail)
	_, err := a.DB.Exec(ctx,
		`INSERT INTO activity_log (actor_id, business_id, action, detail)
		 VALUES (NULLIF($1, 0), NULLIF($2, 0), $3, $4)`, actorID, businessID, action, raw)
	if err != nil {
		log.Printf("bitácora: %v", err)
	}
}

type activityRow struct {
	ID        int64           `json:"id"`
	Action    string          `json:"action"`
	Actor     string          `json:"actor"`
	Business  string          `json:"business"`
	BusinessI *int64          `json:"business_id"`
	Detail    json.RawMessage `json:"detail"`
	CreatedAt time.Time       `json:"created_at"`
}

func (a *API) platformActivity(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 500 {
		limit = n
	}
	var bizFilter int64
	if v := r.URL.Query().Get("business_id"); v != "" {
		bizFilter, _ = strconv.ParseInt(v, 10, 64)
	}
	rows, err := a.DB.Query(r.Context(),
		`SELECT l.id, l.action, COALESCE(u.name, 'Sistema'), COALESCE(b.name, ''), l.business_id, l.detail, l.created_at
		 FROM activity_log l
		 LEFT JOIN users u ON u.id = l.actor_id
		 LEFT JOIN businesses b ON b.id = l.business_id
		 WHERE ($1 = 0 OR l.business_id = $1)
		 ORDER BY l.created_at DESC, l.id DESC LIMIT $2`, bizFilter, limit)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []activityRow{}
	for rows.Next() {
		var x activityRow
		if err := rows.Scan(&x.ID, &x.Action, &x.Actor, &x.Business, &x.BusinessI, &x.Detail, &x.CreatedAt); err != nil {
			serverErr(w)
			return
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, out)
}

func actor(r *http.Request) int64 { return auth.FromContext(r.Context()).UserID }
