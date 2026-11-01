<script>
  // Apertura: el día de un local con cuatro oficios en una sola agenda, vista por profesional.
  // Titular a la izquierda; a la derecha, la agenda de ejemplo, legible completa. Los oficios del titular son la leyenda de sus colores.
  const INICIO = 9 * 60;   // la agenda arranca a las 9:00
  const PX_HORA = 64;      // alto de una hora
  const HORAS = 7;         // 9:00 a 16:00
  const AHORA = 11 * 60 + 20;

  const oficios = {
    barberia: { nombre: 'barbería', tono: 'var(--cuero)', suave: 'var(--s-barberia)' },
    unas: { nombre: 'uñas', tono: 'var(--esmalte)', suave: 'var(--s-unas)' },
    pestanas: { nombre: 'pestañas', tono: 'var(--tinta)', suave: 'var(--s-pestanas)' },
    spa: { nombre: 'spa', tono: 'var(--eucalipto)', suave: 'var(--s-spa)' }
  };

  // Ejemplo ilustrativo: [hora, minutos, servicio, cliente]; servicio vacío = bloqueo
  const equipo = [
    { quien: 'Marco', oficio: 'barberia', citas: [['9:00', 40, 'Corte', 'Toño'], ['9:40', 40, 'Fade', 'Iker'], ['10:20', 60, 'Corte y barba', 'Beto'], ['11:20', 40, 'Corte', 'Memo'], ['12:00', 60, '', 'Comida'], ['13:00', 40, 'Fade', 'Óscar'], ['13:40', 40, 'Barba', 'Rubén'], ['14:20', 40, 'Corte', 'Chuy']] },
    { quien: 'Dani', oficio: 'unas', citas: [['9:30', 150, 'Acrílico', 'Andrea'], ['12:00', 60, 'Gelish', 'Paola'], ['13:00', 60, 'Retoque', 'Karla'], ['14:00', 120, 'Pedicure', 'Dulce']] },
    { quien: 'Sofi', oficio: 'pestanas', citas: [['10:00', 150, 'Extensiones', 'Mariela'], ['12:30', 90, 'Relleno', 'Ivonne'], ['14:00', 90, 'Lifting', 'Sara']] },
    { quien: 'Lu', oficio: 'spa', citas: [['9:00', 60, 'Masaje', 'Fer'], ['10:30', 90, 'Facial', 'Nayeli'], ['12:00', 60, 'Masaje', 'Lupita'], ['13:30', 90, 'Piedras calientes', 'Luis']] }
  ];
  const min = (h) => { const [a, b] = h.split(':').map(Number); return a * 60 + b; };
  const horas = Array.from({ length: HORAS }, (_, i) => `${9 + i}:00`);
</script>

<section class="apertura" aria-labelledby="titulo">
  <div class="envoltura rejilla">
    <div class="texto">
      <p class="kicker"><time>09:00</time>Se abre el día</p>
      <h2 id="titulo" class="titulo">
        Tu agenda de <span class="of" style="--tono: {oficios.barberia.tono}">barbería</span>, <span class="of" style="--tono: {oficios.unas.tono}">uñas</span>, <span class="of" style="--tono: {oficios.pestanas.tono}">pestañas</span> o <span class="of" style="--tono: {oficios.spa.tono}">spa</span> se llena sola.
      </h2>
      <p class="lead">Tus clientes reservan a cualquier hora, dejan su anticipo y tú cobras en pesos. Citas, caja y recordatorios en un solo lugar.</p>
      <div class="acciones">
        <a class="btn" href="/registro">Probar 14 días gratis</a>
        <a class="link" href="#madrugada">Así es un día</a>
      </div>
    </div>

    <figure class="agenda" aria-label="Agenda de un día con cuatro profesionales (ejemplo ilustrativo)">
      <div class="cabeza">
        <span></span>
        {#each equipo as e, i}
          <p class="persona" class:extra={i === 3}><b>{e.quien}</b><small style="--tono: {oficios[e.oficio].tono}">{oficios[e.oficio].nombre}</small></p>
        {/each}
      </div>
      <div class="cuerpo" style="--alto: {HORAS * PX_HORA}px">
        <ol class="horas" aria-hidden="true">{#each horas as h, i}<li style="top: {i * PX_HORA}px">{h}</li>{/each}</ol>
        {#each equipo as e, i}
          <div class="col" class:extra={i === 3}>
            {#each e.citas as [h, dur, servicio, cliente]}
              <div class="cita" class:bloqueo={!servicio} class:corta={dur < 40} style="top: {((min(h) - INICIO) / 60) * PX_HORA}px; height: {(dur / 60) * PX_HORA - 2}px; --tono: {oficios[e.oficio].tono}; --suave: {oficios[e.oficio].suave}">
                <b>{servicio || 'Bloqueado'}</b>
                <small>{h} · {cliente}</small>
              </div>
            {/each}
          </div>
        {/each}
        <p class="ahora" style="top: {((AHORA - INICIO) / 60) * PX_HORA}px" aria-hidden="true"><span>11:20</span></p>
      </div>
      <figcaption class="ejemplo">Ejemplo ilustrativo</figcaption>
    </figure>
  </div>
</section>

<style>
  .apertura { padding: clamp(72px, 9vw, 120px) 0 clamp(72px, 9vw, 112px); background: var(--ticket); color: var(--tinta); }
  .rejilla { display: grid; grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); align-items: start; gap: clamp(32px, 5vw, 80px); }
  .texto { padding-top: clamp(0px, 3vw, 40px); }
  .titulo { font-size: clamp(2.2rem, 4vw, 3.8rem); line-height: 1; }
  /* cada oficio del titular lleva el color con que se ve en la agenda */
  .of { text-decoration: underline; text-decoration-color: var(--tono); text-decoration-thickness: 0.09em; text-underline-offset: 0.12em; }
  .lead { margin: 22px 0 0; max-width: 44ch; font-size: clamp(1rem, 1.25vw, 1.15rem); line-height: 1.55; color: var(--tinta-suave); text-wrap: pretty; }
  .acciones { display: flex; flex-wrap: wrap; align-items: center; gap: 14px 24px; margin-top: 28px; }

  /* La agenda: horas a la izquierda, una columna por persona */
  .agenda { margin: 0; min-width: 0; }
  .cabeza, .cuerpo { display: grid; grid-template-columns: 44px repeat(4, minmax(0, 1fr)); gap: 0 8px; }
  .cabeza { padding-bottom: 10px; border-bottom: 2px solid var(--tinta); }
  .persona { margin: 0; display: grid; }
  .persona b { font-size: 0.95rem; }
  .persona small { font-size: 0.78rem; font-weight: 600; color: var(--tono); }
  .persona small { color: var(--tinta-suave); }
  .persona small::before { content: ''; display: inline-block; width: 8px; height: 8px; margin-right: 6px; border-radius: 2px; background: var(--tono); }
  .cuerpo { position: relative; height: var(--alto); }
  .horas { position: absolute; inset: 0; margin: 0; padding: 0; list-style: none; pointer-events: none; }
  .horas li { position: absolute; left: 0; right: 0; padding-top: 3px; border-top: 1px solid rgb(var(--ink-rgb) / 0.12); font-size: 0.72rem; color: var(--tinta-suave); font-variant-numeric: tabular-nums; }
  .col { position: relative; grid-row: 1; }
  .col:nth-of-type(1) { grid-column: 2; }
  .col:nth-of-type(2) { grid-column: 3; }
  .col:nth-of-type(3) { grid-column: 4; }
  .col:nth-of-type(4) { grid-column: 5; }
  .cita { position: absolute; left: 0; right: 0; z-index: 1; overflow: hidden; display: flex; flex-direction: column; padding: 4px 7px; border-radius: 4px; background: var(--suave); box-shadow: inset 3px 0 0 var(--tono); }
  .cita b { font-size: 0.8rem; font-weight: 650; line-height: 1.2; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .cita small { font-size: 0.7rem; color: var(--tinta-suave); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-variant-numeric: tabular-nums; }
  .cita.bloqueo { background: transparent; box-shadow: inset 0 0 0 1px rgb(var(--ink-rgb) / 0.22); color: var(--tinta-suave); }
  .ahora { position: absolute; z-index: 2; left: 44px; right: 0; margin: 0; height: 0; border-top: 2px solid var(--bermellon); pointer-events: none; }
  .ahora span { position: absolute; right: 0; top: -1px; padding: 1px 6px; border-radius: 0 0 4px 4px; background: var(--bermellon); color: #fff; font-size: 0.7rem; font-weight: 700; font-variant-numeric: tabular-nums; }
  figcaption { margin-top: 14px; }

  @media (max-width: 900px) {
    .rejilla { grid-template-columns: 1fr; }
    .texto { padding-top: 0; }
  }
  /* Celular: tres columnas para que cada cita se lea */
  @media (max-width: 560px) {
    .cabeza, .cuerpo { grid-template-columns: 34px repeat(3, minmax(0, 1fr)); gap: 0 5px; }
    .extra { display: none; }
    .ahora { left: 34px; }
    .cita { padding: 3px 5px; }
    .cita.corta small { display: none; }
  }
</style>
