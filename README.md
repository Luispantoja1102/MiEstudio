# MiEstudio

Agenda de citas y punto de venta para barberías, estéticas y spas.

| Capa | Tecnología |
| --- | --- |
| Frontend | SvelteKit (Svelte 5, SPA) con tema claro y oscuro intercambiables |
| Backend | Go (`net/http`, pgx) |
| Base de datos | PostgreSQL 16 (migraciones en `Backend/go/internal/db/migrations`, se aplican solas al arrancar) |

```
Frontend/    Interfaz: landing, login, registro y panel
Backend/go/  API: autenticación, equipo y (próximamente) agenda y POS
Backend/node/ Servicio auxiliar (por ahora solo /health)
```

## Empezar

Necesitas **Go 1.26+**, **Node 22+** y **Docker** (o un PostgreSQL 16 propio).

```bash
cp .env.example .env     # cambia POSTGRES_PASSWORD y JWT_SECRET (mínimo 32 caracteres)
npm install              # instala la raíz, Frontend y Backend/node
npm run db               # levanta PostgreSQL con Docker
npm run dev              # arranca Frontend, API de Node y API de Go juntos
```

Abre <http://localhost:5173>. El `.env` vive en la raíz y no se sube a git.

## Pruebas

Las pruebas del backend crean una base temporal, así que necesitan un usuario de Postgres que pueda crear bases:

```bash
TEST_DATABASE_URL="postgres://miestudio:change_me@localhost:5432/postgres?sslmode=disable" \
  go -C Backend/go test ./...
```

Sin `TEST_DATABASE_URL`, las pruebas que usan la base se omiten.

## Producción

```bash
docker build -t miestudio .
docker run -p 8080:8080 -e DATABASE_URL=... -e JWT_SECRET=... miestudio
```

Una sola imagen sirve la API (`/api/*`) y el frontend compilado. Pon HTTPS delante con un proxy.
Nota: el límite de intentos usa la IP de la conexión; detrás de un proxy hay que configurar la IP real antes de depender de él.

## API actual

| Método y ruta | Quién |
| --- | --- |
| `POST /api/auth/register`, `POST /api/auth/login` | Público (con límite de intentos) |
| `GET /api/me`, `POST /api/auth/password` | Cualquier usuario autenticado |
| `GET/POST /api/team`, `PATCH /api/team/{id}` | Solo administrador, limitado a su negocio |
