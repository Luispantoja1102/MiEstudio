<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import AuthLayout from '$lib/AuthLayout.svelte';
  import PasswordInput from '$lib/PasswordInput.svelte';
  import { legalLink } from '$lib/legal/legal.svelte.js';
  import { api } from '$lib/api.js';
  import { KINDS, cleanUsername, pesos } from '$lib/format.js';
  import { homeFor } from '$lib/session.svelte.js';

  const MIN = 8;
  let form = $state({ business_name: '', kind: 'barberia', name: '', phone: '', email: '', username: '', password: '', accept_terms: false, plan: 'basico' });
  let confirm = $state('');
  let userEdited = $state(false); // mientras no lo escriban, el usuario se sugiere desde el correo

  function onUsername(e) {
    userEdited = true;
    form.username = cleanUsername(e.currentTarget.value);
  }
  function onEmail() {
    if (!userEdited) form.username = cleanUsername(form.email.split('@')[0]);
    codeSent = false; // otro correo, otro código
    code = '';
  }
  let showPass = $state(false);
  let touched = $state(false);
  let error = $state('');
  let busy = $state(false);
  let mode = $state('open');
  let plans = $state([]); // para elegir el plan que quieren probar
  let verify = $state(false); // el servidor pide verificar el correo con un código
  let codeSent = $state(false);
  let code = $state('');
  let wait = $state(0); // segundos para poder pedir otro código
  let timer;

  async function sendCode() {
    await api('/api/auth/register/code', { method: 'POST', body: { email: form.email.trim() } });
    codeSent = true;
    wait = 60;
    clearInterval(timer);
    timer = setInterval(() => (wait = Math.max(0, wait - 1)), 1000);
  }
  async function resend() {
    error = '';
    busy = true;
    try {
      await sendCode();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  onMount(async () => {
    try {
      const info = await api('/api/auth/registration');
      mode = info.mode;
      verify = !!info.email_verification;
    } catch {}
    try {
      plans = await api('/api/plans');
      const want = new URLSearchParams(location.search).get('plan'); // los botones de la landing llegan con ?plan=
      if (plans.some((p) => p.id === want)) form.plan = want;
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
    phone: form.phone.trim() && form.phone.replace(/\D/g, '').length < 10 ? 'Escribe un celular de 10 dígitos.' : '',
    email: !/^\S+@\S+\.\S+$/.test(form.email.trim()) ? 'Escribe un correo válido.' : '',
    username: !/^[a-z0-9._-]{3,30}$/.test(form.username) ? 'Usa de 3 a 30 caracteres: letras, números, punto, guion o guion bajo.' : '',
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
      if (verify && !codeSent) {
        await sendCode(); // primero se comprueba que el correo es de quien se registra
        return;
      }
      await api('/api/auth/register', { method: 'POST', body: { ...form, email_code: code.replace(/\D/g, ''), business_name: form.business_name.trim(), name: form.name.trim(), phone: form.phone.trim(), email: form.email.trim(), username: form.username } });
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
    <fieldset class="group">
      <legend>Tu negocio</legend>
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
      {#if plans.length}
        <label class="field">Plan que quieres probar
          <select bind:value={form.plan}>
            {#each plans as p (p.id)}<option value={p.id}>{p.name} · {pesos(p.price_month)}/mes</option>{/each}
          </select>
          <span class="hint">{form.plan === 'basico' ? 'Pruebas 14 días gratis, sin tarjeta.' : 'Empiezas con 14 días del plan Básico. Revisamos tu solicitud y te activamos el plan elegido.'}</span>
        </label>
      {/if}
    </fieldset>

    <fieldset class="group">
      <legend>Tus datos <span class="soft">(serás el administrador)</span></legend>
      <div class="grid">
        <label class="field">Nombre y apellido
          <input bind:value={form.name} placeholder="Ana López" autocomplete="name" aria-invalid={touched && !!problems.name} />
          {@render err('name')}
        </label>
        <label class="field">Celular <span class="soft">(opcional)</span>
          <input type="tel" inputmode="tel" bind:value={form.phone} maxlength="30" placeholder="664 123 4567" autocomplete="tel-national" aria-invalid={touched && !!problems.phone} />
          {@render err('phone')}
        </label>
      </div>
    </fieldset>

    <fieldset class="group">
      <legend>Tu acceso</legend>
      <div class="grid">
      <label class="field">Correo electrónico
        <input type="email" bind:value={form.email} oninput={onEmail} placeholder="negocio@correo.com" autocomplete="email" autocapitalize="none" spellcheck="false" aria-invalid={touched && !!problems.email} />
        {@render err('email')}
      </label>
      <label class="field">Usuario <span class="soft">(para entrar)</span>
        <input value={form.username} oninput={onUsername} placeholder="ana.lopez" maxlength="30" autocomplete="username" autocapitalize="none" autocorrect="off" spellcheck="false" aria-invalid={touched && !!problems.username} />
        <span class="hint">Podrás entrar con tu usuario o con tu correo, sin importar mayúsculas.</span>
        {@render err('username')}
      </label>
      <div class="field">
        <label for="r-pass">Contraseña</label>
        <PasswordInput id="r-pass" bind:value={form.password} bind:show={showPass} placeholder="Mínimo {MIN} caracteres" maxlength="72" autocomplete="new-password" aria-invalid={touched && !!problems.password} />
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
    </fieldset>

    <div class="terms">
      <label class="check">
        <input type="checkbox" bind:checked={form.accept_terms} />
        <span>Acepto los <a href="/terminos" target="_blank" rel="noopener" onclick={(e) => legalLink(e, 'terminos')}>términos</a> y el <a href="/privacidad" target="_blank" rel="noopener" onclick={(e) => legalLink(e, 'privacidad')}>aviso de privacidad</a></span>
      </label>
      {@render err('accept_terms')}
    </div>
    {#if verify && codeSent}
      <div class="verify" role="status">
        <label class="field">Código de verificación
          <input bind:value={code} inputmode="numeric" maxlength="6" autocomplete="one-time-code" placeholder="000000" aria-label="Código de 6 dígitos" />
          <span class="hint">Te lo mandamos a {form.email.trim()}. Vale 10 minutos.</span>
        </label>
        <button type="button" class="link" disabled={busy || wait > 0} onclick={resend}>{wait > 0 ? `Pedir otro código en ${wait} s` : 'Pedir otro código'}</button>
      </div>
    {/if}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn block lg" disabled={busy || mode === 'closed' || (verify && codeSent && code.replace(/\D/g, '').length !== 6)}>{busy ? (codeSent ? 'Creando…' : 'Enviando código…') : verify && !codeSent ? 'Enviar código a mi correo' : verify ? 'Verificar y crear mi negocio' : 'Crear mi negocio'}</button>
  </form>

  <p class="alt">¿Ya tienes cuenta? <a href="/login">Inicia sesión</a> · <a class="soft-link" href="/">Volver al inicio</a></p>
</AuthLayout>

<style>
  .grid { display: grid; gap: 0 14px; grid-template-columns: 1fr; }
  .group { margin: 0 0 0.25rem; padding: 0; border: 0; }
  .group + .group { padding-top: 1.1rem; border-top: 1px solid var(--border); }
  .group legend { padding: 0; margin-bottom: 0.7rem; font-size: 0.78rem; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; color: var(--muted); }
  .soft { font-weight: 400; color: var(--muted); }
  .bad { display: block; margin-top: 0.35rem; font-size: 0.82rem; font-weight: 600; color: var(--danger); }
  .meter { display: grid; grid-template-columns: repeat(3, 1fr) auto; align-items: center; gap: 6px; margin-top: 0.6rem; }
  .meter i { height: 6px; border-radius: 999px; background: var(--border); }
  .meter i.on[data-level='1'] { background: var(--danger); }
  .meter i.on[data-level='2'] { background: var(--warn); }
  .meter i.on[data-level='3'] { background: var(--ok, #2f8f5b); }
  .meter b { margin-left: 4px; font-size: 0.78rem; color: var(--muted); }
  .terms { margin-bottom: 1.1rem; }
  .verify { margin: 0 0 1.1rem; padding: 1rem 1rem 0.6rem; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface-2); }
  .verify input { letter-spacing: 0.4em; font-size: 1.3rem; text-align: center; font-variant-numeric: tabular-nums; }
  .verify .link { margin: 0 0 0.4rem; padding: 10px 2px; font: inherit; font-size: 0.88rem; font-weight: 500; color: var(--accent); background: none; border: 0; cursor: pointer; }
  .verify .link:disabled { color: var(--muted); cursor: default; }
  .alt a { display: inline-block; padding: 10px 4px; }
  .alt { text-align: center; color: var(--muted); font-size: 0.9rem; margin: 1.25rem 0 0; padding-top: 1.1rem; border-top: 1px solid var(--border); }
  .soft-link { color: inherit; font-weight: 500; }
  @media (min-width: 560px) {
    .grid { grid-template-columns: 1fr 1fr; }
  }
</style>
