package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

type cashMove struct {
	ID          int64     `json:"id"`
	Kind        string    `json:"kind"`
	AmountCents int       `json:"amount_cents"`
	Concept     string    `json:"concept"`
	By          string    `json:"by"`
	CreatedAt   time.Time `json:"created_at"`
}

type methodTotal struct {
	Method      string `json:"method"`
	AmountCents int    `json:"amount_cents"`
	Count       int    `json:"count"`
}

type cashSummary struct {
	ID            int64         `json:"id"`
	BranchID      int64         `json:"branch_id"`
	OpenedBy      string        `json:"opened_by"`
	OpenedAt      time.Time     `json:"opened_at"`
	OpeningCents  int           `json:"opening_cents"`
	ClosedBy      string        `json:"closed_by"`
	ClosedAt      *time.Time    `json:"closed_at"`
	CountedCents  *int          `json:"counted_cents"`
	ExpectedCents int           `json:"expected_cents"`
	DiffCents     *int          `json:"diff_cents"`
	Note          string        `json:"note"`
	Sales         int           `json:"sales"`
	SalesCents    int           `json:"sales_cents"` // lo vendido, sin propinas
	TipCents      int           `json:"tip_cents"`
	CashInCents   int           `json:"cash_in_cents"` // efectivo cobrado, ya sin el cambio
	MovesIn       int           `json:"moves_in_cents"`
	MovesOut      int           `json:"moves_out_cents"`
	ByMethod      []methodTotal `json:"by_method"`
	Movements     []cashMove    `json:"movements"`
}

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// summarizeCash calcula lo que debe haber en la caja. Las ventas canceladas no cuentan.
// El dinero cuenta en la caja que estaba abierta cuando se recibió.
func summarizeCash(ctx context.Context, q queryer, bizID, id int64) (*cashSummary, error) {
	var c cashSummary
	var expected *int
	err := q.QueryRow(ctx,
		`SELECT cs.id, cs.branch_id, COALESCE(ou.name, ''), cs.opened_at, cs.opening_cents, COALESCE(cu.name, ''),
		        cs.closed_at, cs.counted_cents, cs.expected_cents, cs.note
		 FROM cash_sessions cs JOIN branches b ON b.id = cs.branch_id
		 LEFT JOIN users ou ON ou.id = cs.opened_by LEFT JOIN users cu ON cu.id = cs.closed_by
		 WHERE cs.id = $1 AND b.business_id = $2`, id, bizID).
		Scan(&c.ID, &c.BranchID, &c.OpenedBy, &c.OpenedAt, &c.OpeningCents, &c.ClosedBy, &c.ClosedAt, &c.CountedCents, &expected, &c.Note)
	if err != nil {
		return nil, err
	}
	if err := q.QueryRow(ctx,
		`SELECT count(*), COALESCE(sum(total_cents), 0), COALESCE(sum(tip_cents), 0)
		 FROM sales WHERE cash_session_id = $1 AND status = 'paid'`, id).Scan(&c.Sales, &c.SalesCents, &c.TipCents); err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx,
		`SELECT sp.method, COALESCE(sum(sp.amount_cents), 0), count(*)
		 FROM sale_payments sp JOIN sales s ON s.id = sp.sale_id
		 WHERE sp.cash_session_id = $1 AND s.status = 'paid' GROUP BY sp.method ORDER BY sp.method`, id)
	if err != nil {
		return nil, err
	}
	c.ByMethod = []methodTotal{}
	for rows.Next() {
		var m methodTotal
		if err := rows.Scan(&m.Method, &m.AmountCents, &m.Count); err != nil {
			rows.Close()
			return nil, err
		}
		if m.Method == "cash" {
			c.CashInCents = m.AmountCents
		}
		c.ByMethod = append(c.ByMethod, m)
	}
	rows.Close()

	mrows, err := q.Query(ctx,
		`SELECT m.id, m.kind, m.amount_cents, m.concept, COALESCE(u.name, ''), m.created_at
		 FROM cash_movements m LEFT JOIN users u ON u.id = m.created_by WHERE m.cash_session_id = $1 ORDER BY m.created_at, m.id`, id)
	if err != nil {
		return nil, err
	}
	defer mrows.Close()
	c.Movements = []cashMove{}
	for mrows.Next() {
		var m cashMove
		if err := mrows.Scan(&m.ID, &m.Kind, &m.AmountCents, &m.Concept, &m.By, &m.CreatedAt); err != nil {
			return nil, err
		}
		if m.Kind == "in" {
			c.MovesIn += m.AmountCents
		} else {
			c.MovesOut += m.AmountCents
		}
		c.Movements = append(c.Movements, m)
	}
	c.ExpectedCents = c.OpeningCents + c.CashInCents + c.MovesIn - c.MovesOut
	if c.ClosedAt != nil && expected != nil {
		c.ExpectedCents = *expected // se congela al cerrar
	}
	if c.CountedCents != nil {
		d := *c.CountedCents - c.ExpectedCents
		c.DiffCents = &d
	}
	return &c, nil
}

// openSessionID devuelve la caja abierta de una sucursal (0 si no hay).
func openSessionID(ctx context.Context, q queryer, bizID, branchID int64) (int64, error) {
	var id int64
	err := q.QueryRow(ctx,
		`SELECT cs.id FROM cash_sessions cs JOIN branches b ON b.id = cs.branch_id
		 WHERE cs.branch_id = $1 AND b.business_id = $2 AND cs.closed_at IS NULL`, branchID, bizID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

func queryBranch(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.URL.Query().Get("branch_id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "indica branch_id")
		return 0, false
	}
	return id, true
}

// currentCash devuelve la caja abierta de la sucursal, o null si está cerrada.
func (a *API) currentCash(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	branchID, ok := queryBranch(w, r)
	if !ok {
		return
	}
	id, err := openSessionID(r.Context(), a.DB, p.BusinessID, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	if id == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"session": nil})
		return
	}
	c, err := summarizeCash(r.Context(), a.DB, p.BusinessID, id)
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": c})
}

func (a *API) openCash(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		BranchID     int64 `json:"branch_id"`
		OpeningCents int   `json:"opening_cents"`
	}
	if !decode(w, r, &req) || req.OpeningCents < 0 || req.OpeningCents > maxPriceCents {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	ok, err := activeBranch(r.Context(), a.DB, p.BusinessID, req.BranchID)
	if err != nil {
		serverErr(w)
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "sucursal no encontrada")
		return
	}
	var id int64
	err = a.DB.QueryRow(r.Context(),
		`INSERT INTO cash_sessions (branch_id, opened_by, opening_cents) VALUES ($1, $2, $3) RETURNING id`,
		req.BranchID, p.UserID, req.OpeningCents).Scan(&id)
	if code, _ := pgConstraint(err); code == "23505" {
		writeErr(w, http.StatusConflict, "ya hay una caja abierta en esta sucursal")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// cashMovement registra una entrada o salida de efectivo (gastos, propinas pagadas, fondo, retiros).
func (a *API) cashMovement(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		BranchID    int64  `json:"branch_id"`
		Kind        string `json:"kind"`
		AmountCents int    `json:"amount_cents"`
		Concept     string `json:"concept"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Concept = strings.TrimSpace(req.Concept)
	if (req.Kind != "in" && req.Kind != "out") || req.AmountCents <= 0 || req.AmountCents > maxPriceCents ||
		utf8.RuneCountInString(req.Concept) < 2 || utf8.RuneCountInString(req.Concept) > 200 {
		writeErr(w, http.StatusBadRequest, "indica el tipo, un monto positivo y el concepto")
		return
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	sid, err := openSessionID(ctx, tx, p.BusinessID, req.BranchID)
	if err != nil {
		serverErr(w)
		return
	}
	if sid == 0 {
		writeErr(w, http.StatusConflict, "la caja está cerrada")
		return
	}
	// bloquea la caja para que dos salidas simultáneas no saquen más de lo que hay
	if _, err := tx.Exec(ctx, `SELECT 1 FROM cash_sessions WHERE id = $1 FOR UPDATE`, sid); err != nil {
		serverErr(w)
		return
	}
	if req.Kind == "out" {
		c, err := summarizeCash(ctx, tx, p.BusinessID, sid)
		if err != nil {
			serverErr(w)
			return
		}
		if req.AmountCents > c.ExpectedCents {
			writeErr(w, http.StatusConflict, "no hay tanto efectivo en la caja")
			return
		}
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO cash_movements (cash_session_id, kind, amount_cents, concept, created_by) VALUES ($1, $2, $3, $4, $5)`,
		sid, req.Kind, req.AmountCents, req.Concept, p.UserID); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// closeCash cierra la caja con el efectivo contado. El esperado queda congelado.
func (a *API) closeCash(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		BranchID     int64  `json:"branch_id"`
		CountedCents int    `json:"counted_cents"`
		Note         string `json:"note"`
	}
	if !decode(w, r, &req) || req.CountedCents < 0 || req.CountedCents > maxPriceCents || len(req.Note) > 500 {
		writeErr(w, http.StatusBadRequest, "indica el efectivo contado")
		return
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	sid, err := openSessionID(ctx, tx, p.BusinessID, req.BranchID)
	if err != nil {
		serverErr(w)
		return
	}
	if sid == 0 {
		writeErr(w, http.StatusConflict, "la caja ya está cerrada")
		return
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM cash_sessions WHERE id = $1 FOR UPDATE`, sid); err != nil {
		serverErr(w)
		return
	}
	c, err := summarizeCash(ctx, tx, p.BusinessID, sid)
	if err != nil {
		serverErr(w)
		return
	}
	if _, err := tx.Exec(ctx,
		`UPDATE cash_sessions SET closed_at = now(), closed_by = $2, counted_cents = $3, expected_cents = $4, note = $5 WHERE id = $1`,
		sid, p.UserID, req.CountedCents, c.ExpectedCents, strings.TrimSpace(req.Note)); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	closed, err := summarizeCash(ctx, a.DB, p.BusinessID, sid)
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, closed)
}

// listCashSessions devuelve el historial de cajas (las más recientes primero).
func (a *API) listCashSessions(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	branchID, ok := queryBranch(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(),
		`SELECT cs.id FROM cash_sessions cs JOIN branches b ON b.id = cs.branch_id
		 WHERE cs.branch_id = $1 AND b.business_id = $2 ORDER BY cs.opened_at DESC, cs.id DESC LIMIT 30`, branchID, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			serverErr(w)
			return
		}
		ids = append(ids, id)
	}
	rows.Close()
	out := []*cashSummary{}
	for _, id := range ids {
		c, err := summarizeCash(r.Context(), a.DB, p.BusinessID, id)
		if err != nil {
			serverErr(w)
			return
		}
		c.Movements = nil // el detalle va en la caja actual
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, out)
}

type cashTicket struct {
	Folio      int       `json:"folio"`
	CreatedAt  time.Time `json:"created_at"`
	TotalCents int       `json:"total_cents"`
	TipCents   int       `json:"tip_cents"`
	Status     string    `json:"status"`
	Methods    []string  `json:"methods"`
}

// cashReport es el corte de una caja listo para imprimir: el resumen con el encabezado del negocio,
// lo cancelado y los tickets del turno.
type cashReport struct {
	cashSummary
	BusinessName  string       `json:"business_name"`
	BranchName    string       `json:"branch_name"`
	BranchAddress string       `json:"branch_address"`
	BranchPhone   string       `json:"branch_phone"`
	VoidedCount   int          `json:"voided_count"`
	VoidedCents   int          `json:"voided_cents"`
	Tickets       []cashTicket `json:"tickets"`
}

// getCashSession devuelve el corte de una caja (cerrada, o abierta como corte parcial).
func (a *API) getCashSession(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	c, err := summarizeCash(ctx, a.DB, p.BusinessID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	out := cashReport{cashSummary: *c, Tickets: []cashTicket{}}
	if err := a.DB.QueryRow(ctx,
		`SELECT bz.name, br.name, br.address, br.phone FROM businesses bz, branches br WHERE bz.id = $1 AND br.id = $2`,
		p.BusinessID, c.BranchID).Scan(&out.BusinessName, &out.BranchName, &out.BranchAddress, &out.BranchPhone); err != nil {
		serverErr(w)
		return
	}
	if err := a.DB.QueryRow(ctx,
		`SELECT count(*), COALESCE(sum(total_cents), 0) FROM sales WHERE cash_session_id = $1 AND status = 'void'`, id).Scan(&out.VoidedCount, &out.VoidedCents); err != nil {
		serverErr(w)
		return
	}
	rows, err := a.DB.Query(ctx,
		`SELECT s.folio, s.created_at, s.total_cents, s.tip_cents, s.status,
		        COALESCE(array_agg(DISTINCT sp.method) FILTER (WHERE sp.method IS NOT NULL), '{}')
		 FROM sales s LEFT JOIN sale_payments sp ON sp.sale_id = s.id
		 WHERE s.cash_session_id = $1 GROUP BY s.id ORDER BY s.created_at, s.id LIMIT 500`, id)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t cashTicket
		if err := rows.Scan(&t.Folio, &t.CreatedAt, &t.TotalCents, &t.TipCents, &t.Status, &t.Methods); err != nil {
			serverErr(w)
			return
		}
		out.Tickets = append(out.Tickets, t)
	}
	writeJSON(w, http.StatusOK, out)
}
