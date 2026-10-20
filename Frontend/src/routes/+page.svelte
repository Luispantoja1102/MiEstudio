<script>
  import Header from '$lib/Header.svelte';

  const features = [
    ['Agenda por profesional', 'Horarios, bloqueos y citas sin empalmes. Cada barbero o estilista con su propia agenda.'],
    ['Punto de venta', 'Cobra servicios y productos, maneja la caja del día, propinas y pagos en efectivo, tarjeta o transferencia.'],
    ['Comisiones', 'Calcula automáticamente lo que le toca a cada profesional por servicio.'],
    ['Clientes y seguimiento', 'Ficha de cada cliente con notas: fórmulas, preferencias y cuidados especiales.'],
    ['Reserva en línea', 'Un enlace para que tus clientes agenden y cancelen solos.'],
    ['Inventario', 'Controla tus productos de reventa y su existencia.']
  ];
  const giros = ['Barberías', 'Estéticas', 'Spas', 'Uñas'];
</script>

<Header>
  <a class="btn ghost" href="/login">Iniciar sesión</a>
  <a class="btn" href="/registro">Crear cuenta</a>
</Header>

<main>
  <section class="hero container">
    <h1>Agenda y punto de venta para tu negocio de belleza</h1>
    <p>
      Administra citas, cobros y clientes en un solo lugar. Pensado para barberías, estéticas y spas.
    </p>
    <a class="btn" href="/registro">Empezar ahora</a>
  </section>

  <section class="container giros">
    {#each giros as g}<span>{g}</span>{/each}
  </section>

  <section class="container grid">
    {#each features as [title, text]}
      <article class="card">
        <h3>{title}</h3>
        <p>{text}</p>
      </article>
    {/each}
  </section>

  <section class="container cta card">
    <h2>Lleva tu negocio al siguiente nivel</h2>
    <a class="btn" href="/registro">Crear mi cuenta</a>
  </section>
</main>

<footer class="container">© {new Date().getFullYear()} MiEstudio</footer>

<style>
  .hero { text-align: center; padding-block: 72px 40px; max-width: 760px; }
  .hero h1 { font-size: clamp(2rem, 6vw, 3.2rem); }
  .hero p { color: var(--muted); font-size: 1.1rem; margin-bottom: 1.6rem; }
  .giros { display: flex; flex-wrap: wrap; justify-content: center; gap: 10px; padding-bottom: 40px; }
  .giros span {
    padding: 0.35rem 1rem;
    border: 1px solid var(--border);
    border-radius: 999px;
    color: var(--accent);
    background: var(--accent-soft);
    font-size: 0.9rem;
  }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 16px; padding-bottom: 48px; }
  .grid p { color: var(--muted); margin: 0; }
  .cta { text-align: center; margin-bottom: 48px; }
  footer { text-align: center; color: var(--muted); font-size: 0.85rem; padding-block: 24px; }
</style>
