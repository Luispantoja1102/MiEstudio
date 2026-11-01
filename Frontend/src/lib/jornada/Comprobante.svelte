<script>
  // 11:20. El anticipo: dos estados concretos de la misma cita. Primero llega el comprobante; después el titular la confirma.
  // Al llegar a la pantalla, el segundo paso toma el relevo. Con «reducir movimiento» se ven los dos estados completos.
  import AppointmentSummary from '$lib/ui/AppointmentSummary.svelte';
  import Icon from '$lib/Icon.svelte';
  import { vista } from './vista.js';
</script>

<section class="comprobante" aria-labelledby="t-anticipo">
  <div class="envoltura">
    <div class="intro">
      <div>
        <p class="kicker"><time>11:20</time>Llega un anticipo</p>
        <h2 id="t-anticipo" class="titulo">Andrea apartó su jueves con $150.</h2>
      </div>
      <div class="lado">
        <p>Pide un anticipo para que el horario no se pierda.</p>
        <ul class="puntos">
          <li>Solo el titular confirma los pagos.</li>
          <li>Si el comprobante no llega, el horario se libera solo.</li>
          <li>El dinero cae directo en tu cuenta.</li>
        </ul>
      </div>
    </div>

    <div class="estados" use:vista>
      <!-- Estado 1 -->
      <article class="estado uno" aria-labelledby="e1">
        <h3 id="e1"><span class="n">1</span>Llega el comprobante</h3>
        <AppointmentSummary timeLabel="10:00" client="Andrea Solís" service="Acrílico almendra · jue 19" staff="Dani" statusLabel="Esperando confirmación" tone="warning" />
        <dl>
          <div><dt>Anticipo</dt><dd>$150.00</dd></div>
          <div><dt>Transferencia SPEI</dt><dd>Concepto CITA-214</dd></div>
          <div><dt>Clave de rastreo</dt><dd>2026111911204417</dd></div>
        </dl>
        <p class="aviso"><Icon name="bell" size={18} /> Aviso al titular: comprobante recibido</p>
        <span class="boton">Confirmar pago y cita</span>
      </article>

      <span class="paso" aria-hidden="true"></span>

      <!-- Estado 2 -->
      <article class="estado dos" aria-labelledby="e2">
        <h3 id="e2"><span class="n">2</span>El titular confirma</h3>
        <AppointmentSummary timeLabel="10:00" client="Andrea Solís" service="Acrílico almendra · jue 19" staff="Dani" statusLabel="Confirmada" tone="success" />
        <dl>
          <div><dt>Anticipo</dt><dd>$150.00 · pagado</dd></div>
          <div><dt>Confirmó</dt><dd>Titular · 11:24</dd></div>
          <div><dt>Horario</dt><dd>Apartado para Andrea</dd></div>
        </dl>
      </article>
    </div>
    <p class="ejemplo">Ejemplo ilustrativo</p>

    <!-- Los dos caminos -->
    <dl class="caminos">
      <div>
        <dt>Mercado Pago</dt>
        <dd>Con Mercado Pago se confirma solo.</dd>
      </div>
      <div>
        <dt>Transferencia</dt>
        <dd>Por transferencia, el cliente sube su comprobante desde su cita, o tú lo adjuntas desde la agenda, y te llega un aviso para confirmarla.</dd>
      </div>
    </dl>
  </div>
</section>

<style>
  .comprobante { padding: clamp(72px, 9vw, 112px) 0; background: var(--ticket); color: var(--tinta); }
  .intro { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 0.8fr); align-items: end; gap: 24px clamp(32px, 6vw, 96px); margin-bottom: clamp(32px, 4vw, 56px); }
  .titulo { max-width: 15ch; }
  .lado p { margin: 0; line-height: 1.55; color: var(--tinta-suave); }
  .lado .puntos { margin-top: 14px; }

  .estados { display: grid; grid-template-columns: minmax(0, 1fr) 56px minmax(0, 1fr); align-items: stretch; }
  .estado { display: flex; flex-direction: column; padding: 22px 24px 24px; border: 1px solid var(--border); border-radius: 8px; background: var(--carta); }
  h3 { display: flex; align-items: center; gap: 10px; margin: 0 0 6px; font-size: 0.82rem; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--tinta-suave); }
  .n { display: inline-grid; place-items: center; width: 22px; height: 22px; border-radius: 50%; border: 1.5px solid var(--tinta); color: var(--tinta); font-size: 0.78rem; letter-spacing: 0; }
  dl { margin: 0; }
  .estado dl { margin-top: 6px; }
  .estado dl > div { display: flex; justify-content: space-between; gap: 16px; padding: 9px 0; border-top: 1px solid var(--border); font-size: 0.92rem; }
  dt { color: var(--tinta-suave); }
  dd { margin: 0; text-align: right; font-weight: 600; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
  .aviso { display: flex; align-items: center; gap: 8px; margin: 12px 0 18px; font-size: 0.9rem; font-weight: 600; color: var(--bermellon); }
  .boton { margin-top: auto; display: inline-flex; align-items: center; justify-content: center; min-height: 44px; padding: 0 18px; border-radius: 8px; background: var(--bermellon); color: #fff; font-weight: 700; }

  /* La flecha entre un estado y otro */
  .paso { position: relative; align-self: center; height: 2px; background: var(--tinta); margin: 0 8px; }
  .paso::after { content: ''; position: absolute; right: -1px; top: -5px; width: 10px; height: 10px; border-top: 2px solid var(--tinta); border-right: 2px solid var(--tinta); transform: rotate(45deg); transform-origin: 70% 30%; }
  .ejemplo { margin-top: 14px; }

  .caminos { display: grid; grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.2fr); margin-top: clamp(32px, 4vw, 56px); border-top: 2px solid var(--tinta); }
  .caminos > div { padding: 18px 0 0; }
  .caminos > div + div { padding-left: clamp(20px, 3vw, 40px); border-left: 1px solid var(--border); }
  .caminos dt { margin-bottom: 4px; font-weight: 700; color: var(--tinta); }
  .caminos dd { text-align: left; font-weight: 400; line-height: 1.5; color: var(--tinta-suave); max-width: 52ch; }

  /* Movimiento que explica: el estado 2 toma el relevo del 1 */
  @media (prefers-reduced-motion: no-preference) {
    .estados:not(:global(.vista)) .dos { opacity: 0.35; }
    .estados:not(:global(.vista)) .paso { transform: scaleX(0); }
    .dos { transition: opacity 600ms var(--ease) 900ms; }
    .paso { transform-origin: left; transition: transform 500ms var(--ease) 400ms; }
  }

  @media (max-width: 860px) {
    .intro { grid-template-columns: 1fr; }
    .estados { grid-template-columns: 1fr; }
    .paso { width: 2px; height: 32px; margin: 0 0 0 28px; }
    .paso::after { top: auto; bottom: -1px; right: -5px; transform: rotate(135deg); transform-origin: 70% 30%; }
    .caminos { grid-template-columns: 1fr; }
    .caminos > div + div { padding-left: 0; border-left: 0; border-top: 1px solid var(--border); margin-top: 18px; }
  }
</style>
