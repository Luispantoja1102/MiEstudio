<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { KINDS, zoneClock } from '$lib/format.js';
  import { loadSession, planLimit } from '$lib/session.svelte.js';
  import { ZONES } from '$lib/zones.js';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import FormSection from '$lib/ui/FormSection.svelte';
  import Surface from '$lib/ui/Surface.svelte';

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

{#if error}<div class="gap"><InlineNotice tone="danger" message={error} /></div>{/if}

{#if form}
  <Surface>
    <form onsubmit={save}>
      <FormSection title="Datos del negocio" description="Lo que ven tu equipo y tus clientes.">
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
          <div class="gap"><InlineNotice tone="warning" message="Al cambiar la zona horaria, los horarios de trabajo y las citas se interpretarán en la nueva zona. Revisa tu agenda después." /></div>
        {/if}
        {#snippet actions()}
          <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
          {#if saved}<span class="meta saved" role="status">Guardado</span>{/if}
        {/snippet}
      </FormSection>

      <FormSection title="Tu página de reservas" description="Compártela por WhatsApp, Instagram o donde te escriban tus clientes.">
        <div class="linkrow">
          <code>{link}</code>
          <button type="button" class="btn ghost sm" onclick={copy}>{copied ? 'Copiado' : 'Copiar enlace'}</button>
          <a class="btn ghost sm" href={`/reservar/${slug}`} target="_blank" rel="noopener">Abrir</a>
        </div>
        <p class="meta">No cambia al editar el nombre, para que los enlaces que ya compartiste sigan funcionando.</p>
      </FormSection>

      <FormSection title="Así te ven tus clientes">
        <div class="preview" aria-label="Vista previa">
          <b class="pname">{form.name || 'Tu negocio'}</b>
          <span class="meta">{[KINDS[form.kind], solo && form.address, solo && form.phone].filter(Boolean).join(' · ')}</span>
        </div>
        {#if !solo}<p class="meta">La dirección y el teléfono de cada local se editan en Sucursales.</p>{/if}
      </FormSection>
    </form>
  </Surface>
{/if}

<style>
  .gap { margin-bottom: 16px; }
  .meta { color: var(--muted); font-size: 0.88rem; }
  .saved { align-self: center; }
  code { background: var(--surface-2); padding: 1px 6px; border-radius: 4px; font-size: 0.85em; overflow-wrap: anywhere; }
  .linkrow { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 10px; }
  .linkrow code { flex: 1 1 220px; padding: 10px 12px; }
  .linkrow :global(.btn) { min-height: 44px; display: inline-flex; align-items: center; }
  .preview { display: grid; gap: 2px; padding: 16px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--bg); }
  .pname { font-size: 1.15rem; }
</style>
