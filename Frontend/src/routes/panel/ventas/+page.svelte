<script>
  import { onMount, onDestroy } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import Receipt from '$lib/pos/Receipt.svelte';
  import { api } from '$lib/api.js';
  import { METHODS, money, newToken, toCents, toPesos, plural } from '$lib/format.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { fmtTime, todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';
  import { page } from '$app/state';

  const isAdmin = session.me.role === 'admin';
  const tz = session.me.business.timezone || 'America/Tijuana';

  // ── datos
  let cash = $state(null);
  let ready = $state(false);
  let opening = $state('');
  let services = $state([]);
  let products = $state([]);
  let packages = $state([]); // catálogo de paquetes
  let clientPkgs = $state([]); // paquetes vigentes del cliente del ticket
  let staff = $state([]);
  let appts = $state([]);
  let point = $state({ ok: false, message: '', code: '' });
  let tab = $state('services');
  let q = $state('');

  // ── ticket
  let lines = $state([]);
  let client = $state(null);
  let clientQuery = $state('');
  let clientResults = $state([]);
  let appointmentId = $state(null);
  let discount = $state('');
  let tipMode = $state('none'); // none | 10 | 15 | 20 | custom
  let tipCustom = $state('');
  let tipStaffId = $state('');
  let note = $state('');
  let payments = $state([]);
  let token = $state(newToken());
  let error = $state('');
  let busy = $state(false);
  let receipt = $state(null);
  let ticketOpen = $state(false); // celular: el ticket es una hoja que sube desde abajo
  let timers = {};
  let clientTimer;

  // ── cálculos (todo en centavos)
  const subtotal = $derived(lines.reduce((n, l) => n + l.qty * l.unit, 0));
  const discountCents = $derived(Math.min(Math.max(toCents(discount || '0') || 0, 0), subtotal));
  const total = $derived(subtotal - discountCents);
  const tipCents = $derived(tipMode === 'none' ? 0 : tipMode === 'custom' ? Math.max(toCents(tipCustom || '0') || 0, 0) : Math.round((total * Number(tipMode)) / 100));
  const due = $derived(total + tipCents);
  const paid = $derived(payments.reduce((n, p) => n + (toCents(p.amount || '0') || 0), 0));
  const remaining = $derived(due - paid);
  const staffIds = $derived([...new Set(lines.filter((l) => l.kind === 'service' && l.staffId).map((l) => l.staffId))]);
  const cashRows = $derived(payments.filter((p) => p.method === 'cash'));
  const change = $derived(cashRows.reduce((n, p) => (p.received ? n + Math.max((toCents(p.received) || 0) - (toCents(p.amount || '0') || 0), 0) : n), 0));
  const mpPending = $derived(payments.some((p) => p.method === 'mp_point' && p.charge?.status !== 'approved'));
  const canPay = $derived(lines.length > 0 && due >= 0 && remaining === 0 && !mpPending && !busy && payments.every((p) => (toCents(p.amount || '') || 0) > 0));

  async function load() {
    ready = false;
    const branchId = branches.id;
    if (!branchId) return (ready = true);
    const [c, s, p, a, st, pk] = await Promise.all([
      api(`/api/cash/current?branch_id=${branchId}`),
      api('/api/services'),
      api('/api/products'),
      api('/api/agenda?' + new URLSearchParams({ branch_id: branchId, date: todayIn(tz) })),
      api(`/api/payments/point/status?branch_id=${branchId}`),
      api('/api/packages')
    ]);
    packages = pk;
    cash = c.session;
    services = s.filter((x) => x.active);
    products = p.filter((x) => x.active);
    staff = a.staff;
    appts = a.appointments.filter((x) => x.status === 'scheduled' || x.status === 'completed');
    point = st;
    ready = true;
    // «Cobrar» desde Inicio o la agenda llega con la cita elegida: el ticket ya viene armado
    const wanted = Number(page.url.searchParams.get('cita'));
    if (wanted && !appointmentId && cash) {
      const found = appts.find((x) => x.id === wanted && x.status === 'scheduled');
      if (found) {
        tab = 'appts';
        addAppointment(found);
      }
    }
  }

  onMount(async () => {
    try {
      await loadBranches();
      await load();
    } catch (e) {
      error = e.message;
      ready = true;
    }
  });
  onDestroy(() => Object.values(timers).forEach(clearInterval));

  async function openCash(e) {
    e.preventDefault();
    error = '';
    const cents = toCents(opening || '0');
    if (Number.isNaN(cents)) return (error = 'Escribe un monto válido, por ejemplo 500');
    try {
      await api('/api/cash/open', { method: 'POST', body: { branch_id: branches.id, opening_cents: cents } });
      opening = '';
      await load();
    } catch (err) {
      error = err.message;
    }
  }

  // ── armar el ticket
  function defaultStaff(s) {
    const eligible = s.staff_ids?.length ? staff.filter((x) => s.staff_ids.includes(x.id)) : staff;
    return eligible.length === 1 ? eligible[0].id : '';
  }
  function addService(s) {
    lines.push({ key: crypto.randomUUID?.() ?? Math.random(), kind: 'service', id: s.id, name: s.name, unit: s.price_cents, qty: 1, staffId: defaultStaff(s) });
  }
  function addProduct(p) {
    const found = lines.find((l) => l.kind === 'product' && l.id === p.id);
    if (found) {
      if (found.qty < p.stock) found.qty++;
      return;
    }
    if (p.stock < 1) return;
    lines.push({ key: crypto.randomUUID?.() ?? Math.random(), kind: 'product', id: p.id, name: p.name, unit: p.price_cents, qty: 1, staffId: '', stock: p.stock });
  }
  function addPackage(k) {
    if (!client) return (error = 'Para vender un paquete elige primero al cliente.');
    if (lines.some((l) => l.kind === 'package' && l.id === k.id)) return;
    lines.push({ key: crypto.randomUUID?.() ?? Math.random(), kind: 'package', id: k.id, name: k.name, unit: k.price_cents, qty: 1, staffId: '' });
  }
  function addAppointment(a) {
    if (appointmentId && appointmentId !== a.id && !confirm('Ya hay una cita en el ticket. ¿Cambiarla por esta?')) return;
    lines = lines.filter((l) => !l.fromAppt);
    appointmentId = a.id;
    client = { id: a.client_id, name: a.client_name };
    lines.unshift({ key: 'appt-' + a.id, kind: 'service', id: a.service_id, name: a.service_name, unit: a.price_cents, qty: 1, staffId: a.staff_id, fromAppt: true });
    // el anticipo pagado en línea se descuenta solo (y es obligatorio aplicarlo)
    payments = payments.filter((p) => p.method !== 'deposit');
    if (a.deposit_status === 'paid') {
      payments.unshift({ key: 'deposit-' + a.id, method: 'deposit', amount: toPesos(a.deposit_cents), received: '', reference: '', charge: null, locked: true });
    }
  }
  function removeLine(l) {
    lines = lines.filter((x) => x !== l);
    if (l.fromAppt) {
      appointmentId = null;
      payments = payments.filter((p) => p.method !== 'deposit');
    }
  }
  const maxQty = (l) => (l.kind === 'service' ? 20 : l.kind === 'package' ? 1 : (l.stock ?? 1000));

  function searchClient() {
    clearTimeout(clientTimer);
    clientTimer = setTimeout(async () => {
      clientResults = clientQuery.trim() ? (await api('/api/clients?q=' + encodeURIComponent(clientQuery.trim()))).slice(0, 5) : [];
    }, 250);
  }

  // ── paquetes del cliente: cada sesión paga una unidad de un servicio del ticket
  $effect(() => {
    const cid = client?.id;
    if (!cid) {
      clientPkgs = [];
      return;
    }
    api(`/api/clients/${cid}/packages`)
      .then((r) => client?.id === cid && (clientPkgs = r.filter((k) => k.status === 'active')))
      .catch(() => (clientPkgs = []));
  });
  const unitOf = (serviceId) => lines.find((l) => l.kind === 'service' && l.id === serviceId)?.unit ?? 0;
  const serviceQty = (serviceId) => lines.filter((l) => l.kind === 'service' && l.id === serviceId).reduce((n, l) => n + l.qty, 0);
  const pkgUsed = (k) => payments.filter((p) => p.method === 'package' && p.cpId === k.id).length;
  const pkgUsable = (k) => pkgUsed(k) < k.remaining && payments.filter((p) => p.method === 'package' && p.serviceId === k.service_id).length < serviceQty(k.service_id);
  // si se quita el servicio o al cliente, los canjes sobrantes salen del ticket
  $effect(() => {
    const seen = {};
    const keep = payments.filter((p) => {
      if (p.method !== 'package') return true;
      seen[p.serviceId] = (seen[p.serviceId] ?? 0) + 1;
      return client && seen[p.serviceId] <= serviceQty(p.serviceId);
    });
    if (keep.length !== payments.length) payments = keep;
  });
  $effect(() => {
    if (!client && lines.some((l) => l.kind === 'package')) lines = lines.filter((l) => l.kind !== 'package');
  });
  function addPackagePayment(k) {
    error = '';
    payments.push({ key: crypto.randomUUID?.() ?? Math.random(), method: 'package', cpId: k.id, serviceId: k.service_id, name: k.name, amount: toPesos(unitOf(k.service_id)), received: '', reference: '', charge: null, locked: false, fixed: true });
  }

  // ── pagos
  function addPayment(method) {
    error = '';
    const amount = Math.max(remaining, 0);
    payments.push({ key: crypto.randomUUID?.() ?? Math.random(), method, amount: amount ? toPesos(amount) : '', received: '', reference: '', charge: null });
  }
  function removePayment(p) {
    if (p.charge?.status === 'open') cancelCharge(p);
    stopPolling(p);
    payments = payments.filter((x) => x !== p);
  }
  function stopPolling(p) {
    clearInterval(timers[p.key]);
    delete timers[p.key];
  }

  async function sendToTerminal(p) {
    error = '';
    const cents = toCents(p.amount || '');
    if (!cents || cents < 100) return (error = 'El monto para la terminal debe ser de al menos $1.00');
    try {
      const c = await api('/api/payments/point/charges', { method: 'POST', body: { branch_id: branches.id, amount_cents: cents } });
      p.charge = { id: c.id, status: 'open', detail: '' };
      timers[p.key] = setInterval(() => poll(p), 2000);
    } catch (err) {
      error = err.message;
    }
  }
  async function poll(p) {
    if (!p.charge) return;
    try {
      const c = await api(`/api/payments/point/charges/${p.charge.id}`);
      p.charge = { ...p.charge, status: c.status, detail: c.detail || '' };
      if (c.status !== 'open') stopPolling(p);
    } catch {
      /* se reintenta en el siguiente ciclo */
    }
  }
  async function cancelCharge(p) {
    try {
      await api(`/api/payments/point/charges/${p.charge.id}/cancel`, { method: 'POST' });
      stopPolling(p);
      p.charge = null;
    } catch (err) {
      error = err.message;
    }
  }

  // ── cobrar
  async function charge() {
    error = '';
    busy = true;
    try {
      const body = {
        branch_id: branches.id,
        client_id: client?.id ?? null,
        appointment_id: appointmentId,
        client_token: token,
        note,
        discount_cents: discountCents,
        tip_cents: tipCents,
        tip_staff_id: tipCents && tipStaffId ? Number(tipStaffId) : null,
        lines: lines.map((l) => ({
          [l.kind + '_id']: l.id,
          qty: l.qty,
          staff_id: l.staffId ? Number(l.staffId) : null
        })),
        payments: payments.map((p) => ({
          method: p.method,
          amount_cents: toCents(p.amount),
          received_cents: p.method === 'cash' && p.received ? toCents(p.received) : null,
          reference: p.reference,
          mp_charge_id: p.method === 'mp_point' ? p.charge?.id : undefined,
          client_package_id: p.method === 'package' ? p.cpId : undefined
        }))
      };
      receipt = await api('/api/sales', { method: 'POST', body });
      ticketOpen = false;
      resetTicket();
      await load();
    } catch (err) {
      error = err.message;
      if (err.message.includes('abre la caja')) await load();
    } finally {
      busy = false;
    }
  }

  function resetTicket() {
    Object.values(timers).forEach(clearInterval);
    timers = {};
    lines = [];
    payments = [];
    client = null;
    appointmentId = null;
    discount = '';
    tipMode = 'none';
    tipCustom = '';
    tipStaffId = '';
    note = '';
    token = newToken();
  }

  const shown = $derived(
    (tab === 'services' ? services : products).filter((x) => !q.trim() || x.name.toLowerCase().includes(q.trim().toLowerCase()))
  );
</script>

<div class="page-head">
  <div>
    <h1>Cobrar</h1>
    <p>{#if cash}Caja abierta · efectivo esperado {money(cash.expected_cents)} · {plural(cash.sales, 'venta', 'ventas')}{:else}Abre la caja para empezar a cobrar.{/if}</p>
  </div>
  <div class="form-actions" style="margin: 0">
    <BranchPicker onchange={load} />
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}

{#if !ready}
  <p class="meta">Cargando…</p>
{:else if !branches.id}
  <div class="empty">No hay sucursales activas.</div>
{:else if receipt}
  <div class="done">
    <p class="banner" role="status" style="background: var(--accent-soft); color: var(--accent)">Venta #{receipt.folio} registrada ✓</p>
    <Receipt sale={receipt} business={session.me.business.name} />
    <div class="form-actions" style="justify-content: center; margin-top: 1rem">
      <button class="btn" onclick={() => (receipt = null)}>Nueva venta</button>
      <button class="btn ghost" onclick={() => window.print()}>Imprimir</button>
    </div>
  </div>
{:else if !cash}
  <form class="card open" onsubmit={openCash}>
    <h2>Caja cerrada</h2>
    <p class="meta">Cuenta el efectivo con el que empiezas y ábrela para cobrar.</p>
    <label class="field">Efectivo inicial (MXN) <input inputmode="decimal" bind:value={opening} placeholder="0.00" /></label>
    <button class="btn">Abrir caja</button>
  </form>
{:else}
  <div class="pos">
    <!-- catálogo -->
    <section class="catalog">
      <div class="tabs" role="tablist">
        {#each [['services', 'Servicios'], ['products', 'Productos'], ['packages', 'Paquetes'], ['appts', `Citas de hoy (${appts.length})`]] as [value, label]}
          <button class="chip" role="tab" aria-selected={tab === value} aria-pressed={tab === value} onclick={() => (tab = value)}>{label}</button>
        {/each}
      </div>
      {#if tab === 'packages'}
        {#if !client}<p class="meta">Elige primero al cliente en el ticket para venderle un paquete.</p>{/if}
        <div class="grid">
          {#each packages as k (k.id)}
            <button class="item" disabled={!client} onclick={() => addPackage(k)}>
              <b>{k.name}</b>
              <span>{money(k.price_cents)}</span>
              <small>{k.sessions} sesiones de {k.service_name}{k.valid_days ? ` · ${k.valid_days} días` : ''}</small>
            </button>
          {:else}
            <p class="meta">No hay paquetes. El administrador los crea en Servicios.</p>
          {/each}
        </div>
      {:else if tab !== 'appts'}
        <input type="search" placeholder="Buscar…" aria-label="Buscar en el catálogo" bind:value={q} style="margin-bottom: 0.75rem" />
        <div class="grid">
          {#each shown as it (it.id)}
            <button class="item" disabled={tab === 'products' && it.stock < 1} onclick={() => (tab === 'services' ? addService(it) : addProduct(it))}>
              <b>{it.name}</b>
              <span>{money(it.price_cents)}</span>
              {#if tab === 'products'}<small>{it.stock < 1 ? 'Agotado' : `${it.stock} en existencia`}</small>{:else}<small>{it.duration_min} min</small>{/if}
            </button>
          {:else}
            <p class="meta">No hay {tab === 'services' ? 'servicios' : 'productos'} para mostrar.</p>
          {/each}
        </div>
      {:else}
        <div class="list">
          {#each appts as a (a.id)}
            <button class="item-row appt" onclick={() => addAppointment(a)}>
              <span class="grow"><b>{fmtTime(a.starts_at, tz)} · {a.client_name}</b><small class="meta">{a.service_name} · con {a.staff_name}</small></span>
              <span>{money(a.price_cents)}</span>
              {#if a.deposit_status === 'paid'}<span class="badge">Anticipo {money(a.deposit_cents)} ✓</span>{/if}
              {#if a.status === 'completed'}<span class="badge muted">Completada</span>{/if}
            </button>
          {:else}
            <p class="meta">No hay citas para hoy en esta sucursal.</p>
          {/each}
        </div>
      {/if}
    </section>

    <!-- ticket -->
    <button class="scrim" class:open={ticketOpen} aria-label="Cerrar el ticket" tabindex="-1" onclick={() => (ticketOpen = false)}></button>
    <section class="ticket card" class:open={ticketOpen} aria-label="Ticket">
      <div class="sheet-head">
        <h2>Ticket</h2>
        <button class="btn ghost sm close" onclick={() => (ticketOpen = false)}>Seguir agregando</button>
      </div>

      <div class="client">
        {#if client}
          <div class="item-row"><span class="grow title">{client.name}</span>{#if !appointmentId}<button class="btn ghost sm" onclick={() => (client = null)}>Quitar</button>{/if}</div>
        {:else}
          <input type="search" placeholder="Cliente (opcional)" aria-label="Buscar cliente" bind:value={clientQuery} oninput={searchClient} style="margin: 0" />
          {#each clientResults as c (c.id)}<button class="pick" onclick={() => ((client = c), (clientResults = []), (clientQuery = ''))}>{c.name}<small>{c.phone}</small></button>{/each}
        {/if}
      </div>

      {#if lines.length === 0}
        <p class="empty" style="padding: 1.5rem 1rem">Toca un servicio, producto o cita para agregarlo.</p>
      {:else}
        <ul class="lines">
          {#each lines as l (l.key)}
            <li>
              <div class="l-top">
                <b>{l.name}</b>
                <span class="amt">{money(l.qty * l.unit)}</span>
              </div>
              <div class="l-bot">
                <div class="qty" aria-label="Cantidad" hidden={l.kind === 'package'}>
                  <button aria-label="Menos" onclick={() => (l.qty > 1 ? l.qty-- : removeLine(l))}>−</button>
                  <span>{l.qty}</span>
                  <button aria-label="Más" disabled={l.qty >= maxQty(l)} onclick={() => l.qty++}>+</button>
                </div>
                {#if l.kind === 'package'}
                  <small class="meta">Se le abona al cliente al cobrar</small>
                {:else if l.kind === 'service'}
                  <select bind:value={l.staffId} aria-label="Quién lo realizó" style="width: auto; margin: 0; min-height: 36px">
                    <option value="">Sin asignar</option>
                    {#each staff as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
                  </select>
                {/if}
                <button class="btn ghost sm" onclick={() => removeLine(l)}>Quitar</button>
              </div>
            </li>
          {/each}
        </ul>

        <div class="sums">
          <div><span>Subtotal</span><span>{money(subtotal)}</span></div>
          <label class="disc"><span>Descuento (MXN)</span><input inputmode="decimal" bind:value={discount} placeholder="0.00" aria-label="Descuento" /></label>
          <div class="strong"><span>Total</span><span>{money(total)}</span></div>
        </div>

        <fieldset class="tip">
          <legend>Propina</legend>
          <div class="chips" style="margin: 0">
            {#each [['none', 'Sin'], ['10', '10 %'], ['15', '15 %'], ['20', '20 %'], ['custom', 'Otra']] as [v, label]}
              <button class="chip" aria-pressed={tipMode === v} onclick={() => (tipMode = v)}>{label}</button>
            {/each}
          </div>
          {#if tipMode === 'custom'}<input inputmode="decimal" placeholder="Monto" aria-label="Propina" bind:value={tipCustom} style="margin-top: 8px" />{/if}
          {#if tipCents > 0 && staffIds.length > 1}
            <label class="field" style="margin: 8px 0 0">Para quién
              <select bind:value={tipStaffId}>
                <option value="">Sin asignar</option>
                {#each staff.filter((s) => staffIds.includes(s.id)) as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
              </select>
            </label>
          {/if}
          {#if tipCents > 0}<p class="meta" style="margin: 6px 0 0">Propina: {money(tipCents)}</p>{/if}
        </fieldset>

        <div class="due"><span>A cobrar</span><strong>{money(due)}</strong></div>

        <div class="pay">
          <h3>Pagos</h3>
          <div class="chips">
            {#each ['cash', 'card', 'transfer'] as m}<button class="chip" onclick={() => addPayment(m)}>+ {METHODS[m]}</button>{/each}
            {#each clientPkgs as k (k.id)}
              <button class="chip" disabled={!pkgUsable(k)} title={`Quedan ${k.remaining} sesiones`} onclick={() => addPackagePayment(k)}>+ {k.name} ({k.remaining})</button>
            {/each}
            <button class="chip" disabled={!point.ok} title={point.ok ? 'Cobrar con la terminal' : point.message} onclick={() => addPayment('mp_point')}>+ Terminal MP</button>
          </div>
          {#if !point.ok && point.message}<p class="meta" style="margin: -0.4rem 0 0.6rem">Terminal no disponible: {point.message}</p>{/if}

          {#each payments as p (p.key)}
            <div class="prow">
              <div class="prow-head">
                <b>{METHODS[p.method]}</b>
                {#if !p.locked}<button class="btn ghost sm" onclick={() => removePayment(p)}>Quitar</button>{/if}
              </div>
              <label class="field mini">Monto
                <input inputmode="decimal" bind:value={p.amount} disabled={p.charge !== null || p.locked || p.fixed} />
              </label>
              {#if p.method === 'package'}
                <p class="ok" role="status">✓ Una sesión de «{p.name}»</p>
              {:else if p.method === 'deposit'}
                <p class="ok" role="status">✓ Anticipo pagado en línea. Quita la cita del ticket para no descontarlo.</p>
              {:else if p.method === 'cash'}
                <label class="field mini">Recibido
                  <input inputmode="decimal" bind:value={p.received} placeholder="Exacto" />
                </label>
                {#if p.received && (toCents(p.received) || 0) >= (toCents(p.amount || '0') || 0)}<p class="meta">Cambio: <b>{money((toCents(p.received) || 0) - (toCents(p.amount || '0') || 0))}</b></p>{/if}
              {:else if p.method === 'mp_point'}
                {#if !p.charge}
                  <button class="btn sm" onclick={() => sendToTerminal(p)}>Enviar a la terminal</button>
                {:else if p.charge.status === 'open'}
                  <p class="meta wait" role="status">Esperando el pago en la terminal…</p>
                  <button class="btn ghost sm" onclick={() => cancelCharge(p)}>Cancelar cobro</button>
                {:else if p.charge.status === 'approved'}
                  <p class="ok" role="status">✓ Pago aprobado en la terminal</p>
                {:else}
                  <p class="error" role="alert" style="margin: 0.4rem 0">{p.charge.detail || 'El cobro no se completó.'}</p>
                  <button class="btn ghost sm" onclick={() => (p.charge = null)}>Intentar de nuevo</button>
                {/if}
              {:else}
                <label class="field mini">Referencia (opcional)
                  <input bind:value={p.reference} maxlength="100" />
                </label>
              {/if}
            </div>
          {/each}

          {#if payments.length}
            <p class="balance" class:bad={remaining !== 0}>
              {#if remaining > 0}Faltan {money(remaining)}{:else if remaining < 0}Sobran {money(-remaining)}{:else}Pagos completos ✓{#if change > 0} · cambio {money(change)}{/if}{/if}
            </p>
          {/if}
        </div>

        <label class="field" style="margin: 0.75rem 0 0">Nota (opcional) <input bind:value={note} maxlength="500" /></label>
        <button class="btn block lg" style="margin-top: 0.75rem" disabled={!canPay} onclick={charge}>{busy ? 'Cobrando…' : `Cobrar ${money(due)}`}</button>
        {#if mpPending}<p class="meta" style="text-align: center; margin: 0.5rem 0 0">Espera a que la terminal apruebe el pago.</p>{/if}
      {/if}
    </section>

    {#if lines.length > 0}
      <button class="ticketbar" onclick={() => (ticketOpen = true)}>
        <span class="tb-count">{plural(lines.reduce((n, l) => n + l.qty, 0), 'artículo', 'artículos')}</span>
        <span class="tb-total">{money(due)}</span>
        <span class="tb-go">Ver ticket y cobrar</span>
      </button>
    {/if}
  </div>
{/if}

<style>
  .item-row.appt small { display: block; }
  .open { max-width: 420px; }
  .pos { display: grid; gap: 16px; grid-template-columns: 1fr; align-items: start; }
  @media (min-width: 760px) { .pos { grid-template-columns: 1.1fr 0.9fr; } .ticket { position: sticky; top: 16px; } }

  /* celular: barra con el total siempre a la vista y el ticket como hoja */
  .ticketbar, .scrim, .sheet-head .close { display: none; }
  .sheet-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
  @media (max-width: 759px) {
    .ticketbar {
      position: fixed; inset: auto 12px calc(72px + env(safe-area-inset-bottom)) 12px; z-index: 35;
      display: grid; grid-template-columns: auto 1fr; align-items: center; gap: 0 12px; padding: 0.6rem 1rem; text-align: left;
      font: inherit; color: var(--accent-contrast); background: var(--accent); border: 0; border-radius: var(--radius); box-shadow: var(--shadow); cursor: pointer;
      transition: transform var(--dur-press) var(--ease);
    }
    .ticketbar:active { transform: scale(0.98); }
    .tb-count { font-size: 0.8rem; opacity: 0.85; }
    .tb-total { grid-column: 2; grid-row: 1 / 3; justify-self: end; font-size: 1.5rem; font-weight: 700; font-variant-numeric: tabular-nums; }
    .tb-go { font-weight: 600; }
    .scrim { display: block; position: fixed; inset: 0; z-index: 55; border: 0; padding: 0; background: rgba(20, 18, 14, 0.45); opacity: 0; pointer-events: none; transition: opacity 200ms var(--ease); }
    .scrim.open { opacity: 1; pointer-events: auto; }
    .ticket {
      position: fixed; inset: auto 0 0 0; z-index: 60; max-height: 90vh; overflow-y: auto; border-radius: 16px 16px 0 0; padding-bottom: calc(1.5rem + env(safe-area-inset-bottom));
      transform: translateY(105%); visibility: hidden; transition: transform 260ms var(--ease), visibility 0s linear 260ms; box-shadow: var(--shadow);
    }
    .ticket.open { transform: none; visibility: visible; transition: transform 260ms var(--ease); }
    .sheet-head .close { display: inline-flex; }
    .done, .pos { padding-bottom: 88px; }
  }
  @media (prefers-reduced-motion: reduce) { .ticket, .ticket.open { transition: none; } }

  .tabs { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 0.75rem; }
  .tabs .chip { margin: 0; }
  .grid { display: grid; gap: 10px; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); }
  .item {
    display: grid; gap: 2px; min-height: 84px; padding: 0.75rem; text-align: left; font: inherit; color: var(--text);
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); cursor: pointer;
    transition: border-color 0.2s var(--ease), transform 0.15s var(--ease);
  }
  .item:hover:not(:disabled) { border-color: var(--accent); }
  .item:active:not(:disabled) { transform: scale(0.98); }
  .item:disabled { opacity: 0.5; cursor: not-allowed; }
  .item span { color: var(--accent); font-weight: 600; }
  .item small { color: var(--muted); }
  .appt { width: 100%; text-align: left; font: inherit; color: var(--text); cursor: pointer; }
  .appt:hover { border-color: var(--accent); }

  .ticket h2 { font-size: 1.4rem; margin-bottom: 0.75rem; }
  .ticket h3 { font-size: 1.05rem; margin: 0 0 0.5rem; }
  .client { margin-bottom: 0.75rem; }
  .pick { display: flex; justify-content: space-between; width: 100%; min-height: 40px; align-items: center; padding: 0 0.8rem; margin-top: 4px; font: inherit; color: var(--text); background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; }
  .pick small { color: var(--muted); }

  .lines { list-style: none; margin: 0 0 0.75rem; padding: 0; display: grid; gap: 8px; }
  .lines li { padding: 0.65rem 0.75rem; background: var(--surface-2); border-radius: var(--radius-sm); }
  .l-top { display: flex; justify-content: space-between; gap: 8px; }
  .amt { font-variant-numeric: tabular-nums; font-weight: 600; }
  .l-bot { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin-top: 6px; }
  .qty { display: inline-flex; align-items: center; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface); }
  .qty button { width: 36px; height: 36px; font-size: 1.2rem; color: var(--text); background: none; border: 0; cursor: pointer; }
  .qty button:disabled { opacity: 0.4; cursor: not-allowed; }
  .qty span { min-width: 28px; text-align: center; font-variant-numeric: tabular-nums; }

  .sums { display: grid; gap: 6px; padding: 0.5rem 0; border-top: 1px solid var(--border); font-variant-numeric: tabular-nums; }
  .sums > div, .disc { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
  .disc input { width: 120px; margin: 0; min-height: 38px; text-align: right; }
  .strong { font-weight: 700; font-size: 1.05rem; }

  .tip { margin: 0.5rem 0; padding: 0.6rem 0.8rem 0.8rem; border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .tip legend { padding: 0 6px; font-size: 0.85rem; font-weight: 500; }
  .due { display: flex; justify-content: space-between; align-items: baseline; margin: 0.75rem 0; padding: 0.75rem 1rem; background: var(--accent-soft); color: var(--accent); border-radius: var(--radius-sm); }
  .due strong { font-family: var(--font-heading); font-size: 1.8rem; }

  .prow { padding: 0.7rem 0.8rem; margin-bottom: 8px; border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .prow-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
  .field.mini { margin: 0 0 6px; }
  .field.mini input { min-height: 40px; }
  .ok { color: var(--accent); font-weight: 600; margin: 0.4rem 0 0; }
  .wait::after { content: ''; display: inline-block; width: 10px; height: 10px; margin-left: 8px; border: 2px solid var(--muted); border-top-color: transparent; border-radius: 50%; animation: spin 0.9s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .balance { margin: 0.5rem 0 0; font-weight: 600; color: var(--accent); }
  .balance.bad { color: var(--danger); }

  .done { max-width: 440px; margin: 0 auto; }
</style>
