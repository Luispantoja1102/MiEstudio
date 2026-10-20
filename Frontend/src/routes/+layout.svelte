<script>
  import '../app.css';
  import { onMount } from 'svelte';
  import { initTheme } from '$lib/theme.svelte.js';

  let { children } = $props();
  onMount(initTheme);
</script>

{@render children()}
