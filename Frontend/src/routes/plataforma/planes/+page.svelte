<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { pesos } from '$lib/format.js';
  import { canWritePlatform } from '$lib/session.svelte.js';

  const write = canWritePlatform();
  let plans = $state([]);
  let error = $state('');
  let notice = $state('');
  let editing = $state(null); // id del plan que se edita
  let form = $state(null);
  let busy = $state(false);

  async function load() {
    try {
      plans = await api('/api/platform/plans');
    } catch (e) {
      error = e.message;
    }
  }
  onMount(load);
  const lim = (n) => (n === null ? 'Sin límite' : n);

  function edit(p) {
    notice = error = '';
    editing = p.id;
    form = { description: p.description, price_month: p.price_month, price_year: p.price_year, unlimited: p.max_staff === null, max_staff: p.max_staff ?? 6,
      max_branches: p.max_branches, pos: p.pos, deposits: p.deposits, terminal: p.terminal, support: p.support };
  }

  async function save(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      await api(`/api/platform/plans/${editing}`, {
        method: 'PUT',
        body: { description: form.description, price_month: +form.price_month, price_year: +form.price_year, max_staff: form.unlimited ? null : +form.max_staff,
          max_branches: +form.max_branches, pos: form.pos, deposits: form.deposits, terminal: form.terminal, support: form.support }
      });
      notice = 'Plan guardado. Ya se ve así en la página de inicio, en la suscripción de los negocios y en los cobros nuevos.';
      editing = null;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function reset(p) {
    if (!confirm(`¿Volver el plan ${p.name} a su configuración original?`)) return;
    error = '';
    try {
      await api(`/api/platform/plans/${p.id}`, { method: 'DELETE' });
      notice = `El plan ${p.name} volvió a su configuración original.`;
      editing = null;
      await load();
    } catch (err) {
      error = err.message;
    }
  }
</script>

<div class="page-head">
  <div>
    <h1>Planes</h1>
    <p>Lo que incluye cada suscripción y cuántos negocios la usan.{#if write} Edita precios y límites aquí: se aplican de inmediato.{/if}</p>
  </div>
</div>
{#if error}<p class="error" role="alert">{error}</p>{/if}
{#if notice}<p class="banner" role="status">{notice}</p>{/if}

<div class="plans">
  {#each plans as p (p.id)}
    <div class="card plan">
      {#if editing === p.id}
        <form onsubmit={save}>
          <h2>{p.name}</h2>
          <label class="field">Descripción <input bind:value={form.description} maxlength="120" /></label>
          <div class="two">
            <label class="field">Precio al mes (MXN) <input type="number" min="0" bind:value={form.price_month} required /></label>
            <label class="field">Precio al año (MXN) <input type="number" min="0" bind:value={form.price_year} required /></label>
          </div>
          <div class="two">
            <label class="field">Personas en agenda
              <input type="number" min="1" bind:value={form.max_staff} disabled={form.unlimited} />
            </label>
            <label class="field">Sucursales <input type="number" min="1" bind:value={form.max_branches} required /></label>
          </div>
          <label class="check"><input type="checkbox" bind:checked={form.unlimited} /> Sin límite de personas</label>
          <label class="check"><input type="checkbox" bind:checked={form.pos} /> Caja, ventas, inventario, comisiones y reportes</label>
          <label class="check"><input type="checkbox" bind:checked={form.deposits} /> Anticipos en la reserva en línea</label>
          <label class="check"><input type="checkbox" bind:checked={form.terminal} /> Cobro con terminal de Mercado Pago</label>
          <label class="field" style="margin-top: 0.9rem">Soporte
            <select bind:value={form.support}><option value="correo">Por correo</option><option value="prioritario">Prioritario</option><option value="dedicado">Dedicado</option></select>
          </label>
          <div class="form-actions">
            <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar plan'}</button>
            <button type="button" class="btn ghost" onclick={() => (editing = null)}>Cancelar</button>
          </div>
          <button type="button" class="link" onclick={() => reset(p)}>Volver a la configuración original</button>
        </form>
      {:else}
        <h2>{p.name}</h2>
        <p class="meta">{p.description}</p>
        <p class="price">{pesos(p.price_month)}<small>/mes</small></p>
        <p class="meta">o {pesos(p.price_year)} al año</p>
        <ul>
          <li>Personas en agenda: <b>{lim(p.max_staff)}</b></li>
          <li>Sucursales: <b>{p.max_branches}</b></li>
          <li>Caja, ventas e inventario: <b>{p.pos ? 'Sí' : 'No'}</b></li>
          <li>Anticipos en la reserva: <b>{p.deposits ? 'Sí' : 'No'}</b></li>
          <li>Cobro con terminal: <b>{p.terminal ? 'Sí' : 'No'}</b></li>
          <li>Soporte: <b>{p.support}</b></li>
        </ul>
        <div class="use"><b>{p.businesses}</b> en uso · <b>{p.paying}</b> pagando</div>
        {#if write}<button class="btn ghost sm edit" onclick={() => edit(p)}>Editar plan</button>{/if}
      {/if}
    </div>
  {/each}
</div>

<style>
  .plans { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); align-items: start; }
  .plan h2 { margin-bottom: 0.2rem; }
  .price { font-family: var(--font-body, inherit); font-weight: 700; font-variant-numeric: tabular-nums lining-nums; font-size: 2.2rem; margin: 0.5rem 0 0; }
  .price small { font-size: 1rem; color: var(--muted); font-family: var(--font-body); }
  ul { padding-left: 1.1rem; margin: 1rem 0; }
  li { margin-bottom: 0.3rem; }
  .use { padding-top: 0.75rem; border-top: 1px solid var(--border); color: var(--muted); font-size: 0.9rem; }
  .meta { color: var(--muted); font-size: 0.9rem; }
  .edit { margin-top: 1rem; }
  .two { display: grid; gap: 0 12px; grid-template-columns: 1fr 1fr; }
  .link { margin-top: 0.9rem; padding: 0; font: inherit; font-size: 0.85rem; color: var(--muted); background: none; border: 0; text-decoration: underline; cursor: pointer; }
  .link:hover { color: var(--danger); }
</style>
