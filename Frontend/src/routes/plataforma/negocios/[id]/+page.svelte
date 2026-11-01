<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Iniciales from '../Iniciales.svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { BILLING, KINDS, PAY_METHODS, PLANS, ROLES, ACTIONS, ago, formatDate, formatDay, money, plural, pesos, toCents } from '$lib/format.js';
  import { canWritePlatform } from '$lib/session.svelte.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import SegmentedControl from '$lib/ui/SegmentedControl.svelte';
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import FormSection from '$lib/ui/FormSection.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';

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
  const tabOptions = TABS.map(([value, label]) => ({ value, label }));

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

  const TONES = { pending: 'warning', trialing: 'info', trial_expired: 'danger', active: 'success', past_due: 'danger', suspended: 'danger' };
  const payColumns = [
    { key: 'amount', label: 'Importe', numeric: true },
    { key: 'method', label: 'Método' },
    { key: 'plan', label: 'Plan y periodo' },
    { key: 'by', label: 'Registró' }
  ];
  const teamColumns = [
    { key: 'name', label: 'Persona' },
    { key: 'role', label: 'Rol' },
    { key: 'state', label: 'Acceso' },
    ...(write ? [{ key: 'ops', label: 'Acciones' }] : [])
  ];
  const actColumns = [
    { key: 'action', label: 'Acción' },
    { key: 'actor', label: 'Quién' },
    { key: 'at', label: 'Fecha' }
  ];
</script>

<a class="back" href="/plataforma/negocios"><Icon name="arrow" size={16} />Negocios</a>

{#if error}<div class="gap"><InlineNotice tone="danger" message={error} /></div>{/if}
{#if notice}<div class="gap"><InlineNotice tone="success" message={notice} /></div>{/if}

{#if data}
  {@const b = data.business}
  {@const [label] = BILLING[b.billing_state]}
  <PageHeader title={b.name} description="{KINDS[b.kind]} · identificador {b.slug} · alta {formatDay(b.created_at)}">
    {#snippet leading()}<Iniciales name={b.name} size="3.25rem" />{/snippet}
    {#snippet actions()}
      <StatusBadge {label} tone={TONES[b.billing_state] ?? 'neutral'} />
      {#if write}
        {#if b.status === 'pending'}<button class="btn" onclick={() => patch({ status: 'trial' }, 'Negocio aprobado: inició su prueba de 14 días.')}>Aprobar</button>{/if}
        {#if b.status === 'suspended'}
          <button class="btn" onclick={() => patch({ status: b.last_payment_at || b.current_period_end ? 'active' : 'trial' }, 'Negocio reactivado.')}>Reactivar</button>
        {:else if !suspending}
          <button class="btn ghost" onclick={() => (suspending = true)}>Suspender…</button>
        {/if}
      {/if}
    {/snippet}
  </PageHeader>

  {#if suspending}
    <form class="reset suspend" onsubmit={suspend}>
      <input bind:value={reason} maxlength="300" placeholder="Motivo (opcional, lo verá el negocio)" aria-label="Motivo de la suspensión" />
      <button class="btn sm">Suspender</button>
      <button type="button" class="btn ghost sm" onclick={() => (suspending = false)}>Cancelar</button>
    </form>
  {/if}
  {#if b.suspended_reason}<div class="gap"><InlineNotice tone="warning" message="Suspendido: {b.suspended_reason}" /></div>{/if}

  <div class="tabs"><SegmentedControl bind:value={tab} options={tabOptions} label="Secciones del negocio" /></div>

  <div class="stack">
    {#if tab === 'resumen'}
      <div class="metrics" role="group" aria-label="Resumen del negocio">
        <MetricCard label="Personas" value={b.users} />
        <MetricCard label="Clientes" value={b.clients} />
        {#if data.metrics}
          <MetricCard label="Citas (30 días)" value={data.metrics.appointments_30d} />
          <MetricCard label="Reservas en línea (30 días)" value={data.metrics.online_bookings_30d} />
          <MetricCard label="Ventas (30 días)" value={data.metrics.sales_30d} />
        {/if}
      </div>

      <Surface title="Cuenta y suscripción">
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
            {#if b.requested_plan}
              <small class="req">Pidió el plan <b>{PLANS[b.requested_plan]}</b>{#if write}<br /><button type="button" class="linkbtn" onclick={() => patch({ plan: b.requested_plan }, 'Plan solicitado activado.')}>Activar plan solicitado</button>{/if}</small>
            {/if}
            <small>
              {#if b.status === 'trial'}prueba hasta {formatDay(b.trial_ends_at)}{#if b.trial_days_left !== null} ({plural(b.trial_days_left, 'día', 'días')}){/if}
              {:else if b.current_period_end}vigente hasta {formatDay(b.current_period_end)}
              {:else if b.status === 'active'}activo sin vencimiento (cortesía)
              {:else}sin periodo pagado{/if}
              {#if b.last_payment_at} · último pago {formatDay(b.last_payment_at)}{/if}
            </small>
          </div>
        </dl>
      </Surface>

      {#if data.usage && data.limits}
        <Surface title="Uso del plan">
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
          <ul class="usage" aria-label="Funciones del plan">
            <li>Cobros: {data.limits.pos ? 'sí' : 'no'}</li>
            <li>Anticipos: {data.limits.deposits_mp ? 'transferencia y Mercado Pago' : data.limits.deposits ? 'solo transferencia' : 'no'}</li>
            <li>Terminal: {data.limits.terminal ? 'sí' : 'no'}</li>
          </ul>
        </Surface>
      {/if}
    {/if}

    {#if tab === 'licencia' && write}
      <Surface title="Operaciones">
        <FormSection title="Plan y fechas" description="Cambia el plan o ajusta cuándo termina la prueba o el periodo pagado.">
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
        </FormSection>

        <FormSection title="Registrar un pago" description="Para transferencias, efectivo o cortesías. El periodo se suma al vigente y el negocio queda activo.">
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
        </FormSection>
      </Surface>

      <Surface title="Pagos" padding="flush">
        {#if (data.payments ?? []).length}
          <DataTable caption="Pagos del negocio" columns={payColumns} rows={data.payments}>
            {#snippet cell({ row, column })}
              {#if column.key === 'amount'}<b>{money(row.amount_cents)}</b>
              {:else if column.key === 'method'}{PAY_METHODS[row.method]}
              {:else if column.key === 'plan'}
                <span class="two"><span>Plan {PLANS[row.plan] ?? row.plan} · {plural(row.months, 'mes', 'meses')}</span><span class="sub">hasta {formatDay(row.period_end)}{row.note ? ' · ' + row.note : ''}</span></span>
              {:else}<span class="two"><span>{row.created_by}</span><span class="sub">{formatDate(row.created_at)}</span></span>{/if}
            {/snippet}
          </DataTable>
        {:else}
          <EmptyState title="Aún no hay pagos" />
        {/if}
      </Surface>
    {/if}

    {#if tab === 'personas'}
      <Surface title="Equipo del negocio" padding="flush">
        <DataTable caption="Equipo del negocio" columns={teamColumns} rows={data.team}>
          {#snippet cell({ row: u, column })}
            {#if column.key === 'name'}
              <span class="who" class:off={!u.active}><Iniciales name={u.name} size="2.25rem" /><span class="two"><b>{u.name}</b><span class="sub">{u.email}</span></span></span>
            {:else if column.key === 'role'}{ROLES[u.role]}
            {:else if column.key === 'state'}<StatusBadge label={u.active ? 'Activo' : 'Inactivo'} tone={u.active ? 'success' : 'danger'} />
            {:else}
              <div class="ops">
                <button class="btn ghost sm" onclick={() => ((resetFor = resetFor === u.id ? null : u.id), (newPassword = ''))}>Restablecer contraseña</button>
                <button class="btn ghost sm" onclick={() => resetTwoFactor(u)}>Quitar 2 pasos</button>
                <button class="btn ghost sm" onclick={() => toggleUser(u)}>{u.active ? 'Desactivar' : 'Reactivar'}</button>
              </div>
            {/if}
            {#if column.key === (write ? 'ops' : 'state') && resetFor === u.id}
              <form class="reset" onsubmit={(e) => resetPassword(e, u)}>
                <label class="sr-only" for="pw-{u.id}">Contraseña nueva</label>
                <input id="pw-{u.id}" type="password" bind:value={newPassword} minlength="8" maxlength="72" placeholder="Contraseña nueva (8+ caracteres)" autocomplete="new-password" required />
                <button class="btn sm">Guardar</button>
              </form>
            {/if}
          {/snippet}
        </DataTable>
      </Surface>
    {/if}

    {#if tab === 'actividad'}
      <Surface title="Actividad reciente" padding="flush">
        {#if activity.length}
          <DataTable caption="Actividad reciente del negocio" columns={actColumns} rows={activity}>
            {#snippet cell({ row, column })}
              {#if column.key === 'action'}{ACTIONS[row.action] ?? row.action}
              {:else if column.key === 'actor'}{row.actor}
              {:else}{formatDate(row.created_at)}{/if}
            {/snippet}
          </DataTable>
        {:else}
          <EmptyState title="Sin actividad registrada" />
        {/if}
      </Surface>
    {/if}
  </div>
{/if}

<style>
  .back { display: inline-flex; align-items: center; gap: 6px; min-height: 44px; margin-bottom: 0.5rem; color: var(--muted); text-decoration: none; font-size: 0.9rem; }
  .back :global(svg) { transform: rotate(180deg); }
  .back:hover { color: var(--accent); }
  .gap { margin-bottom: 16px; }
  .tabs { margin-bottom: 24px; }
  .metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 16px; }
  .stack { display: grid; gap: 24px; }

  .facts { margin: 0; display: grid; gap: 20px 24px; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); }
  .facts dt { font-size: 0.8rem; font-weight: 600; color: var(--muted); }
  .facts dd { margin: 2px 0; font-weight: 600; overflow-wrap: anywhere; }
  .facts small { display: block; color: var(--muted); font-size: 0.85rem; }
  .req { margin: 2px 0 4px; color: var(--accent-text, var(--accent)); }
  .linkbtn { padding: 0; min-height: 44px; font: inherit; font-weight: 600; color: var(--accent-text, var(--accent)); background: none; border: 0; text-decoration: underline; cursor: pointer; }

  .meters { display: grid; gap: 16px 24px; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); }
  .mhead { display: flex; justify-content: space-between; gap: 10px; font-size: 0.9rem; margin-bottom: 6px; }
  .num { font-variant-numeric: tabular-nums; }
  .meter { height: 8px; border-radius: 999px; background: var(--surface-2); overflow: hidden; }
  .meter i { display: block; height: 100%; border-radius: inherit; background: var(--accent); }
  .meter.full i { background: var(--warn); }
  .meter.over i { background: var(--danger); }
  .usage { display: flex; flex-wrap: wrap; gap: 8px 24px; margin: 20px 0 0; padding: 16px 0 0; border-top: 1px solid var(--border); list-style: none; font-size: 0.9rem; color: var(--muted); }

  .grid3 { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 16px; }
  .grid3 button { margin-top: 8px; }
  .reset { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 12px; }
  .reset input { margin: 0; flex: 1 1 220px; }
  .suspend { margin: 0 0 16px; }
  .two { display: grid; gap: 2px; min-width: 0; overflow-wrap: anywhere; }
  .who { display: flex; align-items: center; gap: 12px; min-width: 0; }
  .who.off { opacity: 0.6; }
  .sub { color: var(--muted); font-size: 0.85rem; }
  .ops { display: flex; flex-wrap: wrap; gap: 8px; }
</style>
