<script>
  import { onDestroy, untrack } from 'svelte';
  import Modal from '$lib/Modal.svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { money, toCents, toPesos } from '$lib/format.js';

  // «Cobrar venta»: elige cómo paga el cliente lo que falta (due) y arma los pagos para /api/sales.
  // El anticipo y las sesiones de paquete ya vienen descontados en `due`; aquí solo se cobra el resto.
  let { open = $bindable(false), due = 0, items = 0, branchId, point, onpay } = $props();

  const METHODS = [
    ['cash', 'Efectivo', 'wallet'],
    ['card', 'Tarjeta', 'card'],
    ['transfer', 'Transferencia', 'receipt'],
    ['split', 'Mixto', 'percent']
  ];

  let method = $state('cash');
  let received = $state('');
  let cardPart = $state('');
  let reference = $state('');
  let manual = $state(false); // cobrar la tarjeta sin la terminal
  let error = $state('');
  let busy = $state(false);
  let phase = $state('idle'); // idle | waiting | cancelling
  let chargeId = '';
  let timer;

  const none = $derived(due <= 0);
  const cardCents = $derived(none ? 0 : method === 'card' ? due : method === 'split' ? Math.min(Math.max(toCents(cardPart || '0') || 0, 0), due) : 0);
  const cashCents = $derived(none ? 0 : method === 'cash' ? due : method === 'split' ? due - cardCents : 0);
  const receivedCents = $derived(toCents(received || '0') || 0);
  const change = $derived(cashCents > 0 && receivedCents > 0 ? Math.max(receivedCents - cashCents, 0) : 0);
  const short = $derived(cashCents > 0 && receivedCents > 0 ? Math.max(cashCents - receivedCents, 0) : 0);
  const usesTerminal = $derived(cardCents >= 100 && point?.ok && !manual);
  const locked = $derived(busy || phase !== 'idle');
  // Exacto + los billetes que probablemente entregue el cliente
  const bills = $derived.by(() => {
    if (cashCents <= 0) return [];
    const out = [cashCents];
    for (const step of [1000, 2000, 5000, 10000, 20000, 50000, 100000]) {
      const v = Math.ceil(cashCents / step) * step;
      if (v > cashCents && !out.includes(v)) out.push(v);
    }
    return out.slice(0, 5);
  });
  const billLabel = (v, i) => (i === 0 ? 'Exacto' : money(v).replace(/\.00$/, ''));

  // cada vez que se abre, empieza limpio
  let wasOpen = false;
  $effect(() => {
    if (open && !wasOpen) untrack(reset);
    wasOpen = open;
  });
  function reset() {
    method = 'cash';
    received = '';
    cardPart = '';
    reference = '';
    manual = false;
    error = '';
    busy = false;
    phase = 'idle';
    chargeId = '';
    stop();
  }
  onDestroy(stop);
  function stop() {
    clearInterval(timer);
    timer = undefined;
  }
  function pick(id) {
    if (locked) return;
    method = id;
    error = '';
    received = '';
  }

  // Cobro en la terminal: se envía, se espera la aprobación y recién entonces se registra la venta.
  async function chargeTerminal() {
    phase = 'waiting';
    const c = await api('/api/payments/point/charges', { method: 'POST', body: { branch_id: branchId, amount_cents: cardCents } });
    chargeId = c.id;
    return new Promise((resolve, reject) => {
      timer = setInterval(async () => {
        try {
          const s = await api(`/api/payments/point/charges/${chargeId}`);
          if (s.status === 'open') return;
          stop();
          if (s.status === 'approved') resolve(chargeId);
          else reject(new Error(s.detail || 'El cobro no se completó en la terminal.'));
        } catch {
          /* se reintenta en el siguiente ciclo */
        }
      }, 2000);
    });
  }
  async function cancelTerminal() {
    phase = 'cancelling';
    try {
      await api(`/api/payments/point/charges/${chargeId}/cancel`, { method: 'POST' });
    } catch (e) {
      error = e.message;
    }
    stop();
    phase = 'idle';
  }

  async function confirm(e) {
    e?.preventDefault();
    if (locked) return;
    error = '';
    if (!none) {
      if (cashCents > 0 && receivedCents > 0 && short > 0) return (error = `Efectivo insuficiente. Faltan ${money(short)}`);
      if (method === 'split' && (cardCents <= 0 || cardCents >= due)) return (error = 'Escribe cuánto pagan con tarjeta, menos del total.');
      if (cardCents > 0 && cardCents < 100 && point?.ok && !manual) return (error = 'La terminal cobra desde $1.00.');
    }
    busy = true;
    try {
      const payments = [];
      if (cashCents > 0) payments.push({ method: 'cash', amount_cents: cashCents, received_cents: receivedCents >= cashCents ? receivedCents : null });
      if (cardCents > 0) {
        if (usesTerminal) payments.push({ method: 'mp_point', amount_cents: cardCents, mp_charge_id: await chargeTerminal() });
        else payments.push({ method: 'card', amount_cents: cardCents });
      }
      if (method === 'transfer' && !none) payments.push({ method: 'transfer', amount_cents: due, reference: reference.trim() });
      await onpay(payments);
      open = false;
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
      phase = 'idle';
      stop();
    }
  }

  function keys(e) {
    if (open && e.key === 'F12') {
      e.preventDefault();
      confirm();
    }
  }
</script>

<svelte:window onkeydown={keys} />

<Modal bind:open title="Cobrar venta" {locked}>
  <form class="pay" onsubmit={confirm}>
    <div class="total">
      <span>{none ? 'Ya está cubierto' : 'Total a pagar'}</span>
      <strong>{money(due)}</strong>
      <small>{items} {items === 1 ? 'artículo' : 'artículos'}</small>
    </div>

    {#if none}
      <p class="note">El anticipo o el paquete cubren la venta completa. Confirma para registrarla.</p>
    {:else}
      <div class="methods" role="radiogroup" aria-label="Forma de pago">
        {#each METHODS as [id, label, icon]}
          <button type="button" role="radio" aria-checked={method === id} class="method" class:on={method === id} disabled={locked} onclick={() => pick(id)}>
            <Icon name={icon} size={20} />
            <span>{label}</span>
          </button>
        {/each}
      </div>

      {#if method === 'split'}
        <label class="field">Parte con tarjeta
          <input inputmode="decimal" bind:value={cardPart} placeholder="0.00" disabled={locked} />
        </label>
        <p class="note">Resto en efectivo: <b>{money(cashCents)}</b></p>
      {/if}

      {#if cardCents > 0 && point}
        <div class="point" class:ok={point.ok && !manual} class:bad={!point.ok && !manual} role="status" aria-live="polite">
          {#if phase !== 'idle'}
            <strong>{phase === 'cancelling' ? 'Cancelando…' : 'Pasa o inserta la tarjeta en la terminal'}</strong>
            <span>Cobrando {money(cardCents)}. No cierres esta ventana.</span>
            <button type="button" class="btn ghost sm" disabled={phase === 'cancelling'} onclick={cancelTerminal}>Cancelar cobro</button>
          {:else if manual}
            <span>Cobra <b>{money(cardCents)}</b> en tu terminal y confirma cuando salga aprobado.</span>
            {#if point.ok}<button type="button" class="btn ghost sm" onclick={() => (manual = false)}>Usar la terminal conectada</button>{/if}
          {:else if point.ok}
            <strong>Terminal lista</strong>
            <span>Al confirmar se envía el cobro de {money(cardCents)} a la terminal.</span>
            <button type="button" class="btn ghost sm" onclick={() => (manual = true)}>Cobrar manual</button>
          {:else}
            <strong>Terminal no disponible</strong>
            <span>{point.message || 'Cobra con tu terminal y confirma cuando salga aprobado.'}</span>
          {/if}
        </div>
      {:else if cardCents > 0}
        <p class="note">Cobra <b>{money(cardCents)}</b> en tu terminal y confirma cuando salga aprobado.</p>
      {/if}

      {#if cashCents > 0}
        <label class="field">Efectivo recibido
          <input class="big" inputmode="decimal" bind:value={received} placeholder={toPesos(cashCents)} disabled={locked} onfocus={(e) => e.currentTarget.select()} />
        </label>
        <div class="bills">
          {#each bills as v, i (v)}
            <button type="button" class="bill" class:on={receivedCents === v} disabled={locked} onclick={() => (received = toPesos(v))}>{billLabel(v, i)}</button>
          {/each}
        </div>
        <div class="change" class:short={short > 0} class:zero={!short && !change}>
          <span>{short > 0 ? 'Faltan' : 'Cambio'}</span>
          <strong>{money(short > 0 ? short : change)}</strong>
        </div>
      {/if}

      {#if method === 'transfer'}
        <p class="note">Confirma que la transferencia de <b>{money(due)}</b> ya llegó a la cuenta.</p>
        <label class="field">Folio SPEI o últimos dígitos (opcional)
          <input bind:value={reference} maxlength="100" placeholder="Ej. 4821 o clave de rastreo" disabled={locked} />
        </label>
      {/if}
    {/if}

    {#if error}<p class="error" role="alert">{error}</p>{/if}

    <div class="acts">
      <button type="button" class="btn ghost" disabled={locked} onclick={() => (open = false)}>Cancelar</button>
      <button class="btn lg" disabled={locked}>
        {busy ? (phase === 'waiting' ? 'Esperando terminal…' : 'Cobrando…') : usesTerminal ? 'Cobrar en terminal' : 'Confirmar cobro'}
        <kbd>F12</kbd>
      </button>
    </div>
  </form>
</Modal>

<style>
  .pay { display: grid; gap: 0.9rem; }
  .pay .field { margin: 0; }
  .total { display: grid; justify-items: center; gap: 2px; padding: 1rem; border-radius: var(--radius-sm); background: var(--accent-soft); color: var(--accent); }
  .total span { font-size: 0.85rem; font-weight: 600; }
  .total strong { font-family: var(--font-num, inherit); font-size: 2.4rem; line-height: 1.1; font-variant-numeric: tabular-nums; }
  .total small { color: var(--muted); }

  .methods { display: grid; grid-template-columns: repeat(auto-fit, minmax(110px, 1fr)); gap: 8px; }
  .method {
    display: grid; justify-items: center; gap: 4px; min-height: 64px; padding: 0.6rem 0.4rem; font: inherit; font-weight: 600; color: var(--text);
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer;
    transition: border-color var(--dur-ui, 0.2s) var(--ease), background-color var(--dur-ui, 0.2s) var(--ease), transform var(--dur-press, 0.15s) var(--ease);
  }
  .method:hover:not(:disabled) { border-color: var(--muted); }
  .method:active:not(:disabled) { transform: scale(0.97); }
  .method.on { background: var(--accent); border-color: var(--accent); color: var(--accent-contrast); }
  .method:disabled { opacity: 0.6; cursor: not-allowed; }

  .big { min-height: 56px; font-size: 1.4rem; font-variant-numeric: tabular-nums; }
  .bills { display: flex; flex-wrap: wrap; gap: 8px; }
  .bill { min-height: 44px; padding: 0 1rem; font: inherit; font-weight: 600; font-variant-numeric: tabular-nums; color: var(--text); background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; }
  .bill.on { background: var(--accent); border-color: var(--accent); color: var(--accent-contrast); }
  .change { display: flex; justify-content: space-between; align-items: baseline; padding: 0.7rem 1rem; border-radius: var(--radius-sm); background: var(--ok-soft); color: var(--ok); font-weight: 600; }
  .change strong { font-size: 1.5rem; font-variant-numeric: tabular-nums; }
  .change.short { background: var(--danger-soft); color: var(--danger); }
  .change.zero { background: var(--surface-2); color: var(--muted); }

  .point { display: grid; gap: 4px; justify-items: start; padding: 0.75rem 1rem; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface-2); font-size: 0.92rem; }
  .point.ok { border-color: var(--ok); background: var(--ok-soft); }
  .point.bad { border-color: var(--danger); background: var(--danger-soft); }
  .note { margin: 0; color: var(--muted); font-size: 0.92rem; }
  .note b { color: var(--text); }

  .acts { display: flex; justify-content: flex-end; gap: 10px; flex-wrap: wrap; }
  .acts .btn.lg { flex: 1; min-width: 200px; gap: 10px; }
  kbd { padding: 1px 6px; font: inherit; font-size: 0.75rem; border: 1px solid currentColor; border-radius: 4px; opacity: 0.7; }
  @media (pointer: coarse) { kbd { display: none; } }
</style>
