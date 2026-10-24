<script>
  import Icon from '$lib/Icon.svelte';

  // Campo de contraseña con botón para mostrarla u ocultarla. `show` se puede enlazar para que otro campo (confirmar) siga el mismo estado.
  let { value = $bindable(''), show = $bindable(false), ...rest } = $props();
</script>

<div class="pass">
  <input type={show ? 'text' : 'password'} bind:value {...rest} />
  <button type="button" class="eye" aria-label={show ? 'Ocultar contraseña' : 'Mostrar contraseña'} aria-pressed={show} onclick={() => (show = !show)}>
    <Icon name={show ? 'eye-off' : 'eye'} size={20} />
  </button>
</div>

<style>
  .pass { position: relative; margin-top: 0.4rem; }
  .pass input { padding-right: 3rem; }
  .eye { position: absolute; right: 4px; top: 50%; transform: translateY(-50%); display: grid; place-items: center; width: 40px; height: 40px; border: 0; border-radius: 50%; background: none; color: var(--muted); cursor: pointer; }
  .eye:hover { color: var(--text); background: var(--surface-2); }
</style>
