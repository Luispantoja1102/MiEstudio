-- Privacidad: aceptación de términos, aviso por reserva, solicitudes ARCO y borrado (anonimización) de clientes.

-- Quien registra un negocio acepta los términos y el aviso de la plataforma: se guarda qué versión y cuándo.
ALTER TABLE users ADD COLUMN terms_version     TEXT;
ALTER TABLE users ADD COLUMN terms_accepted_at TIMESTAMPTZ;

-- Cada reserva en línea guarda qué versión del aviso aceptó el cliente.
ALTER TABLE appointments ADD COLUMN privacy_version TEXT;

-- Correo del negocio para solicitudes de privacidad (se muestra en su aviso y en la página de derechos).
ALTER TABLE booking_settings ADD COLUMN privacy_email TEXT NOT NULL DEFAULT '';

-- Un cliente anonimizado conserva sus ventas (obligación fiscal) pero pierde todo dato personal.
ALTER TABLE clients ADD COLUMN anonymized_at TIMESTAMPTZ;

-- Solicitudes ARCO (acceso, rectificación, cancelación, oposición) que un cliente envía a un negocio.
CREATE TABLE arco_requests (
  id            BIGSERIAL PRIMARY KEY,
  business_id   BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  client_id     BIGINT REFERENCES clients(id) ON DELETE SET NULL,
  kind          TEXT NOT NULL CHECK (kind IN ('access', 'rectify', 'cancel', 'oppose')),
  requester     TEXT NOT NULL,
  email         TEXT NOT NULL DEFAULT '',
  phone         TEXT NOT NULL DEFAULT '',
  detail        TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'received' CHECK (status IN ('received', 'in_progress', 'resolved', 'rejected')),
  resolution    TEXT NOT NULL DEFAULT '',
  public_token  TEXT NOT NULL UNIQUE,
  due_at        TIMESTAMPTZ NOT NULL,         -- 20 días hábiles desde que se recibe
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at   TIMESTAMPTZ
);
CREATE INDEX arco_requests_business ON arco_requests (business_id, status, due_at);
