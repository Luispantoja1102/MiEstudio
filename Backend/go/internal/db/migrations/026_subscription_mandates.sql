-- Suscripción con cobro automático mensual (preapproval de Mercado Pago, cuenta de la plataforma).
-- Un negocio autoriza su tarjeta una vez; Mercado Pago cobra cada mes y avisa de cada cargo.
CREATE TABLE subscription_mandates (
  id                BIGSERIAL PRIMARY KEY,
  business_id       BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  plan              TEXT NOT NULL,
  amount_cents      INT NOT NULL CHECK (amount_cents > 0),
  status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'authorized', 'paused', 'cancelled')),
  mp_preapproval_id TEXT UNIQUE,
  init_point        TEXT NOT NULL DEFAULT '',
  payer_email       TEXT NOT NULL DEFAULT '',
  created_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX subscription_mandates_business ON subscription_mandates (business_id, created_at DESC);
