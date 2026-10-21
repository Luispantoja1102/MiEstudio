<script>
  import '@fontsource-variable/inter';
  import '@fontsource/playfair-display/500.css';
  import '@fontsource/playfair-display/600.css';
  import '@fontsource/playfair-display/700.css';
  import '../app.css';
  import { onMount } from 'svelte';
  import { initTheme } from '$lib/theme.svelte.js';

  let { children } = $props();
  onMount(initTheme);
</script>

{@render children()}
