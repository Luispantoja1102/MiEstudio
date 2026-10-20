<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import Header from '$lib/Header.svelte';
  import { api, getToken, setToken } from '$lib/api.js';
  import { applyKindTheme } from '$lib/theme.svelte.js';

  let me = $state(null);

  onMount(async () => {
    if (!getToken()) return goto('/login');
    try {
      me = await api('/api/me');
      applyKindTheme(me.business.kind);
    } catch {
      setToken(null);
      goto('/login');
    }
  });

  function logout() {
    setToken(null);
    goto('/');
  }
</script>

<Header><button class="btn ghost" onclick={logout}>Salir</button></Header>

<main class="container wrap">
  {#if me}
    <h1>Hola, {me.name}</h1>
    <p class="card">
      Negocio: <strong>{me.business.name}</strong> · Rol: {me.role}<br />
      Aquí irán la agenda, el punto de venta y tus clientes.
    </p>
  {/if}
</main>

<style>
  .wrap { padding-block: 40px; }
</style>
