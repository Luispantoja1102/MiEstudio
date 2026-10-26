<script>
  import { METHODS, money } from '$lib/format.js';

  // El ticket de una venta, en tinta negra sobre papel térmico (58 u 80 mm). Lo usan la ventana de impresión
  // (/imprimir/venta/[id]), la impresión directa en la térmica (se convierte en imagen) y el historial.
  let { sale, tz = 'America/Tijuana', paper = '80' } = $props();

  const stamp = (iso) =>
    new Intl.DateTimeFormat('es-MX', { timeZone: tz, day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })
      .format(new Date(iso))
      .replace(/\./g, '')
      .replace(',', ' ·');

  const units = $derived(sale.items.reduce((n, i) => n + i.qty, 0));
  const void_ = $derived(sale.status === 'void');
  const received = $derived(sale.payments.find((p) => p.method === 'cash' && p.received_cents && p.received_cents > p.amount_cents));
</script>

<article class="tk-paper" class:w58={paper === '58'} aria-label="Ticket de venta">
  <header class="tk-head">
    <p class="tk-shop">{sale.business_name}</p>
    {#if sale.branch_name}<p class="tk-addr">{sale.branch_name}</p>{/if}
    {#if sale.branch_address}<p class="tk-addr">{sale.branch_address}</p>{/if}
    {#if sale.branch_phone}<p class="tk-addr">Tel. {sale.branch_phone}</p>{/if}
  </header>

  <div class="tk-stub">
    <div><span>Venta</span><strong>Nº {sale.folio}</strong></div>
    <div><p>{stamp(sale.created_at)}</p></div>
  </div>
  <div class="tk-meta">
    <span>Atendió <b>{sale.cashier || '—'}</b></span>
    <span>{units} {units === 1 ? 'artículo' : 'artículos'}</span>
  </div>
  {#if sale.client_name}<p class="tk-meta1">Cliente <b>{sale.client_name}</b></p>{/if}

  <p class="tk-sec">Lo que llevas</p>
  <div class="tk-items">
    {#each sale.items as it, i (it.id)}
      <div class="tk-item">
        <span class="tk-n">{String(i + 1).padStart(2, '0')}</span>
        <div>
          <p class="tk-name">{it.name}</p>
          {#if it.staff_name}<p class="tk-note">{it.staff_name}</p>{/if}
          <div class="tk-line"><span>{it.qty} × {money(it.unit_price_cents)}</span><i class="tk-dots"></i><span>{money(it.total_cents)}</span></div>
        </div>
      </div>
    {/each}
  </div>

  <hr class="tk-rule" />
  <div class="tk-line"><span>Subtotal</span><i class="tk-dots"></i><span>{money(sale.subtotal_cents)}</span></div>
  {#if sale.discount_cents}<div class="tk-line strong"><span>Descuento</span><i class="tk-dots"></i><span>−{money(sale.discount_cents)}</span></div>{/if}

  <div class="tk-total"><span>Total</span><strong>{money(sale.total_cents)}</strong></div>
  {#if sale.tip_cents}
    <div class="tk-line tip"><span>Propina{sale.tip_staff_name ? ` · ${sale.tip_staff_name}` : ''}</span><i class="tk-dots"></i><span>{money(sale.tip_cents)}</span></div>
  {/if}

  {#if !void_}
    <div class="tk-pay" class:single={!sale.change_cents}>
      <div>
        <span>Pagó con</span>
        {#each sale.payments as p}
          <strong>{METHODS[p.method] || p.method} {money(p.amount_cents)}</strong>
          {#if p.reference}<em>Ref. {p.reference}</em>{/if}
        {/each}
        {#if received}<em>Recibido {money(received.received_cents)}</em>{/if}
      </div>
      {#if sale.change_cents}
        <div class="tk-change"><span>Su cambio</span><strong>{money(sale.change_cents)}</strong></div>
      {/if}
    </div>
  {/if}

  <div class="tk-stamp-row">
    <div class="tk-stamp"><b>{void_ ? 'Cancelada' : 'Pagado'}</b></div>
  </div>
  {#if void_ && sale.void_reason}<p class="tk-flag">{sale.void_reason}</p>{/if}

  <p class="tk-thanks">¡Gracias por tu visita!</p>
  <p class="tk-made">Hecho con <b>Barbella</b></p>
</article>

<style>
  .tk-paper {
    --tk-w: 80mm;
    --tk-pad: 4mm;
    position: relative;
    width: var(--tk-w);
    max-width: 100%;
    margin: 0 auto;
    padding: 5mm var(--tk-pad) 7mm;
    background: #fff;
    color: #000;
    /* tipografía del sistema: se ve igual en pantalla, al imprimir y al convertirse en imagen para la térmica */
    font-family: Arial, 'Helvetica Neue', Helvetica, sans-serif;
    font-size: 12px;
    line-height: 1.35;
    font-variant-numeric: tabular-nums;
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
  }
  .tk-paper.w58 { --tk-w: 58mm; --tk-pad: 3.5mm; font-size: 10.5px; }
  p { margin: 0; }
  .tk-paper * { box-sizing: border-box; }

  .tk-head { display: grid; justify-items: center; gap: 0.6mm; text-align: center; }
  .tk-shop { font-weight: 900; font-size: 1.5em; line-height: 1.1; text-transform: uppercase; overflow-wrap: anywhere; }
  .tk-addr { font-size: 0.9em; }

  .tk-stub { display: grid; grid-template-columns: minmax(0, 1fr) auto; margin-top: 3mm; border: 1.5px solid #000; border-radius: 1.6mm; }
  .tk-stub > div { padding: 1.4mm 2.2mm; min-width: 0; }
  .tk-stub > div + div { border-left: 1.5px dashed #000; text-align: right; font-weight: 800; white-space: nowrap; }
  .tk-stub span { display: block; font-size: 0.72em; font-weight: 800; letter-spacing: 0.18em; text-transform: uppercase; }
  .tk-stub strong { display: block; font-size: 1.55em; font-weight: 900; line-height: 1.1; letter-spacing: 0.04em; white-space: nowrap; }
  .tk-meta { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 0.5mm 2mm; margin-top: 1.6mm; font-size: 0.9em; }
  .tk-meta1 { margin-top: 0.6mm; font-size: 0.9em; }
  .tk-meta b, .tk-meta1 b { font-weight: 800; text-transform: uppercase; }

  .tk-sec { display: flex; align-items: center; gap: 2mm; margin: 4mm 0 2mm; font-size: 0.78em; font-weight: 900; letter-spacing: 0.2em; text-transform: uppercase; white-space: nowrap; }
  .tk-sec::before, .tk-sec::after { content: ''; flex: 1; border-top: 1.5px solid #000; }
  .tk-sec::before { flex: 0 0 3mm; }

  .tk-line { display: flex; align-items: baseline; gap: 1mm; min-width: 0; }
  .tk-dots { flex: 1 1 4mm; min-width: 4mm; border-bottom: 1.5px dotted #000; transform: translateY(-0.6mm); }
  .tk-line > :first-child { min-width: 0; overflow-wrap: anywhere; }
  .tk-line > :last-child { flex-shrink: 0; text-align: right; white-space: nowrap; }
  .tk-line + .tk-line { margin-top: 0.8mm; }
  .tk-line.strong { font-weight: 800; }
  .tk-line.tip { margin-top: 1.6mm; font-weight: 700; }

  .tk-items { display: grid; gap: 2mm; }
  .tk-item { display: grid; grid-template-columns: 5.4mm minmax(0, 1fr); column-gap: 1.2mm; align-items: start; }
  .tk-n { display: grid; place-items: center; height: 4.2mm; margin-top: 0.2mm; border: 1.2px solid #000; border-radius: 1mm; font-size: 0.72em; font-weight: 800; }
  .tk-name { font-weight: 800; line-height: 1.2; overflow-wrap: anywhere; }
  .tk-note { font-size: 0.86em; font-style: italic; }
  .tk-item .tk-line { margin-top: 0.4mm; font-size: 0.95em; }
  .tk-rule { height: 0; margin: 2.5mm 0; border: none; border-top: 1.5px dashed #000; }

  .tk-total { display: flex; align-items: center; justify-content: space-between; gap: 2mm; margin: 3mm calc(-1 * var(--tk-pad) + 1mm) 0; padding: 2mm 2.5mm; background: #000; color: #fff; border-radius: 1.2mm; }
  .tk-total span { font-size: 0.95em; font-weight: 900; letter-spacing: 0.22em; text-transform: uppercase; }
  .tk-total strong { font-size: 1.9em; font-weight: 900; line-height: 1; letter-spacing: -0.02em; white-space: nowrap; }

  .tk-pay { display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr); margin-top: 2.5mm; border: 1.5px solid #000; border-radius: 1.2mm; }
  .tk-pay > div { padding: 1.4mm 2mm; min-width: 0; }
  .tk-pay > div + div { border-left: 1.5px solid #000; text-align: right; }
  .tk-pay.single { grid-template-columns: minmax(0, 1fr); }
  .tk-pay span { display: block; font-size: 0.72em; font-weight: 800; letter-spacing: 0.16em; text-transform: uppercase; }
  .tk-pay strong { display: block; font-size: 1.05em; font-weight: 800; overflow-wrap: anywhere; }
  .tk-pay em { display: block; font-style: normal; font-size: 0.9em; }
  .tk-pay .tk-change strong { font-size: 1.45em; font-weight: 900; line-height: 1.1; }

  .tk-stamp-row { display: flex; justify-content: center; margin: 4mm 0 1mm; }
  .tk-stamp { padding: 1.2mm 4mm 1.4mm; border: 4px double #000; border-radius: 2.2mm; transform: rotate(-5deg); text-align: center; }
  .tk-stamp b { display: block; font-size: 1.55em; font-weight: 900; line-height: 1; letter-spacing: 0.2em; margin-right: -0.2em; text-transform: uppercase; }
  .tk-flag { margin-top: 2mm; padding: 1.2mm 2mm; border: 1.5px dashed #000; border-radius: 1.2mm; text-align: center; font-size: 0.9em; font-weight: 800; }
  .tk-thanks { margin-top: 4mm; text-align: center; font-weight: 800; font-size: 1.1em; }
  .tk-made { margin-top: 2mm; text-align: center; font-size: 0.74em; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; }
  .tk-made b { font-weight: 900; }
</style>
