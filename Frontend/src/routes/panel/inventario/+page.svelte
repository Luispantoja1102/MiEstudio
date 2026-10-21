<script>
  import { onMount } from 'svelte';
  import Modal from '$lib/Modal.svelte';
  import { api } from '$lib/api.js';
  import { formatDate, money, toCents, toPesos } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';

  const admin = session.me.role === 'admin';
  const REASON = { initial: 'Existencia inicial', purchase: 'Compra', sale: 'Venta', void: 'Venta cancelada', adjustment: 'Ajuste', loss: 'Pérdida o merma' };

  let products = $state([]);
  let loading = $state(true);
  let error = $state('');
  let editing = $state(null);
  let busy = $state(false);
  let form = $state({ name: '', price: '', cost: '', stock: 0 });

  // existencia
  let stockOpen = $state(false);
  let target = $state(null);
  let movements = $state([]);
  let adj = $state({ type: 'purchase', qty: '', note: '' });

  const load = async () => {
    products = await api('/api/products');
    loading = false;
  };
  onMount(() => load().catch((e) => ((error = e.message), (loading = false))));

  function startNew() {
    form = { name: '', price: '', cost: '', stock: 0 };
    editing = 'new';
  }
  function startEdit(p) {
    form = { name: p.name, price: toPesos(p.price_cents), cost: toPesos(p.cost_cents ?? 0), stock: p.stock };
    editing = p.id;
  }

  async function save(e) {
    e.preventDefault();
    error = '';
    const price_cents = toCents(form.price);
    const cost_cents = toCents(form.cost || '0');
    if (Number.isNaN(price_cents) || Number.isNaN(cost_cents)) return (error = 'Escribe precios válidos, por ejemplo 150 o 150.50');
    busy = true;
    try {
      const body = { name: form.name, price_cents, cost_cents };
      if (editing === 'new') await api('/api/products', { method: 'POST', body: { ...body, stock: Number(form.stock) } });
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

  async function openStock(p) {
    target = p;
    adj = { type: 'purchase', qty: '', note: '' };
    movements = await api(`/api/products/${p.id}/movements`).catch(() => []);
    stockOpen = true;
  }

  async function adjust(e) {
    e.preventDefault();
    error = '';
    const n = Math.abs(Math.trunc(Number(adj.qty)));
    if (!n) return (error = 'Escribe una cantidad mayor a cero');
    // compra suma, pérdida resta; el ajuste puede ser en cualquiera de los dos sentidos
    const [reason, sign] = adj.type === 'purchase' ? ['purchase', 1] : adj.type === 'loss' ? ['loss', -1] : adj.type === 'add' ? ['adjustment', 1] : ['adjustment', -1];
    try {
      await api(`/api/products/${target.id}/stock`, { method: 'POST', body: { delta: sign * n, reason, note: adj.note } });
      await load();
      target = products.find((x) => x.id === target.id);
      movements = await api(`/api/products/${target.id}/movements`);
      adj = { type: adj.type, qty: '', note: '' };
    } catch (err) {
      error = err.message;
    }
  }
</script>

<div class="page-head">
  <div>
    <h1>Inventario</h1>
    <p>Productos de reventa y su existencia. Cada cambio queda en el historial.</p>
  </div>
  {#if admin}<button class="btn" onclick={startNew}>Nuevo producto</button>{/if}
</div>

{#if error && !stockOpen}<p class="error" role="alert">{error}</p>{/if}

{#if editing !== null}
  <form class="card" onsubmit={save} style="margin-bottom: 1.5rem">
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Precio de venta (MXN) <input inputmode="decimal" bind:value={form.price} placeholder="120.00" required /></label>
      <label class="field">Costo (MXN) <input inputmode="decimal" bind:value={form.cost} placeholder="50.00" /></label>
      {#if editing === 'new'}
        <label class="field">Existencia inicial <input type="number" min="0" step="1" bind:value={form.stock} required /></label>
      {/if}
    </div>
    {#if editing !== 'new'}<p class="meta">Para cambiar la existencia usa «Existencia» en la lista.</p>{/if}
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
          <div class="title">{p.name} {#if !p.active}<span class="badge muted">Inactivo</span>{/if}{#if p.stock === 0 && p.active}<span class="badge warn">Agotado</span>{/if}</div>
          <div class="meta">Existencia: {p.stock}{#if p.cost_cents !== undefined} · costo {money(p.cost_cents)}{/if}</div>
        </div>
        <strong>{money(p.price_cents)}</strong>
        {#if admin}
          <button class="btn ghost sm" onclick={() => openStock(p)}>Existencia</button>
          <button class="btn ghost sm" onclick={() => startEdit(p)}>Editar</button>
          <button class="btn ghost sm" onclick={() => toggle(p)}>{p.active ? 'Desactivar' : 'Activar'}</button>
        {/if}
      </div>
    {/each}
  </div>
{/if}

<Modal bind:open={stockOpen} title={target ? `Existencia · ${target.name}` : 'Existencia'}>
  {#if target}
    <p class="meta">Ahora hay <b>{target.stock}</b> piezas.</p>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <form onsubmit={adjust}>
      <div class="form-grid">
        <label class="field">Movimiento
          <select bind:value={adj.type}>
            <option value="purchase">Compra (suma)</option>
            <option value="loss">Pérdida o merma (resta)</option>
            <option value="add">Ajuste: sumar</option>
            <option value="sub">Ajuste: restar</option>
          </select>
        </label>
        <label class="field">Piezas <input type="number" min="1" step="1" bind:value={adj.qty} required /></label>
        <label class="field">Nota <input bind:value={adj.note} maxlength="300" placeholder="Opcional" /></label>
      </div>
      <button class="btn">Registrar</button>
    </form>
    <h3 style="font-size: 1.1rem; margin: 1.5rem 0 0.5rem">Historial</h3>
    <div class="list" style="max-height: 260px; overflow-y: auto">
      {#each movements as m (m.id)}
        <div class="item-row">
          <span class="grow">{REASON[m.reason]}<small class="meta"> · {m.by || 'Sistema'}{m.note ? ' · ' + m.note : ''}</small><br /><small class="meta">{formatDate(m.created_at)}</small></span>
          <b style="color: {m.delta > 0 ? 'var(--accent)' : 'var(--danger)'}">{m.delta > 0 ? '+' : ''}{m.delta}</b>
          <small class="meta">saldo {m.balance}</small>
        </div>
      {:else}<p class="meta">Sin movimientos.</p>{/each}
    </div>
  {/if}
</Modal>
