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
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import FilterBar from '$lib/ui/FilterBar.svelte';
  import SearchField from '$lib/ui/SearchField.svelte';
  import SegmentedControl from '$lib/ui/SegmentedControl.svelte';

  const admin = session.me.role === 'admin';
  const REASON = { initial: 'Existencia inicial', purchase: 'Compra', sale: 'Venta', void: 'Venta cancelada', adjustment: 'Ajuste', loss: 'Pérdida o merma' };
  const STATE = { out: ['Agotado', 'danger'], low: ['Poco stock', 'warning'], ok: ['Disponible', 'success'], off: ['Inactivo', 'neutral'] };
  const EMPTY = { name: '', price: '', cost: '', stock: 0, min: 0, barcode: '' };

  let products = $state([]);
  let loading = $state(true);
  let error = $state('');
  let busy = $state(false);

  // lista
  let filter = $state('all'); // all | out | low
  let query = $state('');
  let sort = $state('urgent'); // urgent | name | value
  let limit = $state(60);

  // alta y edición
  let formOpen = $state(false);
  let editing = $state(null);
  let form = $state({ ...EMPTY });

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

  // ── cálculos
  const stateOf = (p) => (!p.active ? 'off' : p.stock <= 0 ? 'out' : p.min_stock > 0 && p.stock <= p.min_stock ? 'low' : 'ok');
  const rows = $derived(
    products.map((p) => {
      const scale = Math.max(p.stock, p.min_stock * 2, 1); // la barra llega hasta el doble del mínimo o la existencia, lo que sea mayor
      return {
        ...p,
        state: stateOf(p),
        fill: Math.min(100, (p.stock / scale) * 100),
        minPos: (p.min_stock / scale) * 100,
        value: p.stock * (p.cost_cents ?? 0)
      };
    })
  );
  const live = $derived(rows.filter((r) => r.active));
  const counts = $derived({
    withStock: live.filter((r) => r.stock > 0).length,
    out: live.filter((r) => r.state === 'out').length,
    low: live.filter((r) => r.state === 'low').length
  });
  const stockValue = $derived({
    total: live.reduce((n, r) => n + r.value, 0),
    missing: live.filter((r) => r.stock > 0 && !r.cost_cents).length
  });

  // lo más urgente primero: agotados, luego los que están más cerca de acabarse, luego el resto por nombre
  const urgency = (r) => (r.state === 'out' ? 0 : r.state === 'low' ? 1 + r.stock / Math.max(r.min_stock, 1) : r.state === 'ok' ? 10 : 20);
  const shown = $derived(
    rows
      .filter((r) => (filter === 'all' ? true : r.state === filter))
      .filter((r) => {
        const t = query.trim().toLowerCase();
        return !t || r.name.toLowerCase().includes(t) || r.barcode.toLowerCase().includes(t);
      })
      .sort((a, b) => (sort === 'name' ? 0 : sort === 'value' ? b.value - a.value : urgency(a) - urgency(b)) || a.name.localeCompare(b.name, 'es'))
  );
  const headline = $derived(
    counts.out || counts.low
      ? `${counts.out ? `${counts.out} agotado${counts.out === 1 ? '' : 's'}` : ''}${counts.out && counts.low ? ' y ' : ''}${counts.low ? `${counts.low} con poco stock` : ''}.`
      : 'Productos de reventa y su existencia. Cada cambio queda en el historial.'
  );

  // ── alta y edición
  function startNew() {
    form = { ...EMPTY };
    editing = null;
    error = '';
    formOpen = true;
  }
  function startEdit(p) {
    form = { name: p.name, price: toPesos(p.price_cents), cost: toPesos(p.cost_cents ?? 0), stock: p.stock, min: p.min_stock, barcode: p.barcode };
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
      const body = { name: form.name, price_cents, cost_cents, min_stock: Number(form.min) || 0, barcode: form.barcode.trim() };
      if (editing === null) await api('/api/products', { method: 'POST', body: { ...body, stock: Number(form.stock) } });
      else await api(`/api/products/${editing}`, { method: 'PATCH', body });
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

  // ── existencia
  async function openStock(p) {
    target = p;
    adj = { type: 'purchase', qty: '', note: '' };
    error = '';
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

  const columns = $derived([
    { key: 'name', label: 'Producto' },
    { key: 'price', label: 'Precio', numeric: true },
    { key: 'stock', label: 'Existencia' },
    { key: 'status', label: 'Estado' },
    ...(admin ? [{ key: 'value', label: 'Valor', numeric: true }, { key: 'actions', label: 'Acciones' }] : [])
  ]);
  const filters = $derived([
    { value: 'all', label: 'Todos' },
    { value: 'out', label: `Agotados (${counts.out})` },
    { value: 'low', label: `Poco stock (${counts.low})` }
  ]);
</script>

<PageHeader title="Inventario" description={headline}>
  {#snippet actions()}
    {#if admin}<button class="btn" onclick={startNew}>Nuevo producto</button>{/if}
  {/snippet}
</PageHeader>

{#if error && !stockOpen && !formOpen}
  <div class="notice"><InlineNotice tone="danger" message={error} /></div>
{/if}

{#if loading}
  <Surface padding="flush"><LoadingState /></Surface>
{:else if products.length === 0}
  <Surface padding="flush">
    <EmptyState title="Todavía no hay productos" description={admin ? 'Agrega el primero con «Nuevo producto».' : ''}>
      {#snippet actions()}{#if admin}<button class="btn" onclick={startNew}>Nuevo producto</button>{/if}{/snippet}
    </EmptyState>
  </Surface>
{:else}
  <div class="kpis" role="group" aria-label="Resumen del inventario">
    <MetricCard label="Productos" value={live.length} hint="{counts.withStock} con existencia" />
    <MetricCard label="Agotados" value={counts.out} hint="sin existencia" />
    <MetricCard label="Poco stock" value={counts.low} hint="en o bajo su mínimo" />
    {#if admin}<MetricCard label="Valor en anaquel" value={money(stockValue.total)} hint={stockValue.missing ? `${stockValue.missing} sin costo` : 'a precio de costo'} />{/if}
  </div>

  <FilterBar label="Filtrar inventario">
    <SegmentedControl bind:value={filter} options={filters} label="Mostrar" />
    <SearchField bind:value={query} label="Buscar producto o código" placeholder="Buscar producto o código" />
    <label class="sort"><span class="sr-only">Ordenar</span>
      <select bind:value={sort} aria-label="Ordenar">
        <option value="urgent">Lo que falta</option>
        <option value="name">A–Z</option>
        {#if admin}<option value="value">Más dinero</option>{/if}
      </select>
    </label>
  </FilterBar>

  <Surface padding="flush">
    {#if shown.length}
      <DataTable caption="Productos y existencias" {columns} rows={shown.slice(0, limit)}>
        {#snippet cell({ row, column })}
          {#if column.key === 'name'}
            <span class="name" class:off={!row.active}>{row.name}</span>
            {#if row.barcode}<small class="code">{row.barcode}</small>{/if}
          {:else if column.key === 'price'}
            {money(row.price_cents)}
            {#if row.cost_cents !== undefined}<small class="code">costo {money(row.cost_cents)}</small>{/if}
          {:else if column.key === 'stock'}
            <span class="lvl" title={`Existencia ${row.stock} · mínimo ${row.min_stock}`}>
              <span class="num"><b class="qty" class:out={row.state === 'out'} class:low={row.state === 'low'}>{row.stock}</b><small>{row.min_stock ? `mín. ${row.min_stock}` : 'sin mínimo'}</small></span>
              <span class="meter" class:out={row.state === 'out'} class:low={row.state === 'low'} aria-hidden="true">
                <i style:width="{row.fill}%"></i>
                {#if row.min_stock}<em style:left="{row.minPos}%"></em>{/if}
              </span>
            </span>
          {:else if column.key === 'status'}
            <StatusBadge label={STATE[row.state][0]} tone={STATE[row.state][1]} />
          {:else if column.key === 'value'}
            {row.cost_cents ? money(row.value) : '—'}
          {:else}
            <span class="acts">
              <button class="lnk" aria-label={`Ajustar existencia de ${row.name}`} onclick={() => openStock(row)}>Existencia</button>
              <button class="lnk" aria-label={`Editar ${row.name}`} onclick={() => startEdit(row)}>Editar</button>
              <button class="lnk quiet" onclick={() => toggle(row)}>{row.active ? 'Desactivar' : 'Activar'}</button>
            </span>
          {/if}
        {/snippet}
      </DataTable>
      {#if shown.length > limit}
        <p class="more"><button class="btn ghost sm" onclick={() => (limit += 60)}>Ver {Math.min(60, shown.length - limit)} más</button></p>
      {/if}
    {:else}
      <EmptyState title="Nada coincide" description="Prueba con otra búsqueda o quita el filtro." />
    {/if}
  </Surface>
{/if}

<Modal bind:open={formOpen} title={editing === null ? 'Nuevo producto' : 'Editar producto'}>
  <form onsubmit={save}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Precio de venta (MXN) <input inputmode="decimal" bind:value={form.price} placeholder="120.00" required /></label>
      <label class="field">Costo (MXN) <input inputmode="decimal" bind:value={form.cost} placeholder="50.00" /></label>
      {#if editing === null}
        <label class="field">Existencia inicial <input type="number" min="0" step="1" bind:value={form.stock} required /></label>
      {/if}
      <label class="field">Existencia mínima <input type="number" min="0" step="1" bind:value={form.min} /><span class="hint">Avisa «poco stock» cuando llegue a esta cantidad. 0 = sin aviso.</span></label>
      <label class="field">Código de barras <input bind:value={form.barcode} maxlength="64" autocomplete="off" placeholder="Opcional" /><span class="hint">Con un lector, escanéalo en Cobrar para agregarlo al ticket.</span></label>
    </div>
    {#if editing !== null}<p class="hint">Para cambiar la existencia usa «Existencia» en la lista.</p>{/if}
    <div class="actions">
      <button type="button" class="btn ghost" onclick={() => (formOpen = false)}>Cancelar</button>
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
    </div>
  </form>
</Modal>

<Modal bind:open={stockOpen} title={target ? `Existencia · ${target.name}` : 'Existencia'}>
  {#if target}
    <p class="now">Ahora hay <b>{target.stock}</b> {target.stock === 1 ? 'pieza' : 'piezas'}{#if target.min_stock} · mínimo {target.min_stock}{/if}.</p>
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
  .name { display: inline-block; min-width: 150px; font-weight: 600; }
  :global(.search input[type='search']) { padding-left: 38px !important; }
  .name.off { color: var(--muted); }
  .code { display: block; color: var(--muted); font-size: 0.8rem; }
  .sort select { width: auto; min-width: 150px; margin: 0; min-height: 44px; }
  .more { margin: 0; padding: 12px; text-align: center; border-top: 1px solid var(--border); }
  .lvl { display: grid; gap: 6px; min-width: 110px; }
  .num { display: flex; align-items: baseline; justify-content: flex-start; gap: 8px; }
  .qty { font-size: 1.1rem; font-variant-numeric: tabular-nums; }
  .qty.out { color: var(--danger); }
  .qty.low { color: var(--clay); }
  .num small { color: var(--muted); font-size: 0.8rem; }
  .meter { position: relative; display: block; height: 6px; border-radius: 999px; background: var(--surface-2); overflow: hidden; }
  .meter i { position: absolute; inset: 0 auto 0 0; border-radius: 999px; background: var(--ok); }
  .meter.out i { background: var(--danger); }
  .meter.low i { background: var(--clay); }
  .meter em { position: absolute; top: 0; bottom: 0; width: 2px; background: var(--text); opacity: 0.6; }
  .acts { display: inline-flex; flex-wrap: nowrap; gap: 0 4px; justify-content: flex-end; }
  @media (max-width: 1100px) and (min-width: 641px) { .acts { flex-direction: column; align-items: flex-end; gap: 0; } }
  @media (max-width: 640px) { .lvl { min-width: 0; width: 55%; } .name { min-width: 0; } .acts { justify-content: flex-start; flex-wrap: wrap; } }
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
  @media (max-width: 640px) { .kpis :global(.value) { font-size: 1.4rem; white-space: nowrap; } }
</style>
