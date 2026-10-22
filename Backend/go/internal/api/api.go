package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"miestudio/backend-go/internal/auth"
	"miestudio/backend-go/internal/mail"
	"miestudio/backend-go/internal/ratelimit"
	"miestudio/backend-go/internal/whatsapp"
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

	PublicURL       string // dirección pública de la API (sin /api): para los avisos automáticos de Mercado Pago
	MPWebhookSecret string // firma de los avisos; vacío = no se verifica

	PlatformWebhookSecret string   // MPP_WEBHOOK_SECRET: firma de los avisos de pagos de suscripción
	PlatformNoticeTo      string   // PLATFORM_NOTICE_EMAIL: a quién avisan los pagos recibidos
	PlatformMPToken       string   // MPP_ACCESS_TOKEN: cuenta de Mercado Pago de la plataforma, para cobrar suscripciones
	Operator              Operator // datos de quien opera la plataforma (aviso de privacidad)

	Mailer mail.Sender // nil o sin configurar: no se envían correos

	WhatsApp      whatsapp.Sender // nil o sin configurar: no se envían recordatorios por WhatsApp
	WAVerifyToken string          // WHATSAPP_VERIFY_TOKEN: reto con el que Meta verifica el webhook
	WAAppSecret   string          // WHATSAPP_APP_SECRET: firma de los mensajes que llegan; sin él no hay webhook
	AppURL        string          // base de los enlaces de los correos (https://tu-dominio)
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
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("POST /api/auth/login/2fa", a.loginSecondStep)
	mux.HandleFunc("POST /api/auth/forgot", a.forgotPassword)
	mux.HandleFunc("POST /api/auth/reset", a.resetPassword)
	mux.HandleFunc("POST /api/webhooks/mercadopago", a.mpWebhook)
	mux.HandleFunc("POST /api/webhooks/mercadopago-platform", a.platformMPWebhook)
	mux.HandleFunc("GET /api/point/oauth/callback", a.mpCallback) // el navegador vuelve aquí desde Mercado Pago

	// reserva en línea (sin sesión)
	mux.HandleFunc("GET /api/public-appointments/{token}", a.publicAppointment)
	mux.HandleFunc("POST /api/public-appointments/{token}/cancel", a.publicCancel)
	mux.HandleFunc("POST /api/public-appointments/{token}/confirm", a.publicConfirm)
	mux.HandleFunc("POST /api/public-appointments/{token}/proof", a.publicProof)
	mux.HandleFunc("POST /api/public-appointments/{token}/reminders-off", a.publicRemindersOff)
	mux.HandleFunc("GET /api/public/{slug}", a.publicInfo)
	mux.HandleFunc("GET /api/public/{slug}/privacy", a.publicPrivacy)
	mux.HandleFunc("POST /api/public/{slug}/arco", a.publicArco)
	mux.HandleFunc("GET /api/public-arco/{token}", a.publicArcoStatus)
	mux.HandleFunc("GET /api/legal", a.legalInfo)
	mux.HandleFunc("GET /api/public/{slug}/slots", a.publicSlots)
	mux.HandleFunc("GET /api/public/{slug}/month", a.publicMonth)
	mux.HandleFunc("POST /api/public/{slug}/holds", a.publicHold)
	mux.HandleFunc("POST /api/public/{slug}/bookings", a.publicBook)
	mux.HandleFunc("POST /api/public/{slug}/waitlist", a.publicWaitlist)
	mux.HandleFunc("GET /api/webhooks/whatsapp", a.whatsappVerify)
	mux.HandleFunc("POST /api/webhooks/whatsapp", a.whatsappWebhook)
	mux.HandleFunc("GET /api/public-survey/{token}", a.publicSurveyInfo)
	mux.HandleFunc("POST /api/public-survey/{token}", a.publicSurveyAnswer)
	mux.HandleFunc("GET /api/public-unsub/{token}", a.publicUnsubInfo)
	mux.HandleFunc("POST /api/public-unsub/{token}", a.publicUnsub)
	mux.HandleFunc("GET /api/public-waitlist/{token}", a.publicWaitlistInfo)
	mux.HandleFunc("POST /api/public-waitlist/{token}/leave", a.publicWaitlistLeave)

	// cualquier sesión (el estado de la cuenta se informa en /api/me)
	mux.Handle("GET /api/me", a.authed()(http.HandlerFunc(a.me)))
	mux.Handle("POST /api/auth/logout-all", a.authed()(http.HandlerFunc(a.logoutAll)))
	mux.Handle("GET /api/me/2fa", a.authed()(http.HandlerFunc(a.twoFactorStatus)))
	mux.Handle("POST /api/me/2fa/setup", a.authed()(http.HandlerFunc(a.twoFactorSetup)))
	mux.Handle("POST /api/me/2fa/enable", a.authed()(http.HandlerFunc(a.twoFactorEnable)))
	mux.Handle("POST /api/me/2fa/disable", a.authed()(http.HandlerFunc(a.twoFactorDisable)))
	mux.Handle("POST /api/me/2fa/recovery", a.authed()(http.HandlerFunc(a.twoFactorRegenerate)))
	mux.Handle("POST /api/auth/password", a.authed()(http.HandlerFunc(a.changePassword)))

	// negocio: configuración de la reserva en línea
	mux.Handle("GET /api/booking/settings", a.tenant(a.getBookingSettings, admin))
	mux.Handle("PUT /api/booking/settings", a.tenant(a.putBookingSettings, admin))

	// negocio: suscripción (disponible aun con la suscripción vencida)
	mux.Handle("GET /api/billing", a.billingAdmin(a.getBilling))
	mux.Handle("POST /api/billing/checkout", a.billingAdmin(a.startCheckout))
	mux.Handle("GET /api/billing/checkouts/{id}", a.billingAdmin(a.getCheckout))

	// negocio: privacidad (derechos ARCO y datos de clientes)
	mux.Handle("GET /api/arco", a.tenant(a.listArco, admin))
	mux.Handle("PATCH /api/arco/{id}", a.tenant(a.updateArco, admin))
	mux.Handle("GET /api/clients/{id}/export", a.tenant(a.exportClient, admin))
	mux.Handle("POST /api/clients/{id}/erase", a.tenant(a.eraseClient, admin))

	// negocio: datos del negocio
	mux.Handle("GET /api/business", a.tenant(a.getBusiness, admin))
	mux.Handle("PUT /api/business", a.tenant(a.putBusiness, admin))
	mux.Handle("POST /api/business/setup/complete", a.tenant(a.completeSetup, admin))

	// negocio: mensajes a clientes (cumpleaños, reactivación, encuesta con reseña)
	mux.Handle("GET /api/engagement", a.tenant(a.getEngagement, admin))
	mux.Handle("PUT /api/engagement", a.tenant(a.putEngagement, admin))
	mux.Handle("GET /api/surveys", a.tenant(a.listSurveys, admin))

	// negocio: equipo
	mux.Handle("GET /api/team", a.tenant(a.listTeam, admin))
	mux.Handle("POST /api/team", a.tenant(a.createTeamMember, admin))
	mux.Handle("POST /api/team/{id}/2fa/reset", a.tenant(a.resetTeamTwoFactor, admin))
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
	mux.Handle("POST /api/appointments/{id}/confirm", a.tenant(a.markAppointment("confirmed_at"), book...))
	mux.Handle("POST /api/appointments/{id}/arrive", a.tenant(a.markAppointment("arrived_at"), book...))
	mux.Handle("POST /api/appointments/{id}/proof", a.tenant(a.staffProof, book...))
	mux.Handle("GET /api/appointments/{id}/proof", a.tenant(a.getProof, admin, "reception"))
	mux.Handle("POST /api/appointments/{id}/transfer/confirm", a.tenant(a.confirmTransfer, admin))
	mux.Handle("POST /api/appointments/{id}/transfer/reject", a.tenant(a.rejectTransfer, admin))
	mux.Handle("POST /api/appointments/{id}/transfer/refunded", a.tenant(a.transferRefunded, admin))
	mux.Handle("GET /api/transfer-info", a.tenant(a.getTransferInfo, book...))
	mux.Handle("GET /api/inbox", a.tenant(a.getInbox, admin, "reception"))
	mux.Handle("POST /api/inbox/read", a.tenant(a.readInbox, admin, "reception"))
	mux.Handle("GET /api/waitlist", a.tenant(a.listWaitlist, book...))
	mux.Handle("POST /api/waitlist", a.tenant(a.createWaitlist, book...))
	mux.Handle("DELETE /api/waitlist/{id}", a.tenant(a.deleteWaitlist, book...))
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
	mux.Handle("GET /api/packages", a.tenant(a.listPackages, sell...))
	mux.Handle("POST /api/packages", a.tenant(a.createPackage, admin))
	mux.Handle("PATCH /api/packages/{id}", a.tenant(a.updatePackage, admin))
	mux.Handle("GET /api/clients/{id}/packages", a.tenant(a.listClientPackages, sell...))
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
	mux.Handle("POST /api/platform/users/{id}/2fa-reset", a.platform(a.resetPlatformTwoFactor, write))
	mux.Handle("GET /api/platform/stats", a.platform(a.platformOverview, both...))
	mux.Handle("GET /api/platform/overview", a.platform(a.platformOverview, both...))
	mux.Handle("GET /api/platform/plans", a.platform(a.platformPlans, both...))
	mux.Handle("GET /api/platform/payments", a.platform(a.platformPayments, write))
	mux.Handle("GET /api/platform/health", a.platform(a.platformHealth, write))
	mux.Handle("POST /api/platform/businesses/{id}/payments", a.platform(a.platformRecordPayment, write))
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
	var trialEnds, periodEnd *time.Time
	err := a.DB.QueryRow(ctx,
		`SELECT COALESCE(u.business_id, 0), u.role, COALESCE(b.status, ''), u.session_version, b.trial_ends_at, b.current_period_end
		 FROM users u LEFT JOIN businesses b ON b.id = u.business_id
		 WHERE u.id = $1 AND u.active`, userID).Scan(&p.BusinessID, &p.Role, &p.BusinessStatus, &p.SessionVersion, &trialEnds, &periodEnd)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err == nil && p.BusinessID != 0 {
		p.BillingState = BillingState(p.BusinessStatus, trialEnds, periodEnd, a.now())
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
