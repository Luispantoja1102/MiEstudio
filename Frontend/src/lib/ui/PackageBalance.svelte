<script>
  import StatusBadge from './StatusBadge.svelte';
  // Saldo de un paquete: sesiones restantes de las que incluye, vigencia y estado.
  let { name, remaining, total, expiresLabel = '', statusLabel = '' } = $props();
</script>

<div class="pkg">
  <div class="head"><strong>{name}</strong>{#if statusLabel}<StatusBadge label={statusLabel} tone="info" />{/if}</div>
  <div class="ticks" role="img" aria-label="{remaining} de {total} sesiones disponibles">
    {#each Array(total) as _, i}<i class:used={i >= remaining}></i>{/each}
  </div>
  <p>Quedan <strong>{remaining}</strong> de {total}{#if expiresLabel} · {expiresLabel}{/if}</p>
</div>

<style>
  .pkg { display: grid; gap: 8px; padding: 16px; border: 1px solid var(--border); border-radius: var(--radius); background: var(--surface); }
  .head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .ticks { display: flex; flex-wrap: wrap; gap: 4px; }
  i { width: 22px; height: 8px; border-radius: 2px; background: var(--accent); }
  i.used { background: var(--surface-2); }
  p { margin: 0; color: var(--muted); font-size: 0.875rem; }
</style>
