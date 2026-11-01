<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { page } from '$app/state';
  import { BILLING, KINDS, PLANS, formatDay, plural } from '$lib/format.js';
  import { canWritePlatform } from '$lib/session.svelte.js';
  import Modal from '$lib/Modal.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import FilterBar from '$lib/ui/FilterBar.svelte';
  import SegmentedControl from '$lib/ui/SegmentedControl.svelte';
  import SearchField from '$lib/ui/SearchField.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  const filters = [['', 'Todos'], ['pending', 'Pendientes'], ['trialing', 'En prueba'], ['trial_expired', 'Prueba vencida'], ['active', 'Activos'], ['past_due', 'Pago atrasado'], ['suspended', 'Suspendidos']];
  const stateOptions = filters.map(([value, label]) => ({ value, label }));
  const columns = [
    { key: 'name', label: 'Negocio' },
    { key: 'plan', label: 'Giro y plan' },
    { key: 'state', label: 'Estado' },
    { key: 'term', label: 'Vigencia' },
    { key: 'created', label: 'Registro' }
  ];
  const TONES = { pending: 'warning', trialing: 'info', trial_expired: 'danger', active: 'success', past_due: 'danger', suspended: 'danger' };
  const term = (b) => (b.status === 'trial' ? 'Prueba hasta ' + formatDay(b.trial_ends_at) : b.current_period_end ? 'Hasta ' + formatDay(b.current_period_end) : '—');

  let businesses = $state([]);
  let state = $state(page.url.searchParams.get('state') ?? '');
  let plan = $state('');
  let q = $state('');
  let loading = $state(true);
  let error = $state('');
  let showForm = $state(false);
  let busy = $state(false);
  let form = $state(blank());
  let timer;

  function blank() {
    return { business_name: '', kind: 'barberia', name: '', email: '', password: '', status: 'active' };
  }

  async function load() {
    try {
      const params = new URLSearchParams({ state, plan, q: q.trim() });
      businesses = await api('/api/platform/businesses?' + params);
      error = '';
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }
  onMount(load);

  const pick = () => load();
  const onSearch = () => {
    clearTimeout(timer);
    timer = setTimeout(load, 250);
  };

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
      const { id } = await api('/api/platform/businesses', { method: 'POST', body: form });
      goto(`/plataforma/negocios/${id}`);
    } catch (err) {
      error = err.message;
      busy = false;
    }
  }
</script>

<PageHeader title="Negocios" description="Aprueba, suspende y da de alta a los negocios de la plataforma.">
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

<FilterBar label="Filtrar negocios">
  <div class="segs"><SegmentedControl bind:value={state} options={stateOptions} label="Filtrar por estado" onchange={pick} /></div>
  <div class="srch"><SearchField bind:value={q} oninput={onSearch} label="Buscar negocios" placeholder="Nombre, identificador o correo" /></div>
  <label class="field plan">
    <span class="sr-only">Plan</span>
    <select bind:value={plan} onchange={load}>
      <option value="">Todos los planes</option>
      {#each Object.entries(PLANS) as [v, l]}<option value={v}>{l}</option>{/each}
    </select>
  </label>
</FilterBar>

<Surface padding="flush">
  {#if loading}
    <LoadingState />
  {:else if businesses.length === 0}
    {#if state || plan || q.trim()}
      <EmptyState title="Sin resultados" description="No hay negocios con ese filtro." />
    {:else}
      <EmptyState title="Todavía no hay negocios" description="Los negocios aparecen aquí cuando se registran o cuando das uno de alta." />
    {/if}
  {:else}
    <DataTable caption="Negocios de la plataforma" {columns} rows={businesses}>
      {#snippet cell({ row, column })}
        {#if column.key === 'name'}
          <a class="name" href="/plataforma/negocios/{row.id}">{row.name}</a>
          <span class="sub meta">{row.owner_email || 'Sin responsable'} · {plural(row.users, 'persona', 'personas')} · {plural(row.clients, 'cliente', 'clientes')}</span>
        {:else if column.key === 'plan'}
          <span class="two"><span>{PLANS[row.plan]}</span><span class="sub">{KINDS[row.kind]}</span></span>
        {:else if column.key === 'state'}
          {@const [label] = BILLING[row.billing_state]}
          <StatusBadge {label} tone={TONES[row.billing_state] ?? 'neutral'} />
        {:else if column.key === 'term'}
          {term(row)}
        {:else}
          {formatDay(row.created_at)}
        {/if}
      {/snippet}
    </DataTable>
  {/if}
</Surface>

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
      <label class="field">Correo <input type="email" bind:value={form.email} required /></label>
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
  .srch { flex: 1 1 260px; }
  .srch :global(.search) { max-width: 420px; }
  .plan { min-width: 190px; margin: 0; }
  .plan :global(select) { margin-top: 0; }
  .name { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; color: var(--text); text-decoration: none; overflow-wrap: anywhere; }
  .name:hover { color: var(--accent); text-decoration: underline; }
  .sub { color: var(--muted); font-size: 0.85rem; overflow-wrap: anywhere; }
  .two { display: grid; gap: 2px; }
  .meta { display: block; padding-bottom: 6px; }
  .segs { flex: 1 1 100%; }
  .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 20px; }
</style>
