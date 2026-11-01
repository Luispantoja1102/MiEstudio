<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { branches, loadBranches } from '$lib/branch.svelte.js';
  import { planLimit, session } from '$lib/session.svelte.js';
  import PageHeader from '$lib/ui/PageHeader.svelte';
  import Surface from '$lib/ui/Surface.svelte';
  import StatusBadge from '$lib/ui/StatusBadge.svelte';
  import InlineNotice from '$lib/ui/InlineNotice.svelte';
  import DataTable from '$lib/ui/DataTable.svelte';

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
  const columns = [
    { key: 'branch', label: 'Sucursal' },
    { key: 'current', label: 'Terminal actual' },
    { key: 'pick', label: 'Cambiar a' },
    { key: 'act', label: '' }
  ];
  const labelOf = (id) => status?.terminals.find((t) => t.branch_id === id)?.label;
</script>

<PageHeader title="Pagos" description="Cobra con tu terminal Mercado Pago Point desde la pantalla de cobro." />

<div class="stack">
  {#if error}<InlineNotice tone="danger" message={error} />{/if}
  {#if notice}<InlineNotice tone="success" message={notice} />{/if}

  {#if status && !status.enabled}
    <Surface title="Mercado Pago" description="Cobro con terminal Point">
      <div class="state"><StatusBadge label="No habilitado" tone="warning" /></div>
      <p class="meta">Este servidor aún no puede guardar la conexión con Mercado Pago. Pide a quien administra la plataforma que lo configure.</p>
    </Surface>
  {:else if status}
    <Surface title="Cuenta de Mercado Pago">
      {#snippet actions()}
        {#if status.connected}
          <StatusBadge label="Conectada" tone="success" />
        {:else}
          <StatusBadge label="Sin conectar" tone="neutral" />
        {/if}
      {/snippet}
      {#if status.connected}
        <p class="meta">Cuenta conectada con Mercado Pago.</p>
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
    </Surface>

    {#if status.connected && planLimit('terminal') !== false}
      <Surface title="Terminal por sucursal" description="Al registrarla, la terminal pasa a modo punto de venta: solo cobra lo que envía Barbella." padding="flush">
        {#if terminals.length === 0}<div class="pad"><InlineNotice tone="info" message="No encontramos terminales en tu cuenta de Mercado Pago." /></div>{/if}
        <DataTable caption="Terminal de cobro por sucursal" {columns} rows={branches.list}>
          {#snippet cell({ row: b, column })}
            {#if column.key === 'branch'}
              <b>{b.name}</b>
            {:else if column.key === 'current'}
              {#if labelOf(b.id)}<StatusBadge label={labelOf(b.id)} tone="success" />{:else}<span class="meta">Sin terminal</span>{/if}
            {:else if column.key === 'pick'}
              <select bind:value={choice[b.id]} aria-label="Terminal de {b.name}">
                <option value={undefined}>Elige una terminal</option>
                {#each terminals as t (t.id)}<option value={t.id}>{t.external_pos_id || t.id}</option>{/each}
              </select>
            {:else}
              <button class="btn sm" disabled={!choice[b.id] || busy} onclick={() => register(b)}>Registrar</button>
            {/if}
          {/snippet}
        </DataTable>
      </Surface>
    {/if}
  {/if}
</div>

<style>
  .stack { display: grid; gap: 16px; max-width: 880px; }
  .state { margin-bottom: 12px; }
  .meta { margin: 0 0 12px; }
  .pad { padding: 0 24px 16px; }
  select { width: auto; min-width: 180px; margin: 0; min-height: 44px; }
  .btn.sm { min-height: 44px; }
</style>
