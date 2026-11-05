// @vitest-environment happy-dom
// Video nền trên fetch giả: parse nền tảng lạ → nút "Gửi yêu cầu hỗ trợ" chỉ
// gọi một lần; lưới kênh ảo hoá giữ số thẻ trong DOM nhỏ hơn hẳn danh mục;
// video lẻ đang tải hiện thanh tiến độ. Template › Nền: ghim video lưu qua
// onSave({backdrop.channels[].pinned}); cảnh báo ít clip hiện khi mix báo lặp.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { BackdropsTab } from './BackdropsTab'
import { BackdropSection } from '../template/BackdropSection'
import { BackdropPicker } from '../template/BackdropPicker'
import { t } from '../../../i18n/library'
import type { BackdropChannel, BackdropVideo } from '../../../api/backdrop'
import type { TemplateDetail } from '../../../api/library'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))
const calls: { url: string; method: string }[] = []
let handler: (url: URL, method: string) => Promise<Response>

async function render(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(node) })
  return container
}

const flush = async (ms = 0) => act(async () => {
  // Promise.withResolvers chưa có trong lib target của project — giữ executor.
  await new Promise((resolve) => { setTimeout(resolve, ms) })
})

const channel = (over: Partial<BackdropChannel> = {}): BackdropChannel => ({
  id: 'twitch:ndfm', platform: 'twitch', handle: 'nightdrivefm', url: 'https://twitch.tv/nightdrivefm',
  title: 'Night Drive FM', avatarUrl: null, addedAt: '2026-10-29T00:00:00Z',
  listedAt: '2026-10-29T01:00:00Z', listError: null, scanNextPage: null, scanTotal: 0, scanStopKnown: true,
  counts: { clip: 40, highlight: 10, vod: 5 }, pausedUntil: null, pauseReason: null, usedBy: [], ...over,
})

const videoItem = (n: number) => ({
  videoId: `v${n}`, kind: 'clip', title: `Đoạn nền ${n}`, durationSec: 45,
  publishedAt: '2026-10-20T00:00:00Z', thumbnailUrl: null, url: `https://twitch.tv/nightdrivefm/clip/v${n}`,
  chosenBy: [],
})

const bvideo = (over: Partial<BackdropVideo> = {}): BackdropVideo => ({
  id: 'bv1', platform: 'twitch', url: 'https://twitch.tv/videos/1', title: 'Mưa đêm neo',
  durationSec: 120, segment: { fromSec: 0, toSec: 60 },
  file: { path: '/tmp/a.mp4', height: 720, bytes: 1_000_000 },
  state: 'ready', usedBy: [], addedAt: '2026-10-29T00:00:00Z', updatedAt: '2026-10-29T00:00:00Z', ...over,
})

const detail = (over: Partial<TemplateDetail['recipe']['backdrop'] & {}> = {}): TemplateDetail => ({
  id: 't1', name: 'Truyện kể đêm', revision: 1, style: 'cutout',
  images: { mode: 'fixed', count: 6, minutes: 4, perVideo: 6, known: true },
  prompts: { sceneId: 'a', sceneName: 'a', coverId: 'b', coverName: 'b', summary: '' },
  backdrop: { channelCount: 0, videoCount: 0, clipCount: null, clipSec: 60, enabled: true },
  overlayCount: 0, channels: [], preview: { state: 'none', builtAt: null },
  createdAt: '2026-10-29T00:00:00Z', updatedAt: '2026-10-29T00:00:00Z',
  recipe: {
    scenes: {} as TemplateDetail['recipe']['scenes'],
    backdrop: { channels: [], clipCount: null, clipSec: 60, fixedVideoIds: [], ...over },
    overlays: [],
    appearance: { opacity: 0.5, blurRadius: 0, brightness: 0.5, contrast: 0.5, saturation: 0.5, hueShift: 0, speed: 1, zoom: 1, flipX: false, flipY: false },
    output: { compressExport: false },
  },
  price: { perImage: 0.01, known: true, imagesPerVideo: 6, includesCover: true, mode: 'fixed', totalUSD: 0.06 },
  savedAt: '2026-10-29T00:00:00Z',
})

beforeEach(() => {
  calls.length = 0
  cache.clear()
  location.hash = '#/library?tab=backdrops'
  handler = async (url) => respond({ message: 'not stubbed: ' + url.pathname }, 404)
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, location.origin)
    calls.push({ url: url.pathname + url.search, method: init?.method ?? 'GET' })
    return handler(url, init?.method ?? 'GET')
  }))
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation((query: string) => ({
    matches: false, media: query,
    addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {},
  })))
})
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
  cache.clear()
})

describe('BackdropsTab — thêm link', () => {
  it('nền tảng lạ: hiện nút gửi yêu cầu; bấm lần hai không gọi lại', async () => {
    handler = async (url) => {
      if (url.pathname === '/api/backdrop/channels') return respond([])
      if (url.pathname === '/api/backdrop/videos') return respond([])
      if (url.pathname === '/api/platforms') return respond([])
      if (url.pathname === '/api/sources/parse') return respond({ kind: 'unsupported', host: 'youtube.com', label: 'YouTube' })
      if (url.pathname === '/api/platforms/requests') return respond({ host: 'youtube.com', label: 'YouTube', count: 1 })
      return respond({}, 404)
    }
    const c = await render(<BackdropsTab />)
    await flush(10)
    const input = c.querySelector<HTMLInputElement>('#vbd-link')!
    const setInput = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
    await act(async () => {
      setInput.call(input, 'https://youtube.com/watch?v=x')
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await act(async () => {
      c.querySelector<HTMLFormElement>('.vbd-addbar')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    await flush(10)
    const ask = [...c.querySelectorAll('button')].find((b) => b.textContent?.includes('Gửi yêu cầu hỗ trợ'))!
    expect(ask).toBeTruthy()
    await act(async () => { ask.click() })
    await flush(10)
    const sent = calls.filter((call) => call.url === '/api/platforms/requests' && call.method === 'POST')
    expect(sent.length).toBe(1)
    await act(async () => { ask.click() })
    await flush(10)
    expect(calls.filter((call) => call.url === '/api/platforms/requests').length).toBe(1)
    expect([...c.querySelectorAll('button')].some((b) => b.textContent?.includes('Đã ghi yêu cầu'))).toBe(true)
  })
})

describe('BackdropsTab — trang kênh', () => {
  it('lưới ảo hoá giữ số thẻ trong DOM nhỏ hơn hẳn danh mục 100 video', async () => {
    const items = Array.from({ length: 100 }, (_, i) => videoItem(i))
    handler = async (url) => {
      if (url.pathname === '/api/backdrop/channels') return respond([channel()])
      if (url.pathname === '/api/backdrop/videos') return respond([])
      if (url.pathname === '/api/platforms') return respond([])
      if (url.pathname === '/api/backdrop/channels/twitch%3Andfm/videos') {
        return respond({ items, counts: { clip: 100 }, total: 100, unavailable: 0, nextCursor: 'c1' })
      }
      return respond({}, 404)
    }
    const c = await render(<BackdropsTab viewport={{ width: 900, height: 700 }} />)
    await flush(10)
    const rendered = c.querySelectorAll('.vbd-vcard').length
    expect(rendered).toBeGreaterThan(0)
    expect(rendered).toBeLessThan(60)
    expect(c.querySelector('.vbd-pane')).toBeTruthy()
  })
})

describe('BackdropsTab — Video lẻ', () => {
  it('video đang tải hiện thanh tiến độ với nhãn aria', async () => {
    handler = async (url) => {
      if (url.pathname === '/api/backdrop/channels') return respond([])
      if (url.pathname === '/api/backdrop/videos') return respond([bvideo({ state: 'downloading', progress: 0.44 })])
      if (url.pathname === '/api/platforms') return respond([])
      return respond({}, 404)
    }
    const c = await render(<BackdropsTab />)
    await flush(10)
    const dl = c.querySelector('[role="group"]')
    expect(dl?.getAttribute('aria-label')).toContain('Đang tải')
    expect(c.textContent).toContain('44%')
  })
})

describe('BackdropSection — hộp gắn + mix', () => {
  it.each([false, true])('keeps empty source kinds unselected for new links but removable on existing links (linked=%s)', async (linked) => {
    const source = channel({ counts: { clip: 40, highlight: 10, vod: 0 } })
    const tpl = detail(linked ? { channels: [{ channelId: source.id, kinds: ['clip', 'vod'], pinned: [] }] } : {})
    const saves: Array<Partial<TemplateDetail['recipe']>> = []
    handler = async (url) => {
      if (url.pathname === '/api/backdrop/channels') return respond([source])
      if (url.pathname === '/api/backdrop/videos') return respond([])
      if (url.pathname.startsWith('/api/backdrop/channels/')) return respond({ items: [videoItem(1)], counts: source.counts, total: 1, unavailable: 0 })
      return respond({}, 404)
    }
    await render(<BackdropPicker template={tpl} initial={source.id} onClose={vi.fn()}
      onSave={async (patch) => { saves.push(patch) }} />)
    await flush(10)
    const dialog = document.querySelector('[role="dialog"]')!
    const emptyKind = [...dialog.querySelectorAll<HTMLButtonElement>('.vbd-kindchip')].find(button => button.querySelector('.num')?.textContent === '0')!
    expect(emptyKind.getAttribute('aria-pressed')).toBe(String(linked))
    expect(emptyKind.disabled).toBe(!linked)
    if (linked) {
      expect(emptyKind.getAttribute('aria-disabled')).not.toBe('true')
      await act(async () => { emptyKind.click() })
    }
    const go = [...dialog.querySelectorAll<HTMLButtonElement>('button')].find(button => button.textContent === (linked ? t.backdrop.template.picker.goSave : t.backdrop.template.picker.go))!
    await act(async () => { go.click() })
    await flush(10)
    expect(saves.at(-1)?.backdrop?.channels[0]?.kinds).toEqual(linked ? ['clip'] : ['clip', 'highlight'])
  })

  it('ghim video trong picker lưu channels[].pinned qua onSave', async () => {
    const tpl = detail()
    const saves: unknown[] = []
    handler = async (url) => {
      if (url.pathname === '/api/backdrop/channels') return respond([channel()])
      if (url.pathname === '/api/backdrop/videos') return respond([])
      if (url.pathname === '/api/overlays') return respond({ items: [] })
      if (url.pathname === '/api/templates/t1/backdrop/mix') return respond({ fixedSec: 0, perChannel: [], loopSec: 0, need: 0, capped: false })
      if (url.pathname.startsWith('/api/backdrop/channels/')) {
        return respond({ items: [videoItem(1), videoItem(2)], counts: { clip: 2 }, total: 2, unavailable: 0 })
      }
      return respond({}, 404)
    }
    const c = await render(<BackdropSection template={tpl} saving={false}
      onSave={(patch) => { saves.push(patch); return Promise.resolve() }} />)
    await flush(10)
    // Mở hộp gắn
    const attach = [...c.querySelectorAll('button')].find((b) => b.textContent?.includes(t.backdrop.template.attach))!
    await act(async () => { attach.click() })
    await flush(10)
    const dialog = document.querySelector('[role="dialog"]')!
    expect(dialog).not.toBeNull()
    // Bật chế độ "Chỉ từ video tôi chọn" rồi tick một video
    const pick = [...dialog.querySelectorAll('[role=radio]')].find((r) => r.textContent?.includes(t.backdrop.template.picker.modePick))!
    await act(async () => { (pick as HTMLElement).click() })
    await flush(10)
    const box = dialog.querySelector<HTMLElement>('[role=checkbox]')!
    expect(box).toBeTruthy()
    await act(async () => { box.click() })
    await flush(10)
    const go = [...dialog.querySelectorAll('button')].find((b) => b.textContent?.includes(t.backdrop.template.picker.go))!
    await act(async () => { go.click() })
    await flush(10)
    const last = saves.at(-1) as { backdrop?: { channels: { channelId: string; pinned: string[] }[] } }
    expect(last.backdrop?.channels[0]?.pinned).toEqual(['v1'])
  })

  it('cảnh báo ít clip hiện khi mix báo lặp video', async () => {
    const tpl = detail({ channels: [{ channelId: 'twitch:ndfm', kinds: ['clip'], pinned: ['v1', 'v2'] }] })
    handler = async (url) => {
      if (url.pathname === '/api/backdrop/channels') return respond([channel()])
      if (url.pathname === '/api/backdrop/videos') return respond([])
      if (url.pathname === '/api/overlays') return respond({ items: [] })
      if (url.pathname === '/api/templates/t1/backdrop/mix') {
        return respond({ fixedSec: 0, perChannel: [{ channelId: 'twitch:ndfm', clips: 40, freshVideos: 2, repeats: true }], loopSec: 80, need: 40, capped: false })
      }
      return respond({}, 404)
    }
    const c = await render(<BackdropSection template={tpl} saving={false}
      onSave={() => Promise.resolve()} />)
    await flush(10)
    expect(c.textContent).toContain('nền sẽ lặp nhiều')
  })
})
