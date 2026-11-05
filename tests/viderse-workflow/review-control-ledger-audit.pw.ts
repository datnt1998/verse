// V36 Review control ledger — read-only consumer audit plus opt-in A/B mutation acceptance.
//
// The normal review fixture is the source of truth. The default test never
// submits a paid redraw, approves/skips a job, changes usedVersion, uploads a
// file or mutates the original source / A-B replay checkpoints. Replay seeding
// and any destructive acceptance stay with the controller.
//
// The opt-in replay mutation group at the bottom is skipped unless the
// controller sets WORKFLOW_QA_REPLAY_MUTATIONS=1, WORKFLOW_QA_WORKER_STANDBY=1
// and two exact fixture A/B IDs. It fences every API write with route.abort and
// only targets those A/B replicas, never the original source job.
//
// Covered here beyond the already captured prompt/refs/cutout/batch/confirm
// states: version preview identity, image previous/next plus keyboard arrows,
// every real strip option, A/B sidebar video selection, real clipboard readback,
// version compare + V shortcut, diff visibility, local prompt change/undo,
// base-reference removal plus real-button cancel, and every reference candidate
// pressed state. Persisted actions remain BLOCKED here and point at the named
// 129 queue/cancel receipts; paid provider completion is never claimed.
import { test, expect, type Page, type TestInfo } from '@playwright/test'
import { readFileSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'

const repo = resolve(__dirname, '../..')
const cookiePath = process.env.WORKFLOW_QA_COOKIE_FILE
const fixturePath = process.env.WORKFLOW_QA_FIXTURE
if (!cookiePath || !fixturePath || realpathSync(cookiePath).startsWith(repo + sep)) {
  throw new Error('Private isolated QA session required')
}
type Fixture = { username: string; jobs: { id: string; projectId: string }[]; templates: { id: string }[] }
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as Fixture
if (fixture.username !== 'workflow.qa') throw new Error('Only workflow.qa is authorized')

type PromptPart = { kind: string; text: string; source: { label: string } }
type ApiVersion = {
  n: number
  state: 'drawing' | 'ready' | 'failed' | 'cancelled'
  url: string | null
  previewUrl: string | null
  width: number
  height: number
  parts: PromptPart[]
  sentText: string
  refs: { id: string; role: string; url: string; source: { type: string; label: string } }[]
  feedback: string | null
  diffFromPrev: [string, '' | 'ins' | 'del'][] | null
  partsLegacy?: boolean
  provider: string
}
type ApiImage = { id: string; kind: 'scene' | 'cover'; label: string; usedVersion: number; refLimit: number; versions: ApiVersion[] }
type LegacyVersion = { version: number; originalUrl?: string; cutoutUrl?: string; prompt?: string; width?: number; height?: number }
type LegacyImage = { id: string; kind: string; status: string; prompt: string; versions: LegacyVersion[]; sceneId?: string }
type Snapshot = { images?: ApiImage[]; items?: LegacyImage[]; manifest?: { revision: number } | null; approved?: boolean }

const runtimeFaults = new WeakMap<Page, string[]>()

async function settle(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready
    const { promise: painted, resolve } = Promise.withResolvers<void>()
    requestAnimationFrame(() => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
    await painted
  })
  await page.waitForFunction(() => document.getAnimations()
    .filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime))
    .every(animation => animation.playState === 'finished' || animation.playState === 'idle'))
}
async function capture(page: Page, info: TestInfo, name: string) {
  await settle(page)
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
  await page.screenshot({ path: info.outputPath(`${name}.png`) })
}
const displayUrl = (version: ApiVersion | undefined) => version?.url ?? version?.previewUrl ?? null
const hashImage = (page: Page) => new URLSearchParams(new URL(page.url()).hash.slice(new URL(page.url()).hash.indexOf('?') + 1)).get('img')
const hashVideo = (page: Page) => new URLSearchParams(new URL(page.url()).hash.slice(new URL(page.url()).hash.indexOf('?') + 1)).get('video')
const replayCaseReceipt = (caseName: string) =>
  `runs/129-final-app-skin-actions/mutation-matrix-results.json › desktop ${caseName} (pwTestPassed, replayRestored, providerCallsAuthorized 0); desktop/${caseName}/owned-replay-mutation-restoration.json`

function imageSignature(image: ApiImage | undefined) {
  if (!image) return 'missing'
  return JSON.stringify({
    id: image.id,
    usedVersion: image.usedVersion,
    versions: image.versions
      .map(version => ({
        n: version.n,
        state: version.state,
        url: version.url,
        previewUrl: version.previewUrl,
        width: version.width,
        height: version.height,
        sentText: version.sentText,
        feedback: version.feedback,
        diffFromPrev: version.diffFromPrev,
        partsLegacy: version.partsLegacy,
        parts: version.parts.map(part => ({ kind: part.kind, text: part.text, source: part.source.label })),
        refs: version.refs.map(ref => ({ id: ref.id, role: ref.role, url: ref.url, source: ref.source })),
      }))
      .sort((a, b) => a.n - b.n),
  })
}

type ControlSnapshot = {
  scope: 'review-v36' | 'global-navigation'
  role: string
  name: string
  tag: string
  disabled: boolean
  disabledReason: string | null
  pressed: boolean | null
  checked: boolean | null
  selected: boolean | null
  expanded: boolean | null
  surface: string
}
type Control = Omit<ControlSnapshot, 'surface' | 'disabled' | 'disabledReason' | 'pressed' | 'checked' | 'selected' | 'expanded'> & {
  surfaces: string[]
  states: Omit<ControlSnapshot, 'scope' | 'role' | 'name' | 'tag' | 'surface'>[]
}
type Status = 'PASS' | 'COVERED_BY_NAMED_TEST' | 'CAPTURED_ONLY' | 'BLOCKED' | 'NOT_RUN' | 'NOT_APPLICABLE'
type Coverage = {
  status: Status
  action: string
  expected: string
  observed: unknown
  evidenceRef: string | null
  prerequisite?: string
  followUpOwner?: string
}
type LedgerRow = Control & {
  owner: string
  laterOwner: string | null
  targetOwner?: string | null
  feature?: string
  state: Control['states'][number]
  enabled: boolean
  status: Status
  action: string
  expected: string
  observed: unknown
  evidenceRef: string | null
  prerequisite?: string
  followUpOwner?: string
}

const CONTROL_SELECTOR = 'button, a[href], [role="radio"], [role="option"], [role="tab"], [role="button"], [role="checkbox"]'

async function collectControls(page: Page, surface: string): Promise<ControlSnapshot[]> {
  return page.evaluate(({ selector, surface: stateSurface }) => {
    const visible = (element: Element) => {
      if (element.closest('[aria-hidden="true"], [inert]')) return false
      const box = element.getBoundingClientRect()
      const style = getComputedStyle(element)
      return box.width > 1 && box.height > 1 && style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0'
    }
    return Array.from(document.querySelectorAll<HTMLElement>(selector))
      .filter(visible)
      .map(element => {
        const describedBy = (element.getAttribute('aria-describedby') ?? '').split(/\s+/).filter(Boolean)
        const descriptions = describedBy.map(id => document.getElementById(id)?.textContent?.replace(/\s+/g, ' ').trim() ?? '').filter(Boolean)
        const explicitRole = element.getAttribute('role')
        const tag = element.tagName.toLowerCase()
        const role = explicitRole ?? (tag === 'a' ? 'link' : tag)
        const name = (element.getAttribute('aria-label') ?? element.innerText ?? element.textContent ?? element.getAttribute('title') ?? '')
          .replace(/\s+/g, ' ').trim()
        const disabled = element.matches(':disabled') || element.getAttribute('aria-disabled') === 'true'
        const title = element.getAttribute('title')?.replace(/\s+/g, ' ').trim() ?? ''
        const described = descriptions.join(' ').trim()
        const reason = title || described
        const header = element.closest('header')
        const reviewHeader = header?.querySelector('h1') ? header : null
        const scope = element.closest('nav, [role="navigation"]') || (header && !reviewHeader)
          ? 'global-navigation' as const
          : 'review-v36' as const
        return {
          scope,
          role,
          name: name || '(unnamed)',
          tag,
          disabled,
          disabledReason: disabled && reason && reason !== name ? reason : null,
          pressed: element.getAttribute('aria-pressed') === null ? null : element.getAttribute('aria-pressed') === 'true',
          checked: element.getAttribute('aria-checked') === null ? null : element.getAttribute('aria-checked') === 'true',
          selected: element.getAttribute('aria-selected') === null ? null : element.getAttribute('aria-selected') === 'true',
          expanded: element.getAttribute('aria-expanded') === null ? null : element.getAttribute('aria-expanded') === 'true',
          surface: stateSurface,
        }
      })
  }, { selector: CONTROL_SELECTOR, surface })
}

function mergeControls(target: Map<string, Control>, snapshots: ControlSnapshot[]) {
  for (const snapshot of snapshots) {
    const key = `${snapshot.scope}|${snapshot.role}|${snapshot.name}`
    const state = {
      disabled: snapshot.disabled,
      disabledReason: snapshot.disabledReason,
      pressed: snapshot.pressed,
      checked: snapshot.checked,
      selected: snapshot.selected,
      expanded: snapshot.expanded,
    }
    const existing = target.get(key)
    if (!existing) {
      target.set(key, {
        scope: snapshot.scope,
        role: snapshot.role,
        name: snapshot.name,
        tag: snapshot.tag,
        surfaces: [snapshot.surface],
        states: [state],
      })
      continue
    }
    if (!existing.surfaces.includes(snapshot.surface)) existing.surfaces.push(snapshot.surface)
    if (!existing.states.some(candidate => JSON.stringify(candidate) === JSON.stringify(state))) existing.states.push(state)
  }
}

async function openReview(page: Page, hash: string) {
  await page.goto(hash, { waitUntil: 'domcontentloaded' })
  await expect.poll(() => {
    const current = new URL(page.url()).hash
    return current.startsWith('#/setup') ? 'setup' : current.startsWith('#/review') ? 'review' : 'loading'
  }, { timeout: 20_000 }).not.toBe('loading')
  if (!new URL(page.url()).hash.startsWith('#/setup')) return 'review'
  // Native setup escape, not a fabricated session/readiness flag.
  await page.getByRole('link', { name: 'Cài đặt nâng cao', exact: true }).click()
  await expect(page).toHaveURL(/#\/settings/)
  await page.goto(hash, { waitUntil: 'domcontentloaded' })
  await expect.poll(() => {
    const current = new URL(page.url()).hash
    return current.startsWith('#/setup') ? 'setup' : current.startsWith('#/review') ? 'review' : 'loading'
  }, { timeout: 20_000 }).not.toBe('loading')
  return new URL(page.url()).hash.startsWith('#/setup') ? 'blocked' : 'review'
}

type ReplayGate = { a: string; b: string }
type ReplayJob = {
  id: string
  stageGroup: string
  status?: string
  activeStep?: string | null
  source?: { title?: string; url?: string }
}
type WriteRecord = { method: string; path: string; body: unknown }
type BrowserPost = { status: number; body: unknown }

/**
 * Replay mutation acceptance is deliberately separate from the read-only
 * fixture. Each desktop/motion invocation needs its own controller restoration;
 * the normal suite remains zero-write. Mobile mutations belong to PR6.
 */
function requireReplayGate(info: TestInfo): ReplayGate {
  const a = process.env.WORKFLOW_QA_REPLAY_A_ID ?? ''
  const b = process.env.WORKFLOW_QA_REPLAY_B_ID ?? ''
  test.skip(info.project.name.startsWith('mobile'), 'Mobile mutation parity is owned by PR6; desktop projects require isolated controller restoration')
  test.skip(process.env.WORKFLOW_QA_REPLAY_MUTATIONS !== '1', 'Replay mutation acceptance requires WORKFLOW_QA_REPLAY_MUTATIONS=1')
  test.skip(process.env.WORKFLOW_QA_WORKER_STANDBY !== '1', 'Replay mutation acceptance requires a confirmed standby worker (WORKFLOW_QA_WORKER_STANDBY=1)')
  test.skip(!a || !b || a === b, 'WORKFLOW_QA_REPLAY_A_ID and WORKFLOW_QA_REPLAY_B_ID must be two distinct exact fixture job IDs')
  // This paid source is immutable even if a caller overrides the additional
  // source guard for a different fixture.
  expect([a, b], 'The paid source is never a mutable replay').not.toContain('3225b629-51c8-4f81-8456-9ecacde664ac')
  const source = process.env.WORKFLOW_QA_SOURCE_JOB_ID ?? '3225b629-51c8-4f81-8456-9ecacde664ac'
  expect([a, b], 'A/B must never include the original source job').not.toContain(source)
  expect(fixture.jobs.map(job => job.id), 'A/B must be fixture replay jobs').toEqual(expect.arrayContaining([a, b]))
  return { a, b }
}

async function readImageSnapshot(page: Page, job: string): Promise<Snapshot> {
  const response = await page.request.get(`/api/clone/${encodeURIComponent(job)}/images`)
  expect(response.status(), `the live §26 image snapshot for ${job} must be readable`).toBe(200)
  return await response.json() as Snapshot
}

async function readReviewJobs(page: Page): Promise<{ jobs: ReplayJob[] }> {
  const response = await page.request.get('/api/clone?channel=all')
  expect(response.status(), 'review job list must be readable before and after replay mutations').toBe(200)
  return await response.json() as { jobs: ReplayJob[] }
}

async function readCloneJob(page: Page, job: string): Promise<{ status?: string; source?: { url?: string } }> {
  const response = await page.request.get(`/api/clone/${encodeURIComponent(job)}`)
  expect(response.status(), `clone job ${job} must be readable`).toBe(200)
  return await response.json() as { status?: string; source?: { url?: string } }
}

async function browserPost(page: Page, path: string, body: object): Promise<BrowserPost> {
  return page.evaluate(async ({ path: target, body: payload }) => {
    const response = await fetch(target, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    const text = await response.text()
    let parsed: unknown = null
    try { parsed = text ? JSON.parse(text) : null } catch { parsed = text }
    return { status: response.status, body: parsed }
  }, { path, body })
}

/**
 * Proactively fences every API write after the guard is installed. A write is
 * continued only when `allowed` matches the exact method/path/body; everything
 * else is aborted before it reaches the server. `unexpected` is still asserted
 * afterwards, but the server never sees a disallowed mutation if a future UI
 * path regresses.
 */
async function guardWrites(page: Page, allowed: (write: WriteRecord) => boolean) {
  const writes: WriteRecord[] = []
  const unexpected: string[] = []
  await page.route('**/api/**', async route => {
    const request = route.request()
    if (['GET', 'HEAD', 'OPTIONS'].includes(request.method())) {
      await route.continue()
      return
    }
    let body: unknown
    try { body = request.postDataJSON() } catch { body = request.postData() }
    const write = { method: request.method(), path: new URL(request.url()).pathname, body }
    writes.push(write)
    if (allowed(write)) {
      await route.continue()
      return
    }
    unexpected.push(`${write.method} ${write.path} ${(JSON.stringify(write.body) ?? '').slice(0, 240)}`)
    await route.abort('blockedbyclient')
  })
  return { writes, unexpected }
}

async function ensureReplayBaseline(page: Page, gate: ReplayGate) {
  const { jobs } = await readReviewJobs(page)
  const review = jobs.filter(job => job.stageGroup === 'review')
  expect(review.some(job => job.id === gate.a), `A replay job ${gate.a} must be in the review group`).toBe(true)
  expect(review.some(job => job.id === gate.b), `B replay job ${gate.b} must be in the review group`).toBe(true)
  const a = await readImageSnapshot(page, gate.a)
  const b = await readImageSnapshot(page, gate.b)
  for (const [label, snapshot] of [['A', a], ['B', b]] as const) {
    expect((snapshot.items ?? []).filter(item => item.status === 'pending' || item.status === 'running'), `${label} must start without a pending/running image`).toEqual([])
    expect((snapshot.images ?? []).filter(image => image.versions.some(version => version.state === 'drawing')), `${label} must start without a drawing version`).toEqual([])
  }
  return { review, a, b }
}

async function openReadyImage(page: Page, job: string, imageId: string, label: string) {
  const state = await openReview(page, `/#/review?video=${encodeURIComponent(job)}&img=${encodeURIComponent(imageId)}`)
  if (state === 'blocked') return false
  await expect(page.getByRole('option', { name: label, exact: true })).toHaveAttribute('aria-selected', 'true', { timeout: 20_000 })
  await expect(page.getByRole('img', { name: label, exact: true })).toBeVisible({ timeout: 20_000 })
  return true
}

const publishedVersion = (version: ApiVersion) => version.state === 'ready' && !!displayUrl(version)
const nextVersion = (image: ApiImage) => Math.max(0, ...image.versions.map(version => version.n)) + 1
const versionTile = (page: Page, n: number) => page
  .getByRole('group', { name: 'Phiên bản của ảnh', exact: true })
  .getByRole('button', { name: new RegExp(`^Bản ${n}(?: ·|$)`) })
const legacyStatusMap = (snapshot: Snapshot) =>
  Object.fromEntries((snapshot.items ?? []).map(item => [item.id, item.status]))
function expectSnapshotUnchanged(before: Snapshot, after: Snapshot) {
  for (const image of before.images ?? []) {
    const current = after.images?.find(candidate => candidate.id === image.id)
    expect(current, `image ${image.id} must still exist after the reversible replay mutation`).toBeDefined()
    expect(imageSignature(current), `image ${image.id} state/version/refs/prompt must be restored`).toBe(imageSignature(image))
  }
  expect(legacyStatusMap(after), 'legacy review statuses must be restored').toEqual(legacyStatusMap(before))
}

async function cancelPlanned(page: Page, job: string, imageId: string, n: number, usedVersion: number) {
  const path = `/api/clone/${encodeURIComponent(job)}/images/${encodeURIComponent(imageId)}/versions/${n}/cancel`
  const response = await browserPost(page, path, {})
  expect(response.status, `cancel ${imageId} v${n} must be accepted: ${JSON.stringify(response.body)}`).toBe(200)
  const body = response.body as { cancelled: boolean; costImages: number; usedVersion: number; n: number }
  expect(body.cancelled, `cancel ${imageId} v${n} must really remove the queued draft`).toBe(true)
  expect(body.costImages, 'standby worker means a queued redraw cancel never spends a provider image').toBe(0)
  expect(body.usedVersion, 'cancel restores the previously used version').toBe(usedVersion)
  expect(body.n).toBe(n)
  return body
}

const isLearnControl = (name: string) => /^(Lưu quy tắc|Không|Xem \d+ lần sửa|Ẩn ví dụ)$/.test(name)
function targetOwner(name: string) {
  if (/^Kênh|^Khám phá|Thư viện/.test(name)) return 'PR4'
  if (/^Video mới/.test(name)) return 'PR5'
  if (/^(Cài đặt|Tài khoản)/.test(name)) return 'PR6'
  if (/^Duyệt/.test(name)) return 'PR2 (destination only)'
  return 'PR3'
}

test.beforeEach(async ({ context, page }) => {
  const faults: string[] = []
  runtimeFaults.set(page, faults)
  page.on('pageerror', error => faults.push(error.message))
  await context.addCookies(JSON.parse(readFileSync(cookiePath, 'utf8')))
  const me = await context.request.get('/api/me')
  expect(me.status()).toBe(200)
  expect((await me.json()).user.username).toBe('workflow.qa')
})
test.afterEach(async ({ page }, info) => {
  const faults = runtimeFaults.get(page) ?? []
  await info.attach('runtime-errors.json', { body: JSON.stringify(faults), contentType: 'application/json' })
  expect(faults).toEqual([])
})

test('Review V36 safe controls keep identity, navigation, clipboard, compare, diff and reference choices without writes', async ({ page, context }, info) => {
  const mobile = page.viewportSize()!.width < 768
  const prefix = mobile ? 'mobile-' : ''
  const job = fixture.jobs[0]
  if (!job) {
    test.skip(true, 'The isolated fixture has no review job; controller replay seeding is required')
    return
  }
  const base = `/api/clone/${encodeURIComponent(job.id)}`
  const imagesEndpoint = `${base}/images`
  const reviewWrites: string[] = []
  const snapshotResponse = await page.request.get(imagesEndpoint)
  expect(snapshotResponse.status(), 'the live §26 image snapshot must be readable').toBe(200)
  const snapshot = await snapshotResponse.json() as Snapshot
  const images = snapshot.images ?? []
  const ready = (version: ApiVersion) => version.state === 'ready' && !!displayUrl(version)
  const target = images.find(image => image.id === 'sc-04' && image.versions.some(ready))
    ?? images.find(image => image.kind === 'scene' && image.versions.some(ready))
    ?? images.find(image => image.versions.some(ready))
  if (!target) {
    await info.attach('review-control-ledger.json', {
      body: JSON.stringify({
        status: 'BLOCKED',
        prerequisite: 'The normal fixture must expose a live ready image version with a published asset; no synthetic image was substituted.',
        imageCount: images.length,
        reviewWrites,
      }, null, 2),
      contentType: 'application/json',
    })
    test.skip(true, 'No ready live review image version is available; no synthetic state was substituted')
    return
  }

  const legacyTarget = snapshot.items?.find(item => item.id === target.id)
  const state = await openReview(page, `/#/review?video=${encodeURIComponent(job.id)}&img=${encodeURIComponent(target.id)}`)
  if (state === 'blocked') {
    await info.attach('review-control-ledger.json', {
      body: JSON.stringify({
        status: 'BLOCKED',
        prerequisite: 'Machine setup is incomplete. The native “Cài đặt nâng cao” escape returned to #/setup; no readiness/session flag was fabricated.',
        targetImage: target.id,
        reviewWrites,
      }, null, 2),
      contentType: 'application/json',
    })
    test.skip(true, 'Machine setup incomplete; the native setup escape cannot open Review')
    return
  }

  // Fence every API write after Review opens, not just this job's route. A
  // future regression that tries to change the source job, another job, a
  // provider, login, tools or upload is aborted before it reaches the server.
  const writeGate = await guardWrites(page, () => false)

  await expect(page.getByRole('option', { name: target.label, exact: true })).toHaveAttribute('aria-selected', 'true', { timeout: 20_000 })
  await expect(page.getByRole('img', { name: target.label, exact: true })).toBeVisible({ timeout: 20_000 })
  if (mobile) await expect(page.getByRole('dialog'), 'the round45 shared mobile image viewer must stay absent').toHaveCount(0)
  await settle(page)

  const baselineSignature = imageSignature(target)
  const sourceBeforeResponse = await page.request.get(base)
  expect(sourceBeforeResponse.status()).toBe(200)
  const sourceBefore = await sourceBeforeResponse.json() as { status: string; source?: { url?: string } }

  const controls = new Map<string, Control>()
  let stripReadback: Array<{ label: string; id: string; stageAlt: string | null; restored: boolean }> | null = null
  let stripRestored: string | null = null
  let jobReadback: {
    a: { label: string; id: string; heading: string } | null
    b: { label: string; id: string; heading: string } | null
    restored: { id: string; heading: string } | null
  } | null = null
  mergeControls(controls, await collectControls(page, 'default'))
  await capture(page, info, `${prefix}review-control-ledger-default`)

  const rail = page.locator('aside[aria-label="Ảnh và chỉ dẫn"]')
  const versionGroup = page.getByRole('group', { name: 'Phiên bản của ảnh', exact: true })
  const versionButton = (n: number) => mobile
    ? page.getByRole('button', { name: new RegExp(`^Bản ${n}$`) })
    : versionGroup.getByRole('button', { name: new RegExp(`^Bản ${n}(?: ·|$)`) })
  const stage = () => page.locator('main:visible').getByRole('img', { name: target.label, exact: true })

  const optionLabels = await page.getByRole('listbox', { name: 'Ảnh của video', exact: true }).getByRole('option')
    .evaluateAll(nodes => nodes.map(node => ({
      label: node.getAttribute('aria-label') ?? '',
      selected: node.getAttribute('aria-selected') === 'true',
    })))
  let navigation: { from: string | null; to: string | null; selectedLabel: string; returned: string | null; keyboard?: { to: string | null; returned: string | null } } | null = null
  if (optionLabels.length > 1) {
    const selectedIndex = optionLabels.findIndex(entry => entry.selected)
    expect(selectedIndex, 'the addressed image must be selected in the strip').toBeGreaterThanOrEqual(0)
    const next = optionLabels[(selectedIndex + 1) % optionLabels.length]
    const fromId = hashImage(page)
    await page.getByRole('button', { name: 'Ảnh sau', exact: true }).click()
    await expect.poll(() => {
      const selected = page.getByRole('listbox', { name: 'Ảnh của video', exact: true }).getByRole('option', { selected: true })
      return selected.getAttribute('aria-label')
    }).toBe(next.label)
    const toId = hashImage(page)
    expect(toId).not.toBe(fromId)
    await page.getByRole('button', { name: 'Ảnh trước', exact: true }).click()
    await expect.poll(async () => {
      const selected = page.getByRole('listbox', { name: 'Ảnh của video', exact: true }).getByRole('option', { selected: true })
      return selected.getAttribute('aria-label')
    }).toBe(target.label)
    expect(hashImage(page)).toBe(fromId)
    navigation = { from: fromId, to: toId, selectedLabel: next.label, returned: hashImage(page) }
    if (!mobile) {
      await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur())
      await page.keyboard.press('ArrowRight')
      await expect.poll(async () => {
        const selected = page.getByRole('listbox', { name: 'Ảnh của video', exact: true }).getByRole('option', { selected: true })
        return selected.getAttribute('aria-label')
      }).toBe(next.label)
      const keyboardTo = hashImage(page)
      await page.keyboard.press('ArrowLeft')
      await expect.poll(async () => {
        const selected = page.getByRole('listbox', { name: 'Ảnh của video', exact: true }).getByRole('option', { selected: true })
        return selected.getAttribute('aria-label')
      }).toBe(target.label)
      expect(keyboardTo).toBe(toId)
      expect(hashImage(page)).toBe(fromId)
      navigation.keyboard = { to: keyboardTo, returned: hashImage(page) }
      await capture(page, info, `${prefix}review-control-ledger-keyboard-navigation`)
    }
  }

  if (!mobile) {
    const sidebar = page.locator('aside[aria-label="Video chờ duyệt"]')
    const sidebarButtons = sidebar.getByRole('button')
    const originalIndex = await sidebarButtons.evaluateAll(nodes => nodes.findIndex(node => node.getAttribute('aria-pressed') === 'true'))
    expect(originalIndex, 'the addressed review job must be selected in the sidebar').toBeGreaterThanOrEqual(0)
    const originalJobId = hashVideo(page)
    const originalHeading = (await page.getByRole('heading', { level: 1 }).textContent())?.trim() ?? ''
    const reviewJobs = (await readReviewJobs(page)).jobs.filter(entry => entry.stageGroup === 'review')
    const observed: NonNullable<typeof jobReadback> = { a: null, b: null, restored: null }
    for (const side of ['A', 'B'] as const) {
      const expected = reviewJobs.find(entry => (entry.source?.title ?? '').startsWith(`QA replay ${side}`))
      const button = sidebar.getByRole('button', { name: new RegExp(`^QA replay ${side}(?: ·|$)`) })
      if (!expected || !(await button.count())) continue
      await button.click()
      await expect(button).toHaveAttribute('aria-pressed', 'true')
      await expect.poll(() => hashVideo(page)).toBe(expected.id)
      await expect(page.getByRole('heading', { level: 1 })).toHaveText(expected.source?.title ?? '')
      observed[side === 'A' ? 'a' : 'b'] = { label: `QA replay ${side}`, id: expected.id, heading: expected.source?.title ?? '' }
    }
    if (observed.a && observed.b) {
      expect(new Set([observed.a.id, observed.b.id]).size, 'QA replay A and B must resolve to two distinct jobs').toBe(2)
      expect(fixture.jobs.map(entry => entry.id), 'A/B must be exact fixture jobs').toEqual(expect.arrayContaining([observed.a.id, observed.b.id]))
    }
    if (originalIndex >= 0) {
      await sidebarButtons.nth(originalIndex).click()
      await expect.poll(() => hashVideo(page)).toBe(originalJobId)
      await expect(page.getByRole('heading', { level: 1 })).toHaveText(originalHeading)
      await expect(sidebarButtons.nth(originalIndex)).toHaveAttribute('aria-pressed', 'true')
      observed.restored = { id: originalJobId ?? '', heading: originalHeading }
      const strip = page.getByRole('listbox', { name: 'Ảnh của video', exact: true })
      await strip.getByRole('option', { name: target.label, exact: true }).click()
      await expect.poll(() => hashImage(page)).toBe(target.id)
      await expect(strip.getByRole('option', { name: target.label, exact: true })).toHaveAttribute('aria-selected', 'true')
    }
    jobReadback = observed
  }

  const strip = page.getByRole('listbox', { name: 'Ảnh của video', exact: true })
  const stripOptions = await strip.getByRole('option').evaluateAll(nodes => nodes.map(node => ({
    label: node.getAttribute('aria-label') ?? '',
    selected: node.getAttribute('aria-selected') === 'true',
  })))
  const originalStripIndex = stripOptions.findIndex(entry => entry.selected)
  expect(originalStripIndex, 'the addressed image must be selected before the per-option readback').toBeGreaterThanOrEqual(0)
  const observedStrip: NonNullable<typeof stripReadback> = []
  for (const [index, entry] of stripOptions.entries()) {
    const option = strip.getByRole('option').nth(index)
    await option.click()
    await expect(option).toHaveAttribute('aria-selected', 'true')
    const addressId = hashImage(page)
    expect(addressId, `strip option ${entry.label} must address a live image id`).toBeTruthy()
    const liveImage = images.find(image => image.label === entry.label)
    if (liveImage) expect(addressId, `strip option ${entry.label} must address its exact live image`).toBe(liveImage.id)
    const stageImage = page.locator('main:visible').getByRole('img', { name: entry.label, exact: true }).first()
    await expect(stageImage).toBeVisible()
    const stageAlt = await stageImage.getAttribute('alt')
    observedStrip.push({ label: entry.label, id: addressId!, stageAlt, restored: false })
  }
  await capture(page, info, `${prefix}review-control-ledger-strip-selected`)
  await strip.getByRole('option').nth(originalStripIndex).click()
  await expect.poll(() => hashImage(page)).toBe(target.id)
  await expect(strip.getByRole('option').nth(originalStripIndex)).toHaveAttribute('aria-selected', 'true')
  for (const entry of observedStrip) entry.restored = entry.id === target.id
  stripReadback = observedStrip
  stripRestored = hashImage(page)
  expect(observedStrip.filter(entry => entry.restored), 'the original strip option must be restored').toHaveLength(1)

  const observedVersions: Record<number, { natural: [number, number]; api: [number, number] }> = {}
  const mobileLegacyUrl = (n: number) => legacyTarget?.versions.find(version => version.version === n)?.originalUrl ?? null
  const expectedVersionUrl = (version: ApiVersion) => mobile
    ? (mobileLegacyUrl(version.n) ?? displayUrl(version))
    : displayUrl(version)
  const readyVersions = target.versions
    .filter(version => version.state === 'ready' && !!displayUrl(version) && (!mobile || !!mobileLegacyUrl(version.n)))
    .sort((a, b) => a.n - b.n)
  const versionTilesExpected = !mobile || (legacyTarget?.versions.length ?? 0) > 1
  if (versionTilesExpected) {
    for (const version of readyVersions) {
      const button = versionButton(version.n)
      await expect(button).toBeVisible()
      await button.click()
      await expect(button).toHaveAttribute('aria-pressed', 'true')
      const expected = expectedVersionUrl(version)
      expect(expected, `version ${version.n} must have a published asset`).toBeTruthy()
      const image = stage()
      await expect.poll(() => image.getAttribute('src')).toBe(expected)
      await expect.poll(() => image.evaluate(element => (element as HTMLImageElement).complete && (element as HTMLImageElement).naturalWidth > 0)).toBe(true)
      const natural = await image.evaluate(element => [(element as HTMLImageElement).naturalWidth, (element as HTMLImageElement).naturalHeight] as [number, number])
      expect(natural).toEqual([version.width, version.height])
      observedVersions[version.n] = { natural, api: [version.width, version.height] }
    }
  }
  const pair = readyVersions.filter(version => readyVersions.some(previous => previous.n === version.n - 1))
  const later = pair.at(-1) ?? readyVersions.at(-1)
  const previous = later ? target.versions.find(version => version.n === later.n - 1 && version.state === 'ready') : undefined
  if (later && versionTilesExpected) {
    await versionButton(later.n).click()
    await expect(versionButton(later.n)).toHaveAttribute('aria-pressed', 'true')
  }
  await capture(page, info, `${prefix}review-control-ledger-version-preview`)

  let compareObserved: {
    onPressed: string | null
    onAlt: string | null
    onUrlMatchesApi: boolean
    offPressed: string | null
    keyboardV?: { onPressed: string | null; offPressed: string | null }
  } | null = null
  if (later && previous) {
    const compareButton = page.getByRole('button', { name: /^(So với bản|Đang xem bản)/ })
    if (await compareButton.count()) {
      const expectedPrevious = expectedVersionUrl(previous)
      await compareButton.click()
      await expect(compareButton).toHaveAttribute('aria-pressed', 'true')
      await expect(compareButton).toContainText(`Đang xem bản ${previous.n}`)
      // Desktop names the compared asset in the img alt. MReview keeps the
      // image alt as the scene label and identifies the version in the stage
      // tag plus the compare button — do not require a desktop-only alt there.
      const compared = mobile
        ? page.locator('main:visible').getByRole('img', { name: target.label, exact: true })
        : page.getByRole('img', { name: `${target.label} bản ${previous.n}`, exact: true })
      await expect.poll(() => compared.getAttribute('src')).toBe(expectedPrevious)
      const onPressed = await compareButton.getAttribute('aria-pressed')
      const onAlt = await compared.getAttribute('alt')
      const onUrlMatchesApi = await compared.getAttribute('src') === expectedPrevious
      if (mobile) await expect(page.getByText(`bản ${previous.n}`, { exact: true })).toBeVisible()
      await capture(page, info, `${prefix}review-control-ledger-compare`)
      await compareButton.click()
      await expect(compareButton).toHaveAttribute('aria-pressed', 'false')
      const offPressed = await compareButton.getAttribute('aria-pressed')
      let keyboardV: { onPressed: string | null; offPressed: string | null } | undefined
      if (!mobile) {
        await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur())
        await page.keyboard.press('v')
        await expect(compareButton).toHaveAttribute('aria-pressed', 'true')
        await expect(compareButton).toContainText(`Đang xem bản ${previous.n}`)
        const keyboardOn = await compareButton.getAttribute('aria-pressed')
        await page.keyboard.press('v')
        await expect(compareButton).toHaveAttribute('aria-pressed', 'false')
        keyboardV = { onPressed: keyboardOn, offPressed: await compareButton.getAttribute('aria-pressed') }
      }
      compareObserved = { onPressed, onAlt, onUrlMatchesApi, offPressed, ...(keyboardV ? { keyboardV } : {}) }
    }
  }

  const selectedVersion = later ?? readyVersions.at(-1) ?? target.versions.find(version => version.state !== 'cancelled')
  const selectedLegacy = selectedVersion && legacyTarget?.versions.find(version => version.version === selectedVersion.n)
  let clipboard: { status: Status; expectedLength: number; actualLength?: number; exact?: boolean; prerequisite?: string } | null = null
  if (selectedVersion) {
    const expectedText = mobile ? (selectedLegacy?.prompt?.trim() || legacyTarget?.prompt?.trim() || '') : selectedVersion.sentText
    if (!expectedText) {
      clipboard = { status: 'BLOCKED', expectedLength: 0, prerequisite: 'The selected live version has no sentText / mobile prompt to copy.' }
    } else if (mobile && !(await page.getByRole('button', { name: 'Xem prompt đã gửi', exact: true }).count())) {
      clipboard = { status: 'BLOCKED', expectedLength: expectedText.length, prerequisite: 'MReview prompt sheet trigger was not present in the normal fixture.' }
    } else {
      if (mobile) {
        await page.getByRole('button', { name: 'Xem prompt đã gửi', exact: true }).click()
        const sheet = page.getByRole('dialog', { name: /^Prompt đã gửi/ })
        await expect(sheet).toBeVisible()
        mergeControls(controls, await collectControls(page, 'mobile-prompt-sheet'))
        const copyButton = sheet.getByRole('button', { name: 'Chép chỉ dẫn', exact: true })
        let permission = true
        try {
          await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: new URL(page.url()).origin })
        } catch {
          permission = false
        }
        if (!permission) {
          clipboard = { status: 'BLOCKED', expectedLength: expectedText.length, prerequisite: 'Chromium clipboard-read/write permission is unavailable in this runner.' }
        } else {
          await copyButton.click()
          let actual = ''
          try {
            actual = await page.evaluate(() => navigator.clipboard.readText())
          } catch {
            clipboard = { status: 'BLOCKED', expectedLength: expectedText.length, prerequisite: 'navigator.clipboard.readText() was denied by the runner/browser.' }
          }
          if (clipboard === null) {
            expect(actual, 'clipboard must contain the exact prompt shown by MReview').toBe(expectedText)
            clipboard = { status: 'PASS', expectedLength: expectedText.length, actualLength: actual.length, exact: actual === expectedText }
            await capture(page, info, `${prefix}review-control-ledger-copied-prompt`)
          }
        }
        await page.keyboard.press('Escape')
        await expect(sheet).toHaveCount(0)
      } else {
        const copyButton = rail.getByRole('button', { name: /^(Chép|Đã chép)$/ })
        let permission = true
        try {
          await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: new URL(page.url()).origin })
        } catch {
          permission = false
        }
        if (!permission) {
          clipboard = { status: 'BLOCKED', expectedLength: expectedText.length, prerequisite: 'Chromium clipboard-read/write permission is unavailable in this runner.' }
        } else {
          await copyButton.click()
          await expect(copyButton, 'the copy control must visibly confirm the copy').toHaveAttribute('aria-label', 'Đã chép')
          let actual = ''
          try {
            actual = await page.evaluate(() => navigator.clipboard.readText())
          } catch {
            clipboard = { status: 'BLOCKED', expectedLength: expectedText.length, prerequisite: 'navigator.clipboard.readText() was denied by the runner/browser.' }
          }
          if (clipboard === null) {
            expect(actual, 'clipboard must contain the exact sentText of the displayed version').toBe(expectedText)
            clipboard = { status: 'PASS', expectedLength: expectedText.length, actualLength: actual.length, exact: actual === expectedText }
            mergeControls(controls, await collectControls(page, 'prompt-copied'))
            await capture(page, info, `${prefix}review-control-ledger-copied-prompt`)
          }
        }
      }
    }
  }

  let diffObserved: { hiddenMarks: number; shownMarks: number } | null = null
  let rawObserved: { exactSentText: boolean; length: number; headerControlsHiddenInRaw: boolean; headerControlsRestored: boolean } | null = null
  let editObserved: {
    changedMarker: boolean
    saveAriaDisabled: string | null
    undo?: { restoredExactBytes: boolean; dirtyCleared: boolean }
    baseRefRemove?: { before: number; after: number; dirty: boolean } | null
    refsRestored?: number | null
    cancelled: boolean
  } | null = null
  let stageEditObserved = false
  let refsObserved: { tabs: string[]; roles: string[]; picked: boolean; closeButton: boolean; focusReturned: boolean; fileChooserCancelled: boolean } | null = null
  let candidateReadback: Array<{ label: string; pressed: boolean; exactlyOnePressed: boolean }> | null = null
  let pickedCandidate: string | null = null
  let referenceFileChooserCancelled = false

  if (!mobile) {
    const diffToggle = rail.getByRole('button', { name: /^(Ẩn thay đổi|Xem thay đổi)$/ })
    if (await diffToggle.count()) {
      const marks = rail.locator('ins, del')
      const shownMarks = await marks.count()
      expect(shownMarks, 'the version diff must be annotated before hiding it').toBeGreaterThan(0)
      await diffToggle.click()
      await expect(diffToggle).toContainText('Xem thay đổi')
      await expect(marks).toHaveCount(0)
      mergeControls(controls, await collectControls(page, 'prompt-diff-hidden'))
      await capture(page, info, `${prefix}review-control-ledger-diff-hidden`)
      await diffToggle.click()
      await expect(diffToggle).toContainText('Ẩn thay đổi')
      await expect(marks.first()).toBeVisible()
      diffObserved = { hiddenMarks: 0, shownMarks: await marks.count() }
    }

    if (selectedVersion) {
      const rawToggle = rail.getByRole('button', { name: 'Xem nguyên văn gửi máy chủ', exact: true })
      if (await rawToggle.count()) {
        await rawToggle.click()
        const backToParts = rail.getByRole('button', { name: 'Xem theo phần', exact: true })
        await expect(backToParts).toBeVisible()
        await expect(rail.getByRole('button', { name: /^(Chép|Đã chép)$/ })).toHaveCount(0)
        await expect(rail.getByRole('button', { name: 'Sửa', exact: true })).toHaveCount(0)
        const rawText = await rail.locator('pre').textContent()
        expect(rawText, 'raw prompt readback must be byte-identical to the live sentText').toBe(selectedVersion.sentText)
        mergeControls(controls, await collectControls(page, 'prompt-raw'))
        await capture(page, info, `${prefix}review-control-ledger-raw`)
        await backToParts.click()
        await expect(rail.getByRole('button', { name: /^(Chép|Đã chép)$/ })).toBeVisible()
        await expect(rail.getByRole('button', { name: 'Sửa', exact: true })).toBeVisible()
        rawObserved = { exactSentText: true, length: selectedVersion.sentText.length, headerControlsHiddenInRaw: true, headerControlsRestored: true }
      }
    }

    const headerEdit = rail.getByRole('button', { name: 'Sửa', exact: true })
    if (await headerEdit.count() && await headerEdit.isEnabled()) {
      await headerEdit.click()
      const scene = rail.locator('#prompt-scene')
      await expect(scene).toBeVisible()
      const original = await scene.inputValue()
      await scene.fill(`${original} [QA read-only audit change]`)
      await expect(rail.getByText('Đã sửa', { exact: true }).first()).toBeVisible()
      const save = page.getByRole('button', { name: /^Vẽ lại với prompt này(?:\s|$)/ })
      await expect(save).toHaveAttribute('aria-disabled', 'false')
      const saveAriaDisabled = await save.getAttribute('aria-disabled')
      mergeControls(controls, await collectControls(page, 'prompt-edit'))
      await capture(page, info, `${prefix}review-control-ledger-prompt-edit`)

      // Per-part undo restores the exact source bytes and clears the local dirty marker.
      await rail.getByRole('button', { name: 'Hoàn tác', exact: true }).click()
      await expect(scene).toHaveValue(original)
      expect(await scene.inputValue()).toBe(original)
      await expect(rail.getByText('Đã sửa', { exact: true })).toHaveCount(0)
      await expect(save).toHaveAttribute('aria-disabled', 'true')
      const undoObserved = { restoredExactBytes: (await scene.inputValue()) === original, dirtyCleared: (await save.getAttribute('aria-disabled')) === 'true' }
      mergeControls(controls, await collectControls(page, 'prompt-undo'))
      await capture(page, info, `${prefix}review-control-ledger-prompt-undo`)

      // Removing a base reference only changes the local draft; cancel must restore it.
      const baseRefRemove = rail.getByRole('button', { name: /^Bỏ ảnh tham chiếu \d+$/ })
      const baseRefCount = await baseRefRemove.count()
      let baseRefRemoveObserved: { before: number; after: number; dirty: boolean } | null = null
      if (baseRefCount > 0) {
        await baseRefRemove.first().click()
        await expect(baseRefRemove).toHaveCount(baseRefCount - 1)
        await expect(page.getByText(/^Đã sửa \d+ chỗ trong prompt/)).toBeVisible()
        await expect(save).toHaveAttribute('aria-disabled', 'false')
        baseRefRemoveObserved = { before: baseRefCount, after: await baseRefRemove.count(), dirty: true }
        mergeControls(controls, await collectControls(page, 'prompt-edit-ref-removed'))
      }
      const cancelEdit = page.getByRole('button', { name: /^Huỷ sửa(?:\s|$)/ })
      await expect(cancelEdit).toBeVisible()
      await cancelEdit.click()
      await expect(rail.locator('#prompt-scene')).toHaveCount(0)
      await expect(rail.getByRole('button', { name: /^Bỏ ảnh tham chiếu \d+$/ })).toHaveCount(0)
      let refsRestored: number | null = null
      if (baseRefCount > 0) {
        await headerEdit.click()
        await expect(scene).toBeVisible()
        refsRestored = await rail.getByRole('button', { name: /^Bỏ ảnh tham chiếu \d+$/ }).count()
        expect(refsRestored, 'cancel restores the exact base reference set').toBe(baseRefCount)
        await cancelEdit.click()
        await expect(rail.locator('#prompt-scene')).toHaveCount(0)
      }
      editObserved = { changedMarker: true, saveAriaDisabled, undo: undoObserved, baseRefRemove: baseRefRemoveObserved, refsRestored, cancelled: true }
    }

    const stageEdit = page.getByRole('button', { name: 'Sửa prompt', exact: true })
    if (await stageEdit.count() && await stageEdit.isEnabled()) {
      await stageEdit.click()
      await expect(rail.locator('#prompt-scene')).toBeVisible()
      await page.getByRole('button', { name: /^Huỷ sửa(?:\s|$)/ }).click()
      await expect(rail.locator('#prompt-scene')).toHaveCount(0)
      stageEditObserved = true
    }

    const add = rail.getByRole('button', { name: 'Thêm', exact: true })
    if (await add.count() && await add.isEnabled()) {
      await add.click()
      let dialog = page.getByRole('dialog', { name: 'Thêm ảnh tham chiếu', exact: true })
      await expect(dialog).toBeVisible()
      mergeControls(controls, await collectControls(page, 'reference-dialog'))
      const tabs: string[] = []
      const referenceTabs = [
        { label: 'Ảnh trong video', surface: 'reference-images' },
        { label: 'Khung video gốc', surface: 'reference-frames' },
        { label: 'Tải lên', surface: 'reference-upload' },
      ] as const
      for (const tab of referenceTabs) {
        await dialog.getByRole('radio', { name: tab.label, exact: true }).click()
        await expect(dialog.getByRole('radio', { name: tab.label, exact: true })).toHaveAttribute('aria-checked', 'true')
        tabs.push(tab.label)
        if (tab.label === 'Tải lên') {
          await expect(dialog.getByRole('button', { name: 'Tải ảnh tham chiếu', exact: true })).toBeVisible()
          const [chooser] = await Promise.all([
            page.waitForEvent('filechooser'),
            dialog.getByRole('button', { name: 'Tải ảnh tham chiếu', exact: true }).click(),
          ])
          await chooser.setFiles([])
          referenceFileChooserCancelled = true
        }
        mergeControls(controls, await collectControls(page, tab.surface))
      }
      await dialog.getByRole('radio', { name: 'Ảnh trong video', exact: true }).click()
      const roleGroup = dialog.getByRole('radiogroup', { name: 'Vai trò của ảnh', exact: true })
      const roles: string[] = []
      for (const roleName of ['Nhân vật', 'Bố cục', 'Phong cách', 'Đồ vật'] as const) {
        const roleRadio = roleGroup.getByRole('radio', { name: roleName, exact: true })
        await roleRadio.click()
        await expect(roleRadio).toHaveAttribute('aria-checked', 'true')
        roles.push(roleName)
      }
      // Arrow keys wrap between the four role chips and keep focus on the
      // newly checked chip; this is local picker state, not a reference write.
      const firstRole = roleGroup.getByRole('radio', { name: 'Nhân vật', exact: true })
      const lastRole = roleGroup.getByRole('radio', { name: 'Đồ vật', exact: true })
      await firstRole.focus()
      await page.keyboard.press('ArrowLeft')
      await expect(lastRole).toHaveAttribute('aria-checked', 'true')
      await expect(lastRole).toBeFocused()
      await capture(page, info, `${prefix}review-control-ledger-reference-sources`)
      const candidateButtons = dialog.locator('button[aria-pressed]')
      const candidateCount = await candidateButtons.count()
      const initialPressed: string[] = []
      for (let index = 0; index < candidateCount; index += 1) {
        const candidate = candidateButtons.nth(index)
        if (await candidate.getAttribute('aria-pressed') === 'true') initialPressed.push((await candidate.getAttribute('aria-label')) ?? '')
      }
      const observedCandidates: NonNullable<typeof candidateReadback> = []
      for (let index = 0; index < candidateCount; index += 1) {
        const candidate = candidateButtons.nth(index)
        const label = (await candidate.getAttribute('aria-label')) ?? ''
        expect(label, 'every real reference candidate must expose its own aria-label').toBeTruthy()
        await candidate.click()
        await expect(candidate).toHaveAttribute('aria-pressed', 'true')
        const pressedCount = await dialog.locator('button[aria-pressed="true"]').count()
        expect(pressedCount, `only “${label}” may stay pressed`).toBe(1)
        await expect(dialog.getByRole('button', { name: 'Thêm 1 ảnh', exact: true })).toHaveAttribute('aria-disabled', 'false')
        observedCandidates.push({ label, pressed: (await candidate.getAttribute('aria-pressed')) === 'true', exactlyOnePressed: pressedCount === 1 })
      }
      for (let index = 0; index < candidateCount; index += 1) {
        const candidate = candidateButtons.nth(index)
        const label = (await candidate.getAttribute('aria-label')) ?? ''
        const pressed = await candidate.getAttribute('aria-pressed') === 'true'
        if (initialPressed.includes(label) !== pressed) await candidate.click()
      }
      expect(await dialog.locator('button[aria-pressed="true"]').count(), 'the pre-audit candidate selection must be preserved').toBe(initialPressed.length)
      candidateReadback = observedCandidates
      if (candidateCount) {
        if (!initialPressed.length) await candidateButtons.first().click()
        const chosen = dialog.locator('button[aria-pressed="true"]').first()
        pickedCandidate = await chosen.getAttribute('aria-label')
        await expect(chosen).toHaveAttribute('aria-pressed', 'true')
        await expect(dialog.getByRole('button', { name: 'Thêm 1 ảnh', exact: true })).toHaveAttribute('aria-disabled', 'false')
        mergeControls(controls, await collectControls(page, 'reference-picked'))
        await capture(page, info, `${prefix}review-control-ledger-reference-roles`)
      }
      await dialog.getByRole('button', { name: 'Huỷ', exact: true }).click()
      await expect(dialog).toHaveCount(0)
      await expect(add).toBeFocused()
      await add.click()
      dialog = page.getByRole('dialog', { name: 'Thêm ảnh tham chiếu', exact: true })
      await expect(dialog).toBeVisible()
      await dialog.getByRole('button', { name: 'Đóng', exact: true }).click()
      await expect(dialog).toHaveCount(0)
      refsObserved = { tabs, roles, picked: !!pickedCandidate, closeButton: true, focusReturned: true, fileChooserCancelled: referenceFileChooserCancelled }
      await capture(page, info, `${prefix}review-control-ledger-reference-cancelled`)
    }
  }

  const afterResponse = await page.request.get(imagesEndpoint)
  expect(afterResponse.status()).toBe(200)
  const afterSnapshot = await afterResponse.json() as Snapshot
  const afterTarget = afterSnapshot.images?.find(image => image.id === target.id)
  expect(afterTarget, 'the reviewed image must still exist after the read-only audit').toBeDefined()
  expect(imageSignature(afterTarget), 'version state, usedVersion, refs and prompt provenance must not change').toBe(baselineSignature)
  expect(afterSnapshot.items?.find(item => item.id === target.id)?.versions.find(version => version.version === target.usedVersion)?.originalUrl)
    .toBe(legacyTarget?.versions.find(version => version.version === target.usedVersion)?.originalUrl)
  const sourceAfterResponse = await page.request.get(base)
  expect(sourceAfterResponse.status()).toBe(200)
  const sourceAfter = await sourceAfterResponse.json() as { status: string; source?: { url?: string } }
  expect(sourceAfter.status, 'the review job status must not change').toBe(sourceBefore.status)
  expect(sourceAfter.source?.url ?? null, 'the review job source provenance must not be replaced').toBe(sourceBefore.source?.url ?? null)
  expect(writeGate.unexpected, 'this audit must attempt no write at all, including source or global routes').toEqual([])
  expect(writeGate.writes, 'the zero-write audit must not even attempt a disallowed API write').toEqual([])

  const versionState = new Map(target.versions.map(version => [version.n, version]))
  const rules: Array<{ test: (control: Control) => boolean; data: (control: Control) => Coverage }> = []
  const cover = (testRule: (control: Control) => boolean, data: (control: Control) => Coverage) => rules.push({ test: testRule, data })

  const versionCoverage = (control: Control): Coverage => {
    const n = Number(/^Bản (\d+)/.exec(control.name)?.[1] ?? 0)
    if (observedVersions[n]) {
      return {
        status: 'PASS',
        action: `Select version ${n} and compare its stage asset with the live §26 version`,
        expected: 'Pressed version tile and stage image identify the exact live version asset and natural size',
        observed: observedVersions[n],
        evidenceRef: `${prefix}review-control-ledger-version-preview.png + /api/clone/{job}/images readback`,
      }
    }
    const version = versionState.get(n)
    if (version?.state === 'cancelled') {
      return {
        status: 'NOT_APPLICABLE',
        action: 'Cancelled history stays visible but cannot be selected',
        expected: 'Disabled tile with an explicit cancelled-history reason; never used as a redraw source',
        observed: control.states.find(state => state.disabled) ?? null,
        evidenceRef: 'ImageRail.tsx cancelled version contract',
      }
    }
    if (version?.state === 'failed' || version?.state === 'drawing') {
      return {
        status: 'BLOCKED',
        action: `Select version ${n}`,
        expected: 'A settled ready asset is required before preview identity can be compared',
        observed: { state: version.state },
        evidenceRef: null,
        prerequisite: `Live version ${n} is ${version.state}; the normal-fixture audit does not fabricate a ready asset.`,
      }
    }
    return {
      status: 'NOT_RUN',
      action: `Select version ${n}`,
      expected: 'Exact live version asset in the stage',
      observed: null,
      evidenceRef: `${prefix}review-control-ledger-version-preview.png`,
      prerequisite: version ? 'Version was visible but not exercised in this viewport.' : 'No live version state matched the visible tile.',
    }
  }
  cover(control => control.scope === 'review-v36' && control.role === 'button' && /^Bản \d+/.test(control.name), versionCoverage)
  cover(control => control.role === 'button' && /^(Ảnh trước|Ảnh sau)$/.test(control.name), () => navigation ? {
    status: 'PASS',
    action: 'Click the strip navigation control, then return',
    expected: 'Selected strip option and img address change to the adjacent live image; no mutation',
    observed: navigation,
    evidenceRef: `${prefix}review-control-ledger-keyboard-navigation.png or ${prefix}review-control-ledger-default.png + hash/API readback`,
  } : {
    status: 'BLOCKED',
    action: 'Click the strip navigation control',
    expected: 'Adjacent live image opens without mutation',
    observed: null,
    evidenceRef: null,
    prerequisite: 'The live strip contains fewer than two images.',
  })
  cover(control => control.role === 'button' && /^(So với bản|Đang xem bản)/.test(control.name), () => compareObserved ? {
    status: 'PASS',
    action: 'Toggle compare, verify the previous version asset, toggle back; use V on desktop',
    expected: 'Compare shows version N-1 by URL (desktop alt names it, mobile stage tag names it); pressing V toggles the same state without changing usedVersion',
    observed: compareObserved,
    evidenceRef: `${prefix}review-control-ledger-compare.png + live version URL readback`,
  } : {
    status: 'BLOCKED',
    action: 'Toggle version comparison and the V shortcut',
    expected: 'Version N-1 preview is clearly identified',
    observed: null,
    evidenceRef: null,
    prerequisite: 'The live image has no pair of adjacent usable versions (N and N-1) in this viewport.',
  })
  cover(control => control.role === 'button' && /^(Chép|Đã chép)$/.test(control.name), () => clipboard ? {
    status: clipboard.status,
    action: 'Copy the displayed prompt and read the real clipboard back',
    expected: 'Clipboard equals the exact sentText of the displayed version; the button visibly confirms “Đã chép”',
    observed: { expectedLength: clipboard.expectedLength, actualLength: clipboard.actualLength, exact: clipboard.exact },
    evidenceRef: `${prefix}review-control-ledger-copied-prompt.png + navigator.clipboard readback`,
    prerequisite: clipboard.prerequisite,
  } : {
    status: 'NOT_RUN',
    action: 'Copy the displayed prompt and read the clipboard back',
    expected: 'Clipboard equals the displayed sentText',
    observed: null,
    evidenceRef: null,
  })
  cover(control => control.role === 'button' && control.name === 'Chép chỉ dẫn', () => clipboard ? {
    status: clipboard.status,
    action: 'Open Prompt đã gửi, copy and read the real clipboard back',
    expected: 'Clipboard equals the exact prompt shown by MReview',
    observed: { expectedLength: clipboard.expectedLength, actualLength: clipboard.actualLength, exact: clipboard.exact },
    evidenceRef: `${prefix}review-control-ledger-copied-prompt.png + navigator.clipboard readback`,
    prerequisite: clipboard.prerequisite,
  } : {
    status: 'NOT_RUN',
    action: 'Copy the mobile prompt and read the clipboard back',
    expected: 'Clipboard equals the mobile prompt',
    observed: null,
    evidenceRef: null,
  })
  cover(control => control.role === 'button' && /^(Ẩn thay đổi|Xem thay đổi)$/.test(control.name), () => diffObserved ? {
    status: 'PASS',
    action: 'Hide and restore the version diff',
    expected: 'ins/del annotations disappear then restore while the prompt remains readable',
    observed: diffObserved,
    evidenceRef: `${prefix}review-control-ledger-diff-hidden.png`,
  } : {
    status: 'BLOCKED',
    action: 'Hide and restore the version diff',
    expected: 'Diff annotations toggle without mutation',
    observed: null,
    evidenceRef: null,
    prerequisite: 'The selected live version does not expose diffFromPrev annotations.',
  })
  cover(control => control.role === 'button' && /^(Xem nguyên văn gửi máy chủ|Xem theo phần)$/.test(control.name), () => rawObserved ? {
    status: 'PASS',
    action: 'Open the raw sent prompt and return to parts',
    expected: 'Raw readback is byte-identical to the live sentText; no prompt is edited or submitted',
    observed: rawObserved,
    evidenceRef: `${prefix}review-control-ledger-raw.png`,
  } : {
    status: 'NOT_RUN',
    action: 'Open the raw sent prompt and return to parts',
    expected: 'Exact sentText readback',
    observed: null,
    evidenceRef: 'native92 desktop-prompt-raw capture',
  })
  cover(control => control.role === 'button' && control.name === 'Sửa', () => editObserved ? {
    status: 'PASS',
    action: 'Enter prompt editing, change and undo a part, remove a base reference, then cancel without submitting',
    expected: 'Changed marker and enabled save action appear; per-part undo restores exact bytes; cancel restores the draft and sends nothing',
    observed: editObserved,
    evidenceRef: `${prefix}review-control-ledger-prompt-edit.png + ${prefix}review-control-ledger-prompt-undo.png + imageSignature readback`,
  } : {
    status: 'BLOCKED',
    action: 'Enter prompt editing locally',
    expected: 'Editable scene and discard path are reachable',
    observed: null,
    evidenceRef: null,
    prerequisite: 'The live version has no editable parts or mutation is blocked by the current review state.',
  })
  cover(control => control.role === 'button' && control.name === 'Sửa prompt', () => stageEditObserved ? {
    status: 'PASS',
    action: 'Open prompt editing from the stage bar and cancel',
    expected: 'Scene draft opens and Huỷ sửa discards it without a write',
    observed: { opened: true, cancelled: true },
    evidenceRef: `${prefix}review-control-ledger-prompt-edit.png`,
  } : {
    status: 'BLOCKED',
    action: 'Open prompt editing from the stage bar',
    expected: 'Scene draft opens and can be cancelled',
    observed: null,
    evidenceRef: null,
    prerequisite: 'The stage edit action is disabled by legacy parts, drawing/pipeline state or offline mode.',
  })
  cover(control => control.role === 'button' && /^Huỷ sửa(?:\s|$)/.test(control.name), () => editObserved || stageEditObserved ? {
    status: 'PASS',
    action: 'Click the real Huỷ sửa button (with its Esc hint), not the Escape key',
    expected: 'The draft clears, the exact base reference set returns and the view prompt has no mutation',
    observed: editObserved ? { cancelled: editObserved.cancelled, refsRestored: editObserved.refsRestored } : { cancelled: true },
    evidenceRef: `${prefix}review-control-ledger-prompt-edit.png + ${prefix}review-control-ledger-prompt-undo.png`,
  } : {
    status: 'NOT_RUN',
    action: 'Cancel the local prompt draft with the real button',
    expected: 'Draft clears',
    observed: null,
    evidenceRef: null,
  })
  cover(control => control.role === 'button' && /^Vẽ lại với prompt này(?:\s|$)/.test(control.name), control => ({
    status: 'BLOCKED',
    action: 'Submit the edited prompt',
    expected: 'Would call the paid redraw endpoint for one image',
    observed: { visible: true, enabledLocally: control.states.some(state => state.disabled === false) },
    evidenceRef: `${replayCaseReceipt('03-prompt-edit-redraw')}; review-control-ledger-audit.pw.ts › [replay mutation] local prompt edit sends only the changed part, returns 202 and cancel restores versions and prompt bytes; viewer-audit paid-confirmation cancel`,
    prerequisite: 'Paid inference/success remains unexercised (2/2 QA images spent). The named 129 receipt proves only the real queue+exact cancel at zero image cost on A; provider completion is not claimed.',
  }))
  cover(control => control.role === 'button' && control.name === 'Hoàn tác', () => editObserved?.undo ? {
    status: 'PASS',
    action: 'Change the scene part, then click the real per-part Hoàn tác button',
    expected: 'The textarea returns to the exact source bytes and the local dirty marker clears',
    observed: editObserved.undo,
    evidenceRef: `${prefix}review-control-ledger-prompt-undo.png + textarea value readback`,
  } : {
    status: 'NOT_RUN',
    action: 'Reset the changed prompt part',
    expected: 'Locally restore the source text',
    observed: null,
    evidenceRef: null,
    prerequisite: 'The prompt draft was not editable in this viewport/state.',
  })
  cover(control => control.role === 'button' && control.name === 'Thêm', () => refsObserved ? {
    status: 'PASS',
    action: 'Open the reference picker',
    expected: 'Dialog opens with focus moved inside; closing returns focus to Thêm',
    observed: { opened: true, focusReturned: refsObserved.focusReturned, writes: 0 },
    evidenceRef: `${prefix}review-control-ledger-reference-sources.png`,
  } : {
    status: 'BLOCKED',
    action: 'Open the reference picker',
    expected: 'Native reference source/role dialog opens',
    observed: null,
    evidenceRef: null,
    prerequisite: 'Thêm is disabled by mutation/ref-limit state in the current fixture.',
  })
  for (const tab of ['Ảnh trong video', 'Khung video gốc', 'Tải lên']) {
    cover(control => control.role === 'radio' && control.name === tab, () => refsObserved?.tabs.includes(tab) ? {
      status: 'PASS',
      action: `Select reference source “${tab}”`,
      expected: 'Only that source panel is shown; no upload or reference mutation occurs',
      observed: { selected: true, tabs: refsObserved.tabs },
      evidenceRef: `${prefix}review-control-ledger-reference-sources.png`,
    } : {
      status: 'NOT_RUN',
      action: `Select reference source “${tab}”`,
      expected: 'Source panel changes',
      observed: null,
      evidenceRef: null,
    })
  }
  for (const roleName of ['Nhân vật', 'Bố cục', 'Phong cách', 'Đồ vật']) {
    cover(control => control.role === 'radio' && control.name === roleName, () => refsObserved?.roles.includes(roleName) ? {
      status: 'PASS',
      action: `Choose reference role “${roleName}”`,
      expected: 'aria-checked follows the chosen role; arrow keys move the checked chip and focus',
      observed: { checked: true, roles: refsObserved.roles },
      evidenceRef: `${prefix}review-control-ledger-reference-sources.png`,
    } : {
      status: 'NOT_RUN',
      action: `Choose reference role “${roleName}”`,
      expected: 'Role chip becomes checked',
      observed: null,
      evidenceRef: 'native92 reference-picker-labels-and-keyboard (labels only)',
    })
  }
  cover(control => control.role === 'button' && control.surfaces.some(surface => surface.startsWith('reference'))
    && candidateReadback !== null && candidateReadback.some(candidate => candidate.label === control.name), control => {
    const read = candidateReadback!.find(candidate => candidate.label === control.name)!
    return {
      status: 'PASS',
      action: 'Click this exact real reference candidate and read its pressed state',
      expected: 'Only this candidate becomes aria-pressed; the add action enables and cancelling sends no request',
      observed: { ...read, addEnabled: pickedCandidate !== null, cancelled: true },
      evidenceRef: `${prefix}review-control-ledger-reference-roles.png + review-control-ledger.json › safeInteractions.referenceCandidates`,
    }
  })
  cover(control => control.role === 'button' && control.name === 'Chọn ảnh để thêm', control => ({
    status: 'NOT_APPLICABLE',
    action: 'Add without a selected candidate',
    expected: 'Action is intentionally unavailable until a candidate is picked',
    observed: control.states.find(state => state.disabled) ?? null,
    evidenceRef: null,
  }))
  cover(control => control.role === 'button' && control.name === 'Thêm 1 ảnh', () => ({
    status: 'COVERED_BY_NAMED_TEST',
    action: 'Add the selected candidate to the draft',
    expected: 'Reference appears in the local draft and the dialog closes; no request is sent until redraw submit',
    observed: 'Existing unconditional desktop assertion stages Cảnh 5 and then cancels the draft.',
    evidenceRef: 'viewer-audit.pw.ts › review image address survives reload, navigation and cancelling paid feedback › review-staged-reference',
  }))
  cover(control => control.role === 'button' && control.name === 'Tải ảnh tham chiếu', () => referenceFileChooserCancelled ? ({
    status: 'PASS',
    action: 'Open the real file chooser and cancel without selecting bytes',
    expected: 'The upload button responds with a native file chooser; cancelling sends no upload',
    observed: { nativeFileChooserOpened: true, selectionCancelled: true, uploadWrites: 0 },
    evidenceRef: 'review-control-ledger.json › safeInteractions.references.fileChooserCancelled',
  }) : ({
    status: 'NOT_RUN',
    action: 'Open the native file chooser',
    expected: 'Would allow selecting a local reference image',
    observed: null,
    evidenceRef: null,
    prerequisite: 'The upload button was not exercised in this viewport.',
  }))
  cover(control => control.role === 'button' && control.name === 'Đóng', () => refsObserved ? {
    status: 'PASS',
    action: 'Close the reference dialog',
    expected: 'Dialog unmounts without a write',
    observed: { closed: true, writes: 0 },
    evidenceRef: `${prefix}review-control-ledger-reference-cancelled.png`,
  } : {
    status: 'NOT_RUN',
    action: 'Close the reference dialog',
    expected: 'Dialog unmounts',
    observed: null,
    evidenceRef: null,
  })
  cover(control => control.role === 'button' && control.name === 'Huỷ' && control.surfaces.some(surface => surface.startsWith('reference')), () => refsObserved ? {
    status: 'PASS',
    action: 'Cancel the reference dialog',
    expected: 'Dialog closes, focus returns to Thêm and no reference/upload request is sent',
    observed: { closed: true, focusReturned: refsObserved.focusReturned, writes: 0 },
    evidenceRef: `${prefix}review-control-ledger-reference-cancelled.png`,
  } : {
    status: 'NOT_RUN',
    action: 'Cancel the reference dialog',
    expected: 'Dialog closes without writes',
    observed: null,
    evidenceRef: null,
  })
  cover(control => control.role === 'button' && /^Vẽ lại cả \d+ ảnh$/.test(control.name), () => ({
    status: 'BLOCKED',
    action: 'Open the redraw-all confirmation and dismiss it',
    expected: 'Dialog cancel/Esc/scrim/close retain focus and send no request',
    observed: 'Named replay assertions cover scrim, cancel, Escape and close; the named 129 receipt proves the real 202 plus per-image cancels. Paid success remains unexercised.',
    evidenceRef: `runs/92-team-integrated-review-framed/review-state-replay-results.json › redraw-all-scrim / -cancel / -escape / -close; ${replayCaseReceipt('05-redraw-all')}; review-control-ledger-audit.pw.ts › [replay mutation] redraw-all`,
    prerequisite: 'Paid bulk inference/success remains unexercised (exhausted QA image budget). The named 129 receipt proves only the real queue+exact cancels on A at zero image cost; controller reseed is required.',
  }))
  cover(control => control.role === 'button' && /^Duyệt cả video & dựng/.test(control.name), () => ({
    status: 'BLOCKED',
    action: 'Approve the whole video and hand it to building',
    expected: 'Would persist approval and advance the original job',
    observed: { visible: true, receipt: '129 skip-approve retains the authoritative owner-approved + durable-intent wrappers; A ends queued pending (not finished) and B stays review' },
    evidenceRef: `${replayCaseReceipt('06-skip-approve')}; review-control-ledger-audit.pw.ts › [replay mutation] skip image then approve whole video (A/B only)`,
    prerequisite: 'Irreversible original action — BLOCKED in this zero-write audit. The named 129 receipt approves only controller-supplied A/B replicas after a skip and verifies A queued pending while B stays review; paid provider completion is not claimed and controller reseed is mandatory.',
  }))
  cover(control => control.role === 'button' && /^Bỏ ảnh tham chiếu \d+$/.test(control.name), control => {
    if (!editObserved?.baseRefRemove) {
      return {
        status: 'NOT_RUN',
        action: 'Remove a base reference from the local draft and cancel',
        expected: 'Local-only ref removal restores on cancel',
        observed: null,
        evidenceRef: null,
        prerequisite: 'The addressed version had no base reference or the editable draft was unavailable in this viewport.',
      }
    }
    if (control.name !== 'Bỏ ảnh tham chiếu 1') {
      return {
        status: 'NOT_RUN',
        action: 'Remove this base reference from the local draft and cancel',
        expected: 'Local-only ref removal restores on cancel',
        observed: null,
        evidenceRef: null,
        prerequisite: 'This audit removed only the first base reference; the remaining rows share the same local-draft removal path.',
      }
    }
    return {
      status: 'PASS',
      action: 'Remove the first base reference from the local prompt draft, then cancel and reopen the draft',
      expected: 'Only the local draft ref disappears; the real Huỷ sửa button discards the change and reopens the exact base reference set with zero writes',
      observed: { ...editObserved.baseRefRemove, restored: editObserved.refsRestored, cancelled: true },
      evidenceRef: `${prefix}review-control-ledger-prompt-undo.png + imageSignature readback`,
    }
  })
  cover(control => control.role === 'button' && control.name === 'Bỏ ảnh', () => ({
    status: 'BLOCKED',
    action: 'Skip this image for the video',
    expected: 'Would persist a skip on the review job',
    observed: { visible: true },
    evidenceRef: `${replayCaseReceipt('06-skip-approve')}; review-control-ledger-audit.pw.ts › [replay mutation] skip image then approve whole video (A/B only)`,
    prerequisite: 'Original/replay skip is BLOCKED in the zero-write audit. The named 129 receipt skips one scene on A, verifies the skipped status and keeps B untouched; controller reseed is mandatory.',
  }))
  cover(control => control.role === 'button' && /^Dùng bản \d+/.test(control.name), () => ({
    status: 'BLOCKED',
    action: 'Persist the selected version as usedVersion',
    expected: 'Would change the video’s usedVersion',
    observed: { visible: true },
    evidenceRef: `${replayCaseReceipt('01-use-version')}; review-control-ledger-audit.pw.ts › [replay mutation] useVersion 200/restore; viewer-audit two-version preview`,
    prerequisite: 'The normal audit does not change usedVersion. The named 129 receipt changes then restores A only (200/readback); original source and B remain untouched, and controller reseed is required.',
  }))
  cover(control => !mobile && control.role === 'button' && /^Vẽ lại(?:\s*R)?$/.test(control.name), () => ({
    status: 'BLOCKED',
    action: 'Open the paid redraw confirmation and cancel',
    expected: 'Cancel keeps the draft and sends no redraw',
    observed: 'Existing viewer-audit asserts the paid confirmation “Tốn 1 ảnh” and cancel sends no /redraw request. The named 129 receipt proves the real 202, pending state and exact cancel.',
    evidenceRef: `${replayCaseReceipt('02-feedback-redraw')}; review-control-ledger-audit.pw.ts › [replay mutation] plain feedback redraw; viewer-audit.pw.ts › review-paid-confirmation`,
    prerequisite: 'Paid inference/success remains unexercised (2/2 QA images spent). The named 129 receipt proves only a real queue+exact cancel on A at zero image cost; it never waits for provider success.',
  }))
  cover(control => control.role === 'button' && /^Vẽ lại \d+ ảnh · tốn \d+ ảnh$/.test(control.name), () => ({
    status: 'BLOCKED',
    action: 'Submit batch redraw',
    expected: 'Would spend one image per selected image',
    observed: 'Batch state was captured by native92. The named 129 receipt selects two A images and proves the real 202 plus both exact cancels at zero image cost.',
    evidenceRef: `runs/92-team-integrated-review-framed/review-state-replay-results.json › desktop-batch; ${replayCaseReceipt('04-multi-image-redraw')}; review-control-ledger-audit.pw.ts › [replay mutation] multi-image redraw`,
    prerequisite: 'Paid batch inference/success remains unexercised (exhausted QA image budget). The named 129 receipt proves only real queue+exact cancels on A; provider completion is not claimed and controller reseed is required.',
  }))
  cover(control => control.role === 'button' && control.name === 'Bỏ chọn', () => ({
    status: 'CAPTURED_ONLY',
    action: 'Clear the image selection',
    expected: 'Batch bar closes locally',
    observed: 'Native92 captured the batch bar; this audit did not select images.',
    evidenceRef: 'runs/92-team-integrated-review-framed/review-state-replay-results.json › desktop-batch (captured only)',
  }))
  cover(control => control.role === 'radio' && /^(Ảnh gốc|Tách nền)$/.test(control.name), () => ({
    status: 'COVERED_BY_NAMED_TEST',
    action: 'Switch the stage between the saved image and its cutout',
    expected: 'The stage src changes to the real cutout derivative and returns to the original',
    observed: 'Unconditional desktop assertion in the existing viewer audit.',
    evidenceRef: 'viewer-audit.pw.ts › review image address survives reload, navigation and cancelling paid feedback › review-real-cutout',
  }))
  cover(control => control.role === 'option' && stripReadback !== null && stripReadback.some(entry => entry.label === control.name), control => {
    const read = stripReadback!.find(entry => entry.label === control.name)!
    return {
      status: 'PASS',
      action: 'Click this exact image strip option and read the native address and stage identity',
      expected: 'The exact own image id is addressed, its stage image renders, and the original option is restored',
      observed: { ...read, restoredAddress: stripRestored },
      evidenceRef: `${prefix}review-control-ledger-default.png or ${prefix}review-control-ledger-strip-selected.png + hash/API readback`,
    }
  })
  cover(control => control.role === 'option', () => ({
    status: 'NOT_RUN',
    action: 'Select this image from the strip',
    expected: 'Image/version context changes without mutation',
    observed: null,
    evidenceRef: null,
    prerequisite: 'This option was not present during the zero-write strip iteration.',
  }))
  cover(control => control.role === 'button' && /^QA replay [AB](?: ·|$)/.test(control.name), control => {
    const side = control.name.startsWith('QA replay A') ? 'a' : 'b'
    const observed = jobReadback?.[side]
    return observed ? {
      status: 'PASS',
      action: `Select review video ${side.toUpperCase()} in the real sidebar, read its exact job identity, then restore the original`,
      expected: 'The exact own fixture job id and heading load; restoring returns to the original job and image without a write',
      observed: { ...observed, restored: jobReadback?.restored },
      evidenceRef: 'review-control-ledger.json › safeInteractions.videoSelections + /api/clone?channel=all title/id mapping',
    } : {
      status: 'BLOCKED',
      action: `Select review video ${side.toUpperCase()} in the sidebar`,
      expected: 'Exact own job id and heading',
      observed: null,
      evidenceRef: null,
      prerequisite: 'The live review sidebar did not expose this QA replay job as a real button in this run.',
    }
  })
  cover(control => control.role === 'button' && /^(QA replay|Bản ghi)/.test(control.name), () => ({
    status: 'NOT_RUN',
    action: 'Select this review job',
    expected: 'The review context would switch to another replay job',
    observed: null,
    evidenceRef: null,
    prerequisite: 'Only the addressed normal review job and the named A/B replay jobs are in this audit scope.',
  }))
  cover(control => control.role === 'link' && control.name === 'Mở Bàn dựng', () => ({
    status: 'NOT_RUN',
    action: 'Navigate to the workbench',
    expected: 'Leaves the Review screen for the main workbench',
    observed: null,
    evidenceRef: null,
    prerequisite: 'Navigation-only exit is outside this Review control audit.',
    followUpOwner: 'PR3',
  }))
  cover(control => control.role === 'button' && /^Vì sao\??$/.test(control.name), () => ({
    status: 'NOT_RUN',
    action: 'Open the deprecated score explanation',
    expected: 'Score evidence would be shown; this audit deliberately does not click it',
    observed: null,
    evidenceRef: 'viewer-audit.pw.ts › real score explanation … (requires WORKFLOW_QA_TWO_VERSIONS=1)',
    prerequisite: 'Deprecated Vì sao surface is owned by PR3 (V37 removal) and PR6 (V40 mobile parity); the zero-write audit leaves it unclicked.',
    followUpOwner: 'PR3 / PR6',
  }))
  cover(control => control.role === 'button' && /^Xem [Cc]ảnh/.test(control.name), () => ({
    status: 'NOT_RUN',
    action: 'Jump to the low-score image',
    expected: 'Another image is selected without mutation',
    observed: null,
    evidenceRef: 'viewer-audit.pw.ts › real score explanation … (requires WORKFLOW_QA_TWO_VERSIONS=1)',
    prerequisite: 'The score-banner jump needs a real low-score image; this zero-write audit does not click it.',
    followUpOwner: 'PR3',
  }))
  cover(control => control.role === 'button' && control.name === 'Xem prompt đã gửi', () => clipboard ? {
    status: 'PASS',
    action: 'Open the mobile sent-prompt sheet',
    expected: 'Sheet shows the prompt and its copy action; closing sends nothing',
    observed: { opened: true, copyStatus: clipboard.status },
    evidenceRef: `${prefix}review-control-ledger-copied-prompt.png`,
  } : {
    status: 'NOT_RUN',
    action: 'Open the mobile sent-prompt sheet',
    expected: 'Prompt sheet opens',
    observed: null,
    evidenceRef: null,
  })
  cover(control => control.role === 'button' && control.name === 'Sửa chỉ dẫn rồi vẽ lại', () => ({
    status: 'NOT_RUN',
    action: 'Open the mobile edit sheet',
    expected: 'Draft equals the selected version prompt',
    observed: null,
    evidenceRef: 'review-acceptance.md › M1 (mobile edit draft reset)',
    followUpOwner: 'PR6',
    prerequisite: 'Known PR6 mobile edit-draft gap is not claimed by this PR2 audit.',
  }))
  cover(control => control.role === 'button' && /^Vẽ lại$/.test(control.name) && mobile, () => ({
    status: 'NOT_RUN',
    action: 'Open the mobile feedback sheet',
    expected: 'Feedback sheet opens without submitting',
    observed: null,
    evidenceRef: null,
    prerequisite: 'This audit covers copy/compare/version controls on mobile; the paid feedback submit stays blocked.',
  }))
  cover(control => control.role === 'button' && control.name === 'Video sau', () => ({
    status: 'NOT_RUN',
    action: 'Move to the next review video',
    expected: 'Review context changes to the next job',
    observed: null,
    evidenceRef: null,
    prerequisite: 'Job navigation is outside the addressed-image control audit.',
  }))

  const rows: LedgerRow[] = []
  for (const control of controls.values()) {
    const global = control.scope === 'global-navigation'
    const learn = control.scope === 'review-v36' && isLearnControl(control.name)
    const coverage = rules.find(rule => rule.test(control))?.data(control)
    const owner = global ? 'PR3' : learn ? 'PR3' : 'PR2'
    const laterOwner = global ? 'PR3' : learn ? 'PR3' : null
    rows.push({
      ...control,
      owner,
      laterOwner,
      targetOwner: global ? targetOwner(control.name) : null,
      feature: global ? `shared header / global navigation${mobile ? ' (mobile final PR6)' : ''}` : learn ? 'review learn banner removed by V37' : undefined,
      state: control.states[0],
      enabled: !control.states[0].disabled,
      status: coverage?.status ?? (learn ? 'NOT_RUN' : global ? 'NOT_RUN' : 'NOT_RUN'),
      action: coverage?.action ?? (global
        ? 'Navigation/header control is not a V36 Review acceptance target.'
        : learn
          ? 'Learn-banner action is removed by the next PR; no V36 acceptance claim.'
          : 'Observe only; no safe read-only coverage was assigned in this audit.'),
      expected: coverage?.expected ?? (global
        ? `Owned by ${laterOwner}; target ${targetOwner(control.name)}. No V36 claim.`
        : learn
          ? 'Owned by PR3, which removes the learn UI.'
          : 'No acceptance claim; an owner must assign a consumer interaction check.'),
      observed: coverage?.observed ?? null,
      evidenceRef: coverage?.evidenceRef ?? (global ? 'phase-04-v37-compact-studio.md › AppShell/MainNav/AppHeader' : learn ? 'phase-04-v37-compact-studio.md step 4' : null),
      prerequisite: coverage?.prerequisite,
      followUpOwner: coverage?.followUpOwner,
    })
  }

  if (mobile) {
    rows.push({
      scope: 'review-v36',
      role: 'button',
      name: 'Thêm ảnh tham chiếu',
      tag: 'absent',
      surfaces: ['mobile-review'],
      states: [],
      state: { disabled: true, disabledReason: 'MReview has no reference source picker', pressed: null, checked: null, selected: null, expanded: null },
      enabled: false,
      owner: 'PR6',
      laterOwner: 'PR6',
      targetOwner: null,
      feature: 'mobile reference-source parity',
      status: 'BLOCKED',
      action: 'Open the reference source picker, choose a role and cancel',
      expected: 'Same source tabs/role radios as desktop, zero mutation on cancel',
      observed: 'No reference-source control exists in the current MReview surface.',
      evidenceRef: 'review-acceptance.md › M3',
      prerequisite: 'MReview reference strip/source picker is a PR6-owned parity gap (M3); this audit does not synthesize it.',
    })
  }

  const unnamed = rows.filter(row => row.scope === 'review-v36' && row.name === '(unnamed)').map(row => ({ role: row.role, surfaces: row.surfaces }))
  expect(unnamed, 'every visible Review control must expose an accessible name').toEqual([])
  expect(rows.filter(row => row.scope === 'review-v36').length).toBeGreaterThan(10)
  expect(rows.filter(row => row.scope === 'global-navigation').length).toBeGreaterThan(3)
  for (const row of rows) {
    expect(row.owner, `${row.name} needs an owner`).toMatch(/^PR[2-6]$/)
    if (row.scope === 'global-navigation') expect(row.laterOwner, `${row.name} needs an explicit later owner`).toBeTruthy()
    expect(row.action, `${row.name} needs an action`).toBeTruthy()
    expect(row.expected, `${row.name} needs an expected result`).toBeTruthy()
    if (row.status === 'PASS') {
      expect(row.observed, `${row.name} PASS needs observed evidence`).not.toBeNull()
      expect(row.evidenceRef, `${row.name} PASS needs an evidence reference`).toBeTruthy()
    }
    if (row.status === 'BLOCKED') expect(row.prerequisite, `${row.name} BLOCKED needs a real prerequisite`).toBeTruthy()
    if (row.status === 'COVERED_BY_NAMED_TEST' || row.status === 'CAPTURED_ONLY') {
      expect(row.evidenceRef, `${row.name} needs the named evidence reference`).toBeTruthy()
    }
  }

  const counts = rows.reduce<Record<string, number>>((acc, row) => {
    acc[row.status] = (acc[row.status] ?? 0) + 1
    return acc
  }, {})
  const learnPresent = [...controls.values()].some(control => control.scope === 'review-v36' && isLearnControl(control.name))
  await info.attach('review-control-ledger.json', {
    body: JSON.stringify({
      schema: 'viderse-review-control-ledger/v1',
      surface: `${mobile ? 'mobile' : 'desktop'}-${page.viewportSize()!.width}x${page.viewportSize()!.height}`,
      project: info.project.name,
      viewport: page.viewportSize(),
      job: { id: job.id, projectId: job.projectId },
      image: { id: target.id, label: target.label, usedVersion: target.usedVersion, versions: target.versions.map(version => ({ n: version.n, state: version.state })) },
      removedLaterFeatures: [
        { feature: 'round45 shared mobile image viewer', owner: 'round45-excluded', present: mobile ? false : 'not-applicable', observed: mobile ? 'no dialog/viewer surface on the normal review route' : 'desktop route is inline' },
        { feature: 'review learn banner actions', owner: 'PR3', present: learnPresent, evidence: 'phase-04-v37-compact-studio.md step 4' },
      ],
      existingEvidence: [
        { ref: 'viewer-audit.pw.ts › review image address survives reload, navigation and cancelling paid feedback', covers: ['Ảnh gốc', 'Tách nền', 'staged reference add', 'paid redraw confirmation cancel'] },
        { ref: 'runs/92-team-integrated-review-framed/review-state-replay-results.json', covers: ['redraw-all scrim/cancel/Escape/close assertions', 'prompt/raw/refs/cutout/batch/confirm captures', 'reference labels and keyboard'] },
        { ref: 'reports/pr-02/review-acceptance.md', covers: ['MReview.prompt clipboard equals sentText', 'M1 mobile draft reset (PR6)', 'M3 mobile reference parity (PR6)'] },
        { ref: 'review-control-ledger-audit.pw.ts › Replay mutation acceptance (controller-owned A/B checkpoints)', covers: ['useVersion 200 then restore', 'plain/local-edit/multi/redraw-all real 202 + exact cancel cost 0', 'skip then approveAll 202, next video B, A queued pending'], gate: 'WORKFLOW_QA_REPLAY_MUTATIONS=1 + WORKFLOW_QA_WORKER_STANDBY=1 + exact A/B IDs; desktop normal/reduced; controller restoration required per invocation' },
        { ref: 'runs/129-final-app-skin-actions/mutation-matrix-results.json + desktop/<case>/owned-replay-mutation-restoration.json', covers: ['01-use-version', '02-feedback-redraw', '03-prompt-edit-redraw', '04-multi-image-redraw', '05-redraw-all', '06-skip-approve'], receipt: 'queue/cancel/use/skip/approve proof only; paid provider completion unexercised, providerCallsAuthorized 0, replayRestored true' },
      ],
      safeInteractions: {
        navigation,
        videoSelections: jobReadback,
        stripSelections: stripReadback,
        stripRestored,
        observedVersions,
        compare: compareObserved,
        clipboard,
        diff: diffObserved,
        raw: rawObserved,
        promptEdit: editObserved,
        stageEdit: stageEditObserved,
        references: refsObserved,
        referenceCandidates: candidateReadback,
      },
      readback: {
        attemptedWrites: writeGate.writes,
        abortedWrites: writeGate.unexpected,
        imageSnapshotUnchanged: true,
        sourceStatusBefore: sourceBefore.status,
        sourceStatusAfter: sourceAfter.status,
        sourceUrlUnchanged: true,
      },
      counts,
      controls: rows,
    }, null, 2),
    contentType: 'application/json',
  })
  await capture(page, info, `${prefix}review-control-ledger-final`)
})

test('desktop reference add remove and repeat keeps a zero-change draft without writes', async ({ page }, info) => {
  test.skip(info.project.name.startsWith('mobile'), 'Reference picker parity belongs to PR6, not the round45 viewer')
  const job = fixture.jobs[0]
  expect(job, 'Owned replay is required; absence is not a passing smoke').toBeDefined()
  const before = await readImageSnapshot(page, job.id)
  const target = (before.images ?? []).find(image => image.kind === 'scene' &&
    image.versions.some(version => publishedVersion(version) && version.n === image.usedVersion && version.refs.length < 3))
  expect(target, 'A real ready scene with room for two local add attempts is required').toBeDefined()
  const gate = await guardWrites(page, () => false)
  expect(await openReview(page, `/#/review?video=${encodeURIComponent(job.id)}&img=${encodeURIComponent(target!.id)}`)).not.toBe('blocked')
  const rail = page.locator('aside[aria-label="Ảnh và chỉ dẫn"]')
  await expect(rail.getByRole('button', { name: 'Thêm', exact: true })).toBeEnabled()
  const remove = rail.getByRole('button', { name: /^Bỏ ảnh tham chiếu / })
  const originalCount = await remove.count()
  let candidateName = ''
  const addCandidate = async () => {
    await rail.getByRole('button', { name: 'Thêm', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Thêm ảnh tham chiếu', exact: true })
    await expect(dialog).toBeVisible()
    await dialog.getByRole('radio', { name: 'Ảnh trong video', exact: true }).click()
    const candidate = candidateName
      ? dialog.getByRole('button', { name: candidateName, exact: true })
      : dialog.locator('button[aria-pressed]').first()
    await expect(candidate).toBeVisible()
    candidateName = (await candidate.getAttribute('aria-label'))!
    expect(candidateName).toBeTruthy()
    await candidate.click()
    await dialog.getByRole('button', { name: 'Thêm 1 ảnh', exact: true }).click()
    await expect(dialog).toHaveCount(0)
  }
  await addCandidate()
  await expect(remove).toHaveCount(originalCount + 1)
  await addCandidate()
  await expect(remove, 'Adding the same candidate twice must not duplicate it').toHaveCount(originalCount + 1)
  await capture(page, info, 'reference-added-once')
  await remove.last().click()
  await expect(remove, 'Removing a new reference cancels that add').toHaveCount(originalCount)
  const submit = page.getByRole('button', { name: /^Vẽ lại với prompt này(?:\s|$)/ })
  await expect(submit, 'Add then remove must not leave a dirty change').toHaveAttribute('aria-disabled', 'true')
  await expect(page.getByText(/^Đã sửa \d+ chỗ trong prompt/)).toHaveCount(0)
  await capture(page, info, 'reference-add-cancelled')
  await page.getByRole('button', { name: /^Huỷ sửa(?:\s|$)/ }).click()
  expectSnapshotUnchanged(before, await readImageSnapshot(page, job.id))
  expect(gate.writes, 'Draft-only reference changes must never write to the API').toEqual([])
  await info.attach('reference-add-remove-readback.json', {
    body: JSON.stringify({ sourceRevision: process.env.WORKFLOW_SOURCE_REVISION, candidateName, originalCount, duplicatePrevented: true, addRemoved: true, zeroDirtyAfterRemoval: true, snapshotUnchanged: true, writes: gate.writes }),
    contentType: 'application/json',
  })
})

// ---------------------------------------------------------------------------
// Controller-owned A/B replay mutation acceptance.
//
// These checks intentionally exercise the real queue/cancel version paths,
// which the normal zero-write audit must never do. They are skipped unless the
// controller explicitly supplies a standby worker and two exact fixture job
// IDs. Every reversible action is cancelled back to the checkpoint; the final
// skip/approve test is intentionally irreversible and therefore requires the
// controller to reseed its A/B checkpoint before any later suite run.
// ---------------------------------------------------------------------------
test.describe('Replay mutation acceptance (controller-owned A/B checkpoints)', () => {
  test.describe.configure({ mode: 'serial' })

  test('useVersion persists the selected image version, returns 200 and restores the original usedVersion', async ({ page }, info) => {
    const gate = requireReplayGate(info)
    const baseline = await ensureReplayBaseline(page, gate)
    const target = (baseline.a.images ?? []).find(image => {
      const ready = image.versions.filter(publishedVersion)
      return image.usedVersion > 0 && ready.some(version => version.n === image.usedVersion) && ready.some(version => version.n !== image.usedVersion)
    })
    test.skip(!target, 'The A checkpoint needs one image with an original usedVersion and another ready version for this reversible assertion')
    if (!target) return
    const original = target.versions.find(version => version.n === target.usedVersion && publishedVersion(version))
    const alternative = target.versions
      .filter(version => publishedVersion(version) && version.n !== target.usedVersion)
      .sort((a, b) => a.n - b.n)
      .at(-1)
    test.skip(!original || !alternative, 'The original used version or its alternative must still be a real ready asset')
    if (!original || !alternative) return

    const opened = await openReadyImage(page, gate.a, target.id, target.label)
    test.skip(!opened, 'Native setup escape could not open the A review job')
    if (!opened) return

    const base = `/api/clone/${encodeURIComponent(gate.a)}`
    const imagePath = `${base}/images/${encodeURIComponent(target.id)}`
    const allowedUseVersions = new Set([alternative.n, original.n])
    const writeGate = await guardWrites(page, write => write.method === 'POST' &&
      write.path === `${imagePath}/use` && allowedUseVersions.has((write.body as { n?: number } | null)?.n ?? -1))
    const jobBefore = await readCloneJob(page, gate.a)
    let changed = false
    let restored = false
    let usedResponseStatus: number | null = null
    let restoreResponseStatus: number | null = null

    try {
      await versionTile(page, alternative.n).click()
      const useButton = page.getByRole('button', { name: `Dùng bản ${alternative.n} cho video`, exact: true })
      await expect(useButton).toBeVisible()
      await expect(useButton).toBeEnabled()
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST' && new URL(candidate.url()).pathname === `${imagePath}/use`),
        useButton.click(),
      ])
      usedResponseStatus = response.status()
      if (response.status() === 200) changed = true
      expect(response.status(), `Dùng bản must return 200, got ${response.status()}`).toBe(200)
      const body = await response.json() as { imageId: string; n: number; usedVersion: number }
      expect(body.imageId).toBe(target.id)
      expect(body.n).toBe(alternative.n)
      expect(body.usedVersion).toBe(alternative.n)
      await expect.poll(async () => (await readImageSnapshot(page, gate.a)).images?.find(image => image.id === target.id)?.usedVersion).toBe(alternative.n)

      await versionTile(page, original.n).click()
      const restoreButton = page.getByRole('button', { name: `Dùng bản ${original.n} cho video`, exact: true })
      await expect(restoreButton).toBeVisible()
      const [restoreResponse] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST' && new URL(candidate.url()).pathname === `${imagePath}/use`),
        restoreButton.click(),
      ])
      restoreResponseStatus = restoreResponse.status()
      expect(restoreResponse.status(), 'restoring the original usedVersion must return 200').toBe(200)
      const restoreBody = await restoreResponse.json() as { usedVersion: number }
      expect(restoreBody.usedVersion).toBe(original.n)
      await expect.poll(async () => (await readImageSnapshot(page, gate.a)).images?.find(image => image.id === target.id)?.usedVersion).toBe(original.n)
      restored = true

      const after = await readImageSnapshot(page, gate.a)
      expectSnapshotUnchanged(baseline.a, after)
      expect((await readCloneJob(page, gate.a)).source?.url ?? null, 'useVersion must never replace the original source').toBe(jobBefore.source?.url ?? null)
      expect(writeGate.unexpected, 'only the exact target image /use route may be written').toEqual([])
      await info.attach('replay-mutation-use-version.json', {
        body: JSON.stringify({
          gate,
          target: { id: target.id, label: target.label, originalVersion: original.n, alternativeVersion: alternative.n },
          responseStatus: usedResponseStatus,
          restoreResponseStatus,
          restoredUsedVersion: original.n,
          restored,
          writes: writeGate.writes,
          sourceUrlUnchanged: true,
        }, null, 2),
        contentType: 'application/json',
      })
    } finally {
      if (changed && !restored) {
        const recovery = await browserPost(page, `${imagePath}/use`, { n: original.n })
        if (recovery.status !== 200) throw new Error(`Could not restore usedVersion ${original.n}: ${JSON.stringify(recovery.body)}`)
        const after = await readImageSnapshot(page, gate.a)
        expect(after.images?.find(image => image.id === target.id)?.usedVersion, 'failure cleanup must restore the original usedVersion').toBe(original.n)
      }
    }
  })

  test('plain feedback redraw returns 202, really queues the target, then the exact cancel restores the checkpoint at zero image cost', async ({ page }, info) => {
    const gate = requireReplayGate(info)
    const baseline = await ensureReplayBaseline(page, gate)
    const target = (baseline.a.images ?? []).find(image => image.versions.some(publishedVersion))
    test.skip(!target, 'The A checkpoint has no published ready image for a redraw queue assertion')
    if (!target) return

    const opened = await openReadyImage(page, gate.a, target.id, target.label)
    test.skip(!opened, 'Native setup escape could not open the A review job')
    if (!opened) return

    const base = `/api/clone/${encodeURIComponent(gate.a)}`
    const imagePath = `${base}/images/${encodeURIComponent(target.id)}`
    const cancelPath = `${imagePath}/versions/${nextVersion(target)}/cancel`
    const writeGate = await guardWrites(page, write => {
      if (write.method !== 'POST') return false
      if (write.path === `${base}/images/redraw`) {
        const ids = (write.body as { imageIds?: string[] } | null)?.imageIds
        return Array.isArray(ids) && ids.length === 1 && ids[0] === target.id
      }
      return write.path === cancelPath
    })
    const feedback = 'QA replay queue audit: giữ nguyên bố cục và ánh sáng'
    const targetBefore = baseline.a.images?.find(image => image.id === target.id)
    expect(targetBefore).toBeDefined()
    const jobBefore = await readCloneJob(page, gate.a)
    const plannedFallback = { imageId: target.id, n: nextVersion(target) }
    let planned: { imageId: string; n: number } | null = null
    let queued = false
    let redrawStatus: number | null = null
    let cancelBody: unknown = null

    try {
      const input = page.locator('#redraw')
      await expect(input).toBeVisible()
      await input.fill(feedback)
      const redrawButton = page.locator('main:visible').getByRole('button', { name: /^Vẽ lại(?:\s*R)?$/ })
      await redrawButton.click()
      const dialog = page.getByRole('alertdialog')
      await expect(dialog).toBeVisible()
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST' && new URL(candidate.url()).pathname === `${base}/images/redraw`),
        dialog.getByRole('button', { name: 'Vẽ lại', exact: true }).click(),
      ])
      redrawStatus = response.status()
      expect(response.status(), `plain redraw must return the real 202, got ${response.status()}`).toBe(202)
      queued = true
      const requestBody = response.request().postDataJSON() as { imageIds?: string[]; feedback?: string }
      expect(requestBody.imageIds).toEqual([target.id])
      expect(requestBody.feedback).toBe(feedback)
      const body = await response.json() as { versions?: Array<{ imageId: string; n: number }> }
      expect(body.versions?.length, 'one feedback redraw must plan exactly one version').toBe(1)
      planned = body.versions?.[0] ?? plannedFallback
      expect(planned.imageId).toBe(target.id)

      const mid = await readImageSnapshot(page, gate.a)
      expect(mid.items?.find(item => item.id === target.id)?.status, 'the queued redraw must be visible as pending before cancel').toBe('pending')

      const toCancel = planned ?? plannedFallback
      planned = null
      queued = false
      cancelBody = await cancelPlanned(page, gate.a, toCancel.imageId, toCancel.n, targetBefore!.usedVersion)

      const after = await readImageSnapshot(page, gate.a)
      expectSnapshotUnchanged(baseline.a, after)
      const jobAfter = await readCloneJob(page, gate.a)
      expect(jobAfter.source?.url ?? null, 'redraw/cancel must never replace the source job provenance').toBe(jobBefore.source?.url ?? null)
      expect(writeGate.unexpected, 'only the exact redraw route and its cancel may be written').toEqual([])
      await info.attach('replay-mutation-plain-redraw.json', {
        body: JSON.stringify({
          gate,
          target: target.id,
          feedback,
          redrawStatus,
          planned: toCancel,
          cancel: cancelBody,
          restored: true,
          writes: writeGate.writes,
          sourceUrlUnchanged: true,
        }, null, 2),
        contentType: 'application/json',
      })
    } finally {
      if (queued) await cancelPlanned(page, gate.a, (planned ?? plannedFallback).imageId, (planned ?? plannedFallback).n, targetBefore!.usedVersion)
    }
  })

  test('local prompt edit sends only the changed part, returns 202 and cancel restores versions and prompt bytes', async ({ page }, info) => {
    const gate = requireReplayGate(info)
    const baseline = await ensureReplayBaseline(page, gate)
    const target = (baseline.a.images ?? []).find(image => {
      const latest = image.versions.filter(publishedVersion).sort((a, b) => a.n - b.n).at(-1)
      return !!latest && !latest.partsLegacy && latest.parts.some(part => part.kind === 'scene')
    })
    test.skip(!target, 'The A checkpoint needs a latest ready version with a saved scene prompt part for an edit-redraw assertion')
    if (!target) return
    const latest = target.versions.filter(publishedVersion).sort((a, b) => a.n - b.n).at(-1)!
    const originalText = latest.parts.find(part => part.kind === 'scene')?.text ?? ''

    const opened = await openReadyImage(page, gate.a, target.id, target.label)
    test.skip(!opened, 'Native setup escape could not open the A review job')
    if (!opened) return

    const base = `/api/clone/${encodeURIComponent(gate.a)}`
    const imagePath = `${base}/images/${encodeURIComponent(target.id)}`
    const cancelPath = `${imagePath}/versions/${nextVersion(target)}/cancel`
    const writeGate = await guardWrites(page, write => {
      if (write.method !== 'POST') return false
      if (write.path === `${base}/images/redraw`) {
        const ids = (write.body as { imageIds?: string[] } | null)?.imageIds
        return Array.isArray(ids) && ids.length === 1 && ids[0] === target.id
      }
      return write.path === cancelPath
    })
    const targetBefore = baseline.a.images?.find(image => image.id === target.id)
    expect(targetBefore).toBeDefined()
    const plannedFallback = { imageId: target.id, n: nextVersion(target) }
    let planned: { imageId: string; n: number } | null = null
    let queued = false
    let redrawStatus: number | null = null
    let cancelBody: unknown = null

    try {
      await versionTile(page, latest.n).click()
      const rail = page.locator('aside[aria-label="Ảnh và chỉ dẫn"]')
      await rail.getByRole('button', { name: 'Sửa', exact: true }).click()
      const scene = rail.locator('#prompt-scene')
      await expect(scene).toBeVisible()
      const newText = `${originalText}${originalText ? ' ' : ''}[QA replay edit readback]`
      await scene.fill(newText)
      const save = page.locator('main:visible').getByRole('button', { name: /^Vẽ lại với prompt này/ })
      await expect(save).not.toHaveAttribute('aria-disabled', 'true')
      await save.click()
      const dialog = page.getByRole('alertdialog')
      await expect(dialog).toBeVisible()
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST' && new URL(candidate.url()).pathname === `${base}/images/redraw`),
        dialog.getByRole('button', { name: 'Vẽ lại', exact: true }).click(),
      ])
      redrawStatus = response.status()
      expect(response.status(), `edited-prompt redraw must return the real 202, got ${response.status()}`).toBe(202)
      queued = true
      const requestBody = response.request().postDataJSON() as { imageIds?: string[]; baseVersion?: number; parts?: Record<string, string> }
      expect(requestBody.imageIds).toEqual([target.id])
      expect(requestBody.baseVersion).toBe(latest.n)
      expect(requestBody.parts, 'the redraw body must carry only the edited scene part').toEqual({ scene: newText })
      const body = await response.json() as { versions?: Array<{ imageId: string; n: number }> }
      expect(body.versions?.length).toBe(1)
      planned = body.versions?.[0] ?? plannedFallback

      const mid = await readImageSnapshot(page, gate.a)
      expect(mid.items?.find(item => item.id === target.id)?.status).toBe('pending')

      const toCancel = planned ?? plannedFallback
      planned = null
      queued = false
      cancelBody = await cancelPlanned(page, gate.a, toCancel.imageId, toCancel.n, targetBefore!.usedVersion)
      const after = await readImageSnapshot(page, gate.a)
      expectSnapshotUnchanged(baseline.a, after)
      const afterTarget = after.images?.find(image => image.id === target.id)
      expect(afterTarget?.versions.find(version => version.n === latest.n)?.sentText, 'cancel must restore the exact sent prompt').toBe(latest.sentText)
      expect(writeGate.unexpected, 'the local edit path must not write any route except redraw and cancel').toEqual([])
      await info.attach('replay-mutation-edit-redraw.json', {
        body: JSON.stringify({
          gate,
          target: target.id,
          baseVersion: latest.n,
          editedPart: 'scene',
          redrawStatus,
          planned: toCancel,
          cancel: cancelBody,
          restored: true,
          writes: writeGate.writes,
        }, null, 2),
        contentType: 'application/json',
      })
    } finally {
      if (queued) await cancelPlanned(page, gate.a, (planned ?? plannedFallback).imageId, (planned ?? plannedFallback).n, targetBefore!.usedVersion)
    }
  })

  test('multi-image redraw returns 202 for both images and both exact cancels restore every checkpoint field', async ({ page }, info) => {
    const gate = requireReplayGate(info)
    const baseline = await ensureReplayBaseline(page, gate)
    const candidates = (baseline.a.images ?? []).filter(image => image.versions.some(publishedVersion))
    const targets = candidates.filter((image, index) => candidates.findIndex(candidate => candidate.label === image.label) === index).slice(0, 2)
    test.skip(targets.length < 2, 'The A checkpoint needs two ready images with distinct labels for a multi-image redraw assertion')
    if (targets.length < 2) return

    const opened = await openReadyImage(page, gate.a, targets[0].id, targets[0].label)
    test.skip(!opened, 'Native setup escape could not open the A review job')
    if (!opened) return

    const base = `/api/clone/${encodeURIComponent(gate.a)}`
    const targetIds = targets.map(target => target.id)
    const cancelPaths = targets.map(target => `${base}/images/${encodeURIComponent(target.id)}/versions/${nextVersion(target)}/cancel`)
    const writeGate = await guardWrites(page, write => {
      if (write.method !== 'POST') return false
      if (write.path === `${base}/images/redraw`) {
        const ids = (write.body as { imageIds?: string[] } | null)?.imageIds
        return Array.isArray(ids) && ids.length === targetIds.length && targetIds.every(id => ids.includes(id))
      }
      return cancelPaths.includes(write.path)
    })
    const usedVersionById = new Map(targets.map(target => [target.id, target.usedVersion]))
    const remaining = new Map<string, number>(targets.map(target => [target.id, nextVersion(target)]))
    let queued = false
    let redrawStatus: number | null = null
    const cancelBodies: unknown[] = []

    try {
      const list = page.getByRole('listbox', { name: 'Ảnh của video', exact: true })
      for (const target of targets) {
        await list.getByRole('option', { name: target.label, exact: true }).click({ modifiers: ['Shift'] })
      }
      const batchButton = page.getByRole('button', { name: /^Vẽ lại 2 ảnh · tốn 2 ảnh$/ })
      await expect(batchButton).toBeVisible()
      await expect(batchButton).toBeEnabled()
      await batchButton.click()
      const dialog = page.getByRole('alertdialog')
      await expect(dialog).toBeVisible()
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST' && new URL(candidate.url()).pathname === `${base}/images/redraw`),
        dialog.getByRole('button', { name: 'Vẽ lại', exact: true }).click(),
      ])
      redrawStatus = response.status()
      expect(response.status(), `multi-image redraw must return the real 202, got ${response.status()}`).toBe(202)
      queued = true
      const requestBody = response.request().postDataJSON() as { imageIds?: string[] }
      expect([...(requestBody.imageIds ?? [])].sort()).toEqual([...targetIds].sort())
      const body = await response.json() as { versions?: Array<{ imageId: string; n: number }> }
      expect(body.versions?.length, 'two selected images must plan two versions').toBe(2)
      expect([...(body.versions ?? []).map(version => version.imageId)].sort()).toEqual([...targetIds].sort())
      for (const version of body.versions ?? []) remaining.set(version.imageId, version.n)

      const mid = await readImageSnapshot(page, gate.a)
      for (const imageId of targetIds) expect(mid.items?.find(item => item.id === imageId)?.status).toBe('pending')

      for (const [imageId, n] of [...remaining]) {
        const usedVersion = usedVersionById.get(imageId)
        expect(usedVersion).toBeDefined()
        cancelBodies.push(await cancelPlanned(page, gate.a, imageId, n, usedVersion!))
        remaining.delete(imageId)
      }
      const after = await readImageSnapshot(page, gate.a)
      expectSnapshotUnchanged(baseline.a, after)
      expect(writeGate.unexpected, 'multi-image redraw may only write redraw and the two exact cancels').toEqual([])
      await info.attach('replay-mutation-multi-redraw.json', {
        body: JSON.stringify({
          gate,
          targets: targetIds,
          redrawStatus,
          cancels: cancelBodies,
          restored: true,
          writes: writeGate.writes,
        }, null, 2),
        contentType: 'application/json',
      })
    } finally {
      if (queued) {
        for (const [imageId, n] of remaining) {
          await cancelPlanned(page, gate.a, imageId, n, usedVersionById.get(imageId)!)
        }
      }
    }
  })

  test('redraw-all returns the real 202 for every image and every exact cancel restores the checkpoint', async ({ page }, info) => {
    const gate = requireReplayGate(info)
    const baseline = await ensureReplayBaseline(page, gate)
    const images = baseline.a.images ?? []
    test.skip(!images.length, 'The A checkpoint has no images for a redraw-all assertion')
    if (!images.length) return
    const target = images.find(image => image.versions.some(publishedVersion))
    test.skip(!target, 'The A checkpoint has no ready image to open the Review screen')
    if (!target) return
    const opened = await openReadyImage(page, gate.a, target.id, target.label)
    test.skip(!opened, 'Native setup escape could not open the A review job')
    if (!opened) return

    const base = `/api/clone/${encodeURIComponent(gate.a)}`
    const cancelPaths = images.map(image => `${base}/images/${encodeURIComponent(image.id)}/versions/${nextVersion(image)}/cancel`)
    const writeGate = await guardWrites(page, write => {
      if (write.method !== 'POST') return false
      if (write.path === `${base}/images`) {
        const body = write.body as { op?: string; confirmCount?: number } | null
        return body?.op === 'regenerateAll' && body.confirmCount === images.length
      }
      return cancelPaths.includes(write.path)
    })
    const usedVersionById = new Map(images.map(image => [image.id, image.usedVersion]))
    const remaining = new Map<string, number>(images.map(image => [image.id, nextVersion(image)]))
    let queued = false
    let redrawStatus: number | null = null
    const cancelBodies: unknown[] = []
    let responseBody: unknown = null

    try {
      const redrawAll = page.getByRole('button', { name: new RegExp(`^Vẽ lại cả ${images.length} ảnh$`) })
      await expect(redrawAll).toBeVisible()
      await expect(redrawAll).toBeEnabled()
      await redrawAll.click()
      const dialog = page.getByRole('dialog')
      await expect(dialog).toBeVisible()
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST' && new URL(candidate.url()).pathname === `${base}/images`),
        dialog.getByRole('button', { name: new RegExp(`^Vẽ lại ${images.length} ảnh$`) }).click(),
      ])
      redrawStatus = response.status()
      expect(response.status(), `redraw-all must return the real 202, got ${response.status()}`).toBe(202)
      queued = true
      const requestBody = response.request().postDataJSON() as { op?: string; confirmCount?: number }
      expect(requestBody.op).toBe('regenerateAll')
      expect(requestBody.confirmCount).toBe(images.length)
      responseBody = await response.json()
      const body = responseBody as { accepted?: boolean; pending?: string[]; count?: number }
      expect(body.accepted).toBe(true)
      expect(body.count).toBe(images.length)
      const pending = body.pending ?? images.map(image => image.id)
      expect([...pending].sort()).toEqual([...images.map(image => image.id)].sort())

      const mid = await readImageSnapshot(page, gate.a)
      for (const image of images) expect(mid.items?.find(item => item.id === image.id)?.status).toBe('pending')

      for (const imageId of pending) {
        const n = remaining.get(imageId)
        const usedVersion = usedVersionById.get(imageId)
        expect(n).toBeDefined()
        expect(usedVersion).toBeDefined()
        cancelBodies.push(await cancelPlanned(page, gate.a, imageId, n!, usedVersion!))
        remaining.delete(imageId)
      }
      const after = await readImageSnapshot(page, gate.a)
      expectSnapshotUnchanged(baseline.a, after)
      expect(writeGate.unexpected, 'redraw-all may only write regenerateAll and exact per-image cancels').toEqual([])
      await info.attach('replay-mutation-redraw-all.json', {
        body: JSON.stringify({
          gate,
          images: images.map(image => image.id),
          redrawStatus,
          response: responseBody,
          cancels: cancelBodies,
          restored: true,
          writes: writeGate.writes,
        }, null, 2),
        contentType: 'application/json',
      })
    } finally {
      if (queued) {
        for (const [imageId, n] of remaining) {
          await cancelPlanned(page, gate.a, imageId, n, usedVersionById.get(imageId)!)
        }
      }
    }
  })

  test('skip image then approve the whole video returns the real 202s, advances to B and leaves A queued (not finished)', async ({ page }, info) => {
    const gate = requireReplayGate(info)
    const baseline = await ensureReplayBaseline(page, gate)
    const at = baseline.review.findIndex(job => job.id === gate.a)
    expect(at, 'A must be in review').toBeGreaterThanOrEqual(0)
    expect(baseline.review[at + 1]?.id, `B replay job ${gate.b} must immediately follow A; approve's next-video check is exact`).toBe(gate.b)
    const sceneItems = (baseline.a.items ?? []).filter(item => item.kind === 'scene' && item.status !== 'skipped')
    test.skip(sceneItems.length < 2, 'The A checkpoint needs at least two non-skipped scene images so skip can keep a usable neighbour')
    if (sceneItems.length < 2) return
    const target = (baseline.a.images ?? []).find(image => image.kind === 'scene' &&
      sceneItems.some(item => item.id === image.id) && image.versions.some(publishedVersion))
    test.skip(!target, 'The A checkpoint has no ready scene image eligible for skip')
    if (!target) return
    const opened = await openReadyImage(page, gate.a, target.id, target.label)
    test.skip(!opened, 'Native setup escape could not open the A review job')
    if (!opened) return

    const base = `/api/clone/${encodeURIComponent(gate.a)}`
    const imagePath = `${base}/images/${encodeURIComponent(target.id)}`
    const writeGate = await guardWrites(page, write => {
      if (write.method !== 'POST') return false
      const body = write.body as { op?: string; expectedRevision?: number } | null
      const revisionOk = typeof body?.expectedRevision === 'number' && body.expectedRevision > 0
      return revisionOk && (
        (write.path === imagePath && body?.op === 'skip') ||
        (write.path === `${base}/images` && body?.op === 'approveAll')
      )
    })
    const jobBefore = await readCloneJob(page, gate.a)
    const bJob = baseline.review.find(job => job.id === gate.b)
    const receipt: Record<string, unknown> = {
      gate,
      target: target.id,
      requiresControllerReseed: true,
      sourceUrlBefore: jobBefore.source?.url ?? null,
      writes: writeGate.writes,
    }
    try {
      const skipButton = page.getByRole('button', { name: 'Bỏ ảnh', exact: true })
      await expect(skipButton).toBeVisible()
      const [skipResponse] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST' && new URL(candidate.url()).pathname === imagePath),
        skipButton.click(),
      ])
      receipt.skipStatus = skipResponse.status()
      expect(skipResponse.status(), `Bỏ ảnh must return the real 202, got ${skipResponse.status()}`).toBe(202)
      const skipBody = await skipResponse.request().postDataJSON() as { op?: string; expectedRevision?: number }
      expect(skipBody.op).toBe('skip')
      expect(skipBody.expectedRevision).toBeGreaterThan(0)

      await expect.poll(async () => (await readImageSnapshot(page, gate.a)).items?.find(item => item.id === target.id)?.status).toBe('skipped')
      const mid = await readImageSnapshot(page, gate.a)
      expect(mid.items?.find(item => item.id === target.id)?.status).toBe('skipped')
      expect(mid.images?.find(image => image.id === target.id)?.usedVersion, 'skip clears the current version').toBe(0)
      expect((mid.items ?? []).filter(item => item.kind === 'scene' && item.status !== 'skipped').length).toBeGreaterThanOrEqual(1)
      expect(mid.items?.filter(item => item.kind === 'scene' && item.status !== 'skipped').length).toBe(sceneItems.length - 1)
      receipt.skippedImage = mid.items?.find(item => item.id === target.id)?.status ?? null
      receipt.scenesLeft = mid.items?.filter(item => item.kind === 'scene' && item.status !== 'skipped').length ?? 0

      const approveButton = page.getByRole('button', { name: /^Duyệt cả video & dựng/ })
      await expect(approveButton).toBeVisible()
      await expect(approveButton).not.toHaveAttribute('aria-disabled', 'true')
      const [approveResponse] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST' && new URL(candidate.url()).pathname === `${base}/images`),
        approveButton.click(),
      ])
      receipt.approveStatus = approveResponse.status()
      expect(approveResponse.status(), `Duyệt cả video must return the real 202, got ${approveResponse.status()}`).toBe(202)
      const approveBody = await approveResponse.request().postDataJSON() as { op?: string; expectedRevision?: number }
      expect(approveBody.op).toBe('approveAll')
      expect(approveBody.expectedRevision).toBeGreaterThan(0)
      const approvePayload = await approveResponse.json() as { jobId?: string; revision?: number }
      expect(approvePayload.jobId).toBe(gate.a)
      expect(approvePayload.revision).toBeGreaterThan(0)

      await expect.poll(() => new URL(page.url()).hash).toContain(`video=${gate.b}`)
      const nextVideoTitle = bJob?.source?.title
      if (nextVideoTitle) await expect(page.getByRole('heading', { name: nextVideoTitle, exact: true })).toBeVisible()
      const approvedSnapshot = await readImageSnapshot(page, gate.a)
      expect(approvedSnapshot.approved, 'approval must persist the owner gate even while the dispatcher is held in standby').toBe(true)
      expect(approvedSnapshot.manifest?.revision).toBe(approvePayload.revision)
      expect((approvedSnapshot.images ?? []).map(image => image.id), 'approval preserves every canonical image identity').toEqual((baseline.a.images ?? []).map(image => image.id))
      expect(approvedSnapshot.items?.every(item => item.status === 'approved' || item.status === 'skipped')).toBe(true)

      const { jobs } = await readReviewJobs(page)
      const aAfter = jobs.find(job => job.id === gate.a)
      const bAfter = jobs.find(job => job.id === gate.b)
      expect(aAfter?.stageGroup, 'standby has not consumed the durable build intent yet').toBe('review')
      expect(aAfter?.status, 'a queued owner approval must remain pending, never a completed build').toBe('pending')
      expect(bAfter?.stageGroup, 'the next review video B must still be waiting for the owner').toBe('review')
      const bImageSnapshot = await readImageSnapshot(page, gate.b)
      expectSnapshotUnchanged(baseline.b, bImageSnapshot)
      const jobAfter = await readCloneJob(page, gate.a)
      expect(jobAfter.source?.url ?? null, 'skip/approve must never replace or delete the source job').toBe(jobBefore.source?.url ?? null)
      expect(writeGate.unexpected, 'skip/approve may only write the exact image skip and approveAll routes').toEqual([])
      Object.assign(receipt, {
        nextVideo: gate.b,
        nextVideoTitle: bJob?.source?.title ?? null,
        aAfter: aAfter ?? null,
        bAfter: bAfter ?? null,
        bImageSnapshotUnchanged: true,
        persistedOwnerApproval: approvedSnapshot.approved,
        approvedRevision: approvedSnapshot.manifest?.revision,
        dispatcherHeldInStandby: true,
        sourceUrlAfter: jobAfter.source?.url ?? null,
      })
    } finally {
      await info.attach('replay-mutation-skip-approve.json', {
        body: JSON.stringify(receipt, null, 2),
        contentType: 'application/json',
      })
    }
  })
})
