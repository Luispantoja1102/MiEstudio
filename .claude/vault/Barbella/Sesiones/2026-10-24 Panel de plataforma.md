# Sesión 2026-10-24 · Panel de plataforma estilo MiTiendita

- **Qué se pidió:** que el panel del dueño de la plataforma (`/plataforma`) se parezca al de MiTiendita, sin cambiar el menú por grupos de Ajustes y sin tocar el backend.
- **Qué se hizo (sin commit):** `lib/Avatar.svelte` (nuevo), `ago()` movido de `NotificationBell.svelte` a `lib/format.js`, estilos `.kpis/.kpi` pasados de `panel/+page.svelte` a `app.css`, resumen «Hoy» (`plataforma/+page.svelte`), avatares en `negocios/+page.svelte` y ficha con pestañas en `negocios/[id]/+page.svelte` (Resumen, Licencia y pagos, Personas, Actividad; soporte no ve Licencia).
- **Cómo se probó:** `npm run build`; en pantalla con API simulada, escritorio y 375 px, Día y Noche, con pendientes, sin pendientes y con rol de soporte.
- **Pendiente / dudas:** de MiTiendita no se llevó Finanzas, Referidos ni Soporte por tickets: Barbella no tiene esos datos en el backend. Contradicción visible: `Barbella.md` dice que el panel es «azul tinta, Playfair»; el código ya usa los temas Día y Noche con Archivo.
