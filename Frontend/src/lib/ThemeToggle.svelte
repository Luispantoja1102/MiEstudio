<script>
  import { theme, toggleTheme } from '$lib/theme.svelte.js';
</script>

<button class="toggle" onclick={toggleTheme} aria-label="Cambiar tema">
  {theme.name === 'dark' ? '☀ Tema claro' : '☾ Tema oscuro'}
</button>

<style>
  .toggle {
    font: inherit;
    font-size: 0.85rem;
    color: var(--text);
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.35rem 0.8rem;
    cursor: pointer;
  }
  .toggle:hover { border-color: var(--accent); }
</style>
