-- Punto de venta: folios, comisiones, libro de inventario y movimientos de caja.

ALTER TABLE businesses ADD COLUMN sale_seq INT NOT NULL DEFAULT 0;

-- Productos: costo y existencia que nunca baja de cero
ALTER TABLE products ADD COLUMN cost_cents INT NOT NULL DEFAULT 0 CHECK (cost_cents >= 0);
ALTER TABLE products ADD CONSTRAINT products_stock_nonneg CHECK (stock >= 0);

-- Caja
ALTER TABLE cash_sessions ADD COLUMN expected_cents INT;
ALTER TABLE cash_sessions ADD COLUMN note TEXT NOT NULL DEFAULT '';

CREATE TABLE cash_movements (
  id              BIGSERIAL PRIMARY KEY,
  cash_session_id BIGINT NOT NULL REFERENCES cash_sessions(id) ON DELETE CASCADE,
  kind            TEXT NOT NULL CHECK (kind IN ('in', 'out')),
  amount_cents    INT NOT NULL CHECK (amount_cents > 0),
  concept         TEXT NOT NULL,
  created_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX cash_movements_session ON cash_movements (cash_session_id);

-- Ventas
ALTER TABLE sales DROP CONSTRAINT sales_status_check;
UPDATE sales SET status = 'void' WHERE status = 'refunded';
ALTER TABLE sales ADD CONSTRAINT sales_status_check CHECK (status IN ('paid', 'void'));
ALTER TABLE sales ADD COLUMN folio INT;
ALTER TABLE sales ADD COLUMN subtotal_cents INT NOT NULL DEFAULT 0;
ALTER TABLE sales ADD COLUMN discount_cents INT NOT NULL DEFAULT 0 CHECK (discount_cents >= 0);
ALTER TABLE sales ADD COLUMN tip_staff_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE sales ADD COLUMN note TEXT NOT NULL DEFAULT '';
ALTER TABLE sales ADD COLUMN void_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE sales ADD COLUMN voided_at TIMESTAMPTZ;
ALTER TABLE sales ADD COLUMN voided_by BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE sales ADD COLUMN client_token TEXT; -- evita cobrar dos veces si se repite la petición
ALTER TABLE sales ADD CONSTRAINT sales_folio_key UNIQUE (business_id, folio);
ALTER TABLE sales ADD CONSTRAINT sales_client_token_key UNIQUE (business_id, client_token);
CREATE UNIQUE INDEX sales_one_paid_per_appointment ON sales (appointment_id)
  WHERE appointment_id IS NOT NULL AND status = 'paid';
CREATE INDEX sales_business_date ON sales (business_id, created_at DESC);

ALTER TABLE sale_items ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE sale_items ADD COLUMN unit_cost_cents INT NOT NULL DEFAULT 0;
ALTER TABLE sale_items ADD COLUMN discount_cents INT NOT NULL DEFAULT 0 CHECK (discount_cents >= 0);
ALTER TABLE sale_items ADD COLUMN total_cents INT NOT NULL DEFAULT 0;
ALTER TABLE sale_items ADD COLUMN commission_pct NUMERIC(5,2) NOT NULL DEFAULT 0;
ALTER TABLE sale_items ADD COLUMN commission_base_cents INT NOT NULL DEFAULT 0;

ALTER TABLE sale_payments ADD COLUMN received_cents INT;
ALTER TABLE sale_payments ADD COLUMN change_cents INT NOT NULL DEFAULT 0;
ALTER TABLE sale_payments ADD COLUMN reference TEXT NOT NULL DEFAULT '';
-- el dinero cuenta en la caja que estaba abierta cuando se recibió
ALTER TABLE sale_payments ADD COLUMN cash_session_id BIGINT REFERENCES cash_sessions(id) ON DELETE SET NULL;
ALTER TABLE sale_payments DROP CONSTRAINT sale_payments_method_check;
ALTER TABLE sale_payments ADD CONSTRAINT sale_payments_method_check CHECK (method IN ('cash', 'card', 'transfer', 'mp_point'));
ALTER TABLE sale_payments ADD COLUMN mp_charge_id TEXT;
ALTER TABLE sale_payments ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX sale_payments_session ON sale_payments (cash_session_id);

-- Libro de inventario: cada cambio de existencia deja un movimiento
CREATE TABLE stock_movements (
  id          BIGSERIAL PRIMARY KEY,
  business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  delta       INT NOT NULL CHECK (delta <> 0),
  reason      TEXT NOT NULL CHECK (reason IN ('initial', 'purchase', 'sale', 'void', 'adjustment', 'loss')),
  sale_id     BIGINT REFERENCES sales(id) ON DELETE SET NULL,
  note        TEXT NOT NULL DEFAULT '',
  balance     INT NOT NULL CHECK (balance >= 0),
  created_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX stock_movements_product ON stock_movements (product_id, created_at DESC);

-- Comisiones: regla específica por servicio o producto (y opcionalmente por persona).
-- Prioridad: persona + artículo > artículo > comisión general de la persona (solo servicios).
CREATE TABLE commission_rules (
  id          BIGSERIAL PRIMARY KEY,
  business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  user_id     BIGINT REFERENCES users(id) ON DELETE CASCADE,
  service_id  BIGINT REFERENCES services(id) ON DELETE CASCADE,
  product_id  BIGINT REFERENCES products(id) ON DELETE CASCADE,
  percent     NUMERIC(5,2) NOT NULL CHECK (percent BETWEEN 0 AND 100),
  active      BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((service_id IS NULL) <> (product_id IS NULL))
);
CREATE INDEX commission_rules_business ON commission_rules (business_id, active);

-- Mercado Pago (terminal Point). El token se guarda cifrado.
CREATE TABLE mp_accounts (
  business_id   BIGINT PRIMARY KEY REFERENCES businesses(id) ON DELETE CASCADE,
  token_enc     BYTEA NOT NULL,
  connected_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- una terminal por sucursal
CREATE TABLE mp_terminals (
  branch_id   BIGINT PRIMARY KEY REFERENCES branches(id) ON DELETE CASCADE,
  business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  terminal_id TEXT NOT NULL,
  label       TEXT NOT NULL DEFAULT ''
);

-- Cada cobro enviado a la terminal. Un pago aprobado se «reclama» una sola vez al crear la venta.
CREATE TABLE mp_charges (
  id            TEXT PRIMARY KEY, -- id de la orden en Mercado Pago
  business_id   BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  branch_id     BIGINT NOT NULL REFERENCES branches(id),
  amount_cents  INT NOT NULL CHECK (amount_cents > 0),
  status        TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'approved', 'canceled', 'error')),
  external_ref  TEXT NOT NULL,
  mp_payment_id TEXT,
  detail        TEXT NOT NULL DEFAULT '',
  sale_id       BIGINT REFERENCES sales(id) ON DELETE SET NULL,
  refunded_at   TIMESTAMPTZ,
  created_by    BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX mp_charges_business ON mp_charges (business_id, created_at DESC);
CREATE INDEX mp_charges_sale ON mp_charges (sale_id);
