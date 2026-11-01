<script>
  import { onMount } from 'svelte';
  import Modal from '$lib/Modal.svelte';
  import { api } from '$lib/api.js';
  import { formatDate, money, toCents, toPesos } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  const admin = session.me.role === 'admin';
  const REASON = { initial: 'Existencia inicial', purchase: 'Compra', sale: 'Venta', void: 'Venta cancelada', adjustment: 'Ajuste', loss: 'Pérdida o merma' };

  let products = $state([]);
  let loading = $state(true);
  let error = $state('');
  let editing = $state(null);
  let formOpen = $state(false);
  const LOW = 4; // existencia desde la cual se avisa «Poco stock»
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
    error = '';
    formOpen = true;
  }
  function startEdit(p) {
    form = { name: p.name, price: toPesos(p.price_cents), cost: toPesos(p.cost_cents ?? 0), stock: p.stock };
    editing = p.id;
    error = '';
    formOpen = true;
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
      formOpen = false;
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
    error = '';
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

  // Un producto sin cost_cents (rol sin acceso a costos) no muestra la columna de costo.
  const showCost = $derived(products.some((p) => p.cost_cents !== undefined));
  const columns = $derived([
    { key: 'name', label: 'Producto' },
    { key: 'price', label: 'Precio', numeric: true },
    ...(showCost ? [{ key: 'cost', label: 'Costo', numeric: true }] : []),
    { key: 'stock', label: 'Existencia', numeric: true },
    { key: 'status', label: 'Estado' },
    ...(admin ? [{ key: 'actions', label: 'Acciones' }] : [])
  ]);
  const status = (p) => (!p.active ? ['Inactivo', 'neutral'] : p.stock === 0 ? ['Agotado', 'danger'] : p.stock <= LOW ? ['Poco stock', 'warning'] : ['Disponible', 'success']);
</script>

<PageHeader title="Inventario" description="Productos de reventa y su existencia. Cada cambio queda en el historial.">
  {#snippet actions()}
    {#if admin}<button class="btn" onclick={startNew}>Nuevo producto</button>{/if}
  {/snippet}
</PageHeader>

{#if error && !stockOpen && !formOpen}
  <div class="notice"><InlineNotice tone="danger" message={error} /></div>
{/if}

<Surface padding="flush">
  {#if loading}
    <LoadingState />
  {:else if products.length === 0}
    <EmptyState title="Todavía no hay productos" description={admin ? 'Agrega el primero para llevar su existencia.' : ''}>
      {#snippet actions()}{#if admin}<button class="btn" onclick={startNew}>Nuevo producto</button>{/if}{/snippet}
    </EmptyState>
  {:else}
    <DataTable caption="Productos y existencias" {columns} rows={products}>
      {#snippet cell({ row, column })}
        {#if column.key === 'name'}
          <span class="name" class:off={!row.active}>{row.name}</span>
        {:else if column.key === 'price'}
          {money(row.price_cents)}
        {:else if column.key === 'cost'}
          {row.cost_cents === undefined ? '—' : money(row.cost_cents)}
        {:else if column.key === 'stock'}
          <b class="qty" class:low={row.active && row.stock <= LOW}>{row.stock}</b>
        {:else if column.key === 'status'}
          <StatusBadge label={status(row)[0]} tone={status(row)[1]} />
        {:else}
          <span class="acts">
            <button class="lnk" onclick={() => openStock(row)}>Ajustar existencia</button>
            <button class="lnk" onclick={() => startEdit(row)}>Editar</button>
            <button class="lnk quiet" onclick={() => toggle(row)}>{row.active ? 'Desactivar' : 'Activar'}</button>
          </span>
        {/if}
      {/snippet}
    </DataTable>
  {/if}
</Surface>

<Modal bind:open={formOpen} title={editing === 'new' ? 'Nuevo producto' : 'Editar producto'}>
  <form onsubmit={save}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Precio de venta (MXN) <input inputmode="decimal" bind:value={form.price} placeholder="120.00" required /></label>
      <label class="field">Costo (MXN) <input inputmode="decimal" bind:value={form.cost} placeholder="50.00" /></label>
      {#if editing === 'new'}
        <label class="field">Existencia inicial <input type="number" min="0" step="1" bind:value={form.stock} required /></label>
      {/if}
    </div>
    {#if editing !== 'new'}<p class="hint">Para cambiar la existencia usa «Ajustar existencia» en la lista.</p>{/if}
    <div class="actions">
      <button type="button" class="btn ghost" onclick={() => (formOpen = false)}>Cancelar</button>
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
    </div>
  </form>
</Modal>

<Modal bind:open={stockOpen} title={target ? `Existencia · ${target.name}` : 'Existencia'}>
  {#if target}
    <p class="now">Ahora hay <b>{target.stock}</b> {target.stock === 1 ? 'pieza' : 'piezas'}.</p>
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
        <label class="field">Motivo o nota <input bind:value={adj.note} maxlength="300" placeholder="Opcional" /></label>
      </div>
      <div class="actions"><button class="btn">Registrar</button></div>
    </form>
    <h3>Historial</h3>
    <ul class="hist">
      {#each movements as m (m.id)}
        <li>
          <div class="what">
            <span>{REASON[m.reason]}</span>
            <small>{formatDate(m.created_at)} · {m.by || 'Sistema'}{m.note ? ' · ' + m.note : ''}</small>
          </div>
          <b class="delta" class:plus={m.delta > 0} class:minus={m.delta < 0}>{m.delta > 0 ? '+' : ''}{m.delta}</b>
          <small class="bal">saldo {m.balance}</small>
        </li>
      {:else}<li class="none">Sin movimientos.</li>{/each}
    </ul>
  {/if}
</Modal>

<style>
  .notice { margin-bottom: 16px; }
  .name { font-weight: 600; overflow-wrap: anywhere; }
  .name.off { color: var(--muted); }
  .qty { font-variant-numeric: tabular-nums; }
  .qty.low { color: var(--warn); }
  .acts { display: inline-flex; flex-wrap: wrap; gap: 6px; }
  .acts { gap: 0 4px; }
  .lnk { min-height: 44px; padding: 0 10px; border: 0; background: none; font: inherit; font-size: 0.9rem; font-weight: 600; color: var(--accent); cursor: pointer; border-radius: var(--radius-xs); white-space: nowrap; }
  .lnk:hover { background: var(--accent-soft); }
  .lnk.quiet { color: var(--muted); }
  .hint { margin: 4px 0 0; color: var(--muted); font-size: 0.88rem; }
  .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 16px; }
  .now { margin: 0 0 12px; color: var(--muted); }
  .now b { color: var(--text); font-size: 1.25rem; font-variant-numeric: tabular-nums; }
  h3 { margin: 24px 0 8px; padding-top: 16px; border-top: 1px solid var(--border); font-size: 1rem; }
  .hist { list-style: none; margin: 0; padding: 0; max-height: 280px; overflow-y: auto; }
  .hist li { display: grid; grid-template-columns: 1fr auto 4.5rem; align-items: baseline; gap: 12px; padding: 10px 0; border-bottom: 1px solid var(--border); }
  .hist li:last-child { border-bottom: 0; }
  .what { display: grid; gap: 2px; min-width: 0; }
  .what small, .bal { color: var(--muted); font-size: 0.82rem; overflow-wrap: anywhere; }
  .bal { text-align: right; font-variant-numeric: tabular-nums; }
  .delta { font-variant-numeric: tabular-nums; }
  .delta.plus { color: var(--success); }
  .delta.minus { color: var(--danger); }
  .hist .none { display: block; color: var(--muted); }
</style>
