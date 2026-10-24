<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import Shell from '$lib/Shell.svelte';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { isPlatform, loadSession, session } from '$lib/session.svelte.js';

  let { children } = $props();

  const SETTINGS_PATHS = ['/plataforma/planes', '/plataforma/salud', '/plataforma/actividad', '/plataforma/cuenta'];
  // Como en MiTiendita: Hoy, Clientes, Finanzas y Equipo a la vista; lo demás vive en Ajustes.
  const groups = [
    {
      title: 'Plataforma',
      items: [
        ['home', 'Hoy', '/plataforma'],
        ['scissors', 'Clientes', '/plataforma/negocios'],
        ['wallet', 'Finanzas', '/plataforma/finanzas'],
        ['users', 'Equipo', '/plataforma/equipo']
      ]
    }
  ];
  const footer = [['settings', 'Ajustes', '/plataforma/ajustes', SETTINGS_PATHS]];

  onMount(async () => {
    try {
      await loadSession();
      if (!isPlatform()) goto('/panel');
    } catch {
      goto('/login');
    }
  });

  const inSettings = $derived(SETTINGS_PATHS.concat('/plataforma/ajustes/registro').includes(page.url.pathname));

  async function logout() {
    await api('/api/auth/logout', { method: 'POST' }).catch(() => {});
    session.me = null;
    goto('/');
  }
</script>

{#if session.me && isPlatform()}
  <Shell {groups} {footer} tag="Plataforma" title={session.me.name} onlogout={logout}>
    {#if inSettings}<a class="back" href="/plataforma/ajustes"><Icon name="arrow" size={16} />Ajustes</a>{/if}
    {@render children()}
  </Shell>
{/if}

