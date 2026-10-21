<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import Shell from '$lib/Shell.svelte';
  import { getToken, setToken } from '$lib/api.js';
  import { isPlatform, loadSession, session } from '$lib/session.svelte.js';

  let { children } = $props();

  // [icono, etiqueta, ruta (null = próximamente), roles permitidos (vacío = todos)]
  const nav = [
    ['home', 'Inicio', '/panel', []],
    ['calendar', 'Agenda', null, []],
    ['card', 'Ventas', null, []],
    ['users', 'Clientes', '/panel/clientes', []],
    ['scissors', 'Servicios', '/panel/servicios', []],
    ['package', 'Inventario', '/panel/inventario', []],
    ['users', 'Equipo', '/panel/equipo', ['admin']],
    ['percent', 'Comisiones', null, []]
  ];
  const items = $derived(
    nav.filter(([, , , roles]) => !roles.length || roles.includes(session.me?.role)).map(([i, l, h]) => [i, l, h])
  );
  const blocked = $derived(['pending', 'suspended'].includes(session.me?.business?.status));

  onMount(async () => {
    if (!getToken()) return goto('/login');
    try {
      await loadSession();
      if (isPlatform()) goto('/plataforma');
    } catch {
      setToken(null);
      goto('/login');
    }
  });

  function logout() {
    setToken(null);
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
    {:else}
      {@render children()}
    {/if}
  </Shell>
{/if}

<style>
  .state { max-width: 560px; margin-top: 2rem; }
  .state h1 { font-size: 1.8rem; }
  .state p { color: var(--muted); margin: 0; }
</style>
