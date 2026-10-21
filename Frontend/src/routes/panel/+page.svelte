<script>
  import { onMount } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { formatDay, money, plural } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';
  import { fmtTime, longDate, addDays, todayIn } from '$lib/time.js';

  const me = session.me;
  const tz = me.business.timezone || 'America/Tijuana';
  const role = me.role;
  const sells = ['admin', 'cashier', 'reception'].includes(role);
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
        role === 'admin' ? api(`/api/reports/sales?from=${today}&to=${today}&branch_id=${id}`).catch(() => null) : null,
        role === 'admin' ? api(`/api/reports/sales?from=${addDays(today, -1)}&to=${addDays(today, -1)}&branch_id=${id}`).catch(() => null) : null
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

{#if me.business.status === 'trial' && me.business.trial_ends_at}
  <p class="banner">Prueba gratuita hasta el {formatDay(me.business.trial_ends_at)}.</p>
{/if}

<div class="page-head">
  <div>
    <h1>Hola, {me.name}</h1>
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
  <div class="today">
    <section aria-labelledby="agenda-h">
      <h2 id="agenda-h">{role === 'staff' ? 'Tu día' : 'Agenda de hoy'}</h2>
      {#if sorted.length === 0}
        <div class="empty">
          Hoy no hay citas todavía.
          <div style="margin-top: 0.75rem"><a class="btn ghost" href="/panel/agenda">Agendar una cita</a></div>
        </div>
      {:else}
        <ol class="slots">
          {#each sorted as a (a.id)}
            <li class:next={next?.id === a.id} class:done={a.status === 'completed' || a.status === 'no_show'}>
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

    <aside aria-label="Resumen del día">
      <h2>Hoy en números</h2>
      <dl class="facts">
        <div>
          <dt>Citas</dt>
          <dd>{plural(upcoming.length, 'por atender', 'por atender')} de {sorted.length}</dd>
        </div>
        {#if toCharge}
          <div><dt>Sin cobrar</dt><dd class="clay">{plural(toCharge, 'cita ya terminó', 'citas ya terminaron')}</dd></div>
        {/if}
        {#if waiting}
          <div><dt>Anticipos</dt><dd class="clay">{plural(waiting, 'cita espera su anticipo', 'citas esperan su anticipo')}</dd></div>
        {/if}
        {#if sells}
          <div>
            <dt>Caja</dt>
            <dd>
              {#if cash}Abierta · efectivo esperado <b class="num">{money(cash.expected_cents)}</b>
              {:else}Cerrada · <a href="/panel/ventas">abrir caja</a>{/if}
            </dd>
          </div>
        {/if}
        {#if sales}
          <div>
            <dt>Ventas</dt>
            <dd>
              <b class="total num">{money(sales.today.collected_cents)}</b>
              <span class="meta">{plural(sales.today.sales, 'venta', 'ventas')}{diff ? ` · ${diff}` : ''}</span>
            </dd>
          </div>
        {/if}
      </dl>
    </aside>
  </div>
{/if}

<style>
  .today { display: grid; gap: 28px; grid-template-columns: 1fr; align-items: start; }
  @media (min-width: 1000px) { .today { grid-template-columns: minmax(0, 1fr) 320px; } }
  h2 { font-size: 1.35rem; margin-bottom: 0.75rem; }

  .slots { list-style: none; margin: 0; padding: 0; display: grid; gap: 8px; }
  .slots li {
    display: grid;
    grid-template-columns: 4.2rem 1fr auto;
    align-items: center;
    gap: 6px 14px;
    padding: 0.85rem 1rem;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    animation: in 260ms var(--ease) both;
  }
  .slots li:nth-child(2) { animation-delay: 40ms; }
  .slots li:nth-child(3) { animation-delay: 80ms; }
  .slots li:nth-child(n + 4) { animation-delay: 120ms; }
  :global(:root[data-theme='lino']) .slots { gap: 14px; }
  :global(:root[data-theme='lino']) .slots li { padding: 1.35rem 1.6rem; gap: 6px 22px; }
  :global(:root[data-theme='lino']) aside { padding: 1.75rem 1.75rem 0.75rem; }
  :global(:root[data-theme='lino']) .when { font-size: 1.6rem; }
  .slots li.next { background: var(--marigold-soft); border-color: var(--marigold); }
  .slots li.done { opacity: 0.7; }
  @keyframes in { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: none; } }

  .when { font-family: var(--font-heading); font-size: 1.35rem; font-weight: 600; font-variant-numeric: tabular-nums lining-nums; }
  .what { display: grid; min-width: 0; line-height: 1.35; }
  .what .meta { font-size: 0.88rem; color: var(--muted); }
  .dot { display: inline-block; width: 9px; height: 9px; margin-right: 6px; border-radius: 50%; vertical-align: baseline; }
  .tags { display: flex; flex-wrap: wrap; gap: 6px; }
  .tags:empty { display: none; }
  .tags:not(:empty) { margin-top: 4px; }
  @media (max-width: 560px) {
    .slots li { grid-template-columns: 3.4rem 1fr auto; gap: 6px 10px; padding: 0.8rem; }
  }

  aside { padding: 1.25rem 1.25rem 0.5rem; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  .facts { margin: 0; display: grid; }
  .facts > div { padding: 0.8rem 0; border-top: 1px solid var(--border); }
  .facts > div:first-child { border-top: 0; padding-top: 0; }
  dt { font-size: 0.85rem; color: var(--muted); margin-bottom: 2px; }
  dd { margin: 0; }
  dd .meta { display: block; font-size: 0.88rem; color: var(--muted); }
  .total { font-size: 1.9rem; font-weight: 700; line-height: 1.15; letter-spacing: -0.02em; font-variant-numeric: tabular-nums; }
  .clay { color: var(--clay); font-weight: 600; }
</style>
