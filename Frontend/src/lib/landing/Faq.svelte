<script>
  import SectionHead from './SectionHead.svelte';

  const faqs = [
    ['¿Necesito tarjeta para probarlo?', 'No. Tienes 14 días para usar todo con tus propios clientes y decides después si contratas un plan.'],
    ['¿Mis clientes tienen que instalar algo?', 'No. Reservan desde un enlace, en el navegador de su celular. No necesitan crear cuenta.'],
    ['¿Cómo cobro el anticipo?', 'Con Mercado Pago, que se confirma solo, o por transferencia: el cliente sube su comprobante (o lo mandas tú desde la agenda) y confirmas la cita al verlo. El dinero llega directo a tu cuenta.'],
    ['¿Y si el cliente no llega?', 'El anticipo sirve para apartar el horario. Si cancela a tiempo se le devuelve; si no llega, tú decides conservarlo. Además hay recordatorios por correo y WhatsApp.'],
    ['¿Puedo usarlo en varias sucursales?', 'Sí, con el plan Pro: hasta 10 sucursales con su propia agenda y caja. Los planes Básico y Crecimiento incluyen una.'],
    ['¿Qué incluye el plan Básico?', 'La agenda, las citas y la reserva en línea, sin caja ni ventas. Si luego necesitas cobrar en el sistema, pasas a Crecimiento sin perder tus datos.'],
    ['¿Puedo cambiar de plan o cancelar?', 'Sí. Cambias de plan cuando quieras y tus datos siempre están a salvo, aunque tu suscripción venza.'],
    ['¿Qué pasa con los datos de mis clientes?', 'Son tuyos. Cada negocio ve solo lo suyo, y el sistema incluye aviso de privacidad y solicitudes ARCO para cumplir con la ley mexicana.']
  ];
</script>

<section class="section" id="preguntas">
  <div class="container narrow">
    <SectionHead tag="Preguntas frecuentes" title="Lo que casi" accent="todos preguntan" />
    <div class="list">
      {#each faqs as [q, a]}
        <details>
          <summary>{q}</summary>
          <p>{a}</p>
        </details>
      {/each}
    </div>
  </div>
</section>

<style>
  .section { padding-block: 88px; scroll-margin-top: 68px; }
  .narrow { max-width: 820px; }
  .list { display: grid; gap: 10px; }
  details { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); transition: border-color 0.18s var(--ease); }
  details[open] { border-color: var(--accent); }
  summary { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 18px 22px; min-height: 56px; font-weight: 600; cursor: pointer; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  summary::after { content: '+'; flex: none; display: grid; place-items: center; width: 28px; height: 28px; font-size: 1.2rem; font-weight: 500; color: var(--accent); background: var(--accent-soft); border-radius: 50%; transition: transform 0.2s var(--ease); }
  details[open] summary::after { content: '–'; }
  summary:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; border-radius: var(--radius-sm); }
  p { margin: 0; padding: 0 22px 20px; color: var(--muted); font-size: 0.96rem; }
</style>
