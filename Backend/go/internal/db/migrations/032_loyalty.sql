-- Tarjeta de lealtad (sellos). El negocio la activa y define cuántas visitas pide y cuál es el premio. Cada venta cobrada a un
-- cliente con al menos un servicio da un sello (uno por cliente por día). Al completar la tarjeta se canjea el premio.
-- Cada cliente puede tener un enlace privado para ver su tarjeta virtual.
CREATE TABLE loyalty_settings (
  business_id    BIGINT PRIMARY KEY REFERENCES businesses(id) ON DELETE CASCADE,
  enabled        BOOLEAN NOT NULL DEFAULT false,
  visits_required INT NOT NULL DEFAULT 10 CHECK (visits_required BETWEEN 2 AND 50),
  reward         TEXT NOT NULL DEFAULT '' CHECK (length(reward) <= 80),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE loyalty_redemptions (
  id          BIGSERIAL PRIMARY KEY,
  business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  client_id   BIGINT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
  reward      TEXT NOT NULL,          -- el premio tal como estaba al canjearlo
  stamps      INT NOT NULL,           -- cuántos sellos se usaron
  redeemed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
  redeemed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX loyalty_redemptions_client ON loyalty_redemptions (client_id);

CREATE TABLE loyalty_stamps (
  id            BIGSERIAL PRIMARY KEY,
  business_id   BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  client_id     BIGINT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
  sale_id       BIGINT NOT NULL UNIQUE REFERENCES sales(id) ON DELETE CASCADE,
  day           DATE NOT NULL,        -- día de la visita, en la zona del negocio
  redemption_id BIGINT REFERENCES loyalty_redemptions(id) ON DELETE SET NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX loyalty_stamps_client_day ON loyalty_stamps (client_id, day);
CREATE INDEX loyalty_stamps_open ON loyalty_stamps (client_id) WHERE redemption_id IS NULL;

ALTER TABLE clients ADD COLUMN loyalty_token TEXT UNIQUE; -- el secreto del enlace de su tarjeta; NULL = sin enlace
