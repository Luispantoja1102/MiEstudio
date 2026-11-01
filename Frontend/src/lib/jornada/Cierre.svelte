<script>
  import { legalLink } from '$lib/legal/legal.svelte.js';
  // 21:00. Planes, preguntas y el cierre de la jornada (en el ámbito claro de la landing).
  import { onMount } from 'svelte';
  import Logo from '$lib/Logo.svelte';

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

<section id="planes" class="planes" aria-labelledby="t-planes">
  <div class="envoltura">
    <header class="cabeza">
      <div>
        <p class="kicker"><time>21:00</time>Antes de cerrar</p>
        <h2 id="t-planes" class="titulo">El precio de una jornada así.</h2>
        <p class="lead">14 días gratis con tus propios clientes, sin tarjeta. Precios en pesos; cambias de plan cuando quieras.</p>
      </div>
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
          <a class={p.id === 'crecimiento' ? 'btn' : 'btn btn-linea'} href={`/registro?plan=${p.id}`}>Probar 14 días gratis</a>
        </article>
      {/each}
    </div>
  </div>
</section>

<section id="preguntas" class="preguntas" aria-labelledby="t-preguntas">
  <div class="envoltura dos">
    <h2 id="t-preguntas" class="titulo">Lo que preguntan en el mostrador.</h2>
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
<section class="baja" aria-labelledby="t-baja">
  <div class="envoltura final">
    <div>
      <p class="kicker"><time>21:30</time>Fin del día</p>
      <h2 id="t-baja" class="titulo">Mañana abres con la agenda llena.</h2>
    </div>
    <div class="lado">
      <p>Crea tu cuenta, pon tu horario y tus servicios, y comparte tu enlace hoy mismo.</p>
      <div class="acciones">
        <a class="btn" href="/registro">Crear mi cuenta gratis</a>
        <a class="link" href="/login">Ya tengo cuenta</a>
      </div>
    </div>
  </div>
</section>

<footer class="pie">
  <div class="envoltura pie-g">
    <div>
      <Logo />
      <p>Agenda, caja y reservas en línea para barberías, salones de uñas, pestañas, estéticas y spas. Hecho en México.</p>
    </div>
    <nav aria-label="Producto"><h3>Producto</h3><a href="#madrugada">La jornada</a><a href="#oficios">Oficios</a><a href="#planes">Planes</a><a href="#preguntas">Preguntas</a></nav>
    <nav aria-label="Cuenta"><h3>Cuenta</h3><a href="/registro">Crear cuenta</a><a href="/login">Iniciar sesión</a></nav>
    <nav aria-label="Legal"><h3>Legal</h3><a href="/privacidad" onclick={(e) => legalLink(e, 'privacidad')}>Aviso de privacidad</a><a href="/terminos" onclick={(e) => legalLink(e, 'terminos')}>Términos</a></nav>
  </div>
  <div class="envoltura"><p class="copy">© {new Date().getFullYear()} Barbella · Un producto de <a href="https://www.hyperionmkt.com/">Hyperion Software</a></p></div>
</footer>

<style>
  /* Planes: una sola superficie con separadores, no cuatro tarjetas */
  .planes { padding: clamp(72px, 9vw, 112px) 0; background: var(--papel-2); }
  .cabeza { display: grid; grid-template-columns: 1fr auto; align-items: end; gap: 20px 40px; margin-bottom: clamp(28px, 4vw, 48px); }
  .titulo { max-width: 14ch; }
  .lead { margin: 18px 0 0; max-width: 52ch; color: var(--tinta-suave); line-height: 1.5; }
  .periodo { display: inline-flex; padding: 3px; border-radius: 8px; background: var(--carta); box-shadow: inset 0 0 0 1px var(--border); }
  .periodo button { min-height: 44px; padding: 0 18px; border: 0; border-radius: 6px; background: transparent; font: inherit; font-weight: 600; color: var(--tinta-suave); cursor: pointer; }
  .periodo button[aria-pressed='true'] { background: var(--tinta); color: #fff; }
  .periodo button:focus-visible { outline: 3px solid var(--bermellon); outline-offset: 2px; }

  .lista { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border: 1px solid var(--border); border-radius: 8px; background: var(--carta); overflow: hidden; }
  .plan { position: relative; display: flex; flex-direction: column; padding: 30px 26px 28px; }
  .plan + .plan { border-left: 1px solid var(--border); }
  .plan.fuerte { box-shadow: inset 0 3px 0 var(--bermellon); }
  h3 { display: flex; flex-direction: column; margin: 0; font-family: var(--display); font-stretch: var(--ancho); font-size: 1.6rem; font-weight: 700; min-height: 2.4em; justify-content: flex-end; }
  h3 span { order: -1; margin-bottom: 4px; font-family: var(--texto); font-stretch: 100%; font-size: 0.78rem; font-weight: 700; letter-spacing: 0.04em; text-transform: uppercase; color: var(--bermellon); }
  .desc { margin: 8px 0 22px; min-height: 4.2em; color: var(--tinta-suave); line-height: 1.4; font-size: 0.95rem; }
  .precio { margin: 0; font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: 3.2rem; line-height: 1; font-variant-numeric: tabular-nums; }
  .precio small { font-family: var(--texto); font-stretch: 100%; font-size: 1rem; font-weight: 500; margin-left: 4px; color: var(--tinta-suave); }
  .equiv { margin: 8px 0 22px; padding-bottom: 22px; border-bottom: 1px solid var(--border); font-size: 0.9rem; color: var(--tinta-suave); }
  ul { margin: 0 0 28px; padding: 0; list-style: none; display: grid; gap: 10px; flex: 1; align-content: start; font-size: 0.95rem; }
  li { position: relative; padding-left: 20px; line-height: 1.35; }
  li::before { content: ''; position: absolute; left: 0; top: 0.62em; width: 9px; height: 2px; background: var(--tinta); }
  li.no { color: rgb(var(--ink-rgb) / 0.5); text-decoration: line-through; text-decoration-thickness: 1px; }
  li.no::before { opacity: 0.35; }
  .plan .btn { align-self: stretch; }

  /* Preguntas: título a un lado, respuestas sobre blanco */
  .preguntas { padding: clamp(72px, 9vw, 112px) 0; background: var(--carta); }
  .dos { display: grid; grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.2fr); gap: 32px clamp(32px, 6vw, 88px); align-items: start; }
  .dos .titulo { position: sticky; top: 96px; max-width: 11ch; font-size: clamp(2rem, 3.4vw, 3rem); }
  details { border-bottom: 1px solid var(--border); }
  details:first-child { border-top: 2px solid var(--tinta); }
  summary { display: flex; justify-content: space-between; gap: 16px; min-height: 64px; align-items: center; font-weight: 600; font-size: 1.08rem; cursor: pointer; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  summary::after { content: '+'; flex: none; font-family: var(--rotulo); font-size: 1.6rem; font-weight: 500; transition: transform 200ms var(--ease); }
  details[open] summary::after { transform: rotate(45deg); }
  summary:focus-visible { outline: 3px solid var(--bermellon); outline-offset: 4px; }
  details p { margin: 0 0 22px; max-width: 60ch; line-height: 1.6; color: var(--tinta-suave); }

  /* Cierre */
  .baja { padding: clamp(72px, 10vw, 128px) 0; background: var(--ticket); }
  .final { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 0.8fr); align-items: end; gap: 28px clamp(32px, 6vw, 96px); }
  .final .titulo { max-width: 13ch; font-size: clamp(2.4rem, 5vw, 4.4rem); }
  .lado p { margin: 0; max-width: 40ch; color: var(--tinta-suave); line-height: 1.55; }
  .acciones { display: flex; flex-wrap: wrap; align-items: center; gap: 14px 24px; margin-top: 22px; }

  /* Pie */
  .pie { padding: 40px 0 24px; background: var(--papel-2); color: var(--tinta-suave); border-top: 1px solid var(--border); font-size: 0.92rem; }
  .pie-g { display: grid; grid-template-columns: 1.6fr repeat(3, 1fr); gap: 28px; }
  .pie-g p { max-width: 38ch; margin: 12px 0 0; line-height: 1.55; }
  .pie .copy { max-width: none; }
  .pie h3 { display: block; min-height: 0; margin: 0 0 10px; font-family: var(--texto); font-stretch: 100%; font-size: 0.78rem; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--tinta); }
  .pie nav { display: grid; align-content: start; gap: 4px; }
  .pie nav a { display: inline-flex; align-items: center; min-height: 32px; color: inherit; text-decoration: none; }
  .pie nav a:hover { color: var(--tinta); text-decoration: underline; }
  .copy a { color: inherit; text-decoration: underline; }
  .copy { margin-top: 28px; padding-top: 16px; border-top: 1px solid var(--border); font-size: 0.82rem; }

  /* Tableta: dos planes por fila */
  @media (min-width: 901px) and (max-width: 1180px) {
    .lista { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .plan:nth-child(odd) { border-left: 0; }
    .plan:nth-child(n + 3) { border-top: 1px solid var(--border); }
  }
  @media (max-width: 900px) {
    .cabeza { grid-template-columns: 1fr; }
    .periodo { justify-self: start; }
    .lista { grid-template-columns: 1fr; }
    .plan + .plan { border-left: 0; border-top: 1px solid var(--border); }
    h3, .desc { min-height: 0; }
    .dos, .final, .pie-g { grid-template-columns: 1fr; }
    .dos .titulo { position: static; }
  }
  @media (max-width: 520px) { .periodo button { padding: 0 12px; font-size: 0.9rem; } }
</style>
