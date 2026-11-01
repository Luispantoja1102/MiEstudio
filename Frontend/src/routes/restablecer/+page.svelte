<script>
  import { page } from '$app/state';
  import AuthLayout from '$lib/AuthLayout.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  const token = page.url.searchParams.get('token') ?? '';
  let password = $state('');
  let again = $state('');
  let error = $state('');
  let busy = $state(false);
  let done = $state(false);

  async function submit(e) {
    e.preventDefault();
    error = '';
    if (password !== again) return (error = 'Las contraseñas no coinciden.');
    busy = true;
    try {
      const res = await fetch('/api/auth/reset', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ token, password }) });
      const data = res.status === 204 ? {} : await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'No se pudo restablecer.');
      done = true;
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head><title>Nueva contraseña · Barbella</title></svelte:head>

<AuthLayout title="Elige una contraseña" accent="nueva" subtitle="Al guardarla se cerrarán tus sesiones abiertas.">
  {#if !token}
    <InlineNotice tone="danger">Este enlace está incompleto. <a href="/olvide">Pide uno nuevo</a>.</InlineNotice>
  {:else if done}
    <InlineNotice tone="success"><b>Listo, tu contraseña cambió.</b></InlineNotice>
    <a class="btn block lg back" href="/login">Entrar con la contraseña nueva</a>
  {:else}
    <form onsubmit={submit}>
      <label class="field">Contraseña nueva <input type="password" bind:value={password} autocomplete="new-password" minlength="8" maxlength="72" required />
        <span class="hint">De 8 a 72 caracteres.</span></label>
      <label class="field">Repite la contraseña <input type="password" bind:value={again} autocomplete="new-password" required /></label>
      {#if error}<p class="error" role="alert">{error} {#if error.includes('venció')}<a href="/olvide">Pedir otro enlace</a>{/if}</p>{/if}
      <button class="btn block lg" disabled={busy}>{busy ? 'Guardando…' : 'Guardar contraseña'}</button>
    </form>
  {/if}
  {#if !done}<a class="home" href="/login">← Volver a entrar</a>{/if}
</AuthLayout>

<style>
  .back { margin-top: 1.25rem; }
  .home { display: flex; align-items: center; justify-content: center; min-height: 44px; margin-top: 1rem; font-size: 0.88rem; font-weight: 500; color: var(--muted); text-decoration: none; }
  .home:hover { color: var(--text); }
  .error a { color: inherit; font-weight: 600; }
</style>
