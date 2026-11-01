<script>
  // Tabla HTML real. columns: [{ key, label, numeric?, align? }]; rowKey: nombre de la propiedad o función; cell({ row, column }) pinta una celda a medida.
  // Las filas no son botones: las acciones van como botones o enlaces dentro de una celda.
  // stack: en pantallas angostas cada fila se muestra como ficha «etiqueta: valor» (sin desplazamiento horizontal); stack={false} la deja como tabla desplazable.
  let { caption, columns, rows, rowKey = 'id', cell, stack = true } = $props();
  const keyOf = (row, i) => (typeof rowKey === 'function' ? rowKey(row) : (row[rowKey] ?? i));
</script>

<div class="wrap" class:stack>
  <table>
    <caption class="sr-only">{caption}</caption>
    <thead>
      <tr>{#each columns as c (c.key)}<th scope="col" class:num={c.numeric}>{c.label}</th>{/each}</tr>
    </thead>
    <tbody>
      {#each rows as row, i (keyOf(row, i))}
        <tr>
          {#each columns as c (c.key)}
            <td class:num={c.numeric} data-label={c.label}>{#if cell}{@render cell({ row, column: c })}{:else}{row[c.key] ?? ''}{/if}</td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .wrap { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; font-size: 0.925rem; }
  th { padding: 12px 20px; text-align: left; font-size: 0.8rem; font-weight: 600; color: var(--muted); border-bottom: 1px solid var(--border); white-space: nowrap; }
  td { padding: 12px 20px; min-height: 52px; vertical-align: middle; border-bottom: 1px solid var(--border); }
  tbody tr:last-child td { border-bottom: 0; }
  tbody tr:nth-child(even) { background: color-mix(in srgb, var(--surface-2) 45%, var(--surface)); }
  .num { text-align: right; font-variant-numeric: tabular-nums lining-nums; }

  @media (max-width: 640px) {
    .stack table, .stack tbody, .stack tr, .stack td { display: block; }
    .stack thead { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
    .stack tbody tr { padding: 12px 16px; border-bottom: 1px solid var(--border); }
    .stack tbody tr:last-child { border-bottom: 0; }
    .stack td { display: flex; align-items: baseline; justify-content: space-between; gap: 16px; padding: 4px 0; border: 0; text-align: right; min-height: 0; }
    .stack td::before { content: attr(data-label); flex: none; color: var(--muted); font-size: 0.8rem; font-weight: 600; text-align: left; }
    /* la primera columna es el nombre de la fila: va como título, sin etiqueta */
    .stack td:first-child { display: block; padding-bottom: 6px; text-align: left; font-weight: 600; }
    .stack td:first-child::before { display: none; }
  }
</style>
