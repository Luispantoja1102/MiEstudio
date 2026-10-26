// Dónde se imprime lo que se cobra en ESTE dispositivo (se guarda en el navegador, como la impresora).
//  - local:   en este mismo dispositivo (la impresora térmica configurada, o el navegador)
//  - remote:  se manda a la «estación de impresión» (la computadora que tiene la impresora)
//  - station: esta computadora recibe e imprime los tickets que mandan los demás dispositivos
//
// La estación pide trabajos con una petición larga: el servidor contesta en cuanto llega uno, así el ticket sale al
// instante aunque la pestaña esté en segundo plano (los temporizadores del navegador se frenan ahí, las respuestas no).
import { api } from '$lib/api.js';
import { branches, loadBranches } from '$lib/branch.svelte.js';
import { newToken } from '$lib/format.js';
import { session } from '$lib/session.svelte.js';
import { directPrinting, printReceiptDirect } from './printer.svelte.js';

const KEY = 'barbella_print_route';

function read() {
  try {
    const raw = JSON.parse(localStorage.getItem(KEY) || '{}');
    return {
      mode: ['local', 'remote', 'station'].includes(raw.mode) ? raw.mode : 'local',
      stationId: typeof raw.stationId === 'string' && raw.stationId ? raw.stationId : newToken()
    };
  } catch {
    return { mode: 'local', stationId: newToken() };
  }
}

export const printRoute = $state(typeof localStorage === 'undefined' ? { mode: 'local', stationId: newToken() } : read());

export function savePrintRoute(patch = {}) {
  Object.assign(printRoute, patch);
  try {
    localStorage.setItem(KEY, JSON.stringify({ mode: printRoute.mode, stationId: printRoute.stationId }));
  } catch {
    /* sin almacenamiento: queda mientras la pestaña esté abierta */
  }
  syncPrintStation();
}

const SELL = ['admin', 'cashier', 'reception'];
const tz = () => session.me?.business?.timezone;

// ── estación: recibe los tickets de los otros dispositivos
export const stationState = $state({ running: false, waiting: false, printed: 0, lastTicketAt: null, error: '' });
let generation = 0;
const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const shouldRun = () => printRoute.mode === 'station' && SELL.includes(session.me?.role);
const stationName = () => `Computadora ${String(printRoute.stationId).slice(0, 4)}`;

async function handle(job) {
  const send = (ok, error) => api(`/api/print/jobs/${job.id}/result`, { method: 'POST', body: { station_id: printRoute.stationId, ok, error } }).catch(() => {});
  if (!directPrinting()) {
    await send(false, 'Esta computadora no tiene la impresora térmica configurada (Ajustes → Impresora y ticket).');
    stationState.error = 'Llegó un ticket pero esta computadora no tiene impresora térmica configurada.';
    return;
  }
  try {
    await printReceiptDirect(job.sale, { openDrawer: job.open_drawer, tz: tz() });
    await send(true);
    stationState.printed += 1;
    stationState.lastTicketAt = Date.now();
    stationState.error = '';
  } catch (error) {
    await send(false, error?.message || 'La impresora no respondió.');
    stationState.error = `No se pudo imprimir un ticket: ${error?.message || 'la impresora no respondió'}`;
  }
}

async function loop(gen) {
  stationState.running = true;
  while (gen === generation && shouldRun()) {
    try {
      if (!branches.id) await loadBranches();
      stationState.waiting = true;
      const { job } = await api('/api/print/claim', { method: 'POST', body: { station_id: printRoute.stationId, name: stationName(), branch_id: branches.id } });
      stationState.waiting = false;
      if (gen !== generation) break;
      if (job) await handle(job);
    } catch (error) {
      stationState.waiting = false;
      if (error?.status === 401 || error?.status === 403) {
        stationState.error = 'La sesión terminó. Vuelve a entrar para seguir recibiendo tickets.';
        break;
      }
      stationState.error = 'Sin conexión con el servidor. Reintentando…';
      await wait(5000);
    }
  }
  if (gen === generation) {
    stationState.running = false;
    stationState.waiting = false;
  }
}

/** Arranca o detiene la estación según la configuración y la sesión. */
export function syncPrintStation() {
  if (shouldRun()) {
    if (!stationState.running) loop(++generation);
  } else if (stationState.running) {
    generation += 1;
    stationState.running = false;
    stationState.waiting = false;
  }
}

// ── imprimir una venta ya cobrada
export const printing = $state({ busy: false, issue: '' });
let lastSale = null;

export const ticketUrl = (sale, autoprint = false) => `/imprimir/venta/${sale.id}${autoprint ? '?autoprint=1' : ''}`;
export const printInBrowser = (sale) => window.open(ticketUrl(sale, true), '_blank', 'noopener');
/** ¿Imprime sola (térmica o estación)? Con el navegador hay que pedir el ticket. */
export const autoPrints = () => printRoute.mode === 'remote' || directPrinting();

// Manda el ticket a la computadora de la caja y espera hasta 10 s a que confirme que imprimió
async function sendToStation(sale, cash) {
  try {
    const sent = await api('/api/print/jobs', { method: 'POST', body: { sale_id: sale.id, open_drawer: cash } });
    for (let i = 0; i < 10; i += 1) {
      await wait(1000);
      const state = await api(`/api/print/jobs/${sent.id}`);
      if (state.status === 'done') return;
      if (state.status === 'failed') return (printing.issue = state.error || 'La computadora no pudo imprimir el ticket.');
    }
    printing.issue = 'La computadora de la caja no confirmó la impresión. Revisa que tenga Barbella abierto y la impresora lista.';
  } catch (error) {
    printing.issue =
      error.code === 'STATION_OFFLINE' ? 'La computadora de la caja no está conectada (debe tener Barbella abierto).' : error.message || 'No se pudo mandar el ticket a la computadora.';
  }
}

/** Imprime el ticket según este dispositivo. Los errores quedan en printing.issue (la venta ya está registrada). */
export async function printSale(sale) {
  if (!autoPrints()) return;
  lastSale = sale;
  printing.issue = '';
  printing.busy = true;
  const cash = sale.payments.some((p) => p.method === 'cash');
  try {
    if (printRoute.mode === 'remote') await sendToStation(sale, cash);
    else await printReceiptDirect(sale, { openDrawer: cash, tz: tz() });
  } catch (error) {
    printing.issue = error.message || 'La impresora no respondió.';
  } finally {
    printing.busy = false;
  }
}

export const retryPrint = () => lastSale && printSale(lastSale);
export const clearPrintIssue = () => {
  printing.issue = '';
  lastSale = null;
};
