import { test, expect, type Locator, type Page, type TestInfo } from '@playwright/test'
import { readFileSync, realpathSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { basename, resolve, sep } from 'node:path'

// Opt-in V37 Main artboard-state audit. It verifies the states that the existing
// viewer/studio/feedback/ZIP audits do not cover, and it only ever renders rows
// produced by the private owned fixture controller:
//   • stage review · issue · quality · paused · done (saved-state replay of one
//     genuine committed job; real steps/artifacts and real published bytes),
//   • metadata ai · pending · writing · failed · edited + persisted history
//     (edited via the real PATCH the panel uses, on an owned copy only),
//   • info panel (four real agents, source, template, backdrop shortage),
//     progress journal expand/collapse and the real ⋯ menu,
//   • scope all/channel, multi-select bundle rules and six distinct video rows
//     with both panels following the selection,
//   • jobs pill idle and queue-paused states (pause/resume is a real normal
//     action, restored exactly),
//   • stage images/render with a real pause→resume transition and the jobs
//     running/offline states (SIGSTOP only the parent-owned native process).
// The network fence permits GET/HEAD and, per test, only the exact owned writes
// named below. No response is mocked, no provider/clone/generation route is
// touched, and no quality score/provenance/output is invented. Mobile is out of
// scope here: V40/PR6 owns the final phone layout and the shared popover is
// desktop-only; phone Main detail is covered by studio-feedback-audit.
//
// Controller (run it, do not run this file bare):
//   python3 /var/folders/.../verse-v36-owner-safe-yqz0cpa_/run-owned-main-states.py \
//     ROOT private-pg-test-clean-env.json private-runtime-env.json \
//     browser-cookies.json EVIDENCE node_modules/.bin/playwright test
// It sets WORKFLOW_QA_MAIN_STATES=1, WORKFLOW_QA_MAIN_FIXTURES,
// WORKFLOW_QA_MAIN_PHASE and inherits WORKFLOW_QA_API_PID.

const repo = resolve(__dirname, '../..')
const cookiePath = process.env.WORKFLOW_QA_COOKIE_FILE
const fixturePath = process.env.WORKFLOW_QA_MAIN_FIXTURES
const OPT_IN = 'WORKFLOW_QA_MAIN_STATES'
const PHASE = process.env.WORKFLOW_QA_MAIN_PHASE ?? 'all'
const routeParams = (page: Page) => new URLSearchParams(new URL(page.url()).hash.split('?')[1] ?? '')

type JobStep = {
  step: string
  status: string
  error?: { code?: string; message?: string } | null
  progress?: { unitsDone: number; unitsTotal: number | null; phase: string } | null
  output?: { data?: Record<string, unknown>; artifacts?: { id: string; path: string; contentType: string }[] } | null
  startedAt?: string | null
  finishedAt?: string | null
}
type Job = {
  id: string
  channelId: string
  source: { title: string; channelTitle?: string }
  stageGroup: string
  activeStep: string | null
  status: string
  pausedAt: string | null
  subline: string
  templateId: string
  templateSnapshot?: { templateName?: string | null } | null
  templateName?: string | null
  agentView?: { id: string; state: string; now?: string | null }[] | null
  steps?: JobStep[] | null
  exportBytes?: number | null
}
type Meta = {
  status: string
  metadata: { title: string; description: string; tags: string[]; editedBy?: string; edited?: Record<string, boolean> } | null
  revision: number
  descriptionFooter?: string | null
  alwaysTags?: string[] | null
}
type History = { history: { at: string; by: string; title: string; parts: string[]; tags: string[] }[] }
type Images = { items: { id: string; kind: string; status: string }[]; manifest?: unknown; quota?: unknown }
type Backdrop = {
  revision: string
  items: { id: string; status: string; startSec: number; endSec: number }[]
  fixedItems?: { id: string; playbackSec: number }[]
  busy: boolean
  rebuilding: boolean
  coverage: { mode: string; videoSec: number; fixedSec: number; channelSec: number }
}
type Score = { total: number; passed: boolean; blockers: { id: string; message: string }[] | null }

type Fixture = {
  version: number
  phase: string
  username: string
  source: { job: string; title: string; channelId: string }
  channel: { channelId: string; name: string }
  jobs: Record<string, string | undefined>
  titles: Record<string, string>
  expectations: { sourceExportBytes: number; qualityScoreTotal: number; qualityMinScore: number; backdropVideos: number }
}

type Gate = {
  allow: { method: string; path: string; action?: 'pause' | 'resume' }[]
  refused: string[]
  external: string[]
  writes: string[]
}

const gates = new WeakMap<Page, Gate>()
const faultsByPage = new WeakMap<Page, string[]>()
const fixturesByPage = new WeakMap<Page, Fixture>()

function loadFixture(): Fixture {
  if (!cookiePath || !fixturePath) throw new Error(`WORKFLOW_QA_COOKIE_FILE, WORKFLOW_QA_MAIN_FIXTURES and ${OPT_IN}=1 are required`)
  if (realpathSync(cookiePath).startsWith(repo + sep)) throw new Error('Cookies must stay outside the repository')
  const value = JSON.parse(readFileSync(fixturePath, 'utf8')) as Fixture
  if (value.username !== 'workflow.qa' || value.version !== 1) throw new Error('Only the isolated workflow.qa fixture is authorized')
  if (value.source.job !== '3225b629-51c8-4f81-8456-9ecacde664ac' || !value.jobs.done || value.jobs.done === value.source.job) throw new Error('The genuine source stays immutable; done must be a new owned replay')
  if (!value.channel?.channelId) throw new Error('fixture channel is required')
  return value
}

function mobilePage(page: Page): boolean {
  return page.viewportSize()!.width < 768
}

async function settle(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => requestAnimationFrame(() => done()))))
  })
  await page.waitForFunction(() => document.getAnimations()
    .filter((animation) => Number.isFinite(animation.effect?.getComputedTiming().endTime))
    .every((animation) => animation.playState === 'finished' || animation.playState === 'idle'), undefined, { timeout: 10_000 })
}

async function attach(info: TestInfo, name: string, value: unknown) {
  await info.attach(`${name}.json`, { body: JSON.stringify(value, null, 2), contentType: 'application/json' })
}

async function capture(page: Page, info: TestInfo, name: string) {
  await settle(page)
  const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth)
  expect(scrollWidth, `${name} must not overflow the viewport horizontally`).toBeLessThanOrEqual(page.viewportSize()!.width)
  await page.screenshot({ path: info.outputPath(`${name}.png`) })
}

async function readJson<T>(page: Page, path: string): Promise<T> {
  const response = await page.request.get(path)
  expect(response.status(), path).toBe(200)
  return response.json() as Promise<T>
}

function allow(gate: Gate, method: string, path: string, action?: 'pause' | 'resume') {
  gate.allow.push({ method, path, action })
}

function clockLabel(value: number): string {
  return `${Math.floor(value / 60)}:${String(Math.floor(value % 60)).padStart(2, '0')}`
}

async function job(page: Page, id: string): Promise<Job> {
  return readJson<Job>(page, `/api/clone/${id}`)
}

async function meta(page: Page, id: string): Promise<Meta> {
  return readJson<Meta>(page, `/api/clone/${id}/metadata`)
}

async function images(page: Page, id: string): Promise<Images> {
  return readJson<Images>(page, `/api/clone/${id}/images`)
}

async function backdrop(page: Page, id: string): Promise<Backdrop> {
  const payload = await readJson<{ backdrop: Backdrop | null }>(page, `/api/clone/${id}/backdrop`)
  if (!payload.backdrop) throw new Error('owned fixture backdrop must exist')
  return payload.backdrop
}

async function score(page: Page, wire: Job): Promise<Score> {
  const step = wire.steps?.find((item) => item.step === 'score')
  const artifact = step?.output?.artifacts?.find((item) => item.path.endsWith('score.json'))
  if (!artifact) throw new Error('owned quality fixture must expose the real score artifact')
  return readJson<Score>(page, `/api/clone/${wire.id}/artifacts/${encodeURIComponent(artifact.id)}`)
}

function panel(page: Page): Locator {
  return page.getByRole('complementary', { name: 'Bảng bên phải' })
}

function studio(page: Page, title: string): Locator {
  return page.getByRole('region', { name: title, exact: true })
}

function pill(page: Page): Locator {
  return page.locator('a[aria-label^="Việc nền"]').first()
}

async function open(page: Page, id: string, title: string) {
  await page.goto(`/#/?video=${id}`, { waitUntil: 'domcontentloaded' })
  await expect(page.locator('main h1')).toHaveText(title)
  await expect(page.locator('main').getByRole('region').first()).toBeVisible()
}

async function openContent(page: Page) {
  await panel(page).getByRole('tab', { name: 'Nội dung đăng', exact: true }).click()
  await expect(panel(page).getByRole('tab', { name: 'Nội dung đăng', exact: true })).toHaveAttribute('aria-selected', 'true')
}

async function openInfo(page: Page) {
  await panel(page).getByRole('tab', { name: 'Thông tin', exact: true }).click()
  await expect(panel(page).getByRole('tab', { name: 'Thông tin', exact: true })).toHaveAttribute('aria-selected', 'true')
}

function requirePhase(page: Page, phases: string[]) {
  const fx = fixturesByPage.get(page)!
  test.skip(PHASE !== 'all' && !phases.includes(PHASE), `phase ${PHASE} does not own this state`)
  return fx
}

function requireJobs(fx: Fixture, names: string[]) {
  for (const name of names) expect(fx.jobs[name], `fixture must provide ${name}`).toBeTruthy()
  return Object.fromEntries(names.map((name) => [name, fx.jobs[name]!])) as Record<string, string>
}

test.beforeEach(async ({ context, page, baseURL }) => {
  test.skip(process.env[OPT_IN] !== '1', `Set ${OPT_IN}=1 and use run-owned-main-states.py to build the owned fixtures`)
  test.skip(!cookiePath || !fixturePath, 'private cookies and owned fixture manifest are required')
  test.skip(mobilePage(page), 'V40/PR6 owns the final phone Main layout; this audit captures 1440x900 desktop and reduced-motion only')
  expect(baseURL, 'playwright config must provide a loopback baseURL').toBeTruthy()
  const fixture = loadFixture()
  fixturesByPage.set(page, fixture)
  const origin = new URL(baseURL!).origin
  const gate: Gate = { allow: [], refused: [], external: [], writes: [] }
  gates.set(page, gate)
  const faults: string[] = []
  faultsByPage.set(page, faults)
  page.on('pageerror', (error) => faults.push(error.message))
  page.on('dialog', (dialog) => { void dialog.dismiss() })
  await page.route('**/*', (route) => {
    const request = route.request()
    let url: URL
    try { url = new URL(request.url()) } catch { return route.continue() }
    if (!['http:', 'https:'].includes(url.protocol)) return route.continue()
    if (url.origin !== origin) {
      gate.external.push(`${request.method()} ${url.origin}${url.pathname}`)
      return route.abort('blockedbyclient')
    }
    const method = request.method()
    if (method === 'GET' || method === 'HEAD' || method === 'OPTIONS') return route.continue()
    gate.writes.push(`${method} ${url.pathname}`)
    if (gate.allow.some((entry) => entry.method === method && entry.path === url.pathname
      && (entry.action === undefined || request.postDataJSON()?.action === entry.action))) return route.continue()
    gate.refused.push(`${method} ${url.pathname}`)
    return route.abort('blockedbyclient')
  })
  await context.addCookies(JSON.parse(readFileSync(cookiePath!, 'utf8')))
  await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin })
  const identity = await context.request.get('/api/me')
  expect(identity.status()).toBe(200)
  expect((await identity.json()).user.username).toBe('workflow.qa')
})

test.afterEach(async ({ page }, info) => {
  const gate = gates.get(page)
  const faults = faultsByPage.get(page) ?? []
  await info.attach('main-state-network.json', {
    body: JSON.stringify({
      refused: gate?.refused ?? [],
      writes: gate?.writes ?? [],
      externalCount: gate?.external.length ?? 0,
      externalSample: (gate?.external ?? []).slice(0, 20),
    }, null, 2),
    contentType: 'application/json',
  })
  await info.attach('main-state-runtime-errors.json', { body: JSON.stringify(faults), contentType: 'application/json' })
  expect(gate?.refused ?? [], 'Unexpected same-origin writes must be fenced and reported').toEqual([])
  expect(faults, 'The real app must render without page errors').toEqual([])
})

test('@main-static Main stages review, issue, quality, pause and done use the real saved data', async ({ page }, info) => {
  const fx = requirePhase(page, ['static'])
  const ids = requireJobs(fx, ['review', 'issue', 'quality', 'paused', 'done'])
  const observations: Record<string, unknown> = {}

  // review — real tiles, enabled approve/redraw, image click routes to Review.
  const reviewWire = await job(page, ids.review)
  expect(reviewWire.stageGroup).toBe('review')
  const reviewImages = await images(page, ids.review)
  await open(page, ids.review, fx.titles.review)
  await expect(page.locator('main header b').first()).toHaveText('Chờ bạn duyệt')
  await expect(studio(page, 'Bộ ảnh chờ duyệt')).toBeVisible()
  await expect(page.locator('main header').getByRole('status')).toHaveText(
    `Đã vẽ xong ${reviewImages.items.length} ảnh. Vẽ lại ảnh nào chưa ưng, hoặc duyệt để Kiểm định dựng và xuất.`)
  await expect(page.getByRole('button', { name: 'Vẽ lại tất cả', exact: true })).toBeEnabled()
  await expect(page.getByRole('button', { name: 'Duyệt & dựng', exact: true })).toBeEnabled()
  const reviewGrid = page.getByRole('listbox', { name: 'Ảnh của video', exact: true })
  await expect(reviewGrid.getByRole('option')).toHaveCount(reviewImages.items.length)
  await capture(page, info, 'main-stage-review')
  await reviewGrid.getByRole('option').first().click()
  await expect.poll(() => new URL(page.url()).hash).toContain(`/review?video=${ids.review}&img=`)
  await page.goto(`/#/?video=${ids.review}`, { waitUntil: 'domcontentloaded' })
  await expect(page.locator('main h1')).toHaveText(fx.titles.review)

  // issue — replayed retryable fault; banner copy comes from error.code, and the
  // only corrective navigation here is the real Settings link.
  const issueWire = await job(page, ids.issue)
  expect(issueWire.stageGroup).toBe('attention')
  await open(page, ids.issue, fx.titles.issue)
  await expect(page.locator('main header b').first()).toHaveText('Cần xử lý')
  await expect(studio(page, 'Phòng vẽ')).toBeVisible()
  await expect(page.locator('main header').getByRole('alert').first()).toContainText('Bước bị gián đoạn. Chạy tiếp video để làm lại.')
  await expect(page.getByRole('button', { name: 'Thử lại', exact: true })).toBeEnabled()
  const settingsLink = page.getByRole('link', { name: /Đăng nhập lại ở Cài đặt/ })
  await expect(settingsLink).toHaveAttribute('href', '#/settings?section=connect')
  await capture(page, info, 'main-stage-issue')
  await settingsLink.click()
  await expect.poll(() => new URL(page.url()).hash).toContain('/settings?section=connect')
  await page.goto(`/#/?video=${ids.issue}`, { waitUntil: 'domcontentloaded' })

  // quality — the real recorded 75/100 gate, real blockers, never an invented %.
  const qualityWire = await job(page, ids.quality)
  expect(qualityWire.stageGroup).toBe('attention')
  const realScore = await score(page, qualityWire)
  expect(realScore.total).toBe(fx.expectations.qualityScoreTotal)
  expect(realScore.passed).toBe(false)
  await open(page, ids.quality, fx.titles.quality)
  await expect(page.locator('main header b').first()).toHaveText('Chưa đạt độ hoàn thiện')
  await expect(studio(page, 'Kiểm định dừng trước khi xuất')).toBeVisible()
  const blockers = (realScore.blockers ?? []).map((item) => item.message).join(' · ')
  await expect(page.locator('main header').getByRole('alert').first()).toHaveText(
    `Độ hoàn thiện ${realScore.total}/100, cần từ ${fx.expectations.qualityMinScore} để tự xuất.${blockers ? ` ${blockers}.` : ''} Sửa rồi bấm Thử lại, hoặc chọn Vẫn xuất.`)
  await expect(page.getByRole('button', { name: 'Vẫn xuất', exact: true })).toBeEnabled()
  await expect(page.getByRole('button', { name: 'Sửa trong Biên tập', exact: true })).toBeEnabled()
  await capture(page, info, 'main-stage-quality')

  // paused — real saved state; resume is deliberately not clicked here, the
  // working test exercises the real pause→resume transition with state readback.
  const pausedWire = await job(page, ids.paused)
  expect(pausedWire.stageGroup).toBe('paused')
  expect(pausedWire.pausedAt).toBeTruthy()
  await open(page, ids.paused, fx.titles.paused)
  await expect(page.locator('main header b').first()).toHaveText('Tạm dừng')
  await expect(studio(page, 'Phòng vẽ')).toBeVisible()
  await expect(page.locator('main header').getByRole('status')).toHaveText(
    'Video đang tạm dừng. Phần đã làm được giữ; video khác vẫn chạy. Bạn có thể góp ý trước khi chạy tiếp.')
  await expect(page.getByRole('button', { name: 'Chạy tiếp', exact: true })).toBeEnabled()
  await expect(page.getByRole('listbox', { name: 'Ảnh của video', exact: true }).getByRole('option')).toHaveCount((await images(page, ids.paused)).items.length)
  await capture(page, info, 'main-stage-paused')

  // done — bản sao thuộc ca kiểm thử, dùng đúng MP4 và điểm đã lưu từ nguồn thật.
  const doneWire = await job(page, ids.done)
  const doneScore = await score(page, doneWire)
  await open(page, ids.done, fx.titles.done)
  await expect(page.locator('main header b').first()).toHaveText('Sẵn sàng đăng')
  await expect(studio(page, 'Thành phẩm')).toBeVisible()
  await expect(page.getByText(`Độ hoàn thiện ${doneScore.total} / 100`, { exact: true })).toBeVisible()
  await expect(page.locator('main header').getByRole('link', { name: 'Tải gói đăng', exact: true }))
    .toHaveAttribute('href', `/api/clone/download?ids=${ids.done}`)
  const video = page.locator('main video').first()
  await expect(video).toBeVisible()
  expect(await video.getAttribute('src')).toBeTruthy()
  await capture(page, info, 'main-stage-done')

  observations['stages'] = {
    review: { stage: reviewWire.stageGroup, images: reviewImages.items.length },
    issue: { stage: issueWire.stageGroup, stepError: issueWire.steps?.find((step) => step.status === 'failed')?.error },
    quality: { stage: qualityWire.stageGroup, score: realScore },
    paused: { stage: pausedWire.stageGroup, pausedAt: pausedWire.pausedAt },
    done: { stage: doneWire.stageGroup, score: doneScore.total, exportBytes: doneWire.exportBytes },
  }
  info.annotations.push({ type: 'fixture-policy', description: 'Stage rows are saved-state replays of one genuine committed job; review/issue/quality/paused use real steps, artifacts and published bytes. The issue fault code is replayed and labelled, not a provider incident.' })
  expect(gates.get(page)!.writes, 'Stage-state reads and real navigation must not write').toEqual([])
  await attach(info, 'main-stage-observations', { project: info.project.name, viewport: page.viewportSize(), ...observations })
})

test('@main-static Main metadata states ai, pending, writing, failed, edited and history persist for real', async ({ page }, info) => {
  const fx = requirePhase(page, ['static'])
  const ids = requireJobs(fx, ['done', 'metaPending', 'metaWriting', 'metaFailed', 'metaEdited'])
  const observations: Record<string, unknown> = {}

  // ai — the genuine written metadata, copy + rewrite form + persisted history.
  const ai = await meta(page, ids.done)
  expect(ai.status).toBe('ai')
  await open(page, ids.done, fx.titles.done)
  await openContent(page)
  await expect(panel(page).getByRole('textbox', { name: 'Tiêu đề', exact: true })).toHaveValue(ai.metadata!.title)
  await expect(panel(page).getByText('Đạo diễn viết', { exact: true })).toBeVisible()
  await page.evaluate(async () => { await navigator.clipboard.writeText('__main_state_pending__') })
  await panel(page).getByRole('button', { name: 'Sao chép', exact: true }).first().click()
  await expect.poll(() => page.evaluate(async () => navigator.clipboard.readText())).toBe(ai.metadata!.title)
  await panel(page).getByRole('button', { name: 'Viết lại', exact: true }).click()
  const rewrite = panel(page).getByRole('group', { name: 'Viết lại', exact: true })
  await expect(rewrite).toBeVisible()
  await expect(rewrite.getByText('Không tốn ảnh. Phần bạn đã sửa tay bị thay; bản cũ vào Lịch sử.', { exact: true })).toBeVisible()
  await expect(rewrite.getByRole('button', { name: 'Tiêu đề', exact: true })).toHaveAttribute('aria-pressed', 'true')
  await rewrite.getByRole('button', { name: 'Tag', exact: true }).click()
  await expect(rewrite.getByRole('button', { name: 'Tag', exact: true })).toHaveAttribute('aria-pressed', 'true')
  await expect(rewrite.getByRole('button', { name: 'Viết lại 2 phần', exact: true })).toBeEnabled()
  await rewrite.getByRole('button', { name: 'Mô tả', exact: true }).click()
  await expect(rewrite.getByRole('button', { name: 'Viết lại 3 phần', exact: true })).toBeEnabled()
  for (const hint of ['Ngắn hơn', 'Gợi tò mò hơn', 'Đúng nội dung hơn']) {
    await rewrite.getByRole('button', { name: hint, exact: true }).click()
    await expect(rewrite.getByRole('textbox', { name: 'Góp ý cho Đạo diễn', exact: true })).toHaveValue(hint)
  }
  await capture(page, info, 'main-content-rewrite')
  for (const part of ['Tiêu đề', 'Mô tả', 'Tag']) await rewrite.getByRole('button', { name: part, exact: true }).click()
  await expect(rewrite.getByRole('button', { name: 'Viết lại 0 phần', exact: true })).toBeDisabled()
  await expect.poll(async () => (await meta(page, ids.done)).revision).toBe(ai.revision)
  await panel(page).getByRole('button', { name: 'Viết lại', exact: true }).click()
  const history = await readJson<History>(page, `/api/clone/${ids.done}/metadata/history`)
  await panel(page).getByRole('button', { name: 'Lịch sử', exact: true }).click()
  const historyCard = panel(page).getByRole('group', { name: 'Lịch sử nội dung', exact: true })
  await expect(historyCard).toBeVisible()
  await expect(historyCard.getByText('Giữ 10 bản gần nhất. Dùng lại một bản cũ thì bản đang dùng cũng vào lịch sử.', { exact: true })).toBeVisible()
  await expect(historyCard.getByRole('button', { name: 'Dùng lại', exact: true })).toHaveCount(history.history.length)
  await capture(page, info, 'main-content-ai')
  await panel(page).getByRole('button', { name: 'Lịch sử', exact: true }).click()

  // pending / writing / failed — states with no invented text or provider output.
  const pending = await meta(page, ids.metaPending)
  expect(pending.status, 'pending state must not publish metadata').toBe('pending')
  expect(pending.metadata).toBeNull()
  await open(page, ids.metaPending, fx.titles.metaPending)
  await openContent(page)
  await expect(panel(page).getByText('Chưa viết tiêu đề, mô tả, tag', { exact: true })).toBeVisible()
  await expect(panel(page).getByRole('textbox', { name: 'Tiêu đề', exact: true })).toHaveCount(0)
  await capture(page, info, 'main-content-pending')

  const writing = await meta(page, ids.metaWriting)
  expect(writing.status).toBe('writing')
  await open(page, ids.metaWriting, fx.titles.metaWriting)
  await openContent(page)
  await expect(panel(page).getByRole('status')).toContainText('Đạo diễn đang viết tiêu đề, mô tả, tag…')
  await capture(page, info, 'main-content-writing')

  const failed = await meta(page, ids.metaFailed)
  expect(failed.status).toBe('failed')
  await open(page, ids.metaFailed, fx.titles.metaFailed)
  await openContent(page)
  await expect(panel(page).getByRole('alert')).toContainText('Chưa viết được tiêu đề, mô tả, tag.')
  await expect(panel(page).getByRole('button', { name: 'Thử lại', exact: true })).toBeEnabled()
  await expect(panel(page).getByRole('link', { name: /Đăng nhập lại ở Cài đặt/ })).toHaveAttribute('href', '#/settings')
  await openInfo(page)
  await expect(panel(page).getByText(fx.titles.metaFailed, { exact: true }).first()).toBeVisible()
  await capture(page, info, 'main-content-failed-info')

  // edited + history — two real PATCH saves on an owned copy, authoritative
  // readback and reload persistence; the copy is deleted by the controller.
  const editedPath = `/api/clone/${ids.metaEdited}/metadata`
  allow(gates.get(page)!, 'PATCH', editedPath)
  await open(page, ids.metaEdited, fx.titles.metaEdited)
  await openContent(page)
  const before = await meta(page, ids.metaEdited)
  expect(before.status).toBe('ai')
  const newTitle = `${before.metadata!.title} · QA sửa`
  const titleBox = panel(page).getByRole('textbox', { name: 'Tiêu đề', exact: true })
  await titleBox.fill(newTitle)
  await titleBox.press('Tab')
  await expect.poll(async () => (await meta(page, ids.metaEdited)).metadata?.title, { timeout: 15_000 }).toBe(newTitle)
  const afterTitle = await meta(page, ids.metaEdited)
  expect(afterTitle.metadata!.editedBy).toBe('owner')
  expect(afterTitle.metadata!.edited?.title).toBe(true)
  await expect(panel(page).getByText('Bạn đã sửa', { exact: true })).toBeVisible()
  await page.reload({ waitUntil: 'domcontentloaded' })
  await expect(page.locator('main h1')).toHaveText(fx.titles.metaEdited)
  await openContent(page)
  await expect(panel(page).getByRole('textbox', { name: 'Tiêu đề', exact: true })).toHaveValue(newTitle)
  const newDescription = `${before.metadata!.description} QA mô tả`
  const descriptionBox = panel(page).getByRole('textbox', { name: 'Mô tả', exact: true })
  await descriptionBox.fill(newDescription)
  await descriptionBox.press('Tab')
  await expect.poll(async () => (await meta(page, ids.metaEdited)).metadata?.description, { timeout: 15_000 }).toContain('QA mô tả')
  const afterDescription = await meta(page, ids.metaEdited)
  await panel(page).getByRole('button', { name: 'Lịch sử', exact: true }).click()
  const editedHistory = await readJson<History>(page, `/api/clone/${ids.metaEdited}/metadata/history`)
  expect(editedHistory.history.length).toBeGreaterThanOrEqual(1)
  const editedCard = panel(page).getByRole('group', { name: 'Lịch sử nội dung', exact: true })
  await expect(editedCard.getByRole('button', { name: 'Dùng lại', exact: true })).toHaveCount(editedHistory.history.length)
  await expect(editedCard.getByText(newTitle, { exact: true }).first()).toBeVisible()
  await expect(editedCard).toContainText(historyTime(editedHistory.history[0].at))
  await capture(page, info, 'main-content-edited-history')
  const gate = gates.get(page)!
  expect(gate.writes.filter((line) => line === `PATCH ${editedPath}`)).toHaveLength(2)
  expect(gate.writes).toHaveLength(2)

  observations['metadata'] = {
    aiStatus: ai.status,
    aiTitle: ai.metadata!.title,
    originalHistory: history.history.length,
    pendingStatus: pending.status,
    writingStatus: writing.status,
    failedStatus: failed.status,
    edited: {
      title: newTitle,
      descriptionLength: newDescription.length,
      editedBy: afterTitle.metadata!.editedBy,
      revision: afterDescription.revision,
      history: editedHistory.history.map((item) => ({ at: item.at, by: item.by, title: item.title })),
    },
  }
  info.annotations.push({ type: 'fixture-policy', description: 'pending/writing/failed are saved metadata sub-states replayed on owned copies with no invented failure text or provider output; edited/history come from two real panel PATCH saves and authoritative readback.' })
  await attach(info, 'main-metadata-observations', { project: info.project.name, viewport: page.viewportSize(), ...observations })
})

function historyTime(at: string): string {
  return new Date(at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
}

test('@main-static Main info panel, real journal and backdrop shortage plus the real overflow menu', async ({ page }, info) => {
  const fx = requirePhase(page, ['static'])
  const ids = requireJobs(fx, ['done'])
  const wire = await job(page, ids.done)
  await open(page, ids.done, fx.titles.done)
  const aside = panel(page)
  await expect(aside.getByRole('tab', { name: 'Thông tin', exact: true })).toHaveAttribute('aria-selected', 'true')
  await expect(aside.getByRole('tab')).toHaveCount(2)

  for (const name of ['Trinh sát', 'Đạo diễn', 'Hoạ sĩ', 'Kiểm định']) {
    await expect(aside.getByText(name, { exact: true })).toBeVisible()
  }
  const agentStates = wire.agentView ?? []
  expect(agentStates).toHaveLength(4)
  const stateLabel: Record<string, string> = { waiting: 'Chờ lượt', running: 'Đang chạy', done: 'Xong', stopped: 'Dừng', needs_you: 'Chờ bạn' }
  for (const agent of agentStates) {
    await expect(aside.getByText(stateLabel[agent.state] ?? agent.state, { exact: true }).first()).toBeVisible()
  }
  await expect(aside.getByText('Nguồn', { exact: true })).toBeVisible()
  await expect(aside.getByText(fx.titles.done, { exact: true }).first()).toBeVisible()
  const templateName = wire.templateSnapshot?.templateName ?? wire.templateName
  if (!templateName) throw new Error('The saved template snapshot must provide its real display name')
  await expect(aside.getByText(templateName, { exact: true }).first()).toBeVisible()

  const realBackdrop = await backdrop(page, ids.done)
  const covered = realBackdrop.coverage.fixedSec + realBackdrop.coverage.channelSec
  const total = realBackdrop.coverage.videoSec
  const clipCount = realBackdrop.items.length + (realBackdrop.fixedItems?.length ?? 0)
  expect(realBackdrop.busy).toBe(false)
  if (covered < total) {
    await expect(aside.getByText(`Nền sẽ lặp ${clockLabel(total - covered)}`, { exact: true })).toBeVisible()
  } else {
    await expect(aside.getByText('Phủ kín video', { exact: true })).toBeVisible()
  }
  await expect(aside.getByText(`${clipCount} clip`, { exact: true })).toBeVisible()
  const addClip = aside.getByRole('button', { name: 'Thêm clip', exact: true })
  if (covered < total) await expect(addClip).toBeEnabled()
  if (fx.expectations.backdropVideos === 0) {
    info.annotations.push({ type: 'state-gap', description: 'The QA library has zero real backdrop clips (controller receipt backdropVideos=0). The shortage card therefore shows the real 0 clip / full loop duration; a covered (non-short) backdrop state cannot be produced without adding synthetic media, which this audit refuses to do.' })
  }
  if (process.env.WORKFLOW_QA_REAL_BACKDROP_ID) {
    const completeId = requireJobs(fx, ['covered']).covered
    const full = await backdrop(page, completeId)
    expect(full.coverage.fixedSec + full.coverage.channelSec).toBeGreaterThanOrEqual(full.coverage.videoSec)
    expect(full.fixedItems).toHaveLength(1)
    expect(full.fixedItems![0].playbackSec).toBe(10)
    await open(page, completeId, fx.titles.covered)
    await expect(panel(page).getByText('Phủ kín video', { exact: true })).toBeVisible()
    await expect(panel(page).getByText('1 clip', { exact: true })).toBeVisible()
    await panel(page).getByText('Phủ kín video', { exact: true }).scrollIntoViewIfNeeded()
    await expect(panel(page).getByText('Phủ kín video', { exact: true })).toBeInViewport()
    const bar = panel(page).getByRole('progressbar', { name: 'Độ phủ nền video', exact: true })
    await expect(bar).toHaveAttribute('aria-valuenow', String(full.coverage.videoSec))
    await expect(bar).toHaveAttribute('aria-valuemax', String(full.coverage.videoSec))
    const geometry = await bar.evaluate((track) => ({
      track: track.getBoundingClientRect().width,
      fill: track.firstElementChild!.getBoundingClientRect().width,
    }))
    expect(Math.abs(geometry.track - geometry.fill)).toBeLessThanOrEqual(1)
    await expect(panel(page).getByRole('button', { name: 'Thêm clip', exact: true })).toHaveCount(0)
    await capture(page, info, 'main-backdrop-covered')
    await attach(info, 'covered-backdrop-evidence', full)
    await open(page, ids.done, fx.titles.done)
  }

  const steps = wire.steps ?? []
  expect(steps.length).toBeGreaterThan(2)
  const toggle = page.getByRole('button', { name: `Nhật ký · ${steps.length} bước`, exact: true })
  await expect(page.getByRole('list', { name: 'Nhật ký', exact: true }).getByRole('listitem')).toHaveCount(2)
  await toggle.click()
  await expect(page.getByRole('list', { name: 'Nhật ký', exact: true }).getByRole('listitem')).toHaveCount(steps.length)
  await expect(page.getByRole('button', { name: 'Thu gọn nhật ký', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Thu gọn nhật ký', exact: true }).click()
  await expect(page.getByRole('list', { name: 'Nhật ký', exact: true }).getByRole('listitem')).toHaveCount(2)

  await page.getByRole('button', { name: 'Thêm thao tác cho video', exact: true }).click()
  const menu = page.getByRole('menu')
  await expect(menu).toBeVisible()
  const download = page.waitForEvent('download')
  await menu.getByRole('menuitem', { name: 'Tải gói đăng', exact: true }).click()
  const bundle = await download
  expect(await bundle.failure()).toBeNull()
  expect(bundle.suggestedFilename()).toMatch(/\.zip$/)
  await page.getByRole('button', { name: 'Thêm thao tác cho video', exact: true }).click()
  await expect(menu.getByRole('menuitem', { name: 'Đánh dấu đã đăng', exact: true })).toBeVisible()
  await expect(menu.getByRole('menuitem', { name: /^Mở trong Biên tập/ })).toBeEnabled()
  const channelItem = menu.getByRole('menuitem', { name: 'Mở kênh', exact: true })
  await capture(page, info, 'main-info-journal-menu')
  await channelItem.click()
  await expect.poll(() => new URL(page.url()).hash).toContain(`/channels/${fx.channel.channelId}`)
  await page.goto(`/#/?video=${ids.done}`, { waitUntil: 'domcontentloaded' })

  expect(gates.get(page)!.writes, 'Info, journal and menu inspection must stay read-only').toEqual([])
  await attach(info, 'main-info-observations', {
    project: info.project.name,
    viewport: page.viewportSize(),
    agents: agentStates,
    backdrop: { covered, total, clipCount, busy: realBackdrop.busy, rebuilding: realBackdrop.rebuilding, revision: realBackdrop.revision },
    journal: { steps: steps.length, closed: 2 },
    menu: { bundle: `/api/clone/download?ids=${ids.done}`, channel: `#/channels/${fx.channel.channelId}` },
  })
})

test('@main-static Main scope, bundle selection and six real video rows follow both panels', async ({ page }, info) => {
  const fx = requirePhase(page, ['static'])
  const ids = requireJobs(fx, ['done', 'review', 'issue', 'quality', 'paused', 'metaEdited'])
  const observations: Record<string, unknown> = {}

  // Đổi phạm vi danh sách không đổi video đang mở (Main, trạng thái scope).
  await open(page, ids.done, fx.titles.done)
  const side = page.getByRole('complementary', { name: 'Video mọi kênh', exact: true })
  await expect(side).toBeVisible()
  await page.getByRole('button', { name: new RegExp(`^${fx.channel.name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`) }).first().click()
  await expect.poll(() => routeParams(page).get('channel')).toBe(fx.channel.channelId)
  const channelId = routeParams(page).get('channel')!
  const scopedSide = page.getByRole('complementary', { name: 'Video của kênh', exact: true })
  await expect(scopedSide).toBeVisible()
  await expect(scopedSide.getByRole('heading', { name: 'Video của kênh', level: 2 })).toBeVisible()
  const scopedJobs = (await readJson<{ jobs: Job[] | null }>(page, `/api/clone?channel=${channelId}`)).jobs ?? []
  expect(scopedJobs.every((item) => item.channelId === channelId)).toBe(true)
  const preserved = routeParams(page).get('video')
  expect(preserved).toBe(ids.done)
  observations['scope'] = { channel: channelId, scopedJobs: scopedJobs.length, videoPreserved: preserved }
  await capture(page, info, 'main-scope-channel')
  await page.goto('/#/?scope=all', { waitUntil: 'domcontentloaded' })
  await expect(page.getByRole('complementary', { name: 'Video mọi kênh', exact: true })).toBeVisible()

  // Six distinct real rows, both panels following the selection.
  await open(page, ids.done, fx.titles.done)
  const rows: { key: string; id: string; title: string; metaTitle: string }[] = []
  for (const key of ['done', 'review', 'issue', 'quality', 'paused', 'metaEdited']) {
    const id = ids[key]
    const state = await meta(page, id)
    const title = fx.titles[key]
    await page.locator(`aside button[title="${title}"]`).first().click()
    await expect.poll(() => routeParams(page).get('video')).toBe(id)
    await expect(page.locator('main h1')).toHaveText(title)
    await expect(panel(page).getByRole('tab', { name: 'Thông tin', exact: true })).toHaveAttribute('aria-selected', 'true')
    await openContent(page)
    await expect(panel(page).getByRole('textbox', { name: 'Tiêu đề', exact: true })).toHaveValue(state.metadata!.title)
    await expect(page.locator(`aside button[title="${title}"]`).first()).toHaveAttribute('aria-current', 'true')
    rows.push({ key, id, title, metaTitle: state.metadata!.title })
  }
  observations['videos'] = rows
  await capture(page, info, 'main-video-selection')

  // Multi-select — only ready videos join the real bundle link; no download.
  await page.goto(`/#/?video=${ids.done}`, { waitUntil: 'domcontentloaded' })
  const checkboxes = page.locator('aside li button[role="checkbox"]')
  await expect(checkboxes).toHaveCount(0)
  await page.getByRole('button', { name: 'Chọn', exact: true }).click()
  const allRows = page.locator('aside li button[title]')
  const rowCount = await allRows.count()
  expect(rowCount).toBeGreaterThan(5)
  await expect(checkboxes).toHaveCount(rowCount)
  await page.locator(`aside li button[role="checkbox"][title="${fx.titles.review}"]`).click()
  const bar = page.getByRole('region', { name: 'Video đã chọn', exact: true })
  await expect(bar.getByText('1 video đã chọn · chưa có video sẵn sàng đăng', { exact: true })).toBeVisible()
  await expect(bar.getByRole('button', { name: 'Tải gói', exact: true })).toBeDisabled()
  await page.locator(`aside li button[role="checkbox"][title="${fx.titles.done}"]`).click()
  await expect(bar.getByText('2 video đã chọn · 1 sẵn sàng đăng', { exact: true })).toBeVisible()
  await expect(bar.getByRole('link', { name: 'Tải gói 1 video', exact: true }))
    .toHaveAttribute('href', `/api/clone/download?ids=${ids.done}`)
  await capture(page, info, 'main-selection-bundle')
  await bar.getByRole('button', { name: 'Xong', exact: true }).click()
  await expect(checkboxes).toHaveCount(0)
  await expect(bar).toHaveCount(0)
  expect(gates.get(page)!.writes, 'Scope, selection and video-row reads must not write').toEqual([])
  await attach(info, 'main-scope-selection-observations', { project: info.project.name, viewport: page.viewportSize(), rows: rowCount, ...observations })
})

test('@main-static Main jobs pill shows the real idle state and a real queue pause that is restored', async ({ page }, info) => {
  const fx = requirePhase(page, ['static'])
  requireJobs(fx, ['done'])
  await page.goto('/#/', { waitUntil: 'domcontentloaded' })
  const status = await readJson<{ running: number; queued: number; paused: boolean }>(page, '/api/status')
  expect(status.running, 'static phase fixture must contain no working job so idle is honestly reachable').toBe(0)
  const jobsPill = pill(page)
  await expect(jobsPill).toHaveAttribute('data-jobs', status.paused ? 'paused' : 'idle')
  if (!status.paused) {
    await expect(jobsPill).toHaveAttribute('aria-label', 'Việc nền: Máy rảnh')
    await expect(jobsPill.locator('span.num')).toHaveText('Máy rảnh')
  }
  await capture(page, info, 'main-jobs-idle')

  const queueBefore = await readJson<{ paused: boolean }>(page, '/api/queue')
  const queuePath = '/api/queue'
  let pausedByTest = false
  try {
    if (!queueBefore.paused) {
      const pause = await page.request.patch(queuePath, { data: { paused: true } })
      expect(pause.status(), 'queue pause must use the normal endpoint the Queue screen calls').toBe(200)
      pausedByTest = true
      await expect.poll(async () => (await readJson<{ paused: boolean }>(page, queuePath)).paused, { timeout: 15_000 }).toBe(true)
    }
    const queue = await readJson<{ paused: boolean; pausedAt: string | null; items: unknown[] }>(page, queuePath)
    expect(queue.paused).toBe(true)
    expect(queue.pausedAt).toBeTruthy()
    await page.goto(`/#/?video=${fx.jobs.done}`, { waitUntil: 'domcontentloaded' })
    const pausedPill = pill(page)
    await expect(pausedPill).toHaveAttribute('data-jobs', 'paused')
    const time = await page.evaluate((at) => new Date(at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit', hour12: false }), queue.pausedAt)
    await expect(pausedPill).toHaveAttribute('aria-label', `Việc nền: Đã tạm dừng lúc ${time} · ${queue.items.length} chờ lượt`)
    await capture(page, info, 'main-jobs-queue-paused')
  } finally {
    if (pausedByTest) {
      const restore = await page.request.patch(queuePath, { data: { paused: false } })
      expect(restore.status(), 'queue pause must be restored through the normal endpoint').toBe(200)
      await expect.poll(async () => (await readJson<{ paused: boolean }>(page, queuePath)).paused, { timeout: 15_000 }).toBe(false)
    }
  }
  await page.goto(`/#/?video=${fx.jobs.done}`, { waitUntil: 'domcontentloaded' })
  await expect(pill(page)).toHaveAttribute('data-jobs', status.paused ? 'paused' : 'idle')
  if (status.paused) info.annotations.push({ type: 'state-gap', description: 'The parent left the shared queue paused, so the idle pill could not be captured in this run; the paused state itself was still asserted.' })
  expect(gates.get(page)!.writes, 'The queue pause is a normal API action, not a mocked browser write').toHaveLength(0)
  info.annotations.push({ type: 'fixture-policy', description: 'Main jobs.paused uses the same normal PATCH /api/queue the Queue screen calls; the flag is restored in finally and its pausedAt/9-shape comes from the server. No queue items or provider work exist.' })
  await attach(info, 'main-jobs-paused-observations', { project: info.project.name, viewport: page.viewportSize(), initial: status, pausedByTest, queuePath })
})

test('@main-working Main images and render stages plus the real pause→resume transition', async ({ page }, info) => {
  const fx = requirePhase(page, ['working'])
  const imageKey = info.project.name === 'desktop-reduced' ? 'imagesReduced' : 'images'
  const ids = requireJobs(fx, [imageKey, 'render'])
  const imagesId = ids[imageKey]
  allow(gates.get(page)!, 'POST', `/api/clone/${imagesId}`, 'pause')
  allow(gates.get(page)!, 'POST', `/api/clone/${imagesId}`, 'resume')

  const wire = await job(page, imagesId)
  expect(wire.stageGroup).toBe('working')
  expect(wire.activeStep).toBe('images')
  const realImages = await images(page, imagesId)
  await open(page, imagesId, fx.titles[imageKey])
  await expect(page.locator('main header b').first()).toHaveText('Đang làm')
  await expect(studio(page, 'Phòng vẽ trực tiếp')).toBeVisible()
  await expect(page.getByText(`Hoạ sĩ · xong ${realImages.items.filter((item) => item.status !== 'failed').length} / ${realImages.items.length} ảnh`, { exact: true })).toBeVisible()
  const grid = page.getByRole('listbox', { name: 'Ảnh của video', exact: true })
  await expect(grid.getByRole('option')).toHaveCount(realImages.items.length)
  await capture(page, info, 'main-stage-images')
  await grid.getByRole('option').first().click()
  await expect.poll(() => routeParams(page).get('img')).toBeTruthy()
  await page.goto(`/#/?video=${imagesId}`, { waitUntil: 'domcontentloaded' })
  await expect(page.locator('main h1')).toHaveText(fx.titles[imageKey])

  // Real pause → authoritative API state → real resume; no worker/provider run.
  await page.getByRole('button', { name: 'Tạm dừng Esc', exact: true }).click()
  await expect.poll(async () => (await job(page, imagesId)).stageGroup, { timeout: 15_000 }).toBe('paused')
  const pausedWire = await job(page, imagesId)
  expect(pausedWire.pausedAt).toBeTruthy()
  await expect(page.locator('main header b').first()).toHaveText('Tạm dừng')
  await expect(page.locator('main header').getByRole('status')).toHaveText(
    'Video đang tạm dừng. Phần đã làm được giữ; video khác vẫn chạy. Bạn có thể góp ý trước khi chạy tiếp.')
  await expect(page.getByRole('button', { name: 'Chạy tiếp', exact: true })).toBeEnabled()
  await capture(page, info, 'main-stage-images-paused')
  await page.getByRole('button', { name: 'Chạy tiếp', exact: true }).click()
  await expect.poll(async () => (await job(page, imagesId)).stageGroup, { timeout: 15_000 }).toBe('working')
  const resumedWire = await job(page, imagesId)
  expect(resumedWire.pausedAt).toBeNull()
  await expect(page.locator('main header b').first()).toHaveText('Đang làm')
  await capture(page, info, 'main-stage-images-resumed')

  // render — real build step running, later steps pending, no invented percent.
  const renderWire = await job(page, ids.render)
  expect(renderWire.stageGroup).toBe('working')
  expect(renderWire.activeStep).toBe('build')
  await open(page, ids.render, fx.titles.render)
  const renderChip = await page.locator('main header b').first().textContent()
  expect(renderChip, 'statusLine derives the real pending export step, so this nuance is recorded rather than corrected').toBe('Đang xuất video')
  await expect(studio(page, 'Dựng & xuất')).toBeVisible()
  await expect(page.getByText('Kiểm định · bước 1 / 3', { exact: true })).toBeVisible()
  const renderRegion = studio(page, 'Dựng & xuất')
  await expect(renderRegion.locator('ol').last().getByRole('listitem')).toHaveCount(3)
  const buildStep = renderWire.steps!.find((step) => step.step === 'build')!
  expect(buildStep.status).toBe('running')
  expect(buildStep.progress?.unitsTotal ?? null).toBeNull()
  await expect(renderRegion.getByText('Đang dựng video', { exact: true }).first()).toBeVisible()
  await expect(renderRegion.getByRole('progressbar', { name: /^Tiến độ/ })).toHaveCount(0)
  await capture(page, info, 'main-stage-render')

  // Working menu — destructive stop/remove stay locked with the real reason.
  await page.getByRole('button', { name: 'Thêm thao tác cho video', exact: true }).click()
  const menu = page.getByRole('menu')
  await expect(menu.getByRole('menuitem', { name: 'Dừng hẳn', exact: true })).toBeVisible()
  const remove = menu.getByRole('menuitem', { name: 'Xoá video', exact: true })
  await expect(remove).toHaveAttribute('aria-disabled', 'true')
  await expect(remove).toContainText('Tạm dừng trước rồi xoá')
  await capture(page, info, 'main-menu-working-locked')
  await page.keyboard.press('Escape')

  const gate = gates.get(page)!
  expect(gate.writes.filter((line) => line.startsWith('POST ')).sort()).toEqual([
    `POST /api/clone/${imagesId}`,
    `POST /api/clone/${imagesId}`,
  ])
  expect(gate.writes).toHaveLength(2)
  await attach(info, 'main-working-stage-observations', {
    project: info.project.name,
    viewport: page.viewportSize(),
    images: { initial: wire.stageGroup, items: realImages.items.length, pausedAt: pausedWire.pausedAt, resumedStage: resumedWire.stageGroup },
    render: { chip: renderChip, build: { status: buildStep.status, unitsTotal: buildStep.progress?.unitsTotal ?? null }, exportStepStatus: renderWire.steps!.find((step) => step.step === 'export')?.status },
  })
})

test('@main-working Main jobs running, working menu and real offline reconnect', async ({ page }, info) => {
  const fx = requirePhase(page, ['working'])
  const ids = requireJobs(fx, ['images', 'render'])
  await open(page, ids.images, fx.titles.images)
  const status = await readJson<{ running: number; queued: number }>(page, '/api/status')
  expect(status.running).toBeGreaterThanOrEqual(2)
  await expect(pill(page)).toHaveAttribute('data-jobs', 'running')
  await expect(pill(page)).toHaveAttribute('aria-label', `Việc nền: ${status.running} đang chạy · ${status.queued} chờ lượt`)
  await capture(page, info, 'main-jobs-running')

  // Offline — stop only the parent-owned native process long enough to miss the
  // real SSE heartbeat; the pill and disabled actions must reflect that state.
  const apiPid = Number(process.env.WORKFLOW_QA_API_PID)
  expect(Number.isSafeInteger(apiPid) && apiPid > 1, 'the parent must supply its owned native QA process PID').toBe(true)
  const comm = execFileSync('ps', ['-p', String(apiPid), '-o', 'comm='], { encoding: 'utf8' }).trim()
  expect(basename(comm)).toBe('viderse-v37-integrated')
  const pauseButton = page.getByRole('button', { name: 'Tạm dừng Esc', exact: true })
  await expect(pauseButton).toBeEnabled()
  let stopped = false
  try {
    process.kill(apiPid, 'SIGSTOP')
    stopped = true
    await expect(pill(page)).toHaveAttribute('data-jobs', 'offline', { timeout: 25_000 })
    await expect(pill(page)).toHaveAttribute('aria-label', 'Việc nền: Mất kết nối · đang thử lại')
    await expect(pauseButton).toBeDisabled()
    // The shell's offline shield blocks the whole workspace and explains why.
    // A tooltip beneath that shield is not an available interaction.
    await expect(page.getByRole('alert').filter({ hasText: 'Mất kết nối với máy chủ Viderse' })).toBeVisible()
    expect(await pauseButton.evaluate((button) => {
      const rect = button.getBoundingClientRect()
      return button.contains(document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2))
    })).toBe(false)
    await page.getByRole('button', { name: 'Thử lại ngay', exact: true }).click()
    await capture(page, info, 'main-jobs-offline')
    process.kill(apiPid, 'SIGCONT')
    stopped = false
    await expect.poll(async () => (await pill(page).getAttribute('data-jobs')), { timeout: 40_000 }).not.toBe('offline')
  } finally {
    if (stopped) process.kill(apiPid, 'SIGCONT')
  }
  await expect(page.locator('main h1')).toHaveText(fx.titles.images)
  await expect(pauseButton).toBeEnabled()
  await attach(info, 'main-jobs-offline-observations', {
    project: info.project.name,
    viewport: page.viewportSize(),
    status,
    apiPid,
    comm,
    recovered: await pill(page).getAttribute('data-jobs'),
  })
})
