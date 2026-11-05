// @vitest-environment happy-dom
// Ba loại đề xuất: nhận có xác nhận chi phí, bỏ qua im lặng và xử lý dòng đã mất.
// × là bỏ qua im lặng, href chỉ điều hướng, 404 coi như đề xuất đã mất.
// Không còn thẻ, Bản tin, "Vì sao?", "Đừng đề xuất kiểu này" hay Hoàn tác.
import { act } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { confirm, toast } from '@noverse/ui'
import { cache } from '@noverse/ui/data'
import { useSuggestions } from './useSuggestions'
import { SuggestionRow } from './SuggestionRow'
import { button, click, json, mount, suggestion, unmountAll, type Call } from './testFixtures'
import { navigate } from '../../router'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), confirm: vi.fn(), toast: Object.assign(vi.fn(), { err: vi.fn(), ok: vi.fn() }) }))
vi.mock('../../data/status', () => ({ useOnline: () => true }))
vi.mock('../../router', () => ({ navigate: vi.fn(), paths: {} }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let calls: Call[] = []
let open: unknown[] = []
let acceptReply: (call: Call) => Response

function Probe({ scope = 'test' }: { scope?: string }) {
  const { list, accept, dismiss, busy, notice, online } = useSuggestions(scope)
  return <div>
    {list.initialLoading ? <span>Đang tải…</span> : list.error ? <span>Không tải được đề xuất.</span> : null}
    {(list.data?.suggestions ?? []).map((item) => <SuggestionRow key={item.id} suggestion={item}
      busy={busy === item.id} offline={!online} notice={notice?.id === item.id ? notice.text : undefined}
      onAccept={() => void accept(item)} onDismiss={() => void dismiss(item)} />)}
  </div>
}

beforeEach(() => {
  cache.clear()
  calls = []; open = []
  vi.mocked(confirm).mockReset().mockResolvedValue(true)
  vi.mocked(toast).mockReset()
  vi.mocked(navigate).mockReset()
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const call = { url: String(input), method: init?.method ?? 'GET', body: init?.body ? JSON.parse(String(init.body)) : undefined }
    calls.push(call)
    if (call.url.startsWith('/api/suggestions') && call.method === 'GET') return json({ suggestions: open, total: open.length })
    if (call.url.endsWith('/accept')) return acceptReply(call)
    if (call.url.endsWith('/dismiss')) { open = []; return json({ ok: true }) }
    return json({ error: 'not found' }, 404)
  }))
})
afterEach(async () => { await unmountAll(); vi.unstubAllGlobals() })

const posts = (suffix: string) => calls.filter((call) => call.method === 'POST' && call.url.endsWith(suffix))

describe('nhận đề xuất', () => {
  it('tốn ảnh → hỏi màu đỏ nêu số ảnh → gửi confirm → toast kết quả, không Hoàn tác', async () => {
    open = [suggestion('s1', { primary: { label: 'Làm 7 video', op: 'enqueue', params: {}, cost: { images: 7 } } })]
    acceptReply = () => json({ ok: true, summary: 'Đã xếp 7 video' })
    const el = await mount(<Probe />)
    await click(button(el, 'Đề xuất s1'))
    expect(confirm).toHaveBeenCalledTimes(1)
    const asked = vi.mocked(confirm).mock.calls[0][0]
    expect(asked.tone).toBe('danger')
    expect(String(asked.body)).toContain('7 ảnh')
    expect(posts('/accept').map((call) => call.body)).toEqual([{ confirm: true }])
    expect(toast).toHaveBeenCalledTimes(1)
    expect(vi.mocked(toast).mock.calls[0][0]).toBe('Đã xếp 7 video')
    expect(vi.mocked(toast).mock.calls[0][2]).toBeUndefined()
  })

  it('409 confirm_required khi máy chủ hỏi lại → hỏi rồi nhận lại', async () => {
    open = [suggestion('s1')]
    let first = true
    acceptReply = () => {
      if (first) { first = false; return json({ error: 'cần xác nhận', code: 'confirm_required', cost: { images: 2 } }, 409) }
      return json({ ok: true, summary: 'Đã làm' })
    }
    const el = await mount(<Probe />)
    await click(button(el, 'Đề xuất s1'))
    expect(confirm).toHaveBeenCalledOnce()
    expect(posts('/accept')).toHaveLength(2)
    expect(toast).toHaveBeenCalledOnce()
  })

  it('confirms cleanup once and sends the approved destructive action without asking again', async () => {
    open = [suggestion('s1', { kind: 'cleanup', primary: { label: 'Dọn MP4 đã đăng', op: 'cleanup', params: {} } })]
    acceptReply = (call) => (call.body as { confirm?: boolean } | undefined)?.confirm
      ? json({ ok: true, summary: 'Đã dọn MP4 đã đăng' })
      : json({ error: 'cần xác nhận', code: 'confirm_required', cost: { images: 0 } }, 409)
    const el = await mount(<Probe />)
    await click(button(el, 'Đề xuất s1'))
    expect(confirm).toHaveBeenCalledOnce()
    expect(posts('/accept')).toHaveLength(1)
    expect(toast).toHaveBeenCalledOnce()
  })

  it('chỉ điều hướng thì mở href và không gọi nhận', async () => {
    open = [suggestion('s1', { primary: { label: 'Mở template', op: 'noop', params: {}, href: '#/templates/t1' } })]
    const el = await mount(<Probe />)
    await click(button(el, 'Đề xuất s1'))
    expect(navigate).toHaveBeenCalledWith('/templates/t1')
    expect(posts('/accept')).toHaveLength(0)
  })

  it('đề xuất đã mất (404) → báo và làm mới, không toast lỗi', async () => {
    open = [suggestion('s1')]
    acceptReply = () => { open = []; return json({ error: 'gone' }, 404) }
    const el = await mount(<Probe />)
    await click(button(el, 'Đề xuất s1'))
    expect(button(el, 'Đề xuất s1')).toBeUndefined()
    expect(vi.mocked(toast).err).not.toHaveBeenCalled()
  })
})

describe('bỏ qua', () => {
  it('gửi body rỗng, không hỏi lại, không toast', async () => {
    open = [suggestion('s1')]
    const el = await mount(<Probe />)
    await click(el.querySelector('button[aria-label="Bỏ qua"]'))
    expect(confirm).not.toHaveBeenCalled()
    expect(button(el, 'Đề xuất s1')).toBeUndefined()
    expect(toast).not.toHaveBeenCalled()
  })

  it('lỗi nhận hiện ngay dưới dòng, không mở hộp', async () => {
    open = [suggestion('s1')]
    acceptReply = () => json({ error: 'máy chủ lỗi' }, 500)
    const el = await mount(<Probe />)
    await click(button(el, 'Đề xuất s1'))
    expect(el.textContent).toContain('Máy chủ lỗi')
  })
})

