<script>
  import Icon from '$lib/Icon.svelte';
  import SectionHead from './SectionHead.svelte';

  const items = [
    ['Cada negocio ve solo lo suyo', 'Tus clientes, ventas y citas están separados de los de cualquier otro negocio.'],
    ['Tú controlas los pagos', 'Los cobros con Mercado Pago caen en tu cuenta. Las transferencias solo las confirma el titular.'],
    ['Acceso por roles', 'Recepción, caja y profesionales ven únicamente lo que les toca, y puedes activar la verificación en dos pasos.'],
    ['Privacidad en regla', 'Aviso de privacidad, consentimientos y solicitudes ARCO (LFPDPPP) resueltos desde el panel.']
  ];
</script>

<section class="section alt">
  <div class="container">
    <SectionHead tag="Confianza" title="Tu negocio y tus clientes," accent="protegidos" />
    <ul class="grid">
      {#each items as [title, text]}
        <li class="card">
          <span class="ico"><Icon name="shield" size={20} /></span>
          <div><h3>{title}</h3><p>{text}</p></div>
        </li>
      {/each}
    </ul>
  </div>
</section>

<style>
  .section { padding-block: 88px; }
  .alt { background: var(--surface-2); }
  .grid { list-style: none; margin: 0; padding: 0; display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(min(380px, 100%), 1fr)); }
  li { display: flex; gap: 16px; padding: 24px; }
  .ico { flex: none; display: grid; place-items: center; width: 42px; height: 42px; border-radius: 12px; color: var(--accent); background: var(--accent-soft); }
  h3 { margin: 0 0 0.35rem; font-size: 1.15rem; }
  p { margin: 0; font-size: 0.94rem; color: var(--muted); }
</style>
