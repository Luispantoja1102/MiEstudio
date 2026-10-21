package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

const jwt = "0123456789abcdef0123456789abcdef"

func TestSecretKeyFormats(t *testing.T) {
	raw := bytes.Repeat([]byte{0xAB}, 32)

	t.Setenv("K", hex.EncodeToString(raw)) // hexadecimal de 64 caracteres (formato de MiConsultorio)
	if got, err := secretKey("K", jwt, "p"); err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("hex: %x %v", got, err)
	}
	t.Setenv("K", "  "+hex.EncodeToString(raw)+"\n") // con espacios o salto de línea
	if got, err := secretKey("K", jwt, "p"); err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("hex con espacios: %v", err)
	}
	t.Setenv("K", base64.StdEncoding.EncodeToString(raw))
	if got, err := secretKey("K", jwt, "p"); err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("base64: %v", err)
	}
}

func TestSecretKeyDerivedFromJWT(t *testing.T) {
	t.Setenv("K", "")
	got, err := secretKey("K", jwt, "token-enc")
	want := sha256.Sum256([]byte("token-enc:" + jwt))
	if err != nil || !bytes.Equal(got, want[:]) {
		t.Fatalf("derivada: %x %v", got, err)
	}
	// distinto propósito, distinta clave: la de cifrado y la del estado nunca coinciden
	other, _ := secretKey("K", jwt, "oauth-state")
	if bytes.Equal(got, other) {
		t.Fatal("las claves derivadas de distintos propósitos no deben coincidir")
	}
	// y cambiar JWT_SECRET cambia la clave derivada
	if third, _ := secretKey("K", jwt+"x", "token-enc"); bytes.Equal(got, third) {
		t.Fatal("la clave derivada debe depender de JWT_SECRET")
	}
}

func TestSecretKeyRejectsBadValues(t *testing.T) {
	for name, v := range map[string]string{
		"hex corto":        strings.Repeat("ab", 16),
		"hex con basura":   strings.Repeat("zz", 32),
		"base64 de 16":     base64.StdEncoding.EncodeToString(make([]byte, 16)),
		"texto cualquiera": "mi-clave-secreta",
		"hex de 33 bytes":  strings.Repeat("ab", 33),
	} {
		t.Setenv("K", v)
		if _, err := secretKey("K", jwt, "p"); err == nil || !strings.Contains(err.Error(), "K debe ser de 32 bytes") {
			t.Errorf("%s: debía rechazarse con un mensaje claro, dio %v", name, err)
		}
	}
}

func TestLoadAlwaysProvidesKeys(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", jwt)
	t.Setenv("TOKEN_ENC_KEY", "")
	t.Setenv("OAUTH_STATE_SECRET", "")
	c, err := Load()
	if err != nil || len(c.EncKey) != 32 || len(c.OAuthState) != 32 || bytes.Equal(c.EncKey, c.OAuthState) {
		t.Fatalf("sin variables debe derivar dos claves distintas: %v %v", err, c)
	}
	t.Setenv("TOKEN_ENC_KEY", "basura")
	if _, err := Load(); err == nil {
		t.Fatal("una clave inválida debe impedir el arranque")
	}
	// las rutas de retorno de Mercado Pago
	t.Setenv("TOKEN_ENC_KEY", "")
	t.Setenv("MP_OAUTH_REDIRECT", "")
	t.Setenv("API_PUBLIC_URL", "https://app.example.mx/api/") // con o sin /api, con o sin diagonal final
	if c, _ := Load(); c.MPRedirect != "https://app.example.mx/api/point/oauth/callback" {
		t.Fatalf("redirect derivado: %q", c.MPRedirect)
	}
	t.Setenv("API_PUBLIC_URL", "https://app.example.mx")
	if c, _ := Load(); c.MPRedirect != "https://app.example.mx/api/point/oauth/callback" {
		t.Fatalf("redirect derivado sin /api: %q", c.MPRedirect)
	}
	t.Setenv("MP_OAUTH_REDIRECT", "https://otra.mx/api/point/oauth/callback")
	if c, _ := Load(); c.MPRedirect != "https://otra.mx/api/point/oauth/callback" {
		t.Fatalf("MP_OAUTH_REDIRECT explícito debe ganar: %q", c.MPRedirect)
	}
}
