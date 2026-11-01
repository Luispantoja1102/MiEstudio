<script>
  // Barras simples con datos reales. items: [{ id, label, value, formattedValue }]; orientation: 'vertical' | 'horizontal'.
  // La barra seleccionada (selectedId) va en el acento; las demás, en un tono suave. No dibuja nada si no hay datos.
  let { label, items, orientation = 'vertical', selectedId = null } = $props();
  const max = $derived(Math.max(1, ...items.map((i) => i.value)));
</script>

{#if items.length}
  <ul class="chart {orientation}" aria-label={label}>
    {#each items as it (it.id)}
      <li class:sel={it.id === selectedId} style="--v: {Math.max(2, (it.value / max) * 100)}%">
        <span class="val">{it.formattedValue ?? it.value}</span>
        <span class="bar" aria-hidden="true"></span>
        <span class="lbl">{it.label}</span>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .chart { list-style: none; margin: 0; padding: 0; display: flex; }
  li { display: grid; min-width: 0; font-size: 0.8rem; }
  .bar { background: color-mix(in srgb, var(--accent) 24%, var(--surface)); border-radius: var(--radius-xs); }
  .sel .bar { background: var(--accent); }
  .val { font-weight: 600; font-variant-numeric: tabular-nums lining-nums; }
  .lbl { color: var(--muted); }

  .vertical { align-items: end; gap: 8px; height: 220px; }
  .vertical li { flex: 1; grid-template-rows: auto 1fr auto; justify-items: center; gap: 4px; height: 100%; }
  .vertical .bar { width: 100%; max-width: 56px; align-self: end; height: var(--v); min-height: 4px; }
  .vertical .val { align-self: end; }

  .horizontal { flex-direction: column; gap: 10px; }
  .horizontal li { grid-template-columns: minmax(80px, 160px) 1fr auto; align-items: center; gap: 12px; }
  .horizontal .lbl { order: 1; color: var(--text); }
  .horizontal .bar { order: 2; width: var(--v); height: 12px; }
  .horizontal .val { order: 3; }
</style>
