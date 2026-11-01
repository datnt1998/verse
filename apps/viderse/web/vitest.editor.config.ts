import { defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config'

export default mergeConfig(viteConfig, defineConfig({
  test: {
    name: 'editor',
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/editor/engine/test/setup.ts'],
    include: ['src/editor/engine/**/*.test.{ts,tsx}', 'src/editor/ui/**/*.test.tsx', 'src/editor/ui/export/export-runner.test.ts', 'src/editor/ui/clone/quality-gate.test.ts', 'src/editor/ui/clone/accept-export.test.ts', 'src/editor/bridge/**/*.test.{ts,tsx}', 'src/editor/v13/**/*.test.{ts,tsx}', 'src/editor/pages/**/*.test.{ts,tsx}'],
    exclude: ['**/node_modules/**', 'src/editor/**/*.node.test.ts'],
    pool: 'threads',
    maxWorkers: 1,
    testTimeout: 20_000,
  },
}))
