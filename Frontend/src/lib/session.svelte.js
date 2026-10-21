import { api } from '$lib/api.js';

export const session = $state({ me: null });

export async function loadSession() {
  session.me = await api('/api/me');
  return session.me;
}

export const isAdmin = () => session.me?.role === 'admin';
export const canSeeNotes = () => ['admin', 'staff'].includes(session.me?.role);
