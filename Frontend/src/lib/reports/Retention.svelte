<script>
  import { api } from '$lib/api.js';
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

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if r}
  <div class:dim={loading}>
    {#if r.total_clients === 0}
      <div class="empty">Todavía no hay clientes con ventas. Al cobrar, elige al cliente en el ticket para que aparezca aquí su historial.</div>
    {:else}
      <div class="kpis">
        <div class="card"><span class="label">Clientes activos</span><span class="value">{r.active}</span><small>vinieron en el periodo</small></div>
        <div class="card"><span class="label">Nuevos</span><span class="value">{r.new}</span><small>su primera visita fue ahora</small></div>
        <div class="card"><span class="label">Regresaron</span><span class="value">{r.returning}</span><small>ya habían venido antes</small></div>
        <div class="card">
          <span class="label">Retención</span><span class="value">{pctText(r.retention_pct)}</span>
          <small>{r.prev_active ? `${r.retained} de ${r.prev_active} clientes del periodo anterior volvieron` : 'Sin clientes en el periodo anterior'}</small>
        </div>
        <div class="card"><span class="label">Visitas por cliente</span><span class="value">{r.visits_per_client == null ? '—' : r.visits_per_client.toFixed(1)}</span><small>{r.repeaters} vinieron más de una vez</small></div>
        <div class="card"><span class="label">Vuelven cada</span><span class="value">{r.avg_gap_days == null ? '—' : `${Math.round(r.avg_gap_days)} días`}</span><small>de quienes han venido más de una vez</small></div>
      </div>
      <p class="meta" style="margin-top: 1rem">{r.one_visit} de {r.total_clients} clientes han venido una sola vez. Cada visita es un día en que se les cobró algo.</p>

      <section>
        <h2>Clientes en riesgo</h2>
        <p class="meta">Han venido al menos dos veces y llevan más de {r.lapsed_days} días sin volver. Escríbeles antes de perderlos; en <a href="/panel/mensajes">Mensajes</a> puedes activar el aviso automático de reactivación.</p>
        <div class="list">
          {#each r.lapsed as c (c.client_id)}
            <div class="item-row">
              <span class="grow"><a class="title" href={`/panel/clientes/${c.client_id}`}>{c.name}</a><small class="meta">{' · '}{c.visits} visitas · última {ago(c.days_ago)}</small></span>
              <b>{money(c.spent_cents)}</b>
              {#if wa(c)}<a class="btn ghost sm" href={wa(c)} target="_blank" rel="noopener">WhatsApp</a>{/if}
            </div>
          {:else}
            <div class="empty">Nadie en riesgo por ahora.</div>
          {/each}
        </div>
      </section>

      <section>
        <h2>Mejores clientes del periodo</h2>
        <div class="list">
          {#each r.top as c, i (c.client_id)}
            <div class="item-row"><span class="rank">{i + 1}</span><a class="grow title" href={`/panel/clientes/${c.client_id}`}>{c.name}</a><b>{money(c.spent_cents)}</b></div>
          {:else}
            <div class="empty">Sin ventas a clientes en este periodo.</div>
          {/each}
        </div>
      </section>
    {/if}
  </div>
{/if}

<style>
  .dim { opacity: 0.6; transition: opacity 0.2s; }
  .kpis { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); }
  .kpis .card { display: grid; gap: 2px; }
  .label { color: var(--muted); font-size: 0.9rem; }
  .value { font-weight: 700; font-variant-numeric: tabular-nums lining-nums; font-size: 1.9rem; line-height: 1.15; }
  small { color: var(--muted); }
  section { margin-top: 2rem; }
  section h2 { font-size: 1.35rem; }
  .title { color: var(--text); font-weight: 600; text-decoration: none; }
  .title:hover { text-decoration: underline; }
  .rank { width: 1.6rem; color: var(--muted); font-variant-numeric: tabular-nums; }
  b { font-variant-numeric: tabular-nums; }
</style>
