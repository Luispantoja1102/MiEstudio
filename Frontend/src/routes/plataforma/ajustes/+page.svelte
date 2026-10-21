<script>
  import { goto } from '$app/navigation';
  import AccountSettings from '$lib/AccountSettings.svelte';
  import Appearance from '$lib/Appearance.svelte';
  import SettingsHub from '$lib/SettingsHub.svelte';
  import { api } from '$lib/api.js';
  import { session } from '$lib/session.svelte.js';
  import Actividad from '../actividad/+page.svelte';
  import Equipo from '../equipo/+page.svelte';
  import Planes from '../planes/+page.svelte';
  import Salud from '../salud/+page.svelte';
  import Registro from './registro/+page.svelte';

  const groups = [
    {
      title: 'Negocios y cobros',
      items: [
        { id: 'planes', label: 'Planes', desc: 'Qué incluye cada suscripción y cuántos negocios la usan', icon: 'package', component: Planes },
        { id: 'registro', label: 'Registro de negocios', desc: 'Abierto, con aprobación o cerrado', icon: 'globe', component: Registro }
      ]
    },
    {
      title: 'Operación',
      items: [
        { id: 'salud', label: 'Salud del sistema', desc: 'Correo, Mercado Pago, base de datos y bandeja de salida', icon: 'chart', component: Salud },
        { id: 'actividad', label: 'Actividad', desc: 'Bitácora de lo que ocurre en la plataforma', icon: 'bell', component: Actividad },
        { id: 'equipo', label: 'Equipo de plataforma', desc: 'Administradores y soporte, con sus accesos', icon: 'users', component: Equipo }
      ]
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

<SettingsHub intro="Planes, registro, salud del sistema, equipo y tu cuenta." {groups} onlogout={logout} />
