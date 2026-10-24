-- Gastos de operar la plataforma (servidor, dominio, comisiones…), para calcular la ganancia del mes en Finanzas.
CREATE TABLE platform_expenses (
  id           BIGSERIAL PRIMARY KEY,
  label        TEXT NOT NULL,
  amount_cents INT NOT NULL CHECK (amount_cents > 0),
  note         TEXT NOT NULL DEFAULT '',
  day          DATE NOT NULL DEFAULT (now() AT TIME ZONE 'America/Mexico_City')::date,
  created_by   BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX platform_expenses_day ON platform_expenses (day DESC);
