<script>
  // Fila de filtros y acciones sobre una lista o tabla.
  let { label = 'Filtros', children, actions } = $props();
</script>

<div class="bar" role="group" aria-label={label}>
  <div class="filters">{@render children?.()}</div>
  {#if actions}<div class="acts">{@render actions()}</div>{/if}
</div>

<style>
  .bar { display: flex; flex-wrap: wrap; align-items: end; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
  .filters, .acts { display: flex; flex-wrap: wrap; align-items: end; gap: 12px; min-width: 0; }
  .filters > :global(*) { min-width: 0; }
</style>
