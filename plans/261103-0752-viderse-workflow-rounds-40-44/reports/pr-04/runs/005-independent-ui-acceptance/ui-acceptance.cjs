#!/usr/bin/env node
'use strict';
// Controller only: does not start/build the native server. All artifacts stay beside this file.
const { createRequire } = require('node:module');
const fs = require('node:fs/promises');
const path = require('node:path');
const REPO = '/Users/dat/Projects/verse';
const { chromium, expect } = createRequire(path.join(REPO, 'package.json'))('@playwright/test');
const ROOT = __dirname;
const BASE = new URL(process.env.VIDERSE_QA_BASE || 'http://127.0.0.1:8043').origin;
const COOKIE_SOURCE = path.join(REPO, 'plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/channel-native-qa/channel-qa.cjs');
const TITLE = 'Dặn trợ lý về kênh này';
const CHOOSE = 'Dặn trợ lý nào';
const RECIPIENTS = [
  { name: 'Trinh sát', example: 'chỉ lấy video trên 20 phút' },
  { name: 'Đạo diễn', example: 'tiêu đề ngắn dưới 60 ký tự' },
  { name: 'Hoạ sĩ', example: 'ảnh ấm hơn, bớt tối' },
  { name: 'Kiểm định', example: 'kiểm kỹ lỗi tay, mặt và khung hình bị cắt' },
];
const LIBRARY = [
  ['templates', 'Mẫu video'], ['prompts', 'Chỉ dẫn ảnh'],
  ['backdrops', 'Video nền'], ['overlays', 'Lớp phủ'],
];
const SURFACES = [
  { key: 'desktop-normal', width: 1440, height: 900, mobile: false, motion: 'no-preference' },
  { key: 'mobile-normal', width: 390, height: 844, mobile: true, motion: 'no-preference' },
  { key: 'desktop-reduced', width: 1440, height: 900, mobile: false, motion: 'reduce' },
  { key: 'mobile-reduced', width: 390, height: 844, mobile: true, motion: 'reduce' },
];
const report = {
  generated: new Date().toISOString(), status: 'RUNNING', actions: [], blockedMutations: [],
  blockedExternalReads: 0, blockedActionReads: 0, blockedSockets: 0, pageErrorCount: 0,
  checked: [], unproven: [
    'Save/send/autosave, mutation responses and persisted recipient instructions: deliberately not exercised.',
    'Paid AI/provider/image generation, downloads and external network access: deliberately blocked/not exercised.',
    'Native binary freshness and source-to-served-asset parity: parent must establish before running.',
    'Physical device/OS keyboard, non-Chromium engines and assistive-technology announcements.',
    '44px is a mobile acceptance gate; desktop dimensions are recorded without imposing mobile sizing.',
    'Hit geometry requires 44px box/generated-halo extent plus real central-disk/cardinal hit ownership; rounded corners are not required to own an enclosing square.',
  ],
};
class GateError extends Error {
  constructor(code, evidence) { super(code); this.name = 'GateError'; this.evidence = evidence; }
}
function gate(ok, code, evidence) { if (!ok) throw new GateError(code, evidence); }
const prefix = (s) => new RegExp('^' + s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
async function settled(page) {
  await page.evaluate(async () => {
    await document.fonts.ready;
    const finite = document.getAnimations().filter((a) => Number.isFinite(a.effect?.getComputedTiming().endTime));
    await Promise.all(finite.map((a) => a.finished.catch(() => {})));
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });
}
async function check(surface, name, fn) {
  const row = { surface: surface.key, check: name };
  try { row.evidence = await fn(); row.status = 'PASS'; }
  catch (error) {
    row.status = 'FAIL';
    // Never serialize Playwright call logs, request bodies, page text, cookie values or raw exception messages.
    row.reason = error instanceof GateError ? error.message : `browser-or-runtime-${error?.name || 'Error'}`;
    if (error instanceof GateError) row.evidence = error.evidence;
  }
  report.actions.push(row);
  console.log(`${row.status} ${surface.key} ${name}`);
}
async function inViewport(locator) {
  return locator.evaluate((el) => {
    const r = el.getBoundingClientRect();
    let left = 0, top = 0, right = innerWidth, bottom = innerHeight;
    for (let p = el.parentElement; p; p = p.parentElement) {
      const c = getComputedStyle(p), b = p.getBoundingClientRect();
      if (/(hidden|clip|auto|scroll)/.test(c.overflowX)) { left = Math.max(left, b.left); right = Math.min(right, b.right); }
      if (/(hidden|clip|auto|scroll)/.test(c.overflowY)) { top = Math.max(top, b.top); bottom = Math.min(bottom, b.bottom); }
    }
    const range = document.createRange(); range.selectNodeContents(el);
    const text = [...range.getClientRects()].filter((b) => b.width > 0 && b.height > 0);
    const inside = (b) => b.left >= left - 1 && b.top >= top - 1 && b.right <= right + 1 && b.bottom <= bottom + 1;
    return { x: r.x, y: r.y, width: r.width, height: r.height,
      inside: r.width > 0 && r.height > 0 && inside(r), textInside: text.length > 0 && text.every(inside),
      truncated: el.scrollWidth > el.clientWidth + 1 };
  });
}
async function pill(group) {
  return group.evaluate((el) => {
    const selected = el.querySelector('[aria-checked="true"], [aria-selected="true"]');
    const thumb = el.querySelector('.kit-seg-thumb');
    if (!selected || !thumb) return { present: false };
    const a = selected.getBoundingClientRect(), b = thumb.getBoundingClientRect();
    const delta = { x: Math.abs(a.x - b.x), y: Math.abs(a.y - b.y), width: Math.abs(a.width - b.width), height: Math.abs(a.height - b.height) };
    return { present: getComputedStyle(thumb).visibility !== 'hidden', fontStatus: document.fonts.status,
      delta, maxDelta: Math.max(...Object.values(delta)) };
  });
}
async function assertPill(group) {
  const measurement = await pill(group);
  gate(measurement.present && measurement.fontStatus === 'loaded' && measurement.maxDelta <= 2,
    'font-ready-pill-misalignment', measurement);
  return measurement;
}
async function target(locator) {
  // Center deliberately, rather than leaving the last row at a scrollport/sticky edge.
  await locator.evaluate(async (el) => {
    el.scrollIntoView({ block: 'center', inline: 'center', behavior: 'instant' });
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });
  return locator.evaluate((el) => {
    const r = el.getBoundingClientRect(), cs = getComputedStyle(el);
    const owns = (x, y) => { const hit = document.elementFromPoint(x, y); return !!hit && (hit === el || el.contains(hit)); };
    const pseudo = ['::before', '::after'].map((name) => {
      const p = getComputedStyle(el, name);
      const number = (v) => Number.parseFloat(v) || 0;
      const width = number(p.width) + (p.boxSizing === 'border-box' ? 0 : number(p.paddingLeft) + number(p.paddingRight) + number(p.borderLeftWidth) + number(p.borderRightWidth));
      const height = number(p.height) + (p.boxSizing === 'border-box' ? 0 : number(p.paddingTop) + number(p.paddingBottom) + number(p.borderTopWidth) + number(p.borderBottomWidth));
      return { name, content: p.content, position: p.position, pointerEvents: p.pointerEvents,
        top: p.top, right: p.right, bottom: p.bottom, left: p.left, width: p.width, height: p.height,
        extent44: !['none', 'normal'].includes(p.content) && p.pointerEvents !== 'none' && width >= 43.99 && height >= 43.99 };
    });
    // A 44px circular/rounded target need not own the corners of its enclosing square.
    // Require 44px declared extent (box or generated halo), PLUS real hit ownership
    // across the central disk and cardinal extremes. 0.5px inset avoids fractional
    // CSS-coordinate/device-pixel boundary rounding; no missed probe is forgiven.
    const cx = r.x + r.width / 2, cy = r.y + r.height / 2;
    const points = [[0, 0], [-21.5, 0], [21.5, 0], [0, -21.5], [0, 21.5]];
    for (let y = -20; y <= 20; y += 2) for (let x = -20; x <= 20; x += 2) {
      if (x * x + y * y <= 21.5 * 21.5) points.push([x, y]);
    }
    const missed = points.filter(([x, y]) => !owns(cx + x, cy + y));
    const borderBox44 = r.width >= 43.99 && r.height >= 43.99;
    const declaredExtent44 = borderBox44 || pseudo.some((p) => p.extent44);
    const disabled = el.disabled === true || el.getAttribute('aria-disabled') === 'true';
    return { tag: el.tagName.toLowerCase(), role: el.getAttribute('role'), id: el.id || null,
      x: r.x, y: r.y, width: r.width, height: r.height, disabled,
      viewport: { width: innerWidth, height: innerHeight }, radius: cs.borderRadius,
      visible: r.width > 0 && r.height > 0 && cs.visibility !== 'hidden' && cs.display !== 'none',
      borderBox44, declaredExtent44, hitProbeShape: 'central-disk-radius-21.5-plus-cardinal-extremes',
      effective44: declaredExtent44 && missed.length === 0, probeCount: points.length,
      missedSamples: missed.length,
      missedProbes: missed.slice(0, 24).map(([dx, dy]) => {
        const hit = document.elementFromPoint(cx + dx, cy + dy);
        return { dx, dy, hitTag: hit?.tagName.toLowerCase() || null, hitRole: hit?.getAttribute('role') || null,
          hitId: hit?.id || null };
      }), pseudo };
  });
}
async function auditTargets(locator, mobile) {
  const rows = [];
  for (let index = 0; index < await locator.count(); index++) {
    const el = locator.nth(index);
    if (!(await el.isVisible())) continue;
    rows.push({ index, ...await target(el) });
  }
  gate(rows.length > 0, 'no-form-controls-found');
  const small = rows.filter((r) => !r.disabled && !r.effective44);
  gate(!mobile || small.length === 0, 'mobile-hit-area-below-44', { controls: rows, failingIndexes: small.map((r) => r.index) });
  return { enforcement: mobile ? '44px' : 'dimensions-only', controls: rows };
}
const visibleLayers = (page) => page.locator('[role="dialog"]:visible, [role="alertdialog"]:visible, [role="menu"]:visible, [role="listbox"]:visible');
async function singleSheet(page, sheet) {
  const count = await visibleLayers(page).count();
  const geom = await inViewport(sheet);
  gate(count === 1 && geom.inside, 'nested-or-out-of-viewport-overlay', { count, geom });
  return { layerCount: count, bounds: geom };
}
async function trapFocus(page, sheet) {
  const focusable = sheet.locator('button:not(:disabled), a[href], input:not(:disabled), textarea:not(:disabled), select:not(:disabled), [tabindex="0"]');
  const visible = [];
  for (let i = 0; i < await focusable.count(); i++) if (await focusable.nth(i).isVisible()) visible.push(focusable.nth(i));
  gate(visible.length > 0, 'sheet-no-focusable-controls');
  await visible[visible.length - 1].focus(); await page.keyboard.press('Tab');
  const forwardWrapped = await visible[0].evaluate((el) => el === document.activeElement);
  await visible[0].focus(); await page.keyboard.press('Shift+Tab');
  const reverseWrapped = await visible[visible.length - 1].evaluate((el) => el === document.activeElement);
  const contained = await sheet.evaluate((el) => el.contains(document.activeElement));
  gate(forwardWrapped && reverseWrapped && contained, 'sheet-focus-escaped', { forwardWrapped, reverseWrapped, contained });
  return { forwardWrapped, reverseWrapped, contained };
}
async function mobileComposer(page, surface, channelURL) {
  await page.goto(channelURL + '?tab=settings', { waitUntil: 'domcontentloaded' });
  const trigger = page.getByRole('button', { name: prefix(TITLE) });
  await trigger.waitFor(); await settled(page);
  gate(await visibleLayers(page).count() === 0, 'overlay-already-open-before-profile-action');
  await trigger.click();
  const sheet = page.getByRole('dialog', { name: TITLE, exact: true });
  await sheet.waitFor(); await settled(page);
  const initial = await singleSheet(page, sheet);
  const form = sheet.getByRole('form', { name: TITLE, exact: true });
  const group = form.getByRole('radiogroup', { name: CHOOSE, exact: true });
  gate(await group.getByRole('radio').count() === 4, 'mobile-recipient-count-not-four');
  gate(await form.locator('textarea#vch-composer').count() === 1 && await form.getByRole('combobox').count() === 0,
    'quick-input-sheet-must-use-inline-recipients');
  const input = form.locator('#vch-composer');
  const selections = [];
  for (let i = 0; i < RECIPIENTS.length; i++) {
    const recipient = RECIPIENTS[i];
    await group.getByRole('radio', { name: recipient.name, exact: true }).click();
    // Immediate state read: intentionally no animation settle, polling or retry here.
    const state = await form.evaluate((el) => ({
      checked: el.querySelector('[role="radio"][aria-checked="true"]')?.textContent?.trim(),
      placeholder: el.querySelector('#vch-composer')?.getAttribute('placeholder'),
    }));
    gate(state.checked === recipient.name && state.placeholder?.endsWith(recipient.example), 'mobile-recipient-state-not-immediate', { index: i });
    await input.fill(`UI acceptance local draft ${i}`);
    await settled(page);
    selections.push({ index: i, ...await singleSheet(page, sheet), pill: await assertPill(group) });
  }
  await group.getByRole('radio', { name: RECIPIENTS[0].name, exact: true }).click();
  gate(await input.inputValue() === 'UI acceptance local draft 0', 'mobile-recipient-local-draft-lost');
  await settled(page);
  const controls = await auditTargets(form.locator('button, textarea'), true);
  const focus = await trapFocus(page, sheet);
  await page.keyboard.press('Escape');
  await sheet.waitFor({ state: 'detached' });
  gate(await trigger.evaluate((el) => el === document.activeElement), 'escape-focus-not-restored');
  gate(await visibleLayers(page).count() === 0, 'escape-left-overlay');
  await trigger.click(); await sheet.waitFor(); await settled(page);
  await singleSheet(page, sheet);
  // Click only a geometrically proven uncovered scrim point; never a guessed body coordinate.
  const scrim = page.locator('.m-overlay:has(.m-sheet) .m-scrim').last();
  const point = await scrim.evaluate((el) => {
    for (let y = 4; y < innerHeight; y += 8) for (let x = 4; x < innerWidth; x += 8) {
      if (document.elementFromPoint(x, y) === el) return { x, y };
    }
    return null;
  });
  gate(point, 'no-uncovered-sheet-scrim');
  await page.mouse.click(point.x, point.y);
  await sheet.waitFor({ state: 'detached' });
  gate(await visibleLayers(page).count() === 0, 'scrim-left-overlay');
  gate(await trigger.evaluate((el) => el === document.activeElement), 'scrim-focus-not-restored');
  return { semanticSurface: 'Profile → QuickInputSheet (implemented with shared Sheet)', initial, selections,
    controls, focus, escapeDismissed: true, scrimDismissed: true, focusRestoredBoth: true, localDraftRestored: true, saveClicks: 0 };
}
async function desktopComposer(page, channelURL) {
  await page.goto(channelURL, { waitUntil: 'domcontentloaded' });
  const form = page.getByRole('form', { name: TITLE, exact: true });
  await form.waitFor(); await settled(page);
  const trigger = form.getByRole('combobox', { name: CHOOSE, exact: true });
  const input = form.locator('input#vch-composer');
  const rows = [];
  for (let i = 0; i < RECIPIENTS.length; i++) {
    await trigger.click();
    const list = page.getByRole('listbox', { name: CHOOSE, exact: true });
    await list.waitFor(); await settled(page);
    const geometry = await inViewport(list);
    gate(geometry.inside && await list.getByRole('option').count() === 4 && await visibleLayers(page).count() === 1,
      'desktop-recipient-menu-invalid', { geometry });
    await list.getByRole('option', { name: RECIPIENTS[i].name, exact: true }).click();
    // One synchronous DOM snapshot after click: no expect retry, wait, blur or settle.
    const state = await form.evaluate((el) => ({
      expanded: el.querySelector('[role="combobox"]')?.getAttribute('aria-expanded'),
      prefix: el.querySelector('[role="combobox"]')?.textContent?.trim(),
      placeholder: el.querySelector('#vch-composer')?.getAttribute('placeholder'),
      overlayCount: document.querySelectorAll('[role="listbox"]').length,
    }));
    gate(state.expanded === 'false' && state.overlayCount === 0 && state.prefix?.includes(`Bàn với ${RECIPIENTS[i].name}`)
      && state.placeholder?.endsWith(RECIPIENTS[i].example), 'desktop-recipient-state-not-immediate', { index: i });
    await input.fill(`UI acceptance local draft ${i}`);
    rows.push({ index: i, immediateSelection: true, geometry });
  }
  await trigger.click();
  await page.getByRole('option', { name: RECIPIENTS[0].name, exact: true }).click();
  gate(await input.inputValue() === 'UI acceptance local draft 0', 'desktop-recipient-local-draft-lost');
  await trigger.click(); await page.getByRole('listbox', { name: CHOOSE, exact: true }).waitFor();
  await page.keyboard.press('Escape');
  gate(await page.getByRole('listbox', { name: CHOOSE, exact: true }).count() === 0, 'desktop-menu-escape-failed');
  gate(await trigger.evaluate((el) => el === document.activeElement), 'desktop-menu-focus-not-restored');
  await trigger.click();
  const list = page.getByRole('listbox', { name: CHOOSE, exact: true });
  await list.waitFor(); await settled(page);
  const outside = await list.evaluate((el) => {
    const b = el.getBoundingClientRect();
    for (let y = 8; y < innerHeight; y += 16) for (let x = 8; x < innerWidth; x += 16) {
      if (x >= b.left && x <= b.right && y >= b.top && y <= b.bottom) continue;
      const hit = document.elementFromPoint(x, y);
      if (hit && !hit.closest('button, a, input, textarea, select, [role="button"], [role="tab"], [role="radio"], [role="switch"], [role="option"], [role="menuitem"], [contenteditable]')) return { x, y };
    }
    return null;
  });
  gate(outside, 'desktop-no-safe-outside-point');
  await page.mouse.click(outside.x, outside.y);
  await list.waitFor({ state: 'detached' });
  return { rows, localDraftRestored: true, escapeDismissed: true, outsideDismissed: true, saveClicks: 0 };
}
async function runSurface(browser, surface, cookies) {
  const context = await browser.newContext({ viewport: { width: surface.width, height: surface.height },
    deviceScaleFactor: 1, isMobile: surface.mobile, hasTouch: surface.mobile,
    reducedMotion: surface.motion, serviceWorkers: 'block', acceptDownloads: false });
  try {
    await context.addCookies(cookies);
    // Applies to ALL origins and paths, not just /api/. No APIRequestContext is used:
    // browser-context routing does not intercept context.request calls.
    await context.route('**/*', async (route) => {
      const request = route.request(), method = request.method();
      if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
        report.blockedMutations.push({ surface: surface.key, method });
        return route.abort('blockedbyclient');
      }
      const url = new URL(request.url());
      if (!['data:', 'blob:'].includes(url.protocol) && url.origin !== BASE) {
        report.blockedExternalReads++; return route.abort('blockedbyclient');
      }
      if (/\/(download|export|generate|regenerate|run|execute|invoke|oauth|login|logout|callback)(?:\/|$)/i.test(url.pathname)) {
        report.blockedActionReads++; return route.abort('blockedbyclient');
      }
      return route.continue();
    });
    gate(typeof context.routeWebSocket === 'function', 'playwright-websocket-fence-unavailable');
    await context.routeWebSocket('**/*', (socket) => { report.blockedSockets++; socket.close(); });
    const page = await context.newPage(); page.setDefaultTimeout(15000); page.setDefaultNavigationTimeout(30000);
    page.on('pageerror', () => { report.pageErrorCount++; });
    page.on('download', (download) => { void download.cancel(); });
    page.on('popup', (popup) => { void popup.close(); });
    let channelID;
    await check(surface, 'authenticated-channel-discovery', async () => {
      await page.goto(BASE + '/#/library', { waitUntil: 'domcontentloaded' });
      const data = await page.evaluate(async () => {
        const response = await fetch('/api/owner-channels', { credentials: 'same-origin' });
        if (!response.ok) return { ok: false, status: response.status };
        const payload = await response.json();
        return { ok: true, id: payload.channels?.find(channel => channel.id !== 'new')?.id, count: payload.channels?.length || 0 };
      });
      gate(data.ok && typeof data.id === 'string' && data.count > 0, 'owner-channel-prerequisite-missing', { httpStatus: data.status, channelCount: data.count });
      channelID = data.id; return { authenticated: true, channelCount: data.count };
    });
    await check(surface, 'library-four-tabs-visible-and-switchable', async () => {
      await page.goto(BASE + '/#/library', { waitUntil: 'domcontentloaded' });
      const group = page.getByRole(surface.mobile ? 'radiogroup' : 'tablist', { name: 'Thư viện', exact: true });
      await group.waitFor(); await settled(page);
      const role = surface.mobile ? 'radio' : 'tab';
      gate(await group.getByRole(role).count() === 4, 'library-tab-count-not-four');
      const states = [];
      for (const [id, label] of LIBRARY) {
        const geometries = [];
        // Prove all four are visible at once before each selection, not scrolled into view one by one.
        for (const [, name] of LIBRARY) {
          const geometry = await inViewport(group.getByRole(role, { name: prefix(name) }));
          gate(geometry.inside && geometry.textInside && !geometry.truncated, 'library-tab-hidden-clipped-or-truncated', { id, geometry });
          geometries.push(geometry);
        }
        const tab = group.getByRole(role, { name: prefix(label) });
        await tab.click();
        await page.waitForURL((url) => (new URLSearchParams(url.hash.split('?')[1] || '').get('tab') || 'templates') === id, { timeout: 15000 });
        await expect(tab).toHaveAttribute(surface.mobile ? 'aria-checked' : 'aria-selected', 'true', { timeout: 15000 });
        await settled(page);
        states.push({ id, allFourGeometries: geometries, ...(surface.mobile ? { pill: await assertPill(group) } : {}) });
      }
      return { states, controls: await auditTargets(group.getByRole(role), surface.mobile) };
    });
    if (!channelID) return;
    const channelURL = BASE + '/#/channels/' + encodeURIComponent(channelID);
    const labels = surface.mobile ? ['Nguồn', 'Template', 'Hồ sơ'] : ['Nguồn & tự động', 'Template', 'Hồ sơ kênh'];
    for (const [i, id] of ['sources', 'templates', 'settings'].entries()) {
      await check(surface, `channel-${id}-tabs-and-form-hit-areas`, async () => {
        // Independent navigation keeps a failed panel audit from skipping the other panels.
        await page.goto(channelURL + `?tab=${id}`, { waitUntil: 'domcontentloaded' });
        const group = page.getByRole('tablist', { name: 'Trang kênh', exact: true });
        await group.waitFor(); await settled(page);
        gate(await group.getByRole('tab').count() === 3, 'channel-tab-count-not-three');
        const geometries = [];
        for (const label of labels) {
          const geometry = await inViewport(group.getByRole('tab', { name: prefix(label) }));
          gate(geometry.inside && geometry.textInside && !geometry.truncated, 'channel-tab-clipped', { geometry });
          geometries.push(geometry);
        }
        const tab = group.getByRole('tab', { name: prefix(labels[i]) });
        await tab.click();
        gate(await tab.getAttribute('aria-selected') === 'true', 'channel-tab-not-selected', { index: i });
        await settled(page);
        const panel = page.locator('#channel-panel');
        gate(await panel.getAttribute('aria-labelledby') === await tab.getAttribute('id'), 'channel-panel-tab-association');
        return {
          allThreeGeometries: geometries,
          tabs: await auditTargets(group.getByRole('tab'), surface.mobile),
          panel: await auditTargets(panel.locator('button, a[href], input:not([type="hidden"]), textarea, select, [role="switch"], [role="radio"]'), surface.mobile),
        };
      });
    }
    await check(surface, 'profile-font-ready-segmented-pill', async () => {
      await page.goto(channelURL + '?tab=settings', { waitUntil: 'domcontentloaded' });
      const group = page.locator('#channel-panel .kit-seg').first();
      await group.waitFor(); await settled(page);
      // Language buttons autosave. Do NOT click or keyboard-change them.
      return { pill: await assertPill(group), selectionChanged: false };
    });
    await check(surface, surface.mobile ? 'profile-quick-input-sheet' : 'desktop-composer-immediate-selection', () =>
      surface.mobile ? mobileComposer(page, surface, channelURL) : desktopComposer(page, channelURL));
  } finally { await context.close(); }
}
(async () => {
  let browser;
  try {
    const hostname = new URL(BASE).hostname;
    gate(['127.0.0.1', 'localhost', '[::1]'].includes(hostname), 'base-must-be-loopback');
    // Reuse the existing cookie FILE in memory, never duplicate or print its contents.
    const existing = await fs.readFile(COOKIE_SOURCE, 'utf8');
    const cookieFile = process.env.VIDERSE_QA_COOKIES || existing.match(/const COOKIES\s*=\s*'([^']+)'/)?.[1];
    gate(cookieFile, 'existing-cookie-path-unavailable');
    const saved = JSON.parse(await fs.readFile(cookieFile, 'utf8'));
    gate(Array.isArray(saved) && saved.length > 0, 'existing-cookie-array-unavailable');
    const cookies = saved.map(({ name, value, path: cookiePath, expires, httpOnly, sameSite }) => ({
      name, value, domain: hostname === '[::1]' ? '::1' : hostname, path: cookiePath || '/',
      ...(typeof expires === 'number' ? { expires } : {}), httpOnly: !!httpOnly,
      secure: new URL(BASE).protocol === 'https:', ...(sameSite ? { sameSite } : {}),
    }));
    browser = await chromium.launch({ executablePath: process.env.VIDERSE_QA_CHROME || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true });
    for (const surface of SURFACES) {
      await check(surface, 'surface-controller', async () => { await runSurface(browser, surface, cookies); return { viewport: `${surface.width}x${surface.height}`, motion: surface.motion }; });
    }
  } catch (error) {
    report.actions.push({ surface: 'controller', check: 'prerequisites', status: 'FAIL', reason: error instanceof GateError ? error.message : `runtime-${error?.name || 'Error'}` });
  } finally {
    // Browser closure is guaranteed even if setup, routing, a check, or context closure fails.
    if (browser) {
      try { await browser.close(); }
      catch { report.actions.push({ surface: 'controller', check: 'browser-close', status: 'FAIL', reason: 'browser-close-error' }); }
    }
    report.checked = report.actions.filter((r) => r.status === 'PASS').map((r) => `${r.surface}:${r.check}`);
    if (report.blockedMutations.length) report.actions.push({ surface: 'controller', check: 'zero-mutation-attempts', status: 'FAIL', reason: 'unexpected-mutation-attempt-blocked' });
    if (report.pageErrorCount) report.actions.push({ surface: 'controller', check: 'zero-page-errors', status: 'FAIL', reason: 'page-errors-observed' });
    report.status = report.actions.some((r) => r.status === 'FAIL') ? 'FAIL' : 'PASS';
    await fs.writeFile(path.join(ROOT, 'ui-acceptance-results.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(`${report.status}: ${report.actions.filter((r) => r.status === 'FAIL').length} failed checks; blocked mutation attempts: ${report.blockedMutations.length}`);
    if (report.status !== 'PASS') process.exitCode = 1;
  }
})();
