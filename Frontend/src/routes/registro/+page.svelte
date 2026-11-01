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
  const strengthTip = [
    '',
    `Usa al menos ${MIN} caracteres, con mayúsculas, minúsculas y números.`,
    'Para subirla, combina mayúsculas, minúsculas y números, y añade un símbolo o llega a 12 caracteres.',
    'Buena contraseña.'
  ];

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

<AuthLayout wide title="Crea tu" accent="negocio" subtitle="Toma menos de un minuto. Después te ayudamos a configurar horarios, servicios y equipo.">
  {#if mode === 'closed'}
    <div class="notice closed" role="status">
      <strong>El registro está cerrado por ahora.</strong>
      <span>Escríbenos para darte de alta. Mientras tanto, el formulario no se puede enviar.</span>
    </div>
  {:else if mode === 'approval'}
    <div class="notice" role="status">
      <strong>Revisamos cada registro.</strong>
      <span>Tu cuenta se activará cuando la aprobemos; no hace falta que vuelvas a registrarte.</span>
    </div>
  {/if}

  <form onsubmit={submit} novalidate>
    <fieldset disabled={mode === 'closed'}>
      <legend><span class="n">1</span>Tu negocio</legend>
      <div class="grid">
        <label class="field">Nombre del negocio
          <input bind:value={form.business_name} placeholder="Ej. Barbería Don Luis" autocomplete="organization" aria-invalid={touched && !!problems.business_name} />
          <span class="hint">Así lo verán tus clientes al reservar.</span>
          {@render err('business_name')}
        </label>
        <label class="field">Giro
          <select bind:value={form.kind}>
            {#each Object.entries(KINDS) as [value, label]}<option {value}>{label}</option>{/each}
          </select>
          <span class="hint">Define los servicios de ejemplo con los que arrancas.</span>
        </label>
      </div>
    </fieldset>

    <fieldset disabled={mode === 'closed'}>
      <legend><span class="n">2</span>Tu cuenta</legend>
      <div class="grid">
        <label class="field full">Tu nombre
          <input bind:value={form.name} placeholder="Nombre y apellido" autocomplete="name" aria-invalid={touched && !!problems.name} />
          <span class="hint">Serás el administrador del negocio.</span>
          {@render err('name')}
        </label>
        <label class="field full">Correo electrónico
          <input type="email" bind:value={form.email} placeholder="negocio@correo.com" autocomplete="email" autocapitalize="none" spellcheck="false" aria-invalid={touched && !!problems.email} />
          <span class="hint">Con este correo entrarás a tu agenda.</span>
          {@render err('email')}
        </label>
        <div class="field">
          <label for="r-pass">Contraseña</label>
          <div class="pass">
            <input id="r-pass" type={showPass ? 'text' : 'password'} bind:value={form.password} placeholder="Mínimo {MIN} caracteres" maxlength="72" autocomplete="new-password" aria-invalid={touched && !!problems.password} aria-describedby="r-pass-help" />
            <button type="button" class="eye" aria-label={showPass ? 'Ocultar contraseña' : 'Mostrar contraseña'} aria-pressed={showPass} onclick={() => (showPass = !showPass)}>
              <Icon name={showPass ? 'eye-off' : 'eye'} size={20} />
            </button>
          </div>
          <div id="r-pass-help" class="help">
            {#if form.password}
              <div class="meter" role="img" aria-label="Seguridad de la contraseña: {strengthLabel[strength]}">
                {#each [1, 2, 3] as bar}<i class:on={strength >= bar} data-level={strength}></i>{/each}
              </div>
              <span class="hint"><b class="lvl" data-level={strength}>{strengthLabel[strength]}.</b> {strengthTip[strength]}</span>
            {:else}
              <span class="hint">Mínimo {MIN} caracteres; mejor con mayúsculas, minúsculas y números.</span>
            {/if}
          </div>
          {@render err('password')}
        </div>
        <label class="field">Confirmar contraseña
          <input type={showPass ? 'text' : 'password'} bind:value={confirm} maxlength="72" autocomplete="new-password" aria-invalid={touched && !!problems.confirm} />
          <span class="hint">Escríbela otra vez para evitar errores.</span>
          {@render err('confirm')}
        </label>
      </div>
    </fieldset>

    <div class="terms">
      <label class="check">
        <input type="checkbox" bind:checked={form.accept_terms} disabled={mode === 'closed'} />
        <span>Acepto los <a href="/terminos" target="_blank" rel="noopener">términos</a> y el <a href="/privacidad" target="_blank" rel="noopener">aviso de privacidad</a></span>
      </label>
      {@render err('accept_terms')}
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn block lg" disabled={busy || mode === 'closed'}>{busy ? 'Creando…' : mode === 'approval' ? 'Enviar mi registro' : 'Crear mi negocio'}</button>
    {#if mode === 'approval'}<p class="after">Te avisaremos por correo cuando tu cuenta esté aprobada.</p>{/if}
  </form>

  <p class="alt">¿Ya tienes cuenta? <a class="inl" href="/login">Inicia sesión</a><span aria-hidden="true"> · </span><a class="inl soft-link" href="/">Volver al inicio</a></p>
</AuthLayout>

<style>
  fieldset { border: 0; margin: 0 0 0.5rem; padding: 0; min-width: 0; }
  fieldset + fieldset { padding-top: 1.25rem; border-top: 1px solid var(--border); }
  legend { float: left; width: 100%; display: flex; align-items: center; gap: 10px; padding: 0; margin-bottom: 1rem; font-family: var(--font-heading); font-size: 1.05rem; font-weight: 700; color: var(--ink); }
  legend + .grid { clear: both; }
  .n { display: inline-grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; background: var(--ink); color: var(--surface); font-family: var(--font-body, inherit); font-size: 0.8rem; font-weight: 600; font-variant-numeric: tabular-nums; }
  .grid { display: grid; gap: 0 16px; grid-template-columns: 1fr; }
  .notice { display: grid; gap: 2px; margin-bottom: 1.25rem; padding: 0.85rem 1rem; border-left: 3px solid var(--accent); border-radius: var(--radius-sm); background: var(--surface-2); font-size: 0.9rem; color: var(--text); }
  .notice.closed { border-left-color: var(--warn); background: var(--warn-soft); color: var(--warn); }
  .bad { display: block; margin-top: 0.35rem; font-size: 0.82rem; font-weight: 600; color: var(--danger); }
  .pass { position: relative; margin-top: 0.4rem; }
  .pass input { padding-right: 3rem; }
  .eye { position: absolute; right: 1px; top: 50%; transform: translateY(-50%); display: grid; place-items: center; width: 44px; height: 44px; border: 0; border-radius: var(--radius-sm); background: none; color: var(--muted); cursor: pointer; }
  .eye:hover { color: var(--text); background: var(--surface-2); }
  .meter { display: grid; grid-template-columns: repeat(3, 1fr); gap: 4px; margin-top: 0.6rem; }
  .meter i { height: 4px; border-radius: 2px; background: var(--border); }
  .meter i.on[data-level='1'] { background: var(--danger); }
  .meter i.on[data-level='2'] { background: var(--warn); }
  .meter i.on[data-level='3'] { background: var(--success); }
  .lvl { font-weight: 600; }
  .lvl[data-level='1'] { color: var(--danger); }
  .lvl[data-level='2'] { color: var(--warn); }
  .lvl[data-level='3'] { color: var(--success); }
  .terms { margin: 0.5rem 0 1rem; padding-top: 1rem; border-top: 1px solid var(--border); }
  .terms a, .inl { color: var(--accent); }
  .after { margin: 0.75rem 0 0; text-align: center; font-size: 0.85rem; color: var(--muted); }
  .alt { text-align: center; color: var(--muted); font-size: 0.9rem; margin: 1.25rem 0 0; padding-top: 0.5rem; border-top: 1px solid var(--border); }
  .alt { display: flex; flex-wrap: wrap; justify-content: center; align-items: center; gap: 0 8px; }
  .alt a { display: inline-flex; align-items: center; min-height: 44px; }
  .soft-link { color: var(--muted); font-weight: 500; }
  @media (min-width: 560px) {
    .grid { grid-template-columns: 1fr 1fr; }
    .full { grid-column: 1 / -1; }
  }
</style>
