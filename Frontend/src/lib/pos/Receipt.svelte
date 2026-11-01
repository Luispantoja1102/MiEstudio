<script>
  import { METHODS, formatDate, money } from '$lib/format.js';

  let { sale, business = '' } = $props();
</script>

<article class="receipt print-area" aria-label="Comprobante">
  <header>
    <h3>{business}</h3>
    <p class="meta">Venta #{sale.folio} · {formatDate(sale.created_at)}</p>
    {#if sale.client_name}<p class="meta">Cliente: {sale.client_name}</p>{/if}
    {#if sale.status === 'void'}<p class="void">CANCELADA — {sale.void_reason}</p>{/if}
  </header>

  <table>
    <tbody>
      {#each sale.items as it (it.id)}
        <tr>
          <td>{it.qty} × {it.name}{#if it.staff_name}<small> · {it.staff_name}</small>{/if}</td>
          <td class="num">{money(it.total_cents)}</td>
        </tr>
      {/each}
    </tbody>
    <tfoot>
      {#if sale.discount_cents}
        <tr><td>Subtotal</td><td class="num">{money(sale.subtotal_cents)}</td></tr>
        <tr><td>Descuento</td><td class="num">−{money(sale.discount_cents)}</td></tr>
      {/if}
      <tr class="strong"><td>Total</td><td class="num">{money(sale.total_cents)}</td></tr>
      {#if sale.tip_cents}<tr><td>Propina{sale.tip_staff_name ? ` (${sale.tip_staff_name})` : ''}</td><td class="num">{money(sale.tip_cents)}</td></tr>{/if}
      {#each sale.payments as p}
        <tr class="pay"><td>{METHODS[p.method]}{p.reference ? ` · ${p.reference}` : ''}</td><td class="num">{money(p.amount_cents)}</td></tr>
      {/each}
      {#if sale.change_cents}<tr class="strong"><td>Cambio</td><td class="num">{money(sale.change_cents)}</td></tr>{/if}
    </tfoot>
  </table>
  <p class="thanks meta">¡Gracias por tu visita!</p>
</article>

<style>
  .receipt { max-width: 380px; margin: 0 auto; padding: 1.5rem; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  header { text-align: center; margin-bottom: 0.75rem; }
  h3 { margin: 0 0 0.2rem; font-size: 1.2rem; }
  header { padding-bottom: 0.75rem; border-bottom: 1px dashed var(--border); }
  header p { margin: 0; }
  .void { color: var(--danger); font-weight: 700; margin-top: 0.4rem; }
  table { width: 100%; border-collapse: collapse; font-size: 0.92rem; }
  td { padding: 4px 0; vertical-align: top; }
  .num { text-align: right; white-space: nowrap; font-variant-numeric: tabular-nums lining-nums; }
  small { color: var(--muted); }
  tfoot tr:first-child td { border-top: 1px solid var(--border); padding-top: 8px; }
  .strong td { font-weight: 700; }
  .pay td { color: var(--muted); }
  .thanks { text-align: center; margin: 0.9rem 0 0; }

  @media print {
    :global(body *) { visibility: hidden; }
    :global(.print-area), :global(.print-area *) { visibility: visible; }
    :global(.print-area.receipt) { position: fixed; left: 0; top: 0; box-sizing: border-box; width: 100%; max-width: none; margin: 0; padding: 12mm; border: 0; border-radius: 0; background: #fff; color: #000; }
  }
</style>
