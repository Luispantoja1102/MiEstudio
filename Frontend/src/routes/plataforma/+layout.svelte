<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import Shell from '$lib/Shell.svelte';
  import { api } from '$lib/api.js';
  import { isPlatform, loadSession, session } from '$lib/session.svelte.js';

  let { children } = $props();

  const items = [
    ['home', 'Resumen', '/plataforma'],
    ['scissors', 'Negocios', '/plataforma/negocios'],
    ['users', 'Equipo', '/plataforma/equipo'],
    ['bell', 'Actividad', '/plataforma/actividad'],
    ['percent', 'Ajustes', '/plataforma/ajustes']
  ];

  onMount(async () => {
    try {
      await loadSession();
      if (!isPlatform()) goto('/panel');
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

{#if session.me && isPlatform()}
  <Shell {items} tag="Plataforma" title={session.me.name} onlogout={logout}>
    {@render children()}
  </Shell>
{/if}
