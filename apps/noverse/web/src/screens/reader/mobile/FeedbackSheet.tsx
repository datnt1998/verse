// M2b — sheet Góp ý (MFeedback.dc.html fb): các nhóm vòng đời, chân “Gửi n góp ý”,
// hbar số lượng theo nhóm. Nội dung nhóm dùng chung FeedbackBody của máy tính.
import type { ParagraphComment } from '../../../api/types'
import { BottomSheet } from '../../../mobile/primitives'
import { FeedbackBody, FeedbackFooter, type FeedbackAction } from '../FeedbackGroups'

export function FeedbackSheet({ open, onClose, list, chapter, gate, relinkId, openId, onToggle, onAction, onSend, sendBusy, busyId, canChange }: {
  open: boolean
  onClose: () => void
  list: ParagraphComment[]
  chapter: number
  gate: boolean
  relinkId: string | null
  openId: string | null
  onToggle: (id: string) => void
  onAction: (action: FeedbackAction) => void
  onSend: () => void
  sendBusy?: boolean
  busyId: string | null
  /** Được sửa/xoá góp ý này không (người tạo · chủ dự án · quản trị viên). */
  canChange: (comment: ParagraphComment) => boolean
}) {
  const pending = list.filter((comment) => (comment.status ?? (comment.appliedAt ? 'sent' : 'pending')) === 'pending').length
  return (
    <BottomSheet open={open} onClose={onClose} label={`Góp ý chương ${chapter}`} height="auto">
      <div className="fbsheet">
        <h2 className="mr-sheet-title">{`Góp ý chương ${chapter}`}</h2>
        <FeedbackBody list={list} chapter={chapter} relinkId={relinkId} openId={openId}
          onToggle={onToggle} onAction={onAction} busyId={busyId} canChange={canChange} mobile />
      </div>
      <FeedbackFooter pending={pending} chapter={chapter} gate={gate} onSend={onSend} busy={sendBusy} />
    </BottomSheet>
  )
}
