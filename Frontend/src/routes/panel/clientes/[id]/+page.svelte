<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { formatDate } from '$lib/format.js';
  import Surface from '$lib/ui/Surface.svelte';
  import FormSection from '$lib/ui/FormSection.svelte';
  import PackageBalance from '$lib/ui/PackageBalance.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import { canSeeNotes, session } from '$lib/session.svelte.js';

  const id = page.params.id;

  let form = $state(null);
  let title = $state('');
  let notes = $state([]);
  let newNote = $state('');
  let error = $state('');
  let saved = $state(false);
  let busy = $state(false);
  let packages = $state([]);
  const STATUS = { active: 'Vigente', exhausted: 'Agotado', expired: 'Vencido', void: 'Cancelado' };
  const sells = ['admin', 'cashier', 'reception'].includes(session.me.role);

  const MONTHS = ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'];
  const birthdayLabel = (iso) => {
    const [, m, d] = String(iso).split('-').map(Number);
    return m && d ? `${d} de ${MONTHS[m - 1]}` : '';
  };
  const activePackages = $derived(packages.filter((k) => k.status === 'active'));
  const pastPackages = $derived(packages.filter((k) => k.status !== 'active'));
  const expiry = (k) => (k.expires_at ? `vence ${formatDate(k.expires_at)}` : '');

  async function loadNotes() {
    if (canSeeNotes()) notes = await api(`/api/clients/${id}/notes`);
  }

  onMount(async () => {
    try {
      const c = await api(`/api/clients/${id}`);
      form = { name: c.name, phone: c.phone, email: c.email, birthday: c.birthday ?? '', reminders_consent: !!c.reminders_consent, marketing_consent: !!c.marketing_consent, whatsapp_consent: !!c.whatsapp_consent };
      title = c.name;
      await loadNotes();
      if (sells) packages = await api(`/api/clients/${id}/packages`);
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
      title = form.name;
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

  async function erase() {
    if (!confirm('¿Borrar los datos personales de este cliente? Se quitan nombre, teléfono, correo y notas. No se puede deshacer; las ventas se conservan sin sus datos.')) return;
    error = '';
    busy = true;
    try {
      await api(`/api/clients/${id}/erase`, { method: 'POST' });
      goto('/panel/clientes');
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<a class="back" href="/panel/clientes"><Icon name="arrow" size={16} />Clientes</a>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if !form && !error}<LoadingState label="Cargando ficha…" />{/if}

{#if form}
  <header class="page-head ident">
    <div>
      <h1>{title}</h1>
      <p class="contact">
        {#if form.phone}<span>{form.phone}</span>{/if}
        {#if form.email}<span>{form.email}</span>{/if}
        {#if form.birthday && birthdayLabel(form.birthday)}<span>Cumpleaños: {birthdayLabel(form.birthday)}</span>{/if}
        {#if !form.phone && !form.email && !form.birthday}<span>Sin datos de contacto</span>{/if}
      </p>
    </div>
  </header>

  <div class="stack">
    <Surface>
      <form onsubmit={save}>
        <FormSection title="Datos de contacto" description="Con estos datos le llegan sus recordatorios y puedes encontrarlo en el cobro.">
          <div class="form-grid">
            <label class="field">Nombre <input bind:value={form.name} required /></label>
            <label class="field">Teléfono <input type="tel" bind:value={form.phone} /></label>
            <label class="field">Correo <input type="email" bind:value={form.email} /></label>
            <label class="field">Cumpleaños <input type="date" bind:value={form.birthday} /></label>
          </div>
        </FormSection>
        <FormSection title="Consentimientos" description="Solo marca lo que el cliente aceptó. Se activan al registrar su correo o teléfono.">
          <label class="check"><input type="checkbox" bind:checked={form.reminders_consent} disabled={!form.email} /> Aceptó recibir recordatorios de sus citas por correo</label>
          <label class="check"><input type="checkbox" bind:checked={form.whatsapp_consent} disabled={form.phone.replace(/\D/g, '').length < 10} /> Aceptó recibir recordatorios por WhatsApp</label>
          <label class="check"><input type="checkbox" bind:checked={form.marketing_consent} disabled={!form.email} /> Aceptó recibir promociones y felicitación de cumpleaños por correo</label>
          {#snippet actions()}
            <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar cambios'}</button>
            {#if saved}<span class="meta saved" role="status">Cambios guardados</span>{/if}
          {/snippet}
        </FormSection>
      </form>
    </Surface>

    {#if packages.length}
      <Surface title="Paquetes" description="Sesiones prepagadas de este cliente.">
        {#if activePackages.length}
          <div class="pkgs">
            {#each activePackages as k (k.id)}
              <PackageBalance name={k.name} remaining={k.remaining} total={k.sessions_total} expiresLabel={expiry(k)} statusLabel={STATUS[k.status]} />
            {/each}
          </div>
        {/if}
        {#if pastPackages.length}
          <ul class="past">
            {#each pastPackages as k (k.id)}
              <li>
                <span><b>{k.name}</b><small>{k.sessions_used} de {k.sessions_total} usadas{k.expires_at ? ` · vence ${formatDate(k.expires_at)}` : ''}</small></span>
                <StatusBadge label={STATUS[k.status]} />
              </li>
            {/each}
          </ul>
        {/if}
      </Surface>
    {/if}

    {#if canSeeNotes()}
      <Surface title="Notas de seguimiento" description="Fórmulas, preferencias y alergias que el equipo debe recordar.">
        <form class="note-form" onsubmit={addNote}>
          <label class="field">
            <span class="sr-only">Nueva nota</span>
            <textarea bind:value={newNote} maxlength="4000" placeholder="Fórmula de color, tipo de corte, alergias, preferencias…" required></textarea>
          </label>
          <button class="btn sm">Agregar nota</button>
        </form>
        <div class="notes">
          {#each notes as n (n.id)}
            <article class="note">
              <p>{n.body}</p>
              <span class="meta">{n.author || 'Sin autor'} · {formatDate(n.created_at)}</span>
            </article>
          {:else}
            <p class="none">Aún no hay notas para este cliente.</p>
          {/each}
        </div>
      </Surface>
    {/if}

    {#if session.me.role === 'admin'}
      <section class="privacy">
        <div>
          <h2>Privacidad de este cliente</h2>
          <p>Entrega una copia de sus datos si te la pide, o bórralos cuando lo solicite. Las ventas se conservan sin sus datos personales.</p>
        </div>
        <div class="form-actions">
          <a class="btn ghost sm" href={`/api/clients/${id}/export`} download>Descargar sus datos</a>
          <button class="btn ghost sm danger" disabled={busy} onclick={erase}>Borrar sus datos personales</button>
        </div>
      </section>
    {/if}
  </div>
{/if}

<style>
  .back { display: inline-flex; align-items: center; gap: 6px; min-height: 44px; margin-bottom: 4px; color: var(--muted); text-decoration: none; font-size: 0.9rem; }
  .back :global(svg) { transform: rotate(180deg); }
  .back:hover { color: var(--accent); }
  .ident h1 { overflow-wrap: anywhere; }
  .contact { display: flex; flex-wrap: wrap; gap: 4px 20px; margin: 6px 0 0; color: var(--muted); font-size: 0.95rem; overflow-wrap: anywhere; }
  .stack { display: grid; gap: 20px; max-width: 960px; }
  .saved { color: var(--success); font-weight: 600; }
  .pkgs { display: grid; gap: 12px; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); }
  .past { list-style: none; margin: 16px 0 0; padding: 0; }
  .past li { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 0; border-top: 1px solid var(--border); }
  .past small { display: block; color: var(--muted); font-size: 0.85rem; }
  .note-form { display: grid; gap: 8px; justify-items: start; }
  .note-form .field { width: 100%; margin: 0; }
  .notes { margin-top: 20px; }
  .note { padding: 14px 0; border-top: 1px solid var(--border); }
  .note p { margin: 0 0 4px; white-space: pre-wrap; overflow-wrap: anywhere; }
  .none { margin: 0; color: var(--muted); padding-top: 14px; border-top: 1px solid var(--border); }
  .privacy { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px 24px; margin-top: 12px; padding: 20px 0 0; border-top: 1px solid var(--border); }
  .privacy h2 { margin: 0; font-size: 0.95rem; }
  .privacy p { margin: 4px 0 0; max-width: 56ch; color: var(--muted); font-size: 0.88rem; }
  .privacy .form-actions { margin: 0; }
  .privacy .danger { color: var(--danger); }
  .privacy a, .privacy button { min-height: 44px; display: inline-flex; align-items: center; }
</style>
