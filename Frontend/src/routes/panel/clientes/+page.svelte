<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';

  let clients = $state([]);
  let q = $state('');
  let loading = $state(true);
  let error = $state('');
  let showForm = $state(false);
  let busy = $state(false);
  let form = $state({ name: '', phone: '', email: '', birthday: '', reminders_consent: false, marketing_consent: false });
  let timer;

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

<div class="page-head">
  <div>
    <h1>Clientes</h1>
    <p>Tu cartera de clientes y sus datos de contacto.</p>
  </div>
  <button class="btn" onclick={() => (showForm = !showForm)}>{showForm ? 'Cancelar' : 'Nuevo cliente'}</button>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if showForm}
  <form class="card" onsubmit={add} style="margin-bottom: 1.5rem">
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Teléfono <input type="tel" bind:value={form.phone} autocomplete="off" /></label>
      <label class="field">Correo <input type="email" bind:value={form.email} /></label>
      <label class="field">Cumpleaños <input type="date" bind:value={form.birthday} /></label>
    </div>
    <label class="check"><input type="checkbox" bind:checked={form.reminders_consent} disabled={!form.email} /> El cliente aceptó recibir recordatorios de sus citas por correo</label>
    <label class="check"><input type="checkbox" bind:checked={form.marketing_consent} disabled={!form.email} /> El cliente aceptó recibir promociones y felicitación de cumpleaños por correo</label>
    <div class="form-actions"><button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar y abrir ficha'}</button></div>
  </form>
{/if}

<label class="field">
  <span class="sr-only">Buscar clientes</span>
  <input type="search" placeholder="Buscar por nombre, teléfono o correo" bind:value={q} oninput={onSearch} />
</label>

{#if loading}
  <p class="meta">Cargando…</p>
{:else if clients.length === 0}
  <div class="empty">{q ? 'No se encontraron clientes con esa búsqueda.' : 'Todavía no hay clientes. Agrega el primero con «Nuevo cliente».'}</div>
{:else}
  <div class="list">
    {#each clients as c (c.id)}
      <a class="item-row link" href="/panel/clientes/{c.id}">
        <div class="grow">
          <div class="title">{c.name}</div>
          <div class="meta">{[c.phone, c.email].filter(Boolean).join(' · ') || 'Sin datos de contacto'}</div>
        </div>
      </a>
    {/each}
  </div>
  {#if clients.length >= 100}<p class="meta" style="margin-top: 1rem">Mostrando los primeros 100. Usa la búsqueda para acotar.</p>{/if}
{/if}

<style>
  .link { color: var(--text); text-decoration: none; transition: border-color 0.2s var(--ease); }
  .link:hover { border-color: var(--accent); }
</style>
