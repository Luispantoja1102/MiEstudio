<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { KINDS } from '$lib/format.js';
  import { loadSession } from '$lib/session.svelte.js';

  // Datos del negocio: nombre, giro y zona horaria. La dirección pública no se cambia aquí (rompería los enlaces compartidos).
  const ZONES = [
    ['America/Tijuana', 'Tijuana, Baja California (Pacífico)'],
    ['America/Hermosillo', 'Sonora (sin horario de verano)'],
    ['America/Mazatlan', 'Sinaloa, Nayarit, BCS (Montaña)'],
    ['America/Chihuahua', 'Chihuahua'],
    ['America/Ciudad_Juarez', 'Ciudad Juárez'],
    ['America/Mexico_City', 'Ciudad de México y centro del país'],
    ['America/Monterrey', 'Monterrey'],
    ['America/Merida', 'Yucatán'],
    ['America/Cancun', 'Quintana Roo (Cancún)'],
    ['America/Los_Angeles', 'Los Ángeles (EE. UU., Pacífico)'],
    ['America/Phoenix', 'Phoenix (EE. UU.)'],
    ['America/Denver', 'Denver (EE. UU., Montaña)'],
    ['America/Chicago', 'Chicago (EE. UU., Centro)'],
    ['America/New_York', 'Nueva York (EE. UU., Este)'],
    ['America/Bogota', 'Bogotá'],
    ['America/Lima', 'Lima'],
    ['America/Santiago', 'Santiago de Chile'],
    ['America/Argentina/Buenos_Aires', 'Buenos Aires'],
    ['Europe/Madrid', 'Madrid']
  ];

  let form = $state(null);
  let slug = $state('');
  let startTz = '';
  let error = $state('');
  let saved = $state(false);
  let busy = $state(false);

  onMount(async () => {
    try {
      const b = await api('/api/business');
      slug = b.slug;
      startTz = b.timezone;
      form = { name: b.name, kind: b.kind, timezone: b.timezone };
    } catch (e) {
      error = e.message;
    }
  });

  const zones = $derived(form && !ZONES.some(([z]) => z === form.timezone) ? [[form.timezone, form.timezone], ...ZONES] : ZONES);

  async function save(e) {
    e.preventDefault();
    error = '';
    saved = false;
    busy = true;
    try {
      await api('/api/business', { method: 'PUT', body: form });
      startTz = form.timezone;
      saved = true;
      await loadSession(); // el nombre del negocio se muestra arriba
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if form}
  <form class="card" onsubmit={save}>
    <div class="form-grid">
      <label class="field">Nombre del negocio <input bind:value={form.name} required maxlength="120" /></label>
      <label class="field">Giro
        <select bind:value={form.kind}>{#each Object.entries(KINDS) as [v, l]}<option value={v}>{l}</option>{/each}</select>
      </label>
    </div>
    <label class="field">Zona horaria
      <select bind:value={form.timezone}>{#each zones as [v, l]}<option value={v}>{l}</option>{/each}</select>
      <span class="hint">Define a qué hora empieza y termina cada día en tu agenda, tus horarios de trabajo, la caja y los recordatorios.</span>
    </label>
    {#if form.timezone !== startTz}
      <p class="banner" role="status">Al cambiar la zona horaria, los horarios de trabajo y las citas se interpretarán en la nueva zona. Revisa tu agenda después.</p>
    {/if}
    <p class="meta">Tu dirección pública es <code>/reservar/{slug}</code>. No cambia al editar el nombre, para que los enlaces que ya compartiste sigan funcionando.</p>
    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      {#if saved}<span class="meta" role="status" style="align-self: center">Guardado ✓</span>{/if}
    </div>
  </form>
{/if}

<style>
  .meta { color: var(--muted); font-size: 0.88rem; }
  code { background: var(--surface-2); padding: 1px 6px; border-radius: 4px; font-size: 0.85em; }
</style>
