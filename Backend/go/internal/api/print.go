package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

// Cola de impresión entre dispositivos: el celular cobra y manda el ticket; la computadora que tiene la impresora
// (la «estación» de la sucursal) lo recoge y lo imprime.
//   - La estación pide trabajos con una petición larga: el servidor contesta en cuanto llega uno, así el ticket sale
//     al instante aunque la pestaña esté en segundo plano (los temporizadores del navegador se frenan ahí).
//   - Un trabajo lo toma una sola estación a la vez; si no responde en printClaimTimeout, otro intento puede tomarlo.
//   - Un ticket que lleva más de printJobTTL esperando ya no se imprime (no sacar tickets viejos de golpe).
const (
	printJobTTL        = 10 * time.Minute
	printClaimTimeout  = 30 * time.Second
	printStationOnline = 30 * time.Second
)

// Cuánto espera la estación una respuesta y cada cuánto revisa la cola; las pruebas los acortan.
var (
	printClaimWait     = 12 * time.Second
	printClaimInterval = time.Second
)

func (a *API) printStationStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	branchID, ok := queryBranch(w, r)
	if !ok {
		return
	}
	var name string
	var online bool
	err := a.DB.QueryRow(r.Context(),
		`SELECT name, last_seen_at > now() - make_interval(secs => $3) FROM print_stations
		 WHERE business_id = $1 AND branch_id = $2 ORDER BY last_seen_at DESC LIMIT 1`,
		p.BusinessID, branchID, printStationOnline.Seconds()).Scan(&name, &online)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"online": online, "name": name})
}

// enqueuePrint manda a la estación el ticket de una venta ya cobrada.
func (a *API) enqueuePrint(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		SaleID     int64 `json:"sale_id"`
		OpenDrawer bool  `json:"open_drawer"`
	}
	if !decode(w, r, &req) || req.SaleID <= 0 {
		writeErr(w, http.StatusBadRequest, "indica la venta")
		return
	}
	var branchID int64
	var status string
	err := a.DB.QueryRow(r.Context(), `SELECT branch_id, status FROM sales WHERE id = $1 AND business_id = $2`, req.SaleID, p.BusinessID).Scan(&branchID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "venta no encontrada")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	if status != "paid" {
		writeErr(w, http.StatusBadRequest, "solo se imprimen tickets de ventas cobradas")
		return
	}
	var online bool
	if err := a.DB.QueryRow(r.Context(),
		`SELECT EXISTS (SELECT 1 FROM print_stations WHERE business_id = $1 AND branch_id = $2 AND last_seen_at > now() - make_interval(secs => $3))`,
		p.BusinessID, branchID, printStationOnline.Seconds()).Scan(&online); err != nil {
		serverErr(w)
		return
	}
	if !online {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "La computadora de la caja no está conectada.", "code": "STATION_OFFLINE"})
		return
	}
	var id int64
	if err := a.DB.QueryRow(r.Context(),
		`INSERT INTO print_jobs (business_id, branch_id, sale_id, open_drawer, created_by) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		p.BusinessID, branchID, req.SaleID, req.OpenDrawer, p.UserID).Scan(&id); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "status": "pending"})
}

type printJobOut struct {
	ID         int64    `json:"id"`
	OpenDrawer bool     `json:"open_drawer"`
	Sale       *saleOut `json:"sale"`
}

// claimPrint: la estación pide su siguiente trabajo. Deja señal de vida y espera hasta printClaimWait a que haya uno.
func (a *API) claimPrint(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		StationID string `json:"station_id"`
		Name      string `json:"name"`
		BranchID  int64  `json:"branch_id"`
	}
	if !decode(w, r, &req) || req.BranchID <= 0 || strings.TrimSpace(req.StationID) == "" {
		writeErr(w, http.StatusBadRequest, "faltan la estación y la sucursal")
		return
	}
	station := req.StationID
	if len(station) > 64 {
		station = station[:64]
	}
	name := strings.TrimSpace(req.Name)
	if len(name) > 80 {
		name = name[:80]
	}
	ctx := r.Context()
	deadline := time.Now().Add(printClaimWait)
	for {
		// la sucursal debe ser del negocio: si no, no se registra la estación
		tag, err := a.DB.Exec(ctx,
			`INSERT INTO print_stations (business_id, station_id, branch_id, name, last_seen_at)
			 SELECT $1, $2, b.id, $4, now() FROM branches b WHERE b.id = $3 AND b.business_id = $1
			 ON CONFLICT (business_id, station_id) DO UPDATE SET branch_id = EXCLUDED.branch_id, name = EXCLUDED.name, last_seen_at = now()`,
			p.BusinessID, station, req.BranchID, name)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			serverErr(w)
			return
		}
		if tag.RowsAffected() == 0 {
			writeErr(w, http.StatusNotFound, "sucursal no encontrada")
			return
		}
		var j printJobOut
		err = a.DB.QueryRow(ctx,
			`UPDATE print_jobs SET status = 'claimed', station_id = $3, claimed_at = now()
			 WHERE id = (SELECT id FROM print_jobs
			             WHERE business_id = $1 AND branch_id = $2 AND created_at > now() - make_interval(secs => $4)
			               AND (status = 'pending' OR (status = 'claimed' AND claimed_at < now() - make_interval(secs => $5)))
			             ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED)
			 RETURNING id, open_drawer, sale_id`,
			p.BusinessID, req.BranchID, station, printJobTTL.Seconds(), printClaimTimeout.Seconds()).
			Scan(&j.ID, &j.OpenDrawer, new(int64))
		if err == nil {
			sale, err := a.saleOfJob(ctx, p.BusinessID, j.ID)
			if err != nil {
				a.finishPrint(ctx, p.BusinessID, j.ID, station, false, "La venta ya no existe")
				continue
			}
			j.Sale = sale
			writeJSON(w, http.StatusOK, map[string]any{"job": j})
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			if ctx.Err() != nil {
				return
			}
			serverErr(w)
			return
		}
		if time.Now().Add(printClaimInterval).After(deadline) {
			writeJSON(w, http.StatusOK, map[string]any{"job": nil})
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(printClaimInterval):
		}
	}
}

func (a *API) saleOfJob(ctx context.Context, bizID, jobID int64) (*saleOut, error) {
	var saleID int64
	if err := a.DB.QueryRow(ctx, `SELECT sale_id FROM print_jobs WHERE id = $1 AND business_id = $2`, jobID, bizID).Scan(&saleID); err != nil {
		return nil, err
	}
	return loadSale(ctx, a.DB, bizID, saleID)
}

func (a *API) finishPrint(ctx context.Context, bizID, jobID int64, station string, ok bool, msg string) bool {
	status := "failed"
	if ok {
		status = "done"
	}
	if len(msg) > 300 {
		msg = msg[:300]
	}
	tag, err := a.DB.Exec(ctx,
		`UPDATE print_jobs SET status = $4, error = $5 WHERE id = $1 AND business_id = $2 AND station_id = $3 AND status = 'claimed'`,
		jobID, bizID, station, status, msg)
	return err == nil && tag.RowsAffected() == 1
}

// printResult: la estación avisa que imprimió (o que falló).
func (a *API) printResult(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		StationID string `json:"station_id"`
		OK        bool   `json:"ok"`
		Error     string `json:"error"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "datos inválidos")
		return
	}
	if !a.finishPrint(r.Context(), p.BusinessID, id, req.StationID, req.OK, req.Error) {
		writeErr(w, http.StatusConflict, "ese trabajo ya no lo tiene esta estación")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// printJobStatus: el celular consulta cómo va su ticket.
func (a *API) printJobStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var status, msg string
	err := a.DB.QueryRow(r.Context(), `SELECT status, error FROM print_jobs WHERE id = $1 AND business_id = $2`, id, p.BusinessID).Scan(&status, &msg)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "trabajo no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": status, "error": msg})
}
