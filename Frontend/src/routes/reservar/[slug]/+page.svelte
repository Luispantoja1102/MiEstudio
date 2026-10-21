<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';
  import { money, plural } from '$lib/format.js';
  import { addDays, fmtTime, longDate } from '$lib/time.js';

  const slug = page.params.slug;
  const call = async (path, body) => {
    const res = await fetch(path, body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : undefined);
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw Object.assign(new Error(data.error || 'Error inesperado'), { status: res.status });
    return data;
  };

  let info = $state(null);
  let missing = $state(false);
  let error = $state('');
  let step = $state(1); // 1 servicio · 2 lugar y persona · 3 fecha y hora · 4 tus datos
  let service = $state(null);
  let branchId = $state('');
  let staffId = $state('');
  let date = $state('');
  let slots = $state([]);
  let loadingSlots = $state(false);
  let picked = $state(null);
  let busy = $state(false);
  let form = $state({ name: '', phone: '', email: '', notes: '', accept_privacy: false, consent_reminders: false, consent_marketing: false, consent_whatsapp: false });
  let done = $state(null);
  let seq = 0;
  let joined = $state(false);
  let joining = $state(false);
  let wait = $state(false); // formulario de lista de espera abierto
  let waitForm = $state({ name: '', phone: '', email: '', accept_privacy: false, from: '', to: '' });

  const tz = $derived(info?.business.timezone ?? 'America/Mexico_City');
  const lastDay = $derived(info ? addDays(info.today, info.max_days_ahead) : '');

  onMount(async () => {
    try {
      info = await call(`/api/public/${slug}`);
      date = info.today;
      if (info.branches.length === 1) branchId = String(info.branches[0].id);
      // el correo de lista de espera trae el servicio, la sucursal y el día
      const q = page.url.searchParams;
      const svc = info.services.find((s) => String(s.id) === q.get('service'));
      if (svc && info.branches.some((b) => String(b.id) === q.get('branch'))) {
        service = svc;
        branchId = q.get('branch');
        if (/^\d{4}-\d{2}-\d{2}$/.test(q.get('date') ?? '') && q.get('date') >= info.today) date = q.get('date');
        step = 3;
        await loadSlots();
      }
    } catch (e) {
      if (e.status === 404) missing = true;
      else error = e.message;
    }
  });

  function chooseService(s) {
    service = s;
    picked = null;
    step = 2;
  }

  async function loadSlots() {
    picked = null;
    if (!service || !branchId || !date) return;
    const mine = ++seq;
    loadingSlots = true;
    error = '';
    try {
      const q = new URLSearchParams({ service_id: service.id, branch_id: branchId, date });
      if (staffId) q.set('staff_id', staffId);
      const r = await call(`/api/public/${slug}/slots?${q}`);
      if (mine === seq) slots = r.staff;
    } catch (e) {
      if (mine === seq) {
        slots = [];
        error = e.message;
      }
    } finally {
      if (mine === seq) loadingSlots = false;
    }
  }

  // con «cualquiera», un horario puede ofrecerlo más de una persona: se muestra una sola vez
  const merged = $derived.by(() => {
    if (staffId) return slots.flatMap((s) => s.starts.map((iso) => ({ iso, staffId: s.staff_id, who: s.staff_name })));
    const seen = new Map();
    for (const s of slots) for (const iso of s.starts) if (!seen.has(iso)) seen.set(iso, { iso, staffId: 0, who: 'Cualquier profesional' });
    return [...seen.values()].sort((a, b) => a.iso.localeCompare(b.iso));
  });

  const canSubmit = $derived(
    !busy && picked && form.name.trim().length >= 2 && form.phone.replace(/\D/g, '').length >= 10 && form.accept_privacy &&
      ((!form.consent_reminders && !form.consent_marketing) || form.email.trim())
  );

  async function submit(e) {
    e.preventDefault();
    if (!canSubmit) return;
    busy = true;
    error = '';
    try {
      const r = await call(`/api/public/${slug}/bookings`, {
        service_id: service.id, branch_id: Number(branchId), staff_id: picked.staffId, starts_at: picked.iso,
        ...form
      });
      if (r.pay_url) {
        // la cita queda apartada unos minutos; el pago se hace en Mercado Pago y se regresa al enlace de la cita
        done = r;
        location.href = r.pay_url;
        return;
      }
      done = r;
    } catch (err) {
      error = err.message;
      if (err.status === 409) {
        picked = null;
        await loadSlots();
        step = 3;
      }
    } finally {
      busy = false;
    }
  }

  async function joinWaitlist(e) {
    e.preventDefault();
    joining = true;
    error = '';
    try {
      await call(`/api/public/${slug}/waitlist`, {
        service_id: service.id, branch_id: Number(branchId), staff_id: staffId ? Number(staffId) : 0, date,
        from: waitForm.from, to: waitForm.to, name: waitForm.name, phone: waitForm.phone, email: waitForm.email, accept_privacy: waitForm.accept_privacy
      });
      joined = true;
    } catch (err) {
      error = err.message;
    } finally {
      joining = false;
    }
  }

  const staffName = $derived(staffId ? info?.staff.find((p) => String(p.id) === String(staffId))?.name : 'Cualquier profesional');
</script>

<PublicLayout title={missing ? 'Página no disponible' : (info?.business.name ?? '')} subtitle={info?.business.welcome_text ?? ''}>
  {#if missing}
    <div class="card"><p>Esta página de reservas no existe o no está disponible por ahora. Revisa el enlace que te compartió el negocio.</p></div>
  {:else if !info}
    {#if error}<p class="error" role="alert">{error}</p>{:else}<p class="meta">Cargando…</p>{/if}
  {:else if done}
    <div class="card done" role="status">
      {#if done.pay_url}
        <h2>Te llevamos a pagar el anticipo</h2>
        <p>Tu horario está apartado unos minutos. Si no se abre Mercado Pago, <a href={done.pay_url}>entra aquí</a>.</p>
        <p class="meta">Al terminar volverás a tu cita, donde verás la confirmación.</p>
      {:else}
        <h2>¡Cita confirmada!</h2>
        <p>{service.name} · {longDate(picked.iso.slice(0, 10))} a las {fmtTime(picked.iso, tz)}.</p>
        {#if form.email}<p class="meta">Te enviamos la confirmación a {form.email}.</p>{/if}
      {/if}
      <a class="btn" href={done.manage_path}>Ver o cancelar mi cita</a>
      <p class="meta" style="margin-top: 1rem">Guarda ese enlace: es la única forma de ver o cancelar tu cita sin crear una cuenta.</p>
    </div>
  {:else}
    <ol class="steps" aria-label="Pasos">
      {#each ['Servicio', 'Lugar', 'Horario', 'Tus datos'] as label, i}
        <li aria-current={step === i + 1 ? 'step' : undefined} class:on={step >= i + 1}>{label}</li>
      {/each}
    </ol>
    {#if error}<p class="error" role="alert">{error}</p>{/if}

    {#if step === 1}
      <div class="list">
        {#each info.services as s (s.id)}
          <button class="item-row pick" onclick={() => chooseService(s)}>
            <span class="grow"><span class="title">{s.name}</span><span class="meta">{s.duration_min} min{#if s.deposit_cents} · anticipo {money(s.deposit_cents)}{/if}</span></span>
            {#if s.price_cents !== undefined}<b>{money(s.price_cents)}</b>{/if}
          </button>
        {:else}
          <p class="empty">Por ahora no hay servicios disponibles para reservar en línea.</p>
        {/each}
      </div>
    {:else if step === 2}
      <div class="card">
        <p><b>{service.name}</b> · {service.duration_min} min</p>
        {#if info.branches.length > 1}
          <label class="field">Sucursal
            <select bind:value={branchId}>
              <option value="" disabled>Elige una sucursal</option>
              {#each info.branches as b}<option value={String(b.id)}>{b.name}{b.address ? ` · ${b.address}` : ''}</option>{/each}
            </select>
          </label>
        {:else}
          <p class="meta">{info.branches[0].name}{info.branches[0].address ? ` · ${info.branches[0].address}` : ''}</p>
        {/if}
        <label class="field">Con quién
          <select bind:value={staffId}>
            <option value="">Cualquier profesional</option>
            {#each info.staff as p}<option value={String(p.id)}>{p.name}</option>{/each}
          </select>
        </label>
        <div class="form-actions">
          <button class="btn ghost" onclick={() => (step = 1)}>Atrás</button>
          <button class="btn" disabled={!branchId} onclick={() => ((step = 3), loadSlots())}>Continuar</button>
        </div>
      </div>
    {:else if step === 3}
      <div class="card">
        <label class="field">Fecha
          <input type="date" bind:value={date} min={info.today} max={lastDay} onchange={loadSlots} />
        </label>
        <p class="meta">{longDate(date)} · horario de {tz}</p>
        <div aria-live="polite">
          {#if loadingSlots}
            <p class="meta">Buscando horarios…</p>
          {:else if merged.length === 0}
            <p class="meta">No hay horarios ese día. Prueba con otra fecha{info.staff.length > 1 ? ' o con otra persona' : ''}.</p>
            {#if info.waitlist}
              {#if joined}
                <p class="ok" role="status">✓ Listo. Si se libera un lugar ese día, te avisamos por correo.</p>
              {:else if !wait}
                <button class="btn ghost" onclick={() => (wait = true)}>Avisarme si se libera un lugar</button>
              {:else}
                <form class="wait" onsubmit={joinWaitlist}>
                  <p class="meta">Te escribimos si se libera un horario de {service.name} el {longDate(date)}. El primero que lo reserva, se lo queda.</p>
                  <label class="field">Nombre <input bind:value={waitForm.name} autocomplete="name" required maxlength="120" /></label>
                  <label class="field">Teléfono <input type="tel" bind:value={waitForm.phone} autocomplete="tel" inputmode="tel" required maxlength="30" placeholder="10 dígitos" /></label>
                  <label class="field">Correo <input type="email" bind:value={waitForm.email} autocomplete="email" required maxlength="200" /></label>
                  <div class="row2">
                    <label class="field">Desde (opcional) <input type="time" bind:value={waitForm.from} /></label>
                    <label class="field">Hasta <input type="time" bind:value={waitForm.to} /></label>
                  </div>
                  <label class="check"><input type="checkbox" bind:checked={waitForm.accept_privacy} required /> <span>Acepto el <a href="/aviso/{slug}" target="_blank" rel="noopener">aviso de privacidad</a></span></label>
                  <div class="form-actions">
                    <button type="button" class="btn ghost" onclick={() => (wait = false)}>Cancelar</button>
                    <button class="btn" disabled={joining || !waitForm.accept_privacy}>{joining ? 'Guardando…' : 'Avisarme'}</button>
                  </div>
                </form>
              {/if}
            {/if}
          {:else}
            <div class="chips">
              {#each merged as m (m.iso + m.staffId)}
                <button class="chip" aria-pressed={picked?.iso === m.iso && picked?.staffId === m.staffId} onclick={() => (picked = m)}>
                  {fmtTime(m.iso, tz)}{#if staffId && info.staff.length > 1}<small> · {m.who}</small>{/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
        <div class="form-actions">
          <button class="btn ghost" onclick={() => (step = 2)}>Atrás</button>
          <button class="btn" disabled={!picked} onclick={() => (step = 4)}>Continuar</button>
        </div>
      </div>
    {:else}
      <form class="card" onsubmit={submit}>
        <p class="summary">
          <b>{service.name}</b><br />
          {longDate(picked.iso.slice(0, 10))} · {fmtTime(picked.iso, tz)} · {picked.staffId ? picked.who : staffName}
          {#if service.deposit_cents}<br /><span class="meta">Para apartarla se pide un anticipo de {money(service.deposit_cents)}, que se descuenta de tu cuenta.</span>{/if}
        </p>
        <label class="field">Nombre <input bind:value={form.name} autocomplete="name" required maxlength="120" /></label>
        <label class="field">Teléfono <input type="tel" bind:value={form.phone} autocomplete="tel" inputmode="tel" required maxlength="30" placeholder="10 dígitos" /></label>
        <label class="field">Correo (opcional)
          <input type="email" bind:value={form.email} autocomplete="email" maxlength="200" />
          <span class="hint">Te enviamos la confirmación y el enlace para cancelar.</span>
        </label>
        <label class="field">Notas (opcional) <input bind:value={form.notes} maxlength="300" /></label>
        <label class="check"><input type="checkbox" bind:checked={form.consent_reminders} /> Quiero recibir un recordatorio por correo antes de mi cita</label>
        {#if info.whatsapp}<label class="check"><input type="checkbox" bind:checked={form.consent_whatsapp} /> Quiero recibir también el recordatorio por WhatsApp (respondes «BAJA» para dejar de recibirlo)</label>{/if}
        <label class="check"><input type="checkbox" bind:checked={form.consent_marketing} /> Quiero recibir promociones y una felicitación en mi cumpleaños</label>
        {#if (form.consent_reminders || form.consent_marketing) && !form.email.trim()}<p class="meta">Escribe tu correo para recibir los mensajes.</p>{/if}
        <label class="check" style="margin-top: 0.5rem"><input type="checkbox" bind:checked={form.accept_privacy} required /> <span>He leído y acepto el <a href="/aviso/{slug}" target="_blank" rel="noopener">aviso de privacidad</a></span></label>
        <details class="privacy">
          <summary>Aviso de privacidad</summary>
          <p>
            {info.business.name} usará tu nombre, teléfono y correo únicamente para gestionar tu cita: confirmarla, avisarte si cambia y,
            solo si lo aceptas arriba, enviarte un recordatorio (por correo o por WhatsApp), promociones o una felicitación de cumpleaños. Cada correo trae un enlace para darte de baja. No se comparten con terceros, salvo el procesador de pagos cuando se pide anticipo.
            Puedes cancelar tu cita y dejar de recibir recordatorios desde el enlace de tu cita, o pedir que borren tus datos al negocio.
          </p>
        </details>
        <div class="form-actions">
          <button type="button" class="btn ghost" onclick={() => (step = 3)}>Atrás</button>
          <button class="btn" disabled={!canSubmit}>{busy ? 'Reservando…' : service.deposit_cents ? `Reservar y pagar ${money(service.deposit_cents)}` : 'Confirmar reserva'}</button>
        </div>
      </form>
    {/if}
    <p class="meta foot"><a href="/aviso/{slug}">Aviso de privacidad</a> · <a href="/arco/{slug}">Ejercer mis derechos</a><br />Reserva hasta {plural(info.max_days_ahead, 'día', 'días')} adelante · cancela hasta {plural(info.cancel_min_hours, 'hora', 'horas')} antes.</p>
  {/if}
</PublicLayout>

<style>
  .steps { display: flex; gap: 6px; list-style: none; padding: 0; margin: 0 0 1.25rem; font-size: 0.8rem; color: var(--muted); flex-wrap: wrap; }
  .steps li { padding: 4px 12px; border-radius: 999px; background: var(--surface-2); }
  .steps li.on { background: var(--accent-soft); color: var(--accent); font-weight: 600; }
  .pick { cursor: pointer; text-align: left; font: inherit; color: inherit; width: 100%; }
  .pick:hover { border-color: var(--accent); }
  .pick .meta { display: block; }
  .summary { padding: 0.8rem 1rem; background: var(--surface-2); border-radius: var(--radius-sm); }
  .privacy { margin: 0.5rem 0 1rem; font-size: 0.85rem; color: var(--muted); }
  .privacy summary { cursor: pointer; }
  .done { text-align: center; display: grid; gap: 0.5rem; justify-items: center; }
  .done h2 { margin-bottom: 0; }
  .foot { text-align: center; margin-top: 1.5rem; }
  .chip small { opacity: 0.8; }
  .wait { margin-top: 0.75rem; padding-top: 0.75rem; border-top: 1px solid var(--border); }
  .row2 { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .ok { color: var(--accent); font-weight: 600; }
  .meta { color: var(--muted); font-size: 0.88rem; }
</style>
