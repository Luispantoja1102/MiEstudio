<script>
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { money } from '$lib/format.js';
  import { fmtTime, longDate } from '$lib/time.js';
  import SlotPicker from '$lib/SlotPicker.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import TransferPanel from '$lib/TransferPanel.svelte';
  import { hasPos, session } from '$lib/session.svelte.js';

  let { appt, tz, date, canBook, ondone } = $props();

  const LABEL = { scheduled: 'Agendada', completed: 'Completada', no_show: 'No asistió', cancelled: 'Cancelada', pending_payment: 'Esperando anticipo' };
  const TONE = { scheduled: 'info', completed: 'success', no_show: 'warning', cancelled: 'neutral', pending_payment: 'warning' };
  const DEPOSIT = { pending: 'sin pagar', paid: 'pagado', applied: 'aplicado en la venta', refunded: 'devuelto', kept: 'conservado por inasistencia', expired: 'vencido' };
  const isAdmin = session.me?.role === 'admin';
  // quien cobra (administración, caja, recepción) completa la cita cobrándola; el personal solo la marca atendida
  const sells = ['admin', 'cashier', 'reception'].includes(session.me?.role) && hasPos();
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
    <div class="tags">
      <StatusBadge label={LABEL[appt.status]} tone={TONE[appt.status]} />
      {#if appt.status === 'scheduled' && appt.confirmed_at}<StatusBadge label="Confirmada" tone="success" />{/if}
      {#if appt.status === 'scheduled' && appt.arrived_at}<StatusBadge label="Ya llegó" tone="success" />{/if}
      {#if appt.source === 'online'}<StatusBadge label="Reserva en línea" />{/if}
    </div>
    <dl>
      <div><dt>Servicio</dt><dd>{appt.service_name} · {money(appt.price_cents)}</dd></div>
      <div><dt>Profesional</dt><dd>{appt.staff_name}</dd></div>
      <div><dt>Día</dt><dd>{longDate(date)}</dd></div>
      <div><dt>Horario</dt><dd>{fmtTime(appt.starts_at, tz)}–{fmtTime(appt.ends_at, tz)}</dd></div>
      {#if appt.deposit_cents}<div><dt>Anticipo</dt><dd>{money(appt.deposit_cents)} · {DEPOSIT[appt.deposit_status] ?? appt.deposit_status}</dd></div>{/if}
    </dl>
    {#if appt.status === 'pending_payment' && appt.deposit_method !== 'transfer'}
      <p class="meta">El horario está apartado unos minutos mientras el cliente paga. Si no paga, se libera solo.</p>
    {/if}
    {#if appt.refund_due}
      <InlineNotice tone="warning">
        El cliente canceló y hay que devolverle {money(appt.deposit_cents)} por transferencia.
        {#snippet actions()}
          {#if isAdmin}<button class="btn sm" disabled={busy} onclick={() => run(() => api(`/api/appointments/${appt.id}/transfer/refunded`, { method: 'POST' }))}>Ya lo devolví</button>{/if}
        {/snippet}
      </InlineNotice>
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
  {#if error}<InlineNotice tone="danger" message={error} />{/if}
</div>

<style>
  .stack { display: grid; gap: 1.1rem; }
  h3 { font-size: 1.35rem; margin-bottom: 0.5rem; overflow-wrap: anywhere; }
  .tags { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 1rem; }
  dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 12px 20px; margin: 0; padding: 14px 0; border-top: 1px solid var(--border); border-bottom: 1px solid var(--border); font-variant-numeric: tabular-nums lining-nums; }
  dt { font-size: 0.75rem; font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; color: var(--muted); }
  dd { margin: 2px 0 0; font-weight: 600; overflow-wrap: anywhere; }
  .meta { margin: 0.7rem 0 0; color: var(--muted); font-size: 0.9rem; }
  /* la acción principal va primero; cancelar la cita queda al final, sin color de alarma pero separada */
  .stack :global(.form-actions) { margin-top: 0; }
</style>
