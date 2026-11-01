<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import Stamps from '$lib/loyalty/Stamps.svelte';

  // La tarjeta de lealtad de un cliente, sin iniciar sesión: se abre con el enlace privado que le da el negocio.
  const token = page.params.token;
  let card = $state(null);
  let missing = $state(false);
  let error = $state('');

  onMount(async () => {
    try {
      const res = await fetch(`/api/public-card/${token}`);
      if (res.status === 404) missing = true;
      else if (!res.ok) error = 'No pudimos cargar tu tarjeta. Intenta de nuevo en un momento.';
      else card = await res.json();
    } catch {
      error = 'No pudimos cargar tu tarjeta. Revisa tu conexión.';
    }
  });

  const left = $derived(card ? Math.max(card.visits_required - card.stamps, 0) : 0);
  const day = (d) => new Date(d + 'T12:00:00').toLocaleDateString('es-MX', { weekday: 'short', day: 'numeric', month: 'short' });
</script>

<svelte:head><meta name="robots" content="noindex" /></svelte:head>

<PublicLayout title={card?.business ?? 'Tarjeta de lealtad'} subtitle={card ? `Tarjeta de lealtad de ${card.name}` : ''} brand={!!card}>
  {#if missing}
    <Surface title="Esta tarjeta no está disponible"><p class="meta">El enlace cambió o la tarjeta de lealtad ya no está activa. Pídele al negocio un enlace nuevo.</p></Surface>
  {:else if error}
    <InlineNotice tone="danger" message={error} />
  {:else if card}
    <section class="pass" class:ready={card.ready}><Surface>
      <p class="count"><b>{Math.min(card.stamps, card.visits_required)}</b> de {card.visits_required} sellos</p>
      <Stamps required={card.visits_required} stamps={card.stamps} ready={card.ready} />
      {#if card.ready}
        <p class="win">¡Completaste tu tarjeta! Tu premio: <b>{card.reward}</b>. Avísale al negocio en tu próxima visita para canjearlo.</p>
      {:else}
        <p class="meta">{left === 1 ? 'Te falta 1 visita' : `Te faltan ${left} visitas`} para tu premio: <b>{card.reward}</b>.</p>
      {/if}
    </Surface></section>

    {#if card.history.length}
      <section>
        <h2>Tus últimas visitas</h2>
        <ul class="hist">
          {#each card.history as v}<li><span>{day(v.day)}</span>{#if v.redeemed}<small>Canjeado</small>{/if}</li>{/each}
        </ul>
      </section>
    {/if}
    {#if card.redeemed}<p class="meta">Premios que ya canjeaste: {card.redeemed}.</p>{/if}
    <p class="meta fine">Cada visita con un servicio suma un sello. Este enlace es solo tuyo: no lo compartas.</p>
  {:else}
    <LoadingState label="Cargando tu tarjeta…" />
  {/if}
</PublicLayout>

<style>
  .pass :global(.surface) { display: grid; gap: 1rem; }
  .pass.ready :global(.surface) { border-color: var(--accent); background: var(--accent-soft); }
  .count { margin: 0; font-size: 1.1rem; }
  .count b { font-family: var(--font-num, inherit); font-size: 2.4rem; font-variant-numeric: tabular-nums; }
  .win { margin: 0; font-size: 1.05rem; }
  .meta { margin: 0; color: var(--muted); }
  .meta b { color: var(--text); }
  .fine { font-size: 0.82rem; margin-top: 1.5rem; }
  section h2 { font-size: 1.125rem; margin: 1.75rem 0 0.5rem; }
  .hist { list-style: none; margin: 0; padding: 0; display: grid; gap: 6px; }
  .hist li { display: flex; justify-content: space-between; min-height: 44px; align-items: center; padding: 0.55rem 0.8rem; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .hist small { color: var(--ok); font-weight: 600; }
</style>
