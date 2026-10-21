<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import Logo from '$lib/Logo.svelte';
  import ThemeToggle from '$lib/ThemeToggle.svelte';
  import { getToken, setToken } from '$lib/api.js';
  import { loadSession, session } from '$lib/session.svelte.js';

  let { children } = $props();

  // [icono, etiqueta, ruta (null = próximamente), roles permitidos (vacío = todos)]
  const nav = [
    ['home', 'Inicio', '/panel', []],
    ['calendar', 'Agenda', null, []],
    ['card', 'Ventas', null, []],
    ['users', 'Clientes', '/panel/clientes', []],
    ['scissors', 'Servicios', '/panel/servicios', []],
    ['package', 'Inventario', '/panel/inventario', []],
    ['users', 'Equipo', '/panel/equipo', ['admin']],
    ['percent', 'Comisiones', null, []]
  ];
  const items = $derived(nav.filter(([, , , roles]) => !roles.length || roles.includes(session.me?.role)));
  const isActive = (href) => (href === '/panel' ? page.url.pathname === href : page.url.pathname.startsWith(href));

  onMount(async () => {
    if (!getToken()) return goto('/login');
    try {
      await loadSession();
    } catch {
      setToken(null);
      goto('/login');
    }
  });

  function logout() {
    setToken(null);
    session.me = null;
    goto('/');
  }
</script>

{#snippet links()}
  {#each items as [icon, label, href]}
    {#if href}
      <a class="item" class:active={isActive(href)} {href} aria-current={isActive(href) ? 'page' : undefined}>
        <Icon name={icon} size={18} />{label}
      </a>
    {:else}
      <span class="item off" aria-disabled="true"><Icon name={icon} size={18} />{label}<em>Pronto</em></span>
    {/if}
  {/each}
{/snippet}

<div class="shell">
  <aside class="side">
    <Logo />
    <nav aria-label="Módulos">{@render links()}</nav>
  </aside>

  <div class="main">
    <header class="top">
      <span class="biz">{session.me?.business.name ?? ''}</span>
      <div class="tools">
        <ThemeToggle />
        <button class="btn ghost" onclick={logout}><Icon name="logout" size={18} />Salir</button>
      </div>
    </header>
    <nav class="tabs" aria-label="Módulos">{@render links()}</nav>

    <main class="content">
      {#if session.me}{@render children()}{/if}
    </main>
  </div>
</div>

<style>
  .shell { display: grid; min-height: 100vh; }
  .main { min-width: 0; } /* evita que la barra de pestañas ensanche la página */
  .side {
    display: none;
    flex-direction: column;
    gap: 28px;
    padding: 24px 16px;
    background: var(--surface);
    border-right: 1px solid var(--border);
  }
  .side nav { display: grid; gap: 4px; }
  .item {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 44px;
    padding: 0 12px;
    border-radius: var(--radius-sm);
    color: var(--text);
    text-decoration: none;
    font-size: 0.95rem;
    font-weight: 500;
    white-space: nowrap;
  }
  .item:hover { background: var(--surface-2); }
  .item.active { background: var(--accent-soft); color: var(--accent); }
  .item.off { color: var(--muted); cursor: default; }
  .item.off:hover { background: transparent; }
  .item em { margin-left: auto; font-style: normal; font-size: 0.7rem; padding: 2px 8px; border-radius: 999px; background: var(--surface-2); }

  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-height: 68px;
    padding: 0 20px;
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  .biz { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tools { display: flex; gap: 8px; flex: none; }

  /* navegación en celular: barra horizontal desplazable */
  .tabs { display: flex; gap: 4px; padding: 8px 12px; overflow-x: auto; border-bottom: 1px solid var(--border); background: var(--surface); }
  .tabs .item em { display: none; }

  .content { padding: 28px 20px 56px; max-width: 1000px; }

  @media (min-width: 900px) {
    .shell { grid-template-columns: 250px 1fr; }
    .side { display: flex; }
    .tabs { display: none; }
    .content { padding: 40px 40px 64px; }
    .top { padding: 0 40px; }
  }
</style>
