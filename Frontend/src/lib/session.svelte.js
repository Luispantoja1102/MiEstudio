import { api } from '$lib/api.js';

export const session = $state({ me: null });

export async function loadSession() {
  session.me = await api('/api/me');
  return session.me;
}

export const isAdmin = () => session.me?.role === 'admin';
export const canSeeNotes = () => ['admin', 'staff'].includes(session.me?.role);
export const isPlatform = (me = session.me) => me?.role === 'platform_admin' || me?.role === 'platform_support';
export const canWritePlatform = () => session.me?.role === 'platform_admin';

// A dónde va cada persona tras iniciar sesión.
export const homeFor = (me) => (isPlatform(me) ? '/plataforma' : '/panel');
