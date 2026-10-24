<script>
  import { ACTIONS, ago } from '$lib/format.js';

  // Línea de tiempo de la bitácora: qué se hizo, quién y en qué negocio.
  let { items = [], empty = 'Aún no hay movimientos.' } = $props();
</script>

{#if items.length === 0}
  <p class="meta">{empty}</p>
{:else}
  <ul class="timeline">
    {#each items as a (a.id)}
      <li>
        <i aria-hidden="true"></i>
        <div>
          <span>{ACTIONS[a.action] ?? a.action}</span>
          <small class="meta">{a.actor || 'Sistema'}{#if a.business} · <a href="/plataforma/negocios/{a.business_id}">{a.business}</a>{/if}</small>
        </div>
        <time class="when" datetime={a.created_at}>{ago(a.created_at)}</time>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .meta { color: var(--muted); font-size: 0.85rem; }
  .timeline { list-style: none; margin: 0.75rem 0 0; padding: 0; display: grid; gap: 8px; }
  .timeline li { display: grid; grid-template-columns: 10px minmax(0, 1fr) auto; gap: 10px; align-items: start; font-size: 0.92rem; }
  .timeline i { width: 8px; height: 8px; margin-top: 0.45rem; border-radius: 50%; background: var(--accent); }
  .timeline small { display: block; }
  .timeline a { color: inherit; }
  .when { color: var(--muted); font-size: 0.8rem; white-space: nowrap; }
</style>
