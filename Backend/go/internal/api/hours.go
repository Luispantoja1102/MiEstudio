package api

import (
	"net/http"
	"sort"
	"time"

	"miestudio/backend-go/internal/auth"
)

type hourBlock struct {
	BranchID int64  `json:"branch_id"`
	Weekday  int    `json:"weekday"` // 0 = domingo
	Start    string `json:"start"`   // HH:MM
	End      string `json:"end"`
}

func (a *API) getHours(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(),
		`SELECT h.branch_id, h.weekday, to_char(h.start_time, 'HH24:MI'), to_char(h.end_time, 'HH24:MI')
		 FROM working_hours h JOIN users u ON u.id = h.user_id
		 WHERE h.user_id = $1 AND u.business_id = $2 ORDER BY h.weekday, h.start_time`, id, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []hourBlock{}
	for rows.Next() {
		var b hourBlock
		if err := rows.Scan(&b.BranchID, &b.Weekday, &b.Start, &b.End); err != nil {
			serverErr(w)
			return
		}
		out = append(out, b)
	}
	writeJSON(w, http.StatusOK, out)
}

// setHours reemplaza el horario semanal de una persona.
func (a *API) setHours(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Blocks []hourBlock `json:"blocks"`
	}
	if !decode(w, r, &req) || len(req.Blocks) > 100 {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	type span struct{ start, end time.Time }
	byDay := map[int][]span{}
	branchIDs := []int64{}
	for _, b := range req.Blocks {
		s, err1 := time.Parse("15:04", b.Start)
		e, err2 := time.Parse("15:04", b.End)
		if err1 != nil || err2 != nil || b.Weekday < 0 || b.Weekday > 6 || !e.After(s) {
			writeErr(w, http.StatusBadRequest, "horario inválido: usa HH:MM y que el fin sea después del inicio")
			return
		}
		byDay[b.Weekday] = append(byDay[b.Weekday], span{s, e})
		branchIDs = append(branchIDs, b.BranchID)
	}
	for _, spans := range byDay {
		sort.Slice(spans, func(i, j int) bool { return spans[i].start.Before(spans[j].start) })
		for i := 1; i < len(spans); i++ {
			if spans[i].start.Before(spans[i-1].end) {
				writeErr(w, http.StatusBadRequest, "hay horarios encimados el mismo día")
				return
			}
		}
	}
	branchIDs = uniqueIDs(branchIDs)

	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)

	var userOK bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND business_id = $2)`, id, p.BusinessID).Scan(&userOK); err != nil {
		serverErr(w)
		return
	}
	if !userOK {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	var branchesOK int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM branches WHERE business_id = $1 AND id = ANY($2)`, p.BusinessID, branchIDs).Scan(&branchesOK); err != nil {
		serverErr(w)
		return
	}
	if branchesOK != len(branchIDs) {
		writeErr(w, http.StatusBadRequest, "hay sucursales que no pertenecen a tu negocio")
		return
	}
	if _, err := tx.Exec(ctx, `DELETE FROM working_hours WHERE user_id = $1`, id); err != nil {
		serverErr(w)
		return
	}
	for _, b := range req.Blocks {
		if _, err := tx.Exec(ctx,
			`INSERT INTO working_hours (user_id, branch_id, weekday, start_time, end_time) VALUES ($1,$2,$3,$4,$5)`,
			id, b.BranchID, b.Weekday, b.Start, b.End); err != nil {
			serverErr(w)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
