<script>
  import { onMount } from 'svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { money } from '$lib/format.js';

  // Anticipo por transferencia de una cita apartada: datos para pagar, comprobante y confirmación del titular.
  // appt: id, deposit_cents, proof_at, reject_reason, manage_path, client_name (lo que traiga la agenda)
  let { appt, canReview = false, canView = false, onchange } = $props();

  let info = $state(null);
  let error = $state('');
  let busy = $state(false);
  let rejecting = $state(false);
  let reason = $state('');
  let copied = $state('');

  onMount(async () => {
    try {
      info = await api('/api/transfer-info');
    } catch (e) {
      error = e.message;
    }
  });

  const fmtClabe = (c) => c.replace(/(\d{4})(?=\d)/g, '$1 ');
  const reference = $derived(`CITA-${appt.id}`);
  const link = $derived(appt.manage_path ? location.origin + appt.manage_path : '');
  const message = $derived(
    info
      ? `Hola ${appt.client_name ?? ''}, para apartar tu cita transfiere ${money(appt.deposit_cents)} a:\n${info.bank} · ${info.holder}\n${info.clabe ? `CLABE: ${info.clabe}\n` : ''}${info.card ? `Tarjeta: ${info.card}\n` : ''}Concepto: ${reference}\n` +
          (link ? `Sube tu comprobante aquí: ${link}` : 'Envíanos el comprobante por este medio.')
      : ''
  );

  async function copy(text, key) {
    try {
      await navigator.clipboard.writeText(text);
      copied = key;
      setTimeout(() => (copied = ''), 1600);
    } catch {}
  }

  async function run(fn) {
    error = '';
    busy = true;
    try {
      await fn();
      onchange?.();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  function upload(e) {
    const file = e.currentTarget.files?.[0];
    e.currentTarget.value = '';
    if (!file) return;
    const body = new FormData();
    body.append('file', file);
    run(() => api(`/api/appointments/${appt.id}/proof`, { method: 'POST', body }));
  }

  const confirmPay = () => run(() => api(`/api/appointments/${appt.id}/transfer/confirm`, { method: 'POST' }));
  const reject = () =>
    run(async () => {
      await api(`/api/appointments/${appt.id}/transfer/reject`, { method: 'POST', body: { reason } });
      rejecting = false;
      reason = '';
    });
</script>

<section class="tp" aria-label="Anticipo por transferencia">
  <h4><Icon name="receipt" size={18} />Anticipo por transferencia · {money(appt.deposit_cents)}</h4>

  {#if info?.enabled !== false && info}
    <dl>
      <div><dt>Banco</dt><dd>{info.bank}</dd></div>
      <div><dt>Titular</dt><dd>{info.holder}</dd></div>
      {#if info.clabe}<div><dt>CLABE</dt><dd class="mono">{fmtClabe(info.clabe)}<button class="mini" onclick={() => copy(info.clabe, 'clabe')}>{copied === 'clabe' ? 'Copiada' : 'Copiar'}</button></dd></div>{/if}
      {#if info.card}<div><dt>Tarjeta</dt><dd class="mono">{fmtClabe(info.card)}<button class="mini" onclick={() => copy(info.card, 'card')}>{copied === 'card' ? 'Copiada' : 'Copiar'}</button></dd></div>{/if}
      <div><dt>Concepto</dt><dd class="mono">{reference}</dd></div>
    </dl>
    <button class="btn ghost sm" onclick={() => copy(message, 'msg')}>{copied === 'msg' ? 'Mensaje copiado' : 'Copiar mensaje para el cliente'}</button>
  {/if}

  {#if appt.reject_reason}<p class="note bad">Se rechazó el comprobante anterior: {appt.reject_reason}. El cliente puede mandar otro.</p>{/if}

  {#if appt.proof_at}
    <p class="note ok"><Icon name="check" size={15} />Comprobante recibido {new Date(appt.proof_at).toLocaleString('es-MX', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}.
      {#if canView}<a href={`/api/appointments/${appt.id}/proof`} target="_blank" rel="noopener">Ver comprobante</a>{/if}</p>
  {:else}
    <p class="note">Aún no llega el comprobante. El cliente lo sube desde su enlace o te lo manda por WhatsApp: adjúntalo aquí.</p>
  {/if}

  <div class="actions">
    <label class="btn ghost sm file">
      <Icon name="arrow" size={15} />{appt.proof_at ? 'Reemplazar comprobante' : 'Adjuntar comprobante'}
      <input type="file" accept="image/jpeg,image/png,image/webp,application/pdf" onchange={upload} disabled={busy} />
    </label>
    {#if canReview}
      <button class="btn sm" disabled={busy} onclick={confirmPay}>Confirmar pago y cita</button>
      {#if appt.proof_at}<button class="btn ghost sm" disabled={busy} onclick={() => (rejecting = !rejecting)}>Rechazar</button>{/if}
    {:else}
      <span class="hint">Solo el titular confirma el pago.</span>
    {/if}
  </div>

  {#if rejecting}
    <div class="reject">
      <label class="field" style="margin: 0">Motivo (lo verá el cliente)
        <input bind:value={reason} maxlength="200" placeholder="Ej. El monto no coincide" />
      </label>
      <button class="btn sm" disabled={busy || reason.trim().length < 3} onclick={reject}>Rechazar comprobante</button>
    </div>
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
</section>

<style>
  .tp { display: grid; gap: 0.8rem; padding: 1rem; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface-2); }
  h4 { display: flex; align-items: center; gap: 8px; margin: 0; font-family: var(--font-body, inherit); font-size: 0.95rem; }
  dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px 16px; margin: 0; }
  dt { font-size: 0.72rem; font-weight: 600; letter-spacing: 0.1em; text-transform: uppercase; color: var(--muted); }
  dd { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin: 2px 0 0; font-weight: 600; }
  .mono { font-variant-numeric: tabular-nums; letter-spacing: 0.02em; }
  .mini { flex: none; padding: 2px 10px; font: inherit; font-size: 0.78rem; font-weight: 600; color: var(--accent); background: var(--surface); border: 1px solid var(--border); border-radius: 999px; cursor: pointer; }
  .note { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; margin: 0; font-size: 0.88rem; color: var(--muted); }
  .note.ok { color: var(--text); }
  .note.bad { color: var(--danger); }
  .note a { font-weight: 600; }
  .actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
  .file { cursor: pointer; }
  .file input { display: none !important; } /* el botón es la etiqueta; el selector de archivos se abre al tocarla */
  .hint { font-size: 0.82rem; color: var(--muted); }
  .reject { display: grid; gap: 8px; justify-items: start; }
</style>
