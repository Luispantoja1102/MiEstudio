package config

import (
	"encoding/base64"
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	StaticDir   string
	Production  bool   // APP_ENV=production
	EncKey      []byte // TOKEN_ENC_KEY (base64, 32 bytes): cifra los tokens de Mercado Pago
	MPBase      string // MP_API_BASE (opcional)

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
