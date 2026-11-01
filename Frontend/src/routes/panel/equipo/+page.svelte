<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import HoursEditor from '$lib/HoursEditor.svelte';
  import Modal from '$lib/Modal.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import { api } from '$lib/api.js';
  import { ROLES } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';

  let team = $state([]);
  let branches = $state([]);
  let hoursFor = $state(null); // id de la persona cuyo horario se edita
  let loading = $state(true);
  let error = $state('');
  let showForm = $state(false);
  let busy = $state(false);
  let form = $state(blank());

  function blank() {
    return { name: '', email: '', password: '', role: 'staff', bookable: true, commission_pct: 0 };
  }

  async function load() {
    team = await api('/api/team');
    branches = (await api('/api/branches')).filter((b) => b.active);
    loading = false;
  }

  onMount(() => {
    if (session.me.role !== 'admin') return goto('/panel');
    load().catch((e) => ((error = e.message), (loading = false)));
  });

  async function add(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      await api('/api/team', { method: 'POST', body: { ...form, commission_pct: Number(form.commission_pct) } });
      form = blank();
      showForm = false;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function patch(member, changes) {
    error = '';
    try {
      await api(`/api/team/${member.id}`, { method: 'PATCH', body: changes });
      await load();
    } catch (err) {
      error = err.message;
      await load();
    }
  }

  async function resetTwoFactor(m) {
    if (!confirm(`¿Quitar la verificación en dos pasos a ${m.name}? Úsalo si perdió su teléfono; tendrá que volver a iniciar sesión.`)) return;
    error = '';
    try {
      await api(`/api/team/${m.id}/2fa/reset`, { method: 'POST' });
      await load();
    } catch (err) {
      error = err.message;
    }
  }
</script>

<PageHeader title="Equipo" description="Personas con acceso a tu negocio y lo que pueden hacer.">
  {#snippet actions()}
    <button class="btn" onclick={() => ((error = ''), (showForm = true))}>Agregar persona</button>
  {/snippet}
</PageHeader>

{#if error && !showForm}
  <div class="notice"><InlineNotice tone="danger" message={error} /></div>
{/if}

<Surface padding="flush">
  {#if loading}
    <LoadingState />
  {:else if team.length === 0}
    <EmptyState title="Todavía no hay personas" description="Agrega a quien atiende o administra tu negocio." />
  {:else}
    <ul class="team">
      {#each team as m (m.id)}
        {@const me = m.id === session.me.id}
        <li class:off={!m.active}>
          <div class="row">
            <div class="who">
              <span class="name">{m.name}</span>
              <span class="mail">{m.email}</span>
              <span class="tags">
                {#if me}<StatusBadge label="Tú" tone="accent" />{/if}
                <StatusBadge label={m.active ? 'Activo' : 'Inactivo'} tone={m.active ? 'success' : 'neutral'} />
                {#if m.two_factor}<StatusBadge label="2 pasos" tone="info" />{/if}
              </span>
            </div>

            <div class="field-sm">
              <label for="role-{m.id}">Rol</label>
              <select id="role-{m.id}" value={m.role} disabled={me} onchange={(e) => patch(m, { role: e.currentTarget.value })}>
                {#each Object.entries(ROLES) as [value, label]}<option {value}>{label}</option>{/each}
              </select>
            </div>

            <div class="field-sm com">
              <label for="com-{m.id}">Comisión %</label>
              <input id="com-{m.id}" type="number" inputmode="decimal" min="0" max="100" step="0.5" value={m.commission_pct}
                onchange={(e) => patch(m, { commission_pct: Number(e.currentTarget.value) })} />
            </div>

            <label class="check agenda"><input type="checkbox" checked={m.bookable} onchange={(e) => patch(m, { bookable: e.currentTarget.checked })} />En agenda</label>

            <div class="acts">
              {#if m.bookable && m.active}
                <button class="btn ghost sm" aria-expanded={hoursFor === m.id} onclick={() => (hoursFor = hoursFor === m.id ? null : m.id)}>
                  {hoursFor === m.id ? 'Cerrar horario' : 'Horario'}
                </button>
              {/if}
              {#if m.two_factor && !me}
                <button class="btn ghost sm" onclick={() => resetTwoFactor(m)}>Quitar 2 pasos</button>
              {/if}
              {#if !me}
                <button class="btn ghost sm" onclick={() => patch(m, { active: !m.active })}>{m.active ? 'Desactivar' : 'Reactivar'}</button>
              {/if}
            </div>
          </div>
          {#if hoursFor === m.id}<HoursEditor member={m} {branches} />{/if}
        </li>
      {/each}
    </ul>
  {/if}
</Surface>

<Modal bind:open={showForm} title="Agregar persona">
  <form onsubmit={add}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Correo <input type="email" bind:value={form.email} required /></label>
      <label class="field">
        Contraseña inicial
        <input type="password" bind:value={form.password} minlength="8" maxlength="72" autocomplete="new-password" required />
        <span class="hint">Mínimo 8 caracteres. La persona podrá cambiarla.</span>
      </label>
      <label class="field">
        Rol
        <select bind:value={form.role}>
          {#each Object.entries(ROLES) as [value, label]}<option {value}>{label}</option>{/each}
        </select>
      </label>
      <label class="field">
        Comisión (%)
        <input type="number" min="0" max="100" step="0.5" bind:value={form.commission_pct} />
      </label>
    </div>
    <label class="check"><input type="checkbox" bind:checked={form.bookable} />Aparece en la agenda (atiende clientes)</label>
    <div class="actions">
      <button type="button" class="btn ghost" onclick={() => (showForm = false)}>Cancelar</button>
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Agregar'}</button>
    </div>
  </form>
</Modal>

<style>
  .notice { margin-bottom: 16px; }
  .team { list-style: none; margin: 0; padding: 0; }
  li { container-type: inline-size; border-bottom: 1px solid var(--border); padding: 16px 20px; }
  li:last-child { border-bottom: 0; }
  li.off .name, li.off .mail { color: var(--muted); }
  .row { display: grid; grid-template-columns: minmax(0, 1.4fr) 170px 120px auto; gap: 12px 20px; align-items: end; }
  .who { display: grid; gap: 2px; min-width: 0; align-self: center; }
  .name { font-weight: 600; overflow-wrap: anywhere; }
  .mail { color: var(--muted); font-size: 0.875rem; overflow-wrap: anywhere; }
  .tags { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; }
  .field-sm { display: grid; gap: 4px; }
  .field-sm label { font-size: 0.8rem; font-weight: 600; color: var(--muted); }
  .field-sm select, .field-sm input { margin: 0; min-height: 44px; width: 100%; }
  .agenda { align-self: end; }
  .acts { grid-column: 1 / -1; display: flex; flex-wrap: wrap; gap: 8px; }
  @container (min-width: 900px) {
    .row { grid-template-columns: minmax(0, 1fr) 170px 110px 120px 340px; }
    .acts { grid-column: auto; justify-content: flex-end; }
  }
  @container (max-width: 560px) {
    .row { grid-template-columns: 1fr 1fr; }
    .who, .agenda { grid-column: 1 / -1; }
    .acts .btn { flex: 1 1 auto; }
  }
  @media (max-width: 640px) { li { padding: 16px; } }
  .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 20px; }
</style>
