<script>
  // onDark: papel sobre tinta, para el panel de marca que siempre es oscuro
  let { onDark = false } = $props();
</script>

<a class="logo" class:on-dark={onDark} href="/" aria-label="Barbella, inicio">
  <span class="mark" aria-hidden="true">
    <svg viewBox="0 0 56 56" width="30" height="30" fill="none">
      <path d="M28 26c-4-6-12-8-21-3 1 10 12 14 21 6 9 8 20 4 21-6-9-5-17-3-21 3Z" fill="currentColor" />
      <circle cx="28" cy="13" r="4.5" fill="#c07a65" />
    </svg>
  </span>
  <span class="word">Barbella</span>
</a>

<style>
  .logo { display: inline-flex; align-items: center; gap: 10px; color: var(--text); text-decoration: none; }
  .mark {
    display: grid;
    place-items: center;
    width: 36px;
    height: 36px;
    border-radius: 9px;
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .on-dark { color: #f8f6f2; }
  .on-dark .mark { background: #f8f6f2; color: #1c2f5c; }
  .word { font-family: var(--font-heading); font-weight: 700; font-size: 1.3rem; letter-spacing: -0.01em; }
</style>
