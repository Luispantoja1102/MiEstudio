<script>
  import { goto } from '$app/navigation';
  import AuthLayout from '$lib/AuthLayout.svelte';
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

<AuthLayout title="Bienvenido de nuevo" subtitle="Inicia sesión para entrar a tu negocio.">
  {#snippet nav()}<a class="btn ghost" href="/registro">Crear cuenta</a>{/snippet}

  <form onsubmit={submit}>
    <label class="field">Correo <input type="email" bind:value={email} autocomplete="email" required /></label>
    <label class="field">Contraseña <input type="password" bind:value={password} autocomplete="current-password" required /></label>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn block lg" disabled={busy}>{busy ? 'Entrando…' : 'Entrar'}</button>
  </form>
  <p class="alt">¿No tienes cuenta? <a href="/registro">Regístrate</a></p>
</AuthLayout>

<style>
  .alt { text-align: center; color: var(--muted); font-size: 0.9rem; margin: 1.5rem 0 0; }
</style>
