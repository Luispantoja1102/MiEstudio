<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { PLATFORM_ROLES } from '$lib/format.js';
  import { canWritePlatform, session } from '$lib/session.svelte.js';

  const write = canWritePlatform();

  let team = $state([]);
  let error = $state('');
  let showForm = $state(false);
  let busy = $state(false);
  let form = $state({ name: '', email: '', password: '', role: 'platform_support' });

  const load = async () => (team = await api('/api/platform/team'));
  onMount(() => load().catch((e) => (error = e.message)));

  async function add(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      await api('/api/platform/team', { method: 'POST', body: form });
      form = { name: '', email: '', password: '', role: 'platform_support' };
      showForm = false;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function patch(m, changes) {
    error = '';
    try {
      await api(`/api/platform/team/${m.id}`, { method: 'PATCH', body: changes });
    } catch (err) {
      error = err.message;
    }
    await load();
  }

  async function resetTwoFactor(m) {
    if (!confirm(`¿Quitar la verificación en dos pasos a ${m.name}?`)) return;
    error = '';
    try {
      await api(`/api/platform/users/${m.id}/2fa-reset`, { method: 'POST' });
      await load();
    } catch (err) {
      error = err.message;
    }
  }
</script>

<div class="page-head">
  <div>
    <h1>Equipo de la plataforma</h1>
    <p>Quién administra Barbella. El rol de soporte solo puede consultar.</p>
  </div>
  {#if write}<button class="btn" onclick={() => (showForm = !showForm)}>{showForm ? 'Cancelar' : 'Agregar persona'}</button>{/if}
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if showForm}
  <form class="card" onsubmit={add} style="margin-bottom: 1.5rem">
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Correo <input type="email" bind:value={form.email} required /></label>
      <label class="field">Contraseña inicial <input type="password" bind:value={form.password} minlength="8" maxlength="72" autocomplete="new-password" required /></label>
      <label class="field">
        Rol
        <select bind:value={form.role}>{#each Object.entries(PLATFORM_ROLES) as [v, l]}<option value={v}>{l}</option>{/each}</select>
      </label>
    </div>
    <div class="form-actions"><button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Agregar'}</button></div>
  </form>
{/if}

<div class="list">
  {#each team as m (m.id)}
    {@const me = m.id === session.me.id}
    <div class="item-row" class:off={!m.active}>
      <div class="grow">
        <div class="title">{m.name} {#if me}<span class="badge">Tú</span>{/if}{#if !m.active} <span class="badge muted">Inactivo</span>{/if}</div>
        <div class="meta">{m.email}</div>
      </div>
      {#if write && !me}
        <label class="sr-only" for="r-{m.id}">Rol de {m.name}</label>
        <select id="r-{m.id}" value={m.role} onchange={(e) => patch(m, { role: e.currentTarget.value })} style="width: auto; margin: 0">
          {#each Object.entries(PLATFORM_ROLES) as [v, l]}<option value={v}>{l}</option>{/each}
        </select>
        {#if m.two_factor}
          <span class="badge" title="Tiene verificación en dos pasos">2 pasos</span>
          {#if !me}<button class="btn ghost sm" onclick={() => resetTwoFactor(m)}>Quitar 2 pasos</button>{/if}
        {/if}
        <button class="btn ghost sm" onclick={() => patch(m, { active: !m.active })}>{m.active ? 'Desactivar' : 'Reactivar'}</button>
      {:else}
        <span class="badge muted">{PLATFORM_ROLES[m.role]}</span>
      {/if}
    </div>
  {/each}
</div>
