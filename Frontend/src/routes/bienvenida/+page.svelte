<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import HoursEditor from '$lib/HoursEditor.svelte';
  import Icon from '$lib/Icon.svelte';
  import Logo from '$lib/Logo.svelte';
  import { api } from '$lib/api.js';
  import { KINDS, ROLES } from '$lib/format.js';
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
  let selfBookable = $state(true);
  let hoursSaved = $state(false);
  let added = $state([]);
  let member = $state({ name: '', email: '', password: '', role: 'staff', bookable: true });
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

  const saveBusiness = () => api('/api/business', { method: 'PUT', body: { name: biz.name.trim(), kind: biz.kind, timezone: biz.timezone } });

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
      const r = await api('/api/team', { method: 'POST', body: { ...member, commission_pct: 0 } });
      added.push({ id: r.id, name: member.name.trim(), email: member.email.trim(), role: member.role });
      member = { name: '', email: '', password: '', role: member.role, bookable: member.bookable };
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
        {#if step < steps.length - 1}<button type="button" class="btn ghost sm" onclick={finish} disabled={busy}>Configurar después</button>{/if}
      </header>

      <div class="progress" role="progressbar" aria-valuemin="1" aria-valuemax={steps.length} aria-valuenow={step + 1} aria-valuetext="Paso {step + 1} de {steps.length}: {steps[step]}" aria-label="Progreso de la configuración">
        <p class="count">Paso {step + 1} de {steps.length} · {steps[step]}</p>
        <ol>
          {#each steps as label, i}
            <li class:on={i === step} class:past={i < step} aria-current={i === step ? 'step' : undefined}>
              <span class="num" aria-hidden="true">{#if i < step}<Icon name="check" size={14} />{:else}{i + 1}{/if}</span>
              <span class="name">{label}</span>
            </li>
          {/each}
        </ol>
      </div>

      <main class="panel" aria-live="polite">
        {#if step === 0}
          <h1>Bienvenido a Barbella</h1>
          <p class="lead">En unos minutos dejamos tu negocio listo para recibir citas. Empecemos por conocerlo: ¿qué tipo de negocio es?</p>
          <div class="kinds" role="radiogroup" aria-label="Giro del negocio">
            {#each Object.entries(KINDS) as [value, label]}
              <button type="button" role="radio" aria-checked={biz.kind === value} class="kind" class:sel={biz.kind === value} onclick={() => (biz.kind = value)}>
                <span class="mark" aria-hidden="true">{#if biz.kind === value}<Icon name="check" size={13} />{/if}</span>
                <span class="txt"><b>{label}</b><span>{KIND_ABOUT[value]}</span></span>
              </button>
            {/each}
          </div>
        {:else if step === 1}
          <h1>Datos del negocio</h1>
          <p class="lead">Aparecen en tu panel y en tu página de reservas. Podrás cambiarlos cuando quieras.</p>
          <label class="field">Nombre del negocio <input bind:value={biz.name} autocomplete="organization" /></label>
          <label class="field">
            Zona horaria
            <select bind:value={biz.timezone}>{#each zones as [z, l]}<option value={z}>{l}</option>{/each}</select>
            <span class="hint">Tus citas y recordatorios usan esta hora.</span>
          </label>
        {:else if step === 2}
          <h1>Tu horario</h1>
          <p class="lead">Define cuándo atiendes. Evita empalmes y solo ofrece a tus clientes los huecos reales. El horario de cada persona de tu equipo se ajusta después en Ajustes → Equipo.</p>
          <label class="check own"><input type="checkbox" bind:checked={selfBookable} />Yo también atiendo clientes (aparezco en la agenda)</label>
          {#if selfBookable}
            <HoursEditor member={{ id: session.me.id, name: 'tu negocio' }} {branches} onsaved={() => (hoursSaved = true)} />
            {#if hoursSaved}<p class="ok"><Icon name="check" size={15} />Horario guardado.</p>{/if}
          {:else}
            <p class="meta">Sin problema: podrás dar de alta a tu equipo en el siguiente paso.</p>
          {/if}
        {:else if step === 3}
          <h1>Tu equipo</h1>
          <p class="lead">Agrega a quienes van a usar Barbella. Cada quien ve solo lo que le toca. Este paso es opcional: puedes hacerlo después en <b>Ajustes → Equipo</b>.</p>
          {#if added.length}
            <ul class="team">
              {#each added as p (p.id)}
                <li>
                  <span class="who"><b>{p.name}</b><small>{p.email}</small></span>
                  <span class="pill"><Icon name="check" size={12} />{ROLES[p.role]}</span>
                </li>
              {/each}
            </ul>
          {/if}
          <form class="addform" onsubmit={addMember}>
            <label class="field full">Nombre completo <input bind:value={member.name} required minlength="2" autocomplete="off" /></label>
            <label class="field">Correo <input type="email" bind:value={member.email} required autocomplete="off" /></label>
            <label class="field">Contraseña inicial <input type="password" bind:value={member.password} required minlength="8" maxlength="72" autocomplete="new-password" /></label>
            <label class="field">Rol
              <select bind:value={member.role}>{#each Object.entries(ROLES) as [v, l]}<option value={v}>{l}</option>{/each}</select>
            </label>
            <label class="check full"><input type="checkbox" bind:checked={member.bookable} />Aparece en la agenda (atiende clientes)</label>
            <div class="full"><button class="btn ghost" disabled={addBusy}><Icon name="user-plus" size={17} />{addBusy ? 'Agregando…' : 'Agregar a la lista'}</button></div>
          </form>
        {:else}
          <h1>Todo listo</h1>
          <p class="lead">Así quedó tu negocio. Todo se puede cambiar después en <b>Ajustes</b>.</p>
          <dl class="summary">
            <div><dt>Negocio</dt><dd>{biz.name}</dd></div>
            <div><dt>Giro</dt><dd>{KINDS[biz.kind]}</dd></div>
            <div><dt>Horario</dt><dd>{selfBookable ? (hoursSaved ? 'Guardado' : 'Sin definir aún') : 'Lo defines en Equipo'}</dd></div>
            <div><dt>Equipo agregado</dt><dd>{added.length ? added.map((p) => p.name).join(', ') : 'Solo tú por ahora'}</dd></div>
          </dl>
          <h2>Siguientes pasos</h2>
          <ol class="next">
            <li>Registra tus servicios y precios</li>
            <li>Comparte tu página de reservas</li>
            <li>Agenda tu primera cita</li>
            <li>Da de alta a tus clientes</li>
          </ol>
        {/if}
        {#if error}<p class="error" role="alert">{error}</p>{/if}

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
      </main>
    </div>
  </div>
{/if}

<style>
  .boot { padding: 48px 20px; text-align: center; }
  .meta { color: var(--muted); }
  .wiz { min-height: 100vh; min-height: 100dvh; background: var(--bg); }
  .wrap { max-width: 720px; margin: 0 auto; padding: 16px 16px 40px; }
  header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }

  .progress { margin-top: 20px; }
  .count { margin: 0 0 10px; font-size: 0.85rem; font-weight: 600; color: var(--muted); }
  .progress ol { list-style: none; margin: 0; padding: 0; display: flex; gap: 6px; }
  .progress li { flex: 1; display: flex; align-items: center; gap: 8px; min-width: 0; padding-top: 10px; border-top: 3px solid var(--border); color: var(--muted); font-size: 0.85rem; font-weight: 500; }
  .progress li.past, .progress li.on { border-top-color: var(--accent); }
  .progress li.on { color: var(--text); font-weight: 600; }
  .num { flex: none; display: grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; border: 1px solid var(--control-border); font-size: 0.78rem; font-variant-numeric: tabular-nums; }
  .past .num { background: var(--accent); border-color: var(--accent); color: var(--accent-contrast); }
  .on .num { border-color: var(--accent); color: var(--accent); }
  .name { display: none; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  .panel { margin-top: 16px; padding: 24px 20px; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  h1 { font-size: clamp(1.6rem, 5vw, 2.1rem); line-height: 1.1; margin: 0; }
  h2 { font-size: 1rem; margin: 1.75rem 0 0.5rem; }
  .lead { color: var(--muted); margin: 0.6rem 0 1.5rem; max-width: 56ch; }
  footer { display: flex; align-items: center; gap: 12px; margin-top: 28px; padding-top: 20px; border-top: 1px solid var(--border); }
  .grow { flex: 1; }
  .error { margin: 1rem 0 0; }

  .kinds { display: grid; gap: 10px; grid-template-columns: 1fr; }
  .kind { display: flex; align-items: flex-start; gap: 12px; min-height: 64px; text-align: left; padding: 14px 16px; font: inherit; color: var(--text); background: var(--surface); border: 1px solid var(--control-border); border-radius: var(--radius-sm); cursor: pointer; }
  .kind:hover { border-color: var(--text); }
  .kind.sel { border-color: var(--accent); background: var(--accent-soft); }
  .mark { flex: none; display: grid; place-items: center; width: 20px; height: 20px; margin-top: 2px; border-radius: 50%; border: 1.5px solid var(--control-border); background: var(--surface); color: var(--accent-contrast); }
  .sel .mark { background: var(--accent); border-color: var(--accent); }
  .txt { display: grid; gap: 2px; }
  .txt span { font-size: 0.85rem; color: var(--muted); }
  .own { margin-bottom: 1rem; }
  .ok { display: flex; align-items: center; gap: 6px; margin: 0.75rem 0 0; font-size: 0.9rem; font-weight: 500; color: var(--accent); }

  .team { list-style: none; margin: 0 0 1.25rem; padding: 0; border-top: 1px solid var(--border); }
  .team li { display: flex; align-items: center; gap: 12px; padding: 12px 0; border-bottom: 1px solid var(--border); }
  .who { flex: 1; min-width: 0; display: grid; }
  .who b, .who small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .who small { color: var(--muted); }
  .pill { display: inline-flex; align-items: center; gap: 4px; padding: 3px 10px; border-radius: 999px; font-size: 0.78rem; font-weight: 600; background: var(--surface-2); color: var(--text); }
  .addform { display: grid; gap: 0 14px; grid-template-columns: 1fr; padding-top: 4px; }

  .summary { display: grid; gap: 0; grid-template-columns: 1fr; margin: 0; border-top: 1px solid var(--border); }
  .summary div { padding: 12px 0; border-bottom: 1px solid var(--border); }
  .summary dt { font-size: 0.8rem; color: var(--muted); }
  .summary dd { margin: 2px 0 0; font-weight: 600; overflow-wrap: anywhere; }
  .next { margin: 0; padding-left: 1.25rem; display: grid; gap: 6px; font-size: 0.95rem; }

  @media (min-width: 640px) {
    .wrap { padding-top: 24px; }
    .count { display: none; }
    .name { display: block; }
    .panel { padding: 36px 40px; }
    .kinds { grid-template-columns: 1fr 1fr; }
    .addform { grid-template-columns: 1fr 1fr; }
    .full { grid-column: 1 / -1; }
    .summary { grid-template-columns: 1fr 1fr; column-gap: 32px; }
  }
</style>
