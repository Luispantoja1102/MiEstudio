<script>
  import Icon from '$lib/Icon.svelte';
  // Cifra grande + etiqueta chica. Con href es un enlace completo. El ícono es opcional y sin círculo de color.
  let { label, value, hint = '', icon = '', href = '' } = $props();
</script>

<svelte:element this={href ? 'a' : 'div'} class="metric" class:link={!!href} {href}>
  <span class="top"><span class="label">{label}</span>{#if icon}<Icon name={icon} size={18} />{/if}</span>
  <strong class="value">{value}</strong>
  {#if hint}<span class="hint">{hint}</span>{/if}
</svelte:element>

<style>
  .metric { display: grid; gap: 6px; align-content: start; min-width: 0; padding: 20px; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); color: var(--text); text-decoration: none; }
  .link { transition: border-color var(--dur-ui) var(--ease); }
  .link:hover { border-color: var(--control-border); }
  .top { display: flex; align-items: center; justify-content: space-between; gap: 8px; color: var(--muted); }
  .label { font-size: 0.875rem; font-weight: 500; }
  .value { font-size: 2rem; font-weight: 700; line-height: 1.1; letter-spacing: -0.02em; font-variant-numeric: tabular-nums lining-nums; overflow-wrap: anywhere; }
  .hint { font-size: 0.85rem; color: var(--muted); }
</style>
