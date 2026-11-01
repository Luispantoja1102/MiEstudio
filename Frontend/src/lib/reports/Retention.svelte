<script>
  import { api } from '$lib/api.js';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import MetricCard from '$lib/ui/MetricCard.svelte';
  import { money } from '$lib/format.js';

  // Retención: cuántos clientes regresan, cuáles son nuevos y a quiénes se está perdiendo.
  let { from, to, branch = 0 } = $props();

  let r = $state(null);
  let error = $state('');
  let loading = $state(true);

  $effect(() => {
    const params = new URLSearchParams({ from, to });
    if (branch) params.set('branch_id', branch);
    loading = true;
    api('/api/reports/retention?' + params)
      .then((data) => ((r = data), (error = '')))
      .catch((e) => (error = e.message))
      .finally(() => (loading = false));
  });

  const pctText = (v) => (v == null ? '—' : `${Math.round(v)} %`);
  const ago = (d) => (d < 31 ? `hace ${d} días` : d < 365 ? `hace ${Math.round(d / 30)} meses` : 'hace más de un año');
  const wa = (c) => {
    const digits = c.phone.replace(/\D/g, '');
    return digits ? `https://wa.me/${digits.length === 10 ? '52' + digits : digits}` : '';
  };
</script>

{#if error}<InlineNotice tone="danger" message={error} />{/if}

{#if r}
  <div class="wrap" class:dim={loading}>
    {#if r.total_clients === 0}
      <Surface><EmptyState title="Todavía no hay clientes con ventas" description="Al cobrar, elige al cliente en el ticket para que aparezca aquí su historial." /></Surface>
    {:else}
      <div class="kpis">
        <MetricCard label="Clientes activos" value={r.active} hint={`vinieron en el periodo`} />
        <MetricCard label="Nuevos" value={r.new} hint={`su primera visita fue ahora`} />
        <MetricCard label="Regresaron" value={r.returning} hint={`ya habían venido antes`} />
        <MetricCard label="Retención" value={pctText(r.retention_pct)} hint={r.prev_active ? `${r.retained} de ${r.prev_active} clientes del periodo anterior volvieron` : 'Sin clientes en el periodo anterior'} />
        <MetricCard label="Visitas por cliente" value={r.visits_per_client == null ? '—' : r.visits_per_client.toFixed(1)} hint={`${r.repeaters} vinieron más de una vez`} />
        <MetricCard label="Vuelven cada" value={r.avg_gap_days == null ? '—' : `${Math.round(r.avg_gap_days)} días`} hint={`de quienes han venido más de una vez`} />
      </div>
      <p class="meta" style="margin-top: 1rem">{r.one_visit} de {r.total_clients} clientes han venido una sola vez. Cada visita es un día en que se les cobró algo.</p>

      <Surface title="Clientes en riesgo">
        <p class="meta">Han venido al menos dos veces y llevan más de {r.lapsed_days} días sin volver. Escríbeles antes de perderlos; en <a href="/panel/mensajes">Mensajes</a> puedes activar el aviso automático de reactivación.</p>
        <div class="list">
          {#each r.lapsed as c (c.client_id)}
            <div class="item-row">
              <span class="grow"><a class="title" href={`/panel/clientes/${c.client_id}`}>{c.name}</a><small class="meta">{' · '}{c.visits} visitas · última {ago(c.days_ago)}</small></span>
              <b>{money(c.spent_cents)}</b>
              {#if wa(c)}<a class="btn ghost sm" href={wa(c)} target="_blank" rel="noopener">WhatsApp</a>{/if}
            </div>
          {:else}
            <EmptyState title="Nadie en riesgo por ahora." />
          {/each}
        </div>
      </Surface>

      <Surface title="Mejores clientes del periodo">
        <div class="list">
          {#each r.top as c, i (c.client_id)}
            <div class="item-row"><span class="rank">{i + 1}</span><a class="grow title" href={`/panel/clientes/${c.client_id}`}>{c.name}</a><b>{money(c.spent_cents)}</b></div>
          {:else}
            <EmptyState title="Sin ventas a clientes en este periodo." />
          {/each}
        </div>
      </Surface>
    {/if}
  </div>
{/if}

<style>
  .dim { opacity: 0.6; transition: opacity 0.2s; }
  .kpis { margin: 0 0 1rem; display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); }
  .meta { margin: 0.75rem 0 0; color: var(--muted); font-size: 0.9rem; }
  small { color: var(--muted); }
  .wrap { display: grid; gap: 16px; }
  .wrap > :global(.kpis) { margin: 0; }
  .title { color: var(--text); font-weight: 600; text-decoration: none; }
  .title:hover { text-decoration: underline; }
  .rank { width: 1.6rem; color: var(--muted); font-variant-numeric: tabular-nums; }
  b { font-variant-numeric: tabular-nums; }
</style>
