// @vitest-environment happy-dom
// Thẻ Đề xuất: một nút chính, 409 confirm_required → hộp xác nhận (đỏ) → nhận lại với confirm,
// Bỏ qua im lặng (không toast, không hỏi), "Đừng đề xuất kiểu này" gửi forever, chỉ điều hướng thì
// không gọi nhận, toast + Hoàn tác chỉ sau khi người dùng bấm nhận.
import { act } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { confirm, toast } from '@noverse/ui'
import { RecapSuggestions, SuggestionDock } from './SuggestionPanels'
import { button, click, flush, json, mount, suggestion, unmountAll, type Call } from './testFixtures'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), confirm: vi.fn(), toast: Object.assign(vi.fn(), { err: vi.fn(), ok: vi.fn() }) }))
vi.mock('../../data/status', () => ({ useOnline: () => true }))
vi.mock('../../router', () => ({ navigate: vi.fn(), paths: {} }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let calls: Call[] = []
let open: unknown[] = []
let acceptReply: (call: Call) => Response

beforeEach(() => {
  calls = []
  vi.mocked(confirm).mockReset().mockResolvedValue(true)
  vi.mocked(toast).mockReset()
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const call = { url: String(input), method: init?.method ?? 'GET', body: init?.body ? JSON.parse(String(init.body)) : undefined }
    calls.push(call)
    if (call.url.startsWith('/api/suggestions') && call.method === 'GET') return json({ suggestions: open, total: open.length })
    if (call.url.endsWith('/accept')) return acceptReply(call)
    if (call.url.endsWith('/dismiss')) return json({ ok: true })
    if (call.url === '/api/undo') return json({ ok: true })
    return json({ error: 'not found' }, 404)
  }))
})
afterEach(async () => { await unmountAll(); vi.unstubAllGlobals() })

const posts = (suffix: string) => calls.filter((call) => call.method === 'POST' && call.url.endsWith(suffix))

describe('nhận đề xuất', () => {
  it('409 confirm_required → hỏi (nút đỏ, nêu số ảnh) → nhận lại kèm confirm → toast Hoàn tác', async () => {
    open = [suggestion('s1', { primary: { label: 'Làm 7 video', op: 'enqueue', params: {}, cost: { images: 7 } } })]
    let first = true
    acceptReply = (call) => {
      if (call.body && (call.body as { confirm?: boolean }).confirm) return json({ ok: true, summary: 'Đã xếp 7 video', undo: { op: 'unqueue', params: { id: 'q1' } } })
      if (first) { first = false; return json({ error: 'cần xác nhận', code: 'confirm_required', cost: { images: 7 } }, 409) }
      return json({ ok: true, summary: 'x' })
    }
    // Máy chủ chặn trước khi client kịp hỏi: client vẫn hỏi theo primary.cost, rồi gửi confirm.
    const el = await mount(<SuggestionDock scope="t-confirm" onSeeAll={() => undefined} />)
    await click(button(el, 'Làm 7 video'))
    expect(confirm).toHaveBeenCalledTimes(1)
    const asked = vi.mocked(confirm).mock.calls[0][0]
    expect(asked.tone).toBe('danger')
    expect(String(asked.body)).toContain('7 ảnh')
    expect(posts('/accept').map((call) => call.body)).toEqual([{ confirm: true }])
    expect(toast).toHaveBeenCalledTimes(1)
    expect(vi.mocked(toast).mock.calls[0][0]).toBe('Đã xếp 7 video')
    const action = (vi.mocked(toast).mock.calls[0][2] as { label: string; onClick: () => Promise<void> })
    expect(action.label).toBe('Hoàn tác')
    await act(async () => { await action.onClick() })
    expect(calls.some((call) => call.url === '/api/undo' && JSON.stringify(call.body) === JSON.stringify({ op: 'unqueue', params: { id: 'q1' } }))).toBe(true)
  })

  it('đề xuất không khai giá mà server đòi xác nhận (409) → hỏi theo giá của server rồi nhận lại', async () => {
    open = [suggestion('s2', { primary: { label: 'Làm ngay', op: 'enqueue', params: {} } })]
    acceptReply = (call) => (call.body as { confirm?: boolean }).confirm
      ? json({ ok: true, summary: 'Đã làm' })
      : json({ error: 'cần xác nhận', code: 'confirm_required', cost: { images: 3 } }, 409)
    const el = await mount(<SuggestionDock scope="t-409" onSeeAll={() => undefined} />)
    await click(button(el, 'Làm ngay'))
    expect(posts('/accept').map((call) => call.body)).toEqual([{}, { confirm: true }])
    expect(String(vi.mocked(confirm).mock.calls[0][0].body)).toContain('3 ảnh')
  })

  it('người dùng Huỷ hộp xác nhận → không nhận, không toast', async () => {
    open = [suggestion('s3', { primary: { label: 'Làm 7 video', op: 'enqueue', params: {}, cost: { images: 7 } } })]
    acceptReply = () => json({ ok: true, summary: 'x' })
    vi.mocked(confirm).mockResolvedValue(false)
    const el = await mount(<SuggestionDock scope="t-cancel" onSeeAll={() => undefined} />)
    await click(button(el, 'Làm 7 video'))
    expect(posts('/accept')).toHaveLength(0)
    expect(toast).not.toHaveBeenCalled()
  })

  it('việc không tốn ảnh nhận thẳng, không hỏi', async () => {
    open = [suggestion('s4', { primary: { label: 'Đổi nhịp', op: 'set_pace', params: {} } })]
    acceptReply = () => json({ ok: true, summary: 'Đã đổi nhịp' })
    const el = await mount(<SuggestionDock scope="t-plain" onSeeAll={() => undefined} />)
    await click(button(el, 'Đổi nhịp'))
    expect(confirm).not.toHaveBeenCalled()
    expect(posts('/accept')).toHaveLength(1)
  })

  it('primary.href chỉ điều hướng: không gọi nhận, đề xuất vẫn mở', async () => {
    open = [suggestion('s5', { primary: { label: 'Mở template', op: 'open', params: {}, href: '#/library/templates/t1' } })]
    acceptReply = () => json({ ok: true, summary: 'x' })
    const el = await mount(<SuggestionDock scope="t-href" onSeeAll={() => undefined} />)
    await click(button(el, 'Mở template'))
    expect(posts('/accept')).toHaveLength(0)
    expect(toast).not.toHaveBeenCalled()
    expect(el.textContent).toContain('Đề xuất s5')
  })
})

describe('Bỏ qua và Đừng đề xuất kiểu này', () => {
  it('Bỏ qua là nút quiet, gửi dismiss không forever, không hỏi, không toast', async () => {
    open = [suggestion('s6')]
    acceptReply = () => json({ ok: true, summary: 'x' })
    const el = await mount(<SuggestionDock scope="t-dismiss" onSeeAll={() => undefined} />)
    const skip = button(el, 'Bỏ qua')!
    expect(skip.className).not.toBe(button(el, 'Lên đầu')!.className) // quiet ≠ nút chính
    open = []
    await click(skip)
    expect(posts('/dismiss').map((call) => call.body)).toEqual([{}])
    expect(confirm).not.toHaveBeenCalled()
    expect(toast).not.toHaveBeenCalled()
    expect(el.textContent).not.toContain('Đề xuất s6')
  })

  it('bản Bản tin có menu "Đừng đề xuất kiểu này" gửi forever; bản gọn ở cột trái thì không có', async () => {
    open = [suggestion('s7')]
    acceptReply = () => json({ ok: true, summary: 'x' })
    const dock = await mount(<SuggestionDock scope="t-never-dock" onSeeAll={() => undefined} />)
    expect(dock.querySelector('[aria-label^="Thêm:"]')).toBeNull()
    const el = await mount(<RecapSuggestions />)
    await click(el.querySelector('[aria-label^="Thêm:"]'))
    const item = [...document.querySelectorAll<HTMLElement>('[role="menuitem"]')].find((node) => node.textContent?.includes('Đừng đề xuất kiểu này'))
    await click(item)
    expect(posts('/dismiss').map((call) => call.body)).toEqual([{ forever: true }])
    expect(toast).not.toHaveBeenCalled()
  })

  it('đề xuất đã mất (404/409 khi bỏ qua) → coi như xong, thẻ biến mất', async () => {
    open = [suggestion('s8')]
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.endsWith('/dismiss')) return json({ error: 'không tìm thấy' }, 404)
      return json({ suggestions: open, total: open.length })
    }))
    const el = await mount(<SuggestionDock scope="t-gone" onSeeAll={() => undefined} />)
    open = []
    await click(button(el, 'Bỏ qua'))
    await flush()
    expect(el.textContent).not.toContain('Đề xuất s8')
  })
})

describe('trạng thái', () => {
  it('không có đề xuất → dòng trống; Bản tin không chiếm chỗ', async () => {
    open = []
    acceptReply = () => json({ ok: true, summary: 'x' })
    const dock = await mount(<SuggestionDock scope="t-empty" onSeeAll={() => undefined} />)
    expect(dock.textContent).toContain('Chưa có đề xuất')
    const recap = await mount(<RecapSuggestions />)
    expect(recap.querySelector('article')).toBeNull()
  })

  it('lỗi tải → báo lỗi có nút Thử lại', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json({ error: 'lỗi máy chủ' }, 500)))
    const el = await mount(<SuggestionDock scope="t-error" onSeeAll={() => undefined} />)
    expect(el.textContent).toContain('Không tải được đề xuất')
    expect(button(el, 'Thử lại')).toBeTruthy()
  })

  it('Xem cả N chỉ hiện khi tổng nhiều hơn số thẻ hiện', async () => {
    open = [suggestion('a1'), suggestion('a2'), suggestion('a3'), suggestion('a4')]
    vi.stubGlobal('fetch', vi.fn(async () => json({ suggestions: open, total: 5 })))
    const onSeeAll = vi.fn()
    const el = await mount(<SuggestionDock scope="t-more" onSeeAll={onSeeAll} />)
    expect(el.querySelectorAll('article')).toHaveLength(3)
    await click(button(el, 'Xem cả 5 trong Bản tin'))
    expect(onSeeAll).toHaveBeenCalledTimes(1)
  })
})
