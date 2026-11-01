<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { canWritePlatform } from '$lib/session.svelte.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import FormSection from '$lib/ui/FormSection.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';

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

<PageHeader title="Ajustes" description="Controla cómo se registran los negocios." />

{#if error}<div class="gap"><InlineNotice tone="danger" message={error} /></div>{/if}

<FormSection title="Quién puede registrarse" description="Elige cómo entran los negocios nuevos. El cambio se guarda al elegir.">
  <fieldset>
    <legend class="sr">Modo de registro de negocios</legend>
    {#each modes as [value, label, text]}
      <label class="opt" class:sel={mode === value} class:off={!write}>
        <input type="radio" name="mode" {value} checked={mode === value} disabled={!write} onchange={() => pick(value)} />
        <span><strong>{label}</strong><small>{text}</small></span>
      </label>
    {/each}
  </fieldset>
  {#if saved}<p class="note" role="status">{saved}</p>{/if}
  {#if !write}<p class="note">Tu rol es de solo lectura.</p>{/if}
</FormSection>

<style>
  .gap { margin-bottom: 16px; }
  fieldset { margin: 0; padding: 0; border: 0; min-width: 0; display: grid; gap: 8px; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .opt { display: flex; gap: 12px; align-items: flex-start; min-height: 44px; padding: 14px 16px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface); cursor: pointer; color: var(--text); }
  .opt:hover:not(.off) { border-color: var(--control-border); }
  .opt.sel { border-color: var(--accent); background: var(--accent-soft); }
  .opt.off { cursor: default; }
  .opt:has(input:focus-visible) { outline: 2px solid var(--accent); outline-offset: 2px; }
  .opt input { width: 18px; height: 18px; margin: 2px 0 0; flex: none; accent-color: var(--accent); }
  .opt span { display: grid; gap: 2px; min-width: 0; }
  .opt small { color: var(--muted); font-size: 0.88rem; }
  .note { margin: 12px 0 0; color: var(--muted); font-size: 0.9rem; }
</style>
