-- Códigos de verificación del correo al registrarse. Solo se guarda el hash del código; el registro lo gasta.
CREATE TABLE signup_codes (
  email      TEXT PRIMARY KEY,
  code_hash  TEXT NOT NULL,
  tries      INT NOT NULL DEFAULT 0,
  sent_at    TIMESTAMPTZ NOT NULL,
  sends      TIMESTAMPTZ[] NOT NULL DEFAULT '{}', -- envíos de la última hora, para el tope por hora
  expires_at TIMESTAMPTZ NOT NULL
);
