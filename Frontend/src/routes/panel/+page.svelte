<script>
  import { onMount } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import Icon from '$lib/Icon.svelte';
  import AppointmentSummary from '$lib/ui/AppointmentSummary.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import BarChart from '$lib/ui/BarChart.svelte';
  import Surface from '$lib/ui/Surface.svelte';
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
  let week = $state(null); // ventas por día, últimos 7 días (solo quien ve «Ventas de hoy»)
  let now = $state(Date.now());

  async function load() {
    loading = true;
    error = '';
    const id = branches.id;
    if (!id) return (loading = false);
    try {
      const [ag, c, st, sy, wk] = await Promise.all([
        api('/api/agenda?' + new URLSearchParams({ branch_id: id, date: today })),
        sells ? api(`/api/cash/current?branch_id=${id}`).catch(() => null) : null,
        role === 'admin' && hasPos() ? api(`/api/reports/sales?from=${today}&to=${today}&branch_id=${id}`).catch(() => null) : null,
        role === 'admin' && hasPos() ? api(`/api/reports/sales?from=${addDays(today, -1)}&to=${addDays(today, -1)}&branch_id=${id}`).catch(() => null) : null,
        role === 'admin' && hasPos() ? api(`/api/reports/sales?from=${addDays(today, -6)}&to=${today}&branch_id=${id}`).catch(() => null) : null
      ]);
      staff = ag.staff;
      appts = ag.appointments.filter((a) => a.status !== 'cancelled' && (role !== 'staff' || a.staff_id === me.id));
      cash = sells ? (c?.session ?? null) : undefined;
      week = wk?.by_day ?? null;
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
  const chart = $derived.by(() => {
    if (!week || !week.some((d) => d.total_cents > 0)) return [];
    const byDate = Object.fromEntries(week.map((d) => [d.date, d.total_cents]));
    return Array.from({ length: 7 }, (_, i) => {
      const date = addDays(today, i - 6), v = byDate[date] ?? 0;
      return { id: date, label: 'DLMMJVS'[new Date(date + 'T12:00:00').getDay()], value: v, formattedValue: '$' + Math.round(v / 100).toLocaleString('es-MX') };
    });
  });
  const pendingAppts = $derived(sorted.filter((a) => a.status === 'pending_payment'));
  const lateAppts = $derived(sorted.filter(late));
  // estado de la fila: [texto, tono]; sin estado propio, la siguiente cita se rotula «Siguiente»
  const statusOf = (a) =>
    a.status === 'pending_payment' ? ['Espera anticipo', 'warning']
    : a.status === 'completed' ? ['Cobrada', 'neutral']
    : a.status === 'no_show' ? ['No asistió', 'neutral']
    : late(a) ? ['Por cobrar', 'warning']
    : a.deposit_status === 'paid' ? ['Anticipo pagado', 'success']
    : next?.id === a.id ? ['Siguiente', 'accent']
    : ['', 'neutral'];

  const diff = $derived.by(() => {
    if (!sales) return null;
    const t = sales.today.collected_cents, y = sales.yesterday.collected_cents;
    if (!y) return t ? 'Ayer no hubo ventas' : null;
    const pct = Math.round(((t - y) / y) * 100);
    return pct === 0 ? 'Igual que ayer' : `${pct > 0 ? '+' : ''}${pct} % contra ayer`;
  });
</script>

<PageHeader title={`Hola, ${me.name}`} description={longDate(today)}>
  {#snippet actions()}
    <BranchPicker onchange={load} />
    <a class="btn ghost" href="/panel/agenda"><Icon name="calendar" size={18} />Nueva cita</a>
    {#if sells}<a class="btn" href="/panel/ventas"><Icon name="receipt" size={18} />Cobrar</a>{/if}
  {/snippet}
</PageHeader>

{#if error}<InlineNotice tone="danger" message={error} />{/if}

{#if loading}
  <LoadingState />
{:else if !branches.id}
  <EmptyState title="No hay sucursales activas" description="Crea una en Ajustes para empezar a agendar." />
{:else}
  <div class="metrics">
    <MetricCard label="Citas por atender" value={upcoming.length} hint={`de ${plural(sorted.length, 'cita', 'citas')} hoy`} />
    {#if sales}
      <MetricCard label="Ventas de hoy" value={money(sales.today.collected_cents)} hint={`${plural(sales.today.sales, 'venta', 'ventas')}${diff ? ` · ${diff}` : ''}`} />
    {/if}
    {#if sells}
      {#if cash}
        <MetricCard label="Caja abierta" value={money(cash.expected_cents)} hint="Efectivo esperado" href="/panel/ventas" />
      {:else}
        <MetricCard label="Caja" value="Cerrada" hint="Abrir caja" href="/panel/ventas" />
      {/if}
    {/if}
    {#if toCharge}
      <MetricCard label="Sin cobrar" value={toCharge} hint={plural(toCharge, 'cita ya terminó', 'citas ya terminaron')} />
    {/if}
    {#if waiting}
      <MetricCard label="Anticipos pendientes" value={waiting} hint={waiting === 1 ? 'cita espera su anticipo' : 'citas esperan su anticipo'} />
    {/if}
  </div>

  {#if me.business.status === 'trial' && me.business.trial_ends_at}
    <p class="banner">Prueba gratuita hasta el {formatDay(me.business.trial_ends_at)}.</p>
  {/if}
  
  <div class="today">
    <Surface title={role === 'staff' ? 'Tu día' : 'Agenda de hoy'} padding="compact">
      {#if sorted.length === 0}
        <EmptyState title="Hoy no hay citas todavía">
          {#snippet actions()}<a class="btn ghost" href="/panel/agenda">Agendar una cita</a>{/snippet}
        </EmptyState>
      {:else}
        <ol class="slots">
          {#each sorted as a (a.id)}
            {@const st = statusOf(a)}
            <li class:next={next?.id === a.id} class:done={a.status === 'completed' || a.status === 'no_show'}>
              <AppointmentSummary
                client={a.client_name}
                service={a.service_name}
                staff={a.staff_name}
                color={colorOf(a.staff_id)}
                compact
                timeLabel={fmtTime(a.starts_at, tz)}
                statusLabel={st[0]}
                tone={st[1]}
              >
                {#snippet actions()}
                  {#if sells && a.status === 'scheduled'}
                    <a class="btn sm" class:ghost={next?.id !== a.id && !late(a)} href={`/panel/ventas?cita=${a.id}`}>Cobrar</a>
                  {/if}
                {/snippet}
              </AppointmentSummary>
            </li>
          {/each}
        </ol>
      {/if}
    </Surface>

    <aside aria-label="Resumen del día">
      <Surface title="Por resolver" padding="compact">
        {#if lateAppts.length === 0 && pendingAppts.length === 0}
          <p class="quiet">Nada pendiente por ahora.</p>
        {/if}
        {#if lateAppts.length}
          <h3>Sin cobrar · {lateAppts.length}</h3>
          <p class="quiet">{plural(toCharge, 'cita ya terminó', 'citas ya terminaron')}.</p>
          <ul>
            {#each lateAppts as a (a.id)}
              <li><span>{fmtTime(a.starts_at, tz)} · {a.client_name}</span>{#if sells}<a href={`/panel/ventas?cita=${a.id}`}>Cobrar</a>{/if}</li>
            {/each}
          </ul>
        {/if}
        {#if pendingAppts.length}
          <h3>Esperan anticipo · {pendingAppts.length}</h3>
          <ul>
            {#each pendingAppts as a (a.id)}
              <li><span>{fmtTime(a.starts_at, tz)} · {a.client_name}</span></li>
            {/each}
          </ul>
        {/if}
      </Surface>
      {#if chart.length}
        <Surface title="Ventas de los últimos 7 días" padding="compact">
          <BarChart label="Ventas de los últimos 7 días" items={chart} selectedId={today} />
        </Surface>
      {/if}
    </aside>
  </div>
{/if}

<style>
  .metrics { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); margin-bottom: 20px; }
  .today { display: grid; gap: 20px; grid-template-columns: 1fr; align-items: start; }
  @media (min-width: 1000px) { .today { grid-template-columns: minmax(0, 1fr) 320px; } }

  .slots { list-style: none; margin: 0; padding: 0; }
  .slots li { padding: 0 8px; border-left: 3px solid transparent; border-bottom: 1px solid var(--border); }
  .slots li:last-child { border-bottom: 0; }
  .slots li :global(.appt) { border-bottom: 0; }
  .slots li.next { border-left-color: var(--accent); background: var(--accent-soft); border-radius: 0 var(--radius-sm) var(--radius-sm) 0; padding-left: 12px; }
  .slots li.done :global(.who strong) { color: var(--muted); font-weight: 600; }

  aside { display: grid; gap: 20px; }
  @media (max-width: 480px) { .metrics :global(.value) { font-size: 1.5rem; } }
  aside :global(.chart) { gap: 4px; }
  aside :global(.chart .val) { font-size: 0.68rem; letter-spacing: -0.02em; }
  aside h3 { margin: 16px 0 4px; font-size: 0.95rem; }
  aside h3:first-child { margin-top: 0; }
  .quiet { margin: 0 0 8px; color: var(--muted); font-size: 0.9rem; }
  aside ul { list-style: none; margin: 0; padding: 0; }
  aside li { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 44px; border-top: 1px solid var(--border); font-size: 0.92rem; }
  aside li:first-child { border-top: 0; }
  aside li a { font-weight: 600; }
</style>
