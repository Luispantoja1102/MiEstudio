<script>
  import { page } from '$app/state';
  import CashCloseTicket from '$lib/pos/CashCloseTicket.svelte';
  import TicketView from '$lib/pos/TicketView.svelte';
  import { api } from '$lib/api.js';
</script>

<!-- Corte de caja: se ve, se imprime con el navegador o se convierte en imagen para la térmica -->
<TicketView load={() => api(`/api/cash/sessions/${page.params.id}`)} title={(r) => `Corte de caja · turno ${r.id}`}>
  {#snippet children(report, paper, tz)}<CashCloseTicket {report} {paper} {tz} />{/snippet}
</TicketView>
