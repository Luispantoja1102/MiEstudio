<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import HoursEditor from '$lib/HoursEditor.svelte';
  import { api } from '$lib/api.js';
  import { ROLES, cleanUsername } from '$lib/format.js';
  import { hasPos, planLimit, session } from '$lib/session.svelte.js';

  let team = $state([]);
  let branches = $state([]);
  let hoursFor = $state(null); // id de la persona cuyo horario se edita
  let loading = $state(true);
  let error = $state('');
  let showForm = $state(false);
  let busy = $state(false);
  let form = $state(blank());
  // el plan fija cuántas personas caben en la agenda (null = sin tope): al llegar al tope no se suman más
  const maxStaff = $derived(planLimit('max_staff'));
  const staffFull = $derived(typeof maxStaff === 'number' && team.filter((m) => m.active && m.bookable).length >= maxStaff);

  function blank() {
    return { name: '', email: '', username: '', password: '', role: 'staff', bookable: true, commission_pct: 0 };
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
      await api('/api/team', { method: 'POST', body: { ...form, username: form.username || undefined, bookable: form.bookable && !staffFull, commission_pct: Number(form.commission_pct) } });
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

<div class="page-head">
  <div>
    <h1>Equipo</h1>
    <p>Personas con acceso a tu negocio y lo que pueden hacer.</p>
  </div>
  <button class="btn" onclick={() => (showForm = !showForm)}>{showForm ? 'Cancelar' : 'Agregar persona'}</button>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if showForm}
  <form class="card" onsubmit={add} style="margin-bottom: 1.5rem">
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Correo <input type="email" bind:value={form.email} required autocapitalize="none" spellcheck="false" /></label>
      <label class="field">Usuario <span class="hint">opcional · con él o con su correo podrá entrar</span>
        <input value={form.username} oninput={(e) => (form.username = cleanUsername(e.currentTarget.value))} maxlength="30" placeholder="Se genera desde el correo" autocomplete="off" autocapitalize="none" autocorrect="off" spellcheck="false" />
      </label>
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
      {#if hasPos()}
        <label class="field">
          Comisión (%)
          <input type="number" min="0" max="100" step="0.5" bind:value={form.commission_pct} />
        </label>
      {/if}
    </div>
    <label class="check"><input type="checkbox" checked={form.bookable && !staffFull} disabled={staffFull} onchange={(e) => (form.bookable = e.currentTarget.checked)} />Aparece en la agenda (atiende clientes)</label>
    {#if staffFull}<p class="meta">Tu plan {planLimit('name')} permite {maxStaff} {maxStaff === 1 ? 'persona' : 'personas'} en la agenda. Para sumar más, <a href="/panel/ajustes?s=suscripcion">cambia de plan</a>.</p>{/if}
    <div class="form-actions"><button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Agregar'}</button></div>
  </form>
{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else}
  <div class="list">
    {#each team as m (m.id)}
      {@const me = m.id === session.me.id}
      <div class="item-row" class:off={!m.active}>
        <div class="grow">
          <div class="title">{m.name} {#if me}<span class="badge">Tú</span>{/if}{#if !m.active}<span class="badge muted">Inactivo</span>{/if}</div>
          <div class="meta">{m.username} · {m.email}</div>
        </div>
        <label class="sr-only" for="role-{m.id}">Rol de {m.name}</label>
        <select id="role-{m.id}" value={m.role} disabled={me} onchange={(e) => patch(m, { role: e.currentTarget.value })} style="width: auto; margin: 0">
          {#each Object.entries(ROLES) as [value, label]}<option {value}>{label}</option>{/each}
        </select>
        <label class="check"><input type="checkbox" checked={m.bookable} disabled={!m.bookable && staffFull} onchange={(e) => patch(m, { bookable: e.currentTarget.checked })} />En agenda</label>
        {#if hasPos()}
          <label class="meta" for="com-{m.id}">Comisión %</label>
          <input id="com-{m.id}" type="number" min="0" max="100" step="0.5" value={m.commission_pct} style="width: 84px; margin: 0"
            onchange={(e) => patch(m, { commission_pct: Number(e.currentTarget.value) })} />
        {/if}
        {#if m.bookable && m.active}
          <button class="btn ghost sm" onclick={() => (hoursFor = hoursFor === m.id ? null : m.id)}>Horario</button>
        {/if}
        {#if m.two_factor}<span class="badge" title="Tiene verificación en dos pasos">2 pasos</span>{/if}
        {#if m.two_factor && !me}
          <button class="btn ghost sm" onclick={() => resetTwoFactor(m)}>Quitar 2 pasos</button>
        {/if}
        {#if !me}
          {#if m.active || !m.bookable || !staffFull}<button class="btn ghost sm" onclick={() => patch(m, { active: !m.active })}>{m.active ? 'Desactivar' : 'Reactivar'}</button>{/if}
        {/if}
        {#if hoursFor === m.id}<HoursEditor member={m} {branches} />{/if}
      </div>
    {/each}
  </div>
{/if}
