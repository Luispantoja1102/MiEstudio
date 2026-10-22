<script>
  import { onDestroy, onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import Icon from '$lib/Icon.svelte';
  import { api } from '$lib/api.js';

  let open = $state(false);
  let unread = $state(0);
  let toReview = $state(0);
  let items = $state([]);
  let timer;
  let root;

  const ICON = { proof: 'receipt', booking: 'calendar', booking_transfer: 'receipt', cancelled: 'calendar', confirmed: 'check', deposit_paid: 'wallet', refund_due: 'wallet', expired: 'clock' };
  const URGENT = ['proof', 'refund_due'];

  async function load() {
    try {
      const r = await api('/api/inbox');
      unread = r.unread;
      toReview = r.to_review;
      items = r.items;
    } catch {}
  }

  onMount(() => {
    load();
    timer = setInterval(() => !document.hidden && load(), 45000);
  });
  onDestroy(() => clearInterval(timer));

  async function toggle() {
    open = !open;
    if (open) {
      await load();
      // abrir la campana cuenta como verlo todo; lo que estaba sin leer se sigue distinguiendo mientras está abierta
      if (unread) {
        await api('/api/inbox/read', { method: 'POST' }).catch(() => {});
        unread = 0;
      }
    }
  }

  function pick(it) {
    open = false;
    if (it.link) goto(it.link);
  }

  const ago = (iso) => {
    const m = Math.round((Date.now() - new Date(iso)) / 60000);
    if (m < 1) return 'ahora';
    if (m < 60) return `hace ${m} min`;
    const h = Math.round(m / 60);
    if (h < 24) return `hace ${h} h`;
    return new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short' });
  };

  const onwindow = (e) => {
    if (open && root && !root.contains(e.target)) open = false;
  };
</script>

<svelte:window onclick={onwindow} onkeydown={(e) => e.key === 'Escape' && (open = false)} />

<div class="bell" bind:this={root}>
  <button class="toggle" onclick={toggle} aria-label={unread ? `Avisos: ${unread} sin leer` : 'Avisos'} aria-expanded={open} title="Avisos">
    <Icon name="bell" />
    {#if unread}<span class="count" aria-hidden="true">{unread > 9 ? '9+' : unread}</span>{/if}
  </button>

  {#if open}
    <div class="panel" role="dialog" aria-label="Avisos">
      <header><h2>Avisos</h2></header>
      {#if toReview}
        <p class="review"><Icon name="receipt" size={16} />{toReview === 1 ? '1 comprobante espera tu revisión' : `${toReview} comprobantes esperan tu revisión`}</p>
      {/if}
      {#if items.length === 0}
        <p class="empty">Aquí aparecerán las reservas, los comprobantes y las cancelaciones.</p>
      {:else}
        <ul>
          {#each items as it (it.id)}
            <li>
              <button class="row" class:fresh={it.unread} onclick={() => pick(it)}>
                <span class="ico" class:urgent={URGENT.includes(it.kind)}><Icon name={ICON[it.kind] ?? 'bell'} size={17} /></span>
                <span class="txt"><b>{it.title}</b><span>{it.body}</span></span>
                <time datetime={it.created_at}>{ago(it.created_at)}</time>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</div>

<style>
  .bell { position: relative; }
  .toggle {
    position: relative;
    display: inline-grid;
    place-items: center;
    width: 44px;
    height: 44px;
    color: var(--text);
    background: transparent;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background-color 0.2s var(--ease), border-color 0.2s var(--ease), transform 0.12s var(--ease);
  }
  .toggle:hover { background: var(--surface-2); border-color: var(--muted); }
  .toggle:active { transform: scale(0.96); }
  .count {
    position: absolute;
    top: -6px;
    right: -6px;
    min-width: 20px;
    height: 20px;
    padding: 0 5px;
    display: grid;
    place-items: center;
    font-size: 0.72rem;
    font-weight: 700;
    line-height: 1;
    font-variant-numeric: tabular-nums;
    color: var(--side-mark-ink);
    background: var(--side-mark);
    border: 2px solid var(--bg);
    border-radius: 999px;
  }
  .panel {
    position: absolute;
    right: 0;
    top: calc(100% + 10px);
    z-index: 30;
    width: min(400px, calc(100vw - 24px));
    max-height: min(560px, 76vh);
    overflow: auto;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: 0 18px 50px rgba(10, 20, 45, 0.22);
    animation: pop 0.16s var(--ease);
    transform-origin: top right;
  }
  @keyframes pop { from { opacity: 0; transform: scale(0.97) translateY(-4px); } }
  header { position: sticky; top: 0; padding: 14px 18px 10px; background: var(--surface); border-bottom: 1px solid var(--border); }
  h2 { margin: 0; font-size: 1.15rem; }
  .review { display: flex; align-items: center; gap: 8px; margin: 0; padding: 10px 18px; font-size: 0.88rem; font-weight: 600; color: var(--warn); background: var(--warn-soft); }
  .empty { margin: 0; padding: 28px 22px; text-align: center; color: var(--muted); font-size: 0.92rem; }
  ul { list-style: none; margin: 0; padding: 6px; }
  .row {
    display: grid;
    grid-template-columns: 34px 1fr auto;
    gap: 12px;
    align-items: start;
    width: 100%;
    padding: 10px 12px;
    font: inherit;
    text-align: left;
    color: var(--text);
    background: transparent;
    border: 0;
    border-radius: var(--radius-sm);
    cursor: pointer;
  }
  .row:hover { background: var(--surface-2); }
  .row.fresh { background: color-mix(in srgb, var(--accent-soft) 70%, transparent); }
  .ico { display: grid; place-items: center; width: 34px; height: 34px; border-radius: 50%; background: var(--surface-2); color: var(--muted); }
  .ico.urgent { background: var(--warn-soft); color: var(--warn); }
  .txt { display: grid; gap: 2px; min-width: 0; }
  .txt b { font-size: 0.92rem; font-weight: 600; }
  .txt span { font-size: 0.83rem; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  time { font-size: 0.75rem; color: var(--muted); white-space: nowrap; padding-top: 2px; }
  @media (max-width: 520px) { .panel { position: fixed; left: 12px; right: 12px; top: 76px; width: auto; } }
</style>
