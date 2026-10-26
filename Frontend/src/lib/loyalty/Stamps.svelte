<script>
  import Icon from '$lib/Icon.svelte';

  // Los sellos de una tarjeta de lealtad: un círculo por visita que pide el premio; los ganados van llenos.
  let { required, stamps, ready = false } = $props();
  const filled = $derived(Math.min(stamps, required));
</script>

<ol class="stamps" class:ready aria-label={`${filled} de ${required} sellos`}>
  {#each Array.from({ length: required }, (_, i) => i) as i}
    <li class:on={i < filled}>{#if i < filled}<Icon name="check" size={18} />{:else}<span>{i + 1}</span>{/if}</li>
  {/each}
</ol>

<style>
  .stamps { display: grid; grid-template-columns: repeat(auto-fill, minmax(44px, 1fr)); gap: 10px; list-style: none; margin: 0; padding: 0; }
  li { display: grid; place-items: center; aspect-ratio: 1; max-width: 56px; border: 2px dashed var(--border); border-radius: 50%; color: var(--muted); font-size: 0.85rem; font-variant-numeric: tabular-nums; }
  li.on { border: 2px solid var(--accent); background: var(--accent); color: var(--accent-contrast); }
  .ready li.on { box-shadow: 0 0 0 3px var(--accent-soft); }
</style>
