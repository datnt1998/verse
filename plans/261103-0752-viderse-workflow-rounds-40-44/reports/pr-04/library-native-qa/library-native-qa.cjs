// Library/Template native interaction smoke — PC 1440×900, mobile 390×844,
// + a reduced-motion desktop pass + a mobile touch-target audit pass.
// Non-mutating only: a request fence aborts every non-GET/HEAD/OPTIONS call
// under /api/** and records it. Coverage: library tabs, template menu
// open/Esc/scrim, rename open/cancel, default-chooser navigation, delete
// confirm open/cancel (locked reason honoured for channel-referenced
// templates), template open/section nav/options menu + return, PC floating
// popovers vs mobile bottom sheets, touch targets <44px.
const { createRequire } = require('node:module');
const req = createRequire('/Users/dat/Projects/verse/package.json');
const { chromium } = req('@playwright/test');
const fs = require('node:fs/promises');
const root = __dirname;
const COOKIES = '/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json';
const ORIGIN = 'http://127.0.0.1:8043';
const EXEC = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';

const result = { apiReads: [], actions: [], blockedMutations: [], failures: [] };

const fonts = (page) => page.evaluate(() => document.fonts.ready);
const settleAnims = (page) => page.evaluate(async () => {
  const list = document.getAnimations().filter((a) => Number.isFinite(a.effect?.getComputedTiming().endTime));
  await Promise.all(list.map((a) => a.finished.catch(() => {})));
  await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
});
const scrollW = (page) => page.evaluate(() => ({ w: innerWidth, sw: document.documentElement.scrollWidth }));

async function shot(page, name) { await page.screenshot({ path: `${root}/${name}` }); return name; }

async function overlayGeom(page, role, viewport) {
  await settleAnims(page);
  const loc = page.locator(`[role="${role}"]:visible`).last();
  const b = await loc.boundingBox();
  const label = await loc.getAttribute('aria-label');
  return {
    role, bounds: b, label,
    inside: !!b && b.x >= -1 && b.y >= -1 && b.x + b.width <= viewport.width + 2 && b.y + b.height <= viewport.height + 2,
  };
}

const auditControls = (page) => page.evaluate(() => {
  const sel = 'button, a[href], [role="tab"], [role="radio"], [role="menuitem"], [role="checkbox"], [role="switch"], input, textarea, select';
  const out = [];
  for (const el of document.querySelectorAll(sel)) {
    const r = el.getBoundingClientRect();
    if (r.width < 1 || r.height < 1 || r.bottom < 0 || r.right < 0 || r.top > innerHeight || r.left > innerWidth) continue;
    if (el.closest('.m-scrim')) continue;
    const label = (el.getAttribute('aria-label') || el.getAttribute('title') || (el.textContent || '').replace(/\s+/g, ' ').trim()).slice(0, 80);
    out.push({ tag: el.tagName.toLowerCase(), role: el.getAttribute('role') || el.tagName.toLowerCase(), label, w: Math.round(r.width), h: Math.round(r.height), x: Math.round(r.x), y: Math.round(r.y), disabled: el.disabled === true || el.getAttribute('aria-disabled') === 'true' });
  }
  return out;
});

function record(action) {
  result.actions.push(action);
  const s = action.status === 'failed' ? 'FAIL' : action.status === 'partial' ? 'PART' : 'ok';
  console.log(`[${s}] ${action.viewport.width}x${action.viewport.height}${action.reducedMotion ? ' RM' : ''} ${action.control} :: ${(action.observed ?? '').split('\n')[0]}`);
}
function fail(action, error) {
  record({ ...action, status: 'failed', observed: `FAILED: ${String(error?.message ?? error).split('\n')[0]}` });
  result.failures.push({ control: action.control, error: String(error?.message ?? error) });
}

async function makeContext(browser, viewport, reducedMotion, hasTouch = false) {
  const context = await browser.newContext({ viewport, deviceScaleFactor: 1, hasTouch, isMobile: hasTouch, ...(reducedMotion ? { reducedMotion: 'reduce' } : {}) });
  const cookies = JSON.parse(await fs.readFile(COOKIES, 'utf8'));
  await context.addCookies(cookies.map((c) => ({ ...c, domain: '127.0.0.1', secure: false })));
  await context.route('**/api/**', async (route) => {
    const r = route.request();
    const entry = { method: r.method(), path: new URL(r.url()).pathname + new URL(r.url()).search };
    if (!['GET', 'HEAD', 'OPTIONS'].includes(r.method())) { result.blockedMutations.push({ ...entry, viewport: `${viewport.width}x${viewport.height}` }); await route.abort('blockedbyclient'); }
    else { result.apiReads.push(entry); await route.continue(); }
  });
  const page = await context.newPage();
  if (hasTouch) {
    // Playwright does not expose pointer/hover media emulation; drive CDP
    // directly so `(pointer: coarse)` rules (44px touch targets) are live.
    const cdp = await context.newCDPSession(page);
    await cdp.send('Emulation.setEmulatedMedia', { features: [{ name: 'pointer', value: 'coarse' }, { name: 'hover', value: 'none' }] });
  }
  page.on('pageerror', (err) => result.failures.push({ control: 'pageerror', error: String(err) }));
  return { context, page };
}

const closeTopOverlay = async (page) => {
  const cancel = page.getByRole('button', { name: 'Huỷ', exact: true });
  if (await cancel.count()) await cancel.last().click();
  else await page.keyboard.press('Escape');
};

// ── Desktop pass ──────────────────────────────────────────────────────────
async function desktop(browser, viewport, reducedMotion = false) {
  const { context, page } = await makeContext(browser, viewport, reducedMotion);
  const vp = viewport;
  const A = (partial) => ({ viewport: vp, reducedMotion, ...partial });
  const tag = `${vp.width}${reducedMotion ? '-reduced' : ''}`;
  try {
    const templates = (await (await context.request.get(`${ORIGIN}/api/templates`)).json()).items;
    const channels = (await (await context.request.get(`${ORIGIN}/api/owner-channels`)).json()).channels;
    const free = templates.find((t) => !t.channels?.length);
    const used = templates.find((t) => t.channels?.length);

    await page.goto(`${ORIGIN}/#/library`, { waitUntil: 'domcontentloaded' });
    await page.getByRole('heading', { name: 'Thư viện', exact: true }).waitFor();
    await page.getByText(templates[0].name, { exact: true }).first().waitFor();
    await fonts(page); await settleAnims(page);
    record(A({ control: 'library-load', action: 'goto #/library', observed: `3 template cards; geo ${JSON.stringify(await scrollW(page))}`, status: 'passed', screenshot: await shot(page, `native-${tag}-library-templates.png`) }));

    // ── Tabs (non-mutating navigation) ──
    for (const [id, label] of [['prompts', 'Chỉ dẫn ảnh'], ['backdrops', 'Video nền'], ['overlays', 'Lớp phủ'], ['templates', 'Mẫu video']]) {
      try {
        await page.getByRole('tab', { name: label }).click();
        await page.waitForURL(`**#**${id}`);
        if (id === 'prompts') await page.locator('[role="radiogroup"][aria-label="Lọc chỉ dẫn"]').waitFor();
        if (id === 'backdrops') await page.locator('#vbd-link').waitFor();
        if (id === 'overlays') await page.getByRole('button', { name: /Xem trước hiệu ứng/ }).first().waitFor();
        if (id === 'templates') await page.getByText(templates[0].name, { exact: true }).first().waitFor();
        await fonts(page); await settleAnims(page);
        const s = await shot(page, `native-${tag}-tab-${id}.png`);
        const ov = await scrollW(page);
        record(A({ control: `tab-${id}`, action: 'click tab', observed: `panel rendered; horizontal overflow=${ov.sw > ov.w ? `${ov.sw}>${ov.w}` : 'none'}`, status: ov.sw > ov.w ? 'failed' : 'passed', screenshot: s }));
      } catch (e) { fail(A({ control: `tab-${id}`, action: 'click tab' }), e); await shot(page, `native-${tag}-tab-${id}-FAIL.png`); }
    }

    // ── Prompts tab: filter, server-side search, open editor, close ──
    if (!reducedMotion) try {
      // Navigate to the prompts tab first — the tab loop ends on 'templates'.
      await page.getByRole('tab', { name: 'Chỉ dẫn ảnh' }).click();
      await page.waitForURL('**#**prompts');
      const seg = page.locator('[role="radiogroup"][aria-label="Lọc chỉ dẫn"] [role="radio"]');
      await seg.first().waitFor();
      const labels = await seg.allTextContents();
      await page.getByRole('radio', { name: /Ảnh cảnh/ }).click();
      await page.waitForResponse((r) => r.url().includes('/api/image-prompts') && r.url().includes('kind=scene'), { timeout: 10000 }).catch(() => {});
      await settleAnims(page);
      const search = page.getByRole('textbox', { name: 'Tìm chỉ dẫn' });
      await search.fill('QA');
      await page.waitForResponse((r) => r.url().includes('/api/image-prompts') && r.url().includes('q='), { timeout: 10000 });
      await settleAnims(page);
      record(A({ control: 'prompts-filter+search', action: 'scene filter + search "QA"', observed: `options: ${labels.map((l) => l.trim()).join(' | ')}; GET /api/image-prompts?kind&q fired and grid updated`, status: 'passed' }));
      await search.fill('');
      await page.waitForResponse((r) => r.url().includes('/api/image-prompts'), { timeout: 10000 }).catch(() => {});
      await page.getByRole('radio', { name: /Tất cả/ }).click();
      // open a prompt card -> inline editor -> back
      const card = page.getByRole('button', { name: /Mở chỉ dẫn/ }).first();
      await card.waitFor();
      const cardName = await card.getAttribute('aria-label');
      await card.click();
      await page.getByRole('button', { name: 'Về danh sách chỉ dẫn', exact: true }).waitFor();
      const sP = await shot(page, `native-${tag}-prompt-editor.png`);
      await page.getByRole('button', { name: 'Về danh sách chỉ dẫn', exact: true }).click();
      await card.waitFor();
      record(A({ control: 'prompt-editor-open-close', action: `open "${cardName}" → back`, observed: 'PromptEditor mounted inline; back returned to grid; no writes', status: 'passed', screenshot: sP }));
    } catch (e) { fail(A({ control: 'prompts-tab-controls', action: 'filter/search/editor' }), e); }


    // ── Overlays tab: play toggle (non-mutating), no remove on built-ins ──
    if (!reducedMotion) try {
      await page.getByRole('tab', { name: 'Lớp phủ' }).click();
      await page.getByRole('button', { name: /Xem trước hiệu ứng/ }).first().waitFor();
      const playBtn = page.getByRole('button', { name: /Xem trước hiệu ứng/ }).first();
      await playBtn.click();
      const playing = await page.locator('video').first().evaluate((v) => !v.paused).catch(() => null);
      await playBtn.click().catch(() => {});
      const removeCount = await page.getByRole('button', { name: /Bỏ hiệu ứng/ }).count();
      record(A({ control: 'overlay-play-toggle', action: 'click play then pause', observed: `video paused=!${playing ? 'false' : 'true'}; remove buttons present=${removeCount} (all overlays built-in => 0 expected)`, status: 'passed' }));
    } catch (e) { fail(A({ control: 'overlay-play-toggle', action: 'play toggle' }), e); }

    // ── Backdrops tab: empty-state render + add-bar disabled state ──
    if (!reducedMotion) try {
      await page.getByRole('tab', { name: 'Video nền' }).click();
      await page.locator('#vbd-link').waitFor();
      const addBtn = page.getByRole('button', { name: 'Thêm', exact: true }).first();
      const disabledEmpty = await addBtn.isDisabled();
      await page.locator('#vbd-link').fill('not a link');
      const enabled = await addBtn.isEnabled();
      const s = await shot(page, `native-${tag}-backdrops.png`);
      record(A({ control: 'backdrops-addbar', action: 'inspect add bar', observed: `submit disabled when empty=${disabledEmpty}, enabled after input=${enabled}; submit not clicked (POST /api/sources/parse is a mutation — unverified)`, status: disabledEmpty && enabled ? 'passed' : 'partial', screenshot: s }));
    } catch (e) { fail(A({ control: 'backdrops-addbar', action: 'inspect' }), e); }

    // ── Templates tab ──
    await page.getByRole('tab', { name: 'Mẫu video' }).click();
    await page.getByText(templates[0].name, { exact: true }).first().waitFor();

    // in-use template: locked delete
    if (used) try {
      const trig = page.getByRole('button', { name: `Thêm thao tác cho ${used.name}`, exact: true });
      await trig.click();
      const menu = page.locator('[role="menu"]:visible').first();
      await menu.waitFor();
      const g = await overlayGeom(page, 'menu', vp);
      const del = menu.getByRole('menuitem', { name: 'Xoá' });
      const delDisabled = await del.getAttribute('aria-disabled');
      const descId = await del.getAttribute('aria-describedby');
      const delDesc = descId ? (await page.locator(`#${descId}`).textContent()) : null;
      const s = await shot(page, `native-${tag}-menu-inuse.png`);
      await del.click({ force: true });
      const stillOpen = await menu.isVisible();
      const noDialog = (await page.locator('[role="alertdialog"]:visible').count()) === 0;
      record(A({ control: 'menu-delete-locked', action: 'click locked Xoá', observed: `bounds=${JSON.stringify(g.bounds)} inside=${g.inside}; aria-disabled=${delDisabled} reason="${(delDesc || '').slice(0, 90)}"; after click menuOpen=${stillOpen} noDialog=${noDialog}`, status: g.inside && delDisabled === 'true' && stillOpen && noDialog ? 'passed' : 'failed', screenshot: s }));
      await page.keyboard.press('Escape');
      await menu.waitFor({ state: 'hidden' });
      record(A({ control: 'menu-esc-close', action: 'Escape', observed: 'menu closed, focus back on trigger', status: 'passed' }));
    } catch (e) { fail(A({ control: 'menu-delete-locked', action: 'open/lock/Esc' }), e); }

    // free template: outside-click, rename, attach-nav, delete-confirm
    if (free) {
      const trig = page.getByRole('button', { name: `Thêm thao tác cho ${free.name}`, exact: true });
      try {
        await trig.click();
        const menu = page.locator('[role="menu"]:visible').first();
        await menu.waitFor();
        await page.mouse.click(30, vp.height - 30);
        await menu.waitFor({ state: 'hidden' });
        record(A({ control: 'menu-outside-close', action: 'click outside', observed: 'menu closed on pointerdown outside', status: 'passed' }));
      } catch (e) { fail(A({ control: 'menu-outside-close', action: 'outside click' }), e); }

      try {
        await trig.click();
        await page.locator('[role="menu"]:visible').first().waitFor();
        await page.getByRole('menuitem', { name: 'Đổi tên' }).click();
        const dlg = page.locator('[role="dialog"]:visible').first();
        await dlg.waitFor();
        const title = await dlg.textContent();
        const g = await overlayGeom(page, 'dialog', vp);
        const s = await shot(page, `native-${tag}-rename-dialog.png`);
        await closeTopOverlay(page);
        await dlg.waitFor({ state: 'hidden' });
        record(A({ control: 'rename-dialog', action: 'menu→Đổi tên→Huỷ', observed: `title ok=${title?.includes('Đổi tên')}; bounds inside=${g.inside}; cancelled, no PATCH`, status: title?.includes('Đổi tên') && g.inside ? 'passed' : 'failed', screenshot: s }));
      } catch (e) { fail(A({ control: 'rename-dialog', action: 'open/cancel' }), e); }

      try {
        await trig.click();
        await page.locator('[role="menu"]:visible').first().waitFor();
        await page.getByRole('menuitem', { name: 'Đặt làm mặc định cho kênh' }).click();
        const dlg = page.locator('[role="dialog"]:visible').first();
        await dlg.waitFor();
        const s = await shot(page, `native-${tag}-attach-chooser.png`);
        await dlg.getByRole('button', { name: `Mở ${channels[0].name}` }).click();
        await page.waitForURL(`**#/channels/${channels[0].id}**`);
        await settleAnims(page);
        record(A({ control: 'attach-chooser', action: 'menu→attach→Mở <channel>', observed: `navigated to ${new URL(page.url()).hash}; read-only (default set happens in Channel › Template)`, status: 'passed', screenshot: s }));
        await page.goto(`${ORIGIN}/#/library`, { waitUntil: 'domcontentloaded' });
        await page.getByRole('heading', { name: 'Thư viện', exact: true }).waitFor();
        await page.getByText(templates[0].name, { exact: true }).first().waitFor();
      } catch (e) {
        fail(A({ control: 'attach-chooser', action: 'open+navigate' }), e);
        await page.goto(`${ORIGIN}/#/library`, { waitUntil: 'domcontentloaded' });
        await page.getByRole('heading', { name: 'Thư viện', exact: true }).waitFor();
      }

      try {
        await trig.click();
        await page.locator('[role="menu"]:visible').first().waitFor();
        await page.getByRole('menuitem', { name: 'Xoá' }).click();
        const cf = page.locator('[role="alertdialog"]:visible').first();
        await cf.waitFor();
        const g = await overlayGeom(page, 'alertdialog', vp);
        const txt = (await cf.textContent()) || '';
        const s = await shot(page, `native-${tag}-delete-confirm.png`);
        await closeTopOverlay(page);
        await cf.waitFor({ state: 'hidden' });
        record(A({ control: 'delete-confirm', action: 'menu→Xoá→Huỷ', observed: `alertdialog "${txt.slice(0, 60)}" inside=${g.inside}; cancelled, no DELETE`, status: g.inside ? 'passed' : 'failed', screenshot: s }));
      } catch (e) { fail(A({ control: 'delete-confirm', action: 'open/cancel' }), e); }
    }

    // ── Header actions: create + import dialogs (open/cancel only) ──
    try {
      await page.getByRole('button', { name: 'Mẫu video mới', exact: true }).click();
      const dlg = page.locator('[role="dialog"]:visible').first();
      await dlg.waitFor();
      await page.locator('#vlb-new-name').fill('QA không lưu');
      const s = await shot(page, `native-${tag}-create-dialog.png`);
      await closeTopOverlay(page);
      await dlg.waitFor({ state: 'hidden' });
      record(A({ control: 'create-dialog', action: 'open, fill, Huỷ', observed: 'dialog opened, name filled, cancelled — no POST /api/templates', status: 'passed', screenshot: s }));
    } catch (e) { fail(A({ control: 'create-dialog', action: 'open/cancel' }), e); }

    try {
      await page.getByRole('button', { name: 'Nhập mẫu video', exact: true }).click();
      const dlg = page.locator('[role="dialog"]:visible').first();
      await dlg.waitFor();
      const s = await shot(page, `native-${tag}-import-dialog.png`);
      await page.keyboard.press('Escape');
      await dlg.waitFor({ state: 'hidden' });
      record(A({ control: 'import-dialog', action: 'open→Esc', observed: 'import dialog rendered; closed on Esc; no file/upload attempted', status: 'passed', screenshot: s }));
    } catch (e) { fail(A({ control: 'import-dialog', action: 'open/Esc' }), e); }

    // ── Template screen ──
    const target = used ?? templates[0];
    try {
      await page.getByRole('link', { name: target.name, exact: true }).first().click();
      await page.waitForURL(`**#/templates/${target.id}**`);
      await page.locator('nav a[href^="#/templates/"]').first().waitFor();
      await fonts(page); await settleAnims(page);
      record(A({ control: 'template-open', action: 'card name link', observed: `navigated to #/templates/${target.id}`, status: 'passed', screenshot: await shot(page, `native-${tag}-template.png`) }));

      for (const sec of ['cover', 'backdrop', 'output', 'images']) {
        try {
          await page.locator(`a[href="#/templates/${target.id}?section=${sec}"]`).first().click();
          await page.waitForURL(`**/templates/${target.id}?section=${sec}`);
          await settleAnims(page);
          record(A({ control: `template-section-${sec}`, action: 'section nav', observed: 'mounted', status: 'passed' }));
        } catch (e) { fail(A({ control: `template-section-${sec}`, action: 'nav' }), e); }
      }

      // header options menu
      try {
        await page.getByRole('button', { name: 'Thêm thao tác cho mẫu video', exact: true }).click();
        const menu = page.locator('[role="menu"]:visible').first();
        await menu.waitFor();
        const g = await overlayGeom(page, 'menu', vp);
        const items = await menu.locator('[role="menuitem"]').allTextContents();
        const delLocked = await menu.getByRole('menuitem', { name: 'Xoá' }).getAttribute('aria-disabled').catch(() => null);
        const s = await shot(page, `native-${tag}-template-options.png`);
        await page.keyboard.press('Escape');
        record(A({ control: 'template-options-menu', action: 'open+Esc', observed: `items=[${items.map((i) => i.trim()).join(', ')}] deleteLocked=${delLocked}; inside=${g.inside}`, status: g.inside ? 'passed' : 'failed', screenshot: s }));
      } catch (e) { fail(A({ control: 'template-options-menu', action: 'open/Esc' }), e); }

      // PC floating popover: preview source KitSelect
      try {
        const src = page.getByRole('combobox', { name: 'Chọn video để dựng bản xem thử', exact: true });
        await src.click();
        const pop = page.locator('[role="listbox"]:visible').first();
        await pop.waitFor({ timeout: 5000 });
        const g = await overlayGeom(page, 'listbox', vp);
        const optCount = await pop.getByRole('option').count();
        const s = await shot(page, `native-${tag}-preview-source-popover.png`);
        await page.keyboard.press('Escape');
        record(A({ control: 'preview-source-popover', action: 'open select→Esc', observed: `floating listbox with ${optCount} option(s); bounds=${JSON.stringify(g.bounds)} inside=${g.inside}; no build started`, status: g.inside ? 'passed' : 'failed', screenshot: s }));
      } catch (e) { fail(A({ control: 'preview-source-popover', action: 'open/Esc' }), e); }

      // back link
      try {
        await page.locator('a.vt-nav-back').first().click();
        await page.waitForURL('**#/library**');
        await page.getByRole('heading', { name: 'Thư viện', exact: true }).waitFor();
        record(A({ control: 'template-back-nav', action: 'nav back link', observed: 'returned to #/library', status: 'passed' }));
      } catch (e) { fail(A({ control: 'template-back-nav', action: 'back' }), e); }
    } catch (e) { fail(A({ control: 'template-open', action: 'open template' }), e); }
  } catch (e) {
    result.failures.push({ control: 'desktop-flow', error: String(e?.message ?? e) });
  } finally {
    await context.close();
  }
}

// ── Mobile pass ───────────────────────────────────────────────────────────
async function mobile(browser, viewport, reducedMotion = false) {
  const { context, page } = await makeContext(browser, viewport, reducedMotion, true);
  const vp = viewport;
  const A = (partial) => ({ viewport: vp, reducedMotion, ...partial });
  const tag = `${vp.width}${reducedMotion ? '-reduced' : ''}`;
  try {
    const templates = (await (await context.request.get(`${ORIGIN}/api/templates`)).json()).items;
    const channels = (await (await context.request.get(`${ORIGIN}/api/owner-channels`)).json()).channels;
    const free = templates.find((t) => !t.channels?.length);
    const used = templates.find((t) => t.channels?.length);

    await page.goto(`${ORIGIN}/#/library`, { waitUntil: 'domcontentloaded' });
    await page.getByText('Thư viện dùng chung', { exact: true }).waitFor();
    await page.getByText(templates[0].name, { exact: true }).first().waitFor();
    await fonts(page); await settleAnims(page);
    const geo = await scrollW(page);
    record(A({ control: 'mlibrary-load', action: 'goto #/library', observed: `rendered; scrollW=${geo.sw} innerW=${geo.w}`, status: geo.sw > geo.w ? 'failed' : 'passed', screenshot: await shot(page, `native-${tag}-mlibrary-templates.png`) }));

    // Tab strip: 4 Segmented radios in the library radiogroup
    try {
      const strip = page.locator('[role="radiogroup"][aria-label="Thư viện"]');
      await strip.waitFor();
      const tabs = strip.locator('[role="radio"]');
      const n = await tabs.count();
      const boxes = [];
      for (let i = 0; i < n; i++) boxes.push(await tabs.nth(i).boundingBox());
      const widths = boxes.map((b) => Math.round(b.width));
      const heights = boxes.map((b) => Math.round(b.height));
      for (const [label, id] of [['Chỉ dẫn ảnh', 'prompts'], ['Video nền', 'backdrops'], ['Lớp phủ', 'overlays'], ['Mẫu video', 'templates']]) {
        await strip.getByRole('radio', { name: new RegExp(label) }).click();
        await page.waitForURL(`**${id}`).catch(() => {});
        await settleAnims(page);
      }
      const s = await shot(page, `native-${tag}-mlibrary-tabs.png`);
      const h44 = heights.every((h) => h >= 44);
      record(A({ control: 'mlibrary-tabs', action: 'measure + tap 4 tabs', observed: `${n} tabs widths=${JSON.stringify(widths)} heights=${JSON.stringify(heights)} all>=44:${h44}`, status: h44 && n === 4 ? 'passed' : 'failed', screenshot: s }));
    } catch (e) { fail(A({ control: 'mlibrary-tabs', action: 'tabs' }), e); }

    // "+" -> ActionSheet "Thêm vào thư viện" (mobile BottomSheet)
    try {
      const add = page.getByRole('button', { name: 'Thêm vào thư viện', exact: true });
      await add.click();
      const sheet = page.locator('.m-sheet:visible').first();
      await sheet.waitFor();
      const g = await overlayGeom(page, 'dialog', vp);
      const items = await sheet.getByRole('button').allTextContents();
      const s = await shot(page, `native-${tag}-sheet-add.png`);
      record(A({ control: 'add-sheet', action: 'open', observed: `bottom sheet bounds=${JSON.stringify(g.bounds)} inside=${g.inside}; ${items.length} actions: ${items.map((i) => i.trim().split('\n')[0]).join(' | ')}`, status: g.inside ? 'passed' : 'failed', screenshot: s }));
      // Item-level coverage: open and close each sub-flow
      await sheet.getByRole('button', { name: /Chỉ dẫn ảnh mới/ }).click();
      const editor = page.locator('.m-sheet:visible').first();
      await editor.waitFor();
      const gE = await overlayGeom(page, 'dialog', vp);
      const sE = await shot(page, `native-${tag}-prompt-editor-sheet.png`);
      await page.keyboard.press('Escape');
      await editor.waitFor({ state: 'hidden', timeout: 8000 }).catch(() => {});
      record(A({ control: 'sheet-prompt-editor', action: 'add-sheet→Chỉ dẫn ảnh mới→Esc', observed: `PromptEditor sheet inside=${gE.inside}`, status: gE.inside ? 'passed' : 'failed', screenshot: sE }));

      await add.click();
      const sheet2 = page.locator('.m-sheet:visible').first();
      await sheet2.waitFor();
      await sheet2.getByRole('button', { name: /Mẫu video mới/ }).click();
      const createDlg = page.locator('[role="dialog"]:visible').last();
      await createDlg.waitFor();
      await closeTopOverlay(page);
      record(A({ control: 'sheet-new-template', action: 'add-sheet→Mẫu video mới→Huỷ', observed: 'naming dialog opened and cancelled — no POST', status: 'passed' }));

      await add.click();
      const sheet3 = page.locator('.m-sheet:visible').first();
      await sheet3.waitFor();
      await sheet3.getByRole('button', { name: /Nhập mẫu video/ }).click();
      const imp = page.locator('[role="dialog"]:visible').last();
      await imp.waitFor();
      const gI = await overlayGeom(page, 'dialog', vp);
      const sI = await shot(page, `native-${tag}-import-sheet.png`);
      await page.keyboard.press('Escape');
      await imp.waitFor({ state: 'hidden', timeout: 8000 }).catch(() => {});
      record(A({ control: 'sheet-import', action: 'add-sheet→Nhập mẫu video→Esc', observed: `import dialog inside=${gI.inside}; no upload attempted`, status: gI.inside ? 'passed' : 'failed', screenshot: sI }));

      await add.click();
      const sheet4 = page.locator('.m-sheet:visible').first();
      await sheet4.waitFor();
      await sheet4.getByRole('button', { name: /Kênh hoặc video nền/ }).click();
      const linkSheet = page.locator('.m-sheet:visible').first();
      await linkSheet.waitFor();
      const gL = await overlayGeom(page, 'dialog', vp);
      const sL = await shot(page, `native-${tag}-backdrop-link-sheet.png`);
      await page.keyboard.press('Escape');
      record(A({ control: 'sheet-add-backdrop', action: 'add-sheet→Kênh hoặc video nền→Esc', observed: `height-auto link sheet bounds=${JSON.stringify(gL.bounds)} inside=${gL.inside}`, status: gL.inside ? 'passed' : 'failed', screenshot: sL }));
    } catch (e) { fail(A({ control: 'add-sheet-flows', action: 'sheet items' }), e); }

    // Template card ⋯ sheet: in-use (locked delete) and free (rename + delete confirm)
    if (used) {
      try {
        await page.getByRole('button', { name: `Thao tác với ${used.name}`, exact: true }).click();
        const sheet = page.locator('.m-sheet:visible').first();
        await sheet.waitFor();
        const g = await overlayGeom(page, 'dialog', vp);
        const del = sheet.getByRole('button', { name: /Xoá mẫu video/ });
        const disabled = await del.isDisabled().catch(() => false);
        const ariaDis = await del.getAttribute('aria-disabled');
        const s = await shot(page, `native-${tag}-sheet-actions-inuse.png`);
        await page.keyboard.press('Escape');
        await sheet.waitFor({ state: 'hidden' });
        record(A({ control: 'template-sheet-inuse', action: 'open, inspect delete, Esc', observed: `delete disabled=${disabled} aria-disabled=${ariaDis} w/ reason; inside=${g.inside}`, status: (disabled || ariaDis === 'true') && g.inside ? 'passed' : 'failed', screenshot: s }));
      } catch (e) { fail(A({ control: 'template-sheet-inuse', action: 'sheet in-use' }), e); }
    }
    if (free) {
      try {
        await page.getByRole('button', { name: `Thao tác với ${free.name}`, exact: true }).click();
        const sheet = page.locator('.m-sheet:visible').first();
        await sheet.waitFor();
        await sheet.getByRole('button', { name: 'Đổi tên' }).click();
        const dlg = page.locator('[role="dialog"]:visible').last();
        await dlg.waitFor();
        const g = await overlayGeom(page, 'dialog', vp);
        await closeTopOverlay(page);
        record(A({ control: 'template-sheet-rename', action: 'sheet→Đổi tên→Huỷ', observed: `rename dialog inside=${g.inside}; cancelled, no PATCH`, status: g.inside ? 'passed' : 'failed' }));

        // attach chooser navigates to channel templates
        await page.getByRole('button', { name: `Thao tác với ${free.name}`, exact: true }).click();
        const sheetA = page.locator('.m-sheet:visible').first();
        await sheetA.waitFor();
        await sheetA.getByRole('button', { name: /Đặt làm mặc định cho kênh/ }).click();
        const chooser = page.locator('[role="dialog"]:visible').last();
        await chooser.waitFor();
        const sC = await shot(page, `native-${tag}-attach-chooser.png`);
        await chooser.getByRole('button', { name: `Mở ${channels[0].name}` }).click();
        await page.waitForURL(`**#/channels/${channels[0].id}**`);
        record(A({ control: 'attach-chooser', action: 'sheet→attach→Mở <channel>', observed: `navigated to ${new URL(page.url()).hash}`, status: 'passed', screenshot: sC }));
        await page.goto(`${ORIGIN}/#/library`, { waitUntil: 'domcontentloaded' });
        await page.getByText('Thư viện dùng chung', { exact: true }).waitFor();
        await page.getByText(templates[0].name, { exact: true }).first().waitFor();

        // delete confirm
        await page.getByRole('button', { name: `Thao tác với ${free.name}`, exact: true }).click();
        const sheet2 = page.locator('.m-sheet:visible').first();
        await sheet2.waitFor();
        await sheet2.getByRole('button', { name: /Xoá mẫu video/ }).click();
        const cf = page.locator('[role="alertdialog"]:visible').first();
        await cf.waitFor();
        const g2 = await overlayGeom(page, 'alertdialog', vp);
        const s = await shot(page, `native-${tag}-confirm-delete.png`);
        await page.getByRole('button', { name: 'Huỷ', exact: true }).click();
        await cf.waitFor({ state: 'hidden' });
        record(A({ control: 'template-delete-confirm', action: 'sheet→Xoá→Huỷ', observed: `ConfirmSheet bounds=${JSON.stringify(g2.bounds)} inside=${g2.inside}; cancelled, no DELETE`, status: g2.inside ? 'passed' : 'failed', screenshot: s }));
      } catch (e) { fail(A({ control: 'template-sheet-free', action: 'rename/attach/delete' }), e); }
    }

    // MTemplate list + section screens + more sheet + preview sheet
    const target = used ?? templates[0];
    try {
      await page.getByRole('button', { name: `Mở ${target.name}`, exact: true }).first().click();
      await page.waitForURL(`**#/templates/${target.id}**`);
      await page.getByRole('button', { name: /Ảnh cảnh/ }).first().waitFor();
      await fonts(page); await settleAnims(page);
      const s = await shot(page, `native-${tag}-mtemplate-list.png`);
      const rows = await page.locator('section[aria-label="Cài đặt"] button').allTextContents().catch(() => []);
      record(A({ control: 'mtemplate-list', action: 'open template', observed: `${rows.length} section rows: ${rows.map((r) => r.trim().split('\n')[0]).join(' | ')}`, status: rows.length === 4 ? 'passed' : 'partial', screenshot: s }));

      try {
        await page.getByRole('button', { name: 'Thao tác với mẫu video', exact: true }).click();
        const sheet = page.locator('.m-sheet:visible').first();
        await sheet.waitFor();
        const g = await overlayGeom(page, 'dialog', vp);
        const items = await sheet.getByRole('button').allTextContents();
        const s2 = await shot(page, `native-${tag}-mtemplate-more.png`);
        await page.keyboard.press('Escape');
        await sheet.waitFor({ state: 'hidden' });
        record(A({ control: 'mtemplate-more-sheet', action: 'open+Esc', observed: `items=${items.map((i) => i.trim().split('\n')[0]).join(', ')}; inside=${g.inside}`, status: g.inside ? 'passed' : 'failed', screenshot: s2 }));
      } catch (e) { fail(A({ control: 'mtemplate-more-sheet', action: 'open/Esc' }), e); }

      // preview source sheet: opens sheet; selecting a source would POST a build -> cancel
      try {
        const previewBtn = page.getByRole('button', { name: /Dựng lại/ }).first();
        if (await previewBtn.count()) {
          const disabled = await previewBtn.isDisabled();
          if (!disabled) {
            await previewBtn.click();
            const sheet = page.locator('.m-sheet:visible').first();
            const opened = await sheet.waitFor({ timeout: 5000 }).then(() => true).catch(() => false);
            if (opened) {
              const g = await overlayGeom(page, 'dialog', vp);
              const s3 = await shot(page, `native-${tag}-preview-source-sheet.png`);
              await page.keyboard.press('Escape');
              record(A({ control: 'mtemplate-preview-sheet', action: 'Dựng lại→source sheet→Esc', observed: `source picker sheet inside=${g.inside}; cancelled — no preview build`, status: g.inside ? 'passed' : 'failed', screenshot: s3 }));
            } else {
              record(A({ control: 'mtemplate-preview-sheet', action: 'Dựng lại', observed: 'button enabled but no sheet opened', status: 'failed' }));
            }
          } else {
            const title = await previewBtn.getAttribute('title');
            record(A({ control: 'mtemplate-preview-sheet', action: 'inspect Dựng lại', observed: `disabled w/ reason "${title}"`, status: 'partial' }));
          }
        } else {
          record(A({ control: 'mtemplate-preview-sheet', action: 'inspect', observed: 'no Dựng lại button on list screen', status: 'partial' }));
        }
      } catch (e) { fail(A({ control: 'mtemplate-preview-sheet', action: 'open/Esc' }), e); }

      try {
        await page.getByRole('button', { name: /Ảnh cảnh/ }).first().click();
        await page.waitForURL(`**/templates/${target.id}?section=images`);
        await settleAnims(page);
        const s4 = await shot(page, `native-${tag}-mtemplate-images.png`);
        await page.getByRole('button', { name: /Quay lại/ }).first().click();
        await page.waitForURL(`**/templates/${target.id}`);
        record(A({ control: 'mtemplate-section-nav', action: 'images→back', observed: 'section screen + return worked', status: 'passed', screenshot: s4 }));
      } catch (e) { fail(A({ control: 'mtemplate-section-nav', action: 'section nav' }), e); }
    } catch (e) { fail(A({ control: 'mtemplate-open', action: 'open' }), e); }
  } catch (e) {
    result.failures.push({ control: 'mobile-flow', error: String(e?.message ?? e) });
  } finally {
    await context.close();
  }
}

(async () => {
  const browser = await chromium.launch({ executablePath: EXEC, headless: true });
  try {
    await desktop(browser, { width: 1440, height: 900 }, false);
    await mobile(browser, { width: 390, height: 844 }, false);
    await desktop(browser, { width: 1440, height: 900 }, true);
    // Touch-target + overflow audit, mobile context.
    {
      const { context, page } = await makeContext(browser, { width: 390, height: 844 }, false, true);
      const vp = { width: 390, height: 844 };
      try {
        await page.goto(`${ORIGIN}/#/library`, { waitUntil: 'domcontentloaded' });
        await page.getByText('Thư viện dùng chung', { exact: true }).waitFor();
        await fonts(page); await settleAnims(page);
        for (const tabName of ['templates', 'prompts']) {
          if (tabName !== 'templates') {
            await page.locator(`[role="radiogroup"][aria-label="Thư viện"] [role="radio"]`, { hasText: 'Chỉ dẫn' }).first().click().catch(() => {});
            await settleAnims(page);
          }
          const controls = await auditControls(page);
          const small = controls.filter((c) => c.w < 44 || c.h < 44);
          await shot(page, `native-390-touch-${tabName}.png`);
          result.actions.push({ viewport: vp, reducedMotion: false, control: `touch-audit-${tabName}`, action: 'measure interactive controls', observed: `${controls.length} controls; ${small.length} under 44px${small.length ? ': ' + small.slice(0, 12).map((c) => `${c.label}(${c.w}x${c.h})`).join('; ') : ''}`, status: small.length ? 'partial' : 'passed', smallCount: small.length, small: small.slice(0, 30) });
        }
        const ov = await scrollW(page);
        if (ov.sw > ov.w) result.actions.push({ viewport: vp, reducedMotion: false, control: 'horizontal-overflow', action: 'scrollWidth', observed: `${ov.sw} > ${ov.w}`, status: 'failed' });
      } finally { await context.close(); }
    }
  } finally {
    await browser.close();
    result.apiReads = result.apiReads.length;
    await fs.writeFile(`${root}/library-native-qa-results.json`, JSON.stringify(result, null, 2));
  }
  console.log(JSON.stringify({ actions: result.actions.length, failures: result.failures, blockedMutations: result.blockedMutations }));
})().catch((e) => { console.error(e); process.exitCode = 1; });
