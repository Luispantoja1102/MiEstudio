<script>
  import { goto } from '$app/navigation';
  import AccountSettings from '$lib/AccountSettings.svelte';
  import Appearance from '$lib/Appearance.svelte';
  import BusinessProfile from '$lib/BusinessProfile.svelte';
  import SettingsHub from '$lib/SettingsHub.svelte';
  import { api } from '$lib/api.js';
  import { session } from '$lib/session.svelte.js';
  import Equipo from '../equipo/+page.svelte';
  import Pagos from '../pagos/+page.svelte';
  import Privacidad from '../privacidad/+page.svelte';
  import Reserva from '../reserva/+page.svelte';
  import Suscripcion from '../suscripcion/+page.svelte';
  import Sucursales from './sucursales/+page.svelte';

  const A = ['admin'];
  // Agrupado por lo que la persona busca, como en MiConsultorio.
  const groups = [
    {
      title: 'Mi negocio',
      items: [
        { id: 'negocio', label: 'Datos del negocio', desc: 'Nombre, giro y zona horaria', icon: 'home', component: BusinessProfile, roles: A },
        { id: 'equipo', label: 'Equipo', desc: 'Agregar personal, roles, comisiones y horarios', icon: 'users', component: Equipo, roles: A },
        { id: 'sucursales', label: 'Sucursales', desc: 'Tus locales: nombre, dirección y teléfono', icon: 'home', component: Sucursales, roles: A },
        { id: 'suscripcion', label: 'Suscripción y plan', desc: 'Tu plan de MiEstudio, cuánto pagas y tus pagos', icon: 'wallet', component: Suscripcion, roles: A }
      ]
    },
    {
      title: 'Agenda y clientes',
      items: [
        { id: 'reserva', label: 'Reservas y página pública', desc: 'Tu dirección en línea, anticipos y recordatorios por correo', icon: 'globe', component: Reserva, roles: A },
        { id: 'privacidad', label: 'Privacidad y solicitudes ARCO', desc: 'Lo que tus clientes piden sobre sus datos y tu aviso de privacidad', icon: 'shield', component: Privacidad, roles: A }
      ]
    },
    {
      title: 'Cobros',
      items: [{ id: 'pagos', label: 'Cobro con tarjeta (Mercado Pago)', desc: 'Conecta tu cuenta de Mercado Pago y tu terminal', icon: 'card', component: Pagos, roles: A }]
    },
    {
      title: 'Mi cuenta y esta app',
      items: [
        { id: 'cuenta', label: 'Mi cuenta', desc: 'Contraseña, verificación en dos pasos y sesiones', icon: 'user', component: AccountSettings },
        { id: 'apariencia', label: 'Apariencia', desc: 'Tema claro u oscuro', icon: 'sun', component: Appearance }
      ]
    }
  ];

  async function logout() {
    await api('/api/auth/logout', { method: 'POST' }).catch(() => {});
    session.me = null;
    goto('/');
  }
</script>

<SettingsHub intro="Tu negocio, tus clientes, tus cobros y tu cuenta." {groups} onlogout={logout} />
