// D6 — dòng “Đo bằng máy · chương N” của Nhật ký (Activity.dc.html › nhóm “Thẩm duyệt, đo bằng máy & xưng hô”).
// Nhật ký là bản ghi nên hiện ĐỦ số đo của chương, kể cả số đạt: chip đạt viền xám + số xanh,
// chip lệch nền cam; mỗi chip luôn `tên · số đơn vị · chuẩn`. Số đo không áp cho chương (mở đầu
// trùng truyện cũ ngoài chương 1) không có chip; dòng thêm “Từ mệt nhiều nhất · “x”” đứng cuối.
import type { ChapterMetric, TiredWord } from '../../api/types'
import { t } from '../../i18n/vi'
import { metricStandard, metricValue, tiredWordLimit, tiredWordName, tiredWordValue } from '../../lib/metrics'
import { Icon } from '@noverse/ui'
import { formatClock } from '../workspace/format'

/** Một chip của dòng: số đạt viền xám, số lệch nền cam (artboard `MT`). */
function MetricChip({ name, value, standard, ok, severity }: {
  name: string; value: string; standard: string; ok: boolean; severity?: ChapterMetric['severity']
}) {
  return (
    <span title={`${name} ${value}${standard ? ` · ${standard}` : ''}`} style={{
      padding: '3px 8px', borderRadius: 6, fontSize: 12, color: 'var(--color-fg-4)', whiteSpace: 'nowrap',
      maxWidth: '100%', overflow: 'hidden', textOverflow: 'ellipsis',
      background: ok ? 'var(--color-surface-0)' : severity === 'error' ? 'var(--kit-errf-6)' : 'var(--kit-warn-6)',
      border: `1px solid ${ok ? 'var(--line-2)' : severity === 'error' ? 'var(--kit-errf-25)' : 'var(--kit-warn-25)'}`,
    }}>
      {name}{' '}
      <span className="num" style={{ color: ok ? 'var(--color-ok)' : severity === 'error' ? 'var(--kit-err-soft)' : 'var(--kit-warn-soft)' }}>{value}</span>
      {standard !== '' && <>{' '}<span style={{ color: 'var(--kit-fg-off)' }}>· {standard}</span></>}
    </span>
  )
}

export function MetricsLogRow({ chapter, metrics, tiredWord, time }: {
  chapter: number
  metrics: ChapterMetric[]
  /** từ lặp có mật độ cao nhất chương — dòng thêm của D6 */
  tiredWord?: TiredWord | null
  /** giờ của lần nộp chương — dòng số đo thuộc lần nộp đó */
  time: string
}) {
  const applicable = metrics.filter((metric) => metric.value !== null)
  return (
    <li style={{ display: 'flex', gap: 12, padding: '5px 12px', borderRadius: 10 }}>
      <span className="num mono" style={{ flex: '0 0 64px', paddingTop: 2, fontSize: 12, color: 'var(--color-fg-4)' }}>{formatClock(time)}</span>
      <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
          <Icon name="chart-2" size={15} style={{ marginTop: 2, color: 'var(--color-fg-4)' }} />
          <span style={{ flex: 1, minWidth: 0, fontSize: 13, color: 'var(--color-fg-3)' }}>{t.metrics.logRow(chapter)}</span>
        </div>
        <div style={{ marginLeft: 23, display: 'flex', flexWrap: 'wrap', gap: 6 }}>
          {applicable.map((metric) => <MetricChip key={metric.key} name={metric.name}
            value={metricValue(metric)}
            standard={metric.detail ?? (metric.limit === '0' ? '' : metricStandard(metric.limit))}
            ok={metric.ok} severity={metric.severity} />)}
          {tiredWord && <MetricChip name={tiredWordName(tiredWord.word)}
            value={tiredWordValue(tiredWord, true)}
            standard={metricStandard(tiredWordLimit(tiredWord))}
            ok={tiredWord.ok} />}
        </div>
      </div>
    </li>
  )
}
