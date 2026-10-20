CREATE TABLE cash_sessions (
  id            BIGSERIAL PRIMARY KEY,
  branch_id     BIGINT NOT NULL REFERENCES branches(id),
  opened_by     BIGINT NOT NULL REFERENCES users(id),
  closed_by     BIGINT REFERENCES users(id),
  opening_cents INT NOT NULL DEFAULT 0,
  counted_cents INT,
  opened_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  closed_at     TIMESTAMPTZ
);
-- una sola caja abierta por sucursal
CREATE UNIQUE INDEX one_open_cash_per_branch ON cash_sessions (branch_id) WHERE closed_at IS NULL;

CREATE TABLE sales (
  id               BIGSERIAL PRIMARY KEY,
  business_id      BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  branch_id        BIGINT NOT NULL REFERENCES branches(id),
  cash_session_id  BIGINT REFERENCES cash_sessions(id),
  client_id        BIGINT REFERENCES clients(id),
  appointment_id   BIGINT REFERENCES appointments(id),
  cashier_id       BIGINT NOT NULL REFERENCES users(id),
  total_cents      INT NOT NULL CHECK (total_cents >= 0),
  tip_cents        INT NOT NULL DEFAULT 0 CHECK (tip_cents >= 0),
  status           TEXT NOT NULL DEFAULT 'paid' CHECK (status IN ('paid', 'refunded')),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sale_items (
  id               BIGSERIAL PRIMARY KEY,
  sale_id          BIGINT NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
  service_id       BIGINT REFERENCES services(id),
  product_id       BIGINT REFERENCES products(id),
  staff_id         BIGINT REFERENCES users(id), -- a quién se le paga comisión
  qty              INT NOT NULL DEFAULT 1 CHECK (qty > 0),
  unit_price_cents INT NOT NULL CHECK (unit_price_cents >= 0),
  commission_cents INT NOT NULL DEFAULT 0 CHECK (commission_cents >= 0),
  CHECK ((service_id IS NULL) <> (product_id IS NULL))
);

CREATE TABLE sale_payments (
  id          BIGSERIAL PRIMARY KEY,
  sale_id     BIGINT NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
  method      TEXT NOT NULL CHECK (method IN ('cash', 'card', 'transfer')),
  amount_cents INT NOT NULL CHECK (amount_cents > 0)
);
