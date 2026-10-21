package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

const mpToken = "APP_USR-1234567890-valid-token-abcdef"

// fakeMP imita lo que usamos de la API de Mercado Pago (órdenes y terminales Point).
type fakeMP struct {
	mu         sync.Mutex
	srv        *httptest.Server
	terminals  []mpTerminal
	orders     map[string]*fakeOrder
	calls      []string
	lastBody   map[string]any
	busy       bool // la terminal ya tiene una orden en espera
	refundFail string
	cancelFail bool

	// OAuth simulado
	valid        map[string]bool // tokens de acceso vigentes
	accessN      int
	refreshToken string // el de renovación vigente (de un solo uso)
	refreshCalls int
	refreshFail  bool
	expiresIn    int
}

type fakeOrder struct {
	status, paid string
	refunded     int
}

func newFakeMP(t *testing.T) *fakeMP {
	f := &fakeMP{valid: map[string]bool{mpToken: true}, expiresIn: 15552000, orders: map[string]*fakeOrder{}, terminals: []mpTerminal{{ID: "NEWLAND_N950__N950NCB000001", ExternalPosID: "CAJA1", OperatingMode: "STANDALONE"}}}
	mux := http.NewServeMux()
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.calls = append(f.calls, r.Method+" "+r.URL.Path)
			if !f.valid[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")] {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"message":"invalid access token","status":401}`))
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls = append(f.calls, "POST /oauth/token")
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		bad := func(msg string) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"message": msg, "error": "invalid_grant"})
		}
		if body["client_id"] != "CID" || body["client_secret"] != "CSECRET" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"invalid client"}`))
			return
		}
		switch body["grant_type"] {
		case "authorization_code":
			if body["code"] != "CODE123" || body["redirect_uri"] != "https://app.example.mx/api/point/oauth/callback" {
				bad("invalid code or redirect_uri")
				return
			}
		case "refresh_token":
			f.refreshCalls++
			if f.refreshFail {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if body["refresh_token"] != f.refreshToken { // de un solo uso
				bad("invalid refresh token")
				return
			}
			for tok := range f.valid { // el token anterior deja de servir
				if strings.HasPrefix(tok, "OAUTH-AT-") {
					delete(f.valid, tok)
				}
			}
		default:
			bad("unsupported grant_type")
			return
		}
		f.accessN++
		access := fmt.Sprintf("OAUTH-AT-%d", f.accessN)
		f.refreshToken = fmt.Sprintf("OAUTH-RT-%d", f.accessN)
		f.valid[access] = true
		json.NewEncoder(w).Encode(map[string]any{"access_token": access, "refresh_token": f.refreshToken, "expires_in": f.expiresIn, "user_id": 777})
	})
	mux.HandleFunc("GET /terminals/v1/list", guard(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"terminals": f.terminals}})
	}))
	mux.HandleFunc("PATCH /terminals/v1/setup", guard(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Terminals []struct {
				ID            string `json:"id"`
				OperatingMode string `json:"operating_mode"`
			} `json:"terminals"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		for _, bt := range body.Terminals {
			for i := range f.terminals {
				if f.terminals[i].ID == bt.ID {
					f.terminals[i].OperatingMode = bt.OperatingMode
				}
			}
		}
		w.Write([]byte(`{}`))
	}))
	mux.HandleFunc("POST /v1/orders", guard(func(w http.ResponseWriter, r *http.Request) {
		if f.busy {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"errors":[{"code":"already_queued_order_on_terminal","message":"The terminal already has an order in queue"}]}`))
			return
		}
		json.NewDecoder(r.Body).Decode(&f.lastBody)
		id := fmt.Sprintf("ORD%03d", len(f.orders)+1)
		f.orders[id] = &fakeOrder{status: "created"}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "created"})
	}))
	mux.HandleFunc("GET /v1/orders/{id}", guard(func(w http.ResponseWriter, r *http.Request) {
		o := f.orders[r.PathValue("id")]
		if o == nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"order not found"}`))
			return
		}
		payments := []map[string]any{}
		if o.paid != "" {
			payments = append(payments, map[string]any{"id": "PAY-" + r.PathValue("id"), "amount": o.paid, "paid_amount": o.paid})
		}
		json.NewEncoder(w).Encode(map[string]any{"status": o.status, "transactions": map[string]any{"payments": payments}})
	}))
	mux.HandleFunc("POST /v1/orders/{id}/cancel", guard(func(w http.ResponseWriter, r *http.Request) {
		if f.cancelFail {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"message":"cannot cancel"}`))
			return
		}
		f.orders[r.PathValue("id")].status = "canceled"
		w.WriteHeader(http.StatusAccepted)
	}))
	mux.HandleFunc("POST /v1/orders/{id}/refund", guard(func(w http.ResponseWriter, r *http.Request) {
		if f.refundFail != "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"message": f.refundFail})
			return
		}
		f.orders[r.PathValue("id")].refunded++
		w.Write([]byte(`{}`))
	}))
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeMP) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeMP) set(id, status, paid string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orders[id].status, f.orders[id].paid = status, paid
}

func (f *fakeMP) mode() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.terminals[0].OperatingMode
}

type mpFx struct {
	*posFx
	mp *fakeMP
}

// newMP prepara un negocio con la integración habilitada, token conectado y terminal registrada.
func newMP(t *testing.T, ready bool) *mpFx {
	t.Helper()
	p := newPOS(t)
	m := &mpFx{posFx: p, mp: newFakeMP(t)}
	p.a.EncKey = bytes.Repeat([]byte{7}, 32)
	p.a.MPBase = m.mp.srv.URL
	if ready {
		p.must(t, p.a.do(t, "PUT", "/api/payments/mercadopago/token", p.admin, map[string]any{"access_token": mpToken}))
		p.must(t, p.a.do(t, "PUT", "/api/payments/mercadopago/terminals", p.admin, map[string]any{"branch_id": p.branch, "terminal_id": m.mp.terminals[0].ID}))
		p.openCash(t, p.admin, 0)
	}
	return m
}

func (m *mpFx) charge(t *testing.T, amount int) resp {
	return m.a.do(t, "POST", "/api/payments/point/charges", m.cashier, map[string]any{"branch_id": m.branch, "amount_cents": amount})
}

func (m *mpFx) status(t *testing.T, id string) string {
	t.Helper()
	r := m.a.do(t, "GET", "/api/payments/point/charges/"+id, m.cashier, nil)
	if r.Status != 200 {
		t.Fatalf("estado del cobro: %d %v", r.Status, r.Body)
	}
	return r.Body["status"].(string)
}

// approved crea un cobro y lo deja aprobado en la terminal falsa.
func (m *mpFx) approved(t *testing.T, amount int) string {
	t.Helper()
	r := m.charge(t, amount)
	if r.Status != 201 {
		t.Fatalf("crear cobro: %d %v", r.Status, r.Body)
	}
	cid := r.Body["id"].(string)
	m.mp.set(cid, "processed", cents(amount))
	if got := m.status(t, cid); got != "approved" {
		t.Fatalf("estado: %s", got)
	}
	return cid
}

func (m *mpFx) saleWithTerminal(t *testing.T, chargeID string, amount int) resp {
	return m.sale(t, m.cashier, map[string]any{
		"lines":    []any{svcLine(m.svc, m.marco)},
		"payments": []any{map[string]any{"method": "mp_point", "amount_cents": amount, "mp_charge_id": chargeID}},
	})
}

func TestMPConnectEncryptsToken(t *testing.T) {
	m := newMP(t, false)
	a := m.a
	put := func(tok, token string) resp {
		return a.do(t, "PUT", "/api/payments/mercadopago/token", tok, map[string]any{"access_token": token})
	}
	// sin clave de cifrado, la integración está deshabilitada
	a.EncKey = nil
	if r := put(m.admin, mpToken); r.Status != 503 || r.Body["code"] != "mp_disabled" {
		t.Fatalf("sin TOKEN_ENC_KEY: %d %v", r.Status, r.Body)
	}
	a.EncKey = bytes.Repeat([]byte{7}, 32)

	for name, r := range map[string]resp{
		"token corto":        put(m.admin, "abc"),
		"token con espacios": put(m.admin, "APP_USR 1234567890 token con espacios"),
		"token rechazado":    put(m.admin, "APP_USR-0000000000-invalid-token-zzzzzz"),
		"cajero":             put(m.cashier, mpToken),
	} {
		if r.Status == 204 {
			t.Errorf("%s debió rechazarse", name)
		}
	}
	if r := put(m.cashier, mpToken); r.Status != 403 {
		t.Errorf("cajero conectando: %d", r.Status)
	}
	if r := put(m.admin, mpToken); r.Status != 204 {
		t.Fatalf("token válido: %d %v", r.Status, r.Body)
	}
	// el token NO queda en claro en la base
	var blob []byte
	a.DB.QueryRow(context.Background(), `SELECT token_enc FROM mp_accounts`).Scan(&blob)
	if len(blob) == 0 || bytes.Contains(blob, []byte(mpToken)) || bytes.Contains(blob, []byte("APP_USR")) {
		t.Fatal("el token está guardado en claro")
	}
	if plain, err := a.open(blob); err != nil || plain != mpToken {
		t.Fatalf("descifrado: %q %v", plain, err)
	}
	// una clave distinta no descifra
	a.EncKey = bytes.Repeat([]byte{9}, 32)
	if _, err := a.open(blob); err == nil {
		t.Fatal("una clave distinta no debe poder descifrar")
	}
	a.EncKey = bytes.Repeat([]byte{7}, 32)
	if r := a.do(t, "GET", "/api/payments/mercadopago", m.admin, nil); r.Body["connected"] != true || r.Body["enabled"] != true {
		t.Fatalf("estado: %v", r.Body)
	}
	if r := a.do(t, "DELETE", "/api/payments/mercadopago", m.admin, nil); r.Status != 204 {
		t.Fatalf("desconectar: %d", r.Status)
	}
	if r := a.do(t, "GET", "/api/payments/mercadopago", m.admin, nil); r.Body["connected"] != false {
		t.Fatalf("tras desconectar: %v", r.Body)
	}
}

func TestMPTerminalRegistration(t *testing.T) {
	m := newMP(t, false)
	a := m.a
	termID := m.mp.terminals[0].ID
	// sin conectar no se pueden listar terminales
	if r := a.do(t, "GET", "/api/payments/mercadopago/terminals", m.admin, nil); r.Status != 409 || r.Body["code"] != "mp_not_connected" {
		t.Fatalf("sin conectar: %d %v", r.Status, r.Body)
	}
	m.must(t, a.do(t, "PUT", "/api/payments/mercadopago/token", m.admin, map[string]any{"access_token": mpToken}))
	if got := list[mpTerminal](t, a, "/api/payments/mercadopago/terminals", m.admin); len(got) != 1 || got[0].ID != termID {
		t.Fatalf("terminales: %+v", got)
	}
	reg := func(tok string, branch int64, term string) resp {
		return a.do(t, "PUT", "/api/payments/mercadopago/terminals", tok, map[string]any{"branch_id": branch, "terminal_id": term})
	}
	if r := reg(m.admin, m.branch, "OTRA_TERMINAL"); r.Status != 404 {
		t.Errorf("terminal que no es de la cuenta: %d", r.Status)
	}
	if r := reg(m.admin, 99999, termID); r.Status != 404 {
		t.Errorf("sucursal inexistente: %d", r.Status)
	}
	if r := reg(m.cashier, m.branch, termID); r.Status != 403 {
		t.Errorf("cajero registrando: %d", r.Status)
	}
	if m.mp.mode() != "STANDALONE" {
		t.Fatal("la terminal no debe cambiar de modo antes de registrarla")
	}
	if r := reg(m.admin, m.branch, termID); r.Status != 200 || r.Body["label"] != "CAJA1" {
		t.Fatalf("registrar: %d %v", r.Status, r.Body)
	}
	if m.mp.mode() != "PDV" {
		t.Fatalf("la terminal debe quedar en modo punto de venta (PDV), está en %s", m.mp.mode())
	}
	if r := a.do(t, "GET", "/api/payments/mercadopago", m.admin, nil); len(r.Body["terminals"].([]any)) != 1 {
		t.Fatalf("estado: %v", r.Body)
	}
	// otro negocio no ve ni usa lo de este
	other := a.mustRegister(t, "Otro", "otro@x.com")
	if r := a.do(t, "GET", "/api/payments/mercadopago", other, nil); r.Body["connected"] != false || len(r.Body["terminals"].([]any)) != 0 {
		t.Fatalf("otro negocio ve la integración: %v", r.Body)
	}
}

func TestMPPreflight(t *testing.T) {
	m := newMP(t, false)
	a := m.a
	code := func() string {
		r := a.do(t, "GET", fmt.Sprintf("/api/payments/point/status?branch_id=%d", m.branch), m.cashier, nil)
		return r.Body["code"].(string)
	}
	a.EncKey = nil
	if got := code(); got != "mp_disabled" {
		t.Errorf("sin clave: %s", got)
	}
	a.EncKey = bytes.Repeat([]byte{7}, 32)
	if got := code(); got != "mp_not_connected" {
		t.Errorf("sin conectar: %s", got)
	}
	m.must(t, a.do(t, "PUT", "/api/payments/mercadopago/token", m.admin, map[string]any{"access_token": mpToken}))
	if got := code(); got != "no_terminal" {
		t.Errorf("sin terminal: %s", got)
	}
	m.must(t, a.do(t, "PUT", "/api/payments/mercadopago/terminals", m.admin, map[string]any{"branch_id": m.branch, "terminal_id": m.mp.terminals[0].ID}))
	if got := code(); got != "ready" {
		t.Errorf("lista: %s", got)
	}
	// alguien la regresó a otro modo: se corrige sola
	m.mp.mu.Lock()
	m.mp.terminals[0].OperatingMode = "STANDALONE"
	m.mp.mu.Unlock()
	if got := code(); got != "ready" || m.mp.mode() != "PDV" {
		t.Errorf("debe volver a modo PDV: %s / %s", got, m.mp.mode())
	}
	// la terminal desapareció de la cuenta
	m.mp.mu.Lock()
	saved := m.mp.terminals
	m.mp.terminals = nil
	m.mp.mu.Unlock()
	if got := code(); got != "terminal_missing" {
		t.Errorf("terminal ausente: %s", got)
	}
	m.mp.mu.Lock()
	m.mp.terminals = saved
	m.mp.mu.Unlock()
	// el token fue revocado en Mercado Pago
	enc, _ := a.seal("APP_USR-9999999999-revoked-token-zzzzzz")
	a.DB.Exec(context.Background(), `UPDATE mp_accounts SET token_enc = $1`, enc)
	if got := code(); got != "mp_token_revoked" {
		t.Errorf("token revocado: %s", got)
	}
}

func TestMPChargeLifecycle(t *testing.T) {
	m := newMP(t, true)
	a := m.a

	if r := a.do(t, "POST", "/api/payments/point/charges", m.staff, map[string]any{"branch_id": m.branch, "amount_cents": 1000}); r.Status != 403 {
		t.Errorf("profesional cobrando con terminal: %d", r.Status)
	}
	if r := m.charge(t, 99); r.Status != 400 {
		t.Errorf("monto menor a $1: %d", r.Status)
	}
	r := m.charge(t, 15300)
	if r.Status != 201 || r.Body["status"] != "open" {
		t.Fatalf("crear cobro: %d %v", r.Status, r.Body)
	}
	id1 := r.Body["id"].(string)
	// lo que se envió a Mercado Pago
	body := m.mp.lastBody
	pays := body["transactions"].(map[string]any)["payments"].([]any)
	ref := body["external_reference"].(string)
	if body["type"] != "point" || pays[0].(map[string]any)["amount"] != "153.00" ||
		body["config"].(map[string]any)["point"].(map[string]any)["terminal_id"] != m.mp.terminals[0].ID || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(ref) {
		t.Fatalf("orden enviada: %v", body)
	}
	// sigue abierto mientras no pague
	if got := m.status(t, id1); got != "open" {
		t.Fatalf("pendiente: %s", got)
	}
	m.mp.set(id1, "at_terminal", "")
	if got := m.status(t, id1); got != "open" {
		t.Fatalf("en la terminal: %s", got)
	}
	m.mp.set(id1, "processed", "153.00")
	if got := m.status(t, id1); got != "approved" {
		t.Fatalf("aprobado: %s", got)
	}
	// ya aprobado no vuelve a consultarse a Mercado Pago
	before := m.mp.count("GET /v1/orders/")
	m.status(t, id1)
	if m.mp.count("GET /v1/orders/") != before {
		t.Error("un cobro aprobado no debe consultarse de nuevo")
	}

	// la terminal cobró un monto distinto al pedido: error, no se acepta
	r = m.charge(t, 10000)
	id2 := r.Body["id"].(string)
	m.mp.set(id2, "processed", "99.00")
	if got := m.status(t, id2); got != "error" {
		t.Fatalf("monto distinto: %s", got)
	}
	// cancelado o vencido en la terminal
	id3 := m.charge(t, 5000).Body["id"].(string)
	m.mp.set(id3, "canceled", "")
	if got := m.status(t, id3); got != "canceled" {
		t.Fatalf("cancelado: %s", got)
	}
	// la terminal ya tiene un cobro en espera
	m.mp.busy = true
	if r := m.charge(t, 5000); r.Status != 409 {
		t.Errorf("terminal ocupada: %d %v", r.Status, r.Body)
	}
	m.mp.busy = false

	// cancelar desde la caja
	id4 := m.charge(t, 7000).Body["id"].(string)
	if r := a.do(t, "POST", "/api/payments/point/charges/"+id4+"/cancel", m.cashier, nil); r.Status != 200 || r.Body["status"] != "canceled" {
		t.Fatalf("cancelar: %d %v", r.Status, r.Body)
	}
	if r := a.do(t, "POST", "/api/payments/point/charges/"+id4+"/cancel", m.cashier, nil); r.Status != 409 {
		t.Errorf("cancelar uno ya cancelado: %d", r.Status)
	}
	// si Mercado Pago no acepta cancelar, sigue abierto (la terminal aún lo tiene)
	id5 := m.charge(t, 7000).Body["id"].(string)
	m.mp.cancelFail = true
	if r := a.do(t, "POST", "/api/payments/point/charges/"+id5+"/cancel", m.cashier, nil); r.Status != 409 {
		t.Errorf("cancelación rechazada por MP: %d", r.Status)
	}
	if got := m.status(t, id5); got != "open" {
		t.Errorf("debe seguir abierto: %s", got)
	}
	m.mp.cancelFail = false

	// otro negocio no ve cobros ajenos
	other := a.mustRegister(t, "Otro", "otro@x.com")
	if r := a.do(t, "GET", "/api/payments/point/charges/"+id1, other, nil); r.Status != 404 {
		t.Errorf("cobro ajeno: %d", r.Status)
	}
	if r := a.do(t, "POST", "/api/payments/point/charges/"+id1+"/cancel", other, nil); r.Status != 404 {
		t.Errorf("cancelar cobro ajeno: %d", r.Status)
	}
}

func TestSaleWithTerminalPayment(t *testing.T) {
	m := newMP(t, true)
	a := m.a

	// pendiente: no se puede usar
	open := m.charge(t, 15000).Body["id"].(string)
	if r := m.saleWithTerminal(t, open, 15000); r.Status != 409 {
		t.Fatalf("cobro sin aprobar: %d %v", r.Status, r.Body)
	}
	if r := m.saleWithTerminal(t, "NO-EXISTE", 15000); r.Status != 400 {
		t.Fatalf("cobro inexistente: %d", r.Status)
	}
	// aprobado por otro monto: se rechaza y el cobro queda libre (la venta falló completa)
	wrong := m.approved(t, 10000)
	if r := m.saleWithTerminal(t, wrong, 15000); r.Status != 400 {
		t.Fatalf("monto distinto: %d %v", r.Status, r.Body)
	}
	var sale *int64
	a.DB.QueryRow(context.Background(), `SELECT sale_id FROM mp_charges WHERE id = $1`, wrong).Scan(&sale)
	if sale != nil {
		t.Fatal("una venta rechazada no debe consumir el cobro")
	}
	// el mismo cobro dos veces en una misma venta
	good := m.approved(t, 7500)
	dup := m.sale(t, m.cashier, map[string]any{
		"lines": []any{svcLine(m.svc, m.marco)},
		"payments": []any{
			map[string]any{"method": "mp_point", "amount_cents": 7500, "mp_charge_id": good},
			map[string]any{"method": "mp_point", "amount_cents": 7500, "mp_charge_id": good},
		}})
	if dup.Status != 400 {
		t.Fatalf("cobro repetido en la misma venta: %d %v", dup.Status, dup.Body)
	}

	// venta mixta: efectivo $75 + terminal $75
	s := m.mustSale(t, m.cashier, map[string]any{
		"lines": []any{svcLine(m.svc, m.marco)},
		"payments": []any{
			map[string]any{"method": "cash", "amount_cents": 7500},
			map[string]any{"method": "mp_point", "amount_cents": 7500, "mp_charge_id": good},
		}})
	var withCharge *salePaymentOut
	for i := range s.Payments {
		if s.Payments[i].Method == "mp_point" {
			withCharge = &s.Payments[i]
		}
	}
	if withCharge == nil || withCharge.MPChargeID != good {
		t.Fatalf("el pago debe guardar el cobro: %+v", s.Payments)
	}
	// ya usado: no sirve en otra venta (aunque el resto del pago cuadre)
	if r := m.sale(t, m.cashier, map[string]any{"lines": []any{svcLine(m.svc, m.marco)}, "payments": []any{
		map[string]any{"method": "cash", "amount_cents": 7500},
		map[string]any{"method": "mp_point", "amount_cents": 7500, "mp_charge_id": good}}}); r.Status != 409 {
		t.Fatalf("reutilizar el cobro: %d %v", r.Status, r.Body)
	}
	// el pago de terminal queda en el corte por método, pero no en el efectivo
	c := m.cashSummary(t)
	bm := map[string]int{}
	for _, mt := range c.ByMethod {
		bm[mt.Method] = mt.AmountCents
	}
	if bm["mp_point"] != 7500 || c.CashInCents != 7500 {
		t.Fatalf("caja: %+v", c)
	}
	// otro negocio no puede usar un cobro ajeno
	other := a.mustRegister(t, "Otro", "otro@x.com")
	ob := list[branch](t, a, "/api/branches", other)[0].ID
	a.do(t, "POST", "/api/cash/open", other, map[string]any{"branch_id": ob, "opening_cents": 0})
	osvc := id(a.do(t, "POST", "/api/services", other, map[string]any{"name": "X", "duration_min": 30, "price_cents": 7500}))
	spare := m.approved(t, 7500)
	if r := a.do(t, "POST", "/api/sales", other, map[string]any{"branch_id": ob, "lines": []any{map[string]any{"service_id": osvc, "qty": 1}},
		"payments": []any{map[string]any{"method": "mp_point", "amount_cents": 7500, "mp_charge_id": spare}}}); r.Status != 400 {
		t.Fatalf("usar cobro de otro negocio: %d %v", r.Status, r.Body)
	}
}

func TestVoidRefundsTerminalPayment(t *testing.T) {
	m := newMP(t, true)
	a := m.a
	cid := m.approved(t, 15000)
	s := m.mustSale(t, m.cashier, map[string]any{
		"lines":    []any{prdLine(m.cera, 1), svcLine(m.svc, m.marco)},
		"payments": []any{map[string]any{"method": "mp_point", "amount_cents": 27000, "mp_charge_id": m.approved(t, 27000)}},
	})
	_ = cid
	chargeOf := s.Payments[0].MPChargeID
	void := func() resp {
		return a.do(t, "POST", path("/api/sales/", s.ID, "/void"), m.admin, map[string]any{"reason": "Cliente se arrepintió"})
	}

	// si Mercado Pago no puede devolver el dinero, la venta NO se cancela
	m.mp.refundFail = "insufficient funds to refund"
	r := void()
	if r.Status != 502 || r.Body["code"] != "refund_failed" || !strings.Contains(r.Body["error"].(string), "insufficient funds") {
		t.Fatalf("reembolso fallido: %d %v", r.Status, r.Body)
	}
	var st string
	a.DB.QueryRow(context.Background(), `SELECT status FROM sales WHERE id = $1`, s.ID).Scan(&st)
	if st != "paid" || m.stock(t) != 4 {
		t.Fatalf("la venta debe seguir pagada con su existencia (estado %s, existencia %d)", st, m.stock(t))
	}
	// reintento: se devuelve el dinero y se cancela
	m.mp.refundFail = ""
	if r := void(); r.Status != 200 {
		t.Fatalf("cancelar: %d %v", r.Status, r.Body)
	}
	if m.mp.orders[chargeOf].refunded != 1 || m.stock(t) != 5 {
		t.Fatalf("reembolsos %d, existencia %d", m.mp.orders[chargeOf].refunded, m.stock(t))
	}
	var refundedAt *string
	a.DB.QueryRow(context.Background(), `SELECT refunded_at::text FROM mp_charges WHERE id = $1`, chargeOf).Scan(&refundedAt)
	if refundedAt == nil {
		t.Fatal("el cobro debe quedar marcado como reembolsado")
	}
	if r := void(); r.Status != 409 || m.mp.orders[chargeOf].refunded != 1 {
		t.Fatalf("segunda cancelación: %d, reembolsos %d", r.Status, m.mp.orders[chargeOf].refunded)
	}
}

func TestVoidSkipsAlreadyRefundedCharge(t *testing.T) {
	m := newMP(t, true)
	a := m.a
	cid := m.approved(t, 15000)
	s := m.mustSale(t, m.cashier, map[string]any{"lines": []any{svcLine(m.svc, m.marco)},
		"payments": []any{map[string]any{"method": "mp_point", "amount_cents": 15000, "mp_charge_id": cid}}})
	// un intento anterior ya devolvió el dinero pero no alcanzó a cancelar la venta
	a.DB.Exec(context.Background(), `UPDATE mp_charges SET refunded_at = now() WHERE id = $1`, cid)
	if r := a.do(t, "POST", path("/api/sales/", s.ID, "/void"), m.admin, map[string]any{"reason": "Reintento tras fallo"}); r.Status != 200 {
		t.Fatalf("cancelar: %d %v", r.Status, r.Body)
	}
	if m.mp.orders[cid].refunded != 0 {
		t.Fatalf("no debe devolver dos veces el mismo dinero (reembolsos: %d)", m.mp.orders[cid].refunded)
	}
	// y si Mercado Pago responde que ya estaba reembolsado, se tolera
	cid2 := m.approved(t, 15000)
	s2 := m.mustSale(t, m.cashier, map[string]any{"lines": []any{svcLine(m.svc, m.marco)},
		"payments": []any{map[string]any{"method": "mp_point", "amount_cents": 15000, "mp_charge_id": cid2}}})
	m.mp.refundFail = "the order was already refunded"
	if r := a.do(t, "POST", path("/api/sales/", s2.ID, "/void"), m.admin, map[string]any{"reason": "Ya reembolsado en MP"}); r.Status != 200 {
		t.Fatalf("ya reembolsado en MP: %d %v", r.Status, r.Body)
	}
}

// ---------- OAuth ----------

const testRedirect = "https://app.example.mx/api/point/oauth/callback"

func newOAuth(t *testing.T) *mpFx {
	t.Helper()
	m := newMP(t, false)
	m.a.MPClientID, m.a.MPClientSecret, m.a.MPRedirect = "CID", "CSECRET", testRedirect
	m.a.MPAuthBase = "https://auth.mercadopago.com.mx"
	return m
}

// callback simula al navegador volviendo de Mercado Pago (sin sesión: la ruta es pública).
func (m *mpFx) callback(query string) (int, string) {
	req := httptest.NewRequest("GET", "/api/point/oauth/callback?"+query, nil)
	rec := httptest.NewRecorder()
	m.a.Routes().ServeHTTP(rec, req)
	return rec.Code, rec.Header().Get("Location")
}

func (m *mpFx) connectState(t *testing.T, token string) string {
	t.Helper()
	r := m.a.do(t, "GET", "/api/payments/mercadopago/connect", token, nil)
	if r.Status != 200 {
		t.Fatalf("connect: %d %v", r.Status, r.Body)
	}
	u, err := url.Parse(r.Body["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("state")
}

func TestMPOAuthConnectURL(t *testing.T) {
	m := newMP(t, false)
	a := m.a
	if r := a.do(t, "GET", "/api/payments/mercadopago/connect", m.admin, nil); r.Status != 503 || r.Body["code"] != "mp_oauth_disabled" {
		t.Fatalf("sin configurar: %d %v", r.Status, r.Body)
	}
	if r := a.do(t, "GET", "/api/payments/mercadopago", m.admin, nil); r.Body["oauth_available"] != false {
		t.Fatalf("estado sin OAuth: %v", r.Body)
	}
	a.MPClientID, a.MPClientSecret, a.MPRedirect, a.MPAuthBase = "CID", "CSECRET", testRedirect, "https://auth.mercadopago.com.mx"

	r := a.do(t, "GET", "/api/payments/mercadopago/connect", m.admin, nil)
	if r.Status != 200 {
		t.Fatalf("connect: %d %v", r.Status, r.Body)
	}
	u, _ := url.Parse(r.Body["url"].(string))
	q := u.Query()
	if u.Host != "auth.mercadopago.com.mx" || u.Path != "/authorization" || q.Get("client_id") != "CID" || q.Get("response_type") != "code" ||
		q.Get("platform_id") != "mp" || q.Get("redirect_uri") != testRedirect || q.Get("state") == "" {
		t.Fatalf("dirección de autorización: %s", u)
	}
	var biz int64
	a.DB.QueryRow(context.Background(), `SELECT business_id FROM users WHERE email = 'admin@x.com'`).Scan(&biz)
	if got, ok := a.verifyState(q.Get("state")); !ok || got != biz {
		t.Fatalf("el estado debe identificar al negocio: %d %v", got, ok)
	}
	if r := a.do(t, "GET", "/api/payments/mercadopago", m.admin, nil); r.Body["oauth_available"] != true {
		t.Fatalf("estado con OAuth: %v", r.Body)
	}
	for name, tok := range map[string]string{"cajero": m.cashier, "profesional": m.staff} {
		if r := a.do(t, "GET", "/api/payments/mercadopago/connect", tok, nil); r.Status != 403 {
			t.Errorf("%s: %d", name, r.Status)
		}
	}
	if r := a.do(t, "GET", "/api/payments/mercadopago/connect", "", nil); r.Status != 401 {
		t.Errorf("sin sesión: %d", r.Status)
	}
	a.EncKey = nil // sin clave de cifrado tampoco se puede conectar
	if r := a.do(t, "GET", "/api/payments/mercadopago/connect", m.admin, nil); r.Status != 503 {
		t.Errorf("sin TOKEN_ENC_KEY: %d", r.Status)
	}
}

func TestMPOAuthCallback(t *testing.T) {
	m := newOAuth(t)
	a := m.a
	ctx := context.Background()
	state := m.connectState(t, m.admin)

	// el navegador vuelve con el código: conecta y regresa a Pagos
	code, loc := m.callback("code=CODE123&state=" + url.QueryEscape(state))
	if code != http.StatusFound || loc != "/panel/pagos?mp=ok" {
		t.Fatalf("callback: %d %s", code, loc)
	}
	var access, refresh []byte
	var src, user string
	var exp *time.Time
	if err := a.DB.QueryRow(ctx, `SELECT token_enc, refresh_enc, source, mp_user_id, expires_at FROM mp_accounts`).Scan(&access, &refresh, &src, &user, &exp); err != nil {
		t.Fatal(err)
	}
	if src != "oauth" || user != "777" || exp == nil || exp.Before(time.Now().Add(100*24*time.Hour)) {
		t.Fatalf("cuenta: %s %s %v", src, user, exp)
	}
	for _, blob := range [][]byte{access, refresh} {
		if bytes.Contains(blob, []byte("OAUTH-")) {
			t.Fatal("un token de OAuth quedó guardado en claro")
		}
	}
	if plain, _ := a.open(access); plain != "OAUTH-AT-1" {
		t.Fatalf("descifrado: %q", plain)
	}
	if r := a.do(t, "GET", "/api/payments/mercadopago", m.admin, nil); r.Body["connected"] != true || r.Body["source"] != "oauth" {
		t.Fatalf("estado: %v", r.Body)
	}
	if got := list[mpTerminal](t, a, "/api/payments/mercadopago/terminals", m.admin); len(got) != 1 {
		t.Fatalf("el token de OAuth debe servir para usar la terminal: %+v", got)
	}

	// errores: siempre regresan a Pagos explicando qué pasó, y no guardan nada nuevo
	other := a.mustRegister(t, "Otro", "otro@x.com")
	otherState := m.connectState(t, other)
	expired := func() string {
		saved := a.Now
		a.Now = func() time.Time { return time.Now().Add(-time.Hour) } // estado firmado hace una hora
		defer func() { a.Now = saved }()
		return a.signState(1)
	}()
	for name, c := range map[string]struct{ query, reason string }{
		"código incorrecto":  {"code=MAL&state=" + url.QueryEscape(otherState), "oauth_failed"},
		"estado alterado":    {"code=CODE123&state=" + url.QueryEscape(otherState+"x"), "state"},
		"estado vencido":     {"code=CODE123&state=" + url.QueryEscape(expired), "state"},
		"estado inventado":   {"code=CODE123&state=abc.def", "state"},
		"sin estado":         {"code=CODE123", "state"},
		"sin código":         {"state=" + url.QueryEscape(otherState), "cancelled"},
		"el usuario canceló": {"error=access_denied&state=" + url.QueryEscape(otherState), "cancelled"},
	} {
		code, loc := m.callback(c.query)
		if code != http.StatusFound || loc != "/panel/pagos?mp=error&reason="+c.reason {
			t.Errorf("%s: %d %s (esperaba reason=%s)", name, code, loc, c.reason)
		}
	}
	var n int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM mp_accounts`).Scan(&n)
	if n != 1 {
		t.Fatalf("los intentos fallidos no deben crear cuentas (hay %d)", n)
	}
	// un estado firmado con otro secreto no sirve
	a.Secret = "otro-secreto-distinto-de-al-menos-32-caracteres"
	forged := a.signState(1)
	a.Secret = testSecret
	if _, loc := m.callback("code=CODE123&state=" + url.QueryEscape(forged)); !strings.Contains(loc, "reason=state") {
		t.Errorf("estado con otro secreto: %s", loc)
	}
	// el estado conecta SOLO al negocio que lo pidió
	if code, loc := m.callback("code=CODE123&state=" + url.QueryEscape(otherState)); loc != "/panel/pagos?mp=ok" || code != 302 {
		t.Fatalf("el otro negocio debe poder conectar con su propio estado: %s", loc)
	}
	a.DB.QueryRow(ctx, `SELECT count(*) FROM mp_accounts`).Scan(&n)
	if n != 2 {
		t.Fatalf("cada negocio con su cuenta: %d", n)
	}
}

func TestMPTokenRefresh(t *testing.T) {
	m := newOAuth(t)
	a := m.a
	ctx := context.Background()
	m.mp.expiresIn = 3600 // vence en 1 hora: hay que renovarlo
	state := m.connectState(t, m.admin)
	if _, loc := m.callback("code=CODE123&state=" + url.QueryEscape(state)); loc != "/panel/pagos?mp=ok" {
		t.Fatal(loc)
	}
	m.mp.expiresIn = 15552000 // los tokens renovados duran 180 días

	// el primer uso renueva; el segundo ya no
	if got := list[mpTerminal](t, a, "/api/payments/mercadopago/terminals", m.admin); len(got) != 1 {
		t.Fatalf("terminales tras renovar: %+v", got)
	}
	if m.mp.refreshCalls != 1 {
		t.Fatalf("renovaciones: %d", m.mp.refreshCalls)
	}
	var blob []byte
	var exp time.Time
	a.DB.QueryRow(ctx, `SELECT token_enc, expires_at FROM mp_accounts`).Scan(&blob, &exp)
	if plain, _ := a.open(blob); plain != "OAUTH-AT-2" || time.Until(exp) < 100*24*time.Hour {
		t.Fatalf("token renovado: %q vence %v", plain, exp)
	}
	list[mpTerminal](t, a, "/api/payments/mercadopago/terminals", m.admin)
	if m.mp.refreshCalls != 1 {
		t.Fatalf("no debe renovar de nuevo: %d", m.mp.refreshCalls)
	}

	// varias peticiones a la vez con el token por vencer: se renueva UNA vez (el de renovación es de un solo uso)
	a.DB.Exec(ctx, `UPDATE mp_accounts SET expires_at = now() + interval '1 hour'`)
	var wg sync.WaitGroup
	statuses := make([]int, 6)
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			statuses[i] = a.do(t, "GET", "/api/payments/mercadopago/terminals", m.admin, nil).Status
		}(i)
	}
	wg.Wait()
	for i, st := range statuses {
		if st != 200 {
			t.Errorf("petición simultánea %d: %d", i, st)
		}
	}
	if m.mp.refreshCalls != 2 {
		t.Fatalf("la renovación simultánea debe ocurrir una sola vez (renovaciones totales %d, esperaba 2)", m.mp.refreshCalls)
	}

	// si renovar falla pero el token aún sirve, se sigue usando
	a.DB.Exec(ctx, `UPDATE mp_accounts SET expires_at = now() + interval '1 hour'`)
	m.mp.refreshFail = true
	if r := a.do(t, "GET", "/api/payments/mercadopago/terminals", m.admin, nil); r.Status != 200 {
		t.Fatalf("renovación fallida con token vigente: %d %v", r.Status, r.Body)
	}
	// si además ya venció, hay que reconectar
	a.DB.Exec(ctx, `UPDATE mp_accounts SET expires_at = now() - interval '1 hour'`)
	if r := a.do(t, "GET", "/api/payments/mercadopago/terminals", m.admin, nil); r.Status != 502 || r.Body["code"] != "mp_token_revoked" {
		t.Fatalf("token vencido y renovación fallida: %d %v", r.Status, r.Body)
	}
	m.mp.refreshFail = false

	// pegar un token a mano reemplaza la conexión por OAuth y borra sus datos
	if r := a.do(t, "PUT", "/api/payments/mercadopago/token", m.admin, map[string]any{"access_token": mpToken}); r.Status != 204 {
		t.Fatalf("token manual: %d %v", r.Status, r.Body)
	}
	var src string
	var rt []byte
	a.DB.QueryRow(ctx, `SELECT source, refresh_enc FROM mp_accounts`).Scan(&src, &rt)
	if src != "token" || rt != nil {
		t.Fatalf("tras pegar el token: %s %v", src, rt)
	}
}
