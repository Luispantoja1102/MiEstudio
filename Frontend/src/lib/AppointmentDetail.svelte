<script>
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { money } from '$lib/format.js';
  import { fmtTime, longDate } from '$lib/time.js';
  import SlotPicker from '$lib/SlotPicker.svelte';
  import TransferPanel from '$lib/TransferPanel.svelte';
  import { session } from '$lib/session.svelte.js';

  let { appt, tz, date, canBook, ondone } = $props();

  const LABEL = { scheduled: 'Agendada', completed: 'Completada', no_show: 'No asistió', cancelled: 'Cancelada', pending_payment: 'Esperando anticipo' };
  const TONE = { scheduled: '', completed: '', no_show: 'warn', cancelled: 'muted', pending_payment: 'warn' };
  const DEPOSIT = { pending: 'sin pagar', paid: 'pagado', applied: 'aplicado en la venta', refunded: 'devuelto', kept: 'conservado por inasistencia', expired: 'vencido' };
  const isAdmin = session.me?.role === 'admin';
  // quien cobra (administración, caja, recepción) completa la cita cobrándola; el personal solo la marca atendida
  const sells = ['admin', 'cashier', 'reception'].includes(session.me?.role);
  const dayOf = (d) => new Date(d).toLocaleDateString('en-CA', { timeZone: tz });
  // «ya llegó» solo se marca el día de la cita y desde una hora antes (no días antes)
  const canArrive = $derived(dayOf(appt.starts_at) === dayOf(Date.now()) && Date.now() >= new Date(appt.starts_at).getTime() - 60 * 60000);
  const canView = ['admin', 'reception'].includes(session.me?.role);
  const hasPaidDeposit = $derived(appt.deposit_status === 'paid');

  let error = $state('');
  let busy = $state(false);
  let moving = $state(false);
  let moveDate = $state(date);
  let picked = $state(null);
  let notes = $state(appt.notes);

  async function run(fn) {
    error = '';
    busy = true;
    try {
      await fn();
      ondone();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  const mark = (what, on) => run(() => api(`/api/appointments/${appt.id}/${what}`, { method: 'POST', body: { on } }));
  // Completar y cobrar: abre la caja si estaba cerrada y lleva la cita al ticket
  async function charge() {
    error = '';
    busy = true;
    try {
      const cur = await api(`/api/cash/current?branch_id=${appt.branch_id}`);
      if (!cur.session) await api('/api/cash/open', { method: 'POST', body: { branch_id: appt.branch_id, opening_cents: 0 } });
      ondone();
      goto(`/panel/ventas?cita=${appt.id}`);
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }
  const setStatus = (status) => run(() => api(`/api/appointments/${appt.id}`, { method: 'PATCH', body: { status } }));
  const saveNotes = () => run(() => api(`/api/appointments/${appt.id}`, { method: 'PATCH', body: { notes } }));
  const move = () =>
    run(() => api(`/api/appointments/${appt.id}/move`, { method: 'POST', body: { starts_at: picked.startsAt, staff_id: picked.staffId } }));
</script>

<div class="stack">
  <div>
    <h3>{appt.client_name}</h3>
    <p class="meta">
      {appt.service_name} · {money(appt.price_cents)}<br />
      {longDate(date)} · {fmtTime(appt.starts_at, tz)}–{fmtTime(appt.ends_at, tz)} · con {appt.staff_name}
    </p>
    <span class="badge {TONE[appt.status]}">{LABEL[appt.status]}</span>
    {#if appt.status === 'scheduled' && appt.confirmed_at}<span class="badge">Confirmada</span>{/if}
    {#if appt.status === 'scheduled' && appt.arrived_at}<span class="badge">Ya llegó</span>{/if}
    {#if appt.source === 'online'}<span class="badge muted">Reserva en línea</span>{/if}
    {#if appt.deposit_cents}
      <p class="meta" style="margin: 0.6rem 0 0">Anticipo de {money(appt.deposit_cents)}: {DEPOSIT[appt.deposit_status] ?? appt.deposit_status}</p>
    {/if}
    {#if appt.status === 'pending_payment' && appt.deposit_method !== 'transfer'}
      <p class="meta" style="margin: 0.4rem 0 0">El horario está apartado unos minutos mientras el cliente paga. Si no paga, se libera solo.</p>
    {/if}
    {#if appt.refund_due}
      <div class="due">
        <p>El cliente canceló y hay que devolverle {money(appt.deposit_cents)} por transferencia.</p>
        {#if isAdmin}<button class="btn sm" disabled={busy} onclick={() => run(() => api(`/api/appointments/${appt.id}/transfer/refunded`, { method: 'POST' }))}>Ya lo devolví</button>{/if}
      </div>
    {/if}
  </div>

  {#if appt.status === 'pending_payment' && appt.deposit_method === 'transfer'}
    <TransferPanel {appt} canReview={isAdmin} {canView} onchange={ondone} />
  {/if}

  {#if canBook}
    <label class="field" style="margin: 0">
      Notas de la cita
      <input bind:value={notes} maxlength="1000" />
    </label>
    {#if notes !== appt.notes}<button class="btn ghost sm" style="justify-self: start" disabled={busy} onclick={saveNotes}>Guardar notas</button>{/if}
  {:else if appt.notes}
    <p>{appt.notes}</p>
  {/if}

  {#if canBook && appt.status === 'scheduled'}
    {#if moving}
      <div>
        <label class="field" style="margin: 0 0 0.75rem">
          Nueva fecha
          <input type="date" bind:value={moveDate} onchange={() => (picked = null)} />
        </label>
        <SlotPicker branchId={appt.branch_id} serviceId={appt.service_id} date={moveDate} {tz} selected={picked} onpick={(p) => (picked = p)} />
        <div class="form-actions">
          <button class="btn" disabled={!picked || busy} onclick={move}>Reprogramar</button>
          <button class="btn ghost" onclick={() => ((moving = false), (picked = null))}>Cancelar</button>
        </div>
      </div>
    {:else}
      <div class="form-actions">
        {#if !appt.arrived_at}<button class="btn ghost" disabled={busy || !canArrive} title={canArrive ? '' : 'Se puede marcar el día de la cita, desde una hora antes'} onclick={() => mark('arrive', true)}>Ya llegó</button>
        {:else}<button class="btn ghost" disabled={busy} onclick={() => mark('arrive', false)}>Quitar «ya llegó»</button>{/if}
        {#if !appt.confirmed_at}<button class="btn ghost" disabled={busy} onclick={() => mark('confirm', true)}>Marcar confirmada</button>{/if}
      </div>
      <div class="form-actions">
        {#if sells}
          <button class="btn" disabled={busy} onclick={charge}>Completar y cobrar</button>
        {:else}
          <button class="btn" disabled={busy} onclick={() => setStatus('completed')}>Marcar completada</button>
        {/if}
        <button class="btn ghost" disabled={busy} onclick={() => (!hasPaidDeposit || confirm(`Si no asistió, el negocio se queda con el anticipo de ${money(appt.deposit_cents)}. ¿Continuar?`)) && setStatus('no_show')}>No asistió</button>
        <button class="btn ghost" disabled={busy} onclick={() => (moving = true)}>Reprogramar</button>
        <button class="btn ghost" disabled={busy} onclick={() => confirm(hasPaidDeposit ? `¿Cancelar esta cita? Se devolverá el anticipo de ${money(appt.deposit_cents)} al cliente.` : '¿Cancelar esta cita?') && setStatus('cancelled')}>Cancelar cita</button>
      </div>
    {/if}
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
</div>

<style>
  .stack { display: grid; gap: 1rem; }
  h3 { font-size: 1.3rem; margin-bottom: 0.2rem; }
  .meta { margin-bottom: 0.6rem; }
  .due { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-top: 0.7rem; padding: 0.7rem 0.9rem; border-radius: var(--radius-sm); background: var(--warn-soft); color: var(--warn); }
  .due p { margin: 0; font-size: 0.9rem; font-weight: 500; }
</style>
