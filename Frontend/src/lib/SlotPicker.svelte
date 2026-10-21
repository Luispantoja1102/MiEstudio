<script>
  import { api } from '$lib/api.js';
  import { fmtTime } from '$lib/time.js';

  // Muestra los horarios libres de un servicio, agrupados por profesional.
  let { branchId, serviceId, date, staffId = '', tz, selected = null, onpick } = $props();

  let staff = $state([]);
  let loading = $state(false);
  let error = $state('');
  let seq = 0;

  $effect(() => {
    const params = new URLSearchParams({ service_id: serviceId, branch_id: branchId, date });
    if (staffId) params.set('staff_id', staffId);
    if (!serviceId || !branchId || !date) {
      staff = [];
      return;
    }
    const mine = ++seq;
    loading = true;
    error = '';
    api('/api/agenda/slots?' + params)
      .then((r) => mine === seq && (staff = r.staff))
      .catch((e) => mine === seq && ((error = e.message), (staff = [])))
      .finally(() => mine === seq && (loading = false));
  });
</script>

<div class="slots" aria-live="polite">
  {#if loading}
    <p class="meta">Buscando horarios…</p>
  {:else if error}
    <p class="error" role="alert">{error}</p>
  {:else if staff.length === 0}
    <p class="meta">No hay horarios disponibles ese día. Prueba con otra fecha o profesional.</p>
  {:else}
    {#each staff as s (s.staff_id)}
      <div class="group">
        <strong>{s.staff_name}</strong>
        <div class="chips">
          {#each s.starts as iso}
            <button
              type="button"
              class="chip"
              aria-pressed={selected?.staffId === s.staff_id && selected?.startsAt === iso}
              onclick={() => onpick({ staffId: s.staff_id, startsAt: iso })}
            >{fmtTime(iso, tz)}</button>
          {/each}
        </div>
      </div>
    {/each}
  {/if}
</div>

<style>
  .group { margin-bottom: 0.9rem; }
  .group strong { display: block; margin-bottom: 0.4rem; font-size: 0.9rem; }
  .chips { margin-bottom: 0; max-height: 150px; overflow-y: auto; }
</style>
