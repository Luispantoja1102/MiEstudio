<script>
  import { onMount } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { formatDay, money, plural } from '$lib/format.js';
  import { hasPos, session } from '$lib/session.svelte.js';
  import { fmtTime, longDate, addDays, todayIn } from '$lib/time.js';

  const me = session.me;
  const tz = me.business.timezone || 'America/Tijuana';
  const role = me.role;
  const sells = ['admin', 'cashier', 'reception'].includes(role) && hasPos();
  const today = todayIn(tz);

  let loading = $state(true);
  let error = $state('');
  let appts = $state([]);
  let staff = $state([]);
  let cash = $state(undefined); // undefined: aún no se sabe; null: caja cerrada
  let sales = $state(null); // { today, yesterday } solo administrador
  let now = $state(Date.now());

  async function load() {
    loading = true;
    error = '';
    const id = branches.id;
    if (!id) return (loading = false);
    try {
      const [ag, c, st, sy] = await Promise.all([
        api('/api/agenda?' + new URLSearchParams({ branch_id: id, date: today })),
        sells ? api(`/api/cash/current?branch_id=${id}`).catch(() => null) : null,
        role === 'admin' && hasPos() ? api(`/api/reports/sales?from=${today}&to=${today}&branch_id=${id}`).catch(() => null) : null,
        role === 'admin' && hasPos() ? api(`/api/reports/sales?from=${addDays(today, -1)}&to=${addDays(today, -1)}&branch_id=${id}`).catch(() => null) : null
      ]);
      staff = ag.staff;
      appts = ag.appointments.filter((a) => a.status !== 'cancelled' && (role !== 'staff' || a.staff_id === me.id));
      cash = sells ? (c?.session ?? null) : undefined;
      sales = st && sy ? { today: st, yesterday: sy } : null;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(async () => {
    const tick = setInterval(() => (now = Date.now()), 60000);
    try {
      await loadBranches();
      await load();
    } catch (e) {
      error = e.message;
      loading = false;
    }
    return () => clearInterval(tick);
  });

  const sorted = $derived([...appts].sort((a, b) => a.starts_at.localeCompare(b.starts_at)));
  const isOpen = (a) => a.status === 'scheduled' || a.status === 'pending_payment';
  const upcoming = $derived(sorted.filter((a) => isOpen(a) && new Date(a.ends_at).getTime() > now));
  const next = $derived(upcoming[0]);
  const late = (a) => a.status === 'scheduled' && new Date(a.ends_at).getTime() <= now; // ya pasó y no se cobró
  const waiting = $derived(sorted.filter((a) => a.status === 'pending_payment').length);
  const toCharge = $derived(sorted.filter(late).length);
  const colorOf = (id) => `var(--p${(Math.max(0, staff.findIndex((s) => s.id === id)) % 6) + 1})`;

  const diff = $derived.by(() => {
    if (!sales) return null;
    const t = sales.today.collected_cents, y = sales.yesterday.collected_cents;
    if (!y) return t ? 'Ayer no hubo ventas' : null;
    const pct = Math.round(((t - y) / y) * 100);
    return pct === 0 ? 'Igual que ayer' : `${pct > 0 ? '+' : ''}${pct} % contra ayer`;
  });
</script>

<div class="page-head hero-head">
  <div>
    <h1>Hola, <span class="who">{me.name}</span></h1>
    <p>{longDate(today)}</p>
  </div>
  <div class="form-actions" style="margin: 0">
    <BranchPicker onchange={load} />
    <a class="btn ghost" href="/panel/agenda"><Icon name="calendar" size={18} />Nueva cita</a>
    {#if sells}<a class="btn" href="/panel/ventas"><Icon name="receipt" size={18} />Cobrar</a>{/if}
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else if !branches.id}
  <div class="empty">No hay sucursales activas. Crea una en Ajustes para empezar a agendar.</div>
{:else}
  <dl class="kpis" aria-label="Resumen del día">
    <div class="kpi">
      <dt>Citas por atender</dt>
      <dd><b class="v">{upcoming.length}</b><span class="of">de {sorted.length}</span></dd>
    </div>
    {#if sales}
      <div class="kpi">
        <dt>Ventas de hoy</dt>
        <dd><b class="v">{money(sales.today.collected_cents)}</b><span class="sub">{plural(sales.today.sales, 'venta', 'ventas')}{diff ? ` · ${diff}` : ''}</span></dd>
      </div>
    {/if}
    {#if sells}
      <div class="kpi">
        <dt>Caja</dt>
        <dd>
          {#if cash}<b class="v word">Abierta</b><span class="sub">efectivo esperado <b class="num">{money(cash.expected_cents)}</b></span>
          {:else}<b class="v word">Cerrada</b><span class="sub"><a href="/panel/ventas">abrir caja</a></span>{/if}
        </dd>
      </div>
    {/if}
    {#if toCharge}
      <div class="kpi alert">
        <dt>Sin cobrar</dt>
        <dd><b class="v">{toCharge}</b><span class="sub">{plural(toCharge, 'cita ya terminó', 'citas ya terminaron')}</span></dd>
      </div>
    {/if}
    {#if waiting}
      <div class="kpi alert">
        <dt>Anticipos</dt>
        <dd><b class="v">{waiting}</b><span class="sub">{plural(waiting, 'cita espera su anticipo', 'citas esperan su anticipo')}</span></dd>
      </div>
    {/if}
  </dl>

  {#if me.business.status === 'trial' && me.business.trial_ends_at}
    <p class="banner">Prueba gratuita hasta el {formatDay(me.business.trial_ends_at)}.</p>
  {/if}

  <section class="card agenda" aria-labelledby="agenda-h">
    <h2 id="agenda-h">{role === 'staff' ? 'Tu día' : 'Agenda de hoy'}</h2>
    {#if sorted.length === 0}
      <div class="empty">
        Hoy no hay citas todavía.
        <div style="margin-top: 0.75rem"><a class="btn ghost" href="/panel/agenda">Agendar una cita</a></div>
      </div>
    {:else}
      <ol class="slots">
        {#each sorted as a (a.id)}
          <li class:next={next?.id === a.id} class:done={a.status === 'completed' || a.status === 'no_show'} style:--c={colorOf(a.staff_id)}>
            <time class="when" datetime={a.starts_at}>{fmtTime(a.starts_at, tz)}</time>
            <div class="what">
              <b>{a.client_name}</b>
              <span class="meta">{a.service_name} · <i class="dot" style:background={colorOf(a.staff_id)}></i>{a.staff_name}</span>
              <span class="tags">
                {#if next?.id === a.id}<span class="badge gold">Siguiente</span>{/if}
                {#if a.status === 'pending_payment'}<span class="badge clay">Espera anticipo</span>
                {:else if a.status === 'completed'}<span class="badge muted">Cobrada</span>
                {:else if a.status === 'no_show'}<span class="badge muted">No asistió</span>
                {:else if late(a)}<span class="badge clay">Por cobrar</span>
                {:else if a.deposit_status === 'paid'}<span class="badge">Anticipo pagado</span>{/if}
              </span>
            </div>
            {#if sells && a.status === 'scheduled'}
              <a class="btn sm" class:ghost={next?.id !== a.id && !late(a)} href={`/panel/ventas?cita=${a.id}`}>Cobrar</a>
            {/if}
          </li>
        {/each}
      </ol>
    {/if}
  </section>
{/if}

<style>
  h2 { font-size: 1.35rem; margin-bottom: 0.9rem; }
  .agenda { padding: 1.5rem; }

  /* cifras del día */
  .kpis { margin: 0 0 1.25rem; display: grid; gap: 14px; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); }
  .kpi { padding: 1.1rem 1.3rem; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); box-shadow: var(--card-shadow); }
  .kpi dt { font-size: 0.85rem; color: var(--muted); margin-bottom: 0.35rem; }
  .kpi dd { margin: 0; display: grid; gap: 2px; }
  .kpi .v { font-family: var(--font-num); font-stretch: var(--num-stretch); font-size: 2.2rem; font-weight: 800; line-height: 1.05; letter-spacing: -0.01em; font-variant-numeric: tabular-nums; }
  .kpi .v.word { font-size: 1.7rem; }
  .kpi .of { font-size: 0.85rem; color: var(--muted); }
  .kpi .sub { font-size: 0.85rem; color: var(--muted); }
  .kpi.alert .v { color: var(--clay); }

  .slots { list-style: none; margin: 0; padding: 0; display: grid; gap: 10px; }
  .slots li {
    display: grid;
    grid-template-columns: 4.2rem 1fr auto;
    align-items: center;
    gap: 6px 22px;
    padding: 1.1rem 1.4rem;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    animation: in 260ms var(--ease) both;
  }
  .slots li:nth-child(2) { animation-delay: 40ms; }
  .slots li:nth-child(3) { animation-delay: 80ms; }
  .slots li:nth-child(n + 4) { animation-delay: 120ms; }
  .slots li.next { background: var(--marigold-soft); border-color: var(--marigold); }
  .slots li.done { opacity: 0.7; }
  @keyframes in { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: none; } }

  .when { font-family: var(--font-num); font-stretch: var(--num-stretch); font-size: 1.15rem; font-weight: 700; font-variant-numeric: tabular-nums lining-nums; }
  .what { display: grid; min-width: 0; line-height: 1.35; }
  .what .meta { font-size: 0.88rem; color: var(--muted); }
  .dot { display: inline-block; width: 9px; height: 9px; margin-right: 6px; border-radius: 50%; vertical-align: baseline; }
  .tags { display: flex; flex-wrap: wrap; gap: 6px; }
  .tags:empty { display: none; }
  .tags:not(:empty) { margin-top: 4px; }
  @media (max-width: 560px) {
    .agenda { padding: 1rem; }
    .slots li { grid-template-columns: 3.4rem 1fr auto; gap: 6px 10px; padding: 0.8rem; }
    .kpis { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
    .kpi { padding: 0.85rem 0.9rem; }
    .kpi dt { font-size: 0.8rem; }
    .kpi .v { font-size: 1.6rem; }
    .kpi .v.word { font-size: 1.25rem; }
    .kpi .sub, .kpi .of { font-size: 0.78rem; }
  }

  /* Día y Noche: saludo sobre la franja oscura de la barra (ver Shell), franja de color por profesional y horas grandes */
  :global(:root[data-theme='dark']) .hero-head { --text: #ece9e2; --muted: #b3b0a8; --border: rgba(236, 233, 226, 0.22); --surface-2: rgba(236, 233, 226, 0.1); color: var(--text); }
  :global(:root[data-theme='light']) .who { color: var(--accent); }
  :global(:root[data-theme='dark']) .who { color: #ee8a7c; }
  :global(:root[data-theme='light']) .slots li,
  :global(:root[data-theme='dark']) .slots li { position: relative; overflow: hidden; grid-template-columns: 5rem 1fr auto; padding-left: 1.7rem; }
  :global(:root[data-theme='light']) .slots li::before,
  :global(:root[data-theme='dark']) .slots li::before { content: ''; position: absolute; inset: 0 auto 0 0; width: 4px; background: var(--c); }
  :global(:root[data-theme='light']) .when,
  :global(:root[data-theme='dark']) .when { font-size: 1.7rem; line-height: 1; }
  @media (max-width: 560px) {
    :global(:root[data-theme='light']) .slots li,
    :global(:root[data-theme='dark']) .slots li { grid-template-columns: 4rem 1fr auto; padding-left: 1.3rem; }
    :global(:root[data-theme='light']) .when,
    :global(:root[data-theme='dark']) .when { font-size: 1.45rem; }
  }
</style>
