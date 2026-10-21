-- Lista de espera: quien no encontró horario deja sus datos y se le avisa por correo cuando se libera uno.
CREATE TABLE waitlist (
  id          BIGSERIAL PRIMARY KEY,
  business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  branch_id   BIGINT NOT NULL REFERENCES branches(id),
  service_id  BIGINT NOT NULL REFERENCES services(id),
  staff_id    BIGINT REFERENCES users(id),       -- NULL = con cualquiera
  client_id   BIGINT NOT NULL REFERENCES clients(id),
  day         DATE NOT NULL,                     -- día local del negocio
  from_min    INT CHECK (from_min BETWEEN 0 AND 1439),   -- ventana opcional, minutos desde medianoche
  to_min      INT CHECK (to_min BETWEEN 1 AND 1440),
  status      TEXT NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'notified', 'booked', 'cancelled', 'expired')),
  token       TEXT NOT NULL UNIQUE,
  source      TEXT NOT NULL DEFAULT 'panel' CHECK (source IN ('panel', 'online')),
  notified_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((from_min IS NULL) = (to_min IS NULL)),
  CHECK (from_min IS NULL OR from_min < to_min)
);
CREATE INDEX waitlist_open ON waitlist (business_id, day) WHERE status IN ('waiting', 'notified');
CREATE INDEX waitlist_client ON waitlist (client_id);

-- La bandeja de correos deja de depender de una cita: avisos de lista de espera, cumpleaños, etc.
ALTER TABLE notifications ALTER COLUMN appointment_id DROP NOT NULL;
ALTER TABLE notifications ALTER COLUMN for_starts_at DROP NOT NULL;
ALTER TABLE notifications ADD COLUMN client_id BIGINT REFERENCES clients(id) ON DELETE CASCADE;
ALTER TABLE notifications ADD COLUMN waitlist_id BIGINT REFERENCES waitlist(id) ON DELETE CASCADE;
ALTER TABLE notifications ADD COLUMN dedupe_key TEXT;
CREATE UNIQUE INDEX notifications_dedupe ON notifications (business_id, dedupe_key) WHERE dedupe_key IS NOT NULL;
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
  CHECK (kind IN ('booking', 'reminder', 'cancellation', 'deposit', 'waitlist', 'birthday', 'winback', 'survey'));
