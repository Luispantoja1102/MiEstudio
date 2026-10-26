<script>
  import { goto } from '$app/navigation';
  import AccountSettings from '$lib/AccountSettings.svelte';
  import Appearance from '$lib/Appearance.svelte';
  import BusinessProfile from '$lib/BusinessProfile.svelte';
  import SettingsHub from '$lib/SettingsHub.svelte';
  import { api } from '$lib/api.js';
  import { onMount } from 'svelte';
  import { hasMercadoPago, hasPos, planLimit, session } from '$lib/session.svelte.js';
  import Equipo from '../equipo/+page.svelte';
  import Fidelidad from './fidelidad/+page.svelte';
  import Impresora from './impresora/+page.svelte';
  import Mensajes from '../mensajes/+page.svelte';
  import Pagos from '../pagos/+page.svelte';
  import Privacidad from '../privacidad/+page.svelte';
  import Reserva from '../reserva/+page.svelte';
  import Suscripcion from '../suscripcion/+page.svelte';
  import Sucursales from './sucursales/+page.svelte';

  const A = ['admin'];
  // Agrupado por lo que la persona busca, como en MiConsultorio.
  const all = [
    {
      title: 'Mi negocio',
      items: [
        { id: 'negocio', label: 'Datos del negocio', desc: 'Nombre, contacto y zona horaria', icon: 'home', component: BusinessProfile, roles: A },
        { id: 'equipo', label: 'Equipo', desc: 'Personal, roles y horarios', icon: 'users', component: Equipo, roles: A },
        { id: 'sucursales', label: 'Sucursales', desc: 'Tus locales y su dirección', icon: 'home', component: Sucursales, roles: A }
      ]
    },
    {
      title: 'Clientes',
      items: [
        { id: 'fidelidad', label: 'Tarjeta de lealtad', desc: 'Sellos y premio para tus clientes frecuentes', icon: 'user-plus', component: Fidelidad, roles: A },
        { id: 'reserva', label: 'Reservas en línea', desc: 'Tu página, anticipos y recordatorios', icon: 'globe', component: Reserva, roles: A },
        { id: 'mensajes', label: 'Mensajes a clientes', desc: 'Cumpleaños, reactivación y encuesta', icon: 'globe', component: Mensajes, roles: A },
        { id: 'privacidad', label: 'Privacidad y ARCO', desc: 'Aviso y solicitudes de tus clientes', icon: 'shield', component: Privacidad, roles: A }
      ]
    },
    {
      title: 'Dinero',
      items: [
        { id: 'suscripcion', label: 'Suscripción y plan', desc: 'Tu plan de Barbella y tus pagos', icon: 'wallet', component: Suscripcion, roles: A },
        { id: 'impresora', label: 'Impresora y ticket', desc: 'Térmica, cajón y estación de impresión', icon: 'receipt', component: Impresora, roles: [...A, 'cashier', 'reception'] },
        { id: 'pagos', label: 'Mercado Pago', desc: 'Cobro con tarjeta y terminal', icon: 'card', component: Pagos, roles: A }
      ]
    },
    {
      title: 'Mi cuenta',
      items: [
        { id: 'cuenta', label: 'Mi cuenta', desc: 'Contraseña, dos pasos y sesiones', icon: 'user', component: AccountSettings },
        { id: 'apariencia', label: 'Apariencia', desc: 'Elige tu tema claro', icon: 'sun', component: Appearance }
      ]
    }
  ];

  // Sucursales solo aparece si el plan admite varias, o si el negocio ya tiene más de una (por ejemplo, tras bajar de plan, para poder desactivarlas).
  // La dirección y el teléfono de la única sucursal se editan en «Datos del negocio».
  let branchCount = $state(0);
  onMount(() => {
    api('/api/branches').then((l) => (branchCount = l.length)).catch(() => {});
  });
  const manyBranches = $derived((planLimit('max_branches') ?? 1) > 1 || branchCount > 1);
  // Mercado Pago solo aparece si el plan incluye anticipos con MP o terminal
  const groups = $derived(
    all.map((g) => ({
      ...g,
      items: g.items.filter((i) => (i.id !== 'pagos' || hasMercadoPago()) && (i.id !== 'sucursales' || manyBranches) && (i.id !== 'impresora' || hasPos()) && (i.id !== 'fidelidad' || hasPos()))
    }))
  );

  async function logout() {
    await api('/api/auth/logout', { method: 'POST' }).catch(() => {});
    session.me = null;
    goto('/');
  }
</script>

<SettingsHub intro="Tu negocio, tus clientes, tus cobros y tu cuenta." {groups} onlogout={logout} />
