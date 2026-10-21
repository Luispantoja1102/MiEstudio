<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { KINDS, STATUS, formatDay, plural } from '$lib/format.js';
  import { canWritePlatform } from '$lib/session.svelte.js';

  const filters = [['', 'Todos'], ['pending', 'Pendientes'], ['trial', 'En prueba'], ['active', 'Activos'], ['suspended', 'Suspendidos']];

  let businesses = $state([]);
  let status = $state('');
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
      const params = new URLSearchParams({ status, q: q.trim() });
      businesses = await api('/api/platform/businesses?' + params);
      error = '';
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }
  onMount(load);

  const pick = (value) => {
    status = value;
    load();
  };
  const onSearch = () => {
    clearTimeout(timer);
    timer = setTimeout(load, 250);
  };

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

<div class="page-head">
  <div>
    <h1>Negocios</h1>
    <p>Aprueba, suspende y da de alta a los negocios de la plataforma.</p>
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
    <div class="form-actions"><button class="btn" disabled={busy}>{busy ? 'Creando…' : 'Crear negocio'}</button></div>
  </form>
{/if}

<div class="chips" role="group" aria-label="Filtrar por estado">
  {#each filters as [value, label]}
    <button class="chip" aria-pressed={status === value} onclick={() => pick(value)}>{label}</button>
  {/each}
</div>
<label class="field">
  <span class="sr-only">Buscar negocios</span>
  <input type="search" placeholder="Buscar por nombre, identificador o correo" bind:value={q} oninput={onSearch} />
</label>

{#if loading}
  <p class="meta">Cargando…</p>
{:else if businesses.length === 0}
  <div class="empty">No hay negocios con ese filtro.</div>
{:else}
  <div class="list">
    {#each businesses as b (b.id)}
      {@const [label, tone] = STATUS[b.status]}
      <a class="item-row link" href="/plataforma/negocios/{b.id}">
        <div class="grow">
          <div class="title">{b.name}</div>
          <div class="meta">{KINDS[b.kind]} · {b.owner_email || 'sin responsable'} · {plural(b.users, 'persona', 'personas')} · {plural(b.clients, 'cliente', 'clientes')}</div>
        </div>
        <span class="meta">{b.status === 'trial' ? 'Prueba hasta ' + formatDay(b.trial_ends_at) : 'Alta ' + formatDay(b.created_at)}</span>
        <span class="badge {tone}">{label}</span>
      </a>
    {/each}
  </div>
{/if}

<style>
  .link { color: var(--text); text-decoration: none; transition: border-color 0.2s var(--ease); }
  .link:hover { border-color: var(--accent); }
</style>
