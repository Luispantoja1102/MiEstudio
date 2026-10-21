<script>
  import { onMount } from 'svelte';
  import Header from '$lib/Header.svelte';

  // Marco de las páginas legales de la plataforma: carga los datos de quien la opera y la versión vigente.
  let { title, children } = $props();
  let info = $state(null);
  onMount(async () => {
    try {
      info = await (await fetch('/api/legal')).json();
    } catch {}
  });
  const op = $derived(info?.operator ?? {});
</script>

<svelte:head><title>{title} · Barbella</title></svelte:head>
<Header><a class="btn ghost hide-sm" href="/login">Iniciar sesión</a></Header>
<main class="legal">
  <h1>{title}</h1>
  <p class="ver">Versión {info?.version ?? '…'}</p>
  <div class="resp card">
    <b>Responsable</b>
    <span>{op.name || 'Operador de la plataforma (pendiente de configurar: LEGAL_NAME)'}</span>
    {#if op.address}<span>{op.address}</span>{/if}
    <span>Privacidad: {#if op.email}<a href="mailto:{op.email}">{op.email}</a>{:else}pendiente de configurar (LEGAL_EMAIL){/if}</span>
  </div>
  {@render children({ op })}
</main>
<footer class="container foot"><a href="/privacidad">Aviso de privacidad</a> · <a href="/terminos">Términos y condiciones</a></footer>

<style>
  .legal { max-width: 760px; margin: 0 auto; padding: 32px 20px 48px; }
  h1 { font-size: 2.2rem; }
  .ver { color: var(--muted); font-size: 0.9rem; margin-bottom: 1.25rem; }
  .resp { display: grid; gap: 2px; margin-bottom: 1.5rem; font-size: 0.92rem; }
  :global(.legal h2) { font-size: 1.3rem; margin: 1.75rem 0 0.4rem; }
  :global(.legal li) { margin-bottom: 0.35rem; }
  .foot { text-align: center; color: var(--muted); font-size: 0.85rem; padding-bottom: 2rem; }
</style>
