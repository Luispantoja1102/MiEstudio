<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';
  import { formatDay } from '$lib/format.js';

  let d = $state(null);
  let missing = $state(false);
  onMount(async () => {
    const r = await fetch(`/api/public-arco/${page.params.token}`);
    if (r.status === 404) missing = true;
    else d = await r.json();
  });
  const TONE = { received: 'warn', in_progress: 'warn', resolved: '', rejected: 'muted' };
</script>

<PublicLayout title="Estado de tu solicitud">
  {#if missing}
    <div class="card"><p>No encontramos esta solicitud. Revisa que el enlace esté completo.</p></div>
  {:else if d}
    <div class="card">
      <span class="badge {TONE[d.status]}">{d.status_label}</span>
      <h2 style="margin-top: 0.6rem">{d.kind} · {d.business}</h2>
      <p>Recibida el {formatDay(d.created_at)}. Fecha límite de respuesta: <b>{formatDay(d.due_at)}</b>.</p>
      {#if d.resolution}
        <div class="resp"><b>Respuesta del negocio</b><p>{d.resolution}</p></div>
      {:else}
        <p class="meta">Aún no hay respuesta. Esta página se actualiza cuando el negocio la registre.</p>
      {/if}
    </div>
  {/if}
</PublicLayout>

<style>
  .resp { background: var(--surface-2); padding: 1rem; border-radius: var(--radius-sm); }
  .meta { color: var(--muted); font-size: 0.9rem; }
</style>
