// @vitest-environment happy-dom
// Thư viện + cài đặt mẫu video trên fetch giả (đúng wire của API thư viện):
//  · tự lưu: rời ô tên → PATCH kèm revision; 400 giữ chữ đã gõ + lỗi dưới ô;
//    409 giữ bản đang sửa và đưa nút Tải lại; hai lần rời ô liên tiếp → PATCH
//    tuần tự, revision tăng 1 → 2.
//  · xem thử: none/fresh/stale đúng nhãn; mọi trạng thái đều có nút dựng thật,
//    chưa có video xong nên khoá kèm lý do.
//  · xoá prompt 409 in_use hiện tên mẫu video đang dùng; xoá mẫu video bị khoá
//    trong menu khi kênh còn dùng.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { TemplateScreen } from './TemplateScreen'
import { LibraryScreen } from './LibraryScreen'
import { t } from '../../i18n/library'
import { PromptsTab } from './PromptsTab'
import type { ImagePrompt, TemplateDetail, TemplateSummary } from '../../api/library'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))
const calls: { url: string; method: string; body?: string }[] = []
let patchers: ((value: Response) => void)[] = []
let usedBy: { id: string; name: string }[] = [{ id: 'tpl-a', name: 'Template A' }]
let sampleUrls: string[] = []
let detailBody: TemplateDetail
let patchOk: (revision: number) => TemplateDetail

const summary = (over: Partial<TemplateSummary> = {}): TemplateSummary => ({
  id: 't1',
  name: 'Truyện kể đêm',
  revision: 1,
  style: 'cutout',
  images: { mode: 'fixed', count: 6, minutes: 4, perVideo: 6, known: true },
  prompts: { sceneId: 'builtin:scene', sceneName: 'Tả thực điện ảnh', coverId: 'builtin:cover-hook', coverName: 'Chữ hook đậm', summary: 'Tả thực điện ảnh · Tách nền · bìa Chữ hook đậm' },
  backdrop: { channelCount: 1, videoCount: 0, clipCount: null, clipSec: 8, enabled: true },
  overlayCount: 2,
  channels: [{ id: 'ch1', name: 'Kênh Một' }],
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

function installFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const method = init?.method ?? 'GET'
    calls.push({ url, method, body: typeof init?.body === 'string' ? init.body : undefined })
    if (url === '/api/templates/t1' && method === 'GET') return respond(detailBody)
    if (url === '/api/templates/t1' && method === 'PATCH') {
      // lib của web chưa có Promise.withResolvers — dùng executor để giữ resolver.
      return new Promise<Response>((resolve) => { patchers.push(resolve) })
    }
    if (url === '/api/templates') return respond({ items: [summary(), summary({ id: 't2', name: 'Đêm mưa', channels: [] })] })
    if (url === '/api/templates/t1/export' && method === 'GET') {
      return Promise.resolve(new Response(new Blob(['zip']), { status: 200, headers: { 'Content-Disposition': 'attachment; filename="truyen-ke-dem.ytf-template.zip"' } }))
    }
    if (url === '/api/image-prompts/p1' && method === 'DELETE') {
      return respond({ error: 'Prompt đang dùng ở Template A', code: 'in_use', usedBy: ['tpl-a'] }, 409)
    }
    if (/^\/api\/image-prompts\/[^/]+\/samples$/.test(url)) {
      // Đúng wire của PromptSampleStatus — thiếu cost là hợp đồng API bị phá.
      return respond({ sources: [], history: [], cost: { images: 1, provider: 'ChatGPT', label: 'ChatGPT', quotaLeft: 1 }, ready: false })
    }
    if (url.startsWith('/api/image-prompts')) return respond({ items: [prompt({ usedBy, sampleUrls }), prompt({ id: 'b1', name: 'Có sẵn', builtIn: true, usedBy: [] })] })
    if (url === '/api/overlays') return respond({ items: [] })
    if (url === '/api/owner-channels') return respond({ channels: [{ id: 'ch1', name: 'Kênh Một' }] })
    return respond({}, 404)
  }))
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

const flush = async (rounds = 6) => {
  for (let i = 0; i < rounds; i++) await act(async () => { await new Promise((r) => setTimeout(r, 10)) })
}

const setValue = async (input: HTMLInputElement, value: string) => {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
}

const blur = async (input: HTMLInputElement) => {
  await act(async () => { input.dispatchEvent(new FocusEvent('focusout', { bubbles: true })); input.blur() })
  await flush()
}

const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
  await flush()
}

const byText = (root: ParentNode, sel: string, text: string) =>
  [...root.querySelectorAll(sel)].find((el) => el.textContent?.includes(text))

beforeEach(async () => {
  cache.clear()
  calls.length = 0
  patchers = []
  usedBy = [{ id: 'tpl-a', name: 'Template A' }]
  sampleUrls = []
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} })))
  detailBody = detail()
  patchOk = (revision) => detail({ revision, name: 'Mới', savedAt: '2026-10-31T04:05:00Z' })
  installFetch()
})

afterEach(() => {
  for (const root of roots.splice(0)) act(() => root.unmount())
  document.body.innerHTML = ''
})

const templateRoute = { name: 'template' as const, templateId: 't1' }

describe('cài đặt template — tự lưu', () => {
  it('rời ô tên thì PATCH kèm revision và cập nhật mốc đã lưu', async () => {
    patchers = []
  usedBy = [{ id: 'tpl-a', name: 'Template A' }]
    const page = await render(<TemplateScreen route={templateRoute} />)
    const input = page.querySelector<HTMLInputElement>('.vt-name')!
    await setValue(input, 'Mới')
    await blur(input)
    expect(calls.filter((c) => c.method === 'PATCH')).toHaveLength(1)
    expect(JSON.parse(calls.find((c) => c.method === 'PATCH')!.body!)).toEqual({ revision: 1, name: 'Mới' })
    await act(async () => patchers.shift()!(Response.json(patchOk(2))))
    await flush()
    expect(page.querySelector('.vt-saved')!.getAttribute('data-state')).toBe('saved')
  })

  it('400 theo trường: giữ chữ đã gõ và hiện lỗi dưới ô', async () => {
    const page = await render(<TemplateScreen route={templateRoute} />)
    const input = page.querySelector<HTMLInputElement>('.vt-name')!
    await setValue(input, 'Tên rất dài')
    await blur(input)
    await act(async () => patchers.shift()!(Response.json({ error: 'Tên template quá dài', code: 'invalid_input', fields: [{ field: 'name', message: 'Tên template quá dài' }] }, { status: 400 })))
    await flush()
    expect(page.querySelector<HTMLInputElement>('.vt-name')!.value).toBe('Tên rất dài')
    expect(page.textContent).toContain('Tên template quá dài')
  })

  it('409: giữ bản đang sửa và đưa nút Tải lại', async () => {
    const page = await render(<TemplateScreen route={templateRoute} />)
    const input = page.querySelector<HTMLInputElement>('.vt-name')!
    await setValue(input, 'Bản của tôi')
    await blur(input)
    await act(async () => patchers.shift()!(Response.json({ error: 'Template vừa được sửa ở nơi khác', code: 'conflict' }, { status: 409 })))
    await flush()
    expect(page.querySelector<HTMLInputElement>('.vt-name')!.value).toBe('Bản của tôi')
    expect(byText(page, 'button', 'Tải lại')).toBeTruthy()
  })

  it('hai lần rời ô liên tiếp → PATCH tuần tự, revision 1 rồi 2', async () => {
    const page = await render(<TemplateScreen route={templateRoute} />)
    const input = page.querySelector<HTMLInputElement>('.vt-name')!
    await setValue(input, 'A')
    await blur(input)
    await setValue(input, 'B')
    await blur(input)
    const sent = calls.filter((c) => c.method === 'PATCH')
    expect(sent).toHaveLength(1)
    await act(async () => patchers.shift()!(Response.json(detail({ revision: 2 }))))
    await flush()
    const both = calls.filter((c) => c.method === 'PATCH')
    expect(both).toHaveLength(2)
    expect(JSON.parse(both[1].body!).revision).toBe(2)
    expect(JSON.parse(both[1].body!).name).toBe('B')
  })
})

describe('cài đặt template — xem thử', () => {
  const previewCopy = t.library.template.preview
  const cases = [
    { state: 'none' as const, action: previewCopy.rebuildStale },
    { state: 'fresh' as const, action: previewCopy.rebuild },
    { state: 'stale' as const, action: previewCopy.rebuildStale },
  ]
  for (const item of cases) {
    it(`trạng thái ${item.state}`, async () => {
      detailBody = detail({ preview: { state: item.state, builtAt: item.state === 'none' ? null : '2026-10-31T03:20:00Z' } })
      const page = await render(<TemplateScreen route={templateRoute} />)
      expect(page.querySelector('.vt-badge')!.getAttribute('data-state')).toBe(item.state)
      // Mọi trạng thái đều có nút dựng thật; chưa có video xong nên khoá kèm lý do.
      const build = byText(page, 'button', item.action)!
      expect(build.getAttribute('aria-disabled')).toBe('true')
    })
  }
})

describe('thư viện — xoá', () => {
  it('xoá prompt 409 in_use hiện tên template đang dùng', async () => {
    usedBy = []
    const page = await render(<LibraryScreen route={{ name: 'library', tab: 'prompts' }} />)
    const card = [...page.querySelectorAll('.vlb-pcard')].find((el) => el.textContent?.includes('Sương mù đồi thông'))!
    await click(card.querySelector('.vlb-pcard-open'))
    await click(byText(page, 'button', t.library.delete))
    const dialog = document.querySelector('[role="dialog"], [role="alertdialog"]') as HTMLElement
    const danger = [...dialog.querySelectorAll('button')].find((el) => el.textContent?.trim() === t.library.delete)
    await click(danger)
    await flush(10)
    expect(document.body.textContent).toContain('Template A')
    expect(calls.some((c) => c.url === '/api/image-prompts/p1' && c.method === 'DELETE')).toBe(true)
  })

  it('xoá template bị khoá trong menu khi kênh còn dùng', async () => {
    const page = await render(<LibraryScreen route={{ name: 'library', tab: 'templates' }} />)
    await click(page.querySelector(`[aria-label="${t.library.templates.menuAria('Truyện kể đêm')}"]`))
    const menu = document.querySelector('[role="menu"]')!
    const remove = [...menu.querySelectorAll('button')].find((el) => el.textContent?.includes(t.library.templates.deleteItem))!
    expect(remove.getAttribute('aria-disabled')).toBe('true')
    expect(menu.textContent).toContain('Kênh Một')
  })
})

describe('thư viện — liên kết', () => {
  it('mọi liên kết nội bộ render href bắt đầu bằng "#/"', async () => {
    const lib = await render(<LibraryScreen route={{ name: 'library', tab: 'templates' }} />)
    const hrefs = [...lib.querySelectorAll('a[href]')].map((a) => a.getAttribute('href')!)
    expect(hrefs.filter((href) => !href.startsWith('#/'))).toEqual([])

    const tpl = await render(<TemplateScreen route={templateRoute} />)
    const navHrefs = [...tpl.querySelectorAll('.vt-nav a[href]')].map((a) => a.getAttribute('href')!)
    expect(navHrefs.filter((href) => !href.startsWith('#/'))).toEqual([])
  })
})

describe('thư viện — xuất tệp template', () => {
  it('nút Xuất tải tệp thật qua API rồi mới báo thành công', async () => {
    const downloaded: { href: string; name: string }[] = []
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      downloaded.push({ href: this.getAttribute('href') ?? '', name: this.download })
    })
    const createSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:template-export')
    const revokeSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)
    try {
      const page = await render(<LibraryScreen route={{ name: 'library', tab: 'templates' }} />)
      const button = page.querySelector(`[aria-label="${t.library.templates.exportAria('Truyện kể đêm')}"]`)!
      expect(button.getAttribute('aria-disabled')).toBeFalsy()
      await click(button)
      await flush()
      expect(calls.some((call) => call.url === '/api/templates/t1/export' && call.method === 'GET')).toBe(true)
      expect(downloaded).toEqual([{ href: 'blob:template-export', name: 'truyen-ke-dem.ytf-template.zip' }])
      expect(createSpy).toHaveBeenCalled()
      expect(revokeSpy).toHaveBeenCalledWith('blob:template-export')
    } finally {
      clickSpy.mockRestore(); createSpy.mockRestore(); revokeSpy.mockRestore()
    }
  })

})

describe('thư viện — ảnh mẫu prompt', () => {
  it('thẻ prompt vẽ ảnh mẫu thật; ảnh hỏng thì quay về ô trung tính', async () => {
    usedBy = []
    sampleUrls = ['/api/image-prompts/p1/samples/s1/image']
    const page = await render(<LibraryScreen route={{ name: 'library', tab: 'prompts' }} />)
    const card = [...page.querySelectorAll('.vlb-pcard')].find((el) => el.textContent?.includes('Sương mù đồi thông'))!
    const img = card.querySelector('img')!
    expect(img.getAttribute('src')).toBe('/api/image-prompts/p1/samples/s1/image')
    await act(async () => { img.dispatchEvent(new Event('error')) })
    await flush()
    expect(card.querySelector('img')).toBeFalsy()
  })

  it('chưa có ảnh mẫu thì giữ ô trung tính, không bịa ảnh', async () => {
    const page = await render(<LibraryScreen route={{ name: 'library', tab: 'prompts' }} />)
    const card = [...page.querySelectorAll('.vlb-pcard')].find((el) => el.textContent?.includes('Sương mù đồi thông'))!
    expect(card.querySelector('img')).toBeFalsy()
  })
})

it('opens the exact requested prompt after loading and when the target changes within the same view', async () => {
  const onView = vi.fn()
  const page = await render(<PromptsTab view="builtin" promptId="b2" all={[]} allLoading onView={onView} />)
  expect(page.querySelector('#vlb-prompt-name')).toBeNull()
  const other = prompt({ id: 'b1', name: 'Chỉ dẫn có sẵn khác', builtIn: true })
  const requested = prompt({ id: 'b2', name: 'Chỉ dẫn có sẵn cần mở', builtIn: true })
  await act(async () => {
    roots.at(-1)!.render(<PromptsTab view="builtin" promptId="b2" all={[other, requested]} allLoading={false} onView={onView} />)
  })
  await flush()
  expect(page.querySelector<HTMLInputElement>('#vlb-prompt-name')?.value).toBe(requested.name)
  expect(page.querySelector<HTMLInputElement>('#vlb-prompt-name')?.readOnly).toBe(true)
  await act(async () => {
    roots.at(-1)!.render(<PromptsTab view="builtin" promptId="b1" all={[other, requested]} allLoading={false} onView={onView} />)
  })
  await flush()
  expect(page.querySelector<HTMLInputElement>('#vlb-prompt-name')?.value).toBe(other.name)
})

it('keeps a failed template request distinct from an empty library and offers recovery', async () => {
  const previousFetch = globalThis.fetch
  let fail = true
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    return url === '/api/templates' && fail ? respond({ error: 'Không đọc được thư viện.' }, 503) : previousFetch(input, init)
  }))
  const page = await render(<LibraryScreen route={{ name: 'library', tab: 'templates' }} />)
  expect(page.textContent).toContain(t.library.loadFail)
  expect(page.textContent).not.toContain(t.library.templates.emptyTitle)
  fail = false
  await click(byText(page, 'button', t.library.retry))
  expect(page.textContent).toContain('Truyện kể đêm')
  expect(page.textContent).not.toContain(t.library.loadFail)
})

it.each([false, true])('recovers a temporarily unavailable template without claiming it was deleted (mobile=%s)', async (mobile) => {
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation((query: string) => ({
    matches: mobile && query.includes('max-width: 767px'), addEventListener: () => {}, removeEventListener: () => {},
  })))
  const previousFetch = globalThis.fetch
  let fail = true
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    return url === '/api/templates/t1' && fail
      ? respond({ error: 'Internal storage unavailable', code: 'storage_unavailable' }, 503)
      : previousFetch(input, init)
  }))
  const page = await render(<TemplateScreen route={templateRoute} />)
  expect(page.textContent).toContain(t.library.template.loadFail)
  expect(page.textContent).not.toContain(t.library.template.notFound)
  expect(page.textContent).not.toContain('storage_unavailable')
  fail = false
  await click(byText(page, 'button', t.library.retry))
  if (mobile) expect(page.textContent).toContain(detailBody.name)
  else expect(page.querySelector<HTMLInputElement>('.vt-name')?.value).toBe(detailBody.name)
  expect(page.textContent).not.toContain(t.library.template.loadFail)
})

it('cancels a template name with Escape without submitting the cancelled value', async () => {
  const page = await render(<TemplateScreen route={templateRoute} />)
  const input = page.querySelector<HTMLInputElement>('.vt-name')!
  await act(async () => { input.focus() })
  await setValue(input, 'Tên bị huỷ')
  await act(async () => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
  await flush()
  expect(calls.filter(call => call.method === 'PATCH')).toEqual([])
  expect(input.value).toBe(detailBody.name)
})

it('does not build a saved recipe while a different visible draft has failed to save', async () => {
  const previousFetch = globalThis.fetch
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    return url === '/api/templates/t1/preview'
      ? respond({ state: 'none', builtAt: null, minutes: 3, sources: [{ jobId: 'ready-source', title: 'Video nguồn', durationSec: 180 }] })
      : previousFetch(input, init)
  }))
  const page = await render(<TemplateScreen route={templateRoute} />)
  const input = page.querySelector<HTMLInputElement>('.vt-name')!
  await setValue(input, 'Tên chưa lưu')
  await blur(input)
  await act(async () => patchers.shift()!(Response.json({ error: 'Chưa lưu được' }, { status: 500 })))
  await flush()
  const build = byText(page, '.vt-preview-actions button', t.library.template.preview.rebuildStale)!
  expect(build.getAttribute('aria-disabled')).toBe('true')
  await click(build)
  expect(calls.filter(call => call.method === 'POST' && call.url.endsWith('/preview'))).toEqual([])
})
