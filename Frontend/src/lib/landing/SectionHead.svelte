<script>
  // Titular de sección: la última palabra va en cursiva con el color de acento, como en las demás pantallas.
  let { tag = '', title, accent = '', sub = '', center = true } = $props();
</script>

<header class="head" class:center>
  {#if tag}<p class="tag">{tag}</p>{/if}
  <h2>{title}{#if accent}{' '}<em>{accent}</em>{/if}</h2>
  {#if sub}<p class="sub">{sub}</p>{/if}
</header>

<style>
  .head { max-width: 46rem; margin: 0 0 2.5rem; }
  .head.center { margin-inline: auto; text-align: center; }
  .tag { margin: 0 0 0.8rem; font-size: 0.74rem; font-weight: 600; letter-spacing: 0.16em; text-transform: uppercase; color: var(--accent); }
  h2 { font-size: clamp(2rem, 4.4vw, 3rem); line-height: 1.02; margin: 0; }
  h2 em { font-style: italic; color: var(--accent); }
  .sub { margin: 1rem 0 0; font-size: 1.05rem; color: var(--muted); }
</style>
