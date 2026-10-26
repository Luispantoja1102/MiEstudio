package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"math/big"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/mail"
)

// Verificación del correo al registrarse: se manda un código de 6 dígitos y la cuenta solo se crea si lo escriben bien.
// Frena correos falsos y altas en masa. Sin correo configurado en el servidor no hay cómo mandarlo: ahí no se exige
// (tampoco con SIGNUP_VERIFY=off).
const (
	signupCodeTTL        = 10 * time.Minute
	signupCodeCooldown   = time.Minute
	signupCodeMaxTries   = 5
	signupCodeMaxPerHour = 5
)

func (a *API) signupVerifyRequired() bool { return a.mailEnabled() && !a.SignupVerifyOff }

// signupCodeHash liga el código al correo con una clave del servidor: quien lea la base no puede probar códigos fuera de línea.
func (a *API) signupCodeHash(email, code string) string {
	key := a.EncKey
	if len(key) == 0 {
		key = []byte(a.Secret)
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte("signup:" + email + ":" + code))
	return hex.EncodeToString(m.Sum(nil))
}

// sendSignupCode es POST /api/auth/register/code: manda un código nuevo al correo, con espera entre envíos y tope por hora.
func (a *API) sendSignupCode(w http.ResponseWriter, r *http.Request) {
	if !a.signupVerifyRequired() {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "required": false})
		return
	}
	if !a.allow(w, "signup-code-ip:"+clientIP(r), 10, time.Hour) {
		return
	}
	if a.registrationMode(r.Context()) == "closed" {
		writeErr(w, http.StatusForbidden, "el registro está cerrado por ahora")
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	email := lowerTrim(req.Email)
	if !validEmail(email) {
		writeErr(w, http.StatusBadRequest, "escribe un correo válido")
		return
	}
	ctx := r.Context()
	var taken bool
	a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = $1)`, email).Scan(&taken)
	if taken {
		writeErr(w, http.StatusConflict, "ese correo ya está registrado")
		return
	}

	now := a.now()
	var sentAt time.Time
	var sends []time.Time
	err := a.DB.QueryRow(ctx, `SELECT sent_at, sends FROM signup_codes WHERE email = $1`, email).Scan(&sentAt, &sends)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		serverErr(w)
		return
	}
	if err == nil && now.Sub(sentAt) < signupCodeCooldown {
		wait := int((signupCodeCooldown - now.Sub(sentAt)).Seconds()) + 1
		writeErr(w, http.StatusTooManyRequests, fmt.Sprintf("espera %d s para pedir otro código", wait))
		return
	}
	recent := make([]time.Time, 0, len(sends)+1)
	for _, t := range sends {
		if now.Sub(t) < time.Hour {
			recent = append(recent, t)
		}
	}
	if len(recent) >= signupCodeMaxPerHour {
		writeErr(w, http.StatusTooManyRequests, "ya pediste varios códigos para este correo; intenta en una hora")
		return
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		serverErr(w)
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	if _, err := a.DB.Exec(ctx,
		`INSERT INTO signup_codes (email, code_hash, tries, sent_at, sends, expires_at) VALUES ($1, $2, 0, $3, $4, $5)
		 ON CONFLICT (email) DO UPDATE SET code_hash = EXCLUDED.code_hash, tries = 0, sent_at = EXCLUDED.sent_at, sends = EXCLUDED.sends, expires_at = EXCLUDED.expires_at`,
		email, a.signupCodeHash(email, code), now, append(recent, now), now.Add(signupCodeTTL)); err != nil {
		serverErr(w)
		return
	}
	mctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	text := fmt.Sprintf("Tu código para crear tu cuenta en Barbella es %s.\n\nVale %d minutos. Si no lo pediste, ignora este correo.\n\n— Barbella\n", code, int(signupCodeTTL.Minutes()))
	body := `<p>Tu código para crear tu cuenta en Barbella es:</p><p style="font-size:30px;letter-spacing:6px;font-weight:700;margin:12px 0">` + html.EscapeString(code) +
		fmt.Sprintf(`</p><p>Vale %d minutos. Si no lo pediste, ignora este correo.</p>`, int(signupCodeTTL.Minutes()))
	if err := a.Mailer.Send(mctx, mail.Message{To: email, Subject: "Tu código de Barbella: " + code, Text: text, HTML: mailLayout("Verifica tu correo", body)}); err != nil {
		a.DB.Exec(ctx, `DELETE FROM signup_codes WHERE email = $1`, email)
		writeErr(w, http.StatusBadGateway, "no pudimos enviar el correo; revisa que esté bien escrito e inténtalo de nuevo")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "required": true})
}

// consumeSignupCode revisa el código del correo y lo gasta. Devuelve nil si no se exige verificación.
func (a *API) consumeSignupCode(ctx context.Context, email, code string) *httpFail {
	if !a.signupVerifyRequired() {
		return nil
	}
	code = onlyDigits(code)
	if len(code) != 6 {
		return &httpFail{http.StatusBadRequest, "escribe el código de 6 dígitos que te enviamos al correo", "email_code_required"}
	}
	var hash string
	var expires time.Time
	err := a.DB.QueryRow(ctx, `SELECT code_hash, expires_at FROM signup_codes WHERE email = $1`, email).Scan(&hash, &expires)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && expires.Before(a.now())) {
		return &httpFail{http.StatusBadRequest, "el código venció: pide uno nuevo", "email_code_expired"}
	}
	if err != nil {
		return &httpFail{http.StatusInternalServerError, "error del servidor", ""}
	}
	if !hmac.Equal([]byte(hash), []byte(a.signupCodeHash(email, code))) {
		var tries int
		a.DB.QueryRow(ctx, `UPDATE signup_codes SET tries = tries + 1 WHERE email = $1 RETURNING tries`, email).Scan(&tries)
		if tries >= signupCodeMaxTries {
			a.DB.Exec(ctx, `DELETE FROM signup_codes WHERE email = $1`, email)
			return &httpFail{http.StatusBadRequest, "demasiados intentos: pide un código nuevo", "email_code_expired"}
		}
		return &httpFail{http.StatusBadRequest, "el código no es correcto", "email_code_wrong"}
	}
	// se gasta: con DELETE ... RETURNING solo una petición puede usarlo aunque lleguen dos a la vez
	var spent string
	if a.DB.QueryRow(ctx, `DELETE FROM signup_codes WHERE email = $1 RETURNING email`, email).Scan(&spent) != nil {
		return &httpFail{http.StatusBadRequest, "el código venció: pide uno nuevo", "email_code_expired"}
	}
	return nil
}
