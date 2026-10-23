<script>
  // La hora de la jornada: la del capítulo que está en el centro de la pantalla (cada uno trae data-hora y data-momento).
  // Se pregunta al punto central con elementsFromPoint, así respeta el recorrido horizontal de los oficios.
  import { onMount } from 'svelte';

  let hora = $state('09:00');
  let momento = $state('Se abre el local');

  onMount(() => {
    let raf = 0;
    const leer = () => {
      raf = 0;
      for (const el of document.elementsFromPoint(innerWidth / 2, innerHeight / 2)) {
        const c = el.closest('[data-hora]');
        if (c) {
          hora = c.dataset.hora;
          momento = c.dataset.momento;
          return;
        }
      }
    };
    const pedir = () => raf || (raf = requestAnimationFrame(leer));
    addEventListener('scroll', pedir, { passive: true });
    addEventListener('resize', pedir);
    leer();
    return () => {
      removeEventListener('scroll', pedir);
      removeEventListener('resize', pedir);
      cancelAnimationFrame(raf);
    };
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
