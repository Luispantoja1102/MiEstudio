<script>
  import { onMount } from 'svelte';
  import Avatar from '$lib/Avatar.svelte';
  import { api } from '$lib/api.js';
  import { ACTIONS, BILLING, KINDS, PLANS, ago, money, plural } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';

  const MODES = { open: 'Abierto', approval: 'Con aprobación', closed: 'Cerrado' };
  const KIND_TONE = { pending: 'warn', past_due: 'bad', trial_expired: 'bad', trial_ending: 'warn' };
  const LIMIT = 6;

  let d = $state(null);
  let activity = $state([]);
  let error = $state('');
  let showAll = $state(false);

  onMount(async () => {
    try {
      d = await api('/api/platform/overview');
      activity = await api('/api/platform/activity?limit=7').catch(() => []);
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

<div class="page-head">
  <div>
    <h1>Hoy</h1>
    <p>{today}</p>
  </div>
  {#if d}<a class="badge" href="/plataforma/ajustes?s=registro">Registro: {MODES[d.registration_mode]}</a>{/if}
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if d}
  <p class="hello">{hello}.{' '}{#if d.attention.length}{plural(d.attention.length, 'cosa necesita', 'cosas necesitan')} tu atención.{:else}Todo está en orden.{/if}</p>

  <section class="kpis" aria-label="Resumen de atención">
    <a class="kpi" class:alert={waiting('pending')} href="/plataforma/negocios?state=pending">
      <span class="lbl">Por aprobar</span>
      <span class="body"><b class="v">{waiting('pending')}</b><span class="sub">{waiting('pending') ? 'negocios esperando tu visto bueno' : 'nadie en espera'}</span></span>
    </a>
    <a class="kpi" class:alert={d.counts.past_due} href="/plataforma/negocios?state=past_due">
      <span class="lbl">Pago atrasado</span>
      <span class="body"><b class="v">{d.counts.past_due}</b><span class="sub">{d.counts.past_due ? 'negocios por contactar' : 'todos al corriente'}</span></span>
    </a>
    <a class="kpi" class:alert={d.counts.trial_expired + waiting('trial_ending')} href="/plataforma/negocios?state=trial_expired">
      <span class="lbl">Pruebas por cerrar</span>
      <span class="body"><b class="v">{d.counts.trial_expired + waiting('trial_ending')}</b><span class="sub">vencen pronto o ya vencieron</span></span>
    </a>
    <a class="kpi" href="/plataforma/negocios?state=active">
      <span class="lbl">Negocios activos</span>
      <span class="body"><b class="v">{d.counts.active}</b><span class="sub">{d.counts.total} en total · {d.counts.trialing} en prueba</span></span>
    </a>
  </section>

  {#if d.mrr_cents !== undefined || d.usage_30d}
    <section class="kpis" aria-label="Cifras del mes">
      {#if d.mrr_cents !== undefined}
        <div class="kpi">
          <span class="lbl">Ingreso mensual recurrente</span>
          <span class="body"><b class="v">{money(d.mrr_cents)}</b><span class="sub">lo que pagan al mes los activos</span></span>
        </div>
        <a class="kpi" href="/plataforma/pagos">
          <span class="lbl">Cobrado este mes</span>
          <span class="body"><b class="v">{money(d.paid_this_month_cents)}</b><span class="sub">ver pagos</span></span>
        </a>
      {/if}
      <a class="kpi" href="/plataforma/negocios">
        <span class="lbl">Altas (30 días)</span>
        <span class="body"><b class="v">{d.signups_30d}</b><span class="sub">negocios nuevos</span></span>
      </a>
      {#if d.usage_30d}
        <div class="kpi">
          <span class="lbl">Negocios con actividad</span>
          <span class="body"><b class="v">{d.usage_30d.active_businesses}</b><span class="sub">de {d.counts.total} · {d.usage_30d.appointments} citas en 30 días</span></span>
        </div>
      {/if}
    </section>
  {/if}

  <div class="cols">
    <section class="card" aria-labelledby="cola">
      <h2 id="cola">Necesita tu atención</h2>
      <p class="meta">De lo más urgente a lo menos; lo que lleva más tiempo esperando va primero.</p>
      {#if d.attention.length === 0}
        <p class="allclear">✓ Todo en orden: no hay negocios por aprobar, pagos atrasados ni pruebas por vencer.</p>
      {:else}
        <ul class="queue">
          {#each shown as x (x.kind + x.business_id)}
            <li>
              <Avatar name={x.name} />
              <div class="who">
                <b>{x.name}</b>
                <span class="meta">{x.detail}</span>
              </div>
              <span class="badge {KIND_TONE[x.kind]}">{x.title}</span>
              {#if x.at}<span class="when">{ago(x.at)}</span>{/if}
              <a class="btn sm ghost" href="/plataforma/negocios/{x.business_id}">Abrir</a>
            </li>
          {/each}
        </ul>
        {#if d.attention.length > LIMIT}
          <button class="btn ghost sm" onclick={() => (showAll = !showAll)}>{showAll ? 'Ver menos' : `Ver los ${d.attention.length}`}</button>
        {/if}
      {/if}
    </section>

    <div class="stack">
      <section class="card" aria-labelledby="nuevos">
        <div class="head"><h2 id="nuevos">Últimos registros</h2><a class="btn ghost sm" href="/plataforma/negocios">Todos</a></div>
        {#if d.recent.length === 0}
          <p class="meta">Todavía no hay negocios.</p>
        {:else}
          <ul class="recent">
            {#each d.recent as b (b.id)}
              {@const [label, tone] = BILLING[b.billing_state]}
              <li>
                <a href="/plataforma/negocios/{b.id}">
                  <Avatar name={b.name} size="2.2rem" />
                  <span class="who"><b>{b.name}</b><span class="meta">{KINDS[b.kind]} · {PLANS[b.plan]} · {ago(b.created_at)}</span></span>
                  <span class="badge {tone}">{label}</span>
                </a>
              </li>
            {/each}
          </ul>
        {/if}
      </section>

      <section class="card" aria-labelledby="mov">
        <div class="head"><h2 id="mov">Movimientos del equipo</h2><a class="btn ghost sm" href="/plataforma/ajustes?s=actividad">Ver todo</a></div>
        {#if activity.length === 0}
          <p class="meta">Aún no hay movimientos.</p>
        {:else}
          <ul class="timeline">
            {#each activity as a (a.id)}
              <li>
                <i aria-hidden="true"></i>
                <div>
                  <span>{ACTIONS[a.action] ?? a.action}</span>
                  <small class="meta">{a.actor || 'Sistema'}{#if a.business} · <a href="/plataforma/negocios/{a.business_id}">{a.business}</a>{/if}</small>
                </div>
                <time class="when" datetime={a.created_at}>{ago(a.created_at)}</time>
              </li>
            {/each}
          </ul>
        {/if}
      </section>
    </div>
  </div>
{/if}

<style>
  .hello { margin: -0.5rem 0 1.25rem; color: var(--muted); }
  a.badge { text-decoration: none; }
  .cols { display: grid; gap: 20px; grid-template-columns: minmax(0, 1fr); align-items: start; }
  @media (min-width: 1000px) { .cols { grid-template-columns: minmax(0, 1.25fr) minmax(0, 1fr); } }
  .stack { display: grid; gap: 20px; }
  h2 { font-size: 1.2rem; margin-bottom: 0.25rem; }
  .head { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 0.6rem; }
  .head h2 { margin: 0; }
  ul { list-style: none; margin: 0.75rem 0 0; padding: 0; display: grid; gap: 8px; }
  .meta { color: var(--muted); font-size: 0.85rem; }
  .who { display: grid; min-width: 0; line-height: 1.3; }
  .who b { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .when { color: var(--muted); font-size: 0.8rem; white-space: nowrap; }

  .queue li { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; padding: 0.7rem 0.8rem; border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .queue .who { flex: 1 1 160px; }
  .allclear { margin: 0.75rem 0 0; padding: 0.9rem 1rem; border-radius: var(--radius-sm); background: var(--ok-soft, var(--accent-soft)); color: var(--ok, var(--accent)); font-weight: 600; }

  .recent a { display: flex; align-items: center; gap: 10px; padding: 0.5rem 0.2rem; color: var(--text); text-decoration: none; border-radius: var(--radius-sm); }
  .recent a:hover { background: var(--surface-2); }
  .recent .who { flex: 1; }

  .timeline li { display: grid; grid-template-columns: 10px minmax(0, 1fr) auto; gap: 10px; align-items: start; font-size: 0.92rem; }
  .timeline i { width: 8px; height: 8px; margin-top: 0.45rem; border-radius: 50%; background: var(--accent); }
  .timeline small { display: block; }
  .timeline a { color: inherit; }
</style>
