<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import QRCode from 'qrcode';
  import { api } from '$lib/api.js';
  import { session } from '$lib/session.svelte.js';

  // Cuenta propia: contraseña, verificación en dos pasos y cierre de sesiones. Lo usan el panel y la plataforma.
  let status = $state({ enabled: false, recovery_left: 0 });
  let error = $state('');
  let notice = $state('');
  let busy = $state(false);

  let pw = $state({ current: '', next: '', again: '' });

  let setup = $state(null); // { secret, otpauth_url, qr }
  let code = $state('');
  let codes = $state(null); // códigos de respaldo recién generados (se muestran una sola vez)
  let manage = $state(null); // 'disable' | 'recovery'
  let mgmt = $state({ password: '', code: '' });

  async function load() {
    status = await api('/api/me/2fa');
  }
  onMount(() => load().catch((e) => (error = e.message)));

  const run = async (fn) => {
    error = '';
    notice = '';
    busy = true;
    try {
      await fn();
    } catch (e) {
      error = e.message;
    } finally {
      busy = false;
    }
  };

  const changePassword = (e) => {
    e.preventDefault();
    if (pw.next !== pw.again) return (error = 'Las contraseñas nuevas no coinciden.');
    return run(async () => {
      await api('/api/auth/password', { method: 'POST', body: { current: pw.current, new: pw.next } });
      pw = { current: '', next: '', again: '' };
      notice = 'Contraseña cambiada. Cerramos tus sesiones en otros dispositivos.';
    });
  };

  const startSetup = () =>
    run(async () => {
      const s = await api('/api/me/2fa/setup', { method: 'POST' });
      setup = { ...s, qr: await QRCode.toDataURL(s.otpauth_url, { margin: 1, width: 220 }) };
      code = '';
    });

  const enable = (e) => {
    e.preventDefault();
    return run(async () => {
      const r = await api('/api/me/2fa/enable', { method: 'POST', body: { code } });
      codes = r.recovery_codes;
      setup = null;
      await load();
      session.me.two_factor = true;
    });
  };

  const submitManage = (e) => {
    e.preventDefault();
    return run(async () => {
      if (manage === 'disable') {
        await api('/api/me/2fa/disable', { method: 'POST', body: mgmt });
        notice = 'Verificación en dos pasos desactivada.';
        session.me.two_factor = false;
      } else {
        codes = (await api('/api/me/2fa/recovery', { method: 'POST', body: mgmt })).recovery_codes;
      }
      manage = null;
      mgmt = { password: '', code: '' };
      await load();
    });
  };

  const logoutAll = () =>
    run(async () => {
      await api('/api/auth/logout-all', { method: 'POST' });
      session.me = null;
      goto('/login');
    });

  const copyCodes = () => navigator.clipboard?.writeText(codes.join('\n')).then(() => (notice = 'Códigos copiados.')).catch(() => {});
  const downloadCodes = () => {
    const url = URL.createObjectURL(new Blob([`Barbella: códigos de respaldo (${session.me.email})\n\n${codes.join('\n')}\n`], { type: 'text/plain' }));
    Object.assign(document.createElement('a'), { href: url, download: 'miestudio-codigos-de-respaldo.txt' }).click();
    URL.revokeObjectURL(url);
  };
</script>

<div class="page-head">
  <div>
    <h1>Mi cuenta</h1>
    <p>{session.me?.name} · {session.me?.email}</p>
  </div>
</div>

{#if error}<p class="error" role="alert">{error}</p>{/if}
{#if notice}<p class="ok" role="status">{notice}</p>{/if}

<section class="card sec">
  <h2>Contraseña</h2>
  <form onsubmit={changePassword}>
    <div class="form-grid">
      <label class="field">Contraseña actual <input type="password" bind:value={pw.current} autocomplete="current-password" required /></label>
      <label class="field">Contraseña nueva <input type="password" bind:value={pw.next} autocomplete="new-password" minlength="8" maxlength="72" required /></label>
      <label class="field">Repite la nueva <input type="password" bind:value={pw.again} autocomplete="new-password" required /></label>
    </div>
    <button class="btn" disabled={busy}>Cambiar contraseña</button>
  </form>
</section>

<section class="card sec">
  <h2>Verificación en dos pasos {#if status.enabled}<span class="badge">Activa</span>{:else}<span class="badge muted">Desactivada</span>{/if}</h2>
  <p class="meta">Además de tu contraseña, se pide un código de una app de autenticación (Google Authenticator, Authy, 1Password…). Protege tu cuenta aunque alguien robe tu contraseña.</p>

  {#if codes}
    <div class="codes" role="status">
      <p><b>Guarda estos códigos de respaldo.</b> Cada uno sirve una sola vez si pierdes tu teléfono. No volverán a mostrarse.</p>
      <ul>{#each codes as c}<li><code>{c}</code></li>{/each}</ul>
      <div class="form-actions">
        <button class="btn ghost sm" onclick={copyCodes}>Copiar</button>
        <button class="btn ghost sm" onclick={downloadCodes}>Descargar</button>
        <button class="btn sm" onclick={() => (codes = null)}>Ya los guardé</button>
      </div>
    </div>
  {:else if setup}
    <form onsubmit={enable} class="setup">
      <p>1. Escanea este código con tu app de autenticación.</p>
      <img src={setup.qr} alt="Código QR para la app de autenticación" width="220" height="220" />
      <p class="meta">¿No puedes escanear? Escribe esta clave en la app: <code>{setup.secret}</code></p>
      <label class="field">2. Escribe el código de 6 dígitos que muestra la app
        <input inputmode="numeric" autocomplete="one-time-code" maxlength="7" bind:value={code} required />
      </label>
      <div class="form-actions">
        <button class="btn" disabled={busy}>Activar</button>
        <button type="button" class="btn ghost" onclick={() => (setup = null)}>Cancelar</button>
      </div>
    </form>
  {:else if status.enabled}
    <p class="meta">Te quedan {status.recovery_left} códigos de respaldo.</p>
    {#if manage}
      <form onsubmit={submitManage}>
        <p>{manage === 'disable' ? 'Para desactivarla, confirma con tu contraseña y un código.' : 'Se generarán códigos nuevos y los anteriores dejarán de servir.'}</p>
        <div class="form-grid">
          <label class="field">Contraseña <input type="password" bind:value={mgmt.password} autocomplete="current-password" required /></label>
          <label class="field">Código de la app (o de respaldo) <input bind:value={mgmt.code} autocomplete="one-time-code" required /></label>
        </div>
        <div class="form-actions">
          <button class="btn" disabled={busy}>{manage === 'disable' ? 'Desactivar' : 'Generar códigos'}</button>
          <button type="button" class="btn ghost" onclick={() => (manage = null)}>Cancelar</button>
        </div>
      </form>
    {:else}
      <div class="form-actions">
        <button class="btn ghost" onclick={() => (manage = 'recovery')}>Generar códigos de respaldo nuevos</button>
        <button class="btn ghost" onclick={() => (manage = 'disable')}>Desactivar</button>
      </div>
    {/if}
  {:else}
    <button class="btn" disabled={busy} onclick={startSetup}>Activar verificación en dos pasos</button>
  {/if}
</section>

<section class="card sec">
  <h2>Sesiones</h2>
  <p class="meta">Si perdiste un dispositivo o crees que alguien más entró, cierra todas tus sesiones. Tendrás que iniciar sesión de nuevo.</p>
  <button class="btn ghost" disabled={busy} onclick={logoutAll}>Cerrar sesión en todos los dispositivos</button>
</section>

<style>
  .sec { margin-bottom: 1.25rem; }
  .sec h2 { font-size: 1.25rem; display: flex; align-items: center; gap: 10px; }
  .meta { color: var(--muted); font-size: 0.9rem; }
  .ok { color: var(--accent); padding: 0.7rem 0.9rem; border: 1px solid currentColor; border-radius: var(--radius-sm); margin: 0 0 1rem; font-size: 0.9rem; }
  .setup img { display: block; margin: 0.5rem 0 1rem; background: #fff; padding: 8px; border-radius: 8px; }
  code { background: var(--surface-2); padding: 2px 6px; border-radius: 6px; font-size: 0.9rem; word-break: break-all; }
  .codes { background: var(--surface-2); padding: 1rem; border-radius: var(--radius-sm); }
  .codes ul { list-style: none; padding: 0; margin: 0 0 1rem; display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: 8px; }
</style>
