<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import Avatar from '$lib/Avatar.svelte';
  import { api } from '$lib/api.js';
  import { BILLING, KINDS, PLANS, formatDay, plural } from '$lib/format.js';
  import { canWritePlatform } from '$lib/session.svelte.js';

  // Clientes: la lista a la izquierda y, al elegir uno, su ficha a la derecha (en celular, una a la vez).
  let { children } = $props();

  const FILTERS = [['', 'Todos'], ['pending', 'Pendientes'], ['trialing', 'En prueba'], ['trial_expired', 'Prueba vencida'], ['active', 'Activos'], ['past_due', 'Pago atrasado'], ['suspended', 'Suspendidos']];
  const ALERTS = ['pending', 'trial_expired', 'past_due'];
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

<div class="page-head">
  <div>
    <h1>Clientes</h1>
    <p>{plural(all.length, 'negocio', 'negocios')} · {count('active')} activos · {count('trialing')} en prueba</p>
  </div>
  {#if canWritePlatform()}
    <button class="btn" onclick={() => (showForm = !showForm)}>{showForm ? 'Cancelar' : 'Nuevo negocio'}</button>
  {/if}
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if showForm}
  <form class="card" onsubmit={create} style="margin-bottom: 1.5rem">
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
    <div class="form-actions"><button class="btn" disabled={busy}>{busy ? 'Creando…' : 'Crear negocio'}</button></div>
  </form>
{/if}

<div class="tools" class:hide-narrow={id}>
  <label class="field grow">
    <span class="sr-only">Buscar negocio</span>
    <input type="search" placeholder="Buscar negocio, correo o identificador" bind:value={query} />
  </label>
  <label class="field">
    <span class="sr-only">Plan</span>
    <select bind:value={plan}>
      <option value="">Todos los planes</option>
      {#each Object.entries(PLANS) as [v, l]}<option value={v}>{l}</option>{/each}
    </select>
  </label>
  <label class="field">
    <span class="sr-only">Ordenar por</span>
    <select bind:value={sort}>{#each Object.entries(SORTS) as [v, l]}<option value={v}>{l}</option>{/each}</select>
  </label>
</div>
<div class="chips" class:hide-narrow={id} role="group" aria-label="Filtrar por estado">
  {#each FILTERS as [value, label]}
    <button class="chip" aria-pressed={filter === value} onclick={() => (filter = value)}>
      {label} <span class="n" class:alert={ALERTS.includes(value) && count(value)}>{count(value)}</span>
    </button>
  {/each}
</div>

<div class="split" class:detail={id}>
  <section class="card list-pane" aria-label="Lista de negocios">
    {#if loading}
      <p class="meta">Cargando…</p>
    {:else if shown.length === 0}
      <p class="meta none">{all.length ? 'Ningún negocio coincide. Cambia el filtro o la búsqueda.' : 'Todavía no hay negocios.'}</p>
    {:else}
      <ul>
        {#each shown as b (b.id)}
          {@const [label, tone] = BILLING[b.billing_state]}
          <li>
            <a class="row" class:on={String(b.id) === id} href="/plataforma/negocios/{b.id}" aria-current={String(b.id) === id ? 'true' : undefined}>
              <Avatar name={b.name} />
              <span class="body">
                <b>{b.name}</b>
                <span class="meta">{b.owner_email || 'sin responsable'}</span>
                <span class="chipline">
                  <span class="badge {tone}">{label}</span>
                  <span class="badge muted">{PLANS[b.plan]}</span>
                  <span class="meta">{b.status === 'trial' ? 'prueba hasta ' + formatDay(b.trial_ends_at) : b.current_period_end ? 'hasta ' + formatDay(b.current_period_end) : 'alta ' + formatDay(b.created_at)}</span>
                </span>
              </span>
            </a>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <div class="detail-pane">
    {#key id}{@render children()}{/key}
  </div>
</div>

<style>
  .tools { display: flex; flex-wrap: wrap; gap: 12px; }
  .tools .grow { flex: 1 1 260px; }
  .tools :global(select) { width: auto; }
  .n { margin-left: 4px; padding: 0 7px; border-radius: 999px; font-size: 0.75rem; font-weight: 700; background: var(--surface-2); color: var(--muted); }
  .chip[aria-pressed='true'] .n { background: rgb(255 255 255 / 0.22); color: inherit; }
  .n.alert { background: var(--danger-soft); color: var(--danger); }

  .split { display: grid; gap: 20px; grid-template-columns: minmax(0, 1fr); align-items: start; }
  .list-pane { padding: 0.5rem; }
  ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 2px; }
  .row { display: flex; align-items: flex-start; gap: 12px; padding: 0.7rem; color: var(--text); text-decoration: none; border-radius: var(--radius-sm); }
  .row:hover { background: var(--surface-2); }
  .row.on { background: var(--accent-soft); }
  .body { display: grid; gap: 2px; min-width: 0; line-height: 1.3; }
  .body b { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { color: var(--muted); font-size: 0.85rem; }
  .none { padding: 1rem; }
  .chipline { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin-top: 2px; }

  /* con un negocio abierto, en celular se ve solo su ficha, con el enlace de volver */
  @media (max-width: 999px) {
    .hide-narrow { display: none; }
    .split.detail .list-pane { display: none; }
    .split:not(.detail) .detail-pane { display: none; }
  }
  @media (min-width: 1000px) {
    .split { grid-template-columns: minmax(300px, 380px) minmax(0, 1fr); }
    .list-pane { position: sticky; top: 16px; max-height: calc(100vh - 140px); overflow-y: auto; }
    .detail-pane :global(.back) { display: none; }
  }
</style>
