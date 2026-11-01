import * as stylex from '@stylexjs/stylex'
import { text } from '../../styles/shared.stylex'
import { exploreStyles } from './explore.stylex'
// Hàng lọc của lưới Khám phá: radiogroup Sắp xếp (Nổi trội · Mới nhất · Lượt
// xem) rồi ba chip — "Dài ≥ 8 phút" (toggle long=1), "Đăng trong N ngày"
// (listbox 0/7/30/90 → within), "Trạng thái" khoá ở "Tất cả" (lọc theo kênh của
// bạn cần kênh — gói sau). Chip dùng <select> nguyên bản để có listbox + phím;
// chip khoá là nút aria-disabled vẫn nhận focus, lý do ở tooltip +
// aria-describedby. Dòng đếm: "24 / 1.284 video", khi tìm "2 video có “floor”
// · trong 1.284 đã quét" (1.284 = số video đã quét của phạm vi, không lọc).
import { useId } from 'react'
import { Icon, Tooltip } from '@noverse/ui'
import { Segmented } from '../../components/Segmented'
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

export function FilterBar({ sort, long, within, onSort, onLong, onWithin, total, scanned, q, hasChannel, countText, scopeName }: {
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
  /** đã có kênh của bạn → lý do khoá đổi sang "cần hàng chờ" */
  hasChannel: boolean
  /** dòng đếm thay thế cho trạng thái danh sách (lọc rỗng/quét/lỗi/mất kết nối) */
  countText?: string
  /** tên phạm vi — bản một cột nối vào dòng đếm (“24 / 1.284 video · Nocturne Stories”) */
  scopeName?: string
}) {
  const tt = t.screens.explore
  const lockReason = hasChannel ? tt.queueReason : tt.lockedReason
  const lockId = useId()
  const touch = useMediaQuery(TOUCH_COLUMN)
  const narrow = useMediaQuery('(max-width: 767px)')
  const withinLabel = WITHIN.find((w) => w.value === within)?.label ?? tt.withinDays.d30
  const baseCount = countText ?? (total === undefined ? ''
    : !q ? tt.videoCount(num.format(total), num.format(scanned))
      : total > 0 ? tt.searchCount(num.format(total), q, num.format(scanned)) : tt.searchCountNone(q))
  const count = narrow && scopeName && baseCount ? `${baseCount} · ${scopeName}` : baseCount
  return (
    <div className={`vx-filters ${stylex.props(exploreStyles.filters).className}`}>
      <Segmented xstyle={exploreStyles.segmented} buttonStyle={exploreStyles.segmentedButton}
        label={tt.sortLabel}
        size={touch ? TOUCH_TARGET : 'sm'}
       
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
      <button type="button" className={`vx-chip ${stylex.props(exploreStyles.chip, long && exploreStyles.chipCustom).className}`} aria-pressed={long} onClick={() => onLong(!long)}>
        <Icon name="clock" size={14} />
        {tt.filterLong} <b {...stylex.props(exploreStyles.chipValue)}>{tt.filterLongValue}</b>
        <Icon name="arrow-d" size={12} aria-hidden />
      </button>
      <label className={`vx-chip vx-chip--select ${stylex.props(exploreStyles.chip, within !== 30 && exploreStyles.chipCustom).className}`}>
        <Icon name="calendar" size={14} />
        {tt.filterWithin} <b {...stylex.props(exploreStyles.chipValue)}>{withinLabel}</b>
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
      <Tooltip content={lockReason} asChild>
        <button type="button" className={`vx-chip vx-chip--locked ${stylex.props(exploreStyles.chip, exploreStyles.chipLocked).className}`} aria-disabled="true" aria-describedby={lockId}>
          <Icon name="filter" size={14} />
          {tt.filterStatus} <b {...stylex.props(exploreStyles.chipValue)}>{tt.statusAll}</b>
          <Icon name="arrow-d" size={12} aria-hidden />
        </button>
      </Tooltip>
      <span id={lockId} {...stylex.props(text.srOnly)}>{lockReason}</span>
      <span className={`vx-grow ${stylex.props(exploreStyles.grow).className}`} />
      <span className={`vx-count num ${stylex.props(exploreStyles.count).className}`}>{count}</span>
    </div>
  )
}
