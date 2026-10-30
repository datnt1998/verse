// Hàng lọc của lưới Khám phá: radiogroup Sắp xếp (Nổi trội · Mới nhất · Lượt
// xem) rồi ba chip — "Dài ≥ 8 phút" (toggle long=1), "Đăng trong N ngày"
// (listbox 0/7/30/90 → within), "Trạng thái" khoá ở "Tất cả" (lọc theo kênh của
// bạn cần kênh — gói sau). Chip dùng <select> nguyên bản để có listbox + phím;
// chip khoá không có <select> nên không vào thứ tự Tab, lý do khoá nằm ở
// tooltip + aria-describedby của chính nó.
import { Icon, Segmented } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'
import type { VideoSort } from '../../api/rivals'

const WITHIN: { id: string; value: 0 | 7 | 30 | 90; label: string }[] = [
  { id: 'any', value: 0, label: t.screens.explore.withinDays.any },
  { id: 'd7', value: 7, label: t.screens.explore.withinDays.d7 },
  { id: 'd30', value: 30, label: t.screens.explore.withinDays.d30 },
  { id: 'd90', value: 90, label: t.screens.explore.withinDays.d90 },
]

export function FilterBar({ sort, long, within, onSort, onLong, onWithin, shown, total }: {
  sort: VideoSort
  long: boolean
  within: 0 | 7 | 30 | 90
  onSort: (sort: VideoSort) => void
  onLong: (on: boolean) => void
  onWithin: (days: 0 | 7 | 30 | 90) => void
  /** "24 / 1.284 video" — tổng của API sau lọc */
  shown: number
  total: number | undefined
}) {
  const tt = t.screens.explore
  const num = new Intl.NumberFormat('vi-VN')
  const withinLabel = WITHIN.find((w) => w.value === within)?.label ?? tt.withinDays.d30
  return (
    <div className="vx-filters">
      <Segmented
        label={tt.sortLabel}
        size="sm"
        fit="content"
        style={{ flex: '0 0 auto', gap: 'var(--vv-s-2)', background: 'var(--color-surface-1)', borderColor: 'var(--line-1)' }}
        thumbStyle={{ background: 'var(--color-surface-6)' }}
        value={sort}
        onChange={(id) => onSort(id as VideoSort)}
        options={[
          { id: 'outlier', label: tt.sorts.outlier },
          { id: 'newest', label: tt.sorts.newest },
          { id: 'views', label: tt.sorts.views },
        ]}
      />
      <button type="button" className="vx-chip" aria-pressed={long} onClick={() => onLong(!long)}>
        <Icon name="clock" size={14} />
        {tt.filterLong} <b>{tt.filterLongValue}</b>
        <Icon name="arrow-d" size={12} aria-hidden />
      </button>
      <label className="vx-chip vx-chip--select">
        <Icon name="calendar" size={14} />
        {tt.filterWithin} <b>{withinLabel}</b>
        <select
          value={within}
          aria-label={tt.withinLabel}
          onChange={(e) => onWithin(Number(e.target.value) as 0 | 7 | 30 | 90)}
          style={{ position: 'absolute', inset: 0, opacity: 0, cursor: 'pointer', font: 'inherit' }}
        >
          {WITHIN.map((w) => <option key={w.id} value={w.value}>{w.label}</option>)}
        </select>
        <Icon name="arrow-d" size={12} aria-hidden />
      </label>
      {/* Khoá: chỉ "Tất cả" có nghĩa khi chưa có kênh của bạn, nên chip không mở
          listbox và không phải điểm dừng Tab; lý do đọc được bằng trình đọc màn hình. */}
      <span className="vx-chip vx-chip--locked" aria-disabled="true" aria-describedby="vx-status-locked"
        title={tt.lockedReason}>
        <Icon name="filter" size={14} />
        {tt.filterStatus} <b>{tt.statusAll}</b>
        <Icon name="arrow-d" size={12} aria-hidden />
        <span id="vx-status-locked" style={{ position: 'absolute', width: 1, height: 1, overflow: 'hidden', clip: 'rect(0 0 0 0)' }}>{tt.lockedReason}</span>
      </span>
      <span className="vx-grow" />
      <span className="vx-count num">
        {total === undefined ? `${num.format(shown)}` : tt.videoCount(num.format(shown), num.format(total))}
      </span>
    </div>
  )
}
