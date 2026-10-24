-- Nombre de usuario para entrar, además del correo. Se guarda siempre en minúsculas, así que "Ana" y "ana" son el mismo.
-- No lleva "@": un usuario nunca se confunde con un correo.
ALTER TABLE users ADD COLUMN username TEXT;

-- A quien ya existe se le asigna uno a partir de su correo (la parte antes de la @), sin repetir.
DO $$
DECLARE
  r RECORD;
  base TEXT;
  cand TEXT;
  n INT;
BEGIN
  FOR r IN SELECT id, email FROM users ORDER BY id LOOP
    base := regexp_replace(lower(split_part(r.email, '@', 1)), '[^a-z0-9._-]', '', 'g');
    IF length(base) < 3 THEN
      base := base || 'user';
    END IF;
    base := left(base, 24);
    cand := base;
    n := 1;
    WHILE EXISTS (SELECT 1 FROM users WHERE username = cand) LOOP
      n := n + 1;
      cand := base || n::text;
    END LOOP;
    UPDATE users SET username = cand WHERE id = r.id;
  END LOOP;
END $$;

ALTER TABLE users ALTER COLUMN username SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_username_format CHECK (username ~ '^[a-z0-9._-]{3,30}$');
CREATE UNIQUE INDEX users_username_key ON users (username);

-- el inicio de sesión busca el correo sin importar mayúsculas
CREATE INDEX users_email_lower_idx ON users (lower(email));
