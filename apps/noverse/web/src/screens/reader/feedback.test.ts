// §AI — bảo chứng cho logic vòng đời góp ý (feedback.ts / comments.ts):
// thứ tự nhóm, suy luận trạng thái, mốc neo thay chỉ số đoạn, badge và khung reanchor.
import { describe, expect, it } from 'vitest'
import type { ParagraphComment, ParagraphCommentStatus } from '../../api/types'
import { anchorParagraph, commentStatus } from './comments'
import { attentionBadge, candidateOrder, commentTags, feedbackCounts, feedbackGroups, paragraphMarks, reanchoredOf } from './feedback'

let seq = 0
const comment = (over: Partial<ParagraphComment> & { status?: ParagraphCommentStatus }): ParagraphComment => ({
  id: `c-${++seq}`, paragraph: 0, quote: '', chips: [], text: 'Góp ý', author: 'dat', createdAt: '2026-10-28T10:00:00Z', ...over,
})

describe('§AI · commentStatus — suy luận vòng đời', () => {
  it('ưu tiên status; bản cũ thiếu status suy ra từ appliedAt', () => {
    expect(commentStatus(comment({ status: 'pending' }))).toBe('pending')
    expect(commentStatus(comment({ status: 'sent' }))).toBe('sent')
    expect(commentStatus(comment({ status: 'fixed' }))).toBe('fixed')
    expect(commentStatus(comment({ status: 'lost' }))).toBe('lost')
    expect(commentStatus(comment({ status: 'unsure' }))).toBe('unsure')
    expect(commentStatus(comment({ status: 'done' }))).toBe('done')
    expect(commentStatus(comment({ appliedAt: '2026-10-28T10:05:00Z' }))).toBe('sent')
    expect(commentStatus(comment({}))).toBe('pending')
    // Người viết đã chạy nhưng không sửa chỗ này → góp ý quay về Chờ gửi (không phải Đã gửi/Đã sửa).
    expect(commentStatus(comment({ status: 'pending', unfixed: true, sentAt: '2026-10-28T10:05:00Z' }))).toBe('pending')
  })
})

describe('§AI · feedbackGroups — thứ tự 03b', () => {
  it('Cần gắn lại đứng đầu, rồi Chờ gửi → Đã gửi → Đã sửa; Đã xử lý không phải một nhóm', () => {
    const list = [
      comment({ id: 'done', status: 'done' }),
      comment({ id: 'fixed', status: 'fixed' }),
      comment({ id: 'sent', status: 'sent' }),
      comment({ id: 'unfixed', status: 'pending', unfixed: true }),
      comment({ id: 'lost', status: 'lost' }),
      comment({ id: 'pending', status: 'pending' }),
      comment({ id: 'unsure', status: 'unsure' }),
    ]
    const groups = feedbackGroups(list, 4)
    expect(groups.map((group) => group.key)).toEqual(['need', 'pending', 'sent', 'fixed'])
    expect(groups[0].items.map((item) => item.id)).toEqual(['lost', 'unsure'])
    expect(groups[1].items.map((item) => item.id)).toEqual(['unfixed', 'pending'])
    expect(groups[0].title).toBe('Cần gắn lại')
    expect(groups[3].note).toContain('chương 4')
    expect(groups.flatMap((group) => group.items.map((item) => item.id))).not.toContain('done')
  })

  it('nhóm rỗng bị lược bỏ', () => {
    const groups = feedbackGroups([comment({ status: 'pending' })], 2)
    expect(groups.map((group) => group.key)).toEqual(['pending'])
  })
})

describe('§AI · feedbackCounts / attentionBadge', () => {
  it('đếm từng trạng thái; unfixed vẫn nằm trong pending', () => {
    const counts = feedbackCounts([
      comment({ status: 'pending' }), comment({ status: 'pending', unfixed: true }),
      comment({ status: 'sent' }), comment({ status: 'fixed' }),
      comment({ status: 'lost' }), comment({ status: 'unsure' }), comment({ status: 'done' }),
    ])
    expect(counts).toEqual({ pending: 2, sent: 1, fixed: 1, unfixed: 1, need: 2, done: 1 })
  })
  it('badge ưu tiên cần gắn lại (vàng), rồi chờ gửi (tím), không có thì không hiện', () => {
    const base = { pending: 0, sent: 0, fixed: 0, unfixed: 0, need: 0, done: 0 }
    expect(attentionBadge({ ...base, need: 2, pending: 3 })).toEqual({ tone: 'w', n: 2 })
    expect(attentionBadge({ ...base, pending: 3 })).toEqual({ tone: 'p', n: 3 })
    expect(attentionBadge({ ...base, sent: 1, fixed: 2 })).toBeNull()
  })
})

describe('§AI · anchorParagraph — badge theo mốc neo, không theo chỉ số gốc', () => {
  it('anchor.paragraph thắng paragraph gốc; thiếu anchor về paragraph', () => {
    expect(anchorParagraph(comment({ status: 'pending', paragraph: 2, anchor: { paragraph: 5, quote: 'x' } }))).toBe(5)
    expect(anchorParagraph(comment({ status: 'fixed', paragraph: 2, anchor: { paragraph: 7, quote: 'y' } }))).toBe(7)
    expect(anchorParagraph(comment({ status: 'pending', paragraph: 3 }))).toBe(3)
    expect(anchorParagraph(comment({ status: 'pending', paragraph: null }))).toBeNull()
  })
  it('lost/unsure/done không gắn vào đoạn nào', () => {
    for (const status of ['lost', 'unsure', 'done'] as const) {
      expect(anchorParagraph(comment({ status, paragraph: 2, anchor: { paragraph: 5, quote: 'x' } }))).toBeNull()
    }
  })
})

describe('§AI · paragraphMarks — vết gạch theo anchor, không theo chỉ số', () => {
  const text = 'Câu đầu của đoạn. Câu hai dài hơn một chút để trích.'
  it('gắn vết vào đoạn anchor.paragraph và gạch đúng anchor.quote', () => {
    const moved = comment({ id: 'moved', status: 'pending', paragraph: 0, quote: 'Câu đầu của đoạn.', moved: true, anchor: { paragraph: 2, quote: 'Câu hai dài hơn một chút' } })
    const atOld = paragraphMarks([moved], text, 0, null)
    expect(atOld.marks).toHaveLength(0)
    expect(atOld.whole).toBe(false)
    const atAnchor = paragraphMarks([moved], text, 2, null)
    expect(atAnchor.marks).toHaveLength(1)
    expect(atAnchor.marks[0]).toMatchObject({ moved: true, tone: 'p', at: text.indexOf('Câu hai'), len: 'Câu hai dài hơn một chút'.length })
  })
  it('góp ý relink thủ công bỏ trích cũ → đánh dấu cả đoạn; thẻ Đã sửa gạch newQuote', () => {
    const relinked = comment({ id: 'relinked', status: 'pending', paragraph: 1, relinked: true, quote: 'câu cũ đã mất', anchor: { paragraph: 1, quote: 'câu cũ đã mất' } })
    expect(paragraphMarks([relinked], text, 1, null)).toEqual({ marks: [], whole: true })
    const fixed = comment({ id: 'fixed', status: 'fixed', paragraph: 1, quote: 'Câu đầu của đoạn.', newQuote: 'Câu hai dài hơn một chút', anchor: { paragraph: 1, quote: 'Câu hai dài hơn một chút' } })
    const { marks, whole } = paragraphMarks([fixed], text, 1, 'fixed')
    expect(whole).toBe(false)
    expect(marks[0]).toMatchObject({ tone: 'f', open: true, at: text.indexOf('Câu hai') })
  })
  it('lost/unsure/done không tạo vết; mark cờ open khi thẻ mở', () => {
    expect(paragraphMarks([comment({ status: 'lost', paragraph: 0 })], text, 0, null)).toEqual({ marks: [], whole: false })
    const open = comment({ id: 'open', status: 'sent', paragraph: 0, quote: 'Câu đầu của đoạn.' })
    expect(paragraphMarks([open], text, 0, 'open').marks[0].open).toBe(true)
    expect(paragraphMarks([open], text, 0, null).marks[0].tone).toBe('s')
  })
})

describe('§AI · candidateOrder / commentTags', () => {
  it('thẻ unsure mở → đoạn gợi ý; thẻ khác hoặc đóng → rỗng', () => {
    const unsure = comment({ id: 'u', status: 'unsure', candidates: [{ paragraph: 4, snippet: 'a' }, { paragraph: 6, snippet: 'b' }] })
    const list = [unsure, comment({ id: 'p', status: 'pending' })]
    expect(candidateOrder(list, 'u')).toEqual([4, 6])
    expect(candidateOrder(list, 'p')).toEqual([])
    expect(candidateOrder(list, null)).toEqual([])
  })
  it('nhãn thẻ theo cờ vòng đời', () => {
    expect(commentTags(comment({ status: 'pending', unfixed: true }))).toEqual(['Người viết chưa sửa chỗ này'])
    expect(commentTags(comment({ status: 'pending', moved: true, relinked: true }))).toEqual(['Đã tự gắn sang đoạn mới', 'Bạn đã gắn lại'])
    expect(commentTags(comment({ status: 'pending', whole: true }))).toEqual(['Gửi cho cả chương, kèm câu trích cũ'])
    expect(commentTags(comment({ id: 'queued-1', status: 'pending' }))).toEqual(['Chờ gửi · sẽ tự gửi khi có mạng'])
    expect(commentTags(comment({ id: 'pending-9', status: 'pending' }))).toEqual(['Đang gửi…'])
  })
})

describe('§AI · reanchoredOf — khung SSE của đúng chương', () => {
  it('nhận phong bì phẳng và khung live; sai truyện/chương → null', () => {
    const flat = { project: 'p', story: 's', chapter: 24, fixed: 2, unfixed: 1, need: 3 }
    expect(reanchoredOf(flat, 'p', 's', 24)).toEqual({ chapter: 24, fixed: 2, unfixed: 1, need: 3 })
    const live = { kind: 'comments.reanchored', data: { project: 'p', story: 's', chapter: 24, fixed: 1, unfixed: 0, need: 0 }, project: 'p', story: 's' }
    expect(reanchoredOf(live, 'p', 's', 24)).toEqual({ chapter: 24, fixed: 1, unfixed: 0, need: 0 })
    expect(reanchoredOf(flat, 'p', 's', 25)).toBeNull()
    expect(reanchoredOf(flat, 'other', 's', 24)).toBeNull()
    expect(reanchoredOf({ project: 'p', story: 's' }, 'p', 's', 24)).toBeNull()
  })
})
