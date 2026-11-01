// Gói AI — ngăn “Góp ý” bên phải (05-screens.md › 03b · Feedback.dc.html fb).
// Soạn góp ý mới khi có lựa chọn, rồi các nhóm vòng đời từ FeedbackBody.
import { useEffect, useRef, useState } from 'react'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { PARAGRAPH_COMMENT_CHIPS, type ParagraphComments, type ParagraphSelection } from './comments'
import type { ParagraphComment } from '../../api/types'
import { FeedbackBody, FeedbackFooter, type FeedbackAction } from './FeedbackGroups'
import { copy } from './copy'
import './feedback.css'

export function FeedbackPanel({ comments, selection, setSelection, relinkId, openId, onToggle, onAction, onPeekCandidate, onSend, sendBusy, chapter, gate, busyId, canChange, focus, error, onRetry }: {
  comments: ParagraphComments
  selection: ParagraphSelection
  setSelection: (value: ParagraphSelection) => void
  relinkId: string | null
  openId: string | null
  onToggle: (id: string) => void
  onAction: (action: FeedbackAction) => void
  onPeekCandidate: (paragraph: number | null) => void
  onSend: () => void
  sendBusy?: boolean
  chapter: number
  /** Chương đang xem là chương kẹt duyệt (nhãn “Duyệt & gửi …”). */
  gate: boolean
  busyId: string | null
  /** Được sửa/xoá góp ý này không (người tạo · chủ dự án · quản trị viên). */
  canChange: (comment: ParagraphComment) => boolean
  /** Câu 11 — mở từ Chất lượng: con trỏ đặt sẵn trong ô góp ý cả chương. */
  focus?: boolean
  error: string | null
  onRetry: () => void
}) {
  const [chips, setChips] = useState<string[]>([])
  const [text, setText] = useState('')
  const [wholeOpen, setWholeOpen] = useState(false)
  const area = useRef<HTMLTextAreaElement>(null)
  useEffect(() => { if (focus) { setSelection({ paragraph: null, quote: '' }); setWholeOpen(true); setTimeout(() => area.current?.focus(), 0) } }, [focus, setSelection])
  if (comments.error) {
    return (
      <div className="scroll" style={{ flex: 1, padding: 18, display: 'grid', alignContent: 'center', justifyItems: 'center', gap: 10 }}>
        <span role="alert" style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>{typeof comments.error === 'object' && comments.error && 'message' in comments.error ? `Không tải được góp ý: ${(comments.error as Error).message}` : 'Không tải được góp ý'}</span>
        <Button size="sm" icon="refresh-2" onClick={onRetry}>Thử lại</Button>
      </div>
    )
  }
  const list = comments.data?.comments ?? []
  const save = () => {
    const input: import('../../api/types').ParagraphCommentInput = { paragraph: selection.paragraph, quote: selection.quote, chips, text }
    comments.add.run(input)
    setText(''); setChips([]); setWholeOpen(false); setSelection({ paragraph: null, quote: '' })
  }
  return (<>
    <div className="scroll" style={{ flex: 1, minHeight: 0, padding: '10px 14px', display: 'flex', flexDirection: 'column', gap: 14 }}>
      {/* Soạn góp ý: chip của đoạn đang chọn, hoặc nút mở cho cả chương. */}
      {selection.quote ? (
        <>
          <section style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <span style={{ fontSize: 11, fontWeight: 600, letterSpacing: '0.08em', color: 'var(--color-fg-4)' }}>ĐOẠN NÀY</span>
            <blockquote style={{ margin: 0, paddingLeft: 10, borderLeft: '2px solid var(--color-accent)', fontFamily: 'Literata, Georgia, serif', fontSize: 13.5, lineHeight: 1.6, color: 'var(--color-fg-2)', display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{selection.quote}</blockquote>
          </section>
          <section style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {PARAGRAPH_COMMENT_CHIPS.map((chip) => {
                const active = chips.includes(chip)
                return (
                  <button key={chip} type="button" aria-pressed={active}
                    onClick={() => setChips(active ? chips.filter((value) => value !== chip) : [...chips, chip])}
                    style={{ height: 26, padding: '0 10px', borderRadius: 7, fontSize: 12, border: '1px solid var(--line-2)', background: active ? 'var(--fb-p-bg-16)' : 'transparent', color: active ? 'var(--fb-cream)' : 'var(--color-fg-2)' }}>{chip}</button>
                )
              })}
            </div>
            <textarea ref={area} rows={3} placeholder="Thêm nhận xét…" value={text} onChange={(event) => setText(event.target.value)}
              style={{ resize: 'vertical', padding: '9px 12px', borderRadius: 10, border: '1px solid var(--line-2)', background: 'var(--color-surface-0)', color: 'var(--color-fg)', font: 'inherit', fontSize: 13, lineHeight: 1.5 }} />
            <Button variant="primary" size="sm" icon="add" disabled={!text.trim() && chips.length === 0} pending={comments.add.pending} pendingLabel="Đang lưu…" onClick={save}>Lưu góp ý</Button>
          </section>
        </>
      ) : (
        <section style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 10 }}>
            <span style={{ fontSize: 13.5, fontWeight: 600 }}>Nhận xét</span>
            <Button variant="ghost" size="sm" icon="add" onClick={() => { setSelection({ paragraph: null, quote: '' }); setWholeOpen(true); setTimeout(() => area.current?.focus(), 0) }}>Góp ý cả chương</Button>
          </div>
          <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)', lineHeight: 1.5 }}>{'Quét chữ trong bài để gắn góp ý vào đúng đoạn.'}</span>
          {wholeOpen && (
            <>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                {PARAGRAPH_COMMENT_CHIPS.map((chip) => {
                  const active = chips.includes(chip)
                  return (
                    <button key={chip} type="button" aria-pressed={active}
                      onClick={() => setChips(active ? chips.filter((value) => value !== chip) : [...chips, chip])}
                      style={{ height: 26, padding: '0 10px', borderRadius: 7, fontSize: 12, border: '1px solid var(--line-2)', background: active ? 'var(--fb-p-bg-16)' : 'transparent', color: active ? 'var(--fb-cream)' : 'var(--color-fg-2)' }}>{chip}</button>
                  )
                })}
              </div>
              <textarea ref={area} rows={3} placeholder="Thêm nhận xét…" value={text} onChange={(event) => setText(event.target.value)}
                style={{ resize: 'vertical', padding: '9px 12px', borderRadius: 10, border: '1px solid var(--line-2)', background: 'var(--color-surface-0)', color: 'var(--color-fg)', font: 'inherit', fontSize: 13, lineHeight: 1.5 }} />
              <Button variant="primary" size="sm" icon="add" disabled={!text.trim() && chips.length === 0} pending={comments.add.pending} pendingLabel="Đang lưu…" onClick={save}>Lưu góp ý</Button>
            </>
          )}
        </section>
      )}

      {comments.initialLoading ? (
        <div role="status" style={{ display: 'flex', gap: 8, alignItems: 'center', color: 'var(--color-fg-4)', fontSize: 12.5 }}><Spinner size={13} />Đang tải góp ý…</div>
      ) : (
        <FeedbackBody list={list} chapter={chapter} relinkId={relinkId} openId={openId} onToggle={onToggle} onAction={onAction} onPeekCandidate={onPeekCandidate} busyId={busyId} canChange={canChange} />
      )}
      {error && <span role="alert" style={{ fontSize: 12.5, color: 'var(--color-danger-2)' }}>{error}</span>}
    </div>
    <FeedbackFooter pending={list.filter((c) => (c.status ?? (c.appliedAt ? 'sent' : 'pending')) === 'pending').length} chapter={chapter} gate={gate} onSend={onSend} busy={sendBusy} />
  </>)
}

/** Viên chữ nổi + số lượng trên tab Góp ý / hàng chương. */
export function FeedbackBadge({ tone, n }: { tone: 'w' | 'p'; n: number }) {
  return <span className={`tbadge ${tone}`} aria-hidden="true">{n}</span>
}
export function ChapterRowBadge({ comments }: { comments: { need: number; pending: number } | null | undefined }) {
  if (!comments) return null
  if (comments.need > 0) return <span className="fbn w" title={`${comments.need} góp ý cần gắn lại`}>{comments.need}</span>
  if (comments.pending > 0) return <span className="fbn p" title={`${comments.pending} góp ý chờ gửi`}>{comments.pending}</span>
  return null
}

