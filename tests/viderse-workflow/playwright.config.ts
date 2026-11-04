import { defineConfig } from '@playwright/test'

const output = process.env.WORKFLOW_EVIDENCE_DIR
if (!output) throw new Error('WORKFLOW_EVIDENCE_DIR is required; do not overwrite another PR evidence set')
const origin = process.env.WORKFLOW_APP_URL ?? 'http://127.0.0.1:8044'
const url = new URL(origin)
if (!['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) || url.username || url.password || url.pathname !== '/' || url.search || url.hash) throw new Error('Interaction audit requires a credential-free loopback origin')

export default defineConfig({
  testDir: '.',
  testMatch: ['interaction-audit.pw.ts', 'workflow-parity.pw.ts', 'viewer-audit.pw.ts', 'delete-audit.pw.ts', 'settings-state-audit.pw.ts', 'template-state-audit.pw.ts', 'provider-catalog-audit.pw.ts'],
  workers: 1,
  retries: 0,
  timeout: 90_000,
  expect: { timeout: 10_000 },
  outputDir: `${output}/browser`,
  reporter: [['list'], ['html', { outputFolder: `${output}/html`, open: 'never' }], ['json', { outputFile: `${output}/interaction-results.json` }]],
  use: {
    baseURL: origin,
    actionTimeout: 15_000,
    deviceScaleFactor: 1,
    screenshot: 'only-on-failure',
    // Traces contain authenticated HTTP headers. Keep motion video and explicit
    // nonsecret semantic/geometry attachments instead of committing credentials.
    trace: 'off',
    video: 'on',
    launchOptions: { executablePath: process.env.PW_BROWSER_BINARY, args: ['--force-color-profile=srgb'] },
  },
  projects: [
    { name: 'desktop', testIgnore: 'delete-audit.pw.ts', grepInvert: /Editor replacement/, use: { viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 } },
    { name: 'desktop-reduced', testIgnore: 'delete-audit.pw.ts', grepInvert: /Editor replacement/, use: { viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, reducedMotion: 'reduce' } },
    { name: 'mobile', testIgnore: 'delete-audit.pw.ts', grepInvert: /Editor replacement/, use: { viewport: { width: 390, height: 844 }, deviceScaleFactor: 1, isMobile: true, hasTouch: true } },
    { name: 'mobile-reduced', testIgnore: 'delete-audit.pw.ts', grepInvert: /Editor replacement/, use: { viewport: { width: 390, height: 844 }, deviceScaleFactor: 1, isMobile: true, hasTouch: true, reducedMotion: 'reduce' } },
    { name: 'delete-desktop', testMatch: 'delete-audit.pw.ts', dependencies: ['desktop', 'desktop-reduced', 'mobile', 'mobile-reduced'], use: { viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 } },
    { name: 'delete-desktop-reduced', testMatch: 'delete-audit.pw.ts', dependencies: ['delete-desktop'], use: { viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, reducedMotion: 'reduce' } },
    { name: 'post-delete-desktop', testMatch: 'viewer-audit.pw.ts', grep: /Editor replacement/, dependencies: ['delete-desktop-reduced'], use: { viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 } },
    { name: 'post-delete-desktop-reduced', testMatch: 'viewer-audit.pw.ts', grep: /Editor replacement/, dependencies: ['delete-desktop-reduced'], use: { viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, reducedMotion: 'reduce' } },
  ],
})
