<script>
  import { money } from '$lib/format.js';

  // Barras por mes: lo que entró contra lo que costó operar. `trend` = [{ month: 'AAAA-MM', revenue_cents, expenses_cents, profit_cents }]
  let { trend = [] } = $props();

  const H = 150;
  const top = $derived(Math.max(1, ...trend.flatMap((t) => [t.revenue_cents, t.expenses_cents])));
  const bar = (v) => Math.max(v > 0 ? 3 : 0, Math.round((v / top) * H));
  const label = (m) => new Date(m + '-15T12:00:00').toLocaleDateString('es-MX', { month: 'short' });
</script>

<figure>
  <svg viewBox="0 0 {trend.length * 110} {H + 28}" role="img" aria-label="Ingresos y gastos de los últimos meses">
    {#each trend as t, i}
      <g transform="translate({i * 110 + 19} 0)">
        <rect class="in" x="0" y={H - bar(t.revenue_cents)} width="34" height={bar(t.revenue_cents)} rx="3"><title>Ingresos {label(t.month)}: {money(t.revenue_cents)}</title></rect>
        <rect class="out" x="38" y={H - bar(t.expenses_cents)} width="34" height={bar(t.expenses_cents)} rx="3"><title>Gastos {label(t.month)}: {money(t.expenses_cents)}</title></rect>
        <text x="36" y={H + 18} text-anchor="middle">{label(t.month)}</text>
      </g>
    {/each}
    <line x1="0" y1={H} x2={trend.length * 110} y2={H} class="axis" />
  </svg>
  <figcaption><span class="key in"></span>Ingresos <span class="key out"></span>Gastos</figcaption>
</figure>

<style>
  figure { margin: 0; }
  svg { width: 100%; height: auto; display: block; }
  .in { fill: var(--accent); }
  .out { fill: var(--clay); opacity: 0.8; }
  .axis { stroke: var(--border); stroke-width: 1; }
  text { fill: var(--muted); font-size: 12px; font-weight: 600; text-transform: capitalize; }
  figcaption { display: flex; align-items: center; gap: 6px; margin-top: 6px; font-size: 0.82rem; color: var(--muted); }
  .key { width: 10px; height: 10px; border-radius: 3px; margin-left: 10px; }
  .key:first-child { margin-left: 0; }
  .key.in { background: var(--accent); }
  .key.out { background: var(--clay); }
</style>
