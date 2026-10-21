<script>
  import Header from '$lib/Header.svelte';
  import { forceTheme, theme } from '$lib/theme.svelte.js';

  // Marco de las páginas públicas (sin sesión): encabezado con tema y un contenedor angosto.
  // brand: muestra la inicial del negocio como monograma (páginas de reserva)
  // businessTheme: tema que fijó el negocio ('auto' deja elegir a quien visita)
  let { title = '', subtitle = '', brand = false, businessTheme = 'auto', children } = $props();
  $effect(() => {
    if (businessTheme && businessTheme !== 'auto') return forceTheme(businessTheme);
  });
</script>

<svelte:head><title>{title ? `${title} · MiEstudio` : 'MiEstudio'}</title></svelte:head>
<Header hideToggle={theme.forced} />
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
  .wrap { width: 100%; max-width: 640px; margin: 0 auto; padding: 32px 16px 72px; }
  h1 { font-size: 2rem; margin-bottom: 0.3em; }
  .id { display: flex; align-items: center; gap: 16px; margin-bottom: 1.5rem; }
  .id h1 { margin: 0; font-size: 1.75rem; }
  .id .sub { margin: 2px 0 0; }
  .mark { flex: none; display: grid; place-items: center; width: 56px; height: 56px; border-radius: 16px; background: var(--side); color: var(--side-mark); font-family: var(--font-heading); font-size: 1.9rem; font-weight: 700; }
  .sub { color: var(--muted); margin-bottom: 1.5rem; }
  h1:not(:has(+ .sub)) { margin-bottom: 1rem; }
</style>
