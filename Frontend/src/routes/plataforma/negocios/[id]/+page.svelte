<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { KINDS, ROLES, STATUS, ACTIONS, formatDate, formatDay } from '$lib/format.js';
  import { canWritePlatform } from '$lib/session.svelte.js';

  const id = page.params.id;
  const write = canWritePlatform();

  let data = $state(null);
  let activity = $state([]);
  let error = $state('');
  let notice = $state('');
  let trialDate = $state('');
  let resetFor = $state(null);
  let newPassword = $state('');

  async function load() {
    data = await api(`/api/platform/businesses/${id}`);
    activity = await api(`/api/platform/activity?business_id=${id}&limit=20`);
  }
  onMount(() => load().catch((e) => (error = e.status === 404 ? 'No encontramos este negocio.' : e.message)));

  async function run(fn, okMsg) {
    error = '';
    notice = '';
    try {
      await fn();
      notice = okMsg;
      await load();
    } catch (e) {
      error = e.message;
    }
  }

  const setStatus = (status, okMsg) =>
    run(() => api(`/api/platform/businesses/${id}`, { method: 'PATCH', body: { status } }), okMsg);

  const extendTrial = () =>
    run(() => api(`/api/platform/businesses/${id}`, { method: 'PATCH', body: { trial_ends_at: trialDate } }), 'Fecha de prueba actualizada.');

  const toggleUser = (u) =>
    run(() => api(`/api/platform/users/${u.id}`, { method: 'PATCH', body: { active: !u.active } }), u.active ? 'Persona desactivada.' : 'Persona reactivada.');

  async function resetPassword(e, u) {
    e.preventDefault();
    await run(() => api(`/api/platform/users/${u.id}/password`, { method: 'POST', body: { password: newPassword } }), `Contraseña de ${u.name} restablecida. Compártela de forma segura.`);
    newPassword = '';
    resetFor = null;
  }
</script>

<a class="back" href="/plataforma/negocios"><Icon name="arrow" size={16} />Negocios</a>

{#if error}<p class="error" role="alert">{error}</p>{/if}
{#if notice}<p class="banner" role="status">{notice}</p>{/if}

{#if data}
  {@const b = data.business}
  {@const [label, tone] = STATUS[b.status]}
  <div class="page-head">
    <div>
      <h1>{b.name}</h1>
      <p>{KINDS[b.kind]} · identificador <code>{b.slug}</code> · alta {formatDay(b.created_at)}</p>
    </div>
    <span class="badge {tone}">{label}</span>
  </div>

  <section class="card">
    <h2>Estado de la cuenta</h2>
    <p class="meta">
      {b.users} personas · {b.clients} clientes · responsable: {b.owner_email || '—'}
      {#if b.status === 'trial'} · prueba hasta {formatDay(b.trial_ends_at)}{/if}
    </p>
    {#if write}
      <div class="form-actions">
        {#if b.status === 'pending'}<button class="btn" onclick={() => setStatus('trial', 'Negocio aprobado: inició su prueba de 14 días.')}>Aprobar</button>{/if}
        {#if b.status !== 'active'}<button class="btn" class:ghost={b.status === 'pending'} onclick={() => setStatus('active', 'Negocio marcado como activo.')}>Marcar como activo</button>{/if}
        {#if b.status === 'suspended'}<button class="btn ghost" onclick={() => setStatus('trial', 'Negocio reactivado en prueba.')}>Reactivar en prueba</button>{/if}
        {#if b.status !== 'suspended'}<button class="btn ghost" onclick={() => confirm('¿Suspender este negocio? Su equipo perderá el acceso.') && setStatus('suspended', 'Negocio suspendido.')}>Suspender</button>{/if}
      </div>
      <div class="trial">
        <label class="field" style="margin: 0">
          Extender o fijar fin de la prueba
          <input type="date" bind:value={trialDate} />
        </label>
        <button class="btn ghost sm" disabled={!trialDate} onclick={extendTrial}>Guardar fecha</button>
      </div>
    {/if}
  </section>

  <section>
    <h2>Equipo del negocio</h2>
    <div class="list">
      {#each data.team as u (u.id)}
        <div class="item-row" class:off={!u.active}>
          <div class="grow">
            <div class="title">{u.name} <span class="badge muted">{ROLES[u.role]}</span>{#if !u.active} <span class="badge bad">Inactivo</span>{/if}</div>
            <div class="meta">{u.email}</div>
          </div>
          {#if write}
            <button class="btn ghost sm" onclick={() => ((resetFor = resetFor === u.id ? null : u.id), (newPassword = ''))}>Restablecer contraseña</button>
            <button class="btn ghost sm" onclick={() => toggleUser(u)}>{u.active ? 'Desactivar' : 'Reactivar'}</button>
          {/if}
          {#if resetFor === u.id}
            <form class="reset" onsubmit={(e) => resetPassword(e, u)}>
              <label class="sr-only" for="pw-{u.id}">Contraseña nueva</label>
              <input id="pw-{u.id}" type="password" bind:value={newPassword} minlength="8" maxlength="72" placeholder="Contraseña nueva (8+ caracteres)" autocomplete="new-password" required />
              <button class="btn sm">Guardar</button>
            </form>
          {/if}
        </div>
      {/each}
    </div>
  </section>

  <section>
    <h2>Actividad reciente</h2>
    <div class="list">
      {#each activity as a (a.id)}
        <div class="item-row"><span class="grow">{ACTIONS[a.action] ?? a.action}</span><span class="meta">{a.actor} · {formatDate(a.created_at)}</span></div>
      {:else}
        <div class="empty">Sin actividad registrada.</div>
      {/each}
    </div>
  </section>
{/if}

<style>
  .back { display: inline-flex; align-items: center; gap: 6px; margin-bottom: 1rem; color: var(--muted); text-decoration: none; font-size: 0.9rem; }
  .back :global(svg) { transform: rotate(180deg); }
  .back:hover { color: var(--accent); }
  section { margin-top: 2rem; }
  section h2 { font-size: 1.4rem; }
  .card h2 { margin-bottom: 0.3rem; }
  code { background: var(--surface-2); padding: 1px 6px; border-radius: 4px; font-size: 0.85em; }
  .trial { display: flex; flex-wrap: wrap; align-items: end; gap: 12px; margin-top: 1.25rem; }
  .reset { display: flex; gap: 8px; flex: 1 1 100%; }
  .reset input { margin: 0; }
</style>
