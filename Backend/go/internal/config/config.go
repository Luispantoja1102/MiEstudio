package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	StaticDir   string
	Production  bool   // APP_ENV=production
	EncKey      []byte // TOKEN_ENC_KEY (base64, 32 bytes): cifra los tokens de Mercado Pago
	MPBase      string // MP_API_BASE (opcional)

	// OAuth de Mercado Pago (aplicación de la plataforma). Vacío = solo token manual.
	MPClientID     string // MP_CLIENT_ID
	MPClientSecret string // MP_CLIENT_SECRET
	MPRedirect     string // MP_OAUTH_REDIRECT o API_PUBLIC_URL + /api/point/oauth/callback
	MPAuthBase     string // MP_AUTH_BASE (por defecto México)

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
	if raw := os.Getenv("TOKEN_ENC_KEY"); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) != 32 {
			return c, fmt.Errorf("TOKEN_ENC_KEY debe ser base64 de 32 bytes (por ejemplo: openssl rand -base64 32)")
		}
		c.EncKey = key
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
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL es obligatorio")
	}
	if len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("JWT_SECRET es obligatorio (mínimo 32 caracteres)")
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	return c, nil
}
