<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Avatar from '$lib/Avatar.svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { BILLING, KINDS, PAY_METHODS, PLANS, ROLES, ACTIONS, ago, formatDate, formatDay, money, plural, pesos, toCents } from '$lib/format.js';
  import { canWritePlatform } from '$lib/session.svelte.js';

  const id = page.params.id;
  const write = canWritePlatform();

  let data = $state(null);
  let activity = $state([]);
  let error = $state('');
  let notice = $state('');
  let tab = $state('resumen');
  let trialDate = $state('');
  let periodDate = $state('');
  let plan = $state('');
  let reason = $state('');
  let suspending = $state(false);
  let resetFor = $state(null);
  let newPassword = $state('');
  let plans = $state([]);
  let pay = $state({ amount: '', months: 1, plan: '', method: 'transfer', note: '' });

  // soporte es de solo lectura: no ve Licencia y pagos
  const TABS = [['resumen', 'Resumen'], ...(write ? [['licencia', 'Licencia y pagos']] : []), ['personas', 'Personas'], ['actividad', 'Actividad']];

  async function load() {
    data = await api(`/api/platform/businesses/${id}`);
    plan = data.business.plan;
    pay.plan = data.business.plan;
    activity = await api(`/api/platform/activity?business_id=${id}&limit=20`);
    plans = await api('/api/platform/plans');
  }
  onMount(() => load().catch((e) => (error = e.status === 404 ? 'No encontramos este negocio.' : e.message)));

  async function run(fn, okMsg) {
    error = '';
    notice = '';
    try {
      await fn();
      notice = okMsg;
      await load();
      window.dispatchEvent(new Event('clients-changed')); // la lista de negocios refresca su estado y plan
    } catch (e) {
      error = e.message;
    }
  }

  const patch = (body, okMsg) => run(() => api(`/api/platform/businesses/${id}`, { method: 'PATCH', body }), okMsg);
  const toggleUser = (u) =>
    run(() => api(`/api/platform/users/${u.id}`, { method: 'PATCH', body: { active: !u.active } }), u.active ? 'Persona desactivada.' : 'Persona reactivada.');
  const resetTwoFactor = (u) =>
    confirm(`¿Quitar la verificación en dos pasos a ${u.name}?`) && run(() => api(`/api/platform/users/${u.id}/2fa-reset`, { method: 'POST' }), 'Verificación en dos pasos quitada.');

  async function resetPassword(e, u) {
    e.preventDefault();
    await run(() => api(`/api/platform/users/${u.id}/password`, { method: 'POST', body: { password: newPassword } }), `Contraseña de ${u.name} restablecida. Compártela de forma segura.`);
    newPassword = '';
    resetFor = null;
  }

  async function suspend(e) {
    e.preventDefault();
    await patch({ status: 'suspended', suspended_reason: reason }, 'Negocio suspendido.');
    suspending = false;
    reason = '';
  }

  async function record(e) {
    e.preventDefault();
    const cents = pay.method === 'courtesy' ? 0 : toCents(pay.amount);
    if (Number.isNaN(cents) || (pay.method !== 'courtesy' && cents <= 0)) return (error = 'Escribe el monto en pesos, por ejemplo 799');
    await run(
      () => api(`/api/platform/businesses/${id}/payments`, { method: 'POST', body: { amount_cents: cents, months: Number(pay.months), plan: pay.plan, method: pay.method, note: pay.note } }),
      'Pago registrado. La suscripción quedó vigente.'
    );
    pay.amount = '';
    pay.note = '';
  }

  const limitOf = (v) => (v === null || v === undefined ? '∞' : v);
  const pct = (used, max) => (max === null || max === undefined ? 0 : Math.min(100, Math.round((used / Math.max(max, 1)) * 100)));
  const tone = (used, max) => (max === null || max === undefined ? '' : used > max ? 'over' : used === max ? 'full' : '');
  // precio de lista del plan elegido: 12 meses cobra el precio anual; otros periodos, meses × precio mensual
  const priceHint = () => {
    const p = plans.find((x) => x.id === pay.plan);
    return p ? (Number(pay.months) === 12 ? p.price_year : p.price_month * Number(pay.months)) : null;
  };
</script>

<a class="back" href="/plataforma/negocios"><Icon name="arrow" size={16} />Negocios</a>

{#if error}<p class="error" role="alert">{error}</p>{/if}
{#if notice}<p class="banner" role="status">{notice}</p>{/if}

{#if data}
  {@const b = data.business}
  {@const [label, billTone] = BILLING[b.billing_state]}
  <section class="card head">
    <div class="top">
      <Avatar name={b.name} size="3.2rem" />
      <div class="grow">
        <h1>{b.name}</h1>
        <p class="chips">
          <span class="badge {billTone}">{label}</span>
          <span class="chip">{PLANS[b.plan]}</span>
          <span class="chip">{KINDS[b.kind]}</span>
          <span class="meta">identificador <code>{b.slug}</code></span>
        </p>
      </div>
      {#if write}
        <div class="acts">
          {#if b.status === 'pending'}<button class="btn sm" onclick={() => patch({ status: 'trial' }, 'Negocio aprobado: inició su prueba de 14 días.')}>Aprobar</button>{/if}
          {#if b.status === 'suspended'}
            <button class="btn sm" onclick={() => patch({ status: b.last_payment_at || b.current_period_end ? 'active' : 'trial' }, 'Negocio reactivado.')}>Reactivar</button>
          {:else if !suspending}
            <button class="btn ghost sm" onclick={() => (suspending = true)}>Suspender…</button>
          {/if}
        </div>
      {/if}
    </div>
    {#if suspending}
      <form class="reset" onsubmit={suspend}>
        <input bind:value={reason} maxlength="300" placeholder="Motivo (opcional, lo verá el negocio)" aria-label="Motivo de la suspensión" />
        <button class="btn sm">Suspender</button>
        <button type="button" class="btn ghost sm" onclick={() => (suspending = false)}>Cancelar</button>
      </form>
    {/if}
    {#if b.suspended_reason}<p class="banner">Suspendido: {b.suspended_reason}</p>{/if}

    <nav class="tabs" aria-label="Secciones del negocio">
      {#each TABS as [v, l]}
        <button type="button" class="chip" aria-pressed={tab === v} onclick={() => (tab = v)}>{l}</button>
      {/each}
    </nav>
  </section>

  {#if tab === 'resumen'}
    <section class="card">
      <dl class="facts">
        <div>
          <dt>Responsable</dt>
          <dd>{#if b.owner_email}<a href="mailto:{b.owner_email}">{b.owner_email}</a>{:else}Sin responsable{/if}</dd>
          <small>{plural(b.users, 'persona', 'personas')} · {plural(b.clients, 'cliente', 'clientes')}</small>
        </div>
        <div>
          <dt>Alta</dt>
          <dd>{formatDay(b.created_at)}</dd>
          <small>{ago(b.created_at)}</small>
        </div>
        <div>
          <dt>Última cita creada</dt>
          <dd>{data.metrics?.last_appointment_at ? formatDay(data.metrics.last_appointment_at) : 'Nunca'}</dd>
          <small>{data.metrics?.last_appointment_at ? ago(data.metrics.last_appointment_at) : 'sin actividad todavía'}</small>
        </div>
        <div>
          <dt>Plan</dt>
          <dd>{PLANS[b.plan]}</dd>
          <small>
            {#if b.status === 'trial'}prueba hasta {formatDay(b.trial_ends_at)}{#if b.trial_days_left !== null} ({plural(b.trial_days_left, 'día', 'días')}){/if}
            {:else if b.current_period_end}vigente hasta {formatDay(b.current_period_end)}
            {:else if b.status === 'active'}activo sin vencimiento (cortesía)
            {:else}sin periodo pagado{/if}
            {#if b.last_payment_at} · último pago {formatDay(b.last_payment_at)}{/if}
          </small>
        </div>
      </dl>
    </section>

    {#if data.usage && data.limits}
      <section class="card">
        <h2>Uso del plan</h2>
        <div class="meters">
          <div>
            <div class="mhead"><span>Personas en agenda</span><b class="num">{data.usage.staff} / {limitOf(data.limits.max_staff)}</b></div>
            <div class="meter {tone(data.usage.staff, data.limits.max_staff)}"><i style="width: {pct(data.usage.staff, data.limits.max_staff)}%"></i></div>
          </div>
          <div>
            <div class="mhead"><span>Sucursales</span><b class="num">{data.usage.branches} / {data.limits.max_branches}</b></div>
            <div class="meter {tone(data.usage.branches, data.limits.max_branches)}"><i style="width: {pct(data.usage.branches, data.limits.max_branches)}%"></i></div>
          </div>
        </div>
        <p class="meta feats">
          <span>Cobros: {data.limits.pos ? 'sí' : 'no'}</span>
          <span>Anticipos: {data.limits.deposits_mp ? 'transferencia y Mercado Pago' : data.limits.deposits ? 'solo transferencia' : 'no'}</span>
          <span>Terminal: {data.limits.terminal ? 'sí' : 'no'}</span>
        </p>
      </section>
    {/if}

    {#if data.metrics}
      <section class="kpis" aria-label="Uso en los últimos 30 días">
        <div class="kpi"><span class="lbl">Citas (30 días)</span><span class="body"><b class="v">{data.metrics.appointments_30d}</b></span></div>
        <div class="kpi"><span class="lbl">Reservas en línea</span><span class="body"><b class="v">{data.metrics.online_bookings_30d}</b></span></div>
        <div class="kpi"><span class="lbl">Ventas</span><span class="body"><b class="v">{data.metrics.sales_30d}</b></span></div>
      </section>
    {/if}
  {/if}

  {#if tab === 'licencia' && write}
    <section class="card">
      <h2>Plan y fechas</h2>
      <p class="meta">Cambia el plan o ajusta cuándo termina la prueba o el periodo pagado.</p>
      <div class="grid3">
        <div>
          <label class="field" style="margin: 0">Plan
            <select bind:value={plan}>{#each Object.entries(PLANS) as [v, l]}<option value={v}>{l}</option>{/each}</select>
          </label>
          <button class="btn ghost sm" disabled={plan === b.plan} onclick={() => patch({ plan }, 'Plan cambiado.')}>Cambiar plan</button>
        </div>
        <div>
          <label class="field" style="margin: 0">Fin de la prueba <input type="date" bind:value={trialDate} /></label>
          <button class="btn ghost sm" disabled={!trialDate} onclick={() => patch({ trial_ends_at: trialDate }, 'Fecha de prueba actualizada.')}>Guardar</button>
        </div>
        <div>
          <label class="field" style="margin: 0">Fin del periodo pagado <input type="date" bind:value={periodDate} /></label>
          <button class="btn ghost sm" disabled={!periodDate} onclick={() => patch({ current_period_end: periodDate, status: 'active' }, 'Vigencia actualizada.')}>Guardar</button>
        </div>
      </div>
    </section>

    <section class="card">
      <h2>Registrar un pago</h2>
      <p class="meta">Para transferencias, efectivo o cortesías. El periodo se suma al vigente y el negocio queda activo.</p>
      <form onsubmit={record}>
        <div class="form-grid">
          <label class="field">Plan
            <select bind:value={pay.plan}>{#each Object.entries(PLANS) as [v, l]}<option value={v}>{l}</option>{/each}</select>
          </label>
          <label class="field">Meses
            <select bind:value={pay.months}>{#each [1, 2, 3, 6, 12, 24] as m}<option value={m}>{m}</option>{/each}</select>
          </label>
          <label class="field">Método
            <select bind:value={pay.method}>{#each Object.entries(PAY_METHODS).filter(([k]) => k !== 'mercadopago') as [v, l]}<option value={v}>{l}</option>{/each}</select>
          </label>
          <label class="field">Monto (MXN)
            <input inputmode="decimal" bind:value={pay.amount} disabled={pay.method === 'courtesy'} placeholder={priceHint() ? String(priceHint()) : ''} />
            {#if priceHint() && pay.method !== 'courtesy'}<span class="hint">Precio de lista: {pesos(priceHint())}</span>{/if}
          </label>
        </div>
        <label class="field">Nota (opcional) <input bind:value={pay.note} maxlength="300" placeholder="Ej. SPEI 0012345" /></label>
        <button class="btn">Registrar pago</button>
      </form>
    </section>

    <section>
      <h2>Pagos</h2>
      <div class="list">
        {#each data.payments ?? [] as p (p.id)}
          <div class="item-row">
            <div class="grow">
              <div class="title">{money(p.amount_cents)} · {PAY_METHODS[p.method]}</div>
              <div class="meta">Plan {PLANS[p.plan] ?? p.plan} · {plural(p.months, 'mes', 'meses')} · hasta {formatDay(p.period_end)}{p.note ? ' · ' + p.note : ''}</div>
            </div>
            <span class="meta">{p.created_by} · {formatDate(p.created_at)}</span>
          </div>
        {:else}
          <div class="empty">Aún no hay pagos.</div>
        {/each}
      </div>
    </section>
  {/if}

  {#if tab === 'personas'}
    <section>
      <h2>Equipo del negocio</h2>
      <div class="list">
        {#each data.team as u (u.id)}
          <div class="item-row" class:off={!u.active}>
            <Avatar name={u.name} size="2.2rem" />
            <div class="grow">
              <div class="title">{u.name} <span class="badge muted">{ROLES[u.role]}</span>{#if !u.active} <span class="badge bad">Inactivo</span>{/if}</div>
              <div class="meta">{u.email}</div>
            </div>
            {#if write}
              <button class="btn ghost sm" onclick={() => ((resetFor = resetFor === u.id ? null : u.id), (newPassword = ''))}>Restablecer contraseña</button>
              <button class="btn ghost sm" onclick={() => resetTwoFactor(u)}>Quitar 2 pasos</button>
              <button class="btn ghost sm" onclick={() => toggleUser(u)}>{u.active ? 'Desactivar' : 'Reactivar'}</button>
            {/if}
            {#if resetFor === u.id}
              <form class="reset" onsubmit={(e) => resetPassword(e, u)}>
                <label class="sr-only" for="pw-{u.id}">Contraseña nueva</label>
                <input id="pw-{u.id}" type="password" bind:value={newPassword} minlength="8" maxlength="72" placeholder="Contraseña nueva (8+ caracteres)" autocomplete="new-password" required />
                <button class="btn sm">Guardar</button>
              </form>
            {/if}
          </div>
        {/each}
      </div>
    </section>
  {/if}

  {#if tab === 'actividad'}
    <section>
      <h2>Actividad reciente</h2>
      <div class="list">
        {#each activity as a (a.id)}
          <div class="item-row"><span class="grow">{ACTIONS[a.action] ?? a.action}</span><span class="meta">{a.actor} · {formatDate(a.created_at)}</span></div>
        {:else}
          <div class="empty">Sin actividad registrada.</div>
        {/each}
      </div>
    </section>
  {/if}
{/if}

<style>
  .card { margin-bottom: 1rem; }
  section:not(.card):not(.kpis) { margin-top: 1.5rem; }
  section h2 { font-size: 1.3rem; }
  .card h2 { margin-bottom: 0.3rem; }
  code { background: var(--surface-2); padding: 1px 6px; border-radius: 4px; font-size: 0.85em; }
  .meta { color: var(--muted); font-size: 0.88rem; }

  .head h1 { font-size: 1.8rem; margin: 0; }
  .top { display: flex; flex-wrap: wrap; align-items: center; gap: 12px 14px; }
  .top .grow { flex: 1 1 220px; min-width: 0; }
  .chips { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 8px; margin: 0.35rem 0 0; }
  .chip { display: inline-flex; align-items: center; min-height: 28px; padding: 0 0.7rem; font: inherit; font-size: 0.8rem; font-weight: 600; color: var(--muted); background: var(--surface-2); border: 0; border-radius: 999px; }
  .acts { display: flex; gap: 8px; }
  .tabs { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 1.1rem; padding-top: 1rem; border-top: 1px solid var(--border); }
  .tabs .chip { min-height: 36px; padding: 0 1rem; font-size: 0.88rem; cursor: pointer; color: var(--text); background: var(--surface); box-shadow: inset 0 0 0 1px var(--border); }
  .tabs .chip[aria-pressed='true'] { color: var(--accent-contrast); background: var(--accent); box-shadow: none; }

  .facts { margin: 0; display: grid; gap: 18px 24px; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); }
  .facts dt { font-size: 0.78rem; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: var(--muted); }
  .facts dd { margin: 2px 0; font-weight: 600; overflow-wrap: anywhere; }
  .facts small { color: var(--muted); font-size: 0.84rem; }

  .meters { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); margin-top: 0.9rem; }
  .mhead { display: flex; justify-content: space-between; gap: 10px; font-size: 0.9rem; margin-bottom: 6px; }
  .num { font-variant-numeric: tabular-nums; }
  .meter { height: 8px; border-radius: 999px; background: var(--surface-2); overflow: hidden; }
  .meter i { display: block; height: 100%; border-radius: inherit; background: var(--accent); }
  .meter.full i { background: var(--warn); }
  .meter.over i { background: var(--danger); }
  .feats { display: flex; flex-wrap: wrap; gap: 4px 18px; margin: 1rem 0 0; }

  .grid3 { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; margin-top: 1.25rem; }
  .grid3 button { margin-top: 8px; }
  .reset { display: flex; gap: 8px; flex: 1 1 100%; flex-wrap: wrap; margin-top: 0.75rem; }
  .reset input { margin: 0; flex: 1 1 240px; }
</style>
