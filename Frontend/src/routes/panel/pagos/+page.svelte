<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { session } from '$lib/session.svelte.js';

  let status = $state(null);
  let terminals = $state([]);
  let token = $state('');
  let choice = $state({}); // branch_id -> terminal_id
  let error = $state('');
  let notice = $state('');
  let busy = $state(false);

  async function load() {
    status = await api('/api/payments/mercadopago');
    choice = Object.fromEntries(status.terminals.map((t) => [t.branch_id, t.terminal_id]));
    terminals = status.connected ? await api('/api/payments/mercadopago/terminals').catch((e) => ((error = e.message), [])) : [];
  }

  onMount(async () => {
    if (session.me.role !== 'admin') return goto('/panel');
    try {
      await loadBranches();
      await load();
    } catch (e) {
      error = e.message;
    }
  });

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

  const connect = (e) => {
    e.preventDefault();
    return run(() => api('/api/payments/mercadopago/token', { method: 'PUT', body: { access_token: token } }), 'Cuenta conectada.').then(() => (token = ''));
  };
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
  <div class="card"><h2>No habilitado</h2><p class="meta" style="margin: 0">Este servidor no tiene configurada la clave de cifrado (<code>TOKEN_ENC_KEY</code>), así que no puede guardar credenciales de Mercado Pago. Pide a quien administra la plataforma que la configure.</p></div>
{:else if status}
  <section class="card">
    <h2>1. Cuenta de Mercado Pago</h2>
    {#if status.connected}
      <p class="ok">✓ Cuenta conectada</p>
      <div class="form-actions"><button class="btn ghost sm" onclick={disconnect} disabled={busy}>Desconectar</button></div>
    {:else}
      <p class="meta">Pega el <b>access token de producción</b> de tu aplicación en <i>Mercado Pago → Tu negocio → Configuración → Credenciales</i>. Se guarda cifrado y nunca se vuelve a mostrar.</p>
      <form onsubmit={connect}>
        <label class="field">Access token <input type="password" bind:value={token} autocomplete="off" placeholder="APP_USR-…" required /></label>
        <button class="btn" disabled={busy}>{busy ? 'Comprobando…' : 'Conectar'}</button>
      </form>
    {/if}
  </section>

  {#if status.connected}
    <section class="card" style="margin-top: 1rem">
      <h2>2. Terminal por sucursal</h2>
      <p class="meta">Al registrarla, la terminal pasa a modo punto de venta: solo cobra lo que envía MiEstudio.</p>
      {#if terminals.length === 0}<p class="meta">No encontramos terminales en tu cuenta de Mercado Pago.</p>{/if}
      {#each branches.list as b (b.id)}
        <div class="item-row" style="margin-top: 8px">
          <span class="grow"><b>{b.name}</b>{#if labelOf(b.id)}<small class="meta"> · actual: {labelOf(b.id)}</small>{/if}</span>
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
  code { background: var(--surface-2); padding: 1px 6px; border-radius: 4px; font-size: 0.85em; }
</style>
