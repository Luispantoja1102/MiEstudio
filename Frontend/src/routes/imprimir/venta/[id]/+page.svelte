<script>
  import { page } from '$app/state';
  import Receipt from '$lib/pos/Receipt.svelte';
  import TicketView from '$lib/pos/TicketView.svelte';
  import { api } from '$lib/api.js';
</script>

<!-- Ticket de una venta: se ve, se imprime con el navegador o se convierte en imagen para la térmica -->
<TicketView load={() => api(`/api/sales/${page.params.id}`)} title={(sale) => `Ticket #${sale.folio}`}>
  {#snippet children(sale, paper, tz)}<Receipt {sale} {paper} {tz} />{/snippet}
</TicketView>
