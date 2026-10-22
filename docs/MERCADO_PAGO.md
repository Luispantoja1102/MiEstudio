# Probar Mercado Pago de verdad

Hasta ahora la integración se probó contra un **servidor simulado** (las pruebas automáticas) y con el navegador contra ese mismo simulador. Nunca se ha
ejecutado contra Mercado Pago real. Esta guía te lleva de cero a un cobro real sin sorpresas: primero con **cuentas de prueba** (sin dinero) y, solo cuando todo
pase, con una compra real de $1 que devuelves.

Hay tres usos de Mercado Pago en MiEstudio. Cada uno usa credenciales distintas:

| Uso | Quién cobra | Credencial | Dónde se prueba |
|---|---|---|---|
| **Terminal Point** en caja | cada negocio, con su cuenta | token que el negocio autoriza por OAuth | *Ajustes → Cobro con tarjeta* y *Cobrar* |
| **Anticipo** al reservar en línea | cada negocio, con su cuenta | el mismo token del negocio | página pública `/reservar/<slug>` |
| **Suscripción** de MiEstudio | la plataforma (tú) | `MPP_ACCESS_TOKEN` | *Ajustes → Suscripción y plan* |

> **Lo que no se puede probar sin dinero real:** la terminal Point. Mercado Pago no ofrece una terminal de prueba; se prueba con el aparato físico y $1.

---

## 0. Antes de empezar

1. **Una dirección pública con https.** Mercado Pago tiene que poder llamar a tu servidor (avisos de pago) y regresar al cliente a tu página. En tu computadora no
   puede, así que usa un túnel:
   ```bash
   cloudflared tunnel --url http://localhost:8080      # o: ngrok http 8080
   ```
   Te da algo como `https://algo-aleatorio.trycloudflare.com`. **Cambia cada vez que reinicias el túnel** (en ese caso, repite el paso 1 de la sección 1 y vuelve a conectar).
   Para el regreso del cliente a su cita, abre el panel y la página pública a través de la **misma** dirección del túnel apuntando al frontend (`APP_URL`), o usa tu dominio real.
2. En `.env`:
   ```ini
   API_PUBLIC_URL=https://tu-tunel-o-dominio        # sin /api al final
   APP_URL=https://tu-frontend-o-dominio            # donde ve el cliente la página (los enlaces de los correos y el regreso del pago)
   MP_CLIENT_ID=...            # sección 1
   MP_CLIENT_SECRET=...
   MPP_ACCESS_TOKEN=...        # sección 4 (suscripciones)
   ```
3. Revisa todo con un comando (no cobra ni modifica nada):
   ```bash
   npm run check:mp
   ```
   Te dice, con ✓ / ! / ✗, si la dirección pública es alcanzable desde internet, si `MP_CLIENT_ID`/`MP_CLIENT_SECRET` son válidos, si `MPP_ACCESS_TOKEN` funciona y
   **si la cuenta es de prueba o real** (para que no cobres dinero real sin querer). Repítelo cada vez que cambies `.env`.

---

## 1. Crear la aplicación de la plataforma (una sola vez)

1. Entra a <https://www.mercadopago.com.mx/developers/panel/app> con tu cuenta de Mercado Pago → **Crear aplicación**.
   - Producto: **Checkout Pro** (para anticipos y suscripciones). Si ofrece "Point", márcalo también.
   - Modelo de integración: **Marketplace / OAuth** si te lo pregunta (los negocios conectan SU cuenta).
2. En la aplicación, **Redirect URL** = `API_PUBLIC_URL` + `/api/point/oauth/callback`. Debe coincidir **letra por letra** (si no, Mercado Pago responde «redirect_uri no válida»).
   `npm run check:mp` te imprime la URL exacta.
3. Copia **Client ID** y **Client Secret** a `MP_CLIENT_ID` y `MP_CLIENT_SECRET`.
4. (Opcional) En *Webhooks* puedes configurar notificaciones y una **clave secreta**; ponla en `MP_WEBHOOK_SECRET`. No es necesario para empezar: MiEstudio ya pide
   los avisos por pago (`notification_url`) y **consulta cada pago directamente a Mercado Pago** antes de creerle a un aviso, así que un aviso falso no sirve de nada.

## 2. Cuentas de prueba (sin dinero)

En <https://www.mercadopago.com.mx/developers/panel/app> → tu aplicación → **Cuentas de prueba** crea **dos**:

- **Vendedor de prueba** (será «el negocio»; con él se conectan los anticipos y la terminal de pruebas).
- **Comprador de prueba** (será «el cliente»; con él se paga).

Anota usuario y contraseña de ambos. **Úsalos en una ventana de incógnito distinta cada uno**: Mercado Pago recuerda la sesión y mezclar cuentas es el error más común.
El nombre de usuario de una cuenta de prueba empieza con `TEST` (así lo reconoce `npm run check:mp`).

**Tarjetas de prueba (México).** Mercado Pago las publica y cambian de vez en cuando: usa la tabla vigente en *Tus integraciones → Pruebas → Tarjetas de prueba*. Las de siempre:

| Tarjeta | Número | Código | Vencimiento |
|---|---|---|---|
| Mastercard | `5474 9254 3267 0366` | `123` | `11/30` |
| Visa | `4075 5957 1648 3764` | `123` | `11/30` |

El **resultado lo decide el nombre del titular**: `APRO` = aprobado, `OTHE` = rechazado por error general, `CONT` = queda pendiente de pago, `FUND` = fondos insuficientes.
El comprador de prueba paga siempre con estas tarjetas; **nunca uses tarjetas reales en pruebas**.

## 3. Anticipos de reserva (Checkout Pro) — con cuentas de prueba

**Preparar:**
1. Arranca todo (`npm run dev`) con `API_PUBLIC_URL` y `APP_URL` configurados. Entra a `/panel` con un negocio de plan **Crecimiento o Pro** (los anticipos no están en Básico).
2. *Ajustes → Cobro con tarjeta (Mercado Pago)* → **Conectar con Mercado Pago**. En la ventana que abre, inicia sesión con el **vendedor de prueba** y autoriza. Debe regresar al panel diciendo que está conectado.
3. *Ajustes → Reservas y página pública*: enciende la reserva, **Anticipo = 30 %**, y asegúrate de que haya un servicio en línea con precio y horario.

**Probar (cada fila es un caso real que hay que ver funcionar):**

| # | Qué haces | Qué debe pasar |
|---|---|---|
| 1 | Reserva en `/reservar/<slug>` y paga con el **comprador de prueba**, tarjeta con titular `APRO` | Regresas a tu cita; en segundos aparece **confirmada** (sin tocar nada) y llega el correo si hay SMTP. En *Agenda* está agendada. |
| 2 | Reserva y **no pagues**; cierra la pestaña de Mercado Pago | La cita queda «pendiente de pago» **15 minutos** y luego el horario se libera solo. |
| 3 | Reserva y paga con titular `OTHE` | Mercado Pago rechaza; la cita **no** se confirma; puedes reintentar dentro de los 15 min. |
| 4 | Con una cita pagada, **cancélala** desde el enlace del cliente (dentro del plazo) | Se devuelve el anticipo; en tu cuenta de prueba el pago aparece como *devuelto*. |
| 5 | Con otra cita pagada, cancélala desde el **panel** | Se devuelve íntegro. Si Mercado Pago no pudiera devolver, la cita **no** se cancela. |
| 6 | Marca otra cita pagada como **No asistió** | El anticipo se conserva. |
| 7 | Cobra una cita con anticipo en *Cobrar* | El anticipo aparece como forma de pago «Anticipo en línea» y solo se paga el resto. |
| 8 | Reserva, deja pasar los 15 min **y paga después** (guarda la pestaña de pago abierta) | El pago tardío se **devuelve** solo; el horario no se recupera. |

**Mirar por dentro** (si algo no cuadra):
```bash
psql "$DATABASE_URL" -c "select id, appointment_id, amount_cents, status, mp_payment_id from deposits order by id desc limit 5;"
```
Estados: `pending` → `paid` → `applied` (usado en una venta) | `refunded` | `kept` (no-show).

## 4. Suscripciones de MiEstudio (la plataforma cobra a los negocios)

1. Crea (o usa) la cuenta de Mercado Pago **donde quieres recibir el dinero de las suscripciones**. Para ensayar usa un *vendedor de prueba* distinto al de la sección 2.
2. En su aplicación obtén el **Access Token** (para pruebas: el de la sección *Credenciales de prueba*, empieza con `TEST-`; para producción, el de *Credenciales de producción*, `APP_USR-`).
   Ponlo en `MPP_ACCESS_TOKEN`. (Opcional) `MPP_WEBHOOK_SECRET` y `PLATFORM_NOTICE_EMAIL` (a quién avisan los pagos recibidos).
3. `npm run check:mp` debe mostrar «cuenta … es una cuenta de PRUEBA».
4. Entra como un negocio → *Ajustes → Suscripción y plan* → **Pagar 1 mes** y paga con el comprador de prueba (`APRO`).

| # | Qué debe pasar |
|---|---|
| 1 | Al volver, la página muestra el plan **activo** y la vigencia se extiende exactamente 1 mes (o 12). |
| 2 | En *Plataforma → Pagos* aparece el pago con método «Mercado Pago». Si pones `PLATFORM_NOTICE_EMAIL`, llega el aviso. |
| 3 | Pagar dos veces lo mismo: el segundo pago se **devuelve** solo. |
| 4 | Un pago rechazado (`OTHE`) no cambia el plan ni la vigencia. |
| 5 | Si cierras la ventana antes de volver, el aviso automático igual activa el plan (probar con el túnel funcionando). |

> **Límite actual:** es un pago **por periodo**, no un cargo automático recurrente. Al vencer, el negocio ve el aviso y renueva a mano. Los pagos recurrentes
> (suscripciones de Mercado Pago) están pendientes.

## 5. Terminal Point (solo con el aparato físico)

Necesitas una terminal Point **vinculada a la cuenta de Mercado Pago del negocio** (la real, no una de prueba) y una tarjeta propia.

1. *Ajustes → Cobro con tarjeta*: conecta la cuenta (OAuth) y, en la lista de terminales, **Registrar** la de la sucursal (queda en modo punto de venta).
2. Abre caja en *Cobrar*, agrega un servicio de **$1** (precio mínimo del cobro con terminal) y como forma de pago **Terminal MP** → *Enviar a la terminal*.
3. En la terminal aparece el cobro; pasa tu tarjeta. En pantalla pasa a «Pago aprobado» y puedes cobrar la venta.
4. **Devuélvelo:** *Historial* → la venta → **Cancelar venta**. Se reembolsa en Mercado Pago (si el reembolso falla, la venta **no** se cancela). Comprueba en tu app de Mercado Pago que quedó devuelto.
5. Casos: cancelar el cobro antes de pasar la tarjeta (botón *Cancelar cobro*); pagar con una tarjeta rechazada (debe permitir «Intentar de nuevo»); usar el mismo cobro aprobado en dos ventas (la segunda debe rechazarse).

## 6. Pasar a producción

1. **Credenciales de producción** de la aplicación (`MP_CLIENT_ID`/`MP_CLIENT_SECRET` son los mismos; cambia `MPP_ACCESS_TOKEN` a `APP_USR-…` de tu cuenta real). La aplicación debe estar **activada para producción** en el panel de Mercado Pago.
2. `APP_ENV=production`, dominio con **https** real (no túnel), `API_PUBLIC_URL`, `APP_URL`, `TRUSTED_PROXIES` si usas proxy. Redirect URL de la aplicación actualizada al dominio real.
3. Define `TOKEN_ENC_KEY` (`openssl rand -hex 32`) y **guárdala**: si la pierdes o cambias, todos los negocios tienen que volver a conectar su cuenta.
4. `npm run check:mp` sin ✗, y debe decir «cuenta REAL».
5. Cada negocio **vuelve a conectar** su cuenta real (las conexiones de prueba no sirven).
6. Haz **un anticipo real chico** (por ejemplo, un servicio de $10 al 30 % = $3), cancela la cita y verifica la devolución. Después, una suscripción real de 1 mes con una tarjeta tuya.
7. Vigila los registros del servidor las primeras semanas (busca `mercadopago`, `suscripción`) y la tabla `deposits`/`subscription_checkouts` por estados que se queden en `pending`.

## 7. Si algo falla

| Síntoma | Causa probable |
|---|---|
| Al conectar: «redirect_uri no válida» | La Redirect URL de la aplicación no coincide letra por letra con `API_PUBLIC_URL/api/point/oauth/callback` (¿cambió el túnel?). |
| Al conectar regresa y dice que no está conectado | `MP_CLIENT_SECRET` incorrecto, o `TOKEN_ENC_KEY`/`JWT_SECRET` cambiaron. Mira el mensaje en pantalla y los registros de la API. |
| El cliente paga pero la cita queda «pendiente» | Mercado Pago no llegó a tu servidor (`API_PUBLIC_URL` incorrecta, túnel caído) y nadie abrió la cita. Abrir el enlace de la cita la confirma; revisa `npm run check:mp`. |
| «auto_return … solo con https» | `APP_URL` no es https. El pago funciona, pero el cliente no regresa solo. |
| Pago aprobado pero «monto distinto» y se devuelve | El monto pagado no coincide con el anticipo (alguien editó el precio entre medias). Es la protección funcionando. |
| Pruebas con la cuenta real y no de prueba | `npm run check:mp` lo avisa. Cambia `MPP_ACCESS_TOKEN` y reconecta el negocio con el vendedor de prueba. |
| «Cuenta pendiente de verificación» / límites | Mercado Pago pide verificar identidad de la cuenta real antes de recibir montos altos o retirar. |

**Lo que sigue sin verificarse contra Mercado Pago real** (porque nadie lo ha ejecutado todavía): el formato exacto de los avisos con firma (`MP_WEBHOOK_SECRET`),
los códigos de error de las devoluciones, y todo el flujo de la terminal. Si algo de eso difiere, el síntoma será un mensaje de error claro en pantalla o en el registro: repórtalo con el texto y se ajusta.
