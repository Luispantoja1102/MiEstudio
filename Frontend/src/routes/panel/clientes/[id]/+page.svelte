<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { formatDate } from '$lib/format.js';
  import { canSeeNotes } from '$lib/session.svelte.js';

  const id = page.params.id;

  let form = $state(null);
  let notes = $state([]);
  let newNote = $state('');
  let error = $state('');
  let saved = $state(false);
  let busy = $state(false);

  async function loadNotes() {
    if (canSeeNotes()) notes = await api(`/api/clients/${id}/notes`);
  }

  onMount(async () => {
    try {
      const c = await api(`/api/clients/${id}`);
      form = { name: c.name, phone: c.phone, email: c.email, birthday: c.birthday ?? '' };
      await loadNotes();
    } catch (e) {
      error = e.status === 404 ? 'No encontramos a este cliente.' : e.message;
    }
  });

  async function save(e) {
    e.preventDefault();
    error = '';
    saved = false;
    busy = true;
    try {
      await api(`/api/clients/${id}`, { method: 'PATCH', body: form });
      saved = true;
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function addNote(e) {
    e.preventDefault();
    error = '';
    try {
      await api(`/api/clients/${id}/notes`, { method: 'POST', body: { body: newNote } });
      newNote = '';
      await loadNotes();
    } catch (err) {
      error = err.message;
    }
  }
</script>

<a class="back" href="/panel/clientes"><Icon name="arrow" size={16} />Clientes</a>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if form}
  <div class="page-head"><h1>{form.name}</h1></div>

  <form class="card" onsubmit={save}>
    <div class="form-grid">
      <label class="field">Nombre <input bind:value={form.name} required /></label>
      <label class="field">Teléfono <input type="tel" bind:value={form.phone} /></label>
      <label class="field">Correo <input type="email" bind:value={form.email} /></label>
      <label class="field">Cumpleaños <input type="date" bind:value={form.birthday} /></label>
    </div>
    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar cambios'}</button>
      {#if saved}<span class="meta" role="status" style="align-self: center">Guardado ✓</span>{/if}
    </div>
  </form>

  {#if canSeeNotes()}
    <section class="notes">
      <h2>Notas de seguimiento</h2>
      <form onsubmit={addNote}>
        <label class="field">
          <span class="sr-only">Nueva nota</span>
          <textarea bind:value={newNote} maxlength="4000" placeholder="Fórmula de color, tipo de corte, alergias, preferencias…" required></textarea>
        </label>
        <button class="btn sm">Agregar nota</button>
      </form>
      <div class="list" style="margin-top: 1.25rem">
        {#each notes as n (n.id)}
          <article class="item-row" style="display: block">
            <p style="margin: 0 0 0.4rem; white-space: pre-wrap">{n.body}</p>
            <span class="meta">{n.author || 'Sin autor'} · {formatDate(n.created_at)}</span>
          </article>
        {:else}
          <div class="empty">Aún no hay notas para este cliente.</div>
        {/each}
      </div>
    </section>
  {/if}
{/if}

<style>
  .back { display: inline-flex; align-items: center; gap: 6px; margin-bottom: 1rem; color: var(--muted); text-decoration: none; font-size: 0.9rem; }
  .back :global(svg) { transform: rotate(180deg); }
  .back:hover { color: var(--accent); }
  .notes { margin-top: 2.5rem; }
  .notes h2 { font-size: 1.5rem; }
</style>
