<script>
  import { api } from '$lib/api.js';

  // Ocupación: cuánto del tiempo que tu equipo tiene disponible está agendado, quién tiene más lugar y cuándo se llena.
  let { from, to, branch = 0 } = $props();

  let r = $state(null);
  let error = $state('');
  let loading = $state(true);

  $effect(() => {
    const params = new URLSearchParams({ from, to });
    if (branch) params.set('branch_id', branch);
    loading = true;
    api('/api/reports/occupancy?' + params)
      .then((data) => ((r = data), (error = '')))
      .catch((e) => (error = e.message))
      .finally(() => (loading = false));
  });

  const DAYS = ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb'];
  const DAYS_LONG = ['domingo', 'lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábado'];
  const ORDER = [1, 2, 3, 4, 5, 6, 0]; // la semana empieza en lunes
  const hours = (min) => `${(min / 60).toFixed(min % 60 === 0 ? 0 : 1)} h`;
  const pct = (booked, available) => (available > 0 ? Math.round((booked / available) * 100) : null);
  const pctText = (v) => (v == null ? '—' : `${Math.round(v)} %`);

  const noHours = $derived(r && r.available_min === 0);
  // las horas con citas, con una de margen, para que el mapa no sea de 24 columnas casi vacías
  const hourRange = $derived.by(() => {
    if (!r?.heat.length) return [];
    const lo = Math.max(0, Math.min(...r.heat.map((h) => h.hour)) - 1);
    const hi = Math.min(23, Math.max(...r.heat.map((h) => h.hour)) + 1);
    return Array.from({ length: hi - lo + 1 }, (_, i) => lo + i);
  });
  const heatMax = $derived(Math.max(1, ...(r?.heat.map((h) => h.count) ?? [0])));
  const cell = (wd, hour) => r?.heat.find((h) => h.weekday === wd && h.hour === hour)?.count ?? 0;
  const busiest = $derived(r?.heat.length ? r.heat.reduce((a, b) => (b.count > a.count ? b : a)) : null);
</script>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if r}
  <div class:dim={loading}>
    {#if noHours}
      <div class="empty">Aún no hay horarios cargados. Define el horario de cada persona en <a href="/panel/equipo">Equipo</a> para calcular la ocupación.</div>
    {:else}
      <div class="kpis">
        <div class="card"><span class="label">Ocupación</span><span class="value">{pctText(r.occupancy_pct)}</span><small>{hours(r.booked_min)} agendadas de {hours(r.available_min)} disponibles</small></div>
        <div class="card"><span class="label">Citas</span><span class="value">{r.appointments}</span><small>agendadas y terminadas</small></div>
        <div class="card"><span class="label">No llegaron</span><span class="value">{r.no_shows}</span><small>{r.no_show_pct == null ? 'Sin citas' : `${pctText(r.no_show_pct)} de las citas`}</small></div>
        <div class="card"><span class="label">Canceladas</span><span class="value">{r.cancelled}</span><small>en el periodo</small></div>
      </div>

      <section>
        <h2>Por persona</h2>
        <div class="bars">
          {#each r.staff as s (s.staff_id)}
            {@const v = pct(s.booked_min, s.available_min)}
            <div class="bar-row">
              <span class="who">{s.name}</span>
              <div class="track"><div class="fill" style="width: {Math.min(100, v ?? 0)}%"></div></div>
              <b>{pctText(v)}</b>
              <small class="meta">{hours(s.booked_min)} de {hours(s.available_min)} · {s.appointments} {s.appointments === 1 ? 'cita' : 'citas'}{s.no_shows ? ` · ${s.no_shows} no llegó` : ''}</small>
            </div>
          {/each}
        </div>
      </section>

      <section>
        <h2>Por día de la semana</h2>
        <div class="week" role="img" aria-label="Ocupación por día de la semana">
          {#each ORDER as wd}
            {@const d = r.by_weekday[wd]}
            {@const v = pct(d.booked_min, d.available_min)}
            <div class="day">
              <div class="col"><div class="fill v" style="height: {v == null ? 0 : Math.max(2, Math.min(100, v))}%"></div></div>
              <b>{pctText(v)}</b>
              <span class="meta">{DAYS[wd]}</span>
            </div>
          {/each}
        </div>
        <p class="meta">Los días con poca ocupación son buenos para promociones; los llenos, para subir precios o abrir más lugares.</p>
      </section>

      <section>
        <h2>Horas pico</h2>
        {#if hourRange.length}
          <div class="heat" style="grid-template-columns: 3.2rem repeat({hourRange.length}, minmax(0, 1fr))" role="img" aria-label="Citas por día y hora">
            <span></span>
            {#each hourRange as h}<span class="hh">{h}</span>{/each}
            {#each ORDER as wd}
              <span class="dd">{DAYS[wd]}</span>
              {#each hourRange as h}
                {@const n = cell(wd, h)}
                <span class="c" class:on={n > 0} style="--k: {n / heatMax}" title={`${DAYS_LONG[wd]} ${h}:00 · ${n} ${n === 1 ? 'cita' : 'citas'}`}>{n || ''}</span>
              {/each}
            {/each}
          </div>
          {#if busiest}<p class="meta">Lo más lleno: {DAYS_LONG[busiest.weekday]} a las {busiest.hour}:00 ({busiest.count} {busiest.count === 1 ? 'cita' : 'citas'}).</p>{/if}
        {:else}
          <p class="meta">Todavía no hay citas en este periodo.</p>
        {/if}
      </section>
    {/if}
  </div>
{/if}

<style>
  .dim { opacity: 0.6; transition: opacity 0.2s; }
  .kpis { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); }
  .kpis .card { display: grid; gap: 2px; }
  .label { color: var(--muted); font-size: 0.9rem; }
  .value { font-weight: 700; font-variant-numeric: tabular-nums lining-nums; font-size: 1.9rem; line-height: 1.15; }
  small { color: var(--muted); }
  section { margin-top: 2rem; }
  section h2 { font-size: 1.35rem; }
  .bars { display: grid; gap: 12px; }
  .bar-row { display: grid; grid-template-columns: minmax(80px, 160px) 1fr 56px; align-items: center; gap: 4px 12px; font-variant-numeric: tabular-nums; }
  .bar-row small { grid-column: 2 / -1; }
  .who { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .track { height: 14px; background: var(--surface-2); border-radius: 999px; overflow: hidden; }
  .fill { height: 100%; background: var(--accent); border-radius: 999px; min-width: 2px; }
  .week { display: grid; grid-template-columns: repeat(7, 1fr); gap: 8px; align-items: end; }
  .day { display: grid; justify-items: center; gap: 4px; font-variant-numeric: tabular-nums; }
  .col { display: flex; align-items: flex-end; width: 100%; max-width: 56px; height: 140px; background: var(--surface-2); border-radius: var(--radius-sm); overflow: hidden; }
  .fill.v { width: 100%; border-radius: 0; min-width: 0; }
  .heat { display: grid; gap: 3px; align-items: center; font-size: 0.72rem; font-variant-numeric: tabular-nums; overflow-x: auto; }
  .hh, .dd { color: var(--muted); text-align: center; }
  .dd { text-align: left; font-weight: 600; }
  .c { display: grid; place-items: center; min-height: 28px; border-radius: 4px; background: var(--surface-2); color: var(--text); }
  .c.on { background: color-mix(in srgb, var(--accent) calc(18% + var(--k) * 82%), var(--surface-2)); color: color-mix(in srgb, var(--accent-contrast) calc(var(--k) * 100%), var(--text)); font-weight: 700; }
</style>
