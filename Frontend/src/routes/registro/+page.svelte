<script>
  import { goto } from '$app/navigation';
  import Header from '$lib/Header.svelte';
  import { api, setToken } from '$lib/api.js';
  import { applyKindTheme } from '$lib/theme.svelte.js';

  const kinds = [
    ['barberia', 'Barbería'],
    ['estetica', 'Estética / salón'],
    ['spa', 'Spa'],
    ['unas', 'Uñas']
  ];

  let form = $state({ business_name: '', kind: 'barberia', name: '', email: '', password: '' });
  let error = $state('');
  let busy = $state(false);

  // Vista previa del tema del giro (si no se eligió uno manualmente).
  $effect(() => applyKindTheme(form.kind));

  async function submit(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      const { token } = await api('/api/auth/register', { method: 'POST', body: form });
      setToken(token);
      goto('/panel');
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<Header><a class="btn ghost" href="/login">Iniciar sesión</a></Header>

<main class="container wrap">
  <form class="card" onsubmit={submit}>
    <h1>Crea tu cuenta</h1>
    <label>Nombre del negocio <input bind:value={form.business_name} required /></label>
    <label>
      Giro
      <select bind:value={form.kind}>
        {#each kinds as [value, label]}<option {value}>{label}</option>{/each}
      </select>
    </label>
    <label>Tu nombre <input bind:value={form.name} autocomplete="name" required /></label>
    <label>Correo <input type="email" bind:value={form.email} autocomplete="email" required /></label>
    <label>
      Contraseña (mínimo 8 caracteres)
      <input type="password" bind:value={form.password} minlength="8" maxlength="72" autocomplete="new-password" required />
    </label>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn" disabled={busy}>Crear cuenta</button>
    <p class="alt">¿Ya tienes cuenta? <a href="/login">Inicia sesión</a></p>
  </form>
</main>

<style>
  .wrap { max-width: 480px; padding-block: 40px; }
  h1 { font-size: 1.6rem; }
  .btn { width: 100%; }
  .alt { text-align: center; color: var(--muted); font-size: 0.9rem; margin-bottom: 0; }
</style>
