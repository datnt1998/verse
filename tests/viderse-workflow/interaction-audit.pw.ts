import { test, expect, type Page, type TestInfo, type Locator } from '@playwright/test'
import { readFileSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'

const repo = resolve(__dirname, '../..')
const fixturePath = process.env.WORKFLOW_QA_FIXTURE
const cookiePath = process.env.WORKFLOW_QA_COOKIE_FILE
if (!fixturePath || !cookiePath) throw new Error('Fresh isolated QA fixture and private cookies are required')
if (realpathSync(cookiePath).startsWith(repo + sep)) throw new Error('Cookies must stay outside the repository')
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as {
  username: string
  channels: { id: string; name: string }[]
  jobs: { id: string; channelId: string; sourceTitle: string }[]
}
if (fixture.username !== 'workflow.qa') throw new Error('Only the isolated workflow.qa fixture is authorized')

async function settle(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready
    // Flush layout/ResizeObserver/React commits before checking transitions.
    await new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(() => requestAnimationFrame(done))))
  })
  await page.waitForFunction(() => document.getAnimations()
    .filter((animation) => Number.isFinite(animation.effect?.getComputedTiming().endTime))
    .every((animation) => animation.playState === 'finished' || animation.playState === 'idle'), undefined, { timeout: 10_000 })
}
async function attach(info: TestInfo, name: string, value: unknown) {
  await info.attach(`${name}.json`, { body: JSON.stringify(value, null, 2), contentType: 'application/json' })
}
async function withinViewport(page: Page, layer: Locator) {
  await settle(page)
  const box = await layer.boundingBox()
  const viewport = page.viewportSize()!
  expect(box).not.toBeNull()
  expect(box!.x).toBeGreaterThanOrEqual(-1)
  expect(box!.y).toBeGreaterThanOrEqual(-1)
  expect(box!.x + box!.width).toBeLessThanOrEqual(viewport.width + 1)
  expect(box!.y + box!.height).toBeLessThanOrEqual(viewport.height + 1)
}
async function sliderError(page: Page) {
  return page.getByRole('radiogroup', { name: 'Cách bắt đầu' }).evaluate((group) => {
    const selected = group.querySelector('[aria-checked="true"]')!.getBoundingClientRect()
    const thumb = group.querySelector('.kit-seg-thumb')!.getBoundingClientRect()
    return { label: group.querySelector('[aria-checked="true"]')!.textContent, edges: [Math.abs(selected.left - thumb.left), Math.abs(selected.top - thumb.top), Math.abs(selected.right - thumb.right), Math.abs(selected.bottom - thumb.bottom)], fontStatus: document.fonts.status }
  })
}

test.beforeEach(async ({ context, page }) => {
  await context.addCookies(JSON.parse(readFileSync(cookiePath, 'utf8')))
  const identity = await context.request.get('/api/me')
  expect(identity.status()).toBe(200)
  expect((await identity.json()).user.username).toBe('workflow.qa')
  page.on('dialog', (dialog) => { void dialog.dismiss() })
})

test('segmented remeasures after real font loading and viewport changes', async ({ page, context }, info) => {
  const client = await context.newCDPSession(page)
  await client.send('Network.enable')
  await client.send('Network.setCacheDisabled', { cacheDisabled: true })
  await client.send('Network.emulateNetworkConditions', { offline: false, latency: 120, downloadThroughput: 1_500_000, uploadThroughput: 750_000 })
  const fontRequests: string[] = []
  page.on('request', (request) => { if (request.resourceType() === 'font') fontRequests.push(new URL(request.url()).pathname) })
  await page.goto('/#/new', { waitUntil: 'domcontentloaded' })
  await expect(page.getByRole('radiogroup', { name: 'Cách bắt đầu' })).toBeVisible()
  const firstFontStatus = await page.evaluate(() => document.fonts.status)
  await settle(page)
  const initial = await sliderError(page)
  expect(initial.fontStatus).toBe('loaded')
  expect(Math.max(...initial.edges)).toBeLessThanOrEqual(2)
  const width = page.viewportSize()!.width
  await page.setViewportSize({ width: width === 390 ? 430 : 1200, height: page.viewportSize()!.height })
  await settle(page)
  const resized = await sliderError(page)
  expect(Math.max(...resized.edges)).toBeLessThanOrEqual(2)
  expect(fontRequests.length).toBeGreaterThan(0)
  await attach(info, 'font-geometry', { firstFontStatus, fontRequests, initial, resized, throttling: { latencyMs: 120, downloadBytesPerSecond: 1_500_000 } })
  await client.detach()
})

test('shared selections: all channel/template choices, font geometry, responsive dismiss and resize', async ({ page }, info) => {
  const mobile = page.viewportSize()!.width < 768
  const job = fixture.jobs[0]
  const response = await page.request.get(`/api/clone/${job.id}`)
  expect(response.ok()).toBe(true)
  const source = (await response.json()).source.url as string
  await page.goto('/#/new', { waitUntil: 'domcontentloaded' })
  await page.getByRole('textbox', { name: 'Link video' }).fill(source)
  const channel = page.getByRole('combobox', { name: 'Kênh', exact: true })
  await expect(channel).toBeVisible()
  const ledger: unknown[] = []
  for (const option of fixture.channels) {
    await channel.click()
    const list = page.getByRole('listbox').last()
    await expect(list).toBeVisible()
    await withinViewport(page, mobile ? page.locator('.m-sheet').last() : list)
    await page.screenshot({ path: info.outputPath(`channel-${fixture.channels.indexOf(option)}.png`) })
    if (mobile) expect((await channel.boundingBox())!.height).toBeGreaterThanOrEqual(44)
    await list.getByRole('option', { name: option.name, exact: true }).click()
    await expect(list).not.toBeVisible()
    await expect(channel).toContainText(option.name)
    await expect(channel).toBeFocused()
    expect(await page.evaluate(() => document.body.style.overflow)).toBe('')
    ledger.push({ role: 'option', name: option.name, expected: 'Selected channel changes visibly after overlay exits; focus/scroll restored', observed: await channel.textContent(), passed: true })
  }
  const template = page.getByRole('combobox', { name: 'Template', exact: true })
  await template.click()
  await expect(page.getByRole('listbox').last()).toBeVisible()
  const count = await page.getByRole('listbox').last().getByRole('option').count()
  const channelsResponse = await page.request.get('/api/owner-channels')
  expect(channelsResponse.ok()).toBe(true)
  const selectedChannel = (await channelsResponse.json()).channels.find((entry: { id: string }) => entry.id === fixture.channels.at(-1)!.id)
  expect(count).toBe(selectedChannel.templates.length + 1)
  await page.keyboard.press('Escape')
  await expect(page.getByRole('listbox')).not.toBeVisible()
  if (mobile) await expect(page.locator('.m-overlay')).toHaveCount(0)
  for (let index = 0; index < count; index++) {
    await test.step(`template option ${index}`, async () => {
      await template.click()
      // The default option's label changes when the real planning response
      // arrives. Its stable ordered option, not a stale text snapshot, is selected.
      await page.getByRole('listbox').last().getByRole('option').nth(index).click()
      if (mobile) await expect(page.locator('.m-overlay')).toHaveCount(0)
      await expect(page.getByRole('listbox')).not.toBeVisible()
      await template.click()
      await expect(page.getByRole('listbox').last().getByRole('option').nth(index)).toHaveAttribute('aria-selected', 'true')
      const snapshot = await page.evaluate(() => ({
        selectedLabel: document.querySelector('[role="listbox"] [aria-selected="true"]')!.textContent!.trim(),
        visibleValue: document.querySelector('[role="combobox"][aria-label="Template"]')!.textContent!.trim(),
      }))
      expect(snapshot.visibleValue).toContain(snapshot.selectedLabel)
      await page.screenshot({ path: info.outputPath(`template-${index}.png`) })
      await page.keyboard.press('Escape')
      if (mobile) await expect(page.locator('.m-overlay')).toHaveCount(0)
      await expect(page.getByRole('listbox')).not.toBeVisible()
      ledger.push({ role: 'option', name: snapshot.selectedLabel, expected: 'Chosen template remains selected and visible without saving', observed: snapshot, passed: true })
    })
  }
  for (const name of mobile ? ['Nhiều video', 'Dán link'] : ['Nhiều link', 'Một link']) {
    await page.getByRole('radio', { name, exact: true }).click()
    await settle(page)
    const geometry = await sliderError(page)
    expect(Math.max(...geometry.edges)).toBeLessThanOrEqual(2)
    await expect(page.getByRole('radio', { name, exact: true })).toHaveAttribute('aria-checked', 'true')
    ledger.push({ role: 'radio', name, expected: 'Mode and indicator change together', observed: geometry, passed: true })
  }
  await channel.click()
  if (mobile) {
    await expect(page.locator('.m-sheet')).toBeVisible()
    await withinViewport(page, page.locator('.m-sheet'))
    await page.mouse.click(5, 80)
    await expect(page.locator('.m-sheet')).not.toBeVisible()
    await channel.click()
    await expect(page.locator('.m-sheet')).toBeVisible()
    await withinViewport(page, page.locator('.m-sheet'))
    const readiness = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/setup/state' && response.request().method() === 'GET')
    await page.setViewportSize({ width: 768, height: 900 })
    const setup = await (await readiness).json()
    if (!setup.complete) {
      // First desktop entry really prompts for machine setup. This QA HOME
      // deliberately has no provider/models; return via browser Back rather
      // than installing tools or fabricating readiness/session flags.
      await page.waitForURL((url) => url.hash.startsWith('#/setup'))
      await page.goBack()
      await page.waitForURL((url) => url.hash.startsWith('#/new'))
      ledger.push({ role: 'browser-back', name: 'First desktop readiness prompt', expected: 'Return to existing route without installing providers or changing readiness', observed: { complete: setup.complete, route: new URL(page.url()).hash }, passed: true })
    }
    await expect(page.locator('.m-sheet')).not.toBeVisible()
    expect(await page.evaluate(() => document.body.style.overflow)).toBe('')
    // The legacy full-page New screen is replaced by quick input in V39.
    // Its desktop/mobile route containers currently remount; this checks the
    // shared overlay cleanup, not draft preservation in the future dialog.
    await page.getByRole('textbox', { name: 'Link video' }).fill(source)
    await channel.click()
    await expect(page.getByRole('listbox')).toBeVisible()
    expect(await page.locator('.m-sheet').count()).toBe(0)
  }
  await page.keyboard.press('Escape')
  await page.setViewportSize(mobile ? { width: 390, height: 844 } : { width: 1440, height: 900 })
  await page.getByRole('textbox', { name: 'Link video' }).fill(source)
  await settle(page)
  await page.screenshot({ path: info.outputPath('selection.png') })
  await attach(info, 'control-ledger', ledger)
})

test('real video menu hands off to one confirmation; cancel, Esc and scrim submit no mutation', async ({ page }, info) => {
  const job = fixture.jobs[0]
  const writes: { method: string; path: string }[] = []
  page.on('request', (request) => {
    const url = new URL(request.url())
    if (url.pathname.startsWith('/api/') && !['GET', 'HEAD', 'OPTIONS'].includes(request.method())) writes.push({ method: request.method(), path: url.pathname })
  })
  await page.goto(`/#/channels/${job.channelId}?tab=videos`, { waitUntil: 'domcontentloaded' })
  const trigger = page.getByRole('button', { name: `Thêm thao tác cho ${job.sourceTitle}`, exact: true })
  await expect(trigger).toBeVisible()
  const ledger: unknown[] = []
  for (const dismissal of ['cancel', 'escape', 'scrim']) {
    await trigger.click()
    await page.getByRole('menuitem', { name: 'Xoá video', exact: true }).click()
    const dialog = page.getByRole('alertdialog')
    await expect(dialog).toBeVisible()
    await expect(dialog).toContainText(job.sourceTitle)
    expect(await page.locator('[role="dialog"], [role="alertdialog"]').count()).toBe(1)
    await withinViewport(page, dialog)
    await page.screenshot({ path: info.outputPath(`confirmation-${dismissal}.png`) })
    if (dismissal === 'cancel') await dialog.getByRole('button', { name: 'Huỷ', exact: true }).click()
    else if (dismissal === 'escape') await page.keyboard.press('Escape')
    else await page.mouse.click(5, 80)
    await expect(dialog).not.toBeVisible()
    expect(await page.evaluate(() => document.body.style.overflow)).toBe('')
    await expect(trigger).toBeFocused()
    const readback = await page.request.get(`/api/clone/${job.id}`)
    expect(readback.status()).toBe(200)
    ledger.push({ role: 'confirmation-dismiss', name: dismissal, expected: 'No mutation; video remains; focus and scroll restored', observed: { readbackStatus: readback.status(), writes: [...writes] }, passed: true })
  }
  expect(writes).toEqual([])
  await attach(info, 'control-ledger', ledger)
})
