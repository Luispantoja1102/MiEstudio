<script>
  import { api } from '$lib/api.js';
  import { money } from '$lib/format.js';
  import { fmtTime, longDate } from '$lib/time.js';
  import SlotPicker from '$lib/SlotPicker.svelte';

  let { appt, tz, date, canBook, ondone } = $props();

  const LABEL = { scheduled: 'Agendada', completed: 'Completada', no_show: 'No asistió', cancelled: 'Cancelada', pending_payment: 'Esperando anticipo' };
  const TONE = { scheduled: '', completed: '', no_show: 'warn', cancelled: 'muted', pending_payment: 'warn' };
  const DEPOSIT = { pending: 'sin pagar', paid: 'pagado', applied: 'aplicado en la venta', refunded: 'devuelto', kept: 'conservado por inasistencia', expired: 'vencido' };
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
    {#if appt.source === 'online'}<span class="badge muted">Reserva en línea</span>{/if}
    {#if appt.deposit_cents}
      <p class="meta" style="margin: 0.6rem 0 0">Anticipo de {money(appt.deposit_cents)}: {DEPOSIT[appt.deposit_status] ?? appt.deposit_status}</p>
    {/if}
    {#if appt.status === 'pending_payment'}
      <p class="meta" style="margin: 0.4rem 0 0">El horario está apartado unos minutos mientras el cliente paga. Si no paga, se libera solo.</p>
    {/if}
  </div>

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
        <button class="btn" disabled={busy} onclick={() => setStatus('completed')}>Marcar completada</button>
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
</style>
