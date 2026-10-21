import { goto } from '$app/navigation';

const TOKEN = 'miestudio-token';

export const getToken = () => {
  try { return localStorage.getItem(TOKEN); } catch { return null; }
};
export const setToken = (t) => {
  try { t ? localStorage.setItem(TOKEN, t) : localStorage.removeItem(TOKEN); } catch {}
};

export async function api(path, { method = 'GET', body } = {}) {
  const headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;

  const res = await fetch(path, { method, headers, body: body !== undefined ? JSON.stringify(body) : undefined });
  const data = res.status === 204 ? {} : await res.json().catch(() => ({}));
  if (!res.ok) {
    // sesión vencida o usuario dado de baja
    if (res.status === 401 && token && !path.startsWith('/api/auth/')) {
      setToken(null);
      goto('/login');
    }
    throw Object.assign(new Error(data.error || 'Error inesperado'), { status: res.status });
  }
  return data;
}
