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
    <section class="panel" role="status">
      <h2>Esta encuesta no existe</h2>
      <p class="meta">Revisa el enlace de tu correo o mensaje.</p>
    </section>
  {:else if !info}
    {#if error}<p class="error" role="alert">{error}</p>{:else}<p class="meta" role="status">Cargando…</p>{/if}
  {:else if done || info.answered}
    <section class="panel thanks" role="status">
      <h2>¡Gracias por tu opinión!</h2>
      <p class="meta">{done ? 'Ya registramos tu respuesta.' : 'Esta encuesta ya fue contestada.'}</p>
      {#if done?.review_url}
        <div class="review">
          <p>Si quieres, cuéntale a más personas cómo te fue.</p>
          <a class="btn" href={done.review_url} target="_blank" rel="noopener">Dejar una reseña en Google</a>
        </div>
      {/if}
    </section>
  {:else}
    <form class="panel" onsubmit={send}>
      <h2>¿Cómo te fue?</h2>
      <p class="meta">Tu cita de {info.service}. Toma menos de un minuto.</p>
      <div class="opts" role="radiogroup" aria-label="Calificación">
        {#each [1, 2, 3, 4, 5] as n}
          <label class="opt" class:on={score === n}>
            <input type="radio" name="score" value={n} checked={score === n} onchange={() => (score = n)} />
            <span class="dot" aria-hidden="true"></span>
            <span class="lbl">{LABELS[n]}</span>
            <span class="num" aria-hidden="true">{n}</span>
          </label>
        {/each}
      </div>
      <p class="hint" aria-live="polite">{score ? `Elegiste: ${LABELS[score]}` : 'Elige una calificación para enviar.'}</p>
      <label class="field">{score && score < 4 ? '¿Qué podemos mejorar?' : 'Comentarios (opcional)'}
        <textarea rows="4" maxlength="1000" bind:value={comment}></textarea>
      </label>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <button class="btn lg block" disabled={!score || busy}>{busy ? 'Enviando…' : 'Enviar'}</button>
    </form>
  {/if}
</PublicLayout>

<style>
  .panel { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 24px 20px; }
  .panel h2 { margin: 0 0 4px; font-size: 1.4rem; }
  .meta { color: var(--muted); font-size: 0.92rem; margin: 0; }
  .opts { margin: 20px 0 8px; border: 1px solid var(--control-border); border-radius: var(--radius-sm); overflow: hidden; }
  .opt { position: relative; display: flex; align-items: center; gap: 14px; min-height: 56px; padding: 0 16px; cursor: pointer; background: var(--surface); border-bottom: 1px solid var(--border); font-weight: 500; }
  .opt:last-child { border-bottom: 0; }
  .opt:hover { background: var(--surface-2); }
  .opt input { position: absolute; opacity: 0; inset: 0; width: 100%; height: 100%; margin: 0; cursor: pointer; }
  .dot { flex: none; width: 22px; height: 22px; border: 2px solid var(--control-border); border-radius: 50%; background: var(--surface); }
  .lbl { flex: 1; }
  .num { color: var(--muted); font-variant-numeric: tabular-nums; font-size: 0.85rem; }
  .opt.on { background: var(--accent-soft); box-shadow: inset 4px 0 0 var(--accent); font-weight: 700; }
  .opt.on .dot { border-color: var(--accent); background: var(--accent); box-shadow: inset 0 0 0 4px var(--surface); }
  .opt:has(input:focus-visible) { outline: 2px solid var(--accent); outline-offset: -2px; }
  .hint { margin: 0 0 20px; min-height: 1.3em; color: var(--muted); font-size: 0.88rem; }
  .field { margin-bottom: 16px; }
  textarea { margin-top: 0.4rem; }
  .thanks .review { margin-top: 20px; padding-top: 20px; border-top: 1px solid var(--border); display: grid; gap: 12px; justify-items: start; }
  .thanks .review p { margin: 0; }
</style>
