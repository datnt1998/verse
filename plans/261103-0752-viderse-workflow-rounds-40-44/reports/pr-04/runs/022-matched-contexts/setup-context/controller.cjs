'use strict';
// PREPARED ONLY. Main may execute sequentially. No native writes, installer, login, or provider calls.
const { createRequire } = require('node:module');
const { chromium, expect } = createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');
const { createHash } = require('node:crypto');
const ROOT = __dirname;
const REPO = '/Users/dat/Projects/verse';
const ORIGIN = 'http://127.0.0.1:8043';
const COOKIE_FILE = '/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json';
const DESKTOP = { width: 1440, height: 900 }, MOBILE = { width: 390, height: 844 };
const IDS = ['yt-dlp', 'ffmpeg', 'transcription', 'chrome', 'codex', 'codex-login'];
const TITLES = ['Tải video và phụ đề', 'Xử lý hình và tiếng', 'Tách tiếng và tạo phụ đề', 'Dựng và kiểm tra video', 'Công cụ vẽ ảnh', 'Đăng nhập ChatGPT'];
const SIZES = [125000000, 90000000, 1300000000, 0, 300000000, 0];
const PROPS = { step: 'setup', setup: 'running', tplMenu: false, identity: 'running', rivals: 'some' };
const FILES = {
  running: 'FIXTURE-1440-onboard-setup-running-initial.png',
  failed: 'FIXTURE-1440-onboard-setup-failed-initial.png',
  retry503: 'INTERACTION-1440-setup-retry-503.png',
  retryRunning: 'INTERACTION-1440-setup-retry-running.png',
  mobile: 'NONCANONICAL-390-setup-desktop-only-supplement.png',
};
const clone = value => JSON.parse(JSON.stringify(value));
const R = {
  status: 'NOT_RUN', proofClass: 'NATIVE_REACT_LOCAL_RESPONSE_FIXTURE_NOT_NATIVE_STATE_OR_PERSISTENCE',
  cases: [], captures: [], comparisons: [], requests: [], pageErrors: [], concerns: [
    { code: 'FAILED_COPY_NOT_BYTE_PARITY', source: 'web/src/screens/setup/SetupScreen.tsx:38-57; web/src/i18n/setup.ts:69; Onboard.dc.html:1191', detail: 'React translates raw connection error into “Mạng bị ngắt khi tải.”; canonical says “Mất kết nối khi tải mô hình.”. Both retain 806 MB, but wording/line wrapping cannot be made identical through the DTO without changing shipped copy.' },
    { code: 'GEOMETRY_NOT_PREACCEPTED', source: 'Onboard.dc.html:925,934-935; web/src/screens/setup/setup.stylex.ts:7,13-14', detail: 'Canonical shell is height:100vh/overflow:hidden; React setup root is minHeight:100vh. Both use a 720px column, 28px/20px/32px main padding and 18px gaps. No DTO field controls shell geometry; measured bounds are recorded, not fabricated or asserted pixel-identical.' },
    { code: 'MOBILE_NO_CANONICAL_ONBOARD', source: 'plans/reports/orchestrate-261105-095440-v38-acceptance/mobile-tweak-truth.md:7; web/src/screens/setup/SetupScreen.tsx:190-202', detail: '390px is a noncanonical desktop-only notice supplement, with no reference ID and no six-row parity claim.' },
    { code: 'LOGIN_NOT_ACTIVATED', source: 'web/src/screens/settings/connect/ChatGPTLogin.tsx:65-80', detail: 'Primary/ghost CTA presence, enabled state, keyboard focusability, and height are checked; no login click, window.open, provider start, polling, or OAuth is exercised.' },
  ],
  safety: { nativeWritesContinued: 0, externalContinued: 0, providerContinued: 0, downloads: 0, unexpectedNonGetAttempts: 0, providerAttempts: 0 },
};
let browser, cookies;
function setupFixture(kind) {
  const states = kind === 'failed'
    ? ['done', 'done', 'failed', 'done', 'done', 'needs_user']
    : ['done', 'done', 'running', 'done', 'pending', 'needs_user'];
  return {
    state: kind, complete: false, running: kind === 'running',
    disk: { freeBytes: 100000000000, requiredBytes: 4000000000, approxBytes: 1800000000, enough: true },
    steps: IDS.map((id, i) => ({
      id, state: states[i], sizeBytes: SIZES[i], provided: id === 'chrome',
      progress: i === 2 ? {
        phase: 'download_model', phaseLabel: 'Đang tải mô hình nhận giọng nói',
        bytesDone: 806000000, bytesTotal: 1300000000, pct: 62,
        // True DTO timestamp, not a body-text override. 392s elapsed => ~240s remaining.
        startedAt: new Date(Date.now() - 392000).toISOString(),
      } : null,
      error: i === 2 && kind === 'failed' ? 'network connection interrupted while downloading model' : null,
    })),
  };
}
async function json(route, body, status = 200) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
}
function forbiddenGet(u, request) {
  return /\/api\/(?:ai|providers?|inference|generate|sources\/parse|platforms)(?:\/|$)/i.test(u.pathname)
    || /\/(?:export|downloads?|localdownload|externaldownload|discover|discovery|follow)(?:\/|$)/i.test(u.pathname)
    || /\.(?:mp4|webm|mp3|wav|m4a|zip|tar|gz|bin|onnx|safetensors|pt|whl|dmg|pkg)$/i.test(u.pathname)
    || [...u.searchParams.keys()].some(k => /^(download|export|attachment)$/i.test(k))
    || ['media', 'image'].includes(request.resourceType());
}
async function makeContext(row, fixture) {
  const c = await browser.newContext({ viewport: row.viewport, deviceScaleFactor: 1, serviceWorkers: 'block', acceptDownloads: false,
    ...(row.viewport.width === 390 ? { isMobile: true, hasTouch: true } : {}) });
  try {
    await c.addCookies(cookies);
    await c.addInitScript(() => { sessionStorage.setItem('viderse:onboard-done', '1'); });
    await c.route('**/*', async route => {
      const q = route.request(), u = new URL(q.url());
      const event = { case: row.key, method: q.method(), path: u.pathname, queryKeys: [...u.searchParams.keys()] };
      const record = action => R.requests.push({ ...event, action });
      if (u.origin !== ORIGIN) { record('ABORT_EXTERNAL'); return route.abort('blockedbyclient'); }
      if (/\/api\/(?:ai|providers?|inference|generate)(?:\/|$)/i.test(u.pathname)) {
        R.safety.providerAttempts++; record('ABORT_PROVIDER'); return route.abort('blockedbyclient');
      }
      if (q.method() !== 'GET') {
        // Only these two exact UI-client POSTs receive local typed responses. Neither reaches native transport.
        if (row.allowRetry && q.method() === 'POST' && u.pathname === '/api/setup/retry' && fixture.retries < 2) {
          const body = q.postDataJSON();
          expect(body).toEqual({ id: 'transcription' });
          fixture.retries++;
          const before = clone(fixture.data);
          if (fixture.retries === 1) {
            await json(route, { error: 'SYNTHETIC retry unavailable', code: 'fixture_retry_503' }, 503);
            row.responseReceipts.push({ method: 'POST', path: u.pathname, payload: body, status: 503, unchangedSetup: before });
          } else {
            // Retry retains all completed tools and all byte/progress fields. This is NOT canonical initial running.
            fixture.data = clone(before);
            fixture.data.state = 'running'; fixture.data.running = true;
            fixture.data.steps[2].state = 'running'; fixture.data.steps[2].error = null;
            await json(route, fixture.data, 202);
            row.responseReceipts.push({ method: 'POST', path: u.pathname, payload: body, status: 202, response: clone(fixture.data) });
          }
          record('LOCAL_TYPED_RETRY_RESPONSE_NO_NETWORK'); return;
        }
        R.safety.unexpectedNonGetAttempts++; record('ABORT_DEFAULT_NON_GET'); return route.abort('blockedbyclient');
      }
      if (forbiddenGet(u, q)) { record('ABORT_FORBIDDEN_GET'); return route.abort('blockedbyclient'); }
      if (u.pathname === '/api/setup/state') {
        record('LOCAL_SETUP_STATUS_DTO'); row.responseReceipts.push({ method: 'GET', path: u.pathname, status: 200, response: clone(fixture.data) });
        return json(route, fixture.data);
      }
      if (/\/(?:events|stream)$/.test(u.pathname)) {
        record('LOCAL_EMPTY_EVENTS'); return route.fulfill({ contentType: 'text/event-stream', body: ': isolated local fixture\n\n' });
      }
      record(u.pathname.startsWith('/api/') ? 'NATIVE_READ_ONLY_GET' : 'SAME_ORIGIN_ASSET_GET');
      return route.continue();
    });
    await c.routeWebSocket('**/*', socket => { R.requests.push({ case: row.key, action: 'CLOSE_WEBSOCKET' }); socket.close(); });
    return c;
  } catch (error) { await c.close(); throw error; }
}
async function settle(p) {
  await p.evaluate(async () => {
    await document.fonts.ready;
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });
}
async function badge(p, mobile) {
  await p.evaluate(noncanonical => {
    const b = document.createElement('div'); b.id = 'setup-fixture-receipt'; b.setAttribute('aria-hidden', 'true');
    b.textContent = noncanonical ? 'NONCANONICAL MOBILE · LOCAL DTO FIXTURE · NO WRITES / LOGIN' : 'LOCAL DTO FIXTURE · NATIVE REACT · NO WRITES / LOGIN · NOT PERSISTENCE';
    Object.assign(b.style, { position: 'fixed', left: '4px', bottom: '4px', zIndex: '2147483647', pointerEvents: 'none', font: '9px monospace', color: '#fff', background: '#6b165c', padding: '3px' });
    document.body.append(b);
  }, mobile);
}
async function capture(p, row, fixture, stage, file, referenceIds = []) {
  await settle(p);
  const geometry = await p.evaluate(() => {
    const rect = selector => { const node = document.querySelector(selector); if (!node) return null; const r = node.getBoundingClientRect(); const s = getComputedStyle(node); return { x: r.x, y: r.y, width: r.width, height: r.height, padding: s.padding, gap: s.gap, maxWidth: s.maxWidth }; };
    return { viewport: { width: innerWidth, height: innerHeight }, scrollWidth: document.documentElement.scrollWidth, scrollY,
      root: rect('.vv-setup'), header: rect('.vv-setup__head'), main: rect('.vv-setup__main'), column: rect('.vv-setup__col'), card: rect('.vv-setup__card'), list: rect('.vv-setup__steps'),
      rows: [...document.querySelectorAll('.vv-setup__row')].map(el => { const r = el.getBoundingClientRect(); return { classes: el.className, text: el.innerText, x: r.x, y: r.y, width: r.width, height: r.height }; }),
      ctas: [...document.querySelectorAll('.vv-setup__row button')].map(el => { const r = el.getBoundingClientRect(); return { text: el.innerText, height: r.height, width: r.width, disabled: el.disabled }; }) };
  });
  expect(geometry.scrollWidth).toBeLessThanOrEqual(row.viewport.width + 1);
  const entry = { file, stage, referenceIds, canonicalComparison: referenceIds.length > 0, props: referenceIds.length ? { ...PROPS, setup: row.key } : null,
    viewport: row.viewport, fixture: clone(fixture.data), geometry, proofClass: R.proofClass };
  // Exactly one immutable filename per stage; never point failed references at a retry image.
  expect(R.captures.some(s => s.file === file)).toBe(false);
  await p.screenshot({ path: path.join(ROOT, file), animations: 'disabled' });
  R.captures.push(entry); row.captures.push(file);
  for (const referenceId of referenceIds) R.comparisons.push({ referenceId, filename: file, stage, props: entry.props, state: clone(fixture.data) });
}
async function assertDesktop(p, fixture, kind) {
  const rows = p.locator('.vv-setup__row');
  await expect(rows).toHaveCount(6);
  for (let i = 0; i < IDS.length; i++) {
    await expect(rows.nth(i)).toHaveClass(new RegExp('(?:^| )is-' + fixture.data.steps[i].state + '(?: |$)'));
    await expect(rows.nth(i).locator('.vv-setup__row-title')).toHaveText(TITLES[i]);
  }
  await expect(rows.nth(0)).toContainText('Đã cài · 125 MB');
  await expect(rows.nth(1)).toContainText('Đã cài · 90 MB');
  await expect(rows.nth(3)).toContainText('Có sẵn');
  await expect(rows.nth(4)).toContainText(fixture.data.steps[4].state === 'pending' ? 'Chờ lượt · 300 MB' : 'Đã cài · 300 MB');
  const done = fixture.data.steps.filter(s => s.state === 'done').length;
  await expect(p.locator('.vv-setup__card')).toContainText(`${done} / 6 xong`);
  await expect(p.locator('.vv-setup__track')).toHaveAttribute('aria-valuenow', String(Math.round(done / 6 * 100)));
  await expect(p.getByRole('heading', { name: 'Chuẩn bị máy cho 4 trợ lý', exact: true })).toBeVisible();
  await expect(p.locator('.vv-setup__hero')).toContainText('khoảng 1,8 GB');
  if (kind === 'running') await expect(p.locator('.vv-setup__card')).toContainText('Đang cài · còn khoảng 4 phút');
  if (kind === 'failed') {
    await expect(rows.nth(2)).toContainText('Mạng bị ngắt khi tải. Đã giữ 806 MB, thử lại sẽ tải tiếp.');
    await expect(rows.nth(2).getByRole('button', { name: 'Thử lại', exact: true })).toBeEnabled();
    await expect(p.locator('.vv-setup__track')).toHaveClass(/is-fail/);
    await expect(p.locator('.vv-setup__card')).toContainText('Một bước chưa cài được');
    await expect(rows.nth(5)).toHaveClass(/is-focus/);
  } else {
    await expect(rows.nth(2)).toContainText('Đang tải mô hình nhận giọng nói · 806 MB / 1,3 GB');
    await expect(rows.nth(2).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '62');
    await expect(rows.nth(2).locator('.vv-setup__row-side')).toHaveText('62%');
    await expect(rows.nth(5)).not.toHaveClass(/is-focus/);
  }
  const login = rows.nth(5).getByRole('button', { name: kind === 'failed' ? 'Đăng nhập ChatGPT' : 'Đăng nhập ngay', exact: true });
  await expect(login).toBeVisible(); await expect(login).toBeEnabled();
  // Check the actual primary/ghost size and accessibility without activating provider code.
  const box = await login.boundingBox(); expect(box.height).toBe(kind === 'failed' ? 36 : 32);
  await expect(p.getByRole('button', { name: 'Tạo kênh đầu tiên', exact: true })).toHaveCount(0);
}
async function run(kind, mobile = false) {
  const row = { key: mobile ? 'mobile-supplement' : kind, viewport: mobile ? MOBILE : DESKTOP, status: 'RUNNING', allowRetry: kind === 'failed' && !mobile, captures: [], responseReceipts: [], oracles: [] };
  R.cases.push(row);
  const fixture = { data: setupFixture(kind), retries: 0 };
  const initial = clone(fixture.data);
  let c;
  try {
    c = await makeContext(row, fixture); const p = await c.newPage(); p.setDefaultTimeout(12000);
    p.on('pageerror', error => R.pageErrors.push({ case: row.key, message: error.message }));
    p.on('download', download => { R.safety.downloads++; void download.cancel(); });
    p.on('popup', popup => { row.unexpectedPopup = true; void popup.close(); });
    await p.goto(ORIGIN + '/#/setup', { waitUntil: 'domcontentloaded' }); await badge(p, mobile);
    if (mobile) {
      await expect(p.getByRole('heading', { name: 'Chuẩn bị máy chỉ có trên máy tính', exact: true })).toBeVisible();
      await expect(p.locator('.vv-setup__row')).toHaveCount(0);
      await expect(p.getByRole('button', { name: /Đăng nhập|Thử lại|Tạo kênh/ })).toHaveCount(0);
      await capture(p, row, fixture, 'noncanonical-desktop-only-notice', FILES.mobile);
    } else {
      await expect(p.locator('.vv-setup__row')).toHaveCount(6);
      await capture(p, row, fixture, 'initial', FILES[kind], kind === 'running' ? ['Onboard.setup.running', 'Onboard.step.setup'] : ['Onboard.setup.failed']);
      await assertDesktop(p, fixture, kind);
      row.oracles.push('Exact tool keys/order, row states, real copy titles, completed Chrome, 806 MB/1.3 GB via numeric DTO; 62% via progress.pct; overall count remains completed-tools/6, not 62%.');
      if (kind === 'failed') {
        await p.locator('.vv-setup__row.is-failed').getByRole('button', { name: 'Thử lại', exact: true }).click();
        await expect(p.locator('.vv-setup__row.is-failed [role="alert"]')).toContainText('Chưa thử lại được');
        expect(fixture.retries).toBe(1); expect(fixture.data).toEqual(initial);
        await assertDesktop(p, fixture, 'failed');
        await capture(p, row, fixture, 'interaction-retry-503-not-canonical', FILES.retry503);
        await p.locator('.vv-setup__row.is-failed').getByRole('button', { name: 'Thử lại', exact: true }).click();
        await expect(p.locator('.vv-setup__row.is-failed')).toHaveCount(0);
        expect(fixture.retries).toBe(2);
        expect(fixture.data.steps[2].progress).toEqual(initial.steps[2].progress);
        for (const i of [0, 1, 3, 4, 5]) expect(fixture.data.steps[i]).toEqual(initial.steps[i]);
        await assertDesktop(p, fixture, 'retry-running');
        await expect(p.locator('.vv-setup__row [role="alert"]')).toHaveCount(0);
        await capture(p, row, fixture, 'interaction-retry-running-not-canonical', FILES.retryRunning);
        row.oracles.push('Locally fulfilled 503 retains exact setup DTO; locally fulfilled 202 changes only global running/state plus failed transcription state/error. All bytes/pct/startedAt and completed tools remain. No retry image carries a canonical failed reference.');
      }
    }
    row.status = 'PASS_LOCAL_COMPONENT_FIXTURE_ONLY';
  } catch (error) { row.status = 'FAIL'; row.reason = error.message; }
  finally { if (c) { await c.close(); row.contextClosed = true; } }
}
async function sourceReceipts() {
  const files = ['apps/viderse/web/src/api/setup.ts', 'apps/viderse/web/src/screens/setup/SetupScreen.tsx', 'apps/viderse/web/src/screens/setup/setup.stylex.ts', 'apps/viderse/web/src/i18n/setup.ts', 'apps/viderse/web/src/screens/settings/fmt.ts', 'apps/viderse/web/src/screens/settings/connect/ChatGPTLogin.tsx', 'apps/viderse/docs/design/v1/artboards/Onboard.dc.html', 'plans/reports/orchestrate-261105-095440-v38-acceptance/desktop-tweak-truth.md', 'plans/reports/orchestrate-261105-095440-v38-acceptance/mobile-tweak-truth.md'];
  R.sources = [];
  for (const file of files) { const bytes = await fs.readFile(path.join(REPO, file)); R.sources.push({ file, sha256: createHash('sha256').update(bytes).digest('hex') }); }
}
(async () => {
  R.startedAt = new Date().toISOString();
  try {
    await fs.mkdir(ROOT, { recursive: true }); await sourceReceipts();
    // Never log cookie contents or write them into receipts.
    cookies = JSON.parse(await fs.readFile(COOKIE_FILE, 'utf8')).map(c => ({ ...c, domain: '127.0.0.1', secure: false }));
    browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true });
    await run('running'); await run('failed'); await run('running', true);
    R.status = R.cases.some(row => row.status === 'FAIL' || row.unexpectedPopup) || R.pageErrors.length || R.safety.downloads || R.safety.unexpectedNonGetAttempts || R.safety.providerAttempts ? 'FAIL' : 'DONE_WITH_SOURCE_BACKED_CONCERNS';
  } catch (error) { R.status = 'BLOCKED'; R.error = error.message; }
  finally {
    if (browser) { try { await browser.close(); R.browserClosed = true; } catch (error) { R.browserClosed = false; R.closeError = error.message; R.status = 'FAIL'; } }
    R.finishedAt = new Date().toISOString();
    await fs.writeFile(path.join(ROOT, 'results.json'), JSON.stringify(R, null, 2));
  }
  if (R.status === 'FAIL' || R.status === 'BLOCKED') process.exitCode = 1;
  console.log(JSON.stringify({ status: R.status, results: path.join(ROOT, 'results.json'), browserClosed: R.browserClosed, captures: R.captures.map(c => c.file) }, null, 2));
})().catch(error => { console.error(error.message); process.exitCode = 1; });
