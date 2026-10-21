package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
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
}

type fakeOrder struct {
	status, paid string
	refunded     int
}

func newFakeMP(t *testing.T) *fakeMP {
	f := &fakeMP{orders: map[string]*fakeOrder{}, terminals: []mpTerminal{{ID: "NEWLAND_N950__N950NCB000001", ExternalPosID: "CAJA1", OperatingMode: "STANDALONE"}}}
	mux := http.NewServeMux()
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.calls = append(f.calls, r.Method+" "+r.URL.Path)
			if r.Header.Get("Authorization") != "Bearer "+mpToken {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"message":"invalid access token","status":401}`))
				return
			}
			h(w, r)
		}
	}
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
