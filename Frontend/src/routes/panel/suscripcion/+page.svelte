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

  <h2 class="sec">{needsPlan ? 'Elige tu plan' : 'Compara planes'}</h2>
  {#if !b.online}
    <div class="gap"><InlineNotice tone="warning" message="El pago en línea no está habilitado en este momento. Puedes pagar por transferencia y la plataforma registrará tu pago." /></div>
  {/if}
  <Surface padding="flush">
    <DataTable caption="Comparación de planes" columns={planCols} rows={planRows} cell={planCell} />
  </Surface>
  <p class="meta note">Al pagar, el periodo se suma al que ya tengas vigente. Pagas con Mercado Pago (tarjeta, transferencia o efectivo en tiendas). El año cuesta 10 meses: te ahorras 2.</p>

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
        <button class="btn sm" disabled={!!busy} onclick={() => pay(p.id, 1)}>{busy === `${p.id}-1` ? 'Abriendo…' : `1 mes · ${pesos(p.price_month)}`}</button>
        <button class="btn ghost sm" disabled={!!busy} onclick={() => pay(p.id, 12)}>{busy === `${p.id}-12` ? 'Abriendo…' : `1 año · ${pesos(p.price_year)}`}</button>
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
  .pay { display: grid; gap: 8px; min-width: 150px; }
  .meta { color: var(--muted); font-size: 0.88rem; margin: 0; }
  .note { margin-top: 12px; }
</style>
