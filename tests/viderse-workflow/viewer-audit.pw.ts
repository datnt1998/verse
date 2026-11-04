import { test, expect, type Page, type TestInfo } from '@playwright/test'
import { readFileSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'
import { z } from 'zod'

const repo = resolve(__dirname, '../..')
const cookiePath = process.env.WORKFLOW_QA_COOKIE_FILE
const fixturePath = process.env.WORKFLOW_QA_FIXTURE
if (!cookiePath || !fixturePath || realpathSync(cookiePath).startsWith(repo + sep)) throw new Error('Private isolated QA session required')
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as { username: string; jobs: { id: string; projectId: string }[]; templates: { id: string }[] }
if (fixture.username !== 'workflow.qa') throw new Error('Only workflow.qa is authorized')
const runtimeFaults = new WeakMap<Page, string[]>()

async function settle(page: Page) {
  await page.evaluate(async () => { await document.fonts.ready; await new Promise(done => requestAnimationFrame(() => requestAnimationFrame(() => requestAnimationFrame(done)))) })
  await page.waitForFunction(() => document.getAnimations().filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime)).every(animation => animation.playState === 'finished' || animation.playState === 'idle'))
}
async function capture(page: Page, info: TestInfo, name: string) {
  await settle(page)
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
  await page.screenshot({ path: info.outputPath(`${name}.png`) })
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

test('review image address survives reload, navigation and cancelling paid feedback', async ({ page }, info) => {
  const job = fixture.jobs[0].id
  const mutations: string[] = []
  page.on('request', request => { if (request.method() !== 'GET' && new URL(request.url()).pathname.endsWith('/redraw')) mutations.push(request.url()) })
  await page.goto(`/#/review?video=${job}&img=sc-04`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByRole('option', { name: /^Cảnh 0?4$/ })).toHaveAttribute('aria-selected', 'true')
  await capture(page, info, 'review-addressed')
  await page.reload()
  await expect(page.getByRole('option', { name: /^Cảnh 0?4$/ })).toHaveAttribute('aria-selected', 'true')
  if (page.viewportSize()!.width < 768) {
    await page.getByRole('option', { name: /^Cảnh 0?5$/ }).click()
  } else {
    await page.getByRole('button', { name: 'Ảnh sau', exact: true }).click()
  }
  await expect(page).toHaveURL(new RegExp('img=sc-05'))
  await expect(page.getByRole('option', { name: /^Cảnh 0?5$/ })).toHaveAttribute('aria-selected', 'true')
  await page.goBack()
  await expect(page.getByRole('option', { name: /^Cảnh 0?4$/ })).toHaveAttribute('aria-selected', 'true')
  if (page.viewportSize()!.width >= 768) {
    const input = page.locator('#redraw')
    await input.fill('Giữ ánh đèn lạnh trong cảnh')
    await input.press('ArrowRight')
    await expect(page).toHaveURL(new RegExp('img=sc-04'))
    await page.getByRole('button', { name: /^Vẽ lại(?:\s*R)?$/ }).click()
    const confirmation = page.getByRole('alertdialog')
    await expect(confirmation).toContainText('Tốn 1 ảnh')
    await capture(page, info, 'review-paid-confirmation')
    await confirmation.getByRole('button', { name: 'Huỷ', exact: true }).click()
    await expect(confirmation).toHaveCount(0)
    await expect(input).toHaveValue('Giữ ánh đèn lạnh trong cảnh')
    await expect(page.getByRole('button', { name: 'Sửa chỉ dẫn', exact: true })).toBeDisabled()
    await page.keyboard.press('Escape')
    const stage = page.getByRole('img', { name: 'Cảnh 4', exact: true })
    await stage.click()
    await page.keyboard.press('e')
    await expect(page.locator('#prompt-scene')).toHaveCount(0)
    const original = await stage.getAttribute('src')
    await page.locator('[aria-label="Lớp ảnh"] button').filter({ hasText: 'Tách nền' }).click()
    await expect(stage).not.toHaveAttribute('src', original!)
    await expect.poll(() => stage.evaluate(image => (image as HTMLImageElement).complete && (image as HTMLImageElement).naturalWidth > 0)).toBe(true)
    await capture(page, info, 'review-real-cutout')
    await page.locator('[aria-label="Lớp ảnh"] button').filter({ hasText: 'Ảnh gốc' }).click()
    await expect(stage).toHaveAttribute('src', original!)
    await page.getByRole('button', { name: 'Thêm', exact: true }).click()
    const references = page.getByRole('dialog', { name: 'Thêm ảnh tham chiếu', exact: true })
    await references.getByRole('button', { name: 'Cảnh 5', exact: true }).click()
    await references.getByRole('button', { name: 'Thêm 1 ảnh', exact: true }).click()
    await expect(references).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Bỏ ảnh tham chiếu 1', exact: true })).toBeVisible()
    await expect(page.locator('#prompt-scene')).toHaveCount(0)
    await capture(page, info, 'review-staged-reference')
    await page.getByRole('button', { name: /^Huỷ sửa/ }).click()
    await expect(page.getByRole('button', { name: 'Bỏ ảnh tham chiếu 1', exact: true })).toHaveCount(0)
  }
  expect(mutations).toEqual([])
  await page.goto(`/#/review?video=${job}&img=removed-image`, { waitUntil: 'domcontentloaded' })
  await expect.poll(() => new URL(page.url()).hash).not.toContain('removed-image')
  await expect(page.getByRole('option', { selected: true })).toHaveCount(1)
  await info.attach('review-address.json', { body: JSON.stringify({ viewport: page.viewportSize(), finalAddress: new URL(page.url()).hash, redrawMutations: mutations, mobileViewerRound45Excluded: true }, null, 2), contentType: 'application/json' })
})

test('workbench viewer uses stable addresses, contains the natural image and restores focus/scroll', async ({ page }, info) => {
  if (page.viewportSize()!.width < 768) {
    await page.goto(`/#/review?video=${fixture.jobs[0].id}&img=sc-03`)
    await expect(page.getByRole('option', { name: /^Cảnh 0?3$/ })).toHaveAttribute('aria-selected', 'true')
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await capture(page, info, 'mobile-original-review-no-round45-viewer')
    return
  }
  const job = fixture.jobs[1].id
  const imageBase = `/api/clone/${encodeURIComponent(job)}/images`
  type SnapshotVersion = { n: number; state: string; url: string | null; previewUrl: string | null; width: number; height: number }
  type SnapshotImage = { id: string; kind: 'scene' | 'cover'; label: string; usedVersion: number; versions: SnapshotVersion[] }
  type Snapshot = { images?: SnapshotImage[]; quota?: { left: number } | null }
  const readSnapshot = async () => {
    const response = await page.request.get(imageBase)
    expect(response.status(), 'the live §26 image snapshot must be readable').toBe(200)
    return await response.json() as Snapshot
  }
  // Địa chỉ ảnh nằm trong hash (`#/?video=…&img=…`), không phải query của URL.
  const hashParams = () => {
    const hash = new URL(page.url()).hash
    return new URLSearchParams(hash.slice(hash.indexOf('?') + 1))
  }
  const redrawMutations: string[] = []
  const imageMutations: string[] = []
  page.on('request', request => {
    if (request.method() === 'GET') return
    const path = new URL(request.url()).pathname
    if (path === `${imageBase}/redraw`) redrawMutations.push(`${request.method()} ${path}`)
    else if (path.startsWith(`${imageBase}/`)) imageMutations.push(`${request.method()} ${path}`)
  })

  // Bàn dựng thật, không địa chỉ ảnh: chưa có viewer nào được mở.
  await page.goto(`/#/?video=${job}`, { waitUntil: 'domcontentloaded' })
  await expect.poll(async () => new URL(page.url()).hash.startsWith('#/setup') || await page.getByRole('listbox', { name: 'Ảnh của video', exact: true }).isVisible()).toBe(true)
  if (new URL(page.url()).hash.startsWith('#/setup')) {
    await page.getByRole('link', { name: 'Cài đặt nâng cao', exact: true }).click()
    await page.goto(`/#/?video=${job}`, { waitUntil: 'domcontentloaded' })
  }
  expect(new URL(page.url()).hash).toBe(`#/?video=${job}`)
  await expect(page.getByRole('dialog')).toHaveCount(0)
  const grid = page.getByRole('listbox', { name: 'Ảnh của video', exact: true })
  await expect(grid).toBeVisible()
  await expect(page.getByRole('listbox', { name: 'Ảnh của video', exact: true })).toHaveCount(1)
  await expect(page.getByRole('region', { name: /^(Phòng vẽ|Bộ ảnh chờ duyệt)/ })).toBeVisible()
  // Viewer modal đợt 40 chỉ mở ngoài trạng thái chờ duyệt; job chờ duyệt đi thẳng
  // sang màn Duyệt nên không thể thay thế các trạng thái của viewer này.
  expect(await page.getByRole('region', { name: 'Bộ ảnh chờ duyệt', exact: true }).count(), `Main modal-viewer prerequisite: job ${job} must be non-review (stageGroup working/attention/paused) with ready images; a review-stage Main routes every tile to #/review and never opens the round-40 viewer`).toBe(0)

  const snapshot = await readSnapshot()
  const images = snapshot.images ?? []
  expect(images.length, 'the live snapshot must list the studio images').toBeGreaterThan(2)
  const gridLabels = await grid.getByRole('option').evaluateAll(nodes => nodes.map(node => node.getAttribute('aria-label') ?? ''))
  const scenePairs = gridLabels
    .map((label, index) => ({ label, next: gridLabels[index + 1] ?? '' }))
    .filter(entry => entry.label.startsWith('Cảnh ') && entry.next.startsWith('Cảnh '))
  const chosen = scenePairs.find(entry => entry.label === 'Cảnh 3') ?? scenePairs[0]
  expect(chosen, 'a real scene image with a following scene must be reachable from the studio grid').toBeTruthy()
  const sceneLabel = chosen!.label
  const tile = grid.getByRole('option', { name: sceneLabel, exact: true })
  await expect(tile).toBeVisible()
  await expect(tile).toHaveAttribute('tabindex', '0')
  await expect(tile.getByRole('button', { name: `Vẽ lại ${sceneLabel.toLocaleLowerCase('vi')}`, exact: true })).toBeEnabled()
  await capture(page, info, 'workbench-none')

  const historyBefore = await page.evaluate(() => history.length)
  await tile.click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${sceneLabel}`)
  const openedHash = new URL(page.url()).hash
  const openedId = hashParams().get('img')
  expect(openedId, 'the opened address must carry the image id').toBeTruthy()
  expect(openedHash).toBe(`#/?video=${job}&img=${openedId}`)
  const snapImage = images.find(image => image.id === openedId)
  expect(snapImage, 'the opened address must name an image from the live snapshot').toBeDefined()
  expect(snapImage!.label).toBe(sceneLabel)
  expect(await page.evaluate(() => history.length), 'opening pushes exactly one history entry').toBe(historyBefore + 1)
  await expect.poll(() => page.evaluate(() => !!document.activeElement?.closest('[role="dialog"]')), 'opening from the studio must move focus into the viewer').toBe(true)

  const stage = dialog.getByRole('img', { name: sceneLabel, exact: true })
  await expect(stage).toBeVisible()
  await expect.poll(() => stage.evaluate(image => (image as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)
  await settle(page)
  const geometry = await dialog.evaluate(node => {
    const img = node.querySelector('img') as HTMLImageElement
    const stageBox = img.parentElement as HTMLElement
    const box = node.getBoundingClientRect()
    const imageBox = img.getBoundingClientRect()
    const frame = stageBox.getBoundingClientRect()
    return {
      x: box.x, y: box.y, width: box.width, height: box.height,
      natural: [img.naturalWidth, img.naturalHeight] as [number, number],
      rendered: [img.clientWidth, img.clientHeight] as [number, number],
      stage: { width: frame.width, height: frame.height },
      image: { x: imageBox.x, y: imageBox.y, width: imageBox.width, height: imageBox.height },
      src: img.getAttribute('src'),
    }
  })
  expect(geometry.width).toBeCloseTo(1240, 0)
  expect(geometry.height).toBeCloseTo(800, 0)
  expect(geometry.rendered[0]).toBeLessThanOrEqual(geometry.natural[0])
  expect(geometry.rendered[1]).toBeLessThanOrEqual(geometry.natural[1])
  expect(geometry.natural[0]).toBeGreaterThan(geometry.natural[1])
  expect(geometry.image.width).toBeLessThanOrEqual(geometry.stage.width + 1)
  expect(geometry.image.height).toBeLessThanOrEqual(geometry.stage.height + 1)
  expect(Math.abs(geometry.image.width / geometry.image.height - geometry.natural[0] / geometry.natural[1])).toBeLessThan(0.01)

  // So đúng ảnh/phiên bản với asset URL được snapshot máy chủ công bố, không giả định hình dạng route.
  let pressedN: number | null = null
  const pressedVersion = dialog.getByRole('group', { name: 'Phiên bản của ảnh', exact: true }).getByRole('button', { pressed: true })
  if (await pressedVersion.count()) {
    pressedN = Number((await pressedVersion.textContent())?.trim())
    expect(pressedN).toBeGreaterThan(0)
    const shownVersion = snapImage!.versions.find(version => version.n === pressedN)
    expect(shownVersion, `version ${pressedN} must exist in the live snapshot`).toBeDefined()
    expect(geometry.natural).toEqual([shownVersion!.width, shownVersion!.height])
  }
  const shownVersion = snapImage!.versions.find(version => version.n === (pressedN ?? snapImage!.usedVersion))
  expect(shownVersion, 'the displayed version must exist in the live snapshot').toBeDefined()
  expect(geometry.natural).toEqual([shownVersion!.width, shownVersion!.height])
  const shownVersionURL = shownVersion!.url ?? shownVersion!.previewUrl
  expect(shownVersionURL, 'the displayed version must have a published image asset').toBeTruthy()
  const stageURL = new URL(geometry.src!, page.url())
  expect(stageURL.href).toBe(new URL(shownVersionURL!, page.url()).href)

  const environment = await page.evaluate(() => ({ reduced: matchMedia('(prefers-reduced-motion: reduce)').matches, fonts: document.fonts.status }))
  expect(environment.fonts).toBe('loaded')
  expect(environment.reduced).toBe(info.project.use.reducedMotion === 'reduce')
  const motion = await dialog.evaluate(node => {
    const read = (element: Element | null) => {
      if (!element) return null
      const style = getComputedStyle(element)
      return { animationName: style.animationName, animationDuration: style.animationDuration, transitionDuration: style.transitionDuration, running: element.getAnimations().some(animation => animation.playState === 'running') }
    }
    return {
      dialog: read(node),
      scrim: read(document.querySelector('.kit-dlg-scrim')),
      selectedThumb: read(node.querySelector('[role="listbox"] [aria-selected="true"]')),
    }
  })
  if (environment.reduced) {
    expect(motion.dialog?.animationName).toBe('none')
    expect(motion.scrim?.animationName).toBe('none')
    expect(motion.selectedThumb?.running, 'reduced motion must not leave an animated thumbnail transition running').toBe(false)
  } else {
    expect(motion.dialog?.animationName).not.toBe('none')
    expect(motion.scrim?.animationName).not.toBe('none')
    expect(motion.selectedThumb?.transitionDuration).not.toBe('0s')
  }
  await capture(page, info, 'workbench-viewer')

  const panels = await page.locator('aside').evaluateAll(nodes => nodes.map(node => node.getBoundingClientRect().width))
  for (const key of ['[', ']', '.']) await page.keyboard.press(key)
  await settle(page)
  expect(await page.locator('aside').evaluateAll(nodes => nodes.map(node => node.getBoundingClientRect().width))).toEqual(panels)

  const strip = dialog.getByRole('listbox', { name: 'Ảnh của video', exact: true })
  await expect.poll(() => strip.getByRole('option').evaluateAll(nodes => nodes.every(node => {
    const image = node.querySelector('img') as HTMLImageElement | null
    return !image || image.naturalWidth > 0
  }))).toBe(true)
  const stripEntries = await strip.getByRole('option').evaluateAll(nodes => nodes.map(node => ({
    label: (node.getAttribute('aria-label') ?? '').replace(/ · chưa mở được$/, ''),
    selected: node.getAttribute('aria-selected') === 'true',
    disabled: node.getAttribute('aria-disabled') === 'true',
  })))
  const currentIndex = stripEntries.findIndex(entry => entry.selected)
  expect(currentIndex, 'the opened image must be selected in the strip').toBeGreaterThanOrEqual(0)
  const viewableIndexes = stripEntries.flatMap((entry, index) => entry.disabled ? [] : [index])
  expect(viewableIndexes.length, 'arrow navigation needs at least two viewable images').toBeGreaterThan(1)
  const nextIndex = viewableIndexes[(viewableIndexes.indexOf(currentIndex) + 1) % viewableIndexes.length]
  const nextLabel = stripEntries[nextIndex].label

  const historyOpen = await page.evaluate(() => history.length)
  await page.keyboard.press('ArrowRight')
  await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${nextLabel}`)
  const nextId = hashParams().get('img')
  expect(nextId).not.toBe(openedId)
  expect(new URL(page.url()).hash).toBe(`#/?video=${job}&img=${nextId}`)
  expect(images.find(image => image.id === nextId)?.label).toBe(nextLabel)
  const afterNext = new URL(page.url()).hash
  const historyAfterNext = await page.evaluate(() => history.length)
  expect(historyAfterNext, 'arrow navigation replaces, never pushes').toBe(historyOpen)
  await page.keyboard.press('ArrowLeft')
  await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${sceneLabel}`)
  expect(new URL(page.url()).hash).toBe(openedHash)
  expect(await page.evaluate(() => history.length)).toBe(historyOpen)
  await dialog.getByRole('button', { name: 'Ảnh sau (→)', exact: true }).click()
  await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${nextLabel}`)
  expect(new URL(page.url()).hash).toBe(afterNext)
  await dialog.getByRole('button', { name: 'Ảnh trước (←)', exact: true }).click()
  await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${sceneLabel}`)
  expect(new URL(page.url()).hash).toBe(openedHash)

  // Portrait chỉ đo khi job có ảnh dọc thật; fixture hiện tại toàn 16:9.
  const thumbSizes = await strip.getByRole('option').evaluateAll(nodes => nodes.map(node => {
    const image = node.querySelector('img') as HTMLImageElement | null
    return {
      label: (node.getAttribute('aria-label') ?? '').replace(/ · chưa mở được$/, ''),
      disabled: node.getAttribute('aria-disabled') === 'true',
      natural: image ? [image.naturalWidth, image.naturalHeight] as [number, number] : null,
    }
  }))
  const portraitThumb = thumbSizes.find(entry => !entry.disabled && entry.natural && entry.natural[1] > entry.natural[0])
  if (process.env.WORKFLOW_QA_REQUIRE_PORTRAIT === '1') expect(portraitThumb, 'the portrait acceptance run must actually expose its imported portrait image').toBeDefined()
  let portraitProbe: Record<string, unknown>
  if (portraitThumb) {
    await strip.getByRole('option', { name: portraitThumb.label, exact: true }).click()
    await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${portraitThumb.label}`)
    const portraitStage = dialog.getByRole('img', { name: portraitThumb.label, exact: true })
    await expect.poll(() => portraitStage.evaluate(image => (image as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)
    const portraitGeometry = await portraitStage.evaluate(image => {
      const img = image as HTMLImageElement
      const box = img.getBoundingClientRect()
      const frame = (img.parentElement as HTMLElement).getBoundingClientRect()
      return {
        natural: [img.naturalWidth, img.naturalHeight] as [number, number],
        rendered: [img.clientWidth, img.clientHeight] as [number, number],
        stage: { width: frame.width, height: frame.height },
        image: { width: box.width, height: box.height },
      }
    })
    expect(portraitGeometry.natural[1]).toBeGreaterThan(portraitGeometry.natural[0])
    expect(portraitGeometry.rendered[0]).toBeLessThanOrEqual(portraitGeometry.natural[0])
    expect(portraitGeometry.rendered[1]).toBeLessThanOrEqual(portraitGeometry.natural[1])
    expect(portraitGeometry.image.width).toBeLessThanOrEqual(portraitGeometry.stage.width + 1)
    expect(portraitGeometry.image.height).toBeLessThanOrEqual(portraitGeometry.stage.height + 1)
    portraitProbe = { available: true, label: portraitThumb.label, geometry: portraitGeometry }
    await capture(page, info, 'workbench-portrait-viewer')
    await strip.getByRole('option', { name: sceneLabel, exact: true }).click()
    await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${sceneLabel}`)
  } else {
    portraitProbe = {
      available: false,
      prerequisite: `a real portrait image (naturalHeight > naturalWidth) on job ${job}; the live strip only exposes landscape sources (${thumbSizes.filter(entry => entry.natural).map(entry => `${entry.label}:${entry.natural![0]}x${entry.natural![1]}`).join(', ')})`,
    }
  }

  await dialog.getByRole('button', { name: /^Góp ý/ }).click()
  const feedback = dialog.getByRole('textbox', { name: 'Góp ý cho Hoạ sĩ' })
  await expect(feedback).toBeFocused()
  await feedback.fill('Không đổi nhân vật')
  const addressWithFeedback = new URL(page.url()).hash
  await feedback.press('ArrowRight')
  await feedback.press('ArrowLeft')
  expect(new URL(page.url()).hash, 'arrow keys inside the feedback textarea must not change the image').toBe(addressWithFeedback)
  await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${sceneLabel}`)
  await expect(feedback).toHaveValue('Không đổi nhân vật')

  const live = await readSnapshot()
  const quotaLeft = live.quota?.left
  const cost = dialog.getByRole('button', { name: /^Vẽ lại · tốn 1 ảnh/ })
  await expect(cost).toBeVisible()
  if (quotaLeft == null) {
    await expect(dialog.getByText('Chưa tải được hạn mức ảnh nên chưa gửi được.', { exact: true })).toBeVisible()
    await expect(cost).toBeDisabled()
  } else if (quotaLeft > 0) {
    await expect(cost).toContainText(`(còn ${quotaLeft})`)
    await expect(cost).toBeEnabled()
    await expect(dialog.getByText(`Còn ${quotaLeft} ảnh trong hạn mức hôm nay.`, { exact: true })).toBeVisible()
  } else {
    await expect(cost).toContainText('(còn 0)')
    await expect(cost).toBeDisabled()
    await expect(dialog.getByText('Hết hạn mức ảnh hôm nay.', { exact: true })).toBeVisible()
  }
  const costLabel = (await cost.textContent())?.trim() ?? ''
  await capture(page, info, 'workbench-feedback')

  await dialog.getByRole('button', { name: 'Huỷ', exact: true }).click()
  await expect(feedback).toHaveCount(0)
  expect(new URL(page.url()).hash).toBe(addressWithFeedback)
  expect(redrawMutations, 'cancelling feedback must not send a redraw').toEqual([])
  expect(imageMutations, 'opening, browsing and cancelling must not mutate images').toEqual([])

  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  const closedHash = new URL(page.url()).hash
  expect(closedHash).toBe(`#/?video=${job}`)
  expect(await page.evaluate(() => document.body.style.overflow)).toBe('')
  await expect.poll(() => page.evaluate(() => document.activeElement?.getAttribute('aria-label'))).toBe(sceneLabel)
  await expect.poll(() => page.evaluate(() => document.activeElement?.getAttribute('role'))).toBe('option')
  const historyAfterClose = await page.evaluate(() => history.length)
  expect(historyAfterClose, 'closing replaces the viewer address in place').toBe(historyOpen)

  // Mở lại từ đúng ô ảnh: địa chỉ, reload và back/forward giữ nguyên một entry viewer.
  await tile.click()
  await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${sceneLabel}`)
  expect(new URL(page.url()).hash).toBe(openedHash)
  const historyReopen = await page.evaluate(() => history.length)
  expect(historyReopen).toBe(historyAfterClose + 1)
  await page.reload()
  await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${sceneLabel}`)
  expect(new URL(page.url()).hash).toBe(openedHash)
  const historyAfterReload = await page.evaluate(() => history.length)
  expect(historyAfterReload).toBe(historyReopen)
  await page.goBack()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  expect(new URL(page.url()).hash).toBe(`#/?video=${job}`)
  await page.goForward()
  await expect(dialog).toHaveAttribute('aria-label', `Xem ảnh ${sceneLabel}`)
  await dialog.getByRole('button', { name: /^Đóng(?:\s|$)/ }).click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  expect(new URL(page.url()).hash).toBe(`#/?video=${job}`)
  expect(await page.evaluate(() => history.length)).toBe(historyReopen)

  await info.attach('viewer-geometry.json', {
    body: JSON.stringify({
      job,
      viewport: page.viewportSize(),
      baseURL: info.project.use.baseURL ?? process.env.WORKFLOW_APP_URL ?? null,
      sourceRevision: process.env.WORKFLOW_SOURCE_REVISION ?? null,
      fonts: environment.fonts,
      reducedMotion: environment.reduced,
      states: { none: 'workbench-none.png', view: 'workbench-viewer.png', redraw: 'workbench-feedback.png' },
      address: { boot: `#/?video=${job}`, opened: openedHash, afterArrowRight: afterNext, afterArrowLeft: openedHash, afterClose: closedHash },
      history: { before: historyBefore, opened: historyOpen, afterArrowRight: historyAfterNext, afterReload: historyAfterReload, afterClose: historyAfterClose },
      geometry,
      motion,
      orientation: { landscape: { label: sceneLabel, natural: geometry.natural, rendered: geometry.rendered, stage: geometry.stage }, portrait: portraitProbe },
      quota: { snapshotLeft: quotaLeft ?? null, costLabel },
      sourceGuard: { stagePath: stageURL.pathname, snapshotImageId: snapImage!.id, shownVersion: pressedN },
      redrawMutations,
      imageMutations,
    }, null, 2),
    contentType: 'application/json',
  })
  expect(redrawMutations).toEqual([])
  expect(imageMutations).toEqual([])
})

test('template sample eye does not change selection until its explicit action; retired key card is absent', async ({ page }, info) => {
  const removed = await page.request.get('/api/youtube/data-key')
  expect(removed.status()).toBe(404)
  await page.goto(`/#/templates/${fixture.templates[0].id}?section=images`, { waitUntil: 'domcontentloaded' })
  if (page.viewportSize()!.width >= 768) {
    await expect.poll(async () => new URL(page.url()).hash.startsWith('#/setup') || await page.getByRole('region', { name: 'Ảnh cảnh', exact: true }).isVisible()).toBe(true)
    if (new URL(page.url()).hash.startsWith('#/setup')) {
      await page.getByRole('link', { name: 'Cài đặt nâng cao', exact: true }).click()
      await page.goto(`/#/templates/${fixture.templates[0].id}?section=images`, { waitUntil: 'domcontentloaded' })
    }
  }
  if (page.viewportSize()!.width < 768) {
    await expect(page.getByRole('region', { name: 'Ảnh cảnh', exact: true })).toBeVisible()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await capture(page, info, 'template-mobile-existing-layout')
    return
  }
  const eye = page.getByRole('button', { name: 'Xem lớn Ảnh mẫu QA tự tạo · 1', exact: true })
  await expect(eye).toBeVisible()
  const selectedBefore = await page.getByRole('radio', { checked: true }).allTextContents()
  await eye.click()
  const viewer = page.getByRole('dialog')
  await expect(viewer).toHaveAttribute('aria-label', 'Xem ảnh Ảnh mẫu QA tự tạo · 1')
  await capture(page, info, 'template-sample-viewer')
  await viewer.getByRole('button', { name: 'Mẫu sau (→)', exact: true }).click()
  await expect(viewer).toHaveAttribute('aria-label', 'Xem ảnh Ảnh mẫu QA tự tạo · 2')
  await page.keyboard.press('Escape')
  await expect(viewer).toHaveCount(0)
  await eye.click()
  await expect(viewer).toBeVisible()
  await page.mouse.click(16, 16)
  await expect(viewer).toHaveCount(0)
  expect(await page.getByRole('radio', { checked: true }).allTextContents()).toEqual(selectedBefore)
  await expect(eye).toBeFocused()
})

test('template export downloads a readable ZIP instead of the template JSON document', async ({ page }, info) => {
  await page.goto(`/#/templates/${fixture.templates[0].id}?section=output`)
  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Xuất tệp', exact: true }).click()
  const download = await downloadPromise
  expect(await download.failure()).toBeNull()
  expect(download.suggestedFilename()).toMatch(/\.ytf-template\.zip$/)
  const file = await download.path()
  expect(file).not.toBeNull()
  const bytes = readFileSync(file!)
  expect(bytes.subarray(0, 4).toString('hex')).toBe('504b0304')
  await info.attach('template-export.json', { body: JSON.stringify({ filename: download.suggestedFilename(), bytes: bytes.length, signature: bytes.subarray(0, 4).toString('hex') }, null, 2), contentType: 'application/json' })
  await capture(page, info, 'template-export-result')
})

test('Editor replacement changes only the chosen scene source and undo restores the persisted timeline', async ({ page }, info) => {
  test.skip(page.viewportSize()!.width < 768, 'Desktop Editor only; no undesigned mobile Editor or round45 viewer')
  const projectId = fixture.jobs[0].projectId
  const projectURL = `/api/editor/projects/${encodeURIComponent(projectId)}`
  const readProject = async () => {
    const response = await page.request.get(projectURL)
    expect(response.status()).toBe(200)
    return response.json()
  }
  await page.goto(`/#/editor/${projectId}`)
  const clip = page.locator('button[aria-pressed][title="Cánh cửa mở ra"]').last()
  await clip.click()
  const firstVersion = page.getByRole('button', { name: /^Bản 1 ·/ })
  await firstVersion.click()
  const useVersion = page.getByRole('button', { name: 'Dùng bản 1', exact: true })
  if (await useVersion.count()) {
    await useVersion.click()
    await expect(useVersion).toHaveCount(0)
  }
  await expect(page.getByText('Đã lưu', { exact: true })).toBeVisible()
  const before = await readProject()
  const targetId = `${projectId}-scene-sc-03`
  const target = before.timeline.items.find((item: { id: string }) => item.id === targetId)
  expect(target).toBeDefined()
  const mediaResponse = await page.request.get('/api/editor/media')
  expect(mediaResponse.status()).toBe(200)
  const { media } = await mediaResponse.json() as { media: { id: string; fileName: string }[] }
  const replacement = media.find(item => /^scene-(warm|cool)\.webp$/.test(item.fileName) && item.id !== target.mediaId)
  expect(replacement).toBeDefined()
  await page.getByRole('button', { name: 'Xem lớn Cánh cửa mở ra', exact: true }).click()
  const viewer = page.getByRole('dialog')
  await expect(viewer).toHaveAttribute('aria-label', 'Xem ảnh Cảnh 3')
  const clipCount = await page.locator('button[aria-pressed][title]').count()
  await page.keyboard.press('Delete')
  expect(await page.locator('button[aria-pressed][title]').count()).toBe(clipCount)
  await capture(page, info, 'editor-image-viewer')
  await viewer.getByRole('button', { name: 'Thay ảnh', exact: true }).click()
  const picker = page.getByRole('dialog', { name: 'Thay ảnh', exact: true })
  await expect(page.getByRole('dialog')).toHaveCount(1)
  await expect(picker).toBeVisible()
  await capture(page, info, 'editor-replacement-picker')
  await picker.locator(`button[title="${replacement!.fileName}"]`).click()
  await expect(picker).toHaveCount(0)
  await expect.poll(async () => (await readProject()).timeline.items.find((item: { id: string }) => item.id === targetId).mediaId).not.toBe(target.mediaId)
  const replaced = await readProject()
  const changed = replaced.timeline.items.filter((item: { id: string }) => JSON.stringify(item) !== JSON.stringify(before.timeline.items.find((old: { id: string }) => old.id === item.id)))
  expect(changed.map((item: { id: string }) => item.id)).toEqual([targetId])
  const { mediaId: oldMedia, src: oldSource, ...oldGeometry } = target
  const { mediaId: newMedia, src: newSource, ...newGeometry } = changed[0]
  expect(newGeometry).toEqual(oldGeometry)
  expect(replaced.timeline.keyframes).toEqual(before.timeline.keyframes)
  await page.getByRole('button', { name: 'Hoàn tác', exact: true }).click()
  await expect.poll(async () => (await readProject()).timeline.items).toEqual(before.timeline.items)
  expect((await readProject()).timeline.keyframes).toEqual(before.timeline.keyframes)
  await info.attach('scene-replacement.json', { body: JSON.stringify({ targetId, changedClipIds: changed.map((item: { id: string }) => item.id), geometryPreserved: true, keyframesPreserved: true, undoRestoredPersistedItems: true }, null, 2), contentType: 'application/json' })
})

test('mobile sample preview stays in its editing sheet and Back preserves the unsaved draft', async ({ page }, info) => {
  test.skip(page.viewportSize()!.width >= 768, 'Existing mobile editing surface only; desktop uses the shared viewer')
  const writes: string[] = []
  page.on('request', request => {
    if (request.method() !== 'GET' && new URL(request.url()).pathname.startsWith('/api/image-prompts')) writes.push(request.method() + ' ' + new URL(request.url()).pathname)
  })
  await page.goto('/#/library?tab=prompts')
  await page.getByRole('button').filter({ hasText: 'Ảnh mẫu QA tự tạo · 1' }).click()
  const sheet = page.getByRole('dialog')
  await expect(sheet).toHaveCount(1)
  const name = sheet.getByRole('textbox', { name: 'Tên', exact: true })
  const prompt = sheet.getByRole('textbox', { name: 'Chỉ dẫn', exact: true })
  const oldName = await name.inputValue()
  const oldPrompt = await prompt.inputValue()
  await name.fill(oldName + ' · nháp')
  await prompt.fill(oldPrompt + '\nGiữ ánh sáng lạnh trong bản nháp.')
  const trigger = sheet.getByRole('button', { name: 'Xem ảnh mẫu', exact: true }).first()
  await trigger.click()
  const preview = sheet.getByRole('group', { name: 'Ảnh mẫu', exact: true })
  const back = preview.getByRole('button', { name: 'Quay lại Ảnh mẫu', exact: true })
  await expect(preview).toBeVisible()
  await expect(page.locator('[aria-modal="true"]')).toHaveCount(1)
  await expect(back).toBeFocused()
  expect(await back.evaluate(button => button.getBoundingClientRect().height)).toBeGreaterThanOrEqual(44)
  await expect.poll(() => preview.getByRole('img').evaluate(image => (image as HTMLImageElement).complete && (image as HTMLImageElement).naturalWidth > 0)).toBe(true)
  await page.keyboard.press('Shift+Tab')
  expect(await sheet.evaluate(node => node.contains(document.activeElement))).toBe(true)
  await capture(page, info, 'mobile-sample-same-sheet')
  await back.click()
  await expect(preview).toHaveCount(0)
  await expect(name).toHaveValue(oldName + ' · nháp')
  await expect(prompt).toHaveValue(oldPrompt + '\nGiữ ánh sáng lạnh trong bản nháp.')
  await expect(trigger).toBeFocused()
  await trigger.click()
  await page.keyboard.press('Escape')
  await expect(preview).toHaveCount(0)
  await expect(sheet).toHaveCount(1)
  await expect(prompt).toHaveValue(oldPrompt + '\nGiữ ánh sáng lạnh trong bản nháp.')
  await expect(trigger).toBeFocused()
  expect(writes).toEqual([])
  await capture(page, info, 'mobile-sample-returned-draft')
  await info.attach('mobile-preview-receipt.json', { body: JSON.stringify({ oneModal: true, decodedImage: true, unsavedNamePreserved: true, unsavedPromptPreserved: true, focusReturned: true, escapeReturnsToDraft: true, providerCalls: 0, writes }, null, 2), contentType: 'application/json' })
})

test('real two-version history previews without mutation and persists only the explicitly used version', async ({ page }, info) => {
  test.skip(process.env.WORKFLOW_QA_TWO_VERSIONS !== '1', 'Requires separately authorized, captured two-version QA history; does not generate images')
  const job = fixture.jobs[0]
  const endpoint = `/api/clone/${job.id}/images`
  const payload = z.object({ images: z.array(z.object({
    id: z.string(), usedVersion: z.number(),
    versions: z.array(z.object({ n: z.number(), state: z.string(), provider: z.string().optional(), costImages: z.number().optional(), url: z.string() })),
  })) })
  const readImage = async () => {
    const response = await page.request.get(endpoint)
    expect(response.status()).toBe(200)
    const image = payload.parse(await response.json()).images.find(image => image.id === 'sc-04')
    if (!image) throw new Error('Two-version QA image is missing')
    return image
  }
  const before = await readImage()
  const version = (number: number) => {
    const entry = before.versions.find(candidate => candidate.n === number)
    if (!entry) throw new Error(`Required QA version ${number} is missing`)
    return entry
  }
  expect(version(2)).toMatchObject({ state: 'ready', provider: 'chatgpt', costImages: 1 })
  const mutations: string[] = []
  page.on('request', request => {
    if (request.method() !== 'GET' && new URL(request.url()).pathname.startsWith(endpoint)) mutations.push(new URL(request.url()).pathname)
  })
  await page.goto(`/#/review?video=${job.id}&img=sc-04`)
  const first = page.getByRole('button', { name: /^Bản 1(?: ·|$)/ })
  const second = page.getByRole('button', { name: /^Bản 2(?: ·|$)/ })
  const hero = page.getByRole('img', { name: /^Cảnh 0?4$/, exact: true })
  for (const [number, button] of [[1, first], [2, second]] as const) {
    await button.click()
    await expect(button).toHaveAttribute('aria-pressed', 'true')
    const source = version(number).url
    await expect.poll(() => hero.getAttribute('src')).toBe(source)
    await expect.poll(() => hero.evaluate(image => (image as HTMLImageElement).complete && (image as HTMLImageElement).naturalWidth > 0)).toBe(true)
    await capture(page, info, `real-version-${number}-preview`)
  }
  expect(mutations).toEqual([])
  expect((await readImage()).usedVersion).toBe(before.usedVersion)
  for (const [number, button] of [[1, first], [2, second]] as const) {
    await button.click()
    const current = await readImage()
    if (current.usedVersion !== number) {
      await page.getByRole('button', { name: new RegExp(`^Dùng bản ${number}(?: cho video)?$`) }).click()
      await expect.poll(async () => (await readImage()).usedVersion).toBe(number)
    }
  }
  await page.reload()
  await expect(second).toHaveAttribute('aria-pressed', 'true')
  await expect.poll(() => hero.getAttribute('src')).toBe(version(2).url)
  await capture(page, info, 'real-version-2-persisted')
  await info.attach('two-version-receipt.json', { body: JSON.stringify({ source: 'one authorized real ChatGPT draw plus authored legacy version', previewWrites: 0, persistedUsedVersion: 2, newDrawCalls: 0, viewport: page.viewportSize() }, null, 2), contentType: 'application/json' })
})

test('real score explanation keeps long labels and values in separate columns', async ({ page }, info) => {
  test.skip(process.env.WORKFLOW_QA_TWO_VERSIONS !== '1', 'Requires the captured real model score, not synthetic explanation text')
  await page.goto(`/#/review?video=${fixture.jobs[0].id}&img=sc-04`)
  if (page.viewportSize()!.width < 768) await page.getByRole('button', { name: /^Vì sao\??$/ }).click()
  const details = page.getByRole('dialog', { name: /^Vì sao\??$/ })
  await expect(details).toBeVisible()
  const rows = await details.evaluate(panel => Array.from(panel.querySelectorAll('b')).map(value => {
    const row = value.parentElement!
    const label = row.querySelector(':scope > span')!
    const labelBox = label.getBoundingClientRect()
    const valueBox = value.getBoundingClientRect()
    const rowBox = row.getBoundingClientRect()
    return { label: label.textContent, value: value.textContent, labelRight: labelBox.right, valueLeft: valueBox.left, valueRight: valueBox.right, rowRight: rowBox.right, valueScroll: value.scrollWidth, valueWidth: value.clientWidth }
  }))
  expect(rows.some(row => (row.value?.length ?? 0) > 40)).toBe(true)
  for (const row of rows) {
    expect(row.labelRight, `${row.label}: label overlaps value`).toBeLessThanOrEqual(row.valueLeft + 2)
    expect(row.valueRight, `${row.label}: value leaves its row`).toBeLessThanOrEqual(row.rowRight + 2)
    expect(row.valueScroll, `${row.label}: value overflows instead of wrapping`).toBeLessThanOrEqual(row.valueWidth + 2)
  }
  await capture(page, info, 'real-score-long-facts')
  await info.attach('score-layout.json', { body: JSON.stringify(rows, null, 2), contentType: 'application/json' })
})

test('real cover history previews in one surface and Back preserves the cover choices', async ({ page }, info) => {
  test.skip(process.env.WORKFLOW_QA_REAL_COVER !== '1', 'Requires captured authorized real cover history; this test never draws')
  const endpoint = `/api/templates/${fixture.templates[0].id}/cover-trials`
  const response = await page.request.get(endpoint)
  expect(response.status()).toBe(200)
  const status = await response.json() as { history: { id: string; state: string; promptName: string; width: number; height: number; costImages: number; imageUrl: string }[] }
  const sample = status.history.find(row => row.state === 'ready' && row.costImages === 1)
  expect(sample, 'A real completed cover trial is required').toBeDefined()
  const writes: string[] = []
  page.on('request', request => {
    if (request.method() !== 'GET' && new URL(request.url()).pathname.startsWith(`/api/templates/${fixture.templates[0].id}`)) writes.push(request.method() + ' ' + new URL(request.url()).pathname)
  })
  await page.goto(`/#/templates/${fixture.templates[0].id}?section=cover`)
  const source = page.getByRole('combobox', { name: 'Chọn video đã làm xong để vẽ thử ảnh bìa', exact: true })
  await source.selectOption(fixture.jobs[0].id)
  const chosenBefore = await page.getByRole('radio', { checked: true }).allTextContents()
  const trigger = page.getByRole('button', { name: 'Xem ảnh', exact: true }).first()
  await trigger.click()
  if (page.viewportSize()!.width < 768) {
    const preview = page.getByRole('group', { name: sample!.promptName, exact: true })
    await expect(preview).toBeVisible()
    await expect(page.locator('[aria-modal="true"]')).toHaveCount(0)
    const back = preview.getByRole('button', { name: 'Quay lại Ảnh bìa', exact: true })
    await expect(back).toBeFocused()
    expect(await back.evaluate(button => button.getBoundingClientRect().height)).toBeGreaterThanOrEqual(44)
    await expect(preview.getByRole('img')).toHaveAttribute('src', sample!.imageUrl)
    await expect.poll(() => preview.getByRole('img').evaluate((image, dimensions) => (image as HTMLImageElement).complete && (image as HTMLImageElement).naturalWidth === dimensions.width && (image as HTMLImageElement).naturalHeight === dimensions.height, sample!)).toBe(true)
    await capture(page, info, 'real-cover-mobile-same-surface')
    await back.click()
    await expect(preview).toHaveCount(0)
  } else {
    const viewer = page.getByRole('dialog')
    await expect(viewer).toBeVisible()
    const stageImage = viewer.getByRole('img').first()
    await expect(stageImage).toHaveAttribute('src', sample!.imageUrl)
    await expect.poll(() => stageImage.evaluate((image, dimensions) => (image as HTMLImageElement).complete && (image as HTMLImageElement).naturalWidth === dimensions.width && (image as HTMLImageElement).naturalHeight === dimensions.height, sample!)).toBe(true)
    await capture(page, info, 'real-cover-desktop-viewer')
    await page.keyboard.press('Escape')
    await expect(viewer).toHaveCount(0)
  }
  await expect(trigger).toBeFocused()
  await expect(source).toHaveValue(fixture.jobs[0].id)
  expect(await page.getByRole('radio', { checked: true }).allTextContents()).toEqual(chosenBefore)
  expect(writes).toEqual([])
  await capture(page, info, 'real-cover-returned-choices')
  await info.attach('cover-preview-receipt.json', { body: JSON.stringify({ trialId: sample!.id, source: 'authorized actual provider cover trial', writes, selectedSourcePreserved: true, coverChoicesPreserved: true, focusReturned: true, viewport: page.viewportSize() }, null, 2), contentType: 'application/json' })
})

test('mobile review tab returns to the addressed image in its kept pane', async ({ page }, info) => {
  test.skip(page.viewportSize()!.width >= 768 || process.env.WORKFLOW_QA_NAV_REPLAY !== '1', 'Requires isolated recorded-source replay jobs on phone')
  const replay = fixture as typeof fixture & { navigationReplays: { id: string }[] }
  const target = replay.navigationReplays[0].id
  await page.goto(`/#/review?video=${target}&img=sc-03`)
  await expect(page.locator('main:visible').getByRole('img', { name: /^Cảnh 0?3$/ })).toBeVisible()
  await page.getByRole('navigation').getByRole('button', { name: 'Bàn dựng', exact: true }).click()
  await page.getByRole('navigation').getByRole('button', { name: 'Duyệt', exact: true }).click()
  await expect(page).toHaveURL(new RegExp(`video=${target}.*img=sc-03`))
  await expect(page.locator('main:visible').getByRole('img', { name: /^Cảnh 0?3$/ })).toBeVisible()
  await capture(page, info, 'mobile-review-tab-kept-address')
})

test('mobile workbench opens the requested review job after the review pane was hidden', async ({ page }, info) => {
  test.skip(page.viewportSize()!.width >= 768 || process.env.WORKFLOW_QA_NAV_REPLAY !== '1', 'Requires isolated recorded-source replay jobs on phone')
  const replay = fixture as typeof fixture & { navigationReplays: { id: string }[] }
  const [first, target] = replay.navigationReplays
  const writes: string[] = []
  page.on('request', request => { if (request.method() === 'POST') writes.push(new URL(request.url()).pathname) })
  await page.goto(`/#/review?video=${first.id}&img=sc-03`)
  await expect(page.locator('main:visible').getByRole('img', { name: /^Cảnh 0?3$/ })).toBeVisible()
  await page.getByRole('navigation').getByRole('button', { name: 'Bàn dựng', exact: true }).click()
  await page.getByText('QA replay B · saved source, no new AI', { exact: true }).filter({ visible: true }).click()
  await page.getByRole('button', { name: /^Duyệt \d+ ảnh$/ }).click()
  await expect(page).toHaveURL(new RegExp(`video=${target.id}`))
  await expect(page.locator('main:visible')).toContainText('QA replay B · saved source, no new AI')
  await expect(page.locator('main:visible')).not.toContainText('QA replay A · saved source, no new AI')
  expect(writes).toEqual([])
  await capture(page, info, 'mobile-review-workbench-correct-target')
})

test('stale review video address returns to a valid review job with explicit feedback and no writes', async ({ page }, info) => {
  test.skip(process.env.WORKFLOW_QA_NAV_REPLAY !== '1', 'Requires completed original source and two reviewable recorded-source QA replays')
  const replay = fixture as typeof fixture & { navigationReplays: { id: string }[] }
  const original = await page.request.get(`/api/clone/${fixture.jobs[0].id}`)
  expect(original.status()).toBe(200)
  expect((await original.json()).status).toBe('succeeded')
  const writes: string[] = []
  page.on('request', request => { if (request.method() !== 'GET' && new URL(request.url()).pathname.startsWith('/api/clone')) writes.push(request.method() + ' ' + new URL(request.url()).pathname) })
  await page.goto(`/#/review?video=${fixture.jobs[0].id}&img=sc-03`)
  await expect(page).toHaveURL(/#\/review$/)
  await expect(page.getByRole('alert')).toContainText('Video đã chọn không có trong danh sách chờ duyệt.')
  await expect(page.locator('main:visible')).toContainText(/QA replay [AB] · saved source, no new AI/)
  await expect(page.locator('main:visible').getByRole('img').first()).toBeVisible()
  expect(writes).toEqual([])
  await capture(page, info, 'stale-review-address-explicit-fallback')
  await info.attach('stale-target-receipt.json', { body: JSON.stringify({ missingReviewTarget: fixture.jobs[0].id, allowedFallbackTargets: replay.navigationReplays.map(row => row.id), canonicalAddress: new URL(page.url()).hash, writes }, null, 2), contentType: 'application/json' })
})

for (const surface of ['prompt', 'cover'] as const) {
  test(`${surface} sample confirmation cancellation restores its viewer for button Escape and scrim`, async ({ page }, info) => {
    test.skip(page.viewportSize()!.width < 768 || process.env.WORKFLOW_QA_CONFIRM_CANCEL !== '1', 'Desktop sample plus explicit zero-write confirmation guard required')
    const attemptedWrites: string[] = []
    await page.route('**/api/**', async route => {
      if (!['GET', 'HEAD', 'OPTIONS'].includes(route.request().method())) {
        attemptedWrites.push(route.request().method() + ' ' + new URL(route.request().url()).pathname)
        await route.abort('blockedbyclient')
      } else await route.continue()
    })
    if (surface === 'prompt') {
      await page.goto('/#/library?tab=prompts')
      await page.getByRole('button').filter({ hasText: 'Ảnh mẫu QA tự tạo · 1' }).click()
      await page.getByRole('button', { name: 'Xem ảnh mẫu', exact: true }).first().click()
    } else {
      await page.goto(`/#/templates/${fixture.templates[0].id}?section=cover`)
      await page.getByRole('button', { name: 'Xem ảnh', exact: true }).first().click()
    }
    const viewer = page.getByRole('dialog')
    await expect(viewer).toHaveCount(1)
    const originalURL = await viewer.getByRole('img').first().getAttribute('src')
    for (const mode of ['button', 'escape', 'scrim'] as const) {
      await viewer.getByRole('button', { name: /^Vẽ thử · / }).click()
      const confirmation = page.getByRole('alertdialog')
      await expect(confirmation).toHaveCount(1)
      await expect(viewer).toHaveCount(0)
      for (let index = 0; index < 5; index++) {
        await page.keyboard.press(index % 2 ? 'Shift+Tab' : 'Tab')
        expect(await confirmation.evaluate(node => node.contains(document.activeElement))).toBe(true)
      }
      await capture(page, info, `${surface}-sample-confirm-${mode}`)
      if (mode === 'button') await confirmation.getByRole('button', { name: 'Huỷ', exact: true }).click()
      else if (mode === 'escape') await page.keyboard.press('Escape')
      else await page.mouse.click(8, 450)
      await expect(confirmation).toHaveCount(0)
      await expect(viewer).toHaveCount(1)
      await expect(viewer.getByRole('img').first()).toHaveAttribute('src', originalURL!)
      expect(await viewer.evaluate(node => node.contains(document.activeElement))).toBe(true)
      expect(attemptedWrites).toEqual([])
      await capture(page, info, `${surface}-sample-cancel-${mode}-restored`)
    }
    await info.attach('no-paid-request.json', { body: JSON.stringify({ surface, modes: ['button', 'escape', 'scrim'], attemptedWrites, browserBlocksAllApiMutations: true, providerImageCalls: 0 }, null, 2), contentType: 'application/json' })
  })
}

test('mobile real cover preview Escape returns to its settings and trigger', async ({ page }, info) => {
  test.skip(page.viewportSize()!.width >= 768 || process.env.WORKFLOW_QA_REAL_COVER !== '1', 'Existing real cover history on phone required')
  const writes: string[] = []
  page.on('request', request => { if (request.method() !== 'GET' && new URL(request.url()).pathname.startsWith('/api/templates')) writes.push(new URL(request.url()).pathname) })
  await page.goto(`/#/templates/${fixture.templates[0].id}?section=cover`)
  const trigger = page.getByRole('button', { name: 'Xem ảnh', exact: true }).first()
  await trigger.click()
  await expect(page.getByRole('button', { name: 'Quay lại Ảnh bìa', exact: true })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: 'Quay lại Ảnh bìa', exact: true })).toHaveCount(0)
  await expect(trigger).toBeFocused()
  await expect(page.getByRole('combobox', { name: 'Chọn video đã làm xong để vẽ thử ảnh bìa', exact: true })).toBeVisible()
  expect(writes).toEqual([])
  await capture(page, info, 'real-cover-mobile-escape-preserved-settings')
})

test('logout cancellation keeps the shared session without sending a revoke request', async ({ page, context }, info) => {
  const writes: string[] = []
  await page.route('**/api/**', route => {
    const request = route.request()
    if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method())) {
      writes.push(`${request.method()} ${new URL(request.url()).pathname}`)
      return route.abort('blockedbyclient')
    }
    return route.continue()
  })
  await page.goto('/#/account')
  const logout = page.getByRole('button', { name: 'Đăng xuất', exact: true })
  if (page.viewportSize()!.width >= 768) {
    await expect.poll(async () => new URL(page.url()).hash === '#/setup' || await logout.isVisible()).toBe(true)
    if (new URL(page.url()).hash === '#/setup') {
      await page.getByRole('link', { name: 'Cài đặt nâng cao', exact: true }).click()
      await page.goto('/#/account')
    }
  }
  await logout.click()
  const confirmation = page.getByRole('alertdialog')
  await expect(confirmation).toBeVisible()
  await expect(confirmation).toContainText('Noverse')
  await expect(confirmation).toContainText('Viderse')
  await capture(page, info, 'logout-shared-session-confirmation')
  await confirmation.getByRole('button', { name: 'Huỷ', exact: true }).click()
  await expect(confirmation).toHaveCount(0)
  await expect(logout).toBeVisible()
  const me = await context.request.get('/api/me')
  expect(me.status()).toBe(200)
  expect((await me.json()).user.username).toBe('workflow.qa')
  expect(writes.filter(write => write !== 'POST /api/setup/start')).toEqual([])
  await info.attach('logout-no-revoke.json', { body: JSON.stringify({ blockedStartupRequests: writes, logoutRequests: writes.filter(write => write.endsWith(' /api/logout')), sessionRetained: true }), contentType: 'application/json' })
  await capture(page, info, 'logout-cancel-session-preserved')
})

test('exhausted real image quota locks drawing but preserves saved cover preview', async ({ page }, info) => {
  test.skip(process.env.WORKFLOW_QA_EXHAUSTED_QUOTA !== '1', 'Requires real two-image usage and isolated daily image limit2')
  const statusResponse = await page.request.get(`/api/templates/${fixture.templates[0].id}/cover-trials`)
  expect(statusResponse.status()).toBe(200)
  const status = await statusResponse.json()
  expect(status.quota).toMatchObject({ paid: true, left: 0, total: 2 })
  expect(status.ready).toBe(false)
  expect(status.blockedReason).toContain('Đã hết hạn mức ảnh hôm nay')
  const sample = status.history.find((row: { state: string; imageUrl?: string }) => row.state === 'ready' && row.imageUrl)
  expect(sample).toBeTruthy()
  const writes: string[] = []
  await page.route('**/api/**', route => {
    const request = route.request()
    if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method())) {
      writes.push(`${request.method()} ${new URL(request.url()).pathname}`)
      return route.abort('blockedbyclient')
    }
    return route.continue()
  })
  await page.goto(`/#/templates/${fixture.templates[0].id}?section=cover`)
  const draw = page.getByRole('button', { name: /^Vẽ thử · / }).first()
  await expect(draw).toHaveAttribute('aria-disabled', 'true')
  await expect(page.getByText(status.blockedReason, { exact: true }).first()).toBeVisible()
  await draw.click({ force: true })
  await expect(page.getByRole('alertdialog')).toHaveCount(0)
  await capture(page, info, 'real-quota-exhausted-drawing-locked')
  const preview = page.getByRole('button', { name: 'Xem ảnh', exact: true }).first()
  await preview.click()
  const image = page.viewportSize()!.width < 768
    ? page.locator('img').filter({ visible: true }).last()
    : page.getByRole('dialog').getByRole('img').first()
  await expect(image).toHaveAttribute('src', sample.imageUrl)
  expect(await image.evaluate(node => (node as HTMLImageElement).complete && (node as HTMLImageElement).naturalWidth > 0)).toBe(true)
  await capture(page, info, 'real-quota-exhausted-existing-sample-readable')
  if (page.viewportSize()!.width < 768) await page.getByRole('button', { name: 'Quay lại Ảnh bìa', exact: true }).click()
  else await page.keyboard.press('Escape')
  await expect(preview).toBeFocused()
  expect(writes).toEqual([])
  await info.attach('exhausted-quota-no-image-request.json', { body: JSON.stringify({ quota: status.quota, blockedReason: status.blockedReason, savedImageURL: sample.imageUrl, writes, providerCalls: 0, ledgerFabricated: false }), contentType: 'application/json' })
})

test('production confirmation chunk failure is visible and cannot revoke the session', async ({ page, context }, info) => {
  test.skip(process.env.WORKFLOW_QA_CONFIRM_CHUNK_FAULT !== '1', 'Production assets and fresh account-route context required')
  const writes: string[] = []
  const chunkRequests: string[] = []
  let rejectChunk = true
  page.on('request', request => {
    if (/\/assets\/shared-controls-[^/]+\.js$/.test(new URL(request.url()).pathname)) chunkRequests.push(new URL(request.url()).pathname)
  })
  await page.route('**/api/**', route => {
    const request = route.request()
    if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method())) {
      writes.push(`${request.method()} ${new URL(request.url()).pathname}`)
      return route.abort('blockedbyclient')
    }
    return route.continue()
  })
  await page.goto('/#/account')
  const logout = page.getByRole('button', { name: 'Đăng xuất', exact: true })
  await expect(logout).toBeVisible()
  await settle(page)
  expect(chunkRequests).toEqual([])
  await page.route('**/assets/shared-controls-*.js', route => rejectChunk ? route.abort('failed') : route.continue())
  await logout.click()
  await expect(page.getByRole('alert')).toContainText('Không mở được hộp thoại xác nhận.')
  await expect(page.getByRole('alertdialog')).toHaveCount(0)
  expect(chunkRequests).toHaveLength(1)
  expect(writes).toEqual([])
  expect((await (await context.request.get('/api/me')).json()).user.username).toBe('workflow.qa')
  // Wait only for the toast entrance, not its live countdown transitions.
  await page.getByRole('alert').evaluate(async node => {
    const wrap = node.closest('[data-toast-id]') ?? node
    await Promise.all(wrap.getAnimations({ subtree: true })
      .filter(animation => animation instanceof CSSAnimation && ['ui-toast-in', 'vv-tIn'].includes(animation.animationName))
      .map(animation => animation.finished.catch(() => undefined)))
  })
  const feedbackGeometry = await page.getByRole('alert').evaluate(node => {
    const action = [...node.querySelectorAll('button')].find(button => button.textContent === 'Tải lại')!
    const message = [...node.children].find(child => child.tagName === 'SPAN' && child.textContent?.startsWith('Không mở được')) as HTMLElement
    const range = document.createRange()
    range.selectNodeContents(message)
    const textRight = range.getBoundingClientRect().right
    const clipped = ['hidden', 'clip'].includes(getComputedStyle(message).overflowX)
    return {
      paintedTextRight: clipped ? Math.min(textRight, message.getBoundingClientRect().right) : textRight,
      actionLeft: action.getBoundingClientRect().left,
      toastLeft: node.getBoundingClientRect().left,
      toastRight: node.getBoundingClientRect().right,
    }
  })
  expect(feedbackGeometry.paintedTextRight).toBeLessThanOrEqual(feedbackGeometry.actionLeft)
  expect(feedbackGeometry.toastLeft).toBeGreaterThanOrEqual(0)
  expect(feedbackGeometry.toastRight).toBeLessThanOrEqual(page.viewportSize()!.width)
  await page.screenshot({ path: info.outputPath('production-confirmation-load-error-session-retained.png') })
  rejectChunk = false
  await page.getByRole('button', { name: 'Tải lại', exact: true }).click()
  await expect(logout).toBeVisible()
  await logout.click()
  const confirmation = page.getByRole('alertdialog')
  await expect(confirmation).toBeVisible()
  await confirmation.getByRole('button', { name: 'Huỷ', exact: true }).click()
  await expect(confirmation).toHaveCount(0)
  expect(writes).toEqual([])
  expect((await (await context.request.get('/api/me')).json()).user.username).toBe('workflow.qa')
  await capture(page, info, 'production-confirmation-reload-recovers')
  await info.attach('production-confirmation-chunk-failure.json', { body: JSON.stringify({ chunkRequests, writes, sessionRetained: true, recovery: 'Native reload action followed by a real confirmation and cancellation' }), contentType: 'application/json' })
})
