package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
	StaticDir   string

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

		PlatformAdminName:     os.Getenv("PLATFORM_ADMIN_NAME"),
		PlatformAdminEmail:    os.Getenv("PLATFORM_ADMIN_EMAIL"),
		PlatformAdminPassword: os.Getenv("PLATFORM_ADMIN_PASSWORD"),
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
