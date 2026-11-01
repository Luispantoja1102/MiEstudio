<script>
  import { onMount } from 'svelte';
  import Modal from '$lib/Modal.svelte';
  import { api } from '$lib/api.js';
  import { PLATFORM_ROLES, cleanUsername, plural } from '$lib/format.js';
  import Timeline from '$lib/platform/Timeline.svelte';
  import { canWritePlatform, session } from '$lib/session.svelte.js';
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
    { key: 'role', label: 'Perfil' },
    { key: 'two', label: 'Dos pasos' },
    { key: 'access', label: 'Acceso' },
    { key: 'actions', label: 'Acciones' }
  ];

  let team = $state([]);
  let activity = $state([]);
  let error = $state('');
  let flash = $state('');
  let loading = $state(true);
  let showForm = $state(false);
  let busy = $state(false);
  let form = $state(blank());

  function blank() {
    return { name: '', email: '', username: '', password: '', role: 'platform_support' };
  }

  async function load() {
    team = await api('/api/platform/team');
    activity = await api('/api/platform/activity?limit=15').catch(() => []);
    loading = false;
  }
  onMount(() => load().catch((e) => ((error = e.message), (loading = false))));

  async function add(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      await api('/api/platform/team', { method: 'POST', body: { ...form, username: form.username || undefined } });
      flash = `${form.name.trim()} ya tiene acceso. Al entrar por primera vez podrá cambiar su contraseña.`;
      form = blank();
      showForm = false;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function patch(m, changes) {
    error = flash = '';
    try {
      await api(`/api/platform/team/${m.id}`, { method: 'PATCH', body: changes });
    } catch (err) {
      error = err.message;
    }
    await load();
  }

  async function resetTwoFactor(m) {
    if (!confirm(`¿Quitar la verificación en dos pasos a ${m.name}?`)) return;
    error = flash = '';
    try {
      await api(`/api/platform/users/${m.id}/2fa-reset`, { method: 'POST' });
      flash = `Se quitó la verificación en dos pasos a ${m.name}.`;
      await load();
    } catch (err) {
      error = err.message;
    }
  }
</script>

<PageHeader title="Equipo" description={plural(team.length, 'persona con acceso', 'personas con acceso')}>
  {#snippet actions()}
    {#if write}<button class="btn" onclick={() => (showForm = true)}>Agregar persona</button>{/if}
  {/snippet}
</PageHeader>

{#if flash}<div class="notice"><InlineNotice tone="success" message={flash} /></div>{/if}
{#if error && !showForm}<div class="notice"><InlineNotice tone="danger" message={error} /></div>{/if}

<Surface title="Quién tiene acceso" description="Administrador ve el dinero, los planes y al equipo. Soporte solo consulta negocios." padding="flush">
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
            <span class="sub">{[m.email, m.username && `@${m.username}`].filter(Boolean).join(' · ')}</span>
          </span>
        {:else if column.key === 'role'}
          {#if write && !me}
            <label class="sr-only" for="r-{m.id}">Perfil de {m.name}</label>
            <select id="r-{m.id}" class="role" value={m.role} onchange={(e) => patch(m, { role: e.currentTarget.value })}>
              {#each Object.entries(PLATFORM_ROLES) as [v, l]}<option value={v}>{l}</option>{/each}
            </select>
          {:else}
            {PLATFORM_ROLES[m.role]}
          {/if}
        {:else if column.key === 'two'}
          {#if m.two_factor}<StatusBadge label="Activa" tone="success" />{:else}<span title="Todavía no la activa"><StatusBadge label="Pendiente" tone="warning" /></span>{/if}
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
    <p class="hint"><b>Quitar 2 pasos</b> es para quien perdió su celular y sus códigos de respaldo: se le cierran las sesiones y la próxima vez que entre la activa de nuevo.</p>
  {/if}
</Surface>

<div class="movs">
  <Surface title="Movimientos del equipo" description="Lo que se ha cambiado en negocios, pagos, gastos y accesos.">
    <Timeline items={activity} />
  </Surface>
</div>

<Modal bind:open={showForm} title="Nueva persona del equipo">
  <form onsubmit={add}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required minlength="2" autocomplete="off" /></label>
      <label class="field">Perfil
        <select bind:value={form.role}>{#each Object.entries(PLATFORM_ROLES) as [v, l]}<option value={v}>{l}</option>{/each}</select>
      </label>
      <label class="field">Correo <input type="email" bind:value={form.email} required autocomplete="off" autocapitalize="none" spellcheck="false" /></label>
      <label class="field">Usuario <span class="sub">(opcional)</span>
        <input value={form.username} oninput={(e) => (form.username = cleanUsername(e.currentTarget.value))} maxlength="30" placeholder="Se genera desde el correo" autocomplete="off" autocapitalize="none" autocorrect="off" spellcheck="false" />
      </label>
    </div>
    <label class="field">Contraseña inicial
      <input type="password" bind:value={form.password} minlength="8" maxlength="72" autocomplete="new-password" required />
      <span class="hint">Mínimo 8 caracteres; una frase sirve.</span>
    </label>
    <p class="sub"><b>Soporte</b> consulta negocios. <b>Administrador</b> además cambia planes, ve el dinero y maneja al equipo.</p>
    <div class="actions">
      <button type="button" class="btn ghost" onclick={() => (showForm = false)}>Cancelar</button>
      <button class="btn" disabled={busy}>{busy ? 'Creando…' : 'Crear'}</button>
    </div>
  </form>
</Modal>

<style>
  .notice { margin-bottom: 16px; }
  .movs { margin-top: 16px; }
  .who { display: grid; gap: 2px; }
  .name { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-weight: 600; overflow-wrap: anywhere; }
  .sub { color: var(--muted); font-size: 0.85rem; overflow-wrap: anywhere; }
  .role { width: auto; min-width: 190px; margin: 0; }
  .acts { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
  .hint { margin: 0; padding: 14px 24px; border-top: 1px solid var(--border); color: var(--muted); font-size: 0.85rem; }
  .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 20px; }
  @media (max-width: 640px) { .role { min-width: 0; } }
</style>
