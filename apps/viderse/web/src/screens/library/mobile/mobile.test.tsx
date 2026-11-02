// @vitest-environment happy-dom
// Thư viện + Sửa template trên điện thoại (artboard MLibrary / MTemplate, đợt 38):
//  · ≤767px vẽ màn điện thoại; ≥768px vẫn là màn máy tính;
//  · ⋯ của template mở sheet đủ 6 hàng, Xoá tắt khi còn kênh dùng và giữ lý do;
//  · sửa một mục trong Sửa template tự lưu rồi thanh chân hiện "Đã lưu · HH:MM";
//  · Kênh › Template có lối "Mở Thư viện dùng chung" trên điện thoại.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { t } from '../../../i18n/library'
import type { Channel } from '../../../api/channels'
import type { ImagePrompt, TemplateDetail, TemplateSummary } from '../../../api/library'
import { clock } from '../fmt'
import { TemplatesTab } from '../../channel/TemplatesTab'
import { LibraryScreen } from '../LibraryScreen'
import { TemplateScreen } from '../TemplateScreen'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
const calls: { url: string; method: string; body?: string }[] = []
let patchers: ((value: Response) => void)[] = []
let detailBody: TemplateDetail
let sampleUrls: string[] = []

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))

const summary = (over: Partial<TemplateSummary> = {}): TemplateSummary => ({
  id: 't1',
  name: 'Truyện kể đêm',
  revision: 1,
  style: 'cutout',
  images: { mode: 'fixed', count: 6, minutes: 4, perVideo: 6, known: true },
  prompts: { sceneId: 'builtin:scene', sceneName: 'Tả thực điện ảnh', coverId: 'builtin:cover-hook', coverName: 'Chữ hook đậm', summary: 'Tả thực điện ảnh · Tách nền · bìa Chữ hook đậm' },
  backdrop: { channelCount: 1, videoCount: 0, clipCount: null, clipSec: 8, enabled: true },
  overlayCount: 2,
  channels: [{ id: 'ch1', name: 'Kênh Một', position: 1 }],
  preview: { state: 'none', builtAt: null },
  createdAt: '2026-10-29T03:00:00Z',
  updatedAt: '2026-10-31T03:00:00Z',
  ...over,
})

const recipe = () => ({
  scenes: {
    style: 'cutout' as const,
    framing: 'close' as const,
    images: { mode: 'fixed' as const, count: 6, minutes: 4 },
    motion: { kind: 'ken-burns' as const, intensity: 0.35, speed: 1 },
    scenePromptId: 'builtin:scene',
    coverPromptId: 'builtin:cover-hook',
    coverSource: 'ai' as const,
    referenceImages: false,
    referenceStyle: 'composition' as const,
    sceneOpacity: 0.7,
  },
  backdrop: { channels: [{ channelId: 'ch1', kinds: ['video'], pinned: [] }], clipCount: null, clipSec: 8, fixedVideoIds: [] },
  overlays: [],
  appearance: { opacity: 1, blurRadius: 0, brightness: 1, contrast: 1, saturation: 1, hueShift: 0, speed: 1, zoom: 1, flipX: false, flipY: false },
  output: { compressExport: true },
})

const detail = (over: Partial<TemplateDetail> = {}): TemplateDetail => ({
  ...summary(over),
  recipe: recipe(),
  price: { perImage: 0.04, known: true, imagesPerVideo: 6, includesCover: true, mode: 'fixed', totalUSD: 0.28 },
  savedAt: '2026-10-31T03:00:00Z',
  ...over,
})

const prompt = (over: Partial<ImagePrompt> = {}): ImagePrompt => ({
  id: 'p1', name: 'Sương mù đồi thông', kind: 'scene', builtIn: false,
  prompt: 'Chất liệu phim 35 mm, sương mù, ánh sáng dịu',
  updatedAt: '2026-10-31T02:00:00Z', usedBy: [{ id: 'tpl-a', name: 'Template A' }], samples: [], sampleUrls: [],
  ...over,
})

const channel = (over: Partial<Channel> = {}): Channel => ({
  id: 'ch1', name: 'Kênh Một', youtubeUrl: null, language: 'vi',
  styleNote: null, descriptionFooter: null, alwaysTags: [],
  autoPick: true, pacePerDay: 5, templateCursor: 0,
  autonomy: { preset: 'balanced', pick: 'all', approve: { mode: 'ask', min: 85 }, onError: 'retry_once', belowQuality: 'ask' },
  priority: 'steady',
  branding: { brief: '', summary: '', look: '', avatar: '', banner: '' },
  avatarUrl: null, bannerUrl: null, sources: [], templates: [],
  identity: { state: 'ready' },
  createdAt: '2026-10-20T00:00:00Z', updatedAt: '2026-10-20T00:00:00Z',
  ...over,
})

function installFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const method = init?.method ?? 'GET'
    calls.push({ url, method, body: typeof init?.body === 'string' ? init.body : undefined })
    if (url === '/api/templates/t1' && method === 'GET') return respond(detailBody)
    if (url === '/api/templates/t1' && method === 'PATCH') {
      const pending = Promise.withResolvers<Response>()
      patchers.push(pending.resolve)
      return pending.promise
    }
    if (url === '/api/templates') return respond({ items: [summary()] })
    if (url.startsWith('/api/image-prompts')) return respond({ items: [prompt({ sampleUrls })] })
    if (url === '/api/overlays') return respond({ items: [] })
    if (url === '/api/owner-channels') return respond({ channels: [{ id: 'ch1', name: 'Kênh Một' }] })
    if (url === '/api/backdrop/channels') return respond([])
    if (url === '/api/backdrop/videos') return respond([])
    return respond({}, 404)
  }))
}

const flush = async (rounds = 6) => {
  for (let i = 0; i < rounds; i++) await act(async () => { await new Promise((r) => setTimeout(r, 10)) })
}

async function render(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(node) })
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

/** Bề ngang khai báo cho `useMediaQuery('(max-width: 767px)')`. */
const setPhone = (phone: boolean) => {
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({ matches: phone, addEventListener: () => {}, removeEventListener: () => {} })))
}

beforeEach(async () => {
  cache.clear()
  calls.length = 0
  patchers = []
  detailBody = detail()
  sampleUrls = []
  setPhone(true)
  installFetch()
})

afterEach(() => {
  for (const root of roots.splice(0)) act(() => root.unmount())
  document.body.innerHTML = ''
})

const libraryRoute = { name: 'library' as const, tab: 'templates' as const }

describe('thư viện dùng chung — điện thoại', () => {
  it('≤767px vẽ màn điện thoại, ≥768px vẫn là màn máy tính', async () => {
    const phone = await render(<LibraryScreen route={libraryRoute} />)
    expect(phone.querySelector('.m-page')).toBeTruthy()
    expect(phone.querySelector('.vlb-shell')).toBeFalsy()
    expect(phone.textContent).toContain('Thư viện dùng chung')

    setPhone(false)
    const desktop = await render(<LibraryScreen route={libraryRoute} />)
    expect(desktop.querySelector('.vlb-shell')).toBeTruthy()
    expect(desktop.querySelector('.m-page')).toBeFalsy()
  })

  it('⋯ của template mở sheet 6 hàng; Xoá tắt khi còn kênh dùng và ghi lý do', async () => {
    const page = await render(<LibraryScreen route={libraryRoute} />)
    await click(page.querySelector(`[aria-label="${t.library.mobile.templateMenuAria('Truyện kể đêm')}"]`))

    const sheet = document.querySelector('[role="dialog"]') as HTMLElement
    expect(sheet).toBeTruthy()
    for (const label of [
      t.library.mobile.editTemplate,
      t.library.templates.rename,
      t.library.duplicate,
      t.library.templates.addToChannel,
      t.library.mobile.exportFile,
      t.library.mobile.deleteTemplate,
    ]) {
      expect(byText(sheet, 'button', label)).toBeTruthy()
    }
    const remove = byText(sheet, 'button', t.library.mobile.deleteTemplate) as HTMLButtonElement
    expect(remove.disabled).toBe(true)
    expect(remove.textContent).toContain('1 kênh')
  })

  it('template không kênh nào dùng thì Xoá bấm được', async () => {
    // Dữ liệu giả mặc định trả template có kênh; đổi phản hồi để kiểm nhánh còn lại.
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      if (url === '/api/templates') return respond({ items: [summary({ channels: [] })] })
      if (url === '/api/owner-channels') return respond({ channels: [{ id: 'ch1', name: 'Kênh Một' }] })
      if (url === '/api/backdrop/channels') return respond([])
      if (url === '/api/backdrop/videos') return respond([])
      if (url === '/api/image-prompts') return respond({ items: [] })
      if (url === '/api/overlays') return respond({ items: [] })
      return respond({}, 404)
    }))
    cache.clear()
    const free = await render(<LibraryScreen route={libraryRoute} />)
    await click(free.querySelector(`[aria-label="${t.library.mobile.templateMenuAria('Truyện kể đêm')}"]`))
    const sheet = document.querySelector('[role="dialog"]') as HTMLElement
    const remove = byText(sheet, 'button', t.library.mobile.deleteTemplate) as HTMLButtonElement
    expect(remove.disabled).toBe(false)
  })
})

describe('sửa template — điện thoại', () => {
  it('sửa một mục thì tự lưu và thanh chân hiện mốc đã lưu thật', async () => {
    const page = await render(<TemplateScreen route={{ name: 'template', templateId: 't1', section: 'images' }} />)
    const bar = page.querySelector('.m-footer')!
    expect(bar.textContent).toContain(clock(detailBody.savedAt))

    await click(byText(page, 'button', 'Theo độ dài'))
    const patch = calls.find((c) => c.method === 'PATCH')
    expect(patch).toBeTruthy()
    expect(JSON.parse(patch!.body!).recipe.scenes.images.mode).toBe('per-minutes')

    await act(async () => patchers.shift()!(Response.json(detail({ revision: 2, savedAt: '2026-10-31T04:05:00Z' }))))
    await flush()
    expect(page.querySelector('.m-footer')!.textContent).toContain(clock('2026-10-31T04:05:00Z'))
  })

  it('thanh trên dùng tên và metadata thật; màn con không lặp tiêu đề mục', async () => {
    const list = await render(<TemplateScreen route={{ name: 'template', templateId: 't1' }} />)
    const header = list.querySelector('[role="banner"]')!
    expect(header.querySelector('.m-title')?.textContent).toContain('Truyện kể đêm · Tách nền')
    expect(header.textContent).toContain('1 kênh')
    expect(header.textContent).toContain(clock(detailBody.savedAt))
    const build = byText(list, 'button', t.library.template.preview.rebuildStale)!
    expect(build.getAttribute('aria-disabled')).toBe('true')
    const page = await render(<TemplateScreen route={{ name: 'template', templateId: 't1', section: 'images' }} />)
    expect(page.querySelector('.m-page')).toBeTruthy()
    expect(page.querySelector('h2')).toBeFalsy()
  })
})

describe('kênh — lối vào thư viện', () => {
  it('điện thoại có “Mở Thư viện dùng chung” trỏ tới thư viện, máy tính thì không', async () => {
    const phone = await render(<TemplatesTab channel={channel()} />)
    const link = byText(phone, 'a', 'Mở Thư viện dùng chung')!
    expect(link.getAttribute('href')).toBe('#/library?tab=templates')

    setPhone(false)
    const desktop = await render(<TemplatesTab channel={channel()} />)
    expect(byText(desktop, 'a', 'Mở Thư viện dùng chung')).toBeFalsy()
  })
})

describe('thư viện dùng chung — xuất tệp', () => {
  it('hàng Xuất tệp template trong tấm thao tác tải thật, không còn khoá', async () => {
    const downloaded: string[] = []
    const spy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      downloaded.push(this.getAttribute('href') ?? '')
    })
    try {
      const page = await render(<LibraryScreen route={libraryRoute} />)
      await click(page.querySelector(`[aria-label="${t.library.mobile.templateMenuAria('Truyện kể đêm')}"]`))
      const sheet = document.querySelector('[role="dialog"]') as HTMLElement
      const exportRow = byText(sheet, 'button', t.library.mobile.exportFile) as HTMLButtonElement
      expect(exportRow.disabled).toBe(false)
      await click(exportRow)
      expect(downloaded).toEqual(['/api/templates/t1/export'])
    } finally {
      spy.mockRestore()
    }
  })
})

describe('thư viện dùng chung — ảnh mẫu prompt', () => {
  it('ô prompt trên lưới điện thoại vẽ ảnh mẫu thật', async () => {
    sampleUrls = ['/api/image-prompts/p1/samples/s1/image']
    const page = await render(<LibraryScreen route={{ name: 'library', tab: 'prompts' }} />)
    const img = page.querySelector('img[src="/api/image-prompts/p1/samples/s1/image"]')
    expect(img).toBeTruthy()
  })

  it('chưa có ảnh mẫu thì ô prompt giữ nền trung tính', async () => {
    const page = await render(<LibraryScreen route={{ name: 'library', tab: 'prompts' }} />)
    expect(page.querySelector('img[src^="/api/image-prompts/"]')).toBeFalsy()
  })

  it('hộp chọn prompt trong Sửa template vẽ ảnh mẫu; ảnh hỏng thì về ô trung tính', async () => {
    sampleUrls = ['/api/image-prompts/p1/samples/s1/image']
    const page = await render(<TemplateScreen route={{ name: 'template', templateId: 't1', section: 'images' }} />)
    const img = page.querySelector('img[src="/api/image-prompts/p1/samples/s1/image"]')!
    expect(img).toBeTruthy()
    await act(async () => { img.dispatchEvent(new Event('error')) })
    await flush()
    expect(page.querySelector('img[src="/api/image-prompts/p1/samples/s1/image"]')).toBeFalsy()
  })
})
