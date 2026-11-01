<script>
  import { legalLink } from '$lib/legal/legal.svelte.js';
  import Logo from '$lib/Logo.svelte';

  // title + accent forman el titular: accent va en la misma línea, con color de acento
  let { title, accent = '', subtitle = '', wide = false, children } = $props();
</script>

<div class="auth" class:wide>
  <aside class="brand">
    <Logo />
    <p class="phrase">Cada corte, cada tinte y cada cobro, en una sola agenda.</p>
    <p class="kinds">Barberías · Estéticas · Spas · Uñas · Pestañas</p>
  </aside>

  <main>
    <div class="box">
      <header>
        <h1>{title}{#if accent}{" "}<em>{accent}</em>{/if}</h1>
        {#if subtitle}<p class="sub">{subtitle}</p>{/if}
      </header>
      {@render children()}
    </div>
    <p class="legal"><a href="/terminos" onclick={(e) => legalLink(e, 'terminos')}>Términos</a><span aria-hidden="true">·</span><a href="/privacidad" onclick={(e) => legalLink(e, 'privacidad')}>Aviso de privacidad</a></p>
  </main>
</div>

<style>
  .auth { min-height: 100vh; min-height: 100dvh; display: grid; grid-template-columns: 1fr; grid-template-rows: auto 1fr; background: var(--bg); }

  /* panel de marca: claro y plano; en móvil solo queda el logo como encabezado */
  .brand { display: flex; flex-direction: column; align-items: flex-start; padding: 16px 20px; background: var(--surface); border-bottom: 1px solid var(--border); }
  .phrase, .kinds { display: none; }

  main { display: flex; flex-direction: column; align-items: center; justify-content: flex-start; gap: 20px; padding: 24px 16px 32px; min-width: 0; }
  .box { width: 100%; max-width: 432px; padding: 24px 20px; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  .wide .box { max-width: 624px; }
  h1 { font-size: 1.75rem; line-height: 1.1; margin: 0 0 0.4rem; }
  h1 em { font-style: normal; color: var(--accent); }
  .sub { color: var(--muted); font-size: 0.95rem; margin: 0 0 1.5rem; }
  header { margin-bottom: 0.25rem; }
  .legal { display: flex; flex-wrap: wrap; align-items: center; justify-content: center; gap: 4px 8px; font-size: 0.82rem; color: var(--muted); margin: 0; }
  .legal a { color: inherit; display: inline-flex; align-items: center; min-height: 44px; }
  .legal a:hover { color: var(--text); }

  @media (min-width: 480px) {
    main { padding-top: 40px; }
    .box { padding: 32px; }
    h1 { font-size: 2rem; }
  }

  @media (min-width: 900px) {
    .auth { grid-template-columns: minmax(20rem, 0.8fr) minmax(0, 1.2fr); grid-template-rows: 1fr; }
    .wide.auth { grid-template-columns: minmax(18rem, 0.65fr) minmax(0, 1.35fr); }
    .brand { position: sticky; top: 0; height: 100vh; height: 100dvh; justify-content: space-between; padding: 40px 48px; border-bottom: 0; border-right: 1px solid var(--border); }
    .phrase { display: block; max-width: 20ch; margin: 0; font-family: var(--font-heading); font-size: clamp(1.9rem, 2.8vw, 2.6rem); font-weight: 700; line-height: 1.1; letter-spacing: -0.02em; color: var(--ink); }
    .kinds { display: block; margin: 0; font-size: 0.78rem; letter-spacing: 0.12em; text-transform: uppercase; color: var(--muted); }
    main { justify-content: center; padding: 40px 24px; }
    .box { padding: 40px; }
  }
</style>
