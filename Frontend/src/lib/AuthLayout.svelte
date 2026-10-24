<script>
  import Icon from '$lib/Icon.svelte';
  import Logo from '$lib/Logo.svelte';
  import ThemeToggle from '$lib/ThemeToggle.svelte';

  // title + accent forman el titular: la última palabra va en cursiva con color de acento
  let { title, accent = '', subtitle = '', wide = false, children } = $props();
  const points = ['Agenda sin empalmes por profesional', 'Caja, ventas y comisiones en un solo lugar', 'Notas de seguimiento de cada cliente'];
</script>

<div class="auth" class:wide>
  <aside class="brand">
    <Logo onDark />
    <div class="pitch">
      <p class="tag">Software para barberías y salones</p>
      <h2>Tu negocio,<br /><em>en orden.</em></h2>
      <ul>
        {#each points as p}
          <li><span class="tick"><Icon name="check" size={13} /></span>{p}</li>
        {/each}
      </ul>
      <p class="safe"><Icon name="shield" size={16} />Cada negocio ve únicamente sus propios datos</p>
    </div>
    <p class="kinds">Barberías · Estéticas · Spas · Uñas</p>
  </aside>

  <main>
    <div class="top"><ThemeToggle /></div>
    <div class="card box">
      <header>
        <h1>{title}{#if accent}{" "}<em>{accent}</em>{/if}</h1>
        {#if subtitle}<p class="sub">{subtitle}</p>{/if}
      </header>
      {@render children()}
    </div>
    <p class="legal"><a href="/terminos">Términos</a><span aria-hidden="true">·</span><a href="/privacidad">Aviso de privacidad</a></p>
  </main>
</div>

<style>
  .auth { min-height: 100vh; display: grid; grid-template-columns: 1fr; background: var(--bg); }

  /* panel de marca: siempre tinta, como la sección oscura de la landing */
  .brand {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    color: #f8f6f2;
    background: #0f1012;
    background-image: radial-gradient(ellipse 70% 50% at 8% 0%, rgba(207, 84, 68, 0.32), transparent 62%), radial-gradient(ellipse 55% 45% at 100% 100%, rgba(95, 130, 220, 0.22), transparent 60%);
  }
  .pitch, .kinds { display: none; }

  main { position: relative; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 16px; padding: 28px 16px 36px; }
  .top { position: absolute; top: 12px; right: 12px; display: none; }
  .box { width: 100%; max-width: 432px; padding: 28px 24px; }
  .wide .box { max-width: 624px; }
  h1 { font-size: 2.2rem; line-height: 1.02; margin: 0 0 0.4rem; }
  h1 em, h2 em { font-style: italic; color: var(--accent); }
  .sub { color: var(--muted); font-size: 0.95rem; margin: 0 0 1.5rem; }
  header { margin-bottom: 0.25rem; }
  .legal { display: flex; gap: 8px; font-size: 0.82rem; color: var(--muted); margin: 0; }
  .legal a { display: inline-block; padding: 8px 4px; color: inherit; }
  .legal a:hover { color: var(--text); }

  @media (min-width: 900px) {
    .auth { grid-template-columns: minmax(20rem, 0.9fr) minmax(0, 1.1fr); }
    .wide.auth { grid-template-columns: minmax(18rem, 0.75fr) minmax(0, 1.25fr); }
    .brand { position: sticky; top: 0; height: 100vh; flex-direction: column; align-items: flex-start; padding: 36px 48px; }
    .pitch { display: block; max-width: 28rem; }
    .kinds { display: block; }
    .tag { font: 600 0.72rem/1 var(--font-body, inherit); letter-spacing: 0.16em; text-transform: uppercase; color: rgba(248, 246, 242, 0.55); margin: 0 0 1.25rem; }
    h2 { font-size: clamp(2.4rem, 4vw, 3.5rem); line-height: 0.98; margin: 0; color: #f8f6f2; }
    h2 em { color: #ee8a7c; }
    ul { list-style: none; padding: 0; margin: 2rem 0 0; display: grid; gap: 14px; color: rgba(248, 246, 242, 0.82); font-size: 0.95rem; }
    li { display: flex; align-items: flex-start; gap: 12px; }
    .tick { flex: none; display: grid; place-items: center; width: 20px; height: 20px; margin-top: 1px; border-radius: 50%; background: rgba(238, 138, 124, 0.22); color: #ee8a7c; }
    .safe { display: inline-flex; align-items: center; gap: 8px; margin: 2.2rem 0 0; padding: 8px 14px; border-radius: 999px; font-size: 0.85rem; color: rgba(248, 246, 242, 0.82); box-shadow: inset 0 0 0 1px rgba(248, 246, 242, 0.18); }
    .kinds { font-size: 0.72rem; letter-spacing: 0.14em; text-transform: uppercase; color: rgba(248, 246, 242, 0.45); margin: 0; }
    main { padding: 40px 24px; }
    .top { display: block; }
    .box { padding: 36px; }
  }
</style>
