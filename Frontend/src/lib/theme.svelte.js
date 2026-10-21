// Cuatro temas: 'light' (Azul tinta, el predeterminado), 'lino' (lino y terracota), 'ivory' (Marfil: solemne como el oscuro, pero de día) y 'dark' (carbón y champán).
const KEY = 'miestudio-theme';
const LIGHT_KEY = 'miestudio-light'; // el claro preferido, al que vuelve el botón de sol/luna
export const LIGHT_THEMES = ['light', 'lino', 'ivory']; // los que se pueden elegir como «tema claro»
export const THEMES = [...LIGHT_THEMES, 'dark'];

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

export const theme = $state({ name: 'light', light: 'light', forced: false });

export function initTheme() {
  theme.light = LIGHT_THEMES.includes(stored(LIGHT_KEY)) ? stored(LIGHT_KEY) : 'light';
  theme.name = stored() ?? document.documentElement.dataset.theme ?? theme.light;
  apply(theme.name);
}

// Se recuerda la elección en este navegador.
export function setTheme(name) {
  if (!THEMES.includes(name)) return;
  theme.name = name;
  if (name !== 'dark') theme.light = name;
  apply(name);
  try {
    localStorage.setItem(KEY, name);
    if (name !== 'dark') localStorage.setItem(LIGHT_KEY, name);
  } catch {}
}

// La persona elige cuál es su tema claro (cualquiera menos el oscuro) y se muestra de inmediato.
export function setLightTheme(name) {
  if (!LIGHT_THEMES.includes(name)) return;
  setTheme(name);
}

// El botón del encabezado alterna entre el oscuro y el claro que la persona eligió.
export const toggleTheme = () => setTheme(theme.name === 'dark' ? theme.light : 'dark');

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
