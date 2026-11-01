<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import Iniciales from './Iniciales.svelte';
  import Modal from '$lib/Modal.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import FilterBar from '$lib/ui/FilterBar.svelte';
  import SegmentedControl from '$lib/ui/SegmentedControl.svelte';
  import SearchField from '$lib/ui/SearchField.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import { api } from '$lib/api.js';
  import { BILLING, KINDS, PLANS, formatDay, plural } from '$lib/format.js';
  import { canWritePlatform } from '$lib/session.svelte.js';

  // Clientes: la lista a la izquierda y, al elegir uno, su ficha a la derecha (en celular, una a la vez).
  let { children } = $props();

  const FILTERS = [['', 'Todos'], ['pending', 'Pendientes'], ['trialing', 'En prueba'], ['trial_expired', 'Prueba vencida'], ['active', 'Activos'], ['past_due', 'Pago atrasado'], ['suspended', 'Suspendidos']];
  const TONES = { pending: 'warning', trialing: 'info', trial_expired: 'danger', active: 'success', past_due: 'danger', suspended: 'danger' };
  const SORTS = { recent: 'Más recientes', name: 'Nombre', trial: 'Prueba por vencer' };

  let all = $state([]);
  let filter = $state(page.url.searchParams.get('state') ?? '');
  let query = $state('');
  let plan = $state('');
  let sort = $state('recent');
  let loading = $state(true);
  let error = $state('');
  let showForm = $state(false);
  let busy = $state(false);
  let form = $state(blank());

  const id = $derived(page.params.id);
  const stateOptions = $derived(FILTERS.map(([value, label]) => ({ value, label: `${label} ${count(value)}` })));

  function blank() {
    return { business_name: '', kind: 'barberia', name: '', email: '', password: '', status: 'active' };
  }

  async function load() {
    try {
      all = await api('/api/platform/businesses');
      error = '';
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }
  onMount(() => {
    load();
    window.addEventListener('clients-changed', load);
    return () => window.removeEventListener('clients-changed', load);
  });

  const count = (state) => (state ? all.filter((b) => b.billing_state === state).length : all.length);
  const shown = $derived.by(() => {
    const q = query.trim().toLowerCase();
    const list = all.filter(
      (b) => (!filter || b.billing_state === filter) && (!plan || b.plan === plan) && (!q || [b.name, b.slug, b.owner_email].some((x) => (x ?? '').toLowerCase().includes(q)))
    );
    if (sort === 'name') list.sort((a, b) => a.name.localeCompare(b.name, 'es'));
    if (sort === 'trial') list.sort((a, b) => (a.status === 'trial' ? new Date(a.trial_ends_at) : Infinity) - (b.status === 'trial' ? new Date(b.trial_ends_at) : Infinity));
    return list;
  });

  function openForm() {
    error = '';
    form = blank();
    showForm = true;
  }

  async function create(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      const { id: newId } = await api('/api/platform/businesses', { method: 'POST', body: form });
      form = blank();
      showForm = false;
      await load();
      goto(`/plataforma/negocios/${newId}`);
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<PageHeader title="Clientes" description="{plural(all.length, 'negocio', 'negocios')} · {count('active')} activos · {count('trialing')} en prueba">
  {#snippet actions()}
    {#if canWritePlatform()}<button class="btn" onclick={openForm}>Nuevo negocio</button>{/if}
  {/snippet}
</PageHeader>

{#if error && !showForm}
  <div class="notice">
    <InlineNotice tone="danger" message={error}>
      {#snippet actions()}<button class="btn ghost sm" onclick={load}>Reintentar</button>{/snippet}
    </InlineNotice>
  </div>
{/if}

<div class:hide-narrow={id}>
  <FilterBar label="Filtrar clientes">
    <div class="segs"><SegmentedControl bind:value={filter} options={stateOptions} label="Filtrar por estado" /></div>
    <div class="srch"><SearchField bind:value={query} label="Buscar negocio" placeholder="Negocio, correo o identificador" /></div>
    <label class="field sel">
      <span class="sr-only">Plan</span>
      <select bind:value={plan}>
        <option value="">Todos los planes</option>
        {#each Object.entries(PLANS) as [v, l]}<option value={v}>{l}</option>{/each}
      </select>
    </label>
    <label class="field sel">
      <span class="sr-only">Ordenar por</span>
      <select bind:value={sort}>{#each Object.entries(SORTS) as [v, l]}<option value={v}>{l}</option>{/each}</select>
    </label>
  </FilterBar>
</div>

<div class="split" class:detail={id}>
  <div class="list-pane">
    <Surface padding="flush">
      {#if loading}
        <LoadingState />
      {:else if shown.length === 0}
        <EmptyState
          title={all.length ? 'Ningún negocio coincide' : 'Todavía no hay negocios'}
          description={all.length ? 'Cambia el filtro o la búsqueda.' : 'Aparecen aquí cuando se registran o cuando das uno de alta.'}
        />
      {:else}
        <ul aria-label="Lista de negocios">
          {#each shown as b (b.id)}
            {@const [label] = BILLING[b.billing_state]}
            <li>
              <a class="row" class:on={String(b.id) === id} href="/plataforma/negocios/{b.id}" aria-current={String(b.id) === id ? 'true' : undefined}>
                <Iniciales name={b.name} />
                <span class="body">
                  <b>{b.name}</b>
                  <span class="meta">{b.owner_email || 'sin responsable'}</span>
                  <span class="chipline">
                    <StatusBadge {label} tone={TONES[b.billing_state] ?? 'neutral'} />
                    <span class="meta">{PLANS[b.plan]} · {b.status === 'trial' ? 'prueba hasta ' + formatDay(b.trial_ends_at) : b.current_period_end ? 'hasta ' + formatDay(b.current_period_end) : 'alta ' + formatDay(b.created_at)}</span>
                  </span>
                </span>
              </a>
            </li>
          {/each}
        </ul>
      {/if}
    </Surface>
  </div>

  <div class="detail-pane">
    {#key id}{@render children()}{/key}
  </div>
</div>

<Modal bind:open={showForm} title="Nuevo negocio">
  <form onsubmit={create}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="form-grid">
      <label class="field">Nombre del negocio <input bind:value={form.business_name} required /></label>
      <label class="field">
        Giro
        <select bind:value={form.kind}>{#each Object.entries(KINDS) as [v, l]}<option value={v}>{l}</option>{/each}</select>
      </label>
      <label class="field">Nombre de la persona responsable <input bind:value={form.name} required /></label>
      <label class="field">Correo <input type="email" bind:value={form.email} required autocapitalize="none" spellcheck="false" /></label>
      <label class="field">
        Contraseña inicial
        <input type="password" bind:value={form.password} minlength="8" maxlength="72" autocomplete="new-password" required />
        <span class="hint">Compártela de forma segura; la persona puede cambiarla al entrar.</span>
      </label>
      <label class="field">
        Estado inicial
        <select bind:value={form.status}>
          <option value="active">Activo</option>
          <option value="trial">En prueba (14 días)</option>
          <option value="pending">Pendiente</option>
        </select>
      </label>
    </div>
    <div class="actions">
      <button type="button" class="btn ghost" onclick={() => (showForm = false)}>Cancelar</button>
      <button class="btn" disabled={busy}>{busy ? 'Creando…' : 'Crear negocio'}</button>
    </div>
  </form>
</Modal>

<style>
  .notice { margin-bottom: 16px; }
  .segs { flex: 1 1 100%; }
  .srch { flex: 1 1 260px; }
  .srch :global(.search) { max-width: 420px; }
  .sel { min-width: 190px; margin: 0; }
  .sel :global(select) { margin-top: 0; }

  .split { display: grid; gap: 24px; grid-template-columns: minmax(0, 1fr); align-items: start; }
  ul { list-style: none; margin: 0; padding: 6px; display: grid; gap: 2px; }
  .row { display: flex; align-items: flex-start; gap: 12px; min-height: 44px; padding: 12px; color: var(--text); text-decoration: none; border-radius: var(--radius-sm); }
  .row:hover { background: var(--surface-2); }
  .row.on { background: var(--accent-soft); }
  .body { display: grid; gap: 2px; min-width: 0; line-height: 1.3; }
  .body b { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { color: var(--muted); font-size: 0.85rem; overflow: hidden; text-overflow: ellipsis; }
  .chipline { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; margin-top: 4px; }
  .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 20px; }

  /* con un negocio abierto, en celular se ve solo su ficha, con el enlace de volver */
  @media (max-width: 999px) {
    .hide-narrow { display: none; }
    .split.detail .list-pane { display: none; }
    .split:not(.detail) .detail-pane { display: none; }
  }
  @media (min-width: 1000px) {
    .split { grid-template-columns: minmax(300px, 380px) minmax(0, 1fr); }
    .list-pane { position: sticky; top: 16px; max-height: calc(100vh - 140px); overflow-y: auto; border-radius: var(--radius); }
    .detail-pane :global(.back) { display: none; }
  }
</style>
