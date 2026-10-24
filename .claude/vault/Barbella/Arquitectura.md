# Arquitectura

- **Backend:** Go (`net/http` + pgx) en `Backend/go/`. Rutas en `internal/api/api.go`. Migraciones SQL numeradas en `internal/db/migrations/` (hoy hasta la 025).
- **Base de datos:** PostgreSQL 16.
- **Frontend:** SvelteKit 2 / Svelte 5 (SPA estática) en `Frontend/`. Estilos con tokens en `Frontend/src/app.css`.
- **Roles de negocio:** dueño (admin), recepción, personal. Rol aparte: administrador de la plataforma (`/plataforma`).

## Correr y probar
- Desarrollo (raíz del repo): `npm run dev` (Vite en 5173, API en 8080). Necesita `.env` (ver `.env.example`).
- Pruebas del backend: `cd Backend/go && TEST_DATABASE_URL=... go test ./... -timeout 600s` (unos 200 s).
- Revisión del frontend: `cd Frontend && npx svelte-check --threshold error && npm run build`.
- Administrador de plataforma: `npm run admin -- -email ... -password ...`.

## Dónde está cada cosa
- Planes y límites: `plans.go`, `plan_admin.go`.
- Anticipos y transferencias: `deposits.go`, `transfer.go`, `booking_settings.go`.
- Reserva pública: `public_booking.go`. Agenda: `agenda.go`.
- Landing nueva: `Frontend/src/lib/jornada/`. La landing vieja (`lib/landing/`) ya no se usa.
