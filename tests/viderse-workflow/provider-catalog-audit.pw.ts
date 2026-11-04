import { test, expect, type Locator, type Page, type TestInfo } from '@playwright/test'
import { readFileSync } from 'node:fs'
import type { AiProvider } from '../../apps/viderse/web/src/api/ai'

// Real installed Ollama, isolated HOME/models directory, no models/downloads or
// inference. A refused local port exercises the actual unreachable-provider path.
// A full document reload proves the owned record/status/catalog persisted via GET
// and UI; the inline address edit must prefill the saved URL and never PATCH
// before submit. No mocked application API, provider catalog, paid receipt or
// CSS state.
const cookieFile = process.env.WORKFLOW_QA_COOKIE_FILE
const fixtureFile = process.env.WORKFLOW_QA_FIXTURE
if (!cookieFile || !fixtureFile) throw new Error('Isolated workflow QA cookies and fixture required')
const fixture = JSON.parse(readFileSync(fixtureFile, 'utf8')) as { jobs: { id: string }[] }
const readyURL = process.env.WORKFLOW_QA_OLLAMA_URL
const refusedURL = process.env.WORKFLOW_QA_REFUSED_PROVIDER_URL
const events = new WeakMap<Page, { method: string; path: string; boot: boolean; allowed: boolean }[]>()
const booting = new WeakMap<Page, boolean>()

/** Wire record without credential material: never attach key hints or values
 * to evidence, and compare only persisted provider configuration. */
function providerRecord(provider: AiProvider) {
  return {
    id: provider.id,
    kind: provider.kind,
    auth: provider.auth,
    label: provider.label,
    baseUrl: provider.baseUrl ?? null,
    status: provider.status,
    plan: provider.plan ?? null,
    quota: provider.quota ?? null,
    catalogAt: provider.catalogAt ?? null,
    catalogCount: provider.catalogCount,
    selected: provider.selected,
    dailyPaidImageLimit: provider.dailyPaidImageLimit,
  }
}
async function addressEdit(card: Locator, savedURL: string) {
  const edit = card.locator('.conn-edit')
  await expect(edit).toBeVisible()
  const field = edit.getByLabel('Địa chỉ', { exact: true })
  await expect(field).toHaveValue(savedURL)
  await expect(edit.getByRole('button', { name: 'Lưu', exact: true })).toBeEnabled()
  return { edit, field }
}

async function providers(page: Page): Promise<AiProvider[]> {
  const response = await page.request.get('/api/ai/providers')
  expect(response.status()).toBe(200)
  return response.json()
}
async function snapshot(page: Page) {
  const response = await page.request.get(`/api/clone/${fixture.jobs[0].id}`)
  expect(response.status()).toBe(200)
  const job = await response.json()
  const agentsResponse = await page.request.get('/api/ai/agents')
  expect(agentsResponse.status()).toBe(200)
  return { job: { id: job.id, status: job.status, version: job.version, steps: job.steps }, agents: await agentsResponse.json() }
}
async function capture(page: Page, info: TestInfo, name: string) {
  await page.evaluate(async () => { await document.fonts.ready })
  const dimensions = await page.evaluate(() => ({ viewport: innerWidth, content: document.documentElement.scrollWidth }))
  expect(dimensions.content).toBeLessThanOrEqual(dimensions.viewport)
  await page.screenshot({ path: info.outputPath(`${name}.png`) })
}
async function boot(page: Page, reload = false) {
  booting.set(page, true)
  if (reload) await page.reload({ waitUntil: 'domcontentloaded' })
  else await page.goto('/#/settings?section=connect')
  await page.waitForFunction(() => location.hash === '#/setup' || Boolean(document.querySelector('.vv-set__title,.vv-mset__title')))
  if (new URL(page.url()).hash === '#/setup') {
    await page.getByRole('link', { name: 'Cài đặt nâng cao', exact: true }).click()
    await page.goto('/#/settings?section=connect')
  }
  await expect(page.getByText('Kết nối AI', { exact: true }).last()).toBeVisible()
  booting.set(page, false)
}

test.beforeEach(async ({ context, page }) => {
  test.skip(!readyURL || !refusedURL, 'Requires the real isolated Ollama catalog and a proven refused local port; never fake those states')
  for (const url of [readyURL!, refusedURL!]) {
    expect(new URL(url).hostname).toBe('127.0.0.1')
    expect(new URL(url).protocol).toBe('http:')
  }
  await context.addCookies(JSON.parse(readFileSync(cookieFile, 'utf8')))
  const identity = await context.request.get('/api/me')
  expect(identity.status()).toBe(200)
  expect((await identity.json()).user).toMatchObject({ username: 'workflow.qa', role: 'admin' })
  events.set(page, [])
  await page.route('**/api/**', route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    const method = request.method()
    if (['GET', 'HEAD', 'OPTIONS'].includes(method)) return route.continue()
    const payload = method === 'POST' && path === '/api/ai/providers' ? request.postDataJSON() : null
    const create = method === 'POST' && path === '/api/ai/providers' && payload?.kind === 'ollama' && [readyURL, refusedURL].includes(payload?.baseUrl)
    const remove = method === 'DELETE' && /^\/api\/ai\/providers\/ollama-[a-z0-9-]+$/.test(path)
    const allowed = create || remove
    events.get(page)!.push({ method, path, boot: booting.get(page) ?? false, allowed })
    return allowed ? route.continue() : route.abort('blockedbyclient')
  })
})

for (const scenario of ['ready', 'unreachable'] as const) {
  test(`real local catalog ${scenario}: add feedback, model surface and owned-record removal`, async ({ page }, info) => {
    const baseURL = scenario === 'ready' ? readyURL! : refusedURL!
    const beforeProviders = await providers(page)
    expect(beforeProviders.some(provider => provider.kind === 'ollama' && [readyURL, refusedURL].includes(provider.baseUrl))).toBe(false)
    const before = await snapshot(page)
    const beforeIDs = new Set(beforeProviders.map(provider => provider.id))
    const beforeRecords = beforeProviders.map(providerRecord).sort((a, b) => a.id.localeCompare(b.id))
    let owned: AiProvider | undefined
    let passed = false
    try {
      await boot(page)
      const mobile = page.viewportSize()!.width < 768
      await page.getByRole('button', { name: mobile ? 'Thêm nhà cung cấp' : /^Thêm nhà cung cấp —/ }).first().click()
      const form = mobile ? page.getByRole('dialog', { name: 'Thêm nhà cung cấp', exact: true }) : page.locator('.conn-add')
      await expect(form).toBeVisible()
      await form.getByRole('radio', { name: /^Ollama/ }).click()
      await form.getByLabel('Địa chỉ Ollama', { exact: true }).fill(baseURL)
      const responsePromise = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === '/api/ai/providers')
      await form.getByRole('button', { name: 'Kết nối và lấy model', exact: true }).click()
      const response = await responsePromise
      expect(response.status()).toBe(scenario === 'ready' ? 201 : 422)
      await expect.poll(async () => (await providers(page)).find(provider => provider.kind === 'ollama' && provider.baseUrl === baseURL)?.status).toBe(scenario)
      owned = (await providers(page)).find(provider => provider.kind === 'ollama' && provider.baseUrl === baseURL)
      expect(owned).toBeDefined()
      expect(beforeIDs.has(owned!.id)).toBe(false)
      if (scenario === 'unreachable') {
        const feedback = page.locator('[data-toast-id]').last().getByRole('alert')
        await expect(feedback).toBeVisible()
        // Inspect paint after its finite entrance/placement transitions, not a
        // screenshot from the initial blur frame or an arbitrary fixed sleep.
        await expect.poll(() => feedback.evaluate(element => {
          for (let node: Element | null = element; node; node = node.parentElement) {
            if (node.getAnimations().some(animation => Number.isFinite(animation.effect?.getTiming().iterations) && animation.playState !== 'finished' && animation.playState !== 'idle')) return false
          }
          return true
        })).toBe(true)
        const feedbackBox = await feedback.boundingBox()
        expect(feedbackBox).not.toBeNull()
        expect(feedbackBox!.y).toBeGreaterThanOrEqual(0)
        expect(feedbackBox!.y + feedbackBox!.height).toBeLessThanOrEqual(page.viewportSize()!.height)
        if (mobile) {
          const actionBox = await form.getByRole('button', { name: 'Kết nối và lấy model', exact: true }).boundingBox()
          expect(actionBox).not.toBeNull()
          const overlaps = feedbackBox!.x < actionBox!.x + actionBox!.width && feedbackBox!.x + feedbackBox!.width > actionBox!.x
            && feedbackBox!.y < actionBox!.y + actionBox!.height && feedbackBox!.y + feedbackBox!.height > actionBox!.y
          expect(overlaps).toBe(false)
          await info.attach('feedback-layout.json', { body: JSON.stringify({ feedback: feedbackBox, action: actionBox, overlaps }), contentType: 'application/json' })
        }
        await capture(page, info, 'real-refused-catalog-add-feedback')
        if (mobile) await page.keyboard.press('Escape')
        else await form.getByRole('button', { name: 'Đóng', exact: true }).click()
        await expect(form).toHaveCount(0)
      }
      const addedCard = page.getByRole('region', { name: 'Ollama', exact: true })
      await expect(addedCard).toBeVisible()
      if (scenario === 'ready') {
        await expect(addedCard).toContainText('Đã kết nối Ollama và lấy được 0 model.')
        expect(owned!.catalogCount).toBe(0)
        await capture(page, info, 'real-local-provider-added-empty-catalog')
        if (mobile) {
          await addedCard.getByRole('button', { name: 'Chọn model', exact: true }).click()
          const sheet = page.getByRole('dialog', { name: 'Model của Ollama', exact: true })
          await expect(sheet).toBeVisible()
          await expect(sheet).toContainText('Không có model nào khớp.')
          await capture(page, info, 'real-empty-catalog-phone-sheet')
          await page.keyboard.press('Escape')
          await expect(sheet).toHaveCount(0)
        } else {
          await expect(addedCard.getByRole('list', { name: 'Ollama', exact: true })).toBeVisible()
          await expect(addedCard).toContainText('Không có model nào khớp.')
        }
      } else {
        const error = addedCard.getByRole('alert')
        await expect(error).toContainText('Không lấy được danh sách model.')
        await expect(error).toContainText(baseURL)
        await expect(error.getByRole('button', { name: 'Đổi khoá', exact: true })).toHaveCount(0)
        await capture(page, info, 'real-unreachable-provider-card')
      }

      // Full document reload: the locally-added card state is gone and the
      // owned record must still come back from GET with the same id, status,
      // address and catalog count, rendered by the same real card.
      await boot(page, true)
      const reloaded = (await providers(page)).find(provider => provider.id === owned!.id)
      expect(reloaded).toBeDefined()
      expect(reloaded).toMatchObject({
        id: owned!.id,
        kind: 'ollama',
        auth: 'local',
        status: scenario,
        baseUrl: baseURL,
        catalogCount: owned!.catalogCount,
      })
      const card = page.getByRole('region', { name: 'Ollama', exact: true })
      await expect(card).toBeVisible()
      if (scenario === 'ready') {
        await expect(card.locator('.conn-id .conn-sub')).toContainText(new URL(baseURL).host)
        await expect(card.getByRole('button', { name: 'Chọn model', exact: true })).toBeVisible()
        await capture(page, info, 'real-local-provider-reloaded-persisted')
        const catalogResponse = await page.request.get(`/api/ai/providers/${owned!.id}/models?limit=200`)
        expect(catalogResponse.status()).toBe(200)
        const catalogPage = await catalogResponse.json() as { models: { id: string }[]; total: number }
        expect(catalogPage.models).toEqual([])
        expect(catalogPage.total).toBe(0)
        if (mobile) {
          await card.getByRole('button', { name: 'Chọn model', exact: true }).click()
          const sheet = page.getByRole('dialog', { name: 'Model của Ollama', exact: true })
          await expect(sheet).toBeVisible()
          await expect(sheet).toContainText('Không có model nào khớp.')
          await capture(page, info, 'real-local-provider-reloaded-empty-catalog-phone-sheet')
          await page.keyboard.press('Escape')
          await expect(sheet).toHaveCount(0)
          await expect(page.locator('.m-overlay')).toHaveCount(0)
        } else {
          await card.getByRole('button', { name: 'Chọn model', exact: true }).click()
          await expect(card.getByRole('list', { name: 'Ollama', exact: true })).toBeVisible()
          await expect(card).toContainText('Không có model nào khớp.')
          await capture(page, info, 'real-local-provider-reloaded-empty-catalog')
        }
      } else {
        await expect(card.locator('.conn-id .conn-sub')).toContainText(`Không kết nối được ${new URL(baseURL).host}`)
        await expect(card.getByRole('alert')).toContainText('Không lấy được danh sách model.')
        await expect(card.getByRole('alert')).toContainText(baseURL)
        await capture(page, info, 'real-unreachable-provider-reloaded-persisted')
      }

      // Failed (and ready) local records use the same inline address edit. Its
      // field starts at the saved base URL; an unsaved value changes neither
      // the form prefill requirement nor the server record until submitted.
      const changeAddress = scenario === 'unreachable'
        ? card.getByRole('alert').getByRole('button', { name: 'Đổi địa chỉ', exact: true })
        : card.getByRole('button', { name: 'Đổi địa chỉ', exact: true }).first()
      await changeAddress.click()
      const { edit, field } = await addressEdit(card, baseURL)
      await capture(page, info, `real-${scenario}-address-edit-prefilled`)
      const draftURL = `${baseURL.replace(/\/+$/, '')}/qa-unsaved`
      await field.fill(draftURL)
      await expect(field).toHaveValue(draftURL)
      const uncommitted = (await providers(page)).find(provider => provider.id === owned!.id)
      expect(uncommitted).toBeDefined()
      expect(uncommitted).toMatchObject({
        id: owned!.id,
        status: scenario,
        baseUrl: baseURL,
        catalogCount: owned!.catalogCount,
      })
      expect(events.get(page)!.filter(event => event.method === 'PATCH')).toEqual([])
      // Keep the native edit row for the owned DELETE: no save is pressed.
      await edit.getByRole('button', { name: 'Gỡ', exact: true }).click()
      const confirmation = page.getByRole('alertdialog')
      await expect(confirmation).toContainText('địa chỉ đã lưu')
      const removal = page.waitForResponse(response => response.request().method() === 'DELETE' && new URL(response.url()).pathname === `/api/ai/providers/${owned!.id}`)
      await confirmation.getByRole('button', { name: 'Gỡ', exact: true }).click()
      expect((await removal).ok()).toBe(true)
      await expect(card).toHaveCount(0)
      expect((await providers(page)).some(provider => provider.id === owned!.id)).toBe(false)
      await capture(page, info, 'real-owned-provider-removed')
      expect(events.get(page)!.filter(event => !event.allowed && !(event.boot && event.path === '/api/setup/start'))).toEqual([])
      passed = true
    } finally {
      const cleanup = (await providers(page)).filter(provider => !beforeIDs.has(provider.id) && provider.kind === 'ollama' && (provider.id === owned?.id || provider.baseUrl === baseURL))
      for (const provider of cleanup) {
        const response = await page.request.delete(`/api/ai/providers/${provider.id}`)
        expect(response.ok()).toBe(true)
      }
      const finalProviders = await providers(page)
      expect(finalProviders.map(providerRecord).sort((a, b) => a.id.localeCompare(b.id))).toEqual(beforeRecords)
      expect(await snapshot(page)).toEqual(before)
      await info.attach('real-local-provider-catalog-receipt.json', { body: JSON.stringify({
        scenario,
        baseURL,
        sourceRevision: process.env.WORKFLOW_SOURCE_REVISION ?? null,
        appBaseURL: info.project.use.baseURL ?? null,
        ownedRecord: owned ? providerRecord(owned) : null,
        originalProviderRecords: beforeRecords,
        finalProviderRecords: finalProviders.map(providerRecord).sort((a, b) => a.id.localeCompare(b.id)),
        passed,
        realInstalledOllama: scenario === 'ready',
        modelsDownloaded: 0,
        inferenceRequests: 0,
        sourceAndAgentConfigurationUnchanged: true,
        providerRecordsRestored: true,
        browserMutations: events.get(page),
      }), contentType: 'application/json' })
    }
  })
}
