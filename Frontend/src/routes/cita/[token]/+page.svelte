<script>
  import { onDestroy, onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';
  import { money } from '$lib/format.js';
  import { fmtTime, longDate } from '$lib/time.js';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';

  const token = page.params.token;
  let copied = $state('');
  async function copy(text, key) {
    try {
      await navigator.clipboard.writeText(text);
      copied = key;
      setTimeout(() => (copied = ''), 1600);
    } catch {}
  }
  async function sendProof(e) {
    const file = e.currentTarget.files?.[0];
    e.currentTarget.value = '';
    if (!file) return;
    busy = true;
    error = '';
    try {
      const body = new FormData();
      body.append('file', file);
      const res = await fetch(`/api/public-appointments/${token}/proof`, { method: 'POST', body });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'No se pudo subir el comprobante');
      await load();
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
  const call = async (path, method = 'GET') => {
    const res = await fetch(path, { method });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw Object.assign(new Error(data.error || 'Error inesperado'), { status: res.status });
    return data;
  };
  const LABEL = { scheduled: 'Agendada', pending_payment: 'Esperando tu anticipo', completed: 'Completada', no_show: 'No asistió', cancelled: 'Cancelada' };
  const TONE = { scheduled: 'info', pending_payment: 'warning', completed: 'success', no_show: 'danger', cancelled: 'neutral' };
  const DEP = { paid: 'pagado', applied: 'aplicado a tu cuenta', refunded: 'devuelto', kept: 'conservado por inasistencia', expired: 'sin pagar' };
  const group = (n) => n.replace(/(\d{4})(?=\d)/g, '$1 ');

  let a = $state(null);
  let missing = $state(false);
  let error = $state('');
  let busy = $state(false);
  let confirming = $state(false);
  let unsubscribed = $state(false);
  let timer;
  const wantsUnsub = page.url.searchParams.get('baja') === '1';

  async function load() {
    try {
      a = await call(`/api/public-appointments/${token}`);
      error = '';
    } catch (e) {
      if (e.status === 404) missing = true;
      else error = e.message;
    }
  }

  onMount(async () => {
    await load();
    // al volver de Mercado Pago el pago puede tardar unos segundos en confirmarse
    timer = setInterval(() => a?.status === 'pending_payment' && load(), 5000);
  });
  onDestroy(() => clearInterval(timer));

  async function confirmAttendance() {
    busy = true;
    error = '';
    try {
      await call(`/api/public-appointments/${token}/confirm`, 'POST');
      await load();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  async function cancel() {
    busy = true;
    error = '';
    try {
      await call(`/api/public-appointments/${token}/cancel`, 'POST');
      confirming = false;
      await load();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  async function unsubscribe() {
    busy = true;
    error = '';
    try {
      await call(`/api/public-appointments/${token}/reminders-off`, 'POST');
      unsubscribed = true;
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  const tz = $derived(a?.timezone ?? 'America/Mexico_City');
  const day = $derived(a ? new Date(a.starts_at).toLocaleDateString('en-CA', { timeZone: tz }) : '');
  const refundable = $derived(a?.deposit?.status === 'paid');
  const badge = $derived(a ? (a.status === 'scheduled' && a.confirmed ? { label: 'Confirmada', tone: 'success' } : { label: LABEL[a.status] ?? a.status, tone: TONE[a.status] ?? 'neutral' }) : null);
</script>

<PublicLayout title={missing ? 'Cita no encontrada' : 'Tu cita'} businessTheme={a?.theme}>
  {#if missing}
    <div class="card"><p class="plain">No encontramos esta cita. Revisa que el enlace esté completo.</p></div>
  {:else if !a}
    {#if error}<InlineNotice tone="danger" message={error} />{:else}<LoadingState />{/if}
  {:else}
    {#if wantsUnsub}
      <div class="card unsub">
        {#if unsubscribed}
          <p class="plain" role="status">Listo: ya no recibirás recordatorios por correo.</p>
        {:else}
          <p class="plain">¿Quieres dejar de recibir recordatorios por correo de tus citas?</p>
          <button class="btn ghost" disabled={busy} onclick={unsubscribe}>Dejar de recibir recordatorios</button>
        {/if}
      </div>
    {/if}

    <div class="card">
      <div class="state"><StatusBadge label={badge.label} tone={badge.tone} /></div>
      <h2>{a.service}</h2>
      <dl class="rows">
        <div><dt>Día</dt><dd>{longDate(day)}</dd></div>
        <div><dt>Hora</dt><dd>{fmtTime(a.starts_at, tz)}</dd></div>
        <div><dt>Con</dt><dd>{a.staff}</dd></div>
        <div><dt>Lugar</dt><dd>{a.business} · {a.branch}{#if a.address}<span class="meta">{a.address}</span>{/if}</dd></div>
        {#if a.deposit && !(a.status === 'pending_payment')}
          <div><dt>Anticipo</dt><dd>{money(a.deposit.amount_cents)} <span class="meta inline">{DEP[a.deposit.status] ?? a.deposit.status}</span></dd></div>
        {/if}
      </dl>

      {#if a.status === 'pending_payment' && a.deposit?.transfer}
        {@const t = a.deposit.transfer}
        <section class="block" aria-labelledby="tr-h">
          <h3 id="tr-h">Para apartar tu cita transfiere {money(a.deposit.amount_cents)}</h3>
          <p class="meta">Después sube tu comprobante.{#if a.deposit.hold_expires_at}{' '}{/if}{#if a.deposit.hold_expires_at} Tu horario está apartado hasta el {new Date(a.deposit.hold_expires_at).toLocaleString('es-MX', { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit', hour12: false })} h.{/if}</p>
          <dl class="rows bank">
            <div><dt>Banco</dt><dd>{t.bank}</dd></div>
            <div><dt>Titular</dt><dd>{t.holder}</dd></div>
            {#if t.clabe}<div class="copyable"><dt>CLABE</dt><dd class="num">{group(t.clabe)}</dd><button class="btn ghost sm" onclick={() => copy(t.clabe, 'clabe')} aria-label="Copiar CLABE">{copied === 'clabe' ? 'Copiada' : 'Copiar'}</button></div>{/if}
            {#if t.card}<div class="copyable"><dt>Tarjeta</dt><dd class="num">{group(t.card)}</dd><button class="btn ghost sm" onclick={() => copy(t.card, 'card')} aria-label="Copiar número de tarjeta">{copied === 'card' ? 'Copiada' : 'Copiar'}</button></div>{/if}
            <div><dt>Concepto</dt><dd>{t.reference}</dd></div>
          </dl>
          {#if t.reject_reason}<InlineNotice tone="danger">No pudimos aceptar tu comprobante: {t.reject_reason}. Sube otro, por favor.</InlineNotice>{/if}
          {#if t.proof_uploaded}
            <InlineNotice tone="success">Recibimos tu comprobante. El negocio lo revisará y confirmará tu cita; te avisaremos en esta página.</InlineNotice>
          {/if}
          <div class="form-actions">
            <label class="btn" class:ghost={t.proof_uploaded}>
              {busy ? 'Subiendo…' : t.proof_uploaded ? 'Subir otro comprobante' : 'Subir mi comprobante'}
              <input type="file" accept="image/jpeg,image/png,image/webp,application/pdf" onchange={sendProof} disabled={busy} />
            </label>
            {#if t.whatsapp}
              <a class="btn ghost" target="_blank" rel="noopener" href={`https://wa.me/${t.whatsapp}?text=${encodeURIComponent(`Hola, te envío mi comprobante de transferencia para la cita ${t.reference} (${a.business}).`)}`}>Enviar por WhatsApp</a>
            {/if}
          </div>
          <p class="meta">Foto JPG, PNG o PDF de hasta 5 MB.</p>
        </section>
      {:else if a.status === 'pending_payment' && a.deposit}
        <section class="block">
          <h3>Falta el anticipo de {money(a.deposit.amount_cents)}</h3>
          <p class="meta">Es lo que confirma tu cita. Tu horario está apartado unos minutos.</p>
          {#if a.deposit.pay_url}<div class="form-actions"><a class="btn" href={a.deposit.pay_url}>Pagar anticipo</a></div>{/if}
          <p class="meta">Si ya pagaste, esta página se actualiza sola en unos segundos.</p>
        </section>
      {/if}

      {#if error}<InlineNotice tone="danger" message={error} />{/if}

      {#if a.status === 'scheduled' && !a.confirmed && new Date(a.starts_at) > new Date()}
        <section class="block">
          <h3>¿Nos vemos?</h3>
          <p class="meta">Avísanos que sí vas a asistir.</p>
          <div class="form-actions"><button class="btn" disabled={busy} onclick={confirmAttendance}>{busy ? 'Confirmando…' : 'Sí, confirmo mi asistencia'}</button></div>
        </section>
      {/if}

      {#if a.can_cancel}
        {#if confirming}
          <section class="block">
            <h3>¿Seguro que quieres cancelar?</h3>
            {#if refundable}<p class="meta">Te devolveremos tu anticipo de {money(a.deposit.amount_cents)}.</p>{/if}
            <div class="form-actions">
              <button class="btn" disabled={busy} onclick={cancel}>{busy ? 'Cancelando…' : 'Sí, cancelar mi cita'}</button>
              <button class="btn ghost" onclick={() => (confirming = false)}>No, conservarla</button>
            </div>
          </section>
        {:else}
          <div class="block quiet">
            <button class="btn ghost" onclick={() => (confirming = true)}>Cancelar mi cita</button>
            {#if a.status === 'scheduled'}<p class="meta">Puedes cancelar hasta {a.cancel_min_hours} h antes de la cita.</p>{/if}
          </div>
        {/if}
      {:else if a.status === 'scheduled'}
        <p class="meta block">Ya no es posible cancelar en línea. Si necesitas cambiarla, contacta al negocio.</p>
      {/if}
    </div>
  {/if}
  {#if a?.slug}<p class="meta foot"><a href="/aviso/{a.slug}">Aviso de privacidad</a> · <a href="/arco/{a.slug}">Ejercer mis derechos</a></p>{/if}
</PublicLayout>

<style>
  .plain { margin: 0 0 0.75rem; }
  .unsub { margin-bottom: 1rem; }
  .unsub .plain:last-child { margin-bottom: 0; }
  .meta { color: var(--muted); font-size: 0.88rem; margin: 0.25rem 0 0; }
  .meta.inline { display: inline; margin: 0; }
  .state { margin-bottom: 0.75rem; }
  h2 { margin: 0 0 0.75rem; font-size: 1.4rem; line-height: 1.25; overflow-wrap: anywhere; }
  h3 { margin: 0; font-size: 1.05rem; line-height: 1.3; }
  .rows { margin: 0; }
  .rows > div { display: grid; grid-template-columns: 5.5rem 1fr; gap: 12px; padding: 12px 0; border-top: 1px solid var(--border); }
  .rows dt { color: var(--muted); font-size: 0.85rem; padding-top: 2px; }
  .rows dd { margin: 0; min-width: 0; font-weight: 500; overflow-wrap: anywhere; }
  .rows dd .meta { display: block; font-weight: 400; }
  .rows dd .meta.inline { display: inline; margin-left: 4px; }
  .block { display: grid; justify-items: start; gap: 12px; margin-top: 1.25rem; padding-top: 1.25rem; border-top: 1px solid var(--border); }
  .block > * { margin: 0; }
  .block .form-actions { margin: 0; }
  .block :global(.notice) { justify-self: stretch; }
  .block.quiet { gap: 8px; }
  .bank { justify-self: stretch; width: 100%; border-bottom: 1px solid var(--border); }
  .bank > div { grid-template-columns: 5.5rem 1fr auto; align-items: center; }
  .bank > div:not(.copyable) { grid-template-columns: 5.5rem 1fr; }
  .bank .num { font-size: 1.1rem; font-weight: 600; font-variant-numeric: tabular-nums lining-nums; letter-spacing: 0.02em; }
  .bank .copyable .btn { min-width: 88px; }
  @media (max-width: 480px) {
    .bank .copyable { grid-template-columns: 5.5rem 1fr; }
    .bank .copyable .btn { grid-column: 1 / -1; width: 100%; }
  }
  .foot { text-align: center; margin-top: 1.5rem; }
  label.btn { position: relative; cursor: pointer; }
  label.btn:focus-within { outline: 2px solid var(--accent); outline-offset: 2px; }
  label.btn input { position: absolute; inset: 0; width: 100%; height: 100%; opacity: 0; cursor: pointer; }
</style>
