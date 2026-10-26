<script>
  import { onMount } from 'svelte';
  import Modal from '$lib/Modal.svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { money, plural, toCents, toPesos } from '$lib/format.js';
  import { hasPos, session } from '$lib/session.svelte.js';

  const admin = session.me.role === 'admin';
  // los paquetes son de cobros: sin ese plan ni se piden ni se muestran
  const sells = ['admin', 'cashier', 'reception'].includes(session.me.role) && hasPos();

  let services = $state([]);
  let categories = $state([]);
  let team = $state([]);
  let loading = $state(true);
  let error = $state('');
  let busy = $state(false);

  // catálogo: filtros, orden y vista
  let catFilter = $state('all'); // all | none | id
  let status = $state('all'); // all | active | off
  let query = $state('');
  let sort = $state('name');
  let view = $state('list');

  // selección múltiple
  let selecting = $state(false);
  let selected = $state(new Set());

  // alta y edición
  let formOpen = $state(false);
  let editing = $state(null); // null = alta, número = id
  let form = $state(blank());

  // categorías
  let catsOpen = $state(false);
  let newCat = $state('');
  let drafts = $state({});
  let delCat = $state(null);
  let moveTo = $state('');
  let catError = $state('');

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
    return { name: '', duration_min: 30, buffer_min: 0, price: '', staff_ids: [], category_id: '' };
  }

  async function load() {
    [services, categories] = await Promise.all([api('/api/services'), api('/api/service-categories')]);
    if (sells) await loadPackages();
    if (admin) team = (await api('/api/team')).filter((m) => m.active && m.bookable);
    loading = false;
  }
  onMount(() => load().catch((e) => ((error = e.message), (loading = false))));

  // ── cálculos
  const catName = (id) => categories.find((c) => c.id === id)?.name ?? 'Sin categoría';
  const staffNames = (s) => team.filter((m) => s.staff_ids.includes(m.id)).map((m) => m.name).join(', ');
  const active = $derived(services.filter((s) => s.active));
  const avg = (list, key) => (list.length ? Math.round(list.reduce((n, s) => n + s[key], 0) / list.length) : 0);
  const noCat = $derived(services.filter((s) => s.category_id == null).length);
  const rows = $derived(
    services
      .filter((s) => (catFilter === 'all' ? true : catFilter === 'none' ? s.category_id == null : s.category_id === catFilter))
      .filter((s) => (status === 'all' ? true : status === 'active' ? s.active : !s.active))
      .filter((s) => !query.trim() || s.name.toLowerCase().includes(query.trim().toLowerCase()))
      .sort((a, b) => (sort === 'price-desc' ? b.price_cents - a.price_cents : sort === 'price-asc' ? a.price_cents - b.price_cents : sort === 'duration' ? a.duration_min - b.duration_min : 0) || a.name.localeCompare(b.name, 'es'))
  );
  const allSelected = $derived(rows.length > 0 && rows.every((s) => selected.has(s.id)));
  const duration = (s) => `${s.duration_min} min${s.buffer_min ? ` + ${s.buffer_min}` : ''}`;
  const resetFilters = () => ((catFilter = 'all'), (status = 'all'), (query = ''));

  // ── alta y edición
  function startNew() {
    form = { ...blank(), category_id: typeof catFilter === 'number' ? catFilter : '' };
    editing = null;
    error = '';
    formOpen = true;
  }
  function startEdit(s) {
    if (!admin || selecting) return;
    form = { name: s.name, duration_min: s.duration_min, buffer_min: s.buffer_min, price: toPesos(s.price_cents), staff_ids: [...s.staff_ids], category_id: s.category_id ?? '' };
    editing = s.id;
    error = '';
    formOpen = true;
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
      const body = { name: form.name, duration_min: Number(form.duration_min), buffer_min: Number(form.buffer_min), price_cents, category_id: Number(form.category_id) || 0 };
      let id = editing;
      if (editing === null) id = (await api('/api/services', { method: 'POST', body })).id;
      else await api(`/api/services/${editing}`, { method: 'PATCH', body });
      await api(`/api/services/${id}/staff`, { method: 'PUT', body: { user_ids: form.staff_ids } });
      formOpen = false;
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

  // ── selección múltiple: activar, desactivar o mover de categoría
  function toggleSelecting() {
    selecting = !selecting;
    selected = new Set();
  }
  function pick(id) {
    const next = new Set(selected);
    next.has(id) ? next.delete(id) : next.add(id);
    selected = next;
  }
  const pickAll = () => (selected = allSelected ? new Set() : new Set(rows.map((s) => s.id)));
  async function bulk(body) {
    error = '';
    busy = true;
    try {
      await Promise.all([...selected].map((id) => api(`/api/services/${id}`, { method: 'PATCH', body })));
      selected = new Set();
      await load();
    } catch (err) {
      error = err.message;
      await load().catch(() => {});
    } finally {
      busy = false;
    }
  }

  // ── exportar el catálogo
  function exportCsv() {
    const cell = (v) => `"${String(v ?? '').replace(/"/g, '""')}"`;
    const head = ['Servicio', 'Categoría', 'Duración (min)', 'Preparación (min)', 'Precio', 'Estado', 'Personal'];
    const lines = [head, ...rows.map((s) => [s.name, catName(s.category_id), s.duration_min, s.buffer_min, toPesos(s.price_cents), s.active ? 'Activo' : 'Inactivo', staffNames(s)])];
    const url = URL.createObjectURL(new Blob(['﻿' + lines.map((l) => l.map(cell).join(',')).join('\r\n')], { type: 'text/csv;charset=utf-8' }));
    const a = Object.assign(document.createElement('a'), { href: url, download: 'servicios.csv' });
    a.click();
    URL.revokeObjectURL(url);
  }

  // ── categorías
  function openCats() {
    drafts = Object.fromEntries(categories.map((c) => [c.id, c.name]));
    delCat = null;
    catError = '';
    newCat = '';
    catsOpen = true;
  }
  async function addCat(e) {
    e.preventDefault();
    catError = '';
    try {
      const { id } = await api('/api/service-categories', { method: 'POST', body: { name: newCat } });
      newCat = '';
      await load();
      drafts = { ...drafts, [id]: categories.find((c) => c.id === id)?.name ?? '' };
    } catch (err) {
      catError = err.message;
    }
  }
  async function rename(c) {
    const name = (drafts[c.id] ?? '').trim();
    if (!name || name === c.name) return (drafts[c.id] = c.name);
    catError = '';
    try {
      await api(`/api/service-categories/${c.id}`, { method: 'PATCH', body: { name } });
      await load();
    } catch (err) {
      catError = err.message;
      drafts[c.id] = c.name;
    }
  }
  function askDelete(c) {
    delCat = c;
    moveTo = '';
    catError = '';
  }
  async function confirmDelete() {
    catError = '';
    try {
      await api(`/api/service-categories/${delCat.id}${moveTo ? `?move_to=${moveTo}` : ''}`, { method: 'DELETE' });
      if (catFilter === delCat.id) catFilter = 'all';
      delCat = null;
      await load();
      drafts = Object.fromEntries(categories.map((c) => [c.id, c.name]));
    } catch (err) {
      catError = err.message;
    }
  }
</script>

<div class="page-head">
  <div>
    <h1>Servicios</h1>
    <p>{plural(active.length, 'servicio activo', 'servicios activos')} · {plural(categories.length, 'categoría', 'categorías')}</p>
  </div>
  <div class="head-acts">
    {#if services.length}<button class="btn ghost" onclick={exportCsv}>Exportar CSV</button>{/if}
    {#if admin && services.length}<button class="btn ghost" onclick={toggleSelecting}>{selecting ? 'Cancelar selección' : 'Seleccionar'}</button>{/if}
    {#if admin && !selecting}<button class="btn" onclick={startNew}>Nuevo servicio</button>{/if}
  </div>
</div>

{#if error && !formOpen}<p class="error" role="alert">{error}</p>{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else if services.length === 0}
  <div class="empty">Todavía no hay servicios.{#if admin} Crea el primero con «Nuevo servicio».{/if}</div>
{:else}
  <div class="kpis inv" role="group" aria-label="Resumen del catálogo">
    <div class="kpi"><span class="lbl">Servicios activos</span><span class="v">{active.length}</span><span class="sub">{services.length - active.length} inactivos</span></div>
    <div class="kpi"><span class="lbl">Precio promedio</span><span class="v">{money(avg(active, 'price_cents'))}</span><span class="sub">de los activos</span></div>
    <div class="kpi"><span class="lbl">Duración promedio</span><span class="v">{avg(active, 'duration_min')} min</span><span class="sub">sin preparación</span></div>
    <button class="kpi pick" class:on={catFilter === 'none'} aria-pressed={catFilter === 'none'} onclick={() => (catFilter = catFilter === 'none' ? 'all' : 'none')}>
      <span class="lbl">Sin categoría</span><span class="v">{noCat}</span><span class="sub">toca para verlos</span>
    </button>
  </div>

  <div class="cat-body">
    <aside class="rail" aria-label="Categorías">
      <button class="rail-item" class:on={catFilter === 'all'} onclick={() => (catFilter = 'all')}><span>Todas</span><em>{services.length}</em></button>
      {#each categories as c (c.id)}
        <button class="rail-item" class:on={catFilter === c.id} onclick={() => (catFilter = c.id)}><span>{c.name}</span><em>{c.count}</em></button>
      {/each}
      {#if noCat && categories.length}
        <button class="rail-item" class:on={catFilter === 'none'} onclick={() => (catFilter = 'none')}><span>Sin categoría</span><em>{noCat}</em></button>
      {/if}
      {#if admin}
        <div class="rail-acts">
          <button class="chip" onclick={openCats}>+ Categoría</button>
          {#if categories.length}<button class="link" onclick={openCats}>Editar</button>{/if}
        </div>
      {/if}
    </aside>

    <section class="main" aria-label="Catálogo de servicios">
      <div class="tools">
        <label class="search">
          <Icon name="search" size={17} />
          <input type="search" placeholder="Buscar servicio" aria-label="Buscar servicio" autocomplete="off" bind:value={query} />
        </label>
        <select bind:value={sort} aria-label="Ordenar">
          <option value="name">Nombre A–Z</option>
          <option value="price-desc">Precio mayor</option>
          <option value="price-asc">Precio menor</option>
          <option value="duration">Menor duración</option>
        </select>
        <div class="seg" role="group" aria-label="Vista">
          <button aria-pressed={view === 'list'} title="Lista" onclick={() => (view = 'list')}>Lista</button>
          <button aria-pressed={view === 'grid'} title="Mosaico" onclick={() => (view = 'grid')}>Mosaico</button>
        </div>
      </div>
      <div class="chips" style="margin: 0 0 0.75rem">
        {#each [['all', 'Todos', services.length], ['active', 'Activos', active.length], ['off', 'Inactivos', services.length - active.length]] as [id, label, n]}
          <button class="chip" aria-pressed={status === id} onclick={() => (status = id)}>{label} <small>{n}</small></button>
        {/each}
      </div>

      {#if selecting}
        <div class="bar" role="group" aria-label="Acciones para los seleccionados">
          <label class="check"><input type="checkbox" checked={allSelected} onchange={pickAll} /> {selected.size ? plural(selected.size, 'seleccionado', 'seleccionados') : 'Seleccionar todos'}</label>
          <button class="btn sm" disabled={!selected.size || busy} onclick={() => bulk({ active: true })}>Activar</button>
          <button class="btn ghost sm" disabled={!selected.size || busy} onclick={() => bulk({ active: false })}>Desactivar</button>
          <select aria-label="Mover a una categoría" disabled={!selected.size || busy} value="" onchange={(e) => { const v = e.currentTarget.value; e.currentTarget.value = ''; if (v !== '') bulk({ category_id: Number(v) }); }}>
            <option value="" disabled>Mover a…</option>
            <option value="0">Sin categoría</option>
            {#each categories as c (c.id)}<option value={c.id}>{c.name}</option>{/each}
          </select>
        </div>
      {/if}

      {#if !rows.length}
        <div class="empty">Nada coincide con los filtros. <button class="link" onclick={resetFilters}>Quitar filtros</button></div>
      {:else if view === 'list'}
        <div class="tbl-wrap">
          <table class="tbl stack">
            <thead>
              <tr>
                {#if selecting}<th></th>{/if}
                <th>Servicio</th><th>Categoría</th><th class="num">Duración</th><th class="num">Precio</th>
              </tr>
            </thead>
            <tbody>
              {#each rows as s (s.id)}
                <tr class:off={!s.active} class:click={admin} onclick={() => (selecting ? pick(s.id) : startEdit(s))}>
                  {#if selecting}<td class="c-chk" onclick={(e) => e.stopPropagation()}><input type="checkbox" checked={selected.has(s.id)} onchange={() => pick(s.id)} aria-label={`Seleccionar ${s.name}`} /></td>{/if}
                  <td class="c-name" data-label="Servicio">
                    <strong>{s.name}</strong>{#if !s.active} <span class="badge muted">Inactivo</span>{/if}
                    {#if admin && staffNames(s)}<small class="meta">{staffNames(s)}</small>{/if}
                  </td>
                  <td class="c-cat" data-label="Categoría">{catName(s.category_id)}</td>
                  <td class="num c-dur" data-label="Duración">{duration(s)}</td>
                  <td class="num c-price" data-label="Precio"><strong>{money(s.price_cents)}</strong></td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else}
        <div class="tiles">
          {#each rows as s (s.id)}
            <button class="tile" class:off={!s.active} class:in={selected.has(s.id)} onclick={() => (selecting ? pick(s.id) : startEdit(s))}>
              <span class="avatar" aria-hidden="true">{s.name.trim().charAt(0).toUpperCase()}</span>
              <b>{s.name}</b>
              <span class="foot"><strong>{money(s.price_cents)}</strong><small>{duration(s)} · {catName(s.category_id)}</small></span>
            </button>
          {/each}
        </div>
      {/if}
    </section>
  </div>
{/if}

<Modal bind:open={formOpen} title={editing === null ? 'Nuevo servicio' : 'Editar servicio'}>
  <form onsubmit={save}>
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Categoría
        <select bind:value={form.category_id}>
          <option value="">Sin categoría</option>
          {#each categories as c (c.id)}<option value={c.id}>{c.name}</option>{/each}
        </select>
      </label>
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
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      <button type="button" class="btn ghost" onclick={() => (formOpen = false)}>Cancelar</button>
      {#if editing !== null}
        {@const cur = services.find((x) => x.id === editing)}
        {#if cur}<button type="button" class="btn ghost" onclick={() => { toggle(cur); formOpen = false; }}>{cur.active ? 'Desactivar' : 'Activar'}</button>{/if}
      {/if}
    </div>
  </form>
</Modal>

<Modal bind:open={catsOpen} title="Categorías">
  <p class="meta">Cambia el nombre o elimina. Al eliminar, sus servicios pasan a otra categoría o quedan sin categoría.</p>
  <form class="add-cat" onsubmit={addCat}>
    <input bind:value={newCat} maxlength="40" placeholder="Nueva categoría (Cabello, Barba, Uñas…)" aria-label="Nueva categoría" required />
    <button class="btn sm">Agregar</button>
  </form>
  <ul class="cat-list">
    {#each categories as c (c.id)}
      <li>
        <input bind:value={drafts[c.id]} maxlength="40" aria-label={`Nombre de ${c.name}`} onblur={() => rename(c)} onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), e.currentTarget.blur())} />
        <span class="meta">{plural(c.count, 'servicio', 'servicios')}</span>
        <button class="btn ghost sm" aria-label={`Eliminar ${c.name}`} onclick={() => askDelete(c)}><Icon name="trash" size={16} /></button>
      </li>
    {:else}
      <li class="meta">Aún no hay categorías.</li>
    {/each}
  </ul>
  {#if delCat}
    <div class="del" role="alert">
      <p><strong>Eliminar «{delCat.name}».</strong>{delCat.count ? ` Sus ${plural(delCat.count, 'servicio', 'servicios')} pasarán a:` : ''}</p>
      {#if delCat.count}
        <select bind:value={moveTo} aria-label="Pasar sus servicios a">
          <option value="">Sin categoría</option>
          {#each categories.filter((c) => c.id !== delCat.id) as c (c.id)}<option value={c.id}>{c.name}</option>{/each}
        </select>
      {/if}
      <div class="form-actions" style="margin-top: 0.5rem">
        <button class="btn sm" onclick={confirmDelete}>Eliminar categoría</button>
        <button class="btn ghost sm" onclick={() => (delCat = null)}>Cancelar</button>
      </div>
    </div>
  {/if}
  {#if catError}<p class="error" role="alert">{catError}</p>{/if}
</Modal>

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
  .head-acts { display: flex; flex-wrap: wrap; gap: 8px; }
  h3 { font-size: 1.25rem; }
  .staff { border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 0.75rem 1rem; margin: 0 0 1rem; display: flex; flex-wrap: wrap; gap: 4px 20px; }
  .staff legend { padding: 0 6px; font-size: 0.9rem; font-weight: 500; }

  /* tarjetas de cifras (la última filtra) */
  .kpis.inv { grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); }
  .kpi > span { display: block; }
  .kpi.pick { text-align: left; font: inherit; cursor: pointer; transition: border-color var(--dur-ui) var(--ease), background-color var(--dur-ui) var(--ease); }
  .kpi.pick:hover { border-color: var(--muted); }
  .kpi.pick.on { border-color: var(--accent); background: var(--accent-soft); }

  /* categorías a la izquierda, catálogo a la derecha */
  .cat-body { display: grid; grid-template-columns: 210px minmax(0, 1fr); gap: 20px; align-items: start; }
  .rail { position: sticky; top: 16px; display: grid; gap: 4px; }
  .rail-item { display: flex; justify-content: space-between; align-items: center; gap: 8px; min-height: 40px; padding: 0 0.75rem; text-align: left; font: inherit; color: var(--text); background: transparent; border: 1px solid transparent; border-radius: var(--radius-sm); cursor: pointer; }
  .rail-item span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .rail-item em { font-style: normal; font-size: 0.8rem; color: var(--muted); font-variant-numeric: tabular-nums; }
  .rail-item:hover { background: var(--surface-2); }
  .rail-item.on { background: var(--accent-soft); border-color: var(--accent); color: var(--accent); font-weight: 600; }
  .rail-acts { display: flex; align-items: center; gap: 12px; padding: 0.5rem 0.25rem; }
  .link { padding: 0; font: inherit; font-size: 0.85rem; color: var(--accent); background: none; border: 0; text-decoration: underline; cursor: pointer; }

  .tools { display: flex; flex-wrap: wrap; gap: 10px; margin-bottom: 0.75rem; }
  .search { display: flex; flex: 1 1 200px; align-items: center; gap: 8px; padding: 0 0.75rem; color: var(--muted); background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .search:focus-within { border-color: var(--accent); }
  .search input[type='search'], .search input[type='search']:hover, .search input[type='search']:focus { flex: 1; min-width: 0; min-height: 44px; margin: 0; padding: 0; border: 0; outline: 0; box-shadow: none; background: transparent; }
  .tools select { width: auto; min-width: 150px; margin: 0; }
  .seg { display: inline-flex; padding: 3px; border: 1px solid var(--border); border-radius: 999px; background: var(--surface-2); }
  .seg button { min-height: 36px; padding: 0 0.9rem; font: inherit; font-size: 0.85rem; font-weight: 600; color: var(--muted); background: transparent; border: 0; border-radius: 999px; cursor: pointer; }
  .seg button[aria-pressed='true'] { background: var(--accent); color: var(--accent-contrast); }
  .chip small { margin-left: 4px; opacity: 0.7; }

  .bar { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-bottom: 0.75rem; padding: 0.6rem 0.9rem; background: var(--accent-soft); border: 1px solid var(--accent); border-radius: var(--radius-sm); }
  .bar .check { flex: 1 1 160px; margin: 0; }
  .bar select { width: auto; margin: 0; min-height: 36px; }

  .tbl-wrap { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  .tbl tr.click { cursor: pointer; }
  .tbl tr.click:hover td { background: var(--surface-2); }
  .tbl tr.off { opacity: 0.6; }
  .tbl td small { display: block; }
  /* en celular cada servicio ocupa dos líneas: nombre y precio; categoría y duración */
  @media (max-width: 640px) {
    .tbl.stack tr { display: grid; grid-template-columns: minmax(0, 1fr) auto; column-gap: 12px; align-items: baseline; padding: 0.7rem 0.9rem; }
    .tbl.stack td[data-label]::before { display: none; }
    .c-name { grid-column: 1; grid-row: 1; }
    .c-price { grid-column: 2; grid-row: 1; }
    .c-cat { grid-column: 1; grid-row: 2; color: var(--muted); font-size: 0.85rem; }
    .c-dur { grid-column: 2; grid-row: 2; color: var(--muted); font-size: 0.85rem; }
    .c-chk { grid-column: 1 / -1; grid-row: 3; }
  }
  .tiles { display: grid; gap: 10px; grid-template-columns: repeat(auto-fill, minmax(170px, 1fr)); }
  .tile { display: grid; grid-template-rows: auto 1fr auto; justify-items: start; gap: 6px; min-height: 120px; padding: 0.75rem; text-align: left; font: inherit; color: var(--text); background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); cursor: pointer; }
  .tile:hover { border-color: var(--accent); }
  .tile.in { border-color: var(--accent); background: var(--accent-soft); }
  .tile.off { opacity: 0.6; }
  .tile .foot { display: grid; gap: 1px; }
  .tile strong { color: var(--accent); font-variant-numeric: tabular-nums; }
  .tile small { color: var(--muted); }
  .avatar { display: grid; place-items: center; width: 36px; height: 36px; border-radius: 50%; background: var(--accent-soft); color: var(--accent); font-weight: 700; }

  .add-cat { display: flex; gap: 8px; margin: 0.75rem 0; }
  .add-cat input { flex: 1; margin: 0; }
  .cat-list { list-style: none; margin: 0 0 0.75rem; padding: 0; display: grid; gap: 8px; }
  .cat-list li { display: flex; align-items: center; gap: 10px; }
  .cat-list input { flex: 1; margin: 0; }
  .del { margin: 0.75rem 0; padding: 0.75rem 1rem; background: var(--danger-soft); border-radius: var(--radius-sm); }
  .del p { margin: 0 0 0.5rem; }
  .del select { margin: 0; }

  @media (max-width: 760px) {
    .cat-body { grid-template-columns: 1fr; }
    .rail { position: static; display: flex; flex-wrap: wrap; gap: 6px; }
    .rail-item { min-height: 36px; border: 1px solid var(--border); border-radius: 999px; }
    .rail-acts { padding: 0; }
  }
</style>
