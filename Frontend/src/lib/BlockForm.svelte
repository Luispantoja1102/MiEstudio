<script>
  import { api } from '$lib/api.js';
  import { zonedToDate } from '$lib/time.js';

  let { staff, date, tz, selfId = null, ondone } = $props();

  let userId = $state(selfId ?? staff[0]?.id ?? '');
  let start = $state('13:00');
  let end = $state('14:00');
  let reason = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      await api('/api/blocks', {
        method: 'POST',
        body: {
          user_id: Number(userId),
          starts_at: zonedToDate(date, start, tz).toISOString(),
          ends_at: zonedToDate(date, end, tz).toISOString(),
          reason
        }
      });
      ondone();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={submit}>
  <div class="form-grid">
    <label class="field">
      Persona
      <select bind:value={userId} disabled={selfId !== null}>
        {#each staff as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
      </select>
    </label>
    <label class="field">Desde <input type="time" bind:value={start} required /></label>
    <label class="field">Hasta <input type="time" bind:value={end} required /></label>
  </div>
  <label class="field">Motivo (opcional) <input bind:value={reason} maxlength="200" placeholder="Comida, trámite, descanso…" /></label>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Bloquear horario'}</button>
</form>
