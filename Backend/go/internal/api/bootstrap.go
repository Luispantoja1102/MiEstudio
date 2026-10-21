package api

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"

	"miestudio/backend-go/internal/auth"
)

// BootstrapPlatformAdmin crea el primer administrador de plataforma desde la configuración
// si todavía no existe ninguno. Es idempotente: no toca a quien ya exista.
func (a *API) BootstrapPlatformAdmin(ctx context.Context, name, email, password string) error {
	if email == "" || password == "" {
		return nil
	}
	if !validEmail(email) || !validPassword(password) {
		return errors.New("PLATFORM_ADMIN_EMAIL o PLATFORM_ADMIN_PASSWORD inválidos (contraseña de 8 a 72 caracteres)")
	}
	if a.Production && password == ExampleAdminPassword {
		return errors.New("PLATFORM_ADMIN_PASSWORD es la clave de ejemplo: cámbiala antes de usar APP_ENV=production")
	}
	var exists bool
	if err := a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE role = 'platform_admin')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	if name == "" {
		name = "Administrador"
	}
	if _, err := a.insertPlatformUser(ctx, name, email, password, auth.RolePlatformAdmin); err != nil {
		return err
	}
	log.Printf("administrador de plataforma creado: %s", email)
	return nil
}

// ExampleAdminPassword es la clave de .env.example, solo para pruebas locales.
const ExampleAdminPassword = "AdminPlataforma123"

// UpsertPlatformAdmin crea un administrador de plataforma o, si ya existe una persona de
// plataforma con ese correo, le restablece la contraseña, la reactiva y la deja como administrador.
// Nunca toca a personas que pertenecen a un negocio.
func (a *API) UpsertPlatformAdmin(ctx context.Context, name, email, password string) (created bool, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) || !validPassword(password) {
		return false, errors.New("correo inválido o contraseña fuera de 8 a 72 caracteres")
	}
	if a.Production && password == ExampleAdminPassword {
		return false, errors.New("no uses la clave de ejemplo en producción")
	}
	var id int64
	var bizID *int64
	err = a.DB.QueryRow(ctx, `SELECT id, business_id FROM users WHERE email = $1`, email).Scan(&id, &bizID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if strings.TrimSpace(name) == "" {
			name = "Administrador"
		}
		_, err = a.insertPlatformUser(ctx, name, email, password, auth.RolePlatformAdmin)
		return err == nil, err
	case err != nil:
		return false, err
	case bizID != nil:
		return false, errors.New("ese correo pertenece a una persona de un negocio; usa otro")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return false, err
	}
	_, err = a.DB.Exec(ctx, `UPDATE users SET password_hash = $2, role = 'platform_admin', active = true WHERE id = $1`, id, hash)
	return false, err
}
