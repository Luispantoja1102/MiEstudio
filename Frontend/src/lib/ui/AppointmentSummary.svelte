<script>
  import StatusBadge from './StatusBadge.svelte';
  // Una cita en una fila: hora, cliente, servicio y profesional, estado y acciones. Se usa en Inicio y en la lista de la agenda.
  let { client, service, staff = '', timeLabel, statusLabel = '', tone = 'neutral', actions } = $props();
</script>

<div class="appt">
  <span class="time">{timeLabel}</span>
  <div class="who">
    <strong>{client}</strong>
    <span>{service}{#if staff} · {staff}{/if}</span>
  </div>
  {#if statusLabel}<StatusBadge label={statusLabel} {tone} />{/if}
  {#if actions}<div class="acts">{@render actions()}</div>{/if}
</div>

<style>
  .appt { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; padding: 12px 0; border-bottom: 1px solid var(--border); }
  .appt:last-child { border-bottom: 0; }
  .time { flex: none; min-width: 64px; font-weight: 700; font-variant-numeric: tabular-nums lining-nums; }
  .who { display: grid; flex: 1 1 180px; min-width: 0; }
  .who span { color: var(--muted); font-size: 0.875rem; }
  .who strong { overflow-wrap: anywhere; }
  .acts { display: flex; gap: 8px; }
</style>
