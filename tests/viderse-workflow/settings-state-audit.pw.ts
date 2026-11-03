import { test, expect, type Page, type TestInfo, type Locator } from '@playwright/test'
import { readFileSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'

// Settings acceptance smoke for PR2. It boots the native Viderse app against the
// isolated workflow.qa fixture and exercises only states that the real isolated
// backend currently exposes:
//   • frame sections connect/agents/tools/accounts at desktop + mobile,
//   • the genuine provider list (including truthful logged-out subscriptions),
//   • every safe add-provider radio, field and disabled-submit state,
//   • mobile sheet scrim/Esc and focus/scroll restoration,
//   • model catalog filters/search/manual-cancel and models sheet when (and only
//     when) a genuine provider exposes the control,
//   • tools read-only rows/details; recheck POSTs only /api/tools/check.
// It never POSTs/PATCHes/DELETEs providers, models, agents, tools install,
// platform logins or paid endpoints. Unreachable artboard states are recorded as
// explicit state gaps instead of being faked or silently skipped.

const repo = resolve(__dirname, '../..')
const cookiePath = process.env.WORKFLOW_QA_COOKIE_FILE
const fixturePath = process.env.WORKFLOW_QA_FIXTURE
if (!cookiePath || !fixturePath) throw new Error('Private workflow.qa cookies and fixture are required')
if (realpathSync(cookiePath).startsWith(repo + sep)) throw new Error('Cookies must stay outside the repository')
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as { username?: string }
if (fixture.username !== 'workflow.qa') throw new Error('Only the isolated workflow.qa fixture is authorized')

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
type ToolRow = { id: string; state: string; version: string | null; provided?: boolean }
type ToolsStatus = { tools: ToolRow[]; checkedAt: string; dataDir?: string }
type Mutation = { method: string; path: string; search: string }
type Observation = { project: string; viewport: string; control: string; expected: string; observed: string; verified: boolean }
type Gap = { id: string; reason: string; needed: string }

const observations = new WeakMap<Page, Observation[]>()
const mutations = new WeakMap<Page, Mutation[]>()
const runtimeFaults = new WeakMap<Page, string[]>()

const ADD_KINDS = [
  { kind: 'openai', label: 'OpenAI', auth: 'key' },
  { kind: 'gemini', label: 'Gemini', auth: 'key' },
  { kind: 'openrouter', label: 'OpenRouter', auth: 'key' },
  { kind: 'deepseek', label: 'DeepSeek', auth: 'key' },
  { kind: 'xai', label: 'xAI', auth: 'key' },
  { kind: 'anthropic', label: 'Anthropic', auth: 'key' },
  { kind: 'ollama', label: 'Ollama', auth: 'url' },
  { kind: 'openai_compat', label: 'Tương thích OpenAI', auth: 'both' },
] as const

const TOOL_NAMES: Record<string, string> = {
  'yt-dlp': 'Tải video và phụ đề',
  ffmpeg: 'Xử lý hình và tiếng',
  transcription: 'Tách tiếng và tạo phụ đề',
  chrome: 'Dựng và kiểm tra video',
  codex: 'Công cụ vẽ ảnh',
  'codex-login': 'Đăng nhập ChatGPT',
}
const PILLS: Record<string, string> = {
  ready: 'Đã cài',
  update: 'Có bản mới',
  missing: 'Chưa cài',
  failed: 'Chưa cài được',
  installing: 'Đang cài',
  needs_login: 'Cần bạn',
}
const SECTION_TITLE: Record<string, string> = {
  tools: 'Công cụ trên máy',
  connect: 'Kết nối AI',
  agents: 'Trợ lý AI',
  accounts: 'Tài khoản nền tảng',
  storage: 'Lưu trữ',
  logs: 'Báo lỗi',
}

function isMobile(testInfo: TestInfo): boolean {
  return testInfo.project.name.startsWith('mobile')
}
function record(page: Page, testInfo: TestInfo, control: string, expected: string, observed: string, verified = true) {
  observations.get(page)!.push({
    project: testInfo.project.name,
    viewport: `${page.viewportSize()!.width}×${page.viewportSize()!.height}`,
    control,
    expected,
    observed,
    verified,
  })
}
async function attachGaps(page: Page, testInfo: TestInfo, gaps: Gap[]) {
  if (gaps.length === 0) return
  testInfo.annotations.push({ type: 'state-gap', description: gaps.map((gap) => `${gap.id}: ${gap.reason}`).join(' | ') })
  await testInfo.attach('settings-state-gaps.json', { body: JSON.stringify(gaps, null, 2), contentType: 'application/json' })
  record(page, testInfo, 'state-gaps', 'Every artboard tweak either exercised or explicitly reported', gaps.map((gap) => gap.id).join(', '), false)
}
function appMutations(page: Page): Mutation[] {
  return mutations.get(page) ?? []
}
function assertNoMutations(page: Page) {
  expect(appMutations(page)).toEqual([])
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
  if (mobile) await expect(page.locator('.vv-mset__title')).toHaveText(title)
  else await expect(page.locator('.vv-set__title')).toHaveText(title)
}
function waitApi(page: Page, predicate: (url: URL) => boolean, timeout = 30_000) {
  return page.waitForResponse((response) => {
    try {
      return response.request().method() === 'GET' && predicate(new URL(response.url()))
    } catch {
      return false
    }
  }, { timeout })
}
async function getJson<T>(page: Page, path: string): Promise<T> {
  const response = await page.request.get(path)
  expect(response.status(), path).toBe(200)
  return response.json() as Promise<T>
}

test.beforeEach(async ({ context, page }) => {
  observations.set(page, [])
  mutations.set(page, [])
  const faults: string[] = []
  runtimeFaults.set(page, faults)
  page.on('pageerror', (error) => faults.push(error.message))
  page.on('request', (request) => {
    const method = request.method()
    if (method === 'GET' || method === 'HEAD' || method === 'OPTIONS') return
    const url = new URL(request.url())
    if (!url.pathname.startsWith('/api/')) return
    mutations.get(page)!.push({ method, path: url.pathname, search: url.search })
  })
  await page.route('**/api/**', route => {
    const request = route.request()
    const pathname = new URL(request.url()).pathname
    const readOnly = ['GET', 'HEAD', 'OPTIONS'].includes(request.method())
    const toolsProbe = request.method() === 'POST' && pathname === '/api/tools/check'
    return readOnly || toolsProbe ? route.continue() : route.abort('blockedbyclient')
  })
  await context.addCookies(JSON.parse(readFileSync(cookiePath, 'utf8')))
  const identity = await context.request.get('/api/me')
  expect(identity.status()).toBe(200)
  const me = await identity.json() as { user?: { username?: string; role?: string } }
  expect(me.user?.username).toBe('workflow.qa')
  // Add-provider controls are functional only for the admin fixture; a member
  // gets an inert section and this harness must fail with that prerequisite.
  expect(me.user?.role, 'Settings write controls require the isolated admin fixture').toBe('admin')
  page.on('dialog', (dialog) => { void dialog.dismiss() })
})

test.afterEach(async ({ page }, testInfo) => {
  await testInfo.attach('settings-controls.json', { body: JSON.stringify(observations.get(page) ?? [], null, 2), contentType: 'application/json' })
  await testInfo.attach('settings-mutations.json', { body: JSON.stringify(mutations.get(page) ?? [], null, 2), contentType: 'application/json' })
  const faults = runtimeFaults.get(page) ?? []
  await testInfo.attach('runtime-errors.json', { body: JSON.stringify(faults), contentType: 'application/json' })
  expect(faults).toEqual([])
})

test('settings frame: genuine section values, desktop/mobile navigation and read-only agents/accounts', async ({ page }, testInfo) => {
  const mobile = isMobile(testInfo)
  const [providers, platforms, agents, tools] = await Promise.all([
    getJson<Provider[]>(page, '/api/ai/providers'),
    getJson<{ id: string; label: string; state: string; login: { saved: boolean; refreshError?: unknown }; usage: unknown }[]>(page, '/api/platforms'),
    getJson<{ artist: { draw: { provider: string; model: string } } }>(page, '/api/ai/agents'),
    getJson<ToolsStatus>(page, '/api/tools'),
  ])

  await boot(page, '/#/settings?section=tools')
  await expectTitle(page, mobile, 'Công cụ trên máy')
  await capture(page, testInfo, 'settings-frame-tools-entry')
  record(page, testInfo, 'frame.tools', 'Native tools section opens with its desktop/mobile title', (await page.locator(mobile ? '.vv-mset__title' : '.vv-set__title').textContent()) ?? '')

  if (mobile) {
    await page.getByRole('button', { name: 'Quay lại' }).click()
    await expect(page).toHaveURL(/#\/settings$/)
    const list = page.locator('.vv-mset__list')
    await expect(list).toBeVisible()
    // Values in the mobile list come from the same live resources, not static copy.
    const connectRow = list.locator('a[href="#/settings?section=connect"]')
    await expect(connectRow).toContainText(`${providers.length} nhà cung cấp`)
    const accountsRow = list.locator('a[href="#/settings?section=accounts"]')
    await expect(accountsRow).toContainText(`${platforms.length} nền tảng`)
    const artistProvider = providers.find((provider) => provider.id === agents.artist.draw.provider)
    const agentsRow = list.locator('a[href="#/settings?section=agents"]')
    if (agents.artist.draw.provider) await expect(agentsRow).toContainText(`Vẽ bằng ${artistProvider?.label ?? agents.artist.draw.provider}`)
    else await expect(agentsRow).toContainText('Chưa chọn nguồn vẽ')
    const updateCount = tools.tools.filter((tool) => tool.state === 'update').length
    const installingCount = tools.tools.filter((tool) => tool.state === 'installing').length
    const toolsRow = list.locator('a[href="#/settings?section=tools"]')
    if (installingCount > 0) await expect(toolsRow).toContainText(`Đang cài ${installingCount} công cụ`)
    else if (updateCount > 0) await expect(toolsRow).toContainText(`${updateCount} bản cập nhật`)
    else await expect(toolsRow).toContainText('Mọi công cụ đã cài')
    // MSettings has no logs child screen row: "Báo lỗi" is the native export
    // action (a download), so it is asserted but never activated.
    await expect(list.getByText('Xuất log', { exact: true })).toBeVisible()
    await capture(page, testInfo, 'settings-frame-mobile-list')
    record(page, testInfo, 'frame.mobile-list', 'List values match live providers/platforms/agents/tools resources', `${providers.length} providers, ${platforms.length} platforms`)

    for (const section of ['connect', 'agents', 'accounts', 'tools', 'storage'] as const) {
      if (section !== 'connect') {
        await page.getByRole('button', { name: 'Quay lại' }).click()
        await expect(page).toHaveURL(/#\/settings$/)
      }
      const row = page.locator(`a[href="#/settings?section=${section}"]`)
      await row.click()
      await expect(page).toHaveURL(new RegExp(`section=${section}`))
      await expectTitle(page, true, SECTION_TITLE[section])
      if (section === 'storage') await expect(page.locator('.vv-sto__card').first()).toBeVisible()
      await capture(page, testInfo, `settings-frame-${section}`)
      record(page, testInfo, `frame.${section}`, `Mobile list row opens ${SECTION_TITLE[section]}`, (await page.locator('.vv-mset__title').textContent()) ?? '')
    }
    // The router still accepts the logs child screen; MSettings simply has no
    // row for it. Exercise the deep link without pretending a missing row exists.
    await boot(page, '/#/settings?section=logs')
    await expectTitle(page, true, 'Báo lỗi')
    await expect(page.locator('.vv-logs__export')).toBeVisible()
    await capture(page, testInfo, 'settings-frame-logs-deeplink')
    record(page, testInfo, 'frame.logs', 'Mobile logs deep link renders; list row is the native export action, not a section row', 'logs screen + export card visible')
  } else {
    const nav = page.locator('aside.vv-set__side')
    await expect(nav).toBeVisible()
    for (const section of ['connect', 'agents', 'accounts', 'tools', 'storage', 'logs'] as const) {
      const label = SECTION_TITLE[section]
      await nav.getByRole('button', { name: new RegExp(`^${label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`) }).click()
      await expect(page).toHaveURL(new RegExp(`section=${section}`))
      await expectTitle(page, false, label)
      if (section === 'storage') await expect(page.locator('.vv-sto__card').first()).toBeVisible()
      if (section === 'logs') await expect(page.locator('.vv-logs__export')).toBeVisible()
      await capture(page, testInfo, `settings-frame-${section}`)
      record(page, testInfo, `frame.${section}`, `Desktop nav opens ${label}`, (await page.locator('.vv-set__title').textContent()) ?? '')
    }
  }

  // Agents is read-only for this smoke: assert the live director/artist labels
  // and selected model ids are rendered without opening a picker (pick = PUT).
  if (mobile) {
    await page.getByRole('button', { name: 'Quay lại' }).click()
    await page.locator('a[href="#/settings?section=agents"]').click()
    await expectTitle(page, true, 'Trợ lý AI')
  } else {
    await page.locator('aside.vv-set__side').getByRole('button', { name: /^Trợ lý AI/ }).click()
    await expectTitle(page, false, 'Trợ lý AI')
  }
  await expect(page.getByText('Đạo diễn', { exact: true }).first()).toBeVisible()
  await expect(page.getByText(/^Hoạ sĩ/).first()).toBeVisible()
  await expect(page.getByText('Trinh sát', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('Kiểm định', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('gpt-6.1-sol', { exact: true }).first()).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText('gpt-image-2', { exact: true }).first()).toBeVisible({ timeout: 15_000 })
  const drawProvider = providers.find((provider) => provider.id === agents.artist.draw.provider)
  if (drawProvider?.quota) {
    const quota = drawProvider.quota
    const group = page.getByRole('radiogroup', { name: mobile ? 'Hoạ sĩ vẽ ảnh bằng' : 'Vẽ ảnh bằng' })
    await expect(group).toContainText(`${quota.used}/${quota.limit}`)
    await expect(group).toContainText(quota.period === 'week' ? 'tuần này' : 'hôm nay')
    if (!mobile) {
      const bar = group.locator('.agt-drawcard__bar').first()
      const barBox = await bar.boundingBox()
      const fillBox = await bar.locator('span').first().boundingBox()
      expect(fillBox!.width).toBeLessThanOrEqual(barBox!.width + 1)
    }
    record(page, testInfo, 'agents.quota', 'Draw-source quota mirrors provider.quota without inventing a plan', `${quota.used}/${quota.limit} ${quota.period}`)
  } else {
    testInfo.annotations.push({ type: 'state-gap', description: 'agents.quota: draw provider has no provider.quota in genuine state; agents quota row is not renderable without a seeded/logged-in subscription' })
  }
  await capture(page, testInfo, 'settings-agents-readonly')
  record(page, testInfo, 'agents.readonly', 'Live agent names and selected model ids render without a picker mutation', 'director/artist/scout/qa + gpt-6.1-sol/gpt-image-2 visible')

  // Accounts is a read-only render of /api/platforms. Login buttons are present
  // but deliberately not activated (QR start is a server POST).
  if (mobile) {
    await page.getByRole('button', { name: 'Quay lại' }).click()
    await page.locator('a[href="#/settings?section=accounts"]').click()
    await expectTitle(page, true, 'Tài khoản nền tảng')
  } else {
    await page.locator('aside.vv-set__side').getByRole('button', { name: /^Tài khoản nền tảng/ }).click()
    await expectTitle(page, false, 'Tài khoản nền tảng')
  }
  for (const platform of platforms) {
    const card = page.getByRole('region', { name: platform.label })
    await expect(card).toBeVisible()
    const expectedState = platform.state === 'paused'
      ? 'Tạm chặn'
      : platform.login.saved && platform.login.refreshError
        ? mobile ? 'Hết hạn' : 'Cần đăng nhập lại'
        : 'Sẵn sàng'
    await expect(card).toContainText(expectedState)
  }
  await capture(page, testInfo, 'settings-frame-accounts-readonly')
  record(page, testInfo, 'accounts.readonly', 'Platform cards render genuine status and login controls without activation', `${platforms.length} platform cards, states asserted`)
  assertNoMutations(page)
})

test('connect: truthful provider cards, all safe add radios, scrim/Esc and conditional model catalog', async ({ page }, testInfo) => {
  const mobile = isMobile(testInfo)
  const providers = await getJson<Provider[]>(page, '/api/ai/providers')
  const gaps: Gap[] = []
  await boot(page, '/#/settings?section=connect')
  await expectTitle(page, mobile, 'Kết nối AI')

  if (providers.length === 0) {
    await expect(page.getByText('Chưa có nhà cung cấp nào', { exact: false })).toBeVisible()
  } else {
    await expect(page.locator('.conn-card')).toHaveCount(providers.length)
  }
  record(page, testInfo, 'connect.list', 'Provider list matches GET /api/ai/providers', `${providers.length} genuine providers`)

  for (const provider of providers) {
    const card = page.getByRole('region', { name: provider.label })
    await expect(card).toBeVisible()
    const sub = card.locator('.conn-id .conn-sub')
    const planText = provider.plan ? provider.plan.charAt(0).toUpperCase() + provider.plan.slice(1) : ''
    const expected = provider.status === 'logged_out'
      ? 'Chưa đăng nhập'
      : provider.status === 'login_pending'
        ? mobile ? (planText ? `Gói ${planText}` : 'Đã đăng nhập') : 'Đang đăng nhập'
        : provider.status === 'auth_failed'
          ? provider.auth === 'api_key' ? 'chưa lấy được model' : 'Phiên đăng nhập đã hết hạn'
          : provider.status === 'unreachable'
            ? 'Không kết nối được'
            : provider.auth === 'subscription'
              ? mobile ? (planText ? `Gói ${planText}` : 'Đã đăng nhập') : 'Đã đăng nhập'
              : provider.auth === 'api_key'
                ? mobile ? 'Khoá' : 'Khoá API'
                : (provider.baseUrl ?? '').replace(/^https?:\/\//, '').replace(/\/.*$/, '')
    await expect(sub).toContainText(expected)
    record(page, testInfo, `connect.card.${provider.id}`, `Truthful ${provider.status}/${provider.auth} subline`, (await sub.textContent()) ?? '')

    if (provider.status === 'logged_out') {
      const loginName = provider.kind === 'grok' ? 'Đăng nhập Grok' : 'Đăng nhập ChatGPT'
      await expect(card.getByRole('button', { name: loginName })).toBeVisible()
      // ProviderCard hides Model while logged out; asserting absence documents
      // why conn=models/manual are unreachable without a fake login/add.
      await expect(card.getByRole('button', { name: /^(Model · \d+|Chọn model)$/ })).toHaveCount(0)
    }

    if (provider.status === 'auth_failed' || provider.status === 'unreachable') {
      const error = card.locator('.conn-err')
      await expect(error).toBeVisible()
      await expect(error).toContainText('Không lấy được danh sách model.')
      await expect(error.getByRole('button', { name: 'Thử lại' })).toBeVisible()
      await expect(error.getByRole('button', { name: 'Thêm theo tên' })).toBeVisible()
      const secondaryLabel = provider.auth === 'api_key' ? 'Đổi khoá' : provider.auth === 'local' ? 'Đổi địa chỉ' : 'Đăng xuất'
      await expect(error.getByRole('button', { name: secondaryLabel })).toBeVisible()
      record(page, testInfo, `connect.failed.${provider.id}`, 'Orange failed card exposes retry/change/manual without activation', (await error.textContent()) ?? '')
    }

    if (provider.quota) {
      const quota = card.locator(mobile ? '.conn-m__quota' : '.conn-quota')
      await expect(quota).toBeVisible()
      await expect(quota).toContainText(new RegExp(`Ảnh (hôm nay|tuần này) ${provider.quota.used}/${provider.quota.limit}`))
      const bar = quota.locator(mobile ? '.conn-m__bar' : '.conn-bar')
      const barBox = await bar.boundingBox()
      const fillBox = await bar.locator('span').first().boundingBox()
      expect(fillBox!.width).toBeLessThanOrEqual(barBox!.width + 1)
      record(page, testInfo, `connect.quota.${provider.id}`, 'Quota line and bar are truthful and contained', (await quota.textContent()) ?? '')
    }
  }

  if (!providers.some((provider) => provider.quota)) {
    gaps.push({
      id: 'conn.quota',
      reason: 'No genuine provider carries quota (subscriptions are logged_out and quota is optional on the wire)',
      needed: 'Seeded/logged-in subscription with provider.quota, or explicit approval for a login handshake',
    })
  }
  if (!providers.some((provider) => provider.status === 'auth_failed' || provider.status === 'unreachable')) {
    gaps.push({
      id: 'conn.failed',
      reason: 'No genuine provider is auth_failed/unreachable; the orange failed card is unreachable without creating a provider',
      needed: 'Isolated seeded provider with status auth_failed or unreachable (POST add + bad credential is forbidden)',
    })
  }
  gaps.push({
    id: 'conn.added',
    reason: 'conn=added is local state set only by a successful POST /api/ai/providers in this session; no seeded card can produce it and faking is forbidden',
    needed: 'Explicit isolated new-provider record with proven cleanup, or accepted report-only status for this state',
  })

  // ----- Add panel / sheet: every radio and every safe local field state. -----
  const addName = mobile ? 'Thêm nhà cung cấp' : /^Thêm nhà cung cấp —/
  const opener = page.getByRole('button', { name: addName }).first()
  const addPanel = mobile ? page.getByRole('dialog', { name: 'Thêm nhà cung cấp' }) : page.locator('.conn-add')
  async function openAdd() {
    await expect(addPanel).toHaveCount(0)
    await opener.click()
    await expect(addPanel).toHaveCount(1)
    if (mobile) await withinViewport(page, page.locator('.m-sheet--md').last())
  }
  async function closeAdd() {
    if (mobile) {
      await page.locator('.m-overlay .m-scrim').last().click({ position: { x: 10, y: 10 } })
      await expect(page.locator('.m-overlay')).toHaveCount(0)
    } else {
      await page.locator('.conn-add').getByRole('button', { name: 'Đóng' }).click()
      await expect(page.locator('.conn-add')).toHaveCount(0)
    }
    await expect.poll(() => page.evaluate(() => document.body.style.overflow)).toBe('')
  }

  await openAdd()
  const group = page.getByRole('radiogroup', { name: 'Thêm nhà cung cấp' })
  await expect(group.getByRole('radio')).toHaveCount(8)
  record(page, testInfo, 'connect.add.group', 'Eight add-provider radio cards in native order', (await group.getByRole('radio').allTextContents()).map((text) => text.replace(/\s+/g, ' ').trim()).join(' | '))

  for (const kind of ADD_KINDS) {
    const tile = group.getByRole('radio', { name: new RegExp(`^${kind.label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`) })
    await expect(tile).toBeVisible()
    const tileText = (await tile.innerText()).replace(/\s+/g, ' ')
    const taken = tileText.includes('Đã kết nối')
    if (taken) {
      const takenProvider = providers.find((provider) => provider.kind === kind.kind)
      await tile.click()
      await expect(addPanel).toHaveCount(0)
      if (takenProvider?.auth === 'api_key') {
        const target = page.getByRole('region', { name: takenProvider.label })
        await expect(target.locator('.conn-edit')).toBeVisible()
        await target.locator('.conn-edit').getByRole('button', { name: 'Huỷ' }).click()
        await expect(target.locator('.conn-edit')).toHaveCount(0)
      }
      record(page, testInfo, `connect.add.${kind.kind}`, 'Existing kind reveals its card and never creates a duplicate', tileText)
      await openAdd()
      continue
    }

    await tile.click()
    await expect(tile).toHaveAttribute('aria-checked', 'true')
    await expect(group.locator('[role="radio"][aria-checked="true"]')).toHaveCount(1)
    const keyField = page.getByLabel(`Khoá API ${kind.label}`, { exact: true })
    const urlField = page.getByLabel(`Địa chỉ ${kind.label}`, { exact: true })
    await expect(keyField).toHaveCount(kind.auth === 'url' ? 0 : 1)
    await expect(urlField).toHaveCount(kind.auth === 'key' ? 0 : 1)
    const submit = page.getByRole('button', { name: 'Kết nối và lấy model', exact: true })
    await expect(submit).toBeDisabled()
    if (kind.auth !== 'url') await keyField.fill('qa-not-submitted')
    if (kind.auth !== 'key') await urlField.fill('http://127.0.0.1:11434/v1')
    await expect(submit).toBeEnabled()
    if (kind.auth !== 'url') await keyField.fill('')
    if (kind.auth !== 'key') await urlField.fill('')
    await expect(submit).toBeDisabled()
    record(page, testInfo, `connect.add.${kind.kind}`, 'Radio selects, required fields appear, submit enables only with input and was never clicked', `${tileText} [${kind.auth}]`)

    if (kind.kind === 'openai_compat') {
      await capture(page, testInfo, 'settings-connect-add-both-fields')
    }
  }

  // No add-provider field was submitted: no provider/model/agent mutation.
  assertNoMutations(page)

  if (mobile) {
    // Sheet scrim, focus restoration and body scroll lock.
    await page.locator('.m-overlay .m-scrim').last().click({ position: { x: 10, y: 10 } })
    await expect(page.locator('.m-overlay')).toHaveCount(0)
    await expect.poll(() => page.evaluate(() => document.body.style.overflow)).toBe('')
    await expect(opener).toBeFocused()
    await openAdd()
    await page.keyboard.press('Escape')
    await expect(page.locator('.m-overlay')).toHaveCount(0)
    await expect.poll(() => page.evaluate(() => document.body.style.overflow)).toBe('')
    await expect(opener).toBeFocused()
    record(page, testInfo, 'connect.add.overlay', 'Scrim and Escape close the sheet, restore focus and body overflow', 'both dismissal paths passed')
  } else {
    await closeAdd()
    record(page, testInfo, 'connect.add.overlay', 'Desktop inline panel closes via its native Đóng control', 'closed; no dialog layer exists on desktop')
    assertNoMutations(page)
  }

  // ----- Model catalog: genuine control only. No route interception. -----
  const directProvider = providers.find((provider) => !['logged_out', 'login_pending', 'auth_failed', 'unreachable'].includes(provider.status))
  const failedProvider = providers.find((provider) => provider.status === 'auth_failed' || provider.status === 'unreachable')
  const catalogProvider = directProvider ?? failedProvider
  if (!catalogProvider) {
    gaps.push({
      id: 'conn.models',
      reason: 'Every genuine provider is logged_out/login_pending, so ProviderCard exposes neither the Model control nor the failed-card manual entry (models and manual panels are unreachable)',
      needed: 'Genuinely ready provider, or seeded auth_failed/unreachable provider, exposing GET models; no fake list/login',
    })
    gaps.push({
      id: 'conn.manual',
      reason: 'The manual “Thêm theo tên” panel lives inside ModelCatalog, which is gated by the same hidden Model/failed control',
      needed: 'Same as conn.models; manual check/cancel must remain non-submitting',
    })
    await capture(page, testInfo, 'settings-connect-logged-out')
    await attachGaps(page, testInfo, gaps)
    assertNoMutations(page)
    return
  }

  const card = page.getByRole('region', { name: catalogProvider.label })
  const catalogOpener = directProvider
    ? card.getByRole('button', { name: /^(Model · \d+|Chọn model)$/ })
    : card.locator('.conn-err').getByRole('button', { name: 'Thêm theo tên' })
  await expect(catalogOpener).toBeVisible()
  record(page, testInfo, 'connect.models.entry', directProvider ? 'Direct Model control opens the genuine catalog' : 'Failed-card “Thêm theo tên” opens the genuine catalog without retry', directProvider ? 'header Model control' : 'failed error-box manual entry')

  if (mobile) {
    const modelsPath = `/api/ai/providers/${catalogProvider.id}/models`
    const responsePromise = waitApi(page, (url) => url.pathname === modelsPath)
    await catalogOpener.click()
    const response = await responsePromise
    const catalogOk = response.status() === 200
    const pageData = catalogOk ? await response.json() as ModelPage : undefined
    const dialog = page.getByRole('dialog', { name: `Model của ${catalogProvider.label}` })
    await expect(dialog).toBeVisible()
    await withinViewport(page, dialog)
    if (!pageData) {
      await expect(dialog.getByText('Không tải được danh mục model của nhà cung cấp này.')).toBeVisible()
      record(page, testInfo, 'connect.models.sheet', 'Genuine catalog API failure is shown truthfully in the sheet', `HTTP ${response.status()}`)
    } else {
      if (pageData.models.length > 0) {
        await expect.poll(() => dialog.getByRole('checkbox').count()).toBeGreaterThan(0)
      } else {
        await expect(dialog.getByText('Không có model nào khớp.')).toBeVisible()
      }
      record(page, testInfo, 'connect.models.sheet', 'Genuine models sheet renders rows and obeys MSettings (no filter/search/manual)', `${pageData.models.length}/${pageData.total} models in first page`)
    }
    // MSettings intentionally has no filter/search/manual controls in the sheet.
    await expect(dialog.locator('.conn-cat__tools')).toHaveCount(0)
    await expect(dialog.getByRole('button', { name: 'Không thấy model? Thêm theo tên' })).toHaveCount(0)
    await capture(page, testInfo, 'settings-connect-models-sheet')
    await page.locator('.m-overlay .m-scrim').last().click({ position: { x: 10, y: 10 } })
    await expect(page.locator('.m-overlay')).toHaveCount(0)
    await expect.poll(() => page.evaluate(() => document.body.style.overflow)).toBe('')
    await catalogOpener.click()
    await expect(page.getByRole('dialog', { name: `Model của ${catalogProvider.label}` })).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.locator('.m-overlay')).toHaveCount(0)
    await expect.poll(() => page.evaluate(() => document.body.style.overflow)).toBe('')
    record(page, testInfo, 'connect.models.overlay', 'Models sheet scrim and Escape dismiss with focus/scroll restoration', 'both paths passed')
    assertNoMutations(page)
    await attachGaps(page, testInfo, gaps)
    return
  }

  const modelsPath = `/api/ai/providers/${catalogProvider.id}/models`
  const initialResponse = waitApi(page, (url) => url.pathname === modelsPath)
  await catalogOpener.click()
  const first = await initialResponse
  const catalog = card.locator('.conn-cat')
  await expect(catalog).toBeVisible()
  const firstPage = first.status() === 200 ? await first.json() as ModelPage : undefined
  if (!firstPage) {
    await expect(catalog.getByText('Không tải được danh mục model của nhà cung cấp này.')).toBeVisible()
    record(page, testInfo, 'connect.models.catalog', 'Genuine catalog API failure is shown truthfully', `HTTP ${first.status()}`)
  } else {
    const rows = catalog.locator('.conn-row')
    if (firstPage.models.length > 0) await expect.poll(() => rows.count()).toBeGreaterThan(0)
    else await expect(catalog.getByText('Không có model nào khớp.')).toBeVisible()
    await capture(page, testInfo, 'settings-connect-models-catalog')
    record(page, testInfo, 'connect.models.catalog', 'Inline catalog renders genuine first page', `${firstPage.models.length}/${firstPage.total} models`)

    const filterGroup = catalog.getByRole('radiogroup', { name: 'Lọc model' })
    await expect(filterGroup.getByRole('radio', { name: 'Tất cả' })).toHaveAttribute('aria-checked', 'true')

    async function pickFilter(name: string, expectedQuery: (url: URL) => boolean): Promise<ModelPage> {
      const responsePromise = waitApi(page, (url) => url.pathname === modelsPath && expectedQuery(url))
      await filterGroup.getByRole('radio', { name }).click()
      const response = await responsePromise
      expect(response.status()).toBe(200)
      return response.json() as Promise<ModelPage>
    }

    const recommended = await pickFilter('Gợi ý', (url) => url.searchParams.get('filter') === 'recommended')
    if (recommended.models.length === 0 && recommended.total > 0) {
      // ModelCatalog falls back to Tất cả once when the recommended set is empty.
      await expect(filterGroup.getByRole('radio', { name: 'Tất cả' })).toHaveAttribute('aria-checked', 'true')
    } else {
      await expect(filterGroup.getByRole('radio', { name: 'Gợi ý' })).toHaveAttribute('aria-checked', 'true')
    }
    const selectedCount = catalogProvider.selected.length
    await pickFilter(`Đã chọn · ${selectedCount}`, (url) => url.searchParams.get('filter') === 'selected')
    await expect(filterGroup.getByRole('radio', { name: `Đã chọn · ${selectedCount}` })).toHaveAttribute('aria-checked', 'true')
    await pickFilter('Tất cả', (url) => !url.searchParams.has('filter'))
    await expect(filterGroup.getByRole('radio', { name: 'Tất cả' })).toHaveAttribute('aria-checked', 'true')
    record(page, testInfo, 'connect.models.filters', 'Selected/Recommended/All issue the matching API filter and stay selected', 'filter=selected, filter=recommended, no filter')

    if (firstPage.models.length > 0) {
      const query = firstPage.models[0].id.slice(0, 4)
      const search = catalog.getByRole('searchbox')
      const searchResponsePromise = waitApi(page, (url) => url.pathname === modelsPath && url.searchParams.get('q') === query)
      await search.fill(query)
      const searchResponse = await searchResponsePromise
      expect(searchResponse.status()).toBe(200)
      const searchPage = await searchResponse.json() as ModelPage
      await expect(catalog.locator('.conn-cat__foot')).toContainText('đang hiện')
      const clearPromise = waitApi(page, (url) => url.pathname === modelsPath && !url.searchParams.has('q'))
      await search.fill('')
      const clearResponse = await clearPromise
      expect(clearResponse.status()).toBe(200)
      record(page, testInfo, 'connect.models.search', 'Debounced search issues q= and clearing restores the unfiltered page', `q=${query} → ${searchPage.total} total`)
      await capture(page, testInfo, 'settings-connect-models-search-cleared')
    } else {
      gaps.push({ id: 'conn.models.search', reason: 'Genuine catalog first page is empty, so no model id exists to search', needed: 'Catalog with at least one model' })
    }
  }

  const manualOpen = catalog.getByRole('button', { name: 'Không thấy model? Thêm theo tên' })
  await manualOpen.click()
  const manual = catalog.locator('.conn-manual')
  await expect(manual).toBeVisible()
  await manual.getByLabel('Tên model').fill('qa-manual-never-submitted')
  await manual.getByRole('button', { name: 'Đóng' }).click()
  await expect(manual).toHaveCount(0)
  expect(appMutations(page).filter((mutation) => mutation.path.endsWith('/models/check'))).toEqual([])
  record(page, testInfo, 'connect.models.manual', 'Manual panel opens, accepts a name, cancels without POST models/check', firstPage ? 'closed with no mutation' : 'closed with no mutation despite catalog API failure')
  assertNoMutations(page)

  await attachGaps(page, testInfo, gaps)
})

test('tools: genuine read-only rows, status pills, details and no install mutation', async ({ page }, testInfo) => {
  const mobile = isMobile(testInfo)
  const status = await getJson<ToolsStatus>(page, '/api/tools')
  await boot(page, '/#/settings?section=tools')
  await expectTitle(page, mobile, 'Công cụ trên máy')
  await expect(page.locator('.vv-tools__list')).toBeVisible()
  await expect(page.locator('.vv-tools__row')).toHaveCount(status.tools.length)

  const failed = status.tools.filter((tool) => tool.state === 'failed').length
  const installing = status.tools.filter((tool) => tool.state === 'installing').length
  const updates = status.tools.filter((tool) => tool.state === 'update').length
  const missing = status.tools.filter((tool) => tool.state === 'missing').length
  const summary = page.locator('.vv-tools__sum')
  await expect(summary).toBeVisible()
  const expectedSummary = failed > 0 ? `${failed} công cụ cần sửa` : installing > 0 ? /Đang cài/ : updates > 0 ? new RegExp(`Máy sẵn sàng · có ${updates} bản cập nhật`) : missing > 0 ? `${missing} công cụ chưa cài` : 'Máy sẵn sàng'
  await expect(summary.locator('.vv-tools__sum-title')).toHaveText(expectedSummary)
  record(page, testInfo, 'tools.summary', 'Summary severity derives from genuine GET /api/tools', (await summary.locator('.vv-tools__sum-title').textContent()) ?? '')

  const runningIndex = status.tools.findIndex((tool) => tool.state === 'installing')
  for (const [index, tool] of status.tools.entries()) {
    const name = TOOL_NAMES[tool.id] ?? tool.id
    const row = page.locator('.vv-tools__row').filter({ hasText: name }).first()
    // A missing tool after the running install is shown as queued, not installable.
    const queued = runningIndex >= 0 && index > runningIndex && tool.state === 'missing'
    const expectedPill = queued ? 'Chờ' : (PILLS[tool.state] ?? tool.state)
    await expect(row).toBeVisible()
    await expect(row.locator('.vv-tools__pill')).toHaveText(expectedPill)
    if (queued) {
      await expect(row.getByRole('button', { name: /^(Cài|Cập nhật|Tải tiếp)$/ })).toHaveCount(0)
    } else if (tool.state === 'update') {
      await expect(row.getByRole('button', { name: 'Cập nhật' })).toBeVisible()
    } else if (tool.state === 'missing') {
      await expect(row.getByRole('button', { name: 'Cài' })).toBeVisible()
    } else if (tool.state === 'failed') {
      await expect(row.getByRole('button', { name: 'Tải tiếp' })).toBeVisible()
    } else {
      // Ready/needs_login rows intentionally have no install/update action.
      await expect(row.getByRole('button', { name: /^(Cài|Cập nhật|Tải tiếp)$/ })).toHaveCount(0)
    }
    record(page, testInfo, `tools.row.${tool.id}`, `Truthful pill/action for ${queued ? 'queued' : tool.state}`, `${expectedPill}${tool.version ? ` · ${tool.version}` : ''}`)
  }

  const details = page.locator('details.vv-tools__adv')
  await expect(details).toBeVisible()
  await details.locator('summary').click()
  await expect(details).toHaveJSProperty('open', true)
  if (status.dataDir) await expect(details).toContainText(status.dataDir)
  await expect(details).toContainText('yt-dlp')
  await capture(page, testInfo, 'settings-tools-details')
  record(page, testInfo, 'tools.details', 'Advanced data-dir/version details expand natively', 'details open with tech names/dataDir')

  // Install/update buttons are asserted but never clicked.
  expect(appMutations(page).filter((mutation) => /\/api\/tools\/[^/]+\/(install|cancel)$/.test(mutation.path))).toEqual([])
  assertNoMutations(page)
})

test('tools recheck: only the safe POST /api/tools/check probe, never install', async ({ page }, testInfo) => {
  test.setTimeout(150_000)
  const mobile = isMobile(testInfo)
  const before = await getJson<ToolsStatus>(page, '/api/tools')
  expect(before.tools.some((tool) => tool.state === 'installing')).toBe(false)
  await boot(page, '/#/settings?section=tools')
  await expectTitle(page, mobile, 'Công cụ trên máy')
  await expect(page.locator('.vv-tools__row')).toHaveCount(before.tools.length)
  const recheck = page.getByRole('button', { name: 'Kiểm tra lại' })
  // The control exists in both desktop and mobile implementations; if a future
  // MSettings parity change removes it, this assertion is the parity failure.
  await expect(recheck).toBeVisible()

  const responsePromise = page.waitForResponse((response) => {
    const url = new URL(response.url())
    return response.request().method() === 'POST' && url.pathname === '/api/tools/check'
  }, { timeout: 120_000 })
  await recheck.click()
  const response = await responsePromise
  expect(response.status()).toBe(200)
  const after = await response.json() as ToolsStatus
  expect(after.tools).toHaveLength(before.tools.length)
  expect(after.checkedAt).toBeTruthy()
  for (const tool of after.tools) {
    const name = TOOL_NAMES[tool.id] ?? tool.id
    await expect(page.locator('.vv-tools__row').filter({ hasText: name }).first().locator('.vv-tools__pill')).toHaveText(PILLS[tool.state] ?? tool.state)
  }
  const recent = await getJson<ToolsStatus>(page, '/api/tools')
  expect(recent.tools.map((tool) => tool.state)).toEqual(after.tools.map((tool) => tool.state))
  record(page, testInfo, 'tools.recheck', 'POST /api/tools/check recomputes the snapshot with no install/cancel call', `${before.checkedAt || 'cold'} → ${after.checkedAt}; ${after.tools.map((tool) => `${tool.id}:${tool.state}`).join(', ')}`)

  const calls = appMutations(page)
  expect(calls.filter((mutation) => mutation.path === '/api/tools/check' && mutation.method === 'POST')).toHaveLength(1)
  expect(calls.filter((mutation) => !(mutation.path === '/api/tools/check' && mutation.method === 'POST'))).toEqual([])
  expect(calls.filter((mutation) => /\/(install|cancel)$/.test(mutation.path))).toEqual([])
  await testInfo.attach('tools-recheck-receipt.json', { body: JSON.stringify({ before, after, calls }, null, 2), contentType: 'application/json' })
  await capture(page, testInfo, 'settings-tools-rechecked')
})
