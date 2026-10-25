<script>
  import { onMount } from 'svelte';
  import Privacy from './Privacy.svelte';
  import Terms from './Terms.svelte';
  import { legal, loadLegal } from './legal.svelte.js';

  // El texto legal con su versión y el responsable: lo usan la página completa y la ventana.
  let { which } = $props(); // 'privacidad' | 'terminos'

  onMount(loadLegal);
  // mientras llegan los datos del servidor se muestra el nombre por omisión del operador
  const op = $derived({ name: 'Hyperion', email: 'caresia-info@caresia.mx', ...(legal.info?.operator ?? {}) });
</script>

<div class="legal-text">
  <p class="ver">Versión {legal.info?.version ?? '…'}</p>
  <div class="resp card">
    <b>Responsable</b>
    <span>{op.name}</span>
    {#if op.address}<span>{op.address}</span>{/if}
    {#if op.email}<span>Contacto: <a href="mailto:{op.email}">{op.email}</a></span>{/if}
  </div>
  {#if which === 'privacidad'}<Privacy {op} />{:else}<Terms {op} />{/if}
</div>

<style>
  .ver { color: var(--muted); font-size: 0.9rem; margin: 0 0 1rem; }
  .resp { display: grid; gap: 2px; margin-bottom: 1.25rem; font-size: 0.92rem; }
  .legal-text :global(h2) { font-size: 1.2rem; margin: 1.5rem 0 0.4rem; }
  .legal-text :global(li) { margin-bottom: 0.35rem; }
</style>
