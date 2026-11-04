import { test, expect, type Page, type TestInfo, type Locator, type Request as PlaywrightRequest, type Response as PlaywrightResponse } from '@playwright/test'
import { readFileSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'
import { inflateRawSync } from 'node:zlib'

// V36 Settings control ledger. This is a consumer-behaviour audit of the real
// isolated app, not a visual clone: every row below is either exercised through
// a native control in this run or cites the receipt that already proved it.
//
// Exact artboard rows (08-dev-prompt §Settings + reports/artboard-state-matrix):
//   Settings.section.tools/connect/agents/accounts/storage/logs
//   Settings.conn.list/add/added/models/manual/failed
//
// Safety: every non-GET request is blocked at the browser boundary except the
// source-proven read-only probes below. No provider add/remove/PATCH, no model
// inference, no tool install/cancel, no platform login/logout and no storage
// move/delete/cleanup needs to happen to produce this ledger. One opt-in
// ChatGPT catalog-only check (no inference) can run when the controller sets
// WORKFLOW_QA_CATALOG_ONLY_MANUAL=1 and restores its ai_models row.
//   POST /api/tools/check              → toolsvc.Snapshot(fresh) probes local
//                                        binaries/GitHub release metadata only.
//   POST /api/storage/cleanup/preview  → retention.PreviewCleanup collects a
//                                        candidate list; only cleanup/run
//                                        deletes, and that route stays blocked.
//   POST .../models/check               → only when the opt-in guard pins a real
//                                        ChatGPT catalog id; ChatGPTAdapter
//                                        resolves from codex debug models
//                                        --bundled and makes no model call.
// The cleanup preview is used solely to show its consequence form and press
// "Để sau" (cancel). The destructive "Dọn" action is never pressed.
//
// Unreachable states are explicit gaps with owner/prerequisite. They are never
// fabricated with CSS, mocked APIs, route interception, credentials or writes.

const repo = resolve(__dirname, '../..')
const cookiePath = process.env.WORKFLOW_QA_COOKIE_FILE
const fixturePath = process.env.WORKFLOW_QA_FIXTURE
if (!cookiePath || !fixturePath) throw new Error('Private workflow.qa cookies and fixture are required')
if (realpathSync(cookiePath).startsWith(repo + sep)) throw new Error('Cookies must stay outside the repository')
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as { username?: string }
if (fixture.username !== 'workflow.qa') throw new Error('Only the isolated workflow.qa fixture is authorized')

type SectionId = 'tools' | 'connect' | 'agents' | 'accounts' | 'storage' | 'logs'
type Provider = {
  id: string
  kind: string
  auth: 'subscription' | 'api_key' | 'local'
  label: string
  status: 'ready' | 'logged_out' | 'login_pending' | 'auth_failed' | 'unreachable'
  baseUrl?: string
  plan?: string
  selected: string[]
  catalogCount: number
  quota?: { used: number; limit: number; period: 'day' | 'week'; resetAt: string }
}
type ModelPage = { models: { id: string; caps: string[]; recommended?: boolean }[]; total: number; nextCursor: string | null }
type CatalogReadback = { models: (ModelPage['models'][number] & { manual?: boolean; contextK?: number })[]; total: number; nextCursor: string | null }
type ToolRow = { id: string; state: string; version: string | null }
type ToolsStatus = { tools: ToolRow[]; checkedAt: string; dataDir?: string }
type Platform = {
  id: string
  label: string
  state: string
  login: { saved: boolean; type: string; refreshError?: unknown; browserProfile?: string; method?: string }
  lastError?: { code?: string }
}
type ReclaimItem = { id: string; title: string; bytes: number; count: number; action?: string }
type StorageStatus = { projects: { usedBytes: number; volume: { name: string } } }
type ModelRef = { provider: string; model: string }
type AgentDoc = {
  director: { model: ModelRef; effort: string; fallback: ModelRef[] }
  artist: { promptModel: ModelRef; draw: ModelRef; fallbackOn: boolean; drawFallback: ModelRef[] }
}
type EffortChoice = 'auto' | 'low' | 'medium' | 'high'

// Opt-in controller-owned real write set for the agent autosave proof. The
// controller snapshots the raw settings/ai_providers rows and restores them
// after the exact-title run; nothing here is fabricated or mocked.
const AGENT_SELECTION_TARGET = 'gpt-5.5'

type WriteRecord = { method: string; path: string; allowed: boolean; purpose: string | null }
type LedgerRow = {
  id: string
  name: string
  role: string
  state: string
  enabled: boolean | 'n/a'
  disabledReason: string | null
  action: string
  expected: string
  observed: string
  api: string | null
  proof: 'native' | 'existing-proof' | 'blocked'
  receipt?: { path: string; test: string; result: string }
}
type Gap = { id: string; reason: string; needed: string; owner?: string }

const SECTION_TITLE: Record<SectionId, string> = {
  tools: 'Công cụ trên máy',
  connect: 'Kết nối AI',
  agents: 'Trợ lý AI',
  accounts: 'Tài khoản nền tảng',
  storage: 'Lưu trữ',
  logs: 'Báo lỗi',
}
const SECTION_ORDER: SectionId[] = ['tools', 'connect', 'agents', 'accounts', 'storage', 'logs']
const EXACT_SETTINGS_ROWS = [
  'Settings.section.tools',
  'Settings.section.connect',
  'Settings.section.agents',
  'Settings.section.accounts',
  'Settings.section.storage',
  'Settings.section.logs',
  'Settings.conn.list',
  'Settings.conn.add',
  'Settings.conn.added',
  'Settings.conn.models',
  'Settings.conn.manual',
  'Settings.conn.failed',
] as const
const EXISTING_PROVIDER_RECEIPT = {
  path: 'plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-02/runs/87-committed-settings-focus-and-provider/interaction-results.json',
  test: 'real local catalog ready/unreachable: add feedback, model surface and owned-record removal',
  result: '8/8 passed across desktop/mobile normal/reduced; owned Ollama created, GET/readback verified, then removed with cleanup',
}
const EXISTING_SETTINGS_RECEIPT = {
  path: 'plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-02/runs/52-settings-server-oracle/interaction-results.json',
  test: 'connect: truthful provider cards, all safe add radios, scrim/Esc and conditional model catalog',
  result: '14 passed / 0 failed / 2 named mobile-recheck skips across four projects; real ChatGPT catalog (2 models) filters, search and manual cancel passed on desktop',
}

const SAFE_WRITES: Record<string, string> = {
  'POST /api/tools/check': 'toolsvc fresh local probe; no provider/model or inference call',
  'POST /api/storage/cleanup/preview': 'retention candidate list only; cleanup/run is blocked and never pressed',
}

// Guarded by WORKFLOW_QA_CATALOG_ONLY_MANUAL=1 and
// WORKFLOW_QA_CATALOG_ONLY_MANUAL_MODEL=<real ChatGPT catalog id>.
// ChatGPTAdapter.CheckModel only resolves an id against codex debug models
// --bundled (adapters.go:692–704); it makes no model/inference call. The
// controller snapshots/restores the affected ai_models row, so the test never
// restores DB state itself.
let allowedCatalogCheck: { path: string; model: string } | null = null
// Opt-in agent-selection writes. The provider PATCH is pinned to one provider
// path and the exact `selected` array the UI draft is about to send; the agents
// PUT only accepts a complete AgentModels document. Every other write still
// aborts at the browser boundary and fails the afterEach audit.
let allowedProviderPatch: { path: string; selected: string[] } | null = null
let allowedAgentsPut = false

function isModelRef(value: unknown): value is ModelRef {
  const ref = value as ModelRef | null
  return !!ref && typeof ref.provider === 'string' && typeof ref.model === 'string'
}
function isFullAgentDoc(value: unknown): value is AgentDoc {
  const doc = value as AgentDoc | null
  if (!doc || typeof doc !== 'object') return false
  if (!isModelRef(doc.director?.model) || !isModelRef(doc.artist?.promptModel) || !isModelRef(doc.artist?.draw)) return false
  if (!['auto', 'low', 'medium', 'high'].includes(doc.director.effort)) return false
  if (typeof doc.artist.fallbackOn !== 'boolean') return false
  if (!Array.isArray(doc.director.fallback) || !doc.director.fallback.every(isModelRef)) return false
  if (!Array.isArray(doc.artist.drawFallback) || !doc.artist.drawFallback.every(isModelRef)) return false
  return true
}
function safeWritePurpose(request: PlaywrightRequest): string | null {
  const key = `${request.method()} ${new URL(request.url()).pathname}`
  const listed = SAFE_WRITES[key]
  if (listed) return listed
  const patch = allowedProviderPatch
  if (patch && request.method() === 'PATCH' && new URL(request.url()).pathname === patch.path) {
    try {
      const payload = request.postDataJSON() as { selected?: unknown } | null
      const keys = payload && typeof payload === 'object' ? Object.keys(payload) : []
      if (keys.length === 1 && keys[0] === 'selected'
        && Array.isArray(payload!.selected)
        && payload!.selected.length === patch.selected.length
        && payload!.selected.every((id, index) => id === patch.selected[index])) {
        return 'Opt-in agent-selection fixture: owned ChatGPT selected draft only; the controller restores it'
      }
    } catch {
      return null
    }
  }
  if (allowedAgentsPut && request.method() === 'PUT' && new URL(request.url()).pathname === '/api/ai/agents') {
    try {
      return isFullAgentDoc(request.postDataJSON())
        ? 'Opt-in agent-selection fixture: complete PUT /api/ai/agents document; the controller restores it'
        : null
    } catch {
      return null
    }
  }
  const pin = allowedCatalogCheck
  if (!pin || request.method() !== 'POST') return null
  if (new URL(request.url()).pathname !== pin.path) return null
  try {
    const payload = request.postDataJSON() as { model?: string } | null
    return payload?.model === pin.model
      ? 'ChatGPTAdapter.CheckModel compares against codex debug models --bundled; no inference call'
      : null
  } catch {
    return null
  }
}

const ledgerByPage = new WeakMap<Page, LedgerRow[]>()
const gapsByPage = new WeakMap<Page, Gap[]>()
const writesByPage = new WeakMap<Page, WriteRecord[]>()
const faultsByPage = new WeakMap<Page, string[]>()

function addRow(page: Page, row: LedgerRow) {
  const rows = ledgerByPage.get(page)
  if (!rows) throw new Error('Ledger was not initialized for this page')
  const index = rows.findIndex((entry) => entry.id === row.id)
  if (index >= 0) rows[index] = row
  else rows.push(row)
}
function addGap(page: Page, gap: Gap) {
  const gaps = gapsByPage.get(page)
  if (!gaps) throw new Error('Gap list was not initialized for this page')
  gaps.push(gap)
}
function writes(page: Page) {
  const entries = writesByPage.get(page)
  if (!entries) throw new Error('Write audit was not initialized for this page')
  return entries
}
function writeCount(page: Page) {
  return writes(page).length
}
async function expectNoNewWrites(page: Page, before: number, label: string) {
  expect(writes(page).length, `${label}: no API write was attempted`).toBe(before)
}
async function settle(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(() => requestAnimationFrame(done))))
  })
  await page.waitForFunction(() => document.getAnimations()
    .filter((animation) => Number.isFinite(animation.effect?.getComputedTiming().endTime))
    .every((animation) => animation.playState === 'finished' || animation.playState === 'idle'), undefined, { timeout: 10_000 })
}
async function capture(page: Page, testInfo: TestInfo, name: string) {
  await settle(page)
  const width = await page.evaluate(() => document.documentElement.scrollWidth)
  expect(width).toBeLessThanOrEqual(page.viewportSize()!.width)
  await page.screenshot({ path: testInfo.outputPath(`${name}.png`) })
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
async function boot(page: Page, path: string) {
  await page.goto(path, { waitUntil: 'domcontentloaded' })
  await page.waitForFunction(() => location.hash.startsWith('#/setup') || Boolean(document.querySelector('.vv-set__title, .vv-mset__title')), undefined, { timeout: 15_000 })
  if (new URL(page.url()).hash.startsWith('#/setup')) {
    // Existing native exit from first-run setup; no setup state is fabricated.
    await page.getByRole('link', { name: 'Cài đặt nâng cao' }).click()
    await page.goto(path, { waitUntil: 'domcontentloaded' })
  }
}
async function expectTitle(page: Page, mobile: boolean, title: string) {
  await expect(page.locator(mobile ? '.vv-mset__title' : '.vv-set__title')).toHaveText(title)
}
async function openSection(page: Page, mobile: boolean, section: SectionId) {
  const hashSection = new URL(page.url()).hash.match(/section=([a-z]+)/)?.[1] ?? null
  if (hashSection === section) return
  if (mobile) {
    const back = page.getByRole('button', { name: 'Quay lại' })
    if (await page.locator('.vv-mset__list').count() === 0 && await back.count() > 0) {
      await back.click()
      await expect(page).toHaveURL(/#\/settings$/)
    }
    if (section === 'logs') {
      // MSettings intentionally has no logs child row: "Xuất log" is an action.
      // The router still accepts the deep link, so use it truthfully.
      await page.goto('/#/settings?section=logs', { waitUntil: 'domcontentloaded' })
    } else {
      const row = page.locator(`a[href="#/settings?section=${section}"]`)
      await expect(row).toBeVisible()
      await row.click()
    }
  } else {
    await page.locator('aside.vv-set__side')
      .getByRole('button', { name: new RegExp(`^${SECTION_TITLE[section]}`) })
      .click()
  }
  await expect(page).toHaveURL(new RegExp(`section=${section}`))
  await expectTitle(page, mobile, SECTION_TITLE[section])
}
async function getJson<T>(page: Page, path: string): Promise<T> {
  const response = await page.request.get(path)
  expect(response.status(), path).toBe(200)
  return response.json() as Promise<T>
}
async function getOptionalJson<T>(page: Page, path: string): Promise<T | null> {
  const response = await page.request.get(path)
  if (!response.ok()) return null
  return response.json() as Promise<T | null>
}
/** Run one native action and resolve with its real PUT /api/ai/agents response. */
async function putAgents(page: Page, action: () => Promise<void>): Promise<PlaywrightResponse> {
  const [response] = await Promise.all([
    page.waitForResponse((candidate) => new URL(candidate.url()).pathname === '/api/ai/agents' && candidate.request().method() === 'PUT'),
    action(),
  ])
  return response
}
function expectedSubline(provider: Provider, mobile: boolean): string {
  const planText = provider.plan ? provider.plan.charAt(0).toUpperCase() + provider.plan.slice(1) : ''
  if (provider.status === 'logged_out') return 'Chưa đăng nhập'
  if (provider.status === 'login_pending') return mobile ? (planText ? `Gói ${planText}` : 'Đã đăng nhập') : 'Đang đăng nhập'
  if (provider.status === 'auth_failed') return provider.auth === 'api_key' ? 'chưa lấy được model' : 'Phiên đăng nhập đã hết hạn'
  if (provider.status === 'unreachable') return 'Không kết nối được'
  if (provider.auth === 'subscription') return mobile ? (planText ? `Gói ${planText}` : 'Đã đăng nhập') : 'Đã đăng nhập'
  if (provider.auth === 'api_key') return mobile ? 'Khoá' : 'Khoá API'
  return (provider.baseUrl ?? '').replace(/^https?:\/\//, '').replace(/\/.*$/, '')
}
function expectedPlatformStatus(platform: Platform, mobile: boolean): string {
  if (platform.state === 'paused') return 'Tạm chặn'
  if (platform.login.saved && platform.login.refreshError) return mobile ? 'Hết hạn' : 'Cần đăng nhập lại'
  if (platform.login.saved && platform.lastError?.code === 'cookies_failed') return 'Cần cookie'
  return 'Sẵn sàng'
}
function platformRecord(platform: Platform) {
  return {
    id: platform.id,
    state: platform.state,
    saved: platform.login.saved,
    method: platform.login.method ?? null,
    browserProfile: platform.login.browserProfile ?? null,
  }
}

/** Parse ZIP central-directory names and read one entry to prove the report is
 * actually a readable archive, not merely a file with a .zip name. */
type ZipEntry = { name: string; method: number; compressed: Buffer }
function zipEntries(buffer: Buffer): ZipEntry[] {
  const minimum = Math.max(0, buffer.length - 65_557)
  let eocd = -1
  for (let offset = buffer.length - 22; offset >= minimum; offset--) {
    if (buffer.readUInt32LE(offset) === 0x06054b50) { eocd = offset; break }
  }
  if (eocd < 0) throw new Error('ZIP end-of-central-directory not found')
  const count = buffer.readUInt16LE(eocd + 10)
  let offset = buffer.readUInt32LE(eocd + 16)
  const entries: ZipEntry[] = []
  for (let index = 0; index < count; index++) {
    if (buffer.readUInt32LE(offset) !== 0x02014b50) throw new Error(`Bad ZIP central entry ${index}`)
    const method = buffer.readUInt16LE(offset + 10)
    const compressedSize = buffer.readUInt32LE(offset + 20)
    const nameLength = buffer.readUInt16LE(offset + 28)
    const extraLength = buffer.readUInt16LE(offset + 30)
    const commentLength = buffer.readUInt16LE(offset + 32)
    const localOffset = buffer.readUInt32LE(offset + 42)
    const name = buffer.toString('utf8', offset + 46, offset + 46 + nameLength)
    if (buffer.readUInt32LE(localOffset) !== 0x04034b50) throw new Error(`Bad ZIP local entry for ${name}`)
    const localNameLength = buffer.readUInt16LE(localOffset + 26)
    const localExtraLength = buffer.readUInt16LE(localOffset + 28)
    const dataOffset = localOffset + 30 + localNameLength + localExtraLength
    entries.push({ name, method, compressed: buffer.subarray(dataOffset, dataOffset + compressedSize) })
    offset += 46 + nameLength + extraLength + commentLength
  }
  return entries
}
function readZipText(entry: ZipEntry): string {
  if (entry.method === 0) return entry.compressed.toString('utf8')
  if (entry.method === 8) return inflateRawSync(entry.compressed).toString('utf8')
  throw new Error(`Unsupported ZIP method ${entry.method} for ${entry.name}`)
}

test.beforeEach(async ({ context, page }) => {
  allowedCatalogCheck = null
  allowedProviderPatch = null
  allowedAgentsPut = false
  ledgerByPage.set(page, [])
  gapsByPage.set(page, [])
  writesByPage.set(page, [])
  const faults: string[] = []
  faultsByPage.set(page, faults)
  page.on('pageerror', (error) => faults.push(error.message))
  page.on('dialog', (dialog) => { void dialog.dismiss() })
  page.on('request', (request) => {
    const method = request.method()
    if (['GET', 'HEAD', 'OPTIONS'].includes(method)) return
    const url = new URL(request.url())
    if (!url.pathname.startsWith('/api/')) return
    const purpose = safeWritePurpose(request)
    writesByPage.get(page)!.push({ method, path: url.pathname, allowed: purpose !== null, purpose })
  })
  await page.route('**/api/**', (route) => {
    const request = route.request()
    if (['GET', 'HEAD', 'OPTIONS'].includes(request.method())) return route.continue()
    return safeWritePurpose(request) !== null ? route.continue() : route.abort('blockedbyclient')
  })
  await context.addCookies(JSON.parse(readFileSync(cookiePath!, 'utf8')))
  const identity = await context.request.get('/api/me')
  expect(identity.status()).toBe(200)
  const me = await identity.json() as { user?: { username?: string; role?: string } }
  expect(me.user?.username).toBe('workflow.qa')
  expect(me.user?.role, 'Settings write controls require the isolated admin fixture').toBe('admin')
})

test.afterEach(async ({ page }, testInfo) => {
  await testInfo.attach('settings-ledger-evidence.json', {
    body: JSON.stringify({
      project: testInfo.project.name,
      viewport: page.viewportSize(),
      baseURL: testInfo.project.use.baseURL ?? process.env.WORKFLOW_APP_URL ?? null,
      sourceRevision: process.env.WORKFLOW_SOURCE_REVISION ?? null,
      exactRows: EXACT_SETTINGS_ROWS,
      catalogOnlyManual: process.env.WORKFLOW_QA_CATALOG_ONLY_MANUAL === '1'
        ? { model: process.env.WORKFLOW_QA_CATALOG_ONLY_MANUAL_MODEL ?? null }
        : null,
      agentSelection: process.env.WORKFLOW_QA_AGENT_SELECTION === '1'
        ? { target: process.env.WORKFLOW_QA_AGENT_SELECTION_MODEL ?? AGENT_SELECTION_TARGET }
        : null,
    }, null, 2),
    contentType: 'application/json',
  })
  await testInfo.attach('settings-control-ledger.json', {
    body: JSON.stringify(ledgerByPage.get(page) ?? [], null, 2),
    contentType: 'application/json',
  })
  await testInfo.attach('settings-control-gaps.json', {
    body: JSON.stringify(gapsByPage.get(page) ?? [], null, 2),
    contentType: 'application/json',
  })
  await testInfo.attach('settings-write-audit.json', {
    body: JSON.stringify({
      allowed: Object.entries(SAFE_WRITES).map(([key, purpose]) => ({ key, purpose })),
      requests: writesByPage.get(page) ?? [],
    }, null, 2),
    contentType: 'application/json',
  })
  await testInfo.attach('settings-runtime-errors.json', {
    body: JSON.stringify(faultsByPage.get(page) ?? []),
    contentType: 'application/json',
  })
  expect(faultsByPage.get(page) ?? []).toEqual([])
  expect((writesByPage.get(page) ?? []).filter((entry) => !entry.allowed), 'every attempted write must be explicitly authorized, including aborted requests').toEqual([])
})

test('settings control ledger: 12 exact rows and safe native controls', async ({ page }, testInfo) => {
  test.setTimeout(300_000)
  const mobile = testInfo.project.name.startsWith('mobile')
  const [providers, platforms, agents, tools] = await Promise.all([
    getJson<Provider[]>(page, '/api/ai/providers'),
    getJson<Platform[]>(page, '/api/platforms'),
    getJson<{ artist: { draw: { provider: string; model: string } } }>(page, '/api/ai/agents'),
    getJson<ToolsStatus>(page, '/api/tools'),
  ])
  const serverStorage = await getOptionalJson<unknown>(page, '/api/storage/server')

  async function assertSectionContent(section: SectionId): Promise<string> {
    if (section === 'tools') {
      await expect(page.locator('.vv-tools__row')).toHaveCount(tools.tools.length)
      return `${tools.tools.length} tool rows`
    }
    if (section === 'connect') {
      if (providers.length === 0) {
        await expect(page.getByText('Chưa có nhà cung cấp nào', { exact: false })).toBeVisible()
        return 'empty provider state'
      }
      await expect(page.locator('.conn-card')).toHaveCount(providers.length)
      return `${providers.length} provider cards`
    }
    if (section === 'agents') {
      await expect(page.getByText('Đạo diễn', { exact: true }).first()).toBeVisible()
      await expect(page.getByText(/^Hoạ sĩ/).first()).toBeVisible()
      await expect(page.getByText('Trinh sát', { exact: true }).first()).toBeVisible()
      await expect(page.getByText('Kiểm định', { exact: true }).first()).toBeVisible()
      return 'director/artist/scout/qa labels'
    }
    if (section === 'accounts') {
      for (const platform of platforms) await expect(page.getByRole('region', { name: platform.label })).toBeVisible()
      return `${platforms.length} platform cards`
    }
    if (section === 'storage') {
      await expect(page.locator('.vv-sto__card').first()).toBeVisible()
      return serverStorage ? 'server storage cards' : 'local storage cards'
    }
    await expect(page.locator('.vv-logs__export')).toBeVisible()
    return 'export/errors/version cards'
  }

  const sectionApi: Record<SectionId, string> = {
    tools: 'GET /api/tools 200',
    connect: 'GET /api/ai/providers 200',
    agents: 'GET /api/ai/agents 200',
    accounts: 'GET /api/platforms 200',
    storage: serverStorage ? 'GET /api/storage/server 200' : 'GET /api/storage 200',
    logs: 'GET /api/tools/logs/export 200',
  }

  await boot(page, '/#/settings?section=tools')
  await expectTitle(page, mobile, SECTION_TITLE.tools)

  // ----- Six exact section rows, native desktop/mobile entry points. -----
  for (const section of SECTION_ORDER) {
    await openSection(page, mobile, section)
    const observed = await assertSectionContent(section)
    addRow(page, {
      id: `Settings.section.${section}`,
      name: SECTION_TITLE[section],
      role: mobile ? (section === 'logs' ? 'deep link + export action' : 'mobile list row') : 'desktop sidebar navigation',
      state: 'rendered from live resource',
      enabled: true,
      disabledReason: null,
      action: mobile
        ? (section === 'logs' ? 'open settings?section=logs (no fabricated list row)' : 'tap mobile settings row')
        : 'click desktop sidebar button',
      expected: `${SECTION_TITLE[section]} title and genuine section data`,
      observed: `${observed}; ${await page.locator(mobile ? '.vv-mset__title' : '.vv-set__title').textContent()}`,
      api: sectionApi[section],
      proof: 'native',
    })
    await capture(page, testInfo, `settings-ledger-section-${section}`)
    if (section === 'logs') break

    if (section === 'tools') {
      await exerciseTools(page, testInfo, mobile, tools)
    }
    if (section === 'connect') {
      await exerciseConnect(page, testInfo, mobile, providers)
    }
    if (section === 'agents') {
      const before = writeCount(page)
      addRow(page, {
        id: 'control.agents.readonly',
        name: 'Agent model/role rows',
        role: 'read-only status',
        state: agents.artist.draw.provider ? 'configured draw source' : 'no draw source',
        enabled: 'n/a',
        disabledReason: 'Changing an agent model is a PUT and is outside this ledger',
        action: 'assert role labels and live draw source; do not open a picker',
        expected: 'Director/Artist/Scout/QA labels render from /api/ai/agents',
        observed: `draw=${agents.artist.draw.provider || '(none)'}`,
        api: 'GET /api/ai/agents 200',
        proof: 'native',
      })
      await expectNoNewWrites(page, before, 'agents read-only')
    }
    if (section === 'accounts') {
      await exerciseAccounts(page, testInfo, mobile, platforms)
    }
    if (section === 'storage') {
      await exerciseStorage(page, testInfo, mobile, Boolean(serverStorage))
    }
  }

  // ----- Logs export: a real downloadable, readable zip. -----
  const logExport = page.getByRole('button', { name: 'Xuất log', exact: true })
  await expect(logExport).toBeVisible()
  const exportResponsePromise = page.waitForResponse((response) => {
    try {
      return response.request().method() === 'GET' && new URL(response.url()).pathname === '/api/tools/logs/export'
    } catch {
      return false
    }
  }, { timeout: 30_000 })
  const downloadPromise = page.waitForEvent('download')
  await logExport.click()
  const [exportResponse, download] = await Promise.all([exportResponsePromise, downloadPromise])
  expect(exportResponse.status()).toBe(200)
  expect(exportResponse.headers()['content-type']).toContain('application/zip')
  expect(download.suggestedFilename()).toMatch(/^viderse-logs-.+\.zip$/)
  const downloadPath = await download.path()
  expect(downloadPath).not.toBeNull()
  const archive = readFileSync(downloadPath!)
  expect(archive.length).toBeGreaterThan(0)
  const entries = zipEntries(archive)
  const names = entries.map((entry) => entry.name)
  expect(names).toContain('summary.json')
  for (const name of names) {
    expect(name).not.toMatch(/^\/|^[A-Za-z]:[\\/]|(^|\/)\.\.(\/|$)/)
  }
  const summaryEntry = entries.find((entry) => entry.name === 'summary.json')!
  const summary = JSON.parse(readZipText(summaryEntry)) as { generatedAt?: string; appVersion?: string }
  expect(typeof summary.generatedAt).toBe('string')
  expect(typeof summary.appVersion).toBe('string')
  addRow(page, {
    id: 'control.logs.export',
    name: 'Xuất log',
    role: 'primary button',
    state: 'downloaded and parsed',
    enabled: true,
    disabledReason: null,
    action: 'click export, await response + download, parse ZIP central directory and summary.json',
    expected: 'GET /api/tools/logs/export returns a readable attachment',
    observed: `${download.suggestedFilename()}; ${archive.length} bytes; entries ${names.join(', ')}; summary.generatedAt=${summary.generatedAt}`,
    api: 'GET /api/tools/logs/export 200 application/zip',
    proof: 'native',
  })
  await capture(page, testInfo, 'settings-ledger-logs-export-downloaded')

  // LogsSection source has no filter control (export / recent errors / version
  // only), so there is nothing safe to toggle. The version check is a POST and
  // is deliberately not activated.
  addRow(page, {
    id: 'control.logs.filters',
    name: 'Log filters',
    role: 'not present',
    state: 'not-applicable',
    enabled: 'n/a',
    disabledReason: 'LogsSection source exposes export, recent errors and version only; it has no filter control to toggle',
    action: 'inspect source/DOM instead of inventing a filter',
    expected: 'No filter affordance is claimed',
    observed: `filter controls: ${await page.getByRole('searchbox').count()} searchbox, ${await page.getByRole('radiogroup').count()} radiogroup in logs section`,
    api: null,
    proof: 'native',
  })

  // ----- Safety gate and exact-row completeness. -----
  const allWrites = writes(page)
  expect(allWrites.filter((entry) => !entry.allowed), JSON.stringify(allWrites.filter((entry) => !entry.allowed))).toEqual([])
  expect(allWrites.filter((entry) => /\/(install|cancel)$/.test(entry.path))).toEqual([])
  expect(allWrites.filter((entry) => /\/cleanup\/run$/.test(entry.path))).toEqual([])
  expect(allWrites.filter((entry) => entry.method === 'PATCH' || entry.method === 'DELETE')).toEqual([])
  const ledgerIds = ledgerByPage.get(page)!.map((row) => row.id)
  for (const id of EXACT_SETTINGS_ROWS) {
    expect(ledgerIds, `missing exact Settings row ${id}`).toContain(id)
  }
  expect(new Set(ledgerIds).size).toBe(ledgerIds.length)
  await testInfo.attach('settings-exact-12-rows.json', {
    body: JSON.stringify(ledgerByPage.get(page)!.filter((row) => (EXACT_SETTINGS_ROWS as readonly string[]).includes(row.id)), null, 2),
    contentType: 'application/json',
  })
  await testInfo.attach('settings-ledger-mutations.json', {
    body: JSON.stringify(allWrites, null, 2),
    contentType: 'application/json',
  })
})

async function exerciseTools(page: Page, testInfo: TestInfo, mobile: boolean, status: ToolsStatus) {
  // Details/summary is a native reversible disclosure; no install/update.
  const details = page.locator('details.vv-tools__adv')
  await expect(details).toBeVisible()
  const summary = details.locator('summary')
  const directory = status.dataDir ? details.getByText(status.dataDir, { exact: true }) : undefined
  if (directory) await expect(directory).not.toBeVisible()
  const beforeDetails = writeCount(page)
  await summary.click()
  await expect(details).toHaveJSProperty('open', true)
  if (directory) await expect(directory).toBeVisible()
  await expect(details.getByText('yt-dlp', { exact: true })).toBeVisible()
  await capture(page, testInfo, 'settings-ledger-tools-details-expanded')
  await summary.focus()
  await page.keyboard.press('Enter')
  await expect(details).toHaveJSProperty('open', false)
  if (directory) await expect(directory).not.toBeVisible()
  await expectNoNewWrites(page, beforeDetails, 'tools details')
  addRow(page, {
    id: 'control.tools.details',
    name: 'Nâng cao · thư mục dữ liệu và phiên bản',
    role: 'details/summary disclosure',
    state: 'expanded then collapsed by Enter',
    enabled: true,
    disabledReason: null,
    action: 'click summary; collapse with keyboard Enter',
    expected: 'Technical path/binary names are hidden until opened and no install/update runs',
    observed: `open round trip passed; dataDir exposed=${Boolean(status.dataDir)}; yt-dlp technical name visible when open`,
    api: null,
    proof: 'native',
  })

  if (mobile) {
    addRow(page, {
      id: 'control.tools.recheck',
      name: 'Kiểm tra lại',
      role: 'not rendered on MSettings',
      state: 'unavailable-mobile',
      enabled: false,
      disabledReason: 'MSettings hides .vv-tools__sum (settings.stylex toolsSum display:none ≤767px); recheck lives in that summary',
      action: 'none',
      expected: 'Not claimed as mobile parity',
      observed: 'desktop-only control confirmed absent from mobile DOM; PR6 owns functional parity',
      api: null,
      proof: 'blocked',
    })
    addGap(page, {
      id: 'tools.recheck.mobile',
      reason: 'MSettings has no Summary/Recheck control; the only existing proof is desktop run 52',
      needed: 'PR6 mobile functional parity acceptance',
      owner: 'PR6',
    })
    return
  }

  expect(status.tools.some((tool) => tool.state === 'installing')).toBe(false)
  const before = await getJson<ToolsStatus>(page, '/api/tools')
  const beforeWrites = writeCount(page)
  const responsePromise = page.waitForResponse((response) => {
    try {
      return response.request().method() === 'POST' && new URL(response.url()).pathname === '/api/tools/check'
    } catch {
      return false
    }
  }, { timeout: 150_000 })
  await page.getByRole('button', { name: 'Kiểm tra lại' }).click()
  const response = await responsePromise
  expect(response.status()).toBe(200)
  const after = await response.json() as ToolsStatus
  const recent = await getJson<ToolsStatus>(page, '/api/tools')
  expect(recent.tools.map((tool) => tool.state)).toEqual(after.tools.map((tool) => tool.state))
  expect(writeCount(page)).toBe(beforeWrites + 1)
  const calls = writes(page).slice(beforeWrites)
  expect(calls).toEqual([expect.objectContaining({ method: 'POST', path: '/api/tools/check', allowed: true })])
  expect(calls.filter((entry) => /\/(install|cancel)$/.test(entry.path))).toEqual([])
  addRow(page, {
    id: 'control.tools.recheck',
    name: 'Kiểm tra lại',
    role: 'button in desktop summary',
    state: 'fresh local probe completed',
    enabled: true,
    disabledReason: null,
    action: 'click recheck; await POST /api/tools/check and GET readback',
    expected: 'Only /api/tools/check is called; no install, update or cancel',
    observed: `${before.tools.map((tool) => `${tool.id}:${tool.state}`).join(', ')} → ${after.tools.map((tool) => `${tool.id}:${tool.state}`).join(', ')}`,
    api: 'POST /api/tools/check 200; GET /api/tools 200',
    proof: 'native',
  })
  await testInfo.attach('settings-ledger-tools-recheck.json', {
    body: JSON.stringify({ before: before.tools, after: after.tools, requests: calls }, null, 2),
    contentType: 'application/json',
  })
  await capture(page, testInfo, 'settings-ledger-tools-recheck')
}

async function exerciseConnect(page: Page, testInfo: TestInfo, mobile: boolean, providers: Provider[]) {
  await expect(page.getByText('YouTube Data API', { exact: false })).toHaveCount(0)
  await expect(page.getByText('Số liệu YouTube', { exact: true })).toHaveCount(0)
  addRow(page, {
    id: 'Settings.conn.list',
    name: 'Danh sách nhà cung cấp',
    role: 'provider list',
    state: providers.length ? 'rendered' : 'empty',
    enabled: 'n/a',
    disabledReason: null,
    action: 'compare .conn-card count with GET /api/ai/providers and verify truthful sublines',
    expected: 'One genuine card per provider; retired YouTube Data API card absent',
    observed: `${providers.length} providers; ${providers.map((provider) => `${provider.id}:${provider.status}`).join(', ')}; YouTube Data API absent`,
    api: 'GET /api/ai/providers 200',
    proof: 'native',
  })
  for (const provider of providers) {
    const card = page.getByRole('region', { name: provider.label })
    await expect(card).toBeVisible()
    await expect(card.locator('.conn-id .conn-sub')).toContainText(expectedSubline(provider, mobile))
  }

  // Add panel: every radio is genuine; one safe field round trip proves the
  // disabled→enabled rule. Nothing is submitted.
  const opener = page.getByRole('button', { name: mobile ? 'Thêm nhà cung cấp' : /^Thêm nhà cung cấp —/ }).first()
  const panel = mobile ? page.getByRole('dialog', { name: 'Thêm nhà cung cấp', exact: true }) : page.locator('.conn-add')
  const beforeAdd = writeCount(page)
  await expect(panel).toHaveCount(0)
  await opener.click()
  await expect(panel).toHaveCount(1)
  if (mobile) await withinViewport(page, page.locator('.m-sheet--md').last())
  const addGroup = panel.getByRole('radiogroup', { name: 'Thêm nhà cung cấp' })
  await expect(addGroup.getByRole('radio')).toHaveCount(8)
  const radio = addGroup.getByRole('radio', { name: /^OpenRouter/ })
  await expect(radio).toBeVisible()
  let addFieldValidated = false
  if ((await radio.innerText()).includes('Đã kết nối')) {
    addGap(page, {
      id: 'conn.add.representative-radio',
      reason: 'OpenRouter is already connected in this fixture, so the chosen representative add radio is inert',
      needed: 'A fixture without the representative kind connected',
    })
  } else {
    await radio.click()
    await expect(radio).toHaveAttribute('aria-checked', 'true')
    const key = page.getByLabel('Khoá API OpenRouter', { exact: true })
    await expect(key).toHaveCount(1)
    const submit = page.getByRole('button', { name: 'Kết nối và lấy model', exact: true })
    await expect(submit).toBeDisabled()
    await key.fill('qa-ledger-never-submitted')
    await expect(submit).toBeEnabled()
    await key.fill('')
    await expect(submit).toBeDisabled()
    addFieldValidated = true
  }
  if (mobile) {
    await page.keyboard.press('Escape')
    await expect(page.locator('.m-overlay')).toHaveCount(0)
    await expect.poll(() => page.evaluate(() => document.body.style.overflow)).toBe('')
  } else {
    await panel.getByRole('button', { name: 'Đóng', exact: true }).click()
    await expect(panel).toHaveCount(0)
  }
  await expectNoNewWrites(page, beforeAdd, 'connect add panel')
  addRow(page, {
    id: 'Settings.conn.add',
    name: 'Thêm nhà cung cấp',
    role: mobile ? 'sheet' : 'inline panel',
    state: addFieldValidated ? 'opened, validated and cancelled' : 'opened and cancelled; representative radio taken',
    enabled: true,
    disabledReason: null,
    action: 'open add; select OpenRouter when available; fill then clear key; close without submit',
    expected: '8 genuine radios; required field; submit enabled only with input; zero provider writes',
    observed: `8 radios; ${addFieldValidated ? 'OpenRouter field/disabled logic passed' : 'representative OpenRouter radio inert (already connected), field check skipped'}; ${mobile ? 'Escape' : 'Đóng'} closed with 0 writes`,
    api: 'GET /api/ai/providers only; no POST',
    proof: 'native',
  })
  addRow(page, {
    id: 'Settings.conn.added',
    name: 'Thẻ vừa kết nối (conn=added)',
    role: 'session-only provider card state',
    state: 'covered by existing owned-provider receipt',
    enabled: 'n/a',
    disabledReason: 'conn=added is local state set only by a successful POST /api/ai/providers; this ledger is not authorized to create another provider',
    action: 'cite run 87 owned Ollama add/remove receipt instead of replaying writes',
    expected: 'New card shows the added note and an unsaved bar; cancel restores server selection',
    observed: EXISTING_PROVIDER_RECEIPT.result,
    api: 'POST /api/ai/providers + DELETE in owned receipt only (not replayed here)',
    proof: 'existing-proof',
    receipt: EXISTING_PROVIDER_RECEIPT,
  })
  addGap(page, {
    id: 'conn.added.native',
    reason: 'A fresh conn=added state would require an unauthorized provider create in this slice',
    needed: 'Run 87 receipt (owned Ollama add/remove/readback); no new provider write was authorized',
    owner: 'parent acceptance',
  })

  await exerciseCatalog(page, testInfo, mobile, providers)

  const failed = providers.find((provider) => provider.status === 'auth_failed' || provider.status === 'unreachable')
  if (failed) {
    const card = page.getByRole('region', { name: failed.label })
    const error = card.locator('.conn-err')
    await expect(error).toBeVisible()
    await expect(error).toContainText('Không lấy được danh sách model.')
    await expect(error.getByRole('button', { name: 'Thử lại' })).toBeVisible()
    await expect(error.getByRole('button', { name: 'Thêm theo tên' })).toBeVisible()
    addRow(page, {
      id: 'Settings.conn.failed',
      name: 'Thẻ lỗi nhà cung cấp (conn=failed)',
      role: 'orange error card',
      state: `${failed.status}/${failed.auth}`,
      enabled: true,
      disabledReason: 'Retry/login/change actions are not activated; they mutate provider state',
      action: 'assert error text and retry/manual/secondary controls without clicking',
      expected: 'Truthful failure card with retry and manual entry',
      observed: `${failed.label}: ${(await error.textContent())?.replace(/\s+/g, ' ').trim()}`,
      api: 'GET /api/ai/providers 200',
      proof: 'native',
    })
  } else {
    addRow(page, {
      id: 'Settings.conn.failed',
      name: 'Thẻ lỗi nhà cung cấp (conn=failed)',
      role: 'orange error card',
      state: 'covered by existing owned-provider receipt',
      enabled: 'n/a',
      disabledReason: 'No provider in the current isolated fixture is auth_failed/unreachable; creating one is outside this ledger',
      action: 'cite run 87 real refused-port provider receipt',
      expected: 'Truthful failure card with retry/manual/secondary controls',
      observed: EXISTING_PROVIDER_RECEIPT.result,
      api: 'GET /api/ai/providers 200 on current fixture; owned POST/PATCH in receipt only',
      proof: 'existing-proof',
      receipt: EXISTING_PROVIDER_RECEIPT,
    })
    addGap(page, {
      id: 'conn.failed.native',
      reason: 'Current fixture has no auth_failed/unreachable provider; fake or newly written records are forbidden here',
      needed: 'Run 87 real refused Ollama receipt, or an isolated seeded failed provider',
      owner: 'parent acceptance',
    })
  }
}

async function exerciseCatalog(page: Page, testInfo: TestInfo, mobile: boolean, providers: Provider[]) {
  const direct = providers.find((provider) => !['logged_out', 'login_pending', 'auth_failed', 'unreachable'].includes(provider.status))
  const failed = providers.find((provider) => provider.status === 'auth_failed' || provider.status === 'unreachable')
  const catalogProvider = direct ?? failed
  if (!catalogProvider) {
    addRow(page, {
      id: 'Settings.conn.models',
      name: 'Danh mục model',
      role: 'model catalog',
      state: 'covered by existing receipt',
      enabled: 'n/a',
      disabledReason: 'No genuine ready/failed provider exposes a Model control in this fixture',
      action: 'cite run 52 real ChatGPT catalog receipt',
      expected: 'Filter/search/selected rows and manual entry',
      observed: EXISTING_SETTINGS_RECEIPT.result,
      api: null,
      proof: 'existing-proof',
      receipt: EXISTING_SETTINGS_RECEIPT,
    })
    addRow(page, {
      id: 'Settings.conn.manual',
      name: 'Thêm model theo tên',
      role: 'manual model panel',
      state: 'covered by existing receipt',
      enabled: 'n/a',
      disabledReason: 'Manual panel is gated by the same missing catalog control',
      action: 'cite run 52 manual-cancel receipt',
      expected: 'Input validation and cancel with no POST models/check',
      observed: EXISTING_SETTINGS_RECEIPT.result,
      api: null,
      proof: 'existing-proof',
      receipt: EXISTING_SETTINGS_RECEIPT,
    })
    addGap(page, {
      id: 'conn.models.entry',
      reason: 'No provider exposes the Model control, so models/manual cannot be opened natively in this fixture',
      needed: 'Genuine ready provider or seeded auth_failed provider with a real catalog endpoint',
    })
    if (process.env.WORKFLOW_QA_CATALOG_ONLY_MANUAL === '1') {
      expect(false, 'catalog-only manual case requires a genuine ChatGPT provider').toBe(true)
    }
    return
  }

  const card = page.getByRole('region', { name: catalogProvider.label })
  const modelsPath = `/api/ai/providers/${catalogProvider.id}/models`
  const opener = direct
    ? card.getByRole('button', { name: /^(Model · \d+|Chọn model)$/ })
    : card.locator('.conn-err').getByRole('button', { name: 'Thêm theo tên' })
  if (process.env.WORKFLOW_QA_CATALOG_ONLY_MANUAL === '1') {
    await expect(opener.first(), 'A ready ChatGPT catalog control must finish loading before manual verification').toBeVisible()
  }
  if (await opener.count() === 0 || !(await opener.first().isVisible())) {
    addRow(page, {
      id: 'Settings.conn.models',
      name: 'Danh mục model',
      role: 'provider card model control',
      state: 'blocked in current state',
      enabled: false,
      disabledReason: `${catalogProvider.label} exposes no visible Model/manual control in ${catalogProvider.status}`,
      action: 'none; cite run 52 real ChatGPT catalog receipt',
      expected: 'Not claimed for the current fixture',
      observed: EXISTING_SETTINGS_RECEIPT.result,
      api: 'GET /api/ai/providers 200',
      proof: 'existing-proof',
      receipt: EXISTING_SETTINGS_RECEIPT,
    })
    addRow(page, {
      id: 'Settings.conn.manual',
      name: 'Thêm model theo tên',
      role: 'manual model panel',
      state: 'blocked in current state',
      enabled: false,
      disabledReason: 'Manual panel is gated by the missing catalog control',
      action: 'none; cite run 52 manual-cancel receipt',
      expected: 'Not claimed for the current fixture',
      observed: EXISTING_SETTINGS_RECEIPT.result,
      api: null,
      proof: 'existing-proof',
      receipt: EXISTING_SETTINGS_RECEIPT,
    })
    addGap(page, {
      id: 'conn.models.control',
      reason: `${catalogProvider.label} has no visible Model/manual entry in the current state`,
      needed: 'A genuine ready provider card exposing the catalog control',
    })
    if (process.env.WORKFLOW_QA_CATALOG_ONLY_MANUAL === '1') {
      expect(false, 'catalog-only manual case requires a visible Model/manual control').toBe(true)
    }
    return
  }

  const apiResponse = await page.request.get(`${modelsPath}?limit=200`)
  const apiPage = apiResponse.ok() ? await apiResponse.json() as ModelPage : undefined
  const apiStatus = `GET ${modelsPath}?limit=200 ${apiResponse.status()}`
  if (mobile) {
    const dialog = page.getByRole('dialog', { name: `Model của ${catalogProvider.label}` })
    await opener.first().click()
    await expect(dialog).toBeVisible()
    await withinViewport(page, dialog)
    if (!apiPage) {
      await expect(dialog.getByText('Không tải được danh mục model của nhà cung cấp này.')).toBeVisible()
    } else if (apiPage.models.length === 0) {
      await expect(dialog.getByText('Không có model nào khớp.')).toBeVisible()
    } else {
      await expect.poll(() => dialog.getByRole('checkbox').count()).toBeGreaterThan(0)
    }
    await expect(dialog.locator('.conn-cat__tools')).toHaveCount(0)
    await expect(dialog.getByRole('button', { name: 'Không thấy model? Thêm theo tên' })).toHaveCount(0)
    await capture(page, testInfo, 'settings-ledger-connect-models-mobile-sheet')
    await page.keyboard.press('Escape')
    await expect(page.locator('.m-overlay')).toHaveCount(0)
    await expect.poll(() => page.evaluate(() => document.body.style.overflow)).toBe('')
    addRow(page, {
      id: 'Settings.conn.models',
      name: 'Danh mục model',
      role: 'MSettings model sheet',
      state: 'opened with genuine API data and dismissed',
      enabled: true,
      disabledReason: null,
      action: 'open Model · N sheet; assert rows/empty/failure; close with Escape',
      expected: 'Model sheet renders the same provider catalog; MSettings has no filter/search/manual',
      observed: apiPage ? `${apiPage.models.length}/${apiPage.total} models; no catalog tools; no manual entry` : `catalog API HTTP ${apiResponse.status()}`,
      api: apiStatus,
      proof: 'native',
    })
    addRow(page, {
      id: 'Settings.conn.manual',
      name: 'Thêm model theo tên',
      role: 'not rendered on MSettings sheet',
      state: 'unavailable-mobile',
      enabled: false,
      disabledReason: 'MSettings model sheet has no manual row by design; mobile manual is a PR6 parity item',
      action: 'assert absence, cite run 52 desktop manual-cancel receipt',
      expected: 'Not claimed as mobile parity',
      observed: `sheet manual controls: ${await dialog.getByRole('button', { name: 'Không thấy model? Thêm theo tên' }).count()}`,
      api: null,
      proof: 'existing-proof',
      receipt: EXISTING_SETTINGS_RECEIPT,
    })
    addGap(page, {
      id: 'conn.models.mobile-tools',
      reason: 'MSettings sheet intentionally omits filter/search/manual; only desktop exposes them',
      needed: 'PR6 mobile functional parity acceptance',
      owner: 'PR6',
    })
    return
  }

  await opener.first().click()
  const list = card.getByRole('list', { name: catalogProvider.label, exact: true })
  await expect(list).toBeVisible()
  await expect.poll(() => card.locator('.conn-cat__foot').textContent()).toContain('Lấy từ')
  const rowIds = async () => (await card.locator('.conn-row .conn-row__id').allInnerTexts()).map((id) => id.trim()).sort()
  const expectedRows = (data: ModelPage) => [...new Set([...catalogProvider.selected, ...data.models.map((model) => model.id)])].sort()
  const allIds = apiPage ? expectedRows(apiPage) : []
  if (apiPage && apiPage.models.length > 0) {
    await expect.poll(rowIds).toEqual(allIds)
  } else if (apiPage) {
    await expect(card.getByText('Không có model nào khớp.')).toBeVisible()
  }
  await capture(page, testInfo, 'settings-ledger-connect-models-catalog')
  addRow(page, {
    id: 'Settings.conn.models',
    name: 'Danh mục model',
    role: 'inline catalog',
    state: 'opened with genuine API data',
    enabled: true,
    disabledReason: null,
    action: 'open Model · N; compare visible rows with direct GET; exercise filters/search without saving',
    expected: 'Genuine page/filter/search behavior and selected drafts',
    observed: apiPage ? `${apiPage.models.length}/${apiPage.total} models; visible rows ${allIds.length}` : `catalog API HTTP ${apiResponse.status()}`,
    api: apiStatus,
    proof: 'native',
  })

  if (apiPage) {
    const filterGroup = card.getByRole('radiogroup', { name: 'Lọc model' })
    const allRadio = filterGroup.getByRole('radio', { name: 'Tất cả' })
    const recommendedRadio = filterGroup.getByRole('radio', { name: 'Gợi ý' })
    const selectedRadio = filterGroup.getByRole('radio', { name: `Đã chọn · ${catalogProvider.selected.length}` })
    await expect(allRadio).toHaveAttribute('aria-checked', 'true')
    const recommendedResponse = await page.request.get(`${modelsPath}?limit=200&filter=recommended`)
    expect(recommendedResponse.status()).toBe(200)
    const recommendedPage = await recommendedResponse.json() as ModelPage
    await recommendedRadio.click()
    if (recommendedPage.models.length === 0 && (recommendedPage.total > 0 || catalogProvider.catalogCount > 0)) {
      await expect(allRadio).toHaveAttribute('aria-checked', 'true')
      await expect.poll(rowIds).toEqual(allIds)
    } else {
      await expect(recommendedRadio).toHaveAttribute('aria-checked', 'true')
      await expect.poll(rowIds).toEqual(expectedRows(recommendedPage))
    }
    await selectedRadio.click()
    await expect(selectedRadio).toHaveAttribute('aria-checked', 'true')
    await expect.poll(rowIds).toEqual([...catalogProvider.selected].sort())
    await allRadio.click()
    await expect(allRadio).toHaveAttribute('aria-checked', 'true')
    await expect.poll(rowIds).toEqual(allIds)
    addRow(page, {
      id: 'control.connect.models.filters',
      name: 'Lọc model · Đã chọn / Gợi ý / Tất cả',
      role: 'segmented radiogroup',
      state: 'all three filters exercised',
      enabled: true,
      disabledReason: null,
      action: 'click Gợi ý, Đã chọn, Tất cả and compare row ids to direct GET oracles',
      expected: 'Rows follow the selected filter without a provider PATCH',
      observed: `recommended=${recommendedPage.total}; selected=${catalogProvider.selected.length}; all=${allIds.length}`,
      api: `GET ${modelsPath}?limit=200&filter=recommended 200`,
      proof: 'native',
    })

    if (apiPage.models.length > 0) {
      const search = card.getByRole('searchbox')
      const query = apiPage.models[0].id
      const searchResponse = await page.request.get(`${modelsPath}?limit=200&q=${encodeURIComponent(query)}`)
      expect(searchResponse.status()).toBe(200)
      const searchPage = await searchResponse.json() as ModelPage
      const searchIds = expectedRows(searchPage)
      if (allIds.length > catalogProvider.selected.length + 1) expect(searchIds.length).toBeLessThan(allIds.length)
      await search.fill(query)
      await expect(search).toHaveValue(query)
      await expect.poll(rowIds).toEqual(searchIds)
      await capture(page, testInfo, 'settings-ledger-connect-models-search')
      await search.fill('')
      await expect(search).toHaveValue('')
      await expect.poll(rowIds).toEqual(allIds)
      addRow(page, {
        id: 'control.connect.models.search',
        name: 'Tìm model',
        role: 'searchbox',
        state: 'query and clear exercised',
        enabled: true,
        disabledReason: null,
        action: `fill with "${query}", then clear`,
        expected: 'Search narrows to genuine matches and clearing restores the unfiltered page',
        observed: `${searchPage.total} matches; restored ${allIds.length} visible rows`,
        api: `GET ${modelsPath}?limit=200&q=${encodeURIComponent(query)} 200`,
        proof: 'native',
      })
    } else {
      addGap(page, {
        id: 'conn.models.search',
        reason: 'Genuine catalog first page is empty, so no model id exists to search',
        needed: 'Catalog with at least one model',
      })
    }

    if (await card.locator('.conn-row').count() > 0) {
      const firstRow = card.locator('.conn-row').first()
      const checkbox = firstRow.getByRole('checkbox')
      const original = await checkbox.getAttribute('aria-checked')
      const beforeToggleProviders = await getJson<Provider[]>(page, '/api/ai/providers')
      const writesBeforeToggle = writeCount(page)
      await checkbox.click()
      await expect(page.locator('.conn-dirty')).toBeVisible()
      const unsavedProviders = await getJson<Provider[]>(page, '/api/ai/providers')
      expect(unsavedProviders.find((provider) => provider.id === catalogProvider.id)?.selected).toEqual(catalogProvider.selected)
      expect(writes(page).slice(writesBeforeToggle).filter((entry) => entry.method === 'PATCH')).toEqual([])
      await page.locator('.conn-dirty').getByRole('button', { name: 'Huỷ', exact: true }).click()
      await expect(page.locator('.conn-dirty')).toHaveCount(0)
      await expect(checkbox).toHaveAttribute('aria-checked', original ?? 'false')
      const afterCancelProviders = await getJson<Provider[]>(page, '/api/ai/providers')
      expect(afterCancelProviders.find((provider) => provider.id === catalogProvider.id)?.selected).toEqual(catalogProvider.selected)
      expect(beforeToggleProviders.find((provider) => provider.id === catalogProvider.id)?.selected).toEqual(catalogProvider.selected)
      expect(writes(page).slice(writesBeforeToggle)).toEqual([])
      addRow(page, {
        id: 'control.connect.selection-cancel',
        name: 'Chọn model rồi Huỷ',
        role: 'row checkbox + unsaved bar',
        state: 'draft toggled and cancelled',
        enabled: true,
        disabledReason: null,
        action: 'toggle one row checkbox; cancel the unsaved bar; read provider back from GET',
        expected: 'Local draft shows the unsaved bar; Huỷ restores server selection with no PATCH',
        observed: `checkbox ${original}→${original === 'true' ? 'false' : 'true'}→${original}; GET selected unchanged; 0 PATCH`,
        api: 'GET /api/ai/providers 200; no PATCH',
        proof: 'native',
      })
    }
  } else {
    addGap(page, {
      id: 'conn.models.catalog',
      reason: `Genuine catalog endpoint returned HTTP ${apiResponse.status()}`,
      needed: 'Provider catalog endpoint with a successful API response',
    })
  }

  if (mobile) return

  const manualOpen = card.getByRole('button', { name: 'Không thấy model? Thêm theo tên' })
  await expect(manualOpen).toBeVisible()
  const writesBeforeManual = writeCount(page)
  await manualOpen.click()
  const manual = card.locator('.conn-manual')
  await expect(manual).toBeVisible()
  const manualName = manual.getByLabel('Tên model', { exact: true })
  const manualCheck = manual.getByRole('button', { name: 'Kiểm tra & thêm', exact: true })
  await expect(manualName).toHaveValue('')
  await expect(manualCheck).toBeDisabled()

  const catalogOnly = process.env.WORKFLOW_QA_CATALOG_ONLY_MANUAL === '1'
  if (catalogOnly) {
    // This branch is explicitly opt-in. ChatGPTAdapter.CheckModel reads the
    // real bundled Codex catalog only; unlike OpenAI-compatible, Anthropic,
    // Grok or Ollama checks it never calls a model. The target id must exist in
    // the live GET catalog, and both the provider kind and the exact POST path
    // are pinned before the request is allowed out of the browser.
    expect(direct?.kind, 'catalog-only manual case requires a direct ChatGPT provider').toBe('chatgpt')
    expect(direct?.auth, 'catalog-only manual case must use the subscription adapter').toBe('subscription')
    expect(direct?.status, 'catalog-only manual case requires a ready ChatGPT provider').toBe('ready')
    const expectedModel = process.env.WORKFLOW_QA_CATALOG_ONLY_MANUAL_MODEL
    expect(expectedModel, 'WORKFLOW_QA_CATALOG_ONLY_MANUAL_MODEL must name a model from the real GET catalog').toBeTruthy()
    expect(apiPage, 'the real catalog must be readable before choosing a manual target').toBeDefined()
    const catalogIds = apiPage!.models.map((model) => model.id)
    expect(catalogIds, 'manual target must exist in the real ChatGPT catalog').toContain(expectedModel)
    const checkPath = `/api/ai/providers/${catalogProvider.id}/models/check`
    allowedCatalogCheck = { path: checkPath, model: expectedModel! }
    const selectedBefore = (await getJson<Provider[]>(page, '/api/ai/providers')).find((provider) => provider.id === catalogProvider.id)!.selected
    expect(selectedBefore, 'manual target must start unselected so the draft change is observable').not.toContain(expectedModel)
    await manualName.fill(expectedModel!)
    await expect(manualName).toHaveValue(expectedModel!)
    await expect(manualCheck).toBeEnabled()
    const checkResponsePromise = page.waitForResponse((response) => {
      try {
        return response.request().method() === 'POST' && new URL(response.url()).pathname === checkPath
      } catch {
        return false
      }
    }, { timeout: 90_000 })
    await manualCheck.click()
    const checkResponse = await checkResponsePromise
    expect(checkResponse.status()).toBe(200)
    const checkBody = await checkResponse.json() as { ok: boolean; ms: number; caps?: string[]; contextK?: number; error?: string }
    expect(checkBody.ok).toBe(true)
    expect(checkBody.error).toBeUndefined()
    // The single permitted catalog-only check has completed; re-close the gate
    // before any later UI interaction.
    allowedCatalogCheck = null
    await expect(manual.locator('[role="status"]')).toContainText('trả lời được')
    await capture(page, testInfo, 'settings-ledger-connect-manual-success')
    await manual.getByRole('button', { name: 'Đóng', exact: true }).click()
    await expect(manual).toHaveCount(0)
    const targetRow = card.locator('.conn-row').filter({ hasText: expectedModel! }).first()
    await expect(targetRow.locator('.conn-row__id')).toHaveText(expectedModel!)
    await expect(targetRow.getByRole('checkbox')).toHaveAttribute('aria-checked', 'true')
    const dirty = page.locator('.conn-dirty')
    await expect(dirty).toBeVisible()
    const modelReadbackResponse = await page.request.get(`${modelsPath}?limit=200&q=${encodeURIComponent(expectedModel!)}`)
    expect(modelReadbackResponse.status()).toBe(200)
    const modelReadback = await modelReadbackResponse.json() as CatalogReadback
    const readback = modelReadback.models.find((model) => model.id === expectedModel)
    expect(readback, 'GET catalog must return the checked model').toBeDefined()
    expect(readback!.manual, 'checked model row must read back manual:true').toBe(true)
    const selectedAfterCheck = (await getJson<Provider[]>(page, '/api/ai/providers')).find((provider) => provider.id === catalogProvider.id)!.selected
    expect(selectedAfterCheck).toEqual(selectedBefore)
    expect(writes(page).filter((entry) => entry.method === 'PATCH')).toEqual([])
    await dirty.getByRole('button', { name: 'Huỷ', exact: true }).click()
    await expect(dirty).toHaveCount(0)
    await expect(targetRow.getByRole('checkbox')).toHaveAttribute('aria-checked', 'false')
    const selectedAfterCancel = (await getJson<Provider[]>(page, '/api/ai/providers')).find((provider) => provider.id === catalogProvider.id)!.selected
    expect(selectedAfterCancel).toEqual(selectedBefore)
    expect(writes(page).filter((entry) => entry.method === 'PATCH')).toEqual([])
    const checkCalls = writes(page).slice(writesBeforeManual)
    expect(checkCalls).toEqual([expect.objectContaining({ method: 'POST', path: checkPath, allowed: true })])
    addRow(page, {
      id: 'Settings.conn.manual',
      name: 'Thêm model theo tên',
      role: 'inline manual panel',
      state: 'catalog-only check succeeded, draft cancelled',
      enabled: true,
      disabledReason: null,
      action: `check "${expectedModel}" against the real ChatGPT bundled catalog, then cancel the draft`,
      expected: 'POST responds ok:true, model reads back manual:true, draft selects it, cancel restores server selection',
      observed: `HTTP 200 ok=${checkBody.ok} ms=${checkBody.ms}; GET manual=${readback!.manual}; draft checkbox true then false; selected unchanged; 0 provider PATCH`,
      api: `POST ${checkPath} 200; GET ${modelsPath}?q=${encodeURIComponent(expectedModel!)} 200`,
      proof: 'native',
    })
    addRow(page, {
      id: 'control.connect.manual-check',
      name: 'Kiểm tra & thêm · ChatGPT catalog-only',
      role: 'manual check action',
      state: 'success + cancel',
      enabled: true,
      disabledReason: null,
      action: 'type real catalog id; click; verify response, GET manual flag and draft; cancel instead of Save',
      expected: 'ChatGPT check resolves from codex debug models --bundled, no inference; no provider PATCH',
      observed: `only allowed POST was ${checkPath}; readback manual:true; no PATCH or DELETE`,
      api: `POST ${checkPath} 200; GET /api/ai/providers 200`,
      proof: 'native',
    })
    await testInfo.attach('settings-ledger-manual-catalog-only.json', {
      body: JSON.stringify({
        provider: { id: catalogProvider.id, kind: catalogProvider.kind, auth: catalogProvider.auth },
        expectedModel,
        checkPath,
        response: { status: checkResponse.status(), body: checkBody },
        modelReadback: readback,
        selectedBefore,
        selectedAfterCheck,
        selectedAfterCancel,
        browserRequests: checkCalls,
        serverSideEffect: {
          table: 'ai_models',
          primaryKey: { providerId: catalogProvider.id, id: expectedModel },
          columnsRewrittenByUpsert: ['caps', 'context_k', 'in_per_m', 'out_per_m', 'per_image', 'recommended', 'manual'],
          mutation: 'manual set to true; adapter-provided catalog fields upserted',
          restoreOwner: 'parent controller (owned QA CAS snapshot/restore)',
        },
        inferenceCalls: 0,
      }, null, 2),
      contentType: 'application/json',
    })
    return
  }

  await manualName.fill('qa-ledger-never-submitted')
  await expect(manualName).toHaveValue('qa-ledger-never-submitted')
  await expect(manualCheck).toBeEnabled()
  await capture(page, testInfo, 'settings-ledger-connect-manual-cancel')
  await manual.getByRole('button', { name: 'Đóng', exact: true }).click()
  await expect(manual).toHaveCount(0)
  await expectNoNewWrites(page, writesBeforeManual, 'manual model panel')
  expect(writes(page).filter((entry) => entry.path.endsWith('/models/check'))).toEqual([])
  addRow(page, {
    id: 'Settings.conn.manual',
    name: 'Thêm model theo tên',
    role: 'inline manual panel',
    state: 'opened, validated and cancelled',
    enabled: true,
    disabledReason: null,
    action: 'open manual; verify empty→disabled, non-empty→enabled; close without check',
    expected: 'Input validation and cancel; no POST models/check and no inference',
    observed: 'empty disabled; non-empty enabled; closed with 0 writes and 0 models/check',
    api: 'no POST /api/ai/providers/:id/models/check',
    proof: 'native',
  })
  addGap(page, {
    id: 'conn.manual.success',
    reason: 'Successful manual check was not run in this invocation; this fixture only exercised the cancel path, and non-ChatGPT adapters would make a real model call',
    needed: 'Set WORKFLOW_QA_CATALOG_ONLY_MANUAL=1 and WORKFLOW_QA_CATALOG_ONLY_MANUAL_MODEL to a real ChatGPT catalog id, with controller-owned row restore',
    owner: 'parent acceptance',
  })
}

async function exerciseAccounts(page: Page, testInfo: TestInfo, mobile: boolean, platforms: Platform[]) {
  for (const platform of platforms) {
    const card = page.getByRole('region', { name: platform.label })
    await expect(card).toBeVisible()
    await expect(card).toContainText(expectedPlatformStatus(platform, mobile))
  }
  const beforePlatforms = platforms.map(platformRecord)
  for (const platform of platforms) {
    if (platform.login.saved) continue
    const login = page.getByRole('region', { name: platform.label }).getByRole('button', { name: /Đăng nhập|Quét mã|Đăng nhập bằng mã QR/ })
    // Logged-out login buttons are asserted only; they start external sessions.
    if (await login.count() > 0) await expect(login.first()).toBeVisible()
  }
  const writesBefore = writeCount(page)
  let cancelled = ''
  const youtube = platforms.find((platform) => platform.id === 'youtube')
  if (!mobile && youtube?.login.saved) {
    const select = page.getByRole('combobox', { name: 'Lấy cookie từ trình duyệt' })
    if (await select.count() > 0) {
      await select.click()
      const off = page.getByRole('listbox').getByRole('option', { name: 'Tắt', exact: true })
      await expect(off).toBeVisible()
      await off.click()
      const dialog = page.getByRole('alertdialog')
      await expect(dialog).toBeVisible()
      await expect(dialog).toContainText('Tắt cookie YouTube?')
      await expect(dialog).toContainText('Hồ sơ trình duyệt không bị ảnh hưởng.')
      await dialog.getByRole('button', { name: 'Huỷ', exact: true }).click()
      await expect(dialog).toHaveCount(0)
      cancelled = 'YouTube Tắt-cookie confirmation cancelled'
    }
  }
  if (!cancelled && !mobile) {
    const bilibili = platforms.find((platform) => platform.id === 'bilibili')
    if (bilibili?.login.saved) {
      const clear = page.getByRole('button', { name: 'Xoá đăng nhập', exact: true })
      if (await clear.count() > 0) {
        await clear.first().click()
        const dialog = page.getByRole('alertdialog')
        await expect(dialog).toBeVisible()
        await expect(dialog).toContainText('Xoá đăng nhập Bilibili?')
        await expect(dialog).toContainText('Cookie đã lưu bị xoá')
        await dialog.getByRole('button', { name: 'Huỷ', exact: true }).click()
        await expect(dialog).toHaveCount(0)
        cancelled = 'Bilibili Xoá đăng nhập confirmation cancelled'
      }
    }
  }
  const afterPlatforms = await getJson<Platform[]>(page, '/api/platforms')
  expect(afterPlatforms.map(platformRecord)).toEqual(beforePlatforms)
  expect(writes(page).slice(writesBefore).filter((entry) => entry.method === 'DELETE')).toEqual([])
  await expectNoNewWrites(page, writesBefore, 'accounts dialog cancel')
  if (cancelled) {
    addRow(page, {
      id: 'control.accounts.dialog-cancel',
      name: 'Xác nhận đăng xuất nền tảng',
      role: 'warm confirm dialog',
      state: 'opened and cancelled',
      enabled: true,
      disabledReason: null,
      action: 'open native disconnect confirmation; press Huỷ only',
      expected: 'Consequence is explicit; cancel leaves the saved login and platform state unchanged',
      observed: `${cancelled}; GET /api/platforms unchanged; 0 DELETE`,
      api: 'GET /api/platforms 200; no DELETE',
      proof: 'native',
    })
    await capture(page, testInfo, 'settings-ledger-accounts-cancel-dialog')
    return
  }
  addRow(page, {
    id: 'control.accounts.dialog-cancel',
    name: 'Xác nhận đăng xuất nền tảng',
    role: mobile ? 'not present on MSettings' : 'disconnect confirmation',
    state: 'blocked by fixture',
    enabled: false,
    disabledReason: mobile
      ? 'MSettings renders no platform logout/clear control; desktop-only disconnect dialog cannot be opened without changing fixture state'
      : 'No platform has a saved login in this fixture; creating one would require an external login handshake, which is forbidden',
    action: 'none',
    expected: 'Not claimed',
    observed: `saved logins: ${platforms.filter((platform) => platform.login.saved).map((platform) => platform.id).join(', ') || 'none'}`,
    api: 'GET /api/platforms 200',
    proof: 'blocked',
  })
  addGap(page, {
    id: mobile ? 'accounts.dialog.mobile' : 'accounts.dialog.fixture',
    reason: mobile
      ? 'MSettings exposes no platform disconnect/clear action, so no consequence dialog exists to cancel'
      : 'All genuine platform logins are unsaved; an external login/logout handshake is not authorized',
    needed: mobile ? 'PR6 mobile parity ownership' : 'A pre-seeded saved login in the isolated fixture with its disconnect dialog',
    owner: mobile ? 'PR6' : 'parent acceptance',
  })
}

async function exerciseStorage(page: Page, testInfo: TestInfo, mobile: boolean, server: boolean) {
  const writesBefore = writeCount(page)
  const reclaim = await getOptionalJson<{ items: ReclaimItem[] }>(page, '/api/storage/reclaim')
  if (!mobile && !server && reclaim) {
    let cancelled = ''
    for (const item of reclaim.items) {
      if (item.count <= 0) continue
      const label = item.action ?? (item.id === 'src' ? 'Xoá video gốc' : 'Xoá MP4 đã đăng')
      const button = page.getByRole('button', { name: label, exact: true })
      if (await button.count() === 0 || !(await button.first().isEnabled())) continue
      await button.first().click()
      const dialog = page.getByRole('alertdialog')
      await expect(dialog).toBeVisible()
      await expect(dialog).toContainText(item.id === 'src' ? 'Không hoàn tác được.' : 'Xuất lại mất khoảng 3 phút mỗi video.')
      await dialog.getByRole('button', { name: 'Huỷ', exact: true }).click()
      await expect(dialog).toHaveCount(0)
      cancelled = `${label} confirmation cancelled`
      break
    }
    if (cancelled) {
      await capture(page, testInfo, 'settings-ledger-storage-reclaim-cancel')
      addRow(page, {
        id: 'control.storage.reclaim-cancel',
        name: 'Xoá nhóm media · xác nhận',
        role: 'inline alertdialog',
        state: 'opened and cancelled',
        enabled: true,
        disabledReason: null,
        action: 'open destructive confirmation; press Huỷ only',
        expected: 'Consequence text is explicit; no DELETE/POST reclaim request',
        observed: `${cancelled}; 0 storage write`,
        api: 'GET /api/storage/reclaim 200; no reclaim DELETE',
        proof: 'native',
      })
    } else {
      addGap(page, {
        id: 'storage.reclaim-cancel',
        reason: 'Both reclaim groups have count 0 in this fixture, so their destructive buttons are disabled and no confirmation can be opened',
        needed: 'Isolated fixture with a finished/deleted video group, or acceptance of the disabled state',
      })
    }
  }

  if (!server) {
    const storage = await getJson<StorageStatus>(page, '/api/storage')
    expect(storage.projects.usedBytes).toBeGreaterThanOrEqual(0)
  }

  // Preview is source-proven non-destructive; the "Dọn" button is never pressed.
  const previewButton = page.getByRole('button', { name: 'Xem trước những gì sẽ dọn' })
  if (!mobile && await previewButton.count() > 0 && await previewButton.first().isVisible() && await previewButton.first().isEnabled()) {
    const previewResponsePromise = page.waitForResponse((response) => {
      try {
        return response.request().method() === 'POST' && new URL(response.url()).pathname === '/api/storage/cleanup/preview'
      } catch {
        return false
      }
    }, { timeout: 60_000 })
    await previewButton.first().click()
    const previewResponse = await previewResponsePromise
    expect(previewResponse.status()).toBe(200)
    const preview = await previewResponse.json() as { previewId: string; freedBytes: number }
    expect(typeof preview.previewId).toBe('string')
    expect(typeof preview.freedBytes).toBe('number')
    const go = page.locator('.vv-sto__go')
    await expect(go).toBeVisible()
    await expect(go.getByRole('button', { name: 'Để sau', exact: true })).toBeVisible()
    await expect(go.getByRole('button', { name: /^Dọn / })).toBeVisible()
    await capture(page, testInfo, 'settings-ledger-storage-cleanup-consequence')
    await go.getByRole('button', { name: 'Để sau', exact: true }).click()
    await expect(page.locator('.vv-sto__go')).toHaveCount(0)
    await expect(previewButton.first()).toBeVisible()
    expect(writes(page).filter((entry) => /\/cleanup\/run$/.test(entry.path))).toEqual([])
    addRow(page, {
      id: 'control.storage.cleanup-cancel',
      name: 'Xem trước những gì sẽ dọn → Để sau',
      role: 'consequence list + cancel',
      state: 'preview shown and cancelled',
      enabled: true,
      disabledReason: null,
      action: 'open preview (source-proven no deletion), read consequence list, press Để sau; never press Dọn',
      expected: 'Consequence form lists what would be freed; cancel closes it and cleanup/run is never called',
      observed: `previewId issued; freedBytes=${preview.freedBytes}; cancel returned to idle; 0 cleanup/run`,
      api: 'POST /api/storage/cleanup/preview 200 (no deletion); cleanup/run blocked',
      proof: 'native',
    })
    return
  }

  addRow(page, {
    id: 'control.storage.cleanup-cancel',
    name: 'Xem trước những gì sẽ dọn → Để sau',
    role: mobile ? 'not rendered on MSettings storage' : 'cleanup consequence card',
    state: 'unavailable',
    enabled: false,
    disabledReason: mobile
      ? 'Mobile storage omits CleanupCard entirely; reclaim rows delete directly and have no cancel form'
      : 'CleanupCard is not present in this storage mode',
    action: 'none',
    expected: 'Not claimed',
    observed: 'no cleanup consequence button available without another storage mode',
    api: null,
    proof: 'blocked',
  })
  addGap(page, {
    id: 'storage.cleanup-cancel',
    reason: mobile
      ? 'MSettings storage has no cleanup preview/cancel form; its reclaim button deletes immediately'
      : 'This storage mode does not render CleanupCard',
    needed: mobile ? 'PR6 mobile storage parity acceptance' : 'Local/desktop storage mode with CleanupCard',
    owner: mobile ? 'PR6' : 'parent acceptance',
  })
  expect(writes(page).slice(writesBefore).filter((entry) => /\/cleanup\/run$/.test(entry.path))).toEqual([])
}

test('desktop agent model trigger keeps provider context, searchable popover and focus', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name.startsWith('mobile'), 'Desktop closed-trigger contract; mobile uses its separate model sheet')
  const before = await getJson<{ director: { model: { provider: string; model: string } }; artist: { promptModel: { provider: string; model: string } } }>(page, '/api/ai/agents')
  const providerBefore = await getJson<Provider[]>(page, '/api/ai/providers')
  await boot(page, '/#/settings?section=agents')
  for (const entry of [{ name: 'Đạo diễn', label: 'Model', ref: before.director.model }, { name: 'Hoạ sĩ', label: 'Model viết mô tả ảnh', ref: before.artist.promptModel }]) {
    const card = page.locator('.agt-row').filter({ has: page.getByText(entry.name, { exact: true }) })
    const trigger = card.getByRole('combobox', { name: entry.label, exact: true })
    const provider = providerBefore.find(candidate => candidate.id === entry.ref.provider)
    expect(provider).toBeDefined()
    await expect(trigger).toContainText(entry.ref.model)
    await expect(trigger).toContainText(provider!.label)
    await expect(card.locator('input[role="combobox"]')).toHaveCount(0)
    await capture(page, testInfo, `agent-${entry.name}-closed-context`)
    await trigger.click()
    const input = card.getByRole('combobox', { name: entry.label, exact: true })
    await expect(input).toBeFocused()
    const list = page.getByRole('listbox', { name: entry.label, exact: true })
    await expect(list).toBeVisible()
    await withinViewport(page, list.locator('..'))
    const popover = await list.locator('..').boundingBox()
    expect(Math.abs(popover!.width - 400)).toBeLessThanOrEqual(2)
    await input.fill(entry.ref.model)
    await expect(list.getByRole('option').first()).toContainText(entry.ref.model)
    await capture(page, testInfo, `agent-${entry.name}-model-search`)
    await page.keyboard.press('Escape')
    const returned = card.getByRole('combobox', { name: entry.label, exact: true })
    await expect(returned).toBeFocused()
    await expect(list).toHaveCount(0)
    await expect(returned).toContainText(entry.ref.model)
    addRow(page, {
      id: `control.agents.model-${entry.name}`, name: `Model ${entry.name}`, role: 'combobox', state: 'closed → searchable popover → Escape',
      enabled: true, disabledReason: null, action: 'open, search current model, Escape',
      expected: 'closed model + provider context; 400px bounded popover; search focus then return to trigger without changing selection',
      observed: `model=${entry.ref.model}; provider=${provider!.label}; popover=${popover!.width}px; Escape returned focus`,
      api: 'GET /api/ai/agents; GET /api/ai/providers; no writes', proof: 'native',
    })
  }
  await expectNoNewWrites(page, 0, 'Model search/cancel')
  expect(await getJson(page, '/api/ai/agents')).toEqual(before)
  expect(await getJson(page, '/api/ai/providers')).toEqual(providerBefore)
})

test('catalog-only manual model check selects the real ChatGPT model without saving provider selection', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name.startsWith('mobile'), 'Mobile manual-model parity is owned by PR6')
  test.skip(process.env.WORKFLOW_QA_CATALOG_ONLY_MANUAL !== '1', 'Controller must own and restore the actual catalog row')
  const providers = await getJson<Provider[]>(page, '/api/ai/providers')
  await boot(page, '/#/settings?section=connect')
  await exerciseCatalog(page, testInfo, false, providers)
  expect(await getJson(page, '/api/ai/providers')).toEqual(providers)
})

// Opt-in real-write proof for the user-reported agent autosave question. The
// controller (private run-owned-agent-selection.py) snapshots the raw
// settings/ai.agents row, ai_providers/ai_models rows, the original source job,
// the editor GET and image_receipts, runs this exact title in desktop normal +
// reduced, then restores the raw rows exactly. Only two writes are authorized:
//   PATCH /api/ai/providers/<chatgpt> {selected: [before...,gpt-5.5]}
//   PUT   /api/ai/agents               {director,artist} complete document
// The agents PUT is held at the browser route barrier (continue only after the
// pre-response assertions), never fulfilled or mocked.
test('agent model autosave: real ChatGPT gpt-5.5 is visible before the PUT resolves and persists after reload', async ({ page }, testInfo) => {
  test.setTimeout(180_000)
  test.skip(testInfo.project.name.startsWith('mobile'), 'Desktop agent autosave contract; mobile uses its separate model sheet')
  test.skip(process.env.WORKFLOW_QA_AGENT_SELECTION !== '1', 'Controller must own the real ChatGPT selected/agents writes and restore them')
  const target = AGENT_SELECTION_TARGET
  if (process.env.WORKFLOW_QA_AGENT_SELECTION_MODEL) {
    expect(process.env.WORKFLOW_QA_AGENT_SELECTION_MODEL).toBe(target)
  }

  // Preflight: the real ChatGPT provider, the real bundled target catalog row
  // with text+vision caps, and the current director/prompt refs. No model check
  // or inference is involved anywhere in this test.
  const providersBefore = await getJson<Provider[]>(page, '/api/ai/providers')
  const chatgpt = providersBefore.find((provider) => provider.kind === 'chatgpt')
  expect(chatgpt, 'a genuine ChatGPT provider is required').toBeDefined()
  const catalogAdded = !chatgpt!.selected.includes(target)
  if (catalogAdded) expect(chatgpt!.selected).not.toContain(target)
  const catalogResponse = await page.request.get(`/api/ai/providers/${chatgpt!.id}/models?q=${encodeURIComponent(target)}&limit=200`)
  expect(catalogResponse.status()).toBe(200)
  const catalog = await catalogResponse.json() as ModelPage
  const targetModel = catalog.models.find((model) => model.id === target)
  expect(targetModel, `${target} must exist in the real ChatGPT catalog`).toBeDefined()
  expect(targetModel!.caps).toEqual(expect.arrayContaining(['text', 'vision']))
  const agentsBefore = await getJson<AgentDoc>(page, '/api/ai/agents')
  expect(agentsBefore.director.model.provider).toBe(chatgpt!.id)
  expect(agentsBefore.artist.promptModel.provider).toBe(chatgpt!.id)

  // ---- Step 1: real provider catalog PATCH selected through the native UI. ----
  // A sibling desktop project may already have added the target; then this run
  // proves that persisted selection instead of writing it a second time.
  await boot(page, '/#/settings?section=connect')
  await expectTitle(page, false, 'Kết nối AI')
  let connectCard = page.getByRole('region', { name: chatgpt!.label })
  await expect(connectCard.getByRole('button', { name: /^(Model · \d+|Chọn model)$/ }).first()).toBeVisible()
  await connectCard.getByRole('button', { name: /^(Model · \d+|Chọn model)$/ }).first().click()
  await expect(connectCard.getByRole('list', { name: chatgpt!.label, exact: true })).toBeVisible()
  await connectCard.locator('.conn-cat__tools input[type="search"]').fill(target)
  const targetBox = connectCard.getByRole('checkbox', { name: target, exact: true })
  await expect(targetBox).toBeVisible()
  if (catalogAdded) {
    await expect(targetBox).toHaveAttribute('aria-checked', 'false')
    const expectedSelected = [...chatgpt!.selected, target]
    allowedProviderPatch = { path: `/api/ai/providers/${chatgpt!.id}`, selected: expectedSelected }
    await targetBox.click()
    const dirty = page.locator('.conn-dirty')
    await expect(dirty).toBeVisible()
    const [patchResponse] = await Promise.all([
      page.waitForResponse((response) => new URL(response.url()).pathname === `/api/ai/providers/${chatgpt!.id}` && response.request().method() === 'PATCH'),
      dirty.getByRole('button', { name: 'Lưu', exact: true }).click(),
    ])
    expect(patchResponse.status()).toBe(200)
    const patched = await patchResponse.json() as Provider
    expect(patched.selected).toEqual(expectedSelected)
    await expect.poll(async () => (await getJson<Provider[]>(page, '/api/ai/providers')).find((provider) => provider.id === chatgpt!.id)!.selected).toEqual(expectedSelected)
    await capture(page, testInfo, 'agent-selection-connect-selected')
    // Reload proves the selected draft is real server state, not local state.
    await page.reload({ waitUntil: 'domcontentloaded' })
    await expectTitle(page, false, 'Kết nối AI')
    connectCard = page.getByRole('region', { name: chatgpt!.label })
    await connectCard.getByRole('button', { name: /^(Model · \d+|Chọn model)$/ }).first().click()
    await connectCard.locator('.conn-cat__tools input[type="search"]').fill(target)
    await expect(connectCard.getByRole('checkbox', { name: target, exact: true })).toHaveAttribute('aria-checked', 'true')
    allowedProviderPatch = null
    addRow(page, {
      id: 'control.agents.catalog-selected', name: `ChatGPT chọn ${target}`, role: 'provider catalog checkbox + Lưu', state: 'saved through native UI',
      enabled: true, disabledReason: null, action: 'tick real catalog model, press Lưu, reload and re-read the catalog',
      expected: 'PATCH selected 200; GET/reload keep the new id selected', observed: `PATCH 200 selected=${expectedSelected.join(',')}; reload checkbox checked`,
      api: `PATCH /api/ai/providers/${chatgpt!.id} 200; GET /api/ai/providers 200`, proof: 'native',
    })
  } else {
    await expect(targetBox).toHaveAttribute('aria-checked', 'true')
    addRow(page, {
      id: 'control.agents.catalog-selected', name: `ChatGPT chọn ${target}`, role: 'provider catalog checkbox', state: 'already selected by the sibling desktop project in this same guarded run',
      enabled: true, disabledReason: null, action: 'open catalog and read the persisted selected state',
      expected: 'target remains selected from server state', observed: `selected=${chatgpt!.selected.join(',')}`,
      api: 'GET /api/ai/providers 200', proof: 'native',
    })
  }

  // ---- Step 2: agent director model, with the real PUT held at the route. ----
  allowedAgentsPut = true
  await boot(page, '/#/settings?section=agents')
  await expectTitle(page, false, 'Trợ lý AI')
  const agentSection = page.locator('.agt-sec')
  await expect(agentSection.getByRole('button', { name: 'Lưu', exact: true })).toHaveCount(0)
  const directorCard = page.locator('.agt-row').filter({ has: page.getByText('Đạo diễn', { exact: true }) })
  const directorTrigger = directorCard.getByRole('combobox', { name: 'Model', exact: true })
  await expect(directorTrigger).toContainText(agentsBefore.director.model.model)

  // The reduced project starts on the target because the normal project just
  // saved it; prove immediate visibility with another enabled text model there
  // and end both projects on the target. No extra catalog data is invented.
  const agentsAtStart = await getJson<AgentDoc>(page, '/api/ai/agents')
  const selectedCatalogResponse = await page.request.get(`/api/ai/providers/${chatgpt!.id}/models?filter=selected&limit=200`)
  expect(selectedCatalogResponse.status()).toBe(200)
  const selectedCatalog = await selectedCatalogResponse.json() as ModelPage
  const immediateModel = agentsAtStart.director.model.model === target
    ? selectedCatalog.models.find((model) => model.id !== target && model.caps.includes('text'))?.id ?? null
    : target
  expect(immediateModel, 'an enabled text model different from the current director model is required').not.toBeNull()

  const putBodies: AgentDoc[] = []
  const directorGate = Promise.withResolvers<void>()
  const directorHeld = Promise.withResolvers<void>()
  let heldBody: AgentDoc | undefined
  await page.route('**/api/ai/agents', async (route) => {
    const request = route.request()
    if (request.method() !== 'PUT') return route.continue()
    const body = request.postDataJSON() as AgentDoc
    putBodies.push(body)
    if (putBodies.length === 1) {
      heldBody = body
      directorHeld.resolve()
      await directorGate.promise
    }
    await route.continue()
  })

  await directorTrigger.click()
  const directorList = page.getByRole('listbox', { name: 'Model', exact: true })
  await expect(directorList).toBeVisible()
  const immediateOption = directorList.getByRole('option').filter({ hasText: immediateModel! })
  await expect(immediateOption).toHaveCount(1)
  await expect(immediateOption).toBeEnabled()
  await Promise.all([directorHeld.promise, immediateOption.click()])
  expect(heldBody, 'the real PUT must be held before its response').toBeDefined()
  expect(heldBody!.director.model).toEqual({ provider: chatgpt!.id, model: immediateModel! })
  expect(Object.keys(heldBody!).sort()).toEqual(['artist', 'director'])
  expect(isFullAgentDoc(heldBody)).toBe(true)
  // BEFORE the response: closed trigger already shows the new model/provider,
  // the popover is gone, the only affordance is the saving status, and there is
  // no manual Save button anywhere in the section.
  const heldResponsePromise = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/ai/agents' && response.request().method() === 'PUT')
  let gateReleased = false
  const releaseGate = () => { if (!gateReleased) { gateReleased = true; directorGate.resolve() } }
  page.once('close', releaseGate)
  const gateTimer = setTimeout(releaseGate, 60_000)
  gateTimer.unref?.()
  try {
    const heldTrigger = directorCard.getByRole('combobox', { name: 'Model', exact: true })
    await expect(heldTrigger).toContainText(immediateModel!)
    await expect(heldTrigger).toContainText(chatgpt!.label)
    await expect(page.getByRole('listbox', { name: 'Model', exact: true })).toHaveCount(0)
    await expect(page.locator('.agt-applied')).toHaveText('Đang lưu…')
    await expect(agentSection.getByRole('button', { name: 'Lưu', exact: true })).toHaveCount(0)
    await capture(page, testInfo, 'agent-selection-director-held-put')
  } finally {
    clearTimeout(gateTimer)
    releaseGate()
  }
  const heldResponse = await heldResponsePromise
  expect(heldResponse.status()).toBe(200)
  const heldDoc = await heldResponse.json() as AgentDoc
  expect(heldDoc.director.model).toEqual({ provider: chatgpt!.id, model: immediateModel! })
  await expect(page.locator('.agt-applied')).toHaveCount(0)
  await expect.poll(async () => (await getJson<AgentDoc>(page, '/api/ai/agents')).director.model.model).toBe(immediateModel!)

  if (immediateModel !== target) {
    await directorTrigger.click()
    const targetList = page.getByRole('listbox', { name: 'Model', exact: true })
    await expect(targetList).toBeVisible()
    const targetOption = targetList.getByRole('option').filter({ hasText: target })
    await expect(targetOption).toHaveCount(1)
    await expect(targetOption).toBeEnabled()
    const targetResponse = await putAgents(page, () => targetOption.click())
    expect(targetResponse.status()).toBe(200)
    expect((await targetResponse.json() as AgentDoc).director.model).toEqual({ provider: chatgpt!.id, model: target })
    await expect(page.locator('.agt-applied')).toHaveCount(0)
    await expect.poll(async () => (await getJson<AgentDoc>(page, '/api/ai/agents')).director.model.model).toBe(target)
  }

  // Full reload: the closed trigger and GET both read the persisted target.
  await page.reload({ waitUntil: 'domcontentloaded' })
  await expectTitle(page, false, 'Trợ lý AI')
  const reloadedDirector = page.locator('.agt-row').filter({ has: page.getByText('Đạo diễn', { exact: true }) }).getByRole('combobox', { name: 'Model', exact: true })
  await expect(reloadedDirector).toContainText(target)
  expect((await getJson<AgentDoc>(page, '/api/ai/agents')).director.model).toEqual({ provider: chatgpt!.id, model: target })
  await expect(page.locator('.agt-sec').getByRole('button', { name: 'Lưu', exact: true })).toHaveCount(0)
  await capture(page, testInfo, 'agent-selection-reload-director')
  addRow(page, {
    id: 'control.agents.director-autosave', name: 'Model Đạo diễn', role: 'model combobox autosave', state: 'native pick + held real PUT + reload',
    enabled: true, disabledReason: null, action: `choose ${immediateModel} while the real PUT is held, release, then reload`,
    expected: `closed trigger shows ${immediateModel} before the response; PUT 200; GET/reload keep it`,
    observed: `held PUT body director=${immediateModel}; pre-response label visible; 200; GET/reload=${target}`,
    api: 'PUT /api/ai/agents 200; GET /api/ai/agents 200', proof: 'native',
  })

  // ---- Step 3: every currently enabled reasoning choice, real PUT readback. ----
  const agentsSection = page.locator('.agt-sec')
  const effortGroup = agentsSection.getByRole('radiogroup', { name: 'Mức suy nghĩ' })
  await expect(effortGroup).toBeVisible()
  const effortLabels: Record<EffortChoice, string> = { auto: 'Tự động', low: 'Thấp', medium: 'Vừa', high: 'Cao' }
  const effortChoices: EffortChoice[] = ['auto', 'low', 'medium', 'high']
  let currentEffort = (await getJson<AgentDoc>(page, '/api/ai/agents')).director.effort as EffortChoice
  expect(effortChoices).toContain(currentEffort)
  for (const effort of effortChoices) {
    await expect(effortGroup.getByRole('radio', { name: effortLabels[effort], exact: true })).toBeEnabled()
  }
  await expect(effortGroup.getByRole('radio', { name: effortLabels[currentEffort], exact: true })).toHaveAttribute('aria-checked', 'true')
  const effortClicks = effortChoices.filter((effort) => effort !== currentEffort)
  for (const effort of effortClicks) {
    const radio = effortGroup.getByRole('radio', { name: effortLabels[effort], exact: true })
    const response = await putAgents(page, () => radio.click())
    expect(response.status()).toBe(200)
    expect((await response.json() as AgentDoc).director.effort).toBe(effort)
    await expect(page.locator('.agt-applied')).toHaveCount(0)
    await expect.poll(async () => (await getJson<AgentDoc>(page, '/api/ai/agents')).director.effort).toBe(effort)
    await expect(effortGroup.getByRole('radio', { name: effortLabels[effort], exact: true })).toHaveAttribute('aria-checked', 'true')
    currentEffort = effort
  }
  addRow(page, {
    id: 'control.agents.effort-autosave', name: 'Mức suy nghĩ', role: 'segmented radiogroup autosave', state: 'all enabled choices exercised with real PUT',
    enabled: true, disabledReason: null, action: `click ${effortClicks.map((effort) => effortLabels[effort]).join(', ')} and read back`,
    expected: 'each enabled choice autosaves immediately with no Save button', observed: `PUT/GET effort=${currentEffort}; options ${effortChoices.join('/')}`,
    api: 'PUT /api/ai/agents 200 × choice; GET 200', proof: 'native',
  })

  // ---- Step 4: artist prompt model target if the real catalog exposes it. ----
  const artistCard = page.locator('.agt-row').filter({ has: page.getByText('Hoạ sĩ', { exact: true }) })
  const promptTrigger = artistCard.getByRole('combobox', { name: 'Model viết mô tả ảnh', exact: true })
  const currentPrompt = (await getJson<AgentDoc>(page, '/api/ai/agents')).artist.promptModel.model
  const otherPrompt = selectedCatalog.models.find((model) => model.id !== target && model.caps.includes('vision'))?.id
  const promptChoices = currentPrompt === target ? [otherPrompt, target] : [target]
  expect(promptChoices.every(Boolean), 'an alternative enabled vision model is required to prove a real prompt-model change').toBe(true)
  for (const model of promptChoices) {
    await promptTrigger.click()
    const promptList = page.getByRole('listbox', { name: 'Model viết mô tả ảnh', exact: true })
    await expect(promptList).toBeVisible()
    const promptOption = promptList.getByRole('option').filter({ hasText: model! })
    await expect(promptOption, `${model} must be an enabled vision option for the artist prompt`).toHaveCount(1)
    await expect(promptOption).toBeEnabled()
    const promptResponse = await putAgents(page, () => promptOption.click())
    expect(promptResponse.status()).toBe(200)
    expect((await promptResponse.json() as AgentDoc).artist.promptModel).toEqual({ provider: chatgpt!.id, model })
    await expect(page.locator('.agt-applied')).toHaveCount(0)
    await expect.poll(async () => (await getJson<AgentDoc>(page, '/api/ai/agents')).artist.promptModel).toEqual({ provider: chatgpt!.id, model })
    await expect(artistCard.getByRole('combobox', { name: 'Model viết mô tả ảnh', exact: true })).toContainText(model!)
  }
  addRow(page, {
    id: 'control.agents.prompt-autosave', name: 'Model viết mô tả ảnh', role: 'vision model combobox autosave', state: 'native pick with real PUT',
    enabled: true, disabledReason: null, action: `choose ${target} in the artist prompt picker`,
    expected: `${target} enabled because the real catalog row has text+vision`, observed: `PUT/GET artist.promptModel=${target}`,
    api: 'PUT /api/ai/agents 200; GET 200', proof: 'native',
  })

  const providersNow = await getJson<Provider[]>(page, '/api/ai/providers')
  const imageProviders: string[] = []
  for (const candidate of providersNow.filter((provider) => provider.status === 'ready' && provider.selected.length > 0)) {
    const selectedResponse = await page.request.get(`/api/ai/providers/${candidate.id}/models?filter=selected&limit=200`)
    if (!selectedResponse.ok()) continue
    const selectedPage = await selectedResponse.json() as ModelPage
    if (selectedPage.models.some((model) => model.caps.includes('image'))) imageProviders.push(candidate.id)
  }
  expect(imageProviders).toContain(chatgpt!.id)
  // ---- Step 5: fallbackOn off/on through real PUTs when currently enabled. ----
  let fallbackOn = (await getJson<AgentDoc>(page, '/api/ai/agents')).artist.fallbackOn
  const retrySwitch = page.locator('.agt-sec').getByRole('switch', { name: 'Vẽ lại bằng nguồn khác khi lỗi', exact: true })
  await expect(retrySwitch).toHaveCount(1)
  const setFallback = async (want: boolean) => {
    await expect(retrySwitch).toBeEnabled()
    const response = await putAgents(page, () => retrySwitch.click())
    expect(response.status()).toBe(200)
    expect((await response.json() as AgentDoc).artist.fallbackOn).toBe(want)
    await expect(page.locator('.agt-applied')).toHaveCount(0)
    await expect.poll(async () => (await getJson<AgentDoc>(page, '/api/ai/agents')).artist.fallbackOn).toBe(want)
    await expect(page.locator('.agt-sec').getByRole('switch', { name: 'Vẽ lại bằng nguồn khác khi lỗi', exact: true })).toHaveAttribute('aria-checked', String(want))
    fallbackOn = want
  }
  if (imageProviders.length >= 2) {
  if (fallbackOn) {
    await setFallback(false)
    await setFallback(true)
  } else {
    await setFallback(true)
    await setFallback(false)
  }
  addRow(page, {
    id: 'control.agents.fallback-autosave', name: 'Vẽ lại bằng nguồn khác khi lỗi', role: 'switch autosave', state: fallbackOn ? 'off then on through real PUTs' : 'on then off through real PUTs',
    enabled: true, disabledReason: null, action: 'toggle the switch twice and read back each server state',
    expected: 'each toggle autosaves with no Save button', observed: `PUT/GET fallbackOn restored to ${fallbackOn}`,
    api: 'PUT /api/ai/agents 200 ×2; GET 200', proof: 'native',
  })
  } else {
    addRow(page, {
      id: 'control.agents.fallback-autosave', name: 'Vẽ lại bằng nguồn khác khi lỗi', role: 'switch autosave', state: 'blocked in current fixture',
      enabled: await retrySwitch.isEnabled(), disabledReason: null,
      action: 'not exercised without a second selected image source',
      expected: 'A successful fallback assignment requires a second real selected image model',
      observed: 'A fallback source needs a real ready provider and selected image model; successful fallback save is not claimed without both',
      api: 'No fallback PUT in this run', proof: 'blocked',
    })
  }

  // ---- Alternative draw source: claim only what the fixture really exposes. ----
  // The draw radiogroup switches provider only when another provider has a
  // selected image model. Grok is logged out here, so no second source exists;
  // that is an explicit prerequisite, never a fabricated PASS.
  if (imageProviders.length < 2) {
    addRow(page, {
      id: 'control.agents.draw-alternative', name: 'Nguồn vẽ khác', role: 'draw source radiogroup', state: 'blocked in current fixture',
      enabled: false,
      disabledReason: `only ${imageProviders.join(', ') || 'no provider'} exposes a selected image model; there is no second fixed source to switch to`,
      action: 'none', expected: 'Not claimed without a real second image source',
      observed: `image providers=${imageProviders.join(',') || 'none'}`,
      api: 'GET /api/ai/providers + GET /models?filter=selected', proof: 'blocked',
    })
    addGap(page, {
      id: 'agents.draw-alternative',
      reason: 'Only one provider has a selected image model in this fixture, so switching "Vẽ ảnh bằng" to another provider cannot be exercised natively',
      needed: 'A second real image-capable provider selected in Kết nối AI (for example a real Grok login); no fake model or alternative is invented',
      owner: 'prerequisite',
    })
  }

  // ---- Write fence: only the controller-owned real writes happened. ----
  const expectedPutCount = (immediateModel === target ? 1 : 2) + effortClicks.length + promptChoices.length + (imageProviders.length >= 2 ? 2 : 0)
  const agentWrites = writes(page)
  expect(agentWrites.every((entry) => entry.allowed)).toBe(true)
  const kinds = [...new Set(agentWrites.map((entry) => `${entry.method} ${entry.path}`))].sort()
  expect(kinds).toEqual((catalogAdded
    ? [`PATCH /api/ai/providers/${chatgpt!.id}`, 'PUT /api/ai/agents']
    : ['PUT /api/ai/agents']).sort())
  expect(agentWrites.filter((entry) => entry.method === 'PATCH')).toHaveLength(catalogAdded ? 1 : 0)
  expect(agentWrites.filter((entry) => entry.method === 'PUT' && entry.path === '/api/ai/agents')).toHaveLength(expectedPutCount)
  expect(putBodies).toHaveLength(expectedPutCount)
  expect(putBodies.every((body) => isFullAgentDoc(body))).toBe(true)
  await capture(page, testInfo, 'agent-selection-final')
})
