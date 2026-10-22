<script>
  import Icon from '$lib/Icon.svelte';
  import { addDays } from '$lib/time.js';
  import { monthStart, weekStart } from '$lib/agenda.js';

  // Calendario mensual de la página pública: solo se pueden elegir los días con horario libre.
  // available: lista de fechas con lugar (null mientras carga). min/max: primer y último día que se pueden reservar.
  let { value, min, max, available = null, onpick, onmonth } = $props();

  let month = $state(value ? value.slice(0, 7) : min.slice(0, 7));
  $effect(() => onmonth?.(month));

  const first = $derived(`${month}-01`);
  // solo las semanas que tiene el mes
  const days = $derived.by(() => {
    const start = weekStart(first);
    const inMonth = new Date(Date.UTC(+month.slice(0, 4), +month.slice(5), 0)).getUTCDate();
    const lead = (new Date(first + 'T12:00:00Z').getUTCDay() + 6) % 7;
    return Array.from({ length: Math.ceil((lead + inMonth) / 7) * 7 }, (_, i) => addDays(start, i));
  });
  const cap = (t) => t.charAt(0).toUpperCase() + t.slice(1);
  const title = $derived(cap(new Date(first + 'T12:00:00').toLocaleDateString('es-MX', { month: 'long', year: 'numeric' })));
  const canPrev = $derived(monthStart(min).slice(0, 7) < month);
  const canNext = $derived(month < max.slice(0, 7));
  const step = (n) => {
    const [y, m] = month.split('-').map(Number);
    const t = new Date(Date.UTC(y, m - 1 + n, 1));
    month = `${t.getUTCFullYear()}-${String(t.getUTCMonth() + 1).padStart(2, '0')}`;
  };
  const ok = (d) => d >= min && d <= max && d.slice(0, 7) === month && !!available?.includes(d);
</script>

<div class="cal" aria-busy={available === null}>
  <div class="head">
    <button type="button" class="nav" onclick={() => step(-1)} disabled={!canPrev} aria-label="Mes anterior"><span class="flip"><Icon name="arrow" size={16} /></span></button>
    <b>{title}</b>
    <button type="button" class="nav" onclick={() => step(1)} disabled={!canNext} aria-label="Mes siguiente"><Icon name="arrow" size={16} /></button>
  </div>
  <div class="grid" role="grid" aria-label={title}>
    {#each ['L', 'M', 'M', 'J', 'V', 'S', 'D'] as n, i (i)}<span class="dow" aria-hidden="true">{n}</span>{/each}
    {#each days as d (d)}
      {#if d.slice(0, 7) !== month}
        <span class="pad"></span>
      {:else}
        <button type="button" class="day" class:sel={d === value} class:free={ok(d)} disabled={!ok(d)} aria-pressed={d === value}
          aria-label="{new Date(d + 'T12:00:00').toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' })}{ok(d) ? '' : ': sin horarios'}" onclick={() => onpick(d)}>
          {Number(d.slice(8))}
        </button>
      {/if}
    {/each}
  </div>
  <p class="key"><i class="k free"></i>Con horarios <i class="k off"></i>Sin horarios</p>
</div>

<style>
  .cal { font-variant-numeric: tabular-nums lining-nums; font-family: var(--font-body, inherit); }
  .head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.75rem; }
  .head b { font-size: 1rem; }
  .nav { display: grid; place-items: center; width: 40px; height: 40px; color: var(--text); background: var(--surface); border: 1px solid var(--border); border-radius: 999px; cursor: pointer; transition: background-color 0.18s var(--ease), transform 0.12s var(--ease); }
  .nav:hover:not(:disabled) { background: var(--surface-2); }
  .nav:active:not(:disabled) { transform: scale(0.94); }
  .nav:disabled { opacity: 0.35; cursor: default; }
  .flip { display: inline-flex; transform: rotate(180deg); }
  .grid { display: grid; grid-template-columns: repeat(7, 1fr); gap: 4px; }
  .dow { padding: 4px 0 6px; text-align: center; font-size: 0.72rem; font-weight: 600; letter-spacing: 0.08em; color: var(--muted); }
  .pad { aspect-ratio: 1; }
  .day { aspect-ratio: 1; display: grid; place-items: center; font: inherit; font-size: 0.95rem; font-weight: 500; color: var(--muted); background: transparent; border: 1px solid transparent; border-radius: 12px; cursor: default; }
  .day.free { color: var(--text); font-weight: 600; background: var(--surface); border-color: var(--border); cursor: pointer; transition: background-color 0.18s var(--ease), border-color 0.18s var(--ease), transform 0.12s var(--ease); }
  .day.free:hover { border-color: var(--accent); background: var(--accent-soft); }
  .day.free:active { transform: scale(0.95); }
  .day.sel { color: var(--accent-contrast); background: var(--accent); border-color: var(--accent); }
  .day:disabled:not(.free) { opacity: 0.4; }
  .key { display: flex; align-items: center; gap: 8px; margin: 0.75rem 0 0; font-size: 0.78rem; color: var(--muted); }
  .k { display: inline-block; width: 12px; height: 12px; border-radius: 4px; }
  .k.free { background: var(--surface); border: 1px solid var(--accent); }
  .k.off { background: var(--surface-2); margin-left: 8px; }
</style>
