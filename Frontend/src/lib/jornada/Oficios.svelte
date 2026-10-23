<script>
  // 12:40 → 18:00. Cuatro sillas en un recorrido horizontal. El dibujo de cada una sale de su propio oficio:
  // las guías de la máquina, los esmaltes, el mapa de pestañas y la tarjeta de sesiones.
  import { scene } from './scroll.js';

  const guias = [['#4', '12 mm'], ['#3', '9 mm'], ['#2', '6 mm'], ['#1', '3 mm'], ['#0', '1.5 mm']];
  const esmaltes = [['Cereza', '#9e1b22'], ['Nude rosado', '#d9a18f'], ['Ciruela', '#4f1f36'], ['Coral', '#e0614a'], ['Perla', '#efe6dc'], ['Azabache', '#141416']];
  // mapa de pestañas: largo en mm de la esquina interna a la externa
  const largos = [8, 9, 10, 11, 12, 12, 11, 10, 9];
</script>

<section id="oficios" class="oficios" use:scene data-steps="4" style="--s: 0" aria-labelledby="t-oficios">
  <h2 id="t-oficios" class="sr">Para cada oficio</h2>
  <div class="cuadro">
    <div class="pista">
      <!-- Barbería -->
      <article class="silla barberia" style="--i: 0" data-hora="12:40" data-momento="Barbería">
        <div class="texto">
          <p class="hora">12:40 · Barbería</p>
          <h3>Citas de 40 minutos, una tras otra.</h3>
          <p>Agenda por barbero que no deja empalmar, la fila del día a la vista y la comisión de cada servicio calculada sola.</p>
        </div>
        <figure class="dibujo" aria-label="Las guías de la máquina, de la más larga a la más corta, como en un degradado">
          {#each guias as [n, mm], k}
            <div class="guia" style="--k: {k}"><b>{n}</b><span class="barra"></span><small>{mm}</small></div>
          {/each}
          <figcaption>Degradado: de la guía 4 a la 0</figcaption>
        </figure>
      </article>

      <!-- Uñas -->
      <article class="silla unas" style="--i: 1" data-hora="14:00" data-momento="Uñas">
        <div class="texto">
          <p class="hora">14:00 · Uñas</p>
          <h3>Tres horas de acrílico, apartadas con anticipo.</h3>
          <p>Servicios largos con su duración real, anticipo para que nadie te deje plantada y recordatorio por WhatsApp antes de la cita.</p>
        </div>
        <figure class="dibujo" aria-label="Seis tonos de esmalte">
          <div class="unas-fila">
            {#each esmaltes as [nombre, color], k}
              <div class="tono" style="--k: {k}; --c: {color}"><span class="una"></span><small>{nombre}</small></div>
            {/each}
          </div>
        </figure>
      </article>

      <!-- Pestañas -->
      <article class="silla pestanas" style="--i: 2" data-hora="16:30" data-momento="Pestañas">
        <div class="texto">
          <p class="hora">16:30 · Pestañas</p>
          <h3>Tus clientas vuelven a su relleno.</h3>
          <p>Reactivación para quien dejó de venir, lista de espera cuando se libera un lugar y felicitación de cumpleaños con tu nombre.</p>
        </div>
        <figure class="dibujo" aria-label="Mapa de pestañas: largos de 8 a 12 milímetros a lo largo del ojo">
          <svg viewBox="0 0 400 200" role="presentation">
            <path class="parpado" d="M30 150 Q200 40 370 150" />
            {#each largos as l, k}
              {@const t = (k + 0.5) / largos.length}
              {@const x = 30 + 340 * t}
              {@const y = 150 - 4 * t * (1 - t) * 110 + 2}
              {@const ang = (t - 0.5) * 70}
              <g transform="translate({x} {y}) rotate({ang})" style="--k: {k}">
                <line class="pestana" x1="0" y1="0" x2="0" y2={-l * 6.2} pathLength="1" />
                <text x="0" y={-l * 6.2 - 10} text-anchor="middle">{l}</text>
              </g>
            {/each}
          </svg>
          <figcaption>Mapa de largos en milímetros</figcaption>
        </figure>
      </article>

      <!-- Spa -->
      <article class="silla spa" style="--i: 3" data-hora="18:00" data-momento="Spa">
        <div class="texto">
          <p class="hora">18:00 · Spa</p>
          <h3>Diez masajes pagados, canjeados uno por uno.</h3>
          <p>Paquetes y membresías que vendes por adelantado y se canjean en caja, con el saldo de cada cliente siempre a la vista.</p>
        </div>
        <figure class="dibujo tarjeta" aria-label="Tarjeta de un paquete de diez masajes con cuatro sesiones usadas">
          <p class="t-nombre">Paquete 10 masajes <span>Fer Ruiz</span></p>
          <div class="hoyos">
            {#each Array(10) as _, k}<span class:usada={k < 4} style="--k: {k}"></span>{/each}
          </div>
          <p class="t-saldo">Quedan <b>6</b> sesiones</p>
        </figure>
      </article>
    </div>
  </div>
</section>

<style>
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
  .oficios { height: 440vh; }
  .cuadro { position: sticky; top: 0; height: 100svh; overflow: hidden; }
  .pista { display: flex; width: 400vw; height: 100%; transform: translate3d(calc(var(--s) * -300vw), 0, 0); will-change: transform; }
  .silla {
    --q: clamp(0, calc((var(--s) * 3 - var(--i)) * 2.2 + 1), 1);
    flex: none; width: 100vw; height: 100%; display: grid; grid-template-columns: minmax(0, 0.95fr) minmax(0, 1.05fr); align-items: center; gap: clamp(24px, 4vw, 72px);
    padding: 88px clamp(16px, 5vw, 80px) 40px;
  }
  .hora { margin: 0 0 14px; font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: 1.15rem; text-transform: uppercase; letter-spacing: 0.04em; font-variant-numeric: tabular-nums; }
  h3 { margin: 0 0 18px; font-family: var(--display); font-weight: 600; font-size: clamp(2.2rem, 4.6vw, 4.2rem); line-height: 1; max-width: 13ch; text-wrap: balance; }
  .texto > p:last-child { margin: 0; max-width: 42ch; font-size: 1.05rem; line-height: 1.55; text-wrap: pretty; }
  figure { margin: 0; }
  figcaption { margin-top: 18px; font-size: 0.82rem; opacity: 0.7; }

  /* Barbería: cuero de sillón y acero */
  .barberia { background: var(--cuero); color: var(--ticket); }
  .barberia .hora { color: #e9b9a6; }
  .barberia .texto > p:last-child { color: rgb(246 245 241 / 0.8); }
  .guia { display: grid; grid-template-columns: 44px 1fr 64px; align-items: center; gap: 14px; padding: 9px 0; border-bottom: 1px solid rgb(246 245 241 / 0.16); }
  .guia b { font-family: var(--rotulo); font-stretch: 75%; font-size: 1.6rem; }
  .guia small { text-align: right; font-variant-numeric: tabular-nums; opacity: 0.8; }
  .barra { height: 18px; border-radius: 2px; background: #b4babf; box-shadow: inset 0 -5px 0 #8e959b; transform-origin: left; transform: scaleX(calc((1 - var(--k) * 0.19) * clamp(0, calc(var(--q) * 1.4 - var(--k) * 0.08), 1))); }

  /* Uñas: el tono nude de la base */
  .unas { background: #ecd9d1; color: var(--tinta); }
  .unas .hora { color: #9e1b22; }
  .unas .texto > p:last-child { color: var(--tinta-suave); }
  .unas-fila { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 28px 20px; max-width: 440px; }
  .tono { display: grid; justify-items: center; gap: 10px; }
  .una { width: 64px; height: 92px; border-radius: 50% 50% 46% 46% / 64% 64% 36% 36%; background: var(--c); box-shadow: inset 0 0 0 1px rgb(20 20 22 / 0.12), inset 8px 10px 0 -4px rgb(255 255 255 / 0.28);
    /* el esmalte se pinta de la cutícula a la punta */
    clip-path: inset(0 0 calc((1 - clamp(0, calc(var(--q) * 1.6 - var(--k) * 0.12), 1)) * 100%) 0 round 32px); }
  .tono small { font-size: 0.85rem; font-weight: 600; }

  /* Pestañas: tinta */
  .pestanas { background: var(--tinta); color: var(--ticket); }
  .pestanas .hora { color: #efc6c4; }
  .pestanas .texto > p:last-child { color: rgb(246 245 241 / 0.78); }
  svg { width: 100%; max-width: 520px; overflow: visible; }
  .parpado { fill: none; stroke: rgb(246 245 241 / 0.5); stroke-width: 2; }
  .pestana { stroke: var(--ticket); stroke-width: 2.5; stroke-linecap: round; stroke-dasharray: 1; stroke-dashoffset: calc(1 - clamp(0, calc(var(--q) * 1.5 - var(--k) * 0.06), 1)); }
  svg text { fill: #efc6c4; font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: 15px; opacity: clamp(0, calc(var(--q) * 1.5 - var(--k) * 0.06), 1); }

  /* Spa: eucalipto */
  .spa { background: var(--eucalipto); color: var(--ticket); }
  .spa .hora { color: #cfe0d2; }
  .spa .texto > p:last-child { color: rgb(246 245 241 / 0.82); }
  .tarjeta { max-width: 440px; padding: 28px; border-radius: 14px; background: var(--ticket); color: var(--tinta); box-shadow: 0 30px 50px -28px rgb(0 0 0 / 0.55); }
  .t-nombre { margin: 0 0 22px; display: flex; justify-content: space-between; gap: 12px; font-family: var(--display); font-size: 1.4rem; font-weight: 600; }
  .t-nombre span { font-family: var(--texto); font-size: 0.9rem; font-weight: 500; color: var(--tinta-suave); align-self: center; }
  .hoyos { display: grid; grid-template-columns: repeat(5, 44px); justify-content: space-between; gap: 16px; }
  .hoyos span { aspect-ratio: 1; border-radius: 50%; box-shadow: inset 0 0 0 2px rgb(20 20 22 / 0.25); }
  .hoyos .usada { background: var(--eucalipto); box-shadow: inset 0 3px 4px rgb(0 0 0 / 0.35); transform: scale(clamp(0, calc(var(--q) * 1.8 - var(--k) * 0.2), 1)); }
  .t-saldo { margin: 22px 0 0; color: var(--tinta-suave); }
  .t-saldo b { color: var(--tinta); font-variant-numeric: tabular-nums; }

  :global(.oficios.still) { height: auto; }
  :global(.oficios.still) .cuadro { position: relative; height: auto; }
  :global(.oficios.still) .pista { display: block; width: auto; transform: none; }
  :global(.oficios.still) .silla { --q: 1; width: auto; min-height: 100svh; }

  @media (max-width: 860px) {
    .oficios { height: auto; }
    .cuadro { position: relative; height: auto; }
    .pista { display: block; width: auto; transform: none; }
    .silla { --q: 1; width: auto; height: auto; grid-template-columns: 1fr; padding-block: 72px; }
  }
</style>
