// Estado de los textos legales de la plataforma: cuál está abierto en la ventana y los datos de quien opera el servicio.
export const legal = $state({ open: '', info: null }); // open: '' | 'privacidad' | 'terminos'

let loading = false;
export async function loadLegal() {
  if (legal.info || loading) return;
  loading = true;
  try {
    legal.info = await (await fetch('/api/legal')).json();
  } catch {
    // sin datos del operador el texto se muestra igual, con el nombre por omisión
  }
  loading = false;
}

// Para el onclick de un enlace a /privacidad o /terminos: abre la ventana en lugar de cambiar de página.
// Con Ctrl, Cmd, Mayús o clic central se deja que el navegador lo abra en una pestaña, como cualquier enlace.
export function legalLink(e, which) {
  if (e.metaKey || e.ctrlKey || e.shiftKey || e.button) return;
  e.preventDefault();
  legal.open = which;
}
