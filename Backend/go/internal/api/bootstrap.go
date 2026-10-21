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
	// Quién administra hoy la plataforma: se informa siempre para que no haya dudas.
	var admins []string
	rows, err := a.DB.Query(ctx, `SELECT email FROM users WHERE role = 'platform_admin' AND active ORDER BY id LIMIT 5`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			rows.Close()
			return err
		}
		admins = append(admins, e)
	}
	rows.Close()

	if email == "" || password == "" {
		if len(admins) == 0 {
			log.Printf("plataforma: NO hay administrador. Define PLATFORM_ADMIN_EMAIL y PLATFORM_ADMIN_PASSWORD en .env, o crea uno con: npm run admin -- -email tu@correo.com -password '...'")
		} else {
			log.Printf("plataforma: administradores existentes: %s", strings.Join(admins, ", "))
		}
		return nil
	}
	if !validEmail(email) || !validPassword(password) {
		return errors.New("PLATFORM_ADMIN_EMAIL o PLATFORM_ADMIN_PASSWORD inválidos (contraseña de 8 a 72 caracteres)")
	}
	if a.Production && password == ExampleAdminPassword {
		return errors.New("PLATFORM_ADMIN_PASSWORD es la clave de ejemplo: cámbiala antes de usar APP_ENV=production")
	}
	if len(admins) > 0 {
		// El .env solo crea al PRIMER administrador. Si ya hay uno, sus valores no se aplican.
		log.Printf("plataforma: ya existe un administrador (%s); el .env no crea otro ni cambia contraseñas. "+
			"Para crear otro o restablecer una contraseña: npm run admin -- -email %s -password '...'", strings.Join(admins, ", "), email)
		return nil
	}
	if name == "" {
		name = "Administrador"
	}
	if _, err := a.insertPlatformUser(ctx, name, email, password, auth.RolePlatformAdmin); err != nil {
		return err
	}
	log.Printf("plataforma: administrador creado: %s", email)
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

// ExampleDemoPassword es la clave de ejemplo del negocio de prueba de .env.example.
const ExampleDemoPassword = "NegocioDemo123"

// BootstrapDemoBusiness crea un negocio de ejemplo (activo, con su administrador) en la primera ejecución,
// para poder probar sin registrarse. Solo actúa si no hay ningún negocio; después nunca vuelve a crearlo
// ni toca uno existente. Es opcional: sin DEMO_ADMIN_EMAIL/DEMO_ADMIN_PASSWORD no hace nada.
func (a *API) BootstrapDemoBusiness(ctx context.Context, business, kind, name, email, password string) error {
	if email == "" || password == "" {
		return nil
	}
	var n int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM businesses`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if a.Production && password == ExampleDemoPassword {
		return errors.New("DEMO_ADMIN_PASSWORD es la clave de ejemplo: no se crea el negocio de prueba en producción")
	}
	if business == "" {
		business = "Negocio de ejemplo"
	}
	if kind == "" {
		kind = "barberia"
	}
	if name == "" {
		name = "Dueño"
	}
	req := registerReq{BusinessName: business, Kind: kind, Name: name, Email: email, Password: password}
	if msg := validateRegister(&req); msg != "" {
		return errors.New("DEMO_*: " + msg)
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}
	bizID, _, err := a.registerBusiness(ctx, req, hash, "active")
	if err != nil {
		return err
	}
	a.logActivity(ctx, 0, bizID, "demo_business_created", map[string]any{"email": req.Email})
	log.Printf("negocio de ejemplo creado: entra con %s", req.Email)
	return nil
}
