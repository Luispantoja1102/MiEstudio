<script>
  // Tabla HTML real. columns: [{ key, label, numeric?, align? }]; rowKey: nombre de la propiedad o función; cell({ row, column }) pinta una celda a medida.
  // Las filas no son botones: las acciones van como botones o enlaces dentro de una celda.
  let { caption, columns, rows, rowKey = 'id', cell, onrowclick } = $props();
  const keyOf = (row, i) => (typeof rowKey === 'function' ? rowKey(row) : (row[rowKey] ?? i));
</script>

<div class="wrap">
  <table>
    <caption class="sr-only">{caption}</caption>
    <thead>
      <tr>{#each columns as c (c.key)}<th scope="col" class:num={c.numeric}>{c.label}</th>{/each}</tr>
    </thead>
    <tbody>
      {#each rows as row, i (keyOf(row, i))}
        <tr>
          {#each columns as c (c.key)}
            <td class:num={c.numeric}>{#if cell}{@render cell({ row, column: c })}{:else}{row[c.key] ?? ''}{/if}</td>
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
</style>
