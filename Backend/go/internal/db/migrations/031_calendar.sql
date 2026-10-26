-- Calendario: el correo de una cita lleva su archivo .ics (se guarda en la bandeja) y cada profesional puede activar
-- un calendario privado al que suscribirse desde Google Calendar. Solo sale información: nunca se lee el calendario de nadie.
ALTER TABLE notifications ADD COLUMN ics TEXT NOT NULL DEFAULT '';
ALTER TABLE users
  ADD COLUMN calendar_token      TEXT UNIQUE,                          -- el secreto de la dirección del calendario; NULL = apagado
  ADD COLUMN calendar_show_names BOOLEAN NOT NULL DEFAULT false;       -- nombres de clientes en los eventos (apagado: solo «Cita»)
