<script>
  import { onMount } from 'svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { KINDS, formatDay } from '$lib/format.js';

  const MODES = { open: 'Abierto', approval: 'Con aprobación', closed: 'Cerrado' };

  let stats = $state(null);
  let pending = $state([]);
  let error = $state('');

  onMount(async () => {
    try {
      [stats, pending] = await Promise.all([api('/api/platform/stats'), api('/api/platform/businesses?status=pending')]);
    } catch (e) {
      error = e.message;
    }
  });

  const cards = $derived(
    stats && [
      ['Negocios', stats.total, 'scissors'],
      ['Pendientes', stats.businesses.pending, 'bell'],
      ['En prueba', stats.businesses.trial, 'calendar'],
      ['Activos', stats.businesses.active, 'check'],
      ['Suspendidos', stats.businesses.suspended, 'percent'],
      ['Personas en negocios', stats.users, 'users'],
      ['Clientes en total', stats.clients, 'users'],
      ['Altas (30 días)', stats.signups_30d, 'globe']
    ]
  );
</script>

<div class="page-head">
  <div>
    <h1>Resumen de la plataforma</h1>
    <p>Todos los negocios registrados.</p>
  </div>
  {#if stats}<a class="badge" href="/plataforma/ajustes">Registro: {MODES[stats.registration_mode]}</a>{/if}
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if cards}
  <div class="stats">
    {#each cards as [label, value, icon]}
      <div class="card stat">
        <span class="ico"><Icon name={icon} /></span>
        <span class="label">{label}</span>
        <span class="value">{value}</span>
      </div>
    {/each}
  </div>

  {#if pending.length}
    <section>
      <h2>Por aprobar</h2>
      <div class="list">
        {#each pending as b (b.id)}
          <a class="item-row link" href="/plataforma/negocios/{b.id}">
            <div class="grow">
              <div class="title">{b.name}</div>
              <div class="meta">{KINDS[b.kind]} · {b.owner_email} · {formatDay(b.created_at)}</div>
            </div>
            <span class="badge warn">Pendiente</span>
          </a>
        {/each}
      </div>
    </section>
  {/if}
{/if}

<style>
  .stats { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); }
  .stat { display: grid; gap: 4px; }
  .ico { display: grid; place-items: center; width: 40px; height: 40px; margin-bottom: 8px; border-radius: 10px; background: var(--accent-soft); color: var(--accent); }
  .label { color: var(--muted); font-size: 0.9rem; }
  .value { font-family: var(--font-heading); font-size: 2.2rem; line-height: 1.1; }
  section { margin-top: 2.5rem; }
  section h2 { font-size: 1.5rem; }
  .link { color: var(--text); text-decoration: none; }
  .link:hover { border-color: var(--accent); }
  a.badge { text-decoration: none; }
</style>
