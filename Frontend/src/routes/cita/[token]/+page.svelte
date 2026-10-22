<script>
  import { onDestroy, onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';
  import { money } from '$lib/format.js';
  import { fmtTime, longDate } from '$lib/time.js';

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
  const TONE = { scheduled: '', pending_payment: 'warn', completed: '', no_show: 'warn', cancelled: 'muted' };

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
</script>

<PublicLayout title={missing ? 'Cita no encontrada' : 'Tu cita'} businessTheme={a?.theme}>
  {#if missing}
    <div class="card"><p>No encontramos esta cita. Revisa que el enlace esté completo.</p></div>
  {:else if !a}
    {#if error}<p class="error" role="alert">{error}</p>{:else}<p class="meta">Cargando…</p>{/if}
  {:else}
    {#if wantsUnsub}
      <div class="card" style="margin-bottom: 1rem">
        {#if unsubscribed}
          <p role="status">Listo: ya no recibirás recordatorios por correo.</p>
        {:else}
          <p>¿Quieres dejar de recibir recordatorios por correo de tus citas?</p>
          <button class="btn" disabled={busy} onclick={unsubscribe}>Dejar de recibir recordatorios</button>
        {/if}
      </div>
    {/if}

    <div class="card">
      <span class="badge {TONE[a.status]}">{a.status === 'scheduled' && a.confirmed ? 'Confirmada' : (LABEL[a.status] ?? a.status)}</span>
      <h2 style="margin-top: 0.6rem">{a.service}</h2>
      <p>
        <b>{longDate(day)}</b> a las <b>{fmtTime(a.starts_at, tz)}</b><br />
        Con {a.staff}<br />
        {a.business} · {a.branch}{#if a.address}<br /><span class="meta">{a.address}</span>{/if}
      </p>

      {#if a.status === 'pending_payment' && a.deposit?.transfer}
        {@const t = a.deposit.transfer}
        <div class="notice transfer">
          <p><b>Para apartar tu cita transfiere {money(a.deposit.amount_cents)}</b> y sube tu comprobante.</p>
          <dl>
            <div><dt>Banco</dt><dd>{t.bank}</dd></div>
            <div><dt>Titular</dt><dd>{t.holder}</dd></div>
            <div><dt>CLABE</dt><dd>{t.clabe.replace(/(\d{4})(?=\d)/g, '$1 ')} <button class="mini" onclick={() => copy(t.clabe, 'clabe')}>{copied === 'clabe' ? 'Copiada' : 'Copiar'}</button></dd></div>
            <div><dt>Concepto</dt><dd>{t.reference}</dd></div>
          </dl>
          {#if t.reject_reason}<p class="bad">No pudimos aceptar tu comprobante: {t.reject_reason}. Sube otro, por favor.</p>{/if}
          {#if t.proof_uploaded}
            <p class="okline">Recibimos tu comprobante. El negocio lo revisará y confirmará tu cita; te avisaremos en esta página.</p>
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
          <p class="meta" style="margin-top: 0.6rem">Foto JPG, PNG o PDF de hasta 5 MB.{#if a.deposit.hold_expires_at} Tu horario está apartado hasta el {new Date(a.deposit.hold_expires_at).toLocaleString('es-MX', { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit', hour12: false })} h{/if}</p>
        </div>
      {:else if a.status === 'pending_payment' && a.deposit}
        <div class="notice">
          <p>Falta pagar el anticipo de <b>{money(a.deposit.amount_cents)}</b> para confirmar tu cita. Tu horario está apartado unos minutos.</p>
          {#if a.deposit.pay_url}<a class="btn" href={a.deposit.pay_url}>Pagar anticipo</a>{/if}
          <p class="meta" style="margin-top: 0.6rem">Si ya pagaste, esta página se actualiza sola en unos segundos.</p>
        </div>
      {:else if a.deposit}
        <p class="meta">Anticipo de {money(a.deposit.amount_cents)}: {({ paid: 'pagado', applied: 'aplicado a tu cuenta', refunded: 'devuelto', kept: 'conservado por inasistencia', expired: 'sin pagar' })[a.deposit.status] ?? a.deposit.status}</p>
      {/if}

      {#if error}<p class="error" role="alert">{error}</p>{/if}

      {#if a.status === 'scheduled' && !a.confirmed && new Date(a.starts_at) > new Date()}
        <div class="notice">
          <p>¿Nos vemos? Avísanos que sí vas a asistir.</p>
          <button class="btn" disabled={busy} onclick={confirmAttendance}>{busy ? 'Confirmando…' : 'Sí, confirmo mi asistencia'}</button>
        </div>
      {/if}

      {#if a.can_cancel}
        {#if confirming}
          <div class="notice">
            <p>¿Seguro que quieres cancelar?{#if refundable} Te devolveremos tu anticipo de {money(a.deposit.amount_cents)}.{/if}</p>
            <div class="form-actions">
              <button class="btn" disabled={busy} onclick={cancel}>{busy ? 'Cancelando…' : 'Sí, cancelar mi cita'}</button>
              <button class="btn ghost" onclick={() => (confirming = false)}>No, conservarla</button>
            </div>
          </div>
        {:else}
          <button class="btn ghost" onclick={() => (confirming = true)}>Cancelar mi cita</button>
          {#if a.status === 'scheduled'}<p class="meta" style="margin-top: 0.6rem">Puedes cancelar hasta {a.cancel_min_hours} h antes de la cita.</p>{/if}
        {/if}
      {:else if a.status === 'scheduled'}
        <p class="meta">Ya no es posible cancelar en línea. Si necesitas cambiarla, contacta al negocio.</p>
      {/if}
    </div>
  {/if}
  {#if a?.slug}<p class="meta foot"><a href="/aviso/{a.slug}">Aviso de privacidad</a> · <a href="/arco/{a.slug}">Ejercer mis derechos</a></p>{/if}
</PublicLayout>

<style>
  .notice { margin: 1rem 0; padding: 1rem; border-radius: var(--radius-sm); background: var(--surface-2); }
  .meta { color: var(--muted); font-size: 0.88rem; }
  .foot { text-align: center; margin-top: 1.5rem; }
  .transfer dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px 16px; margin: 0.9rem 0; }
  .transfer dt { font-size: 0.72rem; font-weight: 600; letter-spacing: 0.1em; text-transform: uppercase; color: var(--muted); }
  .transfer dd { margin: 2px 0 0; font-weight: 600; word-break: break-all; }
  .mini { margin-left: 6px; padding: 2px 10px; font: inherit; font-size: 0.78rem; font-weight: 600; color: var(--accent); background: var(--surface); border: 1px solid var(--border); border-radius: 999px; cursor: pointer; }
  .bad { color: var(--danger); font-weight: 500; }
  .okline { font-weight: 500; }
  label.btn { cursor: pointer; }
  label.btn input { display: none !important; }
</style>
