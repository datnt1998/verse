// Gói P · P4 — thứ tự ưu tiên của thẻ trạng thái rảnh (Main.dc.html phase=wait|ready|done).
import { describe, expect, it } from 'vitest'
import type { Snapshot } from '../../api/types'
import { idleModel } from './IdleCard'

const SNAP = {
  bookTitle: 'x', phase: 'writing', flow: 'writing', isRunning: false, runtimeState: 'idle',
  statusLabel: '', waitingFor: '', advanceMode: 'auto', advancePermitChapter: 0,
  hasAdvanceHold: false, advanceHoldReason: '', advanceHoldChapter: 0,
  currentChapter: 0, totalChapters: 8, completedCount: 0, inProgressChapter: 0,
  totalWordCount: 0, pendingRewrites: null, plan: null, outline: [], characters: [],
} as unknown as Snapshot

const snap = (patch: Partial<Snapshot>) => ({ ...SNAP, ...patch }) as Snapshot

describe('idleModel', () => {
  it('không có thẻ khi chưa tải, đang chạy, đang lập kế hoạch hoặc đang tạm dừng', () => {
    expect(idleModel(null)).toBeNull()
    expect(idleModel(snap({ isRunning: true }))).toBeNull()
    expect(idleModel(snap({ plan: { done: 0, total: 4, steps: [] } } as Partial<Snapshot>))).toBeNull()
    expect(idleModel(snap({ runtimeState: 'paused' }))).toBeNull()
  })

  it('done thắng mọi trạng thái khác', () => {
    expect(idleModel(snap({ phase: 'complete', completedCount: 8 }))?.phase).toBe('done')
  })

  it('cổng thẩm duyệt giữ khung riêng, không dùng thẻ', () => {
    const gate = snap({ advanceMode: 'review', hasAdvanceHold: true, advanceHoldReason: 'Chờ duyệt chương 4', completedCount: 3 })
    expect(idleModel(gate)).toBeNull()
  })

  it('wait: câu hỏi Trọng tài thật, phương án mặc định, chương chờ', () => {
    const model = idleModel(snap({
      completedCount: 5, runtimeState: 'paused',
      pendingAsk: {
        question: 'Giữ hay cắt?', chapter: 5,
        options: [{ label: 'Giữ', reply: 'Giữ cả hai cảnh' }, { label: 'Cắt', isDefault: true }],
      },
    }))
    expect(model?.phase).toBe('wait')
    expect(model?.question).toBe('Giữ hay cắt?')
    expect(model?.chapter).toBe(5)
    // is_default của Trọng tài được giữ nguyên; reply rỗng rơi về label.
    expect(model?.options).toHaveLength(2)
    expect(model?.options[0]).toMatchObject({ label: 'Giữ', reply: 'Giữ cả hai cảnh', isDefault: false })
    expect(model?.options[1]).toMatchObject({ label: 'Cắt', reply: 'Cắt', isDefault: true })
  })

  it('wait: không phương án nào đánh dấu thì nút đầu là nút chính', () => {
    const model = idleModel(snap({
      completedCount: 2,
      pendingAsk: { question: 'q', options: [{ label: 'A' }, { label: 'B' }] },
    }))
    expect(model?.options.map((option) => option.isDefault)).toEqual([true, false])
  })

  it('wait: không có phương án (ask_user tắt) vẫn hiện câu hỏi thật, không bịa nút', () => {
    const model = idleModel(snap({
      completedCount: 3, waitingFor: 'input', hasAdvanceHold: true, advanceHoldReason: 'Tạm dừng sau chương 3',
    }))
    expect(model).toMatchObject({ phase: 'wait', question: 'Tạm dừng sau chương 3', options: [] })
  })

  it('ready: dàn ý đã lưu, chưa viết chương nào', () => {
    expect(idleModel(snap({ phase: 'outline', completedCount: 0, totalChapters: 8 }))?.phase).toBe('ready')
  })

  it('chưa có dàn ý thì không có thẻ ready', () => {
    expect(idleModel(snap({ phase: 'outline', completedCount: 0, totalChapters: 0, outline: [] }))).toBeNull()
  })
})
