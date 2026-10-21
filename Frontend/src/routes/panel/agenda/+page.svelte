<script>
  import { onMount } from 'svelte';
  import Icon from '$lib/Icon.svelte';
  import Modal from '$lib/Modal.svelte';
  import NewAppointment from '$lib/NewAppointment.svelte';
  import AppointmentDetail from '$lib/AppointmentDetail.svelte';
  import BlockForm from '$lib/BlockForm.svelte';
  import { api } from '$lib/api.js';
  import { addDays, clockToMin, fmtTime, longDate, minToClock, minutesOfDay, todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';

  const PX = 1.4; // píxeles por minuto
  const tz = session.me.business.timezone || 'America/Tijuana';
  const role = session.me.role;
  const canBook = ['admin', 'reception', 'staff'].includes(role);

  let branches = $state([]);
  let branchId = $state(null);
  let date = $state(todayIn(tz));
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
    try {
      const r = await api('/api/agenda?' + new URLSearchParams({ branch_id: branchId, date }));
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
      branches = (await api('/api/branches')).filter((b) => b.active);
      branchId = branches[0]?.id ?? null;
      await load();
    } catch (e) {
      error = e.message;
    }
  });

  const go = (d) => {
    date = d;
    load();
  };

  // rango visible: 8–20 h, ampliado a lo que haya de horarios y citas
  const range = $derived.by(() => {
    let lo = 8 * 60, hi = 20 * 60;
    for (const s of data?.staff ?? []) for (const h of s.hours) (lo = Math.min(lo, clockToMin(h.start)), (hi = Math.max(hi, clockToMin(h.end))));
    for (const a of data?.appointments ?? []) (lo = Math.min(lo, minutesOfDay(a.starts_at, tz)), (hi = Math.max(hi, minutesOfDay(a.ends_at, tz))));
    return { start: Math.floor(lo / 60) * 60, end: Math.ceil(hi / 60) * 60 };
  });
  const height = $derived((range.end - range.start) * PX);
  const hours = $derived(Array.from({ length: (range.end - range.start) / 60 + 1 }, (_, i) => range.start + i * 60));

  // tramos fuera del horario de trabajo (para sombrearlos)
  function closed(staff) {
    const segs = [...staff.hours].map((h) => [clockToMin(h.start), clockToMin(h.end)]).sort((a, b) => a[0] - b[0]);
    const out = [];
    let cursor = range.start;
    for (const [s, e] of segs) {
      if (s > cursor) out.push([cursor, s]);
      cursor = Math.max(cursor, e);
    }
    if (cursor < range.end) out.push([cursor, range.end]);
    return out;
  }

  const apptsFor = (id) =>
    (data?.appointments ?? []).filter((a) => a.staff_id === id && (showCancelled || a.status !== 'cancelled'));
  const blocksFor = (id) => (data?.blocks ?? []).filter((b) => b.user_id === id);
  const top = (iso) => (minutesOfDay(iso, tz) - range.start) * PX;
  const span = (a, b) => Math.max((minutesOfDay(b, tz) - minutesOfDay(a, tz)) * PX, 22);

  const counts = $derived.by(() => {
    const list = (data?.appointments ?? []).filter((a) => a.status === 'scheduled');
    return list.length;
  });

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

  const done = async () => {
    newOpen = blockOpen = detailOpen = false;
    await load();
  };
</script>

<div class="page-head">
  <div>
    <h1>Agenda</h1>
    <p>{longDate(date)} · {counts} {counts === 1 ? 'cita agendada' : 'citas agendadas'}</p>
  </div>
  {#if canBook && branchId}
    <div class="form-actions" style="margin: 0">
      <button class="btn ghost" onclick={() => (blockOpen = true)}>Bloquear horario</button>
      <button class="btn" onclick={() => (newOpen = true)}>Nueva cita</button>
    </div>
  {/if}
</div>

<div class="toolbar">
  <button class="btn ghost sm icon" onclick={() => go(addDays(date, -1))} aria-label="Día anterior"><span class="flip"><Icon name="arrow" size={16} /></span></button>
  <input type="date" bind:value={date} onchange={load} aria-label="Fecha" style="width: auto; margin: 0" />
  <button class="btn ghost sm icon" onclick={() => go(addDays(date, 1))} aria-label="Día siguiente"><Icon name="arrow" size={16} /></button>
  <button class="btn ghost sm" onclick={() => go(todayIn(tz))}>Hoy</button>
  {#if branches.length > 1}
    <select bind:value={branchId} onchange={load} aria-label="Sucursal" style="width: auto; margin: 0">
      {#each branches as b (b.id)}<option value={b.id}>{b.name}</option>{/each}
    </select>
  {/if}
  <label class="check"><input type="checkbox" bind:checked={showCancelled} />Ver canceladas</label>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if data}
  {#if data.staff.length === 0}
    <div class="empty">
      Aún no hay personas en la agenda.
      {#if role === 'admin'}Ve a <a href="/panel/equipo">Equipo</a>, marca «En agenda» y define su horario.{/if}
    </div>
  {:else}
    <div class="wrap">
      <div class="grid" style="grid-template-columns: 52px repeat({data.staff.length}, minmax(170px, 1fr))">
        <div class="corner"></div>
        {#each data.staff as s (s.id)}<div class="colhead">{s.name}</div>{/each}

        <div class="axis" style="height: {height}px">
          {#each hours as h (h)}<span style="top: {(h - range.start) * PX}px">{minToClock(h)}</span>{/each}
        </div>
        {#each data.staff as s (s.id)}
          <div class="col" style="height: {height}px; --hour: {60 * PX}px">
            {#each closed(s) as [a, b]}
              <div class="closed" style="top: {(a - range.start) * PX}px; height: {(b - a) * PX}px"></div>
            {/each}
            {#if s.hours.length === 0}<span class="off">No trabaja este día</span>{/if}

            {#each blocksFor(s.id) as b (b.id)}
              <button class="block" style="top: {top(b.starts_at)}px; height: {span(b.starts_at, b.ends_at)}px" onclick={() => removeBlock(b)}
                aria-label="Bloqueo {fmtTime(b.starts_at, tz)} a {fmtTime(b.ends_at, tz)}">
                <b>{fmtTime(b.starts_at, tz)}–{fmtTime(b.ends_at, tz)}</b> {b.reason || 'Bloqueado'}
              </button>
            {/each}

            {#each apptsFor(s.id) as a (a.id)}
              <button class="appt {a.status}" class:online={a.source === 'online'} style="top: {top(a.starts_at)}px; height: {span(a.starts_at, a.ends_at)}px" onclick={() => openDetail(a)}>
                <b>{fmtTime(a.starts_at, tz)}</b> {a.client_name}
                <small>{a.service_name}{#if a.deposit_status === 'paid'} · anticipo ✓{:else if a.status === 'pending_payment'} · esperando anticipo{/if}</small>
              </button>
            {/each}
          </div>
        {/each}
      </div>
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
  {#if detail}<AppointmentDetail appt={detail} {tz} {date} {canBook} ondone={done} />{/if}
</Modal>

<style>
  .toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 1rem; }
  .icon { width: 40px; padding: 0; }
  .flip { display: inline-flex; transform: rotate(180deg); }

  .wrap { overflow: auto; max-height: 72vh; border: 1px solid var(--border); border-radius: var(--radius); background: var(--surface); }
  .grid { display: grid; min-width: min-content; }
  .corner, .colhead { position: sticky; top: 0; z-index: 3; background: var(--surface); border-bottom: 1px solid var(--border); }
  .corner { left: 0; z-index: 4; }
  .colhead { padding: 0.7rem 0.5rem; text-align: center; font-weight: 600; border-left: 1px solid var(--border); }

  .axis { position: sticky; left: 0; z-index: 2; background: var(--surface); }
  .axis span { position: absolute; right: 6px; transform: translateY(-50%); font-size: 0.75rem; color: var(--muted); font-variant-numeric: tabular-nums; }
  .axis span:first-child { transform: none; }

  .col {
    position: relative;
    border-left: 1px solid var(--border);
    background-image: repeating-linear-gradient(to bottom, transparent 0, transparent calc(var(--hour) - 1px), var(--border) calc(var(--hour) - 1px), var(--border) var(--hour));
  }
  .closed { position: absolute; left: 0; right: 0; background: var(--surface-2); opacity: 0.85; pointer-events: none; }
  .off { position: absolute; top: 8px; left: 0; right: 0; text-align: center; font-size: 0.8rem; color: var(--muted); }

  .appt, .block {
    position: absolute;
    left: 4px;
    right: 4px;
    z-index: 1;
    overflow: hidden;
    padding: 5px 8px;
    font: inherit;
    font-size: 0.8rem;
    line-height: 1.25;
    text-align: left;
    color: var(--text);
    border-radius: 6px;
    cursor: pointer;
  }
  .appt { background: var(--accent-soft); border: 1px solid var(--accent); transition: filter var(--dur-ui) var(--ease), transform var(--dur-press) var(--ease); }
  .appt small { display: block; font-size: 0.75rem; color: var(--muted); }
  .appt.completed { background: var(--surface-2); border-color: var(--muted); }
  .appt.no_show { background: var(--warn-soft); border-color: var(--warn); }
  .appt.pending_payment { background: var(--warn-soft); border-color: var(--warn); border-style: dashed; }
  .appt.online { box-shadow: inset 0 0 0 1px var(--accent); } /* reservada en línea: borde doble */
  .appt.cancelled { opacity: 0.55; text-decoration: line-through; background: var(--surface); border-color: var(--border); }
  .appt:hover { filter: brightness(0.97); }
  .appt:active { transform: scale(0.99); }
  .block {
    color: var(--muted);
    border: 1px dashed var(--muted);
    background: repeating-linear-gradient(45deg, transparent 0 6px, var(--surface-2) 6px 12px);
  }
</style>
