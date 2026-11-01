<script>
  // Sección de formulario: título y explicación a la izquierda, campos a la derecha. Se apila cuando el ESPACIO DISPONIBLE es angosto
  // (consulta de contenedor, no de pantalla): dentro del menú de Ajustes el contenido es mucho más estrecho que la ventana.
  let { title, description = '', children, actions } = $props();
</script>

<section class="fs">
  <div class="form-section">
    <div class="intro">
      <h2>{title}</h2>
      {#if description}<p>{description}</p>{/if}
    </div>
    <div class="body">
      {@render children?.()}
      {#if actions}<div class="acts">{@render actions()}</div>{/if}
    </div>
  </div>
</section>

<style>
  .fs { container-type: inline-size; border-top: 1px solid var(--border); }
  .fs:first-of-type { border-top: 0; }
  .form-section { display: grid; gap: 16px 40px; padding: 24px 0; }
  .fs:first-of-type .form-section { padding-top: 0; }
  h2 { margin: 0; font-size: 1.05rem; }
  p { margin: 4px 0 0; color: var(--muted); font-size: 0.9rem; }
  .body { min-width: 0; }
  .acts { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 16px; }
  @container (min-width: 720px) { .form-section { grid-template-columns: minmax(200px, 1fr) minmax(0, 2fr); } }
</style>
