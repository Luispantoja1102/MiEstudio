<script>
  import { onMount } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import { api } from '$lib/api.js';
  import { METHODS, formatDate, money, toCents, plural } from '$lib/format.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';

  let cash = $state(null);
  let history = $state([]);
  let ready = $state(false);
  let error = $state('');
  let notice = $state('');
  let opening = $state('');
  let move = $state({ kind: 'out', amount: '', concept: '' });
  let counted = $state('');
  let closeNote = $state('');
  let closing = $state(false);

  async function load() {
    if (!branches.id) return (ready = true);
    const [c, h] = await Promise.all([
      api(`/api/cash/current?branch_id=${branches.id}`),
      api(`/api/cash/sessions?branch_id=${branches.id}`)
    ]);
    cash = c.session;
    history = h.filter((s) => s.closed_at);
    ready = true;
  }

  onMount(async () => {
    try {
      await loadBranches();
      await load();
    } catch (e) {
      error = e.message;
      ready = true;
    }
  });

  async function act(fn, ok) {
    error = '';
    notice = '';
    try {
      await fn();
      if (ok) notice = ok; // si no hay mensaje fijo, se conserva el que puso la acción (p. ej. el resultado del corte)
      await load();
    } catch (e) {
      error = e.message;
    }
  }

  const open = (e) => {
    e.preventDefault();
    const c = toCents(opening || '0');
    if (Number.isNaN(c)) return (error = 'Escribe un monto válido');
    return act(() => api('/api/cash/open', { method: 'POST', body: { branch_id: branches.id, opening_cents: c } }), 'Caja abierta.').then(() => (opening = ''));
  };
  const addMove = (e) => {
    e.preventDefault();
    const c = toCents(move.amount);
    if (!c) return (error = 'Escribe un monto válido');
    return act(() => api('/api/cash/movements', { method: 'POST', body: { branch_id: branches.id, kind: move.kind, amount_cents: c, concept: move.concept } }), 'Movimiento registrado.').then(
      () => (move = { kind: move.kind, amount: '', concept: '' })
    );
  };
  const close = (e) => {
    e.preventDefault();
    const c = toCents(counted);
    if (Number.isNaN(c)) return (error = 'Escribe el efectivo contado');
    return act(async () => {
      const r = await api('/api/cash/close', { method: 'POST', body: { branch_id: branches.id, counted_cents: c, note: closeNote } });
      notice = r.diff_cents === 0 ? 'Caja cerrada: cuadra exacto.' : `Caja cerrada con ${r.diff_cents > 0 ? 'sobrante' : 'faltante'} de ${money(Math.abs(r.diff_cents))}.`;
    }, '').then(() => ((counted = ''), (closeNote = ''), (closing = false)));
  };
  const METHOD_COLS = [
    { key: 'method', label: 'Método' },
    { key: 'count', label: 'Pagos', numeric: true },
    { key: 'amount_cents', label: 'Importe', numeric: true }
  ];
  const MOVE_COLS = [
    { key: 'concept', label: 'Concepto' },
    { key: 'created_at', label: 'Fecha' },
    { key: 'amount_cents', label: 'Monto', numeric: true }
  ];
  const HIST_COLS = [
    { key: 'opened_at', label: 'Apertura' },
    { key: 'opened_by', label: 'Abrió' },
    { key: 'sales', label: 'Ventas', numeric: true },
    { key: 'expected_cents', label: 'Esperado', numeric: true },
    { key: 'counted_cents', label: 'Contado', numeric: true },
    { key: 'diff_cents', label: 'Diferencia', numeric: true }
  ];
  const diffLabel = (d) => (d === 0 ? 'Cuadra' : d > 0 ? `+${money(d)}` : `−${money(-d)}`);
</script>

<PageHeader title="Caja" description="Apertura, entradas y salidas de efectivo, y corte del día.">
  {#snippet actions()}<BranchPicker onchange={load} />{/snippet}
</PageHeader>

{#if error}<InlineNotice tone="danger" message={error} />{/if}
{#if notice}<InlineNotice tone="success" message={notice} />{/if}

{#if !ready}
  <LoadingState />
{:else if !cash}
  <div class="narrow">
    <Surface title="Caja cerrada" description="Escribe el efectivo con el que arrancas el día para abrir la caja.">
      <form onsubmit={open}>
        <label class="field">Efectivo inicial (MXN) <input inputmode="decimal" bind:value={opening} placeholder="0.00" /></label>
        <button class="btn">Abrir caja</button>
      </form>
    </Surface>
  </div>
{:else}
  <p class="since">Caja abierta desde {formatDate(cash.opened_at)} · abrió {cash.opened_by}</p>
  <div class="metrics">
    <MetricCard label="Efectivo esperado en caja" value={money(cash.expected_cents)} hint="Inicial + ventas en efectivo + entradas − salidas" />
    <MetricCard label="Efectivo inicial" value={money(cash.opening_cents)} />
    <MetricCard label="Ventas en efectivo" value={money(cash.cash_in_cents)} />
    <MetricCard label="Entradas y salidas" value={`+${money(cash.moves_in_cents)}`} hint={`Salidas −${money(cash.moves_out_cents)}`} />
  </div>

  <div class="two">
    <Surface
      title="Ventas de esta caja"
      description={`${plural(cash.sales, 'venta', 'ventas')} · ${money(cash.sales_cents)}${cash.tip_cents ? ` + ${money(cash.tip_cents)} de propinas` : ''}`}
      padding="flush"
    >
      {#if cash.by_method.length}
        <DataTable caption="Ventas de esta caja por método de pago" columns={METHOD_COLS} rows={cash.by_method} rowKey="method">
          {#snippet cell({ row, column })}
            {#if column.key === 'method'}{METHODS[row.method]}
            {:else if column.key === 'amount_cents'}<b>{money(row.amount_cents)}</b>
            {:else}{row[column.key]}{/if}
          {/snippet}
        </DataTable>
      {:else}
        <EmptyState title="Todavía no hay ventas" />
      {/if}
    </Surface>

    <Surface title="Movimientos de efectivo" description="Entradas y salidas registradas en esta caja." padding="flush">
      {#if cash.movements.length}
        <DataTable caption="Movimientos de efectivo" columns={MOVE_COLS} rows={cash.movements}>
          {#snippet cell({ row, column })}
            {#if column.key === 'concept'}{row.concept}<small class="sub">{row.by}</small>
            {:else if column.key === 'created_at'}<span class="nw">{formatDate(row.created_at)}</span>
            {:else}<b>{row.kind === 'in' ? '+' : '−'}{money(row.amount_cents)}</b>{/if}
          {/snippet}
        </DataTable>
      {:else}
        <EmptyState title="Sin movimientos" description="Registra aquí gastos, retiros o fondo de cambio." />
      {/if}
    </Surface>
  </div>

  <div class="two forms">
    <Surface title="Entrada o salida de efectivo" description="Gastos, propinas pagadas, retiros, fondo o cambio.">
      <form onsubmit={addMove}>
        <label class="field">Tipo
          <select bind:value={move.kind}><option value="out">Salida (gasto, propinas pagadas, retiro)</option><option value="in">Entrada (fondo, cambio)</option></select>
        </label>
        <label class="field">Monto (MXN) <input inputmode="decimal" bind:value={move.amount} required /></label>
        <label class="field">Concepto <input bind:value={move.concept} minlength="2" maxlength="200" placeholder="Ej. Compra de toallas" required /></label>
        <button class="btn ghost">Registrar</button>
      </form>
    </Surface>

    <Surface title={closing ? 'Corte de caja' : 'Cerrar caja'} description={closing ? '' : `Cuenta el efectivo y compáralo con lo esperado (${money(cash.expected_cents)}).`}>
      {#if !closing}
        <button class="btn" onclick={() => (closing = true)}>Hacer corte</button>
      {:else}
        <form onsubmit={close}>
          <label class="field">Efectivo contado (MXN) <input inputmode="decimal" bind:value={counted} required /></label>
          {#if counted && !Number.isNaN(toCents(counted))}
            {@const d = toCents(counted) - cash.expected_cents}
            <p class="balance" class:bad={d !== 0} role="status">Esperado {money(cash.expected_cents)} · {diffLabel(d)}</p>
          {/if}
          <label class="field">Nota (opcional) <input bind:value={closeNote} maxlength="500" /></label>
          <div class="form-actions">
            <button class="btn">Cerrar caja</button>
            <button type="button" class="btn ghost" onclick={() => (closing = false)}>Cancelar</button>
          </div>
        </form>
      {/if}
    </Surface>
  </div>
{/if}

{#if history.length}
  <div class="hist">
    <Surface title="Cortes anteriores" padding="flush">
      <DataTable caption="Cortes anteriores" columns={HIST_COLS} rows={history}>
        {#snippet cell({ row, column })}
          {#if column.key === 'opened_at'}<span class="nw">{formatDate(row.opened_at)}</span>
          {:else if column.key === 'sales'}{row.sales}<small class="sub">{money(row.sales_cents)}</small>
          {:else if column.key === 'diff_cents'}<StatusBadge label={diffLabel(row.diff_cents)} tone={row.diff_cents === 0 ? 'success' : 'warning'} />
          {:else if column.key === 'opened_by'}<span class="nw">{row.opened_by}</span>
          {:else}{money(row[column.key])}{/if}
        {/snippet}
      </DataTable>
    </Surface>
  </div>
{/if}

<style>
  .narrow { max-width: 440px; }
  .since { margin: 0 0 16px; color: var(--muted); font-size: 0.9rem; }
  .metrics { display: grid; gap: 16px; grid-template-columns: 1fr; margin-bottom: 16px; }
  @media (min-width: 560px) { .metrics { grid-template-columns: repeat(2, 1fr); } }
  @media (min-width: 1000px) { .metrics { grid-template-columns: repeat(4, 1fr); } }
  .two { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(min(100%, 480px), 1fr)); margin-bottom: 16px; align-items: start; }
  .hist { margin-top: 24px; }
  .nw { white-space: nowrap; }
  .sub { display: block; color: var(--muted); font-size: 0.8rem; }
  .balance { margin: 0 0 1.1rem; font-weight: 600; color: var(--success); font-variant-numeric: tabular-nums; }
  .balance.bad { color: var(--danger); }
</style>
