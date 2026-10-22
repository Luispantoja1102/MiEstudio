<script>
  import Icon from '$lib/Icon.svelte';

  const giros = ['Barberías', 'Estéticas', 'Spas', 'Uñas'];
  // ejemplo ilustrativo de una semana: [día 0-4, hora de inicio (h), duración (h), persona 1-3, nombre]
  const week = [
    [0, 1, 1, 1, 'Corte clásico'], [0, 3.5, 1.5, 2, 'Color'],
    [1, 0.5, 1, 3, 'Corte y barba'], [1, 2.5, 1, 1, 'Fade'],
    [2, 1, 1.5, 2, 'Peinado'], [2, 3.5, 1, 3, 'Facial'],
    [3, 0, 1, 1, 'Barba'], [3, 2, 1.5, 3, 'Manicure'],
    [4, 1.5, 1, 2, 'Corte'], [4, 3, 1.5, 1, 'Corte y barba']
  ];
</script>

<section class="hero">
  <div class="container grid">
    <div class="copy">
      <p class="tag">Software para barberías y salones de belleza</p>
      <h1>La agenda y la caja de tu negocio, <em>en orden.</em></h1>
      <p class="lead">
        Tus clientes reservan solos, tú confirmas los anticipos y cobras en pesos. Citas, caja, equipo y clientes en un solo lugar, hecho en México.
      </p>
      <div class="actions">
        <a class="btn lg go" href="/registro">Probar 14 días gratis <Icon name="arrow" size={18} /></a>
        <a class="btn ghost lg" href="#funciones">Ver qué incluye</a>
      </div>
      <ul class="trust" aria-label="Ventajas">
        <li><Icon name="check" size={15} />Sin tarjeta para empezar</li>
        <li><Icon name="check" size={15} />Listo en minutos</li>
        <li><Icon name="check" size={15} />Cobros en pesos</li>
      </ul>
      <ul class="giros" aria-label="Giros">
        {#each giros as g}<li>{g}</li>{/each}
      </ul>
    </div>

    <figure class="stage" aria-label="Ejemplo de la agenda y de un comprobante por revisar">
      <div class="screen">
        <div class="bar"><b>Agenda</b><span class="seg"><i>Día</i><i class="on">Semana</i><i>Mes</i></span></div>
        <div class="cols">
          {#each ['Lun', 'Mar', 'Mié', 'Jue', 'Vie'] as d, i}
            <div class="col">
              <span class="dow">{d}</span>
              {#each week.filter((w) => w[0] === i) as [, start, len, who, name]}
                <span class="appt p{who}" style="top: {26 + start * 34}px; height: {len * 34 - 4}px">{name}</span>
              {/each}
            </div>
          {/each}
        </div>
      </div>
      <div class="float proof" role="img" aria-label="Aviso: comprobante recibido de Rosa Medina por 75 pesos">
        <span class="ico"><Icon name="receipt" size={18} /></span>
        <span class="txt"><b>Comprobante recibido</b><small>Rosa Medina · $75.00 · Corte clásico</small></span>
        <span class="ok">Confirmar</span>
      </div>
      <div class="float bell" aria-hidden="true"><Icon name="bell" size={18} /><i>3</i></div>
      <figcaption>Ejemplo ilustrativo</figcaption>
    </figure>
  </div>
</section>

<style>
  /* franja principal: el color de la barra lateral (tinta en claro, carbón en oscuro) */
  .hero { --text: var(--side-text); --muted: var(--side-muted); --accent: var(--side-mark); --accent-contrast: var(--side-mark-ink); position: relative; overflow: hidden; color: var(--side-text); background: var(--side); background-image: radial-gradient(ellipse 60% 50% at 8% 0%, rgba(192, 122, 101, 0.3), transparent 62%), radial-gradient(ellipse 55% 45% at 100% 100%, rgba(95, 130, 220, 0.22), transparent 60%); }
  .hero :global(.btn.ghost) { color: var(--side-text); border-color: var(--side-line); }
  .hero :global(.btn.ghost:hover) { background: var(--side-hover); border-color: var(--side-muted); }
  .grid { display: grid; gap: 56px; align-items: center; padding-block: 56px 72px; }
  .tag { margin: 0 0 1.1rem; font-size: 0.74rem; font-weight: 600; letter-spacing: 0.16em; text-transform: uppercase; color: var(--side-muted); }
  h1 { font-size: clamp(2.4rem, 4.6vw, 3.6rem); line-height: 1; margin: 0; text-wrap: balance; }
  h1 em { font-style: italic; color: var(--side-mark); }
  .lead { max-width: 34rem; margin: 1.4rem 0 2rem; font-size: 1.1rem; color: var(--side-muted); }
  .actions { display: flex; flex-wrap: wrap; gap: 12px; }
  .trust { display: flex; flex-wrap: wrap; gap: 8px 20px; list-style: none; margin: 1.6rem 0 0; padding: 0; font-size: 0.9rem; color: var(--side-muted); }
  .trust li { display: inline-flex; align-items: center; gap: 7px; }
  .trust :global(svg) { color: var(--side-mark); }
  .giros { display: flex; flex-wrap: wrap; gap: 8px; list-style: none; margin: 1.8rem 0 0; padding: 0; }
  .giros li { padding: 6px 16px; font-size: 0.85rem; border: 1px solid var(--side-line); border-radius: 999px; color: var(--side-muted); }

  .stage { position: relative; margin: 0; padding: 0 0 8px; font-variant-numeric: tabular-nums lining-nums; font-family: var(--font-body, inherit); }
  .screen { color: #1f1e1c; background: #fffdf9; border-radius: 22px; padding: 16px; box-shadow: 0 30px 70px rgba(0, 0, 0, 0.35); }
  .bar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; font-size: 0.95rem; }
  .seg { display: inline-flex; padding: 3px; gap: 2px; background: #efeae1; border-radius: 999px; font-size: 0.72rem; font-weight: 600; font-style: normal; }
  .seg i { padding: 3px 10px; border-radius: 999px; font-style: normal; color: #6b675f; }
  .seg i.on { background: #1c2f5c; color: #fff; }
  .cols { display: grid; grid-template-columns: repeat(5, 1fr); gap: 6px; }
  .col { position: relative; height: 210px; border-radius: 10px; background: repeating-linear-gradient(to bottom, transparent 0 33px, #ece6da 33px 34px) 0 26px / 100% 100% no-repeat, #faf6ee; }
  .dow { position: absolute; top: 6px; left: 0; right: 0; text-align: center; font-size: 0.68rem; font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; color: #6b675f; }
  .appt { position: absolute; left: 3px; right: 3px; padding: 3px 5px; overflow: hidden; font-size: 0.62rem; font-weight: 600; line-height: 1.2; border-radius: 7px; color: #1f1e1c; }
  .p1 { background: #dbe8ef; border: 1px solid #4b7f9c; }
  .p2 { background: #f1ddd2; border: 1px solid #b0643c; }
  .p3 { background: #e5e0f1; border: 1px solid #6a5aa6; }

  .float { position: absolute; display: flex; align-items: center; gap: 10px; color: #1f1e1c; background: #fffdf9; border-radius: 16px; box-shadow: 0 18px 44px rgba(0, 0, 0, 0.32); }
  .proof { left: -18px; bottom: 26px; padding: 11px 14px; max-width: 330px; animation: bob 6s ease-in-out infinite; }
  .ico { display: grid; place-items: center; flex: none; width: 34px; height: 34px; border-radius: 50%; background: #f6e7bf; color: #7a4f00; }
  .txt { display: grid; min-width: 0; font-size: 0.82rem; }
  .txt small { color: #6b675f; font-size: 0.72rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .ok { flex: none; padding: 6px 12px; font-size: 0.74rem; font-weight: 700; color: #fff; background: #1c2f5c; border-radius: 999px; }
  .bell { right: -8px; top: -14px; position: absolute; justify-content: center; width: 44px; height: 44px; border-radius: 50%; color: #1c2f5c; }
  .bell i { position: absolute; top: -4px; right: -4px; min-width: 19px; height: 19px; display: grid; place-items: center; font-size: 0.68rem; font-weight: 700; font-style: normal; color: #fff; background: #b0302a; border-radius: 999px; border: 2px solid #fffdf9; }
  figcaption { margin-top: 18px; text-align: center; font-size: 0.78rem; color: var(--side-muted); }
  @keyframes bob { 50% { transform: translateY(-7px); } }
  @media (prefers-reduced-motion: reduce) { .proof { animation: none; } }
  @media (min-width: 980px) { .grid { grid-template-columns: 1.05fr 0.95fr; padding-block: 88px 104px; } }
  @media (max-width: 520px) { .proof { left: 0; right: 0; max-width: none; bottom: 18px; } .bell { right: 4px; } }
</style>
