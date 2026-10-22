<script>
  import { onMount } from 'svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { BILLING, KINDS, PLANS, formatDay, money } from '$lib/format.js';

  const MODES = { open: 'Abierto', approval: 'Con aprobación', closed: 'Cerrado' };

  let d = $state(null);
  let error = $state('');
  onMount(async () => {
    try {
      d = await api('/api/platform/overview');
    } catch (e) {
      error = e.message;
    }
  });

  const cards = $derived(
    d && [
      ['Negocios', d.counts.total, 'scissors', '/plataforma/negocios'],
      ['Activos', d.counts.active, 'check', '/plataforma/negocios?state=active'],
      ['En prueba', d.counts.trialing, 'calendar', '/plataforma/negocios?state=trialing'],
      ['Pago atrasado', d.counts.past_due + d.counts.trial_expired, 'bell', '/plataforma/negocios?state=past_due'],
      ['Suspendidos', d.counts.suspended, 'percent', '/plataforma/negocios?state=suspended'],
      ['Altas (30 días)', d.signups_30d, 'globe', '/plataforma/negocios']
    ]
  );
  const KIND_TONE = { pending: 'warn', past_due: 'bad', trial_expired: 'bad', trial_ending: 'warn' };
</script>

<div class="page-head">
  <div>
    <h1>Resumen de la plataforma</h1>
    <p>Negocios, ingresos y lo que necesita tu atención.</p>
  </div>
  {#if d}<a class="badge" href="/plataforma/ajustes">Registro: {MODES[d.registration_mode]}</a>{/if}
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if d}
  {#if d.mrr_cents !== undefined}
    <div class="money">
      <div class="card stat big">
        <span class="label">Ingreso mensual recurrente</span>
        <span class="value">{money(d.mrr_cents)}</span>
        <span class="hint">Lo que pagan al mes los negocios activos, según su último pago.</span>
      </div>
      <div class="card stat big">
        <span class="label">Cobrado este mes</span>
        <span class="value">{money(d.paid_this_month_cents)}</span>
        <span class="hint"><a href="/plataforma/pagos">Ver pagos</a></span>
      </div>
    </div>
  {/if}

  <div class="stats">
    {#each cards as [label, value, icon, href]}
      <a class="card stat" {href}>
        <span class="ico"><Icon name={icon} /></span>
        <span class="label">{label}</span>
        <span class="value">{value}</span>
      </a>
    {/each}
  </div>

  {#if d.usage_30d}
    <section aria-label="Uso en los últimos 30 días">
      <h2>Uso en 30 días</h2>
      <div class="stats usage">
        <div class="card stat"><span class="label">Citas creadas</span><span class="value">{d.usage_30d.appointments}</span></div>
        <div class="card stat"><span class="label">Reservas en línea</span><span class="value">{d.usage_30d.online_bookings}</span></div>
        <div class="card stat"><span class="label">Ventas cobradas</span><span class="value">{d.usage_30d.sales}</span></div>
        <div class="card stat"><span class="label">Negocios con actividad</span><span class="value">{d.usage_30d.active_businesses}</span><span class="hint">de {d.total}</span></div>
      </div>
    </section>
  {/if}

  <section>
    <h2>Requieren atención {#if d.attention.length}<span class="badge warn">{d.attention.length}</span>{/if}</h2>
    {#if d.attention.length === 0}
      <p class="empty">Nada pendiente. Todo en orden.</p>
    {:else}
      <div class="list">
        {#each d.attention as x (x.kind + x.business_id)}
          <a class="item-row link" href="/plataforma/negocios/{x.business_id}">
            <div class="grow">
              <div class="title">{x.name}</div>
              <div class="meta">{x.detail}{#if x.at}{' · '}{formatDay(x.at)}{/if}</div>
            </div>
            <span class="badge {KIND_TONE[x.kind]}">{x.title}</span>
          </a>
        {/each}
      </div>
    {/if}
  </section>

  <section>
    <h2>Últimos registros</h2>
    <div class="list">
      {#each d.recent as b (b.id)}
        {@const [label, tone] = BILLING[b.billing_state]}
        <a class="item-row link" href="/plataforma/negocios/{b.id}">
          <div class="grow">
            <div class="title">{b.name}</div>
            <div class="meta">{KINDS[b.kind]} · {PLANS[b.plan]} · {b.owner_email} · {formatDay(b.created_at)}</div>
          </div>
          <span class="badge {tone}">{label}</span>
        </a>
      {/each}
    </div>
  </section>
{/if}

<style>
  /* seis indicadores: 2 columnas en el celular, 3 en pantallas medianas y 6 en pantallas anchas */
  .stats { display: grid; gap: 12px; grid-template-columns: repeat(2, 1fr); }
  @media (min-width: 760px) { .stats { gap: 16px; grid-template-columns: repeat(3, 1fr); } }
  @media (min-width: 1180px) { .stats { grid-template-columns: repeat(6, 1fr); } }
  .money { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); margin-bottom: 16px; }
  .stat { display: grid; gap: 4px; color: var(--text); text-decoration: none; }
  a.stat:hover { border-color: var(--accent); }
  .ico { display: grid; place-items: center; width: 40px; height: 40px; margin-bottom: 8px; border-radius: 10px; background: var(--accent-soft); color: var(--accent); }
  .label { color: var(--muted); font-size: 0.9rem; }
  /* cifras con la misma familia y números alineados que el resto de la plataforma, no con la tipografía de títulos */
  .value { font-family: var(--font-body, inherit); font-size: 2rem; font-weight: 700; line-height: 1.1; font-variant-numeric: tabular-nums lining-nums; }
  .big .value { font-size: 2.5rem; color: var(--accent); }
  @media (max-width: 759px) { .stat.card { padding: 1rem; } .ico { width: 34px; height: 34px; margin-bottom: 4px; } .value { font-size: 1.6rem; } }
  .hint { color: var(--muted); font-size: 0.82rem; }
  section { margin-top: 2.5rem; }
  section h2 { font-size: 1.5rem; display: flex; align-items: center; gap: 10px; }
  .link { color: var(--text); text-decoration: none; }
  .link:hover { border-color: var(--accent); }
  a.badge { text-decoration: none; }
</style>
