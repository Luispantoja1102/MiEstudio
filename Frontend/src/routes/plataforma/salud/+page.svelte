<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { formatDate } from '$lib/format.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';

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

<PageHeader title="Salud del sistema" description="Lo que necesita estar configurado y funcionando para operar.">
  {#snippet actions()}
    <button class="btn ghost" disabled={busy} onclick={load}>{busy ? 'Revisando…' : 'Revisar de nuevo'}</button>
  {/snippet}
</PageHeader>

{#if error}<div class="gap"><InlineNotice tone="danger" message={error} /></div>{/if}

{#if d}
  <div class="gap">
    <InlineNotice tone={d.ok ? 'success' : 'warning'}>
      <strong>{d.ok ? 'Todo lo esencial funciona' : 'Hay algo que revisar'}</strong>
      <span class="when">· Revisado {formatDate(d.checked_at)}</span>
    </InlineNotice>
  </div>
  <Surface padding="flush">
    <ul class="checks">
      {#each d.checks as c (c.key)}
        <li>
          <div class="txt">
            <div class="t">{c.label}</div>
            <div class="m">{c.detail}</div>
          </div>
          <StatusBadge label={c.ok ? 'Bien' : 'Revisar'} tone={c.ok ? 'success' : 'warning'} />
        </li>
      {/each}
    </ul>
  </Surface>
{:else if !error}
  <LoadingState label="Revisando el sistema…" />
{/if}

<style>
  .gap { margin-bottom: 16px; }
  .when { font-weight: 400; opacity: 0.85; }
  .checks { list-style: none; margin: 0; padding: 0; }
  li { display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 64px; padding: 14px 20px; }
  li + li { border-top: 1px solid var(--border); }
  .txt { min-width: 0; }
  .t { font-weight: 600; }
  .m { margin-top: 2px; color: var(--muted); font-size: 0.88rem; overflow-wrap: anywhere; }
</style>
