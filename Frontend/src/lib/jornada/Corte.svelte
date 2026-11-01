<script>
  // 20:30. Corte de caja: el recibo real de la caja (el mismo componente del punto de venta) y, al lado, el cierre y las comisiones del día.
  import Receipt from '$lib/pos/Receipt.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';

  // Ejemplo ilustrativo. Las horas se arman con componentes locales para que se vean igual en cualquier zona horaria.
  const venta = {
    folio: 214,
    created_at: new Date(2026, 10, 19, 20, 31).toISOString(),
    client_name: 'Andrea Solís',
    status: 'paid',
    items: [
      { id: 1, qty: 1, name: 'Corte y barba', staff_name: 'Marco', total_cents: 25000 },
      { id: 2, qty: 1, name: 'Pomada mate', staff_name: '', total_cents: 12000 }
    ],
    subtotal_cents: 37000,
    discount_cents: 0,
    total_cents: 37000,
    tip_cents: 4000,
    tip_staff_name: 'Marco',
    change_cents: 0,
    payments: [
      { method: 'deposit', amount_cents: 7500, reference: '' },
      { method: 'mp_point', amount_cents: 33500, reference: '' }
    ]
  };
  const caja = [['Fondo inicial', '$500.00'], ['Ventas en efectivo', '$1,240.00'], ['Salidas', '−$200.00'], ['Efectivo esperado', '$1,540.00'], ['Conteo', '$1,540.00']];
  const comisiones = [['Marco', 'Barbería', '$612'], ['Dani', 'Uñas', '$540'], ['Sofi', 'Pestañas', '$455'], ['Lu', 'Spa', '$690']];
</script>

<section class="corte" aria-labelledby="t-corte">
  <div class="envoltura">
    <div class="intro">
      <div>
        <p class="kicker"><time>20:30</time>Corte de caja</p>
        <h2 id="t-corte" class="titulo">Cierras con la caja cuadrada.</h2>
      </div>
      <p class="lead">Terminas la cita y pasas directo al cobro: se aplica el anticipo, se suma la propina y se calcula la comisión. Efectivo, tarjeta, transferencia o terminal de Mercado Pago.</p>
    </div>

    <div class="cuerpo">
      <div class="recibo">
        <Receipt sale={venta} business="Barbería y Salón Ébano" />
        <p class="ejemplo">Ejemplo ilustrativo</p>
      </div>

      <div class="resumen">
        <section aria-labelledby="h-caja">
          <h3 id="h-caja">Cierre de caja · Caja 1</h3>
          <dl>
            {#each caja as [k, v]}<div class:fuerte={k === 'Efectivo esperado'}><dt>{k}</dt><dd>{v}</dd></div>{/each}
            <div><dt>Diferencia</dt><dd>$0.00 <StatusBadge label="Cuadrada" tone="success" /></dd></div>
          </dl>
        </section>
        <section aria-labelledby="h-com">
          <h3 id="h-com">Comisiones del día</h3>
          <dl>
            {#each comisiones as [quien, oficio, monto]}<div><dt>{quien} <small>{oficio}</small></dt><dd>{monto}</dd></div>{/each}
          </dl>
        </section>
        <p class="ejemplo">Ejemplo ilustrativo</p>
      </div>
    </div>
  </div>
</section>

<style>
  .corte { padding: clamp(72px, 9vw, 112px) 0; background: var(--ticket); color: var(--tinta); }
  .intro { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 0.9fr); align-items: end; gap: 20px clamp(32px, 6vw, 96px); margin-bottom: clamp(32px, 4vw, 56px); }
  .titulo { max-width: 14ch; }
  .lead { margin: 0; max-width: 50ch; line-height: 1.55; color: var(--tinta-suave); text-wrap: pretty; }

  .cuerpo { display: grid; grid-template-columns: minmax(0, 380px) minmax(0, 1fr); align-items: start; gap: clamp(32px, 6vw, 96px); }
  .recibo :global(.receipt) { margin: 0; max-width: none; }
  .recibo .ejemplo, .resumen > .ejemplo { margin-top: 12px; }

  .resumen { display: grid; gap: 36px; max-width: 520px; }
  h3 { margin: 0 0 4px; padding-bottom: 10px; border-bottom: 2px solid var(--tinta); font-size: 0.82rem; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--tinta-suave); }
  dl { margin: 0; }
  dl > div { display: flex; justify-content: space-between; align-items: center; gap: 16px; padding: 12px 0; border-bottom: 1px solid var(--border); }
  dt { font-weight: 500; }
  dt small { margin-left: 6px; color: var(--tinta-suave); }
  dd { margin: 0; display: flex; align-items: center; gap: 10px; font-family: var(--rotulo); font-stretch: 85%; font-weight: 700; font-size: 1.25rem; font-variant-numeric: tabular-nums; }
  .fuerte dt { font-weight: 700; }

  @media (max-width: 860px) {
    .intro, .cuerpo { grid-template-columns: 1fr; }
    .recibo { max-width: 380px; }
  }
</style>
