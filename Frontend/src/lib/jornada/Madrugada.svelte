<script>
  // 02:13. La reserva en línea trabaja de noche: cada aviso aparece en su minuto y aparta su horario en el calendario.
  import { scene } from './scroll.js';

  // Ejemplo ilustrativo
  const avisos = [
    { hora: '23:48', quien: 'Andrea', que: 'Acrílico almendra', cuando: 'jue 10:00', slot: 2 },
    { hora: '01:12', quien: 'Luis', que: 'Corte y barba', cuando: 'jue 11:00', slot: 4 },
    { hora: '02:13', quien: 'Mariela', que: 'Extensiones clásicas', cuando: 'jue 12:30', slot: 7 }
  ];
  const horarios = ['09:00', '09:30', '10:00', '10:30', '11:00', '11:30', '12:00', '12:30'];
  const ocupados = new Set([1, 5]);
  const dias = Array.from({ length: 30 }, (_, i) => i + 1);
  const libres = new Set([3, 4, 5, 6, 10, 11, 12, 13, 17, 18, 19, 20, 24, 25, 26, 27]);
</script>

<section id="madrugada" class="madrugada" use:scene data-hora="02:13" data-momento="Mientras dormías" aria-labelledby="t-madrugada">
  <div class="cuadro">
    <div class="texto">
      <p class="hora">02:13</p>
      <h2 id="t-madrugada">Mientras dormías, se llenó el jueves.</h2>
      <p>Tu página de reservas trabaja toda la noche. Tus clientes eligen servicio, persona y hora, y solo ven lo libre: lo ocupado dice «Ocupado», sin contar de quién.</p>
      <ul>
        <li>El horario se aparta mientras llenan sus datos.</li>
        <li>Recordatorio por correo y WhatsApp antes de la cita.</li>
        <li>Sin apps ni cuentas: reservan desde un enlace en tu Instagram.</li>
      </ul>
    </div>

    <div class="telefono" aria-hidden="true">
      <div class="pantalla">
        <p class="negocio">Barbería y Salón Ébano</p>
        <p class="mes">Jueves 19 de noviembre</p>
        <div class="cal">
          {#each ['L', 'M', 'M', 'J', 'V', 'S', 'D'] as d}<i>{d}</i>{/each}
          {#each dias as d}<span class:libre={libres.has(d)} class:hoy={d === 19}>{d}</span>{/each}
        </div>
        <div class="slots">
          {#each horarios as h, i}
            {@const tomado = avisos.findIndex((a) => a.slot === i)}
            <span class:ocupado={ocupados.has(i)} class:tomado={tomado >= 0} style="--n: {tomado}">
              {h}{#if ocupados.has(i)}<small>Ocupado</small>{:else if tomado >= 0}<small class="ya">Ocupado</small>{/if}
            </span>
          {/each}
        </div>
      </div>
    </div>

    <ol class="avisos" aria-label="Reservas recibidas de madrugada (ejemplo ilustrativo)">
      {#each avisos as a, i}
        <li style="--n: {i}">
          <time>{a.hora}</time>
          <span><b>{a.quien}</b> reservó {a.que}</span>
          <small>{a.cuando}</small>
        </li>
      {/each}
    </ol>
  </div>
</section>

<style>
  .madrugada { height: 260vh; background: var(--noche); color: var(--ticket); }
  .cuadro {
    position: sticky; top: 0; height: 100svh; overflow: hidden;
    display: grid; grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.8fr) minmax(0, 0.9fr); align-items: center; gap: clamp(20px, 3vw, 48px);
    padding: 88px clamp(16px, 4vw, 56px) 40px;
  }
  .hora { margin: 0; font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: clamp(3rem, 7vw, 6rem); line-height: 1; color: var(--luna); font-variant-numeric: tabular-nums; }
  h2 { margin: 8px 0 18px; font-family: var(--display); font-stretch: var(--ancho); font-weight: 650; font-size: clamp(2rem, 3.6vw, 3.3rem); line-height: 1.02; max-width: 14ch; text-wrap: balance; }
  .texto > p:not(.hora) { margin: 0; max-width: 44ch; line-height: 1.55; color: rgb(246 245 241 / 0.78); text-wrap: pretty; }
  ul { margin: 22px 0 0; padding: 0; list-style: none; display: grid; gap: 10px; }
  li { line-height: 1.4; }
  ul li { padding-left: 18px; position: relative; color: rgb(246 245 241 / 0.85); }
  ul li::before { content: ''; position: absolute; left: 0; top: 0.6em; width: 8px; height: 2px; background: var(--luna); }

  /* Teléfono: la página pública de reservas */
  .telefono { justify-self: center; width: min(280px, 100%); aspect-ratio: 9 / 18.5; padding: 10px; border-radius: 38px; background: #050608; box-shadow: inset 0 0 0 2px #2a2d33, 0 30px 60px -20px rgb(0 0 0 / 0.8); }
  .pantalla { height: 100%; border-radius: 30px; background: var(--ticket); color: var(--tinta); padding: 26px 16px 16px; display: grid; grid-template-rows: auto auto auto 1fr; gap: 10px; overflow: hidden; }
  .negocio { margin: 0; font-family: var(--display); font-stretch: var(--ancho); font-weight: 650; font-size: 1.05rem; }
  .mes { margin: 0; font-size: 0.78rem; color: var(--tinta-suave); }
  .cal { display: grid; grid-template-columns: repeat(7, 1fr); gap: 3px; font-size: 0.68rem; text-align: center; }
  .cal i { font-style: normal; font-weight: 700; color: var(--tinta-suave); }
  .cal span { padding: 4px 0; border-radius: 6px; color: rgb(20 20 22 / 0.3); }
  .cal .libre { color: var(--tinta); background: rgb(20 20 22 / 0.05); }
  .cal .hoy { background: var(--tinta); color: var(--ticket); }
  .slots { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; align-content: start; }
  .slots span { display: flex; flex-direction: column; padding: 7px 8px; border-radius: 8px; font-size: 0.78rem; font-weight: 600; box-shadow: inset 0 0 0 1px rgb(20 20 22 / 0.16); font-variant-numeric: tabular-nums; }
  .slots small { font-size: 0.66rem; font-weight: 500; color: var(--tinta-suave); }
  .slots .ocupado { color: rgb(20 20 22 / 0.4); background: rgb(20 20 22 / 0.05); box-shadow: none; }
  /* el horario se toma justo cuando llega su aviso */
  .slots .tomado { --on: clamp(0, calc((var(--p) - 0.18 - var(--n) * 0.22) * 8), 1); background: color-mix(in oklab, var(--tinta) calc(var(--on) * 6%), transparent); color: color-mix(in oklab, var(--tinta) calc(100% - var(--on) * 60%), transparent); }
  .slots .ya { opacity: var(--on); }

  /* Avisos que llegan de madrugada */
  .avisos { margin: 0; padding: 0; list-style: none; display: grid; gap: 12px; }
  .avisos li {
    --on: clamp(0, calc((var(--p) - 0.12 - var(--n) * 0.22) * 6), 1);
    display: grid; grid-template-columns: auto 1fr; gap: 2px 12px; padding: 14px 16px; border-radius: 14px;
    background: rgb(246 245 241 / 0.08); box-shadow: inset 0 0 0 1px rgb(246 245 241 / 0.12);
    opacity: var(--on); transform: translate3d(0, calc((1 - var(--on)) * 24px), 0);
  }
  .avisos time { grid-row: span 2; font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: 1.3rem; color: var(--luna); font-variant-numeric: tabular-nums; }
  .avisos small { color: rgb(246 245 241 / 0.6); }

  :global(.madrugada.still) { height: auto; }
  :global(.madrugada.still) .cuadro { position: relative; height: auto; min-height: 100svh; }

  @media (max-width: 960px) {
    .madrugada { height: auto; }
    .cuadro { position: relative; height: auto; grid-template-columns: 1fr; padding-block: 72px; }
    .avisos li, .slots .tomado { --on: 1; }
  }
</style>
