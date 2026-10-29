// Gói AI — vòng đời góp ý theo đoạn (05-screens.md › 03b · 06-data-contract §AI).
// Logic thuần dùng chung cho bảng máy tính (tab Góp ý), sheet điện thoại và đánh dấu trong bài.
import type { CommentsReanchored, ParagraphComment, ParagraphCommentStatus } from '../../api/types'
import { anchorParagraph, commentStatus, quoteRange } from './comments'

export type FeedbackTone = 'p' | 's' | 'f' | 'w' | 'd'

export const STATUS_LABEL: Record<ParagraphCommentStatus, string> = {
  pending: 'Chờ gửi',
  sent: 'Đã gửi',
  fixed: 'Đã sửa',
  lost: 'Mất vị trí',
  unsure: 'Không chắc vị trí',
  done: 'Đã xử lý',
}
export const STATUS_TONE: Record<ParagraphCommentStatus, FeedbackTone> = {
  pending: 'p', sent: 's', fixed: 'f', lost: 'w', unsure: 'w', done: 'd',
}

export type FeedbackCounts = { pending: number; sent: number; fixed: number; unfixed: number; need: number; done: number }

export function feedbackCounts(list: readonly ParagraphComment[]): FeedbackCounts {
  const counts: FeedbackCounts = { pending: 0, sent: 0, fixed: 0, unfixed: 0, need: 0, done: 0 }
  for (const comment of list) {
    const status = commentStatus(comment)
    if (status === 'pending') { counts.pending++; if (comment.unfixed) counts.unfixed++ }
    else if (status === 'sent') counts.sent++
    else if (status === 'fixed') counts.fixed++
    else if (status === 'done') counts.done++
    else counts.need++
  }
  return counts
}

export type FeedbackGroupKey = 'need' | 'pending' | 'sent' | 'fixed'
export type FeedbackGroup = { key: FeedbackGroupKey; title: string; tone: FeedbackTone; note: string; items: ParagraphComment[] }

/** Thứ tự nhóm của 03b: Cần gắn lại → Chờ gửi → Đã gửi → Đã sửa; Đã xử lý gộp ở hàng cuối. */
export function feedbackGroups(list: readonly ParagraphComment[], nextChapter: number): FeedbackGroup[] {
  const groups: [FeedbackGroupKey, string, FeedbackTone, string, ParagraphCommentStatus[]][] = [
    ['need', 'Cần gắn lại', 'w', 'Câu trích cũ không còn sau lần viết lại', ['lost', 'unsure']],
    ['pending', 'Chờ gửi', 'p', '', ['pending']],
    ['sent', 'Đã gửi', 's', 'Người viết đang sửa', ['sent']],
    ['fixed', 'Đã sửa', 'f', `Tự ẩn khi bạn duyệt chương ${nextChapter}`, ['fixed']],
  ]
  return groups.map(([key, title, tone, note, statuses]) => ({
    key, title, tone, note,
    items: list.filter((comment) => statuses.includes(commentStatus(comment))),
  })).filter((group) => group.items.length > 0)
}

export const doneComments = (list: readonly ParagraphComment[]) => list.filter((comment) => commentStatus(comment) === 'done')

/** Câu trích gạch chân trong bài: mốc neo đã giải (anchor.quote — sau viết lại),
 *  không thì trích gốc; góp ý vừa gắn tay thì bỏ trích cũ → đánh dấu cả đoạn. */
const inlineQuote = (comment: ParagraphComment) =>
  commentStatus(comment) === 'fixed'
    ? (comment.newQuote || comment.anchor?.quote || comment.quote)
    : (comment.relinked ? '' : (comment.anchor?.quote || comment.quote))
export function paragraphMarks(list: readonly ParagraphComment[], text: string, index: number, openId: string | null): { marks: ParagraphMark[]; whole: boolean } {
  const marks: ParagraphMark[] = []
  let whole = false
  for (const comment of list) {
    const status = commentStatus(comment)
    if (status !== 'pending' && status !== 'sent' && status !== 'fixed') continue
    if (anchorParagraph(comment) !== index) continue
    const quote = inlineQuote(comment)
    const range = quote ? quoteRange(text, quote) : null
    if (!range) { whole = true; continue }
    marks.push({ comment, tone: STATUS_TONE[status], moved: !!comment.moved, at: range.at, len: range.len, open: comment.id === openId })
  }
  marks.sort((a, b) => a.at - b.at)
  return { marks, whole }
}

/** Đoạn viền vàng “Gợi ý n/N” khi thẻ Không chắc vị trí đang mở. */
export function candidateOrder(list: readonly ParagraphComment[], openId: string | null): number[] {
  const open = list.find((comment) => comment.id === openId)
  if (!open || commentStatus(open) !== 'unsure') return []
  return (open.candidates ?? []).map((candidate) => candidate.paragraph)
}

/** Nhãn đánh dấu của thẻ (hàng .ftag). */
export function commentTags(comment: ParagraphComment): string[] {
  const tags: string[] = []
  if (comment.unfixed) tags.push('Người viết chưa sửa chỗ này')
  if (comment.moved) tags.push('Đã tự gắn sang đoạn mới')
  if (comment.relinked) tags.push('Bạn đã gắn lại')
  if (comment.whole) tags.push('Gửi cho cả chương, kèm câu trích cũ')
  if (comment.id.startsWith('pending-')) tags.push('Đang gửi…')
  else if (comment.id.startsWith('queued-')) tags.push('Chờ gửi · sẽ tự gửi khi có mạng')
  return tags
}

export const fixedHow = (comment: ParagraphComment): string =>
  comment.fixedBy === 'agent' ? 'Người viết báo đã sửa' : 'Câu trích cũ không còn trong chương'
export type ReanchoredFrame = { project?: string; story?: string; chapter?: number; fixed?: number; unfixed?: number; need?: number; data?: unknown }

/** SSE `comments.reanchored` (phong bì phẳng {project, story, chapter, fixed, unfixed, need},
 *  hoặc khung `live` tương đương) của đúng truyện/chương không. */
export function reanchoredOf(frame: ReanchoredFrame & { kind?: string }, project: string, story: string, chapter: number): CommentsReanchored | null {
  if (frame.project !== project || frame.story !== story) return null
  const inner = frame.data && typeof frame.data === 'object' ? frame.data as Record<string, unknown> : null
  const source = frame.kind === 'comments.reanchored' && inner ? inner : frame as unknown as Record<string, unknown>
  if (Number(source.chapter) !== chapter) return null
  return { chapter, fixed: Number(source.fixed) || 0, unfixed: Number(source.unfixed) || 0, need: Number(source.need) || 0 }
}


/** Thân thẻ góp ý: chips + nhận xét nối “ · ” (artboard fbBody). */
export const commentBody = (comment: ParagraphComment) => [...comment.chips, ...(comment.text ? [comment.text] : [])].join(' · ')
/** Cắt câu trích theo ranh giới chữ (như artboard fbClip), không cắt giữa từ. */
export const clipWords = (text: string, n: number) => text.length <= n ? text : `${text.slice(0, text.lastIndexOf(' ', n))}…`
export function reanchorBanner(chapter: number, counts: FeedbackCounts): string {
  return `Người viết đã viết lại chương ${chapter} theo ${counts.fixed + counts.unfixed + counts.need} góp ý`
}


export type ParagraphMark = { comment: ParagraphComment; tone: FeedbackTone; moved: boolean; at: number; len: number; open: boolean }

/** Số trên tab/nút nổi: ưu tiên cần gắn lại (vàng), không thì chờ gửi (tím). */
export function attentionBadge(counts: FeedbackCounts): { tone: 'w' | 'p'; n: number } | null {
  if (counts.need > 0) return { tone: 'w', n: counts.need }
  if (counts.pending > 0) return { tone: 'p', n: counts.pending }
  return null
}
export function reanchorToast(event: CommentsReanchored): string {
  return `Đã viết lại chương ${event.chapter} · ${event.need} góp ý cần gắn lại`
}
