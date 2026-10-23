<script>
  // Recorrido: el video del local avanza cuadro a cuadro con el scroll (barbería → pasillo → salón → cabina de pestañas)
  // y encima cambian los textos con parallax. Método tomado del motor de scroll-world: el video se carga como Blob
  // (siempre se puede adelantar y regresar, sin depender de rangos HTTP), los saltos no se apilan mientras el
  // decodificador sigue buscando, y la versión de celular es más ligera y con keyframes cada 4 cuadros.
  import { onMount } from 'svelte';
  import { scene, reduced } from './scroll.js';

  // Un texto por lugar del video. El video se detiene en cada lugar mientras se lee su texto (paradas)
  // y viaja sin texto encima entre uno y otro.
  const momentos = [
    { a: -1, b: 0.2, tono: '#c9714f', lugar: 'Barbería', titulo: 'Todo tu local, en una sola agenda.', texto: 'Agenda por persona que no deja empalmar, anticipos y caja para barberías, salones de uñas, pestañas y spas. Hecho en México.', h1: true },
    { a: 0.39, b: 0.66, tono: '#d2a24c', lugar: 'Salón', titulo: 'Color, uñas y estética, sin perseguir pagos.', texto: 'Servicios largos con su duración real, un anticipo que aparta el horario y la comisión de cada quien calculada sola.' },
    { a: 0.85, b: 2, tono: '#e3c3b6', lugar: 'Pestañas y spa', titulo: 'Y tus clientas vuelven solas.', texto: 'Recordatorios por WhatsApp, paquetes de sesiones y reactivación de quien dejó de venir.', cta: true }
  ];
  // [avance del scroll, segundo del video]: se queda quieto en 0 s (barbería), ~3 s (salón) y al final (cabina)
  const paradas = [[0, 0], [0.16, 0], [0.42, 2.97], [0.62, 2.97], [0.86, 5.92], [1, 5.92]];
  const segundo = (p) => {
    for (let i = 1; i < paradas.length; i++) {
      const [p0, t0] = paradas[i - 1], [p1, t1] = paradas[i];
      if (p <= p1) {
        const f = (p - p0) / (p1 - p0);
        const e = f < 0.5 ? 2 * f * f : 1 - Math.pow(-2 * f + 2, 2) / 2; // arranca y frena suave
        return t0 + (t1 - t0) * e;
      }
    }
    return paradas[paradas.length - 1][1];
  };

  let video = $state();
  let seccion = $state();
  let listo = $state(false);
  let estatico = $state(false);
  let actual = $state(0); // espacio del local en pantalla, para el riel

  onMount(() => {
    if (reduced()) {
      estatico = true;
      return;
    }
    const celular = matchMedia('(max-width: 860px), (hover: none) and (pointer: coarse)').matches;
    const src = celular ? '/video/recorrido-854.mp4' : '/video/recorrido-1280.mp4';
    let url = '';
    let vivo = true;
    let raf = 0;
    let mostrado = 0; // tiempo que se está mostrando (suavizado)

    fetch(src)
      .then((r) => r.blob())
      .then((b) => {
        if (!vivo) return;
        url = URL.createObjectURL(b);
        video.src = url;
        // iOS no pinta el primer cuadro hasta reproducir: play→pause en silencio
        video.play().then(() => video.pause()).catch(() => {});
        video.addEventListener('loadeddata', () => (listo = true), { once: true });
      })
      .catch(() => {
        /* sin video: se queda el póster y los textos siguen funcionando */
      });

    const paso = () => {
      raf = requestAnimationFrame(paso);
      const p = +getComputedStyle(seccion).getPropertyValue('--p') || 0;
      actual = p < 0.29 ? 0 : p < 0.74 ? 1 : 2;
      if (!listo || !video.duration) return;
      const meta = Math.min(video.duration - 0.05, segundo(p));
      mostrado += (meta - mostrado) * 0.18;
      if (Math.abs(meta - mostrado) < 0.002) mostrado = meta;
      // un salto a la vez: si el decodificador sigue buscando, se espera al siguiente cuadro
      if (!video.seeking && Math.abs(video.currentTime - mostrado) > 1 / 60) video.currentTime = mostrado;
    };
    raf = requestAnimationFrame(paso);

    return () => {
      vivo = false;
      cancelAnimationFrame(raf);
      if (url) URL.revokeObjectURL(url);
    };
  });
</script>

<section class="recorrido" class:estatico bind:this={seccion} use:scene style="--p: 0" aria-labelledby="t-recorrido">
  <div class="cuadro">
    <div class="pantalla" aria-hidden="true">
      <img class="poster" class:oculto={listo} src="/video/recorrido-poster.webp" alt="" fetchpriority="high" />
      <!-- svelte-ignore a11y_media_has_caption -->
      <video bind:this={video} muted playsinline preload="auto" tabindex="-1"></video>
      <div class="velo"></div>
    </div>

    {#each momentos as m, i}
      <div class="momento" class:primero={i === 0} style="--a: {m.a}; --b: {m.b}; --tono: {m.tono}">
        <p class="lugar">{m.lugar}</p>
        {#if m.h1}
          <h1 id="t-recorrido" class="titulo">{m.titulo}</h1>
        {:else}
          <h2 class="titulo">{m.titulo}</h2>
        {/if}
        <p class="texto">{m.texto}</p>
        {#if m.h1 || m.cta}
          <div class="acciones">
            <a class="btn" href="/registro">Probar 14 días gratis</a>
            {#if m.h1}<span class="pista">Desliza para recorrer el local</span>{:else}<a class="link claro" href="#planes">Ver planes</a>{/if}
          </div>
        {/if}
      </div>
    {/each}

    <ol class="riel" aria-hidden="true">
      {#each ['Barbería', 'Salón', 'Pestañas y spa'] as l, i}
        <li class:on={actual === i}>{l}</li>
      {/each}
    </ol>
    <p class="ejemplo">Recorrido ilustrativo generado con IA</p>
  </div>
</section>

<style>
  .recorrido { height: 620vh; background: #0c0c0e; color: #fff; }
  .cuadro { position: sticky; top: 0; height: 100svh; overflow: hidden; }

  /* El video, con un acercamiento lento que da profundidad al avanzar */
  .pantalla { position: absolute; inset: 0; transform: scale(calc(1.04 + var(--p) * 0.06)); transform-origin: 50% 55%; will-change: transform; }
  video, .poster { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; }
  .poster { z-index: 1; transition: opacity 400ms var(--ease); }
  .poster.oculto { opacity: 0; }
  .velo { position: absolute; inset: 0; z-index: 2; background: rgb(10 10 12 / 0.55); }

  /* Cada momento aparece en su tramo del recorrido; título y texto se mueven a distinta velocidad (parallax) */
  .momento {
    --on: calc(clamp(0, calc((var(--p) - var(--a)) * 16), 1) * clamp(0, calc((var(--b) - var(--p)) * 16), 1));
    --t: clamp(-1, calc((var(--p) - (var(--a) + var(--b)) / 2) * 6), 1);
    position: absolute; z-index: 3; left: clamp(20px, 6vw, 96px); bottom: clamp(72px, 14vh, 140px);
    width: min(720px, calc(100% - 40px));
    opacity: var(--on); pointer-events: none;
  }
  .momento.primero { --t: clamp(0, calc(var(--p) * 6), 1); }
  .momento:last-of-type { --t: clamp(-1, calc((var(--p) - 0.93) * 6), 0); }
  .lugar {
    margin: 0 0 14px; display: inline-flex; align-items: center; gap: 10px;
    font-family: var(--rotulo); font-stretch: 85%; font-weight: 700; font-size: 0.95rem; text-transform: uppercase; letter-spacing: 0.06em; color: var(--tono);
    transform: translate3d(0, calc(var(--t) * -18px), 0);
  }
  .lugar::before { content: ''; width: 22px; height: 2px; background: currentColor; }
  .titulo {
    margin: 0; font-family: var(--display); font-stretch: var(--ancho); font-weight: 700; font-size: clamp(2.4rem, 5.6vw, 5.2rem); line-height: 0.98; letter-spacing: -0.025em; text-wrap: balance;
    transform: translate3d(0, calc(var(--t) * -70px), 0);
  }
  .texto {
    margin: 20px 0 0; max-width: 48ch; font-size: clamp(1.02rem, 1.4vw, 1.2rem); line-height: 1.55; color: rgb(255 255 255 / 0.9); text-wrap: pretty; text-shadow: 0 1px 14px rgb(0 0 0 / 0.5);
    transform: translate3d(0, calc(var(--t) * -36px), 0);
  }
  .acciones { display: flex; flex-wrap: wrap; align-items: center; gap: 14px 24px; margin-top: 28px; pointer-events: auto; transform: translate3d(0, calc(var(--t) * -24px), 0); }
  .momento.primero .acciones, .momento:last-of-type .acciones { pointer-events: auto; }
  .pista { font-size: 0.9rem; color: rgb(255 255 255 / 0.75); }
  .claro { color: #fff; }

  /* Riel: en qué espacio del local vas */
  .riel { position: absolute; z-index: 3; right: clamp(20px, 4vw, 56px); top: 50%; transform: translateY(-50%); margin: 0; padding: 0; list-style: none; display: grid; gap: 14px; text-align: right; }
  .riel li {
    display: flex; align-items: center; justify-content: flex-end; gap: 10px; font-size: 0.82rem; font-weight: 600;
    color: rgb(255 255 255 / 0.45);
  }
  .riel li { transition: color 300ms var(--ease); }
  .riel li::after { content: ''; width: 18px; height: 2px; background: rgb(255 255 255 / 0.35); transition: width 300ms var(--ease), background-color 300ms var(--ease); }
  .riel li.on { color: #fff; }
  .riel li.on::after { width: 40px; background: #fff; }
  .ejemplo { position: absolute; z-index: 3; right: clamp(20px, 4vw, 56px); bottom: 16px; margin: 0; font-size: 0.72rem; color: rgb(255 255 255 / 0.55); }

  /* Reducir movimiento o sin JS: póster quieto y los momentos uno tras otro */
  .estatico { height: auto; }
  .estatico .cuadro { position: relative; height: auto; overflow: visible; }
  .estatico .pantalla { position: sticky; top: 0; height: 100svh; transform: none; }
  .estatico .momento { position: relative; left: auto; bottom: auto; opacity: 1; margin: -60svh auto 40svh; padding: 0 20px; pointer-events: auto; }
  .estatico .lugar, .estatico .titulo, .estatico .texto, .estatico .acciones { transform: none; }
  .estatico .momento:not(.primero):not(:last-of-type) .acciones { display: none; }
  .estatico .riel { display: none; }

  @media (max-width: 860px) {
    .recorrido { height: 480vh; }
    .momento { left: 20px; bottom: 88px; }
    .riel { top: auto; bottom: 22px; right: 20px; transform: none; display: flex; gap: 6px; }
    .riel li { font-size: 0; gap: 0; }
    .riel li::after { width: 22px; }
    .ejemplo { display: none; }
  }
</style>
