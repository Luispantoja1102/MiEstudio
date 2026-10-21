package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

type saleLineIn struct {
	ServiceID      *int64 `json:"service_id"`
	ProductID      *int64 `json:"product_id"`
	Qty            int    `json:"qty"`
	StaffID        *int64 `json:"staff_id"`
	UnitPriceCents *int   `json:"unit_price_cents"` // cambiar el precio: solo administrador
	DiscountCents  int    `json:"discount_cents"`
}

type paymentIn struct {
	Method        string `json:"method"` // cash, card, transfer, mp_point o deposit (anticipo de la cita)
	AmountCents   int    `json:"amount_cents"`
	ReceivedCents *int   `json:"received_cents"` // efectivo entregado; el cambio se calcula
	Reference     string `json:"reference"`
	MPChargeID    string `json:"mp_charge_id"` // cobro aprobado en la terminal
}

type saleIn struct {
	BranchID      int64        `json:"branch_id"`
	ClientID      *int64       `json:"client_id"`
	AppointmentID *int64       `json:"appointment_id"`
	Lines         []saleLineIn `json:"lines"`
	DiscountCents int          `json:"discount_cents"`
	TipCents      int          `json:"tip_cents"`
	TipStaffID    *int64       `json:"tip_staff_id"`
	Payments      []paymentIn  `json:"payments"`
	Note          string       `json:"note"`
	ClientToken   string       `json:"client_token"` // repetir la misma petición no cobra dos veces
}

type saleItemOut struct {
	ID              int64   `json:"id"`
	Kind            string  `json:"kind"`
	Name            string  `json:"name"`
	Qty             int     `json:"qty"`
	UnitPriceCents  int     `json:"unit_price_cents"`
	DiscountCents   int     `json:"discount_cents"`
	TotalCents      int     `json:"total_cents"`
	StaffID         *int64  `json:"staff_id"`
	StaffName       string  `json:"staff_name"`
	CommissionPct   float64 `json:"commission_pct"`
	CommissionCents int     `json:"commission_cents"`
}

type salePaymentOut struct {
	Method        string `json:"method"`
	AmountCents   int    `json:"amount_cents"`
	ReceivedCents *int   `json:"received_cents"`
	ChangeCents   int    `json:"change_cents"`
	Reference     string `json:"reference"`
	MPChargeID    string `json:"mp_charge_id,omitempty"`
}

type saleOut struct {
	ID            int64            `json:"id"`
	Folio         int              `json:"folio"`
	BranchID      int64            `json:"branch_id"`
	ClientID      *int64           `json:"client_id"`
	ClientName    string           `json:"client_name"`
	AppointmentID *int64           `json:"appointment_id"`
	Cashier       string           `json:"cashier"`
	SubtotalCents int              `json:"subtotal_cents"`
	DiscountCents int              `json:"discount_cents"`
	TotalCents    int              `json:"total_cents"`
	TipCents      int              `json:"tip_cents"`
	TipStaffID    *int64           `json:"tip_staff_id"`
	TipStaffName  string           `json:"tip_staff_name"`
	PaidCents     int              `json:"paid_cents"` // total + propina
	ChangeCents   int              `json:"change_cents"`
	Note          string           `json:"note"`
	Status        string           `json:"status"`
	VoidReason    string           `json:"void_reason"`
	VoidedBy      string           `json:"voided_by"`
	CreatedAt     time.Time        `json:"created_at"`
	Items         []saleItemOut    `json:"items"`
	Payments      []salePaymentOut `json:"payments"`
}

// httpFail es un error con el estado HTTP y el mensaje que debe ver quien cobra.
type httpFail struct {
	Status int
	Msg    string
	Code   string
}

func (e *httpFail) Error() string { return e.Msg }

func fail(status int, msg string) *httpFail { return &httpFail{Status: status, Msg: msg} }

func writeFail(w http.ResponseWriter, err error) bool {
	var f *httpFail
	if errors.As(err, &f) {
		body := map[string]string{"error": f.Msg}
		if f.Code != "" {
			body["code"] = f.Code
		}
		writeJSON(w, f.Status, body)
		return true
	}
	return false
}

type pricedLine struct {
	in                     saleLineIn
	kind, name             string
	qty, unit, cost        int
	gross, discount, total int
	staffID                *int64
	pct                    float64
	base, commission       int
}

func isActiveStaff(ctx context.Context, q queryer, bizID int64, id int64) (bool, float64, error) {
	var def float64
	err := q.QueryRow(ctx,
		`SELECT commission_pct::float8 FROM users WHERE id = $1 AND business_id = $2 AND active`, id, bizID).Scan(&def)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, 0, nil
	}
	return err == nil, def, err
}

func (a *API) createSale(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req saleIn
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	req.ClientToken = strings.TrimSpace(req.ClientToken)
	switch {
	case len(req.Lines) == 0 || len(req.Lines) > 50:
		writeErr(w, http.StatusBadRequest, "agrega de 1 a 50 partidas")
		return
	case req.DiscountCents < 0 || req.TipCents < 0 || req.TipCents > maxPriceCents || len(req.Note) > 500 || len(req.ClientToken) > 80,
		len(req.Payments) > 6:
		writeErr(w, http.StatusBadRequest, "descuento, propina, nota o pagos inválidos")
		return
	}

	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)

	// misma petición repetida (doble clic, reintento de red): devuelve la venta ya creada
	if req.ClientToken != "" {
		var existing int64
		err := tx.QueryRow(ctx, `SELECT id FROM sales WHERE business_id = $1 AND client_token = $2`, p.BusinessID, req.ClientToken).Scan(&existing)
		if err == nil {
			s, err := loadSale(ctx, tx, p.BusinessID, existing)
			if err != nil {
				serverErr(w)
				return
			}
			writeJSON(w, http.StatusOK, s)
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			serverErr(w)
			return
		}
	}

	saleID, err := a.buildSale(ctx, tx, p, req)
	if err != nil {
		if !writeFail(w, err) {
			if code, c := pgConstraint(err); code == "23505" && c == "sales_one_paid_per_appointment" {
				writeErr(w, http.StatusConflict, "esa cita ya fue cobrada")
				return
			}
			serverErr(w)
		}
		return
	}
	if err := tx.Commit(ctx); err != nil {
		if code, c := pgConstraint(err); code == "23505" && c == "sales_one_paid_per_appointment" {
			writeErr(w, http.StatusConflict, "esa cita ya fue cobrada")
			return
		}
		serverErr(w)
		return
	}
	s, err := loadSale(ctx, a.DB, p.BusinessID, saleID)
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

// buildSale valida, calcula y guarda la venta dentro de la transacción recibida.
func (a *API) buildSale(ctx context.Context, tx pgx.Tx, p *auth.Principal, req saleIn) (int64, error) {
	bizID := p.BusinessID

	ok, err := activeBranch(ctx, tx, bizID, req.BranchID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fail(http.StatusBadRequest, "sucursal inválida")
	}
	sessionID, err := openSessionID(ctx, tx, bizID, req.BranchID)
	if err != nil {
		return 0, err
	}
	if sessionID == 0 {
		return 0, &httpFail{http.StatusConflict, "abre la caja para poder cobrar", "cash_closed"}
	}

	// cita (opcional): se cobra una sola vez y queda completada
	var apptStaff *int64
	var apptStatus string
	clientID := req.ClientID
	if req.AppointmentID != nil {
		var apptClient, apptStaffID, apptBranch int64
		err := tx.QueryRow(ctx,
			`SELECT status, client_id, staff_id, branch_id FROM appointments WHERE id = $1 AND business_id = $2 FOR UPDATE`,
			*req.AppointmentID, bizID).Scan(&apptStatus, &apptClient, &apptStaffID, &apptBranch)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fail(http.StatusBadRequest, "cita inválida")
		}
		if err != nil {
			return 0, err
		}
		if apptStatus != "scheduled" && apptStatus != "completed" {
			return 0, fail(http.StatusConflict, "no se puede cobrar esta cita: está cancelada, no asistieron o espera su anticipo")
		}
		if apptBranch != req.BranchID {
			return 0, fail(http.StatusBadRequest, "la cita es de otra sucursal")
		}
		var charged bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sales WHERE appointment_id = $1 AND status = 'paid')`, *req.AppointmentID).Scan(&charged); err != nil {
			return 0, err
		}
		if charged {
			return 0, fail(http.StatusConflict, "esa cita ya fue cobrada")
		}
		apptStaff = &apptStaffID
		if clientID == nil {
			clientID = &apptClient
		}
	}
	if clientID != nil {
		var okClient bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM clients WHERE id = $1 AND business_id = $2)`, *clientID, bizID).Scan(&okClient); err != nil {
			return 0, err
		}
		if !okClient {
			return 0, fail(http.StatusBadRequest, "cliente inválido")
		}
	}

	// ----- partidas -----
	rules, err := a.loadRules(ctx, tx, bizID)
	if err != nil {
		return 0, err
	}
	lines := make([]pricedLine, 0, len(req.Lines))
	staffDefaults := map[int64]float64{}
	checkStaff := func(id *int64) error {
		if id == nil {
			return nil
		}
		if _, seen := staffDefaults[*id]; seen {
			return nil
		}
		ok, def, err := isActiveStaff(ctx, tx, bizID, *id)
		if err != nil {
			return err
		}
		if !ok {
			return fail(http.StatusBadRequest, "persona inválida en la venta")
		}
		staffDefaults[*id] = def
		return nil
	}

	subtotal := 0
	for _, in := range req.Lines {
		if (in.ServiceID == nil) == (in.ProductID == nil) {
			return 0, fail(http.StatusBadRequest, "cada partida debe ser un servicio o un producto")
		}
		l := pricedLine{in: in, qty: in.Qty, staffID: in.StaffID}
		maxQty := 1000
		if in.ServiceID != nil {
			maxQty = 20
		}
		if l.qty < 1 || l.qty > maxQty || in.DiscountCents < 0 {
			return 0, fail(http.StatusBadRequest, "cantidad o descuento inválidos")
		}
		if in.UnitPriceCents != nil {
			if p.Role != "admin" {
				return 0, fail(http.StatusForbidden, "solo el administrador puede cambiar un precio")
			}
			if *in.UnitPriceCents < 0 || *in.UnitPriceCents > maxPriceCents {
				return 0, fail(http.StatusBadRequest, "precio inválido")
			}
		}
		if in.ServiceID != nil {
			l.kind = "service"
			err := tx.QueryRow(ctx, `SELECT name, price_cents FROM services WHERE id = $1 AND business_id = $2 AND active`, *in.ServiceID, bizID).Scan(&l.name, &l.unit)
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, fail(http.StatusBadRequest, "servicio inválido o inactivo")
			}
			if err != nil {
				return 0, err
			}
			if l.staffID == nil {
				l.staffID = apptStaff
			}
		} else {
			l.kind = "product"
			err := tx.QueryRow(ctx, `SELECT name, price_cents, cost_cents FROM products WHERE id = $1 AND business_id = $2 AND active`, *in.ProductID, bizID).Scan(&l.name, &l.unit, &l.cost)
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, fail(http.StatusBadRequest, "producto inválido o inactivo")
			}
			if err != nil {
				return 0, err
			}
		}
		if in.UnitPriceCents != nil {
			l.unit = *in.UnitPriceCents
		}
		if err := checkStaff(l.staffID); err != nil {
			return 0, err
		}
		l.gross = l.qty * l.unit
		if l.gross > maxPriceCents || in.DiscountCents > l.gross {
			return 0, fail(http.StatusBadRequest, "el importe de una partida es inválido o el descuento lo excede")
		}
		l.discount, l.total = in.DiscountCents, l.gross-in.DiscountCents
		subtotal += l.total
		lines = append(lines, l)
	}
	if subtotal > maxPriceCents {
		return 0, fail(http.StatusBadRequest, "el monto de la venta es demasiado grande")
	}
	if req.DiscountCents > subtotal {
		return 0, fail(http.StatusBadRequest, "el descuento no puede ser mayor al subtotal")
	}
	total := subtotal - req.DiscountCents

	// comisiones sobre lo realmente cobrado (después de descuentos)
	parts := make([]int, len(lines))
	for i, l := range lines {
		parts[i] = l.total
	}
	shares := allocateDiscount(parts, req.DiscountCents)
	for i := range lines {
		l := &lines[i]
		l.base = l.total - shares[i]
		l.pct = pickPercent(rules, l.staffID, l.in.ServiceID, l.in.ProductID, staffDefaults[derefID(l.staffID)])
		l.commission = commissionOf(l.base, l.pct)
	}

	// propina: a la persona indicada o, si todos los servicios son de la misma, a ella
	tipStaff := req.TipStaffID
	if tipStaff == nil && req.TipCents > 0 {
		tipStaff = soleStaff(lines)
	}
	if req.TipCents == 0 {
		tipStaff = nil
	}
	if err := checkStaff(tipStaff); err != nil {
		return 0, err
	}

	// ----- pagos: deben cubrir total + propina exactamente -----
	payTotal := 0
	for _, pay := range req.Payments {
		if pay.AmountCents <= 0 || pay.AmountCents > maxPriceCents {
			return 0, fail(http.StatusBadRequest, "monto de pago inválido")
		}
		payTotal += pay.AmountCents
	}
	if payTotal != total+req.TipCents {
		return 0, fail(http.StatusBadRequest, fmt.Sprintf("los pagos ($%s) deben cubrir exactamente el total más la propina ($%s)", cents(payTotal), cents(total+req.TipCents)))
	}
	seenCharges := map[string]bool{}
	var depositID int64
	for i := range req.Payments {
		pay := &req.Payments[i]
		switch pay.Method {
		case "cash":
			if pay.ReceivedCents != nil && (*pay.ReceivedCents < pay.AmountCents || *pay.ReceivedCents > maxPriceCents) {
				return 0, fail(http.StatusBadRequest, "el efectivo recibido no alcanza para ese pago")
			}
		case "card", "transfer":
			if pay.ReceivedCents != nil {
				return 0, fail(http.StatusBadRequest, "solo el pago en efectivo lleva monto recibido")
			}
		case "mp_point":
			if pay.ReceivedCents != nil || pay.MPChargeID == "" || seenCharges[pay.MPChargeID] {
				return 0, fail(http.StatusBadRequest, "el pago con terminal necesita su cobro aprobado (uno por pago)")
			}
			seenCharges[pay.MPChargeID] = true
		case "deposit":
			// el anticipo pagado en línea se descuenta una sola vez, por su monto exacto
			if req.AppointmentID == nil || depositID != 0 || pay.ReceivedCents != nil || pay.MPChargeID != "" {
				return 0, fail(http.StatusBadRequest, "el anticipo se aplica una vez y solo a una cita")
			}
			var depAmount int
			var depStatus string
			err := tx.QueryRow(ctx,
				`SELECT id, amount_cents, status FROM deposits WHERE appointment_id = $1 AND business_id = $2 FOR UPDATE`,
				*req.AppointmentID, bizID).Scan(&depositID, &depAmount, &depStatus)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && depStatus != "paid") {
				return 0, fail(http.StatusConflict, "esa cita no tiene un anticipo pagado disponible")
			}
			if err != nil {
				return 0, err
			}
			if pay.AmountCents != depAmount {
				return 0, fail(http.StatusBadRequest, fmt.Sprintf("el anticipo pagado es de $%s", cents(depAmount)))
			}
			if depAmount > total {
				return 0, fail(http.StatusConflict, fmt.Sprintf("el anticipo ($%s) es mayor que la cuenta ($%s); cancela la cita para devolverlo o ajusta la cuenta", cents(depAmount), cents(total)))
			}
		default:
			return 0, fail(http.StatusBadRequest, "método de pago inválido")
		}
		if len(pay.Reference) > 100 {
			return 0, fail(http.StatusBadRequest, "referencia demasiado larga")
		}
	}

	// un anticipo pagado no puede quedarse sin aplicar: se cobraría dos veces al cliente
	if req.AppointmentID != nil && depositID == 0 {
		var pending int
		err := tx.QueryRow(ctx, `SELECT amount_cents FROM deposits WHERE appointment_id = $1 AND business_id = $2 AND status = 'paid' FOR UPDATE`,
			*req.AppointmentID, bizID).Scan(&pending)
		if err == nil {
			return 0, fail(http.StatusConflict, fmt.Sprintf("la cita tiene un anticipo pagado de $%s: aplícalo como forma de pago", cents(pending)))
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, err
		}
	}

	// ----- guardar -----
	var folio int
	if err := tx.QueryRow(ctx, `UPDATE businesses SET sale_seq = sale_seq + 1 WHERE id = $1 RETURNING sale_seq`, bizID).Scan(&folio); err != nil {
		return 0, err
	}
	var saleID int64
	var token *string
	if req.ClientToken != "" {
		token = &req.ClientToken
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO sales (business_id, branch_id, cash_session_id, client_id, appointment_id, cashier_id, folio,
		   subtotal_cents, discount_cents, total_cents, tip_cents, tip_staff_id, note, client_token, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id`,
		bizID, req.BranchID, sessionID, clientID, req.AppointmentID, p.UserID, folio,
		subtotal, req.DiscountCents, total, req.TipCents, tipStaff, req.Note, token, a.now()).Scan(&saleID); err != nil {
		return 0, err
	}
	for _, l := range lines {
		if _, err := tx.Exec(ctx,
			`INSERT INTO sale_items (sale_id, service_id, product_id, staff_id, qty, unit_price_cents, unit_cost_cents, discount_cents,
			   total_cents, name, commission_pct, commission_base_cents, commission_cents)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			saleID, l.in.ServiceID, l.in.ProductID, l.staffID, l.qty, l.unit, l.cost, l.discount, l.total, l.name, l.pct, l.base, l.commission); err != nil {
			return 0, err
		}
	}
	for _, pay := range req.Payments {
		received, change := pay.ReceivedCents, 0
		if pay.Method == "cash" {
			if received == nil {
				r := pay.AmountCents
				received = &r
			}
			change = *received - pay.AmountCents
		}
		var chargeID *string
		if pay.Method == "mp_point" {
			if err := claimMPCharge(ctx, tx, bizID, req.BranchID, pay.MPChargeID, pay.AmountCents, saleID); err != nil {
				return 0, err
			}
			chargeID = &pay.MPChargeID
		}
		var depID *int64
		if pay.Method == "deposit" {
			depID = &depositID
			if _, err := tx.Exec(ctx, `UPDATE deposits SET status = 'applied', sale_id = $2 WHERE id = $1 AND status = 'paid'`, depositID, saleID); err != nil {
				return 0, err
			}
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO sale_payments (sale_id, method, amount_cents, received_cents, change_cents, reference, cash_session_id, mp_charge_id, deposit_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			saleID, pay.Method, pay.AmountCents, received, change, strings.TrimSpace(pay.Reference), sessionID, chargeID, depID); err != nil {
			return 0, err
		}
	}

	// existencia: en orden de producto para no provocar bloqueos cruzados entre ventas simultáneas
	type take struct {
		id   int64
		qty  int
		name string
	}
	byProduct := map[int64]*take{}
	for _, l := range lines {
		if l.in.ProductID != nil {
			t := byProduct[*l.in.ProductID]
			if t == nil {
				t = &take{id: *l.in.ProductID, name: l.name}
				byProduct[t.id] = t
			}
			t.qty += l.qty
		}
	}
	takes := make([]*take, 0, len(byProduct))
	for _, t := range byProduct {
		takes = append(takes, t)
	}
	sort.Slice(takes, func(i, j int) bool { return takes[i].id < takes[j].id })
	for _, t := range takes {
		if _, err := applyStock(ctx, tx, bizID, t.id, -t.qty, "sale", &saleID, "Venta #"+strconv.Itoa(folio), p.UserID); err != nil {
			if errors.Is(err, errInsufficientStock) {
				return 0, fail(http.StatusConflict, "no hay suficiente existencia de «"+t.name+"»")
			}
			return 0, err
		}
	}
	if req.AppointmentID != nil && apptStatus == "scheduled" {
		if _, err := tx.Exec(ctx, `UPDATE appointments SET status = 'completed' WHERE id = $1`, *req.AppointmentID); err != nil {
			return 0, err
		}
	}
	return saleID, nil
}

func derefID(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// soleStaff devuelve la persona de los servicios si todos son de la misma.
func soleStaff(lines []pricedLine) *int64 {
	var who *int64
	for _, l := range lines {
		if l.kind != "service" {
			continue
		}
		if l.staffID == nil || (who != nil && *who != *l.staffID) {
			return nil
		}
		who = l.staffID
	}
	return who
}

// ---------- consulta ----------

func loadSale(ctx context.Context, q queryer, bizID, id int64) (*saleOut, error) {
	var s saleOut
	err := q.QueryRow(ctx,
		`SELECT s.id, s.folio, s.branch_id, s.client_id, COALESCE(c.name, ''), s.appointment_id, COALESCE(cu.name, ''),
		        s.subtotal_cents, s.discount_cents, s.total_cents, s.tip_cents, s.tip_staff_id, COALESCE(tu.name, ''),
		        s.note, s.status, s.void_reason, COALESCE(vu.name, ''), s.created_at
		 FROM sales s LEFT JOIN clients c ON c.id = s.client_id LEFT JOIN users cu ON cu.id = s.cashier_id
		 LEFT JOIN users tu ON tu.id = s.tip_staff_id LEFT JOIN users vu ON vu.id = s.voided_by
		 WHERE s.id = $1 AND s.business_id = $2`, id, bizID).
		Scan(&s.ID, &s.Folio, &s.BranchID, &s.ClientID, &s.ClientName, &s.AppointmentID, &s.Cashier,
			&s.SubtotalCents, &s.DiscountCents, &s.TotalCents, &s.TipCents, &s.TipStaffID, &s.TipStaffName,
			&s.Note, &s.Status, &s.VoidReason, &s.VoidedBy, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	s.PaidCents = s.TotalCents + s.TipCents

	rows, err := q.Query(ctx,
		`SELECT i.id, CASE WHEN i.service_id IS NOT NULL THEN 'service' ELSE 'product' END, i.name, i.qty, i.unit_price_cents,
		        i.discount_cents, i.total_cents, i.staff_id, COALESCE(u.name, ''), i.commission_pct::float8, i.commission_cents
		 FROM sale_items i LEFT JOIN users u ON u.id = i.staff_id WHERE i.sale_id = $1 ORDER BY i.id`, id)
	if err != nil {
		return nil, err
	}
	s.Items = []saleItemOut{}
	for rows.Next() {
		var it saleItemOut
		if err := rows.Scan(&it.ID, &it.Kind, &it.Name, &it.Qty, &it.UnitPriceCents, &it.DiscountCents, &it.TotalCents,
			&it.StaffID, &it.StaffName, &it.CommissionPct, &it.CommissionCents); err != nil {
			rows.Close()
			return nil, err
		}
		s.Items = append(s.Items, it)
	}
	rows.Close()

	prows, err := q.Query(ctx,
		`SELECT method, amount_cents, received_cents, change_cents, reference, COALESCE(mp_charge_id, '') FROM sale_payments WHERE sale_id = $1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	s.Payments = []salePaymentOut{}
	for prows.Next() {
		var pay salePaymentOut
		if err := prows.Scan(&pay.Method, &pay.AmountCents, &pay.ReceivedCents, &pay.ChangeCents, &pay.Reference, &pay.MPChargeID); err != nil {
			return nil, err
		}
		s.ChangeCents += pay.ChangeCents
		s.Payments = append(s.Payments, pay)
	}
	return &s, nil
}

func (a *API) getSale(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s, err := loadSale(r.Context(), a.DB, p.BusinessID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

type saleRow struct {
	ID         int64     `json:"id"`
	Folio      int       `json:"folio"`
	CreatedAt  time.Time `json:"created_at"`
	ClientName string    `json:"client_name"`
	Cashier    string    `json:"cashier"`
	TotalCents int       `json:"total_cents"`
	TipCents   int       `json:"tip_cents"`
	Status     string    `json:"status"`
	Methods    []string  `json:"methods"`
}

// dateRange convierte from/to (AAAA-MM-DD, inclusivos, hora del negocio) en [inicio, fin).
// Sin parámetros usa el día de hoy.
func (a *API) dateRange(ctx context.Context, bizID int64, q map[string][]string) (time.Time, time.Time, string, string, bool) {
	loc := a.businessLoc(ctx, bizID)
	get := func(k string) string {
		if v := q[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	today := a.now().In(loc).Format(dateLayout)
	from, to := get("from"), get("to")
	if from == "" {
		from = today
	}
	if to == "" {
		to = from
	}
	f, err1 := time.ParseInLocation(dateLayout, from, loc)
	t, err2 := time.ParseInLocation(dateLayout, to, loc)
	if err1 != nil || err2 != nil || t.Before(f) || t.Sub(f) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, "", "", false
	}
	return f, t.AddDate(0, 0, 1), from, to, true
}

func (a *API) listSales(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	from, to, _, _, ok := a.dateRange(r.Context(), p.BusinessID, r.URL.Query())
	if !ok {
		writeErr(w, http.StatusBadRequest, "rango de fechas inválido (AAAA-MM-DD, hasta un año)")
		return
	}
	branchID, _ := strconv.ParseInt(r.URL.Query().Get("branch_id"), 10, 64)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	folio, _ := strconv.Atoi(q)
	rows, err := a.DB.Query(r.Context(),
		`SELECT s.id, s.folio, s.created_at, COALESCE(c.name, ''), COALESCE(u.name, ''), s.total_cents, s.tip_cents, s.status,
		        COALESCE((SELECT array_agg(DISTINCT sp.method ORDER BY sp.method) FROM sale_payments sp WHERE sp.sale_id = s.id), '{}')
		 FROM sales s LEFT JOIN clients c ON c.id = s.client_id LEFT JOIN users u ON u.id = s.cashier_id
		 WHERE s.business_id = $1 AND s.created_at >= $2 AND s.created_at < $3 AND ($4 = 0 OR s.branch_id = $4)
		   AND ($5 = '' OR s.folio = $6 OR strpos(lower(COALESCE(c.name, '')), lower($5)) > 0)
		 ORDER BY s.created_at DESC, s.id DESC LIMIT 300`, p.BusinessID, from, to, branchID, q, folio)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []saleRow{}
	for rows.Next() {
		var x saleRow
		if err := rows.Scan(&x.ID, &x.Folio, &x.CreatedAt, &x.ClientName, &x.Cashier, &x.TotalCents, &x.TipCents, &x.Status, &x.Methods); err != nil {
			serverErr(w)
			return
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- cancelación ----------

// voidSale cancela una venta (solo administrador): devuelve la existencia y los cobros con terminal.
// El efectivo de una caja ya cerrada se devuelve con una salida en la caja abierta; si la venta es de
// la caja abierta, simplemente deja de contar.
func (a *API) voidSale(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if n := utf8.RuneCountInString(req.Reason); n < 3 || n > 300 {
		writeErr(w, http.StatusBadRequest, "indica el motivo de la cancelación (3 a 300 caracteres)")
		return
	}
	ctx := r.Context()

	var status string
	var branchID int64
	var folio int
	err := a.DB.QueryRow(ctx, `SELECT status, branch_id, folio FROM sales WHERE id = $1 AND business_id = $2`, id, p.BusinessID).Scan(&status, &branchID, &folio)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	if status != "paid" {
		writeErr(w, http.StatusConflict, "la venta ya estaba cancelada")
		return
	}
	// primero se devuelve el dinero de la terminal (idempotente); si falla, la venta no se cancela
	if err := a.refundMPCharges(ctx, p.BusinessID, id); err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}
	// y el anticipo que se había descontado en esta venta
	if err := a.refundAppliedDeposit(ctx, p.BusinessID, id); err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}

	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx, `SELECT status FROM sales WHERE id = $1 FOR UPDATE`, id).Scan(&status); err != nil {
		serverErr(w)
		return
	}
	if status != "paid" {
		writeErr(w, http.StatusConflict, "la venta ya estaba cancelada")
		return
	}

	// efectivo recibido en cajas ya cerradas: sale de la caja abierta
	current, err := openSessionID(ctx, tx, p.BusinessID, branchID)
	if err != nil {
		serverErr(w)
		return
	}
	var cashFromClosed int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(sum(amount_cents), 0) FROM sale_payments WHERE sale_id = $1 AND method = 'cash'
		   AND cash_session_id IS DISTINCT FROM NULLIF($2, 0)`, id, current).Scan(&cashFromClosed); err != nil {
		serverErr(w)
		return
	}
	if cashFromClosed > 0 {
		if current == 0 {
			writeErr(w, http.StatusConflict, "abre la caja para registrar la devolución del efectivo")
			return
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO cash_movements (cash_session_id, kind, amount_cents, concept, created_by) VALUES ($1, 'out', $2, $3, $4)`,
			current, cashFromClosed, fmt.Sprintf("Cancelación de la venta #%d", folio), p.UserID); err != nil {
			serverErr(w)
			return
		}
	}

	// existencia de vuelta, en orden de producto
	rows, err := tx.Query(ctx,
		`SELECT product_id, sum(qty) FROM sale_items WHERE sale_id = $1 AND product_id IS NOT NULL GROUP BY product_id ORDER BY product_id`, id)
	if err != nil {
		serverErr(w)
		return
	}
	type back struct {
		id  int64
		qty int
	}
	var backs []back
	for rows.Next() {
		var b back
		if err := rows.Scan(&b.id, &b.qty); err != nil {
			rows.Close()
			serverErr(w)
			return
		}
		backs = append(backs, b)
	}
	rows.Close()
	for _, b := range backs {
		if _, err := applyStock(ctx, tx, p.BusinessID, b.id, b.qty, "void", &id, fmt.Sprintf("Cancelación de la venta #%d", folio), p.UserID); err != nil {
			serverErr(w)
			return
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE sales SET status = 'void', void_reason = $2, voided_at = now(), voided_by = $3 WHERE id = $1`, id, req.Reason, p.UserID); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	s, err := loadSale(ctx, a.DB, p.BusinessID, id)
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, s)
}
