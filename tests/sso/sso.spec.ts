// Mandatory two-app SSO smoke table. Every scenario runs against the real
// ainovel-cli + viderse binaries on the real runtime roles (noverse_app,
// viderse_app, viderse_identity, noverse_engine_worker) provisioned by
// global-setup.ts. No mocks, no auth bypasses.
import { test, expect, me, sessionCookie, bodyOf, type ApiResult, type GateBody, type MeBody, type UserView } from './fixture';
import {
  psql, psqlAs, provisionRaceDB, startServer, waitReady, serverEnv, killGroup,
  freePort, run, runExit, existsSync, type ServerProc,
} from './harness';
import type { APIRequestContext, APIResponse } from '@playwright/test';
import { chmodSync, mkdirSync, readFileSync, statSync } from 'node:fs';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import path from 'node:path';

const CSRF_403 = 'yêu cầu khác origin bị từ chối';

async function req(jar: APIRequestContext, method: 'post' | 'put' | 'patch' | 'delete' | 'get', base: string, path: string, body?: unknown, headers: Record<string, string> = {}): Promise<ApiResult> {
  const res = await jar.fetch(`${base}${path}`, {
    method: method.toUpperCase(),
    headers: { 'Content-Type': 'application/json', ...headers },
    data: body === undefined ? undefined : body,
  });
  const setCookies = res.headersArray().filter(h => h.name.toLowerCase() === 'set-cookie').map(h => h.value);
  return { status: res.status(), body: await bodyOf(res), setCookies };
}

const post = (jar: APIRequestContext, base: string, path: string, body: unknown = {}, headers?: Record<string, string>) => req(jar, 'post', base, path, body, headers);
const get = (jar: APIRequestContext, base: string, path: string) => req(jar, 'get', base, path);

function deniedBody(body: unknown): GateBody {
  const b = body as GateBody; // server wire contract: {code,app,error,user}
  expect(b?.code).toBe('app_access_denied');
  expect(b?.error).toContain('chưa được cấp quyền');
  return b;
}

function forcedBody(body: unknown): GateBody {
  const b = body as GateBody; // server wire contract: {code,mustChangePassword,error}
  expect(b?.code).toBe('password_change_required');
  expect(b?.mustChangePassword).toBe(true);
  return b;
}

function meBody(body: unknown): MeBody {
  return body as MeBody; // server wire contract: {user, session?}
}

function usersBody(body: unknown): { users: { username?: string }[] } {
  const b = body as { users?: { username?: string }[] };
  return { users: b?.users ?? [] };
}

interface SessionRow { id?: string; current?: boolean }
async function mySessions(jar: APIRequestContext, base: string): Promise<SessionRow[]> {
  const r = await get(jar, base, '/api/me/sessions');
  expect(r.status).toBe(200);
  const b = r.body as { sessions?: SessionRow[] };
  return b?.sessions ?? [];
}

async function sleep(ms: number): Promise<void> {
  const { promise, resolve } = Promise.withResolvers<void>();
  setTimeout(resolve, ms);
  return promise;
}


test('SSOLocalCookiePortScope', async ({ stack, context, uiLogin }) => {
  const noversePage = await context.newPage();
  await uiLogin(noversePage, stack.urls.noverse, 'both', stack.users['both'].password);

  const jar = context.request;
  const nMe = meBody((await me(jar, stack.urls.noverse)).body);
  expect(nMe.user?.username).toBe('both');
  expect(nMe.apps.map(app => app.id)).toEqual(['noverse', 'viderse']);
  expect(nMe.apps.find(app => app.id === 'viderse')?.origin).toBe(stack.urls.viderse);
  expect(Date.parse(nMe.session!.expiresAt)).toBeGreaterThan(Date.now());

  // the jar holds exactly one host-only verse_session for 127.0.0.1
  const cookie = await sessionCookie(context, stack.urls.noverse);
  expect(cookie).not.toBeNull();
  expect(cookie!.domain).toBe('127.0.0.1');
  expect(cookie!.domain.startsWith('.')).toBe(false);
  expect(cookie!.path).toBe('/');
  expect(cookie!.sameSite).toBe('Strict');
  expect(cookie!.httpOnly).toBe(true);
  expect(cookie!.secure).toBe(false);
  const all = await context.cookies();
  expect(all.filter(c => c.name === 'verse_session')).toHaveLength(1);
  expect(all.filter(c => c.name === 'ainovel_session' || c.name === 'viderse_session')).toHaveLength(0);

  // second page, sibling port, same context: no second login form
  const vidersePage = await context.newPage();
  const booted = vidersePage.waitForResponse(r => r.url().endsWith('/api/me') && r.status() === 200, { timeout: 30_000 });
  await vidersePage.goto(stack.urls.viderse + '/');
  await booted;
  await expect(vidersePage.locator('#lg-user')).toHaveCount(0);
  const vMe = meBody((await me(jar, stack.urls.viderse)).body);
  expect(vMe.user?.username).toBe('both');
  expect(vMe.user?.grants?.viderse).toBe('member');

  // one business GET per app through the same jar
  expect((await get(jar, stack.urls.noverse, '/api/library')).status).toBe(200);
  expect((await get(jar, stack.urls.viderse, '/api/status')).status).toBe(200);
  expect((await context.cookies()).filter(c => c.name === 'verse_session')).toHaveLength(1);
});

test('SSOReverseLogin', async ({ stack, context, uiLogin }) => {
  const jar = context.request;
  const vidersePage = await context.newPage();
  await uiLogin(vidersePage, stack.urls.viderse, 'both', stack.users['both'].password);
  const noversePage = await context.newPage();
  const ready = noversePage.waitForResponse(r => r.url().endsWith('/api/me') && r.status() === 200);
  await noversePage.goto(stack.urls.noverse);
  await ready;
  await expect(noversePage.locator('#lg-user')).toHaveCount(0);

  const vMe = meBody((await me(jar, stack.urls.viderse)).body);
  const nMe = meBody((await me(jar, stack.urls.noverse)).body);
  expect(nMe.user?.username).toBe('both');
  expect(nMe.user?.grants).toEqual(vMe.user?.grants);

  // project membership governs the Noverse list; Viderse has no project surface
  const lib = await get(jar, stack.urls.noverse, '/api/library');
  expect(lib.status).toBe(200);
  const lb = lib.body as { projects?: { id?: string }[] };
  expect((lb.projects ?? []).map(p => p.id)).toContain('e2eproj');
  expect((await get(jar, stack.urls.noverse, '/api/projects/e2eproj/stories')).status).toBe(200);
  expect((await get(jar, stack.urls.viderse, '/api/status')).status).toBe(200);
});

test('SSOLocalHostBoundary', async ({ stack, playwright, apiLogin }) => {
  const jar = await playwright.request.newContext();
  expect((await apiLogin(jar, stack.urls.noverse, 'both', stack.users['both'].password)).status).toBe(200);
  expect((await me(jar, stack.urls.noverse)).status).toBe(200);

  // "localhost" is a different host than 127.0.0.1: the cookie does not travel.
  const port = new URL(stack.urls.noverse).port;
  expect((await me(jar, `http://localhost:${port}`)).status).toBe(401);
  const state = await jar.storageState();
  const session = state.cookies.filter(c => c.name === 'verse_session');
  expect(session).toHaveLength(1);
  expect(session[0].domain).toBe('127.0.0.1');
  expect(session[0].domain.startsWith('.')).toBe(false); // never Domain=127.0.0.1
  await jar.dispose();
});

test('SSOHostedCookieNavigation', async ({ stack, context, uiLogin }) => {
  const loginPage = await context.newPage();
  await uiLogin(loginPage, stack.urls.hostedNoverse, 'both', stack.users['both'].password);

  const cookie = await sessionCookie(context, stack.urls.hostedNoverse);
  expect(cookie).not.toBeNull();
  expect(cookie!.domain).toBe('.verse.test');
  expect(cookie!.path).toBe('/');
  expect(cookie!.sameSite).toBe('Strict');
  expect(cookie!.httpOnly).toBe(true);
  expect(cookie!.secure).toBe(true);

  // navigate the sibling app on the same site: shared cookie opens it
  const page = await context.newPage();
  const booted = page.waitForResponse(r => r.url().endsWith('/api/me') && r.status() === 200, { timeout: 30_000 });
  await page.goto(stack.urls.hostedViderse + '/');
  await booted;
  await expect(page.locator('#lg-user')).toHaveCount(0);
  expect(await page.evaluate(async () => (await fetch('/api/me')).status)).toBe(200);

  // a different parent domain is a different site: no cookie travels → 401
  const proxyPort = new URL(stack.urls.hostedNoverse).port;
  const other = await context.newPage();
  const response = await other.goto(`https://khac.other.test:${proxyPort}/api/me`);
  expect(response?.status()).toBe(401);

  // A direct, spoofed loopback login cannot downgrade the cookie flags.
  const plain = await context.request.post(`${stack.urls.hostedNoverseLoopback}/api/login`, {
    data: { username: 'both', password: stack.users['both'].password },
    headers: { 'X-Forwarded-Proto': 'http' },
  });
  expect(plain.status()).toBe(200);
  const values = plain.headersArray().filter(h => h.name.toLowerCase() === 'set-cookie');
  const sessionSet = values.find(h => h.value.startsWith('verse_session='))?.value ?? '';
  // Assert booleans only: never expose session material in a failing matcher.
  expect(sessionSet.includes('Secure')).toBe(true);
  expect(sessionSet.includes('Domain=verse.test')).toBe(true);
  expect(sessionSet.includes('SameSite=Strict')).toBe(true);
  expect(sessionSet.includes('HttpOnly')).toBe(true);
});

test('SSONoverseOnlyMember', async ({ stack, playwright, apiLogin }) => {
  const jar = await playwright.request.newContext();
  expect((await apiLogin(jar, stack.urls.noverse, 'novonly', stack.users['novonly'].password)).status).toBe(200);

  const vMe = await me(jar, stack.urls.viderse);
  expect(vMe.status).toBe(403);
  const denied = deniedBody(vMe.body);
  expect(denied.app).toBe('viderse');
  expect(denied.user?.username).toBe('novonly');
  expect(denied.user?.grants?.noverse).toBe('member');
  expect(denied.user?.grants?.viderse).toBeUndefined();
  expect(denied.apps).toEqual([{ id: 'noverse', name: 'Noverse', origin: stack.urls.noverse }]);

  // the cookie stays: the session is still valid on its granted app
  expect((await me(jar, stack.urls.noverse)).status).toBe(200);
  const biz = await get(jar, stack.urls.viderse, '/api/status');
  expect(biz.status).toBe(403);
  deniedBody(biz.body);
  // auth-neutral routes still answer on the denied app
  expect((await mySessions(jar, stack.urls.viderse)).length).toBeGreaterThan(0);
  // the SSE business route is denied, never a silent stream
  const ev = await jar.get(`${stack.urls.viderse}/api/events`, { timeout: 10_000 });
  expect(ev.status()).toBe(403);
  await jar.dispose();
});

test('SSOViderseOnlyMember', async ({ stack, playwright, apiLogin }) => {
  const jar = await playwright.request.newContext();
  // correct credentials on the denied app: 403 (not 401), cookie still set
  const login = await apiLogin(jar, stack.urls.noverse, 'vidonly', stack.users['vidonly'].password);
  expect(login.status).toBe(403);
  expect(deniedBody(login.body).app).toBe('noverse');

  const nMe = await me(jar, stack.urls.noverse);
  expect(nMe.status).toBe(403);
  deniedBody(nMe.body);
  expect(deniedBody(nMe.body).apps).toEqual([{ id: 'viderse', name: 'Viderse', origin: stack.urls.viderse }]);
  const lib = await get(jar, stack.urls.noverse, '/api/library');
  expect(lib.status).toBe(403);
  deniedBody(lib.body); // grant gate wins over project membership

  expect((await me(jar, stack.urls.viderse)).status).toBe(200);
  expect((await get(jar, stack.urls.viderse, '/api/status')).status).toBe(200);
  await jar.dispose();
});

test('SSOGlobalAdminAndGrantMutation', async ({ stack, playwright, apiLogin, grantCli }) => {
  const admin = await playwright.request.newContext();
  expect((await apiLogin(admin, stack.urls.noverse, 'boss', stack.users['admin'].password)).status).toBe(200);
  // global admin carries no grant ROWS yet reaches both apps; the effective
  // grants view exposes the admin role per app instead
  const adminMe = meBody((await me(admin, stack.urls.noverse)).body);
  expect(adminMe.user?.role).toBe('admin');
  expect(adminMe.user?.grants?.noverse).toBe('admin');
  expect(adminMe.user?.grants?.viderse).toBe('admin');
  expect((await psql(stack.identityDB, `SELECT count(*) FROM app_grants g JOIN users u ON u.id=g.user_id WHERE u.username='boss'`)).trim()).toBe('0');
  expect((await me(admin, stack.urls.viderse)).status).toBe(200);

  // a member cannot mutate grants over HTTP
  const member = await playwright.request.newContext();
  expect((await apiLogin(member, stack.urls.noverse, 'both', stack.users['both'].password)).status).toBe(200);
  expect((await req(member, 'put', stack.urls.noverse, '/api/users/muttarget/grants/viderse', { role: 'member' })).status).toBe(403);

  // sessions alive before the mutation must die when the grant changes
  const target = await playwright.request.newContext();
  expect((await apiLogin(target, stack.urls.noverse, 'muttarget', stack.users['muttarget'].password)).status).toBe(200);
  const targetV = await playwright.request.newContext();
  expect((await apiLogin(targetV, stack.urls.viderse, 'muttarget', stack.users['muttarget'].password)).status).toBe(403); // denied login still creates a session

  const grant = await req(admin, 'put', stack.urls.noverse, '/api/users/muttarget/grants/viderse', { role: 'member' });
  expect(grant.status).toBe(200);
  expect((await me(target, stack.urls.noverse)).status).toBe(401);
  const grantEnded = await me(targetV, stack.urls.viderse);
  expect(grantEnded.status).toBe(401);
  expect(grantEnded.body).toMatchObject({ code: 'session_ended', reason: 'admin_changed', app: 'noverse' });
  expect((await apiLogin(targetV, stack.urls.viderse, 'muttarget', stack.users['muttarget'].password)).status).toBe(200);

  // CLI grant (already granted → no audit, no revocation)
  expect(await grantCli('noverse', 'grant', 'muttarget', 'viderse')).toContain('Đã cấp');
  expect((await me(targetV, stack.urls.viderse)).status).toBe(200);

  // revoke via the Viderse CLI → sessions die again, fresh login is denied
  await grantCli('viderse', 'revoke', 'muttarget', 'viderse');
  expect((await me(targetV, stack.urls.viderse)).status).toBe(401);
  const reDenied = await apiLogin(targetV, stack.urls.viderse, 'muttarget', stack.users['muttarget'].password);
  expect(reDenied.status).toBe(403);
  deniedBody(reDenied.body);

  // audit rows exist for the real mutations; membership never moved
  const audit = await get(admin, stack.urls.noverse, '/api/admin/audit?type=perm&limit=50');
  expect(audit.status).toBe(200);
  const ab = audit.body as { items?: { action?: string; target?: string; actor?: { username?: string } | null }[] };
  const rows = (ab.items ?? []).filter(i => i.target === 'muttarget' && (i.action ?? '').includes('quyền ứng dụng'));
  expect(rows.length).toBe(2); // HTTP grant + CLI revoke; idempotent grant wrote none
  expect(rows.some(i => i.actor?.username === 'boss')).toBe(true);
  expect(rows.some(i => i.actor === null)).toBe(true);
  const mut = meBody((await get(admin, stack.urls.noverse, '/api/users/muttarget')).body);
  expect(mut.user?.projects ?? {}).toEqual({});
  await admin.dispose(); await member.dispose(); await target.dispose(); await targetV.dispose();
});

test('SSORevocationAcrossBothApps', async ({ stack, playwright, apiLogin }) => {
  // revoke the CURRENT session from Noverse → dead on both apps
  const a = await playwright.request.newContext();
  expect((await apiLogin(a, stack.urls.noverse, 'revtarget', stack.users['revtarget'].password)).status).toBe(200);
  const current = (await mySessions(a, stack.urls.noverse)).find(s => s.current);
  expect(current?.id).toBeTruthy();
  expect((await post(a, stack.urls.noverse, `/api/sessions/${current!.id}/revoke`)).status).toBe(200);
  expect((await me(a, stack.urls.noverse)).status).toBe(401);
  expect((await me(a, stack.urls.viderse)).status).toBe(401);
  await a.dispose();

  // repeat from Viderse (denied login still holds a live session)
  const b = await playwright.request.newContext();
  expect((await apiLogin(b, stack.urls.viderse, 'revtarget', stack.users['revtarget'].password)).status).toBe(403);
  const currentV = (await mySessions(b, stack.urls.viderse)).find(s => s.current);
  expect(currentV?.id).toBeTruthy();
  expect((await post(b, stack.urls.viderse, `/api/sessions/${currentV!.id}/revoke`)).status).toBe(200);
  expect((await me(b, stack.urls.viderse)).status).toBe(401);
  expect((await me(b, stack.urls.noverse)).status).toBe(401);
  await b.dispose();
});

test('SSOLogoutEverywhere', async ({ stack, playwright, apiLogin }) => {
  // logout current session from Viderse → that session dies on both apps
  const one = await playwright.request.newContext();
  expect((await apiLogin(one, stack.urls.noverse, 'revtarget', stack.users['revtarget'].password)).status).toBe(200);
  const observer = await playwright.request.newContext({ storageState: await one.storageState() });
  expect((await post(one, stack.urls.viderse, '/api/logout')).status).toBe(204);
  const endedLogout = await me(observer, stack.urls.noverse);
  expect(endedLogout.status).toBe(401);
  expect(endedLogout.body).toMatchObject({ code: 'session_ended', reason: 'logout', app: 'viderse' });
  await observer.dispose();
  expect((await me(one, stack.urls.noverse)).status).toBe(401);
  expect((await me(one, stack.urls.viderse)).status).toBe(401);

  // Two devices: signing out other devices from Noverse preserves d1.
  const d1 = await playwright.request.newContext();
  const d2 = await playwright.request.newContext();
  expect((await apiLogin(d1, stack.urls.noverse, 'revtarget', stack.users['revtarget'].password)).status).toBe(200);
  expect((await apiLogin(d2, stack.urls.viderse, 'revtarget', stack.users['revtarget'].password)).status).toBe(403);
  const kill = await post(d1, stack.urls.noverse, '/api/me/sessions/revoke-all');
  expect(kill.status).toBe(200);
  expect(kill.body).not.toHaveProperty('reauthenticate');
  expect((await me(d1, stack.urls.noverse)).status).toBe(200);
  expect((await me(d1, stack.urls.viderse)).status).toBe(403);
  const endedD2 = await me(d2, stack.urls.viderse);
  expect(endedD2.status).toBe(401);
  expect(endedD2.body).toMatchObject({ code: 'session_ended', reason: 'logout', app: 'noverse' });

  // and again initiated from Viderse
  const e1 = await playwright.request.newContext();
  const e2 = await playwright.request.newContext();
  expect((await apiLogin(e1, stack.urls.noverse, 'revtarget', stack.users['revtarget'].password)).status).toBe(200);
  expect((await apiLogin(e2, stack.urls.viderse, 'revtarget', stack.users['revtarget'].password)).status).toBe(403);
  expect((await post(e2, stack.urls.viderse, '/api/me/sessions/revoke-all')).status).toBe(200);
  expect((await me(e2, stack.urls.viderse)).status).toBe(403);
  expect((await me(e2, stack.urls.noverse)).status).toBe(200);
  const endedE1 = await me(e1, stack.urls.noverse);
  expect(endedE1.status).toBe(401);
  expect(endedE1.body).toMatchObject({ code: 'session_ended', reason: 'logout', app: 'viderse' });
  await one.dispose(); await d1.dispose(); await d2.dispose(); await e1.dispose(); await e2.dispose();
});

test('SSOPasswordChangeRevokesAll', async ({ stack, playwright, apiLogin }) => {
  const pw = stack.users['both'].password;
  const next = `nv-${Math.random().toString(36).slice(2, 10)}`;

  const dev1 = await playwright.request.newContext();
  const dev2 = await playwright.request.newContext();
  expect((await apiLogin(dev1, stack.urls.noverse, 'both', pw)).status).toBe(200);
  expect((await apiLogin(dev2, stack.urls.viderse, 'both', pw)).status).toBe(200);
  const ch = await post(dev1, stack.urls.noverse, '/api/me/password', { current: pw, new: next });
  expect(ch.status).toBe(200);
  expect((ch.body as { reauthenticate?: boolean })?.reauthenticate).toBe(true);
  const endedPassword = await me(dev2, stack.urls.viderse);
  expect(endedPassword.status).toBe(401);
  expect(endedPassword.body).toMatchObject({ code: 'session_ended', reason: 'password_changed', app: 'noverse' });
  for (const jar of [dev1, dev2]) {
    expect((await me(jar, stack.urls.noverse)).status).toBe(401);
    expect((await me(jar, stack.urls.viderse)).status).toBe(401);
  }
  expect((await apiLogin(dev2, stack.urls.noverse, 'both', pw)).status).toBe(401); // old password is a generic fail
  expect((await apiLogin(dev1, stack.urls.viderse, 'both', next)).status).toBe(200);
  stack.users['both'].password = next;

  // change initiated from Viderse kills every session too
  const newer = `${next}-b`;
  const dev3 = await playwright.request.newContext();
  expect((await apiLogin(dev3, stack.urls.noverse, 'both', next)).status).toBe(200);
  expect((await post(dev1, stack.urls.viderse, '/api/me/password', { current: next, new: newer })).status).toBe(200);
  for (const jar of [dev1, dev3]) {
    expect((await me(jar, stack.urls.noverse)).status).toBe(401);
    expect((await me(jar, stack.urls.viderse)).status).toBe(401);
  }
  expect((await apiLogin(dev3, stack.urls.noverse, 'both', newer)).status).toBe(200);
  stack.users['both'].password = newer;
  await dev1.dispose(); await dev2.dispose(); await dev3.dispose();
});

test('SSOAdminResetAndDisable', async ({ stack, playwright, apiLogin }) => {
  const admin = await playwright.request.newContext();
  expect((await apiLogin(admin, stack.urls.noverse, 'boss', stack.users['admin'].password)).status).toBe(200);
  const victim = await playwright.request.newContext();
  expect((await apiLogin(victim, stack.urls.noverse, 'resetme', stack.users['resetme'].password)).status).toBe(200);

  // admin reset → all sessions die on both apps; temp password forces a change
  expect((await post(admin, stack.urls.noverse, '/api/users/resetme', { password: 'reset-tmp-pass-9' })).status).toBe(200);
  const resetEnded = await me(victim, stack.urls.viderse);
  expect(resetEnded.status).toBe(401);
  expect(resetEnded.body).toMatchObject({ code: 'session_ended', reason: 'admin_changed', app: 'noverse' });
  expect((await me(victim, stack.urls.noverse)).status).toBe(401);
  expect((await apiLogin(victim, stack.urls.noverse, 'resetme', 'reset-tmp-pass-9')).status).toBe(200);
  expect(meBody((await me(victim, stack.urls.noverse)).body).user?.mustChangePassword).toBe(true);
  forcedBody((await get(victim, stack.urls.viderse, '/api/status')).body);
  expect((await post(victim, stack.urls.noverse, '/api/me/password', { current: 'reset-tmp-pass-9', new: 'resetme-final-pw' })).status).toBe(200);
  expect((await apiLogin(victim, stack.urls.noverse, 'resetme', 'resetme-final-pw')).status).toBe(200);

  // disable → sessions die and stay dead; re-enable never resurrects them
  expect((await post(admin, stack.urls.noverse, '/api/users/resetme', { disabled: true, reason: 'kiểm thử' })).status).toBe(200);
  const lockedEnded = await me(victim, stack.urls.viderse);
  expect(lockedEnded.status).toBe(401);
  expect(lockedEnded.body).toMatchObject({ code: 'session_ended', reason: 'locked', app: 'noverse' });
  expect((await me(victim, stack.urls.noverse)).status).toBe(401);
  expect((await apiLogin(victim, stack.urls.noverse, 'resetme', 'resetme-final-pw')).status).toBe(401);
  expect((await post(admin, stack.urls.noverse, '/api/users/resetme', { disabled: false })).status).toBe(200);
  expect((await me(victim, stack.urls.noverse)).status).toBe(401);
  expect((await apiLogin(victim, stack.urls.noverse, 'resetme', 'resetme-final-pw')).status).toBe(200);
  await admin.dispose(); await victim.dispose();
});

test('SSOForcedPasswordBothApps', async ({ stack, playwright, apiLogin }) => {
  const temp = stack.users['forced'].password;
  const jar = await playwright.request.newContext();
  expect((await apiLogin(jar, stack.urls.noverse, 'forced', temp)).status).toBe(200);
  expect(meBody((await me(jar, stack.urls.noverse)).body).user?.mustChangePassword).toBe(true);

  // identical gate on both apps: 403 + code + flag, reads and writes alike
  expect((await get(jar, stack.urls.noverse, '/api/library')).status).toBe(403);
  forcedBody((await get(jar, stack.urls.noverse, '/api/library')).body);
  const writeN = await post(jar, stack.urls.noverse, '/api/projects', { name: 'x' });
  expect(writeN.status).toBe(403);
  forcedBody(writeN.body);
  const bizV = await get(jar, stack.urls.viderse, '/api/status');
  expect(bizV.status).toBe(403);
  forcedBody(bizV.body);
  const writeV = await post(jar, stack.urls.viderse, '/api/notifications/read', {});
  expect(writeV.status).toBe(403);
  forcedBody(writeV.body);
  expect((await get(jar, stack.urls.viderse, '/api/definitely-not-a-route')).status).toBe(403);

  // the consumed temp password still works for the change itself
  expect((await post(jar, stack.urls.noverse, '/api/me/password', { current: temp, new: 'forced-final-pw' })).status).toBe(200);
  expect((await me(jar, stack.urls.noverse)).status).toBe(401);
  expect((await apiLogin(jar, stack.urls.viderse, 'forced', 'forced-final-pw')).status).toBe(200);
  expect(meBody((await me(jar, stack.urls.viderse)).body).user?.mustChangePassword).toBe(false);
  expect(meBody((await me(jar, stack.urls.noverse)).body).user?.mustChangePassword).toBe(false);
  expect((await get(jar, stack.urls.viderse, '/api/status')).status).toBe(200);
  stack.users['forced'].password = 'forced-final-pw';
  await jar.dispose();
});

test('SSOConcurrentSetupOneAdmin', async ({ stack }) => {
  const suffix = stack.identityDB.replace(/^verse_sso_/, '');
  const raceDB = `verse_sso_${suffix}_race`;
  await provisionRaceDB(stack, raceDB);
  stack.extraDBs.push(raceDB);
  const raceDsn = (user: string) => `postgres://${encodeURIComponent(user)}:${encodeURIComponent(stack.passwords[user])}@127.0.0.1:${stack.pgPort}/${raceDB}`;

  const nPort = await freePort();
  const vPort = await freePort();
  const n = startServer('noverse-race', stack.binNoverse, ['serve'], serverEnv(stack, 'noverse', nPort, {
    dataDsn: raceDsn('noverse_app'), workerDsn: raceDsn('noverse_engine_worker'),
  }));
  n.base = `http://127.0.0.1:${nPort}`;
  const v = startServer('viderse-race', stack.binViderse, ['serve'], serverEnv(stack, 'viderse', vPort, {
    identityDsn: raceDsn('viderse_identity'),
  }));
  v.base = `http://127.0.0.1:${vPort}`;
  stack.extraServers.push(n, v);
  await waitReady(n);
  await waitReady(v);

  try {
    const nCode = n.setupCode();
    const vCode = v.setupCode();
    expect(nCode).not.toBe(vCode);
    const pairs: [ServerProc, string][] = [[n, nCode], [v, vCode]];
    const [tokN, tokV] = await Promise.all(
      pairs.map(async ([srv, code]) => {
        const r = await fetch(`${srv.base}/api/setup/verify`, {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ code }),
        });
        expect(r.status).toBe(200);
        const rb = await r.json() as { setupToken?: string };
        return rb.setupToken!;
      }),
    );
    const mk = (srv: typeof n, tok: string, name: string) => fetch(`${srv.base}/api/setup/admin`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ setupToken: tok, name, username: name, password: 'race-admin-pass-1' }),
    });
    const [rN, rV] = await Promise.all([mk(n, tokN, 'racewin'), mk(v, tokV, 'racewin2')]);
    expect([rN.status, rV.status].sort()).toEqual([200, 409]);

    // exactly one user in the shared store; loser reports setup done
    expect((await psql(raceDB, 'SELECT count(*) FROM users')).trim()).toBe('1');
    const loser = rN.status === 409 ? n : v;
    const st = await fetch(`${loser.base}/api/setup`);
    expect(st.status).toBe(200);
    const stb = await st.json() as { needed?: boolean };
    expect(stb.needed).toBe(false);

    // DB down → 503, never a fake "empty store"
    await psql('postgres', `DROP DATABASE ${raceDB} WITH (FORCE)`);
    stack.extraDBs = stack.extraDBs.filter(d => d !== raceDB);
    const downN = await fetch(`${n.base}/api/setup`);
    expect(downN.status).toBe(503);
    const downV = await fetch(`${v.base}/api/setup`);
    expect(downV.status).toBe(503);
  } finally {
    stack.extraServers = stack.extraServers.filter(s => s !== n && s !== v);
    await n.stop();
    await v.stop();
  }
});

test('SSOCSRFNonAllowlistedOrigin', async ({ stack, playwright, apiLogin }) => {
  const jar = await playwright.request.newContext();
  expect((await apiLogin(jar, stack.urls.noverse, 'boss', stack.users['admin'].password)).status).toBe(200);
  const port = Number(new URL(stack.urls.noverse).port);
  const badOrigins = [
    `http://127.0.0.1:${port + 7}`,      // same host, third port — a stranger
    'https://evil.example',               // foreign site
    'null',                               // sandboxed/opaque origin
    `http://127.0.0.1.evil.example`,      // lookalike host
    `http://127.0.0.1:${port}.evil.example`,
  ];
  for (const origin of badOrigins) {
    for (const [method, path] of [
      ['POST', '/api/users'],
      ['PUT', '/api/me'],
      ['PATCH', '/api/me'],
      ['DELETE', '/api/sessions/whatever/revoke'],
    ] as const) {
      const res = await jar.fetch(`${stack.urls.noverse}${path}`, {
        method, headers: { 'Content-Type': 'application/json', Origin: origin }, data: {},
      });
      expect(res.status(), `${method} ${path} origin=${origin}`).toBe(403);
      const b = (await bodyOf(res)) as GateBody;
      expect(b?.error).toBe(CSRF_403);
      expect(res.headers()['access-control-allow-origin']).toBeUndefined();
    }
  }
  for (const origin of badOrigins.slice(0, 3)) {
    const res = await jar.fetch(`${stack.urls.viderse}/api/notifications/read`, {
      method: 'POST', headers: { 'Content-Type': 'application/json', Origin: origin }, data: {},
    });
    expect(res.status()).toBe(403);
    expect(((await bodyOf(res)) as GateBody)?.error).toBe(CSRF_403);
  }
  // DB unchanged: the forged POSTs never reached the handler
  const users = usersBody((await get(jar, stack.urls.noverse, '/api/users')).body);
  expect(users.users.some(u => u.username === 'forged')).toBe(false);

  // allowlisted sibling passes the CSRF layer and hits the real gate instead
  const res = await jar.fetch(`${stack.urls.noverse}/api/users`, {
    method: 'POST', headers: { 'Content-Type': 'application/json', Origin: stack.urls.viderse },
    data: { username: '', password: '', role: 'member' },
  });
  expect(res.status()).not.toBe(403);
  await jar.dispose();
});

test('SSOCRSFRealBrowserForm', async ({ stack, context, playwright, apiLogin }) => {
  // APIRequestContext shares the browser cookie jar; no cookie copying.
  expect((await apiLogin(context.request, stack.urls.noverse, 'boss', stack.users['admin'].password)).status).toBe(200);

  // hostile page on a THIRD loopback port (HTML fulfilled locally — only its
  // origin matters; the form POST goes to the real server). A fresh port, not
  // an offset from the server's, so it always stays below 65536.
  const evilPort = await freePort();
  await context.route(`http://127.0.0.1:${evilPort}/**`, route => route.fulfill({
    contentType: 'text/html',
    body: `<form id="f" method="POST" action="${stack.urls.noverse}/api/users">
      <input name="username" value="forged"><input type="submit"></form>
      <script>document.getElementById('f').submit()</script>`,
  }));
  const page = await context.newPage();
  const respP = page.waitForResponse(r => r.url() === `${stack.urls.noverse}/api/users` && r.request().method() === 'POST');
  await page.goto(`http://127.0.0.1:${evilPort}/`);
  const resp = await respP;
  expect(resp.status()).toBe(403);
  expect(resp.headers()['access-control-allow-origin']).toBeUndefined();
  expect(await resp.text()).toContain(CSRF_403);

  // handler never ran: no forged account
  const admin = await playwright.request.newContext();
  await apiLogin(admin, stack.urls.noverse, 'boss', stack.users['admin'].password);
  const users = usersBody((await get(admin, stack.urls.noverse, '/api/users')).body);
  expect(users.users.some(u => u.username === 'forged')).toBe(false);
  await admin.dispose();
});

test('SSOReturnTargetBoundary', async ({ stack, context, apiLogin }) => {
  expect((await apiLogin(context.request, stack.urls.noverse, 'boss', stack.users['admin'].password)).status).toBe(200);
  const page = await context.newPage();
    for (const probe of [
      '#//evil.example',
      '#/login?next=https://evil.example',
      '?next=//evil.example',
      '#/x?next=javascript:alert(1)',
    ]) {
      await page.goto(stack.urls.noverse + '/' + probe);
      await page.waitForTimeout(500);
      const u = new URL(page.url());
      expect(u.hostname).toBe('127.0.0.1');
      expect(u.protocol).toBe('http:');
      expect(page.url()).not.toContain('verse_session');
      expect(await page.evaluate(() => location.origin)).toBe(stack.urls.noverse);
    }
    await page.goto(stack.urls.noverse + '/#/');
    await expect(page.locator('#lg-user')).toHaveCount(0);
});

test('SSOAlternatingLoginRateLimit', async ({ stack, playwright, apiLogin }) => {
  const jar = await playwright.request.newContext();
  const wrong = 'definitely-wrong-1';

  // generic failure: an unknown user looks exactly like a wrong password
  const known = await apiLogin(jar, stack.urls.noverse, 'rateuser', wrong);
  const unknown = await apiLogin(jar, stack.urls.noverse, 'nosuchuser', wrong);
  expect(known.status).toBe(401);
  expect(unknown.status).toBe(401);
  expect(JSON.stringify(unknown.body)).toBe(JSON.stringify(known.body));

  // alternating apps shares the user bucket: after 4 recorded failures the
  // DB limiter blocks on BOTH apps — no per-app doubling of attempts
  for (const [base] of [[stack.urls.viderse], [stack.urls.noverse]] as const) {
    expect((await apiLogin(jar, base, 'rateuser', wrong)).status).toBe(401);
  }
  const b1 = await apiLogin(jar, stack.urls.viderse, 'rateuser', wrong);
  const b2 = await apiLogin(jar, stack.urls.noverse, 'rateuser', wrong);
  expect([b1.status, b2.status]).toContain(429);
  const blocked = [b1, b2].find(r => r.status === 429)!;
  const firstWait = (blocked.body as { retryAfter?: number })?.retryAfter ?? 0;
  expect(firstWait).toBeGreaterThan(0);
  expect(b1.status).toBe(429);
  expect(b2.status).toBe(429); // both apps honour the shared bucket

  // the budget lives in the shared store
  const rows = await psql(stack.identityDB, 'SELECT count(*) FROM login_rate_limits');
  expect(Number(rows.trim())).toBeGreaterThan(0);
  const budgetBeforeRestart = await psql(stack.identityDB, 'SELECT sum(failures) FROM login_rate_limits');

  // restart Noverse → the DB bucket still blocks (RAM limiter would reset)
  await killGroup(stack.pids.noverse);
  const port = Number(new URL(stack.urls.noverse).port);
  const restarted = startServer('noverse-rst', stack.binNoverse, ['serve'], serverEnv(stack, 'noverse', port));
  restarted.base = stack.urls.noverse;
  stack.pids.noverse = restarted.pid;
  stack.extraServers.push(restarted);
  await waitReady(restarted);
  const budgetAfterRestart = await psql(stack.identityDB, 'SELECT sum(failures) FROM login_rate_limits');
  expect(budgetAfterRestart.trim()).toBe(budgetBeforeRestart.trim());

  // after the block expires one more failure produces a LONGER block
  // (fails kept accumulating across the restart — proof of persistence)
  await sleep(firstWait * 1000 + 1500);
  expect((await apiLogin(jar, stack.urls.noverse, 'rateuser', wrong)).status).toBe(401);
  const b3 = await apiLogin(jar, stack.urls.noverse, 'rateuser', wrong);
  expect(b3.status).toBe(429);
  const secondWait = (b3.body as { retryAfter?: number })?.retryAfter ?? 0;
  expect(secondWait).toBeGreaterThanOrEqual(firstWait);

  // the bucket clears after a successful login
  await sleep(secondWait * 1000 + 1500);
  expect((await apiLogin(jar, stack.urls.noverse, 'rateuser', stack.users['rateuser'].password)).status).toBe(200);
  await jar.dispose();
});

test('SSORoleAndWorkerBoundary', async ({ stack, playwright, apiLogin }) => {
  // ── viderse_identity: account reads work; everything else is denied ──
  const idDsn = stack.dsn.viderseIdentity;
  expect((await psqlAs(idDsn, 'SELECT count(*) >= 1 FROM users')).trim()).toBe('t');
  for (const q of [
    'SELECT count(*) FROM projects',
    'SELECT count(*) FROM stories',
    'SELECT count(*) FROM project_members',
    `UPDATE users SET global_role='admin' WHERE username='both'`,
    `UPDATE users SET status='locked' WHERE username='both'`,
  ]) {
    await expect(psqlAs(idDsn, q)).rejects.toThrow();
  }

  // ── noverse_engine_worker: facts/lease readable, accounts unreachable ──
  const workerDsn = stack.dsn.noverseWorker;
  for (const q of [
    'SELECT count(*) FROM users',
    'SELECT count(*) FROM sessions',
    'SELECT count(*) FROM app_grants',
    'SELECT count(*) FROM audit_logs',
    'SELECT count(*) FROM login_rate_limits',
    'SELECT count(*) FROM jobs',
    'SELECT count(*) FROM blobs',
  ]) {
    await expect(psqlAs(workerDsn, q)).rejects.toThrow();
  }
  await expect(psqlAs(workerDsn, 'SELECT count(*) FROM story_leases')).resolves.toBeDefined();
  await expect(psqlAs(workerDsn, 'SELECT count(*) FROM projects')).resolves.toBeDefined();

  // ── real worker child: env-filtered launch, real lease, worker role only ──
  const admin = await playwright.request.newContext();
  expect((await apiLogin(admin, stack.urls.noverse, 'boss', stack.users['admin'].password)).status).toBe(200);
  const list = await get(admin, stack.urls.noverse, '/api/projects/e2eproj/stories');
  expect(list.status).toBe(200);
  const sl = list.body as { stories?: { id?: string }[] };
  const slug = (sl.stories ?? [])[0]?.id?.split('/').pop();
  expect(slug).toBeTruthy();

  const snap = await get(admin, stack.urls.noverse, `/api/projects/e2eproj/stories/${slug}/snapshot`);
  expect(snap.status).toBe(200); // the worker served the story through the lease

  // the child connected as the worker role
  const conns = await psql(stack.identityDB, `SELECT count(*) FROM pg_stat_activity WHERE usename='noverse_engine_worker'`);
  expect(Number(conns.trim())).toBeGreaterThan(0);

  // captured child env: worker DSN only, never the app/identity/migrate DSNs
  const children = await run('pgrep', ['-P', String(stack.pids.noverse), '-f', '__web-worker']).catch(() => '');
  const pid = children.trim().split('\n').filter(Boolean)[0];
  expect(pid).toBeTruthy();
  const env = await run('ps', ['eww', '-p', pid]);
  // Boolean matchers prevent a failed assertion from dumping the raw env.
  expect(env.includes('DATABASE_URL=postgres://noverse_engine_worker:')).toBe(true);
  expect(env.includes('noverse_app:')).toBe(false);
  expect(env.includes('noverse_migrate:')).toBe(false);
  expect(env.includes('viderse_identity:')).toBe(false);
  expect(env.includes('NOVERSE_WORKER_DATABASE_URL')).toBe(false);
  expect(env.includes('AINOVEL_STORY_SCOPE=e2eproj/')).toBe(true);
  await admin.dispose();
});

test('SSOImportedAccountsAndRollback', async ({ stack, playwright, apiLogin }) => {
  // A second, empty identity DB receives the cutover — the live stack's DB
  // keeps its fixture accounts untouched.
  const suffix = stack.identityDB.replace(/^verse_sso_/, '');
  const impDB = `verse_sso_${suffix}_imp`;
  await provisionRaceDB(stack, impDB);
  stack.extraDBs.push(impDB);
  const userCount = async () => (await psql(impDB, 'SELECT count(*) FROM users')).trim();
  expect(await userCount()).toBe('0');
  const dsnFor = (user: string) =>
    `postgres://${encodeURIComponent(user)}:${encodeURIComponent(stack.passwords[user])}@127.0.0.1:${stack.pgPort}/${impDB}`;

  // Real v4 account files written by the real file store; passwords are
  // generated per run and reach the fixture only through the environment.
  const passwords: Record<string, string> = {};
  for (const u of ['nvadmin', 'nvmember', 'nvtemp', 'vdadmin', 'vdmember']) {
    passwords[u] = `imp-${u}-${randomBytes(9).toString('base64url')}`;
  }
  const accDir = path.join(stack.dir, 'sso-src');
  mkdirSync(accDir, { recursive: true });
  const mkOut = await run(stack.binProxy, ['-mkaccounts', accDir], {
    env: { HOME: stack.dir, SSO_FIXTURE_PASSWORDS: JSON.stringify(passwords) },
  });
  const oldSession = mkOut.match(/^nvmember-session (\S+)$/m)?.[1];
  expect(/^[0-9a-f]+\.[0-9a-f]+$/.test(oldSession ?? '')).toBe(true);

  const nvAcc = path.join(accDir, 'noverse', 'web-accounts.json');
  const vdAcc = path.join(accDir, 'viderse', 'web-accounts.json');
  const vdCollide = path.join(accDir, 'viderse-collide', 'web-accounts.json');
  const libRoot = path.join(accDir, 'library');
  const backupDir = path.join(accDir, 'backup');
  mkdirSync(libRoot, { recursive: true });
  mkdirSync(backupDir, { recursive: true });
  chmodSync(backupDir, 0o700);
  const manifestPath = path.join(backupDir, 'sso-manifest.json');
  const reportPath = path.join(backupDir, 'sso-report.json');
  const impEnv = {
    HOME: stack.dir,
    MIGRATION_DATABASE_URL: dsnFor('noverse_migrate'),
    STORAGE_DRIVER: 'local',
    BLOB_LOCAL_ROOT: path.join(stack.dir, 'blobs'),
    STORAGE_ENV: 'dev',
  };
  // Source ids must stay stable across every run of the same cutover.
  const nvSrcID = randomUUID();
  const vdSrcID = randomUUID();
  const baseArgs = (vd: string) => [
    'storage', 'sso-import',
    '--noverse-store', 'file',
    '--noverse-accounts', nvAcc,
    '--noverse-source-id', nvSrcID,
    '--root', libRoot,
    '--viderse-accounts', vd,
    '--viderse-source-id', vdSrcID,
    '--manifest', manifestPath,
    '--report', reportPath,
  ];
  const reportJSON = () => JSON.parse(readFileSync(reportPath, 'utf8')) as {
    status?: string;
    collisions?: { kind?: string; sourceApp?: string; sourceUsername?: string }[];
    globalAdmins?: { username?: string; origin?: string }[];
    globalAdminsDigest?: string;
    identity?: { usersInserted?: number; usersSkipped?: number; sessionsRevoked?: number; cutoverRunId?: string };
    backupInventoryPath?: string;
  };

  // ── collision preflight: the colliding Viderse file stops dry-run before
  // any write (no users, no .v4.bak).
  const collide = await runExit(stack.binNoverse, baseArgs(vdCollide), { env: impEnv });
  expect(collide.code).toBe(1);
  const collideRep = reportJSON();
  expect(collideRep.status).toBe('preflight_failed');
  expect((collideRep.collisions ?? []).some(c =>
    c.kind === 'two_sources' && c.sourceUsername === 'nvmember')).toBe(true);
  expect(await userCount()).toBe('0');
  for (const f of [nvAcc, vdAcc, vdCollide]) {
    expect(existsSync(f + '.v4.bak'), f).toBe(false);
  }

  // ── clean dry-run: digest + global-admin approval list.
  const dry = await runExit(stack.binNoverse, baseArgs(vdAcc), { env: impEnv });
  expect(dry.code).toBe(0);
  const msha = dry.out.match(/Manifest sha256:\s*([0-9a-f]{64})/)?.[1];
  expect(msha).toMatch(/^[0-9a-f]{64}$/);
  const dryRep = reportJSON();
  const origins = (dryRep.globalAdmins ?? []).map(a => `${a.username}@${a.origin}`).sort();
  expect(origins).toEqual(['nvadmin@noverse', 'vdadmin@viderse']);
  const adminsDigest = dryRep.globalAdminsDigest;
  expect(adminsDigest).toMatch(/^[0-9a-f]{64}$/);
  expect(await userCount()).toBe('0');
  for (const f of [nvAcc, vdAcc]) {
    expect(existsSync(f + '.v4.bak'), f).toBe(false);
  }

  // ── approval gate: a wrong global-admins digest is refused before any
  // write — the report marks the stop, the DB and sources stay untouched.
  const wrong = await runExit(stack.binNoverse, [...baseArgs(vdAcc),
    '--apply', '--accept-manifest', msha!, '--accept-global-admins', '0'.repeat(64),
    '--backup-dir', backupDir], { env: impEnv });
  expect(wrong.code).toBe(1);
  expect(reportJSON().status).toBe('approval_mismatch');
  expect(await userCount()).toBe('0');
  expect(existsSync(nvAcc + '.v4.bak')).toBe(false);
  expect(existsSync(vdAcc + '.v4.bak')).toBe(false);

  // ── pre-cutover dump as the migrate role, over PG* env — never a DSN
  // argument. Taken after the dry-runs so it is provably the before-state;
  // the gate wants it owner-only, like the 0700 backup dir.
  const pgEnv = {
    PGHOST: '127.0.0.1', PGPORT: String(stack.pgPort),
    PGDATABASE: impDB, PGUSER: 'noverse_migrate',
    PGPASSWORD: stack.passwords['noverse_migrate'],
  };
  const dumpPath = path.join(backupDir, 'before.dump');
  await run('pg_dump', ['--format=custom', `--file=${dumpPath}`], { env: pgEnv });
  chmodSync(dumpPath, 0o600);
  const dumpSHA = createHash('sha256').update(readFileSync(dumpPath)).digest('hex');

  // ── apply with both approved digests.
  const apply = await runExit(stack.binNoverse, [...baseArgs(vdAcc),
    '--apply', '--accept-manifest', msha!, '--accept-global-admins', adminsDigest!,
    '--backup-dir', backupDir], { env: impEnv });
  expect(apply.code, apply.out.replace(/[0-9a-f]{12,}/g, '<digest>')).toBe(0);
  const applyRep = reportJSON();
  expect(applyRep.status).toBe('complete');
  expect(applyRep.identity?.usersInserted).toBe(5);
  expect(await userCount()).toBe('5');

  for (const src of [nvAcc, vdAcc]) {
    const bak = src + '.v4.bak';
    expect(readFileSync(bak).equals(readFileSync(src))).toBe(true);
    expect(statSync(bak).mode & 0o777).toBe(0o600);
  }
  const inventoryName = /^sso-backup-inventory-\d{8}T\d{6}Z(-\d+)?\.json$/;
  const readInventory = (p: string) => JSON.parse(readFileSync(p, 'utf8')) as {
    databaseDump?: { path?: string; sha256?: string; format?: string };
  };
  const firstInventory = applyRep.backupInventoryPath!;
  expect(path.dirname(firstInventory)).toBe(backupDir);
  expect(path.basename(firstInventory)).toMatch(inventoryName);
  const inventory = readInventory(firstInventory);
  expect(inventory.databaseDump?.path).toBe(dumpPath);
  expect(inventory.databaseDump?.sha256 === dumpSHA).toBe(true);
  expect(inventory.databaseDump?.format).toBe('custom');

  // ── resume: same command is a verified no-op — skips only, no new users,
  // and the imported password hash stays byte-identical. The first dump no
  // longer matches the store the apply changed, so the gate needs a new one;
  // the first dump and its inventory stay as the rollback point.
  const resumeDump = path.join(backupDir, 'resume.dump');
  await run('pg_dump', ['--format=custom', `--file=${resumeDump}`], { env: pgEnv });
  chmodSync(resumeDump, 0o600);
  const hashBefore = (await psql(impDB,
    `SELECT password_hash FROM user_credentials WHERE user_id = (SELECT id FROM users WHERE username='nvmember')`)).trim();
  expect(hashBefore.startsWith('$argon2id$')).toBe(true);
  const resume = await runExit(stack.binNoverse, [...baseArgs(vdAcc),
    '--apply', '--accept-manifest', msha!, '--accept-global-admins', adminsDigest!,
    '--backup-dir', backupDir], { env: impEnv });
  expect(resume.code, resume.out.replace(/[0-9a-f]{12,}/g, '<digest>')).toBe(0);
  const resumeRep = reportJSON();
  expect(resumeRep.status).toBe('complete');
  expect(resumeRep.identity?.usersInserted).toBe(0);
  expect(resumeRep.identity?.usersSkipped).toBe(5);
  expect(await userCount()).toBe('5');
  const hashAfter = (await psql(impDB,
    `SELECT password_hash FROM user_credentials WHERE user_id = (SELECT id FROM users WHERE username='nvmember')`)).trim();
  expect(hashAfter === hashBefore).toBe(true);
  // Each apply writes its own inventory; the first one still names the
  // pre-cutover dump.
  const secondInventory = resumeRep.backupInventoryPath!;
  expect(secondInventory).not.toBe(firstInventory);
  expect(path.basename(secondInventory)).toMatch(inventoryName);
  expect(readInventory(secondInventory).databaseDump?.path).toBe(resumeDump);
  expect(readInventory(firstInventory).databaseDump?.sha256 === dumpSHA).toBe(true);

  // The rollback point is the dump recorded by the first committed identity
  // transaction; each committed apply has its own record.
  const records = JSON.parse((await psql(impDB, `SELECT json_agg(json_build_object(
      'id', id, 'backup', summary->'cutoverBackup') ORDER BY started_at, id)
    FROM import_runs WHERE source_label='sso-cutover-identity' AND status='verified'`)).trim()) as {
    id: string; backup: { inventoryPath: string; dumpPath: string; dumpSha256: string };
  }[];
  expect(records.map(r => r.id)).toEqual([applyRep.identity?.cutoverRunId, resumeRep.identity?.cutoverRunId]);
  expect(records[0].backup.inventoryPath).toBe(firstInventory);
  expect(records[1].backup.inventoryPath).toBe(secondInventory);
  const rollbackDump = records[0].backup.dumpPath;
  expect(rollbackDump).toBe(dumpPath);
  expect(createHash('sha256').update(readFileSync(rollbackDump)).digest('hex') === records[0].backup.dumpSha256).toBe(true);

  // ── the app pair on the imported store: real login checks over HTTP.
  const nPort = await freePort();
  const vPort = await freePort();
  const nBase = `http://127.0.0.1:${nPort}`;
  const vBase = `http://127.0.0.1:${vPort}`;
  const n = startServer('noverse-imp', stack.binNoverse, ['serve'], serverEnv(stack, 'noverse', nPort, {
    dataDsn: dsnFor('noverse_app'), workerDsn: dsnFor('noverse_engine_worker'),
  }));
  n.base = nBase;
  const v = startServer('viderse-imp', stack.binViderse, ['serve'], serverEnv(stack, 'viderse', vPort, {
    identityDsn: dsnFor('viderse_identity'),
  }));
  v.base = vBase;
  stack.extraServers.push(n, v);
  try {
    await waitReady(n);
    await waitReady(v);

    // Both global admins open both apps with admin/admin effective grants.
    for (const admin of ['nvadmin', 'vdadmin']) {
      for (const loginBase of [nBase, vBase]) {
        const jar = await playwright.request.newContext();
        try {
          expect((await apiLogin(jar, loginBase, admin, passwords[admin])).status).toBe(200);
          for (const base of [nBase, vBase]) {
            const current = await me(jar, base);
            expect(current.status).toBe(200);
            expect(meBody(current.body).user?.grants).toEqual({ noverse: 'admin', viderse: 'admin' });
          }
          expect((await get(jar, nBase, '/api/library')).status).toBe(200);
          expect((await get(jar, vBase, '/api/status')).status).toBe(200);
        } finally {
          await jar.dispose();
        }
      }
    }

    // Per-app members keep their single grant.
    const nvmember = await playwright.request.newContext();
    expect((await apiLogin(nvmember, nBase, 'nvmember', passwords['nvmember'])).status).toBe(200);
    expect((await get(nvmember, nBase, '/api/library')).status).toBe(200);
    const deniedV = await get(nvmember, vBase, '/api/status');
    expect(deniedV.status).toBe(403);
    deniedBody(deniedV.body);
    await nvmember.dispose();

    const vdmember = await playwright.request.newContext();
    expect((await apiLogin(vdmember, vBase, 'vdmember', passwords['vdmember'])).status).toBe(200);
    expect((await get(vdmember, vBase, '/api/status')).status).toBe(200);
    const deniedN = await get(vdmember, nBase, '/api/library');
    expect(deniedN.status).toBe(403);
    deniedBody(deniedN.body);
    await vdmember.dispose();

    // The imported one-time temp password logs in but every business GET is
    // gated until it is changed.
    const temp = await playwright.request.newContext();
    expect((await apiLogin(temp, nBase, 'nvtemp', passwords['nvtemp'])).status).toBe(200);
    expect(meBody((await me(temp, nBase)).body).user?.mustChangePassword).toBe(true);
    const gated = await get(temp, nBase, '/api/library');
    expect(gated.status).toBe(403);
    forcedBody(gated.body);
    await temp.dispose();

    // A pre-cutover file-store bearer is dead on both apps.
    const stale = await playwright.request.newContext();
    expect((await req(stale, 'get', nBase, '/api/me', undefined, { cookie: `verse_session=${oldSession ?? ''}` })).status).toBe(401);
    expect((await req(stale, 'get', vBase, '/api/me', undefined, { cookie: `verse_session=${oldSession ?? ''}` })).status).toBe(401);


    // A wrong password is the same generic 401 as an unknown account.
    const wrongPw = await apiLogin(stale, nBase, 'nvmember', `wrong-${randomBytes(6).toString('base64url')}`);
    const noSuch = await apiLogin(stale, nBase, 'nosuchuser', `wrong-${randomBytes(6).toString('base64url')}`);
    expect(wrongPw.status).toBe(401);
    expect(noSuch.status).toBe(401);
    expect(JSON.stringify(noSuch.body)).toBe(JSON.stringify(wrongPw.body));
    await stale.dispose();
  } finally {
    stack.extraServers = stack.extraServers.filter(s => s !== n && s !== v);
    await n.stop();
    await v.stop();
  }

  // ── restore rehearsal: the recorded rollback dump must rebuild the
  // pre-cutover database — zero users proves it is the before-state.
  const rstDB = `verse_sso_${suffix}_rst`;
  await psql('postgres', `CREATE DATABASE ${rstDB}`);
  try {
    await run('pg_restore', ['--no-owner', '--no-privileges', '-d', rstDB, rollbackDump], {
      env: { PGHOST: '127.0.0.1', PGPORT: String(stack.pgPort), PGUSER: 'dat' },
      timeoutMs: 300_000,
    });
    expect((await psql(rstDB, 'SELECT count(*) FROM users')).trim()).toBe('0');
  } finally {
    await psql('postgres', `DROP DATABASE IF EXISTS ${rstDB} WITH (FORCE)`);
    stack.extraDBs = stack.extraDBs.filter(d => d !== rstDB);
  }
});

