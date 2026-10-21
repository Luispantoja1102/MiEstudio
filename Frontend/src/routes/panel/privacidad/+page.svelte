<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { formatDay } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';

  let items = $state([]);
  let slug = $state('');
  let error = $state('');
  let loading = $state(true);
  let open = $state(null); // id de la solicitud que se está resolviendo
  let resolution = $state('');
  let busy = $state(false);

  const STATUS = { received: ['Recibida', 'warn'], in_progress: ['En proceso', 'warn'], resolved: ['Resuelta', ''], rejected: ['Rechazada', 'muted'] };

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
  const pending = $derived(items.filter((x) => x.status === 'received' || x.status === 'in_progress'));
  const link = $derived(slug ? `${location.origin}/arco/${slug}` : '');
</script>

<div class="page-head">
  <div>
    <h1>Privacidad</h1>
    <p>Solicitudes de tus clientes sobre sus datos personales (derechos ARCO). Tienes 20 días hábiles para responder cada una.</p>
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if slug}
  <div class="card tools">
    <p><b>Enlaces públicos para tus clientes</b></p>
    <p class="meta">Tu aviso: <a href="/aviso/{slug}" target="_blank" rel="noopener">{location.origin}/aviso/{slug}</a><br />Formulario de derechos: <a href={`/arco/${slug}`} target="_blank" rel="noopener">{link}</a></p>
    <p class="meta">Para entregar o borrar los datos de un cliente, abre su ficha en <a href="/panel/clientes">Clientes</a>.</p>
  </div>
{/if}

{#if loading}
  <p class="meta">Cargando…</p>
{:else if items.length === 0}
  <p class="empty">Aún no hay solicitudes de privacidad.</p>
{:else}
  <div class="list">
    {#each items as x (x.id)}
      <div class="item-row col">
        <div class="head">
          <div class="grow">
            <div class="title">{x.kind_label} · {x.requester} <span class="badge {STATUS[x.status][1]}">{STATUS[x.status][0]}</span>
              {#if x.overdue}<span class="badge bad">Vencida</span>{/if}</div>
            <div class="meta">{[x.email, x.phone].filter(Boolean).join(' · ')} · recibida {formatDay(x.created_at)}
              {#if x.status === 'received' || x.status === 'in_progress'} · responder antes del <b>{formatDay(x.due_at)}</b>{#if !x.overdue}&nbsp;({x.days_left} días){/if}{/if}</div>
          </div>
          {#if x.status === 'received'}<button class="btn ghost sm" disabled={busy} onclick={() => patch(x.id, { status: 'in_progress' })}>Empezar</button>{/if}
          {#if x.status === 'received' || x.status === 'in_progress'}
            <button class="btn sm" onclick={() => ((open = open === x.id ? null : x.id), (resolution = ''))}>Responder</button>
          {/if}
        </div>
        <p class="detail">{x.detail}</p>
        {#if x.resolution}<p class="meta"><b>Respuesta:</b> {x.resolution}</p>{/if}
        {#if open === x.id}
          <form onsubmit={(e) => e.preventDefault()}>
            <label class="field">Respuesta para la persona (se muestra en su enlace de seguimiento)
              <textarea bind:value={resolution} maxlength="2000" placeholder="Ej. Adjuntamos tus datos por correo / Borramos tus datos el…"></textarea>
            </label>
            <div class="form-actions">
              <button class="btn" disabled={busy || resolution.trim().length < 3} onclick={() => patch(x.id, { status: 'resolved', resolution })}>Marcar resuelta</button>
              <button class="btn ghost" disabled={busy || resolution.trim().length < 3} onclick={() => patch(x.id, { status: 'rejected', resolution })}>Rechazar con motivo</button>
            </div>
          </form>
        {/if}
      </div>
    {/each}
  </div>
  {#if pending.length}<p class="meta" style="margin-top: 1rem">{pending.length} pendiente(s).</p>{/if}
{/if}

<style>
  .tools { margin-bottom: 1.25rem; }
  .col { flex-direction: column; align-items: stretch; }
  .head { display: flex; flex-wrap: wrap; gap: 8px 16px; align-items: center; }
  .detail { margin: 0.25rem 0; white-space: pre-wrap; }
  .meta { color: var(--muted); font-size: 0.88rem; }
</style>
