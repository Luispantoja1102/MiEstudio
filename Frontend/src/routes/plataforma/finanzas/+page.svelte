<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import SegmentedControl from '$lib/ui/SegmentedControl.svelte';
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
  const TAB_OPTIONS = TABS.map(([value, label]) => ({ value, label }));
  const isCurrent = $derived(books && month === new Date().toISOString().slice(0, 7));
</script>

<PageHeader title="Finanzas" description={books ? label(month) : 'Ingresos, pagos y gastos de la plataforma.'}>
  {#snippet actions()}
    {#if books && tab !== 'pagos'}
      <div class="months" role="group" aria-label="Mes">
        <button type="button" class="iconbtn" aria-label="Mes anterior" onclick={() => shift(-1)}><span class="flip"><Icon name="arrow" size={18} /></span></button>
        <button type="button" class="iconbtn" aria-label="Mes siguiente" disabled={isCurrent} onclick={() => shift(1)}><Icon name="arrow" size={18} /></button>
      </div>
    {/if}
  {/snippet}
</PageHeader>

{#if !canWritePlatform()}
  <EmptyState title="Las finanzas las ve solo el administrador de la plataforma." />
{:else}
  <div class="tabs"><SegmentedControl label="Secciones de finanzas" value={tab} options={TAB_OPTIONS} onchange={setTab} /></div>

  {#if error}<InlineNotice tone="danger" message={error} />{/if}

  {#if tab === 'pagos'}
    <FinancePayments />
  {:else if loading && !books}
    <LoadingState label="Cargando números…" />
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
  .tabs { margin-bottom: 1rem; }
  .months { display: flex; gap: 8px; }
  .iconbtn { width: 44px; height: 44px; display: grid; place-items: center; border: 1px solid var(--border); border-radius: 50%; background: var(--surface); color: var(--text); cursor: pointer; }
  .iconbtn:disabled { opacity: 0.4; cursor: default; }
  .iconbtn:not(:disabled):hover { background: var(--surface-2); }
  .flip { display: grid; transform: rotate(180deg); }
  .busy { opacity: 0.6; transition: opacity 0.2s; }
</style>
