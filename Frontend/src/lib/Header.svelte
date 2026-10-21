<script>
  import Logo from '$lib/Logo.svelte';
  import ThemeToggle from '$lib/ThemeToggle.svelte';
  let { children } = $props();
</script>

<header>
  <div class="container bar">
    <Logo />
    <nav aria-label="Principal">
      <ThemeToggle />
      {@render children?.()}
    </nav>
  </div>
</header>

<style>
  header {
    position: sticky;
    top: 0;
    z-index: 10;
    border-bottom: 1px solid var(--border);
    background: color-mix(in srgb, var(--bg) 82%, transparent);
    backdrop-filter: saturate(1.4) blur(12px);
  }
  .bar { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 68px; }
  nav { display: flex; align-items: center; gap: 8px; }
</style>
