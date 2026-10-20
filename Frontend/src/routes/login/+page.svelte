<script>
  import { goto } from '$app/navigation';
  import Header from '$lib/Header.svelte';
  import { api, setToken } from '$lib/api.js';

  let email = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      const { token } = await api('/api/auth/login', { method: 'POST', body: { email, password } });
      setToken(token);
      goto('/panel');
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<Header><a class="btn ghost" href="/registro">Crear cuenta</a></Header>

<main class="container wrap">
  <form class="card" onsubmit={submit}>
    <h1>Iniciar sesión</h1>
    <label>Correo <input type="email" bind:value={email} autocomplete="email" required /></label>
    <label>Contraseña <input type="password" bind:value={password} autocomplete="current-password" required /></label>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn" disabled={busy}>Entrar</button>
    <p class="alt">¿No tienes cuenta? <a href="/registro">Regístrate</a></p>
  </form>
</main>

<style>
  .wrap { max-width: 440px; padding-block: 48px; }
  h1 { font-size: 1.6rem; }
  .btn { width: 100%; }
  .alt { text-align: center; color: var(--muted); font-size: 0.9rem; margin-bottom: 0; }
</style>
