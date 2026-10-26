// Impresión directa en térmica (sin el diálogo del navegador).
// Se configura por dispositivo (cada caja tiene su impresora): Ajustes → Impresora y ticket.
//  - usb:       WebUSB (Chrome PC y Android con OTG). En Windows la impresora debe usar el driver WinUSB.
//  - serial:    Web Serial (Chrome/Edge PC) para impresoras USB que aparecen como puerto COM.
//  - bluetooth: Web Bluetooth (Chrome Android/PC) para impresoras Bluetooth LE.
//
// Estilo del ticket (printer.style):
//  - "image": imprime el mismo diseño de la ventana del ticket (se convierte en imagen). Es el valor por defecto.
//  - "text":  ticket en texto plano ESC/POS (más rápido, sin diseño).
import { METHODS } from '$lib/format.js';
import { buildCashClose, buildReceipt, buildTestPage, money } from './escpos.js';
import { buildRasterTicket, renderTicketBitmap } from './ticketRaster.js';

const KEY = 'barbella_printer';
const PRINT_TIMEOUT_MS = 8000;

// Servicios BLE de las impresoras térmicas genéricas más comunes
const BT_SERVICES = [
  '000018f0-0000-1000-8000-00805f9b34fb',
  'e7810a71-73ae-499d-8c15-faa9aef0c3f2',
  '49535343-fe7d-4ae5-8fa9-9fafd205e455',
  '0000ff00-0000-1000-8000-00805f9b34fb',
  '0000fee7-0000-1000-8000-00805f9b34fb'
];

const DEFAULTS = { mode: 'browser', paper: '80', drawer: false, baudRate: 9600, deviceName: '', portInfo: null, style: 'image', dots: 0 };

function readSettings() {
  try {
    const raw = JSON.parse(localStorage.getItem(KEY) || '{}');
    return {
      mode: ['browser', 'usb', 'serial', 'bluetooth'].includes(raw.mode) ? raw.mode : DEFAULTS.mode,
      paper: raw.paper === '58' ? '58' : '80',
      drawer: Boolean(raw.drawer),
      baudRate: Number(raw.baudRate) || 9600,
      deviceName: String(raw.deviceName || ''),
      portInfo: raw.portInfo && typeof raw.portInfo === 'object' ? raw.portInfo : null,
      style: raw.style === 'text' ? 'text' : 'image',
      dots: Number(raw.dots) || 0 // 0 = automático (80 mm → 512, 58 mm → 384)
    };
  } catch {
    return { ...DEFAULTS };
  }
}

export const printer = $state({ ...DEFAULTS, connected: false, busy: false });
if (typeof localStorage !== 'undefined') Object.assign(printer, readSettings());

export function savePrinterSettings(patch = {}) {
  const modeChanged = patch.mode && patch.mode !== printer.mode;
  Object.assign(printer, patch);
  if (modeChanged) disconnect();
  try {
    const { mode, paper, drawer, baudRate, deviceName, portInfo, style, dots } = printer;
    localStorage.setItem(KEY, JSON.stringify({ mode, paper, drawer, baudRate, deviceName, portInfo, style, dots }));
  } catch {
    /* sin almacenamiento: queda mientras la pestaña esté abierta */
  }
}

export const directPrinting = () => printer.mode !== 'browser';

export function printerSupport() {
  const nav = typeof navigator === 'undefined' ? {} : navigator;
  return { usb: 'usb' in nav, serial: 'serial' in nav, bluetooth: 'bluetooth' in nav };
}

export const MODE_LABEL = {
  browser: 'Navegador',
  usb: 'Térmica USB',
  serial: 'Térmica USB (puerto COM)',
  bluetooth: 'Térmica Bluetooth'
};

// —— Conexión ——
let conn = null; // { write(bytes), close() }

function disconnect() {
  const c = conn;
  conn = null;
  printer.connected = false;
  if (c) c.close().catch(() => {});
}

function friendly(error) {
  const name = error?.name || '';
  if (name === 'NotFoundError') return 'No elegiste ninguna impresora.';
  if (name === 'SecurityError') return 'El navegador no dio permiso para usar la impresora.';
  if (name === 'NetworkError') return 'Se perdió la conexión con la impresora. Revisa que esté prendida y cerca.';
  if (name === 'InvalidStateError') return 'La impresora está ocupada por otra pestaña o programa.';
  return error?.message || 'La impresora no respondió.';
}

async function openUsb(prompt) {
  // Sin filtro: muchas térmicas genéricas no se anuncian como clase «impresora»
  const device = prompt ? await navigator.usb.requestDevice({ filters: [] }) : (await navigator.usb.getDevices())[0];
  if (!device) return null;
  await device.open();
  if (!device.configuration) await device.selectConfiguration(1);
  let target = null;
  for (const iface of device.configuration.interfaces) {
    for (const alt of iface.alternates) {
      const out = alt.endpoints.find((e) => e.direction === 'out' && e.type === 'bulk');
      if (out && (!target || alt.interfaceClass === 7)) target = { iface, alt, out };
    }
  }
  if (!target) {
    await device.close().catch(() => {});
    throw new Error('Ese dispositivo USB no parece una impresora.');
  }
  await device.claimInterface(target.iface.interfaceNumber);
  if (target.iface.alternates.length > 1) {
    await device.selectAlternateInterface(target.iface.interfaceNumber, target.alt.alternateSetting).catch(() => {});
  }
  return {
    name: device.productName || 'Impresora USB',
    async write(bytes) {
      for (let i = 0; i < bytes.length; i += 4096) {
        await device.transferOut(target.out.endpointNumber, bytes.slice(i, i + 4096));
      }
    },
    close: () => device.close()
  };
}

const serialQueue = new WeakMap();

async function openSerial(prompt) {
  // Recordar el puerto de la impresora para no confundirlo con otro dispositivo serial
  let port;
  if (prompt) {
    port = await navigator.serial.requestPort();
    savePrinterSettings({ portInfo: port.getInfo?.() || null });
  } else {
    const saved = printer.portInfo;
    const ports = await navigator.serial.getPorts();
    port = saved
      ? ports.find((p) => {
          const i = p.getInfo?.() || {};
          return i.usbVendorId === saved.usbVendorId && i.usbProductId === saved.usbProductId;
        })
      : ports.length === 1
        ? ports[0]
        : null;
  }
  if (!port) return null;
  if (!port.writable) await port.open({ baudRate: printer.baudRate || 9600 });
  const info = port.getInfo?.() || {};
  return {
    name: info.usbProductId ? `Puerto COM (USB ${info.usbVendorId?.toString(16)}:${info.usbProductId.toString(16)})` : 'Puerto COM',
    // Un solo escritor a la vez: dos impresiones simultáneas dejaban el stream bloqueado
    write(bytes) {
      const run = async () => {
        const writer = port.writable.getWriter();
        try {
          await writer.write(bytes);
        } finally {
          try {
            writer.releaseLock();
          } catch {
            /* escritura pendiente: se libera sola */
          }
        }
      };
      const next = (serialQueue.get(port) || Promise.resolve()).then(run, run);
      serialQueue.set(port, next.catch(() => {}));
      return next;
    },
    close: async () => {
      await (serialQueue.get(port) || Promise.resolve());
      return port.close();
    }
  };
}

let btDevice = null;
async function openBluetooth(prompt) {
  let device = btDevice;
  if (!device && !prompt && navigator.bluetooth.getDevices) {
    device = (await navigator.bluetooth.getDevices()).find((d) => d.name === printer.deviceName) || null;
  }
  if (!device && prompt) {
    device = await navigator.bluetooth.requestDevice({ acceptAllDevices: true, optionalServices: BT_SERVICES });
  }
  if (!device) return null;
  btDevice = device;
  const server = device.gatt.connected ? device.gatt : await device.gatt.connect();
  let ch = null;
  for (const uuid of BT_SERVICES) {
    let service;
    try {
      service = await server.getPrimaryService(uuid);
    } catch {
      continue;
    }
    for (const c of await service.getCharacteristics()) {
      if (c.properties.writeWithoutResponse || c.properties.write) {
        ch = c;
        break;
      }
    }
    if (ch) break;
  }
  if (!ch) {
    device.gatt.disconnect();
    throw new Error('Esa impresora Bluetooth no es compatible (no encontré por dónde mandarle el ticket).');
  }
  device.addEventListener(
    'gattserverdisconnected',
    () => {
      if (conn?.bt === device) {
        conn = null;
        printer.connected = false;
      }
    },
    { once: true }
  );
  return {
    bt: device,
    name: device.name || 'Impresora Bluetooth',
    async write(bytes) {
      // BLE manda pocos bytes por paquete
      for (let i = 0; i < bytes.length; i += 180) {
        const chunk = bytes.slice(i, i + 180);
        if (ch.properties.writeWithoutResponse) await ch.writeValueWithoutResponse(chunk);
        else await ch.writeValue(chunk);
      }
    },
    async close() {
      device.gatt.disconnect();
    }
  };
}

/**
 * Conecta con la impresora del modo elegido.
 * prompt=true abre el selector del navegador (solo desde un clic); false reusa la que ya tiene permiso.
 */
export async function connectPrinter({ prompt = false } = {}) {
  if (conn) return conn;
  const mode = printer.mode;
  if (mode === 'browser') throw new Error('La impresión directa está apagada.');
  if (!printerSupport()[mode]) throw new Error(`Este navegador no puede usar «${MODE_LABEL[mode]}». Usa Chrome o Edge.`);
  try {
    const opened = mode === 'usb' ? await openUsb(prompt) : mode === 'serial' ? await openSerial(prompt) : await openBluetooth(prompt);
    if (!opened) throw new Error('No hay impresora conectada. Conéctala en Ajustes → Impresora y ticket.');
    conn = opened;
    printer.connected = true;
    if (prompt) savePrinterSettings({ deviceName: opened.name });
    return conn;
  } catch (error) {
    printer.connected = false;
    throw new Error(friendly(error));
  }
}

function withTimeout(promise, ms) {
  let timer;
  return Promise.race([
    promise,
    new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error('La impresora no respondió a tiempo. Revisa que esté prendida y con papel.')), ms);
    })
  ]).finally(() => clearTimeout(timer));
}

/** Manda bytes a la impresora; si la conexión se cayó, reconecta una vez. */
export async function printBytes(bytes, { prompt = false, timeoutMs = PRINT_TIMEOUT_MS } = {}) {
  printer.busy = true;
  try {
    await withTimeout(
      (async () => {
        try {
          await (await connectPrinter({ prompt })).write(bytes);
        } catch (first) {
          if (!conn) throw first;
          disconnect();
          await (await connectPrinter({ prompt: false })).write(bytes);
        }
      })(),
      timeoutMs
    );
  } catch (error) {
    disconnect();
    throw new Error(friendly(error));
  } finally {
    printer.busy = false;
  }
}

// 80 mm = 42 columnas (58 mm = 32)
const cols = () => (printer.paper === '58' ? 32 : 42);

// Ancho de la imagen en puntos. 512 sirve en 80 mm de casi todas; si tu impresora es de 576, usa savePrinterSettings({ dots: 576 })
const dotsWide = () => printer.dots || (printer.paper === '58' ? 384 : 512);

// Una imagen es mucho más grande que un ticket de texto: por Bluetooth tarda bastante más
const imageTimeout = (bytes) => 15000 + (printer.mode === 'bluetooth' ? Math.ceil(bytes.length / 4000) * 1000 : 0);

/**
 * Convierte la vista del ticket `path` en bytes de imagen para la térmica.
 * Devuelve null si el estilo es «texto» o si no se pudo generar (así se imprime en texto y no se pierde el ticket).
 */
async function designBytes(path, { openDrawer = false } = {}) {
  if (printer.style !== 'image') return null;
  printer.busy = true;
  try {
    const bitmap = await renderTicketBitmap(path, { paper: printer.paper, dots: dotsWide() });
    return buildRasterTicket(bitmap, { openDrawer });
  } catch (error) {
    console.warn('No se pudo armar el ticket con diseño, se imprime en texto:', error);
    return null;
  } finally {
    printer.busy = false;
  }
}

export function printTestPage({ shop = 'Barbella', prompt = false } = {}) {
  return printBytes(buildTestPage({ cols: cols(), shop, openDrawer: printer.drawer }), { prompt });
}

// —— Datos del ticket (los mismos que muestra lib/pos/Receipt.svelte) ——
const fmtParts = (iso, tz) =>
  new Intl.DateTimeFormat('es-MX', { timeZone: tz, day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })
    .format(new Date(iso))
    .replace(/\./g, '')
    .replace(',', '');

/** Venta (como la entrega /api/sales) → datos del ticket en pesos. */
export function receiptData(sale, tz = 'America/Tijuana') {
  const pesos = (c) => (c || 0) / 100;
  const units = sale.items.reduce((n, i) => n + i.qty, 0);
  const void_ = sale.status === 'void';
  return {
    shop: sale.business_name,
    branch: sale.branch_name,
    address: sale.branch_address,
    phone: sale.branch_phone,
    folio: sale.folio,
    dateText: fmtParts(sale.created_at, tz),
    cashier: sale.cashier,
    client: sale.client_name,
    countText: `${units} ${units === 1 ? 'artículo' : 'artículos'}`,
    items: sale.items.map((i) => ({ name: i.name, notes: i.staff_name, qtyText: String(i.qty), unit: pesos(i.unit_price_cents), amount: pesos(i.total_cents) })),
    subtotal: pesos(sale.subtotal_cents),
    discount: pesos(sale.discount_cents),
    tip: pesos(sale.tip_cents),
    tipFor: sale.tip_staff_name,
    total: pesos(sale.total_cents),
    paid: !void_,
    payments: sale.payments.flatMap((p) => {
      const rows = [[`${METHODS[p.method] || p.method}${p.reference ? ` · ${p.reference}` : ''}`, money(pesos(p.amount_cents))]];
      if (p.method === 'cash' && p.received_cents && p.received_cents > p.amount_cents) rows.push(['  Recibido', money(pesos(p.received_cents))]);
      return rows;
    }),
    change: pesos(sale.change_cents),
    stamp: void_ ? 'CANCELADA' : 'PAGADO',
    voidReason: void_ ? sale.void_reason : '',
    thanks: '¡Gracias por tu visita!'
  };
}

/** Corte de caja (como lo entrega /api/cash/sessions/{id}) → datos del ticket en pesos. */
export function cashCloseData(r, tz = 'America/Tijuana') {
  const pesos = (c) => (c || 0) / 100;
  const part = (iso, opts) => new Intl.DateTimeFormat('es-MX', { timeZone: tz, hour12: false, ...opts }).format(new Date(iso));
  const clock = (iso) => part(iso, { hour: '2-digit', minute: '2-digit' });
  const closed = Boolean(r.closed_at);
  const diff = pesos(r.diff_cents);
  return {
    shop: r.business_name,
    branch: r.branch_name,
    address: r.branch_address,
    phone: r.branch_phone,
    closed,
    turn: r.id,
    dateText: part(r.opened_at, { day: '2-digit', month: 'short', year: 'numeric' }).replace(/\./g, ''),
    openedClock: clock(r.opened_at),
    openedBy: r.opened_by,
    closedClock: closed ? clock(r.closed_at) : '',
    closedBy: r.closed_by,
    sales: r.sales,
    soldAmount: pesos(r.sales_cents),
    tips: pesos(r.tip_cents),
    average: r.sales ? pesos(Math.round(r.sales_cents / r.sales)) : 0,
    voidedCount: r.voided_count,
    voidedAmount: pesos(r.voided_cents),
    methods: r.by_method.map((m) => [METHODS[m.method] || m.method, pesos(m.amount_cents)]),
    collected: pesos(r.by_method.reduce((n, m) => n + m.amount_cents, 0)),
    opening: pesos(r.opening_cents),
    cashIn: pesos(r.cash_in_cents),
    movesIn: pesos(r.moves_in_cents),
    movesOut: pesos(r.moves_out_cents),
    expected: pesos(r.expected_cents),
    counted: pesos(r.counted_cents),
    diffAmount: diff,
    verdict: diff === 0 ? 'Cuadra' : diff > 0 ? 'Sobrante' : 'Faltante',
    movements: (r.movements || []).map((m) => [`${clock(m.created_at)} ${m.concept}`, `${m.kind === 'in' ? '+' : '-'}${money(pesos(m.amount_cents))}`]),
    note: r.note,
    tickets: (r.tickets || []).map((t) => [`${clock(t.created_at)} #${t.folio}${t.status === 'void' ? ' (cancelada)' : ''}`, money(pesos(t.total_cents + t.tip_cents))]),
    printedText: part(new Date().toISOString(), { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' }).replace(/\./g, '')
  };
}

/** Imprime el corte en la térmica: con diseño (imagen de la vista /imprimir/corte/[id]) o, si falla, en texto. */
export async function printCashCloseDirect(report, { tz } = {}) {
  const bytes = await designBytes(`/imprimir/corte/${report.id}`);
  if (bytes) return printBytes(bytes, { timeoutMs: imageTimeout(bytes) });
  return printBytes(buildCashClose(cashCloseData(report, tz), { cols: cols() }));
}

/**
 * Imprime el ticket de una venta en la térmica. openDrawer abre el cajón (pago en efectivo).
 * Con estilo «image» sale con el mismo diseño de la ventana del ticket; si eso falla, sale en texto.
 */
export async function printReceiptDirect(sale, { openDrawer = false, tz } = {}) {
  const drawer = openDrawer && printer.drawer;
  const bytes = await designBytes(`/imprimir/venta/${sale.id}`, { openDrawer: drawer });
  if (bytes) return printBytes(bytes, { timeoutMs: imageTimeout(bytes) });
  return printBytes(buildReceipt(receiptData(sale, tz), { cols: cols(), openDrawer: drawer }));
}
