<script>
  let { value = $bindable(''), label = 'Buscar', placeholder = 'Buscar', disabled = false, oninput } = $props();
</script>

<label class="search">
  <span class="sr-only">{label}</span>
  <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
  <input type="search" bind:value {placeholder} {disabled} {oninput} />
</label>

<style>
  .search { position: relative; display: block; min-width: min(100%, 260px); }
  svg { position: absolute; left: 12px; top: 50%; transform: translateY(-50%); color: var(--muted); pointer-events: none; }
  .search :global(input) { padding-left: 38px; }
</style>
