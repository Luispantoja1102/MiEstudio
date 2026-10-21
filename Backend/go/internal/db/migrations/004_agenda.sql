-- La preparación entre citas (buffer) se bloquea en la base: blocked_until = fin de la cita + buffer.
ALTER TABLE appointments ADD COLUMN blocked_until TIMESTAMPTZ;
UPDATE appointments SET blocked_until = ends_at;
ALTER TABLE appointments ALTER COLUMN blocked_until SET NOT NULL;
ALTER TABLE appointments ADD CONSTRAINT appointments_blocked_after_end CHECK (blocked_until >= ends_at);

ALTER TABLE appointments DROP CONSTRAINT appointments_staff_id_tstzrange_excl;
ALTER TABLE appointments ADD CONSTRAINT appointments_staff_no_overlap EXCLUDE USING gist (
  staff_id WITH =,
  tstzrange(starts_at, blocked_until) WITH &&
) WHERE (status IN ('scheduled', 'completed'));

CREATE INDEX appointments_staff_time ON appointments (staff_id, starts_at);
CREATE INDEX time_blocks_user_time ON time_blocks (user_id, starts_at);
