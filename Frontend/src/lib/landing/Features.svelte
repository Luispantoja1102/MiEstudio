<script>
  import Icon from '$lib/Icon.svelte';
  import SectionHead from './SectionHead.svelte';

  const items = [
    ['calendar', 'Agenda por persona', 'Día, semana, mes y lista. Filtra por profesional y cambia de vista cuando quieras; no deja empalmar citas.'],
    ['globe', 'Reserva en línea', 'Una página con tu nombre y un calendario mensual. Tus clientes ven los horarios libres y reservan a cualquier hora.'],
    ['wallet', 'Anticipos que sí llegan', 'Pide un adelanto con Mercado Pago o por transferencia. Subes o recibes el comprobante y tú confirmas la cita.'],
    ['receipt', 'Caja y ventas', 'Cobra en efectivo, tarjeta, transferencia o terminal. Caja con apertura y cierre, propinas y cancelaciones.'],
    ['percent', 'Comisiones automáticas', 'Define el porcentaje por persona o por servicio y consulta cuánto le toca a cada quien.'],
    ['package', 'Paquetes y membresías', 'Vende sesiones por adelantado y canjéalas en caja. El saldo de cada cliente siempre a la vista.'],
    ['bell', 'Recordatorios y avisos', 'Correo y WhatsApp antes de la cita, y una campana en tu panel con reservas, comprobantes y cancelaciones.'],
    ['users', 'Clientes que regresan', 'Lista de espera cuando se libera un lugar, felicitación de cumpleaños, reactivación y encuesta con reseña.']
  ];
</script>

<section class="section alt" id="funciones">
  <div class="container">
    <SectionHead tag="Funciones" title="Todo lo del día," accent="en un solo lugar" sub="Pensado para el mostrador de una barbería o un salón, no para una oficina." />
    <div class="grid">
      {#each items as [icon, title, text]}
        <article class="card">
          <span class="ico"><Icon name={icon} size={22} /></span>
          <h3>{title}</h3>
          <p>{text}</p>
        </article>
      {/each}
    </div>
  </div>
</section>

<style>
  .section { padding-block: 88px; scroll-margin-top: 68px; }
  .alt { background: var(--surface-2); }
  .grid { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); }
  article { padding: 26px; transition: transform 0.2s var(--ease), box-shadow 0.2s var(--ease); }
  article:hover { transform: translateY(-3px); }
  .ico { display: grid; place-items: center; width: 46px; height: 46px; margin-bottom: 1.1rem; border-radius: 14px; background: var(--accent-soft); color: var(--accent); }
  h3 { margin: 0 0 0.5rem; font-size: 1.3rem; }
  p { margin: 0; font-size: 0.95rem; color: var(--muted); }
  @media (prefers-reduced-motion: reduce) { article:hover { transform: none; } }
</style>
