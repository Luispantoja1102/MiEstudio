<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { formatDate, money, plural } from '$lib/format.js';
  import { addDays, todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';

  const tz = session.me.business.timezone || 'America/Tijuana';
  const isAdmin = session.me.role === 'admin';
  const today = todayIn(tz);

  let from = $state(today.slice(0, 8) + '01'); // desde el día 1 del mes
  let to = $state(today);
  let report = $state({ staff: [], lines: [] });
  let selected = $state(null);
  let rules = $state([]);
  let services = $state([]);
  let products = $state([]);
  let team = $state([]);
  let error = $state('');
  let form = $state({ kind: 'service', itemId: '', userId: '', percent: '' });

  async function loadReport() {
    try {
      const params = new URLSearchParams({ from, to });
      const who = isAdmin ? selected : session.me.id;
      if (isAdmin && selected) params.set('staff_id', selected);
      report = await api('/api/reports/commissions?' + params);
      error = '';
    } catch (e) {
      error = e.message;
    }
  }
  async function loadRules() {
    [rules, services, products, team] = await Promise.all([
      api('/api/commission-rules'), api('/api/services'), api('/api/products'), api('/api/team')
    ]);
  }
  onMount(async () => {
    await loadReport();
    if (isAdmin) await loadRules().catch((e) => (error = e.message));
  });

  function pick(id) {
    selected = selected === id ? null : id;
    loadReport();
  }

  async function addRule(e) {
    e.preventDefault();
    error = '';
    const percent = Number(form.percent);
    if (!form.itemId || Number.isNaN(percent)) return (error = 'Elige el servicio o producto y escribe el porcentaje');
    try {
      await api('/api/commission-rules', {
        method: 'POST',
        body: { [form.kind === 'service' ? 'service_id' : 'product_id']: Number(form.itemId), user_id: form.userId ? Number(form.userId) : null, percent }
      });
      form = { kind: form.kind, itemId: '', userId: '', percent: '' };
      await loadRules();
    } catch (err) {
      error = err.message;
    }
  }
  async function patchRule(r, body) {
    try {
      await api(`/api/commission-rules/${r.id}`, { method: 'PATCH', body });
    } catch (err) {
      error = err.message;
    }
    await loadRules();
  }
  const totals = $derived(report.staff.reduce((t, s) => ({ c: t.c + s.commission_cents, tip: t.tip + s.tip_cents }), { c: 0, tip: 0 }));
</script>

<div class="page-head">
  <div>
    <h1>Comisiones</h1>
    <p>{isAdmin ? 'Lo que le corresponde a cada persona por lo que vendió.' : 'Lo que has generado en el periodo.'} Las ventas canceladas no cuentan.</p>
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

<div class="toolbar">
  <div class="chips" style="margin: 0">
    <button class="chip" onclick={() => ((from = today.slice(0, 8) + '01'), (to = today), loadReport())}>Este mes</button>
    <button class="chip" onclick={() => ((from = addDays(today, -6)), (to = today), loadReport())}>7 días</button>
  </div>
  <label class="check">Del <input type="date" bind:value={from} onchange={loadReport} style="width: auto; margin: 0 0 0 6px" /></label>
  <label class="check">al <input type="date" bind:value={to} onchange={loadReport} style="width: auto; margin: 0 0 0 6px" /></label>
</div>

{#if report.staff.length === 0}
  <div class="empty">No hay comisiones en este periodo.</div>
{:else}
  <div class="list">
    {#each report.staff as s (s.staff_id)}
      <button class="item-row row" class:sel={selected === s.staff_id} onclick={() => isAdmin && pick(s.staff_id)} disabled={!isAdmin}>
        <span class="grow"><b>{s.name}</b><small class="meta">{' · '}{plural(s.items, 'partida', 'partidas')} · ingreso {money(s.revenue_cents)}</small></span>
        <span>Comisión <b>{money(s.commission_cents)}</b></span>
        <span>Propinas <b>{money(s.tip_cents)}</b></span>
      </button>
    {/each}
  </div>
  {#if report.staff.length > 1}<p class="meta" style="margin-top: 0.75rem">Total: comisiones {money(totals.c)} · propinas {money(totals.tip)}</p>{/if}
{/if}

{#if report.lines.length}
  <section>
    <h2>Detalle</h2>
    <div class="list">
      {#each report.lines as l, i (i)}
        <div class="item-row"><span class="meta">#{l.folio}</span><span class="grow">{l.qty} × {l.item}<small class="meta">{' · '}{formatDate(l.created_at)}</small></span>
          <small class="meta">{money(l.base_cents)} × {l.pct} %</small><b>{money(l.commission_cents)}</b></div>
      {/each}
    </div>
  </section>
{/if}

{#if isAdmin}
  <section>
    <h2>Reglas</h2>
    <p class="meta">Cada persona tiene su comisión general (en Equipo) para servicios. Aquí defines excepciones por servicio o producto. Gana la más específica: persona + artículo, luego el artículo, luego la general.</p>
    <form class="card" onsubmit={addRule} style="margin: 1rem 0">
      <div class="form-grid">
        <label class="field">Aplica a
          <select bind:value={form.kind} onchange={() => (form.itemId = '')}><option value="service">Un servicio</option><option value="product">Un producto</option></select>
        </label>
        <label class="field">{form.kind === 'service' ? 'Servicio' : 'Producto'}
          <select bind:value={form.itemId}><option value="">Elige…</option>{#each form.kind === 'service' ? services : products as it (it.id)}<option value={it.id}>{it.name}</option>{/each}</select>
        </label>
        <label class="field">Persona
          <select bind:value={form.userId}><option value="">Todas</option>{#each team.filter((m) => m.active) as m (m.id)}<option value={m.id}>{m.name}</option>{/each}</select>
        </label>
        <label class="field">Comisión (%) <input type="number" min="0" max="100" step="0.5" bind:value={form.percent} required /></label>
      </div>
      <button class="btn">Agregar regla</button>
    </form>
    <div class="list">
      {#each rules as r (r.id)}
        <div class="item-row" class:off={!r.active}>
          <span class="grow"><b>{r.item_name}</b><small class="meta">{' · '}{r.user_name || 'Todas las personas'}</small></span>
          <input type="number" min="0" max="100" step="0.5" value={r.percent} aria-label="Porcentaje" style="width: 90px; margin: 0; min-height: 38px"
            onchange={(e) => patchRule(r, { percent: Number(e.currentTarget.value) })} /> %
          <button class="btn ghost sm" onclick={() => patchRule(r, { active: !r.active })}>{r.active ? 'Desactivar' : 'Activar'}</button>
        </div>
      {:else}<p class="meta">Aún no hay reglas.</p>{/each}
    </div>
  </section>
{/if}

<style>
  .toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; margin-bottom: 1.25rem; }
  .row { width: 100%; text-align: left; font: inherit; color: var(--text); }
  .row:not(:disabled) { cursor: pointer; }
  .row:not(:disabled):hover, .row.sel { border-color: var(--accent); }
  section { margin-top: 2rem; }
  section h2 { font-size: 1.35rem; }
  b { font-variant-numeric: tabular-nums; }
</style>
