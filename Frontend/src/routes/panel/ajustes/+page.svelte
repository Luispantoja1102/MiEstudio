<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { session } from '$lib/session.svelte.js';

  let branches = $state([]);
  let error = $state('');
  let editing = $state(null); // null | 'new' | id
  let busy = $state(false);
  let form = $state({ name: '', address: '', phone: '' });

  const load = async () => (branches = await api('/api/branches'));
  onMount(() => {
    if (session.me.role !== 'admin') return goto('/panel');
    load().catch((e) => (error = e.message));
  });

  function startNew() {
    form = { name: '', address: '', phone: '' };
    editing = 'new';
  }
  function startEdit(b) {
    form = { name: b.name, address: b.address, phone: b.phone };
    editing = b.id;
  }

  async function save(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      if (editing === 'new') await api('/api/branches', { method: 'POST', body: form });
      else await api(`/api/branches/${editing}`, { method: 'PATCH', body: form });
      editing = null;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function toggle(b) {
    error = '';
    try {
      await api(`/api/branches/${b.id}`, { method: 'PATCH', body: { active: !b.active } });
    } catch (err) {
      error = err.message;
    }
    await load();
  }
</script>

<div class="page-head">
  <div>
    <h1>Sucursales</h1>
    <p>Los lugares donde atiendes. La agenda y los horarios se organizan por sucursal.</p>
  </div>
  <button class="btn" onclick={startNew}>Nueva sucursal</button>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if editing !== null}
  <form class="card" onsubmit={save} style="margin-bottom: 1.5rem">
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Dirección <input bind:value={form.address} maxlength="300" /></label>
      <label class="field">Teléfono <input type="tel" bind:value={form.phone} maxlength="30" /></label>
    </div>
    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      <button type="button" class="btn ghost" onclick={() => (editing = null)}>Cancelar</button>
    </div>
  </form>
{/if}

<div class="list">
  {#each branches as b (b.id)}
    <div class="item-row" class:off={!b.active}>
      <div class="grow">
        <div class="title">{b.name} {#if !b.active}<span class="badge muted">Inactiva</span>{/if}</div>
        <div class="meta">{[b.address, b.phone].filter(Boolean).join(' · ') || 'Sin dirección'}</div>
      </div>
      <button class="btn ghost sm" onclick={() => startEdit(b)}>Editar</button>
      <button class="btn ghost sm" onclick={() => toggle(b)}>{b.active ? 'Desactivar' : 'Activar'}</button>
    </div>
  {/each}
</div>
