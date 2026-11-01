<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { ACTIONS, formatDate } from '$lib/format.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  const columns = [
    { key: 'date', label: 'Fecha' },
    { key: 'actor', label: 'Actor' },
    { key: 'business', label: 'Negocio' },
    { key: 'action', label: 'Acción' },
    { key: 'detail', label: 'Detalle' }
  ];

  let rows = $state([]);
  let loading = $state(true);
  let error = $state('');

  onMount(async () => {
    try {
      rows = await api('/api/platform/activity?limit=200');
    } catch (e) {
      error = e.message;
    }
    loading = false;
  });

  const extra = (d) => {
    const parts = [];
    if (d.from && d.to) parts.push(`${d.from} → ${d.to}`);
    else if (d.status) parts.push(d.status);
    if (d.email) parts.push(d.email);
    if (d.registration_mode) parts.push(`registro: ${d.registration_mode}`);
    if (d.trial_ends_at) parts.push(`prueba hasta ${d.trial_ends_at}`);
    return parts.join(' · ');
  };
</script>

<PageHeader title="Actividad" description="Registro de lo que ocurre en la plataforma." />

{#if error}
  <div class="notice"><InlineNotice tone="danger" message={error} /></div>
{/if}

<Surface padding="flush">
  {#if loading}
    <LoadingState />
  {:else if rows.length === 0}
    <EmptyState title="Todavía no hay actividad" description="Los cambios hechos en la plataforma quedan registrados aquí." />
  {:else}
    <DataTable caption="Bitácora de actividad de la plataforma" {columns} {rows}>
      {#snippet cell({ row, column })}
        {#if column.key === 'date'}
          <span class="date">{formatDate(row.created_at)}</span>
        {:else if column.key === 'actor'}
          <span class="wrap">{row.actor}</span>
        {:else if column.key === 'business'}
          {#if row.business}<a class="name" href="/plataforma/negocios/{row.business_id}">{row.business}</a>{:else}<span class="sub">—</span>{/if}
        {:else if column.key === 'action'}
          <b class="wrap">{ACTIONS[row.action] ?? row.action}</b>
        {:else}
          <span class="wrap sub">{extra(row.detail) || '—'}</span>
        {/if}
      {/snippet}
    </DataTable>
  {/if}
</Surface>

<style>
  .notice { margin-bottom: 16px; }
  .wrap { overflow-wrap: anywhere; }
  .date { white-space: nowrap; font-variant-numeric: tabular-nums; }
  .name { color: var(--text); font-weight: 600; text-decoration: none; overflow-wrap: anywhere; }
  .name:hover { color: var(--accent); text-decoration: underline; }
  .sub { color: var(--muted); }
</style>
