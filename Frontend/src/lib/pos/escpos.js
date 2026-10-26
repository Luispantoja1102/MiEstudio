// Ticket en comandos ESC/POS para impresoras térmicas (58 / 80 mm). Sin imports: se puede probar suelto.

const ESC = 0x1b;
const GS = 0x1d;

// Página de códigos PC850 (ESC t 2): la que traen casi todas las térmicas genéricas y tiene acentos y ñ
const PC850 = {
  á: 0xa0, é: 0x82, í: 0xa1, ó: 0xa2, ú: 0xa3, ñ: 0xa4, Ñ: 0xa5, ü: 0x81, Ü: 0x9a,
  Á: 0xb5, É: 0x90, Í: 0xd6, Ó: 0xe0, Ú: 0xe9, '¿': 0xa8, '¡': 0xad, '°': 0xf8, º: 0xa7, ª: 0xa6
};
const LOOKALIKE = {
  '×': 'x', '−': '-', '–': '-', '—': '-', '·': '-', '•': '-', '★': '*', '“': '"', '”': '"', '‘': "'", '’': "'",
  '«': '"', '»': '"', '…': '...', ' ': ' ', '\t': ' '
};

/** Texto a bytes PC850; lo que no existe se cambia por su parecido o se le quita el acento. */
export function encodeText(text) {
  const out = [];
  for (const ch of String(text ?? '')) {
    const code = ch.charCodeAt(0);
    if (code >= 0x20 && code < 0x7f) out.push(code);
    else if (ch === '\n') out.push(0x0a);
    else if (PC850[ch] != null) out.push(PC850[ch]);
    else if (LOOKALIKE[ch] != null) for (const c of LOOKALIKE[ch]) out.push(c.charCodeAt(0));
    else {
      const plain = ch.normalize('NFD').replace(/[̀-ͯ]/g, '');
      const c = plain.charCodeAt(0);
      out.push(plain && c >= 0x20 && c < 0x7f ? c : 0x3f);
    }
  }
  return out;
}

const visibleLength = (text) => encodeText(text).length;

/** Corta un texto en renglones de `cols` caracteres sin partir palabras si se puede. */
export function wrap(text, cols) {
  const words = String(text ?? '').split(/\s+/).filter(Boolean);
  const lines = [];
  let cur = '';
  for (let word of words) {
    while (visibleLength(word) > cols) {
      if (cur) {
        lines.push(cur);
        cur = '';
      }
      lines.push(word.slice(0, cols));
      word = word.slice(cols);
    }
    if (!cur) cur = word;
    else if (visibleLength(cur) + 1 + visibleLength(word) <= cols) cur += ` ${word}`;
    else {
      lines.push(cur);
      cur = word;
    }
  }
  if (cur) lines.push(cur);
  return lines.length ? lines : [''];
}

/** Texto a la izquierda y a la derecha en el mismo renglón (con relleno). */
export function leftRight(left, right, cols, fill = ' ') {
  const r = String(right ?? '');
  const room = Math.max(1, cols - visibleLength(r) - 1);
  const l = wrap(left, room);
  const last = l.pop();
  const gap = Math.max(1, cols - visibleLength(last) - visibleLength(r));
  return [...l, `${last}${fill.repeat(gap)}${r}`];
}

export class EscPos {
  constructor({ cols = 42 } = {}) {
    this.cols = cols;
    this.bytes = [];
  }
  raw(...b) {
    this.bytes.push(...b.flat());
    return this;
  }
  init() {
    return this.raw(ESC, 0x40, ESC, 0x74, 0x02);
  }
  align(where = 'left') {
    return this.raw(ESC, 0x61, where === 'center' ? 1 : where === 'right' ? 2 : 0);
  }
  bold(on = true) {
    return this.raw(ESC, 0x45, on ? 1 : 0);
  }
  /** Tamaño 1 o 2 (alto y ancho). */
  size(n = 1) {
    return this.raw(GS, 0x21, n === 2 ? 0x11 : 0x00);
  }
  text(t = '') {
    return this.raw(encodeText(t));
  }
  line(t = '') {
    return this.text(t).raw(0x0a);
  }
  lines(list) {
    for (const l of list) this.line(l);
    return this;
  }
  wrapped(t, { cols = this.cols } = {}) {
    return this.lines(wrap(t, cols));
  }
  lr(left, right, fill = ' ') {
    return this.lines(leftRight(left, right, this.cols, fill));
  }
  rule(ch = '-') {
    return this.line(ch.repeat(this.cols).slice(0, this.cols));
  }
  feed(n = 1) {
    return this.raw(ESC, 0x64, n);
  }
  /** Pulso para abrir el cajón de dinero (conector 2). */
  drawer() {
    return this.raw(ESC, 0x70, 0x00, 0x19, 0xfa);
  }
  /** Avanza y corta (corte parcial). */
  cut() {
    return this.raw(GS, 0x56, 0x42, 0x03);
  }
  build() {
    return Uint8Array.from(this.bytes);
  }
}

const moneyFmt = new Intl.NumberFormat('es-MX', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
export const money = (n) => `$${moneyFmt.format(Number(n || 0))}`;

/**
 * Arma el ticket de venta. `t` ya trae todo calculado en pesos (ver printer.svelte.js → receiptData):
 * { shop, branch, address, phone, folio, dateText, cashier, client, countText,
 *   items: [{ name, notes, qtyText, unit, amount }], subtotal, discount, tip, tipFor, total,
 *   paid, payments: [[etiqueta, monto]], change, stamp, voidReason, note, thanks }
 */
export function buildReceipt(t, { cols = 42, openDrawer = false } = {}) {
  const p = new EscPos({ cols });
  const half = Math.floor(cols / 2);
  p.init();
  if (openDrawer) p.drawer();

  p.align('center').bold().size(2).wrapped(String(t.shop || 'Barbella').toUpperCase(), { cols: half });
  p.size(1).bold(false);
  if (t.branch) p.line(t.branch);
  if (t.address) p.wrapped(t.address);
  if (t.phone) p.line(`Tel. ${t.phone}`);
  p.align('left').rule('=');
  p.lr(`Venta No. ${t.folio || ''}`, t.dateText || '');
  p.lr(`Atendió: ${t.cashier || '-'}`, t.countText || '');
  if (t.client) p.line(`Cliente: ${t.client}`);
  p.rule();

  for (const item of t.items || []) {
    p.bold().wrapped(item.name).bold(false);
    if (item.notes) p.wrapped(`  ${item.notes}`);
    p.lr(`  ${item.qtyText} x ${money(item.unit)}`, money(item.amount));
  }
  p.rule();

  p.lr('Subtotal', money(t.subtotal));
  if (Number(t.discount)) p.lr('Descuento', `-${money(t.discount)}`);
  p.bold().size(2);
  p.lines(leftRight('TOTAL', money(t.total), half));
  p.size(1).bold(false);
  if (Number(t.tip)) p.lr(t.tipFor ? `Propina (${t.tipFor})` : 'Propina', money(t.tip));

  if (t.paid) {
    p.rule();
    for (const [label, amount] of t.payments || []) p.lr(label, amount);
    if (Number(t.change)) p.bold().lr('Su cambio', money(t.change)).bold(false);
  }

  p.feed(1).align('center').bold().line(`[ ${t.stamp || ''} ]`).bold(false);
  if (t.voidReason) p.wrapped(t.voidReason);
  if (t.thanks) p.feed(1).wrapped(t.thanks);
  if (t.note) p.wrapped(t.note);
  p.feed(1).line('Hecho con Barbella').align('left').feed(3).cut();
  return p.build();
}

/**
 * Corte de caja en texto. `t` ya trae todo en pesos (ver printer.svelte.js → cashCloseData):
 * { shop, branch, address, phone, closed, turn, dateText, openedClock, openedBy, closedClock, closedBy,
 *   sales, soldAmount, tips, average, voidedCount, voidedAmount, methods: [[etiqueta, monto]], collected,
 *   opening, cashIn, movesIn, movesOut, expected, counted, verdict, diffAmount,
 *   movements: [[texto, monto con signo]], note, tickets: [[texto, monto]], printedText }
 */
export function buildCashClose(t, { cols = 42 } = {}) {
  const p = new EscPos({ cols });
  const half = Math.floor(cols / 2);
  const section = (title) => p.feed(1).bold().line(String(title).toUpperCase()).bold(false).rule();
  p.init();

  p.align('center').bold().size(2).wrapped(String(t.shop || 'Barbella').toUpperCase(), { cols: half });
  p.size(1).bold(false);
  if (t.branch) p.line(t.branch);
  if (t.address) p.wrapped(t.address);
  if (t.phone) p.line(`Tel. ${t.phone}`);
  p.align('left').rule('=');
  p.align('center').bold().size(2).wrapped('CORTE DE CAJA', { cols: half });
  p.size(1).line(t.closed ? 'Z - FINAL' : 'X - PARCIAL').bold(false);
  p.align('left').rule('=');

  p.lr('Turno No.', String(t.turn));
  p.lr('Fecha', t.dateText || '');
  p.lr('Abrió', `${t.openedClock} ${t.openedBy || ''}`.trim());
  p.lr(t.closed ? 'Cerró' : 'Corte', t.closed ? `${t.closedClock} ${t.closedBy || ''}`.trim() : 'Caja abierta');

  section('Resumen');
  p.lr('Ventas', String(t.sales));
  p.lr('Vendido (sin propinas)', money(t.soldAmount));
  p.lr('Propinas', money(t.tips));
  if (t.sales) p.lr('Promedio por venta', money(t.average));
  if (t.voidedCount) p.lr(`Canceladas (${t.voidedCount})`, money(t.voidedAmount));

  section('Cobrado por forma de pago');
  for (const [label, amount] of t.methods || []) p.lr(label, money(amount));
  p.rule().bold().size(2);
  p.lines(leftRight('COBRADO', money(t.collected), half));
  p.size(1).bold(false);

  section('Cuadre del cajón');
  p.lr('  Fondo inicial', money(t.opening));
  p.lr('+ Ventas en efectivo', money(t.cashIn));
  if (Number(t.movesIn)) p.lr('+ Entradas', money(t.movesIn));
  if (Number(t.movesOut)) p.lr('- Salidas', `-${money(t.movesOut)}`);
  p.rule().bold().lr('= Debe haber', money(t.expected)).bold(false);
  if (t.closed) {
    p.bold().lr('  Se contó', money(t.counted)).bold(false);
    p.feed(1).align('center').bold().size(2).wrapped(`[ ${String(t.verdict || '').toUpperCase()} ]`, { cols: half });
    p.size(1);
    if (Number(t.diffAmount)) p.line(money(Math.abs(t.diffAmount)));
    p.bold(false).align('left');
  }

  if ((t.movements || []).length) {
    section('Entradas y salidas');
    for (const [label, amount] of t.movements) p.lr(label, amount);
  }
  if (t.note) {
    section('Observaciones');
    p.wrapped(t.note);
  }
  if ((t.tickets || []).length) {
    section(`Ventas del turno - ${t.tickets.length}`);
    for (const [label, amount] of t.tickets) p.lr(label, amount);
  }

  // firmas
  const w = Math.floor((cols - 4) / 2);
  p.feed(3);
  p.line('-'.repeat(w) + '    ' + '-'.repeat(w));
  p.line(`${'Entregó'.padEnd(w)}    Recibió`);
  p.line(String(t.closedBy || t.openedBy || '').toUpperCase().slice(0, w));
  p.feed(1).align('center');
  if (t.printedText) p.line(`Impreso ${t.printedText}`);
  p.line('Hecho con Barbella').align('left').feed(3).cut();
  return p.build();
}

/** Ticket corto para «Imprimir prueba». */
export function buildTestPage({ cols = 42, shop = 'Barbella', openDrawer = false } = {}) {
  const p = new EscPos({ cols });
  p.init();
  if (openDrawer) p.drawer();
  p.align('center').bold().size(2).wrapped('PRUEBA', { cols: Math.floor(cols / 2) }).size(1).bold(false);
  p.wrapped(shop).rule();
  p.align('left').lr('Papel', `${cols >= 42 ? 80 : 58} mm (${cols} col.)`);
  p.lr('Acentos', 'áéíóú ñÑ ¿¡');
  p.lr('Relleno', money(1234.5), '.');
  p.rule().align('center').line('Si lees esto, la impresora quedó lista.');
  p.feed(3).cut();
  return p.build();
}
