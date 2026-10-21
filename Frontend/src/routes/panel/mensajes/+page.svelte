<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { formatDate } from '$lib/format.js';

  let s = $state(null);
  let meta = $state({ mail_enabled: true, promo_clients: 0, survey_clients: 0 });
  let surveys = $state({ sent: 0, answered: 0, average: null, items: [] });
  let error = $state('');
  let saved = $state(false);
  let busy = $state(false);

  async function load() {
    const [e, v] = await Promise.all([api('/api/engagement'), api('/api/surveys')]);
    s = e.settings;
    meta = e;
    surveys = v;
  }
  onMount(() => load().catch((err) => (error = err.message)));

  async function save(e) {
    e.preventDefault();
    error = '';
    saved = false;
    busy = true;
    try {
      await api('/api/engagement', {
        method: 'PUT',
        body: { ...s, winback_days: Number(s.winback_days), survey_delay_hours: Number(s.survey_delay_hours) }
      });
      saved = true;
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<div class="page-head">
  <div>
    <h1>Mensajes a clientes</h1>
    <p>Correos automáticos para que tus clientes vuelvan y te recomienden.</p>
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if !s}
  <p class="meta">Cargando…</p>
{:else}
  {#if !meta.mail_enabled}
    <p class="banner" role="status">El correo aún no está configurado en el servidor (SMTP), así que nada de esto se enviará todavía.</p>
  {/if}

  <form class="card" onsubmit={save}>
    <p class="meta">Solo se escribe a clientes con correo que lo aceptaron: las promociones (cumpleaños y reactivación) necesitan que el cliente las haya aceptado; la encuesta, que haya aceptado recordatorios o promociones. Cada correo trae un enlace para darse de baja.</p>

    <fieldset>
      <legend>Cumpleaños</legend>
      <label class="check"><input type="checkbox" bind:checked={s.birthday_enabled} /> Felicitar a mis clientes el día de su cumpleaños</label>
      {#if s.birthday_enabled}
        <label class="field">Mensaje (opcional, por ejemplo una promoción)
          <textarea rows="3" maxlength="500" bind:value={s.birthday_message} placeholder="Este mes tu corte tiene 20 % de descuento."></textarea>
        </label>
        <p class="meta">{meta.promo_clients} clientes con correo aceptaron promociones.</p>
      {/if}
    </fieldset>

    <fieldset>
      <legend>Reactivación</legend>
      <label class="check"><input type="checkbox" bind:checked={s.winback_enabled} /> Invitar a volver a quien lleva tiempo sin venir</label>
      {#if s.winback_enabled}
        <label class="field">Días sin venir antes de escribirles
          <input type="number" min="14" max="365" bind:value={s.winback_days} />
        </label>
        <label class="field">Mensaje (opcional)
          <textarea rows="3" maxlength="500" bind:value={s.winback_message} placeholder="Te esperamos con un café de cortesía."></textarea>
        </label>
        <p class="meta">Se manda un solo correo por cada visita (no repite) y solo a quien no tenga una cita pendiente.</p>
      {/if}
    </fieldset>

    <fieldset>
      <legend>Encuesta de satisfacción</legend>
      <label class="check"><input type="checkbox" bind:checked={s.survey_enabled} /> Preguntar cómo les fue después de su cita</label>
      {#if s.survey_enabled}
        <label class="field">Horas después de la cita
          <input type="number" min="1" max="168" bind:value={s.survey_delay_hours} />
        </label>
        <label class="field">Enlace para dejar una reseña en Google (opcional)
          <input type="url" bind:value={s.review_url} placeholder="https://g.page/r/…/review" />
          <span class="hint">Se ofrece a todas las personas que contestan, sea cual sea su calificación: Google no permite pedir reseñas solo a quien está contento.</span>
        </label>
        <p class="meta">Máximo una encuesta por cliente por semana. {meta.survey_clients} clientes pueden recibirla.</p>
      {/if}
    </fieldset>

    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      {#if saved}<span class="meta" role="status" style="align-self: center">Guardado ✓</span>{/if}
    </div>
  </form>

  <section style="margin-top: 1.5rem">
    <h2 style="font-size: 1.25rem">Opiniones</h2>
    {#if surveys.sent === 0}
      <p class="empty">Aún no se envía ninguna encuesta.</p>
    {:else}
      <p class="meta">
        {surveys.answered} de {surveys.sent} contestadas{#if surveys.average !== null} · promedio <b>{surveys.average.toFixed(1)}</b> de 5{/if}
      </p>
      <div class="list">
        {#each surveys.items as it}
          <div class="item-row">
            <div class="grow">
              <div class="title">{it.client} <span class="badge" class:muted={it.score < 4}>{'★'.repeat(it.score)}{'☆'.repeat(5 - it.score)}</span></div>
              <div class="meta">{it.service} · {formatDate(it.answered_at)}</div>
              {#if it.comment}<p style="margin: 0.35rem 0 0">{it.comment}</p>{/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </section>
{/if}

<style>
  fieldset { border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 0.75rem 1rem 1rem; margin: 1rem 0; display: grid; gap: 0.5rem; }
  legend { padding: 0 6px; font-weight: 600; }
  textarea { width: 100%; font: inherit; padding: 0.6rem 0.8rem; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface); color: var(--text); }
  .hint { color: var(--muted); font-size: 0.85rem; }
</style>
