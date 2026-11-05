// @vitest-environment happy-dom
// Cài đặt › Công cụ trên máy — fixture đúng wire `GET /api/tools`
// (internal/web/tools.go: tools[{id,state,hint,version,sizeBytes,provided,progress}],
// checkedAt, dataDir). Thẻ tổng theo mức nặng nhất, một nhãn mỗi hàng, nút chỉ
// khi có việc, vùng Nâng cao giữ thư mục + phiên bản.
import { act } from 'react'
import { invalidate } from '@noverse/ui/data'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ToolsSection from './ToolsSection'
import type { ToolId, ToolRow, ToolsStatus } from '../../../api/tools'
import { tools as copy } from '../../../i18n/tools'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
let tools: ToolsStatus
let calls: { url: string; method: string }[] = []
let mobile = false

const row = (over: Partial<ToolRow> & { id: ToolId }): ToolRow => ({
  state: 'ready', hint: 'Đang dùng', version: null, update: null, progress: null, detail: null, retryAt: null,
  sizeBytes: 400 * 1024 * 1024, ...over,
})

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))

function installFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    calls.push({ url, method: init?.method ?? 'GET' })
    if (url === '/api/tools' && (init?.method ?? 'GET') === 'GET') return respond(tools)
    if (url === '/api/tools/check') return respond(tools)
    const install = url.match(/^\/api\/tools\/([^/]+)\/(install|cancel)$/)
    if (install) return respond({ started: true, taskId: 't1' }, 202)
    return respond({}, 404)
  }))
}

async function render() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<ToolsSection />) })
  for (let i = 0; i < 4; i++) await act(async () => { await new Promise((r) => setTimeout(r, 20)) })
  return container
}

const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
}

const byText = (el: ParentNode, sel: string, text: string) =>
  [...el.querySelectorAll(sel)].find((n) => n.textContent?.includes(text)) as HTMLElement | undefined

beforeEach(() => {
  void invalidate()
  calls = []
  mobile = false
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation((q: string) => ({
    matches: q.includes('767px') ? mobile : false, addEventListener: () => {}, removeEventListener: () => {},
  })))
  tools = {
    checkedAt: '2026-10-30T09:00:00Z',
    dataDir: '/Users/dat/.viderse/tools',
    tools: [
      row({ id: 'yt-dlp', version: '2026.10.09', update: { current: '2026.10.09', latest: '2026.10.22' }, state: 'update', hint: 'Có bản mới' }),
      row({ id: 'ffmpeg', version: '7.1', hint: 'Cắt cảnh · bản 7.1' }),
      row({ id: 'transcription', version: '3.1', hint: 'Chép lời thoại thành chữ ngay trên máy' }),
      row({ id: 'chrome', version: '141', provided: true }),
      row({ id: 'codex', version: '0.9.1' }),
      row({ id: 'codex-login', state: 'needs_login', hint: 'Chưa đăng nhập' }),
    ],
  }
  installFetch()
})

afterEach(async () => {
  await invalidate()
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('Công cụ trên máy', () => {
  it('tên hàng theo việc, kèm dung lượng và nhãn Đã cài', async () => {
    const el = await render()
    const titles = [...el.querySelectorAll('.vv-tools__title')].map((n) => n.textContent)
    expect(titles).toContain('Tải video và phụ đề')
    expect(titles).toContain('Tách tiếng và tạo phụ đề')
    expect(el.textContent).toContain('Đã cài')
    expect(el.textContent).toContain('419,4 MB')
  })

  it.each([false, true])('recheck updates the displayed tool state without installing (mobile=%s)', async (phone) => {
    mobile = phone
    const el = await render()
    tools = { ...tools, checkedAt: '2026-11-05T10:00:00Z', tools: tools.tools.map(tool => tool.id === 'yt-dlp' ? { ...tool, state: 'ready', update: null, version: '2026.10.22' } : tool) }
    const recheck = [...el.querySelectorAll('button')].find(button => button.textContent?.includes(copy.recheck) && Boolean(button.closest('.vv-tools__sum')) !== phone)
    await click(recheck)
    const download = el.querySelector('.vv-tools__row')!
    expect(download.querySelector('.vv-tools__pill')?.textContent).toBe(copy.pills.ready)
    expect([...download.querySelectorAll('button')].some(button => button.textContent?.includes('Cập nhật'))).toBe(false)
    expect(calls.filter(call => call.method === 'POST').map(call => call.url)).toEqual(['/api/tools/check'])
  })

  it('hàng cập nhật chỉ có nút Cập nhật; bấm là gọi install', async () => {
    const el = await render()
    const rowEl = [...el.querySelectorAll('.vv-tools__row')].find((r) => r.textContent?.includes('Tải video và phụ đề'))
    expect(rowEl?.querySelector('.vv-tools__pill')?.textContent).toBe('Có bản mới')
    await click(byText(rowEl!, 'button', 'Cập nhật'))
    expect(calls.some((c) => c.url === '/api/tools/yt-dlp/install' && c.method === 'POST')).toBe(true)
  })

  it('bước đang cài hiện thanh tiến độ và câu trấn an ở thẻ tổng', async () => {
    tools.tools[2] = row({ id: 'transcription', state: 'installing', hint: '', progress: { phase: 'đang tải', bytesDone: 42, bytesTotal: 100, pct: 42, startedAt: null } })
    const el = await render()
    expect(el.querySelector('.vv-tools__sum-title')?.textContent).toBe('Đang cài 3/6')
    expect(el.querySelector('.vv-tools__row .vv-tools__prog')?.textContent).toContain('đang tải')
  })

  it('pha chưa biết tổng (pct null) vẽ thanh chạy vô định, không ghi “0%”', async () => {
    tools.tools[2] = row({ id: 'transcription', state: 'installing', hint: '', progress: { phase: 'downloading', phaseLabel: 'Đang tải mô hình nhận giọng nói', bytesDone: 0, bytesTotal: null, pct: null, startedAt: null } })
    const el = await render()
    const prog = el.querySelector('.vv-tools__row .vv-tools__prog')
    expect(prog?.textContent).toContain('Đang tải mô hình nhận giọng nói')
    expect(prog?.textContent).not.toContain('%')
    const bar = prog?.querySelector('[role="progressbar"]')
    expect(bar?.classList.contains('vv-progress--working')).toBe(true)
    expect(bar?.hasAttribute('aria-valuenow')).toBe(false)
  })

  it('hàng chưa cài đứng sau lượt đang cài hiện “Chờ” và không có nút', async () => {
    // Lượt cài đi theo thứ tự Chuẩn bị máy: hàng sau nó là hàng đợi, hàng trước
    // vẫn là “Chưa cài” có nút Cài.
    tools.tools[0] = row({ id: 'yt-dlp', state: 'missing', hint: 'Chưa cài — app sẽ tự cài bản ghim.', version: null })
    tools.tools[2] = row({ id: 'transcription', state: 'installing', hint: '', progress: { phase: 'downloading', bytesDone: 42, bytesTotal: 100, pct: 42, startedAt: null } })
    tools.tools[4] = row({ id: 'codex', state: 'missing', hint: 'Chưa cài — app sẽ tự cài bản ghim.', version: null })
    const el = await render()
    const codex = [...el.querySelectorAll('.vv-tools__row')].find((r) => r.textContent?.includes('Công cụ vẽ ảnh'))
    expect(codex?.querySelector('.vv-tools__pill')?.textContent).toBe('Chờ')
    expect(codex?.querySelector('button')).toBeNull()
    const yt = [...el.querySelectorAll('.vv-tools__row')].find((r) => r.textContent?.includes('Tải video và phụ đề'))
    expect(yt?.querySelector('.vv-tools__pill')?.textContent).toBe('Chưa cài')
    expect(yt?.querySelector('button')).not.toBeNull()
  })

  it('hàng lỗi hiện câu sửa và nút Tải tiếp', async () => {
    tools.tools[2] = row({ id: 'transcription', state: 'failed', hint: 'Mất kết nối khi tải mô hình.' })
    const el = await render()
    const rowEl = [...el.querySelectorAll('.vv-tools__row')].find((r) => r.textContent?.includes('Tách tiếng và tạo phụ đề'))
    expect(rowEl?.textContent).toContain('Mất kết nối khi tải mô hình.')
    expect(rowEl?.querySelector('.vv-tools__pill')?.textContent).toBe('Chưa cài được')
    await click(byText(rowEl!, 'button', 'Tải tiếp'))
    expect(calls.some((c) => c.url === '/api/tools/transcription/install')).toBe(true)
  })

  it('mobile: hàng ChatGPT chưa đăng nhập giữ câu cần-đăng-nhập của server', async () => {
    mobile = true
    const el = await render()
    const rowEl = [...el.querySelectorAll('.vv-tools__row')].find((r) => r.textContent?.includes('Đăng nhập ChatGPT'))
    expect(rowEl?.textContent).toContain('Chưa đăng nhập')
    expect(rowEl?.textContent).not.toContain('Đã đăng nhập · quản lý ở Kết nối AI')
  })

  it('mobile: thẻ Chi tiết vẫn giữ tên công cụ cạnh phiên bản', async () => {
    mobile = true
    const el = await render()
    const adv = el.querySelector('.vv-tools__adv')
    const keys = [...(adv?.querySelectorAll('.vv-tools__adv-k') ?? [])].map((k) => k.textContent)
    expect(keys).toContain('yt-dlp')
    expect(keys).toContain('faster-whisper')
  })

  it('Nâng cao là hàng cuối của thẻ danh sách, mở ra thư mục và phiên bản', async () => {
    const el = await render()
    const list = el.querySelector('.vv-tools__list')
    const adv = el.querySelector('.vv-tools__adv')
    // Hàng cuối của CHÍNH thẻ danh sách (artboard: nối liền dưới hàng ChatGPT).
    expect(list?.lastElementChild).toBe(adv)
    expect(list?.querySelectorAll('.vv-tools__row').length).toBe(6)
    expect(adv?.tagName).toBe('DETAILS')
    expect(adv?.hasAttribute('open')).toBe(false)
    expect(adv?.textContent).toContain('Nâng cao · thư mục dữ liệu và phiên bản')
    expect(adv?.textContent).toContain('/Users/dat/.viderse/tools')
    expect(adv?.textContent).toContain('faster-whisper')
  })
})
