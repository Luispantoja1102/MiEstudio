import { goto } from '$app/navigation';

// La sesión vive en una cookie HttpOnly que pone el servidor: ningún script de la página puede leerla.
// Las peticiones que cambian datos llevan este encabezado, que un formulario de otro sitio no puede añadir.
export async function api(path, { method = 'GET', body } = {}) {
  const headers = { 'X-Requested-With': 'miestudio' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';

  const res = await fetch(path, { method, headers, credentials: 'same-origin', body: body !== undefined ? JSON.stringify(body) : undefined });
  const data = res.status === 204 ? {} : await res.json().catch(() => ({}));
  if (!res.ok) {
    // sesión vencida, cerrada desde otro lugar o usuario dado de baja
    if (res.status === 401 && !path.startsWith('/api/auth/')) goto('/login');
    throw Object.assign(new Error(data.error || 'Error inesperado'), { status: res.status, code: data.code });
  }
  return data;
}
