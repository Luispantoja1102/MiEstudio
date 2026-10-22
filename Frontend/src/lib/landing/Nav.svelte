<script>
  import Logo from '$lib/Logo.svelte';
  import ThemeToggle from '$lib/ThemeToggle.svelte';

  const links = [['Funciones', '#funciones'], ['Cómo funciona', '#como-funciona'], ['Planes', '#planes'], ['Preguntas', '#preguntas']];
</script>

<header class="nav">
  <div class="container bar">
    <Logo />
    <nav aria-label="Secciones" class="links">
      {#each links as [label, href]}<a {href}>{label}</a>{/each}
    </nav>
    <div class="actions">
      <ThemeToggle />
      <a class="btn ghost hide-sm" href="/login">Iniciar sesión</a>
      <a class="btn" href="/registro">Probar gratis</a>
    </div>
  </div>
</header>

<style>
  .nav { position: sticky; top: 0; z-index: 20; background: color-mix(in srgb, var(--bg) 84%, transparent); backdrop-filter: saturate(1.4) blur(14px); border-bottom: 1px solid var(--border); }
  .bar { display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 68px; }
  .links { display: none; gap: 4px; }
  .links a { padding: 8px 14px; font-size: 0.92rem; font-weight: 500; color: var(--muted); text-decoration: none; border-radius: 999px; transition: color 0.18s var(--ease), background-color 0.18s var(--ease); }
  .links a:hover { color: var(--text); background: var(--surface-2); }
  .actions { display: flex; align-items: center; gap: 8px; }
  @media (min-width: 900px) { .links { display: flex; } }
</style>
