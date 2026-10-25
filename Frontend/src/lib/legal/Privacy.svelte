<script>
  let { op } = $props(); // datos de quien opera la plataforma
</script>

<p>Este aviso explica cómo <b>{op.name}</b>, que opera Barbella, trata los datos de <b>las personas que usan la plataforma para administrar su negocio</b> (dueños y su equipo). Los datos de los clientes de cada negocio los trata ese negocio: consulta el aviso que aparece en su página de reservas.</p>

<h2>Qué datos recibimos</h2>
<ul>
  <li>De tu cuenta: nombre, correo, usuario, contraseña (guardada solo como hash), rol y, si la activas, tu verificación en dos pasos.</li>
  <li>De tu negocio: nombre, giro, sucursales, equipo, servicios, productos, citas, ventas y notas de seguimiento que tú captures.</li>
  <li>De pagos: si conectas Mercado Pago, su identificador de cuenta y un token cifrado. <b>No guardamos números de tarjeta.</b></li>
  <li>Técnicos: dirección IP y registros de actividad, solo para seguridad y para limitar intentos abusivos.</li>
</ul>

<h2>Para qué los usamos</h2>
<ul>
  <li>Darte el servicio: agenda, punto de venta, reportes, correos de confirmación y recordatorios que tu negocio decida enviar.</li>
  <li>Mantener la seguridad de la cuenta (inicio de sesión, recuperación de contraseña, detección de abuso).</li>
  <li>Cobrarte la suscripción, avisarte de tu cuenta y atender tu soporte.</li>
</ul>
<p>No vendemos datos ni los usamos para publicidad de terceros.</p>

<h2>Con quién se comparten</h2>
<p>Solo con proveedores necesarios para operar: alojamiento de la plataforma, envío de correo y Mercado Pago cuando tú lo conectas. También podemos entregar información a una autoridad que lo exija por ley.</p>

<h2>Cookies</h2>
<p>Usamos únicamente una cookie técnica de sesión, necesaria para mantenerte dentro de tu cuenta. No usamos cookies de publicidad ni de seguimiento, por eso no mostramos un banner de cookies.</p>

<h2>Tus derechos (ARCO)</h2>
<p>Puedes acceder a tus datos y rectificarlos en cualquier momento desde tu cuenta (Ajustes). Para cancelarlos u oponerte a su uso, escríbele a {op.name}, el responsable, a <a href="mailto:{op.email}">{op.email}</a>. Responderemos en un máximo de 20 días hábiles. También puedes revocar tu consentimiento o limitar el uso de tus datos por ese mismo correo.</p>

<h2>Conservación</h2>
<p>Conservamos tus datos mientras tengas una cuenta. Al cancelar, borramos o anonimizamos los datos personales, salvo lo que debamos guardar por obligaciones legales o fiscales.</p>

<h2>Cambios a este aviso</h2>
<p>Si cambia de fondo, publicaremos la nueva versión aquí, con su fecha, y la aceptación quedará registrada con el número de versión.</p>
