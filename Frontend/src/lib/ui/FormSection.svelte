<script>
  // Sección de formulario: título y explicación a la izquierda, campos a la derecha (apilado en celular).
  let { title, description = '', children, actions } = $props();
</script>

<section class="form-section">
  <div class="intro">
    <h2>{title}</h2>
    {#if description}<p>{description}</p>{/if}
  </div>
  <div class="body">
    {@render children?.()}
    {#if actions}<div class="acts">{@render actions()}</div>{/if}
  </div>
</section>

<style>
  .form-section { display: grid; gap: 16px 40px; padding: 24px 0; border-top: 1px solid var(--border); }
  .form-section:first-of-type { border-top: 0; padding-top: 0; }
  h2 { margin: 0; font-size: 1.05rem; }
  p { margin: 4px 0 0; color: var(--muted); font-size: 0.9rem; }
  .body { min-width: 0; }
  .acts { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 16px; }
  @media (min-width: 820px) { .form-section { grid-template-columns: minmax(200px, 1fr) minmax(0, 2fr); } }
</style>
