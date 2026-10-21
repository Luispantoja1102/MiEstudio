<script>
  // Diálogo nativo: gestiona foco, Escape y fondo. Cierra con el botón o al hacer clic fuera.
  let { open = $bindable(false), title = '', children } = $props();
  let dlg;

  $effect(() => {
    if (!dlg) return;
    if (open && !dlg.open) dlg.showModal();
    if (!open && dlg.open) dlg.close();
  });
</script>

<dialog bind:this={dlg} onclose={() => (open = false)} onclick={(e) => e.target === dlg && (open = false)} aria-label={title}>
  <div class="box">
    <header>
      <h2>{title}</h2>
      <button class="x" onclick={() => (open = false)} aria-label="Cerrar">×</button>
    </header>
    {#if open}{@render children()}{/if}
  </div>
</dialog>

<style>
  dialog {
    width: min(640px, calc(100vw - 24px));
    max-height: calc(100vh - 32px);
    padding: 0;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow);
  }
  dialog::backdrop { background: rgba(0, 0, 0, 0.5); }
  .box { padding: 1.25rem 1.5rem 1.5rem; }
  header { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 1rem; }
  h2 { margin: 0; font-size: 1.4rem; }
  .x { width: 40px; height: 40px; font-size: 1.6rem; line-height: 1; color: var(--muted); background: transparent; border: 0; border-radius: var(--radius-sm); cursor: pointer; }
  .x:hover { background: var(--surface-2); color: var(--text); }
</style>
