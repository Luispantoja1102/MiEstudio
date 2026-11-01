<script>
  import { onMount } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import BarChart from '$lib/ui/BarChart.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import DateRangeFields from '$lib/ui/DateRangeFields.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import FilterBar from '$lib/ui/FilterBar.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import SegmentedControl from '$lib/ui/SegmentedControl.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import { api } from '$lib/api.js';
  import { METHODS, money, plural } from '$lib/format.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { addDays, todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';

  const tz = session.me.business.timezone || 'America/Tijuana';
  const today = todayIn(tz);

  let from = $state(addDays(today, -6));
  let to = $state(today);
  let period = $state('6'); // periodo rápido activo; '' cuando las fechas se editan a mano
  let r = $state(null);
  let error = $state('');
  let loading = $state(true);

  async function load() {
    try {
      const params = new URLSearchParams({ from, to });
      if (branches.id && branches.list.length > 1) params.set('branch_id', branches.id);
      r = await api('/api/reports/sales?' + params);
      error = '';
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }
  onMount(async () => {
    await loadBranches().catch(() => {});
    await load();
  });

  function preset(days) {
    to = today;
    from = addDays(today, -Number(days));
    load();
  }
  const PERIODS = [
    { value: '0', label: 'Hoy' },
    { value: '6', label: '7 días' },
    { value: '29', label: '30 días' }
  ];
  const KIND = { service: 'Servicio', product: 'Producto' };
  const cents = (v) => '$' + Math.round(v / 100).toLocaleString('es-MX');

  const best = $derived.by(() => {
    let b = null;
    for (const d of r?.by_day ?? []) if (d.total_cents > 0 && (!b || d.total_cents > b.total_cents)) b = d;
    return b;
  });
  const days = $derived((r?.by_day ?? []).map((d) => ({ id: d.date, label: d.date.slice(8) + '/' + d.date.slice(5, 7), value: d.total_cents, formattedValue: cents(d.total_cents) })));
  const methods = $derived((r?.by_method ?? []).map((m) => ({ id: m.method, label: `${METHODS[m.method]} · ${plural(m.count, 'pago', 'pagos')}`, value: m.amount_cents, formattedValue: money(m.amount_cents) })));
  const ITEM_COLS = [
    { key: 'name', label: 'Artículo' },
    { key: 'qty', label: 'Cantidad', numeric: true },
    { key: 'total_cents', label: 'Total', numeric: true }
  ];
  const STAFF_COLS = [
    { key: 'name', label: 'Persona' },
    { key: 'items', label: 'Partidas', numeric: true },
    { key: 'revenue_cents', label: 'Ingreso', numeric: true },
    { key: 'commission_cents', label: 'Comisión', numeric: true },
    { key: 'tip_cents', label: 'Propinas', numeric: true }
  ];
</script>

<PageHeader title="Reportes" description="Ventas del periodo. Las ventas canceladas no cuentan." />

<FilterBar label="Periodo y sucursal">
  <SegmentedControl label="Periodo" bind:value={period} options={PERIODS} onchange={preset} />
  <DateRangeFields bind:from bind:to onchange={() => ((period = ''), load())} />
  <BranchPicker onchange={load} />
</FilterBar>

{#if error}<InlineNotice tone="danger" message={error} />{/if}

{#if loading}
  <LoadingState />
{:else if r}
  <div class="metrics">
    <MetricCard label="Vendido" value={money(r.total_cents)} hint={plural(r.sales, 'venta', 'ventas')} />
    <MetricCard label="Propinas" value={money(r.tip_cents)} hint={`Total cobrado ${money(r.collected_cents)}`} />
    <MetricCard label="Descuentos" value={money(r.discount_cents)} hint={`Subtotal ${money(r.subtotal_cents)}`} />
    <MetricCard label="Costo de productos" value={money(r.cost_cents)} hint={`Lo vendido menos costo: ${money(r.total_cents - r.cost_cents)}`} />
  </div>

  {#if r.sales === 0}
    <Surface><EmptyState title="No hay ventas en este periodo" description="Prueba con otro rango de fechas o con otra sucursal." /></Surface>
  {:else}
    <Surface title="Ventas por día" description={best ? `Mejor día: ${best.date.slice(8)}/${best.date.slice(5, 7)} con ${money(best.total_cents)}` : ''}>
      <div class="scroll" tabindex="0" role="region" aria-label="Ventas por día, desplazable">
        <div style="min-width: {Math.max(0, days.length * 48)}px">
          <BarChart label="Ventas por día" items={days} selectedId={best?.date} />
        </div>
      </div>
    </Surface>

    <div class="two">
      <Surface title="Por método de pago">
        <BarChart label="Por método de pago" items={methods} orientation="horizontal" />
      </Surface>
      <Surface title="Lo más vendido" padding="flush">
        <DataTable caption="Lo más vendido" columns={ITEM_COLS} rows={r.top_items} rowKey={(it) => it.kind + it.name}>
          {#snippet cell({ row, column })}
            {#if column.key === 'name'}{row.name}<small class="kind">{KIND[row.kind]}</small>
            {:else if column.key === 'total_cents'}<b>{money(row.total_cents)}</b>
            {:else}{row[column.key]}{/if}
          {/snippet}
        </DataTable>
      </Surface>
    </div>

    <Surface title="Por persona" padding="flush">
      {#if r.by_staff.length}
        <DataTable caption="Ventas por persona" columns={STAFF_COLS} rows={r.by_staff} rowKey="staff_id">
          {#snippet cell({ row, column })}
            {#if column.key === 'name'}<b class="nm">{row.name}</b>
            {:else if column.key === 'items'}{row.items}
            {:else}{money(row[column.key])}{/if}
          {/snippet}
        </DataTable>
      {:else}
        <EmptyState title="Ninguna partida tiene persona asignada" />
      {/if}
    </Surface>
  {/if}
{/if}

<style>
  .metrics { display: grid; gap: 16px; grid-template-columns: 1fr; margin-bottom: 16px; }
  @media (min-width: 560px) { .metrics { grid-template-columns: repeat(2, 1fr); } }
  @media (min-width: 1000px) { .metrics { grid-template-columns: repeat(4, 1fr); } }
  .two { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(min(100%, 340px), 1fr)); margin: 16px 0; align-items: start; }
  .nm { white-space: nowrap; }
  .kind { display: block; color: var(--muted); font-size: 0.8rem; }
  .scroll { overflow-x: auto; }
  .scroll:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
</style>
