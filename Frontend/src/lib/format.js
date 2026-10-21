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

export const KINDS = { barberia: 'Barbería', estetica: 'Estética / salón', spa: 'Spa', unas: 'Uñas' };

export const STATUS = {
  pending: ['Pendiente', 'warn'],
  trial: ['En prueba', ''],
  active: ['Activo', ''],
  suspended: ['Suspendido', 'bad']
};

export const PLATFORM_ROLES = { platform_admin: 'Administrador', platform_support: 'Soporte (solo lectura)' };

export const formatDay = (iso) => (iso ? new Date(iso).toLocaleDateString('es-MX', { dateStyle: 'medium' }) : '—');

export const ACTIONS = {
  business_registered: 'Negocio registrado',
  business_created_by_platform: 'Negocio dado de alta por la plataforma',
  business_updated: 'Negocio actualizado',
  user_password_reset: 'Contraseña restablecida',
  user_active_changed: 'Persona activada o desactivada',
  platform_user_created: 'Persona de plataforma creada',
  platform_user_updated: 'Persona de plataforma actualizada',
  settings_changed: 'Ajustes cambiados'
};

export const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

export const METHODS = { cash: 'Efectivo', card: 'Tarjeta', transfer: 'Transferencia', mp_point: 'Terminal Mercado Pago', deposit: 'Anticipo en línea' };

// identificador único por intento de cobro: repetir la misma petición no cobra dos veces
export const newToken = () => (globalThis.crypto?.randomUUID ? crypto.randomUUID() : `t-${Date.now()}-${Math.random().toString(36).slice(2)}`);
