// GateCard — thẻ nghiệm thu từng chương (Main.dc.html › phase=gate).
// Điều kiện 06-data-contract.md: advanceMode=review && !isRunning && permit < chương kế.
// D4: chương còn mục “Bắt sửa” thì cửa duyệt đóng; 409 từ /next mang danh sách mục.
import type { Outline, ParagraphComment, ReviewEntry, ReviewItem, Snapshot } from '../../api/types'
import { t } from '../../i18n/vi'
import { translateHost } from '../../i18n/hostStrings'
import { verdictLabel } from '../../lib/labels'
import { packLabel } from '../settings/lib/packs'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { reviewIssueCounts, reviewItemsOf, reviewLevel } from '../../ui/ReviewIssue'

export type GateSnapshot = Pick<Snapshot, 'completedCount' | 'currentChapter' | 'runtimeState' | 'phase' | 'advanceMode' | 'isRunning' | 'advancePermitChapter'>
/** Dòng phụ của “Thẩm duyệt chương N”: “Đạt · còn 2 cảnh báo · chấm theo gói Nữ tần báo thù, lần 2”. */
export function gateReviewMeta(review: ReviewEntry | null): string {
  if (!review) return ''
  const { errors, warnings } = reviewIssueCounts(reviewItemsOf(review))
  const parts = [verdictLabel(review.verdict)]
  if (errors > 0) parts.push(t.workspace.gateMetaErrors(errors))
  else if (warnings > 0) parts.push(t.workspace.gateMetaWarnings(warnings))
  if (review.pack) parts.push(t.workspace.gateMetaPack(packLabel(review.pack)))
  const head = parts.filter(Boolean).join(' · ')
  return review.round ? t.workspace.gateMetaRound(head, review.round) : head
}

/** Mục Bắt sửa / Cảnh báo của lần chấm mới nhất. */
export function gateReviewItems(review: ReviewEntry | null): { errors: ReviewItem[]; warnings: ReviewItem[] } {
  const items = reviewItemsOf(review)
  return {
    errors: items.filter((item) => reviewLevel(item) === 'error'),
    warnings: items.filter((item) => reviewLevel(item) === 'warning'),
  }
}

/**
 * Dải xanh tóm tắt lần chấm trước (Main.dc.html › gateReview). Chỉ hiện khi lần chấm mới
 * nhất đã sạch mục bắt sửa và không phải lần đầu — dữ liệu mục của lần trước chưa có trong
 * hợp đồng nên câu chỉ nêu phần suy ra được.
 */
export function gateRoundNote(review: ReviewEntry | null): string {
  if (!review || !review.round || review.round < 2) return ''
  if (gateReviewItems(review).errors.length > 0) return ''
  const priorRound = review.round - 1
  const prior = review.previous?.find((entry) => entry.round === priorRound)
  if (prior) {
    const errors = prior.items.filter((item) => reviewLevel(item) === 'error')
    if (errors.length === 0) return ''
    const detail = errors.map((item) => `${item.ref} · ${item.text}`).join('; ')
    return t.workspace.gateRoundNoteDetail(prior.round, errors.length, detail, review.chapter)
  }
  return t.workspace.gateRoundNote(priorRound, review.chapter)
}

/** Chương kế tiếp cần được duyệt. */
export function nextGateChapter(snapshot: GateSnapshot | null): number {
  const completed = snapshot?.completedCount ?? 0
  return Math.max(completed + 1, snapshot?.currentChapter ?? 0)
}

/**
 * Truyện đã hoàn thành. `phase=complete` là dấu bền do engine ghi vào tiến độ nên
 * vẫn đúng sau khi worker khởi động lại (`runtimeState` trở về `idle`); phiên vừa
 * chốt xong thì `runtimeState=completed`.
 */
export function isCompleted(snapshot: Pick<Snapshot, 'runtimeState' | 'phase'> | null): boolean {
  return !!snapshot && (snapshot.runtimeState === 'completed' || snapshot.phase === 'complete')
}

export function isGate(snapshot: GateSnapshot | null): boolean {
  if (!snapshot || snapshot.advanceMode !== 'review' || snapshot.isRunning) return false
  if (isCompleted(snapshot)) return false
  return snapshot.advancePermitChapter < nextGateChapter(snapshot)
}

/** Số chương hoàn thành có thể thưa; không dùng tổng số chương làm định danh. */
export function latestGateChapter(snapshot: Snapshot | null, outline: Outline | null): number {
  const completed = outline?.volumes.flatMap((volume) => volume.chapters)
    .filter((chapter) => chapter.completed || chapter.status === 'done')
    .map((chapter) => chapter.chapter) ?? []
  return completed.length ? Math.max(...completed) : snapshot?.completedCount ?? 0
}

export const pendingCommentCount = (comments: ParagraphComment[]) =>
  comments.filter((comment) => !comment.appliedAt).length

export function GateCard({ snapshot, onNext, onReject, pendingComments = 0, commentsLoading = false, pending = false, success = false, rejecting = false, disabled = false, review = null, error = null }: {
  snapshot: Snapshot
  onNext: () => void
  onReject?: () => void
  pendingComments?: number
  commentsLoading?: boolean
  pending?: boolean
  success?: boolean
  rejecting?: boolean
  disabled?: boolean
  /** Thẩm duyệt mới nhất của chương đang chờ — còn mục Bắt sửa thì cửa duyệt đóng */
  review?: ReviewEntry | null
  /** Lỗi của lần duyệt (409 mang `items`) */
  error?: unknown
}) {
  const next = nextGateChapter(snapshot)
  const hold = snapshot.hasAdvanceHold && snapshot.advanceHoldReason ? translateHost(snapshot.advanceHoldReason) : ''
  const { errors } = gateReviewItems(review)
  // Mọi mục Bắt sửa đều đóng cửa duyệt — kể cả lỗi lint “Xưng hô” (22-brief-2510.md › câu 13).
  // BE đặt `blocking` khi vòng chấm có mục error; thiếu field thì suy từ mục Bắt sửa.
  const blocking = review ? (review.blocking ?? errors.length > 0) : false
  const errorText = error instanceof Error ? error.message : ''
  return (
    <div className="card" role="group" aria-label={t.advanceMode.review} style={{
      borderRadius: 12, padding: '10px 12px', display: 'flex', flexWrap: 'wrap',
      alignItems: 'center', gap: '10px 14px', borderColor: 'rgba(246,203,140,0.35)',
    }}>
      <Icon name="flag" size={18} style={{ color: 'var(--color-warn)' }} />
      <span style={{ flex: '1 1 260px', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <span style={{ fontSize: 13, color: 'var(--color-fg-2)' }}>{t.workspace.gate(snapshot.completedCount, next)}</span>
        {hold !== '' && <span style={{ fontSize: 12, color: 'var(--color-warn)' }}>{t.workspace.gateHold(hold)}</span>}
        {blocking && <span style={{ fontSize: 12, color: 'var(--color-err)' }}>{t.workspace.gateBlocking(errors.length)}</span>}
        {!blocking && errorText !== '' && <span style={{ fontSize: 12, color: 'var(--color-err)' }}>{errorText}</span>}
        {pendingComments > 0 && <span style={{ fontSize: 12, color: 'var(--color-warn)' }}>Sẽ áp dụng {pendingComments} góp ý</span>}
      </span>
      {onReject && <Button size="sm" onClick={onReject} disabled={disabled} pending={rejecting}
        style={{ fontSize: 13, paddingInline: 12 }}>Cần sửa</Button>}
      <Button variant="primary" size="sm" onClick={onNext} disabled={disabled || commentsLoading || blocking}
        tooltip={blocking ? t.workspace.gateBlockedHint : undefined}
        pending={pending} success={success} style={{ fontSize: 13, paddingInline: 12 }}>
        {t.workspace.approve(next)}
      </Button>
    </div>
  )
}
