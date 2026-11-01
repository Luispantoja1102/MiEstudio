<script>
  // Aviso en línea. tone: info | success | warning | danger. Los de advertencia y error se anuncian de inmediato (role="alert").
  let { tone = 'info', message = '', live, actions, children } = $props();
  const urgent = $derived(tone === 'danger' || tone === 'warning');
</script>

<div class="notice {tone}" role={urgent ? 'alert' : 'status'} aria-live={live ?? (urgent ? 'assertive' : 'polite')}>
  <div class="msg">{#if children}{@render children()}{:else}{message}{/if}</div>
  {#if actions}<div class="acts">{@render actions()}</div>{/if}
</div>

<style>
  .notice { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 16px; padding: 12px 16px; border-radius: var(--radius-sm); font-size: 0.9rem; background: var(--info-soft); color: var(--info); }
  .success { background: var(--success-soft); color: var(--success); }
  .warning { background: var(--warn-soft); color: var(--warn); }
  .danger { background: var(--danger-soft); color: var(--danger); }
  .msg { min-width: 0; overflow-wrap: anywhere; }
  .msg :global(a) { color: inherit; font-weight: 600; }
  .acts { display: flex; gap: 8px; }
</style>
