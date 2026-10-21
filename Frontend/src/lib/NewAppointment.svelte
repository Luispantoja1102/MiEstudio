<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { longDate } from '$lib/time.js';
  import SlotPicker from '$lib/SlotPicker.svelte';

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

  onMount(async () => {
    services = (await api('/api/services')).filter((s) => s.active);
    staffOptions = (await api('/api/agenda?' + new URLSearchParams({ branch_id: branchId, date }))).staff;
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
      await api('/api/appointments', {
        method: 'POST',
        body: { client_id: client.id, service_id: Number(serviceId), staff_id: picked.staffId, branch_id: branchId, starts_at: picked.startsAt, notes }
      });
      ondone();
    } catch (err) {
      error = err.message;
      picked = null;
    } finally {
      busy = false;
    }
  }
</script>

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

  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <div class="form-actions">
    <button class="btn" disabled={!client || !picked || busy} onclick={submit}>{busy ? 'Agendando…' : 'Agendar cita'}</button>
  </div>
</div>

<style>
  .stack { display: grid; gap: 1.1rem; }
  .lbl { display: block; margin-bottom: 0.4rem; font-size: 0.9rem; font-weight: 500; }
  .pick { display: flex; justify-content: space-between; width: 100%; min-height: 44px; align-items: center; padding: 0 0.9rem; margin-top: 4px; font: inherit; color: var(--text); background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; }
  .pick:hover { border-color: var(--accent); }
  .pick small { color: var(--muted); }
  .link { margin-top: 6px; padding: 0; font: inherit; font-size: 0.9rem; color: var(--accent); background: none; border: 0; cursor: pointer; text-align: left; }
</style>
