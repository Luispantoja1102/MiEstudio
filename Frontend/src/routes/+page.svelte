<script>
  import Header from '$lib/Header.svelte';
  import Icon from '$lib/Icon.svelte';

  const parts = [
    {
      title: 'En el mostrador',
      lead: 'Lo que usas cien veces al día, sin pensar.',
      items: [
        'Agenda por profesional, sin citas empalmadas',
        'Cobra en efectivo, tarjeta, transferencia o con tu terminal de Mercado Pago',
        'Caja diaria con apertura, cierre y propinas',
        'Paquetes y comisiones que se calculan solas'
      ]
    },
    {
      title: 'Para tus clientes',
      lead: 'Que reserven, paguen y lleguen, sin que les marques.',
      items: [
        'Una página de reservas con tu nombre, a cualquier hora',
        'Anticipo con Mercado Pago para que no te dejen plantado',
        'Recordatorio por correo y por WhatsApp',
        'Lista de espera: si se libera un lugar, se les avisa'
      ]
    },
    {
      title: 'Para ti, que eres el dueño',
      lead: 'Saber cómo va el día sin preguntarle a nadie.',
      items: [
        'Ventas, comisiones y existencias en un solo lugar',
        'Felicitación de cumpleaños y encuesta con reseña en Google',
        'Aviso de privacidad y solicitudes ARCO (LFPDPPP) ya resueltos',
        'Equipo con roles: recepción, caja y profesionales'
      ]
    }
  ];
  const steps = [
    ['Crea tu cuenta', 'Tu negocio y tu giro: barbería, estética, spa o uñas.'],
    ['Agrega tu equipo y tus servicios', 'Con sus horarios y precios en pesos.'],
    ['Comparte tu enlace', 'Tus clientes reservan y tú cobras desde el primer día.']
  ];
  const giros = ['Barberías', 'Estéticas', 'Spas', 'Uñas'];
  const sample = [
    ['10:00', 'Corte clásico', 'Marco', true],
    ['10:45', 'Corte y barba', 'Ana', false],
    ['11:30', 'Facial hidratante', 'Lalo', false],
    ['12:30', 'Manicure', 'Lucía', false]
  ];
</script>

<Header>
  <a class="btn ghost hide-sm" href="/login">Iniciar sesión</a>
  <a class="btn" href="/registro">Crear cuenta</a>
</Header>

<main>
  <section class="band">
    <div class="container hero">
      <div class="copy">
        <h1>La agenda y la caja de tu negocio de belleza, hechas en México.</h1>
        <p class="lead">
          Tus clientes reservan solos y pagan su anticipo con Mercado Pago. Tú cobras en pesos y ves qué le toca a cada quien.
        </p>
        <div class="actions">
          <a class="btn lg" href="/registro">Probar 14 días gratis <Icon name="arrow" size={18} /></a>
          <a class="btn ghost lg" href="/login">Ya tengo cuenta</a>
        </div>
        <p class="fine">Sin tarjeta para empezar.</p>
        <ul class="giros" aria-label="Giros">
          {#each giros as g}<li>{g}</li>{/each}
        </ul>
      </div>

      <figure class="preview" aria-label="Ejemplo de la agenda del día">
        <div class="agenda">
          <div class="agenda-head">
            <strong>Hoy</strong>
            <span>4 citas</span>
          </div>
          {#each sample as [time, service, who, next]}
            <div class="row" class:next>
              <span class="time">{time}</span>
              <span class="info"><b>{service}</b><small>con {who}</small></span>
              {#if next}<span class="tag">Siguiente</span>{/if}
            </div>
          {/each}
        </div>
        <figcaption>Ejemplo ilustrativo</figcaption>
      </figure>
    </div>
  </section>

  <section class="container section" id="funciones">
    <h2>Todo lo del día, en un solo lugar</h2>
    <div class="parts">
      {#each parts as p}
        <article>
          <h3>{p.title}</h3>
          <p class="plead">{p.lead}</p>
          <ul>
            {#each p.items as i}<li>{i}</li>{/each}
          </ul>
        </article>
      {/each}
    </div>
  </section>

  <section class="container section">
    <h2>Listo en tres pasos</h2>
    <ol class="steps">
      {#each steps as [title, text], i}
        <li>
          <span class="num">{i + 1}</span>
          <h3>{title}</h3>
          <p>{text}</p>
        </li>
      {/each}
    </ol>
  </section>

  <section class="container section">
    <div class="cta">
      <h2>Empieza hoy, sin tarjeta</h2>
      <p>14 días para probar todo con tus propios clientes.</p>
      <a class="btn lg invert" href="/registro">Crear mi cuenta</a>
    </div>
  </section>
</main>

<footer class="container foot">
  <span>© {new Date().getFullYear()} Barbella</span>
  <span><a href="/privacidad">Aviso de privacidad</a> · <a href="/terminos">Términos</a></span>
</footer>

<style>
  /* la franja principal lleva el color de la barra lateral: berenjena en claro, carbón en oscuro */
  .band {
    --text: var(--side-text);
    --muted: var(--side-muted);
    --accent: var(--side-mark);
    --accent-hover: var(--side-mark);
    --accent-contrast: var(--side-mark-ink);
    background: var(--side);
    color: var(--side-text);
  }
  .band :global(.btn.ghost) { color: var(--side-text); border-color: var(--side-line); }
  .band :global(.btn.ghost:hover) { background: var(--side-hover); border-color: var(--side-muted); }
  .band :global(.btn:not(.ghost):hover) { filter: brightness(1.06); }
  .hero { display: grid; gap: 48px; padding-block: 64px 80px; align-items: center; }
  .copy { animation: rise 0.6s var(--ease) both; }
  h1 { font-size: clamp(2.2rem, 5.6vw, 3.5rem); max-width: 15em; }
  .lead { font-size: 1.15rem; color: var(--side-muted); max-width: 32em; margin-bottom: 1.75rem; }
  .actions { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 0.75rem; }
  .fine { font-size: 0.88rem; color: var(--side-muted); margin-bottom: 1.75rem; }
  .giros { display: flex; flex-wrap: wrap; gap: 8px; list-style: none; padding: 0; margin: 0; }
  .giros li { padding: 0.3rem 0.9rem; border: 1px solid var(--side-line); border-radius: 999px; color: var(--side-muted); font-size: 0.88rem; }

  .preview { margin: 0; animation: rise 0.6s 0.12s var(--ease) both; }
  .agenda { padding: 1.25rem; max-width: 440px; margin-inline: auto; background: #fffdfa; color: #241a24; border-radius: var(--radius); box-shadow: 0 24px 48px -24px rgba(0, 0, 0, 0.55); }
  .agenda-head { display: flex; justify-content: space-between; align-items: baseline; padding: 0 0.25rem 0.9rem; font-family: var(--font-heading); font-size: 1.25rem; }
  .agenda-head span { font-family: var(--font-body); font-size: 0.88rem; color: #5f5463; }
  .row { display: flex; align-items: center; gap: 14px; padding: 0.8rem 0.9rem; margin-top: 8px; border-radius: var(--radius-sm); background: #eee5d8; }
  .row.next { background: #fbedcf; }
  .time { font-family: var(--font-heading); font-variant-numeric: tabular-nums; font-weight: 600; font-size: 1.1rem; min-width: 3rem; }
  .info { display: grid; line-height: 1.3; flex: 1; }
  .info small { color: #5f5463; font-size: 0.85rem; }
  .tag { font-size: 0.75rem; font-weight: 600; padding: 2px 10px; border-radius: 999px; background: #fffdfa; color: #7a4f00; }
  figcaption { margin-top: 10px; text-align: center; font-size: 0.8rem; color: var(--side-muted); }

  .section { padding-block: 64px 8px; }
  .section h2 { font-size: clamp(1.7rem, 4vw, 2.3rem); max-width: 20em; margin-bottom: 1.75rem; }

  .parts { display: grid; gap: 40px 56px; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); }
  .parts h3 { font-size: 1.35rem; margin-bottom: 0.25rem; }
  .plead { color: var(--muted); margin-bottom: 1rem; }
  .parts ul { list-style: none; margin: 0; padding: 0; }
  .parts li { padding: 0.7rem 0; border-top: 1px solid var(--border); font-size: 0.98rem; }

  .steps { display: grid; gap: 16px 40px; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); list-style: none; padding: 0; margin: 0; }
  .steps li { padding-top: 1.25rem; border-top: 2px solid var(--accent); }
  .num { font-family: var(--font-heading); font-size: 2rem; color: var(--accent); line-height: 1; }
  .steps h3 { font-size: 1.15rem; margin-top: 0.5rem; }
  .steps p { color: var(--muted); margin: 0; }

  .cta { text-align: center; padding: clamp(2.5rem, 7vw, 4.5rem) 1.5rem; border-radius: calc(var(--radius) * 1.5); background: var(--side); color: var(--side-text); }
  .cta h2 { font-size: clamp(1.7rem, 4vw, 2.4rem); margin-inline: auto; }
  .cta p { color: var(--side-muted); margin-bottom: 1.5rem; }
  .cta .btn { background: var(--side-mark); border-color: var(--side-mark); color: var(--side-mark-ink); }
  .cta .btn:hover { filter: brightness(1.06); }
  .section:last-of-type { padding-bottom: 64px; }

  .foot { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 8px; padding-block: 32px 40px; color: var(--muted); font-size: 0.88rem; border-top: 1px solid var(--border); }

  @keyframes rise { from { opacity: 0; transform: translateY(12px); } to { opacity: 1; transform: none; } }
  @media (min-width: 900px) { .hero { grid-template-columns: 1.1fr 0.9fr; } }
</style>
