// @vitest-environment happy-dom
// Khám phá trên điện thoại: dải đối thủ + chọn nhiều thay thanh tab, thêm đối
// thủ thật rồi liên kết kênh, và màn đẩy chi tiết video (?video=) với Vì sao?,
// chọn kênh có plan thật, cùng trạng thái "không tìm thấy" trung thực.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { invalidate } from '@noverse/ui/data'
import { ToastHost } from '@noverse/ui'
import { ExploreScreen } from '../ExploreScreen'
import { keys } from '../../../data/keys'
import { t } from '../../../i18n/explore'
import type { Channel } from '../../../api/channels'
import type { CloneEnqueueResult, ClonePlan } from '../../../api/clone'
import type { Rival, RivalList, ScanStatus, Video, VideoPage, VideoPreview } from '../../../api/rivals'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const tt = t.screens.explore

const RIVAL: Rival = {
  id: 'r1', handle: '@motphim', title: 'Một Phim', avatarUrl: null,
  url: 'https://youtube.com/@motphim', subscriberCount: 1_240_000,
  videoCount: 1284, medianViews: 286_000, outliers: 42, newOutliers: 4,
  addedAt: '2026-10-20T00:00:00Z', scannedAt: '2026-10-29T08:00:00Z',
  scan: { state: 'idle', discoveryRunning: false },
}
const RIVAL2: Rival = { ...RIVAL, id: 'r2', handle: '@phimhay', title: 'Phim Hay', newOutliers: 0 }

const video = (i: number): Video => ({
  id: `v${i}`, channelId: 'r1', title: `Video số ${i}`, durationSec: 2391,
  viewCount: 1_200_000, publishedAt: '2026-10-28T04:00:00Z', exactTime: true,
  score: i === 0 ? 4.2 : 2.1, hasCaptions: true,
  scoreReason: i === 0 ? { agent: 'scout', text: 'Tổng lượt xem gấp 4,2 lần trung vị.', facts: [{ label: 'Trung vị kênh', value: '285 N', source: '150 video gần nhất' }] } : undefined,
  thumbnailUrl: `https://i.ytimg.com/vi/v${i}/mqdefault.jpg`, url: `https://youtu.be/v${i}`,
  usage: i === 0
    ? [{ channelId: 'c1', channelName: 'Midnight Tales', stage: 'working', jobId: 'j1', queueItemId: null, updatedAt: '2026-11-01T00:00:00Z' }]
    : undefined,
})

const scan: ScanStatus = {
  channelId: 'r1', historyCap: 2000, indexedVideos: 1284, rescanHours: 6,
  history: { state: 'idle', jobId: null, scanned: 0, added: 0, pages: 0, reachedEnd: false, startedAt: null, updatedAt: null, nextAttemptAt: null, errorCode: null, error: null, recovery: null },
  discovery: { checkedAt: null, caughtUp: true, running: false, nextAt: null },
}

const channel = (id: string, name: string): Channel => ({
  id, name, youtubeUrl: null, language: 'vi', styleNote: null, descriptionFooter: null,
  alwaysTags: [], autoFetch: true, autoApprove: false, perDay: 5, defaultTemplateId: 't1', defaultTemplateName: 'Midnight',
  agentInstructions: {},
  branding: {} as unknown as Channel['branding'],
  avatarUrl: null, bannerUrl: null, sources: [],
  identity: { state: 'ready' } as Channel['identity'],
  createdAt: '2026-10-20T00:00:00Z', updatedAt: '2026-10-20T00:00:00Z',
})

const plan: ClonePlan = {
  accepted: [{ url: 'https://youtu.be/v0', videoId: 'v0', title: 'Video số 0', durationSec: 2391, outlier: 4.2, hasCaptions: true }],
  duplicates: [], invalid: [], position: 'front', etaStart: '2026-11-02T03:10:00Z',
  paceLeftToday: 5, overflowDays: 0, template: { id: 't1', name: 'Midnight' },
  imageCalls: 4, priceUSD: null,
}

const respond = (body: unknown, status = 200) =>
  Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } }))

const calls: { url: string; method: string; body: unknown }[] = []
let listBody: RivalList = { rivals: [RIVAL, RIVAL2], totalVideos: 1284, newOutliers: 4 }
const videos = Array.from({ length: 12 }, (_, i) => video(i))
let chaptersBody: VideoPreview = { videoId: 'v0', source: 'youtube', state: 'ready', chapters: [{ startSec: 90, title: 'Mở đầu' }] }
let chaptersFail = false
let enqueueBody: CloneEnqueueResult = { items: [{ id: 'q1', jobId: null }] }

const stubFetch = () => {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const method = init?.method ?? 'GET'
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    calls.push({ url, method, body })
    if (url === '/api/rivals' && method === 'GET') return respond(listBody)
    if (url === '/api/rivals' && method === 'POST') {
      listBody = { ...listBody, rivals: [...listBody.rivals, { ...RIVAL, id: 'r3', handle: '@moi', title: 'Mới' }] }
      return respond({ ...RIVAL, id: 'r3', handle: '@moi', title: 'Mới' }, 201)
    }
    if (url === '/api/owner-channels') return respond({ channels: [channel('c1', 'Midnight Tales'), channel('c2', 'Second Wind')] })
    if (url.startsWith('/api/rivals/') && url.endsWith('/scan')) return respond(scan)
    if (url.startsWith('/api/rivals/') && url.includes('/posting-slots')) return respond({
      channelId: 'r1', zone: 'vn', sample: 30, total: 30, pending: 0, pausedUntil: null,
      perWeek: 3, weeksTotal: 6, offSlot: 12,
      grid: Array.from({ length: 7 }, () => Array.from({ length: 12 }, () => 1)),
      slots: [],
    })
    if (url.startsWith('/api/rivals/all/videos')) {
      // Chip "Chưa làm cho kênh nào" lọc thật ở máy chủ: trả rỗng để thấy khối lọc rỗng.
      const filtered = new URL(url, 'https://x.test').searchParams.get('notMade') === 'any'
      return respond({ videos: filtered ? [] : videos, total: filtered ? 0 : videos.length, nextCursor: null } satisfies VideoPage)
    }
    if (/^\/api\/rivals\/[^/]+\/videos\/[^/]+\/preview$/.test(url)) {
      if (chaptersFail) return respond({ error: 'boom' }, 500)
      return respond(chaptersBody)
    }
    if (url === '/api/clone/plan') return respond(plan)
    if (url === '/api/jobs') return respond(enqueueBody)
    if (url.startsWith('/api/owner-channels/') && url.endsWith('/sources')) return respond({ ok: true })
    if (url.startsWith('/api/rivals/') && method === 'DELETE') return respond({ ok: true })
    return respond({ error: 'not found' }, 404)
  }))
}

const roots: Root[] = []
const flush = async (ms = 30) => {
  const { promise, resolve } = Promise.withResolvers<void>()
  setTimeout(resolve, ms)
  await act(async () => { await promise })
}
const mount = async (el: React.ReactElement) => {
  const container = document.createElement('div')
  // Pane điện thoại cung cấp khung nhìn cho virtualizer; happy-dom không
  // có bộ máy layout nên kích thước được khai báo ở ranh giới DOM này.
  container.style.overflowY = 'auto'
  Object.defineProperties(container, {
    clientHeight: { configurable: true, value: 844 },
    clientWidth: { configurable: true, value: 390 },
  })
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(el) })
  await flush()
  return container
}
const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
  await flush()
}
const byText = (root: ParentNode, sel: string, text: string) =>
  [...root.querySelectorAll(sel)].find((el) => el.textContent?.includes(text))
const until = async (fn: () => boolean, tries = 40) => {
  for (let i = 0; i < tries; i++) { if (fn()) return; await flush() }
  expect(fn()).toBe(true)
}

let copied: string[] = []
beforeEach(async () => {
  calls.length = 0
  copied = []
  listBody = { rivals: [RIVAL, RIVAL2], totalVideos: 1284, newOutliers: 4 }
  chaptersBody = { videoId: 'v0', source: 'youtube', state: 'ready', chapters: [{ startSec: 90, title: 'Mở đầu' }] }
  chaptersFail = false
  enqueueBody = { items: [{ id: 'q1', jobId: null }] }
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({
    matches: true, media: '(max-width: 767px)', addEventListener: () => {}, removeEventListener: () => {},
  })))
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: vi.fn(async (text: string) => { copied.push(text) }) },
  })
  await invalidate(keys.rivals())
  await invalidate(keys.channels())
  stubFetch()
})

afterEach(async () => {
  for (const root of roots.splice(0)) await act(async () => { root.unmount() })
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

describe('Khám phá trên điện thoại — lọc, trạng thái thật, chương, xếp hàng loạt', () => {
  const bulkMakeForFirstChannel = async (root: ParentNode) => {
    await click(byText(root, 'button', 'Chọn'))
    await click(byText(root.querySelector('[role="toolbar"]')!, 'button', 'Top 10'))
    await click(byText(root.querySelector('[role="toolbar"]')!, 'button', 'Làm 10 video'))
  }

  it('chọn độ dài trong tấm chọn và giữ lựa chọn khi mở lại', async () => {
    const root = await mount(<ExploreScreen route={{ name: 'explore' }} />)
    await until(() => root.textContent?.includes('Video số 0') === true)
    const trigger = root.querySelector<HTMLButtonElement>('[role="combobox"][aria-label="Độ dài video"]')!
    await click(trigger)
    const sheet = document.querySelector('[role="dialog"]')
    expect(sheet).not.toBeNull()
    expect(byText(sheet!, '[role="option"]', 'Mọi độ dài')?.getAttribute('aria-selected')).toBe('true')
    await click(byText(sheet!, '[role="option"]', 'Từ 8 phút'))
    await until(() => !document.querySelector('[role="dialog"]'))
    expect(trigger.textContent).toContain('Từ 8 phút')
    await click(trigger)
    const reopened = document.querySelector('[role="dialog"]')
    expect(byText(reopened!, '[role="option"]', 'Từ 8 phút')?.getAttribute('aria-selected')).toBe('true')
  })

  it('chip Trạng thái lọc thật ở máy chủ; Bỏ lọc trả về danh sách đầy đủ', async () => {
    const root = await mount(<ExploreScreen route={{ name: 'explore' }} />)
    await until(() => root.textContent?.includes('Video số 0') === true)

    await click(root.querySelector(`[role="combobox"][aria-label="${tt.statusLabel}"]`))
    await click(byText(document.body, '[role="option"]', tt.statusOptions.notMade))
    await until(() => calls.some((call) => call.url.includes('notMade=any')))
    expect(calls.filter((call) => call.url.startsWith('/api/rivals/all/videos')).at(-1)?.url).toContain('notMade=any')

    await until(() => root.textContent?.includes(tt.listFilterTitle) === true)
    expect(root.textContent).toContain(tt.noMatchStatusNotMade)
    expect(root.textContent).not.toContain('Video số 0')

    await click(byText(root, 'button', tt.listFilterClear))
    await until(() => root.textContent?.includes('Video số 0') === true)
  })

  it('thẻ video hiện trạng thái thật của kênh đã làm và mở đúng job', async () => {
    const root = await mount(<ExploreScreen route={{ name: 'explore' }} />)
    await until(() => root.textContent?.includes('Video số 0') === true)
    const usageLink = root.querySelector('a[href="#/?video=j1"]')
    expect(usageLink?.textContent).toContain('Midnight Tales · Đang làm')
  })

  it('chi tiết video: mục "Đã làm cho kênh" và chương thật tua được trên trình phát', async () => {
    const root = await mount(<ExploreScreen route={{ name: 'explore', video: 'v0' }} />)
    await until(() => root.textContent?.includes(tt.detailUsedTitle) === true)
    expect(root.textContent).toContain(tt.usageMadeSub(tt.usageStage.working))
    const openJob = [...root.querySelectorAll('a')].find((node) => node.getAttribute('aria-label') === `${tt.usageOpen} Midnight Tales`)
    expect(openJob?.getAttribute('href')).toBe('#/?video=j1')

    await until(() => root.textContent?.includes(tt.chaptersCount('1')) === true)
    const chapter = [...root.querySelectorAll('a')].find((node) => node.textContent?.includes('Mở đầu'))
    expect(chapter?.textContent).toContain('1:30')
    expect(chapter?.getAttribute('href')).toBe('https://youtu.be/v0?t=90s')
  })

  it('chương rỗng hoặc lỗi đọc thì nói đúng trạng thái, không bịa danh sách', async () => {
    chaptersBody = { videoId: 'v0', source: 'download', state: 'empty', chapters: [] }
    const empty = await mount(<ExploreScreen route={{ name: 'explore', video: 'v0' }} />)
    await until(() => empty.textContent?.includes(tt.chaptersEmpty) === true)
    expect(empty.textContent).not.toContain('Mở đầu')

    chaptersFail = true
    const failed = await mount(<ExploreScreen route={{ name: 'explore', video: 'v0' }} />)
    await until(() => failed.textContent?.includes(tt.chaptersError) === true)
    expect(failed.textContent).not.toContain('Mở đầu')
  })

  it('hàng loạt mở cùng MNew bằng nguồn đã chọn và giữ selection khi chưa xác nhận', async () => {
    const open = vi.fn(); window.addEventListener('viderse:quick-create', open)
    const root = await mount(<ExploreScreen route={{ name: 'explore' }} />)
    await until(() => root.textContent?.includes('Video số 0') === true)
    await bulkMakeForFirstChannel(root)
    expect(open.mock.calls[0][0].detail).toEqual({ urls: videos.slice(0, 10).map((video) => video.url), fromSelection: true })
    expect(calls.filter((call) => call.url === '/api/jobs')).toHaveLength(0)
    expect(root.querySelector('[role="toolbar"]')?.textContent).toContain(tt.selected(10))
    window.removeEventListener('viderse:quick-create', open)
  })
})

describe('Khám phá trên điện thoại', () => {
  it('chọn nhiều thay thanh tab: Top 10 chọn 10 video, sao chép đủ link', async () => {
    const root = await mount(<ExploreScreen route={{ name: 'explore' }} />)
    await until(() => root.textContent?.includes('Video số 0') === true)
    expect(byText(root, 'h1', 'Khám phá')).toBeTruthy()
    await click(byText(root, 'button', 'Chọn'))
    const bar = root.querySelector('[role="toolbar"]')
    expect(bar?.textContent).toContain('Đã chọn 0')
    await click(byText(bar!, 'button', 'Top 10'))
    expect(bar?.textContent).toContain('Đã chọn 10')
    await click(byText(bar!, 'button', 'Sao chép link'))
    expect(copied[0].split('\n')).toHaveLength(10)
    expect(copied[0]).toContain('https://youtu.be/v0')
  })

  it('thêm đối thủ thật rồi liên kết kênh đã chọn', async () => {
    const root = await mount(<ExploreScreen route={{ name: 'explore' }} />)
    await until(() => root.textContent?.includes('Một Phim') === true)
    await click(byText(root, 'button', 'Theo dõi'))
    const input = document.querySelector('[role="dialog"] input') as HTMLInputElement
    expect(input).toBeTruthy()
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
      setter.call(input, '@moi')
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await flush()
    const sheet = input.closest('[role="dialog"]') ?? root
    await click(byText(sheet, 'button', 'Theo dõi và liên kết 1 kênh'))
    await until(() => calls.some((call) => call.url === '/api/rivals' && call.method === 'POST'))
    expect(calls.find((call) => call.url === '/api/rivals' && call.method === 'POST')?.body).toEqual({ input: '@moi' })
    expect(calls.some((call) => call.url === '/api/owner-channels/c1/sources')).toBe(true)
  })

  it('chi tiết video: không còn Vì sao, chọn kênh có plan thật rồi thêm thẳng', async () => {
    const root = await mount(<ExploreScreen route={{ name: 'explore', video: 'v0' }} />)
    await until(() => root.textContent?.includes('Video số 0') === true)
    expect(root.textContent).toContain('Video số 0')
    await click(byText(root, 'button', 'Làm video cho kênh'))
    await until(() => calls.some((call) => call.url === '/api/clone/plan'))
    await until(() => (document.body.textContent?.includes('Tiếp theo: Midnight') ?? false))
    const dialog = [...document.querySelectorAll('[role="dialog"]')].find((el) => el.textContent?.includes('Làm video cho kênh nào?'))!
    expect(dialog.textContent).toContain('Tiếp theo: Midnight')
    const make = [...dialog.querySelectorAll('button')].find((el) => el.textContent?.includes('Làm video cho Midnight Tales'))
    expect(make?.hasAttribute('disabled')).toBe(false)
    await click(make)
    await until(() => calls.some((call) => call.url === '/api/jobs'))
    expect(calls.find((call) => call.url === '/api/jobs')?.body).toEqual({ channelId: 'c1', urls: ['https://youtu.be/v0'] })
  })

  it('chi tiết video không có trong trang đầu: nói rõ không tìm thấy, không bịa dữ liệu', async () => {
    const root = await mount(<ExploreScreen route={{ name: 'explore', video: 'khong-co' }} />)
    await until(() => root.textContent?.includes('Không tìm thấy video này') === true)
    expect(root.textContent).toContain('Quay lại Khám phá và chọn lại')
  })
})
