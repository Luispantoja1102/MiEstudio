package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

type packageOut struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ServiceID   int64  `json:"service_id"`
	ServiceName string `json:"service_name"`
	Sessions    int    `json:"sessions"`
	PriceCents  int    `json:"price_cents"`
	ValidDays   *int   `json:"valid_days"`
	Active      bool   `json:"active"`
}

type clientPackageOut struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	ServiceID     int64      `json:"service_id"`
	SessionsTotal int        `json:"sessions_total"`
	SessionsUsed  int        `json:"sessions_used"`
	Remaining     int        `json:"remaining"`
	ExpiresAt     *time.Time `json:"expires_at"`
	Status        string     `json:"status"` // active, exhausted, expired, void
	SaleID        int64      `json:"sale_id"`
	CreatedAt     time.Time  `json:"created_at"`
}

func validPackage(name string, sessions, price int, validDays *int) string {
	switch {
	case !validName(name):
		return "el nombre es obligatorio"
	case sessions < 2 || sessions > 100:
		return "las sesiones deben estar entre 2 y 100"
	case price < 0 || price > maxPriceCents:
		return "precio inválido"
	case validDays != nil && (*validDays < 1 || *validDays > 3650):
		return "la vigencia debe estar entre 1 y 3650 días"
	}
	return ""
}

func (a *API) listPackages(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	rows, err := a.DB.Query(r.Context(),
		`SELECT k.id, k.name, k.service_id, s.name, k.sessions, k.price_cents, k.valid_days, k.active
		 FROM packages k JOIN services s ON s.id = k.service_id
		 WHERE k.business_id = $1 AND ($2 OR k.active) ORDER BY k.name, k.id`, p.BusinessID, p.Role == "admin" && r.URL.Query().Get("all") == "1")
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []packageOut{}
	for rows.Next() {
		var k packageOut
		if err := rows.Scan(&k.ID, &k.Name, &k.ServiceID, &k.ServiceName, &k.Sessions, &k.PriceCents, &k.ValidDays, &k.Active); err != nil {
			serverErr(w)
			return
		}
		out = append(out, k)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createPackage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Name       string `json:"name"`
		ServiceID  int64  `json:"service_id"`
		Sessions   int    `json:"sessions"`
		PriceCents int    `json:"price_cents"`
		ValidDays  *int   `json:"valid_days"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if msg := validPackage(req.Name, req.Sessions, req.PriceCents, req.ValidDays); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	var ok bool
	if err := a.DB.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM services WHERE id = $1 AND business_id = $2)`, req.ServiceID, p.BusinessID).Scan(&ok); err != nil {
		serverErr(w)
		return
	}
	if !ok {
		writeErr(w, http.StatusBadRequest, "servicio inválido")
		return
	}
	var id int64
	if err := a.DB.QueryRow(r.Context(),
		`INSERT INTO packages (business_id, name, service_id, sessions, price_cents, valid_days) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		p.BusinessID, req.Name, req.ServiceID, req.Sessions, req.PriceCents, req.ValidDays).Scan(&id); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// updatePackage cambia nombre, precio, vigencia o estado. El servicio y las sesiones no se
// editan: los saldos ya vendidos conservan su propia copia, y para otra oferta se crea otro paquete.
func (a *API) updatePackage(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name       *string `json:"name"`
		PriceCents *int    `json:"price_cents"`
		ValidDays  *int    `json:"valid_days"` // 0 = sin vigencia
		Active     *bool   `json:"active"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	name, price := "x", 0
	if req.Name != nil {
		t := strings.TrimSpace(*req.Name)
		req.Name, name = &t, t
	}
	if req.PriceCents != nil {
		price = *req.PriceCents
	}
	var days *int
	if req.ValidDays != nil && *req.ValidDays != 0 {
		days = req.ValidDays
	}
	if msg := validPackage(name, 2, price, days); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	tag, err := a.DB.Exec(r.Context(),
		`UPDATE packages SET name = COALESCE($3, name), price_cents = COALESCE($4, price_cents),
		   valid_days = CASE WHEN $5::int IS NULL THEN valid_days WHEN $5 = 0 THEN NULL ELSE $5 END,
		   active = COALESCE($6, active)
		 WHERE id = $1 AND business_id = $2`, id, p.BusinessID, req.Name, req.PriceCents, req.ValidDays, req.Active)
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

func packageStatus(total, used int, expires, voided *time.Time, now time.Time) string {
	switch {
	case voided != nil:
		return "void"
	case used >= total:
		return "exhausted"
	case expires != nil && !expires.After(now):
		return "expired"
	}
	return "active"
}

// listClientPackages devuelve los paquetes de un cliente con su saldo (los vigentes primero).
func (a *API) listClientPackages(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(),
		`SELECT id, name, service_id, sessions_total, sessions_used, expires_at, voided_at, sale_id, created_at
		 FROM client_packages WHERE business_id = $1 AND client_id = $2 ORDER BY created_at DESC, id DESC LIMIT 100`, p.BusinessID, id)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	now := a.now()
	out := []clientPackageOut{}
	for rows.Next() {
		var c clientPackageOut
		var voided *time.Time
		if err := rows.Scan(&c.ID, &c.Name, &c.ServiceID, &c.SessionsTotal, &c.SessionsUsed, &c.ExpiresAt, &voided, &c.SaleID, &c.CreatedAt); err != nil {
			serverErr(w)
			return
		}
		c.Remaining = c.SessionsTotal - c.SessionsUsed
		c.Status = packageStatus(c.SessionsTotal, c.SessionsUsed, c.ExpiresAt, voided, now)
		if c.Status != "active" {
			c.Remaining = 0
		}
		out = append(out, c)
	}
	// vigentes primero, sin perder el orden de compra dentro de cada grupo
	sorted := make([]clientPackageOut, 0, len(out))
	for _, c := range out {
		if c.Status == "active" {
			sorted = append(sorted, c)
		}
	}
	for _, c := range out {
		if c.Status != "active" {
			sorted = append(sorted, c)
		}
	}
	writeJSON(w, http.StatusOK, sorted)
}

// redeemPackage valida y bloquea el paquete que paga una sesión. Devuelve el servicio que cubre.
// uses cuenta los canjes ya pedidos en la misma venta.
func redeemPackage(ctx context.Context, tx pgx.Tx, bizID int64, clientID *int64, cpID int64, uses map[int64]int, now time.Time) (serviceID int64, err error) {
	if clientID == nil {
		return 0, fail(http.StatusBadRequest, "para canjear un paquete la venta necesita al cliente")
	}
	var owner int64
	var total, used int
	var expires, voided *time.Time
	err = tx.QueryRow(ctx,
		`SELECT client_id, service_id, sessions_total, sessions_used, expires_at, voided_at
		 FROM client_packages WHERE id = $1 AND business_id = $2 FOR UPDATE`, cpID, bizID).
		Scan(&owner, &serviceID, &total, &used, &expires, &voided)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != *clientID) {
		return 0, fail(http.StatusBadRequest, "paquete inválido para este cliente")
	}
	if err != nil {
		return 0, err
	}
	switch packageStatus(total, used+uses[cpID], expires, voided, now) {
	case "void":
		return 0, fail(http.StatusConflict, "ese paquete fue cancelado")
	case "exhausted":
		return 0, fail(http.StatusConflict, "ese paquete ya no tiene sesiones disponibles")
	case "expired":
		return 0, fail(http.StatusConflict, "ese paquete está vencido")
	}
	uses[cpID]++
	return serviceID, nil
}

// grantPackage crea el saldo del cliente al venderle un paquete.
func grantPackage(ctx context.Context, tx pgx.Tx, bizID, clientID, saleID, packageID int64, now time.Time) error {
	var name string
	var serviceID int64
	var sessions int
	var days *int
	if err := tx.QueryRow(ctx, `SELECT name, service_id, sessions, valid_days FROM packages WHERE id = $1 AND business_id = $2`, packageID, bizID).
		Scan(&name, &serviceID, &sessions, &days); err != nil {
		return err
	}
	var expires *time.Time
	if days != nil {
		t := now.AddDate(0, 0, *days)
		expires = &t
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO client_packages (business_id, client_id, package_id, sale_id, name, service_id, sessions_total, expires_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, bizID, clientID, packageID, saleID, name, serviceID, sessions, expires, now)
	return err
}

// undoPackagesOfSale deshace los efectos de una venta cancelada: devuelve las sesiones canjeadas y
// anula los paquetes que esa venta vendió (si ya se usaron, la cancelación se rechaza).
func undoPackagesOfSale(ctx context.Context, tx pgx.Tx, saleID int64) error {
	if _, err := tx.Exec(ctx, `SELECT id FROM client_packages WHERE sale_id = $1 FOR UPDATE`, saleID); err != nil {
		return err
	}
	var used int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(sessions_used), 0) FROM client_packages WHERE sale_id = $1 AND voided_at IS NULL`, saleID).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return fail(http.StatusConflict, fmt.Sprintf("el paquete de esta venta ya tiene %d sesión(es) canjeada(s); cancela primero esas ventas", used))
	}
	if _, err := tx.Exec(ctx, `UPDATE client_packages SET voided_at = now() WHERE sale_id = $1 AND voided_at IS NULL`, saleID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx,
		`UPDATE client_packages c SET sessions_used = c.sessions_used - r.n
		 FROM (SELECT client_package_id, count(*)::int AS n FROM sale_payments WHERE sale_id = $1 AND client_package_id IS NOT NULL GROUP BY client_package_id) r
		 WHERE c.id = r.client_package_id`, saleID)
	return err
}
