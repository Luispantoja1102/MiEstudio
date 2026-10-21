<script>
  import { onMount } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import { api } from '$lib/api.js';
  import { METHODS, formatDate, money, toCents } from '$lib/format.js';
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
      notice = ok;
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
  const diffLabel = (d) => (d === 0 ? 'Cuadra' : d > 0 ? `+${money(d)}` : `−${money(-d)}`);
</script>

<div class="page-head">
  <div>
    <h1>Caja</h1>
    <p>Apertura, entradas y salidas de efectivo, y corte del día.</p>
  </div>
  <BranchPicker onchange={load} />
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}
{#if notice}<p class="banner" role="status" style="background: var(--accent-soft); color: var(--accent)">{notice}</p>{/if}

{#if !ready}
  <p class="meta">Cargando…</p>
{:else if !cash}
  <form class="card" onsubmit={open} style="max-width: 420px">
    <h2>Caja cerrada</h2>
    <label class="field">Efectivo inicial (MXN) <input inputmode="decimal" bind:value={opening} placeholder="0.00" /></label>
    <button class="btn">Abrir caja</button>
  </form>
{:else}
  <section class="card">
    <h2>Caja abierta</h2>
    <p class="meta">Desde {formatDate(cash.opened_at)} · abrió {cash.opened_by}</p>
    <dl>
      <div><dt>Efectivo inicial</dt><dd>{money(cash.opening_cents)}</dd></div>
      <div><dt>Ventas en efectivo</dt><dd>{money(cash.cash_in_cents)}</dd></div>
      <div><dt>Entradas</dt><dd>{money(cash.moves_in_cents)}</dd></div>
      <div><dt>Salidas</dt><dd>−{money(cash.moves_out_cents)}</dd></div>
      <div class="total"><dt>Efectivo esperado en caja</dt><dd>{money(cash.expected_cents)}</dd></div>
    </dl>
    <h3>Ventas de esta caja · {cash.sales} · {money(cash.sales_cents)}{#if cash.tip_cents} + {money(cash.tip_cents)} de propinas{/if}</h3>
    <ul class="methods">
      {#each cash.by_method as m}<li><span>{METHODS[m.method]}</span><b>{money(m.amount_cents)}</b><small>{m.count} pagos</small></li>{:else}<li class="meta">Todavía no hay ventas.</li>{/each}
    </ul>
  </section>

  <section class="card" style="margin-top: 1rem">
    <h3>Entrada o salida de efectivo</h3>
    <form onsubmit={addMove}>
      <div class="form-grid">
        <label class="field">Tipo
          <select bind:value={move.kind}><option value="out">Salida (gasto, propinas pagadas, retiro)</option><option value="in">Entrada (fondo, cambio)</option></select>
        </label>
        <label class="field">Monto (MXN) <input inputmode="decimal" bind:value={move.amount} required /></label>
        <label class="field">Concepto <input bind:value={move.concept} minlength="2" maxlength="200" placeholder="Ej. Compra de toallas" required /></label>
      </div>
      <button class="btn ghost">Registrar</button>
    </form>
    {#if cash.movements.length}
      <ul class="methods" style="margin-top: 1rem">
        {#each cash.movements as m (m.id)}
          <li><span>{m.concept}<small> · {m.by}</small></span><b>{m.kind === 'in' ? '+' : '−'}{money(m.amount_cents)}</b><small>{formatDate(m.created_at)}</small></li>
        {/each}
      </ul>
    {/if}
  </section>

  <section class="card" style="margin-top: 1rem">
    {#if !closing}
      <h3>Cerrar caja</h3>
      <p class="meta">Cuenta el efectivo y compáralo con lo esperado ({money(cash.expected_cents)}).</p>
      <button class="btn" onclick={() => (closing = true)}>Hacer corte</button>
    {:else}
      <h3>Corte de caja</h3>
      <form onsubmit={close}>
        <label class="field">Efectivo contado (MXN) <input inputmode="decimal" bind:value={counted} required /></label>
        {#if counted && !Number.isNaN(toCents(counted))}
          <p class="balance" class:bad={toCents(counted) !== cash.expected_cents}>
            Esperado {money(cash.expected_cents)} · {diffLabel(toCents(counted) - cash.expected_cents)}
          </p>
        {/if}
        <label class="field">Nota (opcional) <input bind:value={closeNote} maxlength="500" /></label>
        <div class="form-actions">
          <button class="btn">Cerrar caja</button>
          <button type="button" class="btn ghost" onclick={() => (closing = false)}>Cancelar</button>
        </div>
      </form>
    {/if}
  </section>
{/if}

{#if history.length}
  <section style="margin-top: 2rem">
    <h2>Cortes anteriores</h2>
    <div class="list">
      {#each history as s (s.id)}
        <div class="item-row">
          <div class="grow"><div class="title">{formatDate(s.opened_at)}</div><div class="meta">{s.opened_by} · {s.sales} ventas · {money(s.sales_cents)}</div></div>
          <span class="meta">Esperado {money(s.expected_cents)} · contado {money(s.counted_cents)}</span>
          <span class="badge" class:warn={s.diff_cents !== 0}>{diffLabel(s.diff_cents)}</span>
        </div>
      {/each}
    </div>
  </section>
{/if}

<style>
  h2 { font-size: 1.4rem; }
  h3 { font-size: 1.1rem; margin: 1rem 0 0.5rem; }
  .card h3:first-child { margin-top: 0; }
  dl { display: grid; gap: 6px; margin: 1rem 0; }
  dl div { display: flex; justify-content: space-between; font-variant-numeric: tabular-nums; }
  dt { color: var(--muted); }
  dd { margin: 0; font-weight: 600; }
  .total { padding-top: 8px; border-top: 1px solid var(--border); font-size: 1.15rem; }
  .total dt { color: var(--text); font-weight: 600; }
  .methods { list-style: none; margin: 0; padding: 0; display: grid; gap: 6px; }
  .methods li { display: flex; gap: 12px; align-items: baseline; justify-content: space-between; font-variant-numeric: tabular-nums; }
  .methods li span { flex: 1; }
  .methods small { color: var(--muted); }
  .balance { font-weight: 600; color: var(--accent); }
  .balance.bad { color: var(--danger); }
</style>
