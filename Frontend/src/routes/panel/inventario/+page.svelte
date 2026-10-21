<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { money, toCents, toPesos } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';

  const admin = session.me.role === 'admin';

  let products = $state([]);
  let loading = $state(true);
  let error = $state('');
  let editing = $state(null);
  let busy = $state(false);
  let form = $state({ name: '', price: '', stock: 0 });

  const load = async () => {
    products = await api('/api/products');
    loading = false;
  };
  onMount(() => load().catch((e) => ((error = e.message), (loading = false))));

  function startNew() {
    form = { name: '', price: '', stock: 0 };
    editing = 'new';
  }
  function startEdit(p) {
    form = { name: p.name, price: toPesos(p.price_cents), stock: p.stock };
    editing = p.id;
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
      const body = { name: form.name, price_cents, stock: Number(form.stock) };
      if (editing === 'new') await api('/api/products', { method: 'POST', body });
      else await api(`/api/products/${editing}`, { method: 'PATCH', body });
      editing = null;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function toggle(p) {
    error = '';
    try {
      await api(`/api/products/${p.id}`, { method: 'PATCH', body: { active: !p.active } });
      await load();
    } catch (err) {
      error = err.message;
    }
  }
</script>

<div class="page-head">
  <div>
    <h1>Inventario</h1>
    <p>Productos de reventa y su existencia.</p>
  </div>
  {#if admin}<button class="btn" onclick={startNew}>Nuevo producto</button>{/if}
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if editing !== null}
  <form class="card" onsubmit={save} style="margin-bottom: 1.5rem">
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Precio (MXN) <input inputmode="decimal" bind:value={form.price} placeholder="120.00" required /></label>
      <label class="field">Existencia <input type="number" min="0" step="1" bind:value={form.stock} required /></label>
    </div>
    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      <button type="button" class="btn ghost" onclick={() => (editing = null)}>Cancelar</button>
    </div>
  </form>
{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else if products.length === 0}
  <div class="empty">Todavía no hay productos.{#if admin} Agrega el primero con «Nuevo producto».{/if}</div>
{:else}
  <div class="list">
    {#each products as p (p.id)}
      <div class="item-row" class:off={!p.active}>
        <div class="grow">
          <div class="title">{p.name} {#if !p.active}<span class="badge muted">Inactivo</span>{/if}</div>
          <div class="meta">Existencia: {p.stock}{#if p.stock === 0} · <strong>Agotado</strong>{/if}</div>
        </div>
        <strong>{money(p.price_cents)}</strong>
        {#if admin}
          <button class="btn ghost sm" onclick={() => startEdit(p)}>Editar</button>
          <button class="btn ghost sm" onclick={() => toggle(p)}>{p.active ? 'Desactivar' : 'Activar'}</button>
        {/if}
      </div>
    {/each}
  </div>
{/if}
