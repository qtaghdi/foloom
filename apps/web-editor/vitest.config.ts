import { defineConfig } from 'vitest/config';
import solid from 'vite-plugin-solid';

export default defineConfig({
  plugins: [solid()],
  test: { setupFiles: ['./tests/setup.ts'], environment: 'happy-dom', include: ['tests/**/*.test.tsx'], restoreMocks: true },
});
