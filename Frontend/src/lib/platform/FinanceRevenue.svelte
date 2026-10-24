<script>
  import { money } from '$lib/format.js';
  import TrendChart from './TrendChart.svelte';

  // Ingresos del mes: cifras, tendencia, lo esperado por plan y quién está pagando. `books` = respuesta de /api/platform/finance
  let { books } = $props();

  const top = $derived(Math.max(1, ...books.by_plan.map((p) => p.amount_cents)));
</script>

<div class="kpis">
  <div class="kpi">
    <span class="lbl">Ganancia del mes</span>
    <span class="body"><b class="v">{money(books.profit_cents)}</b><span class="sub">ingresos menos gastos</span></span>
  </div>
  <div class="kpi">
    <span class="lbl">Cobrado en el mes</span>
    <span class="body">
      <b class="v">{money(books.revenue_cents)}</b>
      <span class="sub">{books.manual_cents ? `incluye ${money(books.manual_cents)} cobrado a mano · ` : ''}esperado de planes activos: {money(books.expected_cents)}</span>
    </span>
  </div>
  <div class="kpi">
    <span class="lbl">Negocios que pagan</span>
    <span class="body"><b class="v">{books.clients.active}</b><span class="sub">{books.clients.trialing} en prueba (no suman)</span></span>
  </div>
  <div class="kpi" class:alert={books.clients.past_due}>
    <span class="lbl">Pago atrasado</span>
    <span class="body"><b class="v">{books.clients.past_due}</b><span class="sub">{books.clients.suspended} suspendidos</span></span>
  </div>
</div>

<section class="card block">
  <h2>Últimos 6 meses</h2>
  <p class="meta">Lo que entró contra lo que costó operar.</p>
  <TrendChart trend={books.trend} />
</section>

<div class="two">
  <section class="card">
    <h2>Ingresos por plan</h2>
    <p class="meta">Lo que pagan al mes los negocios activos de cada plan.</p>
    {#each books.by_plan as p (p.id)}
      <div class="plan">
        <span><b>{p.name}</b> <span class="meta">· {p.clients} {p.clients === 1 ? 'activo' : 'activos'} · {money(p.price_cents)} al mes</span></span>
        <b class="amt">{money(p.amount_cents)}</b>
        <span class="meter"><i style="width: {Math.round((p.amount_cents / top) * 100)}%"></i></span>
      </div>
    {/each}
  </section>

  <section class="card">
    <h2>Quién está pagando</h2>
    <p class="meta">Si el plan es anual, se muestra la parte de un mes.</p>
    {#if books.payers.length === 0}
      <p class="meta">Todavía no hay planes activos.</p>
    {:else}
      <div class="tbl-wrap">
        <table class="tbl">
          <thead><tr><th scope="col">Negocio</th><th scope="col">Plan</th><th scope="col">Cobro</th><th scope="col" class="num">Al mes</th></tr></thead>
          <tbody>
            {#each books.payers as row (row.id)}
              <tr>
                <td><a href="/plataforma/negocios/{row.id}">{row.name}</a></td>
                <td>{row.plan}</td>
                <td>{row.interval === 'year' ? 'Anual' : 'Mensual'}</td>
                <td class="num">{money(row.amount_cents)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>
</div>

<style>
  h2 { font-size: 1.2rem; margin-bottom: 0.2rem; }
  .meta { color: var(--muted); font-size: 0.85rem; }
  .block { margin-bottom: 1.25rem; }
  .two { display: grid; gap: 20px; grid-template-columns: minmax(0, 1fr); align-items: start; }
  @media (min-width: 1000px) { .two { grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr); } }
  .plan { display: grid; grid-template-columns: 1fr auto; gap: 4px 10px; margin-top: 0.85rem; }
  .amt { font-variant-numeric: tabular-nums; }
  .meter { grid-column: 1 / -1; height: 6px; border-radius: 999px; background: var(--surface-2); overflow: hidden; }
  .meter i { display: block; height: 100%; border-radius: inherit; background: var(--accent); }
  td a { color: var(--text); font-weight: 600; }
</style>
