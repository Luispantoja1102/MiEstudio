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
  <nav class="list" class:hide={section && !wide} aria-label="Secciones de ajustes">
    <header>
      <h1>{title}</h1>
      {#if intro}<p>{intro}</p>{/if}
    </header>
    <div class="card groups">
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
      <header>
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
    <div class="card"><p class="meta">No tienes ajustes disponibles con tu rol.</p></div>
  {/if}
</div>

<style>
  .hub { display: grid; gap: 24px; }
  @media (min-width: 768px) { .hub { grid-template-columns: minmax(17rem, 21rem) minmax(0, 1fr); align-items: start; } }
  .list { display: grid; gap: 20px; min-width: 0; }
  .list.hide { display: none; }
  .list h1 { font-size: 2.25rem; margin: 0; }
  .list header p { color: var(--muted); margin: 0.4rem 0 0; }
  .groups { display: grid; gap: 20px; padding: 12px; }
  .gt { font-size: 0.72rem; font-weight: 600; letter-spacing: 0.1em; text-transform: uppercase; color: var(--muted); margin: 0 12px 6px; }
  ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 2px; }
  .row { display: flex; align-items: center; gap: 12px; width: 100%; padding: 10px 12px; border: 0; border-radius: 12px; background: transparent; color: var(--muted); font: inherit; text-align: left; text-decoration: none; cursor: pointer; }
  .row:hover { background: var(--surface-2); color: var(--text); }
  .row.on { background: var(--accent-soft); color: var(--accent); }
  .row.danger:hover { background: var(--danger-soft); color: var(--danger); }
  .rt { display: grid; flex: 1; min-width: 0; }
  .rt b { font-size: 0.95rem; font-weight: 500; }
  .rt small { font-size: 0.78rem; line-height: 1.3; opacity: 0.85; }
  .chev { opacity: 0.55; flex: none; }
  .pane { min-width: 0; }
  .pane header { display: flex; align-items: flex-start; gap: 12px; margin-bottom: 1.25rem; }
  .pane h2 { font-size: 2rem; margin: 0; }
  .pane header p { color: var(--muted); margin: 0.3rem 0 0; }
  .backbtn { display: grid; place-items: center; width: 40px; height: 40px; margin-top: 4px; flex: none; border: 1px solid var(--border); border-radius: var(--radius-sm); background: transparent; color: var(--text); cursor: pointer; }
  .backbtn span { display: grid; transform: rotate(180deg); }
  .meta { color: var(--muted); }
  /* el encabezado de cada página ya lo da la sección */
  .body :global(.page-head) { display: none; }
  .body :global(.page-head:has(button)),
  .body :global(.page-head:has(a.btn)) { display: flex; justify-content: flex-end; margin-bottom: 1rem; }
  .body :global(.page-head:has(button) > div:first-child),
  .body :global(.page-head:has(a.btn) > div:first-child) { display: none; }
</style>
