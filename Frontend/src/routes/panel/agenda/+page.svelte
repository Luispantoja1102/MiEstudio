<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import Modal from '$lib/Modal.svelte';
  import NewAppointment from '$lib/NewAppointment.svelte';
  import AppointmentDetail from '$lib/AppointmentDetail.svelte';
  import BlockForm from '$lib/BlockForm.svelte';
  import { api } from '$lib/api.js';
  import { addDays, clockToMin, fmtTime, minToClock, minutesOfDay, todayIn } from '$lib/time.js';
  import { dayHead, gaps, rangeFor, rangeLabel, shift, weekday, withLanes } from '$lib/agenda.js';
  import { session } from '$lib/session.svelte.js';

  const PX = 1.4; // píxeles por minuto
  const tz = session.me.business.timezone || 'America/Tijuana';
  const role = session.me.role;
  const canBook = ['admin', 'reception', 'staff'].includes(role);
  const sells = ['admin', 'cashier', 'reception'].includes(role);
  const VIEWS = [['day', 'Día'], ['week', 'Semana'], ['month', 'Mes'], ['list', 'Lista']];
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
  <span class="t">{fmtTime(a.starts_at, tz)}</span>
  <span class="n">{a.client_name}</span>
  <span class="m">{#if showStaff}{a.staff_name} · {/if}{a.service_name}{#if mark(a)}<em>{mark(a)}</em>{/if}</span>
{/snippet}

<div class="page-head">
  <div>
    <h1>Agenda</h1>
    <p>{cap(rangeLabel(view, date))} · {counts} {counts === 1 ? 'cita agendada' : 'citas agendadas'}{who && staffAll.length ? ' · ' + staffAll.find((s) => s.id === who)?.name : ''}</p>
  </div>
  {#if canBook && branchId}
    <div class="form-actions" style="margin: 0">
      <button class="btn ghost" onclick={() => (blockOpen = true)}>Bloquear horario</button>
      <button class="btn" onclick={() => (newOpen = true)}>Nueva cita</button>
    </div>
  {/if}
</div>

<div class="toolbar">
  <div class="seg" role="group" aria-label="Vista">
    {#each VIEWS as [v, label]}<button aria-pressed={view === v} onclick={() => setView(v)}>{label}</button>{/each}
  </div>
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

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if data}
  {#if staffAll.length === 0}
    <div class="empty">
      Aún no hay personas en la agenda.
      {#if role === 'admin'}Ve a <a href="/panel/equipo">Equipo</a>, marca «En agenda» y define su horario.{/if}
    </div>

  {:else if view === 'list'}
    {#if listItems.length === 0}
      <div class="empty">No hay citas {who ? 'para esta persona' : ''} este día.</div>
    {:else}
      <ol class="daylist">
        {#each listItems as a (a.id)}
          <li class={a.status}>
            <button class="row" onclick={() => openDetail(a)}>
              <time class="when" datetime={a.starts_at}>{fmtTime(a.starts_at, tz)}</time>
              <span class="what">
                <b>{a.client_name}</b>
                <span class="meta">{a.service_name}{#if !who}{' · ' + a.staff_name}{/if}</span>
                {#if STATE[a.status] || a.deposit_status === 'paid' || mark(a)}
                  <span class="badge {STATE[a.status]?.[1] ?? (needsAction(a) ? 'warn' : '')}">{STATE[a.status]?.[0] ?? mark(a) ?? 'Anticipo pagado'}</span>
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
              <button class="appt {a.status}" class:online={a.source === 'online'} class:arrived={a.status === 'scheduled' && a.arrived_at} class:action={needsAction(a)}
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
              <button class="appt tight {a.status}" class:online={a.source === 'online'} class:arrived={a.status === 'scheduled' && a.arrived_at} class:action={needsAction(a)}
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
            <span class="mini" class:cancelled={a.status === 'cancelled'} class:action={needsAction(a)}><i class="dot" style="background: {color(a.staff_id)}"></i><b>{fmtTime(a.starts_at, tz)}</b><span>{a.client_name}</span></span>
          {/each}
          {#if list.length > 3}<span class="more">+{list.length - 3} más</span>{/if}
        </button>
      {/each}
    </div>
  {/if}
{/if}

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
  /* Toda la agenda usa una sola familia (Inter) con cifras de ancho fijo, para que horas y fechas se lean parejas */
  .cal, .seg, .nav, .people, .daylist { font-family: var(--font-body, 'Inter', system-ui, sans-serif); font-variant-numeric: tabular-nums lining-nums; }

  .toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 14px; margin-bottom: 0.9rem; }
  .seg { display: inline-flex; padding: 4px; gap: 2px; background: var(--surface-2); border: 1px solid var(--border); border-radius: 999px; }
  .seg button { min-height: 34px; padding: 0 16px; font: inherit; font-size: 0.88rem; font-weight: 600; color: var(--muted); background: transparent; border: 0; border-radius: 999px; cursor: pointer; transition: background-color 0.18s var(--ease), color 0.18s var(--ease), transform 0.12s var(--ease); }
  .seg button:hover { color: var(--text); }
  .seg button:active { transform: scale(0.96); }
  .seg button[aria-pressed='true'] { color: var(--accent-contrast); background: var(--accent); }

  .nav { display: flex; align-items: center; gap: 6px; }
  .nb { display: inline-grid; place-items: center; min-width: 40px; height: 40px; padding: 0 14px; font: inherit; font-size: 0.88rem; font-weight: 600; color: var(--text); background: var(--surface); border: 1px solid var(--border); border-radius: 999px; cursor: pointer; transition: background-color 0.18s var(--ease), border-color 0.18s var(--ease), transform 0.12s var(--ease); }
  .nb:not(.today) { padding: 0; }
  .nb:hover { background: var(--surface-2); border-color: var(--muted); }
  .nb:active { transform: scale(0.95); }
  .flip { display: inline-flex; transform: rotate(180deg); }
  .dateinput, .branchsel { width: auto; height: 40px; min-height: 40px; margin: 0; padding: 0 14px; font-size: 0.88rem; border-radius: 999px; }

  .people { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 1rem; }
  .people button { display: inline-flex; align-items: center; gap: 7px; min-height: 34px; padding: 0 14px; font: inherit; font-size: 0.85rem; font-weight: 600; color: var(--text); background: var(--surface); border: 1px solid var(--border); border-radius: 999px; cursor: pointer; transition: background-color 0.18s var(--ease), border-color 0.18s var(--ease), transform 0.12s var(--ease); }
  .people button:hover { border-color: var(--muted); }
  .people button:active { transform: scale(0.96); }
  .people button[aria-pressed='true'] { color: var(--accent-contrast); background: var(--accent); border-color: var(--accent); }
  .dot { display: inline-block; width: 9px; height: 9px; border-radius: 50%; flex: none; box-shadow: 0 0 0 2px color-mix(in srgb, var(--surface) 85%, transparent); }

  .daylist { list-style: none; margin: 0; padding: 0; display: grid; gap: 8px; }
  .daylist li { display: flex; align-items: center; gap: 8px; padding-right: 0.75rem; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  .daylist li.cancelled { opacity: 0.55; }
  .daylist .row { flex: 1; min-width: 0; display: grid; grid-template-columns: 3.4rem 1fr; gap: 10px; align-items: center; padding: 0.8rem 0.5rem 0.8rem 0.85rem; font: inherit; text-align: left; color: var(--text); background: transparent; border: 0; cursor: pointer; }
  .daylist .when { font-size: 1.1rem; font-weight: 700; }
  .daylist .what { display: grid; gap: 2px; justify-items: start; min-width: 0; }
  .daylist .meta { font-size: 0.86rem; color: var(--muted); }

  /* rejilla de horas: día y semana */
  .wrap { overflow: auto; max-height: 74vh; border: 1px solid var(--border); border-radius: var(--radius); background: var(--surface); }
  .grid { display: grid; min-width: min-content; }
  .corner, .colhead { position: sticky; top: 0; z-index: 3; background: var(--surface); border-bottom: 1px solid var(--border); }
  .corner { left: 0; z-index: 4; }
  .colhead { display: flex; align-items: center; justify-content: center; gap: 8px; min-height: 56px; padding: 0 0.5rem; font-size: 0.9rem; font-weight: 600; border-left: 1px solid var(--border); }
  .dayhead { flex-direction: column; gap: 2px; justify-content: center; font: inherit; font-size: 0.9rem; font-weight: 600; color: var(--text); border-top: 0; border-right: 0; cursor: pointer; }
  .dayhead span { font-size: 0.7rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.09em; color: var(--muted); }
  .dayhead b { display: grid; place-items: center; min-width: 30px; height: 30px; font-size: 1.05rem; font-weight: 700; border-radius: 999px; }
  .dayhead.today b { color: var(--accent-contrast); background: var(--accent); }
  .dayhead.today span { color: var(--accent); }
  .dayhead:hover { background: var(--surface-2); }

  .axis { position: sticky; left: 0; z-index: 2; background: var(--surface); }
  .axis span { position: absolute; right: 8px; transform: translateY(-50%); font-size: 0.72rem; font-weight: 500; color: var(--muted); }
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
  .off { position: absolute; top: 10px; left: 0; right: 0; text-align: center; font-size: 0.78rem; color: var(--muted); }

  /* citas y bloqueos: misma estructura (hora, nombre, detalle) y escala en todas las vistas */
  .appt, .block { position: absolute; left: 4px; right: 4px; z-index: 1; overflow: hidden; display: flex; flex-direction: column; gap: 1px; padding: 6px 9px; font: inherit; line-height: 1.25; text-align: left; color: var(--text); border-radius: 9px; cursor: pointer; }
  .appt {
    --who: var(--accent);
    background: color-mix(in srgb, var(--who) 15%, var(--surface));
    border: 1px solid color-mix(in srgb, var(--who) 48%, var(--surface));
    transition: filter 0.18s var(--ease), transform 0.12s var(--ease), box-shadow 0.18s var(--ease);
  }
  .appt .t { font-size: 0.72rem; font-weight: 700; letter-spacing: 0.01em; color: color-mix(in srgb, var(--who) 70%, var(--text)); }
  .appt .n { font-size: 0.82rem; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .appt .m { font-size: 0.74rem; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .appt .m em { margin-left: 6px; font-style: normal; font-weight: 700; color: var(--text); }
  .appt.tight { padding: 4px 7px; }
  .appt.tight .n { font-size: 0.78rem; }
  .appt.arrived { background: var(--who); border-color: var(--who); color: #fff; }
  .appt.arrived .t, .appt.arrived .m, .appt.arrived .m em { color: rgba(255, 255, 255, 0.9); }
  .appt.completed { background: var(--surface-2); border-color: var(--border); }
  .appt.completed .t { color: var(--muted); }
  .appt.no_show { background: var(--warn-soft); border-color: color-mix(in srgb, var(--warn) 45%, var(--surface)); }
  .appt.pending_payment { background: var(--surface); border: 1px dashed color-mix(in srgb, var(--who) 70%, var(--surface)); }
  .appt.action { border-style: solid; border-color: var(--warn); box-shadow: 0 0 0 2px color-mix(in srgb, var(--warn) 22%, transparent); }
  .appt.online { box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--who) 30%, transparent); }
  .appt.action.online { box-shadow: 0 0 0 2px color-mix(in srgb, var(--warn) 22%, transparent); }
  .appt.cancelled { opacity: 0.5; background: var(--surface); border-color: var(--border); }
  .appt.cancelled .n { text-decoration: line-through; }
  .appt:hover { filter: brightness(0.97); }
  .appt:active { transform: scale(0.99); }
  .block { color: var(--muted); font-size: 0.78rem; border: 1px dashed var(--muted); background: repeating-linear-gradient(45deg, transparent 0 6px, var(--surface-2) 6px 12px); }
  .block b { font-weight: 700; font-size: 0.72rem; }

  /* mes */
  .month { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); border: 1px solid var(--border); border-radius: var(--radius); background: var(--surface); overflow: hidden; }
  .mh { padding: 0.65rem 0.5rem; text-align: center; font-size: 0.7rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.09em; color: var(--muted); border-bottom: 1px solid var(--border); }
  .cell { display: flex; flex-direction: column; align-items: stretch; gap: 3px; min-height: 112px; padding: 8px; font: inherit; text-align: left; color: var(--text); background: transparent; border: 0; border-right: 1px solid var(--border); border-bottom: 1px solid var(--border); cursor: pointer; overflow: hidden; transition: background-color 0.18s var(--ease); }
  .cell:nth-child(7n + 7) { border-right: 0; }
  .cell:hover { background: var(--surface-2); }
  .cell.out { color: var(--muted); background: color-mix(in srgb, var(--surface-2) 55%, transparent); }
  .num { align-self: flex-start; display: grid; place-items: center; min-width: 28px; height: 28px; padding: 0 6px; font-size: 0.88rem; font-weight: 700; border-radius: 999px; }
  .cell.today .num { background: var(--accent); color: var(--accent-contrast); }
  .mini { display: flex; align-items: center; gap: 6px; min-width: 0; font-size: 0.74rem; line-height: 1.35; }
  .mini b { flex: none; font-weight: 700; }
  .mini span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .mini.cancelled { opacity: 0.5; text-decoration: line-through; }
  .mini.action span { font-weight: 700; color: var(--warn); }
  .more { font-size: 0.74rem; font-weight: 700; color: var(--accent); }

  @media (max-width: 699px) {
    .cell { min-height: 78px; padding: 5px; }
    .mini { display: none; }
    .mini:first-of-type { display: flex; }
    .seg button { padding: 0 12px; }
  }
</style>
