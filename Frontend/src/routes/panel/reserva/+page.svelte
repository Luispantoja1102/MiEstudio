<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { money } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';

  let s = $state(null);
  let meta = $state({ public_path: '', mp_connected: false, mail_enabled: false, whatsapp_available: false, is_owner: false });
  let services = $state([]);
  let error = $state('');
  let saved = $state(false);
  let busy = $state(false);
  let copied = $state(false);

  const link = $derived(meta.public_path ? location.origin + meta.public_path : '');

  async function load() {
    const [r, svc] = await Promise.all([api('/api/booking/settings'), api('/api/services')]);
    s = { ...r.settings };
    meta = { public_path: r.public_path, mp_connected: r.mp_connected, mail_enabled: r.mail_enabled, whatsapp_available: r.whatsapp_available, is_owner: r.is_owner };
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
      const { enabled, min_notice_hours, max_days_ahead, cancel_min_hours, deposit_percent, reminder_hours, show_prices, welcome_text, privacy_email, whatsapp_reminders, public_theme, transfer_enabled, transfer_bank, transfer_holder, transfer_clabe, transfer_whatsapp, transfer_hold_hours } = s;
      await api('/api/booking/settings', {
        method: 'PUT',
        body: { enabled, min_notice_hours: +min_notice_hours, max_days_ahead: +max_days_ahead, cancel_min_hours: +cancel_min_hours,
          deposit_percent: +deposit_percent, reminder_hours: +reminder_hours, show_prices, welcome_text, privacy_email, whatsapp_reminders: !!whatsapp_reminders && meta.whatsapp_available,
          transfer_enabled: !!transfer_enabled, transfer_bank, transfer_holder, transfer_clabe, transfer_whatsapp, transfer_hold_hours: +transfer_hold_hours || 12,
          ...(meta.is_owner ? { public_theme: public_theme || 'auto' } : {}) }
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

    <label class="field" style="margin-top: 1rem">Correo para solicitudes de privacidad (opcional)
      <input type="email" bind:value={s.privacy_email} maxlength="200" placeholder="privacidad@tunegocio.mx" />
      <span class="hint">Se muestra en tu aviso de privacidad. Las solicitudes también llegan a tu panel, en Privacidad.</span>
    </label>

    <h3 style="margin-top: 1.5rem">Anticipo</h3>
    <label class="field">Porcentaje del precio por adelantado
      <input type="number" min="0" max="100" bind:value={s.deposit_percent} disabled={!meta.mp_connected && !s.transfer_enabled} />
      <span class="hint">
        {#if meta.mp_connected}
          0 = sin anticipo. Se cobra con Mercado Pago al reservar y se descuenta al cobrar la cita. Si el cliente cancela a tiempo, se le devuelve.
        {:else if s.transfer_enabled}
          0 = sin anticipo. El cliente transfiere y sube su comprobante; tú confirmas la cita al verlo. Se descuenta al cobrar la cita.
        {:else}
          Para pedir anticipo <a href="/panel/pagos">conecta tu cuenta de Mercado Pago</a> o activa las transferencias aquí abajo.
        {/if}
      </span>
    </label>

    <label class="check"><input type="checkbox" bind:checked={s.transfer_enabled} /> Aceptar anticipo por transferencia</label>
    {#if s.transfer_enabled}
      <div class="transfer">
        <p class="hint" style="margin: 0 0 0.9rem">Estos datos los ve quien reserva. Cuando suba su comprobante recibirás un aviso en la campana y tú confirmas la cita.</p>
        <div class="form-grid">
          <label class="field">Banco <input bind:value={s.transfer_bank} maxlength="60" placeholder="Ej. BBVA" /></label>
          <label class="field">Titular de la cuenta <input bind:value={s.transfer_holder} maxlength="100" placeholder="Nombre o razón social" /></label>
          <label class="field">CLABE (18 dígitos) <input bind:value={s.transfer_clabe} inputmode="numeric" maxlength="24" placeholder="000000000000000000" /></label>
          <label class="field">WhatsApp para comprobantes (opcional)
            <input type="tel" bind:value={s.transfer_whatsapp} maxlength="30" placeholder="664 123 4567" />
            <span class="hint">Si tu cliente prefiere mandarlo por WhatsApp, lo ve aquí. Tú lo adjuntas desde la agenda.</span>
          </label>
          <label class="field">Horas para mandar el comprobante
            <input type="number" min="1" max="168" bind:value={s.transfer_hold_hours} />
            <span class="hint">Mientras tanto el horario queda apartado. Si no llega, se libera solo.</span>
          </label>
        </div>
      </div>
    {/if}

    {#if meta.is_owner}
    <h3 style="margin-top: 1.5rem">Apariencia de tu página pública</h3>
    <fieldset class="themes">
      <legend class="sr-only">Tema de la página pública</legend>
      {#each [['auto', 'Que elija el cliente', 'Respeta el tema que ya usa'], ['lino', 'Lino', 'Claro y editorial'], ['light', 'Azul tinta', 'Claro y fresco'], ['ivory', 'Marfil', 'Claro y solemne'], ['dark', 'Oscuro', 'Carbón y champán']] as [v, name, hint]}
        <label class="theme" class:on={(s.public_theme ?? 'auto') === v}>
          <input type="radio" name="public_theme" value={v} bind:group={s.public_theme} />
          <span class="sw sw-{v}" aria-hidden="true"></span>
          <span><b>{name}</b><small>{hint}</small></span>
        </label>
      {/each}
    </fieldset>
    <p class="hint">Es el tema con el que tus clientes ven la reserva, su cita y tu aviso de privacidad. Solo tú, como titular del negocio, puedes cambiarlo.</p>
    {/if}

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

    <label class="check" style="margin-top: 0.75rem">
      <input type="checkbox" bind:checked={s.whatsapp_reminders} disabled={!meta.whatsapp_available || !+s.reminder_hours} />
      <span>
        También recordar por WhatsApp
        <span class="hint" style="display: block">
          {#if meta.whatsapp_available}
            Solo a quien lo aceptó al reservar. Sale con la plantilla de recordatorio aprobada por Meta; quien responde «BAJA» deja de recibirlos.
          {:else}
            El servidor aún no tiene WhatsApp configurado (WHATSAPP_TOKEN y WHATSAPP_PHONE_ID en .env).
          {/if}
        </span>
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
  .themes { display: grid; gap: 10px; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); margin: 0 0 0.5rem; padding: 0; border: 0; }
  .theme { display: flex; align-items: center; gap: 12px; padding: 0.7rem 0.9rem; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; transition: border-color var(--dur-ui) var(--ease), background-color var(--dur-ui) var(--ease); }
  .theme:hover { border-color: var(--muted); }
  .theme.on { border-color: var(--accent); background: var(--accent-soft); }
  .theme input { position: absolute; opacity: 0; pointer-events: none; }
  .theme input:focus-visible + .sw { outline: 2px solid var(--accent); outline-offset: 2px; }
  .theme small { display: block; font-size: 0.8rem; color: var(--muted); }
  .sw { flex: none; width: 36px; height: 36px; border-radius: 10px; border: 1px solid var(--border); }
  .sw-auto { background: linear-gradient(135deg, #f3f1ec 50%, #101012 50%); }
  .sw-lino { background: linear-gradient(135deg, #a8614c 50%, #f8f6f2 50%); }
  .sw-light { background: linear-gradient(135deg, #13203b 50%, #f3f1ec 50%); }
  .sw-ivory { background: linear-gradient(135deg, #101012 50%, #f4efe6 50%); }
  .sw-dark { background: linear-gradient(135deg, #c9a45c 50%, #101012 50%); }
  .linkbox { display: flex; gap: 8px; align-items: center; margin-top: 0.75rem; flex-wrap: wrap; }
  .linkbox input { flex: 1 1 240px; }
  .meta { color: var(--muted); font-size: 0.88rem; }
  h3 { font-size: 1.1rem; }
  .grow span { display: block; }
  .item-row input[type='checkbox'] { width: 20px; height: 20px; accent-color: var(--accent); }
  .transfer { margin: 0.5rem 0 1rem; padding: 1.1rem 1.1rem 0.2rem; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface-2); }
</style>
