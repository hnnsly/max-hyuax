/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // В разработке API идёт через прокси, как за Caddy на сервере.
    proxy: { '/api': 'http://localhost:8080' },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
});
