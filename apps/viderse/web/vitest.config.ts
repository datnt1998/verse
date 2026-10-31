import { defineConfig } from 'vitest/config'

// Two vitest projects: the app (happy-dom) and the editor engine/UI (jsdom +
// editor setup). Editor files written with node:test run via `npm run test:node`.
export default defineConfig({
  test: {
    pool: 'threads',
    maxWorkers: 2,
    projects: [
      {
        extends: './vite.config.ts',
        test: {
          name: 'app',
          environment: 'happy-dom',
          include: ['src/**/*.test.{ts,tsx}'],
          exclude: ['src/editor/**', '**/node_modules/**'],
        },
      },
      './vitest.editor.config.ts',
    ],
  },
})
