<script>
  import Icon from '$lib/Icon.svelte';
  import SectionHead from './SectionHead.svelte';

  const before = ['Citas en WhatsApp y en una libreta, con empalmes', 'Clientes que reservan y no llegan', 'Comprobantes de transferencia perdidos en el chat', 'La caja y las comisiones en una hoja de cálculo', 'No sabes cuánto vendió cada quien'];
  const after = ['Una agenda por persona que no permite empalmes', 'Anticipo para apartar el horario, por Mercado Pago o transferencia', 'Comprobante en la cita y aviso en la campana para que lo confirmes', 'Caja diaria, propinas y comisiones calculadas solas', 'Ventas y comisiones por persona, en un reporte'];
</script>

<section class="section">
  <div class="container">
    <SectionHead tag="Del caos al orden" title="Deja de perseguir" accent="citas y pagos" sub="Barbella reúne lo que hoy tienes repartido entre el chat, la libreta y el banco." />
    <div class="cmp">
      <article class="card before">
        <h3>Hoy</h3>
        <ul>{#each before as t}<li><span class="mark x"><Icon name="check" size={13} /></span>{t}</li>{/each}</ul>
      </article>
      <article class="card after">
        <h3>Con Barbella</h3>
        <ul>{#each after as t}<li><span class="mark"><Icon name="check" size={13} /></span>{t}</li>{/each}</ul>
      </article>
    </div>
  </div>
</section>

<style>
  .section { padding-block: 88px; }
  .cmp { display: grid; gap: 16px; max-width: 62rem; margin-inline: auto; }
  article { padding: 28px; }
  h3 { margin: 0 0 1.1rem; font-size: 1.5rem; }
  ul { list-style: none; margin: 0; padding: 0; display: grid; gap: 14px; }
  li { display: flex; align-items: flex-start; gap: 12px; font-size: 0.98rem; }
  .mark { flex: none; display: grid; place-items: center; width: 22px; height: 22px; margin-top: 1px; border-radius: 50%; background: var(--accent-soft); color: var(--accent); }
  .before { background: var(--surface-2); box-shadow: none; }
  .before li { color: var(--muted); }
  .before .mark { background: var(--surface); color: var(--muted); }
  .before .mark :global(svg) { display: none; }
  .before .mark::before { content: ''; width: 8px; height: 2px; border-radius: 2px; background: var(--muted); }
  .after { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent), var(--card-shadow); }
  @media (min-width: 820px) { .cmp { grid-template-columns: 1fr 1fr; } }
</style>
