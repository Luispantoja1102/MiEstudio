<script>
  import Icon from '$lib/Icon.svelte';
  import SectionHead from './SectionHead.svelte';

  // calendario de ejemplo (noviembre): días con horarios libres
  const free = new Set([2, 3, 4, 5, 6, 9, 10, 11, 12, 13, 16, 17, 18, 19, 20]);
  const times = [['09:00', 0], ['09:15', 1], ['09:30', 1], ['10:00', 0], ['10:30', 0], ['11:00', 1], ['11:30', 0], ['12:00', 0]];
</script>

<section class="section">
  <div class="container">
    <SectionHead tag="Cómo se ve en el día a día" title="Tres cosas que" accent="te ahorran horas" />

    <div class="block">
      <div class="copy">
        <h3>Tus clientes reservan solos, y solo ven lo libre</h3>
        <p>Una página con tu nombre y un calendario mensual. Los días sin lugar se apagan y los horarios ocupados aparecen como «Ocupado», sin decir por quién ni para qué.</p>
        <ul>
          <li><Icon name="check" size={16} />Elige servicio, profesional y hora en cuatro pasos</li>
          <li><Icon name="check" size={16} />El horario se aparta mientras llenan sus datos</li>
          <li><Icon name="check" size={16} />Recordatorio por correo y WhatsApp</li>
        </ul>
      </div>
      <div class="art" role="img" aria-label="Ejemplo del calendario de reservas con horarios libres y ocupados">
        <div class="phone">
          <b class="mon">Noviembre de 2026</b>
          <div class="days">
            {#each ['L', 'M', 'M', 'J', 'V', 'S', 'D'] as d}<i class="dw">{d}</i>{/each}
            {#each Array.from({ length: 30 }, (_, i) => i + 1) as d}
              <span class="d" class:free={free.has(d)} class:sel={d === 10}>{d}</span>
            {/each}
          </div>
          <div class="times">
            {#each times as [t, busy]}<span class="t" class:busy>{t}{#if busy}<small>Ocupado</small>{/if}</span>{/each}
          </div>
        </div>
      </div>
    </div>

    <div class="block flip">
      <div class="copy">
        <h3>El anticipo por transferencia, sin perseguir a nadie</h3>
        <p>El cliente transfiere y sube su comprobante desde su cita, o te lo manda por WhatsApp y tú lo adjuntas desde la agenda. Llega un aviso a tu campana y confirmas la cita al verlo.</p>
        <ul>
          <li><Icon name="check" size={16} />Solo el titular confirma el pago</li>
          <li><Icon name="check" size={16} />Si no llega el comprobante, el horario se libera solo</li>
          <li><Icon name="check" size={16} />También con Mercado Pago, que se confirma automáticamente</li>
        </ul>
      </div>
      <div class="art" role="img" aria-label="Ejemplo de una cita con comprobante por revisar">
        <div class="panel">
          <p class="who"><b>Rosa Medina</b><span>Corte clásico · jue 22, 12:00</span></p>
          <dl>
            <div><dt>Banco</dt><dd>BBVA</dd></div>
            <div><dt>Concepto</dt><dd>CITA-95</dd></div>
            <div><dt>Anticipo</dt><dd>$75.00</dd></div>
          </dl>
          <p class="note"><Icon name="check" size={15} />Comprobante recibido hoy, 12:27</p>
          <div class="btns"><span class="b main">Confirmar pago y cita</span><span class="b">Rechazar</span></div>
        </div>
      </div>
    </div>

    <div class="block">
      <div class="copy">
        <h3>Cierra el día con la caja cuadrada</h3>
        <p>Completa la cita y pasa directo al cobro: se abre la caja si estaba cerrada, se aplica el anticipo y se calcula la comisión de cada persona.</p>
        <ul>
          <li><Icon name="check" size={16} />Efectivo, tarjeta, transferencia o terminal</li>
          <li><Icon name="check" size={16} />Propinas por persona y paquetes canjeables</li>
          <li><Icon name="check" size={16} />Reportes de ventas y de comisiones</li>
        </ul>
      </div>
      <div class="art" role="img" aria-label="Ejemplo de un ticket de cobro">
        <div class="ticket">
          <p class="who"><b>Ticket</b><span>Caja abierta · Marco</span></p>
          <div class="ln"><span>Corte clásico</span><b>$250.00</b></div>
          <div class="ln"><span>Pomada</span><b>$120.00</b></div>
          <div class="ln sub"><span>Anticipo pagado</span><b>−$75.00</b></div>
          <div class="ln"><span>Propina</span><b>$40.00</b></div>
          <div class="total"><span>A cobrar</span><b>$335.00</b></div>
        </div>
      </div>
    </div>
  </div>
</section>

<style>
  .section { padding-block: 88px; }
  .block { display: grid; gap: 36px; align-items: center; margin-top: 3.5rem; }
  h3 { font-size: clamp(1.6rem, 3vw, 2.1rem); line-height: 1.1; margin: 0 0 1rem; }
  .copy p { margin: 0 0 1.2rem; color: var(--muted); font-size: 1.02rem; }
  ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 10px; }
  li { display: flex; align-items: flex-start; gap: 10px; font-size: 0.96rem; }
  li :global(svg) { flex: none; margin-top: 3px; color: var(--accent); }
  .art { display: grid; place-items: center; padding: 28px; border-radius: 26px; background: var(--surface-2); border: 1px solid var(--border); font-variant-numeric: tabular-nums lining-nums; font-family: var(--font-body, inherit); }
  .phone, .panel, .ticket { width: 100%; max-width: 340px; padding: 18px; color: #1f1e1c; background: #fffdf9; border-radius: 20px; box-shadow: 0 18px 44px rgba(10, 20, 45, 0.16); }
  .mon { display: block; margin-bottom: 10px; text-align: center; font-size: 0.95rem; }
  .days { display: grid; grid-template-columns: repeat(7, 1fr); gap: 4px; }
  .dw { text-align: center; font-size: 0.68rem; font-style: normal; font-weight: 600; color: #6b675f; padding-bottom: 2px; }
  .d { aspect-ratio: 1; display: grid; place-items: center; font-size: 0.8rem; color: #b5b0a5; border-radius: 9px; }
  .d.free { color: #1f1e1c; font-weight: 600; background: #fff; border: 1px solid #e3ddd0; }
  .d.sel { color: #fff; background: #1c2f5c; border-color: #1c2f5c; }
  .times { display: grid; grid-template-columns: repeat(4, 1fr); gap: 6px; margin-top: 12px; }
  .t { display: grid; place-items: center; min-height: 38px; font-size: 0.78rem; font-weight: 600; background: #fff; border: 1px solid #e3ddd0; border-radius: 9px; }
  .t small { font-size: 0.56rem; font-weight: 500; color: #6b675f; }
  .t.busy { color: #6b675f; background: #efeae1; border-color: transparent; }
  .who { display: grid; margin: 0 0 12px; font-size: 0.92rem; }
  .who span { font-size: 0.78rem; color: #6b675f; }
  dl { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin: 0 0 12px; padding: 10px; background: #efeae1; border-radius: 12px; }
  dt { font-size: 0.6rem; font-weight: 600; letter-spacing: 0.1em; text-transform: uppercase; color: #6b675f; }
  dd { margin: 2px 0 0; font-size: 0.82rem; font-weight: 600; }
  .note { display: flex; align-items: center; gap: 6px; margin: 0 0 12px; font-size: 0.78rem; }
  .note :global(svg) { color: #1c2f5c; }
  .btns { display: flex; gap: 8px; }
  .b { padding: 8px 12px; font-size: 0.74rem; font-weight: 700; border: 1px solid #d9d2c3; border-radius: 999px; }
  .b.main { color: #fff; background: #1c2f5c; border-color: #1c2f5c; }
  .ln { display: flex; justify-content: space-between; padding: 8px 0; font-size: 0.84rem; border-bottom: 1px solid #ece6da; }
  .ln.sub { color: #6b675f; }
  .total { display: flex; justify-content: space-between; align-items: baseline; margin-top: 12px; padding: 12px 14px; background: #e4e9f5; border-radius: 12px; }
  .total b { font-size: 1.5rem; color: #1c2f5c; }
  @media (min-width: 900px) {
    .block { grid-template-columns: 1fr 1fr; gap: 64px; }
    .block.flip .copy { order: 2; }
  }
</style>
