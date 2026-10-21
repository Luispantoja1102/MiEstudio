-- Mensajes a clientes: felicitación de cumpleaños, reactivación y encuesta de satisfacción con enlace de reseña.
-- Las promociones solo van a quien las aceptó (marketing_consent); toda persona puede darse de baja con su enlace.
ALTER TABLE clients ADD COLUMN marketing_consent BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE clients ADD COLUMN unsub_token TEXT UNIQUE;

CREATE TABLE engagement_settings (
  business_id        BIGINT PRIMARY KEY REFERENCES businesses(id) ON DELETE CASCADE,
  birthday_enabled   BOOLEAN NOT NULL DEFAULT false,
  birthday_message   TEXT NOT NULL DEFAULT '',
  winback_enabled    BOOLEAN NOT NULL DEFAULT false,
  winback_days       INT NOT NULL DEFAULT 60 CHECK (winback_days BETWEEN 14 AND 365),
  winback_message    TEXT NOT NULL DEFAULT '',
  survey_enabled     BOOLEAN NOT NULL DEFAULT false,
  survey_delay_hours INT NOT NULL DEFAULT 24 CHECK (survey_delay_hours BETWEEN 1 AND 168),
  review_url         TEXT NOT NULL DEFAULT ''
);

CREATE TABLE surveys (
  id             BIGSERIAL PRIMARY KEY,
  business_id    BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  appointment_id BIGINT NOT NULL UNIQUE REFERENCES appointments(id) ON DELETE CASCADE,
  client_id      BIGINT NOT NULL REFERENCES clients(id),
  token          TEXT NOT NULL UNIQUE,
  score          INT CHECK (score BETWEEN 1 AND 5),
  comment        TEXT NOT NULL DEFAULT '',
  answered_at    TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX surveys_business ON surveys (business_id, created_at DESC);
CREATE INDEX surveys_client ON surveys (client_id, created_at DESC);
