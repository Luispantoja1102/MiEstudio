-- Recordatorios por WhatsApp (API en la nube de Meta): consentimiento propio, interruptor por negocio y canal en la bandeja.
ALTER TABLE clients ADD COLUMN whatsapp_consent BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE booking_settings ADD COLUMN whatsapp_reminders BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE notifications ADD COLUMN channel TEXT NOT NULL DEFAULT 'email' CHECK (channel IN ('email', 'whatsapp'));
ALTER TABLE notifications ADD COLUMN payload JSONB;
-- el mismo recordatorio puede salir por correo y por WhatsApp
ALTER TABLE notifications DROP CONSTRAINT notifications_appointment_id_kind_for_starts_at_key;
ALTER TABLE notifications ADD CONSTRAINT notifications_appointment_kind_starts_channel_key UNIQUE (appointment_id, kind, for_starts_at, channel);
DROP INDEX IF EXISTS notifications_due;
CREATE INDEX notifications_due ON notifications (send_after) WHERE status = 'pending';
