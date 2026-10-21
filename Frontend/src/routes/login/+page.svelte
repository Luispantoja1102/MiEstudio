<script>
  import { goto } from '$app/navigation';
  import AuthLayout from '$lib/AuthLayout.svelte';
  import { api } from '$lib/api.js';
  import { homeFor } from '$lib/session.svelte.js';

  let email = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);
  let challenge = $state(''); // con dos pasos, la contraseña sola no abre la sesión
  let code = $state('');

  async function submit(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      const r = challenge
        ? await api('/api/auth/login/2fa', { method: 'POST', body: { challenge, code } })
        : await api('/api/auth/login', { method: 'POST', body: { email, password } });
      if (r.needs_2fa) {
        challenge = r.challenge;
        return;
      }
      goto(homeFor(await api('/api/me')));
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
    {#if challenge}
      <label class="field">Código de verificación
        <input inputmode="numeric" autocomplete="one-time-code" bind:value={code} required />
        <span class="hint">Abre tu app de autenticación y escribe el código de 6 dígitos. ¿Perdiste el teléfono? Usa uno de tus códigos de respaldo.</span>
      </label>
    {:else}
      <label class="field">Correo <input type="email" bind:value={email} autocomplete="email" required /></label>
      <label class="field">Contraseña <input type="password" bind:value={password} autocomplete="current-password" required /></label>
    {/if}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn block lg" disabled={busy}>{busy ? 'Entrando…' : challenge ? 'Verificar' : 'Entrar'}</button>
  </form>
  {#if !challenge}<p class="alt"><a href="/olvide">¿Olvidaste tu contraseña?</a></p>{/if}
  <p class="alt">¿No tienes cuenta? <a href="/registro">Regístrate</a></p>
</AuthLayout>

<style>
  .alt { text-align: center; color: var(--muted); font-size: 0.9rem; margin: 1.5rem 0 0; }
</style>
