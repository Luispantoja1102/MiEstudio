<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import AuthLayout from '$lib/AuthLayout.svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { KINDS } from '$lib/format.js';
  import { homeFor } from '$lib/session.svelte.js';

  const MIN = 8;
  let form = $state({ business_name: '', kind: 'barberia', name: '', email: '', password: '', accept_terms: false });
  let confirm = $state('');
  let showPass = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);
  let mode = $state('open');

  onMount(async () => {
    try {
      mode = (await api('/api/auth/registration')).mode;
    } catch {}
  });

  // 0..3: largo, mayúsculas con minúsculas y números, símbolos o contraseña larga
  const strength = $derived.by(() => {
    const p = form.password;
    let s = 0;
    if (p.length >= MIN) s++;
    if (/[a-z]/.test(p) && /[A-Z]/.test(p) && /\d/.test(p)) s++;
    if (p.length >= 12 || /[^A-Za-z0-9]/.test(p)) s++;
    return p ? Math.max(1, s) : 0;
  });
  const strengthLabel = ['', 'Débil', 'Aceptable', 'Fuerte'];

  const problems = $derived({
    business_name: form.business_name.trim().length < 2 ? 'Escribe el nombre de tu negocio.' : '',
    name: form.name.trim().length < 2 ? 'Escribe tu nombre.' : '',
    email: !/^\S+@\S+\.\S+$/.test(form.email.trim()) ? 'Escribe un correo válido.' : '',
    password: form.password.length < MIN ? `Mínimo ${MIN} caracteres.` : '',
    confirm: confirm !== form.password ? 'Las contraseñas no coinciden.' : '',
    accept_terms: !form.accept_terms ? 'Debes aceptar los términos.' : ''
  });
  const valid = $derived(Object.values(problems).every((p) => !p));

  async function submit(e) {
    e.preventDefault();
    touched = true;
    error = '';
    if (!valid) {
      error = 'Revisa los campos marcados.';
      return;
    }
    busy = true;
    try {
      await api('/api/auth/register', { method: 'POST', body: { ...form, business_name: form.business_name.trim(), name: form.name.trim(), email: form.email.trim() } });
      goto(homeFor(await api('/api/me'))); // un negocio nuevo pasa por el asistente de inicio
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<svelte:head><title>Crear mi negocio · Barbella</title></svelte:head>

{#snippet err(key)}
  {#if touched && problems[key]}<span class="bad">{problems[key]}</span>{/if}
{/snippet}

<AuthLayout wide title="Crea tu" accent="negocio" subtitle="Toma menos de un minuto. Después te ayudamos a configurar todo lo demás.">
  {#if mode === 'closed'}
    <p class="banner" role="status">El registro está cerrado por ahora. Escríbenos para darte de alta.</p>
  {:else if mode === 'approval'}
    <p class="banner" role="status">Revisamos cada registro: tu cuenta se activará cuando la aprobemos.</p>
  {/if}

  <form onsubmit={submit} novalidate>
    <div class="grid">
      <label class="field">Nombre del negocio
        <input bind:value={form.business_name} placeholder="Ej. Barbería Don Luis" autocomplete="organization" aria-invalid={touched && !!problems.business_name} />
        {@render err('business_name')}
      </label>
      <label class="field">Giro
        <select bind:value={form.kind}>
          {#each Object.entries(KINDS) as [value, label]}<option {value}>{label}</option>{/each}
        </select>
      </label>
    </div>

    <div class="grid sep">
      <label class="field full">Tu nombre <span class="soft">(serás el administrador)</span>
        <input bind:value={form.name} placeholder="Nombre y apellido" autocomplete="name" aria-invalid={touched && !!problems.name} />
        {@render err('name')}
      </label>
      <label class="field full">Correo electrónico
        <input type="email" bind:value={form.email} placeholder="negocio@correo.com" autocomplete="email" autocapitalize="none" aria-invalid={touched && !!problems.email} />
        {@render err('email')}
      </label>
      <div class="field">
        <label for="r-pass">Contraseña</label>
        <div class="pass">
          <input id="r-pass" type={showPass ? 'text' : 'password'} bind:value={form.password} placeholder="Mínimo {MIN} caracteres" maxlength="72" autocomplete="new-password" aria-invalid={touched && !!problems.password} />
          <button type="button" class="eye" aria-label={showPass ? 'Ocultar contraseña' : 'Mostrar contraseña'} aria-pressed={showPass} onclick={() => (showPass = !showPass)}>
            <Icon name={showPass ? 'eye-off' : 'eye'} size={20} />
          </button>
        </div>
        {#if form.password}
          <div class="meter" aria-label="Seguridad de la contraseña: {strengthLabel[strength]}">
            {#each [1, 2, 3] as bar}<i class:on={strength >= bar} data-level={strength}></i>{/each}
            <b>{strengthLabel[strength]}</b>
          </div>
        {/if}
        {@render err('password')}
      </div>
      <label class="field">Confirmar contraseña
        <input type={showPass ? 'text' : 'password'} bind:value={confirm} maxlength="72" autocomplete="new-password" aria-invalid={touched && !!problems.confirm} />
        {@render err('confirm')}
      </label>
    </div>

    <div class="terms">
      <label class="check">
        <input type="checkbox" bind:checked={form.accept_terms} />
        <span>Acepto los <a href="/terminos" target="_blank" rel="noopener">términos</a> y el <a href="/privacidad" target="_blank" rel="noopener">aviso de privacidad</a></span>
      </label>
      {@render err('accept_terms')}
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn block lg" disabled={busy || mode === 'closed'}>{busy ? 'Creando…' : 'Crear mi negocio'}</button>
  </form>

  <p class="alt">¿Ya tienes cuenta? <a href="/login">Inicia sesión</a> · <a class="soft-link" href="/">Volver al inicio</a></p>
</AuthLayout>

<style>
  .grid { display: grid; gap: 0 14px; grid-template-columns: 1fr; }
  .sep { margin-top: 0.25rem; padding-top: 1.1rem; border-top: 1px solid var(--border); }
  .soft { font-weight: 400; color: var(--muted); }
  .bad { display: block; margin-top: 0.35rem; font-size: 0.82rem; font-weight: 600; color: var(--danger); }
  .pass { position: relative; margin-top: 0.4rem; }
  .pass input { padding-right: 3rem; }
  .eye { position: absolute; right: 4px; top: 50%; transform: translateY(-50%); display: grid; place-items: center; width: 40px; height: 40px; border: 0; border-radius: 50%; background: none; color: var(--muted); cursor: pointer; }
  .eye:hover { color: var(--text); background: var(--surface-2); }
  .meter { display: grid; grid-template-columns: repeat(3, 1fr) auto; align-items: center; gap: 6px; margin-top: 0.6rem; }
  .meter i { height: 6px; border-radius: 999px; background: var(--border); }
  .meter i.on[data-level='1'] { background: var(--danger); }
  .meter i.on[data-level='2'] { background: var(--warn); }
  .meter i.on[data-level='3'] { background: var(--ok, #2f8f5b); }
  .meter b { margin-left: 4px; font-size: 0.78rem; color: var(--muted); }
  .terms { margin-bottom: 1.1rem; }
  .alt a { display: inline-block; padding: 10px 4px; }
  .alt { text-align: center; color: var(--muted); font-size: 0.9rem; margin: 1.25rem 0 0; padding-top: 1.1rem; border-top: 1px solid var(--border); }
  .soft-link { color: inherit; font-weight: 500; }
  @media (min-width: 560px) {
    .grid { grid-template-columns: 1fr 1fr; }
    .full { grid-column: 1 / -1; }
  }
</style>
