package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP (RFC 6238): HMAC-SHA1, pasos de 30 s y 6 dígitos, lo que hablan todas las apps de autenticación.

const (
	totpStep   = 30
	totpDigits = 6
	totpDrift  = 1 // se aceptan el paso anterior y el siguiente (relojes algo desfasados)
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() string {
	raw := make([]byte, 20)
	rand.Read(raw)
	return b32.EncodeToString(raw)
}

// totpCode es el código de un paso (RFC 4226, truncamiento dinámico).
func totpCode(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return "", err
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[off])&0x7f)<<24 | uint32(sum[off+1])<<16 | uint32(sum[off+2])<<8 | uint32(sum[off+3])
	return fmt.Sprintf("%0*d", totpDigits, bin%1_000_000), nil
}

// totpMatch devuelve el paso cuyo código coincide, mirando la ventana de desfase. 0 = ninguno.
// El paso permite rechazar un código ya usado (debe ser mayor que el último aceptado).
func totpMatch(secret, input string, now time.Time) int64 {
	input = strings.ReplaceAll(strings.TrimSpace(input), " ", "")
	if len(input) != totpDigits {
		return 0
	}
	cur := now.Unix() / totpStep
	for d := int64(-totpDrift); d <= totpDrift; d++ {
		want, err := totpCode(secret, cur+d)
		if err == nil && subtle.ConstantTimeCompare([]byte(want), []byte(input)) == 1 {
			return cur + d
		}
	}
	return 0
}

func otpauthURL(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {fmt.Sprint(totpDigits)}, "period": {fmt.Sprint(totpStep)}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// newRecoveryCodes: 8 códigos tipo «a1b2-c3d4», de un solo uso.
func newRecoveryCodes() []string {
	out := make([]string, 8)
	for i := range out {
		raw := make([]byte, 4)
		rand.Read(raw)
		out[i] = fmt.Sprintf("%x-%x", raw[:2], raw[2:])
	}
	return out
}
