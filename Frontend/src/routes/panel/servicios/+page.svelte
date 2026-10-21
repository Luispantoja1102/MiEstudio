<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { money, toCents, toPesos } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';

  const admin = session.me.role === 'admin';

  let services = $state([]);
  let team = $state([]);
  let loading = $state(true);
  let error = $state('');
  let editing = $state(null); // null = cerrado, 'new' = alta, número = id
  let busy = $state(false);
  let form = $state(blank());

  function blank() {
    return { name: '', duration_min: 30, buffer_min: 0, price: '', staff_ids: [] };
  }

  async function load() {
    services = await api('/api/services');
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

<style>
  .staff { border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 0.75rem 1rem; margin: 0 0 1rem; display: flex; flex-wrap: wrap; gap: 4px 20px; }
  .staff legend { padding: 0 6px; font-size: 0.9rem; font-weight: 500; }
  h3 { font-size: 1.25rem; }
</style>
