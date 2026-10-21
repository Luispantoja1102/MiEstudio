<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { longDate, todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';

  const tz = session.me.business.timezone || 'America/Tijuana';
  const STATUS = { waiting: 'En espera', notified: 'Ya se le avisó' };

  let list = $state([]);
  let services = $state([]);
  let staff = $state([]);
  let loading = $state(true);
  let error = $state('');
  let open = $state(false);
  let busy = $state(false);
  let clientQuery = $state('');
  let clientResults = $state([]);
  let client = $state(null);
  let form = $state(blank());
  let timer;

  function blank() {
    return { branch_id: '', service_id: '', staff_id: '', date: todayIn(tz), from: '', to: '' };
  }

  async function load() {
    list = await api('/api/waitlist');
    loading = false;
  }

  onMount(async () => {
    try {
      await loadBranches();
      const [s, t] = await Promise.all([api('/api/services'), api('/api/agenda?' + new URLSearchParams({ branch_id: branches.id, date: todayIn(tz) }))]);
      services = s.filter((x) => x.active);
      staff = t.staff;
      await load();
    } catch (e) {
      error = e.message;
      loading = false;
    }
  });

  function startNew() {
    form = { ...blank(), branch_id: String(branches.id ?? '') };
    client = null;
    clientQuery = '';
    open = true;
  }

  function search() {
    clearTimeout(timer);
    timer = setTimeout(async () => {
      clientResults = clientQuery.trim() ? (await api('/api/clients?q=' + encodeURIComponent(clientQuery.trim()))).slice(0, 5) : [];
    }, 250);
  }

  async function save(e) {
    e.preventDefault();
    error = '';
    if (!client) return (error = 'Elige al cliente.');
    busy = true;
    try {
      await api('/api/waitlist', {
        method: 'POST',
        body: { client_id: client.id, branch_id: Number(form.branch_id), service_id: Number(form.service_id), staff_id: form.staff_id ? Number(form.staff_id) : 0, date: form.date, from: form.from, to: form.to }
      });
      open = false;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function remove(w) {
    error = '';
    try {
      await api(`/api/waitlist/${w.id}`, { method: 'DELETE' });
      await load();
    } catch (err) {
      error = err.message;
    }
  }
</script>

<div class="page-head">
  <div>
    <h1>Lista de espera</h1>
    <p>Quien no encontró horario. Cuando se libera uno, se le avisa por correo (el primero que lo reserva se lo queda).</p>
  </div>
  <button class="btn" onclick={startNew}>Agregar a la lista</button>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if open}
  <form class="card" onsubmit={save} style="margin-bottom: 1.5rem">
    <h3>Nueva solicitud</h3>
    <div class="client">
      {#if client}
        <div class="item-row"><span class="grow title">{client.name}</span><span class="meta">{client.email || 'sin correo'}</span><button type="button" class="btn ghost sm" onclick={() => (client = null)}>Cambiar</button></div>
      {:else}
        <input type="search" placeholder="Buscar cliente…" aria-label="Buscar cliente" bind:value={clientQuery} oninput={search} />
        {#each clientResults as c (c.id)}<button type="button" class="pick" onclick={() => ((client = c), (clientResults = []), (clientQuery = ''))}>{c.name}<small>{c.email || 'sin correo'}</small></button>{/each}
      {/if}
    </div>
    <div class="form-grid">
      <label class="field">Servicio
        <select bind:value={form.service_id} required>
          <option value="" disabled>Elige…</option>
          {#each services as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
        </select>
      </label>
      <label class="field">Con
        <select bind:value={form.staff_id}>
          <option value="">Cualquiera</option>
          {#each staff as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
        </select>
      </label>
      {#if branches.list?.length > 1}
        <label class="field">Sucursal
          <select bind:value={form.branch_id} required>
            {#each branches.list as b (b.id)}<option value={String(b.id)}>{b.name}</option>{/each}
          </select>
        </label>
      {/if}
      <label class="field">Día <input type="date" bind:value={form.date} min={todayIn(tz)} required /></label>
      <label class="field">Desde (opcional) <input type="time" bind:value={form.from} /></label>
      <label class="field">Hasta <input type="time" bind:value={form.to} /></label>
    </div>
    <div class="form-actions">
      <button class="btn" disabled={busy || !client}>{busy ? 'Guardando…' : 'Agregar'}</button>
      <button type="button" class="btn ghost" onclick={() => (open = false)}>Cancelar</button>
    </div>
  </form>
{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else if list.length === 0}
  <div class="empty">Nadie en lista de espera. Cuando alguien no encuentre horario en la reserva en línea, aparecerá aquí.</div>
{:else}
  <div class="list">
    {#each list as w (w.id)}
      <div class="item-row">
        <div class="grow">
          <div class="title">{w.client_name} <span class="badge" class:muted={w.status === 'notified'}>{STATUS[w.status]}</span></div>
          <div class="meta">{w.service_name}{w.staff_name ? ` con ${w.staff_name}` : ''} · {longDate(w.day)}{w.from ? ` · de ${w.from} a ${w.to}` : ''}{branches.list?.length > 1 ? ` · ${w.branch_name}` : ''} · {w.client_phone}{w.source === 'online' ? ' · desde la reserva en línea' : ''}</div>
        </div>
        <button class="btn ghost sm" onclick={() => remove(w)}>Quitar</button>
      </div>
    {/each}
  </div>
{/if}

<style>
  h3 { font-size: 1.25rem; }
  .client { margin-bottom: 1rem; }
  .pick { display: flex; justify-content: space-between; width: 100%; min-height: 40px; align-items: center; padding: 0 0.8rem; margin-top: 4px; font: inherit; color: var(--text); background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; }
  .pick small { color: var(--muted); }
</style>
