<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import Receipt from '$lib/pos/Receipt.svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { loadSession, session } from '$lib/session.svelte.js';

  // Ventana del ticket: se ve y se imprime en papel de 58 u 80 mm. La impresión directa en la térmica
  // abre esta misma vista en segundo plano (capture=1) y la convierte en imagen.
  const PAPER_KEY = 'barbella_ticket_paper';
  const capture = page.url.searchParams.get('capture') === '1';
  const wanted = page.url.searchParams.get('paper');

  let sale = $state(null);
  let error = $state('');
  let paper = $state(wanted === '58' || wanted === '80' ? wanted : readPaper());

  function readPaper() {
    try {
      return localStorage.getItem(PAPER_KEY) === '58' ? '58' : '80';
    } catch {
      return '80';
    }
  }
  function setPaper(value) {
    paper = value;
    try {
      localStorage.setItem(PAPER_KEY, value);
    } catch {
      /* sin almacenamiento: vale solo para esta ventana */
    }
  }
  // el tamaño de la página de impresión sigue al papel elegido
  $effect(() => {
    const el = document.createElement('style');
    el.textContent = `@page { size: ${paper}mm auto; margin: 0; }`;
    document.head.appendChild(el);
    return () => el.remove();
  });

  const close = () => {
    window.close();
    // si el navegador no dejó cerrarla (no la abrió un script), regresa a cobrar
    setTimeout(() => goto('/panel/ventas'), 150);
  };

  onMount(async () => {
    try {
      await loadSession();
      sale = await api(`/api/sales/${page.params.id}`);
      if (!capture && page.url.searchParams.get('autoprint') === '1') setTimeout(() => window.print(), 400);
    } catch (e) {
      error = e.message || 'No se pudo cargar el ticket.';
    }
  });
  const onKey = (e) => e.key === 'Escape' && !capture && close();
</script>

<svelte:window onkeydown={onKey} />
<svelte:head><title>Ticket</title></svelte:head>

<div class="tk-view">
  {#if !capture}
    <div class="tk-bar">
      <button class="btn ghost sm" onclick={close} title="Cerrar (Esc)"><Icon name="arrow-left" size={16} />Cerrar</button>
      <div class="tk-title">
        <strong>{sale ? `Ticket #${sale.folio}` : 'Ticket'}</strong>
      </div>
      <div class="tk-seg" role="group" aria-label="Ancho del papel">
        <button aria-pressed={paper === '80'} onclick={() => setPaper('80')}>80 mm</button>
        <button aria-pressed={paper === '58'} onclick={() => setPaper('58')}>58 mm</button>
      </div>
      <button class="btn sm" disabled={!sale} onclick={() => window.print()}><Icon name="printer" size={16} />Imprimir</button>
    </div>
  {/if}

  <div class="tk-stage">
    {#if error}
      <p class="tk-state err" role="alert">{error}</p>
    {:else if !sale}
      <p class="tk-state">Cargando ticket…</p>
    {:else}
      <div class="tk-sheet"><Receipt {sale} {paper} tz={session.me?.business?.timezone} /></div>
    {/if}
  </div>
</div>

<style>
  .tk-view { min-height: 100vh; background: var(--surface-2); color: var(--text); }
  .tk-bar { position: sticky; top: 0; z-index: 5; display: flex; align-items: center; gap: 8px; padding: 0.6rem 0.9rem; background: var(--surface); border-bottom: 1px solid var(--border); }
  .tk-title { flex: 1; min-width: 0; }
  .tk-seg { display: inline-flex; padding: 3px; border: 1px solid var(--border); border-radius: 999px; background: var(--surface-2); }
  .tk-seg button { min-height: 32px; padding: 0 0.8rem; font: inherit; font-size: 0.82rem; font-weight: 600; color: var(--muted); background: transparent; border: 0; border-radius: 999px; cursor: pointer; }
  .tk-seg button[aria-pressed='true'] { background: var(--surface); color: var(--text); box-shadow: 0 1px 3px rgba(0, 0, 0, 0.14); }
  .tk-stage { display: grid; justify-items: center; padding: 1.6rem 0.75rem 3rem; }
  .tk-sheet { filter: drop-shadow(0 10px 18px rgba(20, 20, 22, 0.2)); max-width: 100%; }
  .tk-state { padding: 3rem 0; text-align: center; font-weight: 600; }
  .tk-state.err { color: var(--danger); }

  @media print {
    :global(html), :global(body) { background: #fff !important; }
    .tk-view { min-height: 0; background: #fff; }
    .tk-bar { display: none; }
    .tk-stage { display: block; padding: 0; }
    .tk-sheet { filter: none; }
  }
</style>
