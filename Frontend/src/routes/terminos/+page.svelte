<script>
  import LegalPage from '$lib/LegalPage.svelte';
</script>

<LegalPage title="Términos y condiciones">
  {#snippet children({ op })}
    <p>Al crear una cuenta en Barbella aceptas estos términos y nuestro <a href="/privacidad">aviso de privacidad</a>.</p>

    <h2>El servicio</h2>
    <p>Barbella es una plataforma para administrar agenda, citas, cobros, clientes e inventario de salones, spas y barberías. Se ofrece tal como está descrita en la página y puede cambiar o mejorar con el tiempo; avisaremos los cambios importantes.</p>

    <h2>Tu cuenta</h2>
    <ul>
      <li>Eres responsable de la actividad de tu cuenta y de tu equipo, y de cuidar tus contraseñas. Te recomendamos activar la verificación en dos pasos.</li>
      <li>Debes dar datos verdaderos y mantenerlos al día.</li>
      <li>No puedes usar el servicio para actividades ilegales, para enviar correo no solicitado ni para vulnerar la seguridad de la plataforma.</li>
    </ul>

    <h2>Datos de tus clientes</h2>
    <p>Tú decides qué datos de tus clientes capturas y para qué. Eres el responsable de ese tratamiento: debes contar con su consentimiento cuando corresponda y atender sus solicitudes de privacidad. Barbella te da herramientas para hacerlo (aviso de privacidad, solicitudes ARCO, exportar y borrar datos) y actúa como tu encargado, solo para prestarte el servicio.</p>

    <h2>Pruebas, planes y pagos</h2>
    <p>Puedes empezar con un periodo de prueba. Después, el servicio continúa con el plan que elijas y su precio publicado. Si un pago no llega, la cuenta pasa a pago atrasado y, pasada la gracia, puede suspenderse; tus datos se conservan para que puedas reactivarla.</p>

    <h2>Pagos de tus clientes</h2>
    <p>Los cobros con Mercado Pago (terminal, anticipos) se hacen en tu propia cuenta de Mercado Pago, que conectas tú. Barbella no recibe ni retiene ese dinero y no es responsable de las políticas de Mercado Pago.</p>

    <h2>Disponibilidad y límites</h2>
    <p>Trabajamos para que el servicio esté disponible y respaldado, pero no podemos garantizar que no tenga interrupciones. En la medida que la ley permita, nuestra responsabilidad se limita a lo que hayas pagado por el servicio en los últimos tres meses.</p>

    <h2>Cancelación</h2>
    <p>Puedes dejar de usar el servicio cuando quieras. Podemos suspender cuentas que incumplan estos términos. Al cancelar puedes pedir una copia de tus datos.</p>

    <h2>Cambios y contacto</h2>
    <p>Podemos actualizar estos términos; la versión vigente se publica aquí con su fecha y se registra con tu aceptación. Dudas: {#if op.email}<a href="mailto:{op.email}">{op.email}</a>{:else}el correo del responsable{/if}.</p>
  {/snippet}
</LegalPage>
