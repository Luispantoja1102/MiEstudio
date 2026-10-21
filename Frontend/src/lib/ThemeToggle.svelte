<script>
  import Icon from '$lib/Icon.svelte';
  import { theme, toggleTheme } from '$lib/theme.svelte.js';

  const label = $derived(theme.name === 'dark' ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro');
</script>

<button class="toggle" onclick={toggleTheme} aria-label={label} title={label}>
  <Icon name={theme.name === 'dark' ? 'sun' : 'moon'} />
</button>

<style>
  .toggle {
    display: inline-grid;
    place-items: center;
    width: 44px;
    height: 44px;
    color: var(--text);
    background: transparent;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background-color 0.2s var(--ease), border-color 0.2s var(--ease);
  }
  .toggle:hover { background: var(--surface-2); border-color: var(--muted); }
</style>
