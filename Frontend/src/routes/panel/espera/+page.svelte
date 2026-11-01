<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { longDate, todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';
  import Modal from '$lib/Modal.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  const tz = session.me.business.timezone || 'America/Tijuana';
  const STATUS = { waiting: 'En espera', notified: 'Ya se le avisó' };
  const TONE = { waiting: 'warning', notified: 'success' };
  const columns = [
    { key: 'client', label: 'Cliente' },
    { key: 'service', label: 'Servicio' },
    { key: 'when', label: 'Día y franja' },
    { key: 'status', label: 'Estado' },
    { key: 'act', label: 'Acción' }
  ];

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

<PageHeader title="Lista de espera" description="Quien no encontró horario. Cuando se libera uno, se le avisa por correo (el primero que lo reserva se lo queda).">
  {#snippet actions()}
    <button class="btn" onclick={startNew}>Agregar a la lista</button>
  {/snippet}
</PageHeader>

{#if error && !open}
  <div class="notice"><InlineNotice tone="danger" message={error} /></div>
{/if}

<Surface padding="flush">
  {#if loading}
    <LoadingState />
  {:else if list.length === 0}
    <EmptyState title="Nadie en lista de espera" description="Cuando alguien no encuentre horario en la reserva en línea, aparecerá aquí." />
  {:else}
    <DataTable caption="Solicitudes en lista de espera" {columns} rows={list}>
      {#snippet cell({ row: w, column })}
        {#if column.key === 'client'}
          <span class="name">{w.client_name}</span>
          <span class="sub">{w.client_phone}{w.source === 'online' ? ' · desde la reserva en línea' : ''}</span>
        {:else if column.key === 'service'}
          <span class="v"><span class="name">{w.service_name}</span>
          <span class="sub">{w.staff_name ? `Con ${w.staff_name}` : 'Cualquiera'}{branches.list?.length > 1 ? ` · ${w.branch_name}` : ''}</span></span>
        {:else if column.key === 'when'}
          <span class="v"><span class="name">{longDate(w.day)}</span>
          {#if w.from}<span class="sub">de {w.from} a {w.to}</span>{/if}</span>
        {:else if column.key === 'status'}
          <StatusBadge label={STATUS[w.status] ?? w.status} tone={TONE[w.status] ?? 'neutral'} />
        {:else}
          <button class="btn ghost sm" onclick={() => remove(w)} aria-label={`Quitar a ${w.client_name} de la lista`}>Quitar</button>
        {/if}
      {/snippet}
    </DataTable>
  {/if}
</Surface>

<Modal bind:open title="Nueva solicitud">
  <form onsubmit={save}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
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
    <div class="actions">
      <button type="button" class="btn ghost" onclick={() => (open = false)}>Cancelar</button>
      <button class="btn" disabled={busy || !client}>{busy ? 'Guardando…' : 'Agregar'}</button>
    </div>
  </form>
</Modal>

<style>
  .notice { margin-bottom: 16px; }
  .v { display: block; min-width: 0; }
  .name { display: block; font-weight: 600; overflow-wrap: anywhere; }
  .sub { display: block; color: var(--muted); font-size: 0.85rem; overflow-wrap: anywhere; }
  .client { margin-bottom: 1rem; }
  .pick { display: flex; justify-content: space-between; gap: 8px; width: 100%; min-height: 44px; align-items: center; padding: 0 0.8rem; margin-top: 4px; font: inherit; color: var(--text); background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; text-align: left; }
  .pick small { color: var(--muted); }
  .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 20px; }
</style>
