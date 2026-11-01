<script>
  // Estado en etiqueta. El texto lo decide cada pantalla («Pagada», «Agendada»…); el tono solo pone el color.
  // tone: neutral | success | warning | danger | info | accent
  let { label, tone = 'neutral' } = $props();
</script>

<span class="badge {tone}">{label}</span>

<style>
  .badge { display: inline-flex; align-items: center; min-height: 24px; padding: 0 10px; border-radius: var(--radius-xs); font-size: 0.8rem; font-weight: 600; white-space: nowrap; background: var(--surface-2); color: var(--muted); }
  .success { background: var(--success-soft); color: var(--success); }
  .warning { background: var(--warn-soft); color: var(--warn); }
  .danger { background: var(--danger-soft); color: var(--danger); }
  .info { background: var(--info-soft); color: var(--info); }
  .accent { background: var(--accent-soft); color: var(--accent); }
</style>
