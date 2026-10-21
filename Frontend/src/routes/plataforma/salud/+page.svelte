<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { formatDate } from '$lib/format.js';

  let d = $state(null);
  let error = $state('');
  let busy = $state(false);
  async function load() {
    busy = true;
    try {
      d = await api('/api/platform/health');
      error = '';
    } catch (e) {
      error = e.message;
    }
    busy = false;
  }
  onMount(load);
</script>

<div class="page-head">
  <div>
    <h1>Salud del sistema</h1>
    <p>Lo que necesita estar configurado y funcionando para operar.</p>
  </div>
  <button class="btn ghost" disabled={busy} onclick={load}>{busy ? 'Revisando…' : 'Revisar de nuevo'}</button>
</div>
{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if d}
  <p class="status" class:ok={d.ok} role="status">{d.ok ? '✓ Todo lo esencial funciona' : '⚠ Hay algo que revisar'} <small>· {formatDate(d.checked_at)}</small></p>
  <div class="list">
    {#each d.checks as c (c.key)}
      <div class="item-row">
        <span class="dot" class:ok={c.ok} aria-hidden="true"></span>
        <div class="grow">
          <div class="title">{c.label}</div>
          <div class="meta">{c.detail}</div>
        </div>
        <span class="badge {c.ok ? '' : 'warn'}">{c.ok ? 'Bien' : 'Revisar'}</span>
      </div>
    {/each}
  </div>
{/if}

<style>
  .status { font-weight: 600; color: var(--warn); }
  .status.ok { color: var(--accent); }
  .status small { font-weight: 400; color: var(--muted); }
  .dot { width: 12px; height: 12px; border-radius: 50%; background: var(--warn); flex: none; }
  .dot.ok { background: var(--accent); }
  .meta { color: var(--muted); font-size: 0.88rem; }
</style>
