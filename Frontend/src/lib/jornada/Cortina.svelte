<script>
  // 08:58 → 09:00. La cortina metálica del local, con el rótulo pintado, sube con el scroll y deja ver la agenda del día.
  import { scene } from './scroll.js';

  // Ejemplo ilustrativo: [columna, hora de inicio desde las 9:00 (h), duración (h), servicio, clienta/cliente]
  const sillas = [
    { quien: 'Marco', oficio: 'Barbería', tono: 'var(--cuero)' },
    { quien: 'Dani', oficio: 'Uñas', tono: 'var(--esmalte)' },
    { quien: 'Sofi', oficio: 'Pestañas', tono: 'var(--tinta)' },
    { quien: 'Lu', oficio: 'Spa', tono: 'var(--eucalipto)' }
  ];
  const citas = [
    [0, 0, 0.67, 'Corte y barba', 'Luis R.'], [0, 0.75, 0.67, 'Fade', 'Toño'], [0, 1.5, 0.67, 'Corte clásico', 'Iker'], [0, 2.5, 0.67, 'Barba', 'Rubén'],
    [1, 0, 2, 'Acrílico almendra', 'Andrea'], [1, 2.25, 1, 'Gelish', 'Paola'],
    [2, 0.5, 2, 'Extensiones clásicas', 'Mariela'], [2, 2.75, 1, 'Relleno', 'Karla'],
    [3, 1, 1, 'Masaje relajante', 'Fer'], [3, 2.25, 1.25, 'Facial hidratante', 'Ivonne']
  ];
</script>

<section class="cortina" use:scene style="--p: 0" data-hora="09:00" data-momento="Se abre el local" aria-labelledby="titulo">
  <div class="cuadro">
    <!-- Lo que hay detrás de la cortina: la agenda de hoy -->
    <div class="local" aria-hidden="true">
      <div class="agenda">
        <ol class="horas">{#each ['9:00', '10:00', '11:00', '12:00'] as h, k}<li style="--k: {k}">{h}</li>{/each}</ol>
        {#each sillas as s, i}
          <div class="col" style="--tono: {s.tono}">
            <p class="silla"><b>{s.quien}</b><span>{s.oficio}</span></p>
            <div class="carril">
              {#each citas.filter((c) => c[0] === i) as [, ini, dur, servicio, cliente]}
                <span class="cita" class:corta={dur < 1} style="--ini: {ini}; --dur: {dur}"><b>{servicio}</b><small>{cliente}</small></span>
              {/each}
            </div>
          </div>
        {/each}
      </div>
      <p class="pie-local">Así amanece tu agenda: cuatro sillas, cuatro oficios, un solo sistema. <small>Ejemplo ilustrativo</small></p>
    </div>

    <!-- La cortina, con el rótulo pintado -->
    <div class="lamina">
      <div class="rotulo">
        <p class="oficios">Barbería · Uñas · Pestañas · Spa</p>
        <h1 id="titulo">Sube la cortina con la agenda llena.</h1>
        <p class="bajada">Citas en línea, anticipos y caja para tu barbería o salón. Tus clientes reservan solos, tú confirmas el anticipo y cobras en pesos.</p>
        <div class="acciones">
          <a class="btn" href="/registro">Probar 14 días gratis</a>
          <a class="link" href="#madrugada">Ver la jornada completa</a>
        </div>
        <p class="nota">Sin tarjeta para empezar · Hecho en México</p>
      </div>
      <span class="jaladera" aria-hidden="true"></span>
    </div>
  </div>
</section>

<style>
  .cortina { height: 230vh; }
  .cuadro { position: sticky; top: 0; height: 100svh; overflow: hidden; background: var(--tinta); }

  /* Detrás de la cortina */
  .local { position: absolute; inset: 64px 0 0; display: grid; grid-template-rows: 1fr auto; padding: 32px clamp(16px, 4vw, 56px) 28px; color: var(--ticket); }
  .agenda { position: relative; display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; min-height: 0; padding-left: 56px; }
  /* riel de horas: cada línea marca una hora de la agenda */
  .horas { position: absolute; inset: 52px 0 0; margin: 0; padding: 0; list-style: none; pointer-events: none; }
  .horas li { position: absolute; left: 0; right: 0; top: calc(var(--k) / 3.5 * 100%); border-top: 1px solid rgb(246 245 241 / 0.1); font-size: 0.78rem; font-variant-numeric: tabular-nums; color: rgb(246 245 241 / 0.5); padding-top: 4px; }
  .col { display: grid; grid-template-rows: auto 1fr; gap: 12px; min-width: 0; }
  .silla { display: flex; align-items: baseline; gap: 8px; margin: 0; padding-bottom: 10px; border-bottom: 2px solid var(--tono); }
  .silla b { font-family: var(--display); font-size: clamp(1.2rem, 2vw, 1.6rem); font-weight: 600; }
  .silla span { font-size: 0.85rem; opacity: 0.7; }
  .carril { position: relative; }
  .cita {
    position: absolute; left: 0; right: 0;
    top: calc(var(--ini) / 3.5 * 100%); height: calc(var(--dur) / 3.5 * 100% - 6px);
    display: flex; flex-direction: column; justify-content: center; gap: 2px; padding: 6px 10px; overflow: hidden;
    background: color-mix(in oklab, var(--tono) 88%, white 6%); border-radius: 6px;
    box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.12);
    /* cada cita se enciende cuando la cortina ya pasó por encima de ella */
    opacity: clamp(0.12, calc((var(--p) * 1.55 - 0.15 - var(--ini) / 5) * 3), 1);
  }
  .cita b { font-size: 0.9rem; font-weight: 600; line-height: 1.15; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .cita small { font-size: 0.78rem; opacity: 0.75; }
  .pie-local { margin: 20px 0 0; font-family: var(--display); font-size: clamp(1.1rem, 2.2vw, 1.6rem); max-width: 34ch; }
  .pie-local small { display: block; margin-top: 6px; font-family: var(--texto); font-size: 0.78rem; opacity: 0.6; }

  /* La cortina metálica: láminas onduladas de acero galvanizado */
  .lamina {
    position: absolute; inset: 0;
    transform: translate3d(0, calc(clamp(0, var(--p) * 1.55, 1) * -100%), 0);
    background:
      repeating-linear-gradient(180deg, #b9bec2 0 3px, #9fa5aa 3px 16px, #868c92 16px 19px, #c6cacd 19px 21px, #a7adb2 21px 26px);
    display: grid; place-items: center; padding: 96px clamp(16px, 5vw, 64px) 72px;
    box-shadow: 0 30px 40px -10px rgb(0 0 0 / 0.55);
    will-change: transform;
  }
  .lamina::after { content: ''; position: absolute; left: 0; right: 0; bottom: 0; height: 22px; background: #5d6268; box-shadow: inset 0 3px 0 #7b8187; }
  .jaladera { position: absolute; bottom: 28px; left: 50%; width: 120px; height: 14px; margin-left: -60px; border-radius: 3px; background: #4b5056; box-shadow: inset 0 -3px 0 #383c41; }

  /* Rótulo pintado a mano sobre la lámina */
  .rotulo { position: relative; max-width: 980px; padding: clamp(28px, 4vw, 52px) clamp(22px, 4vw, 56px); background: var(--ticket); color: var(--tinta); border-radius: 4px; box-shadow: 0 1px 0 rgb(0 0 0 / 0.25), 0 18px 30px -18px rgb(0 0 0 / 0.6); text-align: center; }
  .oficios { margin: 0 0 18px; font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: 1rem; text-transform: uppercase; letter-spacing: 0.06em; color: var(--bermellon); }
  h1 { margin: 0 auto; max-width: 13ch; font-family: var(--display); font-weight: 600; font-size: clamp(2.6rem, 7.4vw, 6.4rem); line-height: 0.98; letter-spacing: -0.015em; text-wrap: balance; }
  .bajada { margin: 22px auto 0; max-width: 46ch; font-size: clamp(1.02rem, 1.5vw, 1.2rem); line-height: 1.5; color: var(--tinta-suave); text-wrap: pretty; }
  .acciones { display: flex; flex-wrap: wrap; justify-content: center; align-items: center; gap: 14px 24px; margin-top: 30px; }
  .nota { margin: 18px 0 0; font-size: 0.85rem; color: var(--tinta-suave); }

  /* Con «reducir movimiento»: la cortina queda cerrada y la agenda aparece debajo, quieta */
  :global(.cortina.still) { height: auto; }
  :global(.cortina.still) .cuadro { position: relative; height: auto; display: grid; }
  :global(.cortina.still) .lamina { position: relative; transform: none; min-height: 100svh; }
  :global(.cortina.still) .local { position: relative; inset: auto; min-height: 80svh; }
  :global(.cortina.still) .cita { opacity: 1; }

  @media (max-width: 760px) {
    .agenda { grid-template-columns: repeat(2, minmax(0, 1fr)); grid-auto-rows: minmax(0, 1fr); }
    .agenda { padding-left: 0; }
    .horas { display: none; }
    .cita { padding: 4px 8px; }
    .cita.corta small { display: none; }
    .lamina { padding-top: 84px; }
  }
</style>
