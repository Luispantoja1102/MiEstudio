<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { planLimit, session } from '$lib/session.svelte.js';

  let status = $state(null);
  let terminals = $state([]);
  let choice = $state({}); // branch_id -> terminal_id
  let error = $state('');
  let notice = $state('');
  let busy = $state(false);

  async function load() {
    status = await api('/api/payments/mercadopago');
    choice = Object.fromEntries(status.terminals.map((t) => [t.branch_id, t.terminal_id]));
    terminals = status.connected ? await api('/api/payments/mercadopago/terminals').catch((e) => ((error = e.message), [])) : [];
  }

  const REASONS = {
    cancelled: 'Cancelaste la conexión con Mercado Pago.',
    state: 'El enlace de conexión caducó o no es válido. Inténtalo de nuevo.',
    oauth_failed: 'Mercado Pago no aceptó la autorización. Inténtalo de nuevo.',
    unknown: 'No pudimos terminar la conexión. Inténtalo de nuevo.'
  };

  onMount(async () => {
    if (session.me.role !== 'admin') return goto('/panel');
    // resultado del regreso desde Mercado Pago (?mp=ok | ?mp=error&reason=…)
    const back = new URLSearchParams(location.search);
    if (back.get('mp') === 'ok') notice = 'Cuenta de Mercado Pago conectada.';
    if (back.get('mp') === 'error') error = REASONS[back.get('reason')] ?? REASONS.unknown;
    if (back.has('mp')) history.replaceState(null, '', location.pathname);
    try {
      await loadBranches();
      await load();
    } catch (e) {
      error = e.message;
    }
  });

  // lleva al dueño a autorizar la conexión en Mercado Pago
  async function connectOAuth() {
    error = '';
    busy = true;
    try {
      const { url } = await api('/api/payments/mercadopago/connect');
      window.location.href = url;
    } catch (e) {
      error = e.message;
      busy = false;
    }
  }

  async function run(fn, ok) {
    error = '';
    notice = '';
    busy = true;
    try {
      await fn();
      notice = ok;
      await load();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  }

  const register = (b) => run(() => api('/api/payments/mercadopago/terminals', { method: 'PUT', body: { branch_id: b.id, terminal_id: choice[b.id] } }), `Terminal registrada en ${b.name}.`);
  const disconnect = () => confirm('¿Desconectar Mercado Pago? Dejarás de poder cobrar con la terminal.') && run(() => api('/api/payments/mercadopago', { method: 'DELETE' }), 'Cuenta desconectada.');
  const labelOf = (id) => status?.terminals.find((t) => t.branch_id === id)?.label;
</script>

<div class="page-head">
  <div>
    <h1>Pagos</h1>
    <p>Cobra con tu terminal Mercado Pago Point desde la pantalla de cobro.</p>
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}
{#if notice}<p class="banner" role="status" style="background: var(--accent-soft); color: var(--accent)">{notice}</p>{/if}

{#if status && !status.enabled}
  <div class="card"><h2>No habilitado</h2><p class="meta" style="margin: 0">Este servidor aún no puede guardar la conexión con Mercado Pago. Pide a quien administra la plataforma que lo configure.</p></div>
{:else if status}
  <section class="card">
    <h2>1. Cuenta de Mercado Pago</h2>
    {#if status.connected}
      <p class="ok">✓ Cuenta conectada con Mercado Pago</p>
      <div class="form-actions">
        {#if status.oauth_available}<button class="btn ghost sm" onclick={connectOAuth} disabled={busy}>Volver a conectar</button>{/if}
        <button class="btn ghost sm" onclick={disconnect} disabled={busy}>Desconectar</button>
      </div>
    {:else}
      {#if status.oauth_available}
        <p class="meta">Te llevaremos a Mercado Pago para que autorices a Barbella a cobrar con tu terminal. No compartimos tu contraseña.</p>
        <button class="btn" onclick={connectOAuth} disabled={busy}>{busy ? 'Redirigiendo…' : 'Conectar con Mercado Pago'}</button>
      {:else}
        <p class="meta">La conexión con Mercado Pago aún no está habilitada en este servidor. Pide a quien administra la plataforma que la active.</p>
      {/if}
    {/if}
  </section>

  {#if status.connected && planLimit('terminal') !== false}
    <section class="card" style="margin-top: 1rem">
      <h2>2. Terminal por sucursal</h2>
      <p class="meta">Al registrarla, la terminal pasa a modo punto de venta: solo cobra lo que envía Barbella.</p>
      {#if terminals.length === 0}<p class="meta">No encontramos terminales en tu cuenta de Mercado Pago.</p>{/if}
      {#each branches.list as b (b.id)}
        <div class="item-row" style="margin-top: 8px">
          <span class="grow"><b>{b.name}</b>{#if labelOf(b.id)}<small class="meta">{' · '}actual: {labelOf(b.id)}</small>{/if}</span>
          <select bind:value={choice[b.id]} aria-label="Terminal de {b.name}" style="width: auto; margin: 0; min-height: 38px">
            <option value={undefined}>Elige una terminal</option>
            {#each terminals as t (t.id)}<option value={t.id}>{t.external_pos_id || t.id}</option>{/each}
          </select>
          <button class="btn sm" disabled={!choice[b.id] || busy} onclick={() => register(b)}>Registrar</button>
        </div>
      {/each}
    </section>
  {/if}
{/if}

<style>
  h2 { font-size: 1.3rem; }
  .ok { color: var(--accent); font-weight: 600; }
</style>
