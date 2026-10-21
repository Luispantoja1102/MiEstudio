<script>
  import Icon from '$lib/Icon.svelte';
  import { setTheme, theme } from '$lib/theme.svelte.js';

  const options = [
    ['light', 'Azul tinta', 'Claro y fresco: marfil, azul profundo y terracota.', 'sun'],
    ['ivory', 'Marfil', 'Claro y solemne: contenido en marfil con carbón y oro champán.', 'sun'],
    ['dark', 'Oscuro', 'Carbón con detalles dorados, ideal con poca luz.', 'moon']
  ];
</script>

<div class="opts" role="radiogroup" aria-label="Tema">
  {#each options as [value, name, text, icon]}
    <button type="button" class="card opt" role="radio" aria-checked={theme.name === value} class:on={theme.name === value} onclick={() => setTheme(value)}>
      <span class="ico"><Icon name={icon} /></span>
      <span class="txt"><b>{name}</b><small>{text}</small></span>
      {#if theme.name === value}<span class="tick"><Icon name="check" size={18} /></span>{/if}
    </button>
  {/each}
</div>
<p class="meta">Se recuerda en este navegador. El botón del sol y la luna arriba alterna entre el oscuro y tu tema claro. La página pública de tu negocio tiene su propio tema (Ajustes → Reservas en línea).</p>

<style>
  .opts { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); }
  .opt { display: flex; align-items: center; gap: 14px; text-align: left; font: inherit; color: var(--text); cursor: pointer; }
  .opt.on { border-color: var(--accent); background: var(--accent-soft); }
  .ico { display: grid; place-items: center; width: 42px; height: 42px; flex: none; border-radius: 11px; background: var(--surface-2); color: var(--accent); }
  .txt { display: grid; gap: 2px; flex: 1; }
  .txt small { color: var(--muted); font-size: 0.85rem; }
  .tick { color: var(--accent); }
  .meta { color: var(--muted); font-size: 0.88rem; margin-top: 1rem; }
</style>
