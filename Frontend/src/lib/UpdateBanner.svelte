<script>
  import { updated } from '$app/state';
  import { onMount } from 'svelte';

  // «Después» lo oculta hasta que se recargue la página.
  let dismissed = $state(false);

  onMount(() => {
    // al volver a la pestaña se revisa de inmediato, sin esperar al siguiente sondeo
    const check = () => document.visibilityState === 'visible' && updated.check();
    document.addEventListener('visibilitychange', check);
    return () => document.removeEventListener('visibilitychange', check);
  });
</script>

{#if updated.current && !dismissed}
  <div class="update" role="status" aria-live="polite">
    <p>
      <strong>Hay una nueva versión de Barbella.</strong>
      Termina lo que estás haciendo y actualiza cuando quieras; no se cierra nada por sí solo.
    </p>
    <div class="actions">
      <button type="button" class="btn sm" onclick={() => location.reload()}>Actualizar ahora</button>
      <button type="button" class="btn ghost sm" onclick={() => (dismissed = true)}>Después</button>
    </div>
  </div>
{/if}

<style>
  .update {
    position: fixed;
    z-index: 60;
    left: 16px;
    right: 16px;
    bottom: 16px;
    max-width: 460px;
    margin-inline: auto;
    display: grid;
    gap: 10px;
    padding: 14px 16px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow);
  }
  @media (min-width: 640px) {
    .update { left: auto; right: 20px; margin-inline: 0; }
  }
  p { margin: 0; font-size: 0.9rem; line-height: 1.45; color: var(--muted); }
  strong { color: var(--text); display: block; }
  .actions { display: flex; gap: 8px; flex-wrap: wrap; }
</style>
