package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	StaticDir   string
	Production  bool   // APP_ENV=production
	EncKey      []byte // TOKEN_ENC_KEY (hex de 64 caracteres o base64 de 32 bytes): cifra los tokens de Mercado Pago
	OAuthState  []byte // OAUTH_STATE_SECRET (mismo formato): firma el parámetro state del OAuth
	MPBase      string // MP_API_BASE (opcional)

	// OAuth de Mercado Pago (aplicación de la plataforma). Vacío = solo token manual.
	MPClientID                                               string // MP_CLIENT_ID
	MPClientSecret                                           string // MP_CLIENT_SECRET
	MPRedirect                                               string // MP_OAUTH_REDIRECT o API_PUBLIC_URL + /api/point/oauth/callback
	MPAuthBase                                               string // MP_AUTH_BASE (por defecto México)
	PlatformMPToken, PlatformWebhookSecret, PlatformNoticeTo string // MPP_ACCESS_TOKEN, MPP_WEBHOOK_SECRET, PLATFORM_NOTICE_EMAIL
	PublicURL                                                string // API_PUBLIC_URL sin /api: destino de los avisos automáticos de Mercado Pago
	MPWebhookSecret                                          string // MP_WEBHOOK_SECRET (opcional): firma de los avisos

	// Correo saliente (confirmaciones y recordatorios). Sin SMTP_HOST y MAIL_FROM no se envía nada.
	SMTPHost, SMTPUser, SMTPPass, MailFrom string
	SMTPPort                               int
	SignupVerifyOff                        bool // SIGNUP_VERIFY=off: no pedir el código del correo al registrarse
	SMTPSecure                             bool

	// WhatsApp Business (API en la nube de Meta). Sin WHATSAPP_TOKEN y WHATSAPP_PHONE_ID no se envía nada.
	WhatsAppToken, WhatsAppPhoneID, WhatsAppTemplate, WhatsAppLang, WhatsAppBase string
	WhatsAppVerifyToken, WhatsAppAppSecret                                       string
	AppURL                                                                       string // dirección pública de la app: se usa en los enlaces de los correos
	LegalName, LegalAddress, LegalEmail                                          string // datos de quien opera la plataforma, para el aviso de privacidad
	TrustedProxies                                                               string // TRUSTED_PROXIES: proxies de los que se cree X-Forwarded-For (IP real y https)

	// primer administrador de plataforma (opcional; se crea solo si no existe ninguno)
	PlatformAdminName     string
	PlatformAdminEmail    string
	PlatformAdminPassword string

	// negocio de ejemplo para pruebas (opcional; solo se crea en la primera ejecución, con la base sin negocios)
	DemoBusiness, DemoKind, DemoName, DemoEmail, DemoPassword string
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		Port:        os.Getenv("GO_API_PORT"),
		StaticDir:   os.Getenv("STATIC_DIR"),
		Production:  os.Getenv("APP_ENV") == "production",

		PlatformAdminName:     os.Getenv("PLATFORM_ADMIN_NAME"),
		PlatformAdminEmail:    os.Getenv("PLATFORM_ADMIN_EMAIL"),
		PlatformAdminPassword: os.Getenv("PLATFORM_ADMIN_PASSWORD"),
	}
	c.DemoBusiness, c.DemoKind = strings.TrimSpace(os.Getenv("DEMO_BUSINESS_NAME")), strings.TrimSpace(os.Getenv("DEMO_BUSINESS_KIND"))
	c.DemoName, c.DemoEmail, c.DemoPassword = strings.TrimSpace(os.Getenv("DEMO_ADMIN_NAME")), strings.TrimSpace(os.Getenv("DEMO_ADMIN_EMAIL")), os.Getenv("DEMO_ADMIN_PASSWORD")
	c.MPBase = os.Getenv("MP_API_BASE")
	c.MPClientID, c.MPClientSecret = os.Getenv("MP_CLIENT_ID"), os.Getenv("MP_CLIENT_SECRET")
	c.MPRedirect = os.Getenv("MP_OAUTH_REDIRECT")
	if c.MPRedirect == "" { // API_PUBLIC_URL se acepta con o sin el sufijo /api
		if base := strings.TrimSuffix(strings.TrimRight(os.Getenv("API_PUBLIC_URL"), "/"), "/api"); base != "" {
			c.MPRedirect = base + "/api/point/oauth/callback"
		}
	}
	c.PlatformMPToken, c.PlatformWebhookSecret = strings.TrimSpace(os.Getenv("MPP_ACCESS_TOKEN")), strings.TrimSpace(os.Getenv("MPP_WEBHOOK_SECRET"))
	c.PlatformNoticeTo = strings.TrimSpace(os.Getenv("PLATFORM_NOTICE_EMAIL"))
	c.PublicURL = strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(os.Getenv("API_PUBLIC_URL")), "/"), "/api")
	c.MPWebhookSecret = strings.TrimSpace(os.Getenv("MP_WEBHOOK_SECRET"))
	c.MPAuthBase = strings.TrimRight(os.Getenv("MP_AUTH_BASE"), "/")
	if c.MPAuthBase == "" {
		c.MPAuthBase = "https://auth.mercadopago.com.mx"
	}
	c.SMTPHost, c.SMTPUser, c.SMTPPass = strings.TrimSpace(os.Getenv("SMTP_HOST")), os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS")
	c.SMTPSecure = strings.EqualFold(os.Getenv("SMTP_SECURE"), "true")
	c.SMTPPort = 587
	if c.SMTPSecure {
		c.SMTPPort = 465
	}
	if v := strings.TrimSpace(os.Getenv("SMTP_PORT")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 65535 {
			return c, fmt.Errorf("SMTP_PORT debe ser un número de puerto válido")
		}
		c.SMTPPort = n
	}
	c.SignupVerifyOff = strings.EqualFold(strings.TrimSpace(os.Getenv("SIGNUP_VERIFY")), "off")
	c.MailFrom = strings.TrimSpace(os.Getenv("MAIL_FROM"))
	if c.MailFrom == "" && c.SMTPUser != "" && strings.Contains(c.SMTPUser, "@") {
		c.MailFrom = "Barbella <" + c.SMTPUser + ">"
	}
	c.WhatsAppToken, c.WhatsAppPhoneID = strings.TrimSpace(os.Getenv("WHATSAPP_TOKEN")), strings.TrimSpace(os.Getenv("WHATSAPP_PHONE_ID"))
	c.WhatsAppTemplate, c.WhatsAppLang = strings.TrimSpace(os.Getenv("WHATSAPP_TEMPLATE")), strings.TrimSpace(os.Getenv("WHATSAPP_LANG"))
	if c.WhatsAppTemplate == "" {
		c.WhatsAppTemplate = "recordatorio_cita"
	}
	if c.WhatsAppLang == "" {
		c.WhatsAppLang = "es_MX"
	}
	c.WhatsAppBase = strings.TrimRight(strings.TrimSpace(os.Getenv("WHATSAPP_API_BASE")), "/")
	c.WhatsAppVerifyToken, c.WhatsAppAppSecret = strings.TrimSpace(os.Getenv("WHATSAPP_VERIFY_TOKEN")), strings.TrimSpace(os.Getenv("WHATSAPP_APP_SECRET"))
	c.LegalName, c.LegalAddress, c.LegalEmail = strings.TrimSpace(os.Getenv("LEGAL_NAME")), strings.TrimSpace(os.Getenv("LEGAL_ADDRESS")), strings.TrimSpace(os.Getenv("LEGAL_EMAIL"))
	if c.LegalName == "" {
		c.LegalName = "Hyperion" // quien opera la plataforma
	}
	if c.LegalEmail == "" {
		c.LegalEmail = "caresia-info@caresia.mx" // correo de contacto para privacidad y dudas
	}
	c.TrustedProxies = strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	c.AppURL = strings.TrimRight(strings.TrimSpace(os.Getenv("APP_URL")), "/")
	if c.AppURL == "" {
		c.AppURL = "http://localhost:5173"
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL es obligatorio")
	}
	if len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("JWT_SECRET es obligatorio (mínimo 32 caracteres)")
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	var err error
	if c.EncKey, err = secretKey("TOKEN_ENC_KEY", c.JWTSecret, "token-enc"); err != nil {
		return c, err
	}
	if c.OAuthState, err = secretKey("OAUTH_STATE_SECRET", c.JWTSecret, "oauth-state"); err != nil {
		return c, err
	}
	return c, nil
}

// secretKey lee una clave de 32 bytes del entorno: hexadecimal de 64 caracteres (openssl rand -hex 32)
// o base64 de 32 bytes (openssl rand -base64 32). Si no está definida, se deriva de JWT_SECRET, de modo
// que todo funciona sin configurarla. Un valor definido pero inválido es un error: se prefiere fallar
// al arrancar que cifrar con una clave distinta a la que se esperaba.
func secretKey(name, jwtSecret, purpose string) ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		sum := sha256.Sum256([]byte(purpose + ":" + jwtSecret))
		return sum[:], nil
	}
	if key, err := hex.DecodeString(raw); err == nil && len(key) == 32 {
		return key, nil
	}
	if key, err := base64.StdEncoding.DecodeString(raw); err == nil && len(key) == 32 {
		return key, nil
	}
	return nil, fmt.Errorf("%s debe ser de 32 bytes: 64 caracteres hexadecimales (openssl rand -hex 32) o base64 (openssl rand -base64 32)", name)
}
