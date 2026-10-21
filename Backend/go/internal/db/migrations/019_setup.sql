-- Asistente de inicio: se muestra al titular de un negocio nuevo hasta que lo termina u omite.
-- Los negocios que ya existían no lo ven.
ALTER TABLE businesses ADD COLUMN setup_completed_at TIMESTAMPTZ;
UPDATE businesses SET setup_completed_at = now();
