<script>
  import SectionHead from './SectionHead.svelte';

  const steps = [
    ['Crea tu cuenta', 'Pones el nombre de tu negocio y tu giro. Toma menos de un minuto y no pide tarjeta.'],
    ['Configúralo con el asistente', 'Tu horario, tu equipo y tus servicios con sus precios. Si lo omites, lo completas después en Ajustes.'],
    ['Comparte tu enlace', 'Lo pones en tu Instagram o en WhatsApp. Tus clientes reservan y tú cobras desde el primer día.']
  ];
</script>

<section class="section alt" id="como-funciona">
  <div class="container">
    <SectionHead tag="Cómo funciona" title="Listo en" accent="tres pasos" sub="Sin instalar nada: funciona desde el navegador de tu computadora o de tu celular." />
    <ol class="steps">
      {#each steps as [title, text], i}
        <li class="card">
          <span class="num">{i + 1}</span>
          <h3>{title}</h3>
          <p>{text}</p>
        </li>
      {/each}
    </ol>
  </div>
</section>

<style>
  .section { padding-block: 88px; scroll-margin-top: 68px; }
  .alt { background: var(--surface-2); }
  .steps { list-style: none; margin: 0; padding: 0; display: grid; gap: 16px; counter-reset: s; }
  li { padding: 28px; }
  .num { display: grid; place-items: center; width: 42px; height: 42px; margin-bottom: 1rem; font-family: var(--font-heading); font-size: 1.3rem; font-weight: 700; color: var(--accent-contrast); background: var(--accent); border-radius: 50%; }
  h3 { margin: 0 0 0.5rem; font-size: 1.35rem; }
  p { margin: 0; color: var(--muted); font-size: 0.97rem; }
  @media (min-width: 820px) { .steps { grid-template-columns: repeat(3, 1fr); } }
</style>
