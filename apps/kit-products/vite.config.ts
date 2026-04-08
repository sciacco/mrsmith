import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(({ command }) => ({
  base: command === 'build' ? '/apps/kit-products/' : '/',
  plugins: [react()],
  server: {
    port: 5176,
    proxy: {
      '/api': 'http://localhost:8080',
      '/config': 'http://localhost:8080',
    },
  },
}));
