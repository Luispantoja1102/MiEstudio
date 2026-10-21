<script>
  import Icon from '$lib/Icon.svelte';
  import { session } from '$lib/session.svelte.js';

  // groups: [{ title, items: [{ icon, title, text, href, roles? }] }]; roles vacío = todos
  let { title = 'Ajustes', intro = '', groups } = $props();
  const visible = $derived(
    groups
      .map((g) => ({ ...g, items: g.items.filter((i) => !i.roles || i.roles.includes(session.me?.role)) }))
      .filter((g) => g.items.length)
  );
</script>

<div class="page-head">
  <div>
    <h1>{title}</h1>
    {#if intro}<p>{intro}</p>{/if}
  </div>
</div>

{#each visible as g (g.title)}
  <section>
    <h2>{g.title}</h2>
    <div class="grid">
      {#each g.items as i (i.href)}
        <a class="card tile" href={i.href}>
          <span class="ico"><Icon name={i.icon} /></span>
          <span class="txt"><b>{i.title}</b><small>{i.text}</small></span>
          <span class="go"><Icon name="arrow" size={16} /></span>
        </a>
      {/each}
    </div>
  </section>
{/each}

<style>
  section { margin-top: 2rem; }
  section:first-of-type { margin-top: 0.5rem; }
  h2 { font-size: 1rem; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); font-family: var(--font-body); font-weight: 600; margin-bottom: 0.75rem; }
  .grid { display: grid; gap: 12px; grid-template-columns: repeat(auto-fill, minmax(290px, 1fr)); }
  .tile { display: flex; align-items: center; gap: 14px; padding: 1.1rem 1.25rem; color: var(--text); text-decoration: none; transition: border-color 0.2s var(--ease), transform 0.2s var(--ease); }
  .tile:hover { border-color: var(--accent); transform: translateY(-1px); }
  .ico { display: grid; place-items: center; width: 42px; height: 42px; flex: none; border-radius: 11px; background: var(--accent-soft); color: var(--accent); }
  .txt { display: grid; gap: 2px; flex: 1; min-width: 0; }
  .txt small { color: var(--muted); font-size: 0.85rem; line-height: 1.35; }
  .go { color: var(--muted); }
  .tile:hover .go { color: var(--accent); }
</style>
