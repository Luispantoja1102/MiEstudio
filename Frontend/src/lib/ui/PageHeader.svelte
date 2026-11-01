<script>
  // Encabezado de página. La clase global .page-head se conserva: SettingsHub la usa para ocultar el título cuando la página va incrustada en Ajustes.
  let { title, description = '', leading, actions } = $props();
</script>

<header class="page-head">
  <div class="lead">
    {@render leading?.()}
    <div>
      <h1>{title}</h1>
      {#if description}<p>{description}</p>{/if}
    </div>
  </div>
  {#if actions}<div class="acts">{@render actions()}</div>{/if}
</header>

<style>
  .lead { display: flex; align-items: center; gap: 16px; min-width: 0; }
  .acts { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
</style>
