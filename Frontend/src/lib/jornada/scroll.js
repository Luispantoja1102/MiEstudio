// Motor de la landing «La jornada»: cada escena larga publica su avance como --p (0 → 1) mientras su cuadro
// está fijo en pantalla, y el CSS de cada capítulo decide qué mover con ese número.
// Una sola escucha de scroll, pasiva y limitada a un cálculo por cuadro (requestAnimationFrame).
// Con «reducir movimiento» no se escucha nada: la escena queda en su estado final y estático (.still).

const nodes = new Set();
let raf = 0;

function update() {
  raf = 0;
  const vh = window.innerHeight;
  for (const n of nodes) {
    const r = n.getBoundingClientRect();
    const total = r.height - vh;
    const p = total > 0 ? Math.min(1, Math.max(0, -r.top / total)) : 1;
    n.style.setProperty('--p', p.toFixed(4));
    // data-steps="N": N paradas; en cada tramo se queda quieto un rato y luego avanza (--s, también de 0 a 1)
    const steps = +n.dataset.steps;
    if (steps > 1) {
      const x = p * (steps - 1);
      const i = Math.min(steps - 2, Math.floor(x));
      const f = Math.min(1, Math.max(0, (x - i - 0.4) / 0.6));
      const e = f < 0.5 ? 4 * f * f * f : 1 - Math.pow(-2 * f + 2, 3) / 2;
      n.style.setProperty('--s', ((i + e) / (steps - 1)).toFixed(4));
    }
  }
}

function request() {
  if (!raf) raf = requestAnimationFrame(update);
}

export const reduced = () => typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;

/** use:scene en el contenedor alto de un capítulo. */
export function scene(node) {
  if (reduced()) {
    node.classList.add('still');
    node.style.setProperty('--p', '1');
    node.style.setProperty('--s', '1');
    return {};
  }
  nodes.add(node);
  if (nodes.size === 1) {
    addEventListener('scroll', request, { passive: true });
    addEventListener('resize', request);
  }
  request();
  return {
    destroy() {
      nodes.delete(node);
      if (!nodes.size) {
        removeEventListener('scroll', request);
        removeEventListener('resize', request);
      }
    }
  };
}
