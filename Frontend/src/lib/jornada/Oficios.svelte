<script>
  // 12:40 → 18:00. Cuatro oficios, cuatro franjas de proporciones distintas. Cada una trae su propia pieza de producto
  // (agenda por barbero, servicio largo con anticipo, mensajes, saldo de paquete) con datos de ejemplo.
  import AppointmentSummary from '$lib/ui/AppointmentSummary.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import PackageBalance from '$lib/ui/PackageBalance.svelte';

  // Ejemplo ilustrativo
  const barberia = [
    ['12:40', 'Toño', 'Corte', 'Terminada', 'success', '$60'],
    ['13:20', 'Iker', 'Fade', 'En la silla', 'accent', '$60'],
    ['14:00', 'Beto', 'Barba', 'Agendada', 'neutral', '$40'],
    ['14:40', 'Memo', 'Corte', 'Agendada', 'neutral', '$60']
  ];
  const mensajes = [
    ['Reactivación', 'Para quien dejó de venir', 'Hola Sara, hace rato que no te vemos. ¿Agendamos tu relleno?'],
    ['Lista de espera', 'Se liberó un lugar', 'Se liberó el jueves a las 12:30. ¿La quieres, Paola?'],
    ['Cumpleaños', 'Con el nombre de tu negocio', 'Feliz cumpleaños, Ivonne. Te saludamos todos en Barbería y Salón Ébano.']
  ];
  const tramo = ['14:00', '14:30', '15:00', '15:30', '16:00', '16:30', '17:00'];
</script>

<section id="oficios" class="oficios" aria-labelledby="t-oficios">
  <div class="envoltura">
    <h2 id="t-oficios" class="cabeza">Para cada oficio</h2>

    <!-- Barbería -->
    <article class="franja barberia">
      <div class="texto">
        <p class="kicker"><time>12:40</time><span class="of">Barbería</span></p>
        <h3>Citas de 40 minutos, una tras otra.</h3>
        <p>Agenda por barbero que no deja empalmar, la fila del día a la vista y la comisión de cada servicio calculada sola.</p>
      </div>
      <figure class="pieza" aria-label="Agenda de Marco, ejemplo ilustrativo">
        <p class="rotulo"><b>Marco</b> · jueves 19</p>
        {#each barberia as [hora, cliente, servicio, estado, tono, comision]}
          <AppointmentSummary timeLabel={hora} client={cliente} service={servicio} statusLabel={estado} tone={tono}>
            {#snippet actions()}<span class="com">Comisión {comision}</span>{/snippet}
          </AppointmentSummary>
        {/each}
        <figcaption class="ejemplo">Ejemplo ilustrativo</figcaption>
      </figure>
    </article>

    <!-- Uñas -->
    <article class="franja unas">
      <figure class="pieza" aria-label="Un servicio largo con anticipo, ejemplo ilustrativo">
        <p class="rotulo"><b>Acrílico almendra</b> · Dani · 3 horas</p>
        <div class="regla" aria-hidden="true">
          {#each tramo as t}<span>{t}</span>{/each}
        </div>
        <div class="bloque"><b>Andrea Solís</b><small>14:00 a 17:00</small></div>
        <dl>
          <div><dt>Anticipo</dt><dd>$150.00 <StatusBadge label="Pagado" tone="success" /></dd></div>
          <div><dt>Recordatorio</dt><dd>WhatsApp, antes de la cita</dd></div>
        </dl>
        <figcaption class="ejemplo">Ejemplo ilustrativo</figcaption>
      </figure>
      <div class="texto">
        <p class="kicker"><time>14:00</time><span class="of">Uñas</span></p>
        <h3>Tres horas de acrílico, apartadas con anticipo.</h3>
        <p>Servicios largos con su duración real, anticipo para que nadie te deje plantada y recordatorio por WhatsApp antes de la cita.</p>
      </div>
    </article>

    <!-- Pestañas -->
    <article class="franja pestanas">
      <div class="texto">
        <p class="kicker"><time>16:30</time><span class="of">Pestañas</span></p>
        <h3>Tus clientas vuelven a su relleno.</h3>
        <p>Reactivación para quien dejó de venir, lista de espera cuando se libera un lugar y felicitación de cumpleaños con tu nombre.</p>
      </div>
      <figure class="pieza" aria-label="Tres mensajes automáticos, ejemplo ilustrativo">
        <ul class="mensajes">
          {#each mensajes as [tipo, cuando, texto]}
            <li><p class="tipo"><b>{tipo}</b><small>{cuando}</small></p><p class="msg">{texto}</p></li>
          {/each}
        </ul>
        <figcaption class="ejemplo">Ejemplo ilustrativo</figcaption>
      </figure>
    </article>

    <!-- Spa -->
    <article class="franja spa">
      <figure class="pieza" aria-label="Saldo de un paquete de masajes, ejemplo ilustrativo">
        <p class="rotulo"><b>Fer Ruiz</b> · se canjea en caja</p>
        <PackageBalance name="Paquete 10 masajes" remaining={6} total={10} statusLabel="Vigente" />
        <figcaption class="ejemplo">Ejemplo ilustrativo</figcaption>
      </figure>
      <div class="texto">
        <p class="kicker"><time>18:00</time><span class="of">Spa</span></p>
        <h3>Diez masajes pagados, canjeados uno por uno.</h3>
        <p>Paquetes y membresías que vendes por adelantado y se canjean en caja, con el saldo de cada cliente siempre a la vista.</p>
      </div>
    </article>
  </div>
</section>

<style>
  .oficios { padding: clamp(72px, 9vw, 112px) 0 clamp(40px, 6vw, 72px); background: var(--papel-2); color: var(--tinta); }
  .cabeza { margin: 0 0 clamp(24px, 3vw, 40px); padding-bottom: 14px; border-bottom: 2px solid var(--tinta); font-size: 0.82rem; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--tinta-suave); }

  .franja { display: grid; align-items: center; gap: clamp(28px, 5vw, 80px); padding: clamp(36px, 5vw, 64px) 0; border-bottom: 1px solid var(--border); }
  .franja:last-child { border-bottom: 0; }
  /* proporciones y lado distintos en cada una */
  .barberia { grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); }
  .unas { grid-template-columns: minmax(0, 7fr) minmax(0, 5fr); }
  .pestanas { grid-template-columns: minmax(0, 4fr) minmax(0, 8fr); }
  .spa { grid-template-columns: minmax(0, 5fr) minmax(0, 6fr); }
  .spa .pieza { max-width: 460px; }
  .pestanas .pieza { max-width: 640px; justify-self: end; width: 100%; }
  .unas .pieza { max-width: 600px; }

  .barberia { --tono: var(--cuero); }
  .unas { --tono: var(--esmalte); }
  .pestanas { --tono: var(--tinta); }
  .spa { --tono: var(--eucalipto); }
  .of { color: var(--tinta); }
  .of::before { content: ''; display: inline-block; width: 9px; height: 9px; margin-right: 8px; border-radius: 2px; background: var(--tono); }

  h3 { margin: 0 0 16px; font-family: var(--display); font-stretch: var(--ancho); font-weight: 700; font-size: clamp(1.7rem, 2.9vw, 2.6rem); line-height: 1.04; letter-spacing: -0.02em; max-width: 16ch; text-wrap: balance; }
  .texto > p:last-child { margin: 0; max-width: 42ch; line-height: 1.55; color: var(--tinta-suave); text-wrap: pretty; }

  .pieza { margin: 0; min-width: 0; padding: 20px 24px 18px; border: 1px solid var(--border); border-radius: 8px; background: var(--carta); }
  .rotulo { margin: 0 0 6px; font-size: 0.9rem; color: var(--tinta-suave); }
  .rotulo b { color: var(--tinta); }
  .pieza .ejemplo { margin-top: 12px; }
  .com { font-size: 0.85rem; font-weight: 600; font-variant-numeric: tabular-nums; color: var(--tinta-suave); }

  /* Servicio largo: la duración se ve como un bloque, no como un número */
  .regla { display: grid; grid-template-columns: repeat(7, 1fr); margin-top: 14px; font-size: 0.72rem; color: var(--tinta-suave); font-variant-numeric: tabular-nums; }
  .regla span:last-child { text-align: right; }
  .regla span { padding-bottom: 4px; border-bottom: 1px solid var(--tinta); }
  .bloque { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; margin: 6px 0 16px; padding: 18px 16px; border-radius: 4px; background: var(--s-unas); box-shadow: inset 4px 0 0 var(--esmalte); }
  .bloque small { color: var(--tinta-suave); font-variant-numeric: tabular-nums; }
  dl { margin: 0; }
  dl > div { display: flex; justify-content: space-between; align-items: center; gap: 16px; padding: 10px 0; border-top: 1px solid var(--border); font-size: 0.92rem; }
  dt { color: var(--tinta-suave); }
  dd { margin: 0; display: flex; align-items: center; gap: 10px; font-weight: 600; text-align: right; font-variant-numeric: tabular-nums; }

  /* Mensajes */
  .mensajes { margin: 0; padding: 0; list-style: none; }
  .mensajes li { padding: 16px 0; border-bottom: 1px solid var(--border); }
  .mensajes li:first-child { padding-top: 4px; }
  .mensajes li:last-child { border-bottom: 0; }
  .tipo { margin: 0 0 6px; display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 12px; }
  .tipo small { color: var(--tinta-suave); }
  .msg { margin: 0; padding-left: 14px; border-left: 3px solid var(--tinta); line-height: 1.45; }

  @media (max-width: 860px) {
    .franja { grid-template-columns: 1fr; gap: 24px; }
    /* en una columna el texto va primero en todas */
    .unas .texto, .spa .texto { order: -1; }
    .pestanas .pieza { justify-self: stretch; }
  }
</style>
