// Gói AI — thẻ góp ý + nhóm vòng đời (05-screens.md › 03b · §AI), dùng chung cho
// ngăn máy tính (FeedbackPanel) và sheet điện thoại (FeedbackSheet).
import { useEffect, useRef, useState } from 'react'
import type { ParagraphComment } from '../../api/types'
import { Icon } from '@noverse/ui'
import { anchorParagraph, commentStatus } from './comments'
import { STATUS_LABEL, STATUS_TONE, commentBody, commentTags, doneComments, feedbackGroups, fixedHow, type FeedbackTone } from './feedback'

const clip = (text: string) => text.length > 90 ? `${text.slice(0, 87)}…` : text
const SHORT_TIME = new Intl.DateTimeFormat('vi-VN', { hour: '2-digit', minute: '2-digit' })
const timeAt = (iso?: string) => iso ? SHORT_TIME.format(new Date(iso)) : null
/** “x trước” theo vi-VN cho mốc góp ý (meta thẻ / dòng Đã xử lý). */
function ago(iso?: string): string {
  if (!iso) return ''
  const minutes = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 60000))
  if (minutes < 1) return 'vừa xong'
  if (minutes < 60) return `${minutes} phút trước`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours} giờ trước`
  return `${Math.round(hours / 24)} ngày trước`
}

export type FeedbackAction =
  | { kind: 'resolve'; comment: ParagraphComment }
  | { kind: 'reopen'; comment: ParagraphComment }
  | { kind: 'relink'; comment: ParagraphComment }
  | { kind: 'resend'; comment: ParagraphComment }
  | { kind: 'remove'; comment: ParagraphComment }
  | { kind: 'attach'; comment: ParagraphComment; paragraph: number; quote?: string }

export function FeedbackBody({ list, chapter, relinkId, openId, onToggle, onAction, onPeekCandidate, busyId, canChange, mobile }: {
  list: ParagraphComment[]
  /** Số chương sau chương đang xem — dòng ghi chú “Tự ẩn khi bạn duyệt chương N”. */
  chapter: number
  /** Đang ở chế độ gắn lại: chỉ hiện thẻ đích (03b fb:relink). */
  relinkId: string | null
  openId: string | null
  onToggle: (id: string) => void
  onAction: (action: FeedbackAction) => void
  /** Hover vào đoạn gợi ý → làm nổi đoạn đó trong bài (máy tính). */
  onPeekCandidate?: (paragraph: number | null) => void
  busyId: string | null
  /** Được sửa/xoá góp ý này không (người tạo · chủ dự án · quản trị viên). */
  canChange: (comment: ParagraphComment) => boolean
  mobile?: boolean
}) {
  if (relinkId) {
    const target = list.find((comment) => comment.id === relinkId)
    if (!target) return null
    return <div className="fbpanel"><FeedbackCard c={target} open={false} onToggle={onToggle} onAction={onAction} onPeekCandidate={onPeekCandidate} busy={busyId === target.id} editable={canChange(target)} mobile={mobile} /></div>
  }
  const groups = feedbackGroups(list, chapter + 1)
  if (groups.length === 0 && doneComments(list).length === 0) {
    return <div className="fbempty">Chưa có góp ý · quét chữ trong bài để góp ý đoạn</div>
  }
  return <div className="fbpanel">
    {groups.map((group) => (
      <section key={group.key} className="fgrp" aria-label={group.title} data-group={group.key}>
        <div className={`ghd2 ${group.tone}`}>
          <span className="gdot2" aria-hidden="true" />
          <b>{group.title}{` · ${group.items.length}`}</b>
          {group.note && <span>{group.note}</span>}
        </div>
        {group.items.map((comment) => (
          <FeedbackCard key={comment.id} c={comment} open={comment.id === openId} onToggle={onToggle} onAction={onAction} onPeekCandidate={onPeekCandidate} busy={busyId === comment.id} editable={canChange(comment)} mobile={mobile} />
        ))}
      </section>
    ))}
    <DoneRow list={doneComments(list)} />
  </div>
}

function DoneRow({ list }: { list: ParagraphComment[] }) {
  const [open, setOpen] = useState(false)
  if (list.length === 0) return null
  return <div className="fgrp">
    <button type="button" className="histrow" aria-expanded={open} onClick={() => setOpen(!open)}>
      <Icon name="tick-circle" size={15} />
      <span style={{ flex: 1 }}>{`Đã xử lý · ${list.length} góp ý`}</span>
      <Icon className="ic-x" name="arrow-down-2" size={13} />
    </button>
    {open && list.map((comment) => (
      <div key={comment.id} className="hrow">
        <span>{clip(comment.text)}</span>
        <span className="hrow-sub">
          {`${comment.quote ? `“${clip(comment.quote)}”` : 'Cả chương'} · ${ago(comment.sentAt || comment.resolvedAt || comment.fixedAt || comment.createdAt)}${comment.resolvedAt ? ` · xử lý ${ago(comment.resolvedAt)}` : ''}`}
        </span>
      </div>
    ))}
  </div>
}

function StatusChip({ c }: { c: ParagraphComment }) {
  const status = commentStatus(c)
  return <span className={`fchip ${STATUS_TONE[status]}`}>{status === 'sent' && <i className="fb-spin" aria-hidden="true" />}{STATUS_LABEL[status]}</span>
}

function FeedbackCard({ c, open, onToggle, onAction, onPeekCandidate, busy, editable, mobile }: {
  c: ParagraphComment
  open: boolean
  onToggle: (id: string) => void
  onAction: (action: FeedbackAction) => void
  onPeekCandidate?: (paragraph: number | null) => void
  busy: boolean
  /** Thẻ của người khác với cộng tác viên: chỉ đọc, không hiện nút sửa/xoá. */
  editable: boolean
  mobile?: boolean
}) {
  const cardRef = useRef<HTMLDivElement>(null)
  useEffect(() => { if (open) cardRef.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }) }, [open])
  const status = commentStatus(c)
  const tone: FeedbackTone = STATUS_TONE[status]
  const gone = status === 'lost' || status === 'unsure'
  const meta = status === 'done'
    ? `${c.author} · xử lý ${ago(c.resolvedAt ?? c.fixedAt ?? c.createdAt)}`
    : [c.author, timeAt(c.sentAt || c.createdAt) ?? ago(c.createdAt)].join(' · ')
  const candList = c.candidates ?? []
  return (
    <div ref={cardRef} className={`fcard ${tone}`} data-open={open || undefined} data-comment={c.id}
      onMouseEnter={() => onPeekCandidate?.(anchorParagraph(c))} onMouseLeave={() => onPeekCandidate?.(null)}>
      <button type="button" className="fhit" aria-expanded={open} onClick={() => onToggle(c.id)}>
        <span className="fhd">
          <StatusChip c={c} />
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{meta}</span>
        </span>
        {c.quote && <span className={`fq ${gone ? 'gone' : ''}`} title={c.quote}>{`“${clip(c.quote)}”`}</span>}
        <span className="fbody">{commentBody(c) || '—'}</span>
        {commentTags(c).map((tag) => <span key={tag} className="ftag">{tag}</span>)}
        {status === 'unsure' && !open && candList.length > 0 && (
          <span className="unsure-note">{`Có ${candList.length} đoạn có thể khớp · bấm để xem`}</span>
        )}
      </button>

      {status === 'fixed' && open && (
        c.newQuote ? (
          <div className="ba">
            <div><span className="lbl2">Trước</span><span className="old">{c.quote}</span></div>
            <div><span className="lbl2">Sau</span><span className="new">{c.newQuote}</span></div>
          </div>
        ) : <span className="fb-note">{fixedHow(c)}</span>
      )}

      {editable && status === 'unsure' && open && candList.length > 0 && (
        <div className="candlist">
          <span className="unsure-note">{mobile ? 'Có thể là một trong các đoạn sau:' : 'Có thể là một trong các đoạn sau (đang tô viền vàng trong bài):'}</span>
          {candList.map((candidate, index) => (
            <div key={candidate.paragraph} className="cand"
              onMouseEnter={() => onPeekCandidate?.(candidate.paragraph)} onMouseLeave={() => onPeekCandidate?.(null)}>
              <span className="cbadge2">{`Gợi ý ${index + 1}`}</span>
              <span className="cand-snip">{candidate.snippet}</span>
              <button type="button" className="fbtn pri" disabled={busy}
                onClick={() => onAction({ kind: 'attach', comment: c, paragraph: candidate.paragraph, quote: candidate.snippet })}>Gắn vào đây</button>
            </div>
          ))}
        </div>
      )}

      {editable && (status === 'lost' || status === 'unsure' || open) && (
        <div className="facts">
          {status === 'lost' && <>
            <button type="button" className="fbtn pri" disabled={busy} onClick={() => onAction({ kind: 'relink', comment: c })}>Gắn lại</button>
            <button type="button" className="fbtn" disabled={busy} onClick={() => onAction({ kind: 'resend', comment: c })}>Gửi lại</button>
            <button type="button" className="fbtn txt" disabled={busy} onClick={() => onAction({ kind: 'remove', comment: c })}>Xoá</button>
          </>}
          {status === 'unsure' && <>
            <button type="button" className="fbtn" disabled={busy} onClick={() => onAction({ kind: 'relink', comment: c })}>Chọn đoạn khác</button>
            <button type="button" className="fbtn" disabled={busy} onClick={() => onAction({ kind: 'resend', comment: c })}>Gửi lại</button>
            <button type="button" className="fbtn txt" disabled={busy} onClick={() => onAction({ kind: 'remove', comment: c })}>Xoá</button>
          </>}
          {status === 'pending' && open && (
            <button type="button" className="fbtn txt" disabled={busy || c.id.startsWith('queued-')} onClick={() => onAction({ kind: 'remove', comment: c })}>Xoá</button>
          )}
          {status === 'fixed' && open && <>
            <button type="button" className="fbtn pri" disabled={busy} onClick={() => onAction({ kind: 'resolve', comment: c })}>Ổn</button>
            <button type="button" className="fbtn" disabled={busy} onClick={() => onAction({ kind: 'reopen', comment: c })}>Chưa ổn</button>
          </>}
        </div>
      )}
    </div>
  )
}

/** Chân ngăn/sheet: nút gửi góp ý (03b fb:pending). `gate` = chương đang chờ duyệt. */
export function FeedbackFooter({ pending, chapter, gate, onSend, busy }: {
  pending: number
  chapter: number
  gate: boolean
  onSend: () => void
  busy?: boolean
}) {
  if (pending === 0) return null
  return (
    <div className="fbfoot">
      <button type="button" className="fbtn pri" style={{ height: 40, borderRadius: 11, fontSize: 13.5 }} disabled={busy} onClick={onSend}>
        {gate ? `Duyệt & gửi ${pending} góp ý cho Người viết` : `Gửi ${pending} góp ý · viết lại chương ${chapter}`}
      </button>
      <span className="fbfoot-note">{gate
        ? `Chương ${chapter} được viết lại theo góp ý, rồi viết tiếp chương ${chapter + 1}.`
        : 'Chỉ sửa chỗ có góp ý; phần còn lại giữ nguyên.'}</span>
    </div>
  )
}

/** Dải vàng chế độ gắn lại (03b pickbar / M2b .mpick). */
export function RelinkBar({ body, hasCandidates, onCancel, mobile }: {
  /** Câu trích gọn của thẻ đang gắn lại (để người dùng nhận ra đang gắn gì). */
  body: string
  /** Thẻ có đoạn gợi ý → nhắc “viền vàng là đoạn có thể khớp”. */
  hasCandidates?: boolean
  onCancel: () => void
  mobile?: boolean
}) {
  return (
    <div className={mobile ? 'mpick' : 'pickbar'} role="status">
      <Icon name="link-2" size={mobile ? 17 : 16} style={{ color: 'var(--color-warn)', flex: '0 0 auto' }} />
      <span className={mobile ? undefined : 'pickbar-body'} style={{ flex: 1, minWidth: 0 }}>
        {mobile
          ? <><b>Chạm vào đoạn mới</b>{` cho góp ý “${body}”${hasCandidates ? ' · viền vàng là đoạn có thể khớp' : ''}`}</>
          : <><b>Chọn chỗ mới cho góp ý</b>{` “${body}” — bấm vào một đoạn, hoặc bôi đen một câu.`}</>}
      </span>
      <button type="button" className={mobile ? 'mpick-x' : 'pickbar-x'} onClick={onCancel}>Huỷ</button>
    </div>
  )
}
