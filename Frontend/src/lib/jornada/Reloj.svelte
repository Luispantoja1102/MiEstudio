<script>
  // La hora de la jornada: cambia con el capítulo que cruza la mitad de la pantalla (cada uno trae data-hora y data-momento).
  import { onMount } from 'svelte';

  let hora = $state('08:58');
  let momento = $state('Antes de abrir');

  onMount(() => {
    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (!e.isIntersecting) continue;
          hora = e.target.dataset.hora;
          momento = e.target.dataset.momento;
        }
      },
      { rootMargin: '-50% 0px -50% 0px' }
    );
    document.querySelectorAll('[data-hora]').forEach((el) => io.observe(el));
    return () => io.disconnect();
  });
</script>

<p class="reloj" aria-live="off">
  <span class="h">{hora}</span>
  <span class="m">{momento}</span>
</p>

<style>
  .reloj { display: flex; align-items: baseline; gap: 10px; margin: 0; white-space: nowrap; }
  .h { font-family: var(--rotulo); font-stretch: 75%; font-weight: 700; font-size: 1.15rem; font-variant-numeric: tabular-nums; letter-spacing: 0.01em; }
  .m { font-size: 0.85rem; color: var(--tinta-suave); }
  @media (max-width: 860px) { .m { display: none; } }
</style>
