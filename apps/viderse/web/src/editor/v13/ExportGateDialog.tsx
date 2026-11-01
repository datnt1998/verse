import * as stylex from '@stylexjs/stylex'
import { Button, Dialog, Icon } from '@noverse/ui'
import { extraBlockers, type EditorQuality, type QualityRange } from './quality'
import { rangeTime } from './FixPanel'
import { sx } from './editor.stylex'
import { gateStyles as s } from './gate.stylex'

export function ExportGateDialog({ quality, ranges, onAccept, onContinue, onClose }: {
  quality?: EditorQuality; ranges: readonly QualityRange[]
  onAccept: () => void; onContinue: () => void; onClose: () => void
}) {
  const blockers = quality ? extraBlockers(quality) : []
  const score = quality?.total ?? quality?.score?.total
  const explanation = ranges.length
    ? `Còn ${ranges.length} đoạn phụ đề lệch tiếng.`
    : blockers.length ? 'Video còn điều kiện bắt buộc chưa đạt.'
      : quality?.changedSinceMeasure ? 'Bản vừa sửa cần đo lại để cập nhật điểm.'
        : 'Chưa có kết quả đo mới đủ điều kiện xuất.'
  return <Dialog label="Xuất khi chưa đạt độ hoàn thiện?" alertDialog width={460} onClose={onClose} padBody={false}>
    <div {...stylex.props(sx.dialogBody)}>
      <div {...stylex.props(s.head)}><span {...stylex.props(s.icon)}><Icon name="warning" size={20} /></span><div {...stylex.props(s.text)}><h2 {...stylex.props(s.title)}>Xuất khi chưa đạt độ hoàn thiện?</h2><p {...stylex.props(s.description)}>{score ?? 'Chưa đo'}/100, cần 90. {explanation}</p></div></div>
      <ul {...stylex.props(s.list)}>
        {ranges.length > 0 && <li>Phụ đề khớp tiếng: {ranges.map(range => rangeTime(range.start)).join(', ')}</li>}
        {blockers.map(blocker => <li key={blocker.id}>{blocker.message}</li>)}
        <li>Vẫn xuất: video chuyển sang Sẵn sàng, Kiểm định ghi bạn đã chọn bỏ qua.</li>
      </ul>
      <div {...stylex.props(sx.dialogActions)}>
        <Button onClick={onAccept}>Vẫn xuất</Button>
        <Button autoFocus variant="primary" onClick={onContinue}>Sửa tiếp</Button>
      </div>
    </div>
  </Dialog>
}
