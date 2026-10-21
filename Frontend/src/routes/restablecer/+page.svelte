<script>
  import { page } from '$app/state';
  import AuthLayout from '$lib/AuthLayout.svelte';

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

<AuthLayout title="Elige una contraseña" accent="nueva" subtitle="Al guardarla se cerrarán tus sesiones abiertas.">
  {#if !token}
    <p class="error" role="alert">Este enlace está incompleto. <a href="/olvide">Pide uno nuevo</a>.</p>
  {:else if done}
    <div class="card" role="status">
      <p><b>Listo, tu contraseña cambió.</b></p>
      <a class="btn" href="/login">Entrar con la contraseña nueva</a>
    </div>
  {:else}
    <form onsubmit={submit}>
      <label class="field">Contraseña nueva <input type="password" bind:value={password} autocomplete="new-password" minlength="8" maxlength="72" required />
        <span class="hint">De 8 a 72 caracteres.</span></label>
      <label class="field">Repite la contraseña <input type="password" bind:value={again} autocomplete="new-password" required /></label>
      {#if error}<p class="error" role="alert">{error} {#if error.includes('venció')}<a href="/olvide">Pedir otro enlace</a>{/if}</p>{/if}
      <button class="btn block lg" disabled={busy}>{busy ? 'Guardando…' : 'Guardar contraseña'}</button>
    </form>
  {/if}
</AuthLayout>
