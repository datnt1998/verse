import { useState } from 'react'
import { BottomSheet } from '../../../mobile/primitives'

/** Hộp “Tạm dừng Người viết?” của MWork.dc.html (`sh.pause`): biểu tượng Người viết, một nút chính, một nút nguy hiểm, một nút quay lại. */
export function PauseSheet({ open, onClose, chapter, step, onPause }: { open: boolean; onClose: () => void; chapter: number; step: string; onPause: (afterStep: boolean) => Promise<void> }) {
  const [pending, setPending] = useState(false)
  const [failure, setFailure] = useState('')
  const choose = async (afterStep: boolean) => {
    setPending(true); setFailure('')
    try { await onPause(afterStep); onClose() }
    catch (error) { setFailure(error instanceof Error ? error.message : 'Không tạm dừng được') }
    finally { setPending(false) }
  }
  return <BottomSheet open={open} onClose={() => { if (!pending) onClose() }} height="auto" role="alertdialog" label="Tạm dừng Người viết?" style={{ padding: '0 16px 26px' }}>
    <div className="mw-pause-head">
      <span className="mw-pause-orb" aria-hidden="true"><span className="m-orb writer" /></span>
      <span className="mw-pause-title">Tạm dừng Người viết?</span>
      <span className="mw-pause-desc">Chương {chapter} đang ở bước {step}. Bạn chạy tiếp được bất cứ lúc nào.</span>
      {failure && <span className="m-error" role="alert">{failure}</span>}
    </div>
    <div className="mw-pause-actions">
      <button type="button" className="m-button" disabled={pending} aria-busy={pending} onClick={() => void choose(true)}>Tạm dừng sau bước này</button>
      <button type="button" className="m-button secondary mw-pause-danger" disabled={pending} onClick={() => void choose(false)}>Dừng ngay · bỏ phần đang viết</button>
      <button type="button" className="m-button mw-pause-back" disabled={pending} onClick={onClose}>Tiếp tục viết</button>
    </div>
  </BottomSheet>
}
