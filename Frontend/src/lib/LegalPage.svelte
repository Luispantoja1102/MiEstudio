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
  <div class="top">
    <h1>{title}</h1>
    <p class="ver">Versión {info?.version ?? '…'}</p>
  </div>
  <div class="grid">
    <aside class="resp" aria-label="Responsable">
      <b>Responsable</b>
      <span>{op.name || 'Operador de la plataforma (pendiente de configurar: LEGAL_NAME)'}</span>
      {#if op.address}<span>{op.address}</span>{/if}
      <span>Privacidad: {#if op.email}<a href="mailto:{op.email}">{op.email}</a>{:else}pendiente de configurar (LEGAL_EMAIL){/if}</span>
    </aside>
    <article class="texto">
      {@render children({ op })}
    </article>
  </div>
</main>
<footer class="container foot"><a href="/privacidad">Aviso de privacidad</a> · <a href="/terminos">Términos y condiciones</a></footer>

<style>
  .legal { max-width: 960px; margin: 0 auto; padding: 40px 20px 56px; }
  .top { padding-bottom: 20px; margin-bottom: 28px; border-bottom: 1px solid var(--border); }
  h1 { font-size: clamp(1.7rem, 5vw, 2.4rem); line-height: 1.15; margin: 0 0 8px; letter-spacing: -0.01em; }
  .ver { color: var(--muted); font-size: 0.9rem; margin: 0; font-variant-numeric: tabular-nums; }
  .grid { display: grid; gap: 28px; }
  .resp { display: grid; gap: 2px; align-self: start; font-size: 0.92rem; line-height: 1.5; color: var(--muted); padding: 16px 18px; background: var(--surface); border: 1px solid var(--border); border-left: 3px solid var(--accent); border-radius: var(--radius-sm); overflow-wrap: anywhere; }
  .resp b { color: var(--text); font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.06em; margin-bottom: 4px; }
  .texto { max-width: 65ch; font-size: 1.0625rem; line-height: 1.7; counter-reset: cl; }
  .texto :global(p) { margin: 0 0 1em; }
  .texto :global(h2) { counter-increment: cl; display: flex; gap: 0.6em; align-items: baseline; font-size: 1.25rem; line-height: 1.3; margin: 2.2rem 0 0.6rem; padding-top: 1.4rem; border-top: 1px solid var(--border); }
  .texto :global(h2:first-child) { margin-top: 0; }
  .texto :global(h2::before) { content: counter(cl, decimal-leading-zero); flex: none; color: var(--accent); font-size: 0.85rem; font-variant-numeric: tabular-nums; min-width: 1.8em; }
  .texto :global(ul) { margin: 0 0 1em; padding-left: 1.25em; }
  .texto :global(li) { margin-bottom: 0.5rem; padding-left: 0.2em; }
  .texto :global(li::marker) { color: var(--muted); }
  .texto :global(a) { color: var(--accent); text-underline-offset: 3px; }
  @media (min-width: 900px) {
    .grid { grid-template-columns: 240px minmax(0, 1fr); gap: 56px; }
    .resp { position: sticky; top: 84px; }
  }
  .foot { text-align: center; color: var(--muted); font-size: 0.85rem; padding-bottom: 2rem; }
  .foot a { display: inline-block; padding: 12px 6px; }
</style>
