-- Paquetes y membresías: se venden en caja y se canjean sesión por sesión.
CREATE TABLE packages (
  id          BIGSERIAL PRIMARY KEY,
  business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  service_id  BIGINT NOT NULL REFERENCES services(id),
  sessions    INT NOT NULL CHECK (sessions BETWEEN 2 AND 100),
  price_cents INT NOT NULL CHECK (price_cents >= 0),
  valid_days  INT CHECK (valid_days IS NULL OR valid_days BETWEEN 1 AND 3650),
  active      BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX packages_business ON packages (business_id);

-- Lo que un cliente compró: guarda una copia del nombre/servicio para que cambiar el catálogo no altere saldos.
CREATE TABLE client_packages (
  id             BIGSERIAL PRIMARY KEY,
  business_id    BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  client_id      BIGINT NOT NULL REFERENCES clients(id),
  package_id     BIGINT REFERENCES packages(id) ON DELETE SET NULL,
  sale_id        BIGINT NOT NULL REFERENCES sales(id),
  name           TEXT NOT NULL,
  service_id     BIGINT NOT NULL REFERENCES services(id),
  sessions_total INT NOT NULL CHECK (sessions_total > 0),
  sessions_used  INT NOT NULL DEFAULT 0 CHECK (sessions_used >= 0 AND sessions_used <= sessions_total),
  expires_at     TIMESTAMPTZ,
  voided_at      TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX client_packages_client ON client_packages (business_id, client_id);
CREATE INDEX client_packages_sale ON client_packages (sale_id);

ALTER TABLE sale_items ADD COLUMN package_id BIGINT REFERENCES packages(id);
ALTER TABLE sale_items DROP CONSTRAINT sale_items_check;
ALTER TABLE sale_items ADD CONSTRAINT sale_items_one_kind
  CHECK (((service_id IS NOT NULL)::int + (product_id IS NOT NULL)::int + (package_id IS NOT NULL)::int) = 1);

ALTER TABLE sale_payments ADD COLUMN client_package_id BIGINT REFERENCES client_packages(id);
ALTER TABLE sale_payments DROP CONSTRAINT sale_payments_method_check;
ALTER TABLE sale_payments ADD CONSTRAINT sale_payments_method_check
  CHECK (method IN ('cash', 'card', 'transfer', 'mp_point', 'deposit', 'package'));
