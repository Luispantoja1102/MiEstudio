<script>
  import { onMount } from 'svelte';
  import Stamps from '$lib/loyalty/Stamps.svelte';
  import { api } from '$lib/api.js';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';

  // Tarjeta de lealtad: el negocio la activa, elige cuántas visitas pide y cuál es el premio.
  let form = $state({ enabled: false, visits_required: 10, reward: '' });
  let loading = $state(true);
  let busy = $state(false);
  let error = $state('');
  let saved = $state(false);

  onMount(async () => {
    try {
      form = await api('/api/loyalty');
    } catch (e) {
      error = e.message;
    }
    loading = false;
  });

  async function save(e) {
    e.preventDefault();
    error = '';
    saved = false;
    busy = true;
    try {
      form = await api('/api/loyalty', { method: 'PUT', body: { ...form, visits_required: Number(form.visits_required) } });
      saved = true;
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
  const required = $derived(Math.min(50, Math.max(2, Number(form.visits_required) || 2)));
</script>

<PageHeader title="Tarjeta de lealtad" description="Premia a tus clientes frecuentes con una tarjeta de sellos." />

{#if loading}
  <LoadingState />
{:else}
  <div class="stack">
  <Surface>
  <form onsubmit={save}>
    <label class="switch">
      <input type="checkbox" bind:checked={form.enabled} />
      <span><b>{form.enabled ? 'Activada' : 'Desactivada'}</b><small>{form.enabled ? 'Cada visita con un servicio suma un sello al cliente.' : 'Nadie suma sellos mientras esté desactivada; lo ganado se conserva.'}</small></span>
    </label>

    <div class="form-grid">
      <label class="field">Visitas para el premio
        <input type="number" min="2" max="50" step="1" bind:value={form.visits_required} required />
        <span class="hint">Entre 2 y 50. Cuenta una visita por día, aunque el cliente haga varios servicios.</span>
      </label>
      <label class="field">Premio
        <input bind:value={form.reward} maxlength="80" placeholder="Ej. Un corte gratis" required={form.enabled} />
      </label>
    </div>

    <p class="meta">Vista previa de la tarjeta:</p>
    <div class="preview"><Stamps {required} stamps={Math.min(3, required - 1)} /></div>
    <p class="meta">Con {required} sellos el cliente gana: <b>{form.reward || '…'}</b></p>

    {#if error}<InlineNotice tone="danger" message={error} />{/if}
    {#if saved}<InlineNotice tone="success" message="Guardado." />{/if}
    <div class="form-actions"><button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button></div>
  </form>
  </Surface>

  <Surface title="Cómo funciona">
    <ul>
      <li>Al cobrar un servicio y elegir al cliente en el ticket, se le da un sello (uno por día).</li>
      <li>Al completar la tarjeta, en su ficha de <a href="/panel/clientes">Clientes</a> aparece <b>Canjear premio</b>; después aplica el descuento al cobrar.</li>
      <li>En la misma ficha creas el enlace de su <b>tarjeta virtual</b> y se lo mandas por WhatsApp para que vea sus sellos en su celular.</li>
      <li>Si cancelas una venta, su sello se quita.</li>
    </ul>
  </Surface>
  </div>
{/if}

<style>
  .stack { display: grid; gap: 16px; max-width: 680px; }
  form { display: grid; gap: 1rem; }
  .switch { display: flex; align-items: flex-start; gap: 12px; cursor: pointer; }
  .switch { min-height: 44px; }
  .switch input { margin-top: 4px; width: 20px; height: 20px; accent-color: var(--accent); }
  .switch span { display: grid; gap: 2px; }
  .switch small, .meta { color: var(--muted); }
  .meta { margin: 0; font-size: 0.9rem; }
  .meta b { color: var(--text); }
  .preview { padding: 1rem; background: var(--surface-2); border-radius: var(--radius-sm); }
  .how ul { margin: 0; padding-left: 1.2rem; display: grid; gap: 6px; font-size: 0.92rem; }
</style>
