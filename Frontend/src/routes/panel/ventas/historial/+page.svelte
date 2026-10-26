<script>
  import { onMount } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import Modal from '$lib/Modal.svelte';
  import Receipt from '$lib/pos/Receipt.svelte';
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

  const sum = $derived(rows.filter((r) => r.status === 'paid').reduce((n, r) => n + r.total_cents, 0));
</script>

<div class="page-head">
  <div>
    <h1>Historial de ventas</h1>
    <p>{plural(rows.filter((r) => r.status === 'paid').length, 'venta', 'ventas')} · {money(sum)}</p>
  </div>
  <a class="btn ghost" href="/panel/ventas">Volver a cobrar</a>
</div>

<div class="toolbar">
  <label class="check">Del <input type="date" bind:value={from} onchange={load} style="width: auto; margin: 0 0 0 6px" /></label>
  <label class="check">al <input type="date" bind:value={to} onchange={load} style="width: auto; margin: 0 0 0 6px" /></label>
  <BranchPicker onchange={load} />
  <input type="search" placeholder="Folio o cliente" aria-label="Buscar venta" bind:value={q} oninput={onSearch} style="width: 200px; margin: 0" />
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else if rows.length === 0}
  <div class="empty">No hay ventas en ese periodo.</div>
{:else}
  <div class="list">
    {#each rows as r (r.id)}
      <button class="item-row row" class:off={r.status === 'void'} onclick={() => show(r)}>
        <span class="folio">#{r.folio}</span>
        <span class="grow"><b>{r.client_name || 'Mostrador'}</b><small class="meta">{formatDate(r.created_at)} · {r.cashier} · {r.methods.map((m) => METHODS[m]).join(', ')}</small></span>
        {#if r.status === 'void'}<span class="badge bad">Cancelada</span>{/if}
        <b>{money(r.total_cents)}</b>
      </button>
    {/each}
  </div>
{/if}

<Modal bind:open title={sale ? `Venta #${sale.folio}` : 'Venta'}>
  {#if sale}
    <div class="preview"><Receipt {sale} {tz} /></div>
    {#if error}<p class="error" role="alert" style="margin-top: 1rem">{error}</p>{/if}
    {#if printing.issue}<p class="error" role="alert" style="margin-top: 1rem">No se imprimió el ticket. {printing.issue}</p>{/if}
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
  .toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; margin-bottom: 1rem; }
  .row { width: 100%; text-align: left; font: inherit; color: var(--text); cursor: pointer; }
  .row:hover { border-color: var(--accent); }
  .preview { display: grid; justify-items: center; max-height: 60vh; overflow-y: auto; padding: 0.75rem; background: var(--surface-2); border-radius: var(--radius-sm); }
  .folio { font-variant-numeric: tabular-nums; color: var(--muted); min-width: 3rem; }
  b { font-variant-numeric: tabular-nums; }
</style>
