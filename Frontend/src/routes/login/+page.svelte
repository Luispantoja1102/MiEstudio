<script>
  import { goto } from '$app/navigation';
  import AuthLayout from '$lib/AuthLayout.svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { homeFor } from '$lib/session.svelte.js';

  let identifier = $state(''); // usuario o correo, en el mismo campo
  let password = $state('');
  let showPass = $state(false);
  let capsOn = $state(false);
  let error = $state('');
  let busy = $state(false);
  let challenge = $state(''); // con dos pasos, la contraseña sola no abre la sesión
  let code = $state('');
  let recovery = $state(false);

  const caps = (e) => (capsOn = e.getModifierState?.('CapsLock') ?? false);

  async function submit(e) {
    e.preventDefault();
    error = '';
    if (!challenge && (!identifier.trim() || !password)) {
      error = 'Escribe tu usuario o correo y tu contraseña.';
      return;
    }
    busy = true;
    try {
      const r = challenge
        ? await api('/api/auth/login/2fa', { method: 'POST', body: { challenge, code: code.trim() } })
        : await api('/api/auth/login', { method: 'POST', body: { identifier: identifier.trim(), password } });
      if (r.needs_2fa) {
        challenge = r.challenge;
        password = '';
        return;
      }
      goto(homeFor(await api('/api/me')));
    } catch (err) {
      error = err.message;
      if (challenge) code = '';
    } finally {
      busy = false;
    }
  }

  function back() {
    challenge = code = error = '';
    recovery = false;
  }
</script>

<svelte:head><title>Iniciar sesión · Barbella</title></svelte:head>

<AuthLayout title="Bienvenido" accent="de vuelta" subtitle="Entra para ver tu agenda, tus clientes y tu negocio.">
  <form onsubmit={submit} novalidate>
    {#if challenge}
      <h2 class="step">Verificación en dos pasos</h2>
      <label class="field">
        {recovery ? 'Código de respaldo' : 'Código de 6 dígitos'}
        <input class="code" inputmode={recovery ? 'text' : 'numeric'} autocomplete="one-time-code" autocapitalize="none" spellcheck="false" bind:value={code} required />
        <span class="hint">{recovery ? 'Escribe uno de tus códigos de respaldo. Cada uno sirve una sola vez.' : 'Abre tu app de autenticación y escribe el código que muestra.'}</span>
      </label>
    {:else}
      <label class="field">Usuario o correo <input type="text" bind:value={identifier} placeholder="tu usuario o tu@correo.com" autocomplete="username" autocapitalize="none" autocorrect="off" spellcheck="false" /></label>
      <div class="field">
        <div class="row">
          <label for="login-pass">Contraseña</label>
          <a class="link" href="/olvide">¿La olvidaste?</a>
        </div>
        <div class="pass">
          <input id="login-pass" type={showPass ? 'text' : 'password'} bind:value={password} placeholder="Tu contraseña" autocomplete="current-password" onkeydown={caps} onkeyup={caps} />
          <button type="button" class="eye" aria-label={showPass ? 'Ocultar contraseña' : 'Mostrar contraseña'} aria-pressed={showPass} onclick={() => (showPass = !showPass)}>
            <Icon name={showPass ? 'eye-off' : 'eye'} size={20} />
          </button>
        </div>
        {#if capsOn}<span class="caps">Bloq Mayús está activado.</span>{/if}
      </div>
    {/if}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn block lg" disabled={busy}>{busy ? (challenge ? 'Verificando…' : 'Entrando…') : challenge ? 'Verificar y entrar' : 'Entrar'}</button>
    {#if challenge}
      <div class="links">
        <button type="button" class="link" onclick={() => { recovery = !recovery; code = ''; error = ''; }}>{recovery ? 'Usar el código de mi app' : 'Usar un código de respaldo'}</button>
        <button type="button" class="link muted" onclick={back}>← Volver</button>
      </div>
    {/if}
  </form>

  <div class="alt">
    <span>¿Aún no tienes cuenta?</span>
    <a class="btn ghost block lg" href="/registro">Crear mi negocio</a>
  </div>
  <a class="home" href="/">← Volver al inicio</a>
</AuthLayout>

<style>
  .step { font-family: var(--font-body, inherit); font-size: 1.05rem; margin: 0 0 1rem; }
  .row { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; font-size: 0.9rem; font-weight: 500; }
  .link { font: inherit; font-size: 0.85rem; font-weight: 500; color: var(--accent); background: none; border: 0; padding: 10px 2px; cursor: pointer; text-decoration: none; }
  .link:hover { text-decoration: underline; }
  .link.muted { color: var(--muted); }
  .pass { position: relative; margin-top: 0.4rem; }
  .pass input { padding-right: 3rem; }
  .eye { position: absolute; right: 4px; top: 50%; transform: translateY(-50%); display: grid; place-items: center; width: 40px; height: 40px; border: 0; border-radius: 50%; background: none; color: var(--muted); cursor: pointer; }
  .eye:hover { color: var(--text); background: var(--surface-2); }
  .caps { display: block; margin-top: 0.4rem; font-size: 0.82rem; font-weight: 500; color: var(--warn); }
  .code { font-family: ui-monospace, monospace; font-size: 1.15rem; letter-spacing: 0.2em; }
  .links { display: flex; justify-content: space-between; gap: 12px; margin-top: 1rem; }
  .alt { display: grid; gap: 10px; margin-top: 1.5rem; padding-top: 1.25rem; border-top: 1px solid var(--border); text-align: center; color: var(--muted); font-size: 0.9rem; }
  .home { display: block; padding: 10px 0; margin-top: 0.5rem; text-align: center; font-size: 0.88rem; font-weight: 500; color: var(--muted); text-decoration: none; }
  .home:hover { color: var(--text); }
</style>
