<script>
  import { onMount } from 'svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { formatDay } from '$lib/format.js';
  import { todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';

  let counts = $state({ clients: null, services: null, team: null, today: null });

  onMount(async () => {
    const [clients, services, team] = await Promise.all([
      api('/api/clients').catch(() => null),
      api('/api/services').catch(() => null),
      session.me.role === 'admin' ? api('/api/team').catch(() => null) : null
    ]);
    // citas de hoy en la primera sucursal activa
    const branches = (await api('/api/branches').catch(() => [])).filter((b) => b.active);
    const today = branches.length
      ? await api('/api/agenda?' + new URLSearchParams({ branch_id: branches[0].id, date: todayIn(session.me.business.timezone || 'America/Tijuana') })).catch(() => null)
      : null;
    counts = {
      today: today && today.appointments.filter((a) => a.status === 'scheduled').length,
      clients: clients && (clients.length >= 100 ? '100+' : clients.length),
      services: services && services.filter((s) => s.active).length,
      team: team && team.filter((m) => m.active).length
    };
  });

  const cards = $derived([
    ['calendar', 'Citas de hoy', counts.today, '/panel/agenda'],
    ['users', 'Clientes', counts.clients, '/panel/clientes'],
    ['scissors', 'Servicios activos', counts.services, '/panel/servicios'],
    ...(session.me.role === 'admin' ? [['users', 'Equipo activo', counts.team, '/panel/equipo']] : [])
  ]);
</script>

{#if session.me.business.status === 'trial' && session.me.business.trial_ends_at}
  <p class="banner">Prueba gratuita hasta el {formatDay(session.me.business.trial_ends_at)}.</p>
{/if}

<div class="page-head">
  <div>
    <h1>Hola, {session.me.name}</h1>
    <p>Este es el resumen de tu negocio.</p>
  </div>
</div>

<div class="stats">
  {#each cards as [icon, label, value, href]}
    <a class="card stat" {href}>
      <span class="ico"><Icon name={icon} /></span>
      <span class="label">{label}</span>
      <span class="value">{value ?? '—'}</span>
    </a>
  {/each}
</div>

<style>
  .stats { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
  .stat { display: grid; gap: 4px; color: var(--text); text-decoration: none; transition: border-color 0.2s var(--ease), transform 0.2s var(--ease); }
  a.stat:hover { border-color: var(--accent); transform: translateY(-2px); }
  .ico { display: grid; place-items: center; width: 40px; height: 40px; margin-bottom: 8px; border-radius: 10px; background: var(--accent-soft); color: var(--accent); }
  .label { color: var(--muted); font-size: 0.9rem; }
  .value { font-family: var(--font-heading); font-size: 2.2rem; line-height: 1.1; }
</style>
