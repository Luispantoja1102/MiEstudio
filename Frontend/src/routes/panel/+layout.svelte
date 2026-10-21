<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import Icon from '$lib/Icon.svelte';
  import Shell from '$lib/Shell.svelte';
  import { page } from '$app/state';
  import { api } from '$lib/api.js';
  import { isPlatform, loadSession, session } from '$lib/session.svelte.js';

  let { children } = $props();

  const SETTINGS_PATHS = ['/panel/equipo', '/panel/reserva', '/panel/privacidad', '/panel/pagos', '/panel/suscripcion', '/panel/cuenta'];
  // [icono, etiqueta, ruta (null = próximamente), roles permitidos (vacío = todos), rutas extra que la marcan activa]
  const SELL = ['admin', 'cashier', 'reception'];
  const nav = [
    ['home', 'Inicio', '/panel', []],
    ['calendar', 'Agenda', '/panel/agenda', []],
    ['receipt', 'Cobrar', '/panel/ventas', SELL],
    ['wallet', 'Caja', '/panel/caja', SELL],
    ['users', 'Clientes', '/panel/clientes', []],
    ['scissors', 'Servicios', '/panel/servicios', []],
    ['package', 'Inventario', '/panel/inventario', []],
    ['percent', 'Comisiones', '/panel/comisiones', ['admin', 'staff']],
    ['chart', 'Reportes', '/panel/reportes', ['admin']],
    // lo que no se usa a diario vive dentro de Ajustes; sus páginas marcan «Ajustes» como activo
    ['settings', 'Ajustes', '/panel/ajustes', [], SETTINGS_PATHS]
  ];
  const items = $derived(
    nav.filter(([, , , roles]) => !roles.length || roles.includes(session.me?.role)).map(([i, l, h, , extra]) => [i, l, h, extra])
  );
  const blocked = $derived(['pending', 'suspended'].includes(session.me?.business?.status));
  // suscripción vencida: la cuenta entra, pero solo para renovar (y ver su cuenta)
  const lapsed = $derived(['trial_expired', 'past_due'].includes(session.me?.business?.billing_state) && !blocked);
  const lapsedAllowed = $derived(['/panel/suscripcion', '/panel/cuenta', '/panel/ajustes'].includes(page.url.pathname));
  // dentro de una página de ajustes se ofrece volver al índice
  const inSettings = $derived(SETTINGS_PATHS.concat('/panel/ajustes/sucursales').includes(page.url.pathname));
  const trialLeft = $derived.by(() => {
    const b = session.me?.business;
    if (b?.billing_state !== 'trialing' || !b.trial_ends_at) return null;
    return Math.max(0, Math.ceil((new Date(b.trial_ends_at) - Date.now()) / 86400000));
  });

  onMount(async () => {
    try {
      await loadSession();
      if (isPlatform()) goto('/plataforma');
    } catch {
      goto('/login');
    }
  });

  async function logout() {
    await api('/api/auth/logout', { method: 'POST' }).catch(() => {});
    session.me = null;
    goto('/');
  }
</script>

{#if session.me && !isPlatform()}
  <Shell {items} title={session.me.business.name} onlogout={logout}>
    {#if blocked}
      <div class="card state">
        {#if session.me.business.status === 'pending'}
          <h1>Tu cuenta está en revisión</h1>
          <p>Recibimos tu registro. Te avisaremos cuando esté aprobada y puedas empezar a usar MiEstudio.</p>
        {:else}
          <h1>Tu cuenta está suspendida</h1>
          <p>Por ahora no puedes usar la plataforma. Contáctanos para resolverlo.</p>
        {/if}
      </div>
    {:else if lapsed && !lapsedAllowed}
      <div class="card state">
        <h1>{session.me.business.billing_state === 'trial_expired' ? 'Tu prueba terminó' : 'Tu suscripción venció'}</h1>
        <p>Tus datos están a salvo. {session.me.role === 'admin' ? 'Elige un plan para reactivar tu cuenta al instante.' : 'Pide al administrador que renueve el plan para seguir usando MiEstudio.'}</p>
        {#if session.me.role === 'admin'}<p style="margin-top: 1rem"><a class="btn" href="/panel/suscripcion">Elegir un plan</a></p>{/if}
      </div>
    {:else}
      {#if trialLeft !== null && session.me.role === 'admin' && trialLeft <= 7}
        <p class="banner" role="status">Tu prueba gratis termina {trialLeft === 0 ? 'hoy' : `en ${trialLeft} ${trialLeft === 1 ? 'día' : 'días'}`}. <a href="/panel/suscripcion">Elegir un plan</a></p>
      {/if}
      {#if inSettings}<a class="back" href="/panel/ajustes"><Icon name="arrow" size={16} />Ajustes</a>{/if}
      {@render children()}
    {/if}
  </Shell>
{/if}

<style>
  .back { display: inline-flex; align-items: center; gap: 6px; margin-bottom: 1rem; color: var(--muted); text-decoration: none; font-size: 0.9rem; }
  .back :global(svg) { transform: rotate(180deg); }
  .back:hover { color: var(--accent); }
  .state { max-width: 560px; margin-top: 2rem; }
  .state h1 { font-size: 1.8rem; }
  .state p { color: var(--muted); margin: 0; }
</style>
