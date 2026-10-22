<script>
  import Icon from '$lib/Icon.svelte';
</script>

<section class="section">
  <div class="container">
    <div class="cta">
      <h2>Empieza hoy, <em>sin tarjeta</em></h2>
      <p>14 días para probar la agenda, la caja y la reserva en línea con tus propios clientes.</p>
      <div class="actions">
        <a class="btn lg go" href="/registro">Crear mi cuenta <Icon name="arrow" size={18} /></a>
        <a class="btn ghost lg" href="/login">Ya tengo cuenta</a>
      </div>
    </div>
  </div>
</section>

<style>
  .section { padding-block: 40px 96px; }
  .cta { --text: var(--side-text); --muted: var(--side-muted); --accent: var(--side-mark); --accent-contrast: var(--side-mark-ink); padding: clamp(36px, 7vw, 72px) 24px; text-align: center; color: var(--side-text); background: var(--side); background-image: radial-gradient(ellipse 60% 70% at 15% 0%, rgba(192, 122, 101, 0.3), transparent 62%); border-radius: 28px; }
  h2 { margin: 0; font-size: clamp(2.1rem, 5vw, 3.4rem); line-height: 1; }
  h2 em { font-style: italic; color: var(--side-mark); }
  p { max-width: 34rem; margin: 1.1rem auto 1.8rem; color: var(--side-muted); }
  .actions { display: flex; flex-wrap: wrap; justify-content: center; gap: 12px; }
  .cta :global(.btn.ghost) { color: var(--side-text); border-color: var(--side-line); }
  .cta :global(.btn.ghost:hover) { background: var(--side-hover); border-color: var(--side-muted); }
</style>
