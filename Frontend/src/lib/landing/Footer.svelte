<script>
  import Logo from '$lib/Logo.svelte';
</script>

<footer class="foot">
  <div class="container grid">
    <div class="brand">
      <Logo />
      <p>Agenda, caja y reservas en línea para barberías, estéticas, spas y salones de uñas. Hecho en México.</p>
    </div>
    <nav aria-label="Producto"><h3>Producto</h3><a href="#funciones">Funciones</a><a href="#planes">Planes</a><a href="#preguntas">Preguntas</a></nav>
    <nav aria-label="Cuenta"><h3>Cuenta</h3><a href="/registro">Crear cuenta</a><a href="/login">Iniciar sesión</a></nav>
    <nav aria-label="Legal"><h3>Legal</h3><a href="/privacidad">Aviso de privacidad</a><a href="/terminos">Términos</a></nav>
  </div>
  <div class="container legal">© {new Date().getFullYear()} Barbella. Todos los derechos reservados.</div>
</footer>

<style>
  .foot { padding-block: 56px 28px; border-top: 1px solid var(--border); background: var(--surface-2); }
  .grid { display: grid; gap: 32px; }
  .brand p { max-width: 22rem; margin: 1rem 0 0; font-size: 0.9rem; color: var(--muted); }
  nav { display: grid; gap: 8px; align-content: start; }
  h3 { margin: 0 0 0.3rem; font-family: var(--font-body, inherit); font-size: 0.72rem; font-weight: 600; letter-spacing: 0.14em; text-transform: uppercase; color: var(--muted); }
  nav a { font-size: 0.93rem; color: var(--text); text-decoration: none; }
  nav a:hover { color: var(--accent); text-decoration: underline; }
  .legal { margin-top: 36px; padding-top: 20px; border-top: 1px solid var(--border); font-size: 0.82rem; color: var(--muted); }
  @media (max-width: 480px) { .grid { grid-template-columns: 1fr; } }
  @media (min-width: 800px) { .grid { grid-template-columns: 1.6fr 1fr 1fr 1fr; } }
</style>
