<script>
  import { onMount } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import Modal from '$lib/Modal.svelte';
  import Receipt from '$lib/pos/Receipt.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import FilterBar from '$lib/ui/FilterBar.svelte';
  import DateRangeFields from '$lib/ui/DateRangeFields.svelte';
  import SearchField from '$lib/ui/SearchField.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import { api } from '$lib/api.js';
  import { METHODS, formatDate, money, plural } from '$lib/format.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';
  import { autoPrints, clearPrintIssue, printInBrowser, printSale, printing } from '$lib/pos/printing.svelte.js';

  const tz = session.me.business.timezone || 'America/Tijuana';
  const isAdmin = session.me.role === 'admin';

  let from = $state(todayIn(tz));
  let to = $state(todayIn(tz));
  let q = $state('');
  let rows = $state([]);
  let loading = $state(true);
  let error = $state('');
  let open = $state(false);
  let sale = $state(null);
  let reason = $state('');
  let voiding = $state(false);
  let timer;

  async function load() {
    try {
      const params = new URLSearchParams({ from, to, q: q.trim() });
      if (branches.id) params.set('branch_id', branches.id);
      rows = await api('/api/sales?' + params);
      error = '';
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }
  onMount(async () => {
    await loadBranches().catch(() => {});
    await load();
  });
  const onSearch = () => (clearTimeout(timer), (timer = setTimeout(load, 250)));

  async function show(r) {
    error = '';
    sale = await api(`/api/sales/${r.id}`);
    reason = '';
    voiding = false;
    open = true;
  }

  async function voidSale(e) {
    e.preventDefault();
    error = '';
    try {
      sale = await api(`/api/sales/${sale.id}/void`, { method: 'POST', body: { reason } });
      voiding = false;
      await load();
    } catch (err) {
      error = err.message;
    }
  }

  // con térmica o estación sale directo; con el navegador se abre la ventana del ticket
  function reprint() {
    clearPrintIssue();
    if (autoPrints()) printSale(sale);
    else printInBrowser(sale);
  }

  const paid = $derived(rows.filter((r) => r.status === 'paid'));
  const paidCount = $derived(paid.length);
  const sum = $derived(paid.reduce((n, r) => n + r.total_cents, 0));
  const columns = [
    { key: 'folio', label: 'Folio' },
    { key: 'date', label: 'Fecha' },
    { key: 'client', label: 'Cliente' },
    { key: 'total', label: 'Total', numeric: true },
    { key: 'status', label: 'Estado' }
  ];
</script>

<PageHeader title="Historial de ventas" description="{plural(paidCount, 'venta', 'ventas')} · {money(sum)}">
  {#snippet actions()}<a class="btn ghost" href="/panel/ventas">Volver a cobrar</a>{/snippet}
</PageHeader>

<FilterBar label="Filtros del historial">
  <DateRangeFields bind:from bind:to onchange={load} />
  {#if branches.list.length > 1}
    <label class="field">Sucursal<BranchPicker onchange={load} /></label>
  {/if}
  <SearchField bind:value={q} label="Buscar venta" placeholder="Folio o cliente" oninput={onSearch} />
</FilterBar>

{#if error}<InlineNotice tone="danger" message={error} />{/if}

<Surface padding="flush">
  {#if loading}
    <LoadingState />
  {:else if rows.length === 0}
    <EmptyState title="No hay ventas en ese periodo" description="Cambia las fechas, la sucursal o la búsqueda." />
  {:else}
    <DataTable caption="Ventas del periodo" {columns} {rows}>
      {#snippet cell({ row, column })}
        {#if column.key === 'folio'}
          <button class="folio" onclick={() => show(row)} aria-label="Ver venta #{row.folio}">#{row.folio}</button>
        {:else if column.key === 'date'}
          <span class="nw">{formatDate(row.created_at)}</span>
        {:else if column.key === 'client'}
          <span class="cl"><b>{row.client_name || 'Mostrador'}</b>
          <small class="meta">{row.cashier} · {row.methods.map((m) => METHODS[m]).join(', ')}</small></span>
        {:else if column.key === 'total'}
          <span class:off={row.status === 'void'}>{money(row.total_cents)}</span>
        {:else}
          <StatusBadge label={row.status === 'void' ? 'Cancelada' : 'Pagada'} tone={row.status === 'void' ? 'danger' : 'success'} />
        {/if}
      {/snippet}
    </DataTable>
  {/if}
</Surface>

<Modal bind:open title={sale ? `Venta #${sale.folio}` : 'Venta'}>
  {#if sale}
    <div class="preview"><Receipt {sale} {tz} /></div>
    {#if error}<div style="margin-top: 1rem"><InlineNotice tone="danger" message={error} /></div>{/if}
    {#if printing.issue}<div style="margin-top: 1rem"><InlineNotice tone="danger" message="No se imprimió el ticket. {printing.issue}" /></div>{/if}
    <div class="form-actions" style="justify-content: center; margin-top: 1rem">
      <button class="btn ghost" disabled={printing.busy} onclick={reprint}>{printing.busy ? 'Imprimiendo…' : 'Imprimir'}</button>
      {#if isAdmin && sale.status === 'paid' && !voiding}<button class="btn ghost" onclick={() => (voiding = true)}>Cancelar venta</button>{/if}
    </div>
    {#if voiding}
      <form onsubmit={voidSale} style="margin-top: 1rem">
        <label class="field">Motivo de la cancelación <input bind:value={reason} minlength="3" maxlength="300" required /></label>
        <p class="meta">Se devolverá la existencia de los productos y los cobros con terminal.</p>
        <div class="form-actions"><button class="btn">Confirmar cancelación</button><button type="button" class="btn ghost" onclick={() => (voiding = false)}>No cancelar</button></div>
      </form>
    {/if}
  {/if}
</Modal>

<style>
  .preview { display: grid; justify-items: center; max-height: 60vh; overflow-y: auto; padding: 0.75rem; background: var(--surface-2); border-radius: var(--radius-sm); }
  .folio { min-height: 44px; padding: 0 4px; font: inherit; font-weight: 600; font-variant-numeric: tabular-nums; color: var(--ink); background: none; border: 0; cursor: pointer; text-decoration: underline; text-underline-offset: 3px; }
  .folio:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; border-radius: var(--radius-xs); }
  .cl { display: block; min-width: 190px; }
  .cl b, .cl small { display: block; }
  .nw { white-space: nowrap; }
  .off { color: var(--muted); text-decoration: line-through; }
</style>
