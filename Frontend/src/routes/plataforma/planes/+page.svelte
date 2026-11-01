<script>
  import { onMount, tick } from 'svelte';
  import { api } from '$lib/api.js';
  import { pesos } from '$lib/format.js';
  import { canWritePlatform } from '$lib/session.svelte.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import FormSection from '$lib/ui/FormSection.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';

  const write = canWritePlatform();
  let plans = $state([]);
  let error = $state('');
  let notice = $state('');
  let editing = $state(null); // id del plan que se edita
  let form = $state(null);
  let busy = $state(false);
  let loaded = $state(false);
  let editBox = $state(null);

  async function load() {
    try {
      plans = await api('/api/platform/plans');
    } catch (e) {
      error = e.message;
    } finally {
      loaded = true;
    }
  }
  onMount(load);
  const lim = (n) => (n === null ? 'Sin límite' : n);

  function edit(p) {
    notice = error = '';
    editing = p.id;
    form = { description: p.description, price_month: p.price_month, price_year: p.price_year, unlimited: p.max_staff === null, max_staff: p.max_staff ?? 6,
      max_branches: p.max_branches, pos: p.pos, deposits: p.deposits, deposits_mp: p.deposits_mp, terminal: p.terminal, support: p.support };
    tick().then(() => {
      editBox?.scrollIntoView({ block: 'start' });
      editBox?.querySelector('input')?.focus();
    });
  }

  async function save(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      await api(`/api/platform/plans/${editing}`, {
        method: 'PUT',
        body: { description: form.description, price_month: +form.price_month, price_year: +form.price_year, max_staff: form.unlimited ? null : +form.max_staff,
          max_branches: +form.max_branches, pos: form.pos, deposits: form.deposits, deposits_mp: form.deposits_mp, terminal: form.terminal, support: form.support }
      });
      notice = 'Plan guardado. Ya se ve así en la página de inicio, en la suscripción de los negocios y en los cobros nuevos.';
      editing = null;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function reset(p) {
    if (!confirm(`¿Volver el plan ${p.name} a su configuración original?`)) return;
    error = '';
    try {
      await api(`/api/platform/plans/${p.id}`, { method: 'DELETE' });
      notice = `El plan ${p.name} volvió a su configuración original.`;
      editing = null;
      await load();
    } catch (err) {
      error = err.message;
    }
  }

  const edited = $derived(plans.find((p) => p.id === editing));
  const si = (b) => (b ? 'Sí' : 'No');
  const supportLabel = { correo: 'Por correo', prioritario: 'Prioritario', dedicado: 'Dedicado' };
  const cols = $derived([{ key: 'concept', label: 'Concepto' }, ...plans.map((p) => ({ key: String(p.id), label: p.name }))]);
  const rows = $derived([
    { id: 'desc', label: 'Descripción', get: (p) => p.description },
    { id: 'pm', label: 'Precio al mes', get: (p) => pesos(p.price_month) },
    { id: 'py', label: 'Precio al año', get: (p) => pesos(p.price_year) },
    { id: 'staff', label: 'Personas en agenda', get: (p) => lim(p.max_staff) },
    { id: 'br', label: 'Sucursales', get: (p) => p.max_branches },
    { id: 'pos', label: 'Caja, ventas e inventario', get: (p) => si(p.pos) },
    { id: 'dep', label: 'Anticipo por transferencia', get: (p) => si(p.deposits) },
    { id: 'mp', label: 'Anticipo con Mercado Pago', get: (p) => si(p.deposits_mp) },
    { id: 'term', label: 'Cobro con terminal', get: (p) => si(p.terminal) },
    { id: 'sup', label: 'Soporte', get: (p) => supportLabel[p.support] ?? p.support },
    { id: 'use', label: 'Negocios en uso', get: (p) => p.businesses },
    { id: 'pay', label: 'Negocios pagando', get: (p) => p.paying },
    ...(write ? [{ id: 'act', label: 'Edición', action: true }] : [])
  ]);
</script>

<PageHeader title="Planes" description={'Lo que incluye cada suscripción y cuántos negocios la usan.' + (write ? ' Edita precios y límites aquí: se aplican de inmediato.' : '')} />

<div class="stack">
  {#if error}<InlineNotice tone="danger" message={error} />{/if}
  {#if notice}<InlineNotice tone="success" message={notice} />{/if}

  {#if !loaded}
    <LoadingState label="Cargando planes…" />
  {:else if plans.length === 0}
    {#if !error}<EmptyState title="No hay planes configurados" />{/if}
  {:else}
    <Surface padding="flush">
      <DataTable caption="Comparación de planes: precios, límites y funciones" columns={cols} rows={rows} cell={planCell} />
    </Surface>

    {#if editing !== null && edited}
      <div bind:this={editBox}>
        <Surface title={`Editar plan ${edited.name}`} description="Los cambios se aplican a la página de inicio, a la suscripción de los negocios y a los cobros nuevos.">
          <form onsubmit={save}>
            <FormSection title="Precios" description="En pesos mexicanos, por periodo.">
              <label class="field">Descripción <input bind:value={form.description} maxlength="120" /></label>
              <div class="two">
                <label class="field">Precio al mes (MXN) <input type="number" min="0" bind:value={form.price_month} required /></label>
                <label class="field">Precio al año (MXN) <input type="number" min="0" bind:value={form.price_year} required /></label>
              </div>
            </FormSection>
            <FormSection title="Límites" description="Cuánta gente y cuántos locales cubre el plan.">
              <div class="two">
                <label class="field">Personas en agenda
                  <input type="number" min="1" bind:value={form.max_staff} disabled={form.unlimited} />
                </label>
                <label class="field">Sucursales <input type="number" min="1" bind:value={form.max_branches} required /></label>
              </div>
              <label class="check"><input type="checkbox" bind:checked={form.unlimited} /> Sin límite de personas</label>
            </FormSection>
            <FormSection title="Funciones incluidas">
              <label class="check"><input type="checkbox" bind:checked={form.pos} /> Caja, ventas, inventario, comisiones y reportes</label>
              <label class="check"><input type="checkbox" bind:checked={form.deposits} /> Anticipos por transferencia (con comprobante)</label>
              <label class="check"><input type="checkbox" bind:checked={form.deposits_mp} /> Anticipos con Mercado Pago</label>
              <label class="check"><input type="checkbox" bind:checked={form.terminal} /> Cobro con terminal de Mercado Pago</label>
            </FormSection>
            <FormSection title="Soporte">
              <label class="field">Nivel de soporte
                <select bind:value={form.support}><option value="correo">Por correo</option><option value="prioritario">Prioritario</option><option value="dedicado">Dedicado</option></select>
              </label>
            </FormSection>
            <div class="form-actions bar">
              <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar plan'}</button>
              <button type="button" class="btn ghost" onclick={() => (editing = null)}>Cancelar</button>
              <button type="button" class="btn ghost restore" onclick={() => reset(edited)}>Volver a la configuración original</button>
            </div>
          </form>
        </Surface>
      </div>
    {/if}
  {/if}
</div>

{#snippet planCell({ row, column })}
  {#if column.key === 'concept'}
    {row.label}
  {:else}
    {@const p = plans.find((x) => String(x.id) === column.key)}
    {#if row.action}
      <button class="btn ghost sm" onclick={() => edit(p)} aria-label={`Editar plan ${p.name}`} aria-pressed={editing === p.id}>Editar</button>
    {:else if row.id === 'pm' || row.id === 'py'}
      <strong>{row.get(p)}</strong>
    {:else}
      {row.get(p)}
    {/if}
  {/if}
{/snippet}

<style>
  .stack { display: grid; gap: 16px; }
  .two { display: grid; gap: 0 12px; grid-template-columns: 1fr 1fr; }
  form :global(.check) { display: flex; }
  .bar { padding-top: 16px; border-top: 1px solid var(--border); }
  .restore { margin-left: auto; }
  .restore:hover { color: var(--danger); }
  @media (max-width: 520px) { .two { grid-template-columns: 1fr; } .restore { margin-left: 0; } }
</style>
