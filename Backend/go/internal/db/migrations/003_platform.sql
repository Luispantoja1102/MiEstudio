-- Usuarios de plataforma: sin negocio, con rol propio
ALTER TABLE users ALTER COLUMN business_id DROP NOT NULL;
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
  CHECK (role IN ('admin', 'staff', 'reception', 'cashier', 'platform_admin', 'platform_support'));
ALTER TABLE users ADD CONSTRAINT users_tenant_check
  CHECK ((role IN ('platform_admin', 'platform_support')) = (business_id IS NULL));

-- Estado de cada negocio, controlado por la plataforma
ALTER TABLE businesses ADD COLUMN status TEXT NOT NULL DEFAULT 'trial'
  CHECK (status IN ('pending', 'trial', 'active', 'suspended'));
ALTER TABLE businesses ADD COLUMN trial_ends_at TIMESTAMPTZ;
UPDATE businesses SET trial_ends_at = created_at + interval '14 days';

CREATE TABLE platform_settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Bitácora de acciones relevantes
CREATE TABLE activity_log (
  id          BIGSERIAL PRIMARY KEY,
  actor_id    BIGINT REFERENCES users(id) ON DELETE SET NULL,
  business_id BIGINT REFERENCES businesses(id) ON DELETE SET NULL,
  action      TEXT NOT NULL,
  detail      JSONB NOT NULL DEFAULT '{}',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX activity_log_recent ON activity_log (created_at DESC);
CREATE INDEX activity_log_business ON activity_log (business_id, created_at DESC);
