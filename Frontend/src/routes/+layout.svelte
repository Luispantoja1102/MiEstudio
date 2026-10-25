<script>
  import '@fontsource-variable/inter';
  import '@fontsource-variable/archivo/wdth.css';
  import '@fontsource/playfair-display/500.css';
  import '@fontsource/playfair-display/600.css';
  import '@fontsource/playfair-display/700.css';
  import '../app.css';
  import { onMount } from 'svelte';
  import UpdateBanner from '$lib/UpdateBanner.svelte';
  import LegalModal from '$lib/legal/LegalModal.svelte';
  import { initTheme } from '$lib/theme.svelte.js';

  let { children } = $props();
  onMount(initTheme);
</script>

{@render children()}
<UpdateBanner />
<LegalModal />
