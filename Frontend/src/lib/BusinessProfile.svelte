<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { KINDS, zoneClock } from '$lib/format.js';
  import { loadSession, planLimit } from '$lib/session.svelte.js';
  import { ZONES } from '$lib/zones.js';

  // Datos del negocio: nombre, giro, zona horaria y, cuando el plan es de una sola sucursal, la dirección y el teléfono de esa sucursal.
  // La dirección pública no se cambia aquí (rompería los enlaces compartidos).

  let form = $state(null);
  let slug = $state('');
  let startTz = $state('');
  let branch = $state(null); // la única sucursal, solo si el negocio no tiene más
  let error = $state('');
  let saved = $state(false);
  let copied = $state(false);
  let busy = $state(false);

  const solo = $derived(branch !== null && (planLimit('max_branches') ?? 1) <= 1);
  const link = $derived(`${location.origin}/reservar/${slug}`);

  onMount(async () => {
    try {
      const [b, list] = await Promise.all([api('/api/business'), api('/api/branches')]);
      slug = b.slug;
      startTz = b.timezone;
      form = { name: b.name, kind: b.kind, timezone: b.timezone, address: '', phone: '' };
      if (list.length === 1) {
        branch = list[0];
        form.address = list[0].address ?? '';
        form.phone = list[0].phone ?? '';
      }
    } catch (e) {
      error = e.message;
    }
  });

  const zones = $derived(form && !ZONES.some(([z]) => z === form.timezone) ? [[form.timezone, form.timezone], ...ZONES] : ZONES);

  const clock = $derived(form ? zoneClock(form.timezone) : '');

  async function save(e) {
    e.preventDefault();
    error = '';
    saved = false;
    busy = true;
    try {
      await api('/api/business', { method: 'PUT', body: { name: form.name, kind: form.kind, timezone: form.timezone } });
      if (solo) await api(`/api/branches/${branch.id}`, { method: 'PATCH', body: { address: form.address, phone: form.phone } });
      startTz = form.timezone;
      saved = true;
      await loadSession(); // el nombre del negocio se muestra arriba
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      error = 'No se pudo copiar; selecciona el enlace y cópialo.';
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
    {#if solo}
      <div class="form-grid">
        <label class="field">Dirección <input bind:value={form.address} maxlength="300" placeholder="Calle, número y colonia" /><span class="hint">La ven tus clientes al reservar y en el aviso de privacidad.</span></label>
        <label class="field">Teléfono <input type="tel" inputmode="tel" bind:value={form.phone} maxlength="30" placeholder="664 123 4567" /></label>
      </div>
    {/if}
    <label class="field">Zona horaria
      <select bind:value={form.timezone}>{#each zones as [v, l]}<option value={v}>{l}</option>{/each}</select>
      <span class="hint">{#if clock}En tu zona son las <b>{clock}</b>.{' '}{/if}Define a qué hora empieza y termina cada día en tu agenda, tus horarios de trabajo, la caja y los recordatorios.</span>
    </label>
    {#if form.timezone !== startTz}
      <p class="banner" role="status">Al cambiar la zona horaria, los horarios de trabajo y las citas se interpretarán en la nueva zona. Revisa tu agenda después.</p>
    {/if}
    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      {#if saved}<span class="meta" role="status" style="align-self: center">Guardado ✓</span>{/if}
    </div>
  </form>

  <section class="card block">
    <h2>Tu página de reservas</h2>
    <p class="meta">Compártela por WhatsApp, Instagram o donde te escriban tus clientes. No cambia al editar el nombre, para que los enlaces que ya compartiste sigan funcionando.</p>
    <div class="linkrow">
      <code>{link}</code>
      <button type="button" class="btn ghost sm" onclick={copy}>{copied ? 'Copiado ✓' : 'Copiar enlace'}</button>
      <a class="btn ghost sm" href={`/reservar/${slug}`} target="_blank" rel="noopener">Abrir</a>
    </div>
  </section>

  <section class="card block" aria-label="Vista previa">
    <h2>Así te ven tus clientes</h2>
    <div class="preview">
      <b class="pname">{form.name || 'Tu negocio'}</b>
      <span class="meta">{[KINDS[form.kind], solo && form.address, solo && form.phone].filter(Boolean).join(' · ')}</span>
    </div>
    {#if !solo}<p class="meta">La dirección y el teléfono de cada local se editan en Sucursales.</p>{/if}
  </section>
{/if}

<style>
  .meta { color: var(--muted); font-size: 0.88rem; }
  code { background: var(--surface-2); padding: 1px 6px; border-radius: 4px; font-size: 0.85em; overflow-wrap: anywhere; }
  .block { margin-top: 1rem; }
  .block h2 { font-size: 1.15rem; margin-bottom: 0.4rem; }
  .linkrow { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 10px; margin-top: 0.75rem; }
  .linkrow code { flex: 1 1 220px; padding: 8px 10px; }
  .preview { display: grid; gap: 2px; margin-top: 0.6rem; padding: 0.9rem 1rem; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--bg); }
  .pname { font-family: var(--font-heading); font-stretch: var(--heading-stretch); font-weight: var(--heading-weight); font-size: 1.2rem; }
</style>
