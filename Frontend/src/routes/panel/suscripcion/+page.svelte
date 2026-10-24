<script>
  import { onDestroy, onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api.js';
  import { BILLING, PAY_METHODS, formatDay, money, pesos, plural } from '$lib/format.js';
  import { loadSession, session } from '$lib/session.svelte.js';

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
      const r = await api('/api/billing/subscribe', { method: 'POST', body: { plan } });
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
  const needsPlan = $derived(b && ['trialing', 'trial_expired', 'past_due'].includes(b.billing_state));
  const sub = $derived(b?.subscription ?? null);
  const subOn = $derived(sub?.status === 'authorized');
  const nextCharge = $derived(b ? (b.billing_state === 'trialing' ? b.trial_ends_at : b.current_period_end) : null);
</script>

<div class="page-head">
  <div>
    <h1>Suscripción</h1>
    <p>Tu plan, tu vigencia y tus pagos.</p>
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}
{#if notice}<p class="ok" role="status">{notice}</p>{/if}
{#if checking}<p class="banner" role="status">Confirmando tu pago con Mercado Pago…</p>{/if}

{#if b}
  <section class="card now">
    <div>
      <span class="badge {state[1]}">{state[0]}</span>
      <h2>Plan {b.plan.name}</h2>
      <p class="meta">
        {#if b.billing_state === 'trialing'}Prueba gratis hasta el {formatDay(b.trial_ends_at)}.
        {:else if b.billing_state === 'trial_expired'}Tu prueba terminó el {formatDay(b.trial_ends_at)}.
        {:else if b.current_period_end}{b.billing_state === 'past_due' ? 'Venció' : 'Vigente hasta'} el {formatDay(b.current_period_end)}.
        {:else}Activo sin vencimiento.{/if}
        {#if b.billing_state === 'past_due'} Tienes {plural(b.grace_days, 'día', 'días')} de gracia después de la fecha de vencimiento.{/if}
      </p>
    </div>
    <div class="usage">
      <span class:over={b.plan.max_staff !== null && b.usage.staff > b.plan.max_staff}>Personas en agenda: <b>{b.usage.staff}</b> / {lim(b.plan.max_staff)}</span>
      <span class:over={b.usage.branches > b.plan.max_branches}>Sucursales: <b>{b.usage.branches}</b> / {b.plan.max_branches}</span>
      <span>Clientes: <b>{b.usage.clients}</b></span>
    </div>
  </section>

  {#if sub}
    <section class="card subbox" aria-label="Cobro automático">
      {#if sub.status === 'authorized'}
        <div>
          <span class="badge ok">Cobro automático activo</span>
          <h2>{money(sub.amount_cents)} cada mes</h2>
          <p class="meta">Plan {b.plans.find((x) => x.id === sub.plan)?.name ?? sub.plan}. {#if nextCharge}Próximo cobro alrededor del {formatDay(nextCharge)}. {/if}Mercado Pago cobra solo y tu suscripción se renueva.</p>
        </div>
      {:else if sub.status === 'paused'}
        <div>
          <span class="badge">Pausada</span>
          <h2>Mercado Pago pausó el cobro</h2>
          <p class="meta">Suele pasar cuando la tarjeta fue rechazada. Entra a Mercado Pago para reactivarla o vuelve a suscribirte con otra tarjeta.</p>
        </div>
      {:else}
        <div>
          <span class="badge">Pendiente</span>
          <h2>Falta autorizar el cobro</h2>
          <p class="meta">Empezaste una suscripción pero no la terminaste en Mercado Pago. Elige el plan otra vez para continuar.</p>
        </div>
      {/if}
      <button class="btn ghost" disabled={!!busy} onclick={cancelSubscription}>{busy === 'cancel' ? 'Cancelando…' : 'Cancelar cobro automático'}</button>
    </section>
  {/if}

  <h2 class="sec">{needsPlan ? 'Elige tu plan' : 'Cambiar o renovar'}</h2>
  {#if !b.online}
    <p class="banner">El pago en línea no está habilitado en este momento. Puedes pagar por transferencia y la plataforma registrará tu pago.</p>
  {/if}
  <div class="plans">
    {#each b.plans as p (p.id)}
      <div class="card plan" class:current={p.id === b.plan.id}>
        {#if p.id === b.plan.id}<span class="badge">Tu plan</span>{/if}
        <h3>{p.name}</h3>
        <p class="meta">{p.description}</p>
        <p class="price">{pesos(p.price_month)}<small>/mes</small></p>
        <ul>
          <li><b>{lim(p.max_staff)}</b> {p.max_staff === 1 ? 'persona' : 'personas'} en agenda</li>
          <li><b>{p.max_branches}</b> {p.max_branches === 1 ? 'sucursal' : 'sucursales'}</li>
          <li>Caja, ventas e inventario: {p.pos ? 'sí' : 'no'}</li>
          <li>Anticipo por transferencia: {p.deposits ? 'sí' : 'no'}</li>
          <li>Anticipo con Mercado Pago: {p.deposits_mp ? 'sí' : 'no'}</li>
          <li>Cobro con terminal: {p.terminal ? 'sí' : 'no'}</li>
          <li>Soporte {p.support}</li>
        </ul>
        {#if b.online}
          {#if subOn && sub.plan === p.id}
            <p class="meta"><b>Tu suscripción mensual está activa.</b></p>
          {:else}
            <button class="btn block" disabled={!!busy} onclick={() => subscribe(p.id)}>{busy === `sub-${p.id}` ? 'Abriendo…' : subOn ? `Cambiar a este plan · ${pesos(p.price_month)}/mes` : `Suscribirme · ${pesos(p.price_month)}/mes`}</button>
            <p class="meta">Cobro automático cada mes con tu tarjeta. Cancelas cuando quieras.</p>
          {/if}
          <button class="btn ghost block" disabled={!!busy} onclick={() => pay(p.id, 1)}>{busy === `${p.id}-1` ? 'Abriendo…' : `Pagar solo 1 mes · ${pesos(p.price_month)}`}</button>
          <button class="btn ghost block" disabled={!!busy} onclick={() => pay(p.id, 12)}>{busy === `${p.id}-12` ? 'Abriendo…' : `Pagar 1 año · ${pesos(p.price_year)}`}</button>
          <p class="meta">El año cuesta 10 meses: te ahorras 2.</p>
        {/if}
      </div>
    {/each}
  </div>
  <p class="meta">Con la suscripción, el primer cobro es al terminar tu prueba o tu periodo vigente, así no pagas dos veces. Al pagar un mes o un año suelto, el periodo se suma al que ya tengas vigente (tarjeta, transferencia o efectivo en tiendas, por Mercado Pago).</p>

  <h2 class="sec">Historial de pagos</h2>
  <div class="list">
    {#each b.payments as p (p.id)}
      <div class="item-row">
        <div class="grow">
          <div class="title">{money(p.amount_cents)} · {PAY_METHODS[p.method]}</div>
          <div class="meta">Plan {p.plan} · {plural(p.months, 'mes', 'meses')} · hasta {formatDay(p.period_end)}</div>
        </div>
        <span class="meta">{formatDay(p.created_at)}</span>
      </div>
    {:else}
      <div class="empty">Aún no hay pagos.</div>
    {/each}
  </div>
{/if}

<style>
  .now { display: flex; flex-wrap: wrap; gap: 20px; justify-content: space-between; align-items: center; }
  .now h2 { margin: 0.5rem 0 0.2rem; font-size: 1.6rem; }
  .subbox { display: flex; flex-wrap: wrap; gap: 16px; justify-content: space-between; align-items: center; margin-top: 1rem; }
  .subbox h2 { margin: 0.5rem 0 0.2rem; font-size: 1.4rem; }
  .usage { display: grid; gap: 4px; font-size: 0.92rem; color: var(--muted); }
  .usage .over { color: var(--danger); font-weight: 600; }
  .sec { font-size: 1.4rem; margin: 2.25rem 0 1rem; }
  .plans { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); margin-bottom: 1rem; }
  .plan { display: grid; gap: 8px; align-content: start; }
  .plan.current { border-color: var(--accent); }
  .plan h3 { margin: 0.4rem 0 0; font-size: 1.4rem; }
  .price { font-family: var(--font-body, inherit); font-weight: 700; font-variant-numeric: tabular-nums lining-nums; font-size: 2rem; margin: 0.25rem 0 0; }
  .price small { font-size: 1rem; font-family: var(--font-body); color: var(--muted); }
  ul { padding-left: 1.1rem; margin: 0.5rem 0 1rem; font-size: 0.92rem; }
  .meta { color: var(--muted); font-size: 0.88rem; margin: 0; }
  .ok { color: var(--accent); padding: 0.7rem 0.9rem; border: 1px solid currentColor; border-radius: var(--radius-sm); margin: 0 0 1rem; }
</style>
