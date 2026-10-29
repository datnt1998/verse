// D6 — “Kiểm tra 3 chương gần nhất” (Settings.dc.html › metrics=issues · metrics=numbers).
// Dữ liệu: `GET …/metrics?chapters=last3` (chín số đo của gói cho từng chương đã viết).
// Mặc định chỉ hiện chỗ lệch chuẩn; nút “Xem đủ số đo chương N” mở bảng đủ chín dòng của chương mới nhất.
import { useState } from 'react'
import { getStoryMetrics } from '../../../../api/client'
import { keys, useResource } from '../../../../data'
import { t } from '../../../../i18n/vi'
import { metricStandard, metricValue, tiredWordLimit, tiredWordName, tiredWordValue } from '../../../../lib/metrics'
import { paths } from '../../../../router'
import { Card } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { metricsCardRows } from './metrics'

export function MetricsCard({ project, story, packName }: {
  project: string
  story: string
  /** tên gói thể loại của truyện (“Đo bằng máy theo gói …”); rỗng khi truyện chưa chọn gói */
  packName: string
}) {
  // Khoá dưới `chapters` để mọi lần chương đổi (nộp chương) cũng làm số đo tải lại.
  const resource = useResource([...keys.story(project, story, 'chapters'), 'metrics'],
    ({ signal }) => getStoryMetrics(project, story, 'last3', signal), { enabled: !!project && !!story })
  const [open, setOpen] = useState(false)
  const chapters = resource.data?.chapters ?? []
  const rows = metricsCardRows(chapters)
  const latest = chapters.at(-1)
  if (!latest || rows.length === 0) return null
  const label = open ? t.metrics.hide(latest.n) : t.metrics.show(latest.n)

  return <Card radius={16} padding={16} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
    <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13.5, fontWeight: 600 }}>
      <Icon name="shield-tick" size={17} style={{ color: 'var(--color-ok)' }} />{t.metrics.cardTitle}
    </span>
    <span style={{ fontSize: 12, color: 'var(--kit-fg-icon)', marginTop: -4 }}>{t.metrics.cardSub(packName)}</span>
    <div>
      {rows.map((row) => <div key={row.chapter} className="rl-vio" style={{ flexDirection: 'column', gap: 5 }}>
        <span className="num" style={{ fontSize: 11.5, fontWeight: 500, color: 'var(--color-fg-4)', whiteSpace: 'nowrap' }}>{t.metrics.chapter(row.chapter)}</span>
        {row.items.map((item, index) => <span key={`${row.chapter}-${index}`} style={{ minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2, lineHeight: 1.45 }}>
          <span style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
            <span className="rl-pbadge" style={item.level === 'ok'
              ? { background: 'var(--tint-ok)', color: 'var(--color-ok)' }
              : item.level === 'error'
                ? { background: 'var(--kit-errf-12)', color: 'var(--kit-err-soft)' }
                : { background: 'var(--kit-warn-12)', color: 'var(--kit-warn-soft)' }}>
              {item.level === 'ok' ? t.metrics.pass : item.level === 'error' ? t.review.error : t.review.warning}
            </span>
            <span style={{ fontSize: 13, color: 'var(--color-fg)' }}>{item.text}</span>
          </span>
          {item.sub !== '' && <span style={{ fontSize: 12, color: 'var(--kit-fg-icon)' }}>{item.sub}</span>}
        </span>)}
      </div>)}
    </div>
    <button type="button" className="rl-btn-ghost" aria-expanded={open} onClick={() => setOpen((value) => !value)}
      style={{
        alignSelf: 'flex-start', height: 28, padding: '0 8px', marginLeft: -8, border: 'none', borderRadius: 7,
        background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 6,
      }}>
      <Icon name="chart-2" size={14} style={{ color: 'var(--color-fg-3)' }} />{label}
    </button>
    {open && <div className="rl-drawer rl-mtab" aria-label={label}>
      {latest.metrics.map((metric) => <div key={metric.key}>
        <Icon name={metric.ok ? 'tick-circle' : 'warning-2'} size={13}
          style={{ color: metric.ok ? 'var(--kit-fg-arrow)' : 'var(--color-warn)' }} />
        <span style={{ flex: '1 1 auto', minWidth: 0, color: 'var(--color-fg-2)' }}>{metric.name}</span>
        <span className="num" style={{ color: metric.ok ? 'var(--color-fg-2)' : 'var(--kit-warn-soft)', whiteSpace: 'nowrap' }}>{metricValue(metric)}</span>
        <span className="num" title={metricStandard(metric.limit)} style={{ flex: '0 0 64px', textAlign: 'right', color: 'var(--kit-fg-arrow)', whiteSpace: 'nowrap' }}>{metric.limit}</span>
      </div>)}
      {/* D6: dòng thêm “Từ mệt nhiều nhất · “x”” — từ có mật độ cao nhất chương. */}
      {latest.tiredWord && <div>
        <Icon name={latest.tiredWord.ok ? 'tick-circle' : 'warning-2'} size={13}
          style={{ color: latest.tiredWord.ok ? 'var(--kit-fg-arrow)' : 'var(--color-warn)' }} />
        <span style={{ flex: '1 1 auto', minWidth: 0, color: 'var(--color-fg-2)' }}>{tiredWordName(latest.tiredWord.word)}</span>
        <span className="num" style={{ color: latest.tiredWord.ok ? 'var(--color-fg-2)' : 'var(--kit-warn-soft)', whiteSpace: 'nowrap' }}>{tiredWordValue(latest.tiredWord)}</span>
        <span className="num" title={metricStandard(tiredWordLimit(latest.tiredWord))}
          style={{ flex: '0 0 64px', textAlign: 'right', color: 'var(--kit-fg-arrow)', whiteSpace: 'nowrap' }}>{tiredWordLimit(latest.tiredWord)}</span>
      </div>}
    </div>}
    <a href={`#${paths.reader(project, story, latest.n)}`} style={{ fontSize: 12.5, color: 'var(--color-accent)' }}>{t.metrics.openChapter(latest.n)}</a>
  </Card>
}
