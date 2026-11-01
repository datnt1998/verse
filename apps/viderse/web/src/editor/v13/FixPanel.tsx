import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Icon, Spinner } from '@noverse/ui'
import type { EditorQuality, QualityRange } from './quality'
import { canExport, extraBlockers } from './quality'
import { sx } from './editor.stylex'

export function rangeTime(seconds: number) {
  const value = Math.max(0, Math.floor(seconds))
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`
}
export function FixPanel({ quality, ranges, selectedRange, fixedIds, onSelectRange, onAlignAll, onMeasure, aligning }: {
  quality: EditorQuality; ranges: QualityRange[]; selectedRange: number; fixedIds: Set<string>
  onSelectRange: (index: number) => void; onAlignAll: () => Promise<void>; onMeasure: () => Promise<void>; aligning: boolean
}) {
  const [passOpen, setPassOpen] = useState(false)
  const passed = canExport(quality)
  const blockers = extraBlockers(quality)
  const left = ranges.filter((range) => !fixedIds.has(range.cueId ?? `${range.start}`) && range.status !== 'pass')
  const fixed = ranges.length - left.length
  const stale = quality.items.filter((item) => item.stale)
  const remeasure = stale.length ? stale : fixed ? [
    { id: 'captionAlignment', label: 'Phụ đề khớp tiếng', evidence: `${fixed} đoạn vừa khớp` },
    { id: 'captionKaraoke', label: 'Phụ đề karaoke', evidence: 'mốc từng từ đổi' },
  ] : []
  const score = quality.total ?? quality.score?.total
  const passItems = quality.items.filter((item) => item.status === 'pass' && !item.stale)
  return <>
    <div {...stylex.props(sx.fixBody)}>
      <div {...stylex.props(sx.stack)}>
        <div {...stylex.props(sx.row, sx.scoreRow)}><span {...stylex.props(sx.spread, sx.secondary)}>Độ hoàn thiện</span><b {...stylex.props(sx.score, passed ? sx.ok : sx.danger)}>{score ?? '—'}</b><span {...stylex.props(sx.secondary)}>/100</span></div>
        <div {...stylex.props(sx.scoreTrack)} aria-hidden="true"><span {...stylex.props(sx.scoreFill(score ?? 0, passed))} /><span {...stylex.props(sx.threshold)} title="Cần 90" /></div>
        <span {...stylex.props(sx.secondary)}>{passed ? 'Đã đạt. Xuất được rồi; xuất xong video chuyển sang Sẵn sàng.' : quality.measuring ? 'Kiểm định đang đo lại sau khi bạn sửa.' : score == null ? 'Chưa đo độ hoàn thiện. Đo lần đầu để biết video còn thiếu gì.' : left.length ? `Cần 90 để tự xuất. Còn ${left.length} đoạn phụ đề lệch tiếng.` : (blockers.length ? 'Video còn điều kiện bắt buộc chưa đạt. Xem mục bên dưới.' : 'Đã sửa xong. Đo lại để cập nhật điểm.')}</span>
      </div>
      {!passed && !quality.measuring && <section {...stylex.props(sx.stack)} aria-labelledby="fx-h">
        <h3 id="fx-h" {...stylex.props(sx.group)}>Chưa đạt <span {...stylex.props(sx.groupCount)}>{quality.items.filter((item) => item.status === 'fail').length + blockers.length}</span></h3>
        {ranges.length > 0 && <div {...stylex.props(sx.failBox)}>
          <div {...stylex.props(sx.row, sx.failHeading)}><Icon name="danger" size={15} {...stylex.props(sx.warning)} /><div {...stylex.props(sx.failTitleColumn, sx.spread)}><b {...stylex.props(sx.failTitle)}>Phụ đề khớp tiếng</b><span {...stylex.props(sx.small)}>{left.length ? `${left.length} đoạn chữ lệch tiếng` : `Đã khớp cả ${ranges.length} đoạn`}</span></div></div>
          <div role="list" {...stylex.props(sx.cueList)}>{ranges.map((range, index) => {
            const ok = fixedIds.has(range.cueId ?? `${range.start}`) || range.status === 'pass'
            const offset = range.offsetSec
            return <button key={range.cueId ?? `${range.start}-${index}`} role="listitem" type="button" aria-current={index === selectedRange ? 'true' : undefined} {...stylex.props(sx.cueButton, index === selectedRange && sx.selectedCue)} onClick={() => onSelectRange(index)}>
              <span {...stylex.props(sx.cueTime)}>{rangeTime(range.start)}</span><span title={range.text} {...stylex.props(sx.cueText)}>{range.text ?? range.checkId}</span>
              <span {...stylex.props(sx.cueStatus, ok ? sx.ok : sx.danger)}>{ok ? 'đã khớp' : offset == null ? 'lệch tiếng' : `${offset < 0 ? 'sớm' : 'muộn'} ${Math.abs(offset).toLocaleString('vi-VN')} s`}</span>
            </button>
          })}</div>
          {left.length > 0 && <div {...stylex.props(sx.fit)}><Button size="sm" disabled={aligning} onClick={() => { void onAlignAll() }} icon={<Icon name="magic" size={14} />}>Khớp theo giọng cả <span>{left.length}</span> đoạn</Button></div>}
        </div>}
        {quality.items.filter((item) => item.status !== 'pass' && !item.ranges?.length && !ranges.some((range) => range.checkId === item.id)).map((item) => <div key={item.id} {...stylex.props(sx.failBox)}><b>{item.label}</b><span {...stylex.props(sx.small)}>{item.evidence}</span></div>)}
        {blockers.map(blocker => <div key={blocker.id} {...stylex.props(sx.failBox)}><span {...stylex.props(sx.row)}><Icon name="danger" size={15} /><b {...stylex.props(sx.failTitle)}>{blocker.message}</b></span></div>)}
      </section>}
      {remeasure.length > 0 && !quality.measuring && <section {...stylex.props(sx.groupSection)} aria-label="Cần đo lại"><h3 {...stylex.props(sx.group)}>Cần đo lại <span {...stylex.props(sx.groupCount)}>{remeasure.length}</span></h3>{remeasure.map((item) => <span key={item.id} {...stylex.props(sx.remeasure)}><Icon name="clock" size={14} /><span {...stylex.props(sx.spread)}>{item.label}</span><span {...stylex.props(sx.small)}>{item.evidence}</span></span>)}</section>}
      <section {...stylex.props(sx.groupSection)}><div {...stylex.props(sx.fit)}><Button size="sm" variant="quiet" aria-expanded={passOpen || passed} onClick={() => setPassOpen(!passOpen)} icon={<Icon name="arrow-d" size={13} />}>Đã đạt <span>{passItems.length}</span></Button></div>{(passOpen || passed) && <div {...stylex.props(sx.cueList)}>{passItems.map((item) => <span key={item.id} {...stylex.props(sx.passRow)}><Icon name="tick-b" size={14} {...stylex.props(sx.ok)} /><span {...stylex.props(sx.spread)}>{item.label}</span><span {...stylex.props(sx.passEvidence)}>{item.evidence}</span></span>)}</div>}</section>
    </div>
    <div {...stylex.props(sx.fixFooter)}>
      {quality.measuring ? <div {...stylex.props(sx.measureStatus)}><span role="status" {...stylex.props(sx.row)}><Spinner size={16} />Kiểm định đang đo lại</span><div role="progressbar" aria-label="Tiến độ đo độ hoàn thiện" {...stylex.props(sx.scoreTrack, sx.measureTrack)}><span {...stylex.props(sx.measureSweep)} /></div></div> : !passed && <Button block disabled={aligning || !quality.jobId || (score != null && quality.changedSinceMeasure === 0)} variant={remeasure.length > 0 || score == null ? 'primary' : 'ghost'} onClick={() => { void onMeasure() }} icon={<Icon name="refresh" size={15} />}>{score == null ? 'Đo độ hoàn thiện' : 'Đo lại độ hoàn thiện'}</Button>}
      <span {...stylex.props(sx.note)}>{quality.measuring ? 'Bạn vẫn sửa tiếp được. Xem tiến độ ở Việc nền.' : passed ? 'Sửa thêm sẽ cần đo lại trước khi xuất.' : 'Đo lại sau khi sửa. Kiểm định tự cập nhật điểm và danh sách cần sửa.'}</span>
    </div>
  </>
}
