<script>
  import { onMount } from 'svelte';
  import Avatar from '$lib/Avatar.svelte';
  import Modal from '$lib/Modal.svelte';
  import { api } from '$lib/api.js';
  import { PLATFORM_ROLES, cleanUsername, plural } from '$lib/format.js';
  import Timeline from '$lib/platform/Timeline.svelte';
  import { canWritePlatform, session } from '$lib/session.svelte.js';

  const write = canWritePlatform();

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

<div class="page-head">
  <div>
    <h1>Equipo</h1>
    <p>{plural(team.length, 'persona con acceso', 'personas con acceso')}</p>
  </div>
  {#if write}<button class="btn" onclick={() => (showForm = true)}>Agregar persona</button>{/if}
</div>

{#if flash}<p class="banner" role="status">{flash}</p>{/if}
{#if error}<p class="error" role="alert">{error}</p>{/if}

<section class="card" class:busy={loading}>
  <h2>Quién tiene acceso</h2>
  <p class="meta"><b>Administrador</b> ve el dinero, los planes y al equipo. <b>Soporte</b> solo consulta negocios.</p>
  <div class="tbl-wrap">
    <table class="tbl stack">
      <thead>
        <tr><th scope="col">Persona</th><th scope="col">Perfil</th><th scope="col">Dos pasos</th><th scope="col"><span class="sr-only">Acciones</span></th></tr>
      </thead>
      <tbody>
        {#each team as m (m.id)}
          {@const me = m.id === session.me.id}
          <tr class:off={!m.active}>
            <td>
              <div class="who">
                <Avatar name={m.name} />
                <span>
                  <b>{m.name}{#if me} <span class="badge">Tú</span>{/if}{#if !m.active} <span class="badge muted">Inactivo</span>{/if}</b>
                  <small class="meta">{[m.email, m.username && `@${m.username}`].filter(Boolean).join(' · ')}</small>
                </span>
              </div>
            </td>
            <td data-label="Perfil">
              {#if write && !me}
                <label class="sr-only" for="r-{m.id}">Perfil de {m.name}</label>
                <select id="r-{m.id}" value={m.role} onchange={(e) => patch(m, { role: e.currentTarget.value })}>
                  {#each Object.entries(PLATFORM_ROLES) as [v, l]}<option value={v}>{l}</option>{/each}
                </select>
              {:else}
                <span class="badge {m.role === 'platform_admin' ? '' : 'muted'}">{PLATFORM_ROLES[m.role]}</span>
              {/if}
            </td>
            <td data-label="Dos pasos">{#if m.two_factor}<span class="badge">Activa</span>{:else}<span class="badge warn" title="Todavía no la activa">Pendiente</span>{/if}</td>
            <td>
              {#if write && !me}
                <div class="acts">
                  {#if m.two_factor}<button class="btn ghost sm" onclick={() => resetTwoFactor(m)}>Quitar 2 pasos</button>{/if}
                  <button class="btn ghost sm" onclick={() => patch(m, { active: !m.active })}>{m.active ? 'Desactivar' : 'Reactivar'}</button>
                </div>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <p class="meta hint"><b>Quitar 2 pasos</b> es para quien perdió su celular y sus códigos de respaldo: se le cierran las sesiones y la próxima vez que entre la activa de nuevo.</p>
</section>

<section class="card movs">
  <h2>Movimientos del equipo</h2>
  <p class="meta">Lo que se ha cambiado en negocios, pagos, gastos y accesos.</p>
  <Timeline items={activity} />
</section>

<Modal bind:open={showForm} title="Nueva persona del equipo">
  <form onsubmit={add}>
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required minlength="2" autocomplete="off" /></label>
      <label class="field">Perfil
        <select bind:value={form.role}>{#each Object.entries(PLATFORM_ROLES) as [v, l]}<option value={v}>{l}</option>{/each}</select>
      </label>
      <label class="field">Correo <input type="email" bind:value={form.email} required autocomplete="off" autocapitalize="none" spellcheck="false" /></label>
      <label class="field">Usuario <span class="meta">(opcional)</span>
        <input value={form.username} oninput={(e) => (form.username = cleanUsername(e.currentTarget.value))} maxlength="30" placeholder="Se genera desde el correo" autocomplete="off" autocapitalize="none" autocorrect="off" spellcheck="false" />
      </label>
    </div>
    <label class="field">Contraseña inicial
      <input type="password" bind:value={form.password} minlength="8" maxlength="72" autocomplete="new-password" required />
      <span class="hint">Mínimo 8 caracteres; una frase sirve.</span>
    </label>
    <p class="meta"><b>Soporte</b> consulta negocios. <b>Administrador</b> además cambia planes, ve el dinero y maneja al equipo.</p>
    <div class="form-actions"><button class="btn" disabled={busy}>{busy ? 'Creando…' : 'Crear'}</button></div>
  </form>
</Modal>

<style>
  h2 { font-size: 1.2rem; margin-bottom: 0.2rem; }
  .meta { color: var(--muted); font-size: 0.88rem; }
  .busy { opacity: 0.6; }
  .movs { margin-top: 1.25rem; }
  .who { display: flex; align-items: center; gap: 12px; }
  .who > span { display: grid; line-height: 1.3; min-width: 0; }
  .who small { overflow: hidden; text-overflow: ellipsis; }
  .off { opacity: 0.6; }
  .tbl select { width: auto; min-height: 38px; margin: 0; }
  .acts { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
  .hint { margin: 1rem 0 0; }
</style>
