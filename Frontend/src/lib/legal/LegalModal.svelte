<script>
  import Modal from '$lib/Modal.svelte';
  import LegalText from './LegalText.svelte';
  import { legal } from './legal.svelte.js';

  // Ventana con el aviso de privacidad o los términos: se cierra con la ×, con «Cerrar», con Esc o al hacer clic fuera.
  const TITLES = { privacidad: 'Aviso de privacidad', terminos: 'Términos y condiciones' };
</script>

<Modal bind:open={() => legal.open !== '', (v) => { if (!v) legal.open = ''; }} title={TITLES[legal.open] ?? ''}>
  {#key legal.open}<LegalText which={legal.open} />{/key}
  <div class="actions"><button type="button" class="btn ghost" onclick={() => (legal.open = '')}>Cerrar</button></div>
</Modal>

<style>
  .actions { display: flex; justify-content: flex-end; margin-top: 1.5rem; }
</style>
