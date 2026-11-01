<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import Modal from '$lib/Modal.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import SearchField from '$lib/ui/SearchField.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  let clients = $state([]);
  let q = $state('');
  let loading = $state(true);
  let error = $state('');
  let showForm = $state(false);
  let busy = $state(false);
  let form = $state({ name: '', phone: '', email: '', birthday: '', reminders_consent: false, marketing_consent: false, whatsapp_consent: false });
  let timer;

  const columns = [
    { key: 'name', label: 'Cliente' },
    { key: 'contact', label: 'Contacto' }
  ];

  async function load() {
    try {
      clients = await api('/api/clients?q=' + encodeURIComponent(q.trim()));
      error = '';
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }
  onMount(load);

  // busca al dejar de escribir
  function onSearch() {
    clearTimeout(timer);
    timer = setTimeout(load, 250);
  }

  function openForm() {
    error = '';
    showForm = true;
  }

  async function add(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      const { id } = await api('/api/clients', { method: 'POST', body: form });
      goto(`/panel/clientes/${id}`);
    } catch (err) {
      error = err.message;
      busy = false;
    }
  }
</script>

<PageHeader title="Clientes" description="Tu cartera de clientes y sus datos de contacto.">
  {#snippet actions()}
    <button class="btn" onclick={openForm}>Nuevo cliente</button>
  {/snippet}
</PageHeader>

{#if error && !showForm}
  <div class="notice">
    <InlineNotice tone="danger" message={error}>
      {#snippet actions()}<button class="btn ghost sm" onclick={load}>Reintentar</button>{/snippet}
    </InlineNotice>
  </div>
{/if}

<Surface padding="flush">
  <div class="bar"><SearchField bind:value={q} oninput={onSearch} label="Buscar clientes" placeholder="Buscar por nombre, teléfono o correo" /></div>

  {#if loading}
    <LoadingState />
  {:else if clients.length === 0}
    {#if q.trim()}
      <EmptyState title="Sin resultados" description="No se encontraron clientes con esa búsqueda." />
    {:else}
      <EmptyState title="Todavía no hay clientes" description="Agrega el primero para guardar su contacto y sus consentimientos.">
        {#snippet actions()}<button class="btn" onclick={openForm}>Nuevo cliente</button>{/snippet}
      </EmptyState>
    {/if}
  {:else}
    <div class="dir"><DataTable caption="Directorio de clientes" {columns} rows={clients}>
      {#snippet cell({ row, column })}
        {#if column.key === 'name'}
          <a class="name" href="/panel/clientes/{row.id}">{row.name}</a>
          <span class="sub">{[row.phone, row.email].filter(Boolean).join(' · ') || 'Sin datos de contacto'}</span>
        {:else if row.phone || row.email}
          <span class="contact">
            {#if row.phone}<span>{row.phone}</span>{/if}
            {#if row.email}<span class="mail">{row.email}</span>{/if}
          </span>
        {:else}
          <span class="none">Sin datos de contacto</span>
        {/if}
      {/snippet}
    </DataTable></div>
    {#if clients.length >= 100}<p class="limit">Mostrando los primeros 100. Usa la búsqueda para acotar.</p>{/if}
  {/if}
</Surface>

<Modal bind:open={showForm} title="Nuevo cliente">
  <form onsubmit={add}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Teléfono <input type="tel" bind:value={form.phone} autocomplete="off" /></label>
      <label class="field">Correo <input type="email" bind:value={form.email} /></label>
      <label class="field">Cumpleaños <input type="date" bind:value={form.birthday} /></label>
    </div>
    <fieldset>
      <legend>Consentimientos</legend>
      <label class="check"><input type="checkbox" bind:checked={form.reminders_consent} disabled={!form.email} /> El cliente aceptó recibir recordatorios de sus citas por correo</label>
      <label class="check"><input type="checkbox" bind:checked={form.whatsapp_consent} disabled={form.phone.replace(/\D/g, '').length < 10} /> El cliente aceptó recibir recordatorios por WhatsApp</label>
      <label class="check"><input type="checkbox" bind:checked={form.marketing_consent} disabled={!form.email} /> El cliente aceptó recibir promociones y felicitación de cumpleaños por correo</label>
    </fieldset>
    <div class="actions">
      <button type="button" class="btn ghost" onclick={() => (showForm = false)}>Cancelar</button>
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar y abrir ficha'}</button>
    </div>
  </form>
</Modal>

<style>
  .notice { margin-bottom: 16px; }
  .bar { padding: 16px 20px; border-bottom: 1px solid var(--border); }
  .bar :global(.search) { max-width: 420px; }
  .name { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; color: var(--text); text-decoration: none; overflow-wrap: anywhere; }
  .name:hover { color: var(--accent); text-decoration: underline; }
  .sub { display: none; color: var(--muted); font-size: 0.85rem; overflow-wrap: anywhere; padding-bottom: 6px; }
  @media (max-width: 599px) {
    .sub { display: block; }
    .dir :global(th:nth-child(2)), .dir :global(td:nth-child(2)) { display: none; }
    .dir :global(td), .dir :global(th) { padding-left: 16px; padding-right: 16px; }
    .name { min-height: 36px; padding-top: 4px; }
  }
  .contact { display: grid; gap: 2px; font-variant-numeric: tabular-nums; }
  .mail { color: var(--muted); font-size: 0.85rem; overflow-wrap: anywhere; }
  .none { color: var(--muted); }
  .limit { margin: 0; padding: 14px 20px; border-top: 1px solid var(--border); color: var(--muted); font-size: 0.88rem; }
  fieldset { border: 0; margin: 4px 0 0; padding: 12px 0 0; border-top: 1px solid var(--border); display: grid; }
  legend { padding: 0 8px 0 0; font-size: 0.8rem; font-weight: 600; color: var(--muted); }
  .check { align-items: flex-start; padding: 8px 0; }
  .check input { margin-top: 2px; flex: none; }
  .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 20px; }
</style>
