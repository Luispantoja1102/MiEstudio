<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';
  import { longDate } from '$lib/time.js';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';

  const token = page.params.token;
  let info = $state(null);
  let missing = $state(false);
  let error = $state('');
  let busy = $state(false);

  onMount(async () => {
    try {
      const res = await fetch(`/api/public-waitlist/${token}`);
      if (res.status === 404) missing = true;
      else if (!res.ok) error = 'No pudimos cargar tu solicitud.';
      else info = await res.json();
    } catch {
      error = 'No pudimos cargar tu solicitud.';
    }
  });

  async function leave() {
    busy = true;
    error = '';
    try {
      const res = await fetch(`/api/public-waitlist/${token}/leave`, { method: 'POST' });
      if (!res.ok) throw new Error('No pudimos darte de baja. Intenta de nuevo.');
      info = { ...info, status: 'cancelled' };
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }
</script>

<PublicLayout title={info?.business ?? 'Lista de espera'} subtitle="">
  {#if missing}
    <section class="panel"><p class="lead">Esta solicitud no existe. Revisa el enlace de tu correo.</p></section>
  {:else if !info}
    {#if error}<InlineNotice tone="danger" message={error} />{:else}<LoadingState />{/if}
  {:else}
    <section class="panel">
      <div class="top">
        <h2>Lista de espera</h2>
        {#if info.status === 'cancelled'}
          <StatusBadge label="Cancelada" />
        {:else if info.status === 'notified'}
          <StatusBadge label="Te avisamos" tone="accent" />
        {:else if info.status === 'waiting'}
          <StatusBadge label="Esperando horario" tone="info" />
        {:else}
          <StatusBadge label={info.status === 'booked' ? 'Cita reservada' : 'Terminada'} tone={info.status === 'booked' ? 'success' : 'neutral'} />
        {/if}
      </div>
      <dl class="summary">
        <div><dt>Servicio</dt><dd>{info.service}</dd></div>
        <div><dt>Día</dt><dd>{longDate(info.day)}</dd></div>
      </dl>
      {#if info.status === 'cancelled'}
        <p class="note" role="status">Saliste de la lista de espera. No recibirás más avisos por esta solicitud.</p>
      {:else if info.status === 'waiting' || info.status === 'notified'}
        <p class="note">Te avisamos por correo cuando se libera un horario. Si ya no lo necesitas, sal de la lista.</p>
        {#if error}<InlineNotice tone="danger" message={error} />{/if}
        <button class="btn ghost block" disabled={busy} onclick={leave}>{busy ? 'Saliendo…' : 'Salir de la lista de espera'}</button>
      {:else}
        <p class="note">Esta solicitud ya terminó ({info.status === 'booked' ? 'reservaste tu cita' : 'el día ya pasó'}).</p>
      {/if}
    </section>
  {/if}
</PublicLayout>

<style>
  .panel { display: grid; gap: 16px; padding: 24px; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  .top { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 12px; }
  h2 { margin: 0; font-size: 1.125rem; }
  .summary { display: grid; gap: 12px; margin: 0; padding: 14px 16px; background: var(--surface-2); border-left: 3px solid var(--accent); border-radius: var(--radius-sm); }
  .summary dt { color: var(--muted); font-size: 0.8rem; }
  .summary dd { margin: 2px 0 0; font-weight: 600; overflow-wrap: anywhere; }
  .note, .lead { margin: 0; color: var(--muted); line-height: 1.5; }
  .lead { color: var(--text); }
  @media (max-width: 480px) { .panel { padding: 18px; } }
</style>
