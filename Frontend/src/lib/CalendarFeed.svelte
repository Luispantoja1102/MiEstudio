<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';

  // Calendario privado: tus citas aparecen en Google Calendar (u otro calendario) al suscribirte a esta dirección.
  // Es de un solo sentido: Barbella publica tus citas y nunca lee tu calendario.
  let feed = $state({ enabled: false, show_names: false, path: '' });
  let error = $state('');
  let busy = $state(false);
  let copied = $state(false);

  const url = $derived(feed.path ? location.origin + feed.path : '');
  const local = typeof location !== 'undefined' && ['localhost', '127.0.0.1'].includes(location.hostname);

  onMount(() => api('/api/me/calendar').then((f) => (feed = f)).catch((e) => (error = e.message)));

  async function send(action, show_names = feed.show_names) {
    error = '';
    busy = true;
    try {
      feed = await api('/api/me/calendar', { method: 'POST', body: { action, show_names } });
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }
  async function rotate() {
    if (confirm('Se cambiará la dirección: la que ya agregaste a tu calendario dejará de funcionar y tendrás que agregar la nueva. ¿Continuar?')) await send('rotate');
  }
  async function copy() {
    try {
      await navigator.clipboard.writeText(url);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      error = 'No se pudo copiar. Selecciona la dirección y cópiala a mano.';
    }
  }
</script>

<section class="card sec">
  <h2>Calendario de citas {#if feed.enabled}<span class="badge">Activo</span>{:else}<span class="badge muted">Apagado</span>{/if}</h2>
  <p class="meta">Mira tus citas en Google Calendar, junto con el resto de tu día. Solo salen tus citas; Barbella nunca lee tu calendario.</p>

  {#if error}<p class="error" role="alert">{error}</p>{/if}

  {#if !feed.enabled}
    <button class="btn" disabled={busy} onclick={() => send('enable', false)}>{busy ? 'Activando…' : 'Activar mi calendario'}</button>
  {:else}
    <label class="field">Dirección de tu calendario
      <span class="row">
        <input readonly value={url} onfocus={(e) => e.currentTarget.select()} aria-label="Dirección de tu calendario" />
        <button class="btn ghost" onclick={copy}>{copied ? 'Copiada ✓' : 'Copiar'}</button>
      </span>
    </label>
    <ol class="steps">
      <li>Copia la dirección.</li>
      <li>En Google Calendar abre <a href="https://calendar.google.com/calendar/u/0/r/settings/addbyurl" target="_blank" rel="noopener">Otros calendarios → Desde URL</a> y pégala.</li>
      <li>Tus citas aparecen en unos minutos y se actualizan solas (Google tarda hasta unas horas en refrescar).</li>
    </ol>
    {#if local}<p class="meta">Estás en un servidor local: Google Calendar solo puede leer la dirección cuando Barbella está publicado en internet.</p>{/if}
    <label class="check">
      <input type="checkbox" checked={feed.show_names} disabled={busy} onchange={(e) => send('enable', e.currentTarget.checked)} />
      Mostrar el nombre y teléfono de mis clientes en los eventos
    </label>
    <p class="meta">Apagado, cada evento dice solo «Cita · servicio». Prende esta opción solo si tu calendario es privado: la dirección la puede leer cualquiera que la tenga.</p>
    <div class="form-actions">
      <button class="btn ghost" disabled={busy} onclick={rotate}>Cambiar dirección</button>
      <button class="btn ghost" disabled={busy} onclick={() => send('disable')}>Apagar calendario</button>
    </div>
  {/if}
</section>

<style>
  .sec { margin-bottom: 1rem; }
  h2 { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
  .row { display: flex; gap: 8px; align-items: center; }
  .row input { flex: 1; min-width: 0; margin: 0; font-size: 0.85rem; }
  .steps { margin: 0.5rem 0 1rem; padding-left: 1.2rem; display: grid; gap: 4px; font-size: 0.92rem; }
  .check { display: flex; align-items: center; gap: 10px; margin: 0.75rem 0 0.25rem; font-size: 0.92rem; }
</style>
