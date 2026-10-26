package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

// Tarjeta de lealtad (sellos). Cada venta cobrada a un cliente con al menos un servicio da un sello, uno por cliente
// por día. Al juntar los que pide el negocio se canjea el premio. Cada cliente puede tener un enlace privado para ver
// su tarjeta; el enlace solo muestra su primer nombre y sus sellos.

type loyaltySettings struct {
	Enabled        bool   `json:"enabled"`
	VisitsRequired int    `json:"visits_required"`
	Reward         string `json:"reward"`
}

func (a *API) loadLoyaltySettings(ctx context.Context, q queryer, bizID int64) (loyaltySettings, error) {
	s := loyaltySettings{VisitsRequired: 10}
	err := q.QueryRow(ctx, `SELECT enabled, visits_required, reward FROM loyalty_settings WHERE business_id = $1`, bizID).Scan(&s.Enabled, &s.VisitsRequired, &s.Reward)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, nil // sin configurar: apagada
	}
	return s, err
}

func (a *API) getLoyaltySettings(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	s, err := a.loadLoyaltySettings(r.Context(), a.DB, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *API) putLoyaltySettings(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req loyaltySettings
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	req.Reward = strings.TrimSpace(req.Reward)
	switch {
	case req.VisitsRequired < 2 || req.VisitsRequired > 50:
		writeErr(w, http.StatusBadRequest, "las visitas para el premio deben estar entre 2 y 50")
		return
	case utf8.RuneCountInString(req.Reward) > 80:
		writeErr(w, http.StatusBadRequest, "el premio puede tener hasta 80 caracteres")
		return
	case req.Enabled && req.Reward == "":
		writeErr(w, http.StatusBadRequest, "indica cuál es el premio")
		return
	}
	if _, err := a.DB.Exec(r.Context(),
		`INSERT INTO loyalty_settings (business_id, enabled, visits_required, reward) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (business_id) DO UPDATE SET enabled = EXCLUDED.enabled, visits_required = EXCLUDED.visits_required, reward = EXCLUDED.reward, updated_at = now()`,
		p.BusinessID, req.Enabled, req.VisitsRequired, req.Reward); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

// awardStamp da el sello de una venta recién registrada (dentro de su misma transacción). No hace nada si la tarjeta está
// apagada, la venta no tiene cliente ni servicios, o el cliente ya tiene su sello de ese día.
func (a *API) awardStamp(ctx context.Context, tx pgx.Tx, saleID int64) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO loyalty_stamps (business_id, client_id, sale_id, day)
		 SELECT s.business_id, s.client_id, s.id, (s.created_at AT TIME ZONE b.timezone)::date
		   FROM sales s JOIN businesses b ON b.id = s.business_id JOIN loyalty_settings l ON l.business_id = s.business_id AND l.enabled
		  WHERE s.id = $1 AND s.client_id IS NOT NULL AND EXISTS (SELECT 1 FROM sale_items i WHERE i.sale_id = s.id AND i.service_id IS NOT NULL)
		 ON CONFLICT DO NOTHING`, saleID)
	return err
}

type loyaltyVisit struct {
	Day      string `json:"day"`
	Redeemed bool   `json:"redeemed"`
}

type loyaltyCard struct {
	loyaltySettings
	Stamps   int            `json:"stamps"`   // sellos sin canjear
	Ready    bool           `json:"ready"`    // ya completó la tarjeta
	Redeemed int            `json:"redeemed"` // premios canjeados
	Path     string         `json:"path"`     // enlace de su tarjeta virtual ("" si no tiene)
	History  []loyaltyVisit `json:"history"`
}

// loadCard arma la tarjeta de un cliente. El cliente debe ser del negocio (o lo da por hecho quien llama).
func (a *API) loadCard(ctx context.Context, q queryer, bizID, clientID int64) (*loyaltyCard, error) {
	s, err := a.loadLoyaltySettings(ctx, q, bizID)
	if err != nil {
		return nil, err
	}
	c := &loyaltyCard{loyaltySettings: s, History: []loyaltyVisit{}}
	var token *string
	if err := q.QueryRow(ctx,
		`SELECT loyalty_token, (SELECT count(*) FROM loyalty_stamps WHERE client_id = c.id AND redemption_id IS NULL),
		        (SELECT count(*) FROM loyalty_redemptions WHERE client_id = c.id)
		   FROM clients c WHERE c.id = $1 AND c.business_id = $2 AND c.anonymized_at IS NULL`, clientID, bizID).Scan(&token, &c.Stamps, &c.Redeemed); err != nil {
		return nil, err
	}
	if token != nil {
		c.Path = "/tarjeta/" + *token
	}
	c.Ready = c.Enabled && c.Stamps >= c.VisitsRequired
	rows, err := q.Query(ctx, `SELECT to_char(day, 'YYYY-MM-DD'), redemption_id IS NOT NULL FROM loyalty_stamps WHERE client_id = $1 ORDER BY day DESC LIMIT 12`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v loyaltyVisit
		if err := rows.Scan(&v.Day, &v.Redeemed); err != nil {
			return nil, err
		}
		c.History = append(c.History, v)
	}
	return c, rows.Err()
}

func (a *API) clientLoyalty(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := a.loadCard(r.Context(), a.DB, p.BusinessID, id)
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

// redeemLoyalty canjea el premio: usa los sellos más antiguos y deja el resto.
func (a *API) redeemLoyalty(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	// bloquea al cliente: dos canjes a la vez no usan los mismos sellos
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM clients WHERE id = $1 AND business_id = $2 AND anonymized_at IS NULL FOR UPDATE`, id, p.BusinessID).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	} else if err != nil {
		serverErr(w)
		return
	}
	s, err := a.loadLoyaltySettings(ctx, tx, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	if !s.Enabled {
		writeErr(w, http.StatusConflict, "la tarjeta de lealtad está apagada")
		return
	}
	var open int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM loyalty_stamps WHERE client_id = $1 AND redemption_id IS NULL`, id).Scan(&open); err != nil {
		serverErr(w)
		return
	}
	if open < s.VisitsRequired {
		writeErr(w, http.StatusConflict, "todavía no completa los sellos del premio")
		return
	}
	var redemption int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO loyalty_redemptions (business_id, client_id, reward, stamps, redeemed_by) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		p.BusinessID, id, s.Reward, s.VisitsRequired, p.UserID).Scan(&redemption); err != nil {
		serverErr(w)
		return
	}
	if _, err := tx.Exec(ctx,
		`UPDATE loyalty_stamps SET redemption_id = $2 WHERE id IN (SELECT id FROM loyalty_stamps WHERE client_id = $1 AND redemption_id IS NULL ORDER BY day, id LIMIT $3)`,
		id, redemption, s.VisitsRequired); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	c, err := a.loadCard(ctx, a.DB, p.BusinessID, id)
	if err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// loyaltyLink: {"action": "create" | "rotate" | "remove"}. «create» conserva el enlace que ya existe; «rotate» lo cambia
// (el anterior deja de servir) y «remove» lo quita.
func (a *API) loyaltyLink(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	var q string
	var args []any
	switch req.Action {
	case "create":
		q, args = `UPDATE clients SET loyalty_token = COALESCE(loyalty_token, $3) WHERE id = $1 AND business_id = $2 AND anonymized_at IS NULL`, []any{id, p.BusinessID, newPublicToken()}
	case "rotate":
		q, args = `UPDATE clients SET loyalty_token = $3 WHERE id = $1 AND business_id = $2 AND anonymized_at IS NULL`, []any{id, p.BusinessID, newPublicToken()}
	case "remove":
		q, args = `UPDATE clients SET loyalty_token = NULL WHERE id = $1 AND business_id = $2 AND anonymized_at IS NULL`, []any{id, p.BusinessID}
	default:
		writeErr(w, http.StatusBadRequest, "acción no válida")
		return
	}
	tag, err := a.DB.Exec(r.Context(), q, args...)
	if err != nil {
		serverErr(w)
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	a.clientLoyalty(w, r)
}

type publicCardOut struct {
	Business       string         `json:"business"`
	Name           string         `json:"name"` // solo el primer nombre
	VisitsRequired int            `json:"visits_required"`
	Reward         string         `json:"reward"`
	Stamps         int            `json:"stamps"`
	Ready          bool           `json:"ready"`
	Redeemed       int            `json:"redeemed"`
	History        []loyaltyVisit `json:"history"`
}

// publicCard es GET /api/public-card/{token}: la tarjeta virtual de un cliente. Un enlace desconocido, quitado, de un
// cliente borrado, de una tarjeta apagada o de un negocio suspendido responde igual (404).
func (a *API) publicCard(w http.ResponseWriter, r *http.Request) {
	if !a.allow(w, "card:"+clientIP(r), 60, time.Minute) {
		return
	}
	notFound := func() { writeErr(w, http.StatusNotFound, "esta tarjeta no existe") }
	token := r.PathValue("token")
	if !tokenShape.MatchString(token) {
		notFound()
		return
	}
	ctx := r.Context()
	var bizID, clientID int64
	var business, name string
	err := a.DB.QueryRow(ctx,
		`SELECT b.id, c.id, b.name, c.name FROM clients c JOIN businesses b ON b.id = c.business_id JOIN loyalty_settings l ON l.business_id = b.id AND l.enabled
		  WHERE c.loyalty_token = $1 AND c.anonymized_at IS NULL AND `+usableSQL("b.", "$2"), token, a.now()).Scan(&bizID, &clientID, &business, &name)
	if err != nil {
		notFound()
		return
	}
	card, err := a.loadCard(ctx, a.DB, bizID, clientID)
	if err != nil {
		serverErr(w)
		return
	}
	first := name
	if f := strings.Fields(name); len(f) > 0 {
		first = f[0]
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	writeJSON(w, http.StatusOK, publicCardOut{
		Business: business, Name: first, VisitsRequired: card.VisitsRequired, Reward: card.Reward,
		Stamps: card.Stamps, Ready: card.Ready, Redeemed: card.Redeemed, History: card.History,
	})
}
