<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import Shell from '$lib/Shell.svelte';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { isPlatform, loadSession, session } from '$lib/session.svelte.js';

  let { children } = $props();

  const SETTINGS_PATHS = ['/plataforma/planes', '/plataforma/salud', '/plataforma/equipo', '/plataforma/actividad', '/plataforma/cuenta'];
  // Pocas opciones a la vista; lo demás vive en Ajustes (como en MiConsultorio).
  const groups = [
    {
      title: 'Plataforma',
      items: [
        ['home', 'Resumen', '/plataforma'],
        ['scissors', 'Negocios', '/plataforma/negocios'],
        ['wallet', 'Pagos', '/plataforma/pagos']
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

<style>
  .back { display: inline-flex; align-items: center; gap: 6px; min-height: 40px; margin-bottom: 0.4rem; color: var(--muted); text-decoration: none; font-size: 0.9rem; }
  .back :global(svg) { transform: rotate(180deg); }
  .back:hover { color: var(--accent); }
</style>
