<script>
  import Header from '$lib/Header.svelte';
  import LegalText from '$lib/legal/LegalText.svelte';

  // Página completa de un texto legal de la plataforma (la misma que se abre en ventana desde los enlaces).
  let { title, which } = $props(); // which: 'privacidad' | 'terminos'
</script>

<svelte:head><title>{title} · Barbella</title></svelte:head>
<Header><a class="btn ghost hide-sm" href="/login">Iniciar sesión</a></Header>
<main class="legal">
  <h1>{title}</h1>
  <LegalText {which} />
</main>
<footer class="container foot"><a href="/privacidad">Aviso de privacidad</a> · <a href="/terminos">Términos y condiciones</a></footer>

<style>
  .legal { max-width: 760px; margin: 0 auto; padding: 32px 20px 48px; }
  h1 { font-size: 2.2rem; }
  .foot { text-align: center; color: var(--muted); font-size: 0.85rem; padding-bottom: 2rem; }
</style>
