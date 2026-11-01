<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { BILLING, KINDS, PLANS, ago, money, plural } from '$lib/format.js';
  import Timeline from '$lib/platform/Timeline.svelte';
  import { canWritePlatform, session } from '$lib/session.svelte.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

  const MODES = { open: 'Abierto', approval: 'Con aprobación', closed: 'Cerrado' };
  // tono de la etiqueta: 'bad' y 'warn' vienen de format.js y de la API
  const TONE = { '': 'neutral', warn: 'warning', bad: 'danger' };
  const KIND_TONE = { pending: 'warning', past_due: 'danger', trial_expired: 'danger', trial_ending: 'warning' };
  const LIMIT = 6;

  let d = $state(null);
  let activity = $state([]);
  let books = $state(null); // finanzas del mes, solo para el administrador
  let error = $state('');
  let showAll = $state(false);

  onMount(async () => {
    try {
      d = await api('/api/platform/overview');
      activity = await api('/api/platform/activity?limit=7').catch(() => []);
      if (canWritePlatform()) books = await api('/api/platform/finance').catch(() => null);
    } catch (e) {
      error = e.message;
    }
  });

  const hour = new Date().getHours();
  const hello = `${hour < 12 ? 'Buenos días' : hour < 19 ? 'Buenas tardes' : 'Buenas noches'}, ${session.me?.name?.split(' ')[0] ?? ''}`;
  const today = new Date().toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' });

  const waiting = (kind) => d.attention.filter((x) => x.kind === kind).length;
  const shown = $derived(d ? (showAll ? d.attention : d.attention.slice(0, LIMIT)) : []);
</script>

<PageHeader title="Hoy" description={today}>
  {#snippet actions()}
    {#if d}<a class="mode" href="/plataforma/ajustes?s=registro">Registro: {MODES[d.registration_mode]}</a>{/if}
  {/snippet}
</PageHeader>

{#if error}<InlineNotice tone="danger" message={error} />{/if}

{#if !d && !error}
  <LoadingState />
{/if}

{#if d}
  <div class="stack">
    <p class="hello">{hello}.{' '}{#if d.attention.length}{plural(d.attention.length, 'cosa necesita', 'cosas necesitan')} tu atención.{:else}Todo está en orden.{/if}</p>

    <section aria-labelledby="g-aten">
      <h2 id="g-aten" class="group">Para atender</h2>
      <div class="grid four">
        <MetricCard label="Por aprobar" value={waiting('pending')} hint={waiting('pending') ? 'negocios esperando tu visto bueno' : 'nadie en espera'} href="/plataforma/negocios?state=pending" />
        <MetricCard label="Pago atrasado" value={d.counts.past_due} hint={d.counts.past_due ? 'negocios por contactar' : 'todos al corriente'} href="/plataforma/negocios?state=past_due" />
        <MetricCard label="Pruebas por cerrar" value={d.counts.trial_expired + waiting('trial_ending')} hint="vencen pronto o ya vencieron" href="/plataforma/negocios?state=trial_expired" />
        <MetricCard label="Negocios activos" value={d.counts.active} hint="{d.counts.total} en total · {d.counts.trialing} en prueba" href="/plataforma/negocios?state=active" />
      </div>
    </section>

    {#if d.mrr_cents !== undefined || d.usage_30d}
      <section aria-labelledby="g-mes">
        <h2 id="g-mes" class="group">Cifras del mes</h2>
        <div class="grid">
          {#if books}
            <MetricCard label="Ganancia del mes" value={money(books.profit_cents)} hint="ingresos menos gastos" href="/plataforma/finanzas" />
            <MetricCard label="Cobrado este mes" value={money(books.revenue_cents)} hint="{books.clients.active} negocios que pagan" href="/plataforma/finanzas" />
            <MetricCard label="Gastos del mes" value={money(books.expenses_cents)} hint="{books.expenses.length} anotados" href="/plataforma/finanzas?v=gastos" />
          {/if}
          {#if d.mrr_cents !== undefined}
            <MetricCard label="Ingreso mensual recurrente" value={money(d.mrr_cents)} hint="lo que pagan al mes los activos" />
          {/if}
          <MetricCard label="Altas (30 días)" value={d.signups_30d} hint="negocios nuevos" href="/plataforma/negocios" />
          {#if d.usage_30d}
            <MetricCard label="Negocios con actividad" value={d.usage_30d.active_businesses} hint="de {d.counts.total} · {d.usage_30d.appointments} citas en 30 días" />
          {/if}
        </div>
      </section>
    {/if}

    <div class="cols">
      <Surface padding="flush" title="Necesita tu atención" description="De lo más urgente a lo menos; lo que lleva más tiempo esperando va primero.">
        {#if d.attention.length === 0}
          <EmptyState title="Todo en orden" description="No hay negocios por aprobar, pagos atrasados ni pruebas por vencer." />
        {:else}
          <ul class="att">
            {#each shown as x (x.kind + x.business_id)}
              <li class={KIND_TONE[x.kind]}>
                <span class="grow">
                  <span class="name">{x.name}</span>
                  <span class="meta">{x.detail}</span>
                </span>
                <StatusBadge tone={KIND_TONE[x.kind] ?? 'neutral'} label={x.title} />
                {#if x.at}<span class="when">{ago(x.at)}</span>{/if}
                <a class="btn ghost sm" href="/plataforma/negocios/{x.business_id}" aria-label="Abrir {x.name}">Abrir</a>
              </li>
            {/each}
          </ul>
          {#if d.attention.length > LIMIT}
            <div class="more"><button class="btn ghost sm" onclick={() => (showAll = !showAll)}>{showAll ? 'Ver menos' : `Ver los ${d.attention.length}`}</button></div>
          {/if}
        {/if}
      </Surface>

      <div class="side">
        <Surface padding="flush" title="Últimos registros">
          {#snippet actions()}<a class="btn ghost sm" href="/plataforma/negocios">Todos</a>{/snippet}
          {#if d.recent.length === 0}
            <EmptyState title="Todavía no hay negocios" />
          {:else}
            <ul class="recent">
              {#each d.recent as b (b.id)}
                {@const [label, tone] = BILLING[b.billing_state]}
                <li>
                  <a href="/plataforma/negocios/{b.id}">
                    <span class="grow">
                      <span class="name">{b.name}</span>
                      <span class="meta">{KINDS[b.kind]} · {PLANS[b.plan]} · {ago(b.created_at)}</span>
                    </span>
                    <StatusBadge {label} tone={TONE[tone] ?? 'neutral'} />
                  </a>
                </li>
              {/each}
            </ul>
          {/if}
        </Surface>

        <Surface title="Movimientos del equipo">
          {#snippet actions()}<a class="btn ghost sm" href="/plataforma/ajustes?s=actividad">Ver todo</a>{/snippet}
          <Timeline items={activity} />
        </Surface>
      </div>
    </div>
  </div>
{/if}

<style>
  .stack { display: grid; gap: 32px; }
  .hello { margin: 0; color: var(--muted); }
  .group { margin: 0 0 12px; font-size: 0.8rem; font-weight: 600; letter-spacing: 0.04em; text-transform: uppercase; color: var(--muted); }
  .grid { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); }
  @media (min-width: 760px) { .grid { gap: 16px; grid-template-columns: repeat(3, minmax(0, 1fr)); } .grid.four { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
  .cols { display: grid; gap: 20px; grid-template-columns: minmax(0, 1fr); align-items: start; }
  @media (min-width: 1100px) { .cols { grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr); } }
  .side { display: grid; gap: 20px; min-width: 0; }
  .mode { display: inline-flex; align-items: center; min-height: 44px; padding: 0 16px; border: 1px solid var(--control-border); border-radius: var(--radius-sm); color: var(--text); font-size: 0.9rem; font-weight: 500; text-decoration: none; }
  .mode:hover { border-color: var(--accent); }
  ul { list-style: none; margin: 0; padding: 0; }
  .att li { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; min-height: 56px; padding: 12px 20px; border-top: 1px solid var(--border); border-left: 3px solid transparent; }
  .att li.warning { border-left-color: var(--warn); }
  .att li.danger { border-left-color: var(--danger); }
  .grow { display: grid; gap: 2px; min-width: 0; flex: 1 1 160px; }
  .name { font-weight: 600; overflow-wrap: anywhere; }
  .meta { color: var(--muted); font-size: 0.85rem; overflow-wrap: anywhere; }
  .when { color: var(--muted); font-size: 0.8rem; white-space: nowrap; }
  .more { padding: 12px 20px; border-top: 1px solid var(--border); }
  .recent li { border-top: 1px solid var(--border); }
  .recent a { display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 56px; padding: 12px 20px; color: var(--text); text-decoration: none; }
  .recent a:hover { background: var(--surface-2); }
</style>
