<script>
  import Header from '$lib/Header.svelte';
  import Icon from '$lib/Icon.svelte';
  let { title, subtitle, nav, children } = $props();
  const points = ['Agenda sin empalmes por profesional', 'Caja, ventas y comisiones en un solo lugar', 'Notas de seguimiento de cada cliente'];
</script>

<Header>{@render nav?.()}</Header>

<main class="split">
  <aside class="brand" aria-hidden="true">
    <div>
      <h2>Tu negocio, en orden.</h2>
      <ul>
        {#each points as p}
          <li><span class="tick"><Icon name="check" size={14} /></span>{p}</li>
        {/each}
      </ul>
    </div>
  </aside>
  <section class="form">
    <div class="box">
      <h1>{title}</h1>
      {#if subtitle}<p class="sub">{subtitle}</p>{/if}
      {@render children()}
    </div>
  </section>
</main>

<style>
  .split { display: grid; grid-template-columns: 1fr; min-height: calc(100vh - 69px); }
  .brand { display: none; }
  .form { display: flex; justify-content: center; padding: 40px 20px 56px; }
  .box { width: 100%; max-width: 420px; }
  h1 { font-size: 2rem; margin-bottom: 0.3em; }
  .sub { color: var(--muted); margin-bottom: 1.75rem; }

  @media (min-width: 900px) {
    .split { grid-template-columns: 1fr 1fr; }
    .brand {
      display: flex;
      align-items: center;
      padding: 64px;
      background: var(--surface-2);
      border-right: 1px solid var(--border);
    }
    .brand h2 { font-size: 2.4rem; margin-bottom: 1.25rem; }
    ul { list-style: none; padding: 0; margin: 0; display: grid; gap: 14px; color: var(--muted); }
    li { display: flex; align-items: center; gap: 12px; }
    .tick {
      display: grid;
      place-items: center;
      width: 24px;
      height: 24px;
      border-radius: 50%;
      background: var(--accent-soft);
      color: var(--accent);
      flex: none;
    }
    .form { align-items: center; padding-block: 56px; }
  }
</style>
