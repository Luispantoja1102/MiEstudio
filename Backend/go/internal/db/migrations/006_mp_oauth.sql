-- Conexión con Mercado Pago por OAuth: el token se renueva antes de vencer.
ALTER TABLE mp_accounts ADD COLUMN refresh_enc BYTEA;
ALTER TABLE mp_accounts ADD COLUMN expires_at TIMESTAMPTZ;
ALTER TABLE mp_accounts ADD COLUMN mp_user_id TEXT;
ALTER TABLE mp_accounts ADD COLUMN source TEXT NOT NULL DEFAULT 'token' CHECK (source IN ('token', 'oauth'));
