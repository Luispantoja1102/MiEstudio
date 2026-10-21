<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';

  const token = page.params.token;
  let info = $state(null);
  let missing = $state(false);
  let error = $state('');
  let busy = $state(false);
  let done = $state(false);

  onMount(async () => {
    try {
      const res = await fetch(`/api/public-unsub/${token}`);
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
      const res = await fetch(`/api/public-unsub/${token}`, { method: 'POST' });
      if (!res.ok) throw new Error('No pudimos darte de baja. Intenta de nuevo.');
      done = true;
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }
</script>

<PublicLayout title={info?.business ?? 'Darme de baja'} subtitle="">
  {#if missing}
    <div class="card"><p>Este enlace no es válido. Si sigues recibiendo mensajes que no quieres, escríbele al negocio.</p></div>
  {:else if !info}
    {#if error}<p class="error" role="alert">{error}</p>{:else}<p class="meta">Cargando…</p>{/if}
  {:else if done || !info.subscribed}
    <div class="card" role="status"><h2>Listo</h2><p>No recibirás más recordatorios, promociones ni encuestas de {info.business}. Los correos de tus citas (confirmaciones y cambios) seguirán llegando.</p></div>
  {:else}
    <div class="card" style="display: grid; gap: 0.75rem">
      <h2>Dejar de recibir mensajes</h2>
      <p class="meta">Ya no recibirás recordatorios, promociones ni encuestas de {info.business}. Los correos de tus citas seguirán llegando.</p>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <div><button class="btn" disabled={busy} onclick={leave}>{busy ? 'Un momento…' : 'Darme de baja'}</button></div>
    </div>
  {/if}
</PublicLayout>
