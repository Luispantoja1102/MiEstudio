package api

import (
	"testing"
	"time"
)

// Vectores de la RFC 6238 (apéndice B, SHA1, secreto «12345678901234567890», 8 dígitos): aquí se usan los 6 últimos.
func TestTOTPRFC6238Vectors(t *testing.T) {
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	for ts, want8 := range map[int64]string{59: "94287082", 1111111109: "07081804", 1111111111: "14050471", 1234567890: "89005924", 2000000000: "69279037", 20000000000: "65353130"} {
		got, err := totpCode(secret, ts/30)
		if err != nil || got != want8[2:] {
			t.Errorf("t=%d: %s (%v), esperaba %s", ts, got, err, want8[2:])
		}
	}
}

func TestTOTPMatchWindowAndFormat(t *testing.T) {
	secret := newTOTPSecret()
	now := time.Unix(1_800_000_000, 0)
	step := now.Unix() / 30
	code := func(s int64) string { c, _ := totpCode(secret, s); return c }
	if totpMatch(secret, code(step), now) != step || totpMatch(secret, code(step-1), now) != step-1 || totpMatch(secret, code(step+1), now) != step+1 {
		t.Fatal("el paso actual y sus vecinos deben aceptarse")
	}
	if totpMatch(secret, code(step-2), now) != 0 || totpMatch(secret, code(step+2), now) != 0 {
		t.Fatal("fuera de la ventana no")
	}
	spaced := code(step)[:3] + " " + code(step)[3:]
	if totpMatch(secret, spaced, now) != step {
		t.Fatal("se aceptan espacios")
	}
	for _, bad := range []string{"", "12345", "1234567", "abcdef", "000000"} {
		if bad != "000000" && totpMatch(secret, bad, now) != 0 {
			t.Fatalf("%q no debe aceptarse", bad)
		}
	}
}

func TestOtpauthAndRecoveryShape(t *testing.T) {
	u := otpauthURL("MiEstudio", "a@x.com", "ABC")
	if u != "otpauth://totp/MiEstudio:a@x.com?algorithm=SHA1&digits=6&issuer=MiEstudio&period=30&secret=ABC" && u != "otpauth://totp/MiEstudio%3Aa@x.com?algorithm=SHA1&digits=6&issuer=MiEstudio&period=30&secret=ABC" {
		t.Fatalf("url: %s", u)
	}
	seen := map[string]bool{}
	for _, c := range newRecoveryCodes() {
		if len(c) != 9 || seen[c] {
			t.Fatalf("código mal formado o repetido: %q", c)
		}
		seen[c] = true
	}
}
