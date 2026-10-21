<script>
  import Header from '$lib/Header.svelte';

  // Marco de las páginas públicas (sin sesión): encabezado con tema y un contenedor angosto.
  let { title = '', subtitle = '', children } = $props();
</script>

<svelte:head><title>{title ? `${title} · MiEstudio` : 'MiEstudio'}</title></svelte:head>
<Header />
<main class="wrap">
  {#if title}
    <h1>{title}</h1>
    {#if subtitle}<p class="sub">{subtitle}</p>{/if}
  {/if}
  {@render children()}
</main>

<style>
  .wrap { width: 100%; max-width: 640px; margin: 0 auto; padding: 32px 16px 72px; }
  h1 { font-size: 2rem; margin-bottom: 0.3em; }
  .sub { color: var(--muted); margin-bottom: 1.5rem; }
  h1:not(:has(+ .sub)) { margin-bottom: 1rem; }
</style>
