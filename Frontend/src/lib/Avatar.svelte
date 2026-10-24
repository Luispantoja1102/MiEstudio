<script>
  // Círculo con las iniciales de un nombre; el color sale del nombre, así cada negocio siempre tiene el mismo.
  let { name = '', size = '2.4rem' } = $props();

  const initials = $derived(
    name
      .trim()
      .split(/\s+/)
      .slice(0, 2)
      .map((w) => w[0]?.toUpperCase() ?? '')
      .join('') || '?'
  );
  const tone = $derived(([...name].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7) % 6) + 1);
</script>

<span class="avatar" style="--s: {size}; --c: var(--p{tone})" aria-hidden="true">{initials}</span>

<style>
  .avatar {
    flex: none; display: inline-grid; place-items: center; width: var(--s); height: var(--s); border-radius: 50%;
    font-size: calc(var(--s) * 0.38); font-weight: 700; letter-spacing: 0.02em; color: var(--c);
    background: color-mix(in srgb, var(--c) 16%, var(--surface)); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--c) 35%, transparent);
  }
</style>
