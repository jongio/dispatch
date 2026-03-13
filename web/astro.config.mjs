import { defineConfig } from 'astro/config';

export default defineConfig({
  site: 'https://jongio.github.io',
  base: '/dispatch/',
  output: 'static',
  build: {
    assets: '_assets',
  },
});
