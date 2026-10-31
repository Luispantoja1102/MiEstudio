<script>
  import '@fontsource-variable/archivo';
  import '../app.css';
  import { onMount } from 'svelte';
  import UpdateBanner from '$lib/UpdateBanner.svelte';
  import { initTheme } from '$lib/theme.svelte.js';

  let { children } = $props();
  onMount(initTheme);
</script>

{@render children()}
<UpdateBanner />
