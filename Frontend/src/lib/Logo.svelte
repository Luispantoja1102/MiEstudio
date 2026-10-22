<script>
  // href: a dónde lleva el logo. Dentro de la plataforma debe volver al panel, no a la página de inicio.
  // onDark se conserva por compatibilidad: el logo es el mismo sobre cualquier fondo (cuadro blanco, bigote azul).
  let { href = '/', onDark = false } = $props();
</script>

<a class="logo" class:on-dark={onDark} {href} aria-label="Barbella, inicio">
  <span class="mark" aria-hidden="true">
    <svg viewBox="3 7 50 34" width="26" height="18" fill="none">
      <path d="M28 26c-4-6-12-8-21-3 1 10 12 14 21 6 9 8 20 4 21-6-9-5-17-3-21 3Z" fill="#1c2f5c" />
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
    width: 38px;
    height: 38px;
    border-radius: 10px;
    background: #fff;
    box-shadow: inset 0 0 0 1px rgba(28, 47, 92, 0.16), 0 1px 2px rgba(10, 20, 45, 0.12);
  }
  .word { font-family: var(--font-heading); font-weight: 700; font-size: 1.3rem; letter-spacing: -0.01em; }
  .on-dark { color: #f8f6f2; }
</style>
