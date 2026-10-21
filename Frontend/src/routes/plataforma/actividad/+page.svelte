<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { ACTIONS, formatDate } from '$lib/format.js';

  let rows = $state([]);
  let loading = $state(true);
  let error = $state('');

  onMount(async () => {
    try {
      rows = await api('/api/platform/activity?limit=200');
    } catch (e) {
      error = e.message;
    }
    loading = false;
  });

  const extra = (d) => {
    const parts = [];
    if (d.from && d.to) parts.push(`${d.from} → ${d.to}`);
    else if (d.status) parts.push(d.status);
    if (d.email) parts.push(d.email);
    if (d.registration_mode) parts.push(`registro: ${d.registration_mode}`);
    if (d.trial_ends_at) parts.push(`prueba hasta ${d.trial_ends_at}`);
    return parts.join(' · ');
  };
</script>

<div class="page-head">
  <div>
    <h1>Actividad</h1>
    <p>Registro de lo que ocurre en la plataforma.</p>
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else if rows.length === 0}
  <div class="empty">Todavía no hay actividad.</div>
{:else}
  <div class="list">
    {#each rows as r (r.id)}
      <div class="item-row">
        <div class="grow">
          <div class="title">{ACTIONS[r.action] ?? r.action}{#if r.business}{' · '}<a href="/plataforma/negocios/{r.business_id}">{r.business}</a>{/if}</div>
          {#if extra(r.detail)}<div class="meta">{extra(r.detail)}</div>{/if}
        </div>
        <span class="meta">{r.actor} · {formatDate(r.created_at)}</span>
      </div>
    {/each}
  </div>
{/if}
