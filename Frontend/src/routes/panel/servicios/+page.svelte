<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { money, toCents, toPesos } from '$lib/format.js';
  import { hasPos, session } from '$lib/session.svelte.js';

  const admin = session.me.role === 'admin';
  // los paquetes son de cobros: sin ese plan ni se piden ni se muestran
  const sells = ['admin', 'cashier', 'reception'].includes(session.me.role) && hasPos();

  let services = $state([]);
  let team = $state([]);
  let loading = $state(true);
  let error = $state('');
  let editing = $state(null); // null = cerrado, 'new' = alta, número = id
  let busy = $state(false);
  let form = $state(blank());

  // paquetes: varias sesiones de un servicio con descuento
  let packages = $state([]);
  let pkgForm = $state(null); // null = cerrado, 'new' o id
  let pkg = $state(blankPkg());
  function blankPkg() {
    return { name: '', service_id: '', sessions: 5, price: '', valid_days: '' };
  }
  async function loadPackages() {
    packages = await api('/api/packages' + (admin ? '?all=1' : ''));
  }
  function startPkg(k) {
    pkg = k ? { name: k.name, service_id: k.service_id, sessions: k.sessions, price: toPesos(k.price_cents), valid_days: k.valid_days ?? '' } : blankPkg();
    pkgForm = k ? k.id : 'new';
  }
  async function savePkg(e) {
    e.preventDefault();
    error = '';
    const price_cents = toCents(pkg.price);
    if (Number.isNaN(price_cents)) return (error = 'Escribe un precio válido, por ejemplo 600');
    const valid_days = pkg.valid_days === '' ? 0 : Number(pkg.valid_days);
    busy = true;
    try {
      if (pkgForm === 'new') {
        await api('/api/packages', { method: 'POST', body: { name: pkg.name, service_id: Number(pkg.service_id), sessions: Number(pkg.sessions), price_cents, valid_days: valid_days || null } });
      } else {
        await api(`/api/packages/${pkgForm}`, { method: 'PATCH', body: { name: pkg.name, price_cents, valid_days } });
      }
      pkgForm = null;
      await loadPackages();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
  async function togglePkg(k) {
    error = '';
    try {
      await api(`/api/packages/${k.id}`, { method: 'PATCH', body: { active: !k.active } });
      await loadPackages();
    } catch (err) {
      error = err.message;
    }
  }

  function blank() {
    return { name: '', duration_min: 30, buffer_min: 0, price: '', staff_ids: [] };
  }

  async function load() {
    services = await api('/api/services');
    if (sells) await loadPackages();
    if (admin) team = (await api('/api/team')).filter((m) => m.active && m.bookable);
    loading = false;
  }
  onMount(() => load().catch((e) => ((error = e.message), (loading = false))));

  const staffNames = (s) => team.filter((m) => s.staff_ids.includes(m.id)).map((m) => m.name).join(', ');

  function startNew() {
    form = blank();
    editing = 'new';
  }
  function startEdit(s) {
    form = { name: s.name, duration_min: s.duration_min, buffer_min: s.buffer_min, price: toPesos(s.price_cents), staff_ids: [...s.staff_ids] };
    editing = s.id;
  }

  async function save(e) {
    e.preventDefault();
    error = '';
    const price_cents = toCents(form.price);
    if (Number.isNaN(price_cents)) {
      error = 'Escribe un precio válido, por ejemplo 150 o 150.50';
      return;
    }
    busy = true;
    try {
      const body = { name: form.name, duration_min: Number(form.duration_min), buffer_min: Number(form.buffer_min), price_cents };
      let id = editing;
      if (editing === 'new') id = (await api('/api/services', { method: 'POST', body })).id;
      else await api(`/api/services/${editing}`, { method: 'PATCH', body });
      await api(`/api/services/${id}/staff`, { method: 'PUT', body: { user_ids: form.staff_ids } });
      editing = null;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function toggle(s) {
    error = '';
    try {
      await api(`/api/services/${s.id}`, { method: 'PATCH', body: { active: !s.active } });
      await load();
    } catch (err) {
      error = err.message;
    }
  }
</script>

<div class="page-head">
  <div>
    <h1>Servicios</h1>
    <p>Lo que ofreces, con su duración y precio.</p>
  </div>
  {#if admin}<button class="btn" onclick={startNew}>Nuevo servicio</button>{/if}
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if editing !== null}
  <form class="card" onsubmit={save} style="margin-bottom: 1.5rem">
    <h3>{editing === 'new' ? 'Nuevo servicio' : 'Editar servicio'}</h3>
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Precio (MXN) <input inputmode="decimal" bind:value={form.price} placeholder="150.00" required /></label>
      <label class="field">Duración (min) <input type="number" min="5" max="480" step="5" bind:value={form.duration_min} required /></label>
      <label class="field">
        Preparación entre citas (min)
        <input type="number" min="0" max="240" step="5" bind:value={form.buffer_min} />
      </label>
    </div>
    <fieldset class="staff">
      <legend>Quién lo realiza</legend>
      {#each team as m (m.id)}
        <label class="check">
          <input type="checkbox" checked={form.staff_ids.includes(m.id)}
            onchange={(e) => (form.staff_ids = e.currentTarget.checked ? [...form.staff_ids, m.id] : form.staff_ids.filter((x) => x !== m.id))} />
          {m.name}
        </label>
      {:else}
        <span class="meta">Aún no hay personas marcadas «En agenda» en Equipo.</span>
      {/each}
    </fieldset>
    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      <button type="button" class="btn ghost" onclick={() => (editing = null)}>Cancelar</button>
    </div>
  </form>
{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else if services.length === 0}
  <div class="empty">Todavía no hay servicios.{#if admin} Crea el primero con «Nuevo servicio».{/if}</div>
{:else}
  <div class="list">
    {#each services as s (s.id)}
      <div class="item-row" class:off={!s.active}>
        <div class="grow">
          <div class="title">{s.name} {#if !s.active}<span class="badge muted">Inactivo</span>{/if}</div>
          <div class="meta">{s.duration_min} min{#if s.buffer_min} + {s.buffer_min} de preparación{/if}{#if admin && staffNames(s)}{' · ' + staffNames(s)}{/if}</div>
        </div>
        <strong>{money(s.price_cents)}</strong>
        {#if admin}
          <button class="btn ghost sm" onclick={() => startEdit(s)}>Editar</button>
          <button class="btn ghost sm" onclick={() => toggle(s)}>{s.active ? 'Desactivar' : 'Activar'}</button>
        {/if}
      </div>
    {/each}
  </div>
{/if}

{#if sells}
<div class="page-head" style="margin-top: 2.5rem">
  <div>
    <h2 style="font-size: 1.6rem">Paquetes</h2>
    <p>Vende varias sesiones de un servicio por adelantado; se canjean una a una al cobrar.</p>
  </div>
  {#if admin && services.some((s) => s.active)}<button class="btn" onclick={() => startPkg(null)}>Nuevo paquete</button>{/if}
</div>

{#if pkgForm !== null}
  <form class="card" onsubmit={savePkg} style="margin-bottom: 1.5rem">
    <h3>{pkgForm === 'new' ? 'Nuevo paquete' : 'Editar paquete'}</h3>
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={pkg.name} required placeholder="5 cortes" /></label>
      <label class="field">
        Servicio
        <select bind:value={pkg.service_id} required disabled={pkgForm !== 'new'}>
          <option value="" disabled>Elige…</option>
          {#each services.filter((s) => s.active || s.id === pkg.service_id) as s (s.id)}<option value={s.id}>{s.name} · {money(s.price_cents)}</option>{/each}
        </select>
      </label>
      <label class="field">Sesiones <input type="number" min="2" max="100" bind:value={pkg.sessions} required disabled={pkgForm !== 'new'} /></label>
      <label class="field">Precio del paquete (MXN) <input inputmode="decimal" bind:value={pkg.price} placeholder="600.00" required /></label>
      <label class="field">Vigencia (días, opcional) <input type="number" min="1" max="3650" bind:value={pkg.valid_days} placeholder="Sin vencimiento" /></label>
    </div>
    {#if pkgForm !== 'new'}<p class="meta">El servicio y las sesiones no se editan; crea otro paquete para otra oferta. Los paquetes ya vendidos no cambian.</p>{/if}
    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      <button type="button" class="btn ghost" onclick={() => (pkgForm = null)}>Cancelar</button>
    </div>
  </form>
{/if}

{#if !loading}
  {#if packages.length === 0}
    <div class="empty">Todavía no hay paquetes.{#if admin} Crea uno con «Nuevo paquete».{/if}</div>
  {:else}
    <div class="list">
      {#each packages as k (k.id)}
        <div class="item-row" class:off={!k.active}>
          <div class="grow">
            <div class="title">{k.name} {#if !k.active}<span class="badge muted">Inactivo</span>{/if}</div>
            <div class="meta">{k.sessions} sesiones de {k.service_name}{k.valid_days ? ` · vigencia de ${k.valid_days} días` : ''}</div>
          </div>
          <strong>{money(k.price_cents)}</strong>
          {#if admin}
            <button class="btn ghost sm" onclick={() => startPkg(k)}>Editar</button>
            <button class="btn ghost sm" onclick={() => togglePkg(k)}>{k.active ? 'Desactivar' : 'Activar'}</button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
{/if}
{/if}

<style>
  .staff { border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 0.75rem 1rem; margin: 0 0 1rem; display: flex; flex-wrap: wrap; gap: 4px 20px; }
  .staff legend { padding: 0 6px; font-size: 0.9rem; font-weight: 500; }
  h3 { font-size: 1.25rem; }
</style>
