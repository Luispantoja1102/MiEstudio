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
	MPClientID     string // MP_CLIENT_ID
	MPClientSecret string // MP_CLIENT_SECRET
	MPRedirect     string // MP_OAUTH_REDIRECT o API_PUBLIC_URL + /api/point/oauth/callback
	MPAuthBase     string // MP_AUTH_BASE (por defecto México)

	// Correo saliente (confirmaciones y recordatorios). Sin SMTP_HOST y MAIL_FROM no se envía nada.
	SMTPHost, SMTPUser, SMTPPass, MailFrom string
	SMTPPort                               int
	SMTPSecure                             bool
	AppURL                                 string // dirección pública de la app: se usa en los enlaces de los correos

	// primer administrador de plataforma (opcional; se crea solo si no existe ninguno)
	PlatformAdminName     string
	PlatformAdminEmail    string
	PlatformAdminPassword string
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
	c.MPBase = os.Getenv("MP_API_BASE")
	c.MPClientID, c.MPClientSecret = os.Getenv("MP_CLIENT_ID"), os.Getenv("MP_CLIENT_SECRET")
	c.MPRedirect = os.Getenv("MP_OAUTH_REDIRECT")
	if c.MPRedirect == "" { // API_PUBLIC_URL se acepta con o sin el sufijo /api
		if base := strings.TrimSuffix(strings.TrimRight(os.Getenv("API_PUBLIC_URL"), "/"), "/api"); base != "" {
			c.MPRedirect = base + "/api/point/oauth/callback"
		}
	}
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
	c.MailFrom = strings.TrimSpace(os.Getenv("MAIL_FROM"))
	if c.MailFrom == "" && c.SMTPUser != "" && strings.Contains(c.SMTPUser, "@") {
		c.MailFrom = "MiEstudio <" + c.SMTPUser + ">"
	}
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
