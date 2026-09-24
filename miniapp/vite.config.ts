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
    // Порог для чистой логики: форматирование, модели отображения, стек экранов.
    coverage: {
      provider: 'v8',
      include: ['src/shared/lib/**/*.ts', 'src/app/stack.ts'],
      exclude: ['**/*.test.ts'],
      thresholds: { lines: 90, functions: 90, branches: 85, statements: 90 },
    },
  },
});
