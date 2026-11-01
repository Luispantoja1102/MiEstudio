<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { BILLING, KINDS, PLANS, formatDay, money } from '$lib/format.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  const MODES = { open: 'Abierto', approval: 'Con aprobación', closed: 'Cerrado' };

  let d = $state(null);
  let error = $state('');
  onMount(async () => {
    try {
      d = await api('/api/platform/overview');
    } catch (e) {
      error = e.message;
    }
  });

  const cards = $derived(
    d && [
      ['Negocios', d.counts.total, '/plataforma/negocios'],
      ['Activos', d.counts.active, '/plataforma/negocios?state=active'],
      ['En prueba', d.counts.trialing, '/plataforma/negocios?state=trialing'],
      ['Pago atrasado', d.counts.past_due + d.counts.trial_expired, '/plataforma/negocios?state=past_due'],
      ['Suspendidos', d.counts.suspended, '/plataforma/negocios?state=suspended'],
      ['Altas (30 días)', d.signups_30d, '/plataforma/negocios']
    ]
  );
  // tono de la etiqueta: 'bad' y 'warn' vienen de format.js y de la API
  const TONE = { '': 'neutral', warn: 'warning', bad: 'danger' };
  const KIND_TONE = { pending: 'warning', past_due: 'danger', trial_expired: 'danger', trial_ending: 'warning' };

  const columns = [
    { key: 'name', label: 'Negocio' },
    { key: 'kind', label: 'Giro' },
    { key: 'plan', label: 'Plan' },
    { key: 'owner_email', label: 'Titular' },
    { key: 'created_at', label: 'Alta' },
    { key: 'state', label: 'Estado' }
  ];
</script>

<PageHeader title="Resumen de la plataforma" description="Negocios, ingresos y lo que necesita tu atención.">
  {#snippet actions()}
    {#if d}<a class="mode" href="/plataforma/ajustes">Registro: {MODES[d.registration_mode]}</a>{/if}
  {/snippet}
</PageHeader>

{#if error}<InlineNotice tone="danger" message={error} />{/if}

{#if !d && !error}
  <LoadingState />
{/if}

{#if d}
  <div class="stack">
    {#if d.mrr_cents !== undefined}
      <section aria-labelledby="g-ing">
        <h2 id="g-ing" class="group">Ingresos</h2>
        <div class="grid two">
          <MetricCard label="Ingreso mensual recurrente" value={money(d.mrr_cents)} hint="Lo que pagan al mes los negocios activos, según su último pago." />
          <MetricCard label="Cobrado este mes" value={money(d.paid_this_month_cents)} hint="Ver pagos" href="/plataforma/pagos" />
        </div>
      </section>
    {/if}

    <section aria-labelledby="g-neg">
      <h2 id="g-neg" class="group">Negocios por estado</h2>
      <div class="grid six">
        {#each cards as [label, value, href]}
          <MetricCard {label} {value} {href} />
        {/each}
      </div>
    </section>

    {#if d.usage_30d}
      <section aria-labelledby="g-uso">
        <h2 id="g-uso" class="group">Uso en los últimos 30 días</h2>
        <div class="grid four">
          <MetricCard label="Citas creadas" value={d.usage_30d.appointments} />
          <MetricCard label="Reservas en línea" value={d.usage_30d.online_bookings} />
          <MetricCard label="Ventas cobradas" value={d.usage_30d.sales} />
          <MetricCard label="Negocios con actividad" value={d.usage_30d.active_businesses} hint="de {d.counts.total}" />
        </div>
      </section>
    {/if}

    <Surface padding="flush" title="Requieren atención">
      {#snippet actions()}{#if d.attention.length}<StatusBadge tone="warning" label={String(d.attention.length)} />{/if}{/snippet}
      {#if d.attention.length === 0}
        <EmptyState title="Nada pendiente" description="Todo en orden." />
      {:else}
        <ul class="att">
          {#each d.attention as x (x.kind + x.business_id)}
            <li class={KIND_TONE[x.kind]}>
              <a href="/plataforma/negocios/{x.business_id}">
                <span class="grow">
                  <span class="name">{x.name}</span>
                  <span class="meta">{x.detail}{#if x.at}{' · '}{formatDay(x.at)}{/if}</span>
                </span>
                <StatusBadge tone={KIND_TONE[x.kind] ?? 'neutral'} label={x.title} />
              </a>
            </li>
          {/each}
        </ul>
      {/if}
    </Surface>

    <Surface padding="flush" title="Últimos registros">
      {#if d.recent.length === 0}
        <EmptyState title="Todavía no hay registros" />
      {:else}
        <DataTable caption="Últimos negocios registrados" {columns} rows={d.recent}>
          {#snippet cell({ row, column })}
            {#if column.key === 'name'}
              <a class="link" href="/plataforma/negocios/{row.id}">{row.name}</a>
            {:else if column.key === 'kind'}
              {KINDS[row.kind]}
            {:else if column.key === 'plan'}
              {PLANS[row.plan]}
            {:else if column.key === 'created_at'}
              {formatDay(row.created_at)}
            {:else if column.key === 'state'}
              {@const [label, tone] = BILLING[row.billing_state]}
              <StatusBadge {label} tone={TONE[tone] ?? 'neutral'} />
            {:else}
              <span class="mail">{row[column.key]}</span>
            {/if}
          {/snippet}
        </DataTable>
      {/if}
    </Surface>
  </div>
{/if}

<style>
  .stack { display: grid; gap: 32px; }
  .group { margin: 0 0 12px; font-size: 0.8rem; font-weight: 600; letter-spacing: 0.04em; text-transform: uppercase; color: var(--muted); }
  .grid { display: grid; gap: 12px; grid-template-columns: repeat(2, minmax(0, 1fr)); }
  @media (min-width: 760px) {
    .grid { gap: 16px; }
    .grid.six { grid-template-columns: repeat(3, minmax(0, 1fr)); }
    .grid.four { grid-template-columns: repeat(4, minmax(0, 1fr)); }
  }
  @media (min-width: 1180px) { .grid.six { grid-template-columns: repeat(6, minmax(0, 1fr)); } }
  .grid.two :global(.metric) { grid-column: auto; }
  @media (max-width: 479px) { .grid.two { grid-template-columns: minmax(0, 1fr); } }
  .mode { display: inline-flex; align-items: center; min-height: 44px; padding: 0 16px; border: 1px solid var(--control-border); border-radius: var(--radius-sm); color: var(--text); font-size: 0.9rem; font-weight: 500; text-decoration: none; }
  .mode:hover { border-color: var(--accent); }
  .att { list-style: none; margin: 0; padding: 0; }
  .att li { border-top: 1px solid var(--border); border-left: 3px solid transparent; }
  .att li.warning { border-left-color: var(--warn); }
  .att li.danger { border-left-color: var(--danger); }
  .att a { display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 56px; padding: 12px 20px; color: var(--text); text-decoration: none; }
  .att a:hover { background: var(--surface-2); }
  .grow { display: grid; gap: 2px; min-width: 0; }
  .name { font-weight: 600; overflow-wrap: anywhere; }
  .meta { color: var(--muted); font-size: 0.85rem; overflow-wrap: anywhere; }
  .link { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; color: var(--text); text-decoration: none; overflow-wrap: anywhere; }
  .link:hover { color: var(--accent); text-decoration: underline; }
  .mail { overflow-wrap: anywhere; color: var(--muted); }
</style>
