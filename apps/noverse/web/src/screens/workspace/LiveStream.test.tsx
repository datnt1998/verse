// @vitest-environment happy-dom
// Bug 2710 — thẻ bước trong “Luồng viết trực tiếp”: một câu lời thường thay cho lưới chip
// tham số (“Chương 2 · Chương 0 · Chương 0”), có trợ lý và kết quả, không lộ chi tiết kỹ thuật.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import type { ToolEvent } from '../../lib/roundSummary'
import { LiveStream } from './LiveStream'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

// Thân round y như ảnh chụp: đọc một chương nhưng args còn from/to = 0.
const STREAM = '✻ Đọc chương\nchapter: 2\nsource: draft\nfrom: 0\nto: 0\nmax_runes: 10000\ncharacter: null\n'

const readEvent: ToolEvent = {
  id: 'ev-7', time: '', finishedAt: '', failed: false, category: 'TOOL', agent: 'writer',
  summary: 'read_chapter(chương 2 · bản nháp)', kind: '', level: 'info', depth: 1, durationNs: 237,
  retryAt: '', sessionId: 'sess', canRetryNow: false,
  tool: 'read_chapter', args: '{"chapter":2,"source":"draft"}',
  result: '{"chapter":2,"source":"draft","word_count":1546,"content":"Bữa trưa…"}', status: 'ok',
}

let root: Root | null = null

const mount = async (events: ToolEvent[], hash = '#/p/dit-dang-audio/s/01-tra-xanh-cuop-kich-ban/work', streamText = STREAM) => {
  location.hash = hash
  const host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(async () => {
    root?.render(<LiveStream streamText={streamText} isRunning={false} snapshot={null} events={events} loading={false} />)
  })
  return host
}

const openCard = async (host: HTMLElement) => {
  const button = [...host.querySelectorAll('button')].find((node) => node.textContent?.includes('Đọc chương'))
  expect(button, 'thẻ bước “Đọc chương” phải có').toBeTruthy()
  await act(async () => { (button as HTMLButtonElement).click() })
}

afterEach(async () => {
  await act(async () => { root?.unmount() })
  root = null
  document.body.replaceChildren()
})

describe('thẻ bước › tóm tắt tham số (ảnh chụp bug 2710)', () => {
  it('một câu “Đọc lại bản nháp chương 2”, không còn chip “Chương 0”', async () => {
    const host = await mount([readEvent])
    await openCard(host)
    const text = host.textContent ?? ''
    expect(text).toContain('Đọc lại bản nháp chương 2')
    expect(text).not.toContain('Chương 0')
    expect(text).not.toContain('max_runes')
  })

  it('tiêu đề thẻ là “Đọc chương · Người viết” (lấy trợ lý từ dòng Nhật ký)', async () => {
    const host = await mount([readEvent])
    const header = [...host.querySelectorAll('button')].find((node) => node.textContent?.includes('Đọc chương'))
    expect(header?.textContent).toContain('· Người viết')
  })

  it('kết quả của lần gọi đã xong: “đã đọc 1.546 chữ”', async () => {
    const host = await mount([readEvent])
    await openCard(host)
    expect(host.textContent).toContain('đã đọc 1.546 chữ')
  })

  it('không có dòng Nhật ký thì thẻ chỉ có câu tóm tắt, không bịa trợ lý/kết quả', async () => {
    const host = await mount([])
    await openCard(host)
    const text = host.textContent ?? ''
    expect(text).toContain('Đọc lại bản nháp chương 2')
    expect(text).not.toContain('Người viết ·')
    expect(text).not.toContain('đã đọc')
  })
})

describe('thẻ bước › công cụ chỉ có đầu mục (không extractor)', () => {
  it('save_book lấy tên truyện từ dòng Nhật ký, không lộ tên công cụ', async () => {
    const host = await mount([{
      ...readEvent, id: 'ev-8', tool: 'save_book', args: '{"title":"Tuổi Sáng Phản Bộn","synopsis":"Một nữ thư ký…"}',
      result: '{"foundation_ready":false,"saved":true}', status: 'ok', agent: 'architect',
    }], '#/p/dit-dang-audio/s/01-tra-xanh-cuop-kich-ban/work', '✻ save_book\n')
    const button = [...host.querySelectorAll('button')].find((node) => node.textContent?.includes('Lưu thông tin truyện'))
    expect(button?.textContent).toContain('· Kiến trúc sư')
    await act(async () => { (button as HTMLButtonElement).click() })
    const text = host.textContent ?? ''
    expect(text).toContain('Lưu thông tin truyện “Tuổi Sáng Phản Bộn”')
    expect(text).not.toContain('save_book')
    expect(text).toContain('đã lưu')
  })
})

describe('thẻ bước › lỗi hiện câu thường, không tick xanh', () => {
  it('lần gọi hỏng: “thiếu tham số bắt buộc” và icon lỗi thay cho tick', async () => {
    const failedEvent: ToolEvent = {
      ...readEvent, id: 'ev-9', failed: true, level: 'error', status: 'error',
      tool: 'read_chapter', args: '{"chapter":2}',
      result: 'InputValidationError: read_chapter failed due to the following issue: The required parameter `source` is missing.',
    }
    const host = await mount([failedEvent])
    await openCard(host)
    const text = host.textContent ?? ''
    expect(text).toContain('thiếu tham số bắt buộc')
    expect(text).not.toContain('InputValidationError')
    expect(text).not.toContain('required parameter')
  })
})

describe('“Chi tiết kỹ thuật” chỉ hiện ở chế độ Nâng cao/dev', () => {
  it('mặc định ẩn hẳn', async () => {
    const host = await mount([readEvent])
    await openCard(host)
    expect(host.textContent).not.toContain('Chi tiết kỹ thuật')
  })

  it('bản dev mở bằng ?tech=1 → có khối, chứa tham số gốc', async () => {
    const host = await mount([readEvent], '#/p/dit-dang-audio/s/01-tra-xanh-cuop-kich-ban/work?tech=1')
    await openCard(host)
    expect(host.textContent).toContain('Chi tiết kỹ thuật')
    expect(host.textContent).toContain('read_chapter')
  })
})

const QUESTION = '**Đã đọc chương.**\n\n- Chưa gọi `save_arc_summary`.\n- Cần chọn hướng đi.\n\nBạn muốn tôi tóm tắt hay kiểm tra lại?'

describe('thẻ bước › lời cuối của trợ lý', () => {
  it('hiện đậm, mã và danh sách; Việt hoá công cụ, không tick hoàn thành câu hỏi chưa trả lời', async () => {
    const host = await mount([{ ...readEvent, agent: 'editor' }], undefined, `${STREAM}\n${QUESTION}`)
    await openCard(host)
    expect(host.querySelector('strong')?.textContent).toBe('Đã đọc chương.')
    expect(host.querySelectorAll('li')).toHaveLength(2)
    expect(host.textContent).not.toContain('save_arc_summary')
    expect(host.textContent).toContain('Tóm tắt cung')
    expect(host.querySelector('code')?.textContent).toBe('Tóm tắt cung')
    expect(host.textContent).toContain('cần bạn xem')
    expect(host.querySelector('[aria-label="đã xong"]')).toBeNull()
    expect(host.textContent).not.toContain('đã đọc 1.546 chữ')
  })

  it('văn bản stream ngoài thẻ dùng cùng trình Markdown', async () => {
    const host = await mount([], undefined, '**Kết quả**\n\n- Đã dùng `save_arc_summary`.\n- Đã lưu.')
    expect(host.querySelector('strong')?.textContent).toBe('Kết quả')
    expect(host.querySelectorAll('li')).toHaveLength(2)
    expect(host.querySelector('code')?.textContent).toBe('Tóm tắt cung')
  })
})
