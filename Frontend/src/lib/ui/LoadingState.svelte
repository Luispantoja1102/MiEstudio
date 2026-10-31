<script>
  let { label = 'Cargando…', compact = false } = $props();
</script>

<div class="loading" class:compact role="status"><span class="ring" aria-hidden="true"></span>{label}</div>

<style>
  .loading { display: flex; align-items: center; justify-content: center; gap: 10px; padding: 48px 16px; color: var(--muted); font-size: 0.9rem; }
  .compact { padding: 16px; justify-content: flex-start; }
  .ring { width: 16px; height: 16px; border: 2px solid var(--border); border-top-color: var(--accent); border-radius: 50%; animation: spin 0.8s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>
