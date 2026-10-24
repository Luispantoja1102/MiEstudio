<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { canWritePlatform } from '$lib/session.svelte.js';
  import FinanceExpenses from '$lib/platform/FinanceExpenses.svelte';
  import FinancePayments from '$lib/platform/FinancePayments.svelte';
  import FinanceRevenue from '$lib/platform/FinanceRevenue.svelte';

  const TABS = [['ingresos', 'Ingresos'], ['pagos', 'Pagos'], ['gastos', 'Gastos']];

  let books = $state(null);
  let month = $state(''); // AAAA-MM; vacío = el mes actual
  let tab = $state(TABS.some(([id]) => id === page.url.searchParams.get('v')) ? page.url.searchParams.get('v') : 'ingresos');
  let loading = $state(true);
  let error = $state('');

  async function load() {
    try {
      books = await api('/api/platform/finance' + (month ? `?month=${month}` : ''));
      month = books.month;
      error = '';
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }
  onMount(() => canWritePlatform() && load());

  function setTab(id) {
    tab = id;
    history.replaceState(null, '', id === 'ingresos' ? location.pathname : `${location.pathname}?v=${id}`);
  }

  function shift(delta) {
    const [y, m] = month.split('-').map(Number);
    const d = new Date(y, m - 1 + delta, 1);
    month = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`;
    loading = true;
    load();
  }

  const label = (m) => {
    const t = new Date(m + '-15T12:00:00').toLocaleDateString('es-MX', { month: 'long', year: 'numeric' });
    return t.charAt(0).toUpperCase() + t.slice(1);
  };
  const isCurrent = $derived(books && month === new Date().toISOString().slice(0, 7));
</script>

<div class="page-head">
  <div>
    <h1>Finanzas</h1>
    <p>{books ? label(month) : 'Ingresos, pagos y gastos de la plataforma.'}</p>
  </div>
  {#if books && tab !== 'pagos'}
    <div class="months" role="group" aria-label="Mes">
      <button type="button" class="iconbtn" aria-label="Mes anterior" onclick={() => shift(-1)}><span class="flip"><Icon name="arrow" size={18} /></span></button>
      <button type="button" class="iconbtn" aria-label="Mes siguiente" disabled={isCurrent} onclick={() => shift(1)}><Icon name="arrow" size={18} /></button>
    </div>
  {/if}
</div>

{#if !canWritePlatform()}
  <p class="empty">Las finanzas las ve solo el administrador de la plataforma.</p>
{:else}
  <div class="chips" role="group" aria-label="Secciones de finanzas">
    {#each TABS as [id, text]}
      <button class="chip" aria-pressed={tab === id} onclick={() => setTab(id)}>{text}</button>
    {/each}
  </div>

  {#if error}<p class="error" role="alert">{error}</p>{/if}

  {#if tab === 'pagos'}
    <FinancePayments />
  {:else if loading && !books}
    <p class="meta">Cargando números…</p>
  {:else if books}
    <div class:busy={loading}>
      {#if tab === 'ingresos'}
        <FinanceRevenue {books} />
      {:else}
        <FinanceExpenses {books} onchanged={load} />
      {/if}
    </div>
  {/if}
{/if}

<style>
  .meta { color: var(--muted); }
  .months { display: flex; gap: 8px; }
  .iconbtn { width: 40px; height: 40px; display: grid; place-items: center; border: 1px solid var(--border); border-radius: 50%; background: var(--surface); color: var(--text); cursor: pointer; }
  .iconbtn:disabled { opacity: 0.4; cursor: default; }
  .iconbtn:not(:disabled):hover { background: var(--surface-2); }
  .flip { display: grid; transform: rotate(180deg); }
  .busy { opacity: 0.6; transition: opacity 0.2s; }
</style>
