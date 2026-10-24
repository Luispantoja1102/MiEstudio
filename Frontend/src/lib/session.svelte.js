import { api } from '$lib/api.js';

export const session = $state({ me: null });

export async function loadSession() {
  session.me = await api('/api/me');
  return session.me;
}

// ¿el plan del negocio incluye cobros (caja, ventas, inventario, reportes)? Básico solo trae agenda y citas.
export const hasPos = () => session.me?.business?.plan_features?.pos !== false;
// tope o función del plan (max_branches, max_staff, terminal…); undefined si el servidor no lo mandó
export const planLimit = (key) => session.me?.business?.plan_features?.[key];
// Mercado Pago solo sirve para anticipos con MP y para la terminal
export const hasMercadoPago = () => planLimit('deposits_mp') !== false || planLimit('terminal') !== false;
export const isAdmin = () => session.me?.role === 'admin';
export const canSeeNotes = () => ['admin', 'staff'].includes(session.me?.role);
export const isPlatform = (me = session.me) => me?.role === 'platform_admin' || me?.role === 'platform_support';
export const canWritePlatform = () => session.me?.role === 'platform_admin';

// A dónde va cada persona tras iniciar sesión.
export const homeFor = (me) => {
  if (isPlatform(me)) return '/plataforma';
  // un negocio nuevo pasa primero por el asistente de inicio (solo su titular, y si la cuenta ya está activa)
  const b = me?.business;
  if (b?.setup_pending && !['pending', 'suspended'].includes(b.status)) return '/bienvenida';
  return '/panel';
};
