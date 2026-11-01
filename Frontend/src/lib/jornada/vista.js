// Aviso de «ya se ve»: pone la clase .vista una sola vez cuando la pieza entra en pantalla.
// El CSS de cada capítulo mueve algo solo si explica (un aviso que llega, un anticipo que se confirma).
// Con «reducir movimiento» queda puesta desde el principio: todo se ve completo y estático.
import { reduced } from './scroll.js';

export function vista(node) {
  if (reduced() || typeof IntersectionObserver === 'undefined') {
    node.classList.add('vista');
    return {};
  }
  const io = new IntersectionObserver(
    ([e]) => {
      if (e.isIntersecting) {
        node.classList.add('vista');
        io.disconnect();
      }
    },
    { threshold: 0.4 }
  );
  io.observe(node);
  return { destroy: () => io.disconnect() };
}
