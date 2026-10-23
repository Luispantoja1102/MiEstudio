<script>
  // Landing «La jornada»: un día de trabajo en el local, de la primera cita al corte de caja.
  // Tiene su propia dirección de arte (no sigue el tema claro/oscuro del panel): los colores salen de los materiales del oficio.
  import '@fontsource-variable/archivo/wdth.css';
  import Logo from '$lib/Logo.svelte';
  import Reloj from './Reloj.svelte';
  import Apertura from './Apertura.svelte';
  import Madrugada from './Madrugada.svelte';
  import Comprobante from './Comprobante.svelte';
  import Oficios from './Oficios.svelte';
  import Corte from './Corte.svelte';
  import Cierre from './Cierre.svelte';

  const links = [['La jornada', '#madrugada'], ['Oficios', '#oficios'], ['Planes', '#planes'], ['Preguntas', '#preguntas']];
</script>

<div class="jornada">
  <a class="saltar" href="#contenido">Saltar al contenido</a>
  <header class="barra">
    <Logo />
    <nav aria-label="Secciones">{#each links as [l, h]}<a href={h}>{l}</a>{/each}</nav>
    <Reloj />
    <div class="cuenta">
      <a class="entrar" href="/login">Entrar</a>
      <a class="btn btn-chico" href="/registro">Probar gratis</a>
    </div>
  </header>
  <main id="contenido">
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
    position: fixed; z-index: 40; top: 0; left: 0; right: 0; height: 64px;
    display: grid; grid-template-columns: auto 1fr auto auto; align-items: center; gap: 24px;
    padding: 0 clamp(16px, 3vw, 40px); background: var(--ticket); color: var(--tinta);
    box-shadow: 0 1px 0 rgb(20 20 22 / 0.12);
  }
  .barra :global(.word) { font-family: var(--display); font-stretch: var(--ancho); }
  nav { display: flex; gap: 4px; }
  nav a, .entrar { padding: 8px 12px; border-radius: 8px; color: var(--tinta-suave); text-decoration: none; font-weight: 500; }
  nav a:hover, .entrar:hover { color: var(--tinta); background: rgb(20 20 22 / 0.05); }
  .cuenta { display: flex; align-items: center; gap: 6px; }

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

  @media (max-width: 1080px) { nav { display: none; } .barra { grid-template-columns: auto 1fr auto; } }
  @media (max-width: 560px) { .entrar { display: none; } .barra { gap: 12px; } }
</style>
