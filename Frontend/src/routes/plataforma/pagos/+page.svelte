<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { PAY_METHODS, PLANS, formatDate, formatDay, money, plural } from '$lib/format.js';

  let rows = $state([]);
  let error = $state('');
  let loading = $state(true);
  onMount(async () => {
    try {
      rows = await api('/api/platform/payments');
    } catch (e) {
      error = e.message;
    }
    loading = false;
  });
  const total = $derived(rows.filter((p) => p.method !== 'courtesy').reduce((n, p) => n + p.amount_cents, 0));
</script>

<div class="page-head">
  <div>
    <h1>Pagos</h1>
    <p>Los últimos pagos de suscripción de todos los negocios.</p>
  </div>
  {#if rows.length}<span class="badge">{money(total)} en esta lista</span>{/if}
</div>
{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else if rows.length === 0}
  <p class="empty">Aún no hay pagos.</p>
{:else}
  <div class="list">
    {#each rows as p (p.id)}
      <a class="item-row link" href="/plataforma/negocios/{p.business_id}">
        <div class="grow">
          <div class="title">{p.business}</div>
          <div class="meta">Plan {PLANS[p.plan] ?? p.plan} · {plural(p.months, 'mes', 'meses')} · hasta {formatDay(p.period_end)} · {p.created_by}{p.note ? ' · ' + p.note : ''}</div>
        </div>
        <span class="meta">{PAY_METHODS[p.method]} · {formatDate(p.created_at)}</span>
        <b>{money(p.amount_cents)}</b>
      </a>
    {/each}
  </div>
{/if}

<style>
  .link { color: var(--text); text-decoration: none; }
  .link:hover { border-color: var(--accent); }
  .meta { color: var(--muted); font-size: 0.88rem; }
</style>
