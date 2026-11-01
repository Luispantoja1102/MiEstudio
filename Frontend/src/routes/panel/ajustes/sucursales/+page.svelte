<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { planLimit, session } from '$lib/session.svelte.js';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';

  let branches = $state([]);
  // el plan fija cuántas sucursales activas caben: al llegar al tope no se ofrece crear ni reactivar
  const maxBranches = $derived(planLimit('max_branches'));
  const full = $derived(typeof maxBranches === 'number' && branches.filter((b) => b.active).length >= maxBranches);
  let error = $state('');
  let editing = $state(null); // null | 'new' | id
  let busy = $state(false);
  let form = $state({ name: '', address: '', phone: '' });

  const load = async () => (branches = await api('/api/branches'));
  onMount(() => {
    if (session.me.role !== 'admin') return goto('/panel');
    load().catch((e) => (error = e.message));
  });

  function startNew() {
    form = { name: '', address: '', phone: '' };
    editing = 'new';
  }
  function startEdit(b) {
    form = { name: b.name, address: b.address, phone: b.phone };
    editing = b.id;
  }

  async function save(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      if (editing === 'new') await api('/api/branches', { method: 'POST', body: form });
      else await api(`/api/branches/${editing}`, { method: 'PATCH', body: form });
      editing = null;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function toggle(b) {
    error = '';
    try {
      await api(`/api/branches/${b.id}`, { method: 'PATCH', body: { active: !b.active } });
    } catch (err) {
      error = err.message;
    }
    await load();
  }
</script>

<PageHeader title="Sucursales" description="Los lugares donde atiendes. La agenda y los horarios se organizan por sucursal.">
  {#snippet actions()}{#if !full}<button class="btn" onclick={startNew}>Nueva sucursal</button>{/if}{/snippet}
</PageHeader>

{#if full}
  <div class="gap">
    <InlineNotice tone="warning">Tu plan {planLimit('name')} incluye {maxBranches} {maxBranches === 1 ? 'sucursal' : 'sucursales'}. Para agregar más, <a href="/panel/ajustes?s=suscripcion">cambia de plan</a>.</InlineNotice>
  </div>
{/if}

{#if error}<div class="gap"><InlineNotice tone="danger" message={error} /></div>{/if}

{#if editing !== null}
  <div class="gap">
    <Surface title={editing === 'new' ? 'Nueva sucursal' : 'Editar sucursal'}>
      <form onsubmit={save}>
        <div class="form-grid">
          <label class="field">Nombre <input bind:value={form.name} required /></label>
          <label class="field">Dirección <input bind:value={form.address} maxlength="300" /></label>
          <label class="field">Teléfono <input type="tel" bind:value={form.phone} maxlength="30" /></label>
        </div>
        <div class="form-actions">
          <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
          <button type="button" class="btn ghost" onclick={() => (editing = null)}>Cancelar</button>
        </div>
      </form>
    </Surface>
  </div>
{/if}

<Surface padding="flush">
  <ul class="rows">
    {#each branches as b (b.id)}
      <li class="br" class:off={!b.active}>
        <div class="grow">
          <div class="title">{b.name} {#if !b.active}<span class="badge muted">Inactiva</span>{/if}</div>
          <div class="meta">{[b.address, b.phone].filter(Boolean).join(' · ') || 'Sin dirección'}</div>
        </div>
        <button class="btn ghost sm" onclick={() => startEdit(b)}>Editar</button>
        {#if b.active || !full}<button class="btn ghost sm" onclick={() => toggle(b)}>{b.active ? 'Desactivar' : 'Activar'}</button>{/if}
      </li>
    {:else}
      <li class="br"><span class="meta">Aún no hay sucursales.</span></li>
    {/each}
  </ul>
</Surface>

<style>
  .gap { margin-bottom: 16px; }
  .rows { list-style: none; margin: 0; padding: 0; }
  .br { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 14px 20px; border-top: 1px solid var(--border); }
  .br:first-child { border-top: 0; }
  .br.off .grow { opacity: 0.6; }
  .grow { flex: 1 1 200px; min-width: 0; }
  .title { font-weight: 600; overflow-wrap: anywhere; }
  .meta { color: var(--muted); font-size: 0.88rem; overflow-wrap: anywhere; }
</style>
