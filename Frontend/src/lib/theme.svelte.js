// Dos temas intercambiables en toda la app: 'dark' (oscuro y elegante) y 'light' (claro y suave).
const KEY = 'miestudio-theme';

function stored() {
  try {
    const t = localStorage.getItem(KEY);
    return t === 'dark' || t === 'light' ? t : null;
  } catch {
    return null;
  }
}

function apply(name) {
  document.documentElement.dataset.theme = name;
}

export const theme = $state({ name: 'light' });

export function initTheme() {
  theme.name = stored() ?? document.documentElement.dataset.theme ?? 'light';
  apply(theme.name);
}

// Se recuerda la elección en este navegador.
export function setTheme(name) {
  theme.name = name;
  apply(name);
  try {
    localStorage.setItem(KEY, name);
  } catch {}
}

export const toggleTheme = () => setTheme(theme.name === 'dark' ? 'light' : 'dark');
