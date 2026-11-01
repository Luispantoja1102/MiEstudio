<script>
  // Opciones excluyentes en píldora (vista Día/Semana/Mes, periodos, categorías). options: [{ value, label, disabled? }]
  let { value = $bindable(''), options, label = 'Opciones', onchange } = $props();
  const pick = (o) => {
    if (o.disabled || o.value === value) return;
    value = o.value;
    onchange?.(o.value);
  };
</script>

<div class="seg" role="group" aria-label={label}>
  {#each options as o (o.value)}
    <button type="button" class:on={o.value === value} aria-pressed={o.value === value} disabled={o.disabled} onclick={() => pick(o)}>{o.label}</button>
  {/each}
</div>

<style>
  .seg { display: inline-flex; gap: 4px; padding: 4px; max-width: 100%; overflow-x: auto; background: var(--surface-2); border-radius: var(--radius); }
  button { min-height: 44px; padding: 0 16px; border: 0; border-radius: var(--radius-sm); background: transparent; color: var(--muted); font: inherit; font-size: 0.9rem; font-weight: 500; white-space: nowrap; cursor: pointer; transition: background-color var(--dur-ui) var(--ease), color var(--dur-ui) var(--ease); }
  button:hover:not(:disabled):not(.on) { color: var(--text); }
  button.on { background: var(--accent); color: var(--accent-contrast); font-weight: 600; }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
