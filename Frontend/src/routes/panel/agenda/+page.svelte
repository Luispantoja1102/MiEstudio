<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import Modal from '$lib/Modal.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import SegmentedControl from '$lib/ui/SegmentedControl.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import NewAppointment from '$lib/NewAppointment.svelte';
  import AppointmentDetail from '$lib/AppointmentDetail.svelte';
  import BlockForm from '$lib/BlockForm.svelte';
  import { api } from '$lib/api.js';
  import { addDays, clockToMin, fmtTime, minToClock, minutesOfDay, todayIn } from '$lib/time.js';
  import { dayHead, gaps, rangeFor, rangeLabel, shift, weekday, withLanes } from '$lib/agenda.js';
  import { hasPos, session } from '$lib/session.svelte.js';

  const PX = 1.4; // píxeles por minuto
  const tz = session.me.business.timezone || 'America/Tijuana';
  const role = session.me.role;
  const canBook = ['admin', 'reception', 'staff'].includes(role);
  const sells = ['admin', 'cashier', 'reception'].includes(role) && hasPos();
  const VIEWS = [['day', 'Día'], ['week', 'Semana'], ['month', 'Mes'], ['list', 'Lista']];
  const VIEW_OPTIONS = VIEWS.map(([value, label]) => ({ value, label }));
  const KEY = 'miestudio-agenda-view';

  let branches = $state([]);
  let branchId = $state(null);
  let date = $state(todayIn(tz));
  let view = $state('day');
  let who = $state(role === 'staff' ? session.me.id : 0); // 0 = todos
  let data = $state(null);
  let error = $state('');
  let showCancelled = $state(false);
  let newOpen = $state(false);
  let blockOpen = $state(false);
  let detail = $state(null);
  let detailOpen = $state(false);
  let seq = 0;

  async function load() {
    if (!branchId) return;
    const mine = ++seq;
    const { from, days } = rangeFor(view, date);
    try {
      const r = await api('/api/agenda?' + new URLSearchParams({ branch_id: branchId, date: from, days }));
      if (mine === seq) {
        data = r;
        error = '';
      }
    } catch (e) {
      if (mine === seq) error = e.message;
    }
  }

  onMount(async () => {
    try {
      const saved = localStorage.getItem(KEY);
      if (VIEWS.some(([v]) => v === saved)) view = saved;
      else if (window.matchMedia('(max-width: 699px)').matches) view = 'list';
    } catch {}
    try {
      branches = (await api('/api/branches')).filter((b) => b.active);
      branchId = branches[0]?.id ?? null;
      await load();
    } catch (e) {
      error = e.message;
    }
  });

  function setView(v) {
    view = v;
    try { localStorage.setItem(KEY, v); } catch {}
    load();
  }
  const go = (d) => {
    date = d;
    load();
  };
  // desde el mes o la semana, un día se abre en la vista de día
  const openDay = (d) => {
    date = d;
    setView('day');
  };

  const dateOf = (iso) => new Date(iso).toLocaleDateString('en-CA', { timeZone: tz });
  const staffAll = $derived(data?.staff ?? []);
  const shown = $derived(who ? staffAll.filter((s) => s.id === who) : staffAll);
  const color = (id) => `var(--p${(Math.max(0, staffAll.findIndex((s) => s.id === id)) % 6) + 1})`;

  const appts = $derived(
    (data?.appointments ?? []).filter((a) => (showCancelled || a.status !== 'cancelled') && (!who || a.staff_id === who))
  );
  const blocks = $derived((data?.blocks ?? []).filter((b) => !who || b.user_id === who));
  const apptsOn = (d) => appts.filter((a) => dateOf(a.starts_at) === d);
  const blocksOn = (d) => blocks.filter((b) => dateOf(b.starts_at) === d);
  // en el mes solo cuentan los días del mes (la cuadrícula trae también días vecinos)
  const counts = $derived(appts.filter((a) => a.status === 'scheduled' && (view !== 'month' || dateOf(a.starts_at).slice(0, 7) === date.slice(0, 7))).length);
  const cap = (t) => t.charAt(0).toUpperCase() + t.slice(1);

  // horario de trabajo de un día de la semana (0 = domingo) de las personas que se muestran: [[ini, fin], ...] en minutos
  const segsOf = (staff, wd) => (staff.week_hours?.[String(wd)] ?? []).map((h) => [clockToMin(h.start), clockToMin(h.end)]);
  const unionSegs = (wd) => shown.flatMap((s) => segsOf(s, wd));

  // rango de horas visible: 8–20, ampliado a lo que haya de horarios y citas
  const range = $derived.by(() => {
    let lo = 8 * 60, hi = 20 * 60;
    for (const s of shown) for (const wd of Object.keys(s.week_hours ?? {})) for (const [a, b] of segsOf(s, wd)) (lo = Math.min(lo, a), (hi = Math.max(hi, b)));
    for (const a of appts) (lo = Math.min(lo, minutesOfDay(a.starts_at, tz)), (hi = Math.max(hi, minutesOfDay(a.ends_at, tz))));
    return { start: Math.floor(lo / 60) * 60, end: Math.ceil(hi / 60) * 60 };
  });
  const height = $derived((range.end - range.start) * PX);
  const hours = $derived(Array.from({ length: (range.end - range.start) / 60 + 1 }, (_, i) => range.start + i * 60));
  const top = (iso) => (minutesOfDay(iso, tz) - range.start) * PX;
  const span = (a, b) => Math.max((minutesOfDay(b, tz) - minutesOfDay(a, tz)) * PX, 22);

  const weekDays = $derived(view === 'week' ? Array.from({ length: 7 }, (_, i) => addDays(rangeFor('week', date).from, i)) : []);
  const monthDays = $derived(view === 'month' ? Array.from({ length: 42 }, (_, i) => addDays(rangeFor('month', date).from, i)) : []);
  const today = todayIn(tz);
  const inMonth = (d) => d.slice(0, 7) === date.slice(0, 7);

  // citas de un día de la semana acomodadas en carriles cuando se empalman (varias personas a la vez)
  const laneAppts = (d) => withLanes(apptsOn(d), (a) => minutesOfDay(a.starts_at, tz), (a) => Math.max(minutesOfDay(a.ends_at, tz), minutesOfDay(a.starts_at, tz) + 20));
  const lane = (a) => `left: calc(${(a.lane / a.lanes) * 100}% + 2px); width: calc(${100 / a.lanes}% - 4px)`;

  const listItems = $derived([...appts].sort((a, b) => a.starts_at.localeCompare(b.starts_at)));
  // la siguiente cita agendada de hoy (solo presentación: se destaca con un filete)
  const nextId = $derived(view === 'month' ? null : (listItems.find((a) => a.status === 'scheduled' && dateOf(a.starts_at) === today && new Date(a.starts_at).getTime() > Date.now())?.id ?? null));
  const STATE = { completed: ['Cobrada', 'muted'], no_show: ['No asistió', 'muted'], cancelled: ['Cancelada', 'muted'] };
  // lo que dice una cita además de su estado: llegó, confirmada o qué falta de su anticipo por transferencia
  const mark = (a) => {
    if (a.status === 'pending_payment') {
      if (a.deposit_method === 'transfer') return a.proof_at ? 'Revisar comprobante' : 'Falta comprobante';
      return 'Espera anticipo';
    }
    if (a.status !== 'scheduled') return null;
    return a.refund_due ? 'Devolver anticipo' : a.arrived_at ? 'Llegó' : a.confirmed_at ? 'Confirmada' : null;
  };
  const tone = (a) => (STATE[a.status] ? 'neutral' : needsAction(a) || a.status === 'pending_payment' ? 'warning' : 'success');
  const needsAction = (a) => (a.status === 'pending_payment' && a.deposit_method === 'transfer' && !!a.proof_at) || a.refund_due;

  function openDetail(a) {
    detail = a;
    detailOpen = true;
  }

  async function removeBlock(b) {
    if (!canBook || !confirm('¿Quitar este bloqueo?')) return;
    try {
      await api(`/api/blocks/${b.id}`, { method: 'DELETE' });
      await load();
    } catch (e) {
      error = e.message;
    }
  }

  // un aviso de la campana abre la cita: ?cita=ID&date=AAAA-MM-DD
  $effect(() => {
    const id = Number(page.url.searchParams.get('cita'));
    const d = page.url.searchParams.get('date');
    if (id && branchId) openFromLink(id, d);
  });
  async function openFromLink(id, d) {
    await goto('/panel/agenda', { replaceState: true, noScroll: true, keepFocus: true });
    if (d) date = d;
    view = 'day';
    who = 0;
    await load();
    const a = (data?.appointments ?? []).find((x) => x.id === id);
    if (a) openDetail(a);
  }

  const done = async () => {
    newOpen = blockOpen = detailOpen = false;
    await load();
  };
</script>

{#snippet body(a, showStaff)}
  <span class="t">{fmtTime(a.starts_at, tz)}{#if a.id === nextId} · Siguiente{/if}</span>
  <span class="n">{a.client_name}</span>
  <span class="m">{#if showStaff}{a.staff_name} · {/if}{a.service_name}{#if mark(a)}<em>{mark(a)}</em>{/if}</span>
{/snippet}

<PageHeader title="Agenda" description="{cap(rangeLabel(view, date))} · {counts} {counts === 1 ? 'cita agendada' : 'citas agendadas'}{who && staffAll.length ? ' · ' + staffAll.find((s) => s.id === who)?.name : ''}">
  {#snippet actions()}
    {#if canBook && branchId}
      <button class="btn ghost" onclick={() => (blockOpen = true)}>Bloquear horario</button>
      <button class="btn" onclick={() => (newOpen = true)}>Nueva cita</button>
    {/if}
  {/snippet}
</PageHeader>

{#if error}<InlineNotice tone="danger" message={error} />{/if}

<div class="canvas">
  <div class="bar">
    <SegmentedControl label="Vista" options={VIEW_OPTIONS} value={view} onchange={setView} />
    <div class="nav">
      <button class="nb" onclick={() => go(shift(view, date, -1))} aria-label="Anterior"><span class="flip"><Icon name="arrow" size={16} /></span></button>
      <button class="nb today" onclick={() => go(todayIn(tz))}>Hoy</button>
      <button class="nb" onclick={() => go(shift(view, date, 1))} aria-label="Siguiente"><Icon name="arrow" size={16} /></button>
      <input class="dateinput" type="date" bind:value={date} onchange={load} aria-label="Fecha" />
    </div>
    {#if branches.length > 1}
      <select class="branchsel" bind:value={branchId} onchange={load} aria-label="Sucursal">
        {#each branches as b (b.id)}<option value={b.id}>{b.name}</option>{/each}
      </select>
    {/if}
    <label class="check"><input type="checkbox" bind:checked={showCancelled} />Ver canceladas</label>
  </div>

  {#if staffAll.length > 1}
    <div class="people" role="group" aria-label="Persona">
      <button aria-pressed={who === 0} onclick={() => (who = 0)}>Todos</button>
      {#each staffAll as s (s.id)}
        <button aria-pressed={who === s.id} onclick={() => (who = s.id)}><i class="dot" style="background: {color(s.id)}"></i>{s.name}</button>
      {/each}
    </div>
  {/if}

  {#if !data}
    {#if !error}<LoadingState label="Cargando la agenda…" />{/if}
  {:else if staffAll.length === 0}
    <EmptyState title="Aún no hay personas en la agenda.">
      {#snippet actions()}
        {#if role === 'admin'}<span>Ve a <a href="/panel/equipo">Equipo</a>, marca «En agenda» y define su horario.</span>{/if}
      {/snippet}
    </EmptyState>

  {:else if view === 'list'}
    {#if listItems.length === 0}
      <EmptyState title="No hay citas {who ? 'para esta persona' : ''} este día." />
    {:else}
      <ol class="daylist">
        {#each listItems as a (a.id)}
          <li class:cancelled={a.status === 'cancelled'} class:next={a.id === nextId}>
            <button class="row" onclick={() => openDetail(a)}>
              <time class="when" datetime={a.starts_at}>{fmtTime(a.starts_at, tz)}</time>
              <span class="what">
                <b>{a.client_name}</b>
                <span class="meta">{a.service_name}{#if !who}<span class="sep" aria-hidden="true">·</span><span class="stf"><i class="dot" style="background: {color(a.staff_id)}"></i>{a.staff_name}</span>{/if}</span>
                {#if a.id === nextId || STATE[a.status] || a.deposit_status === 'paid' || mark(a)}
                  <span class="tags">
                    {#if a.id === nextId}<StatusBadge label="Siguiente" tone="accent" />{/if}
                    {#if STATE[a.status] || a.deposit_status === 'paid' || mark(a)}<StatusBadge label={STATE[a.status]?.[0] ?? mark(a) ?? 'Anticipo pagado'} tone={tone(a)} />{/if}
                  </span>
                {/if}
              </span>
            </button>
            {#if sells && a.status === 'scheduled'}<a class="btn sm ghost" href={`/panel/ventas?cita=${a.id}`}>Cobrar</a>{/if}
          </li>
        {/each}
      </ol>
    {/if}

  {:else if view === 'day'}
    <div class="wrap cal">
      <div class="grid" style="grid-template-columns: 52px repeat({shown.length}, minmax(170px, 1fr))">
        <div class="corner"></div>
        {#each shown as s (s.id)}<div class="colhead"><i class="dot" style="background: {color(s.id)}"></i>{s.name}</div>{/each}
        <div class="axis" style="height: {height}px">
          {#each hours as h (h)}<span style="top: {(h - range.start) * PX}px">{minToClock(h)}</span>{/each}
        </div>
        {#each shown as s (s.id)}
          <div class="col" style="height: {height}px; --hour: {60 * PX}px">
            {#each gaps(segsOf(s, weekday(date)), range.start, range.end) as [a, b]}
              <div class="closed" style="top: {(a - range.start) * PX}px; height: {(b - a) * PX}px"></div>
            {/each}
            {#if segsOf(s, weekday(date)).length === 0}<span class="off">No trabaja este día</span>{/if}
            {#each blocksOn(date).filter((b) => b.user_id === s.id) as b (b.id)}
              <button class="block" style="top: {top(b.starts_at)}px; height: {span(b.starts_at, b.ends_at)}px" onclick={() => removeBlock(b)}
                aria-label="Bloqueo {fmtTime(b.starts_at, tz)} a {fmtTime(b.ends_at, tz)}">
                <b>{fmtTime(b.starts_at, tz)}–{fmtTime(b.ends_at, tz)}</b> {b.reason || 'Bloqueado'}
              </button>
            {/each}
            {#each apptsOn(date).filter((a) => a.staff_id === s.id) as a (a.id)}
              <button class="appt {a.status}" class:online={a.source === 'online'} class:arrived={a.status === 'scheduled' && a.arrived_at} class:action={needsAction(a)} class:next={a.id === nextId}
                style="top: {top(a.starts_at)}px; height: {span(a.starts_at, a.ends_at)}px; --who: {color(a.staff_id)}" onclick={() => openDetail(a)}>
                {@render body(a, false)}
              </button>
            {/each}
          </div>
        {/each}
      </div>
    </div>

  {:else if view === 'week'}
    <div class="wrap cal">
      <div class="grid" style="grid-template-columns: 52px repeat(7, minmax(128px, 1fr))">
        <div class="corner"></div>
        {#each weekDays as d (d)}
          <button class="colhead dayhead" class:today={d === today} onclick={() => openDay(d)} aria-label="Abrir el día {d}">
            <span>{dayHead(d).name}</span><b>{dayHead(d).num}</b>
          </button>
        {/each}
        <div class="axis" style="height: {height}px">
          {#each hours as h (h)}<span style="top: {(h - range.start) * PX}px">{minToClock(h)}</span>{/each}
        </div>
        {#each weekDays as d (d)}
          <div class="col" class:todaycol={d === today} style="height: {height}px; --hour: {60 * PX}px">
            {#each gaps(unionSegs(weekday(d)), range.start, range.end) as [a, b]}
              <div class="closed" style="top: {(a - range.start) * PX}px; height: {(b - a) * PX}px"></div>
            {/each}
            {#each blocksOn(d) as b (b.id)}
              <button class="block" style="top: {top(b.starts_at)}px; height: {span(b.starts_at, b.ends_at)}px" onclick={() => removeBlock(b)}
                aria-label="Bloqueo {fmtTime(b.starts_at, tz)} a {fmtTime(b.ends_at, tz)}">
                <b>{fmtTime(b.starts_at, tz)}</b> {b.reason || 'Bloqueado'}
              </button>
            {/each}
            {#each laneAppts(d) as a (a.id)}
              <button class="appt tight {a.status}" class:online={a.source === 'online'} class:arrived={a.status === 'scheduled' && a.arrived_at} class:action={needsAction(a)} class:next={a.id === nextId}
                style="top: {top(a.starts_at)}px; height: {span(a.starts_at, a.ends_at)}px; {lane(a)}; --who: {color(a.staff_id)}" onclick={() => openDetail(a)}>
                {@render body(a, !who)}
              </button>
            {/each}
          </div>
        {/each}
      </div>
    </div>

  {:else}
    <div class="month cal">
      {#each ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom'] as n}<div class="mh">{n}</div>{/each}
      {#each monthDays as d (d)}
        {@const list = apptsOn(d)}
        <button class="cell" class:out={!inMonth(d)} class:today={d === today} onclick={() => openDay(d)} aria-label="{d}: {list.length} citas">
          <span class="num">{Number(d.slice(8))}</span>
          {#each list.slice(0, 3) as a (a.id)}
            <span class="mini" class:cancelled={a.status === 'cancelled'} class:action={needsAction(a)} title={a.staff_name}><i class="dot" style="background: {color(a.staff_id)}"></i><b>{fmtTime(a.starts_at, tz)}</b><span>{a.client_name}</span></span>
          {/each}
          {#if list.length > 3}<span class="more">+{list.length - 3} más</span>{/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

<Modal bind:open={newOpen} title="Nueva cita">
  <NewAppointment {branchId} {date} {tz} ondone={done} />
</Modal>
<Modal bind:open={blockOpen} title="Bloquear horario">
  <BlockForm staff={data?.staff ?? []} {date} {tz} selfId={role === 'staff' ? session.me.id : null} ondone={done} />
</Modal>
<Modal bind:open={detailOpen} title="Cita">
  {#if detail}<AppointmentDetail appt={detail} {tz} date={dateOf(detail.starts_at)} {canBook} ondone={done} />{/if}
</Modal>

<style>
  /* Toda la agenda usa una sola familia con cifras de ancho fijo, para que horas y fechas se lean parejas */
  .canvas, .cal, .nav, .people, .daylist { font-variant-numeric: tabular-nums lining-nums; }

  /* lienzo blanco: barra de herramientas arriba y el calendario o la lista debajo */
  .canvas { overflow: hidden; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  .bar { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 16px; padding: 12px 16px; border-bottom: 1px solid var(--border); }
  .nav { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; max-width: 100%; }
  .nb { flex: none; display: inline-grid; place-items: center; min-width: 44px; height: 44px; padding: 0 14px; font: inherit; font-size: 0.9rem; font-weight: 600; color: var(--text); background: var(--surface); border: 1px solid var(--control-border); border-radius: var(--radius-sm); cursor: pointer; transition: background-color var(--dur-ui) var(--ease), border-color var(--dur-ui) var(--ease), transform var(--dur-press) var(--ease); }
  .nb:not(.today) { padding: 0; }
  .nb:hover { background: var(--surface-2); border-color: var(--text); }
  .nb:active { transform: scale(0.96); }
  .flip { display: inline-flex; transform: rotate(180deg); }
  .dateinput, .branchsel { width: auto; min-width: 0; flex: 1 1 150px; height: 44px; min-height: 44px; margin: 0; padding: 0 12px; font-size: 0.9rem; }

  /* filtro por profesional: el color es solo un punto junto al nombre */
  .people { display: flex; flex-wrap: wrap; gap: 6px; padding: 10px 16px; border-bottom: 1px solid var(--border); }
  .people button { display: inline-flex; align-items: center; gap: 8px; min-height: 44px; padding: 0 14px; font: inherit; font-size: 0.9rem; font-weight: 500; color: var(--text); background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; transition: background-color var(--dur-ui) var(--ease), border-color var(--dur-ui) var(--ease); }
  .people button:hover { border-color: var(--text); }
  .people button[aria-pressed='true'] { color: var(--text); background: var(--ink-soft); border-color: var(--ink); font-weight: 600; }
  .dot { display: inline-block; width: 9px; height: 9px; border-radius: 50%; flex: none; }

  /* lista: filas separadas por filetes, operable con teclado (cada fila es un botón) */
  .daylist { list-style: none; margin: 0; padding: 0; }
  .daylist li { display: flex; align-items: center; gap: 8px; padding-right: 16px; border-bottom: 1px solid var(--border); border-left: 3px solid transparent; }
  .daylist li:last-child { border-bottom: 0; }
  .daylist li.next { border-left-color: var(--accent); background: var(--accent-soft); }
  .daylist li.cancelled b { text-decoration: line-through; color: var(--muted); }
  .daylist .row { flex: 1; min-width: 0; display: grid; grid-template-columns: 4rem 1fr; gap: 12px; align-items: center; min-height: 64px; padding: 12px 8px 12px 16px; font: inherit; text-align: left; color: var(--text); background: transparent; border: 0; cursor: pointer; }
  .daylist .row:hover { background: var(--surface-2); }
  .daylist .row:focus-visible { outline-offset: -3px; }
  .daylist .when { font-size: 1.1rem; font-weight: 700; }
  .daylist .what { display: grid; gap: 2px; justify-items: start; min-width: 0; }
  .daylist .meta { font-size: 0.875rem; color: var(--muted); }
  .daylist .sep { margin: 0 6px; }
  .daylist .stf { display: inline-flex; align-items: center; gap: 6px; }
  .tags { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 4px; }

  /* rejilla de horas: día y semana */
  .wrap { overflow: auto; max-height: 74vh; }
  .grid { display: grid; min-width: min-content; }
  .corner, .colhead { position: sticky; top: 0; z-index: 3; background: var(--surface); border-bottom: 1px solid var(--border); }
  .corner { left: 0; z-index: 4; }
  .colhead { display: flex; align-items: center; justify-content: center; gap: 8px; min-height: 56px; padding: 0 0.5rem; font-size: 0.9rem; font-weight: 600; border-left: 1px solid var(--border); }
  .dayhead { flex-direction: column; gap: 2px; justify-content: center; font: inherit; font-size: 0.9rem; font-weight: 600; color: var(--text); border-top: 0; border-right: 0; cursor: pointer; }
  .dayhead span { font-size: 0.72rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.09em; color: var(--muted); }
  .dayhead b { display: grid; place-items: center; min-width: 30px; height: 30px; font-size: 1.05rem; font-weight: 700; border-radius: 999px; }
  .dayhead.today b { color: var(--side-mark-ink); background: var(--side-mark); }
  .dayhead.today span { color: var(--accent); }
  .dayhead:hover { background: var(--surface-2); }

  .axis { position: sticky; left: 0; z-index: 2; background: var(--surface); }
  .axis span { position: absolute; right: 8px; transform: translateY(-50%); font-size: 0.78rem; font-weight: 500; color: var(--muted); }
  .axis span:first-child { transform: none; }

  .col {
    position: relative;
    border-left: 1px solid var(--border);
    background-image:
      repeating-linear-gradient(to bottom, transparent 0, transparent calc(var(--hour) - 1px), color-mix(in srgb, var(--border) 90%, transparent) calc(var(--hour) - 1px), color-mix(in srgb, var(--border) 90%, transparent) var(--hour)),
      repeating-linear-gradient(to bottom, transparent 0, transparent calc(var(--hour) / 2 - 1px), color-mix(in srgb, var(--border) 35%, transparent) calc(var(--hour) / 2 - 1px), color-mix(in srgb, var(--border) 35%, transparent) calc(var(--hour) / 2));
  }
  .todaycol { background-color: color-mix(in srgb, var(--accent-soft) 30%, transparent); }
  .closed { position: absolute; left: 0; right: 0; background: color-mix(in srgb, var(--surface-2) 90%, transparent); pointer-events: none; }
  .off { position: absolute; top: 10px; left: 0; right: 0; text-align: center; font-size: 0.8rem; color: var(--muted); }

  /* citas y bloqueos: la altura sale de la duración (no se agranda); el profesional va como filete a la izquierda */
  .appt, .block { position: absolute; left: 4px; right: 4px; z-index: 1; overflow: hidden; display: flex; flex-direction: column; gap: 1px; padding: 6px 9px; font: inherit; line-height: 1.25; text-align: left; color: var(--text); border-radius: var(--radius-xs); cursor: pointer; }
  .appt {
    --who: var(--accent);
    background: var(--surface);
    border: 1px solid var(--control-border);
    border-left: 4px solid var(--who);
    transition: background-color var(--dur-ui) var(--ease), transform var(--dur-press) var(--ease);
  }
  .appt .t { font-size: 0.75rem; font-weight: 700; letter-spacing: 0.01em; color: var(--text); }
  .appt .n { font-size: 0.85rem; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .appt .m { font-size: 0.78rem; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .appt .m em { margin-left: 6px; font-style: normal; font-weight: 700; color: var(--text); }
  /* citas cortas de la semana: hora y nombre en la misma línea y el detalle debajo, para que quepan en 30 minutos */
  .appt.tight { display: grid; grid-template-columns: auto minmax(0, 1fr); column-gap: 6px; align-content: start; padding: 4px 7px; }
  .appt.tight .m { grid-column: 1 / -1; }
  .appt.tight .n { font-size: 0.8rem; }
  .appt.arrived { background: var(--ink-soft); border-color: var(--ink); border-left-color: var(--who); }
  .appt.completed { background: var(--surface-2); border-color: var(--border); border-left-color: var(--who); }
  .appt.completed .t { color: var(--muted); }
  .appt.no_show { background: var(--warn-soft); border-color: var(--warn); border-left-color: var(--who); }
  .appt.pending_payment { background: var(--surface); border-style: dashed; border-left-style: solid; }
  .appt.action { border-color: var(--warn); outline: 2px solid var(--warn); outline-offset: -3px; }
  .appt.cancelled { background: var(--surface-2); border-color: var(--border); border-left-color: var(--border); }
  .appt.cancelled .n { text-decoration: line-through; color: var(--muted); }
  .appt.next { z-index: 2; border-color: var(--accent); background: var(--accent-soft); outline: 2px solid var(--accent); outline-offset: -3px; }
  .appt.next .t { color: var(--accent); }
  .appt:hover { background: var(--surface-2); }
  .appt.next:hover { background: var(--accent-soft); }
  .appt:active { transform: scale(0.99); }
  .appt:focus-visible, .block:focus-visible { outline: 2px solid var(--ink); outline-offset: -3px; z-index: 3; }
  .block { color: var(--muted); font-size: 0.8rem; border: 1px dashed var(--muted); background: var(--surface-2); }
  .block b { font-weight: 700; font-size: 0.75rem; color: var(--text); }

  /* mes */
  .month { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); }
  .mh { padding: 0.65rem 0.5rem; text-align: center; font-size: 0.72rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.09em; color: var(--muted); border-bottom: 1px solid var(--border); }
  .cell { display: flex; flex-direction: column; align-items: stretch; gap: 3px; min-height: 112px; padding: 8px; font: inherit; text-align: left; color: var(--text); background: transparent; border: 0; border-right: 1px solid var(--border); border-bottom: 1px solid var(--border); cursor: pointer; overflow: hidden; transition: background-color var(--dur-ui) var(--ease); }
  .cell:nth-child(7n + 7) { border-right: 0; }
  .cell:hover { background: var(--surface-2); }
  .cell:focus-visible { outline-offset: -3px; }
  .cell.out { color: var(--muted); background: var(--bg); }
  .num { align-self: flex-start; display: grid; place-items: center; min-width: 28px; height: 28px; padding: 0 6px; font-size: 0.9rem; font-weight: 700; border-radius: 999px; }
  .cell.today .num { background: var(--side-mark); color: var(--side-mark-ink); }
  .mini { display: flex; align-items: center; gap: 6px; min-width: 0; font-size: 0.78rem; line-height: 1.35; }
  .mini b { flex: none; font-weight: 700; }
  .mini span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .mini.cancelled { opacity: 0.6; text-decoration: line-through; }
  .mini.action span { font-weight: 700; color: var(--warn); }
  .more { font-size: 0.78rem; font-weight: 700; color: var(--accent); }

  @media (max-width: 699px) {
    .bar { padding: 12px; }
    .people { flex-wrap: nowrap; overflow-x: auto; padding: 8px 12px; }
    .people button { flex: none; }
    .cell { min-height: 78px; padding: 5px; }
    .mini { display: none; }
    .mini:first-of-type { display: flex; }
    .daylist li { padding-right: 12px; }
    .daylist .row { grid-template-columns: 3.4rem 1fr; padding-left: 12px; }
  }
</style>
