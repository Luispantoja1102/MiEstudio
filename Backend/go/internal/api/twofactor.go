package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

// Verificación en dos pasos. Con ella activa, la contraseña sola no abre la sesión: el inicio de sesión
// devuelve un reto de 5 minutos que solo sirve en /api/auth/login/2fa junto con un código de la app
// (o un código de respaldo). El reto no es una sesión: el middleware no lo acepta en ninguna otra ruta.

const (
	purpose2FA   = "2fa"
	challengeTTL = 5 * time.Minute
	issuerName   = "Barbella"
)

func normRecovery(c string) string {
	return strings.ToLower(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(c)))
}

// secondFactor valida un código TOTP o de respaldo de la persona. Un código TOTP ya usado no sirve de nuevo.
func (a *API) secondFactor(ctx context.Context, userID int64, secretEnc []byte, code string) (bool, error) {
	secret, err := a.open(secretEnc)
	if err != nil {
		return false, err
	}
	if step := totpMatch(secret, code, a.now()); step != 0 {
		tag, err := a.DB.Exec(ctx, `UPDATE users SET totp_last_step = $2 WHERE id = $1 AND totp_last_step < $2`, userID, step)
		return err == nil && tag.RowsAffected() == 1, err
	}
	rc := normRecovery(code)
	if len(rc) != 8 {
		return false, nil
	}
	tag, err := a.DB.Exec(ctx,
		`UPDATE totp_recovery SET used_at = $3 WHERE id = (SELECT id FROM totp_recovery WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL LIMIT 1)`,
		userID, hashToken(rc), a.now())
	return err == nil && tag.RowsAffected() == 1, err
}

func (a *API) loginSecondStep(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	uid, err := auth.ParsePurpose(a.Secret, purpose2FA, req.Challenge)
	if err != nil {
		writeFail(w, &httpFail{http.StatusUnauthorized, "La verificación venció. Vuelve a iniciar sesión.", "challenge_expired"})
		return
	}
	if !a.allow(w, "2fa-user:"+itoa(uid), 6, 10*time.Minute) || !a.allow(w, "2fa-ip:"+clientIP(r), 30, 10*time.Minute) {
		return
	}
	var enc []byte
	var active bool
	if err := a.DB.QueryRow(r.Context(), `SELECT totp_secret_enc, active FROM users WHERE id = $1`, uid).Scan(&enc, &active); err != nil || !active || enc == nil {
		writeErr(w, http.StatusUnauthorized, "código incorrecto")
		return
	}
	ok, err := a.secondFactor(r.Context(), uid, enc, req.Code)
	if err != nil {
		serverErr(w)
		return
	}
	if !ok {
		writeErr(w, http.StatusUnauthorized, "Código incorrecto o ya usado. Revisa la hora de tu teléfono.")
		return
	}
	token, err := a.startSession(w, r, uid)
	if err != nil {
		serverErr(w)
		return
	}
	var left int
	a.DB.QueryRow(r.Context(), `SELECT count(*) FROM totp_recovery WHERE user_id = $1 AND used_at IS NULL`, uid).Scan(&left)
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "recovery_left": left})
}

func (a *API) twoFactorStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var enabled bool
	var left int
	a.DB.QueryRow(r.Context(), `SELECT totp_secret_enc IS NOT NULL FROM users WHERE id = $1`, p.UserID).Scan(&enabled)
	a.DB.QueryRow(r.Context(), `SELECT count(*) FROM totp_recovery WHERE user_id = $1 AND used_at IS NULL`, p.UserID).Scan(&left)
	writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled, "recovery_left": left})
}

// twoFactorSetup genera un secreto nuevo (aún sin activar). Repetirlo reemplaza el pendiente.
func (a *API) twoFactorSetup(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if !a.allow(w, "2fa-setup:"+itoa(p.UserID), 10, time.Hour) {
		return
	}
	var email string
	var enabled bool
	if err := a.DB.QueryRow(r.Context(), `SELECT email, totp_secret_enc IS NOT NULL FROM users WHERE id = $1`, p.UserID).Scan(&email, &enabled); err != nil {
		serverErr(w)
		return
	}
	if enabled {
		writeErr(w, http.StatusConflict, "la verificación en dos pasos ya está activa")
		return
	}
	secret := newTOTPSecret()
	enc, err := a.seal(secret)
	if err != nil {
		if !writeFail(w, err) {
			serverErr(w)
		}
		return
	}
	if _, err := a.DB.Exec(r.Context(), `UPDATE users SET totp_pending_enc = $2 WHERE id = $1`, p.UserID, enc); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"secret": secret, "otpauth_url": otpauthURL(issuerName, email, secret)})
}

func (a *API) saveRecoveryCodes(ctx context.Context, tx pgx.Tx, userID int64) ([]string, error) {
	codes := newRecoveryCodes()
	if _, err := tx.Exec(ctx, `DELETE FROM totp_recovery WHERE user_id = $1`, userID); err != nil {
		return nil, err
	}
	for _, c := range codes {
		if _, err := tx.Exec(ctx, `INSERT INTO totp_recovery (user_id, code_hash) VALUES ($1, $2)`, userID, hashToken(normRecovery(c))); err != nil {
			return nil, err
		}
	}
	return codes, nil
}

// twoFactorEnable confirma el secreto pendiente con un código real y devuelve los códigos de respaldo (única vez).
func (a *API) twoFactorEnable(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	if !a.allow(w, "2fa-enable:"+itoa(p.UserID), 10, 10*time.Minute) {
		return
	}
	ctx := r.Context()
	var pending []byte
	if err := a.DB.QueryRow(ctx, `SELECT totp_pending_enc FROM users WHERE id = $1 AND totp_secret_enc IS NULL`, p.UserID).Scan(&pending); err != nil || pending == nil {
		writeErr(w, http.StatusConflict, "primero genera el código QR")
		return
	}
	secret, err := a.open(pending)
	if err != nil {
		serverErr(w)
		return
	}
	step := totpMatch(secret, req.Code, a.now())
	if step == 0 {
		writeErr(w, http.StatusBadRequest, "Ese código no coincide. Revisa que la hora de tu teléfono sea automática.")
		return
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE users SET totp_secret_enc = totp_pending_enc, totp_pending_enc = NULL, totp_last_step = $2, totp_enabled_at = $3 WHERE id = $1`, p.UserID, step, a.now()); err != nil {
		serverErr(w)
		return
	}
	codes, err := a.saveRecoveryCodes(ctx, tx, p.UserID)
	if err != nil || tx.Commit(ctx) != nil {
		serverErr(w)
		return
	}
	a.logActivity(ctx, p.UserID, p.BusinessID, "two_factor_enabled", nil)
	writeJSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

// verifyOwn exige la contraseña y un código vigente: para desactivar o regenerar respaldos.
func (a *API) verifyOwn(w http.ResponseWriter, r *http.Request, password, code string) (userID int64, ok bool) {
	p := auth.FromContext(r.Context())
	if !a.allow(w, "2fa-manage:"+itoa(p.UserID), 8, 10*time.Minute) {
		return 0, false
	}
	var hash string
	var enc []byte
	if err := a.DB.QueryRow(r.Context(), `SELECT password_hash, totp_secret_enc FROM users WHERE id = $1`, p.UserID).Scan(&hash, &enc); err != nil || enc == nil {
		writeErr(w, http.StatusConflict, "la verificación en dos pasos no está activa")
		return 0, false
	}
	if !auth.CheckPassword(hash, password) {
		writeErr(w, http.StatusUnauthorized, "la contraseña es incorrecta")
		return 0, false
	}
	good, err := a.secondFactor(r.Context(), p.UserID, enc, code)
	if err != nil {
		serverErr(w)
		return 0, false
	}
	if !good {
		writeErr(w, http.StatusUnauthorized, "Código incorrecto o ya usado.")
		return 0, false
	}
	return p.UserID, true
}

func (a *API) twoFactorDisable(w http.ResponseWriter, r *http.Request) {
	var req struct{ Password, Code string }
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	uid, ok := a.verifyOwn(w, r, req.Password, req.Code)
	if !ok {
		return
	}
	if err := a.clearTwoFactor(r.Context(), uid); err != nil {
		serverErr(w)
		return
	}
	p := auth.FromContext(r.Context())
	a.logActivity(r.Context(), p.UserID, p.BusinessID, "two_factor_disabled", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) twoFactorRegenerate(w http.ResponseWriter, r *http.Request) {
	var req struct{ Password, Code string }
	if !decode(w, r, &req) {
		writeErr(w, http.StatusBadRequest, "solicitud inválida")
		return
	}
	uid, ok := a.verifyOwn(w, r, req.Password, req.Code)
	if !ok {
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback(r.Context())
	codes, err := a.saveRecoveryCodes(r.Context(), tx, uid)
	if err != nil || tx.Commit(r.Context()) != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

// clearTwoFactor quita los dos pasos y cierra las sesiones de la persona (quien se los quita puede no ser ella).
func (a *API) clearTwoFactor(ctx context.Context, userID int64) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE users SET totp_secret_enc = NULL, totp_pending_enc = NULL, totp_enabled_at = NULL, totp_last_step = 0 WHERE id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM totp_recovery WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// resetTeamTwoFactor: el administrador del negocio quita los dos pasos a alguien que perdió su teléfono.
func (a *API) resetTeamTwoFactor(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var found int64
	err := a.DB.QueryRow(r.Context(), `SELECT id FROM users WHERE id = $1 AND business_id = $2`, id, p.BusinessID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil || a.clearTwoFactor(r.Context(), id) != nil {
		serverErr(w)
		return
	}
	a.DB.Exec(r.Context(), `UPDATE users SET session_version = session_version + 1 WHERE id = $1`, id)
	a.logActivity(r.Context(), p.UserID, p.BusinessID, "two_factor_reset", map[string]any{"user_id": id})
	w.WriteHeader(http.StatusNoContent)
}

// resetPlatformTwoFactor: el administrador de plataforma quita los dos pasos a cualquier persona.
func (a *API) resetPlatformTwoFactor(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var biz *int64
	err := a.DB.QueryRow(r.Context(), `SELECT business_id FROM users WHERE id = $1`, id).Scan(&biz)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "no encontrado")
		return
	}
	if err != nil || a.clearTwoFactor(r.Context(), id) != nil {
		serverErr(w)
		return
	}
	a.DB.Exec(r.Context(), `UPDATE users SET session_version = session_version + 1 WHERE id = $1`, id)
	var b int64
	if biz != nil {
		b = *biz
	}
	a.logActivity(r.Context(), p.UserID, b, "two_factor_reset", map[string]any{"user_id": id, "by": "platform"})
	w.WriteHeader(http.StatusNoContent)
}
