<script>
  // Landing «La jornada»: un día de trabajo en el local, de la primera cita al corte de caja.
  // Tiene su propia dirección de arte (no sigue el tema claro/oscuro del panel): los colores salen de los materiales del oficio.
  import '@fontsource-variable/archivo/wdth.css';
  import Logo from '$lib/Logo.svelte';
  import Recorrido from './Recorrido.svelte';
  import Apertura from './Apertura.svelte';
  import Madrugada from './Madrugada.svelte';
  import Comprobante from './Comprobante.svelte';
  import Oficios from './Oficios.svelte';
  import Corte from './Corte.svelte';
  import Cierre from './Cierre.svelte';

  const links = [['Oficios', '#oficios'], ['Planes', '#planes'], ['Preguntas', '#preguntas']];
</script>

<div class="jornada">
  <a class="saltar" href="#contenido">Saltar al contenido</a>
  <!-- Barra pequeña, fija arriba a la izquierda -->
  <header class="barra">
    <Logo />
    <nav aria-label="Secciones">{#each links as [l, h]}<a href={h}>{l}</a>{/each}</nav>
    <a class="entrar" href="/login">Entrar</a>
    <a class="btn btn-chico" href="/registro">Probar gratis</a>
  </header>
  <main id="contenido">
    <Recorrido />
    <Apertura />
    <Madrugada />
    <Comprobante />
    <Oficios />
    <Corte />
    <Cierre />
  </main>
</div>

<style>
  .jornada {
    /* Materiales del oficio */
    --tinta: #141416;            /* tinta de pestaña, letreros */
    --tinta-suave: #55555c;
    --ticket: #f6f5f1;           /* papel térmico de la caja */
    --noche: #161a22;            /* la madrugada */
    --luna: #c9d0dc;
    --bermellon: #b5483b;        /* esmalte rojo y franja del poste de barbería */
    --bermellon-osc: #973a2f;
    --cuero: #3d2b29;            /* sillón de barbero */
    --esmalte: #8a5552;
    --eucalipto: #4d5f55;        /* cabina de spa */
    --miel: #8a6a2e;
    --display: 'Archivo Variable', system-ui, sans-serif;
    --ancho: 112%;
    --rotulo: 'Archivo Variable', 'Arial Narrow', sans-serif;
    --texto: 'Archivo Variable', system-ui, sans-serif;
    --ease: cubic-bezier(0.32, 0.72, 0, 1);

    font-family: var(--texto);
    font-size: 1rem;
    color: var(--tinta);
    background: var(--tinta);
    -webkit-font-smoothing: antialiased;
  }
  .jornada :global(*) { box-sizing: border-box; }

  .saltar { position: absolute; left: 12px; top: -60px; z-index: 50; padding: 10px 16px; border-radius: 8px; background: var(--ticket); color: var(--tinta); font-weight: 600; }
  .saltar:focus { top: 12px; }

  .barra {
    position: fixed; z-index: 40; top: 14px; left: 14px;
    display: flex; align-items: center; gap: 4px; padding: 5px 5px 5px 8px; border-radius: 14px;
    background: rgb(246 245 241 / 0.94); color: var(--tinta);
    box-shadow: 0 1px 2px rgb(20 20 22 / 0.12), 0 12px 28px -14px rgb(20 20 22 / 0.4);
  }
  .barra :global(.logo) { gap: 8px; margin-right: 6px; }
  .barra :global(.mark) { width: 30px; height: 30px; border-radius: 8px; }
  .barra :global(.word) { font-family: var(--display); font-stretch: var(--ancho); font-size: 1.02rem; }
  nav { display: flex; }
  nav a, .entrar { padding: 7px 10px; border-radius: 8px; color: var(--tinta-suave); text-decoration: none; font-weight: 500; font-size: 0.9rem; }
  nav a:hover, .entrar:hover { color: var(--tinta); background: rgb(20 20 22 / 0.06); }

  /* Botones de toda la landing */
  .jornada :global(.btn) {
    display: inline-flex; align-items: center; justify-content: center; min-height: 52px; padding: 0 26px; border-radius: 10px; border: 0;
    background: var(--bermellon); color: #fff; font-weight: 700; font-size: 1rem; text-decoration: none;
    transition: background-color 200ms var(--ease), transform 200ms var(--ease);
  }
  .jornada :global(.btn:hover) { background: var(--bermellon-osc); }
  .jornada :global(.btn:active) { transform: translateY(1px); }
  .jornada :global(.btn-chico) { min-height: 42px; padding: 0 16px; font-size: 0.95rem; }
  .jornada :global(.btn-linea) { background: transparent; color: var(--tinta); box-shadow: inset 0 0 0 2px var(--tinta); }
  .jornada :global(.btn-linea:hover) { background: var(--tinta); color: var(--ticket); }
  .jornada :global(.link) { color: inherit; font-weight: 600; text-decoration: underline; text-decoration-thickness: 2px; text-underline-offset: 5px; }
  .jornada :global(a:focus-visible), .jornada :global(button:focus-visible) { outline: 3px solid var(--bermellon); outline-offset: 3px; }

  .barra :global(.btn-chico) { min-height: 36px; padding: 0 14px; font-size: 0.88rem; }
  @media (max-width: 900px) { nav { display: none; } }
  @media (max-width: 420px) { .entrar { display: none; } }
</style>
