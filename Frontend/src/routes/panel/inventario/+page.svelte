<script>
  import { onMount } from 'svelte';
  import Modal from '$lib/Modal.svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { formatDate, money, toCents, toPesos } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';

  const admin = session.me.role === 'admin';
  const REASON = { initial: 'Existencia inicial', purchase: 'Compra', sale: 'Venta', void: 'Venta cancelada', adjustment: 'Ajuste', loss: 'Pérdida o merma' };
  const STATE = { out: ['Agotado', 'bad'], low: ['Poco stock', 'warn'], ok: ['', ''], off: ['Inactivo', 'muted'] };
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
</script>

<div class="page-head">
  <div>
    <h1>Inventario</h1>
    <p>{headline}</p>
  </div>
  {#if admin}<button class="btn" onclick={startNew}>Nuevo producto</button>{/if}
</div>

{#if error && !stockOpen && !formOpen}<p class="error" role="alert">{error}</p>{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else if products.length === 0}
  <div class="empty">Todavía no hay productos.{#if admin} Agrega el primero con «Nuevo producto».{/if}</div>
{:else}
  <div class="kpis inv" role="group" aria-label="Resumen del inventario">
    <button class="kpi pick" class:on={filter === 'all'} aria-pressed={filter === 'all'} onclick={() => (filter = 'all')}>
      <span class="lbl">Productos</span>
      <span class="v">{live.length}</span>
      <span class="sub">{counts.withStock} con existencia</span>
    </button>
    <button class="kpi pick bad" class:on={filter === 'out'} aria-pressed={filter === 'out'} onclick={() => (filter = filter === 'out' ? 'all' : 'out')}>
      <span class="lbl">Agotados</span>
      <span class="v">{counts.out}</span>
      <span class="sub">sin existencia</span>
    </button>
    <button class="kpi pick warn" class:on={filter === 'low'} aria-pressed={filter === 'low'} onclick={() => (filter = filter === 'low' ? 'all' : 'low')}>
      <span class="lbl">Poco stock</span>
      <span class="v">{counts.low}</span>
      <span class="sub">en o bajo su mínimo</span>
    </button>
    {#if admin}
      <div class="kpi">
        <span class="lbl">Valor en anaquel</span>
        <span class="v">{money(stockValue.total)}</span>
        <span class="sub">{stockValue.missing ? `${stockValue.missing} sin costo` : 'a precio de costo'}</span>
      </div>
    {/if}
  </div>

  <div class="tools">
    <label class="search">
      <Icon name="search" size={17} />
      <input type="search" placeholder="Buscar producto o código" aria-label="Buscar producto o código" bind:value={query} />
    </label>
    <select bind:value={sort} aria-label="Ordenar">
      <option value="urgent">Lo que falta</option>
      <option value="name">A–Z</option>
      {#if admin}<option value="value">Más dinero</option>{/if}
    </select>
  </div>

  {#if shown.length}
    <ul class="stock-list">
      {#each shown.slice(0, limit) as p (p.id)}
        <li class="stock-row {p.state}">
          <div class="st-main">
            <strong>{p.name}</strong>
            <small>{money(p.price_cents)}{p.barcode ? ` · ${p.barcode}` : ''}</small>
          </div>
          <div class="st-level" title={`Existencia ${p.stock} · mínimo ${p.min_stock}`}>
            <div class="st-num"><b>{p.stock}</b><small>{p.min_stock ? `mín. ${p.min_stock}` : 'sin mínimo'}</small></div>
            <div class="st-meter" aria-hidden="true">
              <i style:width="{p.fill}%"></i>
              {#if p.min_stock}<span style:left="{p.minPos}%"></span>{/if}
            </div>
          </div>
          {#if STATE[p.state][0]}<span class="badge {STATE[p.state][1]}">{STATE[p.state][0]}</span>{/if}
          {#if admin}
            <b class="st-value">{p.cost_cents ? money(p.value) : '—'}</b>
            <div class="st-acts">
              <button class="btn ghost sm" aria-label={`Ajustar existencia de ${p.name}`} onclick={() => openStock(p)}>Existencia</button>
              <button class="btn ghost sm" aria-label={`Editar ${p.name}`} onclick={() => startEdit(p)}>Editar</button>
              <button class="btn ghost sm" onclick={() => toggle(p)}>{p.active ? 'Desactivar' : 'Activar'}</button>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
    {#if shown.length > limit}
      <p style="text-align: center"><button class="btn ghost sm" onclick={() => (limit += 60)}>Ver {Math.min(60, shown.length - limit)} más</button></p>
    {/if}
  {:else}
    <div class="empty">Nada coincide. Prueba con otra búsqueda o quita el filtro.</div>
  {/if}
{/if}

<Modal bind:open={formOpen} title={editing === null ? 'Nuevo producto' : 'Editar producto'}>
  <form onsubmit={save}>
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
    {#if editing !== null}<p class="meta">Para cambiar la existencia usa «Existencia» en la lista.</p>{/if}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      <button type="button" class="btn ghost" onclick={() => (formOpen = false)}>Cancelar</button>
    </div>
  </form>
</Modal>

<Modal bind:open={stockOpen} title={target ? `Existencia · ${target.name}` : 'Existencia'}>
  {#if target}
    <p class="meta">Ahora hay <b>{target.stock}</b> piezas{#if target.min_stock} · mínimo {target.min_stock}{/if}.</p>
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
          <span class="grow">{REASON[m.reason]}<small class="meta">{' · '}{m.by || 'Sistema'}{m.note ? ' · ' + m.note : ''}</small><br /><small class="meta">{formatDate(m.created_at)}</small></span>
          <b style="color: {m.delta > 0 ? 'var(--accent)' : 'var(--danger)'}">{m.delta > 0 ? '+' : ''}{m.delta}</b>
          <small class="meta">saldo {m.balance}</small>
        </div>
      {:else}<p class="meta">Sin movimientos.</p>{/each}
    </div>
  {/if}
</Modal>

<style>
  /* tarjetas de cifras que también filtran la lista */
  .kpi.pick { text-align: left; font: inherit; cursor: pointer; transition: border-color var(--dur-ui) var(--ease), background-color var(--dur-ui) var(--ease); }
  .kpi.pick:hover { border-color: var(--muted); }
  .kpi.pick.on { border-color: var(--accent); background: var(--accent-soft); }
  .kpis.inv { grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); }
  .kpi > span { display: block; }
  .kpi.bad .v { color: var(--danger); }
  .kpi.warn .v { color: var(--clay); }

  .tools { display: flex; flex-wrap: wrap; gap: 10px; margin-bottom: 1rem; }
  .search { display: flex; flex: 1 1 240px; align-items: center; gap: 8px; padding: 0 0.75rem; color: var(--muted); background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .search:focus-within { border-color: var(--accent); }
  .search input[type='search'], .search input[type='search']:hover, .search input[type='search']:focus { flex: 1; min-width: 0; min-height: 44px; margin: 0; padding: 0; border: 0; outline: 0; box-shadow: none; background: transparent; }
  .tools select { width: auto; min-width: 150px; margin: 0; }

  .stock-list { container-type: inline-size; list-style: none; margin: 0; padding: 0; display: grid; gap: 10px; }
  .stock-row {
    display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(150px, 1fr) auto auto auto; align-items: center; gap: 8px 16px;
    padding: 0.75rem 1rem; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius);
  }
  .stock-row.off { opacity: 0.6; }
  .stock-row.out { border-left: 4px solid var(--danger); }
  .stock-row.low { border-left: 4px solid var(--clay); }
  .st-main { display: grid; min-width: 0; }
  .st-main strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .st-main small, .st-num small { color: var(--muted); }
  .st-level { display: grid; gap: 6px; }
  .st-num { display: flex; align-items: baseline; gap: 8px; }
  .st-num b { font-size: 1.25rem; font-variant-numeric: tabular-nums; }
  .st-meter { position: relative; height: 8px; border-radius: 999px; background: var(--surface-2); overflow: hidden; }
  .st-meter i { position: absolute; inset: 0 auto 0 0; border-radius: 999px; background: var(--ok); }
  .out .st-meter i { background: var(--danger); }
  .low .st-meter i { background: var(--clay); }
  .st-meter span { position: absolute; top: 0; bottom: 0; width: 2px; background: var(--text); opacity: 0.6; }
  .st-value { font-variant-numeric: tabular-nums; min-width: 4.5rem; text-align: right; }
  .st-acts { display: flex; flex-wrap: wrap; gap: 6px; justify-content: flex-end; }
  /* en espacios angostos cada fila baja a dos líneas: nombre y estado arriba, nivel y botones abajo */
  @container (max-width: 820px) {
    .stock-row { grid-template-columns: minmax(0, 1fr) auto auto; }
    .st-level { grid-column: 1 / -1; grid-row: 2; }
    .st-acts { grid-column: 1 / -1; justify-content: flex-start; }
    .st-value { min-width: 0; }
  }
</style>
