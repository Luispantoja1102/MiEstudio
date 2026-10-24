<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';

  // Los pagos ahora viven en Finanzas; esta ruta se queda para que los enlaces viejos sigan funcionando.
  onMount(() => goto('/plataforma/finanzas?v=pagos', { replaceState: true }));
</script>
