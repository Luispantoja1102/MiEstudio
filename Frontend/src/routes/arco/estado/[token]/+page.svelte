<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';
  import { formatDay } from '$lib/format.js';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';

  let d = $state(null);
  let missing = $state(false);
  onMount(async () => {
    const r = await fetch(`/api/public-arco/${page.params.token}`);
    if (r.status === 404) missing = true;
    else d = await r.json();
  });
  const TONE = { received: 'warning', in_progress: 'warning', resolved: 'success', rejected: 'neutral' };
</script>

<PublicLayout title="Estado de tu solicitud">
  {#if missing}
    <div class="card"><p class="none">No encontramos esta solicitud. Revisa que el enlace esté completo.</p></div>
  {:else if d}
    <div class="card doc">
      <StatusBadge label={d.status_label} tone={TONE[d.status] ?? 'neutral'} />
      <h2>{d.kind} · {d.business}</h2>
      <p>Recibida el {formatDay(d.created_at)}. Fecha límite de respuesta: <b>{formatDay(d.due_at)}</b>.</p>
      {#if d.resolution}
        <div class="resp"><b>Respuesta del negocio</b><p>{d.resolution}</p></div>
      {:else}
        <p class="meta">Aún no hay respuesta. Esta página se actualiza cuando el negocio la registre.</p>
      {/if}
    </div>
  {:else}
    <LoadingState />
  {/if}
</PublicLayout>

<style>
  .doc { padding: 24px; display: grid; gap: 0.75rem; justify-items: start; }
  .doc h2, .doc p { margin: 0; overflow-wrap: anywhere; }
  .doc h2 { margin-top: 0.25rem; }
  .none { margin: 0; }
  .resp { width: 100%; box-sizing: border-box; margin-top: 0.5rem; padding: 14px 16px; background: var(--surface-2); border-left: 3px solid var(--accent); border-radius: var(--radius-sm); }
  .resp p { margin-top: 0.4rem; white-space: pre-line; }
  .meta { color: var(--muted); font-size: 0.9rem; }
  @media (max-width: 480px) { .doc { padding: 18px 16px; } }
</style>
