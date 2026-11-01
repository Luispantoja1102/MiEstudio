<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { WEEKDAYS } from '$lib/time.js';

  let { member, branches, onsaved } = $props();

  // orden de la semana de lunes a domingo (0 = domingo en la API)
  const order = [1, 2, 3, 4, 5, 6, 0];
  const firstBranch = branches[0]?.id;

  let days = $state(Object.fromEntries(order.map((d) => [d, { on: false, blocks: [] }])));
  let loading = $state(true);
  let busy = $state(false);
  let error = $state('');
  let saved = $state(false);

  onMount(async () => {
    try {
      for (const h of await api(`/api/team/${member.id}/hours`)) {
        days[h.weekday].on = true;
        days[h.weekday].blocks.push({ start: h.start, end: h.end, branch_id: h.branch_id });
      }
    } catch (e) {
      error = e.message;
    }
    loading = false;
  });

  function toggle(d) {
    days[d].on = !days[d].on;
    if (days[d].on && days[d].blocks.length === 0) days[d].blocks = [{ start: '09:00', end: '18:00', branch_id: firstBranch }];
  }

  async function save() {
    error = '';
    saved = false;
    busy = true;
    try {
      const blocks = order.flatMap((d) => (days[d].on ? days[d].blocks.map((b) => ({ weekday: d, ...b })) : []));
      await api(`/api/team/${member.id}/hours`, { method: 'PUT', body: { blocks } });
      saved = true;
      onsaved?.();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }
</script>

<div class="hours">
  <h4>Horario semanal de {member.name}</h4>
  {#if loading}
    <p class="meta">Cargando…</p>
  {:else}
    <div class="week">
      {#each order as d (d)}
        <div class="day" class:rest={!days[d].on}>
          <label class="check day-name"><input type="checkbox" checked={days[d].on} onchange={() => toggle(d)} />{WEEKDAYS[d]}</label>
          <div class="blocks">
            {#if !days[d].on}
              <span class="meta">No trabaja</span>
            {:else}
              {#each days[d].blocks as b, i}
                <div class="block">
                  <input type="time" bind:value={b.start} aria-label="{WEEKDAYS[d]} desde" />
                  <span class="to">a</span>
                  <input type="time" bind:value={b.end} aria-label="{WEEKDAYS[d]} hasta" />
                  {#if branches.length > 1}
                    <select bind:value={b.branch_id} aria-label="Sucursal {WEEKDAYS[d]}">{#each branches as br (br.id)}<option value={br.id}>{br.name}</option>{/each}</select>
                  {/if}
                  {#if days[d].blocks.length > 1}
                    <button class="btn ghost sm" onclick={() => days[d].blocks.splice(i, 1)} aria-label="Quitar tramo de {WEEKDAYS[d]}">Quitar</button>
                  {/if}
                </div>
              {/each}
              <button class="btn ghost sm" onclick={() => days[d].blocks.push({ start: '16:00', end: '20:00', branch_id: firstBranch })}>+ Tramo</button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="form-actions">
      <button class="btn" disabled={busy} onclick={save}>{busy ? 'Guardando…' : 'Guardar horario'}</button>
      {#if saved}<span class="meta" role="status" style="align-self: center">Guardado ✓</span>{/if}
    </div>
  {/if}
</div>

<style>
  .hours { margin-top: 16px; padding: 20px; background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); }
  h4 { margin: 0 0 8px; font-size: 1rem; }
  .week { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 0 16px; margin: 12px 0; }
  .day { display: grid; grid-template-columns: 140px 1fr; gap: 8px 16px; align-items: center; padding: 8px 0; border-bottom: 1px solid var(--border); }
  .day:last-child { border-bottom: 0; }
  .blocks { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; min-height: 44px; }
  .block { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
  .block input, .block select { width: auto; margin: 0; min-height: 44px; }
  .to { color: var(--muted); }
  .rest .day-name { color: var(--muted); }
  @media (max-width: 520px) { .block { display: grid; grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr); width: 100%; } .block select, .block .btn { grid-column: 1 / -1; width: 100%; } .block input { width: 100%; min-width: 0; } .day { grid-template-columns: 1fr; gap: 0; } .hours { padding: 14px; } }
</style>
