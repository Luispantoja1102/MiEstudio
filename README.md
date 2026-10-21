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

## Administrador de la plataforma

La plataforma (ustedes) no maneja locales: controla quién se registra y en qué estado está cada negocio.

- Al arrancar, la API crea un administrador con `PLATFORM_ADMIN_EMAIL` y `PLATFORM_ADMIN_PASSWORD` del `.env` si todavía no existe ninguno. `.env.example` trae valores de **prueba local**; con `APP_ENV=production` la API se niega a usar la clave de ejemplo.
- Entra en `/login`: te lleva a `/plataforma` (resumen, negocios, equipo, actividad y ajustes).
- Crear otro administrador o restablecer una contraseña olvidada, cuando quieras:

```bash
npm run admin -- -email otra@persona.com -password 'ClaveLarga123' -name 'Nombre'
```

- **Registro de negocios** (Ajustes de plataforma): `abierto` (prueba de 14 días al instante), `con aprobación` (queda pendiente hasta que la plataforma lo apruebe) o `cerrado` (solo la plataforma da de alta).
- Estados de un negocio: pendiente, en prueba, activo y suspendido. Un negocio pendiente o suspendido no puede usar la plataforma.
- El rol de soporte (`platform_support`) es de solo lectura.

## Agenda

- Cada persona «en agenda» tiene un horario semanal (Equipo → Horario). Los horarios libres se calculan con ese horario, las citas, los bloqueos y la preparación entre citas.
- La base de datos impide por sí misma que una persona tenga dos citas encimadas.
- Las horas se guardan en UTC y se muestran en la zona horaria del negocio (por defecto `America/Tijuana`).

## API actual

| Método y ruta | Quién |
| --- | --- |
| `GET /api/auth/registration`, `POST /api/auth/register`, `POST /api/auth/login` | Público (con límite de intentos) |
| `GET /api/me`, `POST /api/auth/password` | Cualquier usuario autenticado |
| `GET/POST /api/team`, `PATCH /api/team/{id}`, `GET/PUT /api/team/{id}/hours` | Administrador del negocio |
| `GET /api/services`, `/api/products`, `/api/branches` | Todo el equipo del negocio |
| `POST/PATCH` de servicios, productos y sucursales; `PUT /api/services/{id}/staff` | Administrador del negocio |
| `/api/clients`, `/api/clients/{id}` | Todo el equipo del negocio |
| `/api/clients/{id}/notes` | Administrador y profesionales |
| `GET /api/agenda`, `GET /api/agenda/slots` | Todo el equipo del negocio |
| `POST/PATCH /api/appointments`, `POST /api/appointments/{id}/move`, `POST/DELETE /api/blocks` | Administrador, recepción y profesionales (estos últimos solo bloquean su propio horario) |
| `/api/platform/*` | Plataforma (soporte solo lee) |

Todo lo de negocio se limita al negocio de quien hace la petición, y se bloquea si la cuenta está pendiente o suspendida.
