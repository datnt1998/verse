import * as stylex from '@stylexjs/stylex'
import { Dialog, Icon } from '@noverse/ui'
import { extraBlockers, type EditorQuality, type QualityRange } from './quality'
import { rangeTime } from './FixPanel'
import { sx } from './editor.stylex'

export function ExportGateDialog({ quality, ranges, onAccept, onContinue, onClose }: {
  quality?: EditorQuality; ranges: readonly QualityRange[]
  onAccept: () => void; onContinue: () => void; onClose: () => void
}) {
  const blockers = quality ? extraBlockers(quality) : []
  const score = quality?.total ?? quality?.score?.total
  const explanation = ranges.length
    ? `Còn ${ranges.length} đoạn phụ đề lệch tiếng; khớp theo giọng mất khoảng 10 giây.`
    : blockers.length ? 'Video còn điều kiện bắt buộc chưa đạt.'
      : quality?.changedSinceMeasure ? 'Bản vừa sửa cần đo lại để cập nhật điểm.'
        : 'Chưa có kết quả đo mới đủ điều kiện xuất.'
  return <Dialog label="Xuất khi chưa đạt độ hoàn thiện?" alertDialog width={460} onClose={onClose}>
    <div {...stylex.props(sx.dialogBody)}>
      <div {...stylex.props(sx.row)}><Icon name="warning" size={20} /><h2 {...stylex.props(sx.dialogTitle)}>Xuất khi chưa đạt độ hoàn thiện?</h2></div>
      <p {...stylex.props(sx.dialogCopy)}>{score ?? 'Chưa đo'}/100, cần 90. {explanation}</p>
      <ul {...stylex.props(sx.dialogCopy)}>
        {ranges.length > 0 && <li>Phụ đề khớp tiếng: {ranges.map(range => rangeTime(range.start)).join(', ')}</li>}
        {blockers.map(blocker => <li key={blocker.id}>{blocker.message}</li>)}
        <li>Vẫn xuất: video chuyển sang Sẵn sàng đăng, Kiểm định ghi bạn đã chọn bỏ qua mục này</li>
      </ul>
      <div {...stylex.props(sx.dialogActions)}>
        <button type="button" {...stylex.props(sx.button)} onClick={onAccept}>Vẫn xuất</button>
        <button type="button" autoFocus {...stylex.props(sx.button, sx.primary)} onClick={onContinue}>Sửa tiếp</button>
      </div>
    </div>
  </Dialog>
}
