<script>
  import SettingsHub from '$lib/SettingsHub.svelte';

  const A = ['admin'];
  const groups = [
    {
      title: 'Mi negocio',
      items: [
        { icon: 'users', title: 'Equipo', text: 'Personas con acceso, roles, comisiones y horarios.', href: '/panel/equipo', roles: A },
        { icon: 'home', title: 'Sucursales', text: 'Tus locales: nombre, dirección y teléfono.', href: '/panel/ajustes/sucursales', roles: A }
      ]
    },
    {
      title: 'Clientes y reservas',
      items: [
        { icon: 'globe', title: 'Reserva en línea', text: 'Tu página pública, anticipos y recordatorios por correo.', href: '/panel/reserva', roles: A },
        { icon: 'shield', title: 'Privacidad', text: 'Solicitudes de tus clientes sobre sus datos (ARCO) y tu aviso.', href: '/panel/privacidad', roles: A }
      ]
    },
    {
      title: 'Cobros y plan',
      items: [
        { icon: 'card', title: 'Pagos', text: 'Conecta Mercado Pago y registra tu terminal.', href: '/panel/pagos', roles: A },
        { icon: 'wallet', title: 'Suscripción', text: 'Tu plan, vigencia, pagos y facturación de MiEstudio.', href: '/panel/suscripcion', roles: A }
      ]
    },
    {
      title: 'Mi cuenta',
      items: [{ icon: 'shield', title: 'Mi cuenta', text: 'Contraseña, verificación en dos pasos y sesiones.', href: '/panel/cuenta' }]
    }
  ];
</script>

<SettingsHub intro="Lo que configuras de vez en cuando." {groups} />
