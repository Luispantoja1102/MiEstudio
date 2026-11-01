<script>
  import Logo from '$lib/Logo.svelte';
  let { children } = $props();
</script>

<header>
  <div class="container bar">
    <Logo />
    <nav aria-label="Principal">
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
    background: var(--surface);
  }
  .bar { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 60px; }
  nav { display: flex; align-items: center; gap: 8px; }
</style>
