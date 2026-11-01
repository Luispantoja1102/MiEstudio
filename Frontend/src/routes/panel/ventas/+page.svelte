<script>
  import { onMount, onDestroy } from 'svelte';
  import BranchPicker from '$lib/BranchPicker.svelte';
  import { autoPrints, clearPrintIssue, printInBrowser, printSale, printing, retryPrint } from '$lib/pos/printing.svelte.js';
  import PayDialog from '$lib/pos/PayDialog.svelte';
  import Modal from '$lib/Modal.svelte';
  import Icon from '$lib/Icon.svelte';
  import EmptyState from '$lib/ui/EmptyState.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import LoadingState from '$lib/ui/LoadingState.svelte';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import SegmentedControl from '$lib/ui/SegmentedControl.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import { api } from '$lib/api.js';
  import { money, newToken, toCents, toPesos, plural } from '$lib/format.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { fmtTime, todayIn } from '$lib/time.js';
  import { session } from '$lib/session.svelte.js';
  import { page } from '$app/state';

  const tz = session.me.business.timezone || 'America/Tijuana';

  // ── datos
  let cash = $state(null);
  let ready = $state(false);
  let opening = $state('');
  let services = $state([]);
  let products = $state([]);
  let packages = $state([]); // catálogo de paquetes
  let clientPkgs = $state([]); // paquetes vigentes del cliente del ticket
  let loyalty = $state(null); // tarjeta de lealtad del cliente del ticket (null si está apagada)
  let loyaltyMsg = $state('');
  let staff = $state([]);
  let appts = $state([]);
  let point = $state({ ok: false, message: '', code: '' });
  let cats = $state([]); // categorías de servicios
  let catPick = $state('all'); // all | none | id
  let tab = $state('services');
  let q = $state('');
  let hit = $state(0); // resultado resaltado en la búsqueda

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
  let prepaid = $state([]); // lo que ya está pagado: anticipo en línea y sesiones de paquete
  let token = $state(newToken());
  let error = $state('');
  let receipt = $state(null);
  let ticketOpen = $state(false); // celular: el ticket es una hoja que sube desde abajo
  let selKey = $state(null); // renglón seleccionado (F2 lo quita, F3 le cambia la cantidad)
  let payOpen = $state(false);
  let clientTimer;
  let searchEl = $state();

  // ventanas
  let discOpen = $state(false);
  let discDraft = $state('');
  let qtyOpen = $state(false);
  let qtyDraft = $state('');
  let heldOpen = $state(false);
  let held = $state([]);

  // ── cálculos (todo en centavos)
  const subtotal = $derived(lines.reduce((n, l) => n + l.qty * l.unit, 0));
  const discountCents = $derived(Math.min(Math.max(toCents(discount || '0') || 0, 0), subtotal));
  const total = $derived(subtotal - discountCents);
  const tipCents = $derived(tipMode === 'none' ? 0 : tipMode === 'custom' ? Math.max(toCents(tipCustom || '0') || 0, 0) : Math.round((total * Number(tipMode)) / 100));
  const due = $derived(total + tipCents);
  const prepaidCents = $derived(prepaid.reduce((n, p) => n + (toCents(p.amount || '0') || 0), 0));
  const toPay = $derived(due - prepaidCents);
  const items = $derived(lines.reduce((n, l) => n + l.qty, 0));
  const staffIds = $derived([...new Set(lines.filter((l) => l.kind === 'service' && l.staffId).map((l) => l.staffId))]);
  const canPay = $derived(lines.length > 0 && toPay >= 0);

  async function load() {
    ready = false;
    const branchId = branches.id;
    if (!branchId) return (ready = true);
    const [c, s, p, a, st, pk, ct] = await Promise.all([
      api(`/api/cash/current?branch_id=${branchId}`),
      api('/api/services'),
      api('/api/products'),
      api('/api/agenda?' + new URLSearchParams({ branch_id: branchId, date: todayIn(tz) })),
      api(`/api/payments/point/status?branch_id=${branchId}`),
      api('/api/packages'),
      api('/api/service-categories')
    ]);
    packages = pk;
    cats = ct;
    cash = c.session;
    services = s.filter((x) => x.active);
    products = p.filter((x) => x.active);
    staff = a.staff;
    appts = a.appointments.filter((x) => x.status === 'scheduled' || x.status === 'completed');
    point = st;
    held = loadHeld();
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
    window.addEventListener('keydown', onKey);
    try {
      await loadBranches();
      await load();
    } catch (e) {
      error = e.message;
      ready = true;
    }
  });
  onDestroy(() => window.removeEventListener('keydown', onKey));

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
  const newKey = () => crypto.randomUUID?.() ?? String(Math.random());
  function defaultStaff(s) {
    const eligible = s.staff_ids?.length ? staff.filter((x) => s.staff_ids.includes(x.id)) : staff;
    return eligible.length === 1 ? eligible[0].id : '';
  }
  function addService(s) {
    error = '';
    const same = lines.find((l) => l.kind === 'service' && l.id === s.id && !l.fromAppt && l.staffId === defaultStaff(s));
    if (same) {
      if (same.qty < maxQty(same)) same.qty++;
      return (selKey = same.key);
    }
    const line = { key: newKey(), kind: 'service', id: s.id, name: s.name, unit: s.price_cents, qty: 1, staffId: defaultStaff(s) };
    lines.push(line);
    selKey = line.key;
  }
  function addProduct(p) {
    error = '';
    const found = lines.find((l) => l.kind === 'product' && l.id === p.id);
    if (found) {
      if (found.qty < p.stock) found.qty++;
      return (selKey = found.key);
    }
    if (p.stock < 1) return;
    const line = { key: newKey(), kind: 'product', id: p.id, name: p.name, unit: p.price_cents, qty: 1, staffId: '', stock: p.stock };
    lines.push(line);
    selKey = line.key;
  }
  function addPackage(k) {
    if (!client) return (error = 'Para vender un paquete elige primero al cliente.');
    if (lines.some((l) => l.kind === 'package' && l.id === k.id)) return;
    const line = { key: newKey(), kind: 'package', id: k.id, name: k.name, unit: k.price_cents, qty: 1, staffId: '' };
    lines.push(line);
    selKey = line.key;
  }
  function addAppointment(a) {
    if (appointmentId && appointmentId !== a.id && !confirm('Ya hay una cita en el ticket. ¿Cambiarla por esta?')) return;
    lines = lines.filter((l) => !l.fromAppt);
    appointmentId = a.id;
    client = { id: a.client_id, name: a.client_name };
    lines.unshift({ key: 'appt-' + a.id, kind: 'service', id: a.service_id, name: a.service_name, unit: a.price_cents, qty: 1, staffId: a.staff_id, fromAppt: true });
    // el anticipo pagado en línea se descuenta solo (y es obligatorio aplicarlo)
    prepaid = prepaid.filter((p) => p.method !== 'deposit');
    if (a.deposit_status === 'paid') {
      prepaid.unshift({ key: 'deposit-' + a.id, method: 'deposit', amount: toPesos(a.deposit_cents) });
    }
  }
  function removeLine(l) {
    lines = lines.filter((x) => x !== l);
    if (l.fromAppt) {
      appointmentId = null;
      prepaid = prepaid.filter((p) => p.method !== 'deposit');
    }
  }
  const maxQty = (l) => (l.kind === 'service' ? 20 : l.kind === 'package' ? 1 : (l.stock ?? 1000));
  function bump(l, d) {
    if (l.qty + d < 1) return removeLine(l);
    l.qty = Math.min(l.qty + d, maxQty(l));
    selKey = l.key;
  }
  function clearTicket() {
    if (lines.length && !confirm('¿Vaciar el ticket?')) return;
    resetTicket();
  }

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
  // tarjeta de lealtad del cliente: sellos y, si ya completó, el canje del premio
  $effect(() => {
    const cid = client?.id;
    loyaltyMsg = '';
    if (!cid) {
      loyalty = null;
      return;
    }
    api(`/api/clients/${cid}/loyalty`)
      .then((l) => client?.id === cid && (loyalty = l.enabled ? l : null))
      .catch(() => (loyalty = null));
  });
  async function redeemReward() {
    if (!confirm(`¿Canjear el premio «${loyalty.reward}»? Después aplica el descuento con «Descuento».`)) return;
    try {
      loyalty = await api(`/api/clients/${client.id}/loyalty/redeem`, { method: 'POST' });
      loyaltyMsg = `Premio canjeado: ${loyalty.reward}. Aplica el descuento al servicio con «Descuento».`;
    } catch (err) {
      error = err.message;
    }
  }
  const unitOf = (serviceId) => lines.find((l) => l.kind === 'service' && l.id === serviceId)?.unit ?? 0;
  const serviceQty = (serviceId) => lines.filter((l) => l.kind === 'service' && l.id === serviceId).reduce((n, l) => n + l.qty, 0);
  const pkgUsed = (k) => prepaid.filter((p) => p.method === 'package' && p.cpId === k.id).length;
  const pkgUsable = (k) => pkgUsed(k) < k.remaining && prepaid.filter((p) => p.method === 'package' && p.serviceId === k.service_id).length < serviceQty(k.service_id);
  // si se quita el servicio o al cliente, los canjes sobrantes salen del ticket
  $effect(() => {
    const seen = {};
    const keep = prepaid.filter((p) => {
      if (p.method !== 'package') return true;
      seen[p.serviceId] = (seen[p.serviceId] ?? 0) + 1;
      return client && seen[p.serviceId] <= serviceQty(p.serviceId);
    });
    if (keep.length !== prepaid.length) prepaid = keep;
  });
  $effect(() => {
    if (!client && lines.some((l) => l.kind === 'package')) lines = lines.filter((l) => l.kind !== 'package');
  });
  function addPackagePayment(k) {
    error = '';
    prepaid.push({ key: newKey(), method: 'package', cpId: k.id, serviceId: k.service_id, name: k.name, amount: toPesos(unitOf(k.service_id)) });
  }

  // ── cobrar
  function startPay() {
    if (!canPay) return;
    error = '';
    ticketOpen = false;
    payOpen = true;
  }
  // el diálogo entrega lo que se cobra ahora; aquí se le suma lo ya pagado (anticipo, paquetes)
  async function sell(extra) {
    const body = {
      branch_id: branches.id,
      client_id: client?.id ?? null,
      appointment_id: appointmentId,
      client_token: token,
      note,
      discount_cents: discountCents,
      tip_cents: tipCents,
      tip_staff_id: tipCents && tipStaffId ? Number(tipStaffId) : null,
      lines: lines.map((l) => ({ [l.kind + '_id']: l.id, qty: l.qty, staff_id: l.staffId ? Number(l.staffId) : null })),
      payments: [
        ...prepaid.map((p) => ({ method: p.method, amount_cents: toCents(p.amount), client_package_id: p.method === 'package' ? p.cpId : undefined })),
        ...extra
      ]
    };
    try {
      receipt = await api('/api/sales', { method: 'POST', body });
    } catch (err) {
      if (err.message.includes('abre la caja')) {
        payOpen = false;
        await load();
      }
      throw err;
    }
    printSale(receipt); // sale sola si este dispositivo tiene térmica o estación; los fallos quedan en printing.issue
    resetTicket();
    await load();
    searchEl?.focus();
  }

  function resetTicket() {
    lines = [];
    prepaid = [];
    client = null;
    appointmentId = null;
    discount = '';
    tipMode = 'none';
    tipCustom = '';
    tipStaffId = '';
    note = '';
    selKey = null;
    token = newToken();
  }

  // ── tickets en espera (se guardan en este equipo, por negocio y sucursal)
  const heldKey = () => `pos_held_${session.me.business.id ?? 0}_${branches.id}`;
  function loadHeld() {
    try {
      const list = JSON.parse(localStorage.getItem(heldKey()) || '[]');
      return Array.isArray(list) ? list.filter((t) => t && Array.isArray(t.lines)) : [];
    } catch {
      return [];
    }
  }
  function saveHeld() {
    try {
      localStorage.setItem(heldKey(), JSON.stringify(held));
    } catch {
      /* sin almacenamiento: la lista sigue en memoria */
    }
  }
  const snapshot = () => $state.snapshot({ at: Date.now(), lines, client, appointmentId, discount, tipMode, tipCustom, tipStaffId, note, prepaid });
  function holdTicket() {
    if (!lines.length) return openHeld();
    held = [{ id: 'h' + Date.now().toString(36), ...snapshot() }, ...held].slice(0, 20);
    saveHeld();
    resetTicket();
    ticketOpen = false;
  }
  function openHeld() {
    ticketOpen = false;
    heldOpen = true;
  }
  function resumeHeld(id) {
    const t = held.find((x) => x.id === id);
    if (!t) return;
    // lo que había en el ticket pasa a la espera para no perderlo
    const rest = held.filter((x) => x.id !== id);
    held = lines.length ? [{ id: 'h' + Date.now().toString(36), ...snapshot() }, ...rest] : rest;
    saveHeld();
    resetTicket();
    ({ lines, client, appointmentId, discount, tipMode, tipCustom, tipStaffId, note, prepaid } = {
      lines: [], client: null, appointmentId: null, discount: '', tipMode: 'none', tipCustom: '', tipStaffId: '', note: '', prepaid: [], ...t
    });
    receipt = null;
    heldOpen = false;
  }
  function dropHeld(id) {
    if (!confirm('¿Descartar este ticket en espera?')) return;
    held = held.filter((x) => x.id !== id);
    saveHeld();
  }
  const heldTotal = (t) => t.lines.reduce((n, l) => n + l.qty * l.unit, 0);

  // la confirmación de la venta anterior se va en cuanto empieza otra
  $effect(() => {
    if (lines.length) {
      receipt = null;
      clearPrintIssue();
    }
  });

  // ── descuento (F9) y cantidad (F3)
  function openDiscount() {
    if (!lines.length) return;
    discDraft = discount;
    discOpen = true;
  }
  function applyDiscount(e) {
    e.preventDefault();
    const c = toCents(discDraft || '0');
    if (Number.isNaN(c)) return;
    discount = c ? toPesos(Math.min(c, subtotal)) : '';
    discOpen = false;
  }
  const discPct = (p) => (discDraft = toPesos(Math.round((subtotal * p) / 100)));
  const selected = $derived(lines.find((l) => l.key === selKey) ?? lines.at(-1));
  function openQty() {
    if (!selected || selected.kind === 'package') return;
    selKey = selected.key;
    qtyDraft = String(selected.qty);
    qtyOpen = true;
  }
  function applyQty(e) {
    e.preventDefault();
    const n = Math.floor(Number(qtyDraft));
    if (Number.isFinite(n) && n >= 1 && selected) selected.qty = Math.min(n, maxQty(selected));
    qtyOpen = false;
  }

  // ── buscador
  const catalog = $derived([
    ...services.map((x) => ({ ...x, kind: 'service' })),
    ...products.map((x) => ({ ...x, kind: 'product' })),
    ...(client ? packages.map((x) => ({ ...x, kind: 'package' })) : [])
  ]);
  const term = $derived(q.trim().toLowerCase());
  const results = $derived(term ? catalog.filter((x) => x.name.toLowerCase().includes(term) || x.barcode?.toLowerCase().includes(term)).slice(0, 30) : []);
  $effect(() => {
    term;
    hit = 0;
  });
  const inCat = (s) => catPick === 'all' || (catPick === 'none' ? s.category_id == null : s.category_id === catPick);
  const shown = $derived(tab === 'services' ? services.filter(inCat) : products);
  const qtyIn = (kind, id) => lines.filter((l) => l.kind === kind && l.id === id).reduce((n, l) => n + l.qty, 0);
  const initial = (name) => String(name || '?').trim().charAt(0).toUpperCase();
  const isOut = (x) => x.kind === 'product' && x.stock < 1;
  function add(x) {
    if (x.kind === 'service') addService(x);
    else if (x.kind === 'product') addProduct(x);
    else addPackage(x);
  }
  function pick(x) {
    add(x);
    q = '';
  }
  function onSearchKey(e) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (results.length) hit = (hit + (e.key === 'ArrowDown' ? 1 : -1) + results.length) % results.length;
    } else if (e.key === 'Enter') {
      e.preventDefault();
      // un lector de códigos teclea el código y Enter: si coincide exacto, se agrega ese producto
      const scanned = catalog.find((x) => x.barcode && x.barcode.toLowerCase() === term);
      const chosen = scanned ?? results[hit];
      if (!chosen) return;
      if (isOut(chosen)) error = `${chosen.name} está agotado.`;
      else pick(chosen);
    } else if (e.key === 'Escape') {
      q = '';
    }
  }

  // ── atajos de teclado (equipo de escritorio)
  const noDialog = () => !payOpen && !discOpen && !qtyOpen && !heldOpen;
  function onKey(e) {
    if (!cash || !noDialog()) return;
    const typing = ['INPUT', 'TEXTAREA', 'SELECT'].includes(e.target?.tagName);
    const run = (fn) => (e.preventDefault(), fn());
    if (e.key === 'F2') run(() => selected && removeLine(selected));
    else if (e.key === 'F3') run(openQty);
    else if (e.key === 'F6') run(holdTicket);
    else if (e.key === 'F9') run(openDiscount);
    else if (e.key === 'F12') run(startPay);
    else if (!typing && (e.key === 'Delete' || e.key === 'Backspace') && selected) run(() => removeLine(selected));
  }
</script>

<PageHeader title="Cobrar" description={cash ? `Caja abierta · efectivo esperado ${money(cash.expected_cents)} · ${plural(cash.sales, 'venta', 'ventas')}` : 'Abre la caja para empezar a cobrar.'}>
  {#snippet actions()}<BranchPicker onchange={load} />{/snippet}
</PageHeader>

{#if error}<InlineNotice tone="danger" message={error} />{/if}

{#if !ready}
  <LoadingState />
{:else if !branches.id}
  <EmptyState title="No hay sucursales activas" description="Activa o crea una sucursal en Ajustes para poder cobrar." />
{:else if !cash}
  <div class="open">
    <Surface title="Caja cerrada" description="Cuenta el efectivo con el que empiezas y ábrela para cobrar.">
      <form onsubmit={openCash}>
        <label class="field">Efectivo inicial (MXN) <input inputmode="decimal" bind:value={opening} placeholder="0.00" /></label>
        <button type="submit" class="btn">Abrir caja</button>
      </form>
    </Surface>
  </div>
{:else}
  <div class="pos">
    <!-- catálogo -->
    <section class="catalog" aria-label="Catálogo">
      <Surface padding="compact">
        <div class="finder">
          <Icon name="search" size={18} />
          <input
            bind:this={searchEl}
            type="search"
            class="scan"
            placeholder="Buscar un servicio o producto…"
            aria-label="Buscar un servicio o producto"
            autocomplete="off"
            bind:value={q}
            onkeydown={onSearchKey}
          />
          {#if q}<button type="button" class="clear" aria-label="Limpiar búsqueda" onclick={() => (q = '')}><Icon name="x" size={16} /></button>{/if}
        </div>

        {#if term}
          <div class="items" role="listbox" aria-label="Resultados">
            <p class="meta head">
              <b>{results.length}</b> {results.length === 1 ? 'resultado' : 'resultados'} para “{q.trim()}”{#if results.length}<span class="only-pc"> · ↑ ↓ para elegir, Enter agrega</span>{/if}
            </p>
            {#each results as x, i (x.kind + x.id)}
              <button type="button" class="item" class:active={i === hit} role="option" aria-selected={i === hit} disabled={isOut(x)} onclick={() => pick(x)}>
                <span class="grow">
                  <b>{x.name}</b>
                  <small>{x.kind === 'service' ? `Servicio · ${x.duration_min} min` : x.kind === 'product' ? (x.stock < 1 ? 'Producto · agotado' : `Producto · ${x.stock} en existencia`) + (x.barcode ? ` · ${x.barcode}` : '') : `Paquete · ${x.sessions} sesiones`}</small>
                </span>
                {#if qtyIn(x.kind, x.id)}<em class="in-ticket">{qtyIn(x.kind, x.id)} en ticket</em>{/if}
                <span class="price">{money(x.price_cents)}</span>
              </button>
            {:else}
              <p class="meta none">No encontramos “{q.trim()}”. Revisa cómo está escrito.</p>
            {/each}
          </div>
        {:else}
          <div class="cat-bar">
            <SegmentedControl
              label="Qué cobrar"
              bind:value={tab}
              options={[
                { value: 'services', label: 'Servicios' },
                { value: 'products', label: 'Productos' },
                { value: 'packages', label: 'Paquetes' },
                { value: 'appts', label: `Citas de hoy (${appts.length})` }
              ]}
            />
            {#if tab === 'services' && cats.length}
              <SegmentedControl
                label="Categorías"
                bind:value={catPick}
                options={[
                  { value: 'all', label: 'Todas' },
                  ...cats.map((c) => ({ value: c.id, label: c.name })),
                  ...(services.some((x) => x.category_id == null) ? [{ value: 'none', label: 'Sin categoría' }] : [])
                ]}
              />
            {/if}
          </div>
          {#if tab === 'packages'}
            {#if !client}<p class="meta hint">Elige primero al cliente en el ticket para venderle un paquete.</p>{/if}
            <div class="items">
              {#each packages as k (k.id)}
                <button type="button" class="item" disabled={!client} onclick={() => addPackage(k)}>
                  <span class="grow"><b>{k.name}</b><small>{k.sessions} sesiones{k.valid_days ? ` · ${k.valid_days} días` : ''}</small></span>
                  {#if qtyIn('package', k.id)}<em class="in-ticket">En ticket</em>{/if}
                  <span class="price">{money(k.price_cents)}</span>
                </button>
              {:else}
                <p class="meta none">No hay paquetes. El administrador los crea en Servicios.</p>
              {/each}
            </div>
          {:else if tab === 'appts'}
            <div class="items">
              {#each appts as a (a.id)}
                <button type="button" class="item" onclick={() => addAppointment(a)}>
                  <span class="grow"><b>{fmtTime(a.starts_at, tz)} · {a.client_name}</b><small>{a.service_name} · con {a.staff_name}</small></span>
                  {#if a.deposit_status === 'paid'}<span class="badge">Anticipo {money(a.deposit_cents)} ✓</span>{/if}
                  {#if a.status === 'completed'}<span class="badge muted">Completada</span>{/if}
                  <span class="price">{money(a.price_cents)}</span>
                </button>
              {:else}
                <p class="meta none">No hay citas para hoy en esta sucursal.</p>
              {/each}
            </div>
          {:else}
            <div class="items">
              {#each shown as it (it.id)}
                {@const kind = tab === 'services' ? 'service' : 'product'}
                <button type="button" class="item" disabled={tab === 'products' && it.stock < 1} onclick={() => (tab === 'services' ? addService(it) : addProduct(it))}>
                  <span class="grow">
                    <b>{it.name}</b>
                    <small class:low={tab === 'products' && it.stock < 1}>{tab === 'products' ? (it.stock < 1 ? 'Agotado' : `${it.stock} en existencia`) : `${it.duration_min} min`}</small>
                  </span>
                  {#if qtyIn(kind, it.id)}<em class="in-ticket">{qtyIn(kind, it.id)} en ticket</em>{/if}
                  <span class="price">{money(it.price_cents)}</span>
                </button>
              {:else}
                <div class="none-box">
                  <p><strong>Aún no hay {tab === 'services' ? 'servicios' : 'productos'}.</strong></p>
                  <p class="meta">Da de alta tu catálogo para empezar a cobrar.</p>
                  <a class="btn ghost sm" href={tab === 'services' ? '/panel/servicios' : '/panel/inventario'}>Ir a {tab === 'services' ? 'Servicios' : 'Inventario'}</a>
                </div>
              {/each}
            </div>
          {/if}
        {/if}

        <p class="keys only-pc">
          <span><kbd>Enter</kbd>agrega</span><span><kbd>F2</kbd>quitar</span><span><kbd>F3</kbd>cantidad</span><span><kbd>F6</kbd>espera</span><span><kbd>F9</kbd>descuento</span><span><kbd>F12</kbd>cobrar</span>
        </p>
      </Surface>
    </section>

    <!-- ticket -->
    <button type="button" class="scrim" class:open={ticketOpen} aria-label="Cerrar el ticket" tabindex="-1" onclick={() => (ticketOpen = false)}></button>
    <aside class="ticket" class:open={ticketOpen} aria-label="Ticket de venta">
      <div class="t-top">
        <header class="t-head">
          <div class="t-title">
            <h2>Venta actual</h2>
            <p class="meta">{lines.length ? plural(items, 'artículo', 'artículos') : 'Ticket vacío'}</p>
          </div>
          <button type="button" class="head-btn" class:has={held.length} title="Tickets en espera (F6)" onclick={openHeld}>
            <Icon name="clock" size={18} /><span>En espera</span>{#if held.length}<b>{held.length}</b>{/if}
          </button>
          <button type="button" class="head-btn icon" title="Vaciar ticket" aria-label="Vaciar ticket" disabled={!lines.length} onclick={clearTicket}><Icon name="trash" size={18} /></button>
          <button type="button" class="head-btn icon close" title="Seguir agregando" aria-label="Seguir agregando" onclick={() => (ticketOpen = false)}><Icon name="x" size={18} /></button>
        </header>

        <div class="client">
          {#if client}
            <div class="client-on"><span class="grow title">{client.name}</span>{#if !appointmentId}<button type="button" class="btn ghost sm" onclick={() => (client = null)}>Quitar</button>{/if}</div>
          {:else}
            <input type="search" placeholder="Cliente (opcional)" aria-label="Buscar cliente" bind:value={clientQuery} oninput={searchClient} style="margin: 0" />
            {#each clientResults as c (c.id)}<button type="button" class="pick" onclick={() => ((client = c), (clientResults = []), (clientQuery = ''))}>{c.name}<small>{c.phone}</small></button>{/each}
          {/if}
        </div>

        {#if loyalty}
          <div class="loyal" class:ready={loyalty.ready}>
            {#if loyalty.ready}
              <span>Premio listo: <b>{loyalty.reward}</b></span>
              <button type="button" class="btn sm" onclick={redeemReward}>Canjear</button>
            {:else}
              <span>Tarjeta de lealtad: <b>{loyalty.stamps} de {loyalty.visits_required}</b> sellos</span>
            {/if}
          </div>
        {/if}
        {#if loyaltyMsg}<p class="ok-msg" role="status">{loyaltyMsg}</p>{/if}
      </div>

      {#if lines.length === 0}
        <div class="t-empty">
          {#if receipt}
            <span class="done-ico"><Icon name="check" size={26} /></span>
            <p><strong>Venta #{receipt.folio} registrada</strong></p>
            <p class="meta">{money(receipt.total_cents)}{receipt.change_cents ? ` · cambio ${money(receipt.change_cents)}` : ''}</p>
            {#if printing.issue}
              <div class="print-issue" role="alert">
                <p><strong>No se imprimió el ticket.</strong> {printing.issue} La venta sí quedó registrada.</p>
                <div>
                  <button type="button" class="btn ghost sm" disabled={printing.busy} onclick={retryPrint}>{printing.busy ? 'Imprimiendo…' : 'Reintentar'}</button>
                  <button type="button" class="btn ghost sm" onclick={() => printInBrowser(receipt)}>Imprimir con el navegador</button>
                </div>
              </div>
            {:else if autoPrints()}
              <button type="button" class="btn ghost sm" disabled={printing.busy} onclick={() => printSale(receipt)}><Icon name="printer" size={16} />{printing.busy ? 'Imprimiendo…' : 'Reimprimir ticket'}</button>
            {:else}
              <button type="button" class="btn ghost sm" onclick={() => printInBrowser(receipt)}><Icon name="printer" size={16} />Imprimir ticket</button>
            {/if}
          {:else}
            <p><strong>Listo para vender</strong></p>
            <p class="meta">Toca un servicio, producto o cita para agregarlo.</p>
          {/if}
        </div>
      {:else}
        <div class="t-body">
          <ul class="lines">
            {#each lines as l (l.key)}
              <li class:on={selected?.key === l.key}>
                <button type="button" class="l-top" onclick={() => (selKey = l.key)}>
                  <b>{l.name}</b>
                  <span class="amt">{money(l.qty * l.unit)}</span>
                </button>
                <p class="meta l-sub">{l.qty} × {money(l.unit)}{l.kind === 'package' ? ' · se le abona al cliente al cobrar' : ''}</p>
                <div class="l-bot">
                  {#if l.kind !== 'package'}
                    <div class="qty" role="group" aria-label="Cantidad">
                      <button type="button" aria-label="Una menos" onclick={() => bump(l, -1)}>−</button>
                      <button type="button" class="val" title="Escribir cantidad (F3)" onclick={() => ((selKey = l.key), openQty())}>{l.qty}</button>
                      <button type="button" aria-label="Una más" disabled={l.qty >= maxQty(l)} onclick={() => bump(l, 1)}>+</button>
                    </div>
                  {/if}
                  {#if l.kind === 'service'}
                    <select bind:value={l.staffId} aria-label="Quién lo realizó" class="who">
                      <option value="">Sin asignar</option>
                      {#each staff as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
                    </select>
                  {/if}
                  <button type="button" class="btn ghost sm" title="Quitar (F2)" onclick={() => removeLine(l)}><Icon name="trash" size={16} />Quitar</button>
                </div>
              </li>
            {/each}
          </ul>

          {#if clientPkgs.length}
            <div class="chips pk">
              {#each clientPkgs as k (k.id)}
                <button type="button" class="chip" disabled={!pkgUsable(k)} title={`Quedan ${k.remaining} sesiones`} onclick={() => addPackagePayment(k)}>Usar paquete: {k.name} ({k.remaining})</button>
              {/each}
            </div>
          {/if}

          <dl class="sums">
            <div><dt>Subtotal</dt><dd>{money(subtotal)}</dd></div>
            {#if discountCents}<div class="disc"><dt>Descuento</dt><dd>−{money(discountCents)}</dd></div>{/if}
            {#if tipCents}<div><dt>Propina</dt><dd>{money(tipCents)}</dd></div>{/if}
            {#each prepaid as p (p.key)}
              <div class="disc">
                <dt>{p.method === 'deposit' ? 'Anticipo pagado en línea' : `Paquete: ${p.name}`}{#if p.method === 'package'} <button type="button" class="link" onclick={() => (prepaid = prepaid.filter((x) => x !== p))}>Quitar</button>{/if}</dt>
                <dd>−{money(toCents(p.amount))}</dd>
              </div>
            {/each}
          </dl>
          {#if toPay < 0}<p class="error" role="alert" style="margin: 0">Lo ya pagado supera el total. Quita un paquete o revisa el descuento.</p>{/if}

          <details class="more" open={tipMode !== 'none' || !!note}>
            <summary>Propina y nota</summary>
            <div class="chips" style="margin: 0.5rem 0 0">
              {#each [['none', 'Sin propina'], ['10', '10 %'], ['15', '15 %'], ['20', '20 %'], ['custom', 'Otra']] as [v, label]}
                <button type="button" class="chip" aria-pressed={tipMode === v} onclick={() => (tipMode = v)}>{label}</button>
              {/each}
            </div>
            {#if tipMode === 'custom'}<input inputmode="decimal" placeholder="Monto de la propina" aria-label="Propina" bind:value={tipCustom} style="margin-top: 8px" />{/if}
            {#if tipCents > 0 && staffIds.length > 1}
              <label class="field" style="margin: 8px 0 0">Para quién
                <select bind:value={tipStaffId}>
                  <option value="">Sin asignar</option>
                  {#each staff.filter((s) => staffIds.includes(s.id)) as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
                </select>
              </label>
            {/if}
            <label class="field" style="margin: 8px 0 0">Nota (opcional) <input bind:value={note} maxlength="500" /></label>
          </details>
        </div>

        <div class="t-foot">
          <div class="quick">
            <button type="button" disabled={!selected || selected.kind === 'package'} title="Cantidad del renglón (F3)" onclick={openQty}><Icon name="hash" size={18} />Cantidad</button>
            <button type="button" class:on={discountCents > 0} title="Descuento al ticket (F9)" onclick={openDiscount}><Icon name="percent" size={18} />{discountCents ? `Dcto ${money(discountCents)}` : 'Descuento'}</button>
            <button type="button" title="Poner en espera (F6)" onclick={holdTicket}><Icon name="pause" size={18} />En espera</button>
          </div>
          <div class="grand"><span>{prepaid.length ? 'Por cobrar' : 'Total'}</span><strong>{money(Math.max(toPay, 0))}</strong></div>
          <button type="button" class="pay-btn" disabled={!canPay} onclick={startPay}>
            <span>Cobrar</span><kbd class="only-pc">F12</kbd>
          </button>
        </div>
      {/if}
    </aside>

    <!-- celular: barra con el total siempre a la vista -->
    <div class="m-pay">
      <button type="button" class="m-ticket" onclick={() => (ticketOpen = true)}>
        <span class="m-count">{items}</span>
        <span class="m-copy"><strong>{money(Math.max(toPay, 0))}</strong><small>{lines.length ? 'Ver ticket' : held.length ? `${held.length} en espera` : 'Ticket vacío'}</small></span>
      </button>
      <button type="button" class="m-go" disabled={!lines.length || !canPay} onclick={startPay}>Cobrar</button>
    </div>
  </div>

  <PayDialog bind:open={payOpen} due={Math.max(toPay, 0)} {items} branchId={branches.id} {point} onpay={sell} />

  <Modal bind:open={discOpen} title="Descuento">
    <form onsubmit={applyDiscount} class="mini-form">
      <div class="chips" style="margin: 0">
        {#each [5, 10, 15, 20] as p}<button type="button" class="chip" onclick={() => discPct(p)}>{p} %</button>{/each}
      </div>
      <label class="field">Monto (MXN)<input inputmode="decimal" bind:value={discDraft} placeholder="0.00" /></label>
      <div class="form-actions">
        <button type="button" class="btn ghost" onclick={() => ((discDraft = ''), (discount = ''), (discOpen = false))}>Quitar</button>
        <button type="submit" class="btn">Aplicar</button>
      </div>
    </form>
  </Modal>

  <Modal bind:open={qtyOpen} title="Cantidad">
    <form onsubmit={applyQty} class="mini-form">
      <p class="meta" style="margin: 0">{selected?.name}</p>
      <label class="field">Cantidad<input inputmode="numeric" bind:value={qtyDraft} onfocus={(e) => e.currentTarget.select()} /></label>
      <div class="form-actions"><button type="submit" class="btn">Aceptar</button></div>
    </form>
  </Modal>

  <Modal bind:open={heldOpen} title="Tickets en espera">
    {#if held.length}
      <ul class="held">
        {#each held as t (t.id)}
          <li>
            <div class="grow"><b>{plural(t.lines.reduce((n, l) => n + l.qty, 0), 'artículo', 'artículos')} · {money(heldTotal(t))}</b><small class="meta">{t.client?.name ? t.client.name + ' · ' : ''}{fmtTime(new Date(t.at).toISOString(), tz)}</small></div>
            <button type="button" class="btn sm" onclick={() => resumeHeld(t.id)}>Retomar</button>
            <button type="button" class="btn ghost sm" aria-label="Descartar" onclick={() => dropHeld(t.id)}><Icon name="trash" size={16} /></button>
          </li>
        {/each}
      </ul>
      {#if lines.length}<p class="meta">El ticket actual pasará a la espera al retomar otro.</p>{/if}
    {:else}
      <p class="empty">No hay tickets en espera. Con <b>En espera</b> guardas la venta actual para atender a alguien más.</p>
    {/if}
  </Modal>
{/if}

<style>
  .open { max-width: 460px; }
  .pos { display: grid; gap: 16px; grid-template-columns: 1fr; align-items: start; }
  @media (min-width: 760px) {
    .pos { grid-template-columns: minmax(0, 1fr) minmax(340px, 400px); }
    .ticket { position: sticky; top: 16px; max-height: calc(100vh - 112px); }
  }
  .only-pc { display: none; }
  @media (hover: hover) and (pointer: fine) { .only-pc { display: inline; } }

  /* catálogo: lista limpia, precio a la derecha en cifras tabulares */
  .catalog { min-width: 0; }
  .finder { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; padding: 0 12px; background: var(--surface); border: 1px solid var(--control-border); border-radius: var(--radius-sm); color: var(--muted); }
  .finder:focus-within { border-color: var(--accent); }
  .finder input.scan, .finder input.scan:hover, .finder input.scan:focus { flex: 1; min-width: 0; min-height: 48px; margin: 0; padding: 0; border: 0; outline: 0; box-shadow: none; background: transparent; font-size: 1rem; }
  .clear { display: grid; place-items: center; width: 44px; height: 44px; margin-right: -8px; color: var(--muted); background: transparent; border: 0; border-radius: 50%; cursor: pointer; }
  .clear:hover { background: var(--surface-2); }
  .cat-bar { display: grid; gap: 8px; justify-items: start; margin-bottom: 8px; max-width: 100%; }
  .cat-bar :global(.seg) { max-width: 100%; }
  .hint { margin: 0 0 8px; }
  .none { margin: 12px 4px; }
  .none-box { display: grid; justify-items: start; gap: 4px; padding: 12px 4px; }
  .none-box p { margin: 0; }
  .items { display: grid; }
  .items .head { margin: 0 0 4px; }
  .item {
    display: flex; align-items: center; gap: 12px; width: 100%; min-height: 60px; padding: 8px 12px; text-align: left; font: inherit; color: var(--text);
    background: none; border: 0; border-bottom: 1px solid var(--border); border-radius: 0; cursor: pointer;
    transition: background-color var(--dur-ui) var(--ease);
  }
  .item:last-child { border-bottom: 0; }
  .item:hover:not(:disabled), .item.active { background: var(--surface-2); }
  .item:active:not(:disabled) { background: var(--accent-soft); }
  .item.active { box-shadow: inset 3px 0 0 var(--accent); }
  .item:disabled { opacity: 0.5; cursor: not-allowed; }
  .item .grow { flex: 1 1 auto; min-width: 0; display: grid; gap: 2px; }
  .item small { color: var(--muted); font-size: 0.85rem; }
  .item small.low { color: var(--danger); }
  .in-ticket { font-size: 0.8rem; font-style: normal; font-weight: 600; color: var(--accent-text, var(--accent)); white-space: nowrap; }
  .price { font-weight: 600; font-variant-numeric: tabular-nums lining-nums; white-space: nowrap; }
  .keys { display: flex; flex-wrap: wrap; gap: 6px 14px; margin: 16px 0 0; padding-top: 12px; border-top: 1px solid var(--border); color: var(--muted); font-size: 0.8rem; }
  kbd { margin-right: 4px; padding: 1px 6px; font: inherit; font-size: 0.75rem; border: 1px solid var(--border); border-radius: 4px; background: var(--surface-2); }

  /* ticket: encabezado y cliente arriba, renglones con scroll, total y cobrar siempre a la vista */
  .ticket { display: flex; flex-direction: column; min-width: 0; overflow: hidden; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  .t-top { padding: 20px 20px 12px; }
  .t-body { flex: 1 1 auto; min-height: 0; overflow-y: auto; padding: 0 20px 16px; }
  .t-foot { display: grid; gap: 10px; padding: 14px 20px calc(16px + env(safe-area-inset-bottom)); background: var(--surface); border-top: 1px solid var(--border); }
  .t-head { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 12px; }
  .t-title { flex: 1; min-width: max-content; }
  .t-head h2 { margin: 0; font-size: 1.25rem; white-space: nowrap; }
  .t-head p { margin: 0; }
  .head-btn { display: inline-flex; align-items: center; gap: 6px; min-height: 44px; padding: 0 14px; font: inherit; font-size: 0.85rem; font-weight: 600; color: var(--text); background: var(--surface-2); border: 1px solid var(--border); border-radius: 999px; cursor: pointer; }
  .head-btn.icon { padding: 0; width: 44px; justify-content: center; }
  .head-btn.has { border-color: var(--accent); color: var(--accent-text, var(--accent)); }
  .head-btn b { min-width: 18px; padding: 0 5px; border-radius: 999px; background: var(--accent); color: var(--accent-contrast); font-size: 0.72rem; text-align: center; }
  .head-btn:disabled { opacity: 0.45; cursor: not-allowed; }
  .head-btn.close { display: none; }
  .client { display: grid; gap: 4px; }
  .client-on { display: flex; align-items: center; gap: 8px; min-height: 44px; }
  .client-on .grow { flex: 1 1 auto; min-width: 0; }
  .pick { display: flex; justify-content: space-between; width: 100%; min-height: 44px; align-items: center; padding: 0 0.8rem; font: inherit; color: var(--text); background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); cursor: pointer; }
  .pick small { color: var(--muted); }

  .loyal { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-top: 12px; padding: 8px 12px; font-size: 0.88rem; background: var(--surface-2); border-radius: var(--radius-sm); }
  .loyal.ready { background: var(--accent-soft); color: var(--accent-text, var(--accent)); }
  .ok-msg { margin: 12px 0 0; padding: 8px 12px; font-size: 0.85rem; color: var(--ok); background: var(--ok-soft); border-radius: var(--radius-sm); }
  .t-empty { display: grid; justify-items: center; gap: 4px; margin: 0 20px 20px; padding: 28px 16px; text-align: center; border: 1px dashed var(--border); border-radius: var(--radius-sm); }
  .t-empty p { margin: 0; }
  .done-ico { display: grid; place-items: center; width: 52px; height: 52px; margin-bottom: 6px; border-radius: 50%; background: var(--ok-soft); color: var(--ok); }
  .print-issue { display: grid; gap: 8px; margin-top: 8px; padding: 12px 16px; text-align: left; background: var(--danger-soft); color: var(--danger); border-radius: var(--radius-sm); font-size: 0.9rem; }
  .print-issue div { display: flex; flex-wrap: wrap; gap: 8px; }

  .lines { list-style: none; margin: 0 0 8px; padding: 0; }
  .lines li { padding: 12px 0 12px 10px; border-bottom: 1px solid var(--border); box-shadow: inset 3px 0 0 transparent; }
  .lines li.on { box-shadow: inset 3px 0 0 var(--accent); }
  .l-top { display: flex; justify-content: space-between; gap: 8px; width: 100%; min-height: 28px; padding: 0; font: inherit; color: var(--text); text-align: left; background: none; border: 0; cursor: pointer; }
  .amt { font-variant-numeric: tabular-nums lining-nums; font-weight: 600; white-space: nowrap; }
  .l-sub { margin: 0; font-size: 0.82rem; font-variant-numeric: tabular-nums; }
  .l-bot { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin-top: 8px; }
  .qty { display: inline-flex; align-items: center; border: 1px solid var(--control-border); border-radius: var(--radius-sm); background: var(--surface); }
  .qty button { width: 44px; height: 44px; font-size: 1.2rem; color: var(--text); background: none; border: 0; cursor: pointer; }
  .qty .val { width: auto; min-width: 44px; padding: 0 4px; font-size: 1rem; font-variant-numeric: tabular-nums; }
  .qty button:disabled { opacity: 0.4; cursor: not-allowed; }
  .who { width: auto; flex: 1 1 110px; min-width: 0; margin: 0; }
  .pk { margin: 0 0 8px; }

  .sums { display: grid; gap: 8px; margin: 0; padding: 12px 0; font-variant-numeric: tabular-nums lining-nums; }
  .sums > div { display: flex; justify-content: space-between; gap: 12px; }
  .sums dt, .sums dd { margin: 0; }
  .sums .disc { color: var(--ok); }
  .link { min-height: 44px; padding: 0 4px; font: inherit; font-size: 0.82rem; color: var(--accent-text, var(--accent)); background: none; border: 0; text-decoration: underline; cursor: pointer; }
  .more { margin-top: 4px; font-size: 0.9rem; }
  .more summary { display: flex; align-items: center; min-height: 44px; cursor: pointer; font-weight: 500; color: var(--muted); }

  .quick { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
  .quick button { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 44px; padding: 0 6px; font: inherit; font-size: 0.8rem; font-weight: 600; color: var(--text); background: var(--surface); border: 1px solid var(--control-border); border-radius: var(--radius-sm); cursor: pointer; }
  .quick button.on { border-color: var(--accent); color: var(--accent-text, var(--accent)); }
  .quick button:disabled { opacity: 0.45; cursor: not-allowed; }
  .grand { display: flex; justify-content: space-between; align-items: baseline; color: var(--text); }
  .grand span { font-weight: 600; }
  .grand strong { font-family: var(--font-num, inherit); font-weight: 700; font-variant-numeric: tabular-nums lining-nums; font-size: 1.9rem; letter-spacing: -0.01em; }
  .pay-btn { display: flex; align-items: center; justify-content: center; gap: 12px; min-height: 56px; padding: 0 20px; font: inherit; font-size: 1.1rem; font-weight: 700; color: var(--accent-contrast); background: var(--accent); border: 0; border-radius: var(--radius-sm); cursor: pointer; transition: background-color var(--dur-ui) var(--ease); }
  .pay-btn:hover:not(:disabled) { background: var(--accent-hover); }
  .pay-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .pay-btn kbd { margin: 0; background: transparent; color: inherit; border-color: currentColor; opacity: 0.7; }

  .mini-form { display: grid; gap: 16px; }
  .mini-form .field { margin: 0; }
  .held { list-style: none; margin: 0 0 12px; padding: 0; display: grid; }
  .held li { display: flex; align-items: center; gap: 8px; padding: 10px 0; border-bottom: 1px solid var(--border); }
  .held li:last-child { border-bottom: 0; }
  .held small { display: block; }

  /* celular: barra de cobro y el ticket como hoja */
  .scrim, .m-pay { display: none; }
  @media (max-width: 759px) {
    .head-btn.close { display: inline-flex; }
    .m-pay {
      position: fixed; inset: auto 12px calc(72px + env(safe-area-inset-bottom)) 12px; z-index: 35;
      display: grid; grid-template-columns: 1fr auto; gap: 8px;
    }
    .m-ticket, .m-go { min-height: 56px; font: inherit; border: 0; border-radius: var(--radius); box-shadow: var(--shadow); cursor: pointer; }
    .m-ticket { display: flex; align-items: center; gap: 12px; padding: 0 16px; text-align: left; color: var(--text); background: var(--surface); border: 1px solid var(--border); }
    .m-count { display: grid; place-items: center; min-width: 30px; height: 30px; border-radius: 50%; background: var(--accent-soft); color: var(--accent-text, var(--accent)); font-weight: 700; font-variant-numeric: tabular-nums; }
    .m-copy { display: grid; }
    .m-copy strong { font-size: 1.25rem; font-variant-numeric: tabular-nums lining-nums; }
    .m-copy small { color: var(--muted); }
    .m-go { padding: 0 24px; font-weight: 700; font-size: 1.05rem; color: var(--accent-contrast); background: var(--accent); }
    .m-go:disabled { opacity: 0.5; cursor: not-allowed; }
    .scrim { display: block; position: fixed; inset: 0; z-index: 55; border: 0; padding: 0; background: rgba(20, 18, 14, 0.45); opacity: 0; pointer-events: none; transition: opacity 200ms var(--ease); }
    .scrim.open { opacity: 1; pointer-events: auto; }
    .ticket {
      position: fixed; inset: auto 0 0 0; z-index: 60; max-height: 90vh; max-height: 90dvh; border-radius: 16px 16px 0 0; border-bottom: 0;
      transform: translateY(105%); visibility: hidden; transition: transform 260ms var(--ease), visibility 0s linear 260ms; box-shadow: var(--shadow);
    }
    .ticket.open { transform: none; visibility: visible; transition: transform 260ms var(--ease); }
    .pos { padding-bottom: 88px; }
  }
  @media (prefers-reduced-motion: reduce) { .ticket, .ticket.open, .scrim { transition: none; } }
</style>
