<script>
  import Nav from '$lib/landing/Nav.svelte';
  import Hero from '$lib/landing/Hero.svelte';
  import Problem from '$lib/landing/Problem.svelte';
  import Features from '$lib/landing/Features.svelte';
  import Modules from '$lib/landing/Modules.svelte';
  import Steps from '$lib/landing/Steps.svelte';
  import Giros from '$lib/landing/Giros.svelte';
  import Security from '$lib/landing/Security.svelte';
  import Pricing from '$lib/landing/Pricing.svelte';
  import Faq from '$lib/landing/Faq.svelte';
  import Cta from '$lib/landing/Cta.svelte';
  import Footer from '$lib/landing/Footer.svelte';
</script>

<svelte:head>
  <title>Barbella · Agenda, caja y reservas para barberías y salones</title>
  <meta name="description" content="Software de agenda, caja y reservas en línea para barberías, estéticas, spas y salones de uñas. Anticipos con Mercado Pago o transferencia, comisiones y recordatorios. Prueba 14 días gratis." />
  <meta property="og:title" content="Barbella · Agenda, caja y reservas para barberías y salones" />
  <meta property="og:description" content="Tus clientes reservan solos, tú confirmas los anticipos y cobras en pesos." />
</svelte:head>

<Nav />
<main>
  <Hero />
  <Problem />
  <Features />
  <Modules />
  <Steps />
  <Giros />
  <Security />
  <Pricing />
  <Faq />
  <Cta />
</main>
<Footer />

<style>
  :global(html) { scroll-behavior: smooth; }
  @media (prefers-reduced-motion: reduce) { :global(html) { scroll-behavior: auto; } }
</style>
