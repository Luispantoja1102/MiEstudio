<script>
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import Logo from '$lib/Logo.svelte';
  import ThemeToggle from '$lib/ThemeToggle.svelte';

  // items: [icono, etiqueta, ruta (null = próximamente), rutas extra que también la marcan activa]
  // groups (opcional): [{ title, items }] con títulos de sección; footer: elementos fijos al pie de la barra lateral (Ajustes)
  let { items = [], groups = null, footer = [], title = '', tag = '', onlogout, children } = $props();
  const blocks = $derived(groups ?? [{ title: '', items }]);

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
      <a class="item" class:active={isActive(href, extra)} {href} data-label={label} aria-current={isActive(href, extra) ? 'page' : undefined}>
        <Icon name={icon} size={18} /><span class="lbl">{label}</span>
      </a>
    {:else}
      <span class="item off" aria-disabled="true"><Icon name={icon} size={18} /><span class="lbl">{label}</span><em>Pronto</em></span>
    {/if}
  {/each}
{/snippet}

<div class="shell">
  <aside class="side">
    <div class="brand"><Logo />{#if tag}<span class="tag">{tag}</span>{/if}</div>
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
        <ThemeToggle />
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

  /* barra lateral: salvia oscura (o carbón en el tema oscuro); aquí el logo y los textos se leen sobre fondo oscuro */
  .side {
    --text: var(--side-text);
    --accent: var(--side-mark);
    --accent-contrast: var(--side-mark-ink);
    display: none;
    flex-direction: column;
    gap: 28px;
    padding: 24px 14px;
    background: var(--side);
    color: var(--side-text);
    position: sticky;
    top: 0;
    height: 100vh;
    overflow-y: auto;
  }
  .brand { display: grid; gap: 8px; justify-items: start; padding: 0 6px; }
  .tag { font-size: 0.75rem; font-weight: 600; color: var(--side-text); background: var(--side-active); padding: 2px 10px; border-radius: 999px; }
  .side nav { display: grid; gap: 2px; }
  .side nav:not(.foot) { flex: 1; align-content: start; }
  .side .foot { padding-top: 12px; border-top: 1px solid var(--side-line); }
  .group { margin: 18px 12px 6px; font-size: 0.78rem; font-weight: 500; color: var(--side-muted); }
  .group:first-child { margin-top: 0; }
  .item {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 44px;
    padding: 0 12px;
    border-radius: var(--radius-sm);
    color: var(--side-text);
    text-decoration: none;
    font-size: 0.95rem;
    font-weight: 500;
    white-space: nowrap;
    transition: background-color var(--dur-ui) var(--ease), transform var(--dur-press) var(--ease);
  }
  .item:active { transform: scale(0.98); }
  .item:hover { background: var(--side-hover); }
  .item.active { background: var(--side-active); font-weight: 600; }
  .item.off { color: var(--side-muted); cursor: default; }
  .item.off:hover { background: transparent; }
  .item em { margin-left: auto; font-style: normal; font-size: 0.75rem; padding: 2px 8px; border-radius: 999px; background: var(--side-hover); }
  .side :global(:focus-visible) { outline-color: var(--side-text); }

  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-height: 64px;
    padding: 0 20px;
    border-bottom: 1px solid var(--border);
    background: var(--bg);
    position: sticky;
    top: 0;
    z-index: 20;
  }
  .biz { font-family: var(--font-heading); font-size: 1.15rem; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tools { display: flex; gap: 8px; flex: none; }
  .exit { display: none; }

  .content { padding: 24px 20px calc(96px + env(safe-area-inset-bottom)); max-width: 1180px; }

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

  .scrim { position: fixed; inset: 0; z-index: 40; background: rgba(20, 18, 14, 0.45); opacity: 0; pointer-events: none; transition: opacity 200ms var(--ease); }
  .scrim.open { opacity: 1; pointer-events: auto; }
  .sheet {
    position: fixed;
    inset: auto 0 0 0;
    z-index: 50;
    display: grid;
    gap: 2px;
    padding: 8px 12px calc(12px + env(safe-area-inset-bottom));
    background: var(--surface);
    border-radius: 16px 16px 0 0;
    box-shadow: var(--shadow);
    transform: translateY(100%);
    visibility: hidden;
    transition: transform 240ms var(--ease), visibility 0s linear 240ms;
  }
  .sheet.open { transform: translateY(0); visibility: visible; transition: transform 240ms var(--ease); }
  .sheet .grab { width: 36px; height: 4px; margin: 0 auto 8px; border-radius: 2px; background: var(--border); }
  .sheet .item { color: var(--text); min-height: 52px; }
  .sheet .item:hover { background: var(--surface-2); }
  .sheet .item.active { background: var(--accent-soft); color: var(--accent); }
  .sheet .out { font: inherit; font-weight: 500; background: transparent; border: 0; cursor: pointer; text-align: left; width: 100%; border-top: 1px solid var(--border); border-radius: 0; margin-top: 6px; }

  @media (min-width: 900px) {
    .shell { grid-template-columns: 248px 1fr; }
    .side { display: flex; }
    .bottom, .scrim, .sheet { display: none; }
    .exit { display: inline-flex; }
    .content { padding: 40px 40px 64px; }
    .top { padding: 0 40px; }
  }
  @media (prefers-reduced-motion: reduce) {
    .sheet, .sheet.open { transition: none; }
  }

  /* Tema Lino: navegación en un riel de íconos sobre porcelana y contenido centrado en la ventana */
  @media (min-width: 900px) {
    :global(:root[data-theme='lino']) .shell { grid-template-columns: 96px 1fr; }
    /* el riel ocupa 96 px a la izquierda; se reserva lo mismo a la derecha para que el contenido quede centrado en la ventana, no en lo que sobra */
    :global(:root[data-theme='lino']) .main { padding-right: 96px; }
    :global(:root[data-theme='lino']) .side {
      align-items: center;
      gap: 12px;
      padding: 32px 0 28px;
      overflow: visible;
      border-right: 1px solid var(--side-line);
    }
    :global(:root[data-theme='lino']) .brand { padding: 0 0 20px; justify-items: center; }
    :global(:root[data-theme='lino']) .brand :global(.word), :global(:root[data-theme='lino']) .tag { display: none; }
    :global(:root[data-theme='lino']) .side nav { justify-items: center; gap: 8px; }
    :global(:root[data-theme='lino']) .group { display: none; }
    :global(:root[data-theme='lino']) .side .foot { border-top: 0; padding-top: 0; }
    :global(:root[data-theme='lino']) .item {
      position: relative;
      justify-content: center;
      width: 48px;
      height: 48px;
      min-height: 0;
      padding: 0;
      border-radius: 14px;
      color: var(--side-muted);
      font-weight: 500;
    }
    :global(:root[data-theme='lino']) .item:hover { color: var(--side-text); }
    :global(:root[data-theme='lino']) .item.active { color: var(--clay); font-weight: 500; }
    :global(:root[data-theme='lino']) .item :global(svg) { width: 22px; height: 22px; stroke-width: 1.4; }
    :global(:root[data-theme='lino']) .item .lbl { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
    :global(:root[data-theme='lino']) .item em { display: none; }
    /* nombre al pasar el cursor o enfocar con el teclado */
    :global(:root[data-theme='lino']) .item::after {
      content: attr(data-label);
      position: absolute;
      left: calc(100% + 12px);
      top: 50%;
      padding: 6px 12px;
      font-size: 0.85rem;
      font-weight: 500;
      white-space: nowrap;
      color: var(--bg);
      background: var(--text);
      border-radius: 8px;
      opacity: 0;
      transform: translate(-4px, -50%);
      pointer-events: none;
      transition: opacity 120ms var(--ease), transform 120ms var(--ease);
      z-index: 30;
    }
    :global(:root[data-theme='lino']) .item:hover::after, :global(:root[data-theme='lino']) .item:focus-visible::after { opacity: 1; transform: translate(0, -50%); }
    :global(:root[data-theme='lino']) .top { background: transparent; border-bottom: 0; padding: 0; position: static; }
    :global(:root[data-theme='lino']) .top, :global(:root[data-theme='lino']) .content { width: 100%; max-width: 1200px; margin-inline: auto; }
    :global(:root[data-theme='lino']) .top { padding: 28px 48px 0; min-height: 0; }
    :global(:root[data-theme='lino']) .content { padding: 24px 48px 72px; }
  }
  :global(:root[data-theme='lino']) .sheet .item::after { display: none; }
</style>
