// Ba nhóm chọn của Quality.dc.html: Cách xem (Việc cần làm/Ma trận) chuyển màn trong cùng trang nên là
// `tablist`; Mức và Phạm vi là bộ lọc dữ liệu nên là `radiogroup` (22-brief-2510.md › Câu 14).
// Dùng chung `ui/Tabs.tsx › Segmented`, giữ đúng `.seg` của artboard: nền card, viền --line-2, không khe
// giữa các mục, mục đang chọn `--quality-seg-on`, nút rộng theo chữ.
import { t } from '../../i18n/vi'
import { Segmented } from '@noverse/ui'
import type { QualityLevelFilter, QualityScope, QualityView } from './model'

export const VIEWS: QualityView[] = ['todo', 'matrix']
export const LEVELS: QualityLevelFilter[] = ['all', 'error', 'warning']
export const SCOPES: QualityScope[] = ['all', 'open']

const SEG_LOOK = { background: 'var(--color-card)', borderColor: 'var(--line-2)', gap: 0 }
const SEG_ITEM = { flex: '0 0 auto', padding: '0 11px' }
const THUMB = { background: 'var(--quality-seg-on)', boxShadow: 'none' }

export function QualityControls({ view, level, scope, todo, onView, onLevel, onScope }: {
  view: QualityView
  level: QualityLevelFilter
  scope: QualityScope
  /** Số việc ở tab “Việc cần làm” (Đang chặn chương + Cần bạn xem) */
  todo: number
  onView: (view: QualityView) => void
  onLevel: (level: QualityLevelFilter) => void
  onScope: (scope: QualityScope) => void
}) {
  return (
    <>
      <Segmented label={t.quality.viewsLabel} value={view} size="sm" radius={7}
        tabs={{ id: 'quality-view', panel: 'quality-view-panel' }} style={SEG_LOOK} thumbStyle={THUMB}
        options={VIEWS.map((option) => ({
          id: option, style: { ...SEG_ITEM, fontWeight: view === option ? 500 : 400 },
          label: <>{t.quality.views[option]}
            {option === 'todo' && <span className="num" style={{ marginLeft: 6, fontSize: 11.5, color: 'var(--color-fg-4)' }}>{todo}</span>}
          </>,
        }))}
        onChange={(next) => onView(next as QualityView)} />
      <Segmented label={t.quality.levelLabel} value={level} size="sm" radius={7}
        style={SEG_LOOK} thumbStyle={THUMB}
        options={LEVELS.map((option) => ({ id: option, label: t.quality.levels[option], style: { ...SEG_ITEM, fontWeight: level === option ? 500 : 400 } }))}
        onChange={(next) => onLevel(next as QualityLevelFilter)} />
      {view === 'matrix' && (
        <Segmented label={t.quality.scopeLabel} value={scope} size="sm" radius={7}
          style={SEG_LOOK} thumbStyle={THUMB}
          options={SCOPES.map((option) => ({ id: option, label: t.quality.scopes[option], style: { ...SEG_ITEM, fontWeight: scope === option ? 500 : 400 } }))}
          onChange={(next) => onScope(next as QualityScope)} />
      )}
    </>
  )
}
