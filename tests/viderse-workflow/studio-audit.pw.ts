import { test, expect, type Page } from '@playwright/test'
import { createHash } from 'node:crypto'
import { readFileSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'
import { inflateRawSync } from 'node:zlib'

const repo = resolve(__dirname, '../..')
const cookiePath = process.env.WORKFLOW_QA_COOKIE_FILE
if (!cookiePath || realpathSync(cookiePath).startsWith(repo + sep)) throw new Error('A private isolated QA session is required')

// Go's archive/zip writes a central directory, including when local records use
// data descriptors. Read that directory rather than guessing entry boundaries.
function zipEntries(bytes: Buffer) {
  let end = bytes.length - 22
  while (end >= Math.max(0, bytes.length - 65557) && bytes.readUInt32LE(end) !== 0x06054b50) end--
  expect(end).toBeGreaterThanOrEqual(0)
  const count = bytes.readUInt16LE(end + 10)
  let offset = bytes.readUInt32LE(end + 16)
  const entries: { name: string; body: Buffer }[] = []
  for (let index = 0; index < count; index++) {
    expect(bytes.readUInt32LE(offset)).toBe(0x02014b50)
    const method = bytes.readUInt16LE(offset + 10)
    const size = bytes.readUInt32LE(offset + 20)
    const unpacked = bytes.readUInt32LE(offset + 24)
    const nameLength = bytes.readUInt16LE(offset + 28)
    const extraLength = bytes.readUInt16LE(offset + 30)
    const commentLength = bytes.readUInt16LE(offset + 32)
    const local = bytes.readUInt32LE(offset + 42)
    const name = bytes.toString('utf8', offset + 46, offset + 46 + nameLength)
    expect(bytes.readUInt32LE(local)).toBe(0x04034b50)
    const start = local + 30 + bytes.readUInt16LE(local + 26) + bytes.readUInt16LE(local + 28)
    const compressed = bytes.subarray(start, start + size)
    expect([0, 8]).toContain(method)
    const body = method === 8 ? inflateRawSync(compressed) : compressed
    expect(body.length).toBe(unpacked)
    expect(name.startsWith('/')).toBe(false)
    expect(name.split('/')).not.toContain('..')
    entries.push({ name, body })
    offset += 46 + nameLength + extraLength + commentLength
  }
  expect(new Set(entries.map(entry => entry.name)).size).toBe(entries.length)
  return entries
}

async function settle(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise<void>(done => requestAnimationFrame(() => requestAnimationFrame(() => done())))
  })
  await page.waitForFunction(() => document.getAnimations().filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime)).every(animation => animation.playState === 'finished' || animation.playState === 'idle'))
}

test('studio bulk selection downloads both genuine ready exports without overwriting same-title files', async ({ context, page }, info) => {
  test.skip(process.env.WORKFLOW_QA_READY_STUDIO !== '1', 'Requires the normal-media ready replay controller')
  const ids = (process.env.WORKFLOW_QA_READY_REPLAYS ?? '').split(',')
  expect(ids).toHaveLength(2)
  expect(ids.every(id => /^[a-f0-9-]{36}$/.test(id))).toBe(true)
  const expectedBytes = Number(process.env.WORKFLOW_QA_READY_EXPORT_SIZE)
  expect(Number.isSafeInteger(expectedBytes) && expectedBytes > 0).toBe(true)
  expect(page.viewportSize()!.width).toBeGreaterThanOrEqual(768)
  await context.addCookies(JSON.parse(readFileSync(cookiePath, 'utf8')))
  const me = await context.request.get('/api/me')
  expect(me.status()).toBe(200)
  expect((await me.json()).user.username).toBe('workflow.qa')
  const faults: string[] = []
  const refused: string[] = []
  page.on('pageerror', error => faults.push(error.message))
  await page.route('**/*', route => {
    const request = route.request()
    const url = new URL(request.url())
    if (!['http:', 'https:'].includes(url.protocol)) return route.continue()
    if (url.origin !== 'http://127.0.0.1:8043' || !['GET', 'HEAD'].includes(request.method())) {
      refused.push(`${request.method()} ${url.origin}${url.pathname}`)
      return route.abort('blockedbyclient')
    }
    return route.continue()
  })
  const jobs: { id: string; source: { title: string }; stageGroup: string }[] = []
  for (const id of ids) {
    const response = await context.request.get(`/api/clone/${id}`)
    expect(response.status()).toBe(200)
    const job = await response.json()
    expect(job.id).toBe(id)
    expect(job.stageGroup).toBe('ready')
    jobs.push(job)
  }
  await page.goto(`/#/?video=${ids[0]}`, { waitUntil: 'domcontentloaded' })
  const videos = page.getByRole('complementary', { name: 'Video mọi kênh', exact: true })
  await expect(videos.getByRole('button', { name: jobs[0].source.title, exact: false })).toBeVisible()
  const inspector = page.getByRole('complementary', { name: 'Bảng bên phải', exact: true })
  await expect(inspector.getByRole('tab', { name: 'Thông tin', exact: true })).toHaveAttribute('aria-selected', 'true')
  await expect(inspector.getByRole('tab')).toHaveCount(2)
  await videos.getByRole('button', { name: 'Chọn', exact: true }).click()
  const selected = videos.getByRole('region', { name: 'Video đã chọn', exact: true })
  await expect(selected.getByRole('button', { name: 'Tải gói', exact: true })).toBeDisabled()
  for (const job of jobs) {
    const checkbox = videos.getByRole('checkbox', { name: job.source.title, exact: false })
    await checkbox.click()
    await expect(checkbox).toBeChecked()
  }
  const downloadLink = selected.getByRole('link', { name: 'Tải gói 2 video', exact: true })
  const href = new URL((await downloadLink.getAttribute('href'))!, page.url())
  expect(href.pathname).toBe('/api/clone/download')
  expect(href.searchParams.get('ids')!.split(',').sort()).toEqual([...ids].sort())
  await settle(page)
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
  await page.screenshot({ path: info.outputPath('studio-two-ready-selected.png') })
  const [download] = await Promise.all([page.waitForEvent('download'), downloadLink.click()])
  expect(await download.failure()).toBeNull()
  const file = await download.path()
  expect(file).not.toBeNull()
  const entries = zipEntries(readFileSync(file!))
  const movies = entries.filter(entry => entry.name.endsWith('.mp4'))
  expect(movies).toHaveLength(2)
  const folders = movies.map(entry => entry.name.slice(0, entry.name.lastIndexOf('/')))
  expect(new Set(folders).size).toBe(2)
  expect(folders.some(folder => folder.endsWith(' (2)'))).toBe(true)
  for (const movie of movies) {
    expect(movie.body.length).toBe(expectedBytes)
    expect(movie.body.toString('ascii', 4, 8)).toBe('ftyp')
  }
  const movieHashes = movies.map(entry => createHash('sha256').update(entry.body).digest('hex'))
  expect(movieHashes[0]).toBe(movieHashes[1])
  for (const folder of folders) {
    expect(entries.find(entry => entry.name === `${folder}/thumbnail.jpg`)?.body.subarray(0, 2).toString('hex')).toBe('ffd8')
    expect(entries.find(entry => entry.name === `${folder}/thong-tin.txt`)?.body.toString('utf8')).toMatch(/^TIÊU ĐỀ\n.+\n\nMÔ TẢ\n/s)
  }
  await expect(selected).toHaveCount(0)
  await expect(videos.getByRole('checkbox')).toHaveCount(0)
  await videos.getByRole('button', { name: jobs[1].source.title, exact: false }).click()
  await expect(page).toHaveURL(new RegExp(`video=${ids[1]}`))
  await expect(inspector.getByRole('tabpanel').getByText(jobs[1].source.title, { exact: true })).toBeVisible()
  const movie = page.locator('video')
  await expect(movie).toHaveCount(1)
  await expect.poll(() => movie.evaluate((element: HTMLVideoElement) => element.readyState)).toBeGreaterThanOrEqual(2)
  await movie.evaluate((element: HTMLVideoElement) => { element.muted = true })
  await page.getByRole('button', { name: 'Phát video', exact: true }).click()
  await expect.poll(() => movie.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeGreaterThan(0)
  const playback = await movie.evaluate((element: HTMLVideoElement) => ({ duration: element.duration, width: element.videoWidth, height: element.videoHeight, paused: element.paused }))
  expect(playback.duration).toBeGreaterThan(0)
  expect(playback.width).toBeGreaterThan(0)
  expect(playback.height).toBeGreaterThan(0)
  expect(playback.paused).toBe(false)
  await page.getByRole('button', { name: 'Tạm dừng video', exact: true }).click()
  await expect.poll(() => movie.evaluate((element: HTMLVideoElement) => element.paused)).toBe(true)
  const seek = page.getByRole('slider', { name: 'Vị trí phát video', exact: true })
  await seek.focus()
  await seek.press('End')
  await expect.poll(() => movie.evaluate((element: HTMLVideoElement) => Math.abs(element.currentTime - element.duration))).toBeLessThan(0.1)
  await seek.press('Home')
  await expect.poll(() => movie.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeLessThan(0.1)
  const frame = await movie.evaluate(element => {
    const box = element.parentElement!.getBoundingClientRect()
    return { width: box.width, height: box.height }
  })
  expect(Math.abs(frame.width / frame.height - 16 / 9)).toBeLessThan(0.02)
  await settle(page)
  await page.screenshot({ path: info.outputPath('studio-ready-info.png') })
  expect(faults).toEqual([])
  expect(refused).toEqual([])
  await info.attach('real-bulk-download.json', { body: JSON.stringify({ ids, movies: movies.map((entry, index) => ({ name: entry.name, bytes: entry.body.length, sha256: movieHashes[index] })), entries: entries.map(entry => entry.name), blockedRequests: refused, runtimeErrors: faults, replayPolicy: 'Two real aliases of one completed source export; no new rendering or provider call' }, null, 2), contentType: 'application/json' })
})
