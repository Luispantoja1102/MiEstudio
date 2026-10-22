<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { pesos } from '$lib/format.js';

  let plans = $state([]);
  let error = $state('');
  onMount(async () => {
    try {
      plans = await api('/api/platform/plans');
    } catch (e) {
      error = e.message;
    }
  });
  const lim = (n) => (n === null ? 'Sin límite' : n);
</script>

<div class="page-head">
  <div>
    <h1>Planes</h1>
    <p>Lo que incluye cada suscripción y cuántos negocios la usan. Los cambios de precio y topes se hacen en el código (<code>plans.go</code>).</p>
  </div>
</div>
{#if error}<p class="error" role="alert">{error}</p>{/if}

<div class="plans">
  {#each plans as p (p.id)}
    <div class="card plan">
      <h2>{p.name}</h2>
      <p class="meta">{p.description}</p>
      <p class="price">{pesos(p.price_month)}<small>/mes</small></p>
      <p class="meta">o {pesos(p.price_year)} al año (2 meses gratis)</p>
      <ul>
        <li>Personas en agenda: <b>{lim(p.max_staff)}</b></li>
        <li>Sucursales: <b>{p.max_branches}</b></li>
        <li>Caja, ventas e inventario: <b>{p.pos ? 'Sí' : 'No'}</b></li>
        <li>Anticipos en la reserva: <b>{p.deposits ? 'Sí' : 'No'}</b></li>
        <li>Cobro con terminal: <b>{p.terminal ? 'Sí' : 'No'}</b></li>
        <li>Soporte: <b>{p.support}</b></li>
      </ul>
      <div class="use"><b>{p.businesses}</b> en uso · <b>{p.paying}</b> pagando</div>
    </div>
  {/each}
</div>

<style>
  .plans { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); }
  .plan h2 { margin-bottom: 0.2rem; }
  .price { font-family: var(--font-heading); font-size: 2.2rem; margin: 0.5rem 0 0; }
  .price small { font-size: 1rem; color: var(--muted); font-family: var(--font-body); }
  ul { padding-left: 1.1rem; margin: 1rem 0; }
  li { margin-bottom: 0.3rem; }
  .use { padding-top: 0.75rem; border-top: 1px solid var(--border); color: var(--muted); font-size: 0.9rem; }
  .meta { color: var(--muted); font-size: 0.9rem; }
  code { background: var(--surface-2); padding: 1px 6px; border-radius: 4px; font-size: 0.85em; }
</style>
