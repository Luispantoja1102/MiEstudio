// Arma el correo de estado del deploy (texto y HTML) para GitHub Actions o una vista previa local:
//   node scripts/deploy-mail.mjs            (con las variables de entorno del workflow)
//   node scripts/deploy-mail.mjs --preview  (datos de ejemplo; deja deploy-status.html para abrirlo)
import fs from 'node:fs';

const preview = process.argv.includes('--preview');
const env = preview
  ? { DEPLOY_RESULT: 'success', REPOSITORY: 'Luispantoja1102/MiEstudio', BRANCH: 'main', SHA: '359dabd0f1c2a3b4c5d6e7f8091a2b3c4d5e6f70', COMMIT_AUTHOR: 'Luispantoja1102', PUSHER: 'Luispantoja1102',
      COMMIT_MESSAGE: 'Registro: verificación del correo con código de 6 dígitos', PROD_URL: 'https://barbella.mx', SERVICE_NAME: 'Barbella',
      COMMIT_URL: 'https://github.com/Luispantoja1102/MiEstudio/commit/359dabd', RUN_URL: 'https://github.com/Luispantoja1102/MiEstudio/actions/runs/1', MAIL_READY: 'true' }
  : process.env;

const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
const ok = env.DEPLOY_RESULT === 'success';
const ciFailed = String(env.CI_FAILED) === 'true';
const service = env.SERVICE_NAME || 'Barbella';
const short = (env.SHA || '').slice(0, 7) || '(sin SHA)';
const message = (env.COMMIT_MESSAGE || '').trim() || '(sin mensaje)';
const when = new Intl.DateTimeFormat('es-MX', { timeZone: 'America/Tijuana', day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: true }).format(new Date()) + ' · Tijuana';

const title = ok ? 'Deploy publicado' : ciFailed ? 'Deploy omitido: el CI falló' : 'El deploy falló';
const lead = ok
  ? `La versión nueva de ${service} ya está publicada en el servidor y respondió bien.`
  : ciFailed
    ? `Las pruebas del CI fallaron, así que no se publicó nada. Producción sigue con la versión anterior.`
    : `No se pudo publicar ${service}. Producción puede seguir con la versión anterior: revisa el registro.`;
const color = ok ? '#2f6a3b' : '#a8472d';
const facts = [['Servicio', service], ['Repositorio', env.REPOSITORY], ['Rama', env.BRANCH], ['SHA', short], ['Autor', env.COMMIT_AUTHOR || '(desconocido)'], ['Publicó', env.PUSHER || '(desconocido)'], ['Fecha', when]];
const links = ok
  ? [['Ver producción', env.PROD_URL], ['Ver el commit', env.COMMIT_URL]]
  : [['Ver el registro del error', env.RUN_URL], ['Ver el commit', env.COMMIT_URL]];

const html = `<div style="font-family:system-ui,sans-serif;max-width:560px;margin:0 auto;padding:24px;color:#1f2a28">
<div style="display:inline-block;padding:4px 12px;border-radius:99px;background:${color};color:#fff;font-size:12px;font-weight:700;letter-spacing:.06em;text-transform:uppercase">${ok ? 'Éxito' : 'Fallo'}</div>
<h2 style="margin:12px 0 8px">${esc(title)}</h2>
<p style="margin:0 0 16px;line-height:1.6">${esc(lead)}</p>
<table style="width:100%;border-collapse:collapse;margin:0 0 16px">${facts.map(([k, v]) => `<tr><td style="padding:6px 12px 6px 0;color:#5b6864;width:110px">${esc(k)}</td><td style="padding:6px 0;${k === 'SHA' ? 'font-family:ui-monospace,Menlo,monospace' : ''}">${esc(v)}</td></tr>`).join('')}</table>
<div style="font-size:11px;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:#5b6864;margin:0 0 6px">Mensaje del commit</div>
<div style="border-left:3px solid ${color};padding:10px 14px;background:#f3f1ec;margin:0 0 18px;line-height:1.5">${esc(message).replace(/\n/g, '<br>')}</div>
${links.filter(([, u]) => u).map(([l, u], i) => `<a href="${esc(u)}" style="display:inline-block;margin:0 8px 8px 0;padding:10px 18px;border-radius:8px;text-decoration:none;font-weight:600;${i === 0 ? `background:${color};color:#fff` : 'border:1px solid #cfc9bb;color:#1f2a28'}">${esc(l)}</a>`).join('')}
<p style="font-size:13px;color:#5b6864;margin-top:18px">— ${esc(service)}</p></div>`;

const text = [`${service} · ${ok ? 'ÉXITO' : 'FALLO'}`, title, '', lead, '', ...facts.map(([k, v]) => `${k}: ${v}`), '', 'Mensaje del commit:', message, '', ...links.filter(([, u]) => u).map(([l, u]) => `${l}: ${u}`), ''].join('\n');
const subject = `${ok ? '✅' : '❌'} ${service}: ${title} (${short})`;

fs.writeFileSync('deploy-status.html', html);
fs.writeFileSync('deploy-status.txt', text);
if (process.env.GITHUB_OUTPUT) {
  fs.appendFileSync(process.env.GITHUB_OUTPUT, `ready=${String(env.MAIL_READY) === 'true'}\nresult=${ok ? 'success' : 'failure'}\nsubject=${subject}\n`);
} else {
  console.log(subject + '\n\n' + text);
}
