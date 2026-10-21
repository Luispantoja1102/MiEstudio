<script>
  import { goto } from '$app/navigation';
  import AuthLayout from '$lib/AuthLayout.svelte';
  import { api, setToken } from '$lib/api.js';

  const kinds = [
    ['barberia', 'Barbería'],
    ['estetica', 'Estética / salón'],
    ['spa', 'Spa'],
    ['unas', 'Uñas']
  ];

  let form = $state({ business_name: '', kind: 'barberia', name: '', email: '', password: '' });
  let error = $state('');
  let busy = $state(false);

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

<AuthLayout title="Crea tu cuenta" subtitle="Configura tu negocio en un par de minutos.">
  {#snippet nav()}<a class="btn ghost" href="/login">Iniciar sesión</a>{/snippet}

  <form onsubmit={submit}>
    <label class="field">Nombre del negocio <input bind:value={form.business_name} required /></label>
    <label class="field">
      Giro
      <select bind:value={form.kind}>
        {#each kinds as [value, label]}<option {value}>{label}</option>{/each}
      </select>
    </label>
    <label class="field">Tu nombre <input bind:value={form.name} autocomplete="name" required /></label>
    <label class="field">Correo <input type="email" bind:value={form.email} autocomplete="email" required /></label>
    <label class="field">
      Contraseña
      <input type="password" bind:value={form.password} minlength="8" maxlength="72" autocomplete="new-password" required />
      <span class="hint">Mínimo 8 caracteres.</span>
    </label>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn block lg" disabled={busy}>{busy ? 'Creando…' : 'Crear cuenta'}</button>
  </form>
  <p class="alt">¿Ya tienes cuenta? <a href="/login">Inicia sesión</a></p>
</AuthLayout>

<style>
  .alt { text-align: center; color: var(--muted); font-size: 0.9rem; margin: 1.5rem 0 0; }
</style>
