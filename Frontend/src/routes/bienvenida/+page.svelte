<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import HoursEditor from '$lib/HoursEditor.svelte';
  import Icon from '$lib/Icon.svelte';
  import Logo from '$lib/Logo.svelte';
  import ThemeToggle from '$lib/ThemeToggle.svelte';
  import { api } from '$lib/api.js';
  import { KINDS, ROLES, cleanUsername, zoneClock } from '$lib/format.js';
  import { homeFor, isPlatform, loadSession, session } from '$lib/session.svelte.js';
  import { ZONES } from '$lib/zones.js';

  const steps = ['Tu negocio', 'Datos', 'Horario', 'Equipo', 'Listo'];
  const KIND_ABOUT = {
    barberia: 'Cortes, barba y afeitado',
    estetica: 'Cabello, color, maquillaje y más',
    spa: 'Masajes, faciales y bienestar',
    unas: 'Manicure, pedicure y diseño'
  };
  let step = $state(0);
  let ready = $state(false);
  let biz = $state({ name: '', kind: 'barberia', timezone: 'America/Mexico_City' });
  let branches = $state([]);
  let place = $state({ address: '', phone: '' }); // dirección y teléfono de la primera sucursal
  let selfBookable = $state(true);
  let hoursSaved = $state(false);
  let added = $state([]);
  let member = $state({ name: '', email: '', username: '', password: '', role: 'staff', bookable: true });
  let addBusy = $state(false);
  let busy = $state(false);
  let error = $state('');

  onMount(async () => {
    try {
      await loadSession();
    } catch {
      return goto('/login', { replaceState: true });
    }
    // solo el titular de un negocio nuevo ve el asistente
    if (isPlatform() || homeFor(session.me) !== '/bienvenida') return goto('/panel', { replaceState: true });
    try {
      biz = await api('/api/business');
      branches = (await api('/api/branches')).filter((b) => b.active);
      place = { address: branches[0]?.address ?? '', phone: branches[0]?.phone ?? '' };
      ready = true;
    } catch (e) {
      error = e.message;
    }
  });

  const zones = $derived(ZONES.some(([z]) => z === biz.timezone) ? ZONES : [[biz.timezone, biz.timezone], ...ZONES]);
  const valid = $derived(step !== 1 || biz.name.trim().length >= 2);

  async function run(fn) {
    error = '';
    busy = true;
    try {
      await fn();
      return true;
    } catch (e) {
      error = e.message;
      return false;
    } finally {
      busy = false;
    }
  }

  const clock = $derived(zoneClock(biz.timezone));

  async function saveBusiness() {
    await api('/api/business', { method: 'PUT', body: { name: biz.name.trim(), kind: biz.kind, timezone: biz.timezone } });
    if (branches[0]) await api(`/api/branches/${branches[0].id}`, { method: 'PATCH', body: { address: place.address.trim(), phone: place.phone.trim() } });
  }
  // cada paso guarda lo que recogió antes de avanzar
  async function next() {
    const ok = await run(async () => {
      if (step <= 1) await saveBusiness();
      else if (step === 2) await api(`/api/team/${session.me.id}`, { method: 'PATCH', body: { bookable: selfBookable } });
    });
    if (ok) step += 1;
  }

  async function addMember(e) {
    e.preventDefault();
    error = '';
    addBusy = true;
    try {
      const r = await api('/api/team', { method: 'POST', body: { ...member, username: member.username || undefined, commission_pct: 0 } });
      // si no escribió usuario, el servidor genera uno desde el correo: se muestra el que quedó
      const saved = (await api('/api/team')).find((m) => m.id === r.id);
      added.push({ id: r.id, name: member.name.trim(), email: member.email.trim(), username: saved?.username ?? '', role: member.role });
      member = { name: '', email: '', username: '', password: '', role: member.role, bookable: member.bookable };
    } catch (err) {
      error = err.message;
    } finally {
      addBusy = false;
    }
  }

  async function finish() {
    const ok = await run(async () => {
      await api('/api/business/setup/complete', { method: 'POST' });
      await loadSession();
    });
    if (ok) goto('/panel', { replaceState: true });
  }
</script>

<svelte:head><title>Configura tu negocio · Barbella</title></svelte:head>

{#if !ready}
  {#if error}<p class="error boot" role="alert">{error}</p>{:else}<p class="boot meta">Cargando…</p>{/if}
{:else}
  <div class="wiz">
    <div class="wrap">
      <header>
        <Logo />
        <div class="tools">
          <ThemeToggle />
          {#if step < steps.length - 1}<button type="button" class="btn ghost sm" onclick={finish} disabled={busy}>Configurar después</button>{/if}
        </div>
      </header>

      <div class="progress">
        <p class="label">Paso {step + 1} de {steps.length} · {steps[step]}</p>
        <div class="bars" role="progressbar" aria-valuemin="1" aria-valuemax={steps.length} aria-valuenow={step + 1} aria-label="Progreso de la configuración">
          {#each steps as _, i}<span class:on={i <= step}></span>{/each}
        </div>
      </div>

      <main class="card" aria-live="polite">
        {#if step === 0}
          <h1>Bienvenido a <em>Barbella</em></h1>
          <p class="lead">En unos minutos dejamos tu negocio listo para recibir citas. Empecemos por conocerlo: ¿qué tipo de negocio es?</p>
          <div class="kinds" role="radiogroup" aria-label="Giro del negocio">
            {#each Object.entries(KINDS) as [value, label]}
              <button type="button" role="radio" aria-checked={biz.kind === value} class="kind" class:sel={biz.kind === value} onclick={() => (biz.kind = value)}>
                <b>{label}</b>
                <span>{KIND_ABOUT[value]}</span>
                {#if biz.kind === value}<i class="ck"><Icon name="check" size={14} /></i>{/if}
              </button>
            {/each}
          </div>
        {:else if step === 1}
          <h1>Datos del <em>negocio</em></h1>
          <p class="lead">Aparecen en tu panel y en tu página de reservas. Podrás cambiarlos cuando quieras.</p>
          <label class="field">Nombre del negocio <input bind:value={biz.name} autocomplete="organization" /></label>
          <div class="two">
            <label class="field">Dirección <span class="meta">(opcional)</span> <input bind:value={place.address} maxlength="300" placeholder="Calle, número y colonia" autocomplete="street-address" /></label>
            <label class="field">Teléfono <span class="meta">(opcional)</span> <input type="tel" inputmode="tel" bind:value={place.phone} maxlength="30" placeholder="664 123 4567" autocomplete="tel-national" /></label>
          </div>
          <label class="field">
            Zona horaria
            <select bind:value={biz.timezone}>{#each zones as [z, l]}<option value={z}>{l}</option>{/each}</select>
            <span class="hint">{#if clock}En tu zona son las <b>{clock}</b>. {/if}Tus citas y recordatorios usan esta hora.</span>
          </label>
        {:else if step === 2}
          <h1>Tu <em>horario</em></h1>
          <p class="lead">Define cuándo atiendes. Evita empalmes y solo ofrece a tus clientes los huecos reales. El horario de cada persona de tu equipo se ajusta después en Ajustes → Equipo.</p>
          <label class="check own"><input type="checkbox" bind:checked={selfBookable} />Yo también atiendo clientes (aparezco en la agenda)</label>
          {#if selfBookable}
            <HoursEditor member={{ id: session.me.id, name: 'tu negocio' }} {branches} onsaved={() => (hoursSaved = true)} />
            {#if hoursSaved}<p class="ok"><Icon name="check" size={15} />Horario guardado.</p>{/if}
          {:else}
            <p class="meta">Sin problema: podrás dar de alta a tu equipo en el siguiente paso.</p>
          {/if}
        {:else if step === 3}
          <h1>Tu <em>equipo</em></h1>
          <p class="lead">Agrega a quienes van a usar Barbella. Cada quien ve solo lo que le toca. Este paso es opcional: puedes hacerlo después en <b>Ajustes → Equipo</b>.</p>
          {#if added.length}
            <ul class="team">
              {#each added as p (p.id)}
                <li>
                  <span class="av" aria-hidden="true">{p.name.charAt(0).toUpperCase()}</span>
                  <span class="who"><b>{p.name}</b><small>{p.username ? `${p.username} · ` : ''}{p.email}</small></span>
                  <span class="pill"><Icon name="check" size={12} />{ROLES[p.role]}</span>
                </li>
              {/each}
            </ul>
          {/if}
          <form class="addform" onsubmit={addMember}>
            <label class="field full">Nombre completo <input bind:value={member.name} required minlength="2" autocomplete="off" /></label>
            <label class="field">Correo <input type="email" bind:value={member.email} required autocomplete="off" autocapitalize="none" spellcheck="false" /></label>
            <label class="field">Usuario <span class="meta">(opcional)</span>
              <input value={member.username} oninput={(e) => (member.username = cleanUsername(e.currentTarget.value))} maxlength="30" placeholder="Se genera desde el correo" autocomplete="off" autocapitalize="none" autocorrect="off" spellcheck="false" />
              <span class="hint">Con él o con su correo podrá entrar.</span>
            </label>
            <label class="field">Contraseña inicial <input type="password" bind:value={member.password} required minlength="8" maxlength="72" autocomplete="new-password" /></label>
            <label class="field">Rol
              <select bind:value={member.role}>{#each Object.entries(ROLES) as [v, l]}<option value={v}>{l}</option>{/each}</select>
            </label>
            <label class="check full"><input type="checkbox" bind:checked={member.bookable} />Aparece en la agenda (atiende clientes)</label>
            <div class="full"><button class="btn ghost" disabled={addBusy}><Icon name="user-plus" size={17} />{addBusy ? 'Agregando…' : 'Agregar a la lista'}</button></div>
          </form>
        {:else}
          <span class="done"><Icon name="check" size={28} /></span>
          <h1>Todo <em>listo</em></h1>
          <p class="lead">Así quedó tu negocio. Todo se puede cambiar después en <b>Ajustes</b>.</p>
          <dl class="summary">
            <div><dt>Negocio</dt><dd>{biz.name}</dd></div>
            <div><dt>Giro</dt><dd>{KINDS[biz.kind]}</dd></div>
            {#if place.address || place.phone}<div><dt>Contacto</dt><dd>{[place.address, place.phone].filter(Boolean).join(' · ')}</dd></div>{/if}
            <div><dt>Para entrar</dt><dd>{session.me?.username ? `usuario ${session.me.username} o tu correo` : 'tu correo'}</dd></div>
            <div><dt>Horario</dt><dd>{selfBookable ? (hoursSaved ? 'Guardado' : 'Sin definir aún') : 'Lo defines en Equipo'}</dd></div>
            <div><dt>Equipo agregado</dt><dd>{added.length ? added.map((p) => p.name).join(', ') : 'Solo tú por ahora'}</dd></div>
          </dl>
          <ul class="next">
            <li><Icon name="scissors" size={19} />Registra tus servicios y precios</li>
            <li><Icon name="globe" size={19} />Comparte tu página de reservas</li>
            <li><Icon name="calendar" size={19} />Agenda tu primera cita</li>
            <li><Icon name="users" size={19} />Da de alta a tus clientes</li>
          </ul>
        {/if}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
      </main>

      <footer>
        {#if step > 0 && step < steps.length - 1}
          <button type="button" class="btn ghost" onclick={() => (step -= 1)} disabled={busy}><Icon name="arrow-left" size={17} />Atrás</button>
        {/if}
        <span class="grow"></span>
        {#if step < steps.length - 1}
          <button type="button" class="btn" onclick={next} disabled={!valid || busy}>{step === 3 && !added.length ? 'Omitir por ahora' : 'Continuar'}<Icon name="arrow" size={17} /></button>
        {:else}
          <button type="button" class="btn" onclick={finish} disabled={busy}>Ir a mi negocio<Icon name="arrow" size={17} /></button>
        {/if}
      </footer>
    </div>
  </div>
{/if}

<style>
  .boot { padding: 48px 20px; text-align: center; }
  .wiz { min-height: 100vh; background: var(--bg) radial-gradient(ellipse 60% 420px at 50% 0%, var(--accent-soft), transparent 75%) no-repeat; }
  .wrap { max-width: 760px; margin: 0 auto; min-height: 100vh; display: flex; flex-direction: column; padding: 20px 16px 16px; }
  header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .tools { display: flex; align-items: center; gap: 8px; }
  .progress { margin-top: 28px; }
  .label { margin: 0 0 12px; font-size: 0.72rem; font-weight: 600; letter-spacing: 0.14em; text-transform: uppercase; color: var(--muted); }
  .bars { display: flex; gap: 6px; }
  .bars span { flex: 1; height: 4px; border-radius: 999px; background: var(--border); transition: background-color 0.5s var(--ease); }
  .bars span.on { background: var(--accent); }
  main.card { flex: 1; margin-top: 20px; padding: 28px 22px; }
  h1 { font-size: clamp(2.1rem, 6vw, 3rem); line-height: 1; margin: 0; }
  h1 em { font-style: italic; color: var(--accent); }
  .lead { color: var(--muted); margin: 0.9rem 0 1.75rem; }
  .meta { color: var(--muted); }
  footer { display: flex; align-items: center; gap: 12px; margin-top: 18px; }
  .grow { flex: 1; }

  .kinds { display: grid; gap: 12px; grid-template-columns: 1fr; }
  .kind { position: relative; display: grid; gap: 4px; text-align: left; padding: 16px 18px; font: inherit; color: var(--text); background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; transition: border-color 0.2s var(--ease), box-shadow 0.2s var(--ease), transform 0.12s var(--ease); }
  .kind:hover { border-color: var(--muted); }
  .kind:active { transform: scale(0.98); }
  .kind span { font-size: 0.85rem; color: var(--muted); }
  .kind.sel { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent), 0 0 0 4px var(--accent-soft); }
  .ck { position: absolute; top: 12px; right: 12px; display: grid; place-items: center; width: 22px; height: 22px; border-radius: 50%; background: var(--accent); color: var(--accent-contrast); }
  .own { margin-bottom: 1.25rem; }
  .two { display: grid; gap: 0 14px; grid-template-columns: 1fr; }
  @media (min-width: 640px) { .two { grid-template-columns: 1fr 1fr; } }
  .ok { display: flex; align-items: center; gap: 6px; margin: 0.75rem 0 0; font-size: 0.9rem; font-weight: 500; color: var(--accent); }

  .team { list-style: none; margin: 0 0 1.25rem; padding: 0; border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .team li { display: flex; align-items: center; gap: 12px; padding: 10px 14px; }
  .team li + li { border-top: 1px solid var(--border); }
  .av { flex: none; display: grid; place-items: center; width: 36px; height: 36px; border-radius: 50%; background: var(--accent-soft); color: var(--accent); font-weight: 700; }
  .who { flex: 1; min-width: 0; display: grid; }
  .who b, .who small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .who small { color: var(--muted); }
  .pill { display: inline-flex; align-items: center; gap: 4px; padding: 3px 10px; border-radius: 999px; font-size: 0.78rem; font-weight: 600; background: var(--accent-soft); color: var(--accent); }
  .addform { display: grid; gap: 0 14px; grid-template-columns: 1fr; padding: 16px; border-radius: var(--radius-sm); background: var(--surface-2); }

  .done { display: grid; place-items: center; width: 56px; height: 56px; margin-bottom: 1.25rem; border-radius: 16px; background: var(--accent-soft); color: var(--accent); }
  .summary { display: grid; gap: 16px 32px; grid-template-columns: 1fr; margin: 0 0 1.5rem; padding: 20px; border-radius: var(--radius-sm); background: var(--surface-2); }
  .summary dt { font-size: 0.72rem; font-weight: 600; letter-spacing: 0.14em; text-transform: uppercase; color: var(--muted); }
  .summary dd { margin: 4px 0 0; font-weight: 600; }
  .next { list-style: none; margin: 0; padding: 0; display: grid; gap: 10px; grid-template-columns: 1fr; font-size: 0.92rem; }
  .next li { display: flex; align-items: center; gap: 12px; padding: 12px 14px; border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .next :global(svg) { color: var(--accent); flex: none; }

  @media (min-width: 640px) {
    main.card { padding: 40px 44px; }
    .kinds, .next, .summary { grid-template-columns: 1fr 1fr; }
    .addform { grid-template-columns: 1fr 1fr; }
    .full { grid-column: 1 / -1; }
  }
</style>
