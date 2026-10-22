<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { longDate } from '$lib/time.js';
  import SlotPicker from '$lib/SlotPicker.svelte';
  import TransferPanel from '$lib/TransferPanel.svelte';
  import { money, toCents, toPesos } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';

  let { branchId, date: initialDate, tz, ondone } = $props();

  let services = $state([]);
  let query = $state('');
  let results = $state([]);
  let client = $state(null);
  let quick = $state(false);
  let quickForm = $state({ name: '', phone: '' });
  let serviceId = $state('');
  let date = $state(initialDate);
  let staffId = $state('');
  let staffOptions = $state([]);
  let picked = $state(null);
  let notes = $state('');
  let error = $state('');
  let busy = $state(false);
  let timer;
  // anticipo por transferencia: se aparta el horario hasta que llegue el comprobante
  let transfer = $state(null);
  let wantDeposit = $state(false);
  let depositPesos = $state('');
  let created = $state(null); // cita apartada: se muestran los datos para pagar
  const isAdmin = session.me?.role === 'admin';

  onMount(async () => {
    services = (await api('/api/services')).filter((s) => s.active);
    staffOptions = (await api('/api/agenda?' + new URLSearchParams({ branch_id: branchId, date }))).staff;
    try {
      const t = await api('/api/transfer-info');
      transfer = t.enabled ? t : null;
    } catch {}
  });

  // al elegir servicio se propone el porcentaje de anticipo del negocio (o la mitad si no hay)
  $effect(() => {
    if (service && wantDeposit && !depositPesos) depositPesos = toPesos(Math.max(100, Math.round((service.price_cents * (transfer?.deposit_percent || 50)) / 100)));
  });

  const service = $derived(services.find((s) => String(s.id) === String(serviceId)));
  // si el servicio tiene personas asignadas, solo ellas
  const eligible = $derived(service?.staff_ids?.length ? staffOptions.filter((s) => service.staff_ids.includes(s.id)) : staffOptions);

  function search() {
    clearTimeout(timer);
    timer = setTimeout(async () => {
      results = query.trim() ? (await api('/api/clients?q=' + encodeURIComponent(query.trim()))).slice(0, 6) : [];
    }, 250);
  }

  async function createQuick(e) {
    e.preventDefault();
    error = '';
    try {
      const { id } = await api('/api/clients', { method: 'POST', body: quickForm });
      client = { id, name: quickForm.name };
      quick = false;
    } catch (err) {
      error = err.message;
    }
  }

  async function submit() {
    error = '';
    busy = true;
    try {
      const cents = wantDeposit ? toCents(depositPesos) : 0;
      if (wantDeposit && (Number.isNaN(cents) || cents <= 0)) throw new Error('Escribe el monto del anticipo, por ejemplo 100');
      const r = await api('/api/appointments', {
        method: 'POST',
        body: { client_id: client.id, service_id: Number(serviceId), staff_id: picked.staffId, branch_id: branchId, starts_at: picked.startsAt, notes, ...(wantDeposit ? { deposit_cents: cents } : {}) }
      });
      if (wantDeposit) {
        // la cita quedó apartada: se muestran los datos de pago para que el cliente transfiera
        created = { id: r.id, deposit_cents: cents, client_name: client.name, manage_path: r.manage_path, proof_at: null };
        return;
      }
      ondone();
    } catch (err) {
      error = err.message;
      picked = null;
    } finally {
      busy = false;
    }
  }
</script>

{#if created}
  <div class="stack">
    <p class="okmsg">Cita apartada. Pasa los datos de pago al cliente y, cuando mande el comprobante, adjúntalo aquí o desde la agenda.</p>
    <TransferPanel appt={created} canReview={isAdmin} canView={true} onchange={ondone} />
    <div class="form-actions"><button class="btn ghost" onclick={ondone}>Listo, cerrar</button></div>
  </div>
{:else}
<div class="stack">
  <div>
    <span class="lbl">Cliente</span>
    {#if client}
      <div class="item-row"><span class="grow title">{client.name}</span><button class="btn ghost sm" onclick={() => (client = null)}>Cambiar</button></div>
    {:else if quick}
      <form class="item-row" onsubmit={createQuick} style="gap: 8px">
        <input placeholder="Nombre" bind:value={quickForm.name} required style="flex: 1 1 160px; margin: 0" aria-label="Nombre del cliente" />
        <input placeholder="Teléfono" type="tel" bind:value={quickForm.phone} style="flex: 1 1 130px; margin: 0" aria-label="Teléfono" />
        <button class="btn sm">Crear</button>
        <button type="button" class="btn ghost sm" onclick={() => (quick = false)}>Cancelar</button>
      </form>
    {:else}
      <input type="search" placeholder="Buscar cliente por nombre, teléfono o correo" bind:value={query} oninput={search} aria-label="Buscar cliente" style="margin: 0" />
      {#each results as c (c.id)}
        <button type="button" class="pick" onclick={() => ((client = c), (results = []))}>{c.name}<small>{c.phone}</small></button>
      {/each}
      <button type="button" class="link" onclick={() => (quick = true)}>+ Cliente nuevo</button>
    {/if}
  </div>

  <div class="form-grid">
    <label class="field" style="margin: 0">
      Servicio
      <select bind:value={serviceId} onchange={() => ((staffId = ''), (picked = null))}>
        <option value="">Elige un servicio</option>
        {#each services as s (s.id)}<option value={s.id}>{s.name} · {s.duration_min} min</option>{/each}
      </select>
    </label>
    <label class="field" style="margin: 0">
      Fecha
      <input type="date" bind:value={date} onchange={() => (picked = null)} required />
    </label>
    <label class="field" style="margin: 0">
      Profesional
      <select bind:value={staffId} onchange={() => (picked = null)}>
        <option value="">Cualquiera</option>
        {#each eligible as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
      </select>
    </label>
  </div>

  {#if serviceId}
    <div>
      <span class="lbl">Horarios disponibles · {longDate(date)}</span>
      <SlotPicker {branchId} {serviceId} {date} {staffId} {tz} selected={picked} onpick={(p) => (picked = p)} />
    </div>
  {/if}

  <label class="field" style="margin: 0">
    Notas (opcional)
    <input bind:value={notes} maxlength="1000" placeholder="Ej. trae foto de referencia" />
  </label>

  {#if transfer && service?.price_cents}
    <div class="dep">
      <label class="check"><input type="checkbox" bind:checked={wantDeposit} />Pedir anticipo por transferencia</label>
      {#if wantDeposit}
        <label class="field" style="margin: 0.6rem 0 0">Monto del anticipo (pesos)
          <input inputmode="decimal" bind:value={depositPesos} />
          <span class="hint">Servicio de {money(service.price_cents)}. El horario queda apartado {transfer.hold_hours} h mientras llega el comprobante.</span>
        </label>
      {/if}
    </div>
  {/if}

  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="form-actions">
    <button class="btn" disabled={!client || !picked || busy} onclick={submit}>{busy ? 'Agendando…' : wantDeposit ? 'Apartar y pedir anticipo' : 'Agendar cita'}</button>
  </div>
</div>
{/if}

<style>
  .stack { display: grid; gap: 1.1rem; }
  .lbl { display: block; margin-bottom: 0.4rem; font-size: 0.9rem; font-weight: 500; }
  .pick { display: flex; justify-content: space-between; width: 100%; min-height: 44px; align-items: center; padding: 0 0.9rem; margin-top: 4px; font: inherit; color: var(--text); background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; }
  .pick:hover { border-color: var(--accent); }
  .pick small { color: var(--muted); }
  .dep { padding: 0.9rem 1rem; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface-2); }
  .okmsg { margin: 0; padding: 0.7rem 0.9rem; border-radius: var(--radius-sm); background: var(--accent-soft); font-size: 0.9rem; }
  .link { margin-top: 6px; padding: 0; font: inherit; font-size: 0.9rem; color: var(--accent); background: none; border: 0; cursor: pointer; text-align: left; }
</style>
