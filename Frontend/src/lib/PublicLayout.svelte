<script>
  import Header from '$lib/Header.svelte';
  import { forceTheme } from '$lib/theme.svelte.js';

  // Marco de las páginas públicas (sin sesión): encabezado y un contenedor angosto. Sin selector de tema; solo el tema que fijó el negocio.
  // brand: muestra la inicial del negocio como monograma (páginas de reserva)
  // businessTheme: tema que fijó el negocio ('auto' deja elegir a quien visita)
  let { title = '', subtitle = '', brand = false, businessTheme = 'auto', children } = $props();
  $effect(() => {
    if (businessTheme && businessTheme !== 'auto') return forceTheme(businessTheme);
  });
</script>

<svelte:head><title>{title ? `${title} · Barbella` : 'Barbella'}</title></svelte:head>
<Header />
<main class="wrap">
  {#if title && brand}
    <div class="id">
      <span class="mark" aria-hidden="true">{title.trim().charAt(0).toUpperCase()}</span>
      <div>
        <h1>{title}</h1>
        {#if subtitle}<p class="sub">{subtitle}</p>{/if}
      </div>
    </div>
  {:else if title}
    <h1>{title}</h1>
    {#if subtitle}<p class="sub">{subtitle}</p>{/if}
  {/if}
  {@render children()}
</main>

<style>
  .wrap { width: 100%; max-width: 640px; margin: 0 auto; padding: 28px 16px 72px; }
  h1 { font-size: 1.75rem; line-height: 1.2; margin-bottom: 0.3em; overflow-wrap: anywhere; }
  .id { display: flex; align-items: flex-start; gap: 14px; margin-bottom: 1.5rem; }
  .id > div { min-width: 0; }
  .id h1 { margin: 0; font-size: 1.5rem; }
  .id .sub { margin: 2px 0 0; }
  .mark { flex: none; display: grid; place-items: center; width: 48px; height: 48px; border-radius: var(--radius-sm); background: var(--ink, var(--text)); color: var(--surface); font-family: var(--font-heading); font-size: 1.5rem; font-weight: 700; }
  .sub { color: var(--muted); margin-bottom: 1.5rem; line-height: 1.5; }
  h1:not(:has(+ .sub)) { margin-bottom: 1rem; }
</style>
