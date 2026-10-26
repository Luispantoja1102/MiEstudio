<script>
  import './ticket.css';
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
