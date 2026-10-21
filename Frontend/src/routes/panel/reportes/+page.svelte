<script>
  import { onMount } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import { api } from '$lib/api.js';
  import { METHODS, money } from '$lib/format.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { addDays, todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';

  const tz = session.me.business.timezone || 'America/Tijuana';
  const today = todayIn(tz);

  let from = $state(addDays(today, -6));
  let to = $state(today);
  let r = $state(null);
  let error = $state('');

  async function load() {
    try {
      const params = new URLSearchParams({ from, to });
      if (branches.id && branches.list.length > 1) params.set('branch_id', branches.id);
      r = await api('/api/reports/sales?' + params);
      error = '';
    } catch (e) {
      error = e.message;
    }
  }
  onMount(async () => {
    await loadBranches().catch(() => {});
    await load();
  });

  function preset(days) {
    to = today;
    from = addDays(today, -days);
    load();
  }
  const maxDay = $derived(Math.max(1, ...(r?.by_day.map((d) => d.total_cents) ?? [0])));
  const KIND = { service: 'Servicio', product: 'Producto' };
</script>

<div class="page-head">
  <div>
    <h1>Reportes</h1>
    <p>Ventas del periodo. Las ventas canceladas no cuentan.</p>
  </div>
</div>

<div class="toolbar">
  <div class="chips" style="margin: 0">
    <button class="chip" onclick={() => preset(0)}>Hoy</button>
    <button class="chip" onclick={() => preset(6)}>7 días</button>
    <button class="chip" onclick={() => preset(29)}>30 días</button>
  </div>
  <label class="check">Del <input type="date" bind:value={from} onchange={load} style="width: auto; margin: 0 0 0 6px" /></label>
  <label class="check">al <input type="date" bind:value={to} onchange={load} style="width: auto; margin: 0 0 0 6px" /></label>
  <BranchPicker onchange={load} />
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if r}
  <div class="kpis">
    <div class="card"><span class="label">Vendido</span><span class="value">{money(r.total_cents)}</span><small>{r.sales} ventas</small></div>
    <div class="card"><span class="label">Propinas</span><span class="value">{money(r.tip_cents)}</span><small>Total cobrado {money(r.collected_cents)}</small></div>
    <div class="card"><span class="label">Descuentos</span><span class="value">{money(r.discount_cents)}</span><small>Subtotal {money(r.subtotal_cents)}</small></div>
    <div class="card"><span class="label">Costo de productos</span><span class="value">{money(r.cost_cents)}</span><small>Lo vendido menos costo: {money(r.total_cents - r.cost_cents)}</small></div>
  </div>

  {#if r.sales === 0}
    <div class="empty" style="margin-top: 1.5rem">No hay ventas en este periodo.</div>
  {:else}
    <section>
      <h2>Por día</h2>
      <div class="bars" role="img" aria-label="Ventas por día">
        {#each r.by_day as d (d.date)}
          <div class="bar-row"><span class="meta">{d.date.slice(5)}</span><div class="track"><div class="fill" style="width: {(d.total_cents / maxDay) * 100}%"></div></div><b>{money(d.total_cents)}</b><small class="meta">{d.sales}</small></div>
        {/each}
      </div>
    </section>

    <div class="two">
      <section>
        <h2>Por método de pago</h2>
        <div class="list">
          {#each r.by_method as m}<div class="item-row"><span class="grow">{METHODS[m.method]}</span><small class="meta">{m.count} pagos</small><b>{money(m.amount_cents)}</b></div>{/each}
        </div>
      </section>
      <section>
        <h2>Lo más vendido</h2>
        <div class="list">
          {#each r.top_items as it}<div class="item-row"><span class="grow">{it.name}<small class="meta"> · {KIND[it.kind]} · {it.qty}</small></span><b>{money(it.total_cents)}</b></div>{/each}
        </div>
      </section>
    </div>

    <section>
      <h2>Por persona</h2>
      <div class="list">
        {#each r.by_staff as s (s.staff_id)}
          <div class="item-row"><span class="grow"><b>{s.name}</b><small class="meta"> · {s.items} partidas</small></span>
            <span class="meta">Ingreso {money(s.revenue_cents)}</span><span>Comisión <b>{money(s.commission_cents)}</b></span><span>Propinas <b>{money(s.tip_cents)}</b></span></div>
        {:else}<p class="meta">Ninguna partida tiene persona asignada.</p>{/each}
      </div>
    </section>
  {/if}
{/if}

<style>
  .toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; margin-bottom: 1.25rem; }
  .kpis { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
  .kpis .card { display: grid; gap: 2px; }
  .label { color: var(--muted); font-size: 0.9rem; }
  .value { font-family: var(--font-heading); font-size: 2rem; line-height: 1.15; font-variant-numeric: tabular-nums; }
  small { color: var(--muted); }
  section { margin-top: 2rem; }
  section h2 { font-size: 1.35rem; }
  .two { display: grid; gap: 24px; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); }
  .bars { display: grid; gap: 8px; }
  .bar-row { display: grid; grid-template-columns: 52px 1fr 100px 28px; align-items: center; gap: 10px; font-variant-numeric: tabular-nums; }
  .track { height: 14px; background: var(--surface-2); border-radius: 999px; overflow: hidden; }
  .fill { height: 100%; background: var(--accent); border-radius: 999px; min-width: 2px; }
  b { font-variant-numeric: tabular-nums; }
</style>
