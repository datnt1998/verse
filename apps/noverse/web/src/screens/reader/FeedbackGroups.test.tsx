// @vitest-environment happy-dom
// §AI — bảo chứng UI ngăn/sheet góp ý: chip trạng thái theo vòng đời, nhóm thứ tự,
// chế độ gắn lại và lệnh cấm hiện "Đoạn N" trong UI góp ý của trình đọc.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ParagraphComment, ParagraphCommentStatus } from '../../api/types'
import { FeedbackBody, RelinkBar, type FeedbackAction } from './FeedbackGroups'
import { FeedbackSheet } from './mobile/FeedbackSheet'
import { CommentSheet } from './mobile/CommentSheet'

vi.mock('../../mobile/primitives', () => ({
  BottomSheet: ({ open, label, children }: { open: boolean; label?: string; children: React.ReactNode }) =>
    open ? <section role="dialog" aria-label={label}>{children}</section> : null,
}))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  Icon: () => null }))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let seq = 0
const comment = (over: Partial<ParagraphComment> & { status?: ParagraphCommentStatus }): ParagraphComment => ({
  id: `c-${++seq}`, paragraph: 0, quote: 'Câu trích', chips: [], text: 'Nội dung góp ý', author: 'dat', createdAt: '2026-10-28T10:00:00Z', ...over,
})
const noop = () => {}
const roots: { root: Root; container: HTMLDivElement }[] = []
const mount = async (node: React.ReactNode) => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  await act(async () => root.render(node))
  roots.push({ root, container })
  return container
}
afterEach(async () => {
  while (roots.length) {
    const { root, container } = roots.pop()!
    await act(async () => root.unmount())
    container.remove()
  }
})
const body = (list: ParagraphComment[], over: Partial<Parameters<typeof FeedbackBody>[0]> = {}) =>
  <FeedbackBody list={list} chapter={3} relinkId={null} openId={null} onToggle={noop} onAction={noop} busyId={null} {...over} />

describe('§AI · chip trạng thái theo vòng đời', () => {
  it('mỗi trạng thái hiện đúng chip; Người viết chưa sửa (unfixed) vẫn là Chờ gửi', async () => {
    const container = await mount(body([
      comment({ id: 'p', status: 'pending' }),
      comment({ id: 's', status: 'sent' }),
      comment({ id: 'f', status: 'fixed' }),
      comment({ id: 'u', status: 'unsure' }),
      comment({ id: 'l', status: 'lost' }),
      comment({ id: 'd', status: 'done' }),
      comment({ id: 'x', status: 'pending', unfixed: true }),
    ]))
    const chips = [...container.querySelectorAll('.fchip')].map((node) => node.textContent)
    expect(chips).toEqual(['Không chắc vị trí', 'Mất vị trí', 'Chờ gửi', 'Chờ gửi', 'Đã gửi', 'Đã sửa'])
    // Thẻ unfixed xếp nhóm Chờ gửi và mang nhãn báo Người viết chưa sửa.
    expect(container.querySelector('[data-comment="x"] .ftag')?.textContent).toBe('Người viết chưa sửa chỗ này')
    const need = container.querySelector('[data-group="need"]')
    expect(need?.querySelector('[data-comment="l"]')).not.toBeNull()
    expect(need?.querySelector('[data-comment="u"]')).not.toBeNull()
    // Nhóm đầu tiên là Cần gắn lại; hàng Đã xử lý nằm cuối.
    expect(container.querySelector('.fgrp')?.getAttribute('data-group')).toBe('need')
    expect(container.textContent).toContain('Đã xử lý · 1 góp ý')
  })

  it('thẻ Đã sửa mở có Trước/Sau và nút Ổn rồi · Chưa ổn; unsure mở có Gắn vào đây theo gợi ý', async () => {
    const container = await mount(body([
      comment({ id: 'f', status: 'fixed', quote: 'Câu cũ', newQuote: 'Câu mới sắc hơn' }),
      comment({ id: 'u', status: 'unsure', candidates: [{ paragraph: 2, snippet: 'Đoạn gợi ý một' }, { paragraph: 4, snippet: 'Đoạn gợi ý hai' }] }),
    ], { openId: 'f' }))
    expect(container.textContent).toContain('Ổn')
    expect(container.textContent).toContain('Chưa ổn')
    const container2 = await mount(body([
      comment({ id: 'u', status: 'unsure', candidates: [{ paragraph: 2, snippet: 'Đoạn gợi ý một' }, { paragraph: 4, snippet: 'Đoạn gợi ý hai' }] }),
    ], { openId: 'u' }))
    expect(container2.textContent).toContain('Gợi ý 1')
    expect(container2.textContent).toContain('Gắn vào đây')
    expect(container2.textContent).toContain('Chọn đoạn khác')
  })
})

describe('§AI · gắn lại (relink)', () => {
  it('thẻ Mất vị trí luôn có Gắn lại / Gửi lại / Xoá; bấm phát đúng action', async () => {
    const actions: FeedbackAction[] = []
    const lost = comment({ id: 'l', status: 'lost' })
    const container = await mount(body([lost], { openId: 'l', onAction: (action) => actions.push(action) }))
    const click = async (label: string) => {
      const button = [...container.querySelectorAll('.facts .fbtn')].find((node) => node.textContent === label)!
      await act(async () => (button as HTMLButtonElement).click())
    }
    await click('Gắn lại')
    await click('Gửi lại')
    await click('Xoá')
    expect(actions.map((action) => action.kind)).toEqual(['relink', 'resend', 'remove'])
    expect(actions[0].comment.id).toBe('l')
  })

  it('relinkId → chỉ còn thẻ đích trong bảng; dải RelinkBar nhắc đoạn gợi ý', async () => {
    const lost = comment({ id: 'l', status: 'lost', candidates: [{ paragraph: 2, snippet: 'a' }, { paragraph: 3, snippet: 'b' }] })
    const container = await mount(body([lost, comment({ id: 'p', status: 'pending' })], { relinkId: 'l' }))
    expect(container.querySelectorAll('.fcard')).toHaveLength(1)
    expect(container.querySelector('[data-comment="l"]')).not.toBeNull()
    expect(container.querySelector('[data-comment="p"]')).toBeNull()
    const bar = await mount(<RelinkBar body="câu trích góp ý" hasCandidates onCancel={noop} />)
    expect(bar.textContent).toContain('Chọn chỗ mới cho góp ý')
    expect(bar.textContent).toContain('câu trích góp ý')
    const mbar = await mount(<RelinkBar mobile body="câu trích góp ý" hasCandidates onCancel={noop} />)
    expect(mbar.textContent).toContain('Chạm vào đoạn mới')
    expect(mbar.textContent).toContain('viền vàng là đoạn có thể khớp')
    expect(mbar.textContent).toContain('Huỷ')
  })

  it('unsure: bấm Gắn vào đây trên đoạn gợi ý phát attach kèm đoạn', async () => {
    const actions: FeedbackAction[] = []
    const unsure = comment({ id: 'u', status: 'unsure', candidates: [{ paragraph: 2, snippet: 'Đoạn A' }, { paragraph: 4, snippet: 'Đoạn B' }] })
    const container = await mount(body([unsure], { openId: 'u', onAction: (action) => actions.push(action) }))
    const attach = [...container.querySelectorAll('.cand .fbtn')].find((node) => node.textContent === 'Gắn vào đây') as HTMLButtonElement
    await act(async () => attach.click())
    expect(actions).toEqual([{ kind: 'attach', comment: unsure, paragraph: 2, quote: 'Đoạn A' }])
  })
})

describe('§AI · không còn nhãn “Đoạn N” trong UI góp ý', () => {
  const list = [
    comment({ id: 'p', status: 'pending', paragraph: 2 }),
    comment({ id: 'l', status: 'lost', paragraph: 5 }),
    comment({ id: 'u', status: 'unsure', paragraph: 1, candidates: [{ paragraph: 3, snippet: 'x' }] }),
    comment({ id: 'f', status: 'fixed', paragraph: 0 }),
    comment({ id: 'd', status: 'done', paragraph: 4 }),
    comment({ id: 'w', status: 'pending', paragraph: null }),
  ]
  it('FeedbackBody và FeedbackSheet không render số đoạn', async () => {
    const panel = renderToStaticMarkup(body(list, { openId: 'u' }))
    expect(panel).not.toMatch(/Đoạn\s*\d|đoạn\s*\d/)
    const sheet = renderToStaticMarkup(
      <FeedbackSheet open onClose={noop} list={list} chapter={24} gate={false}
        relinkId={null} openId={null} onToggle={noop} onAction={noop} onSend={noop} busyId={null} />,
    )
    expect(sheet).not.toMatch(/Đoạn\s*\d|đoạn\s*\d/)
    expect(sheet).toContain('Góp ý chương 24')
  })
  it('CommentSheet gọi “đoạn này”, không đánh số', () => {
    const sheet = renderToStaticMarkup(
      <CommentSheet open onClose={noop} selected={{ paragraph: 4, quote: 'Câu trích' }} chips={[]} setChips={noop}
        note="" setNote={noop} onSubmit={noop} busy={false} error={null} keyboard={0} rewriteChapter={null} offline={false} />,
    )
    expect(sheet).toContain('Góp ý cho đoạn này')
    expect(sheet).not.toMatch(/Đoạn\s*\d|đoạn\s*\d/)
  })
})
