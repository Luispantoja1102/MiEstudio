package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
	"miestudio/backend-go/internal/mail"
)

// Recuperar la contraseña por correo.
//
//   - La respuesta es la misma exista o no la cuenta, así nadie puede averiguar quién tiene cuenta.
//   - El enlace vale una hora y se usa una sola vez; pedir otro anula los anteriores.
//   - En la base solo queda el hash del token: quien lea la base no puede restablecer cuentas.
//   - Al restablecer se cierran todas las sesiones de la persona (lo hace el disparador de session_version).
//   - Las cuentas de administrador de plataforma no usan este camino: su clave la restablece otro administrador
//     o la línea de comandos (npm run admin), porque quien controla su correo no debería controlar la plataforma.

const resetTTL = time.Hour

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// sendBackground envía un correo sin hacer esperar a quien hizo la petición ni exponer si el envío falló.
func (a *API) sendBackground(m mail.Message, what string) {
	if !a.mailEnabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := a.Mailer.Send(ctx, m); err != nil {
			log.Printf("correo (%s): %v", what, err)
		}
	}()
}

func (a *API) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !validEmail(email) {
		writeErr(w, http.StatusBadRequest, "escribe un correo válido")
		return
	}
	if !a.allow(w, "forgot-ip:"+clientIP(r), 10, time.Hour) || !a.allow(w, "forgot-email:"+email, 3, time.Hour) {
		return
	}
	if !a.mailEnabled() {
		writeFail(w, &httpFail{http.StatusServiceUnavailable, "Este servidor no tiene correo configurado. Pide a tu administrador que restablezca tu contraseña.", "mail_disabled"})
		return
	}
	ctx := r.Context()
	var id int64
	var name string
	err := a.DB.QueryRow(ctx, `SELECT id, name FROM users WHERE email = $1 AND active AND role <> $2`, email, auth.RolePlatformAdmin).Scan(&id, &name)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// misma respuesta que si existiera
	case err != nil:
		serverErr(w)
		return
	default:
		raw := make([]byte, 32)
		rand.Read(raw)
		token := base64.RawURLEncoding.EncodeToString(raw)
		tx, err := a.DB.Begin(ctx)
		if err != nil {
			serverErr(w)
			return
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `DELETE FROM password_resets WHERE user_id = $1`, id); err != nil {
			serverErr(w)
			return
		}
		if _, err := tx.Exec(ctx, `INSERT INTO password_resets (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`, id, hashToken(token), a.now().Add(resetTTL)); err != nil {
			serverErr(w)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			serverErr(w)
			return
		}
		link := a.AppURL + "/restablecer?token=" + token
		first := name
		if f := strings.Fields(name); len(f) > 0 {
			first = f[0]
		}
		a.sendBackground(mail.Message{
			To:      email,
			Subject: "Restablece tu contraseña de MiEstudio",
			Text:    fmt.Sprintf("Hola %s,\n\nPara elegir una contraseña nueva abre este enlace (vale 1 hora y solo se puede usar una vez):\n%s\n\nSi no lo pediste tú, ignora este correo: tu contraseña sigue igual.\n\n— MiEstudio\n", first, link),
			HTML:    fmt.Sprintf(`<div style="font-family:system-ui,sans-serif;max-width:520px;margin:0 auto;padding:24px;color:#1f2a28"><h2 style="margin:0 0 12px">Restablece tu contraseña</h2><p>Hola %s, para elegir una contraseña nueva usa este botón. Vale 1 hora y solo se puede usar una vez.</p><p><a href="%s" style="display:inline-block;padding:10px 18px;border-radius:8px;background:#2f6f5e;color:#fff;text-decoration:none">Elegir contraseña nueva</a></p><p style="font-size:13px;color:#5b6864">Si no lo pediste tú, ignora este correo: tu contraseña sigue igual.</p></div>`, html.EscapeString(first), html.EscapeString(link)),
		}, "restablecer contraseña")
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true})
}

func (a *API) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) || req.Token == "" || len(req.Token) > 200 {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if !a.allow(w, "reset-ip:"+clientIP(r), 20, time.Hour) {
		return
	}
	if !validPassword(req.Password) {
		writeErr(w, http.StatusBadRequest, "la contraseña debe tener de 8 a 72 caracteres")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		serverErr(w)
		return
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	// DELETE ... RETURNING: el enlace se consume en el mismo paso que se valida, así dos intentos a la vez no pasan los dos
	var userID int64
	err = tx.QueryRow(ctx, `DELETE FROM password_resets WHERE token_hash = $1 AND expires_at > $2 RETURNING user_id`, hashToken(req.Token), a.now()).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeFail(w, &httpFail{http.StatusBadRequest, "El enlace venció o ya se usó. Pide uno nuevo.", "reset_invalid"})
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	var bizID *int64
	if err := tx.QueryRow(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1 AND active AND role <> $3 RETURNING business_id`, userID, hash, auth.RolePlatformAdmin).Scan(&bizID); err != nil {
		writeFail(w, &httpFail{http.StatusBadRequest, "El enlace venció o ya se usó. Pide uno nuevo.", "reset_invalid"})
		return
	}
	if _, err := tx.Exec(ctx, `DELETE FROM password_resets WHERE user_id = $1`, userID); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		serverErr(w)
		return
	}
	var biz int64
	if bizID != nil {
		biz = *bizID
	}
	a.logActivity(ctx, userID, biz, "password_reset_by_email", nil)
	w.WriteHeader(http.StatusNoContent)
}

// purgeResets borra enlaces vencidos (se llama desde el proceso de segundo plano).
func (a *API) purgeResets(ctx context.Context) {
	a.DB.Exec(ctx, `DELETE FROM password_resets WHERE expires_at < $1`, a.now())
}
