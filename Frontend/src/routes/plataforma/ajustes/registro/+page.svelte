<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { canWritePlatform } from '$lib/session.svelte.js';

  const write = canWritePlatform();
  const modes = [
    ['open', 'Abierto', 'Cualquiera puede registrarse y empieza una prueba de 14 días al instante.'],
    ['approval', 'Con aprobación', 'Cada registro queda pendiente y no puede usar la plataforma hasta que lo apruebes.'],
    ['closed', 'Cerrado', 'Nadie puede registrarse por su cuenta. Solo tú das de alta negocios.']
  ];

  let mode = $state('open');
  let saved = $state('');
  let error = $state('');

  onMount(async () => {
    try {
      mode = (await api('/api/platform/settings')).registration_mode;
    } catch (e) {
      error = e.message;
    }
  });

  async function pick(value) {
    error = '';
    saved = '';
    const previous = mode;
    mode = value;
    try {
      await api('/api/platform/settings', { method: 'PUT', body: { registration_mode: value } });
      saved = 'Guardado.';
    } catch (e) {
      mode = previous;
      error = e.message;
    }
  }
</script>

<div class="page-head">
  <div>
    <h1>Ajustes</h1>
    <p>Controla cómo se registran los negocios.</p>
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

<fieldset class="card">
  <legend>Registro de negocios</legend>
  {#each modes as [value, label, text]}
    <label class="opt" class:sel={mode === value}>
      <input type="radio" name="mode" {value} checked={mode === value} disabled={!write} onchange={() => pick(value)} />
      <span><strong>{label}</strong><small>{text}</small></span>
    </label>
  {/each}
  {#if saved}<p class="meta" role="status" style="margin: 0.75rem 0 0">{saved}</p>{/if}
  {#if !write}<p class="meta" style="margin: 0.75rem 0 0">Tu rol es de solo lectura.</p>{/if}
</fieldset>

<style>
  fieldset { margin: 0; border: 1px solid var(--border); }
  legend { padding: 0 8px; font-family: var(--font-heading); font-size: 1.2rem; }
  .opt { display: flex; gap: 12px; align-items: flex-start; padding: 0.9rem 1rem; margin-bottom: 8px; border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; color: var(--text); font-size: 1rem; }
  .opt.sel { border-color: var(--accent); background: var(--accent-soft); }
  .opt input { width: 18px; height: 18px; margin: 3px 0 0; accent-color: var(--accent); }
  .opt span { display: grid; gap: 2px; }
  .opt small { color: var(--muted); font-size: 0.88rem; }
</style>
