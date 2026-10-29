import { useRef } from 'react'
import { BottomSheet } from '../../../mobile/primitives'
import { PARAGRAPH_COMMENT_CHIPS, type ParagraphSelection } from '../comments'

export function CommentSheet({ open, onClose, selected, chips, setChips, note, setNote, onSubmit, busy, error, keyboard, rewriteChapter, offline }: {
  open: boolean; onClose: () => void; selected: ParagraphSelection; chips: string[]; setChips: (chips: string[]) => void
  note: string; setNote: (note: string) => void; onSubmit: () => void; busy: boolean; error: unknown
  keyboard: number; rewriteChapter: number | null; offline: boolean
}) {
  const input = useRef<HTMLTextAreaElement>(null)
  const typing = keyboard > 0
  const title = rewriteChapter !== null ? `Viết lại chương ${rewriteChapter}` : selected.paragraph === null ? 'Góp ý cho cả chương' : 'Góp ý cho đoạn này'
  return <BottomSheet open={open} onClose={onClose} label={title} height="auto" style={{ bottom: keyboard, maxHeight: `calc(100dvh - ${keyboard}px - 52px)`, paddingBottom: typing ? 14 : 26 }}>
    <div className="mr-sheet-content mr-comment">
      <h2 className="mr-sheet-title mr-comment-title">{title}</h2>
      {selected.quote && <blockquote className="mr-quote">{selected.quote}</blockquote>}
      <div className="mr-quick">{PARAGRAPH_COMMENT_CHIPS.map((chip) => <button key={chip} className={`m-chip${chips.includes(chip) ? ' on' : ''}`} aria-pressed={chips.includes(chip)} onClick={() => setChips(chips.includes(chip) ? chips.filter((item) => item !== chip) : [...chips, chip])}>{chip}</button>)}</div>
      <label className="mr-comment-label" htmlFor="mr-comment">Hoặc nói rõ muốn sửa thế nào<textarea ref={input} id="mr-comment" className="m-field" rows={3} value={note} onChange={(event) => setNote(event.target.value)} placeholder="Ví dụ: để Nhược Vy phản ứng gắt hơn" /></label>
      {error != null && <div className="m-error mr-comment-extra" role="alert">{error instanceof Error ? error.message : 'Không gửi được góp ý. Nội dung của bạn vẫn còn, hãy thử lại.'}</div>}
      {offline && rewriteChapter === null && <p className="mr-comment-hint mr-comment-extra">Góp ý sẽ chờ gửi và tự gửi khi có mạng.</p>}
      <div className="mr-comment-actions"><button className="m-button secondary" onClick={() => { if (typing) input.current?.blur(); else onClose() }}>{typing ? 'Xong' : 'Huỷ'}</button><button className="m-button" disabled={busy || (!note.trim() && !chips.length) || (rewriteChapter !== null && offline)} onClick={onSubmit}>{busy ? 'Đang gửi…' : 'Gửi cho Người viết'}</button></div>
      {!typing && <p className="mr-comment-hint mr-comment-foot">{rewriteChapter !== null ? `Chương ${rewriteChapter} sẽ được viết lại theo góp ý này.` : 'Chương sẽ được viết lại theo góp ý khi bạn bấm Duyệt.'}</p>}
    </div>
  </BottomSheet>
}
