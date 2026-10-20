<script>
  import ThemeToggle from '$lib/ThemeToggle.svelte';
  let { children } = $props();
</script>

<header>
  <div class="container bar">
    <a class="logo" href="/">MiEstudio</a>
    <nav>
      <ThemeToggle />
      {@render children?.()}
    </nav>
  </div>
</header>

<style>
  header { border-bottom: 1px solid var(--border); background: var(--surface); }
  .bar { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding-block: 12px; }
  .logo {
    font-family: var(--font-heading);
    font-weight: var(--heading-weight);
    font-size: 1.35rem;
    letter-spacing: var(--heading-tracking);
    color: var(--text);
    text-decoration: none;
  }
  nav { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; justify-content: flex-end; }
</style>
