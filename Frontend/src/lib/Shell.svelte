<script>
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import Logo from '$lib/Logo.svelte';

  // items: [icono, etiqueta, ruta (null = próximamente), rutas extra que también la marcan activa]
  // groups (opcional): [{ title, items }] con títulos de sección; footer: elementos fijos al pie de la barra lateral (Ajustes)
  let { items = [], groups = null, footer = [], title = '', tag = '', onlogout, tools, children } = $props();
  const blocks = $derived(groups ?? [{ title: '', items }]);
  // el logo vuelve al inicio del panel (nunca a la página pública, que parece haber cerrado la sesión)
  const home = $derived(blocks[0]?.items?.[0]?.[2] || '/panel');

  // las rutas raíz (/panel, /plataforma) solo coinciden exactas; las demás, por prefijo
  const isActive = (href, extra = []) =>
    href.split('/').length <= 2
      ? page.url.pathname === href
      : [href, ...extra].some((h) => page.url.pathname === h || page.url.pathname.startsWith(h + '/'));

  // celular: cuatro destinos a un toque del pulgar y «Más» con el resto
  const all = $derived(blocks.flatMap((g) => g.items).concat(footer).filter(([, , href]) => href));
  const primary = $derived(all.length > 5 ? all.slice(0, 4) : all);
  const more = $derived(all.length > 5 ? all.slice(4) : []);
  const moreActive = $derived(more.some(([, , href, extra]) => isActive(href, extra)));
  let sheet = $state(false);
  $effect(() => {
    page.url.pathname;
    sheet = false;
  });
  const onkey = (e) => e.key === 'Escape' && (sheet = false);
</script>

<svelte:window onkeydown={onkey} />

{#snippet links(list)}
  {#each list as [icon, label, href, extra]}
    {#if href}
      <a class="item" class:active={isActive(href, extra)} {href} aria-current={isActive(href, extra) ? 'page' : undefined}>
        <Icon name={icon} size={20} /><span class="lbl">{label}</span>
      </a>
    {:else}
      <span class="item off" aria-disabled="true"><Icon name={icon} size={20} /><span class="lbl">{label}</span><em>Pronto</em></span>
    {/if}
  {/each}
{/snippet}

<div class="shell">
  <aside class="side">
    <div class="brand"><Logo href={home} />{#if tag}<span class="tag">{tag}</span>{/if}</div>
    <nav aria-label="Módulos">
      {#each blocks as g}
        {#if g.title}<p class="group">{g.title}</p>{/if}
        {@render links(g.items)}
      {/each}
    </nav>
    {#if footer.length}<nav class="foot" aria-label="Ajustes">{@render links(footer)}</nav>{/if}
  </aside>

  <div class="main">
    <header class="top">
      <span class="biz">{title}</span>
      <div class="tools">
        {@render tools?.()}
        <button class="btn ghost exit" onclick={onlogout}><Icon name="logout" size={18} />Salir</button>
      </div>
    </header>

    <main class="content">{@render children()}</main>
  </div>

  <nav class="bottom" aria-label="Módulos">
    {#each primary as [icon, label, href, extra]}
      <a href={href} class:active={isActive(href, extra)} aria-current={isActive(href, extra) ? 'page' : undefined}>
        <Icon name={icon} size={22} /><span>{label}</span>
      </a>
    {/each}
    {#if more.length}
      <button class:active={moreActive} aria-expanded={sheet} aria-controls="more-sheet" onclick={() => (sheet = !sheet)}>
        <Icon name="menu" size={22} /><span>Más</span>
      </button>
    {/if}
  </nav>

  <div class="scrim" class:open={sheet} onclick={() => (sheet = false)} aria-hidden="true"></div>
  <div class="sheet" id="more-sheet" class:open={sheet} role="dialog" aria-label="Más opciones" aria-hidden={!sheet} inert={!sheet}>
    <div class="grab" aria-hidden="true"></div>
    {#each more as [icon, label, href, extra]}
      <a class="item" class:active={isActive(href, extra)} {href}><Icon name={icon} size={20} />{label}</a>
    {/each}
    <button class="item out" onclick={onlogout}><Icon name="logout" size={20} />Salir</button>
  </div>
</div>

<style>
  .shell { display: grid; min-height: 100vh; }
  .main { min-width: 0; } /* evita que el contenido ancho ensanche la página */

  /* Barra lateral blanca con etiquetas: la estructura es la misma en panel y plataforma; el color sale de los tokens --side-* */
  .side {
    display: none;
    flex-direction: column;
    gap: 4px;
    padding: 20px 12px 16px;
    background: var(--side);
    color: var(--side-text);
    border-right: 1px solid var(--side-line);
    position: sticky;
    top: 0;
    height: 100vh;
    overflow-y: auto;
  }
  .brand { display: flex; flex-direction: column; align-items: flex-start; gap: 6px; padding: 4px 12px 20px; }
  .tag { font-size: 0.7rem; font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; color: var(--side-muted); }
  .side nav { display: grid; gap: 2px; }
  .side nav:not(.foot) { flex: 1; align-content: start; }
  .foot { padding-top: 12px; border-top: 1px solid var(--side-line); }
  .group { margin: 16px 12px 4px; font-size: 0.72rem; font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; color: var(--side-muted); }
  .item {
    position: relative;
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 44px;
    padding: 0 12px;
    border-radius: var(--radius-sm);
    color: var(--side-muted);
    text-decoration: none;
    font-size: 0.95rem;
    font-weight: 500;
    white-space: nowrap;
    transition: background-color var(--dur-ui) var(--ease), color var(--dur-ui) var(--ease), transform var(--dur-press) var(--ease);
  }
  .item:active { transform: scale(0.98); }
  .item:hover { color: var(--side-text); background: var(--side-hover); }
  .item.active { color: var(--side-active-ink); background: var(--side-active); font-weight: 600; }
  .item.off { cursor: default; opacity: 0.6; }
  .item.off:hover { background: transparent; color: var(--side-muted); }
  .item :global(svg) { flex: none; stroke-width: 1.6; }
  .item em { margin-left: auto; font-size: 0.7rem; font-style: normal; font-weight: 600; color: var(--side-muted); }

  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-height: 64px;
    padding: 12px 16px;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
  }
  .biz { font-size: 1.05rem; font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tools { display: flex; align-items: center; gap: 8px; flex: none; }
  .exit { display: none; }

  .content { width: 100%; max-width: 1280px; margin-inline: auto; padding: 24px 16px calc(96px + env(safe-area-inset-bottom)); }

  /* celular: barra inferior */
  .bottom {
    position: fixed;
    inset: auto 0 0 0;
    z-index: 30;
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: 1fr;
    padding: 6px 6px calc(6px + env(safe-area-inset-bottom));
    background: var(--surface);
    border-top: 1px solid var(--border);
  }
  .bottom a, .bottom button {
    display: grid;
    justify-items: center;
    align-content: center;
    gap: 2px;
    min-height: 56px;
    padding: 4px 2px;
    font: inherit;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--muted);
    background: transparent;
    border: 0;
    border-radius: var(--radius-sm);
    text-decoration: none;
    cursor: pointer;
    transition: color var(--dur-ui) var(--ease), background-color var(--dur-ui) var(--ease), transform var(--dur-press) var(--ease);
  }
  .bottom a:active, .bottom button:active { transform: scale(0.95); }
  .bottom .active { color: var(--accent); background: var(--accent-soft); font-weight: 600; }

  .scrim { position: fixed; inset: 0; z-index: 40; background: rgba(27, 34, 51, 0.4); opacity: 0; pointer-events: none; transition: opacity 200ms var(--ease); }
  .scrim.open { opacity: 1; pointer-events: auto; }
  .sheet {
    position: fixed;
    inset: auto 0 0 0;
    z-index: 50;
    display: grid;
    gap: 2px;
    padding: 8px 12px calc(12px + env(safe-area-inset-bottom));
    background: var(--surface);
    border-radius: var(--radius) var(--radius) 0 0;
    box-shadow: var(--shadow);
    transform: translateY(100%);
    visibility: hidden;
    transition: transform 240ms var(--ease), visibility 0s linear 240ms;
  }
  .sheet.open { transform: translateY(0); visibility: visible; transition: transform 240ms var(--ease); }
  .sheet .grab { width: 36px; height: 4px; margin: 0 auto 8px; border-radius: 2px; background: var(--border); }
  .sheet .item { height: 52px; color: var(--text); font-size: 0.95rem; }
  .sheet .item:hover { background: var(--surface-2); color: var(--text); }
  .sheet .item.active { background: var(--accent-soft); color: var(--accent); }
  .sheet .out { font: inherit; font-weight: 500; background: transparent; border: 0; cursor: pointer; text-align: left; width: 100%; border-top: 1px solid var(--border); border-radius: 0; margin-top: 4px; }

  @media (min-width: 900px) {
    .shell { grid-template-columns: 224px 1fr; }
    .side { display: flex; }
    .bottom, .scrim, .sheet { display: none; }
    .exit { display: inline-flex; }
    .top { min-height: 72px; padding: 12px 32px; }
    .content { padding: 32px 32px 72px; }
  }
  @media (prefers-reduced-motion: reduce) {
    .sheet, .sheet.open { transition: none; }
  }
</style>
