<script>
  import Header from '$lib/Header.svelte';
  import Icon from '$lib/Icon.svelte';

  const features = [
    ['calendar', 'Agenda por profesional', 'Horarios, bloqueos y citas sin empalmes. Cada barbero o estilista con su propia agenda.'],
    ['card', 'Punto de venta', 'Cobra servicios y productos, con caja diaria, propinas y pagos en efectivo, tarjeta o transferencia.'],
    ['percent', 'Comisiones claras', 'Calcula automáticamente lo que le toca a cada profesional por servicio.'],
    ['notes', 'Clientes y seguimiento', 'Una ficha por cliente con notas: fórmulas, preferencias y cuidados especiales.'],
    ['globe', 'Reserva en línea', 'Un enlace para que tus clientes agenden y cancelen solos, a cualquier hora.'],
    ['package', 'Inventario', 'Controla tus productos de reventa y su existencia.']
  ];
  const steps = [
    ['Crea tu cuenta', 'Registra tu negocio y elige tu giro.'],
    ['Configura tu equipo', 'Agrega servicios, profesionales y horarios.'],
    ['Empieza a agendar y cobrar', 'Recibe citas y lleva tu caja sin papel.']
  ];
  const giros = ['Barberías', 'Estéticas', 'Spas', 'Uñas'];
  const sample = [
    ['10:00', 'Corte clásico', 'Marco', 'accent'],
    ['10:45', 'Barba y toalla caliente', 'Marco', 'accent'],
    ['11:00', 'Facial hidratante', 'Ana', 'soft'],
    ['12:30', 'Manicure', 'Lucía', 'soft']
  ];
</script>

<Header>
  <a class="btn ghost hide-sm" href="/login">Iniciar sesión</a>
  <a class="btn" href="/registro">Crear cuenta</a>
</Header>

<main>
  <section class="container hero">
    <div class="copy">
      <span class="eyebrow">Agenda y punto de venta</span>
      <h1>Gestiona tu negocio de belleza con elegancia.</h1>
      <p class="lead">
        Citas, cobros y clientes en un solo lugar. Pensado para barberías, estéticas y spas que quieren verse y operar como los mejores.
      </p>
      <div class="actions">
        <a class="btn lg" href="/registro">Empezar ahora <Icon name="arrow" size={18} /></a>
        <a class="btn ghost lg" href="/login">Ya tengo cuenta</a>
      </div>
      <ul class="giros" aria-label="Giros">
        {#each giros as g}<li>{g}</li>{/each}
      </ul>
    </div>

    <div class="preview" aria-hidden="true">
      <div class="card agenda">
        <div class="agenda-head">
          <strong>Hoy</strong>
          <span>4 citas</span>
        </div>
        {#each sample as [time, service, who, tone]}
          <div class="row" class:soft={tone === 'soft'}>
            <span class="time">{time}</span>
            <span class="info"><b>{service}</b><small>con {who}</small></span>
          </div>
        {/each}
      </div>
    </div>
  </section>

  <section class="container section" id="funciones">
    <div class="head">
      <span class="eyebrow">Todo en uno</span>
      <h2>Lo que necesitas para operar cada día</h2>
    </div>
    <div class="grid">
      {#each features as [icon, title, text]}
        <article class="card feature">
          <span class="ico"><Icon name={icon} size={22} /></span>
          <h3>{title}</h3>
          <p>{text}</p>
        </article>
      {/each}
    </div>
  </section>

  <section class="container section">
    <div class="head">
      <span class="eyebrow">Cómo funciona</span>
      <h2>Listo en tres pasos</h2>
    </div>
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
      <h2>Lleva tu negocio al siguiente nivel</h2>
      <p>Crea tu cuenta y empieza a organizar tu agenda hoy.</p>
      <a class="btn lg invert" href="/registro">Crear mi cuenta</a>
    </div>
  </section>
</main>

<footer class="container foot">
  <span>© {new Date().getFullYear()} MiEstudio</span>
  <span><a href="/privacidad">Aviso de privacidad</a> · <a href="/terminos">Términos</a></span>
</footer>

<style>
  .hero { display: grid; gap: 48px; padding-block: 56px 72px; align-items: center; }
  .copy { animation: rise 0.6s var(--ease) both; }
  h1 { font-size: clamp(2.3rem, 6vw, 3.6rem); }
  .lead { font-size: 1.15rem; color: var(--muted); max-width: 34em; margin-bottom: 1.75rem; }
  .actions { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 2rem; }
  .giros { display: flex; flex-wrap: wrap; gap: 8px; list-style: none; padding: 0; margin: 0; }
  .giros li {
    padding: 0.3rem 0.9rem;
    border: 1px solid var(--border);
    border-radius: 999px;
    color: var(--muted);
    font-size: 0.85rem;
    background: var(--surface);
  }

  .preview { animation: rise 0.6s 0.12s var(--ease) both; }
  .agenda { padding: 1.25rem; box-shadow: var(--shadow); max-width: 440px; margin-inline: auto; }
  .agenda-head { display: flex; justify-content: space-between; align-items: baseline; padding: 0 0.25rem 0.9rem; }
  .agenda-head span { font-size: 0.85rem; color: var(--muted); }
  .row {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 0.8rem 0.9rem;
    margin-top: 8px;
    border-radius: var(--radius-sm);
    background: var(--accent-soft);
  }
  .row.soft { background: var(--surface-2); }
  .time { font-variant-numeric: tabular-nums; font-weight: 600; font-size: 0.9rem; min-width: 2.8rem; }
  .info { display: grid; line-height: 1.3; }
  .info small { color: var(--muted); }

  .section { padding-block: 56px; }
  .head { max-width: 640px; margin-bottom: 2rem; }
  .head h2 { font-size: clamp(1.7rem, 4vw, 2.3rem); margin: 0; }

  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px; }
  .feature { transition: transform 0.25s var(--ease), box-shadow 0.25s var(--ease), border-color 0.25s var(--ease); }
  .feature:hover { transform: translateY(-3px); box-shadow: var(--shadow); border-color: var(--accent); }
  .feature h3 { font-size: 1.2rem; }
  .feature p { color: var(--muted); margin: 0; font-size: 0.95rem; }
  .ico {
    display: inline-grid;
    place-items: center;
    width: 44px;
    height: 44px;
    margin-bottom: 1rem;
    border-radius: 10px;
    background: var(--accent-soft);
    color: var(--accent);
  }

  .steps { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); list-style: none; padding: 0; margin: 0; }
  .steps li { padding-top: 1.25rem; border-top: 1px solid var(--border); }
  .num { font-family: var(--font-heading); font-size: 2rem; color: var(--accent); line-height: 1; }
  .steps h3 { font-size: 1.15rem; margin-top: 0.5rem; }
  .steps p { color: var(--muted); margin: 0; }

  .cta {
    text-align: center;
    padding: clamp(2.5rem, 7vw, 4.5rem) 1.5rem;
    border-radius: calc(var(--radius) * 1.5);
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .cta h2 { font-size: clamp(1.7rem, 4vw, 2.4rem); }
  .cta p { opacity: 0.9; margin-bottom: 1.5rem; }
  .btn.invert { background: var(--accent-contrast); color: var(--accent); border-color: var(--accent-contrast); }
  .btn.invert:hover { background: var(--accent-contrast); filter: brightness(0.94); }

  .foot { padding-block: 32px 40px; color: var(--muted); font-size: 0.85rem; border-top: 1px solid var(--border); }

  @keyframes rise { from { opacity: 0; transform: translateY(12px); } to { opacity: 1; transform: none; } }

  @media (min-width: 900px) {
    .hero { grid-template-columns: 1.15fr 0.85fr; }
  }
</style>
