import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const backendTarget = process.env.VITE_DEV_BACKEND_URL || 'http://localhost:8080';

export default defineConfig(({ command }) => ({
  base: command === 'build' ? '/apps/training/' : '/',
  plugins: [react()],
  server: {
    port: 5191,
    proxy: {
      '/api': backendTarget,
      '/config': backendTarget,
    },
  },
}));
