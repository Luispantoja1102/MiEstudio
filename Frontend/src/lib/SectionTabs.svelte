<script>
  import { page } from '$app/state';
  import { session } from '$lib/session.svelte.js';

  // Pestañas de una sección del menú (por ejemplo Cobros: Cobrar · Caja · Historial · Servicios · Inventario).
  // tabs: [{ label, href, roles?, also?, exact? }]; se ocultan las que el rol no puede ver; con una sola no se muestra la barra.
  let { tabs, label = 'Secciones' } = $props();
  const shown = $derived(tabs.filter((t) => !t.roles || t.roles.includes(session.me?.role)));
  const current = (t) => [t.href, ...(t.also ?? [])].some((h) => page.url.pathname === h || (!t.exact && page.url.pathname.startsWith(h + '/')));
</script>

{#if shown.length > 1}
  <nav class="tabs" aria-label={label}>
    {#each shown as t (t.href)}
      <a href={t.href} class:on={current(t)} aria-current={current(t) ? 'page' : undefined}>{t.label}</a>
    {/each}
  </nav>
{/if}

<style>
  .tabs { display: flex; gap: 4px; padding: 4px; margin-bottom: 1.5rem; overflow-x: auto; background: var(--surface-2); border-radius: 999px; width: fit-content; max-width: 100%; }
  a { white-space: nowrap; padding: 6px 16px; border-radius: 999px; color: var(--muted); text-decoration: none; font-size: 0.9rem; font-weight: 500; }
  a:hover { color: var(--text); }
  a.on { background: var(--surface); color: var(--text); box-shadow: var(--shadow-sm); }
</style>
