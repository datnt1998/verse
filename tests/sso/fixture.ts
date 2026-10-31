// Shared fixtures for the SSO smoke suite.
//
// `stack` exposes the provisioned run state (URLs, users, DSNs, binaries) that
// global-setup.ts built. Tests drive the real servers through
// `context.request` (the browser context's own cookie jar — the same jar the
// pages use) or through standalone `request.newContext()` jars for extra
// devices/admin actors.
import { test as base, expect, type APIRequestContext, type APIResponse, type BrowserContext, type Page } from '@playwright/test';
import {
  loadState, run, startServer, waitReady, serverEnv, noverseDataEnv, viderseCliEnv,
  freePort, provisionRaceDB, psql, psqlAs, type Stack, type ServerProc,
} from './harness';

export { expect };

// Wire shapes of the shared account contract (platform/identity).
export interface UserView {
  username?: string;
  name?: string;
  role?: string;
  grants?: Record<string, string> | null;
  projects?: Record<string, string>;
  disabled?: boolean;
  mustChangePassword?: boolean;
}
export interface AppLink { id: 'noverse' | 'viderse'; name: string; origin: string }
export interface MeBody { user?: UserView; session?: { expiresAt: string }; apps: AppLink[] }
export interface GateBody {
  code?: string;
  app?: string;
  error?: string;
  user?: UserView;
  apps?: AppLink[];
  reason?: 'logout' | 'password_changed' | 'admin_changed' | 'locked' | 'expired' | 'unknown';
  mustChangePassword?: boolean;
}
export interface ApiResult {
  status: number;
  /** parsed JSON body, or raw text when the response is not JSON */
  body: unknown;
  /** raw Set-Cookie header values — used only to assert attributes, never logged */
  setCookies: string[];
}

export interface SsoFixture {
  stack: Stack;
  /** fresh API jar, not shared with the browser */
  api: APIRequestContext;
  /** login through the real HTTP form endpoint into a request jar */
  apiLogin: (jar: APIRequestContext, baseURL: string, username: string, password: string) => Promise<ApiResult>;
  /** login through the real browser form on a page of `context` */
  uiLogin: (page: Page, baseURL: string, username: string, password: string) => Promise<void>;
  /** `serve user grant|revoke` and `viderse user grant|revoke` on the real CLIs */
  grantCli: (app: 'noverse' | 'viderse', verb: 'grant' | 'revoke', username: string, target: 'noverse' | 'viderse') => Promise<string>;
  /** start an extra server instance (tracked for teardown) */
  startExtra: (app: 'noverse' | 'viderse', opts?: { hosted?: boolean; identityDsn?: string; dataDsn?: string }) => Promise<ServerProc>;
  /** query a DSN as that runtime role; returns trimmed scalar output */
  sql: (dsn: string, query: string) => Promise<string>;
  /** psql on the throwaway admin connection (superuser `dat`) */
  sqlAdmin: (db: string, query: string) => Promise<string>;
}

export const test = base.extend<SsoFixture>({
  stack: async ({}, use) => {
    await use(loadState());
  },
  api: async ({ playwright }, use) => {
    const jar = await playwright.request.newContext({ ignoreHTTPSErrors: true });
    await use(jar);
    await jar.dispose();
  },
  apiLogin: async ({}, use) => {
    await use(async (jar, baseURL, username, password) => {
      const res = await jar.post(`${baseURL}/api/login`, {
        data: { username, password },
        headers: { 'Content-Type': 'application/json' },
      });
      const setCookies = res.headersArray().filter(h => h.name.toLowerCase() === 'set-cookie').map(h => h.value);
      return { status: res.status(), body: await bodyOf(res), setCookies };
    });
  },
  uiLogin: async ({}, use) => {
    await use(async (page, baseURL, username, password) => {
      await page.goto(baseURL + '/');
      await page.locator('#lg-user').fill(username);
      await page.locator('#lg-pass').fill(password);
      const done = page.waitForResponse(res => res.url().endsWith('/api/login'), { timeout: 30_000 });
      await page.locator('button[type="submit"]').first().click();
      expect((await done).status()).toBe(200);
      await expect(page.locator('#lg-user')).toHaveCount(0);
    });
  },
  grantCli: async ({ stack }, use) => {
    await use(async (app, verb, username, target) => {
      if (app === 'noverse') {
        return run(stack.binNoverse, ['serve', 'user', verb, username, '--app', target], { env: noverseDataEnv(stack) });
      }
      return run(stack.binViderse, ['user', verb, username, '--app', target], { env: viderseCliEnv(stack) });
    });
  },
  startExtra: async ({ stack }, use) => {
    await use(async (app, opts = {}) => {
      const port = await freePort();
      const bin = app === 'noverse' ? stack.binNoverse : stack.binViderse;
      const srv = startServer(`${app}-x${port}`, bin, ['serve'], serverEnv(stack, app, port, opts));
      srv.base = opts?.hosted
        ? (app === 'noverse' ? stack.urls.hostedNoverse : stack.urls.hostedViderse)
        : `http://127.0.0.1:${port}`;
      stack.extraServers.push(srv);
      await waitReady(srv);
      return srv;
    });
  },
  sql: async ({}, use) => {
    await use((dsn, query) => psqlAs(dsn, query));
  },
  sqlAdmin: async ({}, use) => {
    await use((db, query) => psql(db, query).then(out => out.trim()));
  },
});

// ── shared assertions ──────────────────────────────────────────────────────

export async function bodyOf(res: APIResponse): Promise<unknown> {
  try { return await res.json(); } catch { return await res.text(); }
}

export async function me(jar: APIRequestContext, baseURL: string): Promise<ApiResult> {
  const res = await jar.get(`${baseURL}/api/me`);
  const setCookies = res.headersArray().filter(h => h.name.toLowerCase() === 'set-cookie').map(h => h.value);
  return { status: res.status(), body: await bodyOf(res), setCookies };
}

export async function sessionCookie(context: BrowserContext, url: string) {
  const cookies = await context.cookies([url]);
  return cookies.find(c => c.name === 'verse_session') ?? null;
}

export const NOVERSE = (s: Stack) => s.urls.noverse;
export const VIDERSE = (s: Stack) => s.urls.viderse;
