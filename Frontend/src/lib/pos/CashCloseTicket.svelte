<script>
  import './ticket.css';
  import { METHODS, money } from '$lib/format.js';

  // El corte de caja en papel térmico (58 u 80 mm): turno cerrado (Z) o corte parcial con la caja abierta (X).
  let { report, tz = 'America/Tijuana', paper = '80' } = $props();

  const fmt = (iso, opts) => new Intl.DateTimeFormat('es-MX', { timeZone: tz, hour12: false, ...opts }).format(new Date(iso));
  const day = (iso) => fmt(iso, { day: '2-digit', month: 'short', year: 'numeric' }).replace(/\./g, '');
  const clock = (iso) => fmt(iso, { hour: '2-digit', minute: '2-digit' });
  const closed = $derived(Boolean(report.closed_at));
  const total = $derived(report.by_method.reduce((n, m) => n + m.amount_cents, 0));
  const diff = $derived(report.diff_cents ?? 0);
  const verdict = $derived(diff === 0 ? 'Cuadra' : diff > 0 ? 'Sobrante' : 'Faltante');
  const tickets = $derived(report.tickets ?? []);
</script>

<article class="tk-paper" class:w58={paper === '58'} aria-label="Corte de caja">
  <header class="tk-head">
    <p class="tk-shop">{report.business_name}</p>
    {#if report.branch_name}<p class="tk-addr">{report.branch_name}</p>{/if}
    {#if report.branch_address}<p class="tk-addr">{report.branch_address}</p>{/if}
    {#if report.branch_phone}<p class="tk-addr">Tel. {report.branch_phone}</p>{/if}
  </header>

  <div class="tk-total"><span>Corte de caja</span><strong class="small">{closed ? 'Z · Final' : 'X · Parcial'}</strong></div>

  <div class="tk-stub">
    <div><span>Turno</span><strong>Nº {report.id}</strong></div>
    <div><p>{day(report.opened_at)}</p></div>
  </div>
  <div class="tk-meta"><span>Abrió <b>{report.opened_by || '—'}</b></span><span>{clock(report.opened_at)}</span></div>
  <div class="tk-meta">
    <span>{closed ? 'Cerró' : 'Corte'} <b>{closed ? report.closed_by || '—' : 'Caja abierta'}</b></span>
    <span>{closed ? clock(report.closed_at) : clock(new Date().toISOString())}</span>
  </div>

  <p class="tk-sec">Resumen</p>
  <div class="tk-line"><span>Ventas</span><i class="tk-dots"></i><span>{report.sales}</span></div>
  <div class="tk-line"><span>Vendido (sin propinas)</span><i class="tk-dots"></i><span>{money(report.sales_cents)}</span></div>
  <div class="tk-line"><span>Propinas</span><i class="tk-dots"></i><span>{money(report.tip_cents)}</span></div>
  {#if report.sales}<div class="tk-line"><span>Promedio por venta</span><i class="tk-dots"></i><span>{money(Math.round(report.sales_cents / report.sales))}</span></div>{/if}
  {#if report.voided_count}<div class="tk-line strong"><span>Canceladas ({report.voided_count})</span><i class="tk-dots"></i><span>{money(report.voided_cents)}</span></div>{/if}

  <p class="tk-sec">Cobrado por forma de pago</p>
  {#each report.by_method as m}
    <div class="tk-line"><span>{METHODS[m.method] || m.method}</span><i class="tk-dots"></i><span>{money(m.amount_cents)}</span></div>
  {:else}
    <p class="tk-note">Sin ventas en este turno.</p>
  {/each}
  <div class="tk-total"><span>Cobrado</span><strong>{money(total)}</strong></div>

  <p class="tk-sec">Cuadre del cajón</p>
  <div class="tk-line"><span>Fondo inicial</span><i class="tk-dots"></i><span>{money(report.opening_cents)}</span></div>
  <div class="tk-line"><span>+ Ventas en efectivo</span><i class="tk-dots"></i><span>{money(report.cash_in_cents)}</span></div>
  {#if report.moves_in_cents}<div class="tk-line"><span>+ Entradas</span><i class="tk-dots"></i><span>{money(report.moves_in_cents)}</span></div>{/if}
  {#if report.moves_out_cents}<div class="tk-line"><span>− Salidas</span><i class="tk-dots"></i><span>−{money(report.moves_out_cents)}</span></div>{/if}
  <hr class="tk-rule" />
  <div class="tk-line strong"><span>= Debe haber</span><i class="tk-dots"></i><span>{money(report.expected_cents)}</span></div>
  {#if closed}
    <div class="tk-line strong"><span>Se contó</span><i class="tk-dots"></i><span>{money(report.counted_cents)}</span></div>
    <div class="tk-stamp-row">
      <div class="tk-stamp"><b>{verdict}</b>{#if diff}<small>{money(Math.abs(diff))}</small>{/if}</div>
    </div>
  {/if}

  {#if report.movements?.length}
    <p class="tk-sec">Entradas y salidas</p>
    {#each report.movements as m (m.id)}
      <div class="tk-line"><span>{clock(m.created_at)} {m.concept}</span><i class="tk-dots"></i><span>{m.kind === 'in' ? '+' : '−'}{money(m.amount_cents)}</span></div>
    {/each}
  {/if}

  {#if report.note}
    <p class="tk-sec">Observaciones</p>
    <p class="tk-note left">{report.note}</p>
  {/if}

  {#if tickets.length}
    <p class="tk-sec">Ventas del turno · {tickets.length}</p>
    {#each tickets as t}
      <div class="tk-line" class:void={t.status === 'void'}>
        <span>{clock(t.created_at)} #{t.folio}{t.status === 'void' ? ' (cancelada)' : ''}</span><i class="tk-dots"></i><span>{money(t.total_cents + t.tip_cents)}</span>
      </div>
    {/each}
  {/if}

  <div class="tk-sign">
    <div><i></i><span>Entregó</span><b>{report.closed_by || report.opened_by}</b></div>
    <div><i></i><span>Recibió</span></div>
  </div>
  <p class="tk-made">Hecho con <b>Barbella</b></p>
</article>

<style>
  /* lo propio del corte; el resto del diseño viene de ticket.css */
  .small { font-size: 1.2em; }
  .left { text-align: left; }
  .void { text-decoration: line-through; opacity: 0.7; }
  .tk-sign { display: grid; grid-template-columns: 1fr 1fr; gap: 6mm; margin-top: 10mm; }
  .tk-sign div { display: grid; gap: 0.6mm; text-align: center; font-size: 0.85em; }
  .tk-sign i { border-top: 1.5px solid #000; }
  .tk-sign span { font-weight: 800; letter-spacing: 0.1em; text-transform: uppercase; font-size: 0.8em; }
  .tk-sign b { text-transform: uppercase; font-weight: 700; }
</style>
