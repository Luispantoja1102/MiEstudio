// Dos temas intercambiables: 'dark' (barbería) y 'light' (spa).
const KEY = 'miestudio-theme';

// Tema por defecto según el giro del negocio.
export const themeForKind = (kind) => (kind === 'barberia' ? 'dark' : 'light');

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

// Cambio manual: se recuerda y gana sobre el tema del giro.
export function setTheme(name) {
  theme.name = name;
  apply(name);
  try {
    localStorage.setItem(KEY, name);
  } catch {}
}

export const toggleTheme = () => setTheme(theme.name === 'dark' ? 'light' : 'dark');

// Aplica el tema del giro, salvo que la persona ya haya elegido uno.
export function applyKindTheme(kind) {
  if (stored()) return;
  theme.name = themeForKind(kind);
  apply(theme.name);
}
