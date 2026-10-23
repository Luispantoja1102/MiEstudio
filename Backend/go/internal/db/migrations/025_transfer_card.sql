-- Además de la CLABE, el titular puede dar el número de su tarjeta para recibir transferencias (o las dos cosas).
ALTER TABLE booking_settings ADD COLUMN transfer_card TEXT NOT NULL DEFAULT '';
