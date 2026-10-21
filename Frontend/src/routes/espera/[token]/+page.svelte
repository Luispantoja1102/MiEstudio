<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';
  import { longDate } from '$lib/time.js';

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
    <div class="card"><p>Esta solicitud no existe. Revisa el enlace de tu correo.</p></div>
  {:else if !info}
    {#if error}<p class="error" role="alert">{error}</p>{:else}<p class="meta">Cargando…</p>{/if}
  {:else}
    <div class="card" style="display: grid; gap: 0.75rem">
      <h2>Lista de espera</h2>
      <p>{info.service} · {longDate(info.day)}</p>
      {#if info.status === 'cancelled'}
        <p class="meta" role="status">Saliste de la lista de espera. No recibirás más avisos por esta solicitud.</p>
      {:else if info.status === 'waiting' || info.status === 'notified'}
        <p class="meta">Te avisamos por correo cuando se libera un horario. Si ya no lo necesitas, sal de la lista.</p>
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        <div><button class="btn ghost" disabled={busy} onclick={leave}>{busy ? 'Saliendo…' : 'Salir de la lista de espera'}</button></div>
      {:else}
        <p class="meta">Esta solicitud ya terminó ({info.status === 'booked' ? 'reservaste tu cita' : 'el día ya pasó'}).</p>
      {/if}
    </div>
  {/if}
</PublicLayout>
