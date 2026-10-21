const mxn = new Intl.NumberFormat('es-MX', { style: 'currency', currency: 'MXN' });

export const money = (cents) => mxn.format(cents / 100);
export const toPesos = (cents) => (cents / 100).toFixed(2);

// "150.50" -> 15050; devuelve NaN si no es un monto válido.
export const toCents = (value) => {
  const n = Number(String(value).trim().replace(',', '.'));
  return Number.isFinite(n) && n >= 0 ? Math.round(n * 100) : NaN;
};

export const ROLES = {
  admin: 'Administrador',
  staff: 'Profesional',
  reception: 'Recepción',
  cashier: 'Caja'
};

export const formatDate = (iso) =>
  new Date(iso).toLocaleString('es-MX', { dateStyle: 'medium', timeStyle: 'short' });
