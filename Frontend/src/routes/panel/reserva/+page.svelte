<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { money } from '$lib/format.js';
  import { session } from '$lib/session.svelte.js';
  import { BANKS } from '$lib/banks.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import FormSection from '$lib/ui/FormSection.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';

  let s = $state(null);
  const canTransfer = session.me?.business?.plan_features?.deposits !== false;
  const canMP = session.me?.business?.plan_features?.deposits_mp !== false;
  const canDeposit = canTransfer || canMP;
  const mpReady = $derived(canMP && meta.mp_connected);
  let meta = $state({ public_path: '', mp_connected: false, mail_enabled: false, whatsapp_available: false, is_owner: false });
  let services = $state([]);
  let error = $state('');
  let saved = $state(false);
  let busy = $state(false);
  let bankOther = $state(false);
  const bankSelect = $derived(bankOther ? '__otro' : (s?.transfer_bank ?? ''));
  function pickBank(e) {
    const v = e.currentTarget.value;
    if (v === '__otro') { bankOther = true; s.transfer_bank = ''; } else { bankOther = false; s.transfer_bank = v; }
  }
  let copied = $state(false);

  const link = $derived(meta.public_path ? location.origin + meta.public_path : '');

  async function load() {
    const [r, svc] = await Promise.all([api('/api/booking/settings'), api('/api/services')]);
    s = { ...r.settings };
    meta = { public_path: r.public_path, mp_connected: r.mp_connected, mail_enabled: r.mail_enabled, whatsapp_available: r.whatsapp_available, is_owner: r.is_owner };
    services = svc.filter((x) => x.active);
    bankOther = !!s.transfer_bank && !BANKS.includes(s.transfer_bank);
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
      const { enabled, min_notice_hours, max_days_ahead, cancel_min_hours, deposit_percent, reminder_hours, show_prices, welcome_text, privacy_email, whatsapp_reminders, public_theme, transfer_enabled, transfer_bank, transfer_holder, transfer_clabe, transfer_card, transfer_whatsapp, transfer_hold_hours } = s;
      await api('/api/booking/settings', {
        method: 'PUT',
        body: { enabled, min_notice_hours: +min_notice_hours, max_days_ahead: +max_days_ahead, cancel_min_hours: +cancel_min_hours,
          deposit_percent: +deposit_percent, reminder_hours: +reminder_hours, show_prices, welcome_text, privacy_email, whatsapp_reminders: !!whatsapp_reminders && meta.whatsapp_available,
          transfer_enabled: !!transfer_enabled, transfer_bank, transfer_holder, transfer_clabe, transfer_card, transfer_whatsapp, transfer_hold_hours: +transfer_hold_hours || 12,
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

<PageHeader title="Reserva en línea" description="Una página pública donde tus clientes agendan solos, sin crear cuenta." />

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if !s}
  {#if !error}<LoadingState />{/if}
{:else}
  <form onsubmit={save}>
    <FormSection title="Enlace y disponibilidad" description="Quién puede reservar, con cuánta anticipación y qué ve.">
      <label class="check"><input type="checkbox" bind:checked={s.enabled} /> <b>Aceptar reservas en línea</b></label>

      <div class="linkbox">
        <input readonly value={link} aria-label="Enlace público" onfocus={(e) => e.currentTarget.select()} />
        <button type="button" class="btn ghost sm" onclick={copy}>{copied ? '¡Copiado!' : 'Copiar'}</button>
        <a class="btn ghost sm" href={meta.public_path} target="_blank" rel="noopener">Abrir</a>
      </div>
      {#if !s.enabled}<p class="meta">Mientras esté apagado, el enlace responde «no disponible».</p>{/if}

      <label class="field top">Mensaje de bienvenida (opcional)
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

      <label class="field top">Correo para solicitudes de privacidad (opcional)
        <input type="email" bind:value={s.privacy_email} maxlength="200" placeholder="privacidad@tunegocio.mx" />
        <span class="hint">Se muestra en tu aviso de privacidad. Las solicitudes también llegan a tu panel, en Privacidad.</span>
      </label>
    </FormSection>

    {#if meta.is_owner}
      <FormSection title="Apariencia de tu página pública" description="Solo tú, como titular del negocio, puedes cambiarlo.">
        <fieldset class="themes">
          <legend class="sr-only">Tema de la página pública</legend>
          {#each [['auto', 'Que elija el cliente', 'Respeta el tema que ya usa', '#f3f1ec', '#101012'], ['lino', 'Lino', 'Claro y editorial', '#a8614c', '#f8f6f2'], ['light', 'Azul tinta', 'Claro y fresco', '#b8402f', '#f1eee7'], ['ivory', 'Marfil', 'Claro y solemne', '#101012', '#f4efe6'], ['dark', 'Oscuro', 'Carbón y champán', '#c9a45c', '#101012']] as [v, name, hint, c1, c2]}
            <label class="theme" class:on={(s.public_theme ?? 'auto') === v}>
              <input type="radio" name="public_theme" value={v} bind:group={s.public_theme} />
              <span class="sw" style="--c1: {c1}; --c2: {c2}" aria-hidden="true"></span>
              <span><b>{name}</b><small>{hint}</small></span>
            </label>
          {/each}
        </fieldset>
        <p class="hint">Es el tema con el que tus clientes ven la reserva, su cita y tu aviso de privacidad.</p>
      </FormSection>
    {/if}

    <FormSection title="Anticipo" description="Un porcentaje del precio que el cliente paga al reservar.">
      {#if !canDeposit}
        <InlineNotice tone="info">Tu plan no incluye anticipos. <a href="/panel/suscripcion">Ver planes</a></InlineNotice>
      {:else if !canMP}
        <InlineNotice tone="info">Tu plan cobra anticipos por transferencia. El cobro con Mercado Pago está en Crecimiento y Pro. <a href="/panel/suscripcion">Ver planes</a></InlineNotice>
      {/if}
      <label class="field">Porcentaje del precio por adelantado
        <input type="number" min="0" max="100" bind:value={s.deposit_percent} disabled={!canDeposit || (!mpReady && !(canTransfer && s.transfer_enabled))} />
        <span class="hint">
          {#if mpReady}
            0 = sin anticipo. Se cobra con Mercado Pago al reservar y se descuenta al cobrar la cita. Si el cliente cancela a tiempo, se le devuelve.
          {:else if canTransfer && s.transfer_enabled}
            0 = sin anticipo. El cliente transfiere y sube su comprobante; tú confirmas la cita al verlo. Se descuenta al cobrar la cita.
          {:else}
            {#if canMP}Para pedir anticipo <a href="/panel/pagos">conecta tu cuenta de Mercado Pago</a> o activa las transferencias aquí abajo.{:else}Para pedir anticipo activa las transferencias aquí abajo y captura tu CLABE.{/if}
          {/if}
        </span>
      </label>
    </FormSection>

    <FormSection title="Transferencia y datos bancarios" description="Para cobrar el anticipo por SPEI sin Mercado Pago.">
      <label class="check"><input type="checkbox" bind:checked={s.transfer_enabled} disabled={!canTransfer} /> Aceptar anticipo por transferencia</label>
      {#if s.transfer_enabled}
        <p class="hint">Estos datos los ve quien reserva. Cuando suba su comprobante recibirás un aviso en la campana y tú confirmas la cita.</p>
        <div class="form-grid">
          <label class="field">Banco
            <select value={bankSelect} onchange={pickBank}>
              <option value="" disabled>Elige tu banco</option>
              {#each BANKS as b (b)}<option value={b}>{b}</option>{/each}
              <option value="__otro">Otro banco…</option>
            </select>
          </label>
          {#if bankOther}<label class="field">Nombre del banco <input bind:value={s.transfer_bank} maxlength="60" placeholder="Escribe el nombre" /></label>{/if}
          <label class="field">Titular de la cuenta <input bind:value={s.transfer_holder} maxlength="100" placeholder="Nombre o razón social" /></label>
          <label class="field">CLABE interbancaria (18 dígitos) <input bind:value={s.transfer_clabe} inputmode="numeric" maxlength="24" placeholder="000000000000000000" />
            <span class="hint">Para transferencias SPEI desde cualquier banco.</span></label>
          <label class="field">Número de tarjeta (15 o 16 dígitos) <input bind:value={s.transfer_card} inputmode="numeric" maxlength="23" placeholder="0000 0000 0000 0000" autocomplete="off" />
            <span class="hint">Opcional si ya pusiste la CLABE. Puedes dar las dos y quien paga elige. Pon al menos una.</span></label>
          <label class="field">WhatsApp para comprobantes (opcional)
            <input type="tel" bind:value={s.transfer_whatsapp} maxlength="30" placeholder="664 123 4567" />
            <span class="hint">Si tu cliente prefiere mandarlo por WhatsApp, lo ve aquí. Tú lo adjuntas desde la agenda.</span>
          </label>
          <label class="field">Horas para mandar el comprobante
            <input type="number" min="1" max="168" bind:value={s.transfer_hold_hours} />
            <span class="hint">Mientras tanto el horario queda apartado. Si no llega, se libera solo.</span>
          </label>
        </div>
      {/if}
    </FormSection>

    <FormSection title="Recordatorios" description="Aviso antes de la cita para que no se les pase.">
      <label class="field">Enviar el recordatorio por correo (horas antes)
        <input type="number" min="0" max="168" bind:value={s.reminder_hours} disabled={!meta.mail_enabled} />
        <span class="hint">
          {#if meta.mail_enabled}
            0 = sin recordatorios. Solo se envían a quien los aceptó y dejó su correo.
          {:else}
            Este servidor aún no tiene el correo habilitado, por eso no se envían correos.
          {/if}
        </span>
      </label>

      <label class="check wa">
        <input type="checkbox" bind:checked={s.whatsapp_reminders} disabled={!meta.whatsapp_available || !+s.reminder_hours} />
        <span>
          También recordar por WhatsApp
          <span class="hint block">
            {#if meta.whatsapp_available}
              Solo a quien lo aceptó al reservar. Sale con la plantilla de recordatorio aprobada por Meta; quien responde «BAJA» deja de recibirlos.
            {:else}
              Este servidor aún no tiene WhatsApp habilitado.
            {/if}
          </span>
        </span>
      </label>
    </FormSection>

    <div class="form-actions save">
      <button class="btn" disabled={busy}>{busy ? 'Guardando…' : 'Guardar'}</button>
      {#if saved}<span class="meta" role="status">Guardado ✓</span>{/if}
    </div>
  </form>

  <div class="catalog">
    <FormSection title="Servicios que se ofrecen en línea" description="Se aplica al marcar, sin pulsar Guardar. Solo aparecen personas con horario de trabajo y que estén en la agenda.">
      <div class="list">
        {#each services as x (x.id)}
          <label class="item-row">
            <span class="grow"><span class="title">{x.name}</span><span class="meta">{x.duration_min} min · {money(x.price_cents)}</span></span>
            <input type="checkbox" checked={x.online_bookable} onchange={() => toggleService(x)} aria-label="Ofrecer {x.name} en línea" />
          </label>
        {:else}
          <EmptyState title="Aún no tienes servicios." description="Créalos en la sección Servicios." />
        {/each}
      </div>
    </FormSection>
  </div>
{/if}

<style>
  .top { margin-top: 16px; }
  .meta { color: var(--muted); font-size: 0.88rem; }
  .hint { color: var(--muted); font-size: 0.85rem; margin: 8px 0 0; }
  .hint.block { display: block; margin: 2px 0 0; }
  .wa { align-items: flex-start; margin-top: 8px; }
  .wa input { margin-top: 13px; }
  .linkbox { display: flex; gap: 8px; align-items: center; margin-top: 12px; flex-wrap: wrap; }
  .linkbox input { flex: 1 1 240px; min-width: 0; }
  .linkbox :global(.btn) { min-height: 44px; display: inline-flex; align-items: center; }
  .themes { display: grid; gap: 10px; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); margin: 0 0 8px; padding: 0; border: 0; min-width: 0; }
  .theme { display: flex; align-items: center; gap: 12px; padding: 10px 14px; min-height: 44px; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; transition: border-color var(--dur-ui) var(--ease), background-color var(--dur-ui) var(--ease); }
  .theme:hover { border-color: var(--muted); }
  .theme.on { border-color: var(--accent); background: var(--accent-soft); }
  .theme input { position: absolute; opacity: 0; pointer-events: none; }
  .theme input:focus-visible + .sw { outline: 2px solid var(--accent); outline-offset: 2px; }
  .theme small { display: block; font-size: 0.8rem; color: var(--muted); }
  /* Muestra en dos tintas planas (sin degradado): mitad c1, mitad c2. */
  .sw { flex: none; width: 36px; height: 36px; border-radius: 10px; border: 1px solid var(--border); background: var(--c2); position: relative; overflow: hidden; }
  .sw::after { content: ''; position: absolute; inset: 0; background: var(--c1); clip-path: polygon(0 0, 100% 0, 0 100%); }
  .grow span { display: block; }
  .item-row input[type='checkbox'] { width: 20px; height: 20px; accent-color: var(--accent); }
  .save { align-items: center; padding-top: 24px; border-top: 1px solid var(--border); }
  .catalog { margin-top: 24px; padding-top: 24px; border-top: 1px solid var(--border); }
  .catalog :global(.form-section) { padding-top: 0; }
</style>
