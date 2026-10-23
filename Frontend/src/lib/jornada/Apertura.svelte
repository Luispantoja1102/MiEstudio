<script>
  // Apertura: la semana del local a pantalla completa. Con el scroll entran las citas, oficio por oficio
  // (barbería, uñas, pestañas, spa), hasta llenar la pantalla. El titular es una cita más de esa agenda,
  // y su palabra cambia con el oficio que está entrando.
  import { scene } from './scroll.js';

  const oficios = [
    { nombre: 'barbería', tono: 'var(--cuero)', suave: '#eadfd8', servicios: [['Corte', 2], ['Fade', 2], ['Barba', 1], ['Corte y barba', 3]] },
    { nombre: 'uñas', tono: 'var(--esmalte)', suave: '#f1dfdc', servicios: [['Gelish', 2], ['Acrílico', 5], ['Pedicure', 2], ['Retoque', 2]] },
    { nombre: 'pestañas', tono: 'var(--tinta)', suave: '#e4e4ea', servicios: [['Extensiones', 4], ['Relleno', 2], ['Lifting', 2]] },
    { nombre: 'spa', tono: 'var(--eucalipto)', suave: '#dde7e0', servicios: [['Masaje', 2], ['Facial', 3], ['Piedras calientes', 3]] }
  ];
  const clientes = ['Andrea', 'Luis', 'Mariela', 'Toño', 'Paola', 'Karla', 'Iker', 'Fer', 'Ivonne', 'Rubén', 'Dulce', 'Beto', 'Sara', 'Memo', 'Lupita', 'Óscar', 'Nayeli', 'Chuy'];
  const dias = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb'];
  const FILAS = 22; // de 9:00 a 20:00 en medias horas
  // El titular ocupa martes a viernes, de 11:30 a 16:30
  const titular = { d0: 1, d1: 4, r0: 5, r1: 15 };

  // Agenda de ejemplo, siempre la misma (generador con semilla)
  let semilla = 7;
  const azar = () => ((semilla = (semilla * 16807) % 2147483647) - 1) / 2147483646;
  const citas = [];
  for (let d = 0; d < dias.length; d++) {
    let r = Math.floor(azar() * 2);
    while (r < FILAS) {
      const tapa = d >= titular.d0 && d <= titular.d1;
      if (tapa && r >= titular.r0 && r < titular.r1) {
        r = titular.r1;
        continue;
      }
      const o = Math.floor(azar() * 4);
      const [servicio, largo] = oficios[o].servicios[Math.floor(azar() * oficios[o].servicios.length)];
      let fin = Math.min(FILAS, r + largo);
      if (tapa && r < titular.r0 && fin > titular.r0) fin = titular.r0;
      if (fin - r >= 1) {
        const h = 9 + r / 2;
        citas.push({
          d, r, fin, o, servicio,
          cliente: clientes[Math.floor(azar() * clientes.length)],
          hora: `${Math.floor(h)}:${h % 1 ? '30' : '00'}`,
          t: o * 0.21 + azar() * 0.17, // entra en el tramo de su oficio
          choque: d === 0 && r < titular.r1 && fin > titular.r0
        });
      }
      r = fin + (azar() < 0.35 ? 1 : 0);
    }
  }
  const horas = Array.from({ length: 12 }, (_, i) => `${9 + i}:00`);
</script>

<section class="apertura" use:scene style="--p: 1" data-hora="09:00" data-momento="Se abre la semana" aria-labelledby="titulo">
  <div class="cuadro">
    <div class="semana" style="--filas: {FILAS}">
      <div class="dias" aria-hidden="true">{#each dias as d, i}<span class:fin={i >= 3}>{d}</span>{/each}</div>
      <ol class="horas" aria-hidden="true">{#each horas as h, i}<li style="--i: {i}">{h}</li>{/each}</ol>

      {#each citas as c}
        <span
          class="cita"
          class:mini={c.fin - c.r < 2}
          class:fin={c.d >= 3}
          class:choque={c.choque}
          aria-hidden="true"
          style="grid-column: {c.d + 2}; grid-row: {c.r + 2} / {c.fin + 2}; --t: {c.t.toFixed(3)}; --tono: {oficios[c.o].tono}; --suave: {oficios[c.o].suave}"
        ><b>{c.servicio}</b><small>{c.hora} · {c.cliente}</small></span>
      {/each}

      <!-- El titular: la cita más grande de la semana -->
      <div class="titular" style="grid-column: {titular.d0 + 2} / {titular.d1 + 3}; grid-row: {titular.r0 + 2} / {titular.r1 + 2}">
        <h1 id="titulo">
          Tu agenda de
          <span class="rueda">
            {#each oficios as o, i}<span class="palabra" class:ultima={i === 3} aria-hidden="true" style="--i: {i}; color: {o.tono}">{o.nombre}</span>{/each}
            <span class="sr">barbería, uñas, pestañas o spa</span>
          </span>
          se llena sola.
        </h1>
        <div class="lado">
          <p>Tus clientes reservan a cualquier hora, dejan su anticipo y tú cobras en pesos. Citas, caja y recordatorios en un solo lugar.</p>
          <div class="acciones">
            <a class="btn" href="/registro">Probar 14 días gratis</a>
            <a class="link" href="#madrugada">Así es un día</a>
          </div>
        </div>
      </div>
    </div>
    <p class="leyenda">Semana de ejemplo · desliza para llenarla</p>
  </div>
</section>

<style>
  .apertura { height: 300vh; background: var(--ticket); color: var(--tinta); }
  .cuadro { position: sticky; top: 0; height: 100svh; overflow: hidden; padding: 76px clamp(10px, 2vw, 28px) 34px; }

  .semana {
    position: relative; height: 100%; display: grid; gap: 3px 6px;
    grid-template-columns: 44px repeat(6, minmax(0, 1fr));
    grid-template-rows: 26px repeat(var(--filas), minmax(0, 1fr));
  }
  .dias { display: contents; }
  .dias span { grid-row: 1; font-weight: 700; font-size: 0.85rem; padding-left: 2px; }
  .dias span:nth-child(1) { grid-column: 2; }
  .dias span:nth-child(2) { grid-column: 3; }
  .dias span:nth-child(3) { grid-column: 4; }
  .dias span:nth-child(4) { grid-column: 5; }
  .dias span:nth-child(5) { grid-column: 6; }
  .dias span:nth-child(6) { grid-column: 7; }
  /* líneas de cada hora: la retícula es la agenda misma */
  .horas { position: absolute; left: 0; right: 0; top: 29px; bottom: 0; margin: 0; padding: 0; list-style: none; pointer-events: none; }
  .horas li { position: absolute; left: 0; right: 0; top: calc(var(--i) * 2 / var(--filas) * 100%); padding-top: 2px; border-top: 1px solid rgb(20 20 22 / 0.07); font-size: 0.72rem; color: var(--tinta-suave); font-variant-numeric: tabular-nums; }

  .cita {
    --on: clamp(0, calc((var(--p) * 1.06 - var(--t)) * 14), 1);
    position: relative; z-index: 1; min-width: 0; overflow: hidden;
    display: flex; flex-direction: column; justify-content: flex-start; gap: 1px; padding: 5px 8px;
    border-radius: 6px; background: var(--suave); box-shadow: inset 3px 0 0 var(--tono);
    opacity: var(--on); transform: translate3d(0, calc((1 - var(--on)) * 10px), 0) scale(calc(0.96 + var(--on) * 0.04));
  }
  .cita b { font-size: 0.82rem; font-weight: 650; line-height: 1.15; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .cita small { font-size: 0.7rem; color: var(--tinta-suave); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-variant-numeric: tabular-nums; }
  .cita.mini { justify-content: center; padding-block: 0; }
  .cita.mini small { display: none; }

  .titular {
    position: relative; z-index: 2; display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr); align-items: end; gap: 18px clamp(20px, 3vw, 40px);
    padding: clamp(20px, 3vw, 44px); border-radius: 10px; background: var(--ticket);
    box-shadow: inset 0 0 0 1.5px var(--tinta);
  }
  h1 { margin: 0; align-self: center; font-family: var(--display); font-stretch: var(--ancho); font-weight: 700; font-size: clamp(2.2rem, 3.9vw, 4.2rem); line-height: 0.98; letter-spacing: -0.025em; }
  .lado { display: grid; gap: 18px; }
  .rueda { display: grid; }
  .palabra {
    grid-area: 1 / 1;
    /* cada palabra se ve mientras entran las citas de su oficio */
    --a: clamp(0, calc((var(--p) - var(--i) * 0.21) * 70), 1);
    --b: clamp(0, calc(((var(--i) + 1) * 0.21 - var(--p)) * 70), 1);
    opacity: calc(var(--a) * var(--b));
    transform: translate3d(0, calc((1 - var(--a)) * 0.25em - (1 - var(--b)) * 0.25em), 0);
  }
  .palabra:first-child { --a: 1; }
  .palabra.ultima { --b: 1; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
  .titular p { margin: 0; max-width: 46ch; font-size: clamp(1rem, 1.25vw, 1.15rem); line-height: 1.5; color: var(--tinta-suave); text-wrap: pretty; }
  .acciones { display: flex; flex-wrap: wrap; align-items: center; gap: 12px 22px; }
  .leyenda { position: absolute; left: 0; right: 0; bottom: 8px; margin: 0; text-align: center; font-size: 0.75rem; color: var(--tinta-suave); }

  :global(.apertura.still) { height: auto; }
  :global(.apertura.still) .cuadro { position: relative; height: 100svh; }

  @media (max-width: 900px) {
    .apertura { height: 240vh; }
    .semana { grid-template-columns: 0 repeat(3, minmax(0, 1fr)); gap: 3px 4px; }
    .dias span.fin, .cita.fin, .cita.choque { display: none; }
    .horas { display: none; }
    .titular { grid-column: 2 / 5 !important; grid-template-columns: 1fr; align-content: center; padding: 20px; gap: 14px; }
    h1 { font-size: clamp(1.9rem, 8.5vw, 2.6rem); }
    .titular p { font-size: 0.95rem; }
    .cita { padding: 3px 6px; }
    .cita b { font-size: 0.72rem; }
    .cita small { display: none; }
  }
</style>
