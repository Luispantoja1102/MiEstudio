-- Planes, vigencia de la suscripción y pagos.

ALTER TABLE businesses ADD COLUMN plan TEXT NOT NULL DEFAULT 'profesional' CHECK (plan IN ('esencial', 'profesional', 'estudio'));
-- Fin del periodo pagado. NULL en un negocio activo = cortesía sin vencimiento (lo decide la plataforma).
ALTER TABLE businesses ADD COLUMN current_period_end TIMESTAMPTZ;
ALTER TABLE businesses ADD COLUMN suspended_reason TEXT NOT NULL DEFAULT '';

-- Pagos de la suscripción de cada negocio (manuales o por Mercado Pago).
CREATE TABLE payments (
  id            BIGSERIAL PRIMARY KEY,
  business_id   BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  amount_cents  INT NOT NULL CHECK (amount_cents >= 0),
  method        TEXT NOT NULL CHECK (method IN ('transfer', 'cash', 'card', 'mercadopago', 'courtesy')),
  plan          TEXT NOT NULL,
  months        INT NOT NULL CHECK (months BETWEEN 1 AND 24),
  period_start  TIMESTAMPTZ NOT NULL,
  period_end    TIMESTAMPTZ NOT NULL,
  note          TEXT NOT NULL DEFAULT '',
  external_ref  TEXT UNIQUE,                 -- id del pago en Mercado Pago (evita acreditar dos veces)
  created_by    BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX payments_business ON payments (business_id, created_at DESC);
CREATE INDEX payments_recent ON payments (created_at DESC);

-- Cobros en línea de la suscripción (Checkout Pro con la cuenta de la plataforma).
CREATE TABLE subscription_checkouts (
  id            BIGSERIAL PRIMARY KEY,
  business_id   BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  plan          TEXT NOT NULL,
  months        INT NOT NULL CHECK (months IN (1, 12)),
  amount_cents  INT NOT NULL CHECK (amount_cents > 0),
  status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'expired')),
  external_ref  TEXT NOT NULL UNIQUE,
  pay_url       TEXT NOT NULL DEFAULT '',
  mp_payment_id TEXT,
  created_by    BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  paid_at       TIMESTAMPTZ
);
CREATE INDEX subscription_checkouts_business ON subscription_checkouts (business_id, created_at DESC);

-- Avisos de cobro ya enviados (uno por tipo y periodo), para no repetirlos.
CREATE TABLE billing_notices (
  business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  kind        TEXT NOT NULL,
  period_key  TEXT NOT NULL,
  sent_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (business_id, kind, period_key)
);
