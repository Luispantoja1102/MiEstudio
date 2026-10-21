<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import Icon from '$lib/Icon.svelte';
  import Logo from '$lib/Logo.svelte';
  import ThemeToggle from '$lib/ThemeToggle.svelte';
  import { api, getToken, setToken } from '$lib/api.js';

  let me = $state(null);

  const nav = [
    ['home', 'Inicio', true],
    ['calendar', 'Agenda'],
    ['card', 'Ventas'],
    ['users', 'Clientes'],
    ['scissors', 'Servicios'],
    ['package', 'Inventario'],
    ['percent', 'Comisiones']
  ];
  const cards = [
    ['calendar', 'Citas de hoy'],
    ['card', 'Ventas del día'],
    ['users', 'Clientes']
  ];

  onMount(async () => {
    if (!getToken()) return goto('/login');
    try {
      me = await api('/api/me');
    } catch {
      setToken(null);
      goto('/login');
    }
  });

  function logout() {
    setToken(null);
    goto('/');
  }
</script>

<div class="shell">
  <aside class="side">
    <Logo />
    <nav aria-label="Módulos">
      {#each nav as [icon, label, active]}
        {#if active}
          <a class="item active" href="/panel" aria-current="page"><Icon name={icon} />{label}</a>
        {:else}
          <span class="item off" aria-disabled="true"><Icon name={icon} />{label}<em>Pronto</em></span>
        {/if}
      {/each}
    </nav>
  </aside>

  <div class="main">
    <header class="top">
      <span class="biz">{me?.business.name ?? ''}</span>
      <div class="tools">
        <ThemeToggle />
        <button class="btn ghost" onclick={logout}><Icon name="logout" size={18} />Salir</button>
      </div>
    </header>

    <main class="content">
      {#if me}
        <h1>Hola, {me.name}</h1>
        <p class="sub">Este es el resumen de tu negocio.</p>

        <div class="stats">
          {#each cards as [icon, label]}
            <div class="card stat">
              <span class="ico"><Icon name={icon} /></span>
              <span class="label">{label}</span>
              <span class="value">—</span>
              <span class="soon">Disponible pronto</span>
            </div>
          {/each}
        </div>
      {/if}
    </main>
  </div>
</div>

<style>
  .shell { display: grid; min-height: 100vh; }
  .side {
    display: none;
    flex-direction: column;
    gap: 28px;
    padding: 24px 16px;
    background: var(--surface);
    border-right: 1px solid var(--border);
  }
  .side nav { display: grid; gap: 4px; }
  .item {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 44px;
    padding: 0 12px;
    border-radius: var(--radius-sm);
    color: var(--text);
    text-decoration: none;
    font-size: 0.95rem;
    font-weight: 500;
  }
  .item.active { background: var(--accent-soft); color: var(--accent); }
  .item.off { color: var(--muted); cursor: default; }
  .item em { margin-left: auto; font-style: normal; font-size: 0.7rem; padding: 2px 8px; border-radius: 999px; background: var(--surface-2); }

  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-height: 68px;
    padding: 0 20px;
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  .biz { font-weight: 600; }
  .tools { display: flex; gap: 8px; }

  .content { padding: 32px 20px 56px; max-width: 1000px; }
  h1 { font-size: 2rem; margin-bottom: 0.2em; }
  .sub { color: var(--muted); margin-bottom: 2rem; }

  .stats { display: grid; gap: 16px; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); }
  .stat { display: grid; gap: 4px; }
  .ico { display: grid; place-items: center; width: 40px; height: 40px; margin-bottom: 8px; border-radius: 10px; background: var(--accent-soft); color: var(--accent); }
  .label { color: var(--muted); font-size: 0.9rem; }
  .value { font-family: var(--font-heading); font-size: 2.2rem; line-height: 1.1; }
  .soon { font-size: 0.8rem; color: var(--muted); }

  @media (min-width: 900px) {
    .shell { grid-template-columns: 250px 1fr; }
    .side { display: flex; }
    .content { padding: 40px 40px 64px; }
    .top { padding: 0 40px; }
  }
</style>
