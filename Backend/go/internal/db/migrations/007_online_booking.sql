-- Reserva en línea, anticipos y recordatorios.

ALTER TABLE services ADD COLUMN online_bookable BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE clients ADD COLUMN reminders_consent BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE booking_settings (
  business_id      BIGINT PRIMARY KEY REFERENCES businesses(id) ON DELETE CASCADE,
  enabled          BOOLEAN NOT NULL DEFAULT false,
  min_notice_hours INT NOT NULL DEFAULT 2   CHECK (min_notice_hours BETWEEN 0 AND 720),
  max_days_ahead   INT NOT NULL DEFAULT 30  CHECK (max_days_ahead BETWEEN 1 AND 365),
  cancel_min_hours INT NOT NULL DEFAULT 2   CHECK (cancel_min_hours BETWEEN 0 AND 720),
  deposit_percent  INT NOT NULL DEFAULT 0   CHECK (deposit_percent BETWEEN 0 AND 100),
  reminder_hours   INT NOT NULL DEFAULT 24  CHECK (reminder_hours BETWEEN 0 AND 168), -- 0 = sin recordatorios
  show_prices      BOOLEAN NOT NULL DEFAULT true,
  welcome_text     TEXT NOT NULL DEFAULT ''
);

-- Citas: estado «pendiente de pago» (aparta el horario mientras se espera el anticipo) y datos de la reserva en línea
ALTER TABLE appointments DROP CONSTRAINT appointments_status_check;
ALTER TABLE appointments ADD CONSTRAINT appointments_status_check
  CHECK (status IN ('scheduled', 'completed', 'no_show', 'cancelled', 'pending_payment'));
ALTER TABLE appointments ADD COLUMN source TEXT NOT NULL DEFAULT 'staff' CHECK (source IN ('staff', 'online'));
ALTER TABLE appointments ADD COLUMN public_token TEXT;
ALTER TABLE appointments ADD CONSTRAINT appointments_public_token_key UNIQUE (public_token);
ALTER TABLE appointments ADD COLUMN hold_expires_at TIMESTAMPTZ;
ALTER TABLE appointments ADD COLUMN reminders_consent BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE appointments ADD COLUMN privacy_accepted_at TIMESTAMPTZ;

-- el horario también se aparta mientras se espera el anticipo
ALTER TABLE appointments DROP CONSTRAINT appointments_staff_no_overlap;
ALTER TABLE appointments ADD CONSTRAINT appointments_staff_no_overlap EXCLUDE USING gist (
  staff_id WITH =,
  tstzrange(starts_at, blocked_until) WITH &&
) WHERE (status IN ('scheduled', 'completed', 'pending_payment'));
CREATE INDEX appointments_holds ON appointments (hold_expires_at) WHERE status = 'pending_payment';

-- Anticipos (Mercado Pago). 'applied' = ya se descontó en una venta; 'kept' = se conserva (inasistencia)
CREATE TABLE deposits (
  id               BIGSERIAL PRIMARY KEY,
  business_id      BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  appointment_id   BIGINT NOT NULL UNIQUE REFERENCES appointments(id) ON DELETE CASCADE,
  amount_cents     INT NOT NULL CHECK (amount_cents > 0),
  status           TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'applied', 'refunded', 'kept', 'expired')),
  external_ref     TEXT NOT NULL UNIQUE,
  mp_preference_id TEXT,
  pay_url          TEXT NOT NULL DEFAULT '',
  mp_payment_id    TEXT,
  paid_at          TIMESTAMPTZ,
  refunded_at      TIMESTAMPTZ,
  sale_id          BIGINT REFERENCES sales(id) ON DELETE SET NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE sale_payments DROP CONSTRAINT sale_payments_method_check;
ALTER TABLE sale_payments ADD CONSTRAINT sale_payments_method_check
  CHECK (method IN ('cash', 'card', 'transfer', 'mp_point', 'deposit'));
ALTER TABLE sale_payments ADD COLUMN deposit_id BIGINT REFERENCES deposits(id);

-- Bandeja de salida de correos (confirmaciones, recordatorios, cancelaciones). Una por cita, tipo y hora.
CREATE TABLE notifications (
  id             BIGSERIAL PRIMARY KEY,
  business_id    BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  appointment_id BIGINT NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
  kind           TEXT NOT NULL CHECK (kind IN ('booking', 'reminder', 'cancellation', 'deposit')),
  for_starts_at  TIMESTAMPTZ NOT NULL,
  to_addr        TEXT NOT NULL,
  subject        TEXT NOT NULL,
  body_text      TEXT NOT NULL,
  body_html      TEXT NOT NULL DEFAULT '',
  status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed', 'skipped')),
  attempts       INT NOT NULL DEFAULT 0,
  last_error     TEXT NOT NULL DEFAULT '',
  send_after     TIMESTAMPTZ NOT NULL DEFAULT now(),
  sent_at        TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (appointment_id, kind, for_starts_at)
);
CREATE INDEX notifications_due ON notifications (send_after) WHERE status = 'pending';
