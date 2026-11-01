<script>
  // Iniciales sobre fondo neutro (sin color por nombre): el avatar sobrio de la lista de clientes y de la ficha.
  let { name = '', size = '2.5rem' } = $props();
  const initials = $derived(
    name
      .trim()
      .split(/\s+/)
      .slice(0, 2)
      .map((w) => w[0]?.toUpperCase() ?? '')
      .join('') || '?'
  );
</script>

<span class="ini" style="--s: {size}" aria-hidden="true">{initials}</span>

<style>
  .ini { flex: none; display: inline-grid; place-items: center; width: var(--s); height: var(--s); border-radius: 50%; font-size: calc(var(--s) * 0.36); font-weight: 600; letter-spacing: 0.02em; color: var(--muted); background: var(--surface-2); box-shadow: inset 0 0 0 1px var(--border); }
</style>
