import { test, expect, type Page, type TestInfo, type Locator } from '@playwright/test'
import { readFileSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'

// PR2 / V36 Template state audit. Native UI only: no DOM or API-response
// mocking, no provider call, no template write. The counted real cover trial is
// read through GET and opened with the existing Xem ảnh control; the draw
// button is never pressed. Every state is captured on all configured projects
// (desktop/mobile, normal/reduced motion).
const repo = resolve(__dirname, '../..')
const cookiePath = process.env.WORKFLOW_QA_COOKIE_FILE
const fixturePath = process.env.WORKFLOW_QA_FIXTURE
if (!cookiePath || !fixturePath || realpathSync(cookiePath).startsWith(repo + sep)) throw new Error('Private isolated QA session required')
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as {
  username: string
  templates: { id: string; name: string }[]
  jobs: { id: string }[]
}
if (fixture.username !== 'workflow.qa') throw new Error('Only workflow.qa is authorized')
if (!fixture.templates[0]?.id) throw new Error('The isolated fixture must expose templates[0]')
const templateId = fixture.templates[0].id
const runtimeFaults = new WeakMap<Page, string[]>()

type TemplateDetail = {
  id: string
  name: string
  revision: number
  preview: { state: string }
  prompts: { coverId: string; coverName: string }
  recipe: { scenes: { coverPromptId: string; coverSource: 'ai' | 'upload'; coverMediaId?: string } }
}
type CoverTrial = {
  id: string
  jobId: string
  jobTitle: string
  promptId: string
  promptName: string
  composition: string
  state: string
  hookText?: string
  width?: number
  height?: number
  costImages: number
  createdAt: string
  imageUrl?: string
}
type CoverTrialStatus = {
  sources: { jobId: string; title: string; durationSec: number }[]
  history: CoverTrial[]
  quota?: { paid: boolean; left: number; total: number; label: string }
  cost: { images: number; provider: string; label: string; quotaLeft: number }
  ready: boolean
  blockedReason?: string
}
type ImagePrompt = { id: string; name: string; composition?: string; sampleUrls: string[] }

const COMPOSITION_LABEL: Record<string, string> = {
  clean: 'Ảnh sạch',
  hook: 'Chữ hook',
  'circle-arrow': 'Khoanh tròn + mũi tên',
  'hook-arrow': 'Chữ hook + mũi tên',
  split: 'Trước / sau',
  closeup: 'Cận mặt',
}
const PREVIEW_LABEL: Record<string, string> = {
  none: 'Chưa có bản xem thử',
  fresh: 'Khớp cài đặt',
  stale: 'Cần dựng lại',
  building: 'Đang dựng',
}

function compositionLabel(value: string | undefined): string {
  return value ? COMPOSITION_LABEL[value] ?? value : ''
}

/** Fonts + finite animations. Never call this while a 4s toast is live: wait its entrance animation instead. */
async function settle(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise(done => requestAnimationFrame(() => requestAnimationFrame(() => requestAnimationFrame(done))))
  })
  await page.waitForFunction(() => document.getAnimations()
    .filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime))
    .every(animation => animation.playState === 'finished' || animation.playState === 'idle'), undefined, { timeout: 10_000 })
}
async function capture(page: Page, info: TestInfo, name: string) {
  await settle(page)
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
  await page.screenshot({ path: info.outputPath(`${name}.png`) })
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
async function decoded(locator: Locator, width: number, height: number) {
  await expect.poll(() => locator.evaluate((image, dimensions) => {
    const node = image as HTMLImageElement
    return node.complete && node.naturalWidth === dimensions.width && node.naturalHeight === dimensions.height
  }, { width, height })).toBe(true)
}
async function dialogMotion(dialog: Locator) {
  return dialog.evaluate(node => {
    const style = getComputedStyle(node)
    return {
      animationName: style.animationName,
      animationDuration: style.animationDuration,
      animations: node.getAnimations({ subtree: true }).map(animation => ({
        playState: animation.playState,
        duration: Number(animation.effect?.getComputedTiming().duration ?? 0),
      })),
    }
  })
}
async function blockMutations(page: Page) {
  const writes: string[] = []
  await page.route('**/api/**', route => {
    const request = route.request()
    if (['GET', 'HEAD', 'OPTIONS'].includes(request.method())) return route.continue()
    writes.push(`${request.method()} ${new URL(request.url()).pathname}`)
    return route.abort('blockedbyclient')
  })
  return writes
}
async function getTemplate(page: Page): Promise<TemplateDetail> {
  const response = await page.request.get(`/api/templates/${encodeURIComponent(templateId)}`)
  expect(response.status()).toBe(200)
  return await response.json() as TemplateDetail
}
async function getCoverStatus(page: Page): Promise<CoverTrialStatus> {
  const response = await page.request.get(`/api/templates/${encodeURIComponent(templateId)}/cover-trials`)
  expect(response.status()).toBe(200)
  return await response.json() as CoverTrialStatus
}
async function getCoverPrompts(page: Page): Promise<ImagePrompt[]> {
  const response = await page.request.get('/api/image-prompts?kind=cover')
  expect(response.status()).toBe(200)
  return (await response.json() as { items: ImagePrompt[] }).items
}
async function getPreviewState(page: Page): Promise<string> {
  const response = await page.request.get(`/api/templates/${encodeURIComponent(templateId)}/preview`)
  expect(response.status()).toBe(200)
  return (await response.json() as { state: string }).state
}
function mobile(page: Page) {
  return page.viewportSize()!.width < 768
}
async function coverRegion(page: Page): Promise<Locator> {
  if (mobile(page)) await expect(page.getByRole('banner')).toContainText('Ảnh bìa')
  else await expect(page.getByRole('heading', { name: 'Ảnh bìa', exact: true })).toBeVisible()
  return page.getByRole('region', { name: 'Ảnh bìa', exact: true })
}
async function openCover(page: Page) {
  await page.goto(`/#/templates/${encodeURIComponent(templateId)}?section=cover`, { waitUntil: 'domcontentloaded' })
  return coverRegion(page)
}

test.beforeEach(async ({ context, page }) => {
  const faults: string[] = []
  runtimeFaults.set(page, faults)
  page.on('pageerror', error => faults.push(error.message))
  await context.addCookies(JSON.parse(readFileSync(cookiePath, 'utf8')))
  const identity = await context.request.get('/api/me')
  expect(identity.status()).toBe(200)
  expect((await identity.json()).user.username).toBe('workflow.qa')
})
test.afterEach(async ({ page }, info) => {
  // Keep the same fail-on-runtime-error contract as viewer-audit.pw.ts.
  const faults = runtimeFaults.get(page) ?? []
  await info.attach('runtime-errors.json', { body: JSON.stringify(faults), contentType: 'application/json' })
  expect(faults).toEqual([])
})

test('template default and cover tab bind native section state without writes', async ({ page }, info) => {
  const writes = await blockMutations(page)
  const detail = await getTemplate(page)
  const viewport = page.viewportSize()!
  const onMobile = mobile(page)

  if (onMobile) {
    await page.goto(`/#/templates/${encodeURIComponent(templateId)}`, { waitUntil: 'domcontentloaded' })
    const settings = page.getByRole('region', { name: 'Cài đặt', exact: true })
    await expect(settings).toBeVisible()
    const coverRow = settings.getByRole('button').filter({ hasText: 'Ảnh bìa' }).first()
    await expect(coverRow).toBeVisible()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await capture(page, info, 'template-default-mobile-list')

    await coverRow.click()
    await expect(page).toHaveURL(new RegExp(`section=cover`))
    const cover = await coverRegion(page)
    await expect(page.getByRole('status').filter({ hasText: /Đã lưu · \d{2}:\d{2}/ }).first()).toBeVisible()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect(page.locator('[aria-modal="true"]')).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width)
    await capture(page, info, 'template-cover-tab-mobile')

    await page.getByRole('button', { name: 'Quay lại', exact: true }).click()
    await expect(page).not.toHaveURL(/section=cover/)
    await expect(page.getByRole('region', { name: 'Cài đặt', exact: true })).toBeVisible()
    await capture(page, info, 'template-cover-back-mobile')
  } else {
    await page.goto(`/#/templates/${encodeURIComponent(templateId)}`, { waitUntil: 'domcontentloaded' })
    const nav = page.getByRole('navigation', { name: 'Mục cài đặt mẫu video', exact: true })
    await expect(nav).toBeVisible()
    const imagesLink = nav.getByRole('link').filter({ hasText: 'Ảnh cảnh' }).first()
    const coverLink = nav.getByRole('link').filter({ hasText: 'Ảnh bìa' }).first()
    await expect(imagesLink).toHaveAttribute('aria-current', 'page')
    await expect(coverLink).not.toHaveAttribute('aria-current', 'page')
    await expect(page.getByRole('heading', { name: 'Ảnh cảnh', exact: true })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Xem thử', exact: true })).toBeVisible()
    const previewState = await getPreviewState(page)
    await expect(page.locator('.vt-badge')).toHaveAttribute('data-state', previewState)
    await expect(page.locator('.vt-badge')).toHaveText(PREVIEW_LABEL[previewState] ?? /./)
    await expect(page.getByText(/Đã lưu · \d{2}:\d{2}/).first()).toBeVisible()
    await capture(page, info, 'template-default-desktop-images')

    await coverLink.click()
    await expect(page).toHaveURL(new RegExp(`section=cover`))
    await expect(coverLink).toHaveAttribute('aria-current', 'page')
    await expect(imagesLink).not.toHaveAttribute('aria-current', 'page')
    await expect(page.getByRole('heading', { name: 'Ảnh bìa', exact: true })).toBeVisible()
    await expect(page.getByText(/Đã lưu · \d{2}:\d{2}/).first()).toBeVisible()
    await capture(page, info, 'template-cover-tab-desktop')

    await imagesLink.click()
    await expect(page).toHaveURL(new RegExp(`section=images`))
    await expect(imagesLink).toHaveAttribute('aria-current', 'page')
    await expect(coverLink).not.toHaveAttribute('aria-current', 'page')
    await expect(page.getByRole('heading', { name: 'Ảnh cảnh', exact: true })).toBeVisible()
    await capture(page, info, 'template-images-return-desktop')
  }

  expect(writes).toEqual([])
  const readback = await getTemplate(page)
  expect(readback.revision).toBe(detail.revision)
  await attach(info, 'template-section-state', {
    templateId,
    viewport,
    productState: onMobile ? 'mobile list → cover child screen → back' : 'default images → cover tab → images return',
    noWrites: true,
    revisionBefore: detail.revision,
    revisionAfter: readback.revision,
    previewState: onMobile ? null : await getPreviewState(page),
  })
})

test('cover prompt samples keep the saved cover selection and expose the correct group label', async ({ page }, info) => {
  const writes = await blockMutations(page)
  const detail = await getTemplate(page)
  const prompts = await getCoverPrompts(page)
  const cover = await openCover(page)

  const selected = prompts.find(prompt => prompt.id === detail.recipe.scenes.coverPromptId)
  expect(selected, 'The saved cover prompt must still be listed').toBeDefined()
  for (const prompt of prompts) {
    await expect(cover.getByRole('radio', { name: prompt.name, exact: true })).toBeVisible()
    await expect(cover.getByRole('radio', { name: prompt.name, exact: true })).toHaveAttribute('aria-checked', prompt.id === selected!.id ? 'true' : 'false')
  }
  await expect(cover.getByRole('radiogroup', { name: 'Chỉ dẫn ảnh bìa', exact: true })).toBeVisible()
  const promptGroups = await cover.getByRole('radiogroup').evaluateAll(nodes => nodes.map(node => node.getAttribute('aria-label')))
  const sourceGroup = cover.getByRole('radiogroup', { name: 'Cách lấy ảnh bìa', exact: true })
  await expect(sourceGroup).toBeVisible()

  const selectedLabel = compositionLabel(selected!.composition)
  if (selectedLabel) await expect(cover.getByText(selectedLabel, { exact: true }).first()).toBeVisible()

  const compositions = [...new Set(prompts.map(prompt => prompt.composition).filter((value): value is string => Boolean(value)))].sort()

  if (mobile(page)) {
    // Round 45 mobile viewer is excluded by plan; the authored mobile cover
    // surface has no eye control and no modal.
    await expect(cover.getByRole('button', { name: /^Xem lớn / })).toHaveCount(0)
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await capture(page, info, 'template-cover-prompt-inventory-mobile')
  } else {
    const previewable = prompts.find(prompt => prompt.sampleUrls[0])
    if (previewable) {
      const eye = cover.getByRole('button', { name: `Xem lớn ${previewable.name}`, exact: true })
      await expect(eye).toBeEnabled()
      await eye.click()
      const dialog = page.getByRole('dialog')
      await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${previewable.name}`)
      const image = dialog.getByRole('img').first()
      await expect(image).toHaveAttribute('src', previewable.sampleUrls[0])
      await expect.poll(() => image.evaluate(node => (node as HTMLImageElement).complete && (node as HTMLImageElement).naturalWidth > 0)).toBe(true)
      await withinViewport(page, dialog)
      await capture(page, info, 'template-cover-prompt-sample-viewer-desktop')
      await page.keyboard.press('Escape')
      await expect(dialog).toHaveCount(0)
      await expect(eye).toBeFocused()
    } else {
      const locked = cover.getByRole('button', { name: /^Xem lớn / }).first()
      await expect(locked).toBeDisabled()
      await locked.hover({ force: true })
      const explanation = page.getByRole('tooltip')
      await expect(explanation).toBeVisible()
      await expect(explanation).toContainText('Chưa có ảnh mẫu')
      await page.mouse.move(0, 0)
      await expect(explanation).toHaveCount(0)
      await expect(page.getByRole('dialog')).toHaveCount(0)
      await capture(page, info, 'template-cover-prompt-sample-locked-desktop')
    }
    await capture(page, info, 'template-cover-prompt-inventory-desktop')
  }

  expect(writes).toEqual([])
  await attach(info, 'cover-composition-inventory', {
    templateId,
    selectedPrompt: { id: selected!.id, name: selected!.name, composition: selected!.composition ?? null, label: selectedLabel || null },
    prompts: prompts.map(prompt => ({
      id: prompt.id,
      name: prompt.name,
      composition: prompt.composition ?? null,
      label: compositionLabel(prompt.composition) || null,
      hasSample: Boolean(prompt.sampleUrls[0]),
    })),
    compositions,
    promptGroupAccessibleNames: promptGroups,
    noWrites: true,
  })
})

test('cover default and source controls mirror the saved recipe without writes', async ({ page }, info) => {
  const writes = await blockMutations(page)
  const detail = await getTemplate(page)
  const cover = await openCover(page)
  const scenes = detail.recipe.scenes
  const sourceGroup = cover.getByRole('radiogroup', { name: 'Cách lấy ảnh bìa', exact: true })
  const expectedSource = scenes.coverSource === 'upload' ? 'Dùng ảnh mặc định' : 'Hoạ sĩ vẽ theo chỉ dẫn'
  const aiRadio = sourceGroup.getByRole('radio').filter({ hasText: 'Hoạ sĩ vẽ theo chỉ dẫn' })
  const uploadRadio = sourceGroup.getByRole('radio').filter({ hasText: 'Dùng ảnh mặc định' })
  await expect(aiRadio).toHaveAttribute('aria-checked', scenes.coverSource === 'ai' ? 'true' : 'false')
  await expect(uploadRadio).toHaveAttribute('aria-checked', scenes.coverSource === 'upload' ? 'true' : 'false')
  await expect(aiRadio).toContainText('Mỗi video một ảnh bìa mới theo chỉ dẫn và câu chuyện.')
  await expect(uploadRadio).toContainText('Mọi video dùng chung một ảnh bìa.')

  const defaultCard = cover.locator('.vt-card').filter({ hasText: 'Ảnh bìa mặc định' })
  await expect(defaultCard).toHaveCount(1)
  const thumb = defaultCard.locator('.vt-cover-thumb').first()
  if (scenes.coverMediaId) {
    await expect(defaultCard.getByText('Đang dùng ảnh bìa mặc định cho mọi video.', { exact: true })).toBeVisible()
    await expect(defaultCard.getByRole('button', { name: 'Bỏ ảnh', exact: true })).toBeVisible()
    await expect(thumb).toHaveAttribute('src', `/api/editor/media/${encodeURIComponent(scenes.coverMediaId)}`)
    await expect.poll(() => thumb.evaluate(node => (node as HTMLImageElement).complete && (node as HTMLImageElement).naturalWidth > 0)).toBe(true)
  } else {
    await expect(defaultCard.getByText(/^Chưa có · /)).toBeVisible()
    await expect(defaultCard.getByRole('button', { name: 'Chọn ảnh', exact: true })).toBeVisible()
    await expect(thumb).toHaveAttribute('aria-hidden', 'true')
  }
  await capture(page, info, `template-cover-default-${mobile(page) ? 'mobile' : 'desktop'}`)
  expect(writes).toEqual([])
  await attach(info, 'cover-default-source-state', {
    templateId,
    coverSource: scenes.coverSource,
    expectedSource,
    coverMediaId: scenes.coverMediaId ?? null,
    sourceRadios: await sourceGroup.getByRole('radio').allTextContents(),
    noWrites: true,
  })
})

test('actual saved cover trial previews in one surface and preserves source, settings and focus', async ({ page }, info) => {
  test.skip(process.env.WORKFLOW_QA_REAL_COVER !== '1', 'Requires the captured authorized real cover trial; this test never draws')
  const status = await getCoverStatus(page)
  const readyRows = status.history.filter(row => row.state === 'ready' && row.imageUrl)
  const sample = readyRows.find(row => row.costImages === 1) ?? readyRows[0]
  expect(sample, 'A ready saved cover trial is required; no image is generated by this test').toBeDefined()
  expect(sample!.width, 'The saved cover trial must expose its decoded width').toBeGreaterThan(0)
  expect(sample!.height, 'The saved cover trial must expose its decoded height').toBeGreaterThan(0)
  const triggerIndex = readyRows.findIndex(row => row.id === sample!.id)
  expect(triggerIndex).toBeGreaterThanOrEqual(0)

  const writes = await blockMutations(page)
  const cover = await openCover(page)
  const source = cover.getByRole('combobox', { name: 'Chọn video đã làm xong để vẽ thử ảnh bìa', exact: true })
  if (status.sources.length > 0) {
    await source.selectOption(status.sources[0].jobId)
    await expect(source).toHaveValue(status.sources[0].jobId)
  } else {
    await expect(source).toHaveCount(0)
  }
  const chosenBefore = await cover.getByRole('radio', { checked: true }).allTextContents()
  const trigger = cover.getByRole('button', { name: 'Xem ảnh', exact: true }).nth(triggerIndex)
  await expect(trigger).toBeVisible()
  await trigger.click()

  if (mobile(page)) {
    const preview = page.getByRole('group', { name: sample!.promptName, exact: true })
    await expect(preview).toBeVisible()
    await expect(page.locator('[aria-modal="true"]')).toHaveCount(0)
    const back = preview.getByRole('button', { name: 'Quay lại Ảnh bìa', exact: true })
    await expect(back).toBeFocused()
    expect(await back.evaluate(button => button.getBoundingClientRect().height)).toBeGreaterThanOrEqual(44)
    const image = preview.getByRole('img')
    await expect(image).toHaveAttribute('src', sample!.imageUrl!)
    await decoded(image, sample!.width!, sample!.height!)
    if (sample!.composition) await expect(preview).toContainText(compositionLabel(sample!.composition))
    await capture(page, info, 'real-cover-mobile-same-surface')
    await back.click()
    await expect(preview).toHaveCount(0)
  } else {
    const viewer = page.getByRole('dialog')
    await expect(viewer).toHaveAttribute('aria-label', /^Xem ảnh /)
    expect(await viewer.getAttribute('aria-label')).toContain(sample!.jobTitle)
    await expect(viewer).toBeVisible()
    await expect(viewer.getByText(sample!.promptName, { exact: true }).first()).toBeVisible()
    await withinViewport(page, viewer)
    const image = viewer.getByRole('img').first()
    await expect(image).toHaveAttribute('src', sample!.imageUrl!)
    await decoded(image, sample!.width!, sample!.height!)
    const motion = await dialogMotion(viewer)
    const reduced = test.info().project.name.includes('reduced')
    if (reduced) expect(motion.animationName).toBe('none')
    else expect(motion.animationName).toContain('kit-dialog')
    await capture(page, info, 'real-cover-desktop-viewer')
    await info.attach('cover-viewer-motion.json', { body: JSON.stringify({ project: test.info().project.name, reduced, ...motion }, null, 2), contentType: 'application/json' })
    await page.keyboard.press('Escape')
    await expect(viewer).toHaveCount(0)
  }

  await expect(trigger).toBeFocused()
  if (status.sources.length > 0) await expect(source).toHaveValue(status.sources[0].jobId)
  expect(await cover.getByRole('radio', { checked: true }).allTextContents()).toEqual(chosenBefore)
  expect(writes).toEqual([])
  const hookTextMatches = sample!.hookText ? await cover.getByText(sample!.hookText, { exact: true }).count() : null
  await capture(page, info, 'real-cover-returned-settings')
  await attach(info, 'cover-trial-state', {
    templateId,
    trialId: sample!.id,
    source: 'authorized actual provider cover trial (reused, not regenerated)',
    composition: sample!.composition,
    compositionLabel: compositionLabel(sample!.composition),
    hookTextPresent: Boolean(sample!.hookText),
    coverSectionHookTextMatches: hookTextMatches,
    dimensions: { width: sample!.width, height: sample!.height },
    costImages: sample!.costImages,
    quota: status.quota ?? null,
    readyToDrawNow: status.ready,
    drawPressed: false,
    writes,
    providerCalls: 0,
    viewport: page.viewportSize(),
    surface: mobile(page) ? 'same in-page cover section (approved #81)' : 'kit Dialog',
  })
})
