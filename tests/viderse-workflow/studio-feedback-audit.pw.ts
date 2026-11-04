import { test, expect, type BrowserContext, type Locator, type Page, type Response, type TestInfo } from '@playwright/test'
import { readFileSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'

// Opt-in V37 studio feedback audit. It boots the native Viderse app against the
// isolated workflow.qa fixture and verifies only controls that exist in the
// current source:
//   • Main desktop inspector: default Thông tin, two tabs preserved through a
//     real read-only reconnect refresh and through real video-row selection,
//     with both panels following the selected job,
//   • real clipboard readback for title, description and whole posting package,
//     plus visible toast feedback,
//   • journal expand/collapse against the job's real step list and all four
//     assistant rows against the real agentView payload,
//   • desktop ActivityPopover with real /api/activity + /api/notifications data,
//     outside-click/Escape dismissal and viewport containment,
//   • NotificationSettings' single "Đề xuất của trợ lý" boolean: save,
//     authoritative readback, reload persistence and exact final restore,
//   • phone Main detail: four assistants, real content values, package copy and
//     the real progress journal (the two-tab inspector/popover are desktop-only).
// The network fence permits GET/HEAD and, only in the notification test, the
// exact own PUT /api/notifications/settings. It never fulfils a request with
// mocked data, never disables a feature to make a test pass, and never triggers
// clone/agent/generation/provider routes. External origins are refused.

const repo = resolve(__dirname, '../..')
const cookiePath = process.env.WORKFLOW_QA_COOKIE_FILE
const fixturePath = process.env.WORKFLOW_QA_FIXTURE
const OPT_IN = 'WORKFLOW_QA_STUDIO_FEEDBACK'
if (!cookiePath || !fixturePath) throw new Error(`Private workflow.qa cookies, fixture and ${OPT_IN}=1 are required`)
if (realpathSync(cookiePath).startsWith(repo + sep)) throw new Error('Cookies must stay outside the repository')
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as { username?: string }
if (fixture.username !== 'workflow.qa') throw new Error('Only the isolated workflow.qa fixture is authorized')

type JobStep = { step: string; status: string }
type Agent = { id: string; state: string; now?: string | null }
type Job = {
  id: string
  channelId: string
  source: { title: string }
  stageGroup: string
  templateId: string
  version: number
  steps?: JobStep[] | null
  agentView?: Agent[] | null
}
type Metadata = { title: string; description: string; tags: string[] }
type MetaResponse = { status: string; metadata: Metadata | null; revision: number; descriptionFooter?: string | null }
type Entry = { job: Job; meta: MetaResponse }
type Gate = { allowSettingsWrite: boolean; refused: string[]; external: string[]; writes: string[] }
type NotificationSettings = { kinds: Record<string, unknown>; doNotDisturb: unknown; badge: boolean; merge: boolean; suggestions: boolean }

const gates = new WeakMap<Page, Gate>()
const faultsByPage = new WeakMap<Page, string[]>()

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
async function readJson<T>(page: Page, path: string): Promise<T> {
  const response = await page.request.get(path)
  expect(response.status(), path).toBe(200)
  return response.json() as Promise<T>
}
async function tryJson<T>(page: Page, path: string): Promise<T | null> {
  const response = await page.request.get(path)
  if (response.status() !== 200) return null
  return response.json() as Promise<T>
}

/** Real non-queued jobs with written metadata and distinguishable source rows. */
async function loadEntries(page: Page): Promise<Entry[]> {
  const list = await readJson<{ jobs: Job[] | null }>(page, '/api/clone?channel=all')
  const jobs = list.jobs ?? []
  const titleCount = new Map<string, number>()
  for (const job of jobs) titleCount.set(job.source.title, (titleCount.get(job.source.title) ?? 0) + 1)
  const entries: Entry[] = []
  for (const job of jobs.slice(0, 24)) {
    if (!job.source?.title || titleCount.get(job.source.title) !== 1 || job.stageGroup === 'queued') continue
    const meta = await tryJson<MetaResponse>(page, `/api/clone/${job.id}/metadata`)
    const metadata = meta?.metadata
    if (!metadata?.title || !metadata.description) continue
    entries.push({ job, meta: meta! })
  }
  return entries
}
function selectPair(entries: Entry[]) {
  const a = entries.find((entry) => (entry.job.steps?.length ?? 0) > 2)
  if (!a) return null
  const b = entries.find((entry) => entry.job.id !== a.job.id
    && entry.job.source.title !== a.job.source.title)
  return b ? { a, b } : null
}

async function copyAndAssert(page: Page, button: Locator, expected: string, label: string) {
  await page.evaluate(async () => { await navigator.clipboard.writeText('__studio_feedback_pending__') })
  await button.click()
  await expect.poll(() => page.evaluate(async () => navigator.clipboard.readText()), {
    timeout: 5_000,
    message: `${label} must write the exact value into the real clipboard`,
  }).toBe(expected)
  const actual = await page.evaluate(async () => navigator.clipboard.readText())
  expect(actual, `${label} clipboard value`).toBe(expected)
  const toast = page.locator('.ui-toast-glass').last()
  await expect(toast, `${label} must show visible feedback`).toBeVisible()
  const feedback = ((await toast.textContent()) ?? '').trim()
  expect(feedback.length, `${label} feedback must not be empty`).toBeGreaterThan(0)
  return { expectedLength: expected.length, actualLength: actual.length, feedback }
}

/** Pause the owned native QA process long enough to miss its real SSE heartbeat.
 * Chromium offline emulation does not reliably interrupt an already-open fetch
 * stream; a stopped server cannot send bytes. No response is mocked or edited. */
async function revalidateReadOnly(page: Page, _context: BrowserContext, jobId: string) {
  const seen: string[] = []
  const onResponse = (response: Response) => {
    if (response.request().method() !== 'GET') return
    try {
      const url = new URL(response.url())
      if (url.pathname === '/api/clone' || url.pathname === `/api/clone/${jobId}`) seen.push(`${url.pathname}${url.search}`)
    } catch { /* non-http response */ }
  }
  const apiPid = Number(process.env.WORKFLOW_QA_API_PID)
  expect(Number.isSafeInteger(apiPid) && apiPid > 1, 'the parent must supply its owned native QA process PID').toBe(true)
  page.on('response', onResponse)
  let paused = false
  try {
    process.kill(apiPid, 'SIGSTOP')
    paused = true
    await page.waitForTimeout(5_000)
    process.kill(apiPid, 'SIGCONT')
    paused = false
    await expect.poll(() => seen.length, {
      timeout: 25_000,
      message: 'the read-only reconnect must refetch the current workbench list/job over GET',
    }).toBeGreaterThan(0)
    await settle(page)
  } finally {
    page.off('response', onResponse)
    if (paused) process.kill(apiPid, 'SIGCONT')
  }
  return seen
}

test.beforeEach(async ({ context, page, baseURL }) => {
  test.skip(process.env[OPT_IN] !== '1', `Set ${OPT_IN}=1 to run the opt-in studio feedback audit`)
  expect(baseURL, 'playwright config must provide a loopback baseURL').toBeTruthy()
  const origin = new URL(baseURL!).origin
  const gate: Gate = { allowSettingsWrite: false, refused: [], external: [], writes: [] }
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
    if (gate.allowSettingsWrite && method === 'PUT' && url.pathname === '/api/notifications/settings') return route.continue()
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
  await info.attach('studio-feedback-network.json', {
    body: JSON.stringify({
      refused: gate?.refused ?? [],
      writes: gate?.writes ?? [],
      externalCount: gate?.external.length ?? 0,
      externalSample: (gate?.external ?? []).slice(0, 20),
    }, null, 2),
    contentType: 'application/json',
  })
  await info.attach('studio-feedback-runtime-errors.json', { body: JSON.stringify(faults), contentType: 'application/json' })
  expect(gate?.refused ?? [], 'Unexpected same-origin writes must be fenced and reported').toEqual([])
  expect(faults, 'The real app must render without page errors').toEqual([])
})

test('Main desktop inspector preserves its tab on reconnect, returns to Info for another video, and copies actual values', async ({ page, context }, info) => {
  test.skip(mobilePage(page), 'The V37 two-tab inspector is desktop-only; the phone detail is audited by the phone test')
  const entries = await loadEntries(page)
  const pair = selectPair(entries)
  expect(pair, `Requires two real non-queued jobs with written metadata, distinct source rows and a multi-step journal; found ${entries.length}`).not.toBeNull()
  const { a, b } = pair!

  const reduced = await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)
  expect(reduced, 'the reduced-motion project must really emulate prefers-reduced-motion').toBe(info.project.name.includes('reduced'))

  await page.goto(`/#/?video=${a.job.id}`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByRole('heading', { level: 1, name: a.job.source.title })).toBeVisible()
  const videos = page.getByRole('complementary', { name: 'Video mọi kênh', exact: true })
  const inspector = page.getByRole('complementary', { name: 'Bảng bên phải', exact: true })
  const infoTab = inspector.getByRole('tab', { name: 'Thông tin', exact: true })
  const contentTab = inspector.getByRole('tab', { name: 'Nội dung đăng', exact: true })
  await expect(inspector.getByRole('tab')).toHaveCount(2)
  await expect(infoTab).toHaveAttribute('aria-selected', 'true')
  await expect(contentTab).toHaveAttribute('aria-selected', 'false')
  await expect(inspector.getByText(a.job.source.title, { exact: true })).toBeVisible()
  await capture(page, info, 'main-inspector-default-info')

  await contentTab.click()
  await expect(contentTab).toHaveAttribute('aria-selected', 'true')
  const contentPanel = inspector.getByRole('tabpanel')
  const titleBox = contentPanel.getByRole('textbox', { name: 'Tiêu đề', exact: true })
  await expect(titleBox).toHaveValue(a.meta.metadata!.title)
  await capture(page, info, 'main-inspector-content')

  const refetched = await revalidateReadOnly(page, context, a.job.id)
  await expect(contentTab).toHaveAttribute('aria-selected', 'true')
  await expect(titleBox).toHaveValue(a.meta.metadata!.title)
  await capture(page, info, 'main-inspector-after-readonly-refresh')

  const copies = []
  copies.push({ control: 'title', ...(await copyAndAssert(
    page,
    titleBox.locator('xpath=..').getByRole('button', { name: 'Sao chép', exact: true }),
    a.meta.metadata!.title,
    'title copy',
  )) })
  const descriptionBox = contentPanel.getByRole('textbox', { name: 'Mô tả', exact: true })
  copies.push({ control: 'description', ...(await copyAndAssert(
    page,
    descriptionBox.locator('xpath=..').getByRole('button', { name: 'Sao chép', exact: true }),
    a.meta.metadata!.description,
    'description copy',
  )) })
  const packageText = `${a.meta.metadata!.title}\n\n${a.meta.metadata!.description}\n\n${a.meta.metadata!.tags.join(', ')}`
  copies.push({ control: 'package', ...(await copyAndAssert(
    page,
    contentPanel.getByRole('button', { name: 'Sao chép cả gói đăng', exact: true }),
    packageText,
    'posting package copy',
  )) })
  await capture(page, info, 'main-inspector-copied-package')
  await page.keyboard.press(']')
  await expect(inspector).toBeHidden()
  await page.keyboard.press(']')
  await expect(inspector).toBeVisible()
  await expect(contentTab).toHaveAttribute('aria-selected', 'true')

  await videos.getByRole('button', { name: b.job.source.title, exact: false }).click()
  await expect.poll(() => page.url().includes(`video=${b.job.id}`)).toBe(true)
  await expect(page.getByRole('heading', { level: 1, name: b.job.source.title })).toBeVisible()
  await expect(contentTab).toHaveAttribute('aria-selected', 'false')
  await expect(infoTab).toHaveAttribute('aria-selected', 'true')
  await contentTab.click()
  await expect(contentPanel.getByRole('textbox', { name: 'Tiêu đề', exact: true })).toHaveValue(b.meta.metadata!.title)
  await capture(page, info, 'main-inspector-source-b-content')

  await infoTab.click()
  await expect(infoTab).toHaveAttribute('aria-selected', 'true')
  await expect(inspector.getByText(b.job.source.title, { exact: true })).toBeVisible()
  const agentCards = inspector.getByText('Bốn trợ lý', { exact: true }).locator('xpath=..').locator('xpath=./div')
  await expect(agentCards).toHaveCount(4)
  const agentOrder = ['artist', 'qa', 'director', 'scout']
  for (let index = 0; index < agentOrder.length; index++) {
    const card = agentCards.nth(index)
    await expect(card).toBeVisible()
    expect(((await card.textContent()) ?? '').trim().length).toBeGreaterThan(0)
    const agent = b.job.agentView?.find((item) => item.id === agentOrder[index])
    if (agent?.now) await expect(card).toContainText(agent.now)
  }
  await capture(page, info, 'main-inspector-source-b-info')

  await videos.getByRole('button', { name: a.job.source.title, exact: false }).click()
  await expect(page.getByRole('heading', { level: 1, name: a.job.source.title })).toBeVisible()
  await expect(infoTab).toHaveAttribute('aria-selected', 'true')
  await expect(inspector.getByText(a.job.source.title, { exact: true })).toBeVisible()

  const steps = a.job.steps ?? []
  const logToggle = page.getByRole('button', { name: /Nhật ký|Thu gọn nhật ký/ }).first()
  const journal = page.getByRole('list', { name: 'Nhật ký', exact: true })
  await expect(logToggle).toHaveAttribute('aria-expanded', 'false')
  await expect(journal.getByRole('listitem')).toHaveCount(Math.min(2, steps.length))
  await logToggle.click()
  await expect(logToggle).toHaveAttribute('aria-expanded', 'true')
  await expect(journal.getByRole('listitem')).toHaveCount(steps.length)
  await capture(page, info, 'main-journal-expanded')
  await logToggle.click()
  await expect(logToggle).toHaveAttribute('aria-expanded', 'false')
  await expect(journal.getByRole('listitem')).toHaveCount(Math.min(2, steps.length))

  expect(gates.get(page)!.writes, 'No write is allowed while auditing the Main inspector').toEqual([])
  await attach(info, 'main-inspector-observations', {
    project: info.project.name,
    viewport: page.viewportSize(),
    reducedMotion: reduced,
    defaultTab: 'info',
    tabPreservedAfterRefresh: true,
    refetchedPaths: refetched,
    sourceSwitch: { from: a.job.id, to: b.job.id, contentTitle: b.meta.metadata!.title, infoSource: b.job.source.title },
    copies,
    packageLength: packageText.length,
    journal: { steps: steps.length, closed: Math.min(2, steps.length), open: steps.length },
    agents: 4,
  })
})

test('desktop ActivityPopover renders the real activity payload and closes on outside click and Escape', async ({ page }, info) => {
  test.skip(mobilePage(page), 'ActivityPopover is desktop-only; phone uses the QueueSummarySheet')
  const list = await readJson<{ jobs: Job[] | null }>(page, '/api/clone?channel=all')
  const safe = (list.jobs ?? []).find((job) => job.stageGroup !== 'working')
  await page.goto(safe ? `/#/?video=${safe.id}` : '/#/settings', { waitUntil: 'domcontentloaded' })
  const reduced = await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)
  expect(reduced, 'the reduced-motion project must really emulate prefers-reduced-motion').toBe(info.project.name.includes('reduced'))

  const trigger = page.locator('a[data-jobs], button[data-jobs]').first()
  await expect(trigger).toBeVisible()
  await trigger.click()
  await expect(trigger).toHaveAttribute('aria-expanded', 'true')
  const dialog = page.getByRole('dialog', { name: 'Việc nền', exact: true })
  await expect(dialog).toBeVisible()
  await withinViewport(page, dialog)

  const activity = await readJson<{ running: { title: string }[]; queued: number; unread: unknown[] }>(page, '/api/activity')
  const runningTab = dialog.getByRole('tab', { name: /Đang chạy/ })
  await expect(runningTab).toHaveAttribute('aria-selected', 'true')
  await expect(runningTab).toContainText(String(activity.running.length))
  const panel = dialog.getByRole('tabpanel')
  if (activity.running.length > 0) {
    for (const task of activity.running.slice(0, 3)) {
      await expect(dialog.getByText(task.title, { exact: true }).first()).toBeVisible()
    }
  } else if (activity.queued > 0) {
    await expect(panel.locator('a[href^="#/queue"]').first()).toContainText(String(activity.queued))
  } else {
    await expect(panel.getByRole('link', { name: 'Làm video', exact: true })).toBeVisible()
  }
  await capture(page, info, 'activity-popover-current')

  const inboxTab = dialog.getByRole('tab', { name: /Thông báo/ })
  await inboxTab.click()
  await expect(inboxTab).toHaveAttribute('aria-selected', 'true')
  await expect(inboxTab).toContainText(String(activity.unread.length))
  const history = await readJson<{ notifications: { text: string }[] }>(page, '/api/notifications')
  if (history.notifications.length > 0) {
    await expect(dialog.locator('.vv-note')).not.toHaveCount(0)
    await expect(dialog.getByText(history.notifications[0].text, { exact: true }).first()).toBeVisible()
  } else {
    await expect(dialog.locator('.vv-note')).toHaveCount(0)
    await expect(inboxTab).toContainText('0')
  }
  await capture(page, info, 'activity-popover-inbox')

  await page.mouse.click(2, Math.floor(page.viewportSize()!.height / 2))
  await expect(dialog).toHaveCount(0)
  await expect(trigger).toHaveAttribute('aria-expanded', 'false')

  await trigger.click()
  await expect(dialog).toBeVisible()
  await expect(trigger).toHaveAttribute('aria-expanded', 'true')
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(trigger).toHaveAttribute('aria-expanded', 'false')
  await expect(trigger).toBeFocused()

  expect(gates.get(page)!.writes, 'Opening and dismissing ActivityPopover must not write').toEqual([])
  await attach(info, 'activity-popover-observations', {
    project: info.project.name,
    viewport: page.viewportSize(),
    route: safe ? 'workbench' : 'settings',
    reducedMotion: reduced,
    running: activity.running.length,
    queued: activity.queued,
    unread: activity.unread.length,
    history: history.notifications.length,
    outsideDismissed: true,
    escapeDismissed: true,
    focusRestoredOnEscape: true,
  })
})

test('notification suggestions is one boolean that saves, reads back, survives reload and restores exactly', async ({ page }, info) => {
  const gate = gates.get(page)!
  const me = await readJson<{ user?: { username?: string; role?: string } }>(page, '/api/me')
  expect(me.user?.role, 'The isolated workflow.qa fixture must be admin for the real notification write').toBe('admin')
  gate.allowSettingsWrite = true

  const original = await readJson<NotificationSettings>(page, '/api/notifications/settings')
  const target = !original.suggestions
  let restored = false
  try {
    await page.goto('/#/settings?section=notifications', { waitUntil: 'domcontentloaded' })
    const toggle = page.getByRole('switch', { name: 'Đề xuất của trợ lý', exact: true })
    await expect(toggle).toBeVisible()
    await expect(toggle).toBeEnabled()
    await expect(toggle).toHaveAttribute('aria-checked', String(original.suggestions))
    await capture(page, info, 'notification-suggestions-before')

    const [response] = await Promise.all([
      page.waitForResponse((candidate) => candidate.request().method() === 'PUT'
        && new URL(candidate.url()).pathname === '/api/notifications/settings'),
      toggle.click(),
    ])
    expect(response.status()).toBe(200)
    await expect(toggle).toHaveAttribute('aria-checked', String(target))
    const saved = await readJson<NotificationSettings>(page, '/api/notifications/settings')
    expect(saved.suggestions, 'authoritative readback after save').toBe(target)
    expect({ ...saved, suggestions: original.suggestions }, 'saving one boolean must preserve every unrelated preference').toEqual(original)
    await capture(page, info, 'notification-suggestions-saved')

    await page.reload({ waitUntil: 'domcontentloaded' })
    const reloaded = page.getByRole('switch', { name: 'Đề xuất của trợ lý', exact: true })
    await expect(reloaded).toBeVisible()
    await expect(reloaded).toHaveAttribute('aria-checked', String(target))
    const persisted = await readJson<NotificationSettings>(page, '/api/notifications/settings')
    expect(persisted.suggestions, 'reload must show the persisted boolean').toBe(target)
    await capture(page, info, 'notification-suggestions-reloaded')
  } finally {
    const response = await page.request.put('/api/notifications/settings', { data: original })
    expect(response.status(), 'restoring the exact pre-test notification preferences must succeed').toBe(200)
    const readback = await readJson<NotificationSettings>(page, '/api/notifications/settings')
    expect(readback, 'the audit must restore every original notification preference exactly').toEqual(original)
    restored = true
  }
  expect(restored).toBe(true)
  expect(gate.writes.every((write) => write === 'PUT /api/notifications/settings'), 'only the exact own notification write is allowed').toBe(true)
  expect(gate.writes.length, 'the UI toggle must have produced the real notification write').toBeGreaterThanOrEqual(1)
  await attach(info, 'notification-suggestions-observations', {
    project: info.project.name,
    viewport: page.viewportSize(),
    before: original.suggestions,
    saved: target,
    reloaded: target,
    restored: original.suggestions,
    writes: gate.writes,
    unrelatedPreferencesPreserved: true,
  })
})

test('phone Main detail keeps the real content values, package copy, four assistants and progress journal', async ({ page }, info) => {
  test.skip(!mobilePage(page), 'Phone-only mobile detail; the desktop inspector is covered above')
  const entries = await loadEntries(page)
  const entry = entries[0]
  expect(entry, `Requires a real non-queued job with written title/description/tags metadata; found ${entries.length}`).toBeTruthy()
  const { job, meta } = entry!
  const metadata = meta.metadata!

  await page.goto(`/#/?video=${job.id}`, { waitUntil: 'domcontentloaded' })
  await expect(page.locator('header:visible').getByText(job.source.title, { exact: true })).toBeVisible()

  const agents = page.getByRole('list', { name: 'Bốn trợ lý', exact: true }).getByRole('listitem')
  await expect(agents).toHaveCount(4)
  for (let index = 0; index < 4; index++) {
    await expect(agents.nth(index)).toBeVisible()
  }

  const content = page.locator('section[aria-labelledby="pub-h"]')
  await expect(content.getByRole('button', { name: 'Tiêu đề', exact: true })).toHaveText(metadata.title)
  await expect(content.getByRole('button', { name: 'Mô tả', exact: true })).toHaveText(metadata.description)
  const packageText = `${metadata.title}\n\n${metadata.description}\n\n${metadata.tags.join(', ')}`
  const copy = await copyAndAssert(
    page,
    content.getByRole('button', { name: 'Sao chép gói', exact: true }),
    packageText,
    'phone package copy',
  )
  await capture(page, info, 'phone-main-package-copied')

  const journal = page.getByRole('region', { name: 'Nhật ký tiến độ', exact: true })
  await expect(journal).toBeVisible()
  const steps = job.steps ?? []
  await expect(journal.locator('xpath=./div')).toHaveCount(1 + Math.min(3, steps.length))
  await capture(page, info, 'phone-main-journal')

  info.annotations.push({
    type: 'state-gap',
    description: 'V37 two-tab inspector and desktop ActivityPopover do not exist on phone; the phone detail sections and the shared notification settings are audited instead.',
  })
  expect(gates.get(page)!.writes, 'No write is allowed while auditing phone Main detail').toEqual([])
  await attach(info, 'phone-main-observations', {
    project: info.project.name,
    viewport: page.viewportSize(),
    job: job.id,
    stepGroup: job.stageGroup,
    agents: 4,
    contentTitle: metadata.title,
    copy,
    packageLength: packageText.length,
    journal: { steps: steps.length, rendered: 1 + Math.min(3, steps.length) },
  })
})
