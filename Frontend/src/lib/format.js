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
  plan_changed: 'Plan editado (precio o límites)',
  plan_reset: 'Plan restablecido a su configuración original',
  business_profile_changed: 'Datos del negocio actualizados',
  demo_business_created: 'Negocio de demostración creado',
  mp_connected: 'Mercado Pago conectado',
  mp_disconnected: 'Mercado Pago desconectado',
  mp_terminal_registered: 'Terminal de Mercado Pago registrada',
  transfer_confirmed: 'Pago por transferencia confirmado',
  transfer_rejected: 'Comprobante de transferencia rechazado',
  business_registered: 'Negocio registrado',
  business_created_by_platform: 'Negocio dado de alta por la plataforma',
  business_updated: 'Negocio actualizado',
  user_password_reset: 'Contraseña restablecida',
  user_active_changed: 'Persona activada o desactivada',
  platform_user_created: 'Persona de plataforma creada',
  platform_user_updated: 'Persona de plataforma actualizada',
  settings_changed: 'Ajustes cambiados',
  payment_recorded: 'Pago registrado',
  password_reset_by_email: 'Contraseña restablecida por correo',
  two_factor_enabled: 'Verificación en dos pasos activada',
  two_factor_disabled: 'Verificación en dos pasos desactivada',
  two_factor_reset: 'Verificación en dos pasos restablecida',
  arco_updated: 'Solicitud de privacidad atendida',
  client_exported: 'Datos de un cliente exportados',
  client_erased: 'Datos de un cliente borrados',
  booking_settings_changed: 'Reserva en línea configurada'
};

export const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

export const METHODS = { cash: 'Efectivo', card: 'Tarjeta', transfer: 'Transferencia', mp_point: 'Terminal Mercado Pago', deposit: 'Anticipo en línea', package: 'Paquete' };

// identificador único por intento de cobro: repetir la misma petición no cobra dos veces
export const newToken = () => (globalThis.crypto?.randomUUID ? crypto.randomUUID() : `t-${Date.now()}-${Math.random().toString(36).slice(2)}`);

// Estado de cobro de un negocio: [etiqueta, tono de la insignia]
export const BILLING = {
  pending: ['Pendiente de aprobar', 'warn'],
  trialing: ['En prueba', ''],
  trial_expired: ['Prueba vencida', 'bad'],
  active: ['Activo', ''],
  past_due: ['Pago atrasado', 'bad'],
  suspended: ['Suspendido', 'bad']
};

export const PLANS = { individual: 'Individual', basico: 'Básico', crecimiento: 'Crecimiento', pro: 'Pro' };
export const PAY_METHODS = { transfer: 'Transferencia', cash: 'Efectivo', card: 'Tarjeta', mercadopago: 'Mercado Pago', courtesy: 'Cortesía' };

// precio de lista en pesos (los planes vienen en pesos enteros)
export const pesos = (n) => new Intl.NumberFormat('es-MX', { style: 'currency', currency: 'MXN', maximumFractionDigits: 0 }).format(n);

// «hace 5 min», «hace 3 h»; pasado un día, la fecha corta
export const ago = (iso) => {
  const m = Math.round((Date.now() - new Date(iso)) / 60000);
  if (m < 1) return 'ahora';
  if (m < 60) return `hace ${m} min`;
  const h = Math.round(m / 60);
  if (h < 24) return `hace ${h} h`;
  return new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short' });
};

// El usuario para entrar va en minúsculas: letras, números, punto, guion y guion bajo (3 a 30).
export const cleanUsername = (s) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().replace(/[^a-z0-9._-]/g, '').slice(0, 30);
