import { defineConfig } from '@playwright/test'

// Native desktop harness (no route interception, no style injection).
// One worker + one browser context; reduced motion is emulated on the same page.
// Auth is whatever the operator already holds: pass PW_STATE=<storageState.json>
// produced from the owned QA cookies. Without it only the canonical boards run.
const evidence = process.env.PC_EVIDENCE_DIR ?? 'test-results/pc-uikit'

export default defineConfig({
  testDir: '.',
  testMatch: 'pc-uikit.pw.ts',
  workers: 1,
  retries: 0,
  timeout: 120_000,
  expect: { timeout: 10_000 },
  outputDir: evidence,
  reporter: [['list'], ['json', { outputFile: `${evidence}/results.json` }]],
  use: {
    viewport: { width: Number(process.env.PC_VIEWPORT ?? 1440), height: 1000 },
    baseURL: process.env.PC_APP_URL ?? 'http://127.0.0.1:8060',
    storageState: process.env.PW_STATE || undefined,
    screenshot: 'off',
    trace: 'off',
    video: 'off',
    launchOptions: { args: ['--force-color-profile=srgb'] },
  },
})
