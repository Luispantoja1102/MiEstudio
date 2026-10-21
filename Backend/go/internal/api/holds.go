package api

import (
	"context"
	"net/http"
	"time"
)

// Retención de horario: al elegir una hora en la página pública se aparta unos minutos para esa persona,
// de modo que otra visita no la vea libre mientras se llenan los datos. No es una cita: caduca sola.

const holdTTL = 5 * time.Minute

// validHolder acepta el identificador anónimo que genera el navegador.
func validHolder(h string) bool { return len(h) >= 16 && len(h) <= 64 }

// withoutOthersHolds quita de lo ofrecido los horarios que otra visita tiene apartados.
// Cada apartado cubre la duración del servicio más su preparación.
func (a *API) withoutOthersHolds(ctx context.Context, offers []staffSlots, svc *svcInfo, holder string) ([]staffSlots, error) {
	if len(offers) == 0 {
		return offers, nil
	}
	ids := make([]int64, len(offers))
	for i, o := range offers {
		ids[i] = o.StaffID
	}
	rows, err := a.DB.Query(ctx,
		`SELECT staff_id, starts_at, ends_at FROM booking_holds WHERE staff_id = ANY($1) AND expires_at > $2 AND holder <> $3`, ids, a.now(), holder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type span struct{ s, e time.Time }
	held := map[int64][]span{}
	for rows.Next() {
		var id int64
		var s, e time.Time
		if err := rows.Scan(&id, &s, &e); err != nil {
			return nil, err
		}
		held[id] = append(held[id], span{s, e})
	}
	if len(held) == 0 {
		return offers, nil
	}
	span2 := svc.Duration + svc.Buffer
	out := make([]staffSlots, 0, len(offers))
	for _, o := range offers {
		kept := []string{}
		for _, st := range o.Starts {
			t, err := time.Parse(time.RFC3339, st)
			if err != nil {
				continue
			}
			free := true
			for _, h := range held[o.StaffID] {
				if t.Before(h.e) && t.Add(span2).After(h.s) {
					free = false
					break
				}
			}
			if free {
				kept = append(kept, st)
			}
		}
		if len(kept) > 0 {
			o.Starts = kept
			out = append(out, o)
		}
	}
	return out, nil
}

type holdReq struct {
	ServiceID int64  `json:"service_id"`
	BranchID  int64  `json:"branch_id"`
	StaffID   int64  `json:"staff_id"` // 0 = cualquiera
	StartsAt  string `json:"starts_at"`
	Holder    string `json:"holder"`
}

// publicHold es POST /api/public/{slug}/holds: aparta el horario elegido por unos minutos.
func (a *API) publicHold(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !a.allow(w, "pub-hold:"+clientIP(r), 60, time.Minute) {
		return
	}
	b, err := a.publicBusiness(ctx, r.PathValue("slug"))
	if err != nil {
		serverErr(w)
		return
	}
	if b == nil {
		notFound(w)
		return
	}
	var req holdReq
	if !decode(w, r, &req) || !validHolder(req.Holder) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	start, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "horario inválido")
		return
	}
	svc, _, _, err := a.onlineService(ctx, b.ID, req.ServiceID)
	if err != nil {
		serverErr(w)
		return
	}
	okBranch, err := activeBranch(ctx, a.DB, b.ID, req.BranchID)
	if err != nil {
		serverErr(w)
		return
	}
	if svc == nil || !okBranch {
		writeErr(w, http.StatusBadRequest, "servicio o sucursal no disponibles")
		return
	}
	now := a.now()
	ls := start.In(b.Loc)
	day := time.Date(ls.Year(), ls.Month(), ls.Day(), 0, 0, 0, 0, b.Loc)
	if !b.withinHorizon(day, now) {
		writeErr(w, http.StatusConflict, "ese horario ya no está disponible")
		return
	}
	earliest := now.Add(time.Duration(b.Settings.MinNoticeHours) * time.Hour)
	offers, err := a.computeSlots(ctx, b.ID, svc, req.ServiceID, req.BranchID, req.StaffID, day, b.Loc, earliest)
	if err == nil {
		offers, err = a.withoutOthersHolds(ctx, offers, svc, req.Holder)
	}
	if err != nil {
		serverErr(w)
		return
	}
	staffID, ok, err := a.pickStaff(ctx, b.ID, offers, start.Format(time.RFC3339), req.StaffID, day)
	if err != nil {
		serverErr(w)
		return
	}
	if !ok {
		writeErr(w, http.StatusConflict, "ese horario ya no está disponible")
		return
	}
	expires := now.Add(holdTTL)
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	// cada visita aparta un solo horario a la vez; de paso se limpian los caducados
	if _, err := tx.Exec(ctx, `DELETE FROM booking_holds WHERE holder = $1 OR expires_at <= $2`, req.Holder, now); err != nil {
		serverErr(w)
		return
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO booking_holds (business_id, staff_id, starts_at, ends_at, holder, expires_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		b.ID, staffID, start, start.Add(svc.Duration+svc.Buffer), req.Holder, expires); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"staff_id": staffID, "expires_at": expires.UTC().Format(time.RFC3339), "seconds": int(holdTTL.Seconds())})
}
