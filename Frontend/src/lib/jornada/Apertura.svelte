<script>
  // 09:00. La apertura: el titular a la izquierda y la agenda del día a la derecha, que se llena con el scroll,
  // carril por carril (barbería, uñas, pestañas, spa).
  import { scene } from './scroll.js';

  const sillas = [
    { quien: 'Marco', oficio: 'Barbería', tono: 'var(--cuero)', suave: '#efe5df' },
    { quien: 'Dani', oficio: 'Uñas', tono: 'var(--esmalte)', suave: '#f4e4e2' },
    { quien: 'Sofi', oficio: 'Pestañas', tono: 'var(--tinta)', suave: '#e9e9ee' },
    { quien: 'Lu', oficio: 'Spa', tono: 'var(--eucalipto)', suave: '#e2ebe5' }
  ];
  // Ejemplo ilustrativo: [silla, inicio desde las 9:00 (h), duración (h), servicio, cliente]
  const citas = [
    [0, 0, 0.67, 'Corte y barba', 'Luis R.'], [0, 0.75, 0.67, 'Fade', 'Toño'], [0, 1.5, 0.67, 'Corte clásico', 'Iker'], [0, 2.5, 0.67, 'Barba', 'Rubén'],
    [1, 0, 2, 'Acrílico almendra', 'Andrea'], [1, 2.25, 1, 'Gelish', 'Paola'],
    [2, 0.5, 2, 'Extensiones clásicas', 'Mariela'], [2, 2.75, 1, 'Relleno', 'Karla'],
    [3, 1, 1, 'Masaje relajante', 'Fer'], [3, 2.25, 1.25, 'Facial hidratante', 'Ivonne']
  ];
  const horas = ['9:00', '10:00', '11:00', '12:00'];
</script>

<section class="apertura" use:scene style="--p: 1" data-hora="09:00" data-momento="Se abre el local" aria-labelledby="titulo">
  <div class="cuadro">
    <div class="texto">
      <p class="oficios">Barbería · Uñas · Pestañas · Spa</p>
      <h1 id="titulo">Tu agenda se llena sola.</h1>
      <p class="bajada">Citas en línea, anticipos y caja para tu barbería o salón. Tus clientes reservan a cualquier hora, tú confirmas el anticipo y cobras en pesos.</p>
      <div class="acciones">
        <a class="btn" href="/registro">Probar 14 días gratis</a>
        <a class="link" href="#madrugada">Ver un día completo</a>
      </div>
      <p class="nota">Sin tarjeta para empezar · Hecho en México</p>
    </div>

    <figure class="agenda" aria-label="Ejemplo de la agenda de un jueves con cuatro personas: barbería, uñas, pestañas y spa">
      <div class="cab" aria-hidden="true">
        <span></span>
        {#each sillas as s}<p style="--tono: {s.tono}"><b>{s.quien}</b><small>{s.oficio}</small></p>{/each}
      </div>
      <div class="cuerpo" aria-hidden="true">
        <ol class="horas">{#each horas as h, k}<li style="--k: {k}">{h}</li>{/each}</ol>
        {#each sillas as s, i}
          <div class="carril">
            {#each citas.filter((c) => c[0] === i) as [, ini, dur, servicio, cliente]}
              <span class="cita" class:corta={dur < 1} style="--ini: {ini}; --dur: {dur}; --tono: {s.tono}; --suave: {s.suave}"><b>{servicio}</b><small>{cliente}</small></span>
            {/each}
          </div>
        {/each}
      </div>
      <figcaption>Jueves · ejemplo ilustrativo</figcaption>
    </figure>
  </div>
</section>

<style>
  .apertura { height: 190vh; background: var(--ticket); color: var(--tinta); }
  .cuadro {
    position: sticky; top: 0; height: 100svh; overflow: hidden;
    display: grid; grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr); align-items: center; gap: clamp(28px, 4vw, 72px);
    padding: 96px clamp(16px, 4vw, 56px) 40px;
  }
  .oficios { margin: 0 0 18px; font-family: var(--rotulo); font-stretch: 85%; font-weight: 700; font-size: 0.95rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--bermellon); }
  h1 { margin: 0; max-width: 11ch; font-family: var(--display); font-stretch: var(--ancho); font-weight: 700; font-size: clamp(2.8rem, 6vw, 5.4rem); line-height: 0.98; letter-spacing: -0.025em; text-wrap: balance; }
  .bajada { margin: 22px 0 0; max-width: 44ch; font-size: clamp(1.05rem, 1.4vw, 1.2rem); line-height: 1.55; color: var(--tinta-suave); text-wrap: pretty; }
  .acciones { display: flex; flex-wrap: wrap; align-items: center; gap: 14px 24px; margin-top: 30px; }
  .nota { margin: 18px 0 0; font-size: 0.85rem; color: var(--tinta-suave); }

  /* La agenda del día */
  .agenda { margin: 0; height: min(620px, 72svh); display: grid; grid-template-rows: auto 1fr auto; padding: 20px; border-radius: 18px; background: #fff; box-shadow: 0 1px 2px rgb(20 20 22 / 0.06), 0 30px 60px -36px rgb(20 20 22 / 0.35); }
  .cab, .cuerpo { display: grid; grid-template-columns: 48px repeat(4, minmax(0, 1fr)); gap: 8px; }
  .cab p { margin: 0; display: grid; padding-bottom: 10px; border-bottom: 2px solid var(--tono); }
  .cab b { font-weight: 700; }
  .cab small { font-size: 0.8rem; color: var(--tinta-suave); }
  .cuerpo { position: relative; margin-top: 10px; min-height: 0; }
  .horas { position: absolute; inset: 0; margin: 0; padding: 0; list-style: none; }
  .horas li { position: absolute; left: 0; right: 0; top: calc(var(--k) / 3.5 * 100%); padding-top: 3px; border-top: 1px solid rgb(20 20 22 / 0.08); font-size: 0.75rem; font-variant-numeric: tabular-nums; color: var(--tinta-suave); }
  .carril { position: relative; }
  .carril:first-of-type { grid-column: 2; }
  .cita {
    /* cada cita entra cuando el scroll llega a su hora */
    --on: clamp(0, calc((var(--p) * 1.1 + 0.28 - var(--ini) / 4.2) * 4), 1);
    position: absolute; left: 0; right: 0;
    top: calc(var(--ini) / 3.5 * 100%); height: calc(var(--dur) / 3.5 * 100% - 5px);
    display: flex; flex-direction: column; justify-content: center; gap: 1px; padding: 5px 9px; overflow: hidden;
    border-radius: 7px; background: var(--suave); box-shadow: inset 3px 0 0 var(--tono);
    opacity: var(--on); transform: translate3d(0, calc((1 - var(--on)) * 14px), 0);
  }
  .cita b { font-size: 0.85rem; font-weight: 650; line-height: 1.15; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .cita small { font-size: 0.75rem; color: var(--tinta-suave); }
  .cita.corta small { display: none; }
  figcaption { margin-top: 12px; font-size: 0.78rem; color: var(--tinta-suave); }

  :global(.apertura.still) { height: auto; }
  :global(.apertura.still) .cuadro { position: relative; height: auto; min-height: 100svh; }

  @media (max-width: 900px) {
    .apertura { height: auto; }
    .cuadro { position: relative; height: auto; grid-template-columns: 1fr; padding-top: 88px; }
    .cita { --on: 1; }
    .agenda { height: 480px; padding: 14px; }
    .cab, .cuerpo { grid-template-columns: 0 repeat(4, minmax(0, 1fr)); gap: 5px; }
    .horas { display: none; }
    .cab small { display: none; }
    .cita { padding: 4px 6px; }
    .cita small { display: none; }
    .cita b { font-size: 0.72rem; white-space: normal; }
  }
</style>
