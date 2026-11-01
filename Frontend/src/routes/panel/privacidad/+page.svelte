<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { formatDay } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';

  let items = $state([]);
  let slug = $state('');
  let error = $state('');
  let loading = $state(true);
  let open = $state(null); // id de la solicitud que se está resolviendo
  let resolution = $state('');
  let busy = $state(false);

  const STATUS = { received: ['Recibida', 'warning'], in_progress: ['En proceso', 'info'], resolved: ['Resuelta', 'success'], rejected: ['Rechazada', 'neutral'] };

  async function load() {
    items = await api('/api/arco');
  }
  onMount(async () => {
    if (session.me.role !== 'admin') return goto('/panel');
    try {
      const [, b] = await Promise.all([load(), api('/api/booking/settings')]);
      slug = b.slug;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  async function patch(id, body) {
    error = '';
    busy = true;
    try {
      await api(`/api/arco/${id}`, { method: 'PATCH', body });
      open = null;
      resolution = '';
      await load();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }
  const COLUMNS = [
    { key: 'who', label: 'Solicitante' },
    { key: 'kind', label: 'Derecho' },
    { key: 'status', label: 'Estado' },
    { key: 'due', label: 'Vencimiento' },
    { key: 'acts', label: 'Acciones' }
  ];
  const current = $derived(items.find((x) => x.id === open));
  function toggle(x) {
    open = open === x.id ? null : x.id;
    resolution = '';
  }
  const pending = $derived(items.filter((x) => x.status === 'received' || x.status === 'in_progress'));
  const link = $derived(slug ? `${location.origin}/arco/${slug}` : '');
</script>

<PageHeader title="Privacidad" description="Solicitudes de tus clientes sobre sus datos personales (derechos ARCO). Tienes 20 días hábiles para responder cada una." />

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if slug}
  <div class="gap">
    <Surface title="Enlaces públicos para tus clientes" description="Compártelos o ponlos en tu local. Para entregar o borrar los datos de un cliente, abre su ficha en Clientes.">
      <dl class="links">
        <div><dt>Aviso de privacidad</dt><dd><a href="/aviso/{slug}" target="_blank" rel="noopener">{location.origin}/aviso/{slug}</a></dd></div>
        <div><dt>Formulario de derechos</dt><dd><a href={`/arco/${slug}`} target="_blank" rel="noopener">{link}</a></dd></div>
      </dl>
      <a class="btn ghost sm" href="/panel/clientes">Ir a Clientes</a>
    </Surface>
  </div>
{/if}

{#if loading}
  <LoadingState />
{:else if items.length === 0}
  <Surface><EmptyState title="Aún no hay solicitudes de privacidad." description="Cuando un cliente use el formulario de derechos, su solicitud aparecerá aquí con su fecha límite." /></Surface>
{:else}
  <Surface title="Solicitudes" description={pending.length ? `${pending.length} pendiente(s).` : 'No hay solicitudes pendientes.'} padding="flush">
    <DataTable caption="Solicitudes de privacidad" columns={COLUMNS} rows={items}>
      {#snippet cell({ row: x, column })}
        {#if column.key === 'who'}
          <div class="who">
            <strong>{x.requester}</strong>
            <span class="meta">{[x.email, x.phone].filter(Boolean).join(' · ')}</span>
            <span class="meta">Recibida {formatDay(x.created_at)}</span>
            <p class="detail">{x.detail}</p>
            {#if x.resolution}<p class="resp"><b>Respuesta:</b> {x.resolution}</p>{/if}
          </div>
        {:else if column.key === 'kind'}
          {x.kind_label}
        {:else if column.key === 'status'}
          <div class="states">
            <StatusBadge label={STATUS[x.status][0]} tone={STATUS[x.status][1]} />
            {#if x.overdue}<StatusBadge label="Vencida" tone="danger" />{/if}
          </div>
        {:else if column.key === 'due'}
          {#if x.status === 'received' || x.status === 'in_progress'}
            <b class="num">{formatDay(x.due_at)}</b>
            {#if !x.overdue}<span class="meta"> {x.days_left} días</span>{/if}
          {:else}
            <span class="meta">—</span>
          {/if}
        {:else}
          <div class="acts">
            {#if x.status === 'received'}<button class="btn ghost sm" disabled={busy} onclick={() => patch(x.id, { status: 'in_progress' })}>Empezar</button>{/if}
            {#if x.status === 'received' || x.status === 'in_progress'}
              <button class="btn sm" aria-expanded={open === x.id} onclick={() => toggle(x)}>{open === x.id ? 'Cerrar' : 'Responder'}</button>
            {/if}
          </div>
        {/if}
      {/snippet}
    </DataTable>
  </Surface>

  {#if current}
    <div class="gap top">
      <Surface title="Responder a {current.requester}" description="{current.kind_label}. Se muestra a la persona en su enlace de seguimiento.">
        <form onsubmit={(e) => e.preventDefault()}>
          <label class="field">Respuesta para la persona
            <textarea bind:value={resolution} maxlength="2000" rows="4" placeholder="Ej. Adjuntamos tus datos por correo / Borramos tus datos el…"></textarea>
          </label>
          <div class="form-actions">
            <button class="btn" disabled={busy || resolution.trim().length < 3} onclick={() => patch(current.id, { status: 'resolved', resolution })}>Marcar resuelta</button>
            <button class="btn ghost" disabled={busy || resolution.trim().length < 3} onclick={() => patch(current.id, { status: 'rejected', resolution })}>Rechazar con motivo</button>
          </div>
        </form>
      </Surface>
    </div>
  {/if}
{/if}

<style>
  .gap { margin-bottom: 20px; }
  .top { margin-top: 20px; }
  .links { margin: 0 0 16px; display: grid; gap: 12px; }
  .links dt { font-size: 0.8rem; font-weight: 600; color: var(--muted); }
  .links dd { margin: 2px 0 0; overflow-wrap: anywhere; }
  .meta { color: var(--muted); font-size: 0.85rem; }
  .who { display: grid; gap: 2px; min-width: 220px; max-width: 46ch; }
  .detail { margin: 6px 0 0; font-weight: 400; white-space: pre-wrap; overflow-wrap: anywhere; }
  .resp { margin: 6px 0 0; font-size: 0.88rem; color: var(--muted); overflow-wrap: anywhere; }
  .states { display: flex; flex-wrap: wrap; gap: 4px; }
  .num { font-variant-numeric: tabular-nums; white-space: nowrap; }
  .acts { display: flex; flex-wrap: wrap; gap: 8px; }
  textarea { width: 100%; font: inherit; padding: 0.6rem 0.8rem; border: 1px solid var(--control-border); border-radius: var(--radius-sm); background: var(--surface); color: var(--text); }
</style>
