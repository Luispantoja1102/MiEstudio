<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import Icon from '$lib/Icon.svelte';
  import SectionTabs from '$lib/SectionTabs.svelte';
  import Shell from '$lib/Shell.svelte';
  import NotificationBell from '$lib/NotificationBell.svelte';
  import { page } from '$app/state';
  import { api } from '$lib/api.js';
  import { hasPos, homeFor, isPlatform, loadSession, session } from '$lib/session.svelte.js';
  import { syncPrintStation } from '$lib/pos/printing.svelte.js';

  let { children } = $props();

  const SELL = ['admin', 'cashier', 'reception'];
  const SETTINGS_PATHS = ['/panel/equipo', '/panel/reserva', '/panel/mensajes', '/panel/privacidad', '/panel/pagos', '/panel/suscripcion', '/panel/cuenta', '/panel/ajustes'];
  const COBROS_PATHS = ['/panel/ventas', '/panel/caja', '/panel/servicios', '/panel/inventario'];
  const REPORT_PATHS = ['/panel/reportes', '/panel/comisiones'];
  const AGENDA_PATHS = ['/panel/agenda', '/panel/espera'];

  // Una lista corta, como en MiConsultorio: Cobros y Reportes se abren con sus propias pestañas;
  // lo que no se usa a diario (equipo, sucursales, reserva en línea, pagos, plan, privacidad, cuenta) vive en Ajustes.
  // [icono, etiqueta, ruta, roles permitidos (vacío = todos), rutas extra que la marcan activa]
  const nav = [
    ['home', 'Inicio', '/panel', []],
    ['calendar', 'Agenda', '/panel/agenda', [], AGENDA_PATHS],
    ['users', 'Clientes', '/panel/clientes', []],
    ['receipt', 'Cobrar', '/panel/ventas', SELL, COBROS_PATHS],
    ['scissors', 'Catálogo', '/panel/servicios', ['staff'], COBROS_PATHS],
    ['chart', 'Reportes', '/panel/reportes', ['admin'], REPORT_PATHS],
    ['percent', 'Comisiones', '/panel/comisiones', ['staff']]
  ];
  const footer = [['settings', 'Ajustes', '/panel/ajustes', SETTINGS_PATHS.slice(0, -1).concat('/panel/ajustes/sucursales')]];
  const COBROS_TABS = [
    { label: 'Cobrar', href: '/panel/ventas', exact: true, roles: SELL },
    { label: 'Caja', href: '/panel/caja', roles: SELL },
    { label: 'Historial', href: '/panel/ventas/historial', roles: SELL },
    { label: 'Servicios', href: '/panel/servicios' },
    { label: 'Inventario', href: '/panel/inventario' }
  ];
  const AGENDA_TABS = [
    { label: 'Agenda', href: '/panel/agenda', exact: true, roles: ['admin', 'reception', 'staff'] },
    { label: 'Lista de espera', href: '/panel/espera', roles: ['admin', 'reception', 'staff'] }
  ];
  const REPORT_TABS = [
    { label: 'Ventas', href: '/panel/reportes', roles: ['admin'] },
    { label: 'Comisiones', href: '/panel/comisiones', roles: ['admin', 'staff'] }
  ];
  const under = (paths) => paths.some((h) => page.url.pathname === h || page.url.pathname.startsWith(h + '/'));
  // Básico solo trae agenda y citas: sin caja, ventas, inventario, comisiones ni reportes (el catálogo de servicios sí)
  const POS_PATHS = ['/panel/ventas', '/panel/caja', '/panel/inventario', '/panel/reportes', '/panel/comisiones'];
  const posBlocked = $derived(!hasPos() && under(POS_PATHS));
  const tabs = $derived(!hasPos() && under(COBROS_PATHS) ? null : under(COBROS_PATHS) ? COBROS_TABS : under(REPORT_PATHS) ? REPORT_TABS : under(AGENDA_PATHS) ? AGENDA_TABS : null);
  const items = $derived(
    nav
      .filter(([, , href]) => hasPos() || !['/panel/ventas', '/panel/reportes', '/panel/comisiones'].includes(href))
      .filter(([, label, , roles]) => !roles.length || roles.includes(session.me?.role) || (!hasPos() && label === 'Catálogo' && ['admin', 'reception'].includes(session.me?.role)))
      .map(([i, l, h, , extra]) => [i, l, h, extra])
  );
  const blocked = $derived(['pending', 'suspended'].includes(session.me?.business?.status));
  // suscripción vencida: la cuenta entra, pero solo para renovar (y ver su cuenta)
  const lapsed = $derived(['trial_expired', 'past_due'].includes(session.me?.business?.billing_state) && !blocked);
  const lapsedAllowed = $derived(['/panel/suscripcion', '/panel/cuenta', '/panel/ajustes'].includes(page.url.pathname));
  // dentro de una página de ajustes se ofrece volver al índice
  const inSettings = $derived(SETTINGS_PATHS.filter((p) => p !== '/panel/ajustes').concat('/panel/ajustes/sucursales').includes(page.url.pathname));
  const trialLeft = $derived.by(() => {
    const b = session.me?.business;
    if (b?.billing_state !== 'trialing' || !b.trial_ends_at) return null;
    return Math.max(0, Math.ceil((new Date(b.trial_ends_at) - Date.now()) / 86400000));
  });

  onMount(async () => {
    try {
      await loadSession();
      if (isPlatform()) goto('/plataforma');
      else if (homeFor(session.me) === '/bienvenida') goto('/bienvenida', { replaceState: true });
      else syncPrintStation(); // la computadora de la caja empieza a recibir los tickets de los otros dispositivos
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
  <Shell {items} {footer} title={session.me.business.name} onlogout={logout}>
    {#snippet tools()}{#if ['admin', 'reception'].includes(session.me.role)}<NotificationBell />{/if}{/snippet}
    {#if blocked}
      <div class="card state">
        {#if session.me.business.status === 'pending'}
          <h1>Tu cuenta está en revisión</h1>
          <p>Recibimos tu registro. Te avisaremos cuando esté aprobada y puedas empezar a usar Barbella.</p>
        {:else}
          <h1>Tu cuenta está suspendida</h1>
          <p>Por ahora no puedes usar la plataforma. Contáctanos para resolverlo.</p>
        {/if}
      </div>
    {:else if lapsed && !lapsedAllowed}
      <div class="card state">
        <h1>{session.me.business.billing_state === 'trial_expired' ? 'Tu prueba terminó' : 'Tu suscripción venció'}</h1>
        <p>Tus datos están a salvo. {session.me.role === 'admin' ? 'Elige un plan para reactivar tu cuenta al instante.' : 'Pide al administrador que renueve el plan para seguir usando Barbella.'}</p>
        {#if session.me.role === 'admin'}<p style="margin-top: 1rem"><a class="btn" href="/panel/suscripcion">Elegir un plan</a></p>{/if}
      </div>
    {:else}
      {#if trialLeft !== null && session.me.role === 'admin' && trialLeft <= 7}
        <p class="banner" role="status">Tu prueba gratis termina {trialLeft === 0 ? 'hoy' : `en ${trialLeft} ${trialLeft === 1 ? 'día' : 'días'}`}. <a href="/panel/suscripcion">Elegir un plan</a></p>
      {/if}
      {#if inSettings}<a class="back" href="/panel/ajustes"><Icon name="arrow" size={16} />Ajustes</a>{/if}
      {#if tabs}<SectionTabs {tabs} />{/if}
      {#if posBlocked}
        <div class="card state">
          <h1>Esto es de los planes Crecimiento y Pro</h1>
          <p>Tu plan Básico incluye la agenda y las citas. Para cobrar en caja, llevar inventario y ver reportes de ventas pasa a Crecimiento.</p>
          {#if session.me.role === 'admin'}<p style="margin-top: 1rem"><a class="btn" href="/panel/suscripcion">Ver planes</a></p>{/if}
        </div>
      {:else}
        {@render children()}
      {/if}
    {/if}
  </Shell>
{/if}

<style>
  .state { max-width: 560px; margin-top: 2rem; }
  .state h1 { font-size: 1.8rem; }
  .state p { color: var(--muted); margin: 0; }
</style>
