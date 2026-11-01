<script>
  import AuthLayout from '$lib/AuthLayout.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  let email = $state('');
  let error = $state('');
  let busy = $state(false);
  let sent = $state(false);

  async function submit(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      const res = await fetch('/api/auth/forgot', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email }) });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'No se pudo enviar. Intenta de nuevo.');
      sent = true;
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head><title>Recuperar contraseña · Barbella</title></svelte:head>

<AuthLayout title="Recupera tu" accent="contraseña" subtitle="Te enviaremos un enlace para elegir una nueva.">
  {#if sent}
    <InlineNotice tone="success">
      <b>Revisa tu correo.</b>
      <span class="meta">Si hay una cuenta con <b>{email}</b>, recibirás un enlace que vale 1 hora. Si no llega, revisa la carpeta de spam.</span>
    </InlineNotice>
    <a class="btn ghost block lg back" href="/login">Volver a entrar</a>
  {:else}
    <form onsubmit={submit}>
      <label class="field">Correo <input type="email" bind:value={email} placeholder="tu@correo.com" autocomplete="email" autocapitalize="none" spellcheck="false" required /></label>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button class="btn block lg" disabled={busy}>{busy ? 'Enviando…' : 'Enviar enlace'}</button>
    </form>
    <a class="home" href="/login">← Volver a entrar</a>
  {/if}
</AuthLayout>

<style>
  .meta { display: block; margin-top: 4px; }
  .back { margin-top: 1.25rem; }
  .home { display: flex; align-items: center; justify-content: center; min-height: 44px; margin-top: 1rem; font-size: 0.88rem; font-weight: 500; color: var(--muted); text-decoration: none; }
  .home:hover { color: var(--text); }
</style>
