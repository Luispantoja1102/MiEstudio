import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  envDir: '..',
  plugins: [sveltekit()],
  server: {
    proxy: { '/api': `http://localhost:${process.env.GO_API_PORT || 8080}` }
  }
});
