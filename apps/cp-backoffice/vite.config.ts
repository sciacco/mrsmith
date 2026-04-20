import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const backend = process.env.VITE_DEV_BACKEND_URL || 'http://localhost:8080';

export default defineConfig(({ command }) => ({
  base: command === 'build' ? '/apps/cp-backoffice/' : '/',
  plugins: [react()],
  server: {
    port: 5187,
    proxy: {
      '/api': backend,
      '/config': backend,
    },
  },
}));
