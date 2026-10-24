<script>
  import { api } from '$lib/api.js';
  import { money, toCents } from '$lib/format.js';

  // Gastos del mes: lo que cuesta operar la plataforma. `books` = respuesta de /api/platform/finance; `onchanged` recarga las cifras.
  let { books, onchanged } = $props();

  let label = $state('');
  let amount = $state('');
  let note = $state('');
  let saving = $state(false);
  let error = $state('');
  let confirmId = $state(null);

  async function add(e) {
    e.preventDefault();
    error = '';
    const cents = toCents(amount);
    if (!Number.isFinite(cents) || cents <= 0) return (error = 'Escribe el monto en pesos, por ejemplo 350.');
    saving = true;
    try {
      await api('/api/platform/expenses', { method: 'POST', body: { label: label.trim(), amount_cents: cents, note: note.trim() } });
      label = amount = note = '';
      await onchanged();
    } catch (err) {
      error = err.message;
    } finally {
      saving = false;
    }
  }

  async function remove(row) {
    if (confirmId !== row.id) return (confirmId = row.id);
    error = '';
    try {
      await api(`/api/platform/expenses/${row.id}`, { method: 'DELETE' });
      confirmId = null;
      await onchanged();
    } catch (err) {
      error = err.message;
    }
  }
</script>

<div class="kpis">
  <div class="kpi">
    <span class="lbl">Gastos del mes</span>
    <span class="body"><b class="v">{money(books.expenses_cents)}</b><span class="sub">todo lo anotado</span></span>
  </div>
  <div class="kpi">
    <span class="lbl">Anotados</span>
    <span class="body"><b class="v">{books.expenses.length}</b><span class="sub">este mes</span></span>
  </div>
</div>

<section class="card">
  <h2>Anotar un gasto</h2>
  <p class="meta">Servidor, dominio, comisiones… lo que cuesta operar Barbella. Se resta de lo cobrado para calcular la ganancia.</p>
  <form onsubmit={add}>
    <div class="form-grid">
      <label class="field">Concepto <input bind:value={label} required minlength="2" maxlength="80" placeholder="Servidor, dominio, comisión…" /></label>
      <label class="field">Monto (MXN) <input bind:value={amount} inputmode="decimal" required placeholder="0.00" /></label>
      <label class="field">Nota <span class="meta">(opcional)</span> <input bind:value={note} maxlength="200" /></label>
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn" disabled={saving}>{saving ? 'Guardando…' : 'Agregar'}</button>
  </form>

  {#if books.expenses.length === 0}
    <p class="meta empty-note">No hay gastos anotados este mes.</p>
  {:else}
    <ul>
      {#each books.expenses as row (row.id)}
        <li>
          <span class="who"><b>{row.label}</b><span class="meta">{row.day}{row.note ? ` · ${row.note}` : ''}</span></span>
          <b class="amt">{money(row.amount_cents)}</b>
          <button type="button" class="btn ghost sm" aria-label="Quitar el gasto {row.label}" onclick={() => remove(row)}>{confirmId === row.id ? '¿Seguro?' : 'Quitar'}</button>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  h2 { font-size: 1.2rem; margin-bottom: 0.2rem; }
  .meta { color: var(--muted); font-size: 0.85rem; }
  form { margin-top: 1rem; }
  .empty-note { margin: 1.25rem 0 0; }
  ul { list-style: none; margin: 1.25rem 0 0; padding: 0; display: grid; gap: 8px; }
  li { display: flex; align-items: center; gap: 12px; padding: 0.7rem 0.9rem; border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .who { flex: 1; display: grid; min-width: 0; }
  .amt { font-variant-numeric: tabular-nums; }
</style>
