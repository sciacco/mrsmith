import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const backendTarget = process.env.VITE_DEV_BACKEND_URL || 'http://localhost:8080';

export default defineConfig(({ command }) => ({
  base: command === 'build' ? '/apps/quotes/' : '/',
  plugins: [react()],
  server: {
    port: 5179,
    proxy: {
      '/api': backendTarget,
      '/config': backendTarget,
    },
  },
}));
