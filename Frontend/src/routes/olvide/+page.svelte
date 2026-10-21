<script>
  import AuthLayout from '$lib/AuthLayout.svelte';

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

<AuthLayout title="Recupera tu" accent="contraseña" subtitle="Te enviaremos un enlace para elegir una nueva.">
  {#if sent}
    <div class="card" role="status">
      <p><b>Revisa tu correo.</b></p>
      <p class="meta">Si hay una cuenta con <b>{email}</b>, recibirás un enlace que vale 1 hora. Si no llega, revisa la carpeta de spam.</p>
    </div>
  {:else}
    <form onsubmit={submit}>
      <label class="field">Correo <input type="email" bind:value={email} autocomplete="email" required /></label>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button class="btn block lg" disabled={busy}>{busy ? 'Enviando…' : 'Enviar enlace'}</button>
    </form>
  {/if}
  <p class="alt"><a href="/login">Volver a entrar</a></p>
</AuthLayout>

<style>
  .alt { text-align: center; color: var(--muted); font-size: 0.9rem; margin: 1.5rem 0 0; }
  .meta { color: var(--muted); font-size: 0.9rem; }
</style>
