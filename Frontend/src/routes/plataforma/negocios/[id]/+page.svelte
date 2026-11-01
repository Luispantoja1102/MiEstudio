<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { BILLING, KINDS, PAY_METHODS, PLANS, ROLES, ACTIONS, formatDate, formatDay, money, plural, pesos, toCents } from '$lib/format.js';
  import { canWritePlatform } from '$lib/session.svelte.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
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
  let trialDate = $state('');
  let periodDate = $state('');
  let plan = $state('');
  let reason = $state('');
  let suspending = $state(false);
  let resetFor = $state(null);
  let newPassword = $state('');
  let plans = $state([]);
  let pay = $state({ amount: '', months: 1, plan: '', method: 'transfer', note: '' });

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
    {#snippet actions()}<StatusBadge {label} tone={TONES[b.billing_state] ?? 'neutral'} />{/snippet}
  </PageHeader>

  <div class="metrics" role="group" aria-label="Resumen del negocio">
    <MetricCard label="Personas" value={b.users} hint="Responsable: {b.owner_email || '—'}" />
    <MetricCard label="Clientes" value={b.clients} />
    {#if data.metrics}
      <MetricCard label="Citas (30 días)" value={data.metrics.appointments_30d} hint="Última cita creada: {data.metrics.last_appointment_at ? formatDay(data.metrics.last_appointment_at) : 'Nunca'}" />
      <MetricCard label="Reservas en línea (30 días)" value={data.metrics.online_bookings_30d} />
      <MetricCard label="Ventas (30 días)" value={data.metrics.sales_30d} />
    {/if}
  </div>

  <div class="stack">
    <Surface title="Suscripción">
      <p class="lead">
        Plan <b>{PLANS[b.plan]}</b> ·
        {#if b.status === 'trial'}prueba hasta {formatDay(b.trial_ends_at)}{#if b.trial_days_left !== null}{' '}({plural(b.trial_days_left, 'día', 'días')}){/if}
        {:else if b.current_period_end}vigente hasta {formatDay(b.current_period_end)}
        {:else if b.status === 'active'}activo sin vencimiento (cortesía)
        {:else}sin periodo pagado{/if}
        {#if b.last_payment_at} · último pago {formatDay(b.last_payment_at)}{/if}
      </p>
      {#if b.suspended_reason}<div class="gap-top"><InlineNotice tone="warning" message="Motivo de la suspensión: {b.suspended_reason}" /></div>{/if}

      {#if data.usage && data.limits}
        <ul class="usage" aria-label="Uso y límites del plan">
          <li class:over={data.limits.max_staff !== null && data.usage.staff > data.limits.max_staff}>Agenda: {data.usage.staff} / {limitOf(data.limits.max_staff)}</li>
          <li class:over={data.usage.branches > data.limits.max_branches}>Sucursales: {data.usage.branches} / {data.limits.max_branches}</li>
          <li>Cobros: {data.limits.pos ? 'sí' : 'no'}</li>
          <li>Anticipos: {data.limits.deposits_mp ? 'transferencia y Mercado Pago' : data.limits.deposits ? 'solo transferencia' : 'no'}</li>
          <li>Terminal: {data.limits.terminal ? 'sí' : 'no'}</li>
        </ul>
      {/if}

      {#if write}
        <div class="form-actions">
          {#if b.status === 'pending'}<button class="btn" onclick={() => patch({ status: 'trial' }, 'Negocio aprobado: inició su prueba de 14 días.')}>Aprobar</button>{/if}
          {#if b.status === 'suspended'}
            <button class="btn" onclick={() => patch({ status: b.last_payment_at || b.current_period_end ? 'active' : 'trial' }, 'Negocio reactivado.')}>Reactivar</button>
          {:else if !suspending}
            <button class="btn ghost" onclick={() => (suspending = true)}>Suspender…</button>
          {/if}
        </div>
        {#if suspending}
          <form class="reset" onsubmit={suspend}>
            <input bind:value={reason} maxlength="300" placeholder="Motivo (opcional, lo verá el negocio)" aria-label="Motivo de la suspensión" />
            <button class="btn sm">Suspender</button>
            <button type="button" class="btn ghost sm" onclick={() => (suspending = false)}>Cancelar</button>
          </form>
        {/if}
      {/if}
    </Surface>

    {#if write}
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

    <Surface title="Equipo del negocio" padding="flush">
      <DataTable caption="Equipo del negocio" columns={teamColumns} rows={data.team}>
        {#snippet cell({ row: u, column })}
          {#if column.key === 'name'}
            <span class="two" class:off={!u.active}><b>{u.name}</b><span class="sub">{u.email}</span></span>
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
  </div>
{/if}

<style>
  .back { display: inline-flex; align-items: center; gap: 6px; min-height: 44px; margin-bottom: 0.5rem; color: var(--muted); text-decoration: none; font-size: 0.9rem; }
  .back :global(svg) { transform: rotate(180deg); }
  .back:hover { color: var(--accent); }
  .gap { margin-bottom: 16px; }
  .gap-top { margin-top: 16px; }
  .metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 16px; margin-bottom: 24px; }
  .stack { display: grid; gap: 24px; }
  .lead { margin: 0; line-height: 1.5; }
  .grid3 { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 16px; }
  .grid3 button { margin-top: 8px; }
  .usage { display: flex; flex-wrap: wrap; gap: 8px 24px; margin: 16px 0 0; padding: 16px 0 0; border-top: 1px solid var(--border); list-style: none; font-size: 0.9rem; color: var(--muted); }
  .usage .over { color: var(--danger); font-weight: 600; }
  .form-actions { margin-top: 16px; }
  .reset { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 12px; }
  .reset input { margin: 0; flex: 1 1 220px; }
  .two { display: grid; gap: 2px; min-width: 0; overflow-wrap: anywhere; }
  .two.off { opacity: 0.6; }
  .sub { color: var(--muted); font-size: 0.85rem; }
  .ops { display: flex; flex-wrap: wrap; gap: 8px; }
</style>
