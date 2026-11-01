<script>
  import { onDestroy, onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api.js';
  import { BILLING, PAY_METHODS, formatDay, money, pesos, plural } from '$lib/format.js';
  import { loadSession, session } from '$lib/session.svelte.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';

  let b = $state(null);
  let error = $state('');
  let notice = $state('');
  let busy = $state('');
  let payerEmail = $state('');
  let checking = $state(false);
  let timer;

  async function load() {
    b = await api('/api/billing');
  }

  onMount(async () => {
    if (session.me.role !== 'admin') return goto('/panel');
    try {
      await load();
    } catch (e) {
      return (error = e.message);
    }
    const id = page.url.searchParams.get('checkout');
    if (id) follow(id);
    const sid = page.url.searchParams.get('subscription');
    if (sid) followSubscription(sid);
  });
  onDestroy(() => clearTimeout(timer));

  // al volver de Mercado Pago el pago puede tardar unos segundos en confirmarse
  async function follow(id, tries = 0) {
    checking = true;
    try {
      const c = await api(`/api/billing/checkouts/${id}`);
      if (c.status === 'paid') {
        checking = false;
        notice = '¡Listo! Recibimos tu pago y tu suscripción está vigente.';
        await Promise.all([load(), loadSession()]);
        history.replaceState(null, '', '/panel/suscripcion');
        return;
      }
    } catch (e) {
      error = e.message;
      checking = false;
      return;
    }
    if (tries < 40) timer = setTimeout(() => follow(id, tries + 1), 3000);
    else {
      checking = false;
      notice = 'Aún no vemos tu pago. Si ya pagaste, se reflejará en unos minutos; si no, vuelve a intentarlo.';
    }
  }

  // al volver de Mercado Pago la autorización puede tardar unos segundos en reflejarse
  async function followSubscription(id, tries = 0) {
    checking = true;
    try {
      const c = await api(`/api/billing/subscriptions/${id}`);
      if (c.status === 'authorized') {
        checking = false;
        notice = '¡Listo! Autorizaste el cobro automático. Mercado Pago cobrará cada mes y tu suscripción se renovará sola.';
        await Promise.all([load(), loadSession()]);
        history.replaceState(null, '', '/panel/suscripcion');
        return;
      }
      if (c.status === 'cancelled') {
        checking = false;
        notice = 'La suscripción no se completó. Puedes intentarlo de nuevo cuando quieras.';
        await load();
        history.replaceState(null, '', '/panel/suscripcion');
        return;
      }
    } catch (e) {
      error = e.message;
      checking = false;
      return;
    }
    if (tries < 40) timer = setTimeout(() => followSubscription(id, tries + 1), 3000);
    else {
      checking = false;
      notice = 'Aún no vemos la autorización. Si ya la hiciste, se reflejará en unos minutos.';
      await load();
    }
  }

  async function subscribe(plan) {
    error = '';
    busy = `sub-${plan}`;
    try {
      const r = await api('/api/billing/subscribe', { method: 'POST', body: { plan, payer_email: payerEmail.trim() || undefined } });
      location.href = r.pay_url;
    } catch (e) {
      error = e.message;
      busy = '';
    }
  }

  async function cancelSubscription() {
    if (!confirm('¿Cancelar el cobro automático? Conservas tu plan hasta el final del periodo ya pagado, y después tendrás que renovar a mano.')) return;
    error = '';
    busy = 'cancel';
    try {
      await api('/api/billing/subscription/cancel', { method: 'POST', body: {} });
      notice = 'Cancelamos el cobro automático. Tu plan sigue vigente hasta el final del periodo pagado.';
      await load();
    } catch (e) {
      error = e.message;
    }
    busy = '';
  }

  async function pay(plan, months) {
    error = '';
    busy = `${plan}-${months}`;
    try {
      const r = await api('/api/billing/checkout', { method: 'POST', body: { plan, months } });
      location.href = r.pay_url;
    } catch (e) {
      error = e.message;
      busy = '';
    }
  }

  const lim = (n) => (n === null ? 'Sin límite' : n);
  const state = $derived(b ? BILLING[b.billing_state] : ['', '']);
  const TONE = { trialing: 'info', active: 'success', trial_expired: 'danger', past_due: 'danger', suspended: 'danger', pending: 'warning' };
  const yn = (v) => (v ? 'Sí' : 'No');
  const FEATURES = [
    { id: 'staff', label: 'Personas en agenda', get: (p) => lim(p.max_staff) },
    { id: 'branches', label: 'Sucursales', get: (p) => p.max_branches },
    { id: 'pos', label: 'Caja, ventas e inventario', get: (p) => yn(p.pos) },
    { id: 'deposits', label: 'Anticipo por transferencia', get: (p) => yn(p.deposits) },
    { id: 'deposits_mp', label: 'Anticipo con Mercado Pago', get: (p) => yn(p.deposits_mp) },
    { id: 'terminal', label: 'Cobro con terminal', get: (p) => yn(p.terminal) },
    { id: 'support', label: 'Soporte', get: (p) => p.support }
  ];
  const planCols = $derived(
    b ? [{ key: 'f', label: 'Plan' }, ...b.plans.map((p) => ({ key: p.id, label: p.id === b.plan.id ? `${p.name} · tu plan` : p.name }))] : []
  );
  const planRows = $derived([
    { id: 'price', label: 'Precio', get: (p) => `${pesos(p.price_month)} / mes` },
    ...FEATURES,
    ...(b?.online ? [{ id: 'pay', label: needsPlan ? 'Elige tu plan' : 'Cambiar o renovar' }] : [])
  ]);
  const payCols = [
    { key: 'created_at', label: 'Fecha' },
    { key: 'amount', label: 'Importe', numeric: true },
    { key: 'method', label: 'Método' },
    { key: 'plan', label: 'Plan' },
    { key: 'period', label: 'Cubre' }
  ];
  const needsPlan = $derived(b && ['trialing', 'trial_expired', 'past_due'].includes(b.billing_state));
  const sub = $derived(b?.subscription ?? null);
  const subOn = $derived(sub?.status === 'authorized');
  const nextCharge = $derived(b ? (b.billing_state === 'trialing' ? b.trial_ends_at : b.current_period_end) : null);
</script>

<PageHeader title="Suscripción" description="Tu plan, tu vigencia y tus pagos." />

{#if error}<div class="gap"><InlineNotice tone="danger" message={error} /></div>{/if}
{#if notice}<div class="gap"><InlineNotice tone="success" message={notice} /></div>{/if}
{#if checking}<div class="gap"><InlineNotice tone="info" message="Confirmando tu pago con Mercado Pago…" /></div>{/if}

{#if !b}
  {#if !error}<LoadingState />{/if}
{:else}
  {#if b.billing_state === 'trial_expired'}
    <div class="gap"><InlineNotice tone="warning" message="Tu prueba terminó. Elige un plan para seguir usando Barbella." /></div>
  {:else if b.billing_state === 'past_due'}
    <div class="gap"><InlineNotice tone="warning" message="Tu pago está atrasado. Renueva tu plan para evitar la suspensión." /></div>
  {/if}

  <Surface>
    <div class="now">
      <div>
        <StatusBadge label={state[0]} tone={TONE[b.billing_state] ?? 'neutral'} />
        <h2 class="plan-name">Plan {b.plan.name}</h2>
      </div>
      <div class="when">
        {#if b.billing_state === 'trialing'}
          <span class="lbl">Prueba gratis hasta</span><strong>{formatDay(b.trial_ends_at)}</strong>
        {:else if b.billing_state === 'trial_expired'}
          <span class="lbl">Tu prueba terminó el</span><strong>{formatDay(b.trial_ends_at)}</strong>
        {:else if b.current_period_end}
          <span class="lbl">{b.billing_state === 'past_due' ? 'Venció el' : 'Vigente hasta'}</span><strong>{formatDay(b.current_period_end)}</strong>
        {:else}
          <span class="lbl">Vigencia</span><strong>Activo sin vencimiento</strong>
        {/if}
        {#if b.billing_state === 'past_due'}<span class="lbl">Tienes {plural(b.grace_days, 'día', 'días')} de gracia después de la fecha de vencimiento.</span>{/if}
      </div>
    </div>
  </Surface>

  <div class="usage">
    <MetricCard label="Personas en agenda" value="{b.usage.staff} / {lim(b.plan.max_staff)}" hint={b.plan.max_staff !== null && b.usage.staff > b.plan.max_staff ? 'Superas el límite de tu plan' : ''} />
    <MetricCard label="Sucursales" value="{b.usage.branches} / {b.plan.max_branches}" hint={b.usage.branches > b.plan.max_branches ? 'Superas el límite de tu plan' : ''} />
    <MetricCard label="Clientes" value={b.usage.clients} />
  </div>

  {#if sub}
    <div class="gap-top">
      <Surface>
        <div class="now subnow" aria-label="Cobro automático">
          {#if sub.status === 'authorized'}
            <div>
              <StatusBadge label="Cobro automático activo" tone="success" />
              <h2 class="sub-title">{money(sub.amount_cents)} cada mes</h2>
              <p class="meta">Plan {b.plans.find((x) => x.id === sub.plan)?.name ?? sub.plan}. {#if nextCharge}Próximo cobro alrededor del {formatDay(nextCharge)}. {/if}Mercado Pago cobra solo y tu suscripción se renueva.</p>
            </div>
          {:else if sub.status === 'paused'}
            <div>
              <StatusBadge label="Pausada" tone="warning" />
              <h2 class="sub-title">Mercado Pago pausó el cobro</h2>
              <p class="meta">Suele pasar cuando la tarjeta fue rechazada. Entra a Mercado Pago para reactivarla o vuelve a suscribirte con otra tarjeta.</p>
            </div>
          {:else}
            <div>
              <StatusBadge label="Pendiente" tone="warning" />
              <h2 class="sub-title">Falta autorizar el cobro</h2>
              <p class="meta">Empezaste una suscripción pero no la terminaste en Mercado Pago. Elige el plan otra vez para continuar.</p>
            </div>
          {/if}
          <button class="btn ghost" disabled={!!busy} onclick={cancelSubscription}>{busy === 'cancel' ? 'Cancelando…' : 'Cancelar cobro automático'}</button>
        </div>
      </Surface>
    </div>
  {/if}

  <h2 class="sec">{needsPlan ? 'Elige tu plan' : 'Compara planes'}</h2>
  {#if !b.online}
    <div class="gap"><InlineNotice tone="warning" message="El pago en línea no está habilitado en este momento. Puedes pagar por transferencia y la plataforma registrará tu pago." /></div>
  {/if}
  {#if b.online}
    <label class="field payer">
      Correo de tu cuenta de Mercado Pago (opcional)
      <input type="email" bind:value={payerEmail} placeholder="el mismo con el que entras a Mercado Pago" autocomplete="email" />
      <span class="hint">Si lo dejas vacío usamos el de tu usuario. Para suscribirte debes entrar a Mercado Pago con este correo.</span>
    </label>
  {/if}
  <Surface padding="flush">
    <DataTable caption="Comparación de planes" columns={planCols} rows={planRows} cell={planCell} />
  </Surface>
  <p class="meta note">{#if b.online}Cobro automático cada mes con tu tarjeta; cancelas cuando quieras. {/if}Con la suscripción, el primer cobro es al terminar tu prueba o tu periodo vigente, así no pagas dos veces. Al pagar un mes o un año suelto, el periodo se suma al que ya tengas vigente (tarjeta, transferencia o efectivo en tiendas, por Mercado Pago). El año cuesta 10 meses: te ahorras 2.</p>

  <h2 class="sec">Historial de pagos</h2>
  {#if b.payments.length}
    <Surface padding="flush">
      <DataTable caption="Historial de pagos" columns={payCols} rows={b.payments} cell={payCell} />
    </Surface>
  {:else}
    <div class="empty">Aún no hay pagos.</div>
  {/if}
{/if}

{#snippet planCell({ row, column })}
  {#if column.key === 'f'}
    <b class="fl">{row.label}</b>
  {:else}
    {@const p = b.plans.find((x) => x.id === column.key)}
    {#if row.id === 'pay'}
      <div class="pay">
        {#if subOn && sub.plan === p.id}
          <span class="meta"><b>Tu suscripción mensual está activa.</b></span>
        {:else}
          <button class="btn sm" disabled={!!busy} onclick={() => subscribe(p.id)}>{busy === `sub-${p.id}` ? 'Abriendo…' : subOn ? `Cambiar a este plan · ${pesos(p.price_month)}/mes` : `Suscribirme · ${pesos(p.price_month)}/mes`}</button>
        {/if}
        <button class="btn ghost sm" disabled={!!busy} onclick={() => pay(p.id, 1)}>{busy === `${p.id}-1` ? 'Abriendo…' : `Pagar solo 1 mes · ${pesos(p.price_month)}`}</button>
        <button class="btn ghost sm" disabled={!!busy} onclick={() => pay(p.id, 12)}>{busy === `${p.id}-12` ? 'Abriendo…' : `Pagar 1 año · ${pesos(p.price_year)}`}</button>
      </div>
    {:else}
      <span class:price={row.id === 'price'}>{row.get(p)}</span>
    {/if}
  {/if}
{/snippet}

{#snippet payCell({ row, column })}
  {#if column.key === 'created_at'}{formatDay(row.created_at)}
  {:else if column.key === 'amount'}{money(row.amount_cents)}
  {:else if column.key === 'method'}{PAY_METHODS[row.method]}
  {:else if column.key === 'plan'}{row.plan}
  {:else}{plural(row.months, 'mes', 'meses')} · hasta {formatDay(row.period_end)}{/if}
{/snippet}

<style>
  .gap { margin-bottom: 16px; }
  .now { display: flex; flex-wrap: wrap; gap: 16px 40px; justify-content: space-between; align-items: flex-end; }
  .plan-name { margin: 12px 0 0; font-size: 2.25rem; line-height: 1.1; letter-spacing: -0.02em; }
  .when { display: grid; gap: 4px; }
  .when strong { font-size: 2rem; line-height: 1.1; font-weight: 700; letter-spacing: -0.02em; font-variant-numeric: tabular-nums lining-nums; }
  .lbl { color: var(--muted); font-size: 0.9rem; }
  .usage { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); margin-top: 16px; }
  .sec { font-size: 1.25rem; margin: 2.25rem 0 1rem; }
  .fl { font-weight: 500; }
  .price { font-weight: 700; font-size: 1.05rem; font-variant-numeric: tabular-nums lining-nums; white-space: nowrap; }
  .pay { display: grid; gap: 8px; min-width: 190px; }
  .gap-top { margin-top: 16px; }
  .sub-title { margin: 12px 0 4px; font-size: 1.5rem; line-height: 1.15; letter-spacing: -0.01em; }
  .subnow { align-items: center; }
  .subnow p { max-width: 60ch; }
  .payer { max-width: 480px; }
  .meta { color: var(--muted); font-size: 0.88rem; margin: 0; }
  .note { margin-top: 12px; }
</style>
