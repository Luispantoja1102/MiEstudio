<script>
  // 21:00. Planes, preguntas y el cierre de la jornada.
  import { onMount } from 'svelte';
  import Logo from '$lib/Logo.svelte';
  import { legalLink } from '$lib/legal/legal.svelte.js';

  // Respaldo por si no carga el catálogo; la fuente es GET /api/plans
  let plans = $state([
    { id: 'individual', name: 'Individual', description: 'Para quien trabaja solo: agenda, citas y anticipo por transferencia', price_month: 150, price_year: 1500, max_staff: 1, max_branches: 1, pos: false, deposits: true, deposits_mp: false, terminal: false, support: 'correo' },
    { id: 'basico', name: 'Básico', description: 'Agenda y citas, con anticipo por transferencia', price_month: 350, price_year: 3500, max_staff: 2, max_branches: 1, pos: false, deposits: true, deposits_mp: false, terminal: false, support: 'correo' },
    { id: 'crecimiento', name: 'Crecimiento', description: 'Todo incluido: citas, caja, ventas, anticipos y más', price_month: 700, price_year: 7000, max_staff: 6, max_branches: 1, pos: true, deposits: true, deposits_mp: true, terminal: true, support: 'prioritario' },
    { id: 'pro', name: 'Pro', description: 'Todo, con varias sucursales y equipos grandes', price_month: 1200, price_year: 12000, max_staff: null, max_branches: 10, pos: true, deposits: true, deposits_mp: true, terminal: true, support: 'dedicado' }
  ]);
  let yearly = $state(false);
  onMount(async () => {
    try {
      const r = await fetch('/api/plans');
      if (r.ok) plans = await r.json();
    } catch {
      /* se queda el respaldo */
    }
  });
  const money = (n) => n.toLocaleString('es-MX', { style: 'currency', currency: 'MXN', maximumFractionDigits: 0 });
  const rows = (p) => [
    [true, 'Agenda y reserva en línea'],
    [true, p.max_staff === null ? 'Equipo sin límite' : p.max_staff === 1 ? '1 persona' : `Hasta ${p.max_staff} personas`],
    [true, p.max_branches === 1 ? '1 sucursal' : `Hasta ${p.max_branches} sucursales`],
    [p.pos, 'Caja, ventas, paquetes e inventario'],
    [p.pos, 'Comisiones y reportes'],
    [p.deposits, 'Anticipo por transferencia, con comprobante'],
    [p.deposits_mp, 'Anticipo con Mercado Pago'],
    [p.terminal, 'Cobro con terminal Mercado Pago'],
    [true, `Soporte ${p.support}`]
  ];

  const faqs = [
    ['¿Necesito tarjeta para probarlo?', 'No. Tienes 14 días para usar todo con tus propios clientes y decides después si contratas un plan.'],
    ['¿Mis clientes tienen que instalar algo?', 'No. Reservan desde un enlace, en el navegador de su celular, sin crear cuenta.'],
    ['¿Y si el cliente no llega?', 'El anticipo aparta el horario. Si cancela a tiempo se le devuelve; si no llega, tú decides conservarlo. Además hay recordatorios por correo y WhatsApp.'],
    ['¿Sirve para varias sucursales?', 'Sí, con el plan Pro: hasta 10 sucursales con su propia agenda y caja.'],
    ['¿Puedo cambiar de plan o cancelar?', 'Sí, cuando quieras. Tus datos siguen a salvo aunque tu suscripción venza.'],
    ['¿Qué pasa con los datos de mis clientes?', 'Son tuyos. Cada negocio ve solo lo suyo, y el sistema incluye aviso de privacidad y solicitudes ARCO para cumplir con la ley mexicana.']
  ];
</script>

<section id="planes" class="planes" data-hora="21:00" data-momento="Antes de cerrar" aria-labelledby="t-planes">
  <div class="envoltura">
    <header class="cabeza">
      <h2 id="t-planes">El precio de una jornada así.</h2>
      <p>14 días gratis con tus propios clientes, sin tarjeta. Precios en pesos; cambias de plan cuando quieras.</p>
      <div class="periodo" role="group" aria-label="Periodo de pago">
        <button type="button" aria-pressed={!yearly} onclick={() => (yearly = false)}>Mensual</button>
        <button type="button" aria-pressed={yearly} onclick={() => (yearly = true)}>Anual · 2 meses gratis</button>
      </div>
    </header>

    <div class="lista">
      {#each plans as p (p.id)}
        <article class="plan" class:fuerte={p.id === 'crecimiento'}>
          <h3>{p.name}{#if p.id === 'crecimiento'}<span>El más elegido</span>{/if}</h3>
          <p class="desc">{p.description}</p>
          <p class="precio">{money(yearly ? p.price_year : p.price_month)}<small>/{yearly ? 'año' : 'mes'}</small></p>
          <p class="equiv">{yearly ? `Equivale a ${money(Math.round(p.price_year / 12))} al mes` : `o ${money(p.price_year)} al año`}</p>
          <ul>
            {#each rows(p) as [on, text]}<li class:no={!on}>{text}</li>{/each}
          </ul>
          <a class={p.id === 'crecimiento' ? 'btn' : 'btn btn-linea'} href="/registro">Probar 14 días gratis</a>
        </article>
      {/each}
    </div>
  </div>
</section>

<section id="preguntas" class="preguntas" aria-labelledby="t-preguntas">
  <div class="envoltura dos">
    <h2 id="t-preguntas">Lo que preguntan en el mostrador.</h2>
    <div>
      {#each faqs as [q, a]}
        <details>
          <summary>{q}</summary>
          <p>{a}</p>
        </details>
      {/each}
    </div>
  </div>
</section>

<!-- Fin de la jornada -->
<section class="baja" data-hora="21:30" data-momento="Fin del día" aria-labelledby="t-baja">
  <div class="final">
    <h2 id="t-baja">Mañana abres con la agenda llena.</h2>
    <p>Crea tu cuenta, pon tu horario y tus servicios, y comparte tu enlace hoy mismo.</p>
    <div class="acciones">
      <a class="btn" href="/registro">Crear mi cuenta gratis</a>
      <a class="link" href="/login">Ya tengo cuenta</a>
    </div>
  </div>
</section>

<footer class="pie">
  <div class="envoltura pie-g">
    <div>
      <Logo onDark />
      <p>Agenda, caja y reservas en línea para barberías, salones de uñas, pestañas, estéticas y spas. Hecho en México.</p>
    </div>
    <nav aria-label="Producto"><h3>Producto</h3><a href="#madrugada">La jornada</a><a href="#oficios">Oficios</a><a href="#planes">Planes</a><a href="#preguntas">Preguntas</a></nav>
    <nav aria-label="Cuenta"><h3>Cuenta</h3><a href="/registro">Crear cuenta</a><a href="/login">Iniciar sesión</a></nav>
    <nav aria-label="Legal"><h3>Legal</h3><a href="/privacidad" onclick={(e) => legalLink(e, 'privacidad')}>Aviso de privacidad</a><a href="/terminos" onclick={(e) => legalLink(e, 'terminos')}>Términos</a></nav>
  </div>
  <p class="envoltura copy">© {new Date().getFullYear()} Barbella</p>
</footer>

<style>
  .envoltura { max-width: 1200px; margin: 0 auto; padding: 0 clamp(16px, 4vw, 40px); }
  h2 { margin: 0; font-family: var(--display); font-stretch: var(--ancho); font-weight: 650; font-size: clamp(2.2rem, 4.6vw, 4rem); line-height: 1; letter-spacing: -0.01em; text-wrap: balance; }

  /* Planes */
  .planes { padding: 120px 0 96px; background: var(--ticket); color: var(--tinta); }
  .cabeza { display: grid; grid-template-columns: 1fr auto; align-items: end; gap: 16px 40px; margin-bottom: 48px; }
  .cabeza h2 { max-width: 12ch; }
  .cabeza p { grid-column: 1; margin: 0; max-width: 52ch; color: var(--tinta-suave); line-height: 1.5; }
  .periodo { grid-column: 2; grid-row: 1 / span 2; align-self: end; display: inline-flex; padding: 4px; border-radius: 999px; box-shadow: inset 0 0 0 1px rgb(var(--ink-rgb) / 0.2); }
  .periodo button { min-height: 44px; padding: 0 18px; border: 0; border-radius: 999px; background: transparent; font: inherit; font-weight: 600; color: var(--tinta-suave); cursor: pointer; }
  .periodo button[aria-pressed='true'] { background: var(--tinta); color: var(--ticket); }
  .periodo button:focus-visible { outline: 3px solid var(--bermellon); outline-offset: 2px; }
  .lista { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border-top: 2px solid var(--tinta); }
  /* cada plan comparte las filas de la lista: título, texto, precio, equivalencia, lista y botón quedan a la misma altura */
  .plan { position: relative; display: grid; grid-row: span 6; grid-template-rows: subgrid; padding: 40px 28px 28px; }
  .plan + .plan { border-left: 1px solid rgb(var(--ink-rgb) / 0.18); }
  .plan.fuerte { background: var(--carta); box-shadow: inset 0 0 0 2px var(--tinta); border-radius: 0 0 14px 14px; }
  h3 { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 0; font-family: var(--display); font-stretch: var(--ancho); font-size: 1.9rem; font-weight: 650; }
  h3 { flex-wrap: wrap; }
  h3 span { position: absolute; top: 16px; left: 28px; font-family: var(--texto); font-size: 0.82rem; font-weight: 600; color: var(--bermellon); }
  .desc { margin: 8px 0 22px; color: var(--tinta-suave); line-height: 1.4; }
  .precio { margin: 0; font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: 3.4rem; line-height: 1; font-variant-numeric: tabular-nums; }
  .precio small { font-family: var(--texto); font-stretch: 100%; font-size: 1rem; font-weight: 500; margin-left: 4px; }
  .equiv { margin: 8px 0 22px; font-size: 0.9rem; color: var(--tinta-suave); }
  ul { margin: 0 0 28px; padding: 0; list-style: none; display: grid; align-content: start; gap: 10px; }
  li { position: relative; padding-left: 20px; line-height: 1.35; }
  li::before { content: ''; position: absolute; left: 0; top: 0.55em; width: 9px; height: 2px; background: currentColor; }
  li.no { color: rgb(var(--ink-rgb) / 0.4); text-decoration: line-through; text-decoration-thickness: 1px; }
  li.no::before { opacity: 0.4; }
  .plan .btn { align-self: stretch; }

  /* Preguntas */
  .preguntas { padding: 96px 0 120px; background: var(--ticket); color: var(--tinta); border-top: 1px solid rgb(var(--ink-rgb) / 0.12); }
  .dos { display: grid; grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.2fr); gap: 40px 64px; }
  .dos h2 { max-width: 11ch; font-size: clamp(2rem, 3.6vw, 3.2rem); }
  details { border-bottom: 1px solid rgb(var(--ink-rgb) / 0.16); }
  summary { display: flex; justify-content: space-between; gap: 16px; min-height: 64px; align-items: center; font-weight: 600; font-size: 1.1rem; cursor: pointer; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  summary::after { content: '+'; font-family: var(--rotulo); font-size: 1.6rem; font-weight: 500; transition: transform 300ms var(--ease); }
  details[open] summary::after { transform: rotate(45deg); }
  summary:focus-visible { outline: 3px solid var(--bermellon); outline-offset: 4px; }
  details p { margin: 0 0 22px; max-width: 60ch; line-height: 1.6; color: var(--tinta-suave); }

  /* Cierre */
  .baja { padding: clamp(96px, 14vw, 160px) 20px; background: var(--ticket); color: var(--tinta); border-top: 1px solid rgb(var(--ink-rgb) / 0.12); }
  .final { display: grid; justify-items: center; gap: 18px; text-align: center; }
  .final h2 { max-width: 13ch; font-size: clamp(2.4rem, 5.6vw, 4.8rem); }
  .final p { margin: 0; max-width: 46ch; color: var(--tinta-suave); line-height: 1.5; }
  .acciones { display: flex; flex-wrap: wrap; justify-content: center; align-items: center; gap: 14px 24px; margin-top: 10px; }

  /* Pie */
  .pie { padding: 64px 0 28px; background: #08080a; color: rgb(var(--ink-rgb) / 0.72); border-top: 1px solid rgb(var(--ink-rgb) / 0.1); }
  .pie-g { display: grid; grid-template-columns: 1.6fr repeat(3, 1fr); gap: 32px; }
  .pie-g p { max-width: 36ch; line-height: 1.55; }
  .pie h3 { margin: 0 0 12px; font-size: 0.9rem; color: var(--ticket); }
  .pie nav { display: grid; align-content: start; gap: 10px; }
  .pie a { color: inherit; text-decoration: none; }
  .pie a:hover { color: var(--ticket); }
  .copy { margin-top: 40px; font-size: 0.85rem; opacity: 0.6; }

  /* Tableta: dos planes por fila */
  @media (min-width: 901px) and (max-width: 1180px) {
    .lista { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .plan:nth-child(odd) { border-left: 0; }
    .plan:nth-child(n + 3) { border-top: 1px solid rgb(var(--ink-rgb) / 0.18); }
    .plan { padding-bottom: 28px; }
  }
  @media (max-width: 900px) {
    .cabeza { grid-template-columns: 1fr; }
    .periodo { grid-column: 1; grid-row: auto; justify-self: start; }
    .lista { grid-template-columns: 1fr; }
    .plan + .plan { border-left: 0; border-top: 1px solid rgb(var(--ink-rgb) / 0.18); }
    .plan { padding-bottom: 28px; }
    .dos, .pie-g { grid-template-columns: 1fr; }
  }
</style>
