<script>
  import { onMount } from 'svelte';
  import Stamps from './Stamps.svelte';
  import { api } from '$lib/api.js';
  import { session } from '$lib/session.svelte.js';

  // La tarjeta de lealtad de un cliente, vista por el negocio: sellos, canje del premio y su enlace de tarjeta virtual.
  let { clientId, name = '', phone = '' } = $props();

  let card = $state(null);
  let error = $state('');
  let busy = $state(false);
  let copied = $state(false);

  onMount(() => api(`/api/clients/${clientId}/loyalty`).then((c) => (card = c)).catch((e) => (error = e.message)));

  const url = $derived(card?.path ? location.origin + card.path : '');
  const first = $derived(name.trim().split(/\s+/)[0] || '');
  const wa = $derived.by(() => {
    const digits = phone.replace(/\D/g, '');
    if (!digits || !url) return '';
    const msg = `Hola ${first}, esta es tu tarjeta de lealtad de ${session.me.business.name}: ${url}`;
    return `https://wa.me/${digits.length === 10 ? '52' + digits : digits}?text=${encodeURIComponent(msg)}`;
  });
  const missing = $derived(card ? Math.max(card.visits_required - card.stamps, 0) : 0);

  async function run(fn) {
    error = '';
    busy = true;
    try {
      card = await fn();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }
  const redeem = () => confirm(`¿Canjear el premio «${card.reward}»? Se descuentan ${card.visits_required} sellos.`) && run(() => api(`/api/clients/${clientId}/loyalty/redeem`, { method: 'POST' }));
  const link = (action) => run(() => api(`/api/clients/${clientId}/loyalty/link`, { method: 'POST', body: { action } }));
  const rotate = () => confirm('Se cambiará el enlace: el que ya compartiste dejará de funcionar. ¿Continuar?') && link('rotate');
  async function copy() {
    try {
      await navigator.clipboard.writeText(url);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      error = 'No se pudo copiar. Selecciona el enlace y cópialo a mano.';
    }
  }
  const day = (d) => new Date(d + 'T12:00:00').toLocaleDateString('es-MX', { day: 'numeric', month: 'short' });
</script>

{#if card?.enabled}
  <section class="card loy">
    <h2>Tarjeta de lealtad {#if card.ready}<span class="badge">Premio listo</span>{/if}</h2>
    {#if error}<p class="error" role="alert">{error}</p>{/if}

    <Stamps required={card.visits_required} stamps={card.stamps} ready={card.ready} />
    <p class="meta">
      {#if card.ready}
        Completó su tarjeta. Premio: <b>{card.reward}</b>.
      {:else}
        {card.stamps} de {card.visits_required} sellos · le {missing === 1 ? 'falta 1 visita' : `faltan ${missing} visitas`} para: {card.reward}.
      {/if}
      {#if card.redeemed}Premios canjeados: {card.redeemed}.{/if}
    </p>
    {#if card.ready}<button class="btn" disabled={busy} onclick={redeem}>Canjear premio</button>{/if}
    {#if card.history.length}
      <p class="meta hist">Últimas visitas con sello: {card.history.map((v) => day(v.day) + (v.redeemed ? ' ✓' : '')).join(' · ')}</p>
    {/if}

    <div class="linkbox">
      <b>Tarjeta virtual</b>
      {#if card.path}
        <p class="meta">Con este enlace el cliente ve sus sellos desde su celular. Solo muestra su primer nombre.</p>
        <span class="row">
          <input readonly value={url} onfocus={(e) => e.currentTarget.select()} aria-label="Enlace de la tarjeta" />
          <button class="btn ghost sm" onclick={copy}>{copied ? 'Copiado ✓' : 'Copiar'}</button>
          {#if wa}<a class="btn ghost sm" href={wa} target="_blank" rel="noopener">Enviar por WhatsApp</a>{/if}
        </span>
        <span class="row small">
          <button class="link" disabled={busy} onclick={rotate}>Cambiar enlace</button>
          <button class="link" disabled={busy} onclick={() => link('remove')}>Quitar enlace</button>
        </span>
      {:else}
        <p class="meta">Crea un enlace privado para que el cliente vea su tarjeta en su celular.</p>
        <button class="btn ghost sm" disabled={busy} onclick={() => link('create')}>Crear enlace de la tarjeta</button>
      {/if}
    </div>
  </section>
{/if}

<style>
  .loy { margin-top: 1.25rem; display: grid; gap: 0.75rem; }
  h2 { font-size: 1.15rem; display: flex; align-items: center; gap: 10px; margin: 0; }
  .meta { margin: 0; color: var(--muted); font-size: 0.9rem; }
  .meta b { color: var(--text); }
  .hist { font-size: 0.82rem; }
  .linkbox { display: grid; gap: 6px; padding-top: 0.75rem; border-top: 1px solid var(--border); }
  .row { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  .row input { flex: 1 1 220px; min-width: 0; margin: 0; font-size: 0.85rem; }
  .small { gap: 16px; }
  .link { padding: 0; font: inherit; font-size: 0.85rem; color: var(--accent); background: none; border: 0; text-decoration: underline; cursor: pointer; }
</style>
