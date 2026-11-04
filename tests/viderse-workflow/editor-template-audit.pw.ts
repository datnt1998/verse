import { test, expect, type Locator, type Page, type TestInfo } from '@playwright/test'
import { createHash } from 'node:crypto'
import { readFileSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'

// PR3 / V37. Native UI only: no route.fulfill, no provider call, no image
// generation and no writes to the completed source. The Editor part is an
// owned duplicate of the real completed project; the Template part only
// changes local selection and closes overlays. Both cases are opt-in so a
// normal run cannot touch the isolated QA account.
const repo = resolve(__dirname, '../..')
const cookiePath = process.env.WORKFLOW_QA_COOKIE_FILE
const fixturePath = process.env.WORKFLOW_QA_FIXTURE
if (!cookiePath || !fixturePath || realpathSync(cookiePath).startsWith(repo + sep)) throw new Error('Private isolated QA session required')
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as {
  username: string
  templates: { id: string; name: string }[]
  jobs: { id: string; projectId: string }[]
}
if (fixture.username !== 'workflow.qa') throw new Error('Only workflow.qa is authorized')
if (!fixture.templates[0]?.id) throw new Error('The isolated fixture must expose templates[0]')
const templateId = fixture.templates[0].id

// The completed source is read-only in this audit. The duplicate endpoint
// returns the only project id that may receive edits, and the only project id
// that may be deleted afterwards.
const SOURCE_JOB = '3225b629-51c8-4f81-8456-9ecacde664ac'
const SOURCE_PROJECT = `clone-${SOURCE_JOB}-g1`
const OPT_IN = 'WORKFLOW_QA_EDITOR_TEMPLATE'

const runtimeFaults = new WeakMap<Page, string[]>()

type TimelineCue = { id: string; text: string; startSeconds: number; endSeconds: number }
type TimelineItem = {
  id: string
  type: string
  trackId?: string
  label?: string
  from: number
  durationInFrames: number
  cues?: TimelineCue[]
}
type TimelineTrack = { id: string; name: string; templateSlot?: string; visible?: boolean; muted?: boolean }
type EditorProject = { id: string; name: string; timeline?: { items?: TimelineItem[]; tracks?: TimelineTrack[] } }
type PreviewSource = { jobId: string; title: string; durationSec: number; disabled?: boolean; reason?: string }
type PreviewStatus = { state: string; sources: PreviewSource[] }
type CoverTrialStatus = { sources: { jobId: string; title: string; durationSec: number }[] }
type ImagePrompt = { id: string; name: string; sampleUrls?: string[]; usedBy?: { id: string; name: string }[] }

function sourceLabel(source: PreviewSource) {
  return `${source.title} · ${Math.max(1, Math.round(source.durationSec / 60))} phút${source.disabled && source.reason ? ` — ${source.reason}` : ''}`
}
function coverSourceLabel(source: CoverTrialStatus['sources'][number]) {
  return `${source.title} · ${Math.max(1, Math.round(source.durationSec / 60))} phút`
}

async function settle(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise<void>(done => requestAnimationFrame(() => requestAnimationFrame(() => requestAnimationFrame(() => done()))))
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
function expectSameBox(actual: { x: number; y: number; width: number; height: number } | null, expected: { x: number; y: number; width: number; height: number } | null, label: string) {
  expect(actual, `${label} must stay mounted`).not.toBeNull()
  expect(expected, `${label} must have a measured baseline`).not.toBeNull()
  for (const key of ['x', 'y', 'width', 'height'] as const) expect(Math.abs(actual![key] - expected![key]), `${label}.${key} must not move when the overlay opens`).toBeLessThanOrEqual(0.5)
}
async function readJson<T>(page: Page, path: string): Promise<T> {
  const response = await page.request.get(path)
  expect(response.status(), `GET ${path} must succeed`).toBe(200)
  return await response.json() as T
}
async function readRaw(page: Page, path: string) {
  const response = await page.request.get(path)
  expect(response.status(), `GET ${path} must succeed`).toBe(200)
  return { text: await response.text(), etag: response.headers()['etag'] ?? null }
}
async function waitPreviewText(page: Page, path: string, ready: Locator) {
  await page.goto(path, { waitUntil: 'domcontentloaded' })
  await expect.poll(async () => new URL(page.url()).hash.startsWith('#/setup') || await ready.isVisible(), { timeout: 15_000 }).toBe(true)
  if (!new URL(page.url()).hash.startsWith('#/setup')) return
  const advanced = page.getByRole('link', { name: 'Cài đặt nâng cao', exact: true })
  if (await advanced.count()) await advanced.click()
  await page.goto(path, { waitUntil: 'domcontentloaded' })
}

test.beforeEach(async ({ context, page }) => {
  const faults: string[] = []
  runtimeFaults.set(page, faults)
  page.on('pageerror', error => faults.push(error.message))
  await context.addCookies(JSON.parse(readFileSync(cookiePath, 'utf8')))
  const me = await context.request.get('/api/me')
  expect(me.status()).toBe(200)
  expect(new URL(me.url()).origin, 'this acceptance must run against the native V37 QA API on 127.0.0.1:8043').toBe('http://127.0.0.1:8043')
  expect((await me.json()).user.username).toBe('workflow.qa')
})
test.afterEach(async ({ page }, info) => {
  const faults = runtimeFaults.get(page) ?? []
  await info.attach('runtime-errors.json', { body: JSON.stringify(faults), contentType: 'application/json' })
  expect(faults).toEqual([])
})

test('owned duplicate Editor wires the V37 transport, caption, timeline and shortcut controls without touching the completed source', async ({ page }, info) => {
  test.skip(process.env[OPT_IN] !== '1', `Requires ${OPT_IN}=1 and the isolated workflow.qa fixture`)

  if (page.viewportSize()!.width < 1024) {
    // The implemented Editor is wide-screen only. Read the completed source once
    // (GET only, fenced) to verify the real guard instead of skipping the project.
    const mobileWrites: string[] = []
    await page.route('**/*', route => {
      const request = route.request()
      const url = new URL(request.url())
      if (!url.pathname.startsWith('/api/') || ['GET', 'HEAD', 'OPTIONS'].includes(request.method())) return route.continue()
      mobileWrites.push(`${request.method()} ${url.pathname}`)
      return route.abort('blockedbyclient')
    })
    try {
      await page.goto(`/#/editor/${encodeURIComponent(SOURCE_PROJECT)}`, { waitUntil: 'domcontentloaded' })
      await expect(page.getByText('Cần màn hình rộng ≥ 1024px', { exact: true })).toBeVisible()
      await expect(page.getByRole('tablist', { name: 'Bảng bên trái', exact: true })).toBeHidden()
      await expect(page.locator('section[aria-label="Dòng thời gian"]')).toBeHidden()
      await capture(page, info, 'editor-mobile-guard')
      expect(mobileWrites, 'the narrow guard must not write to the completed source').toEqual([])
      await attach(info, 'editor-template-editor-mobile', {
        viewport: page.viewportSize(),
        guard: 'Cần màn hình rộng ≥ 1024px',
        wideEditorHidden: true,
        writes: mobileWrites,
        source: { jobId: SOURCE_JOB, projectId: SOURCE_PROJECT, access: 'GET read only' },
        providerCalls: 0,
      })
    } finally {
      await page.unroute('**/*')
    }
    return
  }

  const sourceProjectPath = `/api/editor/projects/${encodeURIComponent(SOURCE_PROJECT)}`
  const sourceJobPath = `/api/clone/${encodeURIComponent(SOURCE_JOB)}`
  const sourceImagesPath = `/api/clone/${encodeURIComponent(SOURCE_JOB)}/images`

  const sourceBefore = await readRaw(page, sourceProjectPath)
  expect(sourceBefore.etag, 'the completed source must carry a revision').toBeTruthy()
  const sourceProject = JSON.parse(sourceBefore.text) as EditorProject
  expect(sourceProject.id).toBe(SOURCE_PROJECT)
  const sourceJobBefore = JSON.parse((await readRaw(page, sourceJobPath)).text) as Record<string, unknown>
  const sourceImagesBefore = await readJson<Record<string, unknown>>(page, sourceImagesPath)
  expect(Array.isArray(sourceImagesBefore.items), 'the source must expose the actual image collection').toBe(true)

  const duplicate = await page.request.post(`${sourceProjectPath}/duplicate`, { data: { name: `QA V37 editor-template ${Date.now()}` } })
  expect(duplicate.status(), 'the normal duplicate endpoint must create an owned project').toBe(201)
  const created = await duplicate.json() as { project: EditorProject; revision: string }
  const ownedId = created.project.id
  const ownedName = created.project.name
  expect(ownedId).not.toBe(SOURCE_PROJECT)
  const ownedProjectPath = `/api/editor/projects/${encodeURIComponent(ownedId)}`
  const readOwned = async () => await readJson<EditorProject>(page, ownedProjectPath)

  const writes: { method: string; path: string; body: string | null }[] = []
  const violations: string[] = []
  const staticFontReads: string[] = []
  // A project write is only legal under the returned duplicate id. Any write
  // aimed at the completed source project, its clone job or its images is
  // blocked before it leaves the browser and reported as a fence violation.
  await page.route('**/*', route => {
    const request = route.request()
    const url = new URL(request.url())
    if (!['http:', 'https:'].includes(url.protocol)) return route.continue()
    const staticFont = request.method() === 'GET' && url.protocol === 'https:' && (
      (url.hostname === 'fonts.googleapis.com' && url.pathname === '/css2')
      || (url.hostname === 'fonts.gstatic.com' && url.pathname.startsWith('/s/') && /\.(woff2?|ttf|otf)$/.test(url.pathname))
    )
    if (staticFont) {
      staticFontReads.push(url.href)
      return route.continue()
    }
    if (url.origin !== 'http://127.0.0.1:8043') {
      violations.push(`${request.method()} ${url.origin}${url.pathname}`)
      return route.abort('blockedbyclient')
    }
    if (['GET', 'HEAD', 'OPTIONS'].includes(request.method())) return route.continue()
    const write = { method: request.method(), path: url.pathname, body: request.postData()?.slice(0, 4000) ?? null }
    const sourceRelated = url.pathname.includes(encodeURIComponent(SOURCE_PROJECT))
      || url.pathname.includes(SOURCE_PROJECT)
      || url.pathname.includes(encodeURIComponent(SOURCE_JOB))
      || url.pathname.includes(SOURCE_JOB)
      || url.pathname.startsWith('/api/clone/')
    const ownedWrite = url.pathname === ownedProjectPath && ['PUT', 'PATCH'].includes(request.method())
    if (sourceRelated || !ownedWrite) {
      violations.push(`${write.method} ${write.path}`)
      return route.abort('blockedbyclient')
    }
    writes.push(write)
    return route.continue()
  })

  let deleted = false
  try {
    expect(created.revision).toBeTruthy()
    expect(ownedName.startsWith('QA V37 editor-template '), 'the duplicate must carry the QA-prefixed name').toBe(true)
    await page.goto(`/#/editor/${encodeURIComponent(ownedId)}`, { waitUntil: 'domcontentloaded' })
    await expect(page.getByRole('main', { name: 'Trình chỉnh sửa video', exact: true })).toBeVisible()
    await expect(page.getByRole('heading', { level: 1, name: ownedName, exact: true })).toBeVisible()
    await expect(page.getByRole('tablist', { name: 'Bảng bên trái', exact: true })).toBeVisible()
    const timeline = page.locator('section[aria-label="Dòng thời gian"]')
    await expect(timeline.locator('button[aria-pressed]').first()).toBeVisible()
    await expect(page.getByRole('status').filter({ hasText: /Đã lưu/ }).first()).toBeVisible()
    await capture(page, info, 'editor-owned-loaded')

    // ── 1. Phím tắt: keyboard/button open, floating box, Esc and outside close, focus return.
    const shortcutDialog = page.getByRole('dialog', { name: 'Phím tắt', exact: true })
    const shortcutButton = page.getByRole('button', { name: 'Phím tắt', exact: true })
    const leftPanel = page.locator('#ed-left')
    const previewMain = page.locator('main[aria-label="Xem trước"]')
    const layout = async () => ({
      timeline: await timeline.boundingBox(),
      left: await leftPanel.boundingBox(),
      preview: await previewMain.boundingBox(),
      scrollHeight: await page.evaluate(() => document.documentElement.scrollHeight),
    })
    const layoutBefore = await layout()
    await page.locator('[data-shortcut-scope="preview"]').first().click({ position: { x: 12, y: 12 } })
    await page.keyboard.press('Shift+/')
    await expect(shortcutDialog).toBeVisible()
    await expect(shortcutDialog).toBeFocused()
    await settle(page)
    const layoutOpen = await layout()
    expectSameBox(layoutOpen.timeline, layoutBefore.timeline, 'timeline')
    expectSameBox(layoutOpen.left, layoutBefore.left, 'left panel')
    expectSameBox(layoutOpen.preview, layoutBefore.preview, 'preview')
    expect(layoutOpen.scrollHeight).toBe(layoutBefore.scrollHeight)
    const shortcutBox = await shortcutDialog.boundingBox()
    const shortcutButtonBox = await shortcutButton.boundingBox()
    expect(shortcutBox).not.toBeNull()
    expect(shortcutButtonBox).not.toBeNull()
    expect(shortcutBox!.y, 'the shortcut popover must sit below its trigger, not in the document flow').toBeGreaterThanOrEqual(shortcutButtonBox!.y + shortcutButtonBox!.height - 1)
    expect(await shortcutDialog.evaluate(node => getComputedStyle(node).position)).toBe('absolute')
    expect(shortcutBox!.x).toBeGreaterThanOrEqual(0)
    expect(shortcutBox!.x + shortcutBox!.width).toBeLessThanOrEqual(page.viewportSize()!.width + 1)
    expect(shortcutBox!.y + shortcutBox!.height).toBeLessThanOrEqual(page.viewportSize()!.height + 1)
    await capture(page, info, 'editor-shortcuts-floating')
    await page.keyboard.press('Escape')
    await expect(shortcutDialog).toHaveCount(0)
    await expect(shortcutButton).toBeFocused()
    await shortcutButton.click()
    await expect(shortcutDialog).toBeVisible()
    await shortcutButton.click()
    await expect(shortcutDialog).toHaveCount(0)
    await expect(shortcutButton).toHaveAttribute('aria-expanded', 'false')
    await shortcutButton.click()
    await expect(shortcutDialog).toBeVisible()
    await previewMain.click({ position: { x: 12, y: 12 } })
    await expect(shortcutDialog).toHaveCount(0)
    await shortcutButton.click()
    await expect(shortcutDialog).toBeVisible()
    await expect(shortcutDialog).toBeFocused()
    await page.keyboard.press('Tab')
    await expect(shortcutDialog).toHaveCount(0)
    await shortcutButton.click()
    await page.keyboard.press('Escape')
    await expect(shortcutButton).toBeFocused()

    // ── 2. Vertical tool rail keeps real arrow-key navigation.
    const rail = page.getByRole('tablist', { name: 'Bảng bên trái', exact: true })
    await expect(rail).toHaveAttribute('aria-orientation', 'vertical')
    const tabs = rail.getByRole('tab')
    const tabCount = await tabs.count()
    expect(tabCount).toBeGreaterThan(1)
    const firstTab = tabs.first()
    await firstTab.click()
    await firstTab.focus()
    const firstId = await firstTab.getAttribute('id')
    await firstTab.press('ArrowDown')
    const selectedAfterDown = rail.getByRole('tab', { selected: true })
    expect(await selectedAfterDown.getAttribute('id')).not.toBe(firstId)
    expect(await page.evaluate(() => document.activeElement?.getAttribute('role'))).toBe('tab')
    await selectedAfterDown.press('ArrowUp')
    await expect(tabs.first()).toHaveAttribute('aria-selected', 'true')
    expect(await page.evaluate(() => document.activeElement?.getAttribute('id'))).toBe(firstId)

    // ── 3. Transport: start/end/play/pause and preview mute.
    const ruler = page.getByRole('slider', { name: 'Vị trí phát', exact: true })
    const startButton = page.getByRole('button', { name: 'Về đầu', exact: true })
    const endButton = page.getByRole('button', { name: 'Tới cuối', exact: true })
    const playButton = page.getByRole('button', { name: 'Phát', exact: true })
    const pauseButton = page.getByRole('button', { name: 'Tạm dừng', exact: true })
    await startButton.click()
    await expect(ruler).toHaveAttribute('aria-valuenow', '0')
    await endButton.click()
    const endFrame = Number(await ruler.getAttribute('aria-valuenow'))
    expect(endFrame).toBeGreaterThan(0)
    await startButton.click()
    await playButton.click()
    await expect(pauseButton).toBeVisible()
    await expect.poll(async () => Number(await ruler.getAttribute('aria-valuenow')), { timeout: 10_000 }).toBeGreaterThan(0)
    await pauseButton.click()
    await expect(playButton).toBeVisible()

    await page.getByRole('button', { name: 'Âm lượng xem trước', exact: true }).click()
    const volumeGroup = page.getByRole('group', { name: 'Âm lượng xem trước', exact: true })
    await expect(volumeGroup).toBeVisible()
    const muteButton = volumeGroup.getByRole('button', { name: /^(Tắt|Bật) tiếng xem trước$/ })
    await expect(muteButton).toHaveAttribute('aria-pressed', 'false')
    await muteButton.click()
    await expect(muteButton).toHaveAttribute('aria-pressed', 'true')
    await muteButton.click()
    await expect(muteButton).toHaveAttribute('aria-pressed', 'false')
    await page.keyboard.press('Escape')
    await expect(volumeGroup).toHaveCount(0)

    // ── 4. Caption timing ±0,1 s and replay on a real measured caption.
    await page.getByRole('button', { name: 'Vừa', exact: true }).click()
    const sourceItems = sourceProject.timeline?.items ?? []
    const sourceTracks = sourceProject.timeline?.tracks ?? []
    const caption = sourceItems.find(item => item.type === 'subtitle' && (item.cues?.length ?? 0) > 0)
    expect(caption, `the completed source must contain a caption item (project ${SOURCE_PROJECT})`).toBeDefined()
    const allCues = sourceItems.filter(item => item.type === 'subtitle').flatMap(item => item.cues ?? [])
    const cue = caption!.cues!.find(entry => entry.text.trim() && allCues.filter(other => other.text === entry.text).length === 1) ?? caption!.cues![0]
    const captionTrack = sourceTracks.find(track => track.id === caption!.trackId)
    const captionGroup = page.getByRole('group', { name: `Rãnh ${captionTrack?.name ?? 'Phụ đề'}`, exact: true })
    await expect(captionGroup).toBeVisible()
    const captionClip = captionGroup.getByRole('button', { name: `${cue.text}, ` }).first()
    await expect(captionClip).toBeVisible()
    const captionClipBefore = await captionClip.getAttribute('aria-label')
    await captionClip.click()
    const captionInspector = page.locator('aside[aria-label="Phụ đề"]')
    await expect(captionInspector).toBeVisible()
    const cueStart = async () => {
      const project = await readOwned()
      return (project.timeline?.items ?? []).find(item => item.id === caption!.id)?.cues?.find(entry => entry.id === cue.id)?.startSeconds
    }
    const originalStart = await cueStart()
    expect(originalStart, 'the selected cue must be readable from the owned project').toBeDefined()
    await captionInspector.getByRole('button', { name: '+0,1 s', exact: true }).click()
    await expect.poll(cueStart).toBeGreaterThan(originalStart!)
    const laterStart = (await cueStart())!
    expect(Math.abs((laterStart - originalStart!) - 0.1)).toBeLessThanOrEqual(0.02)
    await expect(captionClip).not.toHaveAttribute('aria-label', captionClipBefore!)
    await captionInspector.getByRole('button', { name: '−0,1 s', exact: true }).click()
    await expect.poll(async () => Math.abs((await cueStart())! - originalStart!)).toBeLessThanOrEqual(0.001)
    const beforeReplay = Number(await ruler.getAttribute('aria-valuenow'))
    await captionInspector.getByRole('button', { name: 'Nghe lại đoạn này', exact: true }).click()
    await expect.poll(async () => (await pauseButton.isVisible()) || Number(await ruler.getAttribute('aria-valuenow')) !== beforeReplay, { timeout: 10_000 }).toBe(true)
    if (await pauseButton.isVisible()) await pauseButton.click()
    await expect(playButton).toBeVisible()
    await capture(page, info, 'editor-caption-controls')

    // ── 5. Track visibility, timeline zoom and the real persisted readback.
    const sceneTrack = sourceTracks.find(track => track.templateSlot === 'scenes') ?? sourceTracks.find(track => sourceItems.some(item => item.trackId === track.id && item.type === 'image'))
    expect(sceneTrack, 'the completed source must expose a scene track').toBeDefined()
    const hideTrack = page.getByRole('button', { name: `Ẩn rãnh ${sceneTrack!.name}`, exact: true }).first()
    const showTrack = page.getByRole('button', { name: `Hiện rãnh ${sceneTrack!.name}`, exact: true }).first()
    await hideTrack.click()
    await expect(showTrack).toHaveAttribute('aria-pressed', 'false')
    await expect.poll(async () => (await readOwned()).timeline?.tracks?.find(track => track.id === sceneTrack!.id)?.visible).toBe(false)
    await showTrack.click()
    await expect(page.getByRole('button', { name: `Ẩn rãnh ${sceneTrack!.name}`, exact: true }).first()).toHaveAttribute('aria-pressed', 'true')
    await expect.poll(async () => (await readOwned()).timeline?.tracks?.find(track => track.id === sceneTrack!.id)?.visible).toBe(true)

    const zoom = page.getByRole('slider', { name: 'Thu phóng dòng thời gian', exact: true })
    const zoomBefore = Number(await zoom.inputValue())
    await page.getByRole('button', { name: 'Phóng to dòng thời gian', exact: true }).click()
    const zoomIn = Number(await zoom.inputValue())
    expect(zoomIn).toBeGreaterThan(zoomBefore)
    await page.getByRole('button', { name: 'Thu nhỏ dòng thời gian', exact: true }).click()
    expect(Number(await zoom.inputValue())).toBeLessThan(zoomIn)
    await page.getByRole('button', { name: 'Vừa', exact: true }).click()

    // ── 6. Split / delete / undo / redo affect only the owned duplicate.
    const sceneItem = sourceItems.find(item => item.type === 'image' && item.trackId === sceneTrack!.id && item.label)
    expect(sceneItem, 'the scene track must contain a labelled image clip to split').toBeDefined()
    const sceneClip = page.getByRole('group', { name: `Rãnh ${sceneTrack!.name}`, exact: true }).getByRole('button', { name: `${sceneItem!.label}, ` }).first()
    await sceneClip.click()
    await expect(page.locator('aside[aria-label="Cảnh"]')).toBeVisible()
    await page.getByRole('button', { name: 'Vừa', exact: true }).click()
    const rulerBox = await ruler.boundingBox()
    expect(rulerBox).not.toBeNull()
    const sceneClipBox = await sceneClip.boundingBox()
    expect(sceneClipBox, 'the scene clip must stay mounted to measure the timeline frame scale').not.toBeNull()
    const targetFrame = sceneItem!.from + Math.floor(sceneItem!.durationInFrames / 2)
    // "Vừa" fits a window of at least 10 s (fitZoomLevel clamps short videos),
    // so the ruler element can be wider than the duration it actually maps.
    // Read the pixel scale from the rendered clip and click above the scene
    // midpoint; deriving x from ruler.width / aria-valuemax would over-shoot.
    const pixelsPerFrame = sceneClipBox!.width / sceneItem!.durationInFrames
    expect(pixelsPerFrame).toBeGreaterThan(0)
    const targetX = sceneClipBox!.x + sceneClipBox!.width / 2 - rulerBox!.x
    await ruler.click({ modifiers: ['Alt'], position: { x: Math.max(2, Math.min(rulerBox!.width - 2, targetX)), y: rulerBox!.height / 2 } })
    const frameTolerance = Math.ceil(1 / pixelsPerFrame) + 1
    await expect.poll(async () => Math.abs(Number(await ruler.getAttribute('aria-valuenow')) - targetFrame)).toBeLessThanOrEqual(frameTolerance)
    const baseCount = (await readOwned()).timeline?.items?.length ?? 0
    const splitButton = page.locator('button[title="Tách tại vị trí phát (S)"]')
    await expect(splitButton).toBeEnabled()
    await splitButton.click()
    await expect.poll(async () => (await readOwned()).timeline?.items?.length, { timeout: 15_000 }).toBeGreaterThan(baseCount)
    const splitCount = (await readOwned()).timeline?.items?.length ?? 0
    await capture(page, info, 'editor-timeline-split')
    const undoButton = page.getByRole('button', { name: 'Hoàn tác', exact: true })
    const redoButton = page.getByRole('button', { name: 'Làm lại', exact: true })
    await undoButton.click()
    await expect.poll(async () => (await readOwned()).timeline?.items?.length, { timeout: 15_000 }).toBe(baseCount)
    await redoButton.click()
    await expect.poll(async () => (await readOwned()).timeline?.items?.length, { timeout: 15_000 }).toBe(splitCount)
    await sceneClip.click()
    await expect(sceneClip).toHaveAttribute('aria-pressed', 'true')
    const deleteButton = page.locator('button[title="Xoá phần đang chọn (⌫)"]')
    await expect(deleteButton).toBeEnabled()
    await deleteButton.click()
    await expect.poll(async () => (await readOwned()).timeline?.items?.length, { timeout: 15_000 }).toBe(baseCount)
    await undoButton.click()
    await expect.poll(async () => (await readOwned()).timeline?.items?.length, { timeout: 15_000 }).toBe(splitCount)
    await undoButton.click()
    await expect.poll(async () => (await readOwned()).timeline?.items?.length, { timeout: 15_000 }).toBe(baseCount)
    await capture(page, info, 'editor-timeline-restored')

    const forbiddenProjectWrites = writes.filter(write => write.path.startsWith('/api/editor/projects/')
      && write.path !== ownedProjectPath
      && !write.path.startsWith(`${ownedProjectPath}/`))
    expect(forbiddenProjectWrites, 'every Editor project write must stay on the returned duplicate id').toEqual([])
    expect(violations, 'no write may target the completed source project, job or images').toEqual([])

    // Leave the duplicate before deleting it, so no autosave/poll can race the
    // cleanup. The source is re-read while the duplicate is still intact.
    await page.goto('/#/', { waitUntil: 'domcontentloaded' })
    await expect(page.locator('[data-editor-root]')).toHaveCount(0)
    const sourceAfterInteractions = await readRaw(page, sourceProjectPath)
    expect(sourceAfterInteractions.text, 'the completed source project JSON must stay byte-identical').toBe(sourceBefore.text)
    expect(sourceAfterInteractions.etag).toBe(sourceBefore.etag)
    const sourceJobAfterInteractions = JSON.parse((await readRaw(page, sourceJobPath)).text) as Record<string, unknown>
    delete sourceJobAfterInteractions.subline
    const sourceJobBaseline = structuredClone(sourceJobBefore)
    delete sourceJobBaseline.subline
    expect(sourceJobAfterInteractions, 'the completed clone job record must stay identical apart from its time-relative subline').toEqual(sourceJobBaseline)
    const sourceImagesAfterInteractions = await readJson<Record<string, unknown>>(page, sourceImagesPath)
    expect(sourceImagesAfterInteractions, 'the completed source image snapshot must stay identical').toEqual(sourceImagesBefore)

    const removed = await page.request.delete(ownedProjectPath)
    expect(removed.status(), 'DELETE must remove only the owned duplicate').toBe(204)
    deleted = true
    expect((await page.request.get(ownedProjectPath)).status()).toBe(404)
    const sourceAfterDelete = await readRaw(page, sourceProjectPath)
    expect(sourceAfterDelete.text, 'deleting the duplicate must not mutate the source project').toBe(sourceBefore.text)
    expect(sourceAfterDelete.etag).toBe(sourceBefore.etag)
    const sourceJobAfterDelete = JSON.parse((await readRaw(page, sourceJobPath)).text) as Record<string, unknown>
    delete sourceJobAfterDelete.subline
    expect(sourceJobAfterDelete).toEqual(sourceJobBaseline)
    const sourceImagesAfterDelete = await readJson<Record<string, unknown>>(page, sourceImagesPath)
    expect(sourceImagesAfterDelete, 'deleting the duplicate must not reclaim or mutate source images').toEqual(sourceImagesBefore)

    await attach(info, 'editor-template-editor', {
      source: { jobId: SOURCE_JOB, projectId: SOURCE_PROJECT, projectSha256: createHash('sha256').update(sourceBefore.text).digest('hex'), projectEtag: sourceBefore.etag },
      owned: { projectId: ownedId, name: ownedName, revision: created.revision, deleted, readback404: true },
      controls: {
        shortcuts: { keyboard: true, button: true, float: 'position:absolute', escClosed: true, outsideClosed: true, focusReturned: true, layoutUnchanged: true },
        verticalRail: { tabCount, arrowKeys: true, ariaOrientation: 'vertical' },
        transport: { startFrame: 0, endFrame, playPause: true, previewMuteToggle: true },
        caption: { itemId: caption!.id, cueId: cue.id, originalStart, laterStart, stepSec: laterStart - originalStart!, replayStartedAndStopped: true },
        trackVisibility: { trackId: sceneTrack!.id, persistedHiddenThenVisible: true },
        zoom: { before: zoomBefore, zoomedIn: zoomIn, fitAfter: true },
        timelineEdit: { baseCount, splitPersisted: splitCount, undoPersisted: baseCount, redoPersisted: splitCount, deletePersisted: baseCount, restored: baseCount },
      },
      fence: { writes, violations, staticFontReads, forbiddenProjectWrites, allowedProjectPrefix: ownedProjectPath },
      sourceUnchanged: { projectBytes: true, projectEtag: true, cloneRecordNormalized: true, images: true, afterInteractions: true, afterDelete: true },
      providerCalls: 0,
      generationRequests: 0,
    })
  } finally {
    if (!deleted) {
      // Never leave a QA duplicate behind. The source id is never used here.
      const cleanup = await page.request.delete(ownedProjectPath)
      expect([204, 404], 'owned duplicate cleanup must succeed even after an assertion fails').toContain(cleanup.status())
      expect((await page.request.get(ownedProjectPath)).status(), 'the owned duplicate must not be orphaned').toBe(404)
    }
    await page.unroute('**/*')
  }
})

test('template video selects float on desktop and open a Sheet on phone while the sample viewer only reads', async ({ page }, info) => {
  test.skip(process.env[OPT_IN] !== '1', `Requires ${OPT_IN}=1 and the isolated workflow.qa fixture`)

  const templatePath = `/api/templates/${encodeURIComponent(templateId)}`
  const preview = await readJson<PreviewStatus>(page, `${templatePath}/preview`)
  expect(['none', 'fresh', 'stale'], `the template preview must not already be building (state ${preview.state})`).toContain(preview.state)
  const buildable = preview.sources.filter(source => !source.disabled)
  expect(preview.sources.length, 'the isolated fixture must expose real preview sources').toBeGreaterThan(0)
  expect(buildable.length, 'the isolated fixture must expose at least one buildable preview source').toBeGreaterThan(0)
  const cover = await readJson<CoverTrialStatus>(page, `${templatePath}/cover-trials`)
  expect(cover.sources.length, 'the isolated fixture must expose real cover-trial sources').toBeGreaterThan(0)

  const writes: string[] = []
  await page.route('**/*', route => {
    const request = route.request()
    const url = new URL(request.url())
    if (!url.pathname.startsWith('/api/') || ['GET', 'HEAD', 'OPTIONS'].includes(request.method())) return route.continue()
    writes.push(`${request.method()} ${url.pathname}`)
    return route.abort('blockedbyclient')
  })

  try {
    if (page.viewportSize()!.width < 768) {
      // ── Phone: the rebuilt-preview source picker and the cover trial source
      // control are Sheets. No round-45 image viewer is opened here.
      const settings = page.getByRole('region', { name: 'Cài đặt', exact: true })
      await waitPreviewText(page, `/#/templates/${encodeURIComponent(templateId)}`, settings)
      await expect(settings).toBeVisible()
      const rebuildName = preview.state === 'fresh' ? 'Dựng lại' : 'Dựng lại bản xem thử'
      const rebuild = page.getByRole('button', { name: rebuildName, exact: true })
      await expect(rebuild).toBeVisible()
      await expect.poll(() => rebuild.getAttribute('aria-disabled'), { message: 'a buildable source must unlock the preview action' }).not.toBe('true')
      await rebuild.scrollIntoViewIfNeeded()
      await settle(page)

      const baseline = {
        settings: await settings.boundingBox(),
        rebuild: await rebuild.boundingBox(),
        scrollHeight: await page.evaluate(() => document.documentElement.scrollHeight),
        overflow: await page.evaluate(() => document.body.style.overflow),
      }
      await rebuild.click()
      const previewSheet = page.getByRole('dialog', { name: 'Chọn video đã làm', exact: true })
      await expect(previewSheet).toBeVisible()
      await settle(page)
      expect(await page.evaluate(() => document.body.style.overflow)).toBe('hidden')
      for (const source of buildable) {
        const row = previewSheet.getByRole('button').filter({ hasText: source.title }).first()
        await expect(row, `phone Sheet must list real buildable source ${source.title}`).toBeVisible()
        await expect(row).toContainText(sourceLabel(source).split(' — ')[0])
      }
      const previewSheetBox = await previewSheet.boundingBox()
      const viewport = page.viewportSize()!
      expect(previewSheetBox).not.toBeNull()
      expect(previewSheetBox!.x).toBeGreaterThanOrEqual(-1)
      expect(previewSheetBox!.y).toBeGreaterThanOrEqual(-1)
      expect(previewSheetBox!.x + previewSheetBox!.width).toBeLessThanOrEqual(viewport.width + 1)
      expect(previewSheetBox!.y + previewSheetBox!.height).toBeLessThanOrEqual(viewport.height + 1)
      expect(await page.getByRole('dialog', { name: /^Xem ảnh/ }).count(), 'no round-45 image viewer on phone').toBe(0)
      await capture(page, info, 'template-mobile-preview-source-sheet')
      await page.keyboard.press('Escape')
      await expect(previewSheet).toHaveCount(0)
      expect(await page.evaluate(() => document.body.style.overflow)).toBe(baseline.overflow)
      expectSameBox(await settings.boundingBox(), baseline.settings, 'mobile settings region')
      expectSameBox(await rebuild.boundingBox(), baseline.rebuild, 'mobile rebuild trigger')
      expect(await page.evaluate(() => document.documentElement.scrollHeight)).toBe(baseline.scrollHeight)

      await rebuild.click()
      await expect(previewSheet).toBeVisible()
      await page.mouse.click(4, 4)
      await expect(previewSheet).toHaveCount(0)

      const coverRegion = page.getByRole('region', { name: 'Ảnh bìa', exact: true })
      await waitPreviewText(page, `/#/templates/${encodeURIComponent(templateId)}?section=cover`, coverRegion)
      await expect(coverRegion).toBeVisible()
      const coverSelect = page.getByRole('combobox', { name: 'Chọn video đã làm xong để vẽ thử ảnh bìa', exact: true })
      await expect(coverSelect).toBeVisible()
      await coverSelect.scrollIntoViewIfNeeded()
      await settle(page)
      const coverBaseline = { region: await coverRegion.boundingBox(), trigger: await coverSelect.boundingBox(), scrollHeight: await page.evaluate(() => document.documentElement.scrollHeight) }
      await coverSelect.click()
      const coverSheet = page.getByRole('dialog', { name: 'Chọn video đã làm xong để vẽ thử ảnh bìa', exact: true })
      await expect(coverSheet).toBeVisible()
      await settle(page)
      const coverList = coverSheet.getByRole('listbox', { name: 'Chọn video đã làm xong để vẽ thử ảnh bìa', exact: true })
      await expect(coverList).toBeVisible()
      await expect(coverList.getByRole('option')).toHaveCount(cover.sources.length)
      for (const source of cover.sources) {
        const option = coverList.getByRole('option', { name: coverSourceLabel(source), exact: true })
        await expect(option).toHaveCount(1)
        expect(await option.evaluate(node => node.getBoundingClientRect().height)).toBeGreaterThanOrEqual(44)
      }
      expect(await page.getByRole('dialog', { name: /^Xem ảnh/ }).count(), 'no round-45 image viewer on phone').toBe(0)
      await capture(page, info, 'template-mobile-cover-source-sheet')
      await page.keyboard.press('Escape')
      await expect(coverSheet).toHaveCount(0)
      await expect(coverSelect).toHaveAttribute('aria-expanded', 'false')
      await expect(coverSelect).toBeFocused()
      expect(await page.evaluate(() => document.body.style.overflow)).not.toBe('hidden')
      expectSameBox(await coverRegion.boundingBox(), coverBaseline.region, 'mobile cover region')
      expectSameBox(await coverSelect.boundingBox(), coverBaseline.trigger, 'mobile cover select')
      expect(await page.evaluate(() => document.documentElement.scrollHeight)).toBe(coverBaseline.scrollHeight)
      expect(writes, 'opening and closing template sheets must not write').toEqual([])

      await attach(info, 'editor-template-template-phone', {
        viewport: page.viewportSize(),
        preview: { state: preview.state, sourceCount: preview.sources.length, buildable: buildable.map(source => source.title), sheetTitle: 'Chọn video đã làm', escClosed: true, scrimClosed: true, layoutUnchanged: true },
        cover: { sourceCount: cover.sources.length, sheetTitle: 'Chọn video đã làm xong để vẽ thử ảnh bìa', options44px: true, escClosed: true, focusReturned: true, layoutUnchanged: true },
        writes,
        viewerRound45Opened: false,
        providerCalls: 0,
      })
      return
    }

    // ── Desktop: both video selects are real floating lists; the template
    // sample eye opens the shared viewer read-only.
    const previewRegion = page.getByRole('region', { name: 'Xem thử', exact: true })
    await waitPreviewText(page, `/#/templates/${encodeURIComponent(templateId)}?section=images`, previewRegion)
    await expect(previewRegion).toBeVisible()
    const previewSelect = page.getByRole('combobox', { name: 'Chọn video để dựng bản xem thử', exact: true })
    await expect(previewSelect).toBeVisible()
    const previewBaseline = { pane: await previewRegion.boundingBox(), trigger: await previewSelect.boundingBox(), scrollHeight: await page.evaluate(() => document.documentElement.scrollHeight) }
    await previewSelect.click()
    const previewList = page.getByRole('listbox', { name: 'Chọn video để dựng bản xem thử', exact: true })
    await expect(previewList).toBeVisible()
    await expect(previewList.getByRole('option')).toHaveCount(preview.sources.length)
    for (const source of preview.sources) {
      const option = previewList.getByRole('option', { name: sourceLabel(source), exact: true })
      await expect(option, `source option must expose its real label${source.disabled ? ' and disabled reason' : ''}`).toHaveCount(1)
      if (source.disabled) {
        expect(source.reason?.trim().length, `disabled source ${source.title} must carry a real reason`).toBeGreaterThan(0)
        await expect(option).toHaveAttribute('aria-disabled', 'true')
      }
    }
    const previewListBox = await previewList.boundingBox()
    expect(previewListBox).not.toBeNull()
    expect(previewListBox!.y).toBeGreaterThanOrEqual((await previewSelect.boundingBox())!.y)
    expect(previewListBox!.x + previewListBox!.width).toBeLessThanOrEqual(page.viewportSize()!.width + 1)
    expect(previewListBox!.y + previewListBox!.height).toBeLessThanOrEqual(page.viewportSize()!.height + 1)
    await capture(page, info, 'template-desktop-preview-source-floating')
    await page.keyboard.press('Escape')
    await expect(previewList).toHaveCount(0)
    await expect(previewSelect).toBeFocused()
    expectSameBox(await previewRegion.boundingBox(), previewBaseline.pane, 'preview pane')
    expectSameBox(await previewSelect.boundingBox(), previewBaseline.trigger, 'preview select')
    expect(await page.evaluate(() => document.documentElement.scrollHeight)).toBe(previewBaseline.scrollHeight)
    await previewSelect.click()
    await previewRegion.getByRole('heading', { name: 'Xem thử', exact: true }).click()
    await expect(previewList).toHaveCount(0)

    const coverRegion = page.getByRole('region', { name: 'Ảnh bìa', exact: true })
    await waitPreviewText(page, `/#/templates/${encodeURIComponent(templateId)}?section=cover`, coverRegion)
    await expect(coverRegion).toBeVisible()
    const coverSelect = page.getByRole('combobox', { name: 'Chọn video đã làm xong để vẽ thử ảnh bìa', exact: true })
    await expect(coverSelect).toBeVisible()
    const coverBaseline = { region: await coverRegion.boundingBox(), trigger: await coverSelect.boundingBox(), scrollHeight: await page.evaluate(() => document.documentElement.scrollHeight) }
    await coverSelect.click()
    const coverList = page.getByRole('listbox', { name: 'Chọn video đã làm xong để vẽ thử ảnh bìa', exact: true })
    await expect(coverList).toBeVisible()
    await expect(coverList.getByRole('option')).toHaveCount(cover.sources.length)
    for (const source of cover.sources) await expect(coverList.getByRole('option', { name: coverSourceLabel(source), exact: true })).toHaveCount(1)
    await capture(page, info, 'template-desktop-cover-source-floating')
    await page.keyboard.press('Escape')
    await expect(coverList).toHaveCount(0)
    await expect(coverSelect).toBeFocused()
    expectSameBox(await coverRegion.boundingBox(), coverBaseline.region, 'cover region')
    expectSameBox(await coverSelect.boundingBox(), coverBaseline.trigger, 'cover select')
    expect(await page.evaluate(() => document.documentElement.scrollHeight)).toBe(coverBaseline.scrollHeight)

    const promptResponse = await readJson<{ items: ImagePrompt[] }>(page, '/api/image-prompts?kind=scene')
    const prompt = promptResponse.items.find(item => (item.sampleUrls?.length ?? 0) > 0)
    expect(prompt, 'the isolated fixture must expose a real scene prompt sample').toBeDefined()
    await waitPreviewText(page, `/#/templates/${encodeURIComponent(templateId)}?section=images`, previewRegion)
    const eye = page.getByRole('button', { name: `Xem lớn ${prompt!.name}`, exact: true }).first()
    await expect(eye).toBeVisible()
    const selectedBefore = await page.getByRole('radio', { checked: true }).allTextContents()
    await eye.click()
    const sampleViewer = page.getByRole('dialog', { name: `Xem ảnh ${prompt!.name}`, exact: true })
    await expect(sampleViewer).toBeVisible()
    await expect.poll(() => sampleViewer.locator('img').first().evaluate(image => (image as HTMLImageElement).complete && (image as HTMLImageElement).naturalWidth > 0)).toBe(true)
    await capture(page, info, 'template-sample-viewer-readonly')
    await page.keyboard.press('Escape')
    await expect(sampleViewer).toHaveCount(0)
    await expect(eye).toBeFocused()
    expect(await page.getByRole('radio', { checked: true }).allTextContents()).toEqual(selectedBefore)
    expect(writes, 'template selection and the sample viewer must not write').toEqual([])

    await attach(info, 'editor-template-template-desktop', {
      viewport: page.viewportSize(),
      preview: {
        state: preview.state,
        sources: preview.sources.map(source => ({ title: source.title, disabled: Boolean(source.disabled), reason: source.reason ?? null, label: sourceLabel(source) })),
        listboxFloating: true,
        escClosed: true,
        outsideClosed: true,
        layoutUnchanged: true,
        buildableCount: buildable.length,
      },
      cover: { sources: cover.sources.map(source => ({ title: source.title, label: coverSourceLabel(source) })), listboxFloating: true, escClosed: true, layoutUnchanged: true },
      sampleViewer: { promptId: prompt!.id, promptName: prompt!.name, decoded: true, escClosed: true, focusReturned: true, selectionUnchanged: true, readOnly: true },
      writes,
      providerCalls: 0,
      generationOrRedrawRequests: 0,
    })
  } finally {
    await page.unroute('**/*')
  }
})
