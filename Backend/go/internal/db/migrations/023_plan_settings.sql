-- Precios y límites de los planes que la plataforma ajusta desde su panel. Sin fila, el plan usa lo definido en el código.
CREATE TABLE plan_settings (
  plan_id      TEXT PRIMARY KEY CHECK (plan_id IN ('basico', 'crecimiento', 'pro')),
  description  TEXT NOT NULL DEFAULT '',
  price_month  INT NOT NULL CHECK (price_month BETWEEN 0 AND 1000000),
  price_year   INT NOT NULL CHECK (price_year BETWEEN 0 AND 10000000),
  max_staff    INT CHECK (max_staff BETWEEN 1 AND 10000), -- NULL = sin límite
  max_branches INT NOT NULL CHECK (max_branches BETWEEN 1 AND 1000),
  pos          BOOLEAN NOT NULL,
  deposits     BOOLEAN NOT NULL,
  terminal     BOOLEAN NOT NULL,
  support      TEXT NOT NULL CHECK (support IN ('correo', 'prioritario', 'dedicado')),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by   BIGINT REFERENCES users(id) ON DELETE SET NULL
);
