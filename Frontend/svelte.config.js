import adapter from '@sveltejs/adapter-static';

// Cada compilación lleva un nombre de versión. La app consulta /_app/version.json y, si cambió, avisa al usuario
// (ver UpdateBanner.svelte); nunca recarga sola. APP_VERSION permite fijarlo desde CI (por ejemplo, el commit).
// El nombre se fija una sola vez por compilación (svelte.config se evalúa varias veces durante el build y
// todas deben coincidir, o el HTML y los scripts quedarían con versiones distintas).
process.env.APP_VERSION ||= String(Date.now());

export default {
  kit: {
    adapter: adapter({ fallback: 'index.html' }),
    version: { name: process.env.APP_VERSION, pollInterval: 60_000 }
  }
};
