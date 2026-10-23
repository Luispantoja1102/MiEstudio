<script>
  // 11:20. El anticipo por transferencia: el comprobante entra a la cita, suena la campana y el titular la confirma.
  import { scene } from './scroll.js';
</script>

<section class="comprobante" use:scene data-hora="11:20" data-momento="Llega un anticipo" aria-labelledby="t-anticipo">
  <div class="cuadro">
    <div class="texto">
      <p class="hora">11:20</p>
      <h2 id="t-anticipo">Andrea apartó su jueves con $150.</h2>
      <p>Pide un anticipo para que el horario no se pierda. Con Mercado Pago se confirma solo; por transferencia, el cliente sube su comprobante desde su cita, o tú lo adjuntas desde la agenda, y te llega un aviso para confirmarla.</p>
      <ul>
        <li>Solo el titular confirma los pagos.</li>
        <li>Si el comprobante no llega, el horario se libera solo.</li>
        <li>El dinero cae directo en tu cuenta.</li>
      </ul>
    </div>

    <div class="mesa" aria-hidden="true">
      <!-- La cita en la agenda -->
      <div class="cita">
        <p class="quien"><b>Andrea Solís</b><span>Acrílico almendra · jue 19, 10:00</span></p>
        <dl>
          <div><dt>Con</dt><dd>Dani</dd></div>
          <div><dt>Anticipo</dt><dd>$150.00</dd></div>
          <div><dt>Estado</dt><dd class="estado"><span class="antes">Esperando comprobante</span><span class="despues">Confirmada</span></dd></div>
        </dl>
        <span class="boton">Confirmar pago y cita</span>
        <span class="sello">Confirmada</span>
        <span class="campana"><svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M6 8a6 6 0 1 1 12 0c0 7 3 9 3 9H3s3-2 3-9" /><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" /></svg><i>1</i></span>
      </div>

      <!-- El comprobante del banco que llega -->
      <div class="recibo">
        <p class="banco">Transferencia SPEI</p>
        <p class="monto">$150.00</p>
        <p class="concepto">Concepto <b>CITA-214</b></p>
        <p class="rastreo">Clave de rastreo 2026111911204417</p>
      </div>

    </div>
    <p class="ejemplo">Ejemplo ilustrativo</p>
  </div>
</section>

<style>
  .comprobante { height: 240vh; background: var(--ticket); color: var(--tinta); }
  .cuadro { position: sticky; top: 0; height: 100svh; overflow: hidden; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.05fr); align-items: center; gap: clamp(24px, 4vw, 64px); padding: 88px clamp(16px, 4vw, 56px) 40px; }
  .hora { margin: 0; font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: clamp(3rem, 7vw, 6rem); line-height: 1; color: var(--bermellon); font-variant-numeric: tabular-nums; }
  h2 { margin: 8px 0 18px; font-family: var(--display); font-weight: 600; font-size: clamp(2rem, 3.6vw, 3.3rem); line-height: 1.02; max-width: 15ch; text-wrap: balance; }
  .texto > p:not(.hora) { margin: 0; max-width: 46ch; line-height: 1.55; color: var(--tinta-suave); text-wrap: pretty; }
  ul { margin: 22px 0 0; padding: 0; list-style: none; display: grid; gap: 10px; }
  ul li { padding-left: 18px; position: relative; line-height: 1.4; }
  ul li::before { content: ''; position: absolute; left: 0; top: 0.6em; width: 8px; height: 2px; background: var(--bermellon); }

  .mesa { position: relative; height: min(520px, 70svh); }

  .cita {
    --ok: clamp(0, calc((var(--p) - 0.62) * 6), 1);
    position: absolute; z-index: 2; left: 0; bottom: 6%; width: min(420px, 92%); padding: 24px; border-radius: 16px; background: #fff;
    box-shadow: 0 1px 2px rgb(20 20 22 / 0.08), 0 24px 48px -24px rgb(20 20 22 / 0.35);
  }
  .quien { display: grid; gap: 4px; margin: 0 0 16px; }
  .quien b { font-family: var(--display); font-size: 1.5rem; font-weight: 600; }
  .quien span { color: var(--tinta-suave); font-size: 0.95rem; }
  dl { margin: 0; display: grid; gap: 8px; }
  dl div { display: flex; justify-content: space-between; gap: 12px; padding-bottom: 8px; border-bottom: 1px dashed rgb(20 20 22 / 0.18); }
  dt { color: var(--tinta-suave); }
  dd { margin: 0; font-weight: 600; font-variant-numeric: tabular-nums; }
  .estado { display: grid; text-align: right; }
  .estado span { grid-area: 1 / 1; }
  .antes { opacity: calc(1 - var(--ok)); color: var(--miel); }
  .despues { opacity: var(--ok); color: var(--eucalipto); }
  .boton { display: block; margin-top: 18px; padding: 12px; border-radius: 10px; text-align: center; font-weight: 600; background: var(--tinta); color: var(--ticket); opacity: calc(1 - var(--ok) * 0.6); }
  .sello {
    position: absolute; left: 50%; bottom: 20px; padding: 6px 14px; background: #fff; border: 3px solid var(--eucalipto); border-radius: 6px;
    font-family: var(--rotulo); font-stretch: 75%; font-weight: 800; font-size: 1.4rem; text-transform: uppercase; color: var(--eucalipto);
    transform: translateX(-50%) rotate(-6deg) scale(calc(1.6 - var(--ok) * 0.6)); opacity: var(--ok);
  }

  /* el comprobante viaja de la esquina hasta la cita */
  .recibo {
    --mv: clamp(0, calc((var(--p) - 0.08) * 2.2), 1);
    --tx: -78%;
    position: absolute; right: 0; top: 0; width: min(260px, 70%); padding: 18px 20px 22px; background: #fbfbfa; border-radius: 4px;
    box-shadow: 0 1px 2px rgb(20 20 22 / 0.1), 0 18px 32px -18px rgb(20 20 22 / 0.4);
    transform: translate3d(calc(var(--mv) * var(--tx)), calc(var(--mv) * 46%), 0) rotate(calc(5deg - var(--mv) * 8deg));
  }
  .recibo::after { content: ''; position: absolute; left: 0; right: 0; bottom: -8px; height: 8px; background: radial-gradient(circle at 6px 0, transparent 5px, #fbfbfa 5.5px) 0 0 / 12px 8px repeat-x; }
  .banco { margin: 0; font-size: 0.8rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--tinta-suave); }
  .monto { margin: 6px 0 10px; font-family: var(--rotulo); font-stretch: 80%; font-weight: 700; font-size: 2.3rem; font-variant-numeric: tabular-nums; }
  .concepto, .rastreo { margin: 0; font-size: 0.85rem; color: var(--tinta-suave); }
  .rastreo { margin-top: 4px; font-size: 0.75rem; font-variant-numeric: tabular-nums; }

  .campana {
    --on: clamp(0, calc((var(--p) - 0.45) * 8), 1);
    position: absolute; right: -18px; top: -18px; display: grid; place-items: center; width: 46px; height: 46px; border-radius: 50%;
    background: var(--tinta); color: var(--ticket); opacity: var(--on); transform: scale(calc(0.6 + var(--on) * 0.4));
  }
  .campana i { position: absolute; top: -2px; right: -2px; display: grid; place-items: center; width: 18px; height: 18px; border-radius: 50%; background: var(--bermellon); font-style: normal; font-size: 0.7rem; font-weight: 700; }
  .ejemplo { position: absolute; right: clamp(16px, 4vw, 56px); bottom: 18px; margin: 0; font-size: 0.75rem; color: var(--tinta-suave); }

  :global(.comprobante.still) { height: auto; }
  :global(.comprobante.still) .cuadro { position: relative; height: auto; min-height: 100svh; }

  @media (max-width: 900px) {
    .comprobante { height: 200vh; }
    .cuadro { grid-template-columns: 1fr; align-content: start; gap: 28px; padding-top: 80px; }
    .texto ul { display: none; }
    .mesa { height: 380px; }
    .recibo { --tx: -12%; }
  }
</style>
