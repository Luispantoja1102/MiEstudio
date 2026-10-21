<script>
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';

  const slug = page.params.slug;
  const KINDS = [
    ['access', 'Acceso', 'Quiero saber qué datos tienen de mí.'],
    ['rectify', 'Rectificación', 'Quiero corregir datos que están mal.'],
    ['cancel', 'Cancelación', 'Quiero que borren mis datos.'],
    ['oppose', 'Oposición', 'No quiero que usen mis datos para un fin (por ejemplo, recordatorios).']
  ];
  let form = $state({ kind: 'access', name: '', email: '', phone: '', detail: '', accept_privacy: false });
  let error = $state('');
  let busy = $state(false);
  let done = $state(null);

  async function submit(e) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      const res = await fetch(`/api/public/${slug}/arco`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(form) });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(res.status === 404 ? 'Esta página no está disponible.' : data.error || 'No se pudo enviar.');
      done = data;
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<PublicLayout title="Ejercer mis derechos" subtitle="Acceso, rectificación, cancelación u oposición sobre tus datos personales.">
  {#if done}
    <div class="card" role="status">
      <h2>Recibimos tu solicitud</h2>
      <p>El negocio te responderá en un máximo de {done.days} días hábiles{form.email ? ` al correo ${form.email}` : ''}.</p>
      <p>Guarda este enlace para ver su estado:</p>
      <a class="btn" href={done.status_path}>Ver el estado de mi solicitud</a>
    </div>
  {:else}
    <form class="card" onsubmit={submit}>
      <fieldset class="kinds">
        <legend>¿Qué derecho quieres ejercer?</legend>
        {#each KINDS as [value, label, hint]}
          <label class="check kind"><input type="radio" name="kind" {value} bind:group={form.kind} /> <span><b>{label}</b><small>{hint}</small></span></label>
        {/each}
      </fieldset>
      <label class="field">Tu nombre <input bind:value={form.name} autocomplete="name" required maxlength="120" /></label>
      <div class="form-grid">
        <label class="field">Correo <input type="email" bind:value={form.email} autocomplete="email" maxlength="200" /></label>
        <label class="field">Teléfono <input type="tel" bind:value={form.phone} autocomplete="tel" maxlength="30" /></label>
      </div>
      <p class="hint">Deja al menos uno para poder responderte. Usaremos el mismo con el que reservaste para identificarte.</p>
      <label class="field">Cuéntanos qué necesitas
        <textarea bind:value={form.detail} required minlength="5" maxlength="1500"></textarea>
      </label>
      <label class="check"><input type="checkbox" bind:checked={form.accept_privacy} required /> He leído el <a href="/aviso/{slug}">aviso de privacidad</a></label>
      {#if error}<p class="error" role="alert" style="margin-top: 1rem">{error}</p>{/if}
      <div class="form-actions"><button class="btn" disabled={busy || !form.accept_privacy}>{busy ? 'Enviando…' : 'Enviar solicitud'}</button></div>
    </form>
  {/if}
</PublicLayout>

<style>
  .kinds { border: 0; padding: 0; margin: 0 0 1.25rem; display: grid; gap: 8px; }
  .kinds legend { font-weight: 600; margin-bottom: 0.5rem; padding: 0; }
  .kind { align-items: flex-start; }
  .kind span { display: grid; }
  .kind small { color: var(--muted); font-weight: 400; }
  .hint { color: var(--muted); font-size: 0.85rem; margin: -0.4rem 0 1rem; }
</style>
