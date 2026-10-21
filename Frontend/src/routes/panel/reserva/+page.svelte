<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { money } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';

  let s = $state(null);
  let meta = $state({ public_path: '', mp_connected: false, mail_enabled: false });
  let services = $state([]);
  let error = $state('');
  let saved = $state(false);
  let busy = $state(false);
  let copied = $state(false);

  const link = $derived(meta.public_path ? location.origin + meta.public_path : '');

  async function load() {
    const [r, svc] = await Promise.all([api('/api/booking/settings'), api('/api/services')]);
    s = { ...r.settings };
    meta = { public_path: r.public_path, mp_connected: r.mp_connected, mail_enabled: r.mail_enabled };
    services = svc.filter((x) => x.active);
  }

  onMount(() => {
    if (session.me.role !== 'admin') return goto('/panel');
    load().catch((e) => (error = e.message));
  });

  async function save(e) {
    e?.preventDefault();
    error = '';
    saved = false;
    busy = true;
    try {
      const { enabled, min_notice_hours, max_days_ahead, cancel_min_hours, deposit_percent, reminder_hours, show_prices, welcome_text } = s;
      await api('/api/booking/settings', {
        method: 'PUT',
        body: { enabled, min_notice_hours: +min_notice_hours, max_days_ahead: +max_days_ahead, cancel_min_hours: +cancel_min_hours,
          deposit_percent: +deposit_percent, reminder_hours: +reminder_hours, show_prices, welcome_text }
      });
      saved = true;
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }

  async function toggleService(x) {
    error = '';
    try {
      await api(`/api/services/${x.id}`, { method: 'PATCH', body: { online_bookable: !x.online_bookable } });
      x.online_bookable = !x.online_bookable;
    } catch (err) {
      error = err.message;
    }
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      error = 'No se pudo copiar; selecciona el enlace y cópialo a mano.';
    }
  }
</script>

<div class="page-head">
  <div>
    <h1>Reserva en línea</h1>
    <p>Una página pública donde tus clientes agendan solos, sin crear cuenta.</p>
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if s}
  <form class="card" onsubmit={save}>
    <label class="check"><input type="checkbox" bind:checked={s.enabled} /> <b>Aceptar reservas en línea</b></label>

    <div class="linkbox">
      <input readonly value={link} aria-label="Enlace público" onfocus={(e) => e.currentTarget.select()} />
      <button type="button" class="btn ghost sm" onclick={copy}>{copied ? '¡Copiado!' : 'Copiar'}</button>
      <a class="btn ghost sm" href={meta.public_path} target="_blank" rel="noopener">Abrir</a>
    </div>
    {#if !s.enabled}<p class="meta">Mientras esté apagado, el enlace responde «no disponible».</p>{/if}

    <label class="field" style="margin-top: 1rem">Mensaje de bienvenida (opcional)
      <input bind:value={s.welcome_text} maxlength="300" placeholder="Ej. Atendemos con cita, ¡te esperamos!" />
    </label>

    <div class="form-grid">
      <label class="field">Aviso mínimo (horas)
        <input type="number" min="0" max="720" bind:value={s.min_notice_hours} />
        <span class="hint">Con cuánta anticipación se puede reservar.</span>
      </label>
      <label class="field">Días hacia adelante
        <input type="number" min="1" max="365" bind:value={s.max_days_ahead} />
        <span class="hint">Qué tan lejos pueden agendar.</span>
      </label>
      <label class="field">Cancelar hasta (horas antes)
        <input type="number" min="0" max="720" bind:value={s.cancel_min_hours} />
        <span class="hint">Después ya no pueden cancelar en línea.</span>
      </label>
    </div>

    <label class="check"><input type="checkbox" bind:checked={s.show_prices} /> Mostrar precios en la página</label>

    <h3 style="margin-top: 1.5rem">Anticipo</h3>
    <label class="field">Porcentaje del precio por adelantado
      <input type="number" min="0" max="100" bind:value={s.deposit_percent} disabled={!meta.mp_connected} />
      <span class="hint">
        {#if meta.mp_connected}
          0 = sin anticipo. Se cobra con Mercado Pago al reservar y se descuenta al cobrar la cita. Si el cliente cancela a tiempo, se le devuelve.
        {:else}
          Para pedir anticipo primero <a href="/panel/pagos">conecta tu cuenta de Mercado Pago</a>.
        {/if}
      </span>
    </label>

    <h3 style="margin-top: 1.5rem">Recordatorios por correo</h3>
    <label class="field">Enviar el recordatorio (horas antes)
      <input type="number" min="0" max="168" bind:value={s.reminder_hours} disabled={!meta.mail_enabled} />
      <span class="hint">
        {#if meta.mail_enabled}
          0 = sin recordatorios. Solo se envían a quien los aceptó y dejó su correo.
        {:else}
          El servidor aún no tiene correo configurado (SMTP_HOST y MAIL_FROM en .env), por eso no se envían correos.
        {/if}
      </span>
    </label>

    <div class="form-actions">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      {#if saved}<span class="meta" role="status">Guardado ✓</span>{/if}
    </div>
  </form>

  <h2 style="margin-top: 2rem">Servicios que se ofrecen en línea</h2>
  <div class="list">
    {#each services as x (x.id)}
      <label class="item-row">
        <span class="grow"><span class="title">{x.name}</span><span class="meta">{x.duration_min} min · {money(x.price_cents)}</span></span>
        <input type="checkbox" checked={x.online_bookable} onchange={() => toggleService(x)} aria-label="Ofrecer {x.name} en línea" />
      </label>
    {:else}
      <p class="empty">Aún no tienes servicios. Créalos en la sección Servicios.</p>
    {/each}
  </div>
  <p class="meta" style="margin-top: 0.75rem">Solo aparecen personas con horario de trabajo y que estén en la agenda.</p>
{/if}

<style>
  .linkbox { display: flex; gap: 8px; align-items: center; margin-top: 0.75rem; flex-wrap: wrap; }
  .linkbox input { flex: 1 1 240px; }
  .meta { color: var(--muted); font-size: 0.88rem; }
  h3 { font-size: 1.1rem; }
</style>
