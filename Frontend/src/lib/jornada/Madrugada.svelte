<script>
  // 02:13. La reserva en línea trabaja de noche: una reserva móvil (la que ve el cliente) y, al lado, el registro de lo que fue entrando.
  // Al llegar a la pantalla, cada reserva toma su horario en el calendario, en orden. Con «reducir movimiento» todo se ve ya completo.
  import { vista } from './vista.js';

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

<section id="madrugada" class="madrugada" aria-labelledby="t-madrugada">
  <div class="envoltura">
    <div class="texto">
      <p class="kicker"><time>02:13</time>Mientras dormías</p>
      <h2 id="t-madrugada" class="titulo">Mientras dormías, se llenó el jueves.</h2>
      <p class="lead">Tu página de reservas trabaja toda la noche. Tus clientes eligen servicio, persona y hora, y solo ven lo libre: lo ocupado dice «Ocupado», sin contar de quién.</p>
      <ul class="puntos">
        <li>El horario se aparta mientras llenan sus datos.</li>
        <li>Recordatorio por correo y WhatsApp antes de la cita.</li>
        <li>Sin apps ni cuentas: reservan desde un enlace en tu Instagram.</li>
      </ul>
    </div>

    <div class="pieza" use:vista>
      <!-- Lo que ve la clienta, en su celular -->
      <figure class="reserva" aria-label="Página pública de reservas, jueves 19 de noviembre (ejemplo ilustrativo)">
        <p class="negocio">Barbería y Salón Ébano</p>
        <p class="mes">Jueves 19 de noviembre</p>
        <div class="cal" aria-hidden="true">
          {#each ['L', 'M', 'M', 'J', 'V', 'S', 'D'] as d}<i>{d}</i>{/each}
          {#each dias as d}<span class:libre={libres.has(d)} class:hoy={d === 19}>{d}</span>{/each}
        </div>
        <div class="slots">
          {#each horarios as h, i}
            {@const n = avisos.findIndex((a) => a.slot === i)}
            <span class:ocupado={ocupados.has(i)} class:tomado={n >= 0} style="--n: {n}">
              {h}<small>{ocupados.has(i) || n >= 0 ? 'Ocupado' : 'Libre'}</small>
            </span>
          {/each}
        </div>
        <figcaption class="ejemplo">Ejemplo ilustrativo</figcaption>
      </figure>

      <!-- Lo que fue entrando -->
      <div class="registro">
        <h3>Reservas de la madrugada</h3>
        <ol aria-label="Reservas recibidas de madrugada (ejemplo ilustrativo)">
          {#each avisos as a, i}
            <li style="--n: {i}">
              <time>{a.hora}</time>
              <span><b>{a.quien}</b> reservó {a.que}</span>
              <small>{a.cuando}</small>
            </li>
          {/each}
        </ol>
        <p class="ejemplo">Ejemplo ilustrativo</p>
      </div>
    </div>
  </div>
</section>

<style>
  .madrugada { padding: clamp(72px, 9vw, 112px) 0; background: var(--papel-2); color: var(--tinta); }
  .envoltura { display: grid; grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr); align-items: center; gap: clamp(32px, 5vw, 80px); }
  .titulo { max-width: 14ch; }
  .lead { margin: 20px 0 0; max-width: 44ch; line-height: 1.55; color: var(--tinta-suave); text-wrap: pretty; }

  .pieza { display: grid; grid-template-columns: minmax(0, 280px) minmax(0, 1fr); align-items: start; gap: clamp(20px, 3vw, 40px); }

  /* La reserva móvil: una hoja blanca con borde fino, sin carcasa */
  .reserva { margin: 0; padding: 22px 18px 16px; border: 1px solid var(--border); border-radius: 8px; background: var(--carta); display: grid; gap: 10px; }
  .negocio { margin: 0; font-family: var(--display); font-stretch: var(--ancho); font-weight: 650; font-size: 1.05rem; }
  .mes { margin: 0; font-size: 0.8rem; color: var(--tinta-suave); }
  .cal { display: grid; grid-template-columns: repeat(7, 1fr); gap: 3px; font-size: 0.7rem; text-align: center; font-variant-numeric: tabular-nums; }
  .cal i { font-style: normal; font-weight: 700; color: var(--tinta-suave); }
  .cal span { padding: 5px 0; border-radius: 4px; color: rgb(var(--ink-rgb) / 0.32); }
  .cal .libre { color: var(--tinta); background: var(--papel-2); }
  .cal .hoy { background: var(--tinta); color: #fff; }
  .slots { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
  .slots span { display: flex; flex-direction: column; padding: 7px 9px; border-radius: 6px; font-size: 0.8rem; font-weight: 650; box-shadow: inset 0 0 0 1px rgb(var(--ink-rgb) / 0.22); font-variant-numeric: tabular-nums; }
  .slots small { font-size: 0.7rem; font-weight: 500; color: var(--tinta-suave); }
  .slots .ocupado, .slots .tomado { background: var(--papel-2); box-shadow: none; color: rgb(var(--ink-rgb) / 0.55); }
  .slots .tomado small { color: var(--tinta-suave); }
  .reserva .ejemplo { padding-top: 4px; }

  /* El registro lateral */
  .registro { padding-top: 6px; }
  h3 { margin: 0 0 6px; font-size: 0.82rem; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--tinta-suave); }
  ol { margin: 0; padding: 0; list-style: none; }
  ol li { display: grid; grid-template-columns: auto 1fr; gap: 2px 14px; padding: 16px 0; border-bottom: 1px solid var(--border); }
  ol li:first-child { border-top: 2px solid var(--tinta); }
  ol time { grid-row: span 2; font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: 1.5rem; line-height: 1.1; font-variant-numeric: tabular-nums; }
  ol small { color: var(--tinta-suave); }
  .registro .ejemplo { margin-top: 12px; }

  /* Movimiento que explica: la reserva llega y toma su horario en el calendario */
  @media (prefers-reduced-motion: no-preference) {
    .pieza:not(:global(.vista)) .registro li { opacity: 0; transform: translateY(14px); }
    .registro li { transition: opacity 500ms var(--ease), transform 500ms var(--ease); transition-delay: calc(var(--n) * 700ms + 200ms); }
    .pieza:not(:global(.vista)) .slots .tomado { background: transparent; box-shadow: inset 0 0 0 1px rgb(var(--ink-rgb) / 0.22); color: var(--tinta); }
    .slots .tomado { transition: background-color 400ms var(--ease), color 400ms var(--ease); transition-delay: calc(var(--n) * 700ms + 500ms); }
  }

  @media (max-width: 960px) {
    .envoltura { grid-template-columns: 1fr; }
  }
  @media (max-width: 600px) {
    .pieza { grid-template-columns: 1fr; }
    .reserva { max-width: 320px; }
  }
</style>
