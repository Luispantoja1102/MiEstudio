-- Verificación en dos pasos (TOTP, el estándar de Google Authenticator, Authy, 1Password…).
ALTER TABLE users ADD COLUMN totp_secret_enc  BYTEA;       -- secreto cifrado; NULL = sin dos pasos
ALTER TABLE users ADD COLUMN totp_pending_enc BYTEA;       -- secreto generado y aún sin confirmar
ALTER TABLE users ADD COLUMN totp_last_step   BIGINT NOT NULL DEFAULT 0; -- evita reutilizar un código ya usado
ALTER TABLE users ADD COLUMN totp_enabled_at  TIMESTAMPTZ;

-- Códigos de respaldo de un solo uso (se guardan con hash).
CREATE TABLE totp_recovery (
  id        BIGSERIAL PRIMARY KEY,
  user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash TEXT NOT NULL,
  used_at   TIMESTAMPTZ
);
CREATE INDEX totp_recovery_user ON totp_recovery (user_id);
