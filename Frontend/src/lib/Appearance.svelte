<script>
  import { LIGHT_THEMES, setLightTheme, setTheme, theme } from '$lib/theme.svelte.js';

  // Cada persona elige su tema claro; el oscuro no es una opción de «tema claro» y se activa aparte.
  const options = {
    lino: ['Lino', 'Lino y terracota, editorial y cálido.', 'linear-gradient(135deg, #a8614c 50%, #f8f6f2 50%)'],
    light: ['Azul tinta', 'Marfil fresco con azul profundo.', 'linear-gradient(135deg, #1c2f5c 50%, #f3f1ec 50%)'],
    ivory: ['Marfil', 'Marfil solemne con carbón y champán.', 'linear-gradient(135deg, #17171b 50%, #f4efe6 50%)']
  };
  const dark = $derived(theme.name === 'dark');
</script>

<h3>Tema claro</h3>
<div class="opts" role="radiogroup" aria-label="Tema claro">
  {#each LIGHT_THEMES as value}
    {@const [name, text, swatch] = options[value]}
    <button type="button" class="card opt" role="radio" aria-checked={theme.light === value} class:on={theme.light === value} onclick={() => setLightTheme(value)}>
      <span class="sw" style:background={swatch} aria-hidden="true"></span>
      <span class="txt"><b>{name}</b><small>{text}</small></span>
    </button>
  {/each}
</div>

<h3 style="margin-top: 1.5rem">Modo oscuro</h3>
<label class="check">
  <input type="checkbox" checked={dark} onchange={(e) => setTheme(e.currentTarget.checked ? 'dark' : theme.light)} />
  Usar el tema oscuro (carbón con detalles dorados)
</label>
<p class="meta">Se recuerda en este navegador. El botón del sol y la luna, arriba, alterna entre el oscuro y tu tema claro. La página pública de tu negocio la decide el administrador en Reservas en línea.</p>

<style>
  h3 { font-size: 1.2rem; margin-bottom: 0.75rem; }
  .opts { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); }
  .opt { display: flex; align-items: center; gap: 14px; text-align: left; font: inherit; color: var(--text); cursor: pointer; transition: border-color var(--dur-ui) var(--ease), background-color var(--dur-ui) var(--ease), transform var(--dur-press) var(--ease); }
  .opt:active { transform: scale(0.98); }
  .opt.on { border-color: var(--accent); background: var(--accent-soft); }
  .sw { flex: none; width: 42px; height: 42px; border-radius: 12px; border: 1px solid var(--border); }
  .txt { display: grid; gap: 2px; flex: 1; }
  .txt small { color: var(--muted); font-size: 0.85rem; }
  .meta { color: var(--muted); font-size: 0.88rem; margin-top: 1rem; }
</style>
