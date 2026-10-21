<script>
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import PublicLayout from '$lib/PublicLayout.svelte';

  const slug = page.params.slug;
  let d = $state(null);
  let missing = $state(false);
  onMount(async () => {
    const r = await fetch(`/api/public/${slug}/privacy`);
    if (r.status === 404) missing = true;
    else d = await r.json();
  });
</script>

<PublicLayout title={missing ? 'Página no disponible' : 'Aviso de privacidad'} subtitle={d ? `${d.business} · versión ${d.version}` : ''}>
  {#if missing}
    <div class="card"><p>No encontramos esta página.</p></div>
  {:else if d}
    <div class="card legal">
      <p><b>{d.business}</b> es responsable del tratamiento de tus datos personales cuando reservas o te atiende.</p>
      <h2>Qué datos usamos</h2>
      <p>Tu nombre, teléfono y, si lo das, tu correo; los servicios que reservas o recibes, y las notas de seguimiento que el negocio registre para atenderte mejor (por ejemplo preferencias o cuidados).</p>
      <h2>Para qué</h2>
      <p>Para agendar y atenderte, confirmarte tu cita, avisarte si cambia, cobrar lo que corresponda y, solo si lo aceptas, enviarte recordatorios por correo. No vendemos tus datos.</p>
      <h2>Con quién se comparten</h2>
      <p>La plataforma MiEstudio{#if d.platform} ({d.platform}){/if} los procesa por cuenta del negocio para prestar el servicio, y Mercado Pago cuando pagas un anticipo. No se comparten con nadie más salvo que una autoridad lo exija.</p>
      <h2>Tus derechos</h2>
      <p>Puedes pedir acceso a tus datos, rectificarlos, cancelarlos (borrarlos) u oponerte a su uso. Responderemos en máximo {d.days} días hábiles.</p>
      <p><a class="btn" href="/arco/{slug}">Ejercer mis derechos</a></p>
      {#if d.privacy_email}<p class="meta">También puedes escribir a <a href="mailto:{d.privacy_email}">{d.privacy_email}</a>.</p>{/if}
      <h2>Dónde estamos</h2>
      <ul>{#each d.branches as b}<li><b>{b.name}</b>{#if b.address} · {b.address}{/if}{#if b.phone} · {b.phone}{/if}</li>{/each}</ul>
      <p class="meta">Conservamos tus datos mientras sigas siendo cliente y, después, solo lo que la ley nos obligue a guardar (por ejemplo, ventas). Si cambia este aviso, la nueva versión se publica aquí.</p>
    </div>
  {/if}
</PublicLayout>

<style>
  .legal h2 { font-size: 1.1rem; margin: 1.25rem 0 0.3rem; }
  .meta { color: var(--muted); font-size: 0.88rem; }
</style>
