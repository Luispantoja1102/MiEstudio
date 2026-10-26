<script>
  import { onDestroy, onMount } from 'svelte';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { session } from '$lib/session.svelte.js';
  import { MODE_LABEL, connectPrinter, printTestPage, printer, printerSupport, savePrinterSettings } from '$lib/pos/printer.svelte.js';
  import { printRoute, savePrintRoute, stationState } from '$lib/pos/printing.svelte.js';

  // Cada dispositivo (caja, celular, tableta) decide aquí cómo imprime; se guarda en este navegador.
  const support = printerSupport();
  let msg = $state('');
  let failed = $state(false);
  let station = $state({ online: false, name: '' });
  let timer;

  const say = (text, bad = false) => ((msg = text), (failed = bad));

  const ROUTES = [
    ['local', 'En este dispositivo', 'Con la impresora térmica conectada a él, o con el navegador.'],
    ['remote', 'En la computadora de la caja', 'Para el celular o la tableta: el ticket se manda a la computadora que tiene la impresora.'],
    ['station', 'Esta computadora imprime los tickets de los demás', 'Déjala con Barbella abierto y la impresora lista: recibe y saca los tickets de los otros dispositivos.']
  ];
  const MODES = ['browser', 'usb', 'serial', 'bluetooth'];
  const HINT = { usb: 'Chrome o Edge en computadora, o Chrome en Android con cable OTG.', serial: 'Chrome o Edge en computadora (impresoras que aparecen como puerto COM).', bluetooth: 'Chrome en Android o computadora (impresoras Bluetooth LE).' };

  async function checkStation() {
    if (!branches.id || printRoute.mode !== 'remote') return;
    station = await api(`/api/print/station?branch_id=${branches.id}`).catch(() => ({ online: false, name: '' }));
  }
  onMount(async () => {
    await loadBranches().catch(() => {});
    checkStation();
    timer = setInterval(checkStation, 5000);
  });
  onDestroy(() => clearInterval(timer));

  function chooseRoute(mode) {
    savePrintRoute({ mode });
    msg = '';
    checkStation();
  }
  function chooseMode(mode) {
    savePrinterSettings({ mode, deviceName: mode === printer.mode ? printer.deviceName : '' });
    msg = '';
  }
  async function choosePrinter() {
    try {
      await connectPrinter({ prompt: true });
      say(`Impresora lista: ${printer.deviceName}`);
    } catch (e) {
      say(e.message, true);
    }
  }
  async function testPrint() {
    try {
      await printTestPage({ shop: session.me.business.name, prompt: !printer.connected && !printer.deviceName });
      say('Se mandó la página de prueba.');
    } catch (e) {
      say(e.message, true);
    }
  }
  const stationLine = $derived(
    stationState.error ? stationState.error : stationState.waiting ? 'Esperando tickets…' : stationState.running ? 'Conectando…' : 'Detenida: inicia sesión para recibir tickets.'
  );
</script>

<div class="page-head">
  <div>
    <h1>Impresora y ticket</h1>
    <p>Cómo sale el ticket en este dispositivo. Se guarda en este navegador.</p>
  </div>
</div>

<section class="card">
  <h2>¿Dónde se imprime?</h2>
  <div class="choices" role="radiogroup" aria-label="Dónde se imprime">
    {#each ROUTES as [id, label, desc]}
      <button class="choice" role="radio" aria-checked={printRoute.mode === id} onclick={() => chooseRoute(id)}>
        <b>{label}</b><small>{desc}</small>
      </button>
    {/each}
  </div>
  {#if printRoute.mode === 'remote'}
    <p class="state" class:on={station.online}><i></i>{station.online ? `Conectada: ${station.name || 'computadora de la caja'}` : 'La computadora de la caja no está conectada. Debe tener Barbella abierto en Ajustes → Impresora y ticket → «Esta computadora imprime los tickets de los demás».'}</p>
  {:else if printRoute.mode === 'station'}
    <p class="state" class:on={stationState.waiting && !stationState.error}><i></i>{stationLine}</p>
    {#if stationState.printed}<p class="meta">Tickets impresos en esta sesión: {stationState.printed}</p>{/if}
  {/if}
</section>

{#if printRoute.mode !== 'remote'}
  <section class="card">
    <h2>Impresora de este dispositivo</h2>
    <div class="choices modes" role="radiogroup" aria-label="Tipo de impresión">
      {#each MODES as m}
        <button class="choice" role="radio" aria-checked={printer.mode === m} disabled={m !== 'browser' && !support[m]} onclick={() => chooseMode(m)}>
          <b>{MODE_LABEL[m]}</b>
          <small>{m === 'browser' ? 'Abre el ticket para imprimirlo con el diálogo normal.' : support[m] ? HINT[m] : 'Este navegador no lo permite.'}</small>
        </button>
      {/each}
    </div>

    {#if printer.mode !== 'browser'}
      <p class="state" class:on={printer.connected}>
        <i></i>{printer.connected ? `Conectada: ${printer.deviceName}` : printer.deviceName ? `Elegida: ${printer.deviceName} (se conecta al imprimir)` : 'Sin impresora elegida'}
      </p>
      <div class="row">
        <div class="seg" role="group" aria-label="Ancho del papel">
          <button aria-pressed={printer.paper === '80'} onclick={() => savePrinterSettings({ paper: '80' })}>Papel 80 mm</button>
          <button aria-pressed={printer.paper === '58'} onclick={() => savePrinterSettings({ paper: '58' })}>Papel 58 mm</button>
        </div>
        <div class="seg" role="group" aria-label="Estilo del ticket">
          <button aria-pressed={printer.style === 'image'} onclick={() => savePrinterSettings({ style: 'image' })}>Con diseño</button>
          <button aria-pressed={printer.style === 'text'} onclick={() => savePrinterSettings({ style: 'text' })}>Solo texto</button>
        </div>
      </div>
      {#if printer.mode === 'serial'}
        <label class="field">Velocidad del puerto
          <select value={printer.baudRate} onchange={(e) => savePrinterSettings({ baudRate: Number(e.currentTarget.value) })}>
            {#each [9600, 19200, 38400, 57600, 115200] as b}<option value={b}>{b}</option>{/each}
          </select>
        </label>
      {/if}
      <label class="check">
        <input type="checkbox" checked={printer.drawer} onchange={(e) => savePrinterSettings({ drawer: e.currentTarget.checked })} />
        Abrir el cajón de dinero al cobrar en efectivo
      </label>
      <div class="form-actions">
        <button class="btn ghost" disabled={printer.busy} onclick={choosePrinter}><Icon name="printer" size={16} />{printer.deviceName ? 'Cambiar impresora' : 'Elegir impresora'}</button>
        <button class="btn" disabled={printer.busy} onclick={testPrint}>{printer.busy ? 'Imprimiendo…' : 'Imprimir prueba'}</button>
      </div>
      {#if msg}<p class={failed ? 'error' : 'ok'} role="status">{msg}</p>{/if}
    {/if}
  </section>
{/if}

<style>
  .card { margin-bottom: 1rem; }
  h2 { margin: 0 0 0.75rem; font-size: 1.15rem; }
  .choices { display: grid; gap: 8px; }
  .choices.modes { grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); }
  .choice { display: grid; gap: 2px; padding: 0.75rem 1rem; text-align: left; font: inherit; color: var(--text); background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; }
  .choice small { color: var(--muted); }
  .choice[aria-checked='true'] { border-color: var(--accent); background: var(--accent-soft); }
  .choice:disabled { opacity: 0.5; cursor: not-allowed; }
  .state { display: flex; align-items: center; gap: 8px; margin: 1rem 0 0.5rem; font-size: 0.92rem; color: var(--muted); }
  .state i { flex: none; width: 10px; height: 10px; border-radius: 50%; background: var(--muted); }
  .state.on { color: var(--ok); }
  .state.on i { background: var(--ok); }
  .row { display: flex; flex-wrap: wrap; gap: 12px; margin: 0.75rem 0; }
  .seg { display: inline-flex; padding: 3px; border: 1px solid var(--border); border-radius: 999px; background: var(--surface-2); }
  .seg button { min-height: 36px; padding: 0 1rem; font: inherit; font-size: 0.85rem; font-weight: 600; color: var(--muted); background: transparent; border: 0; border-radius: 999px; cursor: pointer; }
  .seg button[aria-pressed='true'] { background: var(--accent); color: var(--accent-contrast); }
  .check { display: flex; align-items: center; gap: 10px; margin: 0.75rem 0; font-size: 0.92rem; }
  .ok { color: var(--ok); font-weight: 600; }
</style>
