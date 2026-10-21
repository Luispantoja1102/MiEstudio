<script>
  import SettingsHub from '$lib/SettingsHub.svelte';

  const groups = [
    {
      title: 'Negocio y cobros',
      items: [
        { icon: 'package', title: 'Planes', text: 'Qué incluye cada suscripción y cuántos negocios la usan.', href: '/plataforma/planes' },
        { icon: 'globe', title: 'Registro de negocios', text: 'Abierto, con aprobación o cerrado.', href: '/plataforma/ajustes/registro' }
      ]
    },
    {
      title: 'Operación',
      items: [
        { icon: 'chart', title: 'Salud del sistema', text: 'Correo, Mercado Pago, base de datos y bandeja de salida.', href: '/plataforma/salud' },
        { icon: 'bell', title: 'Actividad', text: 'Bitácora de lo que ocurre en la plataforma.', href: '/plataforma/actividad' },
        { icon: 'users', title: 'Equipo de plataforma', text: 'Administradores y soporte, con sus accesos.', href: '/plataforma/equipo' }
      ]
    },
    {
      title: 'Mi cuenta',
      items: [{ icon: 'shield', title: 'Mi cuenta', text: 'Contraseña, verificación en dos pasos y sesiones.', href: '/plataforma/cuenta' }]
    }
  ];
</script>

<SettingsHub intro="Lo que configuras de vez en cuando." {groups} />
