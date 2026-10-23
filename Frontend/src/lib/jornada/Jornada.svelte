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
    /* Tema oscuro. «tinta» es el color del texto y «ticket» el del fondo: al invertir sus valores, todo lo que ya los usa en pareja
       (botones, selectores, tarjetas) se invierte con ellos. */
    --tinta: #ece9e2;
    --tinta-suave: #a5a39b;
    --ink-rgb: 236 233 226;       /* mismos canales de «tinta», para líneas y fondos con transparencia */
    --ticket: #0f1012;
    --ticket-rgb: 15 16 18;
    --carta: #1a1b1e;             /* tarjetas y teléfono: un escalón sobre el fondo */
    --noche: #161a22;
    --luna: #c9d0dc;
    --bermellon: #cf5444;
    --bermellon-osc: #b3412f;
    --cuero: #d49a7e;             /* el café del sillón, aclarado para leerse sobre oscuro */
    --esmalte: #d98e88;
    --eucalipto: #8fb29f;
    --miel: #d2a24c;
    --s-barberia: #2a2120;        /* fondos suaves de cada oficio */
    --s-unas: #2e2020;
    --s-pestanas: #202127;
    --s-spa: #1d2723;
    --display: 'Archivo Variable', system-ui, sans-serif;
    --ancho: 112%;
    --rotulo: 'Archivo Variable', 'Arial Narrow', sans-serif;
    --texto: 'Archivo Variable', system-ui, sans-serif;
    --ease: cubic-bezier(0.32, 0.72, 0, 1);

    font-family: var(--texto);
    font-size: 1rem;
    color: var(--tinta);
    background: var(--ticket);
    color-scheme: dark;
    -webkit-font-smoothing: antialiased;
  }
  .jornada :global(*) { box-sizing: border-box; }

  .saltar { position: absolute; left: 12px; top: -60px; z-index: 50; padding: 10px 16px; border-radius: 8px; background: var(--ticket); color: var(--tinta); font-weight: 600; }
  .saltar:focus { top: 12px; }

  .barra {
    position: fixed; z-index: 40; top: 14px; left: 14px;
    display: flex; align-items: center; gap: 4px; padding: 5px 5px 5px 8px; border-radius: 14px;
    background: rgb(var(--ticket-rgb) / 0.4); color: var(--tinta);
    -webkit-backdrop-filter: blur(18px) saturate(1.3); backdrop-filter: blur(18px) saturate(1.3);
    box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.14), 0 12px 28px -14px rgb(0 0 0 / 0.6);
  }
  .barra :global(.logo) { gap: 8px; margin-right: 6px; }
  .barra :global(.mark) { width: 30px; height: 30px; border-radius: 8px; }
  .barra :global(.word) { font-family: var(--display); font-stretch: var(--ancho); font-size: 1.02rem; color: var(--tinta); }
  nav { display: flex; }
  nav a, .entrar { padding: 7px 10px; border-radius: 8px; color: var(--tinta); text-decoration: none; font-weight: 500; font-size: 0.9rem; }
  nav a:hover, .entrar:hover { color: var(--tinta); background: rgb(var(--ink-rgb) / 0.06); }

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
