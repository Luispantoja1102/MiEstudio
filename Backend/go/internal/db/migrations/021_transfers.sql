-- Anticipo por transferencia: el negocio da sus datos bancarios, el cliente (o el personal) sube el comprobante
-- y el titular confirma la cita al verlo.
ALTER TABLE booking_settings
  ADD COLUMN transfer_enabled    BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN transfer_bank       TEXT NOT NULL DEFAULT '',
  ADD COLUMN transfer_holder     TEXT NOT NULL DEFAULT '',
  ADD COLUMN transfer_clabe      TEXT NOT NULL DEFAULT '',
  ADD COLUMN transfer_whatsapp   TEXT NOT NULL DEFAULT '',
  ADD COLUMN transfer_hold_hours INT NOT NULL DEFAULT 12 CHECK (transfer_hold_hours BETWEEN 1 AND 168);

ALTER TABLE deposits
  ADD COLUMN method        TEXT NOT NULL DEFAULT 'mp' CHECK (method IN ('mp', 'transfer')),
  ADD COLUMN proof_at      TIMESTAMPTZ,                 -- cuándo llegó el comprobante (nulo = aún no)
  ADD COLUMN reject_reason TEXT NOT NULL DEFAULT '',
  ADD COLUMN confirmed_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN refund_due    BOOLEAN NOT NULL DEFAULT false; -- una transferencia no se devuelve sola: el negocio lo hace a mano

CREATE TABLE deposit_proofs (
  deposit_id   BIGINT PRIMARY KEY REFERENCES deposits(id) ON DELETE CASCADE,
  content_type TEXT NOT NULL,
  filename     TEXT NOT NULL DEFAULT '',
  data         BYTEA NOT NULL,
  uploaded_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Bandeja de avisos dentro de la plataforma (la campana). Cada persona recuerda hasta qué aviso ya vio.
CREATE TABLE inbox (
  id             BIGSERIAL PRIMARY KEY,
  business_id    BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  kind           TEXT NOT NULL,
  title          TEXT NOT NULL,
  body           TEXT NOT NULL DEFAULT '',
  link           TEXT NOT NULL DEFAULT '',
  appointment_id BIGINT REFERENCES appointments(id) ON DELETE SET NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX inbox_business ON inbox (business_id, id DESC);
ALTER TABLE users ADD COLUMN inbox_seen_id BIGINT NOT NULL DEFAULT 0;
