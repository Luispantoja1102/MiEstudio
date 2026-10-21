package api

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

// Integración con la terminal Point de Mercado Pago (API de órdenes).
// Cada negocio conecta su propia cuenta con su token de acceso, que se guarda cifrado (AES-GCM).
// Una terminal por sucursal. El estado de un cobro se consulta a Mercado Pago, que es la fuente de verdad.

const defaultMPBase = "https://api.mercadopago.com"

var mpHTTP = &http.Client{Timeout: 20 * time.Second}

var errNoEncKey = &httpFail{http.StatusServiceUnavailable, "el cobro con terminal no está habilitado en este servidor (falta TOKEN_ENC_KEY)", "mp_disabled"}

func (a *API) seal(plain string) ([]byte, error) {
	if len(a.EncKey) != 32 {
		return nil, errNoEncKey
	}
	block, err := aes.NewCipher(a.EncKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(plain), nil), nil
}

func (a *API) open(blob []byte) (string, error) {
	if len(a.EncKey) != 32 {
		return "", errNoEncKey
	}
	block, err := aes.NewCipher(a.EncKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(blob) < gcm.NonceSize() {
		return "", errors.New("token cifrado inválido")
	}
	plain, err := gcm.Open(nil, blob[:gcm.NonceSize()], blob[gcm.NonceSize():], nil)
	return string(plain), err
}

type mpError struct {
	Status int
	Msg    string
	Code   string
}

func (e *mpError) Error() string {
	return "mercado pago (" + strconv.Itoa(e.Status) + "): " + e.Msg + " [" + e.Code + "]"
}

// parseMPError lee los formatos de error de Mercado Pago (órdenes, API clásica y {"error"}).
func parseMPError(status int, data []byte) *mpError {
	var e struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Errors  []struct {
			Code    string   `json:"code"`
			Message string   `json:"message"`
			Details []string `json:"details"`
		} `json:"errors"`
		Cause []struct {
			Code        any    `json:"code"`
			Description string `json:"description"`
		} `json:"cause"`
	}
	_ = json.Unmarshal(data, &e)
	me := &mpError{Status: status, Msg: e.Message}
	if me.Msg == "" {
		me.Msg = e.Error
	}
	for _, it := range e.Errors {
		if me.Code == "" {
			me.Code = it.Code
		}
		parts := append([]string{it.Message}, it.Details...)
		if joined := strings.TrimSpace(strings.Join(parts, " — ")); joined != "" && !strings.Contains(me.Msg, joined) {
			if me.Msg != "" {
				me.Msg += "; "
			}
			me.Msg += joined
		}
	}
	for _, c := range e.Cause {
		if me.Code == "" && c.Code != nil {
			me.Code = fmt.Sprint(c.Code)
		}
		if c.Description != "" && !strings.Contains(me.Msg, c.Description) {
			if me.Msg != "" {
				me.Msg += "; "
			}
			me.Msg += c.Description
		}
	}
	if me.Msg == "" {
		me.Msg = "respuesta " + strconv.Itoa(status) + " sin detalle"
	}
	return me
}

func (a *API) mpBase() string {
	if a.MPBase != "" {
		return strings.TrimRight(a.MPBase, "/")
	}
	return defaultMPBase
}

// mpCall llama a Mercado Pago con el token. Un 4xx/5xx se vuelve *mpError.
func (a *API) mpCall(ctx context.Context, token, method, path string, headers map[string]string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.mpBase()+path, rdr)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if method == http.MethodPost {
		key := make([]byte, 16)
		rand.Read(key)
		req.Header.Set("X-Idempotency-Key", hex.EncodeToString(key))
	}
	res, err := mpHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("mercado pago: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return parseMPError(res.StatusCode, data)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// providerFail convierte un error de Mercado Pago en una respuesta clara para quien cobra.
func providerFail(err error) error {
	var me *mpError
	if errors.As(err, &me) {
		if me.Status == 401 || me.Status == 403 {
			return &httpFail{http.StatusBadGateway, "Mercado Pago rechazó la conexión. Vuelve a conectar la cuenta.", "mp_token_revoked"}
		}
		return &httpFail{http.StatusBadGateway, "Mercado Pago: " + me.Msg, "mp_error"}
	}
	log.Printf("mercado pago: %v", err)
	return &httpFail{http.StatusBadGateway, "No pudimos comunicarnos con Mercado Pago. Intenta de nuevo.", "mp_unreachable"}
}

// mpToken devuelve un token de acceso válido del negocio. Si la cuenta se conectó por OAuth y el token
// está por vencer, lo renueva (con la fila bloqueada: el token de renovación es de un solo uso, así que
// dos peticiones simultáneas no deben renovar a la vez).
func (a *API) mpToken(ctx context.Context, bizID int64) (string, error) {
	var blob, refresh []byte
	var expires *time.Time
	var source string
	err := a.DB.QueryRow(ctx, `SELECT token_enc, refresh_enc, expires_at, source FROM mp_accounts WHERE business_id = $1`, bizID).
		Scan(&blob, &refresh, &expires, &source)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &httpFail{http.StatusConflict, "conecta tu cuenta de Mercado Pago para cobrar con terminal", "mp_not_connected"}
	}
	if err != nil {
		return "", err
	}
	token, err := a.open(blob)
	if err != nil {
		return "", err
	}
	if source != "oauth" || expires == nil || time.Until(*expires) > 24*time.Hour || a.MPClientID == "" {
		return token, nil
	}
	return a.refreshMPToken(ctx, bizID, token)
}

func (a *API) refreshMPToken(ctx context.Context, bizID int64, fallback string) (string, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return fallback, nil
	}
	defer tx.Rollback(ctx)
	var blob, refreshBlob []byte
	var expires *time.Time
	if err := tx.QueryRow(ctx, `SELECT token_enc, refresh_enc, expires_at FROM mp_accounts WHERE business_id = $1 FOR UPDATE`, bizID).
		Scan(&blob, &refreshBlob, &expires); err != nil {
		return fallback, nil
	}
	current, err := a.open(blob)
	if err != nil {
		return "", err
	}
	if expires != nil && time.Until(*expires) > 24*time.Hour {
		return current, nil // otra petición ya lo renovó mientras esperábamos
	}
	refresh, err := a.open(refreshBlob)
	if err != nil || refresh == "" {
		return current, nil
	}
	tok, err := a.mpTokenRequest(ctx, map[string]string{"grant_type": "refresh_token", "refresh_token": refresh})
	if err != nil {
		// si el token actual aún sirve, se usa; si ya venció, hay que reconectar
		if expires != nil && time.Now().After(*expires) {
			return "", &httpFail{http.StatusBadGateway, "la conexión con Mercado Pago venció. Vuelve a conectar la cuenta.", "mp_token_revoked"}
		}
		log.Printf("renovar token de Mercado Pago (negocio %d): %v", bizID, err)
		return current, nil
	}
	at, e1 := a.seal(tok.AccessToken)
	rt, e2 := a.seal(tok.RefreshToken)
	if e1 != nil || e2 != nil {
		return current, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE mp_accounts SET token_enc = $2, refresh_enc = $3, expires_at = $4 WHERE business_id = $1`,
		bizID, at, rt, tok.expiry()); err != nil {
		return current, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return current, nil
	}
	return tok.AccessToken, nil
}

type oauthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	UserID       int64  `json:"user_id"`
}

func (t oauthToken) expiry() time.Time {
	if t.ExpiresIn <= 0 {
		return time.Now().Add(180 * 24 * time.Hour)
	}
	return time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
}

// mpTokenRequest pide un token a Mercado Pago (código de autorización o renovación).
func (a *API) mpTokenRequest(ctx context.Context, extra map[string]string) (oauthToken, error) {
	form := map[string]string{"client_id": a.MPClientID, "client_secret": a.MPClientSecret}
	for k, v := range extra {
		form[k] = v
	}
	var tok oauthToken
	if err := a.mpCall(ctx, "", http.MethodPost, "/oauth/token", nil, form, &tok); err != nil {
		return tok, err
	}
	if tok.AccessToken == "" {
		return tok, errors.New("Mercado Pago no devolvió un token")
	}
	return tok, nil
}

// ---------- conexión por OAuth ----------

func (a *API) oauthEnabled() bool {
	return a.MPClientID != "" && a.MPClientSecret != "" && a.MPRedirect != "" && len(a.EncKey) == 32
}

// El estado del OAuth enlaza la autorización con el negocio que la pidió y caduca a los 15 minutos.
func (a *API) stateKey() []byte {
	if len(a.OAuthState) == 32 {
		return a.OAuthState
	}
	sum := sha256.Sum256([]byte("mp-oauth-state|" + a.Secret))
	return sum[:]
}

func (a *API) signState(bizID int64) string {
	payload := strconv.FormatInt(bizID, 10) + "." + strconv.FormatInt(a.now().Add(15*time.Minute).Unix(), 10)
	mac := hmac.New(sha256.New, a.stateKey())
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + hex.EncodeToString(mac.Sum(nil))
}

func (a *API) verifyState(state string) (int64, bool) {
	parts := strings.SplitN(state, ".", 2)
	if len(parts) != 2 {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, false
	}
	mac := hmac.New(sha256.New, a.stateKey())
	mac.Write(raw)
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(parts[1])) {
		return 0, false
	}
	f := strings.SplitN(string(raw), ".", 2)
	if len(f) != 2 {
		return 0, false
	}
	id, err1 := strconv.ParseInt(f[0], 10, 64)
	exp, err2 := strconv.ParseInt(f[1], 10, 64)
	if err1 != nil || err2 != nil || id <= 0 || a.now().Unix() > exp {
		return 0, false
	}
	return id, true
}

// mpConnectURL devuelve la dirección de Mercado Pago donde el dueño autoriza la conexión.
func (a *API) mpConnectURL(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if !a.oauthEnabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "la conexión con Mercado Pago no está configurada en el servidor (MP_CLIENT_ID, MP_CLIENT_SECRET, MP_OAUTH_REDIRECT y TOKEN_ENC_KEY)",
			"code":  "mp_oauth_disabled"})
		return
	}
	q := url.Values{
		"client_id": {a.MPClientID}, "response_type": {"code"}, "platform_id": {"mp"},
		"state": {a.signState(p.BusinessID)}, "redirect_uri": {a.MPRedirect},
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": a.MPAuthBase + "/authorization?" + q.Encode()})
}

// mpCallback termina el OAuth: el navegador llega aquí desde Mercado Pago con un código.
// Siempre regresa a la pantalla de Pagos diciendo qué pasó.
func (a *API) mpCallback(w http.ResponseWriter, r *http.Request) {
	back := func(result, reason string) {
		q := url.Values{"mp": {result}}
		if reason != "" {
			q.Set("reason", reason)
		}
		http.Redirect(w, r, "/panel/pagos?"+q.Encode(), http.StatusFound)
	}
	query := r.URL.Query()
	if query.Get("error") != "" || query.Get("code") == "" {
		back("error", "cancelled")
		return
	}
	bizID, ok := a.verifyState(query.Get("state"))
	if !ok || !a.oauthEnabled() {
		back("error", "state")
		return
	}
	tok, err := a.mpTokenRequest(r.Context(), map[string]string{
		"grant_type": "authorization_code", "code": query.Get("code"), "redirect_uri": a.MPRedirect})
	if err != nil {
		log.Printf("OAuth de Mercado Pago (negocio %d): %v", bizID, err)
		back("error", "oauth_failed")
		return
	}
	at, e1 := a.seal(tok.AccessToken)
	rt, e2 := a.seal(tok.RefreshToken)
	if e1 != nil || e2 != nil {
		back("error", "unknown")
		return
	}
	if _, err := a.DB.Exec(r.Context(),
		`INSERT INTO mp_accounts (business_id, token_enc, refresh_enc, expires_at, mp_user_id, source) VALUES ($1, $2, $3, $4, $5, 'oauth')
		 ON CONFLICT (business_id) DO UPDATE SET token_enc = EXCLUDED.token_enc, refresh_enc = EXCLUDED.refresh_enc,
		   expires_at = EXCLUDED.expires_at, mp_user_id = EXCLUDED.mp_user_id, source = 'oauth', connected_at = now()`,
		bizID, at, rt, tok.expiry(), strconv.FormatInt(tok.UserID, 10)); err != nil {
		log.Printf("guardar cuenta de Mercado Pago: %v", err)
		back("error", "unknown")
		return
	}
	a.logActivity(r.Context(), 0, bizID, "mp_connected", map[string]any{"via": "oauth"})
	back("ok", "")
}

type mpTerminal struct {
	ID            string `json:"id"`
	ExternalPosID string `json:"external_pos_id,omitempty"`
	OperatingMode string `json:"operating_mode"`
}

func (a *API) mpListTerminals(ctx context.Context, token string) ([]mpTerminal, error) {
	var out struct {
		Data struct {
			Terminals []mpTerminal `json:"terminals"`
		} `json:"data"`
		Terminals []mpTerminal `json:"terminals"`
	}
	if err := a.mpCall(ctx, token, http.MethodGet, "/terminals/v1/list?limit=50", nil, nil, &out); err != nil {
		return nil, err
	}
	if len(out.Data.Terminals) > 0 {
		return out.Data.Terminals, nil
	}
	return out.Terminals, nil
}

func (a *API) mpSetMode(ctx context.Context, token, id, mode string) error {
	body := map[string]any{"terminals": []map[string]string{{"id": id, "operating_mode": mode}}}
	return a.mpCall(ctx, token, http.MethodPatch, "/terminals/v1/setup", nil, body, nil)
}

// ---------- configuración (administrador) ----------

type mpTerminalView struct {
	BranchID   int64  `json:"branch_id"`
	BranchName string `json:"branch_name"`
	TerminalID string `json:"terminal_id"`
	Label      string `json:"label"`
}

func (a *API) mpStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var connected bool
	if err := a.DB.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM mp_accounts WHERE business_id = $1)`, p.BusinessID).Scan(&connected); err != nil {
		serverErr(w)
		return
	}
	rows, err := a.DB.Query(r.Context(),
		`SELECT t.branch_id, b.name, t.terminal_id, t.label FROM mp_terminals t JOIN branches b ON b.id = t.branch_id
		 WHERE t.business_id = $1 ORDER BY b.id`, p.BusinessID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	terms := []mpTerminalView{}
	for rows.Next() {
		var t mpTerminalView
		if err := rows.Scan(&t.BranchID, &t.BranchName, &t.TerminalID, &t.Label); err != nil {
			serverErr(w)
			return
		}
		terms = append(terms, t)
	}
	var source string
	a.DB.QueryRow(r.Context(), `SELECT source FROM mp_accounts WHERE business_id = $1`, p.BusinessID).Scan(&source)
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": len(a.EncKey) == 32, "oauth_available": a.oauthEnabled(), "connected": connected, "source": source, "terminals": terms})
}

// mpConnect guarda el token de acceso del negocio (cifrado) tras comprobar que Mercado Pago lo acepta.
func (a *API) mpConnect(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		AccessToken string `json:"access_token"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	token := strings.TrimSpace(req.AccessToken)
	if len(token) < 20 || len(token) > 400 || strings.ContainsAny(token, " \t\r\n") {
		writeErr(w, http.StatusBadRequest, "el token de acceso no tiene un formato válido")
		return
	}
	enc, err := a.seal(token)
	if err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}
	if _, err := a.mpListTerminals(r.Context(), token); err != nil {
		var me *mpError
		if errors.As(err, &me) && (me.Status == 401 || me.Status == 403) {
			writeErr(w, http.StatusBadRequest, "Mercado Pago no aceptó ese token de acceso")
			return
		}
		writeFail(w, providerFail(err))
		return
	}
	if _, err := a.DB.Exec(r.Context(),
		`INSERT INTO mp_accounts (business_id, token_enc, source) VALUES ($1, $2, 'token')
		 ON CONFLICT (business_id) DO UPDATE SET token_enc = EXCLUDED.token_enc, refresh_enc = NULL, expires_at = NULL,
		   mp_user_id = NULL, source = 'token', connected_at = now()`, p.BusinessID, enc); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(r.Context(), p.UserID, p.BusinessID, "mp_connected", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) mpDisconnect(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM mp_terminals WHERE business_id = $1`, p.BusinessID); err != nil {
		serverErr(w)
		return
	}
	if _, err := tx.Exec(ctx, `DELETE FROM mp_accounts WHERE business_id = $1`, p.BusinessID); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	a.logActivity(ctx, p.UserID, p.BusinessID, "mp_disconnected", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) mpTerminals(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	token, err := a.mpToken(r.Context(), p.BusinessID)
	if err == nil {
		var list []mpTerminal
		if list, err = a.mpListTerminals(r.Context(), token); err == nil {
			if list == nil {
				list = []mpTerminal{}
			}
			writeJSON(w, http.StatusOK, list)
			return
		}
		err = providerFail(err)
	}
	if !writeFail(w, err) {
		serverErr(w)
	}
}

// mpRegisterTerminal asigna una terminal a una sucursal y la pone en modo punto de venta (PDV),
// para que solo cobre lo que envía la aplicación.
func (a *API) mpRegisterTerminal(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		BranchID   int64  `json:"branch_id"`
		TerminalID string `json:"terminal_id"`
	}
	if !decode(w, r, &req) || req.TerminalID == "" {
		writeErr(w, http.StatusBadRequest, "indica la sucursal y la terminal")
		return
	}
	ctx := r.Context()
	if !a.planOf(ctx, a.DB, p.BusinessID).Terminal {
		writeFail(w, errPlanTerminal)
		return
	}
	if ok, err := activeBranch(ctx, a.DB, p.BusinessID, req.BranchID); err != nil {
		serverErr(w)
		return
	} else if !ok {
		writeErr(w, http.StatusNotFound, "sucursal no encontrada")
		return
	}
	token, err := a.mpToken(ctx, p.BusinessID)
	if err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}
	list, err := a.mpListTerminals(ctx, token)
	if err != nil {
		writeFail(w, providerFail(err))
		return
	}
	var found *mpTerminal
	for i := range list {
		if list[i].ID == req.TerminalID {
			found = &list[i]
		}
	}
	if found == nil {
		writeErr(w, http.StatusNotFound, "esa terminal no pertenece a tu cuenta de Mercado Pago")
		return
	}
	if err := a.mpSetMode(ctx, token, found.ID, "PDV"); err != nil {
		writeFail(w, providerFail(err))
		return
	}
	label := found.ExternalPosID
	if label == "" {
		label = found.ID
	}
	_, err = a.DB.Exec(ctx,
		`INSERT INTO mp_terminals (branch_id, business_id, terminal_id, label) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (branch_id) DO UPDATE SET terminal_id = EXCLUDED.terminal_id, label = EXCLUDED.label`,
		req.BranchID, p.BusinessID, found.ID, label)
	if err != nil {
		serverErr(w)
		return
	}
	a.logActivity(ctx, p.UserID, p.BusinessID, "mp_terminal_registered", map[string]any{"branch_id": req.BranchID, "terminal": label})
	writeJSON(w, http.StatusOK, map[string]any{"terminal_id": found.ID, "label": label})
}

// ---------- cobros con la terminal ----------

type pointState struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Label   string `json:"label,omitempty"`
}

// preflight responde «¿puedo cobrar en la terminal ahora mismo?» para una sucursal.
func (a *API) preflight(ctx context.Context, bizID, branchID int64) (pointState, string, string) {
	if !a.planOf(ctx, a.DB, bizID).Terminal {
		return pointState{Code: "plan_feature", Message: errPlanTerminal.Msg}, "", ""
	}
	if len(a.EncKey) != 32 {
		return pointState{Code: "mp_disabled", Message: "El cobro con terminal no está habilitado en este servidor."}, "", ""
	}
	var terminal, label string
	err := a.DB.QueryRow(ctx, `SELECT terminal_id, label FROM mp_terminals WHERE branch_id = $1 AND business_id = $2`, branchID, bizID).Scan(&terminal, &label)
	if errors.Is(err, pgx.ErrNoRows) {
		var connected bool
		a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mp_accounts WHERE business_id = $1)`, bizID).Scan(&connected)
		if !connected {
			return pointState{Code: "mp_not_connected", Message: "Conecta tu cuenta de Mercado Pago para cobrar con terminal."}, "", ""
		}
		return pointState{Code: "no_terminal", Message: "Esta sucursal no tiene una terminal registrada."}, "", ""
	}
	if err != nil {
		return pointState{Code: "check_failed", Message: "No pudimos revisar la terminal."}, "", ""
	}
	st := pointState{Label: label}
	token, err := a.mpToken(ctx, bizID)
	if err != nil {
		st.Code, st.Message = "check_failed", "No pudimos revisar la terminal."
		var f *httpFail
		if errors.As(err, &f) {
			st.Code, st.Message = f.Code, f.Msg
		}
		return st, "", ""
	}
	list, err := a.mpListTerminals(ctx, token)
	if err != nil {
		st.Code, st.Message = "check_failed", "No pudimos hablar con Mercado Pago para revisar la terminal."
		var me *mpError
		if errors.As(err, &me) && (me.Status == 401 || me.Status == 403) {
			st.Code, st.Message = "mp_token_revoked", "Mercado Pago rechazó la conexión. Vuelve a conectar la cuenta."
		}
		return st, "", ""
	}
	for _, t := range list {
		if t.ID != terminal {
			continue
		}
		if t.OperatingMode != "" && t.OperatingMode != "PDV" {
			if err := a.mpSetMode(ctx, token, terminal, "PDV"); err != nil {
				st.Code, st.Message = "check_failed", "No pudimos poner la terminal en modo punto de venta."
				return st, "", ""
			}
		}
		st.OK, st.Code, st.Message = true, "ready", "Terminal lista"
		return st, token, terminal
	}
	st.Code, st.Message = "terminal_missing", "La terminal ya no aparece en tu cuenta de Mercado Pago. Vuelve a registrarla."
	return st, "", ""
}

func (a *API) pointStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	branchID, ok := queryBranch(w, r)
	if !ok {
		return
	}
	st, _, _ := a.preflight(r.Context(), p.BusinessID, branchID)
	writeJSON(w, http.StatusOK, st)
}

type chargeOut struct {
	ID          string `json:"id"`
	Status      string `json:"status"` // open | approved | canceled | error
	AmountCents int    `json:"amount_cents"`
	Used        bool   `json:"used"`
	Detail      string `json:"detail,omitempty"`
}

// pointCharge envía a la terminal un cobro por el monto indicado. La venta lo «consume» cuando está aprobado.
func (a *API) pointCharge(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		BranchID    int64 `json:"branch_id"`
		AmountCents int   `json:"amount_cents"`
	}
	if !decode(w, r, &req) || req.AmountCents < 100 || req.AmountCents > maxPriceCents {
		writeErr(w, http.StatusBadRequest, "el monto debe ser de al menos $1.00")
		return
	}
	ctx := r.Context()
	st, token, terminal := a.preflight(ctx, p.BusinessID, req.BranchID)
	if !st.OK {
		writeJSON(w, http.StatusConflict, map[string]string{"error": st.Message, "code": st.Code})
		return
	}
	// Mercado Pago solo acepta letras, dígitos, "-" y "_" en la referencia externa
	ref := "mie_" + strconv.FormatInt(p.BusinessID, 36) + "_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	body := map[string]any{
		"type": "point", "external_reference": ref, "description": "Cobro MiEstudio",
		"transactions": map[string]any{"payments": []map[string]string{{"amount": cents(req.AmountCents)}}},
		"config":       map[string]any{"point": map[string]string{"terminal_id": terminal}},
	}
	var order struct {
		ID string `json:"id"`
	}
	if err := a.mpCall(ctx, token, http.MethodPost, "/v1/orders", nil, body, &order); err != nil || order.ID == "" {
		var me *mpError
		if errors.As(err, &me) && regexp.MustCompile(`(?i)already has an order|en espera|already_queued|queued_order`).MatchString(me.Msg+" "+me.Code) {
			writeErr(w, http.StatusConflict, "la terminal tiene un cobro pendiente: termínalo o cancélalo en la terminal")
			return
		}
		if err == nil {
			err = errors.New("la orden no trae id")
		}
		writeFail(w, providerFail(err))
		return
	}
	if _, err := a.DB.Exec(ctx,
		`INSERT INTO mp_charges (id, business_id, branch_id, amount_cents, external_ref, created_by) VALUES ($1, $2, $3, $4, $5, $6)`,
		order.ID, p.BusinessID, req.BranchID, req.AmountCents, ref, p.UserID); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, chargeOut{ID: order.ID, Status: "open", AmountCents: req.AmountCents})
}

func (a *API) loadCharge(ctx context.Context, bizID int64, id string) (chargeOut, error) {
	var c chargeOut
	var sale *int64
	err := a.DB.QueryRow(ctx, `SELECT id, status, amount_cents, sale_id, detail FROM mp_charges WHERE id = $1 AND business_id = $2`, id, bizID).
		Scan(&c.ID, &c.Status, &c.AmountCents, &sale, &c.Detail)
	c.Used = sale != nil
	return c, err
}

func (a *API) setCharge(ctx context.Context, id, status, detail string) {
	a.DB.Exec(ctx, `UPDATE mp_charges SET status = $2, detail = $3 WHERE id = $1 AND status = 'open'`, id, status, detail)
}

// flexFloat acepta el monto como número o como texto ("150.00").
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	*f = flexFloat(v)
	return err
}

// syncCharge pregunta a Mercado Pago qué pasó con el cobro y lo actualiza. Es idempotente.
func (a *API) syncCharge(ctx context.Context, bizID int64, c chargeOut) (chargeOut, error) {
	if c.Status != "open" {
		return c, nil
	}
	token, err := a.mpToken(ctx, bizID)
	if err != nil {
		return c, err
	}
	var order struct {
		Status       string `json:"status"`
		Transactions struct {
			Payments []struct {
				ID         string    `json:"id"`
				Amount     flexFloat `json:"amount"`
				PaidAmount flexFloat `json:"paid_amount"`
			} `json:"payments"`
		} `json:"transactions"`
	}
	if err := a.mpCall(ctx, token, http.MethodGet, "/v1/orders/"+url.PathEscape(c.ID), nil, nil, &order); err != nil {
		return c, providerFail(err)
	}
	switch strings.ToLower(order.Status) {
	case "processed":
		var paid float64
		payID := c.ID
		if len(order.Transactions.Payments) > 0 {
			pay := order.Transactions.Payments[0]
			paid = float64(pay.PaidAmount)
			if paid == 0 {
				paid = float64(pay.Amount)
			}
			if pay.ID != "" {
				payID = pay.ID
			}
		}
		if int(paid*100+0.5) != c.AmountCents {
			c.Detail = "El monto cobrado en la terminal no coincide con la cuenta."
			a.setCharge(ctx, c.ID, "error", c.Detail)
			c.Status = "error"
			return c, nil
		}
		a.DB.Exec(ctx, `UPDATE mp_charges SET status = 'approved', mp_payment_id = $2 WHERE id = $1 AND status = 'open'`, c.ID, payID)
		c.Status = "approved"
	case "failed", "canceled", "cancelled", "expired":
		a.setCharge(ctx, c.ID, "canceled", "")
		c.Status = "canceled"
	}
	return c, nil
}

func (a *API) pointChargeStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	c, err := a.loadCharge(r.Context(), p.BusinessID, r.PathValue("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "cobro no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	if c, err = a.syncCharge(r.Context(), p.BusinessID, c); err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (a *API) pointChargeCancel(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	id := r.PathValue("id")
	c, err := a.loadCharge(ctx, p.BusinessID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "cobro no encontrado")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	if c.Status != "open" {
		writeErr(w, http.StatusConflict, "ese cobro ya no está pendiente")
		return
	}
	token, err := a.mpToken(ctx, p.BusinessID)
	if err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}
	// Con la orden ya en la pantalla de la terminal, Mercado Pago solo la cancela con este encabezado.
	cancelErr := a.mpCall(ctx, token, http.MethodPost, "/v1/orders/"+url.PathEscape(id)+"/cancel",
		map[string]string{"x-allow-cancelable-status": "at_terminal"}, nil, nil)
	if c, err = a.syncCharge(ctx, p.BusinessID, c); err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}
	if c.Status == "open" {
		// solo se marca cancelado si Mercado Pago aceptó: si no, la terminal todavía tiene el cobro
		if cancelErr != nil {
			log.Printf("cancelar cobro %s: %v", id, cancelErr)
			writeErr(w, http.StatusConflict, "no se pudo cancelar en la terminal: cancélalo también en la terminal o intenta de nuevo")
			return
		}
		a.setCharge(ctx, id, "canceled", "")
		c.Status = "canceled"
	}
	writeJSON(w, http.StatusOK, c)
}

// refundMPCharges devuelve el dinero de los cobros con terminal de una venta (al cancelarla).
// Es idempotente: un cobro ya reembolsado se omite. Si Mercado Pago falla, la venta no se cancela.
func (a *API) refundMPCharges(ctx context.Context, bizID, saleID int64) error {
	rows, err := a.DB.Query(ctx,
		`SELECT id FROM mp_charges WHERE business_id = $1 AND sale_id = $2 AND status = 'approved' AND refunded_at IS NULL ORDER BY id`, bizID, saleID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil
	}
	token, err := a.mpToken(ctx, bizID)
	if err != nil {
		return err
	}
	already := regexp.MustCompile(`(?i)already.*refund|ya.*reembols|fully refunded`)
	for _, id := range ids {
		if err := a.mpCall(ctx, token, http.MethodPost, "/v1/orders/"+url.PathEscape(id)+"/refund", nil, nil, nil); err != nil {
			var me *mpError
			if !(errors.As(err, &me) && already.MatchString(me.Msg)) {
				msg := "Mercado Pago no pudo devolver el dinero"
				if me != nil {
					msg += ": " + me.Msg
				}
				return &httpFail{http.StatusBadGateway, msg + ". La venta no se canceló.", "refund_failed"}
			}
		}
		if _, err := a.DB.Exec(ctx, `UPDATE mp_charges SET refunded_at = now() WHERE id = $1`, id); err != nil {
			return err
		}
	}
	return nil
}
