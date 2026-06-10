import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import electron from 'vite-plugin-electron';
import { resolve } from 'path';

export default defineConfig({
  plugins: [
    react({
      include: ['src/renderer/**/*.{tsx,jsx}'],
    }),
    tailwindcss(),
    electron([
      {
        entry: resolve(__dirname, 'src/main/index.ts'),
        vite: {
          build: {
            outDir: resolve(__dirname, 'dist/main'),
            rolldownOptions: {
              external: ['electron', 'better-sqlite3', 'chokidar'],
            },
          },
        },
      },
      {
        entry: resolve(__dirname, 'src/preload/index.ts'),
        onstart(args) {
          args.reload();
        },
        vite: {
          build: {
            outDir: resolve(__dirname, 'dist/preload'),
          },
        },
      },
    ]),
  ],
  root: resolve(__dirname),
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src/renderer'),
    },
  },
  build: {
    outDir: resolve(__dirname, 'dist/renderer'),
  },
  test: {
    root: resolve(__dirname),
    include: ['tests/**/*.{test,spec}.ts'],
  },
});
