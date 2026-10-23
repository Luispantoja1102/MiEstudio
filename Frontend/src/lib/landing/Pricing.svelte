<script>
  import { onMount } from 'svelte';
  import Icon from '$lib/Icon.svelte';
  import SectionHead from './SectionHead.svelte';

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
    } catch {}
  });

  const money = (n) => n.toLocaleString('es-MX', { style: 'currency', currency: 'MXN', maximumFractionDigits: 0 });
  const rows = (p) => [
    [true, 'Agenda día, semana y mes, y reserva en línea'],
    [true, p.max_staff === null ? 'Equipo sin límite en la agenda' : p.max_staff === 1 ? '1 persona en la agenda' : `Hasta ${p.max_staff} personas en la agenda`],
    [true, p.max_branches === 1 ? '1 sucursal' : `Hasta ${p.max_branches} sucursales`],
    [p.pos, 'Caja, ventas, paquetes e inventario'],
    [p.pos, 'Comisiones y reportes'],
    [p.deposits, 'Anticipo por transferencia, con comprobante'],
    [p.deposits_mp, 'Anticipo con Mercado Pago'],
    [p.terminal, 'Cobro con terminal de Mercado Pago'],
    [true, `Soporte ${p.support}`]
  ];
</script>

<section class="section alt" id="planes">
  <div class="container">
    <SectionHead tag="Planes" title="Elige el plan de" accent="tu negocio" sub="14 días gratis para probar todo con tus propios clientes. Sin tarjeta." />
    <div class="toggle" role="group" aria-label="Periodo de pago">
      <button aria-pressed={!yearly} onclick={() => (yearly = false)}>Mensual</button>
      <button aria-pressed={yearly} onclick={() => (yearly = true)}>Anual <small>2 meses gratis</small></button>
    </div>
    <div class="plans">
      {#each plans as p (p.id)}
        <article class="card plan" class:hot={p.id === 'crecimiento'}>
          {#if p.id === 'crecimiento'}<span class="badge">El más elegido</span>{/if}
          <h3>{p.name}</h3>
          <p class="desc">{p.description}</p>
          <p class="price">{money(yearly ? p.price_year : p.price_month)}<small>/{yearly ? 'año' : 'mes'}</small></p>
          <p class="alt-price">{yearly ? `Equivale a ${money(Math.round(p.price_year / 12))} al mes` : `o ${money(p.price_year)} al año`}</p>
          <a class="btn block" class:ghost={p.id !== 'crecimiento'} href="/registro">Probar 14 días gratis</a>
          <ul>
            {#each rows(p) as [on, text]}
              <li class:off={!on}><span class="mk"><Icon name={on ? 'check' : 'arrow'} size={12} /></span>{text}</li>
            {/each}
          </ul>
        </article>
      {/each}
    </div>
    <p class="fine">Precios en pesos mexicanos. Puedes cambiar de plan cuando quieras.</p>
  </div>
</section>

<style>
  .section { padding-block: 88px; scroll-margin-top: 68px; }
  .alt { background: var(--surface-2); }
  .toggle { display: flex; justify-content: center; width: fit-content; margin: 0 auto 2rem; padding: 4px; gap: 2px; background: var(--surface); border: 1px solid var(--border); border-radius: 999px; }
  .toggle button { min-height: 40px; padding: 0 20px; font: inherit; font-size: 0.9rem; font-weight: 600; color: var(--muted); background: transparent; border: 0; border-radius: 999px; cursor: pointer; transition: background-color 0.18s var(--ease), color 0.18s var(--ease); }
  .toggle button small { margin-left: 6px; font-size: 0.72rem; font-weight: 600; opacity: 0.85; }
  .toggle button[aria-pressed='true'] { color: var(--accent-contrast); background: var(--accent); }
  .plans { display: grid; gap: 16px; align-items: stretch; }
  .plan { position: relative; display: flex; flex-direction: column; padding: 28px; }
  .plan.hot { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent), var(--card-shadow); }
  .badge { position: absolute; top: -13px; left: 28px; padding: 4px 12px; font-size: 0.72rem; font-weight: 700; color: var(--accent-contrast); background: var(--accent); border-radius: 999px; }
  h3 { margin: 0; font-size: 1.6rem; }
  .desc { min-height: 3.2em; margin: 0.4rem 0 1.2rem; font-size: 0.93rem; color: var(--muted); }
  .price { margin: 0; font-family: var(--font-body, inherit); font-size: 2.6rem; font-weight: 700; font-variant-numeric: tabular-nums lining-nums; line-height: 1; }
  .price small { margin-left: 4px; font-size: 0.95rem; font-weight: 500; color: var(--muted); }
  .alt-price { margin: 0.4rem 0 1.4rem; font-size: 0.85rem; color: var(--muted); }
  ul { list-style: none; margin: 1.5rem 0 0; padding: 1.4rem 0 0; display: grid; gap: 11px; border-top: 1px solid var(--border); }
  li { display: flex; align-items: flex-start; gap: 10px; font-size: 0.92rem; }
  .mk { flex: none; display: grid; place-items: center; width: 20px; height: 20px; margin-top: 1px; border-radius: 50%; color: var(--accent); background: var(--accent-soft); }
  li.off { color: var(--muted); }
  li.off .mk { color: var(--muted); background: var(--surface-2); opacity: 0.7; }
  li.off .mk :global(svg) { display: none; }
  li.off .mk::before { content: ''; width: 7px; height: 2px; border-radius: 2px; background: currentColor; }
  .fine { margin: 1.6rem 0 0; text-align: center; font-size: 0.85rem; color: var(--muted); }
  @media (min-width: 680px) { .plans { grid-template-columns: repeat(2, 1fr); } }
  @media (min-width: 1120px) { .plans { grid-template-columns: repeat(4, 1fr); } }
</style>
