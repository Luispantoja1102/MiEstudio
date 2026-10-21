<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';

  const token = page.params.token;
  let info = $state(null);
  let missing = $state(false);
  let error = $state('');
  let score = $state(Number(page.url.searchParams.get('s')) || 0); // el enlace del correo solo preselecciona
  let comment = $state('');
  let busy = $state(false);
  let done = $state(null);

  onMount(async () => {
    try {
      const res = await fetch(`/api/public-survey/${token}`);
      if (res.status === 404) missing = true;
      else if (!res.ok) error = 'No pudimos cargar la encuesta.';
      else info = await res.json();
      if (score < 1 || score > 5) score = 0;
    } catch {
      error = 'No pudimos cargar la encuesta.';
    }
  });

  async function send(e) {
    e.preventDefault();
    if (!score) return;
    busy = true;
    error = '';
    try {
      const res = await fetch(`/api/public-survey/${token}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ score, comment })
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'No pudimos guardar tu respuesta.');
      done = data;
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
  const LABELS = ['', 'Muy mal', 'Mal', 'Regular', 'Bien', 'Excelente'];
</script>

<PublicLayout title={info?.business ?? 'Encuesta'} subtitle="">
  {#if missing}
    <div class="card"><p>Esta encuesta no existe. Revisa el enlace de tu correo.</p></div>
  {:else if !info}
    {#if error}<p class="error" role="alert">{error}</p>{:else}<p class="meta">Cargando…</p>{/if}
  {:else if done || info.answered}
    <div class="card" style="display: grid; gap: 0.75rem; text-align: center" role="status">
      <h2>¡Gracias por tu opinión!</h2>
      {#if done?.review_url}
        <p>Si quieres, cuéntale a más personas cómo te fue:</p>
        <div><a class="btn" href={done.review_url} target="_blank" rel="noopener">Dejar una reseña en Google</a></div>
      {/if}
    </div>
  {:else}
    <form class="card" onsubmit={send} style="display: grid; gap: 0.9rem">
      <h2>¿Cómo te fue?</h2>
      <p class="meta">Tu cita de {info.service}. Toma menos de un minuto.</p>
      <div class="stars" role="radiogroup" aria-label="Calificación">
        {#each [1, 2, 3, 4, 5] as n}
          <button type="button" role="radio" aria-checked={score === n} class:on={score >= n} aria-label={`${n}: ${LABELS[n]}`} onclick={() => (score = n)}>★</button>
        {/each}
      </div>
      <p class="meta" aria-live="polite">{score ? LABELS[score] : 'Elige una calificación'}</p>
      <label class="field">{score && score < 4 ? '¿Qué podemos mejorar?' : 'Comentarios (opcional)'}
        <textarea rows="3" maxlength="1000" bind:value={comment}></textarea>
      </label>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <div><button class="btn" disabled={!score || busy}>{busy ? 'Enviando…' : 'Enviar'}</button></div>
    </form>
  {/if}
</PublicLayout>

<style>
  .stars { display: flex; gap: 6px; }
  .stars button { width: 48px; height: 48px; font-size: 1.8rem; line-height: 1; color: var(--border); background: var(--surface-2); border: 1px solid var(--border); border-radius: 10px; cursor: pointer; }
  .stars button.on { color: #e0a800; border-color: #e0a800; }
  textarea { width: 100%; font: inherit; padding: 0.6rem 0.8rem; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface); color: var(--text); }
</style>
