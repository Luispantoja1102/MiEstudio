// Utilidades de fecha y hora en la zona horaria del negocio.

const pad = (n) => String(n).padStart(2, '0');

function offsetMs(date, tz) {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: tz, hourCycle: 'h23', year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit'
  }).formatToParts(date);
  const o = Object.fromEntries(parts.map((p) => [p.type, p.value]));
  return Date.UTC(o.year, o.month - 1, o.day, o.hour, o.minute, o.second) - date.getTime();
}

// "2026-11-09" + "10:30" en la zona tz -> Date (instante real)
export function zonedToDate(dateStr, timeStr, tz) {
  const [y, m, d] = dateStr.split('-').map(Number);
  const [hh, mm] = timeStr.split(':').map(Number);
  const wall = Date.UTC(y, m - 1, d, hh, mm);
  let utc = wall;
  for (let i = 0; i < 2; i++) utc = wall - offsetMs(new Date(utc), tz);
  return new Date(utc);
}

export const todayIn = (tz) => new Date().toLocaleDateString('en-CA', { timeZone: tz });

export function addDays(dateStr, n) {
  const [y, m, d] = dateStr.split('-').map(Number);
  const t = new Date(Date.UTC(y, m - 1, d + n));
  return `${t.getUTCFullYear()}-${pad(t.getUTCMonth() + 1)}-${pad(t.getUTCDate())}`;
}

export const fmtTime = (iso, tz) =>
  new Date(iso).toLocaleTimeString('es-MX', { timeZone: tz, hour: '2-digit', minute: '2-digit', hour12: false });

// minutos desde la medianoche local
export function minutesOfDay(iso, tz) {
  const [h, m] = fmtTime(iso, tz).split(':').map(Number);
  return h * 60 + m;
}

export const clockToMin = (s) => {
  const [h, m] = s.split(':').map(Number);
  return h * 60 + m;
};
export const minToClock = (n) => `${pad(Math.floor(n / 60))}:${pad(n % 60)}`;

export const longDate = (dateStr) =>
  new Date(dateStr + 'T12:00:00').toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });

export const WEEKDAYS = ['Domingo', 'Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado'];
