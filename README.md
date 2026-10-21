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

`npm run dev` revisa tu entorno antes de arrancar y, si algo falla, **dice por qué y se detiene** (falta el `.env`, falta una variable, PostgreSQL apagado, la API no compila o no arranca). Los mensajes `[go]` son los de la API.

**Primera ejecución** (como en MiConsultorio): si la base de datos de `DATABASE_URL` no existe, el servidor la crea
sola (el usuario de Postgres debe poder crear bases); aplica las migraciones con un candado, para que dos instancias
no choquen; crea el administrador de plataforma de `PLATFORM_ADMIN_*` si aún no hay ninguno; y, solo si no hay ningún
negocio, crea el negocio de ejemplo de `DEMO_*` (`demo@miestudio.local`) para probar sin registrarte. Ninguno de los
dos pisa datos existentes, y en producción rechazan las claves de ejemplo.

### Si algo no arranca

| Síntoma | Qué hacer |
| --- | --- |
| `Faltan variables en .env` | Tu `.env` viene de una versión anterior: compáralo con `.env.example` y copia lo que falte. |
| `PostgreSQL no responde` | Con Docker: `npm run db` y espera unos segundos. Sin Docker: enciende tu PostgreSQL y revisa `DATABASE_URL`. |
| `No se pudo compilar la API` | Necesitas Go 1.26+ (`go version`). |
| Entras con el administrador del `.env.example` y da «credenciales incorrectas» | El `.env` **solo crea al primer administrador**; si ya existía uno, el arranque lo avisa en `[go]` con su correo. Crea otro o restablece la contraseña con `npm run admin -- -email tu@correo.com -password '...'`. |

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

## Punto de venta

- **Caja:** una por sucursal. Se abre con el efectivo inicial; hay entradas y salidas de efectivo; al cerrar se compara lo contado con lo esperado (`apertura + efectivo cobrado − salidas`) y el esperado queda congelado.
- **Cobrar:** servicios, productos y citas, con descuento, propina, pagos mixtos (efectivo con cambio, tarjeta, transferencia y terminal) y folio consecutivo. Los pagos deben cubrir exactamente el total más la propina.
- **Comisiones:** se calculan al vender sobre lo realmente cobrado (después del descuento) y **se guardan en la venta**: cambiar las reglas después no altera el historial. Prioridad: persona + artículo, luego el artículo, luego la comisión general de la persona (solo servicios).
- **Inventario:** cada cambio de existencia deja un movimiento. Vender no puede dejar la existencia en negativo, ni siquiera con ventas simultáneas.
- **Cancelar una venta** (solo administrador): devuelve la existencia, reembolsa los cobros con terminal y, si el efectivo era de una caja ya cerrada, lo saca de la caja abierta. Un cobro repetido por doble clic no genera dos ventas.
- **Reportes:** ventas por periodo (por día, método, persona y artículo) y comisiones por persona.

### Terminal Mercado Pago (Point)

1. **Una vez, por quien administra la plataforma:** registra una aplicación en Mercado Pago, pon su *Redirect URL* (`<API_PUBLIC_URL>/api/point/oauth/callback`) y llena `MP_CLIENT_ID`, `MP_CLIENT_SECRET` y `API_PUBLIC_URL` (o `MP_OAUTH_REDIRECT`) en `.env`. Ver los comentarios de `.env.example`.
2. **Cada negocio**, en el panel → **Pagos**: pulsa *Conectar con Mercado Pago*, autoriza, y registra la terminal de cada sucursal (queda en modo punto de venta). Sin la aplicación de la plataforma, la pantalla ofrece pegar un token de acceso a mano.
3. En **Cobrar**, agrega un pago «Terminal MP», envíalo y espera la aprobación. Un pago aprobado solo sirve para una venta.

`TOKEN_ENC_KEY` y `OAUTH_STATE_SECRET` son claves propias del servidor (cifrado de tokens y firma del enlace de retorno). Son opcionales: si las omites se derivan de `JWT_SECRET`.

Los tokens de OAuth se renuevan solos antes de vencer. La integración está probada contra un servidor simulado de la API de Mercado Pago; antes de usarla con clientes reales, pruébala con una cuenta de pruebas y tu terminal.

## Reserva en línea y correos

Cada negocio activa su página pública desde los ajustes (`/api/booking/settings`): `/reservar/<slug>`. El cliente
elige servicio, persona y horario sin crear cuenta; recibe un enlace privado para ver o cancelar su cita.
Los correos (confirmación, cambio, cancelación y recordatorio) salen de una bandeja con reintentos
(5 y 20 minutos, 3 intentos); una falla del servidor de correo nunca estropea la reserva. Los recordatorios
solo se envían a quien los aceptó, y cada uno trae el enlace para darse de baja. Configura `SMTP_*`, `MAIL_FROM`
y `APP_URL` en `.env` (ver `.env.example`). WhatsApp no está implementado.

**Anticipos.** Con Mercado Pago conectado, el negocio puede pedir un porcentaje por adelantado (Checkout Pro).
La cita queda apartada 15 minutos como «pendiente de pago» y se confirma solo con un pago aprobado por el monto
exacto, verificado siempre con Mercado Pago (aviso automático, regreso del cliente a su enlace o vencimiento del
apartado). Un pago tardío o duplicado se devuelve; si el cliente cancela dentro del plazo, o el negocio cancela,
se devuelve el anticipo (y si Mercado Pago no puede, la cita no se cancela); una inasistencia lo conserva.

## Ajustes

El menú del panel solo trae lo del día a día (inicio, agenda, cobrar, caja, clientes, servicios, inventario, comisiones y
reportes). Lo que se configura de vez en cuando vive en **Ajustes** (`/panel/ajustes`): equipo, sucursales, reserva en línea,
privacidad, pagos (Mercado Pago), suscripción y mi cuenta; cada sección se ve según el rol. El panel de plataforma funciona igual
(`/plataforma/ajustes`: planes, registro de negocios, salud, actividad, equipo y mi cuenta). Las sucursales ahora están en
`/panel/ajustes/sucursales`.

## Cuenta y seguridad

- **Sesión:** cookie `HttpOnly` + `SameSite=Lax` (ningún script de la página puede leerla); las escrituras con cookie exigen
  el encabezado `X-Requested-With`. Cambiar la contraseña, el rol, el correo o dar de baja a alguien cierra sus sesiones al
  instante (versión de sesión en la base). «Cerrar sesión en todos los dispositivos» está en *Mi cuenta*.
- **Recuperar contraseña:** `/olvide` envía un enlace de 1 hora y un solo uso (solo se guarda su hash); la respuesta es la
  misma exista o no la cuenta. Los administradores de plataforma no lo usan: su clave se restablece con `npm run admin`.
- **Verificación en dos pasos (TOTP):** en *Mi cuenta* con QR y 8 códigos de respaldo; el administrador de un negocio puede
  quitarla a su equipo (*Equipo → Quitar 2 pasos*) y plataforma a cualquiera. El secreto se guarda cifrado.
- **Privacidad:** `/privacidad` y `/terminos` (con tus datos de `LEGAL_*`; revisa los textos con tu abogado: son una base, no
  asesoría legal) y aceptación registrada con su versión al crear la cuenta. Cada negocio tiene su aviso en `/aviso/<negocio>`,
  un formulario de derechos ARCO en `/arco/<negocio>` (20 días hábiles de plazo, seguimiento por enlace) y las atiende en
  *Privacidad*. En la ficha de cada cliente se puede descargar todo lo que se guarda de él o borrar sus datos personales
  (las ventas se conservan sin ellos). Solo hay una cookie técnica de sesión: no hace falta banner de cookies.
- **Proxy:** declara tus proxies en `TRUSTED_PROXIES` para que los límites usen la IP real.

## Planes, suscripciones y panel de plataforma

- **Planes** (`internal/api/plans.go`): Esencial, Profesional y Estudio, con topes de personas en agenda y sucursales y
  funciones (anticipos, terminal). Edita ahí precios y topes. Al registrarse, el negocio arranca 14 días de prueba en Profesional.
- **Estados de cobro:** en prueba, prueba vencida, activo, pago atrasado (3 días de gracia tras el vencimiento) y suspendido.
  Con la prueba vencida o el pago atrasado la cuenta entra pero solo para renovar (`/panel/suscripcion`); la página pública de
  reservas y los recordatorios se pausan. Los datos nunca se borran.
- **Pagos:** el administrador del negocio paga 1 mes o 1 año con Mercado Pago (cuenta de la plataforma, `MPP_ACCESS_TOKEN`);
  el pago se confirma siempre con Mercado Pago (aviso, regreso a la página o consulta), por el monto exacto y una sola vez.
  La plataforma también puede registrar pagos a mano (transferencia, efectivo, cortesía). Aún no hay cobro recurrente automático.
- **Panel de plataforma** (`/plataforma`): resumen con ingreso mensual recurrente y cobrado del mes, lista de lo que requiere
  atención (pendientes de aprobar, pagos atrasados, pruebas por terminar), negocios con filtros por estado y plan, detalle con
  plan, vigencia, pagos, suspensión con motivo y equipo, planes, pagos, salud del sistema, equipo de plataforma y bitácora.
  Soporte lo ve todo menos el dinero y no puede cambiar nada.
- **Avisos por correo:** a los negocios cuando su prueba o periodo está por terminar o terminó (una vez por periodo), recibo de
  cada pago, y a la plataforma cuando entra un pago.

## Avisos de nueva versión

Cada compilación del frontend lleva un nombre de versión (`APP_VERSION`, o la hora de la compilación). La app
consulta `/_app/version.json` cada minuto y al volver a la pestaña; si cambió, muestra un aviso con
«Actualizar ahora» / «Después». Nunca recarga sola, para no interrumpir una venta o una cita en curso. El servidor
sirve `index.html` y `version.json` sin caché y los archivos con huella como inmutables.

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
| `/api/cash/*`, `POST/GET /api/sales`, `/api/payments/point/*` | Administrador, caja y recepción |
| `POST /api/sales/{id}/void`, `/api/products/{id}/stock`, `/api/commission-rules`, `/api/reports/sales`, `/api/payments/mercadopago/*` | Administrador |
| `GET /api/reports/commissions` | Administrador (todos) y profesionales (solo lo suyo) |
| `POST/PATCH /api/appointments`, `POST /api/appointments/{id}/move`, `POST/DELETE /api/blocks` | Administrador, recepción y profesionales (estos últimos solo bloquean su propio horario) |
| `/api/platform/*` | Plataforma (soporte solo lee) |

Todo lo de negocio se limita al negocio de quien hace la petición, y se bloquea si la cuenta está pendiente o suspendida.
