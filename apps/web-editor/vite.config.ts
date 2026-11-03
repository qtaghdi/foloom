import solid from 'vite-plugin-solid';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [solid(), tailwindcss()],
  esbuild: {
    charset: 'ascii',
  },
  build: {
    emptyOutDir: true,
    rollupOptions: { input: ['index.html', 'docx.html'] },
    minify: 'terser',
    outDir: '../../internal/transport/httpapi/static',
    terserOptions: {
      format: {
        ascii_only: true,
      },
    },
  },
});
