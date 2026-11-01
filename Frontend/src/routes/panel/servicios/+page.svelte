<script>
  import { onMount } from 'svelte';
  import Modal from '$lib/Modal.svelte';
  import Icon from '$lib/Icon.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import FilterBar from '$lib/ui/FilterBar.svelte';
  import SegmentedControl from '$lib/ui/SegmentedControl.svelte';
  import SearchField from '$lib/ui/SearchField.svelte';
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
    error = '';
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
  const catOptions = $derived([
    { value: 'all', label: `Todas · ${services.length}` },
    ...categories.map((c) => ({ value: c.id, label: `${c.name} · ${c.count}` })),
    ...(noCat && categories.length ? [{ value: 'none', label: `Sin categoría · ${noCat}` }] : [])
  ]);
  const statusOptions = $derived([
    { value: 'all', label: `Todos · ${services.length}` },
    { value: 'active', label: `Activos · ${active.length}` },
    { value: 'off', label: `Inactivos · ${services.length - active.length}` }
  ]);
  const svcCols = $derived([
    { key: 'name', label: 'Servicio' },
    { key: 'cat', label: 'Categoría' },
    { key: 'duration', label: 'Duración + preparación', numeric: true },
    { key: 'price', label: 'Precio', numeric: true },
    { key: 'state', label: 'Estado' },
    ...(admin ? [{ key: 'acts', label: 'Acciones' }] : [])
  ]);
  const pkgCols = [
    { key: 'name', label: 'Paquete' },
    { key: 'sessions', label: 'Sesiones', numeric: true },
    { key: 'valid', label: 'Vigencia' },
    { key: 'price', label: 'Precio', numeric: true },
    { key: 'state', label: 'Estado' },
    ...(admin ? [{ key: 'acts', label: 'Acciones' }] : [])
  ];
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

<PageHeader title="Servicios" description="{plural(active.length, 'servicio activo', 'servicios activos')} · {plural(categories.length, 'categoría', 'categorías')}">
  {#snippet actions()}
    {#if services.length}<button class="btn ghost" onclick={exportCsv}>Exportar CSV</button>{/if}
    {#if admin && services.length}<button class="btn ghost" onclick={toggleSelecting}>{selecting ? 'Cancelar selección' : 'Seleccionar'}</button>{/if}
    {#if admin && !selecting}<button class="btn" onclick={startNew}>Nuevo servicio</button>{/if}
  {/snippet}
</PageHeader>

{#if error && !formOpen && pkgForm === null}
  <div class="notice"><InlineNotice tone="danger" message={error} /></div>
{/if}

{#if loading}
  <Surface padding="flush"><LoadingState /></Surface>
{:else if services.length === 0}
  <Surface padding="flush">
    <EmptyState title="Todavía no hay servicios" description={admin ? 'Crea el primero para poder agendarlo y cobrarlo.' : 'Un administrador debe crear los servicios.'}>
      {#snippet actions()}{#if admin}<button class="btn" onclick={startNew}>Nuevo servicio</button>{/if}{/snippet}
    </EmptyState>
  </Surface>
{:else}
  <div class="metrics" role="group" aria-label="Resumen del catálogo">
    <MetricCard label="Servicios activos" value={active.length} hint="{services.length - active.length} inactivos" />
    <MetricCard label="Precio promedio" value={money(avg(active, 'price_cents'))} hint="de los activos" />
    <MetricCard label="Duración promedio" value="{avg(active, 'duration_min')} min" hint="sin preparación" />
    <button class="metric-btn" class:on={catFilter === 'none'} aria-pressed={catFilter === 'none'} onclick={() => (catFilter = catFilter === 'none' ? 'all' : 'none')}>
      <MetricCard label="Sin categoría" value={noCat} hint="toca para verlos" />
    </button>
  </div>

  <div class="cats">
    <SegmentedControl bind:value={catFilter} options={catOptions} label="Categorías" />
    {#if admin}
      <span class="cat-acts">
        <button class="btn ghost sm" onclick={openCats}>{categories.length ? 'Editar categorías' : '+ Categoría'}</button>
        {#if categories.length}<button class="btn ghost sm" onclick={openCats}>+ Categoría</button>{/if}
      </span>
    {/if}
  </div>

  <FilterBar label="Filtros del catálogo">
    <div class="srch"><SearchField bind:value={query} label="Buscar servicio" placeholder="Buscar servicio" /></div>
    <SegmentedControl bind:value={status} options={statusOptions} label="Estado" />
    <label class="sel">
      <span class="sr-only">Ordenar</span>
      <select bind:value={sort}>
        <option value="name">Nombre A–Z</option>
        <option value="price-desc">Precio mayor</option>
        <option value="price-asc">Precio menor</option>
        <option value="duration">Menor duración</option>
      </select>
    </label>
    <SegmentedControl bind:value={view} options={[{ value: 'list', label: 'Lista' }, { value: 'grid', label: 'Mosaico' }]} label="Vista" />
  </FilterBar>

  {#if selecting}
    <div class="bulk" role="group" aria-label="Acciones para los seleccionados">
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
    <Surface padding="flush">
      <EmptyState title="Nada coincide con los filtros" description="Prueba con otra categoría, estado o búsqueda.">
        {#snippet actions()}<button class="btn ghost" onclick={resetFilters}>Quitar filtros</button>{/snippet}
      </EmptyState>
    </Surface>
  {:else if view === 'list'}
    <div class="tight"><Surface padding="flush">
      <DataTable caption="Servicios" columns={svcCols} {rows}>
        {#snippet cell({ row: s, column })}
          {#if column.key === 'name'}
            {#if selecting}
              <label class="check pickrow"><input type="checkbox" checked={selected.has(s.id)} onchange={() => pick(s.id)} aria-label={`Seleccionar ${s.name}`} /><span class="name" class:off={!s.active}>{s.name}</span></label>
            {:else}
              <span class="name" class:off={!s.active}>{s.name}</span>
            {/if}
            {#if admin && staffNames(s)}<span class="sub">{staffNames(s)}</span>{/if}
          {:else if column.key === 'cat'}<span class="nc" class:muted={s.category_id == null}>{catName(s.category_id)}</span>
          {:else if column.key === 'duration'}{duration(s)}
          {:else if column.key === 'price'}<strong>{money(s.price_cents)}</strong>
          {:else if column.key === 'state'}<StatusBadge label={s.active ? 'Activo' : 'Inactivo'} tone={s.active ? 'success' : 'neutral'} />
          {:else}
            <span class="acts">
              <button class="btn ghost sm" disabled={selecting} onclick={() => startEdit(s)}>Editar</button>
              <button class="btn ghost sm" disabled={selecting} onclick={() => toggle(s)}>{s.active ? 'Desactivar' : 'Activar'}</button>
            </span>
          {/if}
        {/snippet}
      </DataTable>
    </Surface></div>
  {:else}
    <div class="tiles">
      {#each rows as s (s.id)}
        <button class="tile" class:off={!s.active} class:in={selected.has(s.id)} disabled={!admin && !selecting} onclick={() => (selecting ? pick(s.id) : startEdit(s))}>
          <b>{s.name}</b>
          <span class="foot"><strong>{money(s.price_cents)}</strong><small>{duration(s)} · {catName(s.category_id)}</small></span>
          {#if !s.active}<StatusBadge label="Inactivo" tone="neutral" />{/if}
        </button>
      {/each}
    </div>
  {/if}
{/if}

{#if sells}
  <div class="gap"></div>
  <div class="tight"><Surface padding="flush" title="Paquetes" description="Vende varias sesiones de un servicio por adelantado; se canjean una a una al cobrar.">
    {#snippet actions()}
      {#if admin && services.some((s) => s.active)}<button class="btn" onclick={() => startPkg(null)}>Nuevo paquete</button>{/if}
    {/snippet}
    {#if loading}
      <LoadingState />
    {:else if packages.length === 0}
      <EmptyState title="Todavía no hay paquetes" description={admin ? 'Crea uno con «Nuevo paquete».' : ''} />
    {:else}
      <DataTable caption="Paquetes" columns={pkgCols} rows={packages}>
        {#snippet cell({ row: k, column })}
          {#if column.key === 'name'}
            <span class="name" class:off={!k.active}>{k.name}</span>
            <span class="sub">{k.sessions} sesiones de {k.service_name}</span>
          {:else if column.key === 'sessions'}{k.sessions}
          {:else if column.key === 'valid'}{k.valid_days ? `${k.valid_days} días` : 'Sin vencimiento'}
          {:else if column.key === 'price'}<strong>{money(k.price_cents)}</strong>
          {:else if column.key === 'state'}<StatusBadge label={k.active ? 'Activo' : 'Inactivo'} tone={k.active ? 'success' : 'neutral'} />
          {:else}
            <span class="acts">
              <button class="btn ghost sm" onclick={() => startPkg(k)}>Editar</button>
              <button class="btn ghost sm" onclick={() => togglePkg(k)}>{k.active ? 'Desactivar' : 'Activar'}</button>
            </span>
          {/if}
        {/snippet}
      </DataTable>
    {/if}
  </Surface></div>
{/if}

<Modal bind:open={formOpen} title={editing === null ? 'Nuevo servicio' : 'Editar servicio'}>
  <form onsubmit={save}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
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
    <div class="form-actions">
      {#if editing !== null}
        {@const cur = services.find((x) => x.id === editing)}
        {#if cur}<button type="button" class="btn ghost left" onclick={() => { toggle(cur); formOpen = false; }}>{cur.active ? 'Desactivar' : 'Activar'}</button>{/if}
      {/if}
      <button type="button" class="btn ghost" onclick={() => (formOpen = false)}>Cancelar</button>
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
    </div>
  </form>
</Modal>

<Modal bind:open={catsOpen} title="Categorías">
  <p class="meta">Cambia el nombre o elimina. Al eliminar, sus servicios pasan a otra categoría o quedan sin categoría.</p>
  <form class="add-cat" onsubmit={addCat}>
    <input bind:value={newCat} maxlength="40" placeholder="Nueva categoría (Cabello, Barba, Uñas…)" aria-label="Nueva categoría" required />
    <button class="btn">Agregar</button>
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
      <div class="form-actions">
        <button class="btn ghost" onclick={() => (delCat = null)}>Cancelar</button>
        <button class="btn" onclick={confirmDelete}>Eliminar categoría</button>
      </div>
    </div>
  {/if}
  {#if catError}<p class="error" role="alert">{catError}</p>{/if}
</Modal>

<Modal bind:open={() => pkgForm !== null, (v) => { if (!v) pkgForm = null; }} title={pkgForm === 'new' ? 'Nuevo paquete' : 'Editar paquete'}>
  <form onsubmit={savePkg}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
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
      <button type="button" class="btn ghost" onclick={() => (pkgForm = null)}>Cancelar</button>
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
    </div>
  </form>
</Modal>

<style>
  .notice { margin-bottom: 16px; }
  .gap { height: 32px; }
  .metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 16px; margin-bottom: 24px; }
  .metric-btn { display: block; padding: 0; font: inherit; text-align: left; background: none; border: 0; border-radius: var(--radius); cursor: pointer; }
  .metric-btn :global(.metric) { height: 100%; border: 1px solid var(--border); transition: border-color var(--dur-ui) var(--ease); }
  .metric-btn:hover :global(.metric) { border-color: var(--control-border); }
  .metric-btn.on :global(.metric) { border-color: var(--accent); background: var(--accent-soft); }
  .cats { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
  .cats :global(.seg) { flex: 0 1 auto; }
  .cat-acts { display: inline-flex; flex-wrap: wrap; gap: 8px; }
  .cat-acts .btn { min-height: 44px; }
  .srch :global(.search input[type='search']) { padding-left: 40px; min-height: 44px; }
  .sel select { width: auto; min-width: 160px; min-height: 44px; margin: 0; }
  @media (min-width: 641px) { .name, .sub { min-width: 140px; } .pickrow .name { min-width: 0; } }
  .name { display: block; font-weight: 600; overflow-wrap: anywhere; }
  .name.off { color: var(--muted); }
  .muted { color: var(--muted); }
  @media (max-width: 1100px) { .tight :global(th), .tight :global(td) { padding-inline: 10px; } }
  .nc { white-space: nowrap; }
  .sub { display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; max-width: 56ch; color: var(--muted); font-size: 0.85rem; overflow-wrap: anywhere; }
  .acts { display: inline-flex; gap: 8px; justify-content: flex-end; white-space: nowrap; }
  @media (max-width: 1100px) and (min-width: 641px) { .acts { flex-direction: column; align-items: stretch; } }
  .acts .btn { min-height: 44px; padding-inline: 12px; }
  .pickrow { display: flex; align-items: center; gap: 10px; }
  .pickrow .name { flex: 1; min-width: 0; }
  .bulk { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-bottom: 16px; padding: 8px 16px; background: var(--surface); border: 1px solid var(--accent); border-radius: var(--radius); }
  .bulk .check { flex: 1 1 180px; margin: 0; font-weight: 600; }
  .bulk .btn { min-height: 44px; }
  .bulk select { width: auto; min-height: 44px; margin: 0; }
  .tiles { display: grid; gap: 12px; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); }
  .tile { display: grid; align-content: space-between; justify-items: start; gap: 12px; min-height: 110px; padding: 16px; text-align: left; font: inherit; color: var(--text); background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); cursor: pointer; }
  .tile:hover:not(:disabled) { border-color: var(--control-border); }
  .tile:disabled { cursor: default; }
  .tile.in { border-color: var(--accent); background: var(--accent-soft); }
  .tile.off b { color: var(--muted); }
  .tile .foot { display: grid; gap: 2px; }
  .tile small { color: var(--muted); }
  .staff { border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 0.75rem 1rem; margin: 0 0 1rem; display: flex; flex-wrap: wrap; gap: 4px 20px; }
  .staff legend { padding: 0 6px; font-size: 0.9rem; font-weight: 500; }
  .form-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 20px; }
  .form-actions .left { margin-right: auto; }
  .add-cat { display: flex; gap: 8px; margin: 12px 0; }
  .add-cat input { flex: 1; margin: 0; }
  .cat-list { list-style: none; margin: 0 0 12px; padding: 0; display: grid; gap: 8px; }
  .cat-list li { display: flex; align-items: center; gap: 10px; }
  .cat-list input { flex: 1; min-width: 0; margin: 0; }
  .del { margin: 12px 0; padding: 12px 16px; background: var(--danger-soft); border-radius: var(--radius-sm); }
  .del p { margin: 0 0 8px; }
  .del select { margin: 0; }
</style>
