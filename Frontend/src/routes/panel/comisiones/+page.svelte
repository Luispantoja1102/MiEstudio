<script>
  import { onMount } from 'svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import DateRangeFields from '$lib/ui/DateRangeFields.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import FilterBar from '$lib/ui/FilterBar.svelte';
  import FormSection from '$lib/ui/FormSection.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import SegmentedControl from '$lib/ui/SegmentedControl.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import { api } from '$lib/api.js';
  import { formatDate, money, plural } from '$lib/format.js';
  import { addDays, todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';

  const tz = session.me.business.timezone || 'America/Tijuana';
  const isAdmin = session.me.role === 'admin';
  const today = todayIn(tz);

  let from = $state(today.slice(0, 8) + '01'); // desde el día 1 del mes
  let to = $state(today);
  let period = $state('month'); // periodo rápido activo; '' cuando las fechas se editan a mano
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
  function preset(p) {
    to = today;
    from = p === 'month' ? today.slice(0, 8) + '01' : addDays(today, -6);
    loadReport();
  }
  const PERIODS = [
    { value: 'month', label: 'Este mes' },
    { value: '6', label: '7 días' }
  ];
  const STAFF_COLS = [
    { key: 'name', label: 'Persona' },
    { key: 'items', label: 'Partidas', numeric: true },
    { key: 'revenue_cents', label: 'Ingreso', numeric: true },
    { key: 'commission_cents', label: 'Comisión', numeric: true },
    { key: 'tip_cents', label: 'Propinas', numeric: true }
  ];
  const LINE_COLS = [
    { key: 'folio', label: 'Folio' },
    { key: 'item', label: 'Partida' },
    { key: 'created_at', label: 'Fecha' },
    { key: 'base_cents', label: 'Base', numeric: true },
    { key: 'pct', label: '%', numeric: true },
    { key: 'commission_cents', label: 'Comisión', numeric: true }
  ];
  const RULE_COLS = [
    { key: 'item_name', label: 'Aplica a' },
    { key: 'user_name', label: 'Persona' },
    { key: 'percent', label: 'Comisión', numeric: true },
    { key: 'active', label: 'Estado' },
    { key: 'act', label: 'Acción' }
  ];
</script>

<PageHeader title="Comisiones" description={`${isAdmin ? 'Lo que le corresponde a cada persona por lo que vendió.' : 'Lo que has generado en el periodo.'} Las ventas canceladas no cuentan.`} />

{#if error}<InlineNotice tone="danger" message={error} />{/if}

<FilterBar label="Periodo">
  <SegmentedControl label="Periodo" bind:value={period} options={PERIODS} onchange={preset} />
  <DateRangeFields bind:from bind:to onchange={() => ((period = ''), loadReport())} />
</FilterBar>

<div class="metrics">
  <MetricCard label="Comisiones" value={money(totals.c)} hint={plural(report.staff.reduce((n, s) => n + s.items, 0), 'partida', 'partidas')} />
  <MetricCard label="Propinas" value={money(totals.tip)} hint="Aparte de la comisión" />
</div>

<Surface title={isAdmin ? 'Por persona' : 'Tu periodo'} description={isAdmin ? 'Toca un nombre para ver solo sus partidas; vuelve a tocarlo para quitar el filtro.' : ''} padding="flush">
  {#if report.staff.length === 0}
    <EmptyState title="No hay comisiones en este periodo" description="Prueba con otro rango de fechas." />
  {:else}
    <DataTable caption="Comisiones por persona" columns={STAFF_COLS} rows={report.staff} rowKey="staff_id">
      {#snippet cell({ row, column })}
        {#if column.key === 'name'}
          {#if isAdmin}
            <button class="pick" class:sel={selected === row.staff_id} aria-pressed={selected === row.staff_id} onclick={() => pick(row.staff_id)}>{row.name}</button>
          {:else}<b>{row.name}</b>{/if}
        {:else if column.key === 'items'}{row.items}
        {:else if column.key === 'commission_cents'}<b>{money(row.commission_cents)}</b>
        {:else}{money(row[column.key])}{/if}
      {/snippet}
    </DataTable>
  {/if}
</Surface>

{#if report.lines.length}
  <div class="gap">
    <Surface title="Detalle" description={`${plural(report.lines.length, 'partida', 'partidas')} del periodo`} padding="flush">
      <DataTable caption="Detalle de partidas con comisión" columns={LINE_COLS} rows={report.lines} rowKey={(l) => l.folio + '|' + l.item + '|' + l.created_at + '|' + l.base_cents}>
        {#snippet cell({ row, column })}
          {#if column.key === 'folio'}#{row.folio}
          {:else if column.key === 'item'}{row.qty} × {row.item}
          {:else if column.key === 'created_at'}<span class="nw">{formatDate(row.created_at)}</span>
          {:else if column.key === 'pct'}{row.pct} %
          {:else if column.key === 'commission_cents'}<b>{money(row.commission_cents)}</b>
          {:else}{money(row.base_cents)}{/if}
        {/snippet}
      </DataTable>
    </Surface>
  </div>
{/if}

{#if isAdmin}
  <h2 class="sep">Reglas</h2>
  <Surface>
    <FormSection title="Excepciones por artículo" description="Cada persona tiene su comisión general (en Equipo) para servicios. Aquí defines excepciones por servicio o producto. Gana la más específica: persona + artículo, luego el artículo, luego la general.">
      <form onsubmit={addRule}>
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
    </FormSection>
  </Surface>
  <div class="gap">
    <Surface title="Reglas vigentes" padding="flush">
      {#if rules.length === 0}
        <EmptyState title="Aún no hay reglas" description="Se usa la comisión general de cada persona." />
      {:else}
        <DataTable caption="Reglas de comisión" columns={RULE_COLS} rows={rules} rowKey="id">
          {#snippet cell({ row, column })}
            {#if column.key === 'item_name'}<b>{row.item_name}</b>
            {:else if column.key === 'user_name'}{row.user_name || 'Todas las personas'}
            {:else if column.key === 'percent'}
              <span class="pct"><input type="number" min="0" max="100" step="0.5" value={row.percent} aria-label={`Porcentaje de ${row.item_name}`}
                onchange={(e) => patchRule(row, { percent: Number(e.currentTarget.value) })} /> %</span>
            {:else if column.key === 'active'}<StatusBadge label={row.active ? 'Activa' : 'Inactiva'} tone={row.active ? 'success' : 'neutral'} />
            {:else}<button class="btn ghost sm" onclick={() => patchRule(row, { active: !row.active })}>{row.active ? 'Desactivar' : 'Activar'}</button>{/if}
          {/snippet}
        </DataTable>
      {/if}
    </Surface>
  </div>
{/if}

<style>
  .metrics { display: grid; gap: 16px; grid-template-columns: 1fr; margin-bottom: 16px; }
  @media (min-width: 560px) { .metrics { grid-template-columns: repeat(2, 1fr); } }
  .gap { margin-top: 16px; }
  .sep { margin: 40px 0 16px; font-size: 1.35rem; }
  .pick { min-height: 44px; padding: 0; border: 0; background: none; color: var(--text); font: inherit; font-weight: 700; text-align: left; cursor: pointer; text-decoration: underline; text-decoration-color: transparent; text-underline-offset: 4px; }
  .pick:hover, .pick.sel { color: var(--accent); text-decoration-color: currentColor; }
  .nw { white-space: nowrap; }
  .pct { display: inline-flex; align-items: center; gap: 6px; }
  .pct input { width: 90px; margin: 0; min-height: 44px; }
</style>
