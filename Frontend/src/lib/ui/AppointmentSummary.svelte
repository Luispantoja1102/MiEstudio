<script>
  import StatusBadge from './StatusBadge.svelte';
  // Una cita en una fila: hora, cliente, servicio y profesional, estado y acciones. Se usa en Inicio y en la lista de la agenda.
  // color: color del profesional (un punto junto a su nombre); compact: fila más baja para listas largas.
  let { client, service, staff = '', color = '', timeLabel, statusLabel = '', tone = 'neutral', compact = false, actions } = $props();
</script>

<div class="appt" class:compact>
  <span class="time">{timeLabel}</span>
  <div class="who">
    <strong>{client}</strong>
    <span>{service}{#if staff}<span class="sep" aria-hidden="true">·</span><span class="staff">{#if color}<i style="background: {color}"></i>{/if}{staff}</span>{/if}</span>
  </div>
  {#if statusLabel || actions}
    <div class="side">
      {#if statusLabel}<StatusBadge label={statusLabel} {tone} />{/if}
      {#if actions}<div class="acts">{@render actions()}</div>{/if}
    </div>
  {/if}
</div>

<style>
  .appt { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; padding: 12px 0; border-bottom: 1px solid var(--border); }
  .appt:last-child { border-bottom: 0; }
  .compact { padding: 8px 0; }
  .time { flex: none; min-width: 64px; font-weight: 700; font-variant-numeric: tabular-nums lining-nums; }
  .who { display: grid; flex: 1 1 180px; min-width: 0; }
  .who > span { color: var(--muted); font-size: 0.875rem; }
  .who strong { overflow-wrap: anywhere; }
  .sep { margin: 0 6px; }
  .staff { display: inline-flex; align-items: center; gap: 6px; }
  i { width: 8px; height: 8px; border-radius: 50%; flex: none; }
  .side { display: flex; align-items: center; gap: 8px; margin-left: auto; }
  .acts { display: flex; gap: 8px; }
  /* celular: la etiqueta y las acciones bajan debajo del nombre, alineadas con él, para que el nombre respire */
  @media (max-width: 560px) {
    .who { flex-basis: calc(100% - 88px); }
    .side { flex-basis: 100%; margin-left: 80px; justify-content: space-between; }
  }
</style>
