<script>
  // 20:30. Corte de caja: el ticket sale de la impresora con el scroll; abajo, lo que le toca a cada quien.
  import { scene } from './scroll.js';

  // Ejemplo ilustrativo
  const lineas = [['Corte y barba · Marco', '250.00'], ['Pomada mate', '120.00'], ['Anticipo pagado', '−75.00'], ['Propina para Marco', '40.00']];
  const comisiones = [['Marco', 'Barbería', '612'], ['Dani', 'Uñas', '540'], ['Sofi', 'Pestañas', '455'], ['Lu', 'Spa', '690']];
</script>

<section class="corte" use:scene data-hora="20:30" data-momento="Corte de caja" aria-labelledby="t-corte">
  <div class="cuadro">
    <div class="texto">
      <p class="hora">20:30</p>
      <h2 id="t-corte">Cierras con la caja cuadrada.</h2>
      <p>Terminas la cita y pasas directo al cobro: se aplica el anticipo, se suma la propina y se calcula la comisión. Efectivo, tarjeta, transferencia o terminal de Mercado Pago.</p>
      <dl class="comisiones" aria-label="Comisiones del día (ejemplo ilustrativo)">
        {#each comisiones as [quien, oficio, monto], k}
          <div style="--k: {k}"><dt>{quien} <small>{oficio}</small></dt><dd>${monto}</dd></div>
        {/each}
      </dl>
    </div>

    <div class="impresora" aria-hidden="true">
      <span class="ranura"></span>
      <div class="papel">
        <div class="ticket">
          <p class="t-neg">Barbería y Salón Ébano</p>
          <p class="t-meta">Caja 1 · jue 19 nov · 20:31</p>
          <hr />
          {#each lineas as [concepto, monto]}<p class="t-linea"><span>{concepto}</span><span>{monto}</span></p>{/each}
          <hr />
          <p class="t-total"><span>A cobrar</span><span>$335.00</span></p>
          <p class="t-pago">Pagado con terminal · Mercado Pago</p>
          <p class="t-gracias">Gracias, vuelve pronto</p>
        </div>
      </div>
    </div>
    <p class="ejemplo">Ejemplo ilustrativo</p>
  </div>
</section>

<style>
  .corte { height: 230vh; background: #1d1e22; color: var(--ticket); }
  .cuadro { position: sticky; top: 0; height: 100svh; overflow: hidden; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 0.9fr); align-items: center; gap: clamp(24px, 4vw, 64px); padding: 88px clamp(16px, 4vw, 56px) 40px; }
  .hora { margin: 0; font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: clamp(3rem, 7vw, 6rem); line-height: 1; color: #f0b44c; font-variant-numeric: tabular-nums; }
  h2 { margin: 8px 0 18px; font-family: var(--display); font-weight: 600; font-size: clamp(2rem, 3.6vw, 3.3rem); line-height: 1.02; max-width: 14ch; text-wrap: balance; }
  .texto > p:not(.hora) { margin: 0; max-width: 46ch; line-height: 1.55; color: rgb(246 245 241 / 0.78); text-wrap: pretty; }
  .comisiones { margin: 28px 0 0; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 28px; max-width: 460px; }
  .comisiones div { --on: clamp(0, calc((var(--p) - 0.6 - var(--k) * 0.06) * 8), 1); display: flex; justify-content: space-between; align-items: baseline; padding: 12px 0; border-bottom: 1px solid rgb(246 245 241 / 0.16); opacity: calc(0.25 + var(--on) * 0.75); }
  dt { font-weight: 600; }
  dt small { margin-left: 6px; font-weight: 400; opacity: 0.65; }
  dd { margin: 0; font-family: var(--rotulo); font-stretch: 80%; font-weight: 700; font-size: 1.35rem; font-variant-numeric: tabular-nums; }

  /* La impresora térmica de la caja */
  .impresora { position: relative; justify-self: center; width: min(340px, 100%); height: min(560px, 74svh); }
  .ranura { position: absolute; z-index: 2; top: 0; left: -14px; right: -14px; height: 26px; border-radius: 8px; background: #0e0f11; box-shadow: inset 0 -6px 0 #2a2c31; }
  .papel { position: absolute; top: 14px; left: 0; right: 0; bottom: 0; overflow: hidden; }
  .ticket {
    padding: 34px 26px 30px; background: #f7f7f3; color: #1b1b1d; font-family: var(--rotulo); font-stretch: 90%; font-size: 0.95rem;
    transform: translate3d(0, calc((1 - clamp(0, calc(var(--p) * 1.6), 1)) * -100%), 0);
  }
  .ticket::after { content: ''; display: block; height: 10px; margin: 26px -26px -40px; background: radial-gradient(circle at 7px 10px, #1d1e22 6px, transparent 6.5px) 0 0 / 14px 10px repeat-x; }
  .ticket p { margin: 0; }
  .t-neg { font-weight: 800; font-size: 1.1rem; text-align: center; text-transform: uppercase; letter-spacing: 0.03em; }
  .t-meta { margin-top: 4px !important; text-align: center; font-size: 0.8rem; opacity: 0.7; }
  hr { margin: 16px 0; border: 0; border-top: 2px dashed rgb(27 27 29 / 0.35); }
  .t-linea, .t-total { display: flex; justify-content: space-between; gap: 12px; padding: 4px 0; font-variant-numeric: tabular-nums; }
  .t-total { font-weight: 800; font-size: 1.3rem; }
  .t-pago, .t-gracias { margin-top: 10px !important; text-align: center; font-size: 0.82rem; }
  .t-gracias { margin-top: 18px !important; font-weight: 700; text-transform: uppercase; letter-spacing: 0.04em; }
  .ejemplo { position: absolute; right: clamp(16px, 4vw, 56px); bottom: 18px; margin: 0; font-size: 0.75rem; color: rgb(246 245 241 / 0.55); }

  :global(.corte.still) { height: auto; }
  :global(.corte.still) .cuadro { position: relative; height: auto; min-height: 100svh; }

  @media (max-width: 900px) {
    .corte { height: 200vh; }
    .cuadro { grid-template-columns: 1fr; align-content: start; padding-top: 80px; gap: 24px; }
    .comisiones { display: none; }
    .impresora { height: 440px; }
  }
</style>
