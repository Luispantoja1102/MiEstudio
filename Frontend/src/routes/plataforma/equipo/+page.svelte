<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { PLATFORM_ROLES } from '$lib/format.js';
  import { canWritePlatform, session } from '$lib/session.svelte.js';
  import Modal from '$lib/Modal.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  const write = canWritePlatform();

  const columns = [
    { key: 'person', label: 'Persona' },
    { key: 'role', label: 'Rol' },
    { key: 'two', label: 'Dos pasos' },
    { key: 'access', label: 'Acceso' },
    { key: 'actions', label: 'Acciones' }
  ];

  let team = $state([]);
  let loading = $state(true);
  let error = $state('');
  let showForm = $state(false);
  let busy = $state(false);
  let form = $state({ name: '', email: '', password: '', role: 'platform_support' });

  const load = async () => (team = await api('/api/platform/team'));
  onMount(() => load().catch((e) => (error = e.message)).finally(() => (loading = false)));

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

<PageHeader title="Equipo de la plataforma" description="Quién administra Barbella. El rol de soporte solo puede consultar.">
  {#snippet actions()}
    {#if write}<button class="btn" onclick={() => (showForm = true)}>Agregar persona</button>{/if}
  {/snippet}
</PageHeader>

{#if error && !showForm}<div class="notice"><InlineNotice tone="danger" message={error} /></div>{/if}

<Surface padding="flush">
  {#if loading}
    <LoadingState />
  {:else if team.length === 0}
    <EmptyState title="Todavía no hay personas en el equipo" description="Agrega a quien vaya a administrar la plataforma o dar soporte." />
  {:else}
    <DataTable caption="Equipo de la plataforma" {columns} rows={team}>
      {#snippet cell({ row: m, column })}
        {@const me = m.id === session.me.id}
        {#if column.key === 'person'}
          <span class="who">
            <span class="name">{m.name}{#if me} <StatusBadge label="Tú" tone="accent" />{/if}</span>
            <span class="sub">{m.email}</span>
          </span>
        {:else if column.key === 'role'}
          {#if write && !me}
            <label class="sr-only" for="r-{m.id}">Rol de {m.name}</label>
            <select id="r-{m.id}" class="role" value={m.role} onchange={(e) => patch(m, { role: e.currentTarget.value })}>
              {#each Object.entries(PLATFORM_ROLES) as [v, l]}<option value={v}>{l}</option>{/each}
            </select>
          {:else}
            {PLATFORM_ROLES[m.role]}
          {/if}
        {:else if column.key === 'two'}
          <StatusBadge label={m.two_factor ? 'Activo' : 'No activo'} tone={m.two_factor ? 'success' : 'neutral'} />
        {:else if column.key === 'access'}
          <StatusBadge label={m.active ? 'Activo' : 'Inactivo'} tone={m.active ? 'success' : 'neutral'} />
        {:else if write && !me}
          <span class="acts">
            {#if m.two_factor}<button class="btn ghost sm" onclick={() => resetTwoFactor(m)}>Quitar 2 pasos</button>{/if}
            <button class="btn ghost sm" onclick={() => patch(m, { active: !m.active })}>{m.active ? 'Desactivar' : 'Reactivar'}</button>
          </span>
        {:else}
          <span class="sub">—</span>
        {/if}
      {/snippet}
    </DataTable>
  {/if}
</Surface>

<Modal bind:open={showForm} title="Agregar persona">
  <form onsubmit={add}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Correo <input type="email" bind:value={form.email} required /></label>
      <label class="field">Contraseña inicial <input type="password" bind:value={form.password} minlength="8" maxlength="72" autocomplete="new-password" required /></label>
      <label class="field">
        Rol
        <select bind:value={form.role}>{#each Object.entries(PLATFORM_ROLES) as [v, l]}<option value={v}>{l}</option>{/each}</select>
      </label>
    </div>
    <div class="actions">
      <button type="button" class="btn ghost" onclick={() => (showForm = false)}>Cancelar</button>
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Agregar'}</button>
    </div>
  </form>
</Modal>

<style>
  .notice { margin-bottom: 16px; }
  .who { display: grid; gap: 2px; }
  .name { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-weight: 600; overflow-wrap: anywhere; }
  .sub { color: var(--muted); font-size: 0.85rem; overflow-wrap: anywhere; }
  .role { width: auto; min-width: 190px; margin: 0; }
  .acts { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
  .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 20px; }
</style>
