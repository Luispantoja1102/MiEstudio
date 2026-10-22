<script>
  import Icon from '$lib/Icon.svelte';
  import SectionHead from './SectionHead.svelte';

  const giros = [
    ['scissors', 'Barberías', 'Cortes, barba y afeitado. Citas rápidas por barbero, fila del día y comisión por servicio.'],
    ['user', 'Estéticas y salones', 'Cabello, color y maquillaje. Servicios largos, varias personas y paquetes de tratamientos.'],
    ['shield', 'Spas', 'Masajes y faciales con anticipo para apartar la cabina y membresías de sesiones.'],
    ['card', 'Uñas', 'Manicure, pedicure y diseño. Recordatorios para que no se pierdan las citas y reactivación de clientas.']
  ];
</script>

<section class="section">
  <div class="container">
    <SectionHead tag="Para tu giro" title="Hecho para" accent="barberías y salones" sub="Un solo sistema que se adapta a cómo trabaja cada negocio de belleza." />
    <div class="grid">
      {#each giros as [icon, title, text]}
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
  .section { padding-block: 88px; }
  .grid { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); }
  article { padding: 26px; }
  .ico { display: grid; place-items: center; width: 46px; height: 46px; margin-bottom: 1.1rem; border-radius: 50%; color: var(--accent); background: var(--accent-soft); }
  h3 { margin: 0 0 0.5rem; font-size: 1.3rem; }
  p { margin: 0; font-size: 0.95rem; color: var(--muted); }
</style>
