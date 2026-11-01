<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';

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
    <section class="panel"><p class="lead">Este enlace no es válido. Si sigues recibiendo mensajes que no quieres, escríbele al negocio.</p></section>
  {:else if !info}
    {#if error}<InlineNotice tone="danger" message={error} />{:else}<LoadingState />{/if}
  {:else if done || !info.subscribed}
    <section class="panel ok" role="status">
      <h2>Listo</h2>
      <p class="note">No recibirás más recordatorios, promociones ni encuestas de {info.business}. Los correos de tus citas (confirmaciones y cambios) seguirán llegando.</p>
    </section>
  {:else}
    <section class="panel">
      <h2>Dejar de recibir mensajes</h2>
      <p class="note">Ya no recibirás recordatorios, promociones ni encuestas de {info.business}. Los correos de tus citas seguirán llegando.</p>
      {#if error}<InlineNotice tone="danger" message={error} />{/if}
      <button class="btn block" disabled={busy} onclick={leave}>{busy ? 'Un momento…' : 'Darme de baja'}</button>
    </section>
  {/if}
</PublicLayout>

<style>
  .panel { display: grid; gap: 16px; padding: 24px; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  .ok { border-left: 3px solid var(--success); }
  h2 { margin: 0; font-size: 1.125rem; }
  .note, .lead { margin: 0; color: var(--muted); line-height: 1.5; }
  .lead { color: var(--text); }
  @media (max-width: 480px) { .panel { padding: 18px; } }
</style>
