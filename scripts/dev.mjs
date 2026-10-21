// Arranque de desarrollo: revisa el entorno, compila la API de Go, y levanta API, servicio Node y frontend.
// Si algo falla explica por qué y detiene todo, en vez de dejar al frontend sin API.
import { spawn, spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import net from 'node:net';
import path from 'node:path';

const win = process.platform === 'win32';
const COLORS = { dev: 35, go: 33, node: 32, front: 36 };
const paint = (tag, s) => `\x1b[${COLORS[tag]}m[${tag}]\x1b[0m ${s}`;
const say = (tag, s) => console.log(paint(tag, s));

function die(msg, hint) {
  console.error(`\n\x1b[31m✖ ${msg}\x1b[0m`);
  if (hint) console.error(hint.replace(/^/gm, '  '));
  console.error('');
  process.exit(1);
}

// ── 1. .env y variables obligatorias
if (!existsSync('.env')) {
  die('No existe el archivo .env', 'Créalo desde el ejemplo:\n  macOS/Linux:  cp .env.example .env\n  Windows:      copy .env.example .env');
}
process.loadEnvFile('.env');
const env = process.env;
const missing = ['DATABASE_URL', 'JWT_SECRET'].filter((k) => !env[k]);
if (missing.length) {
  die(`Faltan variables en .env: ${missing.join(', ')}`, 'Tu .env puede ser de una versión anterior. Compáralo con .env.example y copia las variables que falten.');
}
if (env.JWT_SECRET.length < 32) die('JWT_SECRET es muy corto (mínimo 32 caracteres)', 'Genera uno con: openssl rand -hex 32');
if (!env.PLATFORM_ADMIN_EMAIL || !env.PLATFORM_ADMIN_PASSWORD) {
  say('dev', 'Aviso: no hay PLATFORM_ADMIN_EMAIL/PLATFORM_ADMIN_PASSWORD en .env (copia las de .env.example para crear un administrador de plataforma).');
}
const apiPort = env.GO_API_PORT || '8080';

// ── 2. PostgreSQL responde
let dbUrl;
try {
  dbUrl = new URL(env.DATABASE_URL);
} catch {
  die('DATABASE_URL no es una URL válida', 'Ejemplo: postgres://usuario:clave@localhost:5432/miestudio?sslmode=disable');
}
const dbHost = dbUrl.hostname || 'localhost';
const dbPort = Number(dbUrl.port || 5432);
const reachable = (host, port) =>
  new Promise((resolve) => {
    const s = net.connect({ host, port, timeout: 3000 });
    s.once('connect', () => (s.destroy(), resolve(true)));
    s.once('timeout', () => (s.destroy(), resolve(false)));
    s.once('error', () => resolve(false));
  });
if (!(await reachable(dbHost, dbPort))) {
  die(`PostgreSQL no responde en ${dbHost}:${dbPort}`, 'Con Docker:  npm run db   (y espera unos segundos)\nSin Docker: enciende tu PostgreSQL y revisa DATABASE_URL en .env.');
}

// ── 3. Compilar la API (se ejecuta el binario para poder detenerlo limpio)
say('dev', 'Compilando la API de Go…');
// ruta absoluta y sin shell: en Windows, cmd.exe leería «Backend/go/...» como el comando «Backend» con la opción /go
const exe = path.resolve('Backend', 'go', 'bin', win ? 'server.exe' : 'server');
const build = spawnSync('go', ['-C', 'Backend/go', 'build', '-o', win ? 'bin/server.exe' : 'bin/server', './cmd/server'], { stdio: 'inherit' });
if (build.error?.code === 'ENOENT') die('No se encontró el comando «go»', 'Instala Go 1.26+ desde https://go.dev/dl/ y abre una terminal nueva.');
if (build.status !== 0) die('No se pudo compilar la API de Go', 'Revisa el error de arriba. Si dice que necesitas una versión más nueva de Go, instala Go 1.26+.');

// ── 4. Procesos
const children = [];
let stopping = false;

function stop(child) {
  if (child.exitCode !== null || child.killed) return;
  try {
    if (win) spawn('taskkill', ['/pid', String(child.pid), '/f', '/t'], { stdio: 'ignore' });
    else process.kill(-child.pid, 'SIGTERM'); // todo el grupo (npm deja procesos hijos)
  } catch {}
}
function shutdown(code) {
  if (stopping) return;
  stopping = true;
  children.forEach(stop);
  setTimeout(() => process.exit(code), 400);
}
process.on('SIGINT', () => shutdown(0));
process.on('SIGTERM', () => shutdown(0));

function run(tag, cmd, args, { critical = true, shell = false } = {}) {
  // con shell se pasa una sola cadena (evita el aviso DEP0190 y el escape de argumentos de Node)
  const child = (shell ? spawn(`${cmd} ${args.join(' ')}`, { env, shell: true, detached: !win, stdio: ['ignore', 'pipe', 'pipe'] }) : spawn(cmd, args, { env, detached: !win, stdio: ['ignore', 'pipe', 'pipe'] }));
  children.push(child);
  const pipe = (stream, out) => {
    let buf = '';
    stream.on('data', (d) => {
      buf += d;
      const lines = buf.split(/\r?\n/);
      buf = lines.pop();
      lines.forEach((l) => l.trim() && out(paint(tag, l)));
    });
  };
  pipe(child.stdout, console.log);
  pipe(child.stderr, console.error);
  child.on('exit', (code, signal) => {
    if (stopping) return;
    console.error(`\n\x1b[31m✖ El proceso «${tag}» terminó (${signal || 'código ' + code}).\x1b[0m`);
    if (tag === 'go') console.error('  Revisa los mensajes [go] de arriba: ahí está el motivo (base de datos, migraciones o configuración).');
    if (critical) shutdown(1);
  });
  return child;
}

const go = run('go', exe, []);

// ── 5. Esperar a que la API responda antes de abrir el frontend
const alive = async () => {
  try {
    return (await fetch(`http://127.0.0.1:${apiPort}/health`, { signal: AbortSignal.timeout(1500) })).ok;
  } catch {
    return false;
  }
};
const started = Date.now();
while (!stopping && !(await alive())) {
  if (Date.now() - started > 30000) die(`La API no respondió en 30 s en el puerto ${apiPort}`, 'Revisa los mensajes [go] de arriba. ¿Otro programa usa ese puerto?');
  await new Promise((r) => setTimeout(r, 300));
}
if (!stopping) {
  say('dev', `API lista en http://localhost:${apiPort}`);
  run('node', 'npm', ['--prefix', 'Backend/node', 'run', 'dev'], { critical: false, shell: true });
  run('front', 'npm', ['--prefix', 'Frontend', 'run', 'dev'], { shell: true });
}
