-- El tema «Lino» pasa a ser una opción más (y el predeterminado de la aplicación).
ALTER TABLE booking_settings DROP CONSTRAINT booking_settings_public_theme_check;
ALTER TABLE booking_settings ADD CONSTRAINT booking_settings_public_theme_check CHECK (public_theme IN ('auto', 'lino', 'light', 'ivory', 'dark'));
