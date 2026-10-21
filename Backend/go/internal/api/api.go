package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"miestudio/backend-go/internal/auth"
	"miestudio/backend-go/internal/ratelimit"
)

type API struct {
	DB         *pgxpool.Pool
	Secret     string
	Limiter    *ratelimit.Limiter // nil desactiva el límite de intentos
	StaticDir  string             // si se define, sirve el frontend compilado
	Now        func() time.Time   // reloj inyectable para pruebas; nil usa time.Now
	Production bool               // rechaza claves de ejemplo en el alta del administrador
	EncKey     []byte             // 32 bytes: cifra los tokens de Mercado Pago (vacío = integración deshabilitada)
	MPBase     string             // URL base de Mercado Pago (para pruebas); vacío usa la oficial

	// OAuth de Mercado Pago (aplicación de la plataforma)
	OAuthState     []byte // firma del state del OAuth; vacío = se deriva de Secret
	MPClientID     string
	MPClientSecret string
	MPRedirect     string // debe coincidir exactamente con la registrada en la aplicación de Mercado Pago
	MPAuthBase     string
}

// authed exige sesión válida (negocio o plataforma).
func (a *API) authed() func(http.Handler) http.Handler { return auth.Middleware(a.Secret, a.lookup) }

// tenant exige un usuario de negocio con cuenta en uso y, si se indican, uno de los roles.
func (a *API) tenant(h http.HandlerFunc, roles ...string) http.Handler {
	var next http.Handler = h
	if len(roles) > 0 {
		next = auth.RequireRole(roles...)(next)
	}
	return a.authed()(auth.RequireTenant(next))
}

// platform exige un usuario de plataforma con uno de los roles indicados.
func (a *API) platform(h http.HandlerFunc, roles ...string) http.Handler {
	return a.authed()(auth.RequireRole(roles...)(h))
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	const admin = "admin"
	both := []string{auth.RolePlatformAdmin, auth.RolePlatformSupport}
	write := auth.RolePlatformAdmin

	// públicas
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("GET /api/auth/registration", a.registrationInfo)
	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("GET /api/point/oauth/callback", a.mpCallback) // el navegador vuelve aquí desde Mercado Pago

	// reserva en línea (sin sesión)
	mux.HandleFunc("GET /api/public-appointments/{token}", a.publicAppointment)
	mux.HandleFunc("POST /api/public-appointments/{token}/cancel", a.publicCancel)
	mux.HandleFunc("GET /api/public/{slug}", a.publicInfo)
	mux.HandleFunc("GET /api/public/{slug}/slots", a.publicSlots)
	mux.HandleFunc("POST /api/public/{slug}/bookings", a.publicBook)

	// cualquier sesión (el estado de la cuenta se informa en /api/me)
	mux.Handle("GET /api/me", a.authed()(http.HandlerFunc(a.me)))
	mux.Handle("POST /api/auth/password", a.authed()(http.HandlerFunc(a.changePassword)))

	// negocio: configuración de la reserva en línea
	mux.Handle("GET /api/booking/settings", a.tenant(a.getBookingSettings, admin))
	mux.Handle("PUT /api/booking/settings", a.tenant(a.putBookingSettings, admin))

	// negocio: equipo
	mux.Handle("GET /api/team", a.tenant(a.listTeam, admin))
	mux.Handle("POST /api/team", a.tenant(a.createTeamMember, admin))
	mux.Handle("PATCH /api/team/{id}", a.tenant(a.updateTeamMember, admin))
	mux.Handle("GET /api/team/{id}/hours", a.tenant(a.getHours, admin))
	mux.Handle("PUT /api/team/{id}/hours", a.tenant(a.setHours, admin))

	// negocio: catálogo (lectura para todo el equipo, cambios solo del administrador)
	mux.Handle("GET /api/services", a.tenant(a.listServices))
	mux.Handle("POST /api/services", a.tenant(a.createService, admin))
	mux.Handle("PATCH /api/services/{id}", a.tenant(a.updateService, admin))
	mux.Handle("PUT /api/services/{id}/staff", a.tenant(a.setServiceStaff, admin))
	mux.Handle("GET /api/products", a.tenant(a.listProducts))
	mux.Handle("POST /api/products", a.tenant(a.createProduct, admin))
	mux.Handle("PATCH /api/products/{id}", a.tenant(a.updateProduct, admin))
	mux.Handle("GET /api/branches", a.tenant(a.listBranches))
	mux.Handle("POST /api/branches", a.tenant(a.createBranch, admin))
	mux.Handle("PATCH /api/branches/{id}", a.tenant(a.updateBranch, admin))

	// negocio: clientes (notas de seguimiento solo administrador y profesionales)
	mux.Handle("GET /api/clients", a.tenant(a.listClients))
	mux.Handle("POST /api/clients", a.tenant(a.createClient))
	mux.Handle("GET /api/clients/{id}", a.tenant(a.getClient))
	mux.Handle("PATCH /api/clients/{id}", a.tenant(a.updateClient))
	mux.Handle("GET /api/clients/{id}/notes", a.tenant(a.listNotes, admin, "staff"))
	mux.Handle("POST /api/clients/{id}/notes", a.tenant(a.createNote, admin, "staff"))

	// negocio: agenda (todos leen; agendar y bloquear: administrador, recepción y profesionales)
	book := []string{admin, "reception", "staff"}
	mux.Handle("GET /api/agenda", a.tenant(a.agendaDay))
	mux.Handle("GET /api/agenda/slots", a.tenant(a.slots))
	mux.Handle("POST /api/appointments", a.tenant(a.createAppointment, book...))
	mux.Handle("PATCH /api/appointments/{id}", a.tenant(a.updateAppointment, book...))
	mux.Handle("POST /api/appointments/{id}/move", a.tenant(a.moveAppointment, book...))
	mux.Handle("POST /api/blocks", a.tenant(a.createBlock, book...))
	mux.Handle("DELETE /api/blocks/{id}", a.tenant(a.deleteBlock, book...))

	// negocio: punto de venta. Cobran administrador, caja y recepción; los profesionales no.
	sell := []string{admin, "cashier", "reception"}
	mux.Handle("GET /api/cash/current", a.tenant(a.currentCash, sell...))
	mux.Handle("POST /api/cash/open", a.tenant(a.openCash, sell...))
	mux.Handle("POST /api/cash/movements", a.tenant(a.cashMovement, sell...))
	mux.Handle("POST /api/cash/close", a.tenant(a.closeCash, sell...))
	mux.Handle("GET /api/cash/sessions", a.tenant(a.listCashSessions, sell...))
	mux.Handle("POST /api/sales", a.tenant(a.createSale, sell...))
	mux.Handle("GET /api/sales", a.tenant(a.listSales, sell...))
	mux.Handle("GET /api/sales/{id}", a.tenant(a.getSale, sell...))
	mux.Handle("POST /api/sales/{id}/void", a.tenant(a.voidSale, admin))
	mux.Handle("POST /api/products/{id}/stock", a.tenant(a.adjustStock, admin))
	mux.Handle("GET /api/products/{id}/movements", a.tenant(a.listStockMovements, admin))
	mux.Handle("GET /api/commission-rules", a.tenant(a.listRules, admin))
	mux.Handle("POST /api/commission-rules", a.tenant(a.createRule, admin))
	mux.Handle("PATCH /api/commission-rules/{id}", a.tenant(a.updateRule, admin))

	// negocio: reportes (ventas: administrador; comisiones: administrador y profesionales, cada quien lo suyo)
	mux.Handle("GET /api/reports/sales", a.tenant(a.salesReport, admin))
	mux.Handle("GET /api/reports/commissions", a.tenant(a.commissionsReport, admin, "staff"))

	// negocio: terminal Mercado Pago
	mux.Handle("GET /api/payments/mercadopago", a.tenant(a.mpStatus, admin))
	mux.Handle("GET /api/payments/mercadopago/connect", a.tenant(a.mpConnectURL, admin))
	mux.Handle("PUT /api/payments/mercadopago/token", a.tenant(a.mpConnect, admin))
	mux.Handle("DELETE /api/payments/mercadopago", a.tenant(a.mpDisconnect, admin))
	mux.Handle("GET /api/payments/mercadopago/terminals", a.tenant(a.mpTerminals, admin))
	mux.Handle("PUT /api/payments/mercadopago/terminals", a.tenant(a.mpRegisterTerminal, admin))
	mux.Handle("GET /api/payments/point/status", a.tenant(a.pointStatus, sell...))
	mux.Handle("POST /api/payments/point/charges", a.tenant(a.pointCharge, sell...))
	mux.Handle("GET /api/payments/point/charges/{id}", a.tenant(a.pointChargeStatus, sell...))
	mux.Handle("POST /api/payments/point/charges/{id}/cancel", a.tenant(a.pointChargeCancel, sell...))

	// plataforma
	mux.Handle("GET /api/platform/stats", a.platform(a.platformStats, both...))
	mux.Handle("GET /api/platform/businesses", a.platform(a.platformListBusinesses, both...))
	mux.Handle("POST /api/platform/businesses", a.platform(a.platformCreateBusiness, write))
	mux.Handle("GET /api/platform/businesses/{id}", a.platform(a.platformGetBusiness, both...))
	mux.Handle("PATCH /api/platform/businesses/{id}", a.platform(a.platformUpdateBusiness, write))
	mux.Handle("POST /api/platform/users/{id}/password", a.platform(a.platformResetPassword, write))
	mux.Handle("PATCH /api/platform/users/{id}", a.platform(a.platformUpdateUser, write))
	mux.Handle("GET /api/platform/team", a.platform(a.platformListTeam, both...))
	mux.Handle("POST /api/platform/team", a.platform(a.platformCreateTeam, write))
	mux.Handle("PATCH /api/platform/team/{id}", a.platform(a.platformUpdateTeam, write))
	mux.Handle("GET /api/platform/settings", a.platform(a.platformGetSettings, both...))
	mux.Handle("PUT /api/platform/settings", a.platform(a.platformPutSettings, write))
	mux.Handle("GET /api/platform/activity", a.platform(a.platformActivity, both...))

	mux.Handle("/api/", http.NotFoundHandler())
	if a.StaticDir != "" {
		mux.Handle("/", spa(a.StaticDir))
	}
	return mux
}

// lookup lee de la base el rol, el negocio y el estado de la cuenta actuales del usuario.
func (a *API) lookup(ctx context.Context, userID int64) (*auth.Principal, error) {
	p := &auth.Principal{UserID: userID}
	err := a.DB.QueryRow(ctx,
		`SELECT COALESCE(u.business_id, 0), u.role, COALESCE(b.status, '')
		 FROM users u LEFT JOIN businesses b ON b.id = u.business_id
		 WHERE u.id = $1 AND u.active`, userID).Scan(&p.BusinessID, &p.Role, &p.BusinessStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	if err := a.DB.Ping(r.Context()); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

// spa sirve archivos estáticos y devuelve index.html para rutas del cliente.
func spa(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			w.Header().Set("Cache-Control", staticCache(r.URL.Path))
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}

// staticCache: los archivos con huella (/_app/immutable) no cambian nunca; lo demás (index.html, version.json)
// se revalida siempre, para que una versión nueva se note en cuanto se publica.
func staticCache(urlPath string) string {
	if strings.HasPrefix(urlPath, "/_app/immutable/") {
		return "public, max-age=31536000, immutable"
	}
	return "no-cache"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(v) == nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
