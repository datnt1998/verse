// Hàng lọc của lưới Khám phá: radiogroup Sắp xếp (Nổi trội · Mới nhất · Lượt
// xem) rồi ba chip — "Dài ≥ 8 phút" (toggle long=1), "Đăng trong N ngày"
// (listbox 0/7/30/90 → within), "Trạng thái" khoá ở "Tất cả" (lọc theo kênh của
// bạn cần kênh — gói sau). Chip dùng <select> nguyên bản để có listbox + phím;
// chip khoá là nút aria-disabled vẫn nhận focus, lý do ở tooltip +
// aria-describedby. Dòng đếm: "24 / 1.284 video", khi tìm "2 video có “floor”
// · trong 1.284 đã quét" (1.284 = số video đã quét của phạm vi, không lọc).
import { useId } from 'react'
import { Icon, Segmented, Tooltip } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'
import type { VideoSort } from '../../api/rivals'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { TOUCH_COLUMN, TOUCH_TARGET } from './media'

export const WITHIN: { id: string; value: 0 | 7 | 30 | 90; label: string }[] = [
  { id: 'any', value: 0, label: t.screens.explore.withinDays.any },
  { id: 'd7', value: 7, label: t.screens.explore.withinDays.d7 },
  { id: 'd30', value: 30, label: t.screens.explore.withinDays.d30 },
  { id: 'd90', value: 90, label: t.screens.explore.withinDays.d90 },
]

const num = new Intl.NumberFormat('vi-VN')

export function FilterBar({ sort, long, within, onSort, onLong, onWithin, total, scanned, q }: {
  sort: VideoSort
  long: boolean
  within: 0 | 7 | 30 | 90
  onSort: (sort: VideoSort) => void
  onLong: (on: boolean) => void
  onWithin: (days: 0 | 7 | 30 | 90) => void
  /** `total` của API (sau lọc và tìm); undefined khi trang đầu chưa về */
  total: number | undefined
  /** số video đã quét của phạm vi đang xem (một đối thủ hoặc tất cả) */
  scanned: number
  /** chuỗi tìm của dữ liệu đang hiện (đã cắt khoảng trắng); rỗng = không tìm */
  q: string
}) {
  const tt = t.screens.explore
  const lockId = useId()
  const touch = useMediaQuery(TOUCH_COLUMN)
  const withinLabel = WITHIN.find((w) => w.value === within)?.label ?? tt.withinDays.d30
  const count = total === undefined ? ''
    : !q ? tt.videoCount(num.format(total), num.format(scanned))
      : total > 0 ? tt.searchCount(num.format(total), q, num.format(scanned)) : tt.searchCountNone(q)
  return (
    <div className="vx-filters">
      <Segmented
        label={tt.sortLabel}
        size={touch ? TOUCH_TARGET : 'sm'}
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
      {/* Khoá: chỉ "Tất cả" có nghĩa khi chưa có kênh của bạn. Nút vẫn nhận focus,
          bấm không mở gì; lý do đọc được bằng trình đọc màn hình. */}
      <Tooltip content={tt.lockedReason} asChild>
        <button type="button" className="vx-chip vx-chip--locked" aria-disabled="true" aria-describedby={lockId}>
          <Icon name="filter" size={14} />
          {tt.filterStatus} <b>{tt.statusAll}</b>
          <Icon name="arrow-d" size={12} aria-hidden />
        </button>
      </Tooltip>
      <span id={lockId} className="sr-only">{tt.lockedReason}</span>
      <span className="vx-grow" />
      <span className="vx-count num">{count}</span>
    </div>
  )
}
