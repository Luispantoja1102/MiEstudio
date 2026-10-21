-- Tema con el que el negocio muestra sus páginas públicas (reserva, cita, aviso de privacidad).
-- 'auto' respeta la elección de quien visita; los demás lo fijan.
ALTER TABLE booking_settings ADD COLUMN public_theme TEXT NOT NULL DEFAULT 'auto' CHECK (public_theme IN ('auto', 'light', 'ivory', 'dark'));
