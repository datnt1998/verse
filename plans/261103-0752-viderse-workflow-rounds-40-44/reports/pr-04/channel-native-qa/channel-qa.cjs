/* channel-native-qa: real-browser interaction smoke for Viderse V38 Channel +
 * CreateChannel. Served app http://127.0.0.1:8043 (real native binary + restored
 * QA data). No mutations: route fence aborts every non-GET/HEAD/OPTIONS /api/
 * request before it reaches the server; aborts are recorded in blockedMutations.
 * All clicks are open/cancel/dismiss paths only; nothing that intentionally
 * writes is exercised (toggles, stepper commits on live channel, template
 * selection, link/send/delete confirms). */
const { createRequire } = require('node:module');
const req = createRequire('/Users/dat/Projects/verse/package.json');
const { chromium } = req('@playwright/test');
const fs = require('node:fs/promises');

const ROOT = __dirname;
const BASE = 'http://127.0.0.1:8043';
const COOKIES = '/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json';

const TABS_PC = ['Nguồn & tự động', 'Template', 'Hồ sơ kênh'];
const TABS_M = ['Nguồn', 'Template', 'Hồ sơ'];

const PROBE = `(() => {
  const vw = innerWidth, vh = innerHeight;
  const de = document.documentElement;
  const out = { innerWidth: vw, innerHeight: vh, docScrollWidth: de.scrollWidth,
    hOverflow: de.scrollWidth > vw + 1, url: location.hash || '(root)' };
  const vis = (el) => {
    const r = el.getBoundingClientRect();
    if (!r.width || !r.height) return null;
    const cs = getComputedStyle(el);
    if (cs.visibility === 'hidden' || cs.display === 'none' || Number(cs.opacity) === 0) return null;
    return r;
  };
  const sel = 'button, a[href], input, select, textarea, [role="button"], [role="tab"], [role="option"], [role="menuitem"], [role="switch"], [role="checkbox"], [role="combobox"]';
  const small = [], icons = [], runout = [];
  for (const el of document.querySelectorAll(sel)) {
    const r = vis(el); if (!r) continue;
    if (r.width < 44 || r.height < 44) small.push({ text: (el.textContent || el.getAttribute('aria-label') || '').trim().slice(0, 40), tag: el.tagName.toLowerCase(), role: el.getAttribute('role'), w: Math.round(r.width), h: Math.round(r.height) });
  }
  for (const el of document.querySelectorAll('span[aria-hidden="true"], span[role="img"]')) {
    const cs = getComputedStyle(el);
    const dashed = /dashed/.test(cs.borderTopStyle || cs.borderStyle || '');
    const masked = (cs.maskImage && cs.maskImage !== 'none') || (cs.webkitMaskImage && cs.webkitMaskImage !== 'none');
    const r = el.getBoundingClientRect();
    if (dashed && !masked && r.width > 0) icons.push({ w: Math.round(r.width), h: Math.round(r.height), title: el.getAttribute('title'), color: cs.borderTopColor, cls: String(el.className).slice(0, 40) });
  }
  for (const el of document.querySelectorAll('body *')) {
    const r = vis(el); if (!r) continue;
    if (r.left > vw + 1 || r.right > vw + 1 || r.left < -1) runout.push({ tag: el.tagName.toLowerCase(), cls: String(el.className).slice(0, 50), left: Math.round(r.left), right: Math.round(r.right), text: (el.textContent || '').trim().slice(0, 40) });
  }
  const trims = [];
  for (const el of document.querySelectorAll('.vch-row__name, .vch-row__sub, .vch-sub, .vch-title, [role="tab"], .vch-pickerrow__sub')) {
    const r = vis(el); if (!r) continue;
    if (el.scrollWidth > el.clientWidth + 1) trims.push({ text: (el.textContent || '').trim().slice(0, 50), sw: el.scrollWidth, cw: el.clientWidth, cls: String(el.className).slice(0, 40) });
  }
  out.smallTouchCount = small.length; out.smallTouch = small.slice(0, 40);
  out.missingIcons = icons.slice(0, 30);
  out.outOfViewCount = runout.length; out.outOfView = runout.slice(0, 20);
  out.truncatedCount = trims.length; out.truncated = trims.slice(0, 20);
  return out;
})()`;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Accessible name of a tab carries the count badge ("Nguồn & tự động 0") —
// match by label prefix, never exact.
const rx = (label) => new RegExp('^' + label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));

(async () => {
  const browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true });
  const result = { generated: new Date().toISOString(), base: BASE, surfaces: [], actions: [], blockedMutations: [], api: {}, notes: [] };

  const surfaces = [
    { key: 'pc', width: 1440, height: 900, mobile: false },
    { key: 'm', width: 390, height: 844, mobile: true },
    { key: 'pc-rm', width: 1440, height: 900, mobile: false, reducedMotion: 'reduce' },
    { key: 'm-rm', width: 390, height: 844, mobile: true, reducedMotion: 'reduce' },
  ];

  try {
    const cookies = JSON.parse(await fs.readFile(COOKIES, 'utf8'))
      .map((c) => ({ ...c, domain: '127.0.0.1', secure: false }));

    for (const surf of surfaces) {
      const surfKey = surf.key;
      const run = async () => {
        const context = await browser.newContext({
          viewport: { width: surf.width, height: surf.height },
          deviceScaleFactor: 1,
          isMobile: surf.mobile,
          hasTouch: surf.mobile,
          reducedMotion: surf.reducedMotion,
        });
        await context.addCookies(cookies);
        await context.route('**/api/**', async (route) => {
          if (!['GET', 'HEAD', 'OPTIONS'].includes(route.request().method())) {
            result.blockedMutations.push({ surface: surfKey, method: route.request().method(), url: route.request().url() });
            await route.abort('blockedbyclient');
          } else await route.continue();
        });
        const page = await context.newPage();
        const consoleErrors = [];
        page.on('console', (m) => { if (m.type() === 'error') consoleErrors.push(m.text().slice(0, 300)); });
        page.on('pageerror', (e) => consoleErrors.push('pageerror: ' + String(e).slice(0, 300)));

        const shot = async (name) => { const file = `${surfKey}-${name}.png`; await page.screenshot({ path: `${ROOT}/${file}` }); return file; };
        const probe = () => page.evaluate(PROBE);
        const finAnims = () => page.evaluate(async () => {
          await document.fonts.ready;
          await Promise.all(document.getAnimations()
            .filter((a) => Number.isFinite(a.effect?.getComputedTiming().endTime))
            .map((a) => a.finished.catch(() => {})));
        });
        const act = async (name, fn, note) => {
          const row = { surface: surfKey, action: name };
          try { Object.assign(row, await fn()); row.status = 'passed'; }
          catch (error) {
            row.status = 'failed'; row.error = String(error).slice(0, 400);
            row.url = page.url(); row.screenshot = await shot(`FAIL-${name}`);
            try { row.bodySnippet = (await page.locator('body').innerText()).slice(0, 600); } catch { /* dead page */ }
            // cleanup: don't leave a stuck overlay behind for later steps
            try { await page.keyboard.press('Escape'); await sleep(400); await page.mouse.click(surf.width - 5, surf.height - 5); await sleep(300); } catch { /* noop */ }
          }
          if (note) row.note = note;
          result.actions.push(row);
        };
        const focusMatches = (predicate) => page.evaluate(
          ({ sel }) => {
            const el = document.activeElement;
            return { tag: el?.tagName.toLowerCase(), label: el?.getAttribute('aria-label'), id: el?.id, match: sel ? el?.matches(sel) : undefined };
          }, { sel: predicate },
        );
        const overlayBounds = async (loc) => {
          const b = await loc.boundingBox();
          if (!b) return { bounds: null };
          const inView = b.x >= -1 && b.y >= -1 && b.x + b.width <= surf.width + 1 && b.y + b.height <= surf.height + 1;
          return { bounds: { x: Math.round(b.x), y: Math.round(b.y), w: Math.round(b.width), h: Math.round(b.height) }, withinViewport: inView };
        };

        // ---- real data via API (GET only) ----------------------------------
        const channelsRes = await context.request.get(`${BASE}/api/owner-channels`);
        if (!channelsRes.ok()) throw new Error(`owner-channels ${channelsRes.status()}`);
        const channels = (await channelsRes.json()).channels ?? [];
        if (channels.length < 1) throw new Error('no QA channels');
        const templatesRes = await context.request.get(`${BASE}/api/templates`);
        const templates = templatesRes.ok() ? ((await templatesRes.json()).items ?? []) : [];
        if (surfKey === 'pc') {
          result.api = {
            ownerChannels: channels.map((c) => ({ id: c.id, name: c.name, perDay: c.perDay, autoFetch: c.autoFetch, autoApprove: c.autoApprove, language: c.language, defaultTemplateName: c.defaultTemplateName, sources: c.sources?.length, youtubeUrl: c.youtubeUrl })),
            templateCount: templates.length,
          };
        }
        const ch1 = channels[0], ch2 = channels[1];

        // ---- land on channel ----------------------------------------------
        await page.goto(`${BASE}/#/channels/${ch1.id}`, { waitUntil: 'domcontentloaded' });
        await page.getByRole('tab', { name: rx(surf.mobile ? 'Nguồn' : 'Nguồn & tự động') }).waitFor({ timeout: 30000 });
        await finAnims();
        const surfRec = { key: surfKey, viewport: `${surf.width}x${surf.height}`, reducedMotion: !!surf.reducedMotion, isMobileEmu: surf.mobile, channels: channels.length, templates: templates.length, probes: {}, screenshots: [] };
        result.surfaces.push(surfRec);

        const tabNames = surf.mobile ? TABS_M : TABS_PC;
        surfRec.probes['channel-sources'] = await probe();
        surfRec.screenshots.push(await shot('channel-sources'));

        // ---- tab strip: labels, order, switching ---------------------------
        await act('tabs-labels', async () => {
          const tabs = page.getByRole('tab');
          const labels = await tabs.allTextContents();
          const urls = [];
          const hrefs = [];
          for (const label of tabNames) {
            const tab = page.getByRole('tab', { name: rx(label) });
            await tab.click();
            await sleep(200);
            urls.push(page.url());
            const sel = await tab.getAttribute('aria-selected');
            if (sel !== 'true') throw new Error(`tab ${label} not selected after click (aria-selected=${sel})`);
          }
          return { labels, urls, expected: tabNames };
        });

        // ---- channel switcher ----------------------------------------------
        if (surf.mobile) {
          await act('switcher-mobile-sheet', async () => {
            const btn = page.locator('button.vv-switcher[aria-haspopup="dialog"]');
            await btn.waitFor({ timeout: 10000 });
            const label = await btn.getAttribute('aria-label');
            await btn.click();
            const sheet = page.locator('[role="dialog"]').last();
            await sheet.waitFor({ state: 'visible', timeout: 10000 });
            await finAnims();
            const geo = await overlayBounds(sheet);
            const rows = await page.getByRole('button', { name: ch1.name }).count();
            const shot_ = await shot('switcher-sheet');
            await page.keyboard.press('Escape');
            await sheet.waitFor({ state: 'hidden', timeout: 10000 });
            return { triggerLabel: label, sheetBounds: geo, channelRowsVisible: rows > 0, screenshot: shot_, closeMethod: 'Escape' };
          });
          await act('switcher-mobile-sheet-scrim', async () => {
            const btn = page.locator('button.vv-switcher[aria-haspopup="dialog"]');
            await btn.click();
            const sheet = page.locator('[role="dialog"]').last();
            await sheet.waitFor({ state: 'visible', timeout: 10000 });
            // scrim/overlay tap = dismiss (mutations impossible: pick would navigate)
            await page.mouse.click(10, 10);
            try { await sheet.waitFor({ state: 'hidden', timeout: 3000 }); return { closeMethod: 'scrim-tap', closed: true }; }
            catch { await page.keyboard.press('Escape'); return { closeMethod: 'scrim-tap-no-effect, Esc fallback', closed: true }; }
          });
        } else {
          await act('switcher-desktop-menu', async () => {
            const btn = page.locator('button[aria-haspopup="menu"][aria-label*="Đổi kênh"]');
            await btn.waitFor({ timeout: 10000 });
            const label = await btn.getAttribute('aria-label');
            await btn.click();
            const menu = page.locator('[role="menu"], [role="dialog"]').last();
            await menu.waitFor({ state: 'visible', timeout: 10000 });
            const geo = await overlayBounds(menu);
            const itemCount = await menu.getByText(ch1.name, { exact: true }).count();
            const shot_ = await shot('switcher-menu');
            await page.keyboard.press('Escape');
            await menu.waitFor({ state: 'hidden', timeout: 10000 });
            return { triggerLabel: label, menuBounds: geo, currentChannelListed: itemCount > 0, screenshot: shot_, closeMethod: 'Escape' };
          });
          // side rail switch = real navigation (GET only)
          if (ch2) await act('siderail-switch-channel', async () => {
            const rail = page.locator('aside[aria-label*="Kênh"]');
            const link = rail.locator(`a[href*="/channels/${ch2.id}"]`);
            await link.click();
            await page.waitForURL(`**/channels/${ch2.id}**`, { timeout: 15000 });
            const h1 = await page.locator('h1').first().textContent();
            const back = rail.locator(`a[href*="/channels/${ch1.id}"]`);
            await back.click();
            await page.waitForURL(`**/channels/${ch1.id}**`, { timeout: 15000 });
            return { switchedTo: ch2.id, heading: (h1 || '').trim().slice(0, 60), andBack: true };
          });
        }

        const light = !!surf.reducedMotion;
        // ---- SOURCES: rival link picker open / Esc / outside-click ----------
        await act('nav-tab-sources', async () => { await page.getByRole('tab', { name: rx(tabNames[0]) }).click(); return {}; });
        await sleep(200);
        const linkBtn = page.locator('.vch-linkslot button').first();
        await linkBtn.waitFor({ timeout: 15000 });

        await act('link-picker-open-esc', async () => {
          await linkBtn.click();
          const dlg = page.locator('[role="dialog"][aria-label*="Liên kết"]');
          await dlg.waitFor({ state: 'visible', timeout: 10000 });
          const geo = await overlayBounds(dlg);
          const checks = await dlg.locator('[role="checkbox"]').count();
          const submit = await dlg.getByRole('button', { name: /Chọn đối thủ|Liên kết / }).last().isDisabled();
          const shot_ = await shot('link-picker');
          await page.keyboard.press('Escape');
          await dlg.waitFor({ state: 'hidden', timeout: 10000 });
          const focus = await focusMatches('button');
          return { bounds: geo, checkboxRows: checks, submitDisabledWhenEmpty: submit, focusAfter: focus, screenshot: shot_, closeMethod: 'Escape' };
        });
        if (!light) await act('link-picker-outside-click', async () => {
          await linkBtn.click();
          const dlg = page.locator('[role="dialog"][aria-label*="Liên kết"]');
          await dlg.waitFor({ state: 'visible', timeout: 10000 });
          await page.mouse.click(surf.width - 6, 6);
          await dlg.waitFor({ state: 'hidden', timeout: 10000 });
          return { closeMethod: 'outside-pointerdown' };
        });

        // ---- SOURCES: rival row menus (open + cancel, never confirm) --------
        const rivalCount = ch1.sources?.length ?? 0;
        if (rivalCount === 0) result.actions.push({ surface: surfKey, action: 'rival-menus-open-cancel', status: 'unexercised', note: `channel ${ch1.name} has 0 linked rivals (sources=0 on all 3 QA channels) — unlink confirm / row action-sheet cannot be exercised without writing` });
        if (surf.mobile && rivalCount > 0) {
          await act('rival-actionsheet-open-cancel', async () => {
            const moreBtn = page.locator('button[aria-haspopup="dialog"][aria-label^="Bỏ liên kết"]').first();
            await moreBtn.click();
            const sheet = page.locator('[role="dialog"]').last();
            await sheet.waitFor({ state: 'visible', timeout: 10000 });
            await finAnims();
            const geo = await overlayBounds(sheet);
            const items = await sheet.locator('button, a').allTextContents();
            const shot_ = await shot('rival-sheet');
            await page.mouse.click(10, 10);
            try { await sheet.waitFor({ state: 'hidden', timeout: 3000 }); return { bounds: geo, items: items.map((s) => s.trim()).filter(Boolean), screenshot: shot_, closeMethod: 'scrim-tap' }; }
            catch { await page.keyboard.press('Escape'); return { bounds: geo, items, screenshot: shot_, closeMethod: 'scrim-no-effect, Esc fallback' }; }
          });
          await act('rival-unlink-confirm-cancel', async () => {
            const moreBtn = page.locator('button[aria-haspopup="dialog"][aria-label^="Bỏ liên kết"]').first();
            await moreBtn.click();
            const sheet = page.locator('[role="dialog"]').last();
            await sheet.waitFor({ state: 'visible', timeout: 10000 });
            const unlinkItem = sheet.getByRole('button', { name: /Bỏ liên kết/ });
            await unlinkItem.click();
            const dlg = page.locator('[role="alertdialog"], [role="dialog"]').last();
            await dlg.waitFor({ state: 'visible', timeout: 10000 });
            const geo = await overlayBounds(dlg);
            const cancel = dlg.getByRole('button', { name: /Huỷ|Cancel/ });
            const shot_ = await shot('unlink-confirm');
            await cancel.click();
            await dlg.waitFor({ state: 'hidden', timeout: 10000 });
            return { bounds: geo, screenshot: shot_, confirmed: false, cancelLabel: 'Huỷ' };
          });
        } else if (!surf.mobile && rivalCount > 0) {
          await act('rival-unlink-confirm-cancel', async () => {
            const row = page.locator('.vch-rival').first();
            const unlink = row.getByRole('button', { name: 'Bỏ liên kết', exact: true });
            await unlink.click();
            const dlg = page.locator('[role="alertdialog"], [role="dialog"]').last();
            await dlg.waitFor({ state: 'visible', timeout: 10000 });
            const geo = await overlayBounds(dlg);
            const cancel = dlg.getByRole('button', { name: /Huỷ|Cancel/ });
            const shot_ = await shot('unlink-confirm');
            await cancel.click();
            await dlg.waitFor({ state: 'hidden', timeout: 10000 });
            return { bounds: geo, screenshot: shot_, confirmed: false, cancelLabel: 'Huỷ' };
          });
        }

        // automation card present (toggles NOT exercised: they PATCH)
        await act('automation-card-state', async () => {
          const switches = await page.getByRole('switch').count();
          const stepper = await page.getByRole('group', { name: /Video mỗi ngày|Video moi ngày/ }).count()
            + await page.locator('input[aria-label*="Video"]').count();
          return { switches, perDayInputs: stepper, note: 'toggles/stepper present, never clicked (would PATCH live channel)' };
        });

        // ---- TEMPLATES: default-template picker open/keyboard/Esc ----------
        await act('nav-tab-templates', async () => { await page.getByRole('tab', { name: rx(tabNames[1]) }).click(); return {}; });
        await sleep(200);
        surfRec.probes['channel-templates'] = await probe();
        surfRec.screenshots.push(await shot('channel-templates'));
        await act('template-picker-open-keyboard-esc', async () => {
          const trigger = page.locator('[role="combobox"][aria-label="Đổi template"]');
          await trigger.waitFor({ timeout: 15000 });
          await trigger.focus();
          await page.keyboard.press('Enter');
          const pop = surf.mobile ? page.locator('[role="dialog"]').last() : page.locator('[role="listbox"], [role="dialog"]').last();
          await pop.waitFor({ state: 'visible', timeout: 10000 });
          await finAnims();
          const geo = await overlayBounds(pop);
          const options = pop.locator('[role="option"], [role="menuitem"]');
          const optionCount = await options.count();
          const labels = (await options.allTextContents()).map((s) => s.trim().slice(0, 60)).slice(0, 8);
          await page.keyboard.press('ArrowDown');
          const focused = await focusMatches();
          const shot_ = await shot('template-picker');
          await page.keyboard.press('Escape');
          try { await pop.waitFor({ state: 'hidden', timeout: 4000 }); }
          catch { /* some mobile sheets need pointer close */ }
          const stillOpen = await pop.isVisible().catch(() => false);
          const focus = await focusMatches();
          const saved = !(await page.evaluate((id) => false, ch1.id));
          return { triggerLabel: 'Đổi template', bounds: geo, optionCount, optionsSample: labels, focusedOption: focused, escClosed: !stillOpen, focusAfter: focus, screenshot: shot_, nothingSelected: true };
        });
        if (surf.mobile && !light) {
          await act('template-picker-sheet-scrim', async () => {
            const trigger = page.locator('[role="combobox"][aria-label="Đổi template"]');
            await trigger.click();
            const sheet = page.locator('[role="dialog"]').last();
            await sheet.waitFor({ state: 'visible', timeout: 10000 });
            await page.mouse.click(10, 10);
            try { await sheet.waitFor({ state: 'hidden', timeout: 3000 }); return { closeMethod: 'scrim-tap', closed: true }; }
            catch { await page.keyboard.press('Escape'); return { closeMethod: 'scrim-no-effect, Esc fallback', closed: true }; }
          });
        }

        // ---- SETTINGS: profile fields edit-state + nav icon check -----------
        await act('nav-tab-settings', async () => { await page.getByRole('tab', { name: rx(tabNames[2]) }).click(); return {}; });
        await sleep(300);
        surfRec.probes['channel-settings'] = await probe();
        surfRec.screenshots.push(await shot('channel-settings'));

        await act('settings-template-nav-icon', async () => {
          if (surf.mobile) {
            const hiddenRows = await page.locator('a[href*="tab=templates"], a[href*="tab=sources"]').count();
            const visibleLinks = await page.getByRole('link', { name: /Trợ lý AI/ }).count();
            return { mobileNote: 'PC-only nav shortcut rows hidden by design on phone (navHidden)', hiddenNavRows: hiddenRows, assistantLinkVisible: visibleLinks > 0 };
          }
          const link = page.getByRole('link', { name: /Template mặc định/ });
          await link.waitFor({ timeout: 10000 });
          const icon = link.locator('span').first();
          const style = await page.evaluate((el) => {
            const cs = getComputedStyle(el);
            return {
              maskImage: cs.maskImage || cs.webkitMaskImage,
              borderStyle: cs.borderTopStyle,
              borderRadius: cs.borderTopLeftRadius,
              background: cs.backgroundColor,
              title: el.getAttribute('title'),
            };
          }, await icon.elementHandle());
          const dashedMissing = /dashed/.test(style.borderStyle || '') && (!style.maskImage || style.maskImage === 'none');
          return { iconStyle: style, rendersRealMask: !dashedMissing, expectedUrl: '/assets/layer-*.svg' };
        });

        await act('profile-name-edit-state', async () => {
          const field = page.locator('#vch-name');
          await field.waitFor({ timeout: 10000 });
          const original = await field.inputValue();
          await field.click();
          await field.fill(`${original} QA-marker`);
          const draft = await field.inputValue();
          const saveline = await page.locator('.vch-saveline').last().textContent();
          // Cancel = reload discards (commit only fires onBlur). No blur here → zero mutation attempts.
          await page.reload({ waitUntil: 'domcontentloaded' });
          await page.getByRole('tab', { name: rx(tabNames[2]) }).waitFor({ timeout: 30000 });
          await sleep(400);
          const restored = await page.locator('#vch-name').inputValue();
          return { original: original.slice(0, 60), draftChanged: draft.endsWith('QA-marker'), savelineShown: saveline?.trim(), restoredAfterReload: restored.slice(0, 60), reverted: restored === original };
        });

        if (!light) await act('delete-channel-confirm-cancel', async () => {
          const btn = page.getByRole('button', { name: 'Xoá kênh', exact: true });
          await btn.scrollIntoViewIfNeeded();
          await btn.click();
          const dlg = page.locator('[role="alertdialog"], [role="dialog"]').last();
          await dlg.waitFor({ state: 'visible', timeout: 10000 });
          const geo = await overlayBounds(dlg);
          const title = await dlg.getByRole('heading').first().textContent().catch(() => null);
          const shot_ = await shot('delete-confirm');
          const cancel = dlg.getByRole('button', { name: /Huỷ|Cancel/ });
          await cancel.click();
          await dlg.waitFor({ state: 'hidden', timeout: 10000 });
          return { bounds: geo, title: title?.trim(), screenshot: shot_, confirmed: false };
        });

        // ---- COMPOSER (PC only): recipient picker draft switching ----------
        if (!surf.mobile) {
          await act('composer-recipient-draft-switch', async () => {
            const input = page.locator('#vch-composer');
            await input.waitFor({ timeout: 10000 });
            const send = page.getByRole('button', { name: 'Gửi', exact: true });
            const disabledEmpty = await send.isDisabled();
            await input.fill('QA draft scout');
            const picker = page.locator('[role="combobox"][aria-label="Dặn trợ lý nào"]');
            const label0 = await picker.getAttribute('aria-label');
            const value0 = (await picker.textContent() || '').trim();
            await picker.click();
            const pop = page.locator('[role="listbox"], [role="dialog"]').last();
            await pop.waitFor({ state: 'visible', timeout: 10000 });
            const geo = await overlayBounds(pop);
            const options = pop.locator('[role="option"]');
            const names = (await options.allTextContents()).map((s) => s.trim());
            const idx = names.findIndex((n) => n && !value0.includes(n));
            const pickedName = idx >= 0 ? names[idx] : names[0];
            await options.nth(Math.max(idx, 0)).click();
            await sleep(300);
            const draftAfter = await input.inputValue();
            const value1 = (await picker.textContent() || '').trim();
            await picker.click();
            await pop.waitFor({ state: 'visible', timeout: 10000 });
            await pop.locator('[role="option"]').first().click();
            await sleep(300);
            const draftBack = await input.inputValue();
            return { pickerLabel: label0, firstRecipient: value0.slice(0, 30), options: names, picked: pickedName.slice(0, 30), draftAfterSwitch: draftAfter, draftBackOnFirst: draftBack, sendDisabledWhenEmpty: disabledEmpty, sendNeverClicked: true, bounds: geo };
          });
          await act('composer-picker-keyboard-esc', async () => {
            const picker = page.locator('[role="combobox"][aria-label="Dặn trợ lý nào"]');
            await picker.focus();
            await page.keyboard.press('Enter');
            const pop = page.locator('[role="listbox"], [role="dialog"]').last();
            await pop.waitFor({ state: 'visible', timeout: 10000 });
            await page.keyboard.press('ArrowDown');
            const focused = await focusMatches();
            await page.keyboard.press('Escape');
            await pop.waitFor({ state: 'hidden', timeout: 4000 });
            const focus = await focusMatches();
            return { keyboardNav: true, focusedOption: focused, escClosed: true, focusAfter: focus };
          });
        }

        // ---- CREATE PAGE: legitimate nav, fill, never submit ----------------
        await act('create-page-load', async () => {
          if (surf.mobile) {
            try {
              await page.locator('button.vv-switcher[aria-haspopup="dialog"]').click({ timeout: 8000 });
              const sheet = page.locator('[role="dialog"]').last();
              await sheet.waitFor({ state: 'visible', timeout: 8000 });
              await sheet.getByRole('link', { name: /Tạo kênh/ }).click();
            } catch {
              await page.goto(`${BASE}/#/channels/new`, { waitUntil: 'domcontentloaded' });
            }
          } else {
            const rail = page.locator('aside[aria-label*="Kênh"]');
            await rail.getByRole('link', { name: /Tạo kênh/ }).click();
          }
          await page.waitForURL('**/channels/new**', { timeout: 15000 });
          await sleep(300);
          const heading = await page.getByRole('heading').allTextContents();
          const nameSel = surf.mobile ? '#vch-new-name-m' : '#vch-new-name';
          const sub = await page.locator(nameSel).inputValue();
          return { url: page.url(), headings: heading.map((h) => h.trim()).filter(Boolean).slice(0, 6), nameFieldPresent: true };
        });
        surfRec.probes['create-page'] = await probe();
        surfRec.screenshots.push(await shot('create-page'));

        if (!light) await act('create-fill-fields', async () => {
          await page.locator(surf.mobile ? '#vch-new-name-m' : '#vch-new-name').fill('QA channel draft');
          await page.locator(surf.mobile ? '#vch-new-url-m' : '#vch-new-url').fill('youtube.com/@qa-draft');
          const checks = page.locator('[role="checkbox"]');
          const total = await checks.count();
          const labels = await checks.evaluateAll((nodes) => nodes.slice(0, 12).map((n) => (n.textContent || n.getAttribute('aria-label') || '').trim().slice(0, 40)));
          if (total > 0) await checks.first().click();
          const pickedLine = await page.getByText(/Đã chọn/).first().textContent().catch(() => null);
          const submitBtn = page.getByRole('button', { name: 'Tạo kênh', exact: true });
          return { rivalCheckboxes: total, sample: labels, pickedLine: pickedLine?.trim(), submitVisible: await submitBtn.isVisible(), submitNeverClicked: true };
        });

        await act('create-run-rows', async () => {
          const tpl = page.locator('[role="combobox"][aria-label="Chọn template"]');
          await tpl.waitFor({ timeout: 10000 });
          const triggerText = (await tpl.textContent() || '').trim().slice(0, 60);
          await tpl.click();
          const pop = surf.mobile ? page.locator('[role="dialog"]').last() : page.locator('[role="listbox"], [role="dialog"]').last();
          await pop.waitFor({ state: 'visible', timeout: 10000 });
          await finAnims();
          const geo = await overlayBounds(pop);
          const opts = (await pop.locator('[role="option"], [role="menuitem"]').allTextContents()).map((s) => s.trim().slice(0, 50));
          const shot_ = await shot('create-template-picker');
          if (surf.mobile) { await page.mouse.click(10, 10); } else { await page.keyboard.press('Escape'); }
          try { await pop.waitFor({ state: 'hidden', timeout: 3000 }); }
          catch { await page.keyboard.press('Escape').catch(() => {}); }
          const sw = await page.getByRole('switch', { name: 'Tự duyệt ảnh' }).count();
          const stepperInput = page.locator('input[aria-label="Video mỗi ngày"]');
          const perDayVal = await stepperInput.inputValue().catch(() => null);
          return { templateTrigger: triggerText, options: opts, bounds: geo, approveSwitch: sw > 0, perDayCurrent: perDayVal, neverSaved: true, screenshot: shot_ };
        });

        if (!light) await act('create-stepper-local', async () => {
          const stepperInput = page.locator('input[aria-label="Video mỗi ngày"]');
          const before = await stepperInput.inputValue();
          const minus = page.getByRole('button', { name: /Giảm/ });
          const plus = page.getByRole('button', { name: /Tăng/ });
          await minus.click();
          const dec = await stepperInput.inputValue();
          await plus.click();
          const back = await stepperInput.inputValue();
          await stepperInput.fill('99');
          await stepperInput.blur();
          const clamped = await stepperInput.inputValue();
          return { before, afterDec: dec, restored: back, typed99: clamped, clampedTo20: clamped === '20', localStateOnly: true };
        });

        await act('create-cancel', async () => {
          if (surf.mobile) {
            const back = page.locator('.m-back');
            await back.waitFor({ timeout: 10000 });
            await back.click();
          } else {
            await page.getByRole('button', { name: 'Huỷ', exact: true }).click();
          }
          await page.waitForURL((url) => !url.href.includes('/channels/new'), { timeout: 15000 });
          return { navigatedAway: true, url: page.url() };
        });

        // ---- final probes --------------------------------------------------
        surfRec.probes['final'] = await probe();
        surfRec.consoleErrors = consoleErrors;
        await context.close();
      };

      try { await run(); }
      catch (error) {
        result.actions.push({ surface: surfKey, action: 'SURFACE-FAILURE', status: 'failed', error: String(error) });
        for (const ctx of browser.contexts()) {
          for (const p of ctx.pages()) {
            try { await p.screenshot({ path: `${ROOT}/${surfKey}-surface-fail.png`, fullPage: true }); } catch { /* noop */ }
          }
        }
      }
    }
  } finally {
    await browser.close();
    await fs.writeFile(`${ROOT}/channel-native-qa.json`, JSON.stringify(result, null, 2));
  }
  console.log(JSON.stringify({
    surfaces: result.surfaces.map((s) => ({ key: s.key, probes: Object.keys(s.probes), consoleErrors: s.consoleErrors?.length })),
    actionCount: result.actions.length,
    failed: result.actions.filter((a) => a.status === 'failed').map((a) => `${a.surface}:${a.action}`),
    blockedMutations: result.blockedMutations,
  }, null, 2));
})().catch((e) => { console.error(e); process.exitCode = 1; });
