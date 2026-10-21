package api

import (
	"context"
	"errors"
	"log"

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
