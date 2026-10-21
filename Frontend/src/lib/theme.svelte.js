// Cuatro temas: 'lino' (predeterminado: lino y terracota, navegación en riel), 'light' (Azul tinta), 'ivory' (Marfil: solemne como el oscuro, pero de día) y 'dark' (carbón y champán).
const KEY = 'miestudio-theme';
const LIGHT_KEY = 'miestudio-light'; // el claro preferido, al que vuelve el botón de sol/luna
export const THEMES = ['lino', 'light', 'ivory', 'dark'];

function stored(key = KEY) {
  try {
    const t = localStorage.getItem(key);
    return THEMES.includes(t) ? t : null;
  } catch {
    return null;
  }
}

function apply(name) {
  document.documentElement.dataset.theme = name;
}

export const theme = $state({ name: 'lino', forced: false });

export function initTheme() {
  theme.name = stored() ?? document.documentElement.dataset.theme ?? 'lino';
  apply(theme.name);
}

// Se recuerda la elección en este navegador.
export function setTheme(name) {
  if (!THEMES.includes(name)) return;
  theme.name = name;
  apply(name);
  try {
    localStorage.setItem(KEY, name);
    if (name !== 'dark') localStorage.setItem(LIGHT_KEY, name);
  } catch {}
}

// El botón del encabezado alterna entre el oscuro y el claro que la persona prefiere.
export const toggleTheme = () => setTheme(theme.name === 'dark' ? (stored(LIGHT_KEY) ?? 'lino') : 'dark');

// Páginas públicas de un negocio que fijó su tema: se muestra ese sin tocar la elección guardada de quien visita.
// Devuelve la función que lo deshace (al salir de la página vuelve el tema de la persona).
export function forceTheme(name) {
  if (!THEMES.includes(name)) return () => {};
  theme.forced = true;
  apply(name);
  return () => {
    theme.forced = false;
    apply(theme.name);
  };
}
