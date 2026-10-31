<script>
  // Superficie blanca con borde fino: la única «tarjeta» del panel. padding: 'normal' | 'compact' | 'flush' (sin relleno, para tablas).
  let { title = '', description = '', padding = 'normal', actions, children } = $props();
</script>

<section class="surface {padding}">
  {#if title || actions}
    <header>
      <div>
        {#if title}<h2>{title}</h2>{/if}
        {#if description}<p>{description}</p>{/if}
      </div>
      {#if actions}<div class="acts">{@render actions()}</div>{/if}
    </header>
  {/if}
  {@render children?.()}
</section>

<style>
  .surface { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); min-width: 0; }
  .normal { padding: 24px; }
  .compact { padding: 16px; }
  .flush { padding: 0; overflow: hidden; }
  header { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
  .flush header { padding: 20px 24px 0; }
  h2 { margin: 0; font-size: 1.125rem; }
  p { margin: 4px 0 0; color: var(--muted); font-size: 0.9rem; }
  .acts { display: flex; flex-wrap: wrap; gap: 8px; }
</style>
