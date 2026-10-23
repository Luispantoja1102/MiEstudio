<script>
  import Jornada from '$lib/jornada/Jornada.svelte';
</script>

<svelte:head>
  <title>Barbella · Agenda, caja y reservas para barberías y salones</title>
  <meta name="description" content="Software de agenda, caja y reservas en línea para barberías, salones de uñas, pestañas, estéticas y spas. Anticipos con Mercado Pago o transferencia, comisiones y recordatorios. Prueba 14 días gratis." />
  <meta property="og:title" content="Barbella · Agenda, caja y reservas para barberías y salones" />
  <meta property="og:description" content="Tus clientes reservan solos, tú confirmas los anticipos y cobras en pesos." />
</svelte:head>

<Jornada />

<style>
  :global(html) { scroll-behavior: smooth; }
  @media (prefers-reduced-motion: reduce) { :global(html) { scroll-behavior: auto; } }
</style>
