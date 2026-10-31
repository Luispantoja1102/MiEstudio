<script>
  let { title, description = '', actions } = $props();
</script>

<div class="empty-state">
  <p class="t">{title}</p>
  {#if description}<p class="d">{description}</p>{/if}
  {#if actions}<div class="acts">{@render actions()}</div>{/if}
</div>

<style>
  .empty-state { display: grid; justify-items: center; gap: 4px; padding: 48px 16px; text-align: center; }
  .t { margin: 0; font-weight: 600; }
  .d { margin: 0; max-width: 44ch; color: var(--muted); font-size: 0.9rem; }
  .acts { margin-top: 12px; display: flex; flex-wrap: wrap; justify-content: center; gap: 8px; }
</style>
