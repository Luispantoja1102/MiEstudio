<script>
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import { session } from '$lib/session.svelte.js';

  // Ajustes al estilo de dos columnas: a la izquierda las secciones agrupadas (con su descripción) y a la derecha
  // la que se eligió, sin salir de la página (?s=id). En celular primero va la lista y luego la sección, con flecha para volver.
  // groups: [{ title, items: [{ id, label, desc, icon, component?, to?, roles? }] }]
  // component: la página que se muestra dentro; to: abre otra página en lugar de una sección.
  let { title = 'Ajustes', intro = '', groups, onlogout } = $props();

  const LAST = groups.at(-1)?.title;
  const visible = $derived(
    groups
      .map((g) => ({ ...g, items: g.items.filter((i) => !i.roles || i.roles.includes(session.me?.role)) }))
      .filter((g) => g.items.length)
  );
  const sections = $derived(visible.flatMap((g) => g.items).filter((i) => !i.to));
  const requested = $derived(page.url.searchParams.get('s') ?? '');

  let wide = $state(true);
  $effect(() => {
    const mq = window.matchMedia('(min-width: 768px)');
    wide = mq.matches;
    const on = (e) => (wide = e.matches);
    mq.addEventListener('change', on);
    return () => mq.removeEventListener('change', on);
  });

  const section = $derived(sections.find((i) => i.id === requested) ?? (wide ? sections[0] : null) ?? null);
  const open = (id) => goto(`${page.url.pathname}?s=${id}`, { keepFocus: true });
  const back = () => goto(page.url.pathname, { keepFocus: true });
</script>

<div class="hub">
  <nav class="index" class:hide={section && !wide} aria-label="Secciones de ajustes">
    <header class="ih">
      <h1>{title}</h1>
      {#if intro}<p>{intro}</p>{/if}
    </header>
    <div class="groups">
      {#each visible as g (g.title)}
        <div class="group">
          <p class="gt">{g.title}</p>
          <ul>
            {#each g.items as i (i.id)}
              {@const on = !i.to && section?.id === i.id}
              <li>
                {#if i.to}
                  <a class="row" href={i.to}>
                    <Icon name={i.icon} size={20} />
                    <span class="rt"><b>{i.label}</b><small>{i.desc}</small></span>
                    <span class="chev"><Icon name="arrow" size={16} /></span>
                  </a>
                {:else}
                  <button type="button" class="row" class:on aria-current={on ? 'page' : undefined} onclick={() => open(i.id)}>
                    <Icon name={i.icon} size={20} />
                    <span class="rt"><b>{i.label}</b><small>{i.desc}</small></span>
                    <span class="chev"><Icon name="arrow" size={16} /></span>
                  </button>
                {/if}
              </li>
            {/each}
            {#if g.title === LAST && onlogout}
              <li>
                <button type="button" class="row danger" onclick={onlogout}>
                  <Icon name="logout" size={20} />
                  <span class="rt"><b>Cerrar sesión</b><small>Salir de este dispositivo</small></span>
                </button>
              </li>
            {/if}
          </ul>
        </div>
      {/each}
    </div>
  </nav>

  {#if section}
    {@const C = section.component}
    <main class="pane">
      <header class="ph">
        {#if !wide}<button type="button" class="backbtn" aria-label="Volver a Ajustes" onclick={back}><span><Icon name="arrow" size={20} /></span></button>{/if}
        <div>
          <h2>{section.label}</h2>
          <p>{section.desc}</p>
        </div>
      </header>
      <!-- cada sección conserva su página; su propio encabezado se omite porque ya lo muestra este -->
      <div class="body">{#key section.id}<C />{/key}</div>
    </main>
  {:else if wide}
    <div class="empty"><p class="meta">No tienes ajustes disponibles con tu rol.</p></div>
  {/if}
</div>

<style>
  .hub { display: grid; gap: 24px; }
  @media (min-width: 768px) { .hub { grid-template-columns: 17rem minmax(0, 1fr); gap: 40px; align-items: start; } }
  .index { display: grid; gap: 20px; min-width: 0; }
  .index.hide { display: none; }
  @media (min-width: 768px) { .index { position: sticky; top: 16px; } }
  .index h1 { font-size: 1.75rem; margin: 0; }
  .ih p { color: var(--muted); margin: 0.4rem 0 0; font-size: 0.9rem; }
  .groups { display: grid; gap: 20px; padding: 12px; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  .gt { font-size: 0.72rem; font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; color: var(--muted); margin: 4px 12px 6px; }
  ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 2px; }
  .row { display: flex; align-items: center; gap: 12px; width: 100%; min-height: 44px; padding: 8px 12px; border: 0; border-radius: var(--radius-sm); background: transparent; color: var(--text); font: inherit; text-align: left; text-decoration: none; cursor: pointer; }
  .row:hover { background: var(--surface-2); }
  .row:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
  .row.on { background: var(--accent-soft); color: var(--accent-hover); box-shadow: inset 3px 0 0 var(--accent); }
  .row.danger { color: var(--danger); }
  .row.danger:hover { background: var(--danger-soft); }
  .rt { display: grid; flex: 1; min-width: 0; }
  .rt b { font-size: 0.95rem; font-weight: 500; }
  .rt small { font-size: 0.78rem; line-height: 1.3; color: var(--muted); }
  .row.on .rt small { color: inherit; }
  /* en pantalla ancha la descripción ya se lee en el encabezado de la sección */
  @media (min-width: 768px) { .rt small, .chev { display: none; } }
  .chev { opacity: 0.55; flex: none; }
  .pane { min-width: 0; }
  .ph { display: flex; align-items: flex-start; gap: 12px; margin-bottom: 1.5rem; padding-bottom: 1rem; border-bottom: 1px solid var(--border); }
  .pane h2 { font-size: 1.5rem; margin: 0; }
  .ph p { color: var(--muted); margin: 0.3rem 0 0; }
  .backbtn { display: grid; place-items: center; width: 44px; height: 44px; flex: none; border: 1px solid var(--control-border); border-radius: var(--radius-sm); background: var(--surface); color: var(--text); cursor: pointer; }
  .backbtn:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  .backbtn span { display: grid; transform: rotate(180deg); }
  .meta { color: var(--muted); }
  /* El título de cada página incrustada ya lo da la sección: se oculta el encabezado (.page-head, también el de lib/ui/PageHeader),
     pero si trae acciones (botón de alta, filtros) se deja solo esa parte, alineada a la derecha. */
  .body :global(.page-head) { display: none; }
  .body :global(.page-head:has(.acts *, > button, > a.btn, > .btn, > div > button, > div > a.btn)) { display: flex; justify-content: flex-end; margin-bottom: 1rem; }
  .body :global(.page-head:has(.acts *, > button, > a.btn, > .btn, > div > button, > div > a.btn) > div:first-child:not(:has(> button, > a.btn))) { display: none; }
</style>
