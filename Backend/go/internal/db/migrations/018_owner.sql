-- Titular del negocio: el primer administrador (quien lo registró). Solo él decide el tema de la página pública.
ALTER TABLE businesses ADD COLUMN owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
UPDATE businesses b SET owner_user_id = (SELECT u.id FROM users u WHERE u.business_id = b.id AND u.role = 'admin' ORDER BY u.id LIMIT 1);
