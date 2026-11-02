import { test, expect, type Locator, type Page, type TestInfo } from '@playwright/test'
import { readFileSync } from 'node:fs'

// Desktop parity harness — drives the REAL served app (8053 API behind the 8060
// frozen web build) and the canonical boards (8067). Rules:
//  - no page.route / no request rewriting; no style or DOM injection;
//  - assertions are semantic (roles, aria-*, focus, open/closed) or the one
//    positional contract that a menu opens from the trigger's left edge;
//  - all pixel/colour/font metadata is CAPTURED to JSON attachments, never
//    asserted, so a design refresh does not repin the suite;
//  - offline and confirm dialogs need real states/data: they only run with the
//    explicit operator envs and otherwise report “outside API state”.

const APP = process.env.PC_APP_URL ?? 'http://127.0.0.1:8060'
const CANON = process.env.PC_CANON_URL ?? 'http://127.0.0.1:8067'
// PW_STATE is a full Playwright storageState. PW_COOKIE_FILE points at the
// owned QA cookie JSON the sibling PC harnesses already use; it stays in /tmp
// and is read in place (no copy, no staging).
const COOKIE_FILE = process.env.PW_COOKIE_FILE ?? ''
const hasAuth = !!process.env.PW_STATE || !!COOKIE_FILE

test.beforeEach(async ({ context }) => {
  if (process.env.PW_STATE || !COOKIE_FILE) return
  const jar = JSON.parse(readFileSync(COOKIE_FILE, 'utf8')) as Record<string, string>
  await context.addCookies(Object.entries(jar).map(([name, value]) => ({ name, value, url: APP, httpOnly: true, sameSite: 'Lax' as const })))
})

// Page errors / console errors / the actual entry asset served in this run.
const runtimeLog = new Map<string, string[]>()
test.beforeEach(async ({ page }, testInfo) => {
  const log: string[] = []
  runtimeLog.set(testInfo.testId, log)
  page.on('pageerror', (error) => log.push(`pageerror: ${error.message}`))
  page.on('console', (message) => { if (message.type() === 'error') log.push(`console: ${message.text()}`) })
  page.on('response', (response) => {
    if (/\/assets\/index-[^/]+\.js$/.test(response.url())) log.push(`entry ${response.status()} ${response.url()}`)
    if (response.status() >= 400) log.push(`http ${response.status()} ${response.request().method()} ${response.url()}`)
  })
})
test.afterEach(async ({}, testInfo) => {
  const log = runtimeLog.get(testInfo.testId)
  runtimeLog.delete(testInfo.testId)
  if (log) await testInfo.attach('runtime-errors.json', { body: JSON.stringify(log, null, 2), contentType: 'application/json' })
})

async function attachJson(testInfo: TestInfo, name: string, data: unknown) {
  await testInfo.attach(`${name}.json`, { body: JSON.stringify(data, null, 2), contentType: 'application/json' })
}

async function shot(page: Page, testInfo: TestInfo, name: string, target?: Locator | null) {
  const path = testInfo.outputPath(`${name}.png`)
  await (target ?? page).screenshot({ path })
  await testInfo.attach(name, { path, contentType: 'image/png' })
}

/** Geometry + computed values for evidence only (never asserted). */
async function capture(locator: Locator) {
  return locator.evaluate((el) => {
    const r = el.getBoundingClientRect()
    const s = getComputedStyle(el)
    return {
      rect: { x: Math.round(r.x), y: Math.round(r.y), width: Math.round(r.width), height: Math.round(r.height) },
      color: s.color, background: s.backgroundColor, border: s.borderColor, radius: s.borderRadius,
      font: `${s.fontSize}/${s.fontWeight}`, transition: s.transition, animation: s.animationName,
    }
  })
}

async function openApp(page: Page) {
  // Real first-run gate: the app checks the toolchain once per tab and stores
  // sessionStorage 'viderse:setup-checked'. Let it run (GET-only), then reload
  // into the workbench. No init script / DOM patch.
  await page.goto('/#/', { waitUntil: 'domcontentloaded' })
  await page.waitForFunction(() => sessionStorage.getItem('viderse:setup-checked') === '1', null, { timeout: 30_000 })
  await page.goto('/#/', { waitUntil: 'domcontentloaded' })
  await page.evaluate(() => document.fonts.ready)
  await expect(page.locator('header nav')).toBeVisible()
  await expect(page.locator('[data-jobs]')).toBeVisible()
}

const switcher = (page: Page) => page.locator('button[aria-haspopup="menu"][aria-label^="Đổi kênh"]')
const account = (page: Page) => page.getByRole('button', { name: 'Tài khoản của bạn' })
const jobs = (page: Page) => page.locator('header [data-jobs]')
const topMenu = (page: Page) => page.locator('[role="menu"]').last()
const activity = (page: Page) => page.getByRole('dialog', { name: 'Việc nền' })

test.describe('desktop shell', () => {
  test.skip(!hasAuth, 'set PW_STATE to the owned QA storageState')

  test('header, hover/pressed states, focus ring, reduced motion', async ({ page }, testInfo) => {
    await openApp(page)
    const header = page.locator('header[role="banner"]')
    await shot(page, testInfo, `01-shell-${page.viewportSize()?.width ?? 'x'}`)
    await shot(page, testInfo, '02-header', header)

    await expect(page.locator('header nav a[aria-current="page"]')).toHaveCount(1)
    await expect(switcher(page)).toBeVisible()
    await expect(jobs(page)).toBeVisible()
    await expect(page.locator('header a[aria-label^="Cài đặt"]')).toBeVisible()
    await expect(account(page)).toBeVisible()

    await attachJson(testInfo, '03-header-metrics', {
      activeNav: await capture(page.locator('header nav a[aria-current="page"]')),
      activeNavIcon: await capture(page.locator('header nav a[aria-current="page"] span').first()),
      switcher: await capture(switcher(page)),
      gear: await capture(page.locator('header a[aria-label^="Cài đặt"]')),
      jobs: await capture(jobs(page)),
      avatar: await capture(account(page)),
    })

    // Real hover on every shell control; the screenshots are the proof.
    for (const [name, locator] of [
      ['nav-active', page.locator('header nav a[aria-current="page"]')],
      ['switcher', switcher(page)],
      ['gear', page.locator('header a[aria-label^="Cài đặt"]')],
      ['avatar', account(page)],
    ] as const) {
      await locator.hover()
      await shot(page, testInfo, `04-hover-${name}`, header)
    }

    // Real pressed state: mouse down, capture, release. The switcher opens on
    // release (native behaviour) — close it again with Escape.
    const box = await switcher(page).boundingBox()
    if (box) {
      await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
      await page.mouse.down()
      await shot(page, testInfo, '05-pressed-switcher', header)
      await page.mouse.up()
      await page.keyboard.press('Escape')
    }

    // Focus ring: Tab from the top reaches the logo and switcher with a visible
    // outline; the computed outline is captured, not asserted.
    await page.locator('body').press('Tab')
    await attachJson(testInfo, '06-focus-first', await page.evaluate(() => {
      const el = document.activeElement as HTMLElement
      const s = getComputedStyle(el)
      return { tag: el.tagName, label: el.getAttribute('aria-label'), outline: `${s.outlineWidth} ${s.outlineStyle} ${s.outlineColor}`, offset: s.outlineOffset }
    }))

    // Reduced motion is emulated on the same page/context (no second browser).
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await switcher(page).click()
    await expect(topMenu(page)).toBeVisible()
    await attachJson(testInfo, '07-reduced-motion-menu', await page.evaluate(() => document.getAnimations().map((a) => ({
      name: (a as CSSAnimation).animationName ?? a.constructor.name, state: a.playState,
    }))))
    expect(await page.evaluate(() => document.getAnimations().some((a) => (a as CSSAnimation).animationName === 'kit-pop'))).toBe(false)
    await page.keyboard.press('Escape')
    await page.emulateMedia({ reducedMotion: 'no-preference' })
  })
})

test.describe('channel switcher menu', () => {
  test.skip(!hasAuth, 'set PW_STATE to the owned QA storageState')

  test('start alignment, separators, keyboard focus, Escape restore, outside close', async ({ page }, testInfo) => {
    await openApp(page)
    const trigger = switcher(page)
    await trigger.click()
    const menu = topMenu(page)
    await expect(menu).toBeVisible()
    await shot(page, testInfo, '10-switcher-open')

    const triggerBox = await trigger.boundingBox()
    // `.kit-pop` is the portalled shell; the role=menu element inside is inset by
    // its 6px padding + 1px border. Measure the shell, after the open transform.
    const menuShell = menu.locator('..')
    await menuShell.evaluate((el) => Promise.all(el.getAnimations().map((a) => a.finished.catch(() => undefined))))
    const menuBox = await menuShell.boundingBox()
    expect(triggerBox && menuBox).toBeTruthy()
    if (triggerBox && menuBox) {
      await attachJson(testInfo, '11-switcher-geometry', {
        trigger: triggerBox,
        menu: menuBox,
        debug: await menuShell.evaluate((el) => {
          const s = getComputedStyle(el)
          return {
            left: s.left, top: s.top, width: s.width, transform: s.transform, position: s.position,
            offsetWidth: (el as HTMLElement).offsetWidth,
            styleAttr: el.getAttribute('style'), className: el.className,
            parent: el.parentElement?.tagName,
            animations: el.getAnimations().map((a) => ({ name: (a as CSSAnimation).animationName ?? a.constructor.name, state: a.playState })),
          }
        }),
      })
      // Only the documented contract: opens from the trigger's left edge, not
      // right-aligned to it. Whole-pixel tolerance, no design values.
      expect(Math.abs(menuBox.x - triggerBox.x)).toBeLessThanOrEqual(4)
    }

    const radios = menu.getByRole('menuitemradio')
    const separators = menu.getByRole('separator')
    expect(await radios.count()).toBeGreaterThan(0)
    expect(await separators.count()).toBeGreaterThanOrEqual(2)
    await attachJson(testInfo, '12-switcher-menu-metrics', {
      menu: await capture(menuShell),
      firstRadio: await capture(radios.first()),
      separator: await capture(separators.first()),
    })

    // Hover a row (real pointer), then keyboard traversal.
    await radios.first().hover()
    await shot(page, testInfo, '13-switcher-hover-row')

    await page.keyboard.press('Escape')
    await expect(menu).toBeHidden()
    await expect(trigger).toBeFocused()

    // Keyboard open: ArrowDown opens and focuses the first item.
    await trigger.focus()
    await page.keyboard.press('ArrowDown')
    await expect(menu).toBeVisible()
    await expect(radios.first()).toBeFocused()
    await page.keyboard.press('ArrowDown')
    await expect(radios.nth(1)).toBeFocused()
    await page.keyboard.press('ArrowUp')
    await expect(radios.first()).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(trigger).toBeFocused()

    // Outside click closes (no Escape involved).
    await trigger.click()
    await expect(menu).toBeVisible()
    await expect(trigger).toHaveAttribute('aria-expanded', 'true')
    // Inert header padding, not a control: proves pointerdown-outside closes
    // without a route change (parent Menus close-callback fix).
    await page.locator('header[role="banner"]').click({ position: { x: 600, y: 6 } })
    await expect(menu).toBeHidden()
    await expect(trigger).toHaveAttribute('aria-expanded', 'false')
  })
})

test.describe('account menu', () => {
  test.skip(!hasAuth, 'set PW_STATE to the owned QA storageState')

  test('opens under the avatar, arrow keys, Escape restore, outside close', async ({ page }, testInfo) => {
    await openApp(page)
    const trigger = account(page)
    await trigger.click()
    const menu = topMenu(page)
    await expect(menu).toBeVisible()
    await shot(page, testInfo, '20-account-open')
    await attachJson(testInfo, '21-account-metrics', {
      menu: await capture(menu),
      firstItem: await capture(menu.getByRole('menuitem').first()),
    })

    await expect(menu.getByRole('menuitem').first()).toBeFocused()
    await page.keyboard.press('ArrowDown')
    await expect(menu.getByRole('menuitem').nth(1)).toBeFocused()
    await page.keyboard.press('ArrowUp')
    await expect(menu.getByRole('menuitem').first()).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(menu).toBeHidden()
    await expect(trigger).toBeFocused()

    await trigger.click()
    await menu.getByRole('menuitem').first().hover()
    await shot(page, testInfo, '22-account-hover-row')
    await expect(trigger).toHaveAttribute('aria-expanded', 'true')
    await page.locator('header[role="banner"]').click({ position: { x: 600, y: 6 } })
    await expect(menu).toBeHidden()
    await expect(trigger).toHaveAttribute('aria-expanded', 'false')
  })
})

test.describe('activity popover', () => {
  test.skip(!hasAuth, 'set PW_STATE to the owned QA storageState')

  test('portal, caret, tabs, settings layer, Escape restore, outside close', async ({ page }, testInfo) => {
    await openApp(page)
    const pill = jobs(page)
    await pill.click()
    const dialog = activity(page)
    await expect(dialog).toBeVisible()
    await expect(dialog).toBeFocused()
    expect(await dialog.evaluate((el) => !el.closest('header'))).toBe(true)
    await shot(page, testInfo, '30-activity-running')
    await shot(page, testInfo, '31-activity-anchor', page.locator('header[role="banner"]'))

    await attachJson(testInfo, '32-activity-metrics', {
      dialog: await capture(dialog),
      caret: await capture(page.locator('body > span[aria-hidden="true"]:has(+ section[role="dialog"])')),
      activeTab: await capture(dialog.getByRole('tab', { selected: true })),
    })

    // Tabs are a connected tablist/tabpanel pair.
    const tablist = dialog.getByRole('tablist', { name: 'Việc nền' })
    await expect(tablist.getByRole('tab')).toHaveCount(2)
    const inbox = tablist.getByRole('tab', { name: /Thông báo/ })
    await inbox.click()
    await expect(inbox).toHaveAttribute('aria-selected', 'true')
    const panel = dialog.getByRole('tabpanel')
    await expect(panel).toHaveAttribute('aria-labelledby', await inbox.getAttribute('id') ?? '')
    await shot(page, testInfo, '33-activity-inbox')
    await inbox.press('ArrowLeft')
    await expect(tablist.getByRole('tab', { name: /Đang chạy/ })).toHaveAttribute('aria-selected', 'true')

    // Settings is a nested layer: focus must stay in the dialog after the gear
    // disappears, and the back button returns without dropping focus to body.
    await dialog.getByRole('button', { name: 'Cài đặt thông báo' }).first().click()
    const back = dialog.getByRole('button', { name: 'Quay lại Việc nền' })
    await expect(back).toBeVisible()
    await expect(dialog.getByRole('region', { name: 'Cài đặt thông báo' })).toBeVisible()
    await expect(dialog).toBeFocused()
    await shot(page, testInfo, '34-activity-settings')
    await back.click()
    await expect(dialog.getByRole('tablist', { name: 'Việc nền' })).toBeVisible()
    await expect(dialog).toBeFocused()

    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
    // Known gap in the frozen served bundle (anchor is the wrapper div, so focus
    // fell to body). Source repair is in ActivityPopover focusTrigger; soft so
    // the rest of the outside-press proof still runs.
    await expect.soft(pill).toBeFocused()

    // Outside click closes too (portalled dialog + anchor exclusion).
    await pill.click()
    await expect(dialog).toBeVisible()
    await expect(pill).toHaveAttribute('aria-expanded', 'true')
    await page.locator('header[role="banner"]').click({ position: { x: 600, y: 6 } })
    await expect(dialog).toBeHidden()
    await expect(pill).toHaveAttribute('aria-expanded', 'false')
  })
})

test.describe('offline banner', () => {
  // Real outage only: the banner needs /api/status + SSE to fail. No fake route,
  // no fake data. Operator stops/recovers the API outside this harness.
  test.skip(process.env.PC_OFFLINE !== '1', 'outside API state: set PC_OFFLINE=1 during a real outage')

  test('explanation is visible, retry is real', async ({ page }, testInfo) => {
    await openApp(page)
    const banner = page.getByRole('alert').filter({ hasText: 'Mất kết nối với máy chủ Viderse' })
    await expect(banner).toBeVisible()
    await shot(page, testInfo, '40-offline-banner', banner)
    await attachJson(testInfo, '41-offline-metrics', { banner: await capture(banner) })
    // Body must be readable without any click (board-authoritative).
    await expect(banner).toContainText('Video đang chạy trên máy vẫn tiếp tục')
    const retry = banner.getByRole('button', { name: 'Thử lại ngay' })
    await expect(retry).toBeVisible()
    // A retry is a GET/SSE reconnect, not a user-job approval.
    await retry.click()
    await shot(page, testInfo, '42-offline-retry-clicked')
  })
})

test.describe('confirm dialog', () => {
  // Opening a confirm requires a real destructive affordance with real data and
  // is a product write path; run it only when the operator names a safe trigger.
  test.skip(!process.env.PC_CONFIRM_URL || !process.env.PC_CONFIRM_TRIGGER, 'outside API state: set PC_CONFIRM_URL + PC_CONFIRM_TRIGGER for a read-only confirm entry')

  test('focus goes to the safe action, Escape cancels', async ({ page }, testInfo) => {
    await page.goto(process.env.PC_CONFIRM_URL!, { waitUntil: 'domcontentloaded' })
    await page.evaluate(() => document.fonts.ready)
    await page.getByRole('button', { name: process.env.PC_CONFIRM_TRIGGER!, exact: true }).first().click()
    const dialog = page.getByRole('alertdialog')
    await expect(dialog).toBeVisible()
    await shot(page, testInfo, '50-confirm')
    await attachJson(testInfo, '51-confirm-metrics', { dialog: await capture(dialog) })
    const cancel = dialog.getByRole('button', { name: process.env.PC_CONFIRM_CANCEL ?? 'Huỷ' })
    await expect(cancel).toBeVisible()
    await expect(cancel).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
  })
})

test.describe('canonical boards', () => {
  test('capture Main/Activity/Auth/Kit/Dialogs/System at 1440', async ({ page }, testInfo) => {
    const boards = ['Main', 'Activity', 'Auth', 'Kit', 'Dialogs', 'System']
    let loaded = 0
    for (const board of boards) {
      for (const prefix of ['', '/artboards/']) {
        const url = `${CANON}${prefix}${board}.dc.html`
        try {
          const res = await page.goto(url, { waitUntil: 'networkidle', timeout: 20_000 })
          if (res?.ok()) {
            await page.evaluate(() => document.fonts.ready)
            await shot(page, testInfo, `60-canon-${board}`)
            loaded++
            break
          }
        } catch {
          // try next prefix
        }
      }
    }
    test.skip(loaded === 0, `canonical base ${CANON} did not serve any board html`)
    expect(loaded).toBeGreaterThan(0)
  })
})
