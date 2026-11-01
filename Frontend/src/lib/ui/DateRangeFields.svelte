<script>
  let { from = $bindable(''), to = $bindable(''), disabled = false, onchange } = $props();
</script>

<div class="range">
  <label class="field">Desde<input type="date" bind:value={from} {disabled} onchange={() => onchange?.()} /></label>
  <label class="field">Hasta<input type="date" bind:value={to} {disabled} onchange={() => onchange?.()} /></label>
</div>

<style>
  .range { display: flex; flex-wrap: wrap; gap: 12px; }
  .field { margin: 0; }
</style>
