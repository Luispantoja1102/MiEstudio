<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { PAY_METHODS, PLANS, formatDate, formatDay, money, plural } from '$lib/format.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  const columns = [
    { key: 'business', label: 'Negocio' },
    { key: 'amount', label: 'Importe', numeric: true },
    { key: 'method', label: 'Método' },
    { key: 'period', label: 'Periodo' },
    { key: 'plan', label: 'Plan' }
  ];

  let rows = $state([]);
  let error = $state('');
  let loading = $state(true);
  onMount(async () => {
    try {
      rows = await api('/api/platform/payments');
    } catch (e) {
      error = e.message;
    }
    loading = false;
  });
  const total = $derived(rows.filter((p) => p.method !== 'courtesy').reduce((n, p) => n + p.amount_cents, 0));
</script>

<PageHeader title="Pagos" description="Los últimos pagos de suscripción de todos los negocios.">
  {#snippet actions()}
    {#if rows.length}<span class="badge">{money(total)} en esta lista</span>{/if}
  {/snippet}
</PageHeader>

{#if error}
  <div class="notice"><InlineNotice tone="danger" message={error} /></div>
{/if}

<Surface padding="flush">
  {#if loading}
    <LoadingState />
  {:else if rows.length === 0}
    <EmptyState title="Aún no hay pagos" description="Los pagos de suscripción aparecen aquí cuando se registran." />
  {:else}
    <DataTable caption="Pagos de suscripción" {columns} {rows}>
      {#snippet cell({ row, column })}
        {#if column.key === 'business'}
          <a class="name" href="/plataforma/negocios/{row.business_id}">{row.business}</a>
          <span class="sub">{row.created_by}{row.note ? ' · ' + row.note : ''}</span>
        {:else if column.key === 'amount'}
          <b>{money(row.amount_cents)}</b>
        {:else if column.key === 'method'}
          <span class="two"><span>{PAY_METHODS[row.method]}</span><span class="sub">{formatDate(row.created_at)}</span></span>
        {:else if column.key === 'period'}
          <span class="two"><span>{plural(row.months, 'mes', 'meses')}</span><span class="sub">hasta {formatDay(row.period_end)}</span></span>
        {:else}
          {PLANS[row.plan] ?? row.plan}
        {/if}
      {/snippet}
    </DataTable>
  {/if}
</Surface>

<style>
  .notice { margin-bottom: 16px; }
  .name { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; color: var(--text); text-decoration: none; overflow-wrap: anywhere; }
  .name:hover { color: var(--accent); text-decoration: underline; }
  .sub { display: block; color: var(--muted); font-size: 0.85rem; overflow-wrap: anywhere; }
  .two { display: grid; gap: 2px; }
</style>
