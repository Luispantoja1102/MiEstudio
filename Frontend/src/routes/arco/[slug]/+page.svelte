<script>
  import { onMount } from 'svelte';
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
  let bizTheme = $state('auto');
  onMount(async () => {
    try {
      const r = await fetch(`/api/public/${slug}/privacy`);
      if (r.ok) bizTheme = (await r.json()).theme ?? 'auto';
    } catch {}
  });

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

<PublicLayout title="Ejercer mis derechos" subtitle="Acceso, rectificación, cancelación u oposición sobre tus datos personales." businessTheme={bizTheme}>
  {#if done}
    <div class="card done" role="status">
      <h2>Recibimos tu solicitud</h2>
      <p>El negocio te responderá en un máximo de {done.days} días hábiles{form.email ? ` al correo ${form.email}` : ''}.</p>
      <p>Guarda este enlace para ver su estado:</p>
      <a class="btn" href={done.status_path}>Ver el estado de mi solicitud</a>
    </div>
  {:else}
    <form class="card form" onsubmit={submit}>
      <fieldset class="kinds">
        <legend>¿Qué derecho quieres ejercer?</legend>
        {#each KINDS as [value, label, hint]}
          <label class="kind" class:on={form.kind === value}><input type="radio" name="kind" {value} bind:group={form.kind} /> <span><b>{label}</b><small>{hint}</small></span></label>
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
      <label class="check accept"><input type="checkbox" bind:checked={form.accept_privacy} required /> He leído el <a href="/aviso/{slug}">aviso de privacidad</a></label>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <div class="form-actions"><button class="btn lg" disabled={busy || !form.accept_privacy}>{busy ? 'Enviando…' : 'Enviar solicitud'}</button></div>
    </form>
  {/if}
</PublicLayout>

<style>
  .form { padding: 24px; }
  .kinds { border: 0; padding: 0; margin: 0 0 1.5rem; display: grid; }
  .kinds legend { font-weight: 600; font-size: 1.05rem; margin-bottom: 0.75rem; padding: 0; }
  .kind { display: flex; align-items: flex-start; gap: 12px; min-height: 44px; padding: 14px 12px; border-bottom: 1px solid var(--border); cursor: pointer; border-left: 3px solid transparent; }
  .kind:first-of-type { border-top: 1px solid var(--border); }
  .kind:hover { background: var(--surface-2); }
  .kind.on { background: var(--accent-soft); border-left-color: var(--accent); }
  .kind input { width: 18px; height: 18px; margin: 3px 0 0; accent-color: var(--accent); flex: none; }
  .kind span { display: grid; gap: 2px; min-width: 0; }
  .kind small { color: var(--muted); font-size: 0.88rem; }
  .kind:has(input:focus-visible) { outline: 2px solid var(--accent); outline-offset: -2px; }
  .hint { color: var(--muted); font-size: 0.85rem; margin: -0.4rem 0 1.25rem; }
  .accept { margin-top: 0.25rem; }
  .error { margin: 1rem 0 0; }
  .done { padding: 24px; display: grid; gap: 0.75rem; justify-items: start; border-left: 3px solid var(--accent); }
  .done h2, .done p { margin: 0; }
  @media (max-width: 480px) { .form, .done { padding: 18px 16px; } .form-actions .btn { width: 100%; } }
</style>
