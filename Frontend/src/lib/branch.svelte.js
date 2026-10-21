import { api } from '$lib/api.js';

// Sucursal activa para ventas, caja y reportes. Se recuerda en este navegador.
const KEY = 'miestudio-branch';
export const branches = $state({ list: [], id: null });

export async function loadBranches() {
  branches.list = (await api('/api/branches')).filter((b) => b.active);
  let saved = null;
  try {
    saved = Number(localStorage.getItem(KEY));
  } catch {}
  branches.id = branches.list.find((b) => b.id === saved)?.id ?? branches.list[0]?.id ?? null;
}

export function pickBranch(id) {
  branches.id = Number(id);
  try {
    localStorage.setItem(KEY, String(id));
  } catch {}
}
