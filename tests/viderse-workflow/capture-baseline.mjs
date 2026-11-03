#!/usr/bin/env node
import { chromium, expect } from '@playwright/test'
import { readFile, writeFile, mkdir, rename } from 'node:fs/promises'
import { resolve, dirname, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const plan = 'plans/261103-0752-viderse-workflow-rounds-40-44'
const matrix = JSON.parse(await readFile(resolve(repo, plan, 'reports/artboard-state-matrix.json'), 'utf8'))
const mode = process.argv[2]
if (!['reference', 'app'].includes(mode)) throw new Error('Usage: capture-baseline.mjs reference|app [board ...]')
const canonicalOut = resolve(repo, plan, 'reports/pr-00')
const out = resolve(process.env.WORKFLOW_EVIDENCE_DIR ?? canonicalOut)
const selected = new Set(process.argv.slice(3))
if (selected.size && out === canonicalOut) throw new Error('Filtered captures require a separate WORKFLOW_EVIDENCE_DIR')
const rows = matrix.rows.filter(row => !selected.size || selected.has(row.board))
if (!rows.length) throw new Error('No matching artboard rows')
const referenceOrigin = process.env.WORKFLOW_REFERENCE_URL ?? 'http://127.0.0.1:8041'
const appOrigin = process.env.WORKFLOW_APP_URL ?? 'http://127.0.0.1:8044'
for (const origin of [referenceOrigin, appOrigin]) {
  const url = new URL(origin)
  if (!['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) || !['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash || url.pathname !== '/') throw new Error('Baseline origins must be credential-free loopback origins')
}
const revision = process.env.WORKFLOW_REVISION ?? null
const captures = [], failures = [], apiErrors = []
let browser, context, page, browserInfo, currentBoard, fatalError
const viewportFor = row => row.page === 'mobile' ? { width: 390, height: 844 } : { width: 1440, height: 900 }
await mkdir(resolve(out, mode === 'reference' ? 'reference' : 'app'), { recursive: true })
const reportPath = resolve(out, mode === 'reference' ? 'reference-captures.json' : 'app-baseline.json')
const save = async status => {
  const temporary = reportPath + `.tmp-${process.pid}`
  await writeFile(temporary, JSON.stringify({
    capturedAt: new Date().toISOString(), revision, status, error: fatalError ?? null,
    runner: 'Playwright Chromium; headless isolated context', browser: browserInfo ?? null,
    command: process.argv.slice(2), origin: mode === 'reference' ? referenceOrigin : appOrigin,
    selectedBoards: [...new Set(rows.map(row => row.board))], captures, failures, apiErrors, productAcceptance: false,
  }, null, 2) + '\n')
  await rename(temporary, reportPath)
}
const watch = target => {
  const pending = new Set(), requestBoards = new WeakMap()
  target.on('request', request => {
    requestBoards.set(request, currentBoard)
    const url = new URL(request.url())
    if (url.pathname.startsWith('/api/') && request.resourceType() !== 'eventsource' && url.pathname !== '/api/events') pending.add(request)
  })
  target.on('requestfinished', request => pending.delete(request))
  target.on('requestfailed', request => pending.delete(request))
  target.on('response', response => {
    const path = new URL(response.url()).pathname, request = response.request()
    if (!path.startsWith('/api/') || response.status() < 400) return
    const board = requestBoards.get(request) ?? currentBoard
    const expected = (board === 'MAuth' && response.status() === 401) || (request.method() === 'GET' && path === '/api/storage/server' && response.status() === 404)
    apiErrors.push({ board, method: request.method(), path, status: response.status(), expected, observedAt: new Date().toISOString() })
  })
  return pending
}
const metadata = async () => page.evaluate(() => ({
  url: location.href, title: document.title, fontStatus: document.fonts.status,
  width: innerWidth, height: innerHeight, dpr: devicePixelRatio, visibility: document.visibilityState,
  scrollWidth: document.documentElement.scrollWidth,
  pendingImages: [...document.images].filter(image => !image.complete).length,
  brokenImages: [...document.images].filter(image => image.complete && image.naturalWidth === 0).map(image => image.getAttribute('src')),
  renderErrors: [...document.querySelectorAll('.sc-logic-error,.sc-placeholder-error')].map(node => node.textContent),
}))
const settle = async () => {
  await page.evaluate(async () => {
    await document.fonts.ready
    await Promise.all([...document.images].map(image => image.decode()))
    await new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done)))
  })
  await page.waitForFunction(() => document.getAnimations().filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime)).every(animation => animation.playState === 'finished' || animation.playState === 'idle'), undefined, { timeout: 10_000 })
  const result = await metadata()
  if (result.renderErrors.length || result.brokenImages.length || result.pendingImages) throw new Error('Capture has render errors or incomplete images: ' + JSON.stringify(result))
  return result
}
await save('running')
try {
  if (!revision) throw new Error('WORKFLOW_REVISION must identify the captured source')
  if (mode === 'app') {
    for (const key of ['WORKFLOW_QA_FIXTURE', 'WORKFLOW_QA_COOKIE_FILE']) if (!process.env[key]) throw new Error(`${key} is required`)
    if (resolve(process.env.WORKFLOW_QA_COOKIE_FILE).startsWith(repo + sep)) throw new Error('QA cookies must remain outside the repository')
  }
  browser = await chromium.launch({ headless: true, executablePath: process.env.PW_BROWSER_BINARY || undefined })
  const session = await browser.newBrowserCDPSession()
  const processes = await session.send('SystemInfo.getProcessInfo')
  browserInfo = { engine: process.env.PW_BROWSER_BINARY ? 'Chromium, custom executablePath' : 'Playwright Chromium', version: browser.version(), pid: processes.processInfo.find(process => process.type === 'browser')?.id, profile: 'fresh Playwright temporary profile; isolated newContext, no user profile' }
  await session.detach()
  context = await browser.newContext({ deviceScaleFactor: 1 })
  page = await context.newPage()
  let pending = watch(page)
  if (mode === 'reference') {
    let board
    for (const row of rows) {
      currentBoard = row.board
      try {
        const viewport = viewportFor(row)
        if (board !== row.board) {
          await page.setViewportSize(viewport)
          const response = await page.goto(`${referenceOrigin}/artboards/${row.board}.dc.html`, { waitUntil: 'domcontentloaded' })
          if (!response?.ok()) throw new Error(`Artboard HTTP ${response?.status()}`)
          await page.waitForFunction(() => typeof window.__dcSetProps === 'function' && typeof window.__dcRootName === 'function')
          board = row.board
        }
        const props = { ...row.defaults, ...row.context, ...row.patch }
        await page.evaluate(props => {
          const root = window.__dcRootName(), registry = window.__dcRegistry[root]
          for (const [key, value] of Object.entries(props)) {
            const meta = registry.propsMeta[key]
            if (!meta || (meta.editor === 'enum' && !meta.options.includes(value)) || (meta.editor === 'boolean' && typeof value !== 'boolean')) throw new Error(`Invalid tweak ${key}`)
          }
          window.__dcSetProps(root, props)
          if (Object.keys(registry.propOverrides).length !== Object.keys(props).length || Object.entries(props).some(([key, value]) => registry.propOverrides[key] !== value)) throw new Error('Artboard props readback mismatch')
        }, props)
        await settle()
        await page.evaluate(() => {
          for (const element of document.querySelectorAll('*')) if (element.scrollTop || element.scrollLeft) element.scrollTo({ top: 0, left: 0, behavior: 'instant' })
        })
        if (row.captureTarget) {
          await page.locator(row.captureTarget).evaluate(element => element.scrollIntoView({ block: 'center', inline: 'nearest', behavior: 'instant' }))
          await expect(page.locator(row.captureTarget)).toBeInViewport()
        }
        const metrics = await settle()
        const reference = `reference/${row.id.replace(/[^a-zA-Z0-9._-]/g, '_')}.png`
        await page.screenshot({ path: resolve(out, reference) })
        const framing = { target: row.captureTarget ?? null, targetBounds: row.captureTarget ? await page.locator(row.captureTarget).boundingBox() : null, scroll: await page.evaluate(() => [...document.querySelectorAll('*')].filter(element => element.scrollTop || element.scrollLeft).map(element => ({ tag: element.tagName, id: element.id, label: element.getAttribute('aria-label'), top: element.scrollTop, left: element.scrollLeft }))) }
        const opposite = row.page === 'mobile' ? { width: 1440, height: 900 } : { width: 390, height: 844 }
        const notApplicable = [{ viewport: opposite, scope: 'reference only, not app parity', source: `${row.file} data-props.$preview`, reason: 'This artboard specifies only its recorded design viewport. Use the separately designed counterpart where available; do not invent a resized reference or an excluded mobile Editor/viewer.' }]
        captures.push({ id: row.id, board: row.board, ownerPR: row.ownerPR, prerequisite: row.prerequisite, expectedVisible: row.expectedVisible, props, viewport, notApplicable, framing, reference, capturedAt: new Date().toISOString(), metrics, status: 'reference-captured', visibleStateVerified: false, productAcceptance: false })
      } catch (error) { failures.push({ id: row.id, board: row.board, error: String(error), productAcceptance: false }) }
      await save('running')
    }
  } else {
    const fixture = JSON.parse(await readFile(process.env.WORKFLOW_QA_FIXTURE, 'utf8'))
    const cookies = JSON.parse(await readFile(process.env.WORKFLOW_QA_COOKIE_FILE, 'utf8'))
    await context.addCookies(cookies)
    const account = await context.request.get(`${appOrigin}/api/me`), identity = await account.json()
    if (!account.ok() || fixture.username !== 'workflow.qa' || identity.user?.username !== fixture.username) throw new Error('QA account isolation check failed')
    const job = fixture.jobs[0], channel = fixture.channels[0], template = fixture.templates[0]
    const routes = {
      Main: `#/?video=${job.id}`, Review: `#/review?video=${job.id}`, Editor: `#/editor/${fixture.jobs[1].projectId}`,
      Template: `#/templates/${template.id}`, Library: '#/library?tab=templates', Settings: '#/settings?section=connect',
      Channel: `#/channels/${channel.id}?tab=sources`, CreateChannel: '#/channels/new', Onboard: '#/channels/new',
      NewVideo: `#/new?channel=${channel.id}`, Explore: '#/explore', Queue: '#/queue', Activity: '#/', Dialogs: `#/channels/${channel.id}`, System: '#/',
      MMain: '#/', MReview: `#/review?video=${job.id}`, MChannel: `#/channels/${channel.id}?tab=sources`, MLibrary: '#/library?tab=templates',
      MTemplate: `#/templates/${template.id}`, MNew: `#/new?channel=${channel.id}`, MExplore: '#/explore', MCompVideo: '#/explore',
      MQueue: '#/queue', MSystem: `#/channels/${channel.id}`, MSettings: '#/settings?section=connect', MCreate: '#/channels/new', MAuth: '#/',
    }
    const unavailable = {
      Activity: 'Background-activity popover not opened; route background only', System: 'System overlay prerequisites not seeded; route background only',
      Dialogs: 'Confirmation not opened; channel background only', MSystem: 'Confirmation sheet not opened; channel background only',
      MCompVideo: 'No QA rival/source video detail fixture; Explore background only', Onboard: 'Onboarding wizard needs a channel-less account; this route renders CreateChannel',
    }
    const boards = [...new Map(rows.map(row => [row.board, row])).values()].sort((a, b) => Number(a.board === 'MAuth') - Number(b.board === 'MAuth'))
    for (const row of boards) {
      currentBoard = row.board
      try {
        await context.close()
        context = await browser.newContext({ deviceScaleFactor: 1 })
        if (row.board !== 'MAuth') await context.addCookies(cookies)
        page = await context.newPage()
        pending = watch(page)
        await page.setViewportSize(viewportFor(row))
        await page.goto(`${appOrigin}/${routes[row.board]}`, { waitUntil: 'domcontentloaded' })
        if (row.board === 'MAuth') await page.getByRole('button', { name: 'Đăng nhập', exact: true }).waitFor()
        else {
          await page.waitForFunction(hash => location.hash.split('?')[0] === hash && document.visibilityState === 'visible' && document.body.innerText.trim().length > 0, routes[row.board].split('?')[0])
          await page.locator('[aria-busy="true"]').filter({ visible: true }).first().waitFor({ state: 'hidden' })
          await page.evaluate(() => new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done))))
        }
        await expect.poll(() => pending.size, { timeout: 30_000, message: 'Route API requests must finish before baseline capture' }).toBe(0)
        if (['Review', 'MReview', 'Main'].includes(row.board)) await page.locator('img[src*="/api/clone/"]').first().waitFor()
        const metrics = await settle()
        const app = `app/${row.board}.baseline.png`
        await page.screenshot({ path: resolve(out, app) })
        const actual = await page.evaluate(() => ({
          text: document.body.innerText.slice(0, 14000),
          images: [...document.images].map(image => ({ src: image.getAttribute('src'), complete: image.complete, width: image.naturalWidth })),
          controls: [...document.querySelectorAll('button,a,[role="tab"],[role="radio"],[role="switch"]')].filter(element => element.getClientRects().length).map(element => ({
            role: element.getAttribute('role') || element.tagName.toLowerCase(), label: element.getAttribute('aria-label') || element.textContent.trim(), disabled: element.disabled || element.getAttribute('aria-disabled') === 'true', href: element.getAttribute('href'),
          })),
        }))
        Object.assign(actual, metrics)
        captures.push({ board: row.board, ownerPR: row.ownerPR, viewport: viewportFor(row), app, actual, capturedAt: new Date().toISOString(), status: unavailable[row.board] ? 'target-prerequisite-missing' : 'baseline-only', blockedReason: unavailable[row.board] ?? null, targetStateMatched: false, productAcceptance: false })
      } catch (error) { failures.push({ board: row.board, error: String(error), pendingRequests: [...pending].map(request => ({ method: request.method(), path: new URL(request.url()).pathname, resourceType: request.resourceType() })), productAcceptance: false }) }
      await save('running')
    }
  }
  const unexpected = apiErrors.filter(error => !error.expected)
  if (unexpected.length) failures.push({ board: 'api', error: 'Unexpected API responses', responses: unexpected })
} catch (error) { fatalError = String(error) }
finally {
  await browser?.close().catch(error => { fatalError ??= `Browser cleanup failed: ${error}` })
  await save(fatalError ? 'aborted' : failures.length ? 'completed-with-failures' : 'completed')
}
console.log(JSON.stringify({ mode, captured: captures.length, failed: failures.length, apiErrors: apiErrors.length, unexpectedApiErrors: apiErrors.filter(error => !error.expected).length, error: fatalError ?? null, out }))
if (failures.length || fatalError) process.exitCode = 1
