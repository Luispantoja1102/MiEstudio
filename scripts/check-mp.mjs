// Revisa la configuración de Mercado Pago antes de probar con dinero (real o de prueba):
//   npm run check:mp
// No cobra nada ni modifica nada: solo lee. Sale con código 1 si algo obligatorio falla.
import { existsSync } from 'node:fs';

if (existsSync('.env')) process.loadEnvFile('.env');
const env = process.env;
const base = (env.MP_API_BASE || 'https://api.mercadopago.com').replace(/\/$/, '');
const authBase = (env.MP_AUTH_BASE || 'https://auth.mercadopago.com.mx').replace(/\/$/, '');
let bad = 0;
const ok = (m) => console.log(`\x1b[32m✓\x1b[0m ${m}`);
const warn = (m) => console.log(`\x1b[33m!\x1b[0m ${m}`);
const fail = (m) => (bad++, console.log(`\x1b[31m✗\x1b[0m ${m}`));
const info = (m) => console.log(`  ${m}`);

async function http(url, opts = {}) {
  try {
    const res = await fetch(url, { signal: AbortSignal.timeout(15000), ...opts });
    const text = await res.text();
    let json = null;
    try { json = JSON.parse(text); } catch { /* no es JSON */ }
    return { status: res.status, json, text };
  } catch (e) {
    return { status: 0, error: e.cause?.code || e.message };
  }
}

console.log('\nMercado Pago — revisión de configuración\n');

// 1. dirección pública (los avisos automáticos y el regreso del pago necesitan que Mercado Pago te alcance)
const pub = (env.API_PUBLIC_URL || '').replace(/\/(api)?\/?$/, '');
if (!pub) {
  fail('API_PUBLIC_URL está vacío. Sin una dirección pública, Mercado Pago no puede avisarte de los pagos (la cita quedaría «pendiente» hasta que alguien la abra).');
  info('En local usa un túnel (ngrok, cloudflared) y pon aquí su dirección https.');
} else {
  if (!pub.startsWith('https://')) warn(`API_PUBLIC_URL (${pub}) no es https: Mercado Pago rechaza el regreso automático y los avisos en http.`);
  let host = '';
  try { host = new URL(pub).hostname; } catch { /* se avisa abajo */ }
  if (!host) fail(`API_PUBLIC_URL (${pub}) no es una dirección válida.`);
  else if (['localhost', '127.0.0.1', '[::1]', '::1', '0.0.0.0'].includes(host) || host.endsWith('.local')) fail('API_PUBLIC_URL apunta a esta computadora: Mercado Pago no puede llegar ahí. Usa un túnel o tu dominio.');
  else {
    const h = await http(`${pub}/health`);
    if (h.status === 200) ok(`${pub}/health responde (la API es alcanzable desde internet)`);
    else fail(`${pub}/health no responde (${h.status || h.error}). ¿Está corriendo la API y el túnel/proxy apunta a ella?`);
    const w = await http(`${pub}/api/webhooks/mercadopago?biz=0`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: '{}' });
    if (w.status && w.status < 500 && w.status !== 404) ok(`el aviso /api/webhooks/mercadopago es alcanzable (respondió ${w.status})`);
    else fail(`/api/webhooks/mercadopago no es alcanzable (${w.status || w.error}). Revisa el proxy: debe reenviar /api/* a la API.`);
  }
}

// 2. aplicación de la plataforma (OAuth: cada negocio conecta su cuenta)
if (!env.MP_CLIENT_ID || !env.MP_CLIENT_SECRET) {
  warn('MP_CLIENT_ID / MP_CLIENT_SECRET vacíos: los negocios no podrán conectar su cuenta con un clic (solo pegando un token a mano).');
} else {
  const r = await http(`${base}/oauth/token`, {
    method: 'POST',
    headers: { 'content-type': 'application/x-www-form-urlencoded', accept: 'application/json' },
    body: new URLSearchParams({ grant_type: 'client_credentials', client_id: env.MP_CLIENT_ID, client_secret: env.MP_CLIENT_SECRET })
  });
  if (r.status === 200) ok('MP_CLIENT_ID y MP_CLIENT_SECRET son válidos');
  else if (r.status === 0) warn(`no se pudo contactar a Mercado Pago (${r.error}); no se validó la aplicación`);
  else fail(`Mercado Pago rechazó MP_CLIENT_ID/MP_CLIENT_SECRET (${r.status}${r.json?.message ? ': ' + r.json.message : ''})`);
  const redirect = env.MP_OAUTH_REDIRECT || (pub ? `${pub}/api/point/oauth/callback` : '');
  if (redirect) info(`Redirect URL que debe tener TU aplicación en Mercado Pago, letra por letra: ${redirect}`);
}

// 3. cuenta de la plataforma para cobrar suscripciones
const who = async (label, token) => {
  const r = await http(`${base}/users/me`, { headers: { authorization: `Bearer ${token}` } });
  if (r.status === 200) {
    const nick = r.json?.nickname || r.json?.email || r.json?.id;
    const test = /^TEST/i.test(String(r.json?.nickname || '')) || (r.json?.tags || []).includes?.('test_user');
    ok(`${label}: cuenta «${nick}» (país ${r.json?.site_id || '?'})${test ? ' — es una cuenta de PRUEBA' : ' — es una cuenta REAL'}`);
    if (r.json?.site_id && r.json.site_id !== 'MLM') warn(`${label}: la cuenta es de ${r.json.site_id}, no de México (MLM); los cobros son en MXN.`);
    if (env.APP_ENV === 'production' && test) warn(`${label}: APP_ENV=production pero la cuenta es de prueba.`);
    if (env.APP_ENV !== 'production' && !test) warn(`${label}: es una cuenta real; los cobros que pruebes serán dinero real.`);
  } else if (r.status === 0) warn(`${label}: no se pudo contactar a Mercado Pago (${r.error})`);
  else fail(`${label}: Mercado Pago no aceptó el token (${r.status}${r.json?.message ? ': ' + r.json.message : ''})`);
};
if (env.MPP_ACCESS_TOKEN) await who('MPP_ACCESS_TOKEN (suscripciones)', env.MPP_ACCESS_TOKEN);
else warn('MPP_ACCESS_TOKEN vacío: las suscripciones se registran a mano desde el panel de plataforma (no hay pago en línea).');

// 4. firmas de los avisos
env.MP_WEBHOOK_SECRET ? ok('MP_WEBHOOK_SECRET definido: los avisos de anticipos se verifican por firma') : info('MP_WEBHOOK_SECRET vacío: opcional; cada pago se verifica de todas formas consultándolo en Mercado Pago.');
env.MPP_WEBHOOK_SECRET ? ok('MPP_WEBHOOK_SECRET definido') : info('MPP_WEBHOOK_SECRET vacío: opcional, por la misma razón.');
if (!env.TOKEN_ENC_KEY) info('TOKEN_ENC_KEY no definida: se deriva de JWT_SECRET. Si cambias JWT_SECRET, cada negocio tendrá que volver a conectar Mercado Pago.');

console.log(bad ? `\n${bad} problema(s) por resolver.\n` : '\nTodo lo obligatorio está en orden.\n');
process.exit(bad ? 1 : 0);
