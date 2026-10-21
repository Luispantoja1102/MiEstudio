-- Sesiones revocables y recuperación de contraseña.

-- Cada token de sesión lleva la versión de sesión de la persona. Subirla cierra todas sus sesiones.
-- Un disparador la sube sola cuando cambia lo que obliga a volver a entrar (clave, rol o baja),
-- así ninguna ruta, ni futura, puede olvidarlo.
ALTER TABLE users ADD COLUMN session_version INT NOT NULL DEFAULT 0;

CREATE FUNCTION users_bump_session() RETURNS trigger AS $$
BEGIN
  IF NEW.password_hash IS DISTINCT FROM OLD.password_hash
     OR NEW.role IS DISTINCT FROM OLD.role
     OR (OLD.active AND NOT NEW.active)
     OR NEW.email IS DISTINCT FROM OLD.email THEN
    NEW.session_version := OLD.session_version + 1;
  END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER users_bump_session BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION users_bump_session();

-- Enlaces para restablecer la contraseña: solo se guarda el hash del token.
CREATE TABLE password_resets (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX password_resets_user ON password_resets (user_id);
