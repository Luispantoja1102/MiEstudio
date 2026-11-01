<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import AuthLayout from '$lib/AuthLayout.svelte';
  import PasswordInput from '$lib/PasswordInput.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
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
  const wanted = $derived(plans.find((p) => p.id === form.plan));
  const needsCode = $derived(verify && codeSent);
  const strengthTip = [
    '',
    `Usa al menos ${MIN} caracteres, con mayúsculas, minúsculas y números.`,
    'Para subirla, combina mayúsculas, minúsculas y números, y añade un símbolo o llega a 12 caracteres.',
    'Buena contraseña.'
  ];

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
      {#if plans.length}
        <label class="field">Plan que quieres probar
          <select bind:value={form.plan}>
            {#each plans as p (p.id)}<option value={p.id}>{p.name} · {pesos(p.price_month)}/mes</option>{/each}
          </select>
          {#if form.plan === 'basico'}<span class="hint">Pruebas 14 días gratis, sin tarjeta.</span>{/if}
        </label>
        {#if form.plan !== 'basico'}
          <div class="planned">
            <InlineNotice tone="info">
              <b>Plan solicitado: {wanted?.name ?? form.plan}.</b>
              <span class="meta">Empiezas con 14 días del plan Básico. Revisamos tu solicitud y te activamos el plan elegido.</span>
            </InlineNotice>
          </div>
        {/if}
      {/if}
    </fieldset>

    <fieldset disabled={mode === 'closed'}>
      <legend><span class="n">2</span>Tus datos <span class="soft">(serás el administrador)</span></legend>
      <div class="grid">
        <label class="field">Nombre y apellido
          <input bind:value={form.name} placeholder="Ana López" autocomplete="name" aria-invalid={touched && !!problems.name} />
          {@render err('name')}
        </label>
        <label class="field">Celular <span class="soft">(opcional)</span>
          <input type="tel" inputmode="tel" bind:value={form.phone} maxlength="30" placeholder="664 123 4567" autocomplete="tel-national" aria-invalid={touched && !!problems.phone} />
          <span class="hint">Para que podamos avisarte de tu cuenta.</span>
          {@render err('phone')}
        </label>
      </div>
    </fieldset>

    <fieldset disabled={mode === 'closed'}>
      <legend><span class="n">3</span>Tu acceso</legend>
      <div class="grid">
        <label class="field">Correo electrónico
          <input type="email" bind:value={form.email} oninput={onEmail} placeholder="negocio@correo.com" autocomplete="email" autocapitalize="none" spellcheck="false" aria-invalid={touched && !!problems.email} />
          <span class="hint">{verify ? 'Te mandaremos un código para confirmar que es tuyo.' : 'Con este correo o con tu usuario entrarás a tu agenda.'}</span>
          {@render err('email')}
        </label>
        <label class="field">Usuario <span class="soft">(para entrar)</span>
          <input value={form.username} oninput={onUsername} placeholder="ana.lopez" maxlength="30" autocomplete="username" autocapitalize="none" autocorrect="off" spellcheck="false" aria-invalid={touched && !!problems.username} />
          <span class="hint">Podrás entrar con tu usuario o con tu correo, sin importar mayúsculas.</span>
          {@render err('username')}
        </label>
        <div class="field">
          <label for="r-pass">Contraseña</label>
          <PasswordInput id="r-pass" bind:value={form.password} bind:show={showPass} placeholder="Mínimo {MIN} caracteres" maxlength="72" autocomplete="new-password" aria-invalid={touched && !!problems.password} aria-describedby="r-pass-help" />
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

    {#if needsCode}
      <fieldset class="verify" disabled={mode === 'closed'}>
        <legend><span class="n">4</span>Confirma tu correo</legend>
        <label class="field">Código de 6 dígitos
          <input class="code" bind:value={code} inputmode="numeric" maxlength="6" autocomplete="one-time-code" placeholder="000000" />
          <span class="hint">Te lo mandamos a <b>{form.email.trim()}</b>. Vale 10 minutos; si no llega, revisa la carpeta de spam.</span>
        </label>
        <button type="button" class="link" disabled={busy || wait > 0} onclick={resend}>{wait > 0 ? `Pedir otro código en ${wait} s` : 'Pedir otro código'}</button>
      </fieldset>
    {/if}

    <div class="terms">
      <label class="check">
        <input type="checkbox" bind:checked={form.accept_terms} disabled={mode === 'closed'} />
        <span>Acepto los <a href="/terminos" target="_blank" rel="noopener" onclick={(e) => legalLink(e, 'terminos')}>términos</a> y el <a href="/privacidad" target="_blank" rel="noopener" onclick={(e) => legalLink(e, 'privacidad')}>aviso de privacidad</a></span>
      </label>
      {@render err('accept_terms')}
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button class="btn block lg" disabled={busy || mode === 'closed' || (needsCode && code.replace(/\D/g, '').length !== 6)}>{busy ? (codeSent ? 'Creando…' : 'Enviando código…') : verify && !codeSent ? 'Enviar código a mi correo' : verify ? 'Verificar y crear mi negocio' : mode === 'approval' ? 'Enviar mi registro' : 'Crear mi negocio'}</button>
    {#if mode === 'approval'}<p class="after">Te avisaremos por correo cuando tu cuenta esté aprobada.</p>{/if}
  </form>

  <p class="alt">¿Ya tienes cuenta? <a class="inl" href="/login">Inicia sesión</a><span aria-hidden="true"> · </span><a class="inl soft-link" href="/">Volver al inicio</a></p>
</AuthLayout>

<style>
  fieldset { border: 0; margin: 0 0 0.5rem; padding: 0; min-width: 0; }
  fieldset + fieldset { padding-top: 1.25rem; border-top: 1px solid var(--border); }
  legend { float: left; width: 100%; display: flex; align-items: center; gap: 10px; padding: 0; margin-bottom: 1rem; font-family: var(--font-heading); font-size: 1.05rem; font-weight: 700; color: var(--ink); }
  legend + .grid, legend + .field { clear: both; }
  .n { display: inline-grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; background: var(--ink); color: var(--surface); font-family: var(--font-body, inherit); font-size: 0.8rem; font-weight: 600; font-variant-numeric: tabular-nums; }
  .soft { font-weight: 400; color: var(--muted); }
  .grid { display: grid; gap: 0 16px; grid-template-columns: 1fr; }
  .notice { display: grid; gap: 2px; margin-bottom: 1.25rem; padding: 0.85rem 1rem; border-left: 3px solid var(--accent); border-radius: var(--radius-sm); background: var(--surface-2); font-size: 0.9rem; color: var(--text); }
  .notice.closed { border-left-color: var(--warn); background: var(--warn-soft); color: var(--warn); }
  .planned { margin-bottom: 1rem; }
  .meta { display: block; margin-top: 2px; }
  .bad { display: block; margin-top: 0.35rem; font-size: 0.82rem; font-weight: 600; color: var(--danger); }
  .meter { display: grid; grid-template-columns: repeat(3, 1fr); gap: 4px; margin-top: 0.6rem; }
  .meter i { height: 4px; border-radius: 2px; background: var(--border); }
  .meter i.on[data-level='1'] { background: var(--danger); }
  .meter i.on[data-level='2'] { background: var(--warn); }
  .meter i.on[data-level='3'] { background: var(--success); }
  .lvl { font-weight: 600; }
  .lvl[data-level='1'] { color: var(--danger); }
  .lvl[data-level='2'] { color: var(--warn); }
  .lvl[data-level='3'] { color: var(--success); }
  .verify { margin-top: 0.5rem; }
  .code { max-width: 14rem; min-height: 52px; font-family: ui-monospace, monospace; font-size: 1.5rem; letter-spacing: 0.4em; text-align: center; font-variant-numeric: tabular-nums; }
  .link { display: inline-flex; align-items: center; min-height: 44px; padding: 0; font: inherit; font-size: 0.88rem; font-weight: 500; color: var(--accent); background: none; border: 0; cursor: pointer; }
  .link:hover:not(:disabled) { text-decoration: underline; }
  .link:disabled { color: var(--muted); cursor: default; }
  .terms { margin: 0.5rem 0 1rem; padding-top: 1rem; border-top: 1px solid var(--border); }
  .terms a, .inl { color: var(--accent); }
  .after { margin: 0.75rem 0 0; text-align: center; font-size: 0.85rem; color: var(--muted); }
  .alt { display: flex; flex-wrap: wrap; justify-content: center; align-items: center; gap: 0 8px; text-align: center; color: var(--muted); font-size: 0.9rem; margin: 1.25rem 0 0; padding-top: 0.5rem; border-top: 1px solid var(--border); }
  .alt a { display: inline-flex; align-items: center; min-height: 44px; }
  .soft-link { color: var(--muted); font-weight: 500; }
  @media (min-width: 560px) {
    .grid { grid-template-columns: 1fr 1fr; }
  }
</style>
