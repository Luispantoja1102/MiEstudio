-- Confirmada y llegó: marcas sobre una cita agendada (el estado sigue siendo 'scheduled' hasta que se atiende).
ALTER TABLE appointments ADD COLUMN confirmed_at TIMESTAMPTZ, ADD COLUMN arrived_at TIMESTAMPTZ;

-- Quien elige un horario en la página pública lo aparta unos minutos mientras llena sus datos,
-- para que dos personas no lleguen al mismo horario sin saberlo.
CREATE TABLE booking_holds (
  id          BIGSERIAL PRIMARY KEY,
  business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  staff_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  starts_at   TIMESTAMPTZ NOT NULL,
  ends_at     TIMESTAMPTZ NOT NULL,
  holder      TEXT NOT NULL CHECK (length(holder) BETWEEN 16 AND 64),
  expires_at  TIMESTAMPTZ NOT NULL,
  CHECK (ends_at > starts_at)
);
CREATE INDEX booking_holds_staff ON booking_holds (staff_id, starts_at);
CREATE INDEX booking_holds_holder ON booking_holds (holder);
