// Cálculos de fechas y de acomodo para las vistas de la agenda (día, semana, mes). Las fechas son 'AAAA-MM-DD'.
import { addDays } from '$lib/time.js';

const pad = (n) => String(n).padStart(2, '0');

// 0 = domingo
export const weekday = (d) => new Date(d + 'T12:00:00Z').getUTCDay();
// lunes de la semana de d
export const weekStart = (d) => addDays(d, -((weekday(d) + 6) % 7));
export const monthStart = (d) => d.slice(0, 8) + '01';

export function addMonths(d, n) {
  const [y, m, day] = d.split('-').map(Number);
  const t = new Date(Date.UTC(y, m - 1 + n, 1));
  const last = new Date(Date.UTC(t.getUTCFullYear(), t.getUTCMonth() + 1, 0)).getUTCDate();
  return `${t.getUTCFullYear()}-${pad(t.getUTCMonth() + 1)}-${pad(Math.min(day, last))}`;
}

// qué pedirle a la API según la vista: desde qué día y cuántos
export function rangeFor(view, d) {
  if (view === 'week') return { from: weekStart(d), days: 7 };
  if (view === 'month') return { from: weekStart(monthStart(d)), days: 42 };
  return { from: d, days: 1 };
}

// cuánto avanza el botón siguiente/anterior
export function shift(view, d, dir) {
  if (view === 'week') return addDays(d, 7 * dir);
  if (view === 'month') return addMonths(d, dir);
  return addDays(d, dir);
}

const fmt = (d, opts) => new Date(d + 'T12:00:00').toLocaleDateString('es-MX', opts);

export function rangeLabel(view, d) {
  if (view === 'month') return fmt(d, { month: 'long', year: 'numeric' });
  if (view === 'week') {
    const a = weekStart(d);
    const b = addDays(a, 6);
    const same = a.slice(0, 7) === b.slice(0, 7);
    return `${fmt(a, { day: 'numeric', ...(same ? {} : { month: 'short' }) })} – ${fmt(b, { day: 'numeric', month: 'short', year: 'numeric' })}`;
  }
  return fmt(d, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
}

export const dayHead = (d) => ({ name: fmt(d, { weekday: 'short' }).replace('.', ''), num: Number(d.slice(8)) });

// tramos libres entre los de trabajo, dentro de [lo, hi]; segs = [[inicio, fin], ...] en minutos
export function gaps(segs, lo, hi) {
  const sorted = [...segs].sort((a, b) => a[0] - b[0]);
  const out = [];
  let cursor = lo;
  for (const [s, e] of sorted) {
    if (s > cursor) out.push([cursor, Math.min(s, hi)]);
    cursor = Math.max(cursor, e);
  }
  if (cursor < hi) out.push([cursor, hi]);
  return out.filter(([a, b]) => b > a);
}

// Acomoda en carriles las citas que se empalman (de distintas personas) para que se vean lado a lado.
// Devuelve el mismo arreglo con lane y lanes en cada elemento: left = lane / lanes, ancho = 1 / lanes.
export function withLanes(items, start, end) {
  const sorted = [...items].sort((a, b) => start(a) - start(b) || end(a) - end(b));
  const out = [];
  let cluster = [];
  let clusterEnd = -1;
  const flush = () => {
    const lanes = [];
    for (const it of cluster) {
      let l = lanes.findIndex((last) => last <= start(it));
      if (l < 0) l = lanes.length;
      lanes[l] = end(it);
      it.lane = l;
    }
    for (const it of cluster) it.lanes = lanes.length;
    out.push(...cluster);
    cluster = [];
  };
  for (const it of sorted) {
    if (cluster.length && start(it) >= clusterEnd) {
      flush();
      clusterEnd = -1;
    }
    cluster.push({ ...it });
    clusterEnd = Math.max(clusterEnd, end(it));
  }
  if (cluster.length) flush();
  return out;
}
