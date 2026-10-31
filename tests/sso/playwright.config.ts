import { defineConfig } from '@playwright/test';

// One worker: the suite provisions exactly one throwaway stack (databases,
// roles, four server processes, TLS proxy) and every scenario runs against it
// serially — the tests deliberately mutate shared session/grant state.
export default defineConfig({
  testDir: '.',
  testMatch: 'sso.spec.ts',
  workers: 1,
  timeout: 180_000,
  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',
  // No traces, no videos, no screenshots: auth flows carry session material.
  // Failures print redacted server logs via the test itself.
  use: {
    // The hosted rehearsal terminates TLS at tests/sso/fixture with a
    // throwaway CA that is never installed anywhere; the browser ignores the
    // unknown issuer instead of widening any trust store.
    ignoreHTTPSErrors: true,
    trace: 'off',
    video: 'off',
    screenshot: 'off',
    launchOptions: {
      args: ['--host-resolver-rules=MAP *.verse.test 127.0.0.1, MAP *.other.test 127.0.0.1'],
    },
  },
  reporter: [['list']],
});
