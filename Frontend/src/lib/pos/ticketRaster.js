// Imprime el MISMO diseño que ves en la ventana del ticket, pero directo en la térmica.
// Carga la vista del ticket en un iframe oculto, la convierte en imagen y la manda como bitmap ESC/POS.
import { toCanvas } from 'html-to-image';

const PAPER_SELECTOR = '.tk-paper';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** Repite `fn` hasta que devuelva algo (o lance error) o se acabe el tiempo. */
async function waitFor(fn, { timeout = 15000, step = 80, message = 'El ticket tardó demasiado en cargar.' } = {}) {
  const t0 = Date.now();
  for (;;) {
    const v = fn();
    if (v) return v;
    if (Date.now() - t0 > timeout) throw new Error(message);
    await sleep(step);
  }
}

/**
 * Abre `path` (una ruta de la app, por ejemplo /imprimir/venta/12) en un iframe oculto
 * y devuelve un canvas con el ticket del ancho exacto de la impresora (`dots` puntos).
 */
export async function renderTicketCanvas(path, { paper = '80', dots = 512 } = {}) {
  const iframe = document.createElement('iframe');
  iframe.setAttribute('aria-hidden', 'true');
  iframe.tabIndex = -1;
  iframe.style.cssText = 'position:fixed;left:-10000px;top:0;width:900px;height:1600px;border:0;opacity:0;pointer-events:none;';
  // capture=1 avisa a la vista que no imprima ni se cierre sola (la usa esta ventana)
  iframe.src = `${path}${path.includes('?') ? '&' : '?'}capture=1&paper=${paper}`;

  const loaded = new Promise((resolve, reject) => {
    iframe.addEventListener('load', resolve, { once: true });
    iframe.addEventListener('error', () => reject(new Error('No se pudo preparar el ticket.')), { once: true });
  });
  document.body.appendChild(iframe);

  try {
    await loaded;
    const doc = iframe.contentDocument;
    if (!doc) throw new Error('No se pudo leer el ticket.');

    // Espera a que la vista termine de cargar sus datos
    const el = await waitFor(() => {
      const err = doc.querySelector('.tk-state.err');
      if (err) throw new Error(err.textContent.trim() || 'No se pudo cargar el ticket.');
      const paperEl = doc.querySelector(PAPER_SELECTOR);
      if (!paperEl || paperEl.querySelector('.tk-state')) return null;
      return paperEl;
    });

    await doc.fonts?.ready;
    await sleep(150);

    const width = el.offsetWidth;
    if (!width) throw new Error('El ticket no tiene tamaño.');

    const raw = await toCanvas(el, {
      pixelRatio: dots / width,
      backgroundColor: '#ffffff',
      style: { margin: '0', boxShadow: 'none', transform: 'none' }
    });

    // Ancho exacto de la impresora
    const out = document.createElement('canvas');
    out.width = dots;
    out.height = Math.max(1, Math.round((raw.height * dots) / raw.width));
    const ctx = out.getContext('2d');
    ctx.fillStyle = '#fff';
    ctx.fillRect(0, 0, out.width, out.height);
    ctx.drawImage(raw, 0, 0, out.width, out.height);
    return out;
  } finally {
    iframe.remove();
  }
}

/** Canvas → bitmap de 1 bit (negro / blanco) listo para ESC/POS. Quita el blanco sobrante al final. */
export function canvasToBitmap(canvas, { threshold = 180 } = {}) {
  const { width, height } = canvas;
  const px = canvas.getContext('2d', { willReadFrequently: true }).getImageData(0, 0, width, height).data;
  const bytesPerRow = Math.ceil(width / 8);
  const data = new Uint8Array(bytesPerRow * height);
  let lastInk = 0;
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const i = (y * width + x) * 4;
      const a = px[i + 3] / 255;
      const lum = (0.299 * px[i] + 0.587 * px[i + 1] + 0.114 * px[i + 2]) * a + 255 * (1 - a);
      if (lum < threshold) {
        data[y * bytesPerRow + (x >> 3)] |= 0x80 >> (x & 7);
        lastInk = y;
      }
    }
  }
  return { width, height: Math.min(height, lastInk + 12), bytesPerRow, data };
}

export async function renderTicketBitmap(path, opts = {}) {
  return canvasToBitmap(await renderTicketCanvas(path, opts));
}

/** Bytes ESC/POS: imagen por bandas (GS v 0), avance y corte. */
export function buildRasterTicket(bitmap, { openDrawer = false, feed = 4 } = {}) {
  const parts = [];
  const put = (...b) => parts.push(Uint8Array.from(b));

  put(0x1b, 0x40); // init
  if (openDrawer) put(0x1b, 0x70, 0x00, 0x19, 0xfa); // pulso al cajón
  put(0x1b, 0x61, 0x01); // centrado, por si la impresora es más ancha que la imagen

  const BAND = 128; // filas por comando: las impresoras con poca memoria lo agradecen
  for (let y = 0; y < bitmap.height; y += BAND) {
    const h = Math.min(BAND, bitmap.height - y);
    put(0x1d, 0x76, 0x30, 0x00, bitmap.bytesPerRow & 0xff, bitmap.bytesPerRow >> 8, h & 0xff, h >> 8);
    parts.push(bitmap.data.subarray(y * bitmap.bytesPerRow, (y + h) * bitmap.bytesPerRow));
  }

  put(0x1b, 0x61, 0x00, 0x1b, 0x64, feed, 0x1d, 0x56, 0x42, 0x03); // izquierda, avance, corte parcial

  const total = parts.reduce((n, p) => n + p.length, 0);
  const bytes = new Uint8Array(total);
  let off = 0;
  for (const p of parts) {
    bytes.set(p, off);
    off += p.length;
  }
  return bytes;
}
