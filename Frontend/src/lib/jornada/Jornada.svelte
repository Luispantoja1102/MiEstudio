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
  import { onMount } from 'svelte';

  // La barra se pensó sobre el video oscuro. Al pasar sobre los capítulos claros (.despues) cambia a tinta sobre vidrio blanco para leerse;
  // sobre el hero conserva exactamente su aspecto.
  let claro = $state(false);
  onMount(() => {
    const el = document.querySelector('.despues');
    if (!el) return;
    const io = new IntersectionObserver(([e]) => (claro = e.isIntersecting), { rootMargin: '-30px 0px -90% 0px' });
    io.observe(el);
    return () => io.disconnect();
  });

  const links = [['Oficios', '#oficios'], ['Planes', '#planes'], ['Preguntas', '#preguntas']];
</script>

<div class="jornada">
  <a class="saltar" href="#contenido">Saltar al contenido</a>
  <!-- Barra pequeña, fija arriba a la izquierda -->
  <header class="barra" class:claro>
    <Logo />
    <nav aria-label="Secciones">{#each links as [l, h]}<a href={h}>{l}</a>{/each}</nav>
    <a class="entrar" href="/login">Entrar</a>
    <a class="btn btn-chico" href="/registro">Probar gratis</a>
  </header>
  <main id="contenido">
    <Recorrido />
    <!-- Todo lo posterior al video vive en un ámbito claro propio: el hero y la barra conservan sus tokens oscuros -->
    <div class="despues">
      <Apertura />
      <Madrugada />
      <Comprobante />
      <Oficios />
      <Corte />
      <Cierre />
    </div>
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
  .barra.claro {
    --tinta: #1c2f5c; --ink-rgb: 28 47 92; --ticket-rgb: 255 255 255;
    background: rgb(255 255 255 / 0.78);
    box-shadow: inset 0 0 0 1px rgb(28 47 92 / 0.14), 0 8px 24px -14px rgb(28 47 92 / 0.35);
  }
  .barra { transition: background-color 200ms var(--ease), box-shadow 200ms var(--ease); }
  .barra :global(.logo) { gap: 8px; margin-right: 6px; }
  .barra :global(.mark) { width: 30px; height: 30px; border-radius: 8px; }
  .barra :global(.word) { font-family: var(--display); font-stretch: var(--ancho); font-size: 1.02rem; color: var(--tinta); }
  nav { display: flex; }
  nav a, .entrar { padding: 7px 10px; border-radius: 8px; color: var(--tinta); text-decoration: none; font-weight: 500; font-size: 0.9rem; }
  nav a:hover, .entrar:hover { color: var(--tinta); background: rgb(var(--ink-rgb) / 0.06); }

  /* Ámbito claro: papel marfil, tinta azul y el bermellón como único acento. Redefine los mismos nombres de token que usan los botones
     y los componentes, así que todo lo que cae dentro se invierte solo, sin tocar .jornada (hero y barra). */
  .despues {
    --tinta: #182544;
    --tinta-suave: #4f5a73;
    --ink-rgb: 24 37 68;
    --ticket: #f6f2ea;           /* papel */
    --ticket-rgb: 246 242 234;
    --papel-2: #ede7da;          /* un escalón más hondo, para franjas */
    --carta: #ffffff;            /* superficie de las piezas de producto */
    --bermellon: #bd4636;        /* la marca, un punto más hondo para que el texto blanco pase AA */
    --bermellon-osc: #a33a2b;
    --cuero: #9a5836;
    --esmalte: #b04a55;
    --eucalipto: #3f7a62;
    --miel: #9a6f14;
    --s-barberia: #f2e4d8;
    --s-unas: #f6dfe0;
    --s-pestanas: #e3e8f2;
    --s-spa: #dcede4;
    /* tokens de la interfaz del panel, para reutilizar sus componentes (lib/ui, Receipt) */
    --text: var(--tinta);
    --muted: var(--tinta-suave);
    --border: rgb(var(--ink-rgb) / 0.14);
    --surface: #ffffff;
    --surface-2: #f0ebe0;
    --radius: 8px;
    --radius-sm: 6px;
    --radius-xs: 4px;
    --accent: var(--bermellon);
    --accent-soft: #f9e6e1;

    background: var(--ticket);
    color: var(--tinta);
    color-scheme: light;
  }
  .despues :global(.envoltura) { max-width: 1200px; margin: 0 auto; padding: 0 clamp(18px, 4vw, 40px); }
  .despues :global(.sr) { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
  /* Rótulo de cada capítulo: la hora del día en que ocurre */
  .despues :global(.kicker) { display: flex; align-items: center; gap: 12px; margin: 0 0 18px; font-size: 0.82rem; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--tinta-suave); }
  .despues :global(.kicker time) { font-family: var(--rotulo); font-stretch: 75%; font-size: 1.15rem; letter-spacing: 0.02em; color: var(--bermellon); font-variant-numeric: tabular-nums; }
  .despues :global(.kicker::after) { content: ''; width: 36px; height: 1px; background: rgb(var(--ink-rgb) / 0.3); order: 1; }
  .despues :global(.titulo) { margin: 0; font-family: var(--display); font-stretch: var(--ancho); font-weight: 700; font-size: clamp(2rem, 3.8vw, 3.4rem); line-height: 1.02; letter-spacing: -0.02em; text-wrap: balance; }
  .despues :global(.ejemplo) { margin: 0; font-size: 0.78rem; font-weight: 600; letter-spacing: 0.02em; color: var(--tinta-suave); }
  .despues :global(.puntos) { margin: 22px 0 0; padding: 0; list-style: none; display: grid; gap: 10px; }
  .despues :global(.puntos li) { position: relative; padding-left: 20px; line-height: 1.45; }
  .despues :global(.puntos li::before) { content: ''; position: absolute; left: 0; top: 0.7em; width: 9px; height: 2px; background: var(--bermellon); }

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
  /* celular: la barra ocupa el ancho, «Entrar» sigue visible y en pantallas muy angostas se queda solo la marca del logo */
  @media (max-width: 420px) {
    .barra { right: 14px; }
    .barra :global(.logo) { margin-right: auto; }
    .entrar { display: inline-flex; align-items: center; min-height: 36px; padding: 0 12px; box-shadow: inset 0 0 0 1px rgb(var(--ink-rgb) / 0.3); }
  }
  @media (max-width: 360px) { .barra :global(.word) { display: none; } }
</style>
