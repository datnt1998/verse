import * as stylex from '@stylexjs/stylex'
import { exploreStyles } from './explore.stylex'
// Hàng lọc của lưới Khám phá: radiogroup Sắp xếp (Nổi trội · Mới nhất · Lượt
// xem) rồi ba chip — "Dài ≥ 8 phút" (toggle long=1), "Đăng trong N ngày"
// (listbox 0/7/30/90 → within), "Trạng thái" (Tất cả · Chưa làm cho kênh nào ·
// Đã làm, lọc theo lượt làm thật ở máy chủ). ChipSelect của kit sở hữu listbox
// và bàn phím. Dòng đếm: "24 / 1.284 video", khi tìm "2 video có “floor”
// · trong 1.284 đã quét" (1.284 = số video đã quét của phạm vi, không lọc).
import { Button, ChipSelect, Icon } from '@noverse/ui'
import { Segmented } from '../../components/Segmented'
import { t } from '../../i18n/explore'
import type { VideoSort } from '../../api/rivals'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { TOUCH_COLUMN } from './media'

export const WITHIN: { id: string; value: 0 | 7 | 30 | 90; label: string }[] = [
  { id: 'any', value: 0, label: t.screens.explore.withinDays.any },
  { id: 'd7', value: 7, label: t.screens.explore.withinDays.d7 },
  { id: 'd30', value: 30, label: t.screens.explore.withinDays.d30 },
  { id: 'd90', value: 90, label: t.screens.explore.withinDays.d90 },
]

const num = new Intl.NumberFormat('vi-VN')

export type StatusFilter = 'all' | 'notMade' | 'made'

export function FilterBar({ sort, long, within, status, onSort, onLong, onWithin, onStatus, total, scanned, q, countText, scopeName }: {
  sort: VideoSort
  long: boolean
  within: 0 | 7 | 30 | 90
  status: StatusFilter
  onSort: (sort: VideoSort) => void
  onLong: (on: boolean) => void
  onWithin: (days: 0 | 7 | 30 | 90) => void
  onStatus: (status: StatusFilter) => void
  /** `total` của API (sau lọc và tìm); undefined khi trang đầu chưa về */
  total: number | undefined
  /** số video đã quét của phạm vi đang xem (một đối thủ hoặc tất cả) */
  scanned: number
  /** chuỗi tìm của dữ liệu đang hiện (đã cắt khoảng trắng); rỗng = không tìm */
  q: string
  /** dòng đếm thay thế cho trạng thái danh sách (lọc rỗng/quét/lỗi/mất kết nối) */
  countText?: string
  /** tên phạm vi — bản một cột nối vào dòng đếm (“24 / 1.284 video · Nocturne Stories”) */
  scopeName?: string
}) {
  const tt = t.screens.explore
  const touch = useMediaQuery(TOUCH_COLUMN)
  const narrow = useMediaQuery('(max-width: 767px)')
  const baseCount = countText ?? (total === undefined ? ''
    : !q ? tt.videoCount(num.format(total), num.format(scanned))
      : total > 0 ? tt.searchCount(num.format(total), q, num.format(scanned)) : tt.searchCountNone(q))
  const count = narrow && scopeName && baseCount ? `${baseCount} · ${scopeName}` : baseCount
  return (
    <div className={`vx-filters ${stylex.props(exploreStyles.filters).className}`}>
      <Segmented
        label={tt.sortLabel}
        size={touch ? 44 : 'sm'}
        fit="content"
        value={sort}
        onChange={(id) => onSort(id as VideoSort)}
        options={[
          { id: 'outlier', label: tt.sorts.outlier },
          { id: 'newest', label: tt.sorts.newest },
          { id: 'views', label: tt.sorts.views },
        ]}
      />
      <Button variant="ghost" size={touch ? 'lg' : 'sm'} xstyle={touch ? undefined : exploreStyles.filterChip}
        aria-pressed={long} active={long} onClick={() => onLong(!long)} icon={<Icon name="clock" size={14} />}>
        {tt.filterLong} <b {...stylex.props(exploreStyles.filterValue)}>{tt.filterLongValue}</b>
        <Icon name="arrow-d" size={12} />
      </Button>
      <ChipSelect label={tt.withinLabel} prefix={tt.filterWithin} size={touch ? 'm' : 'sm'} custom={within !== 30} value={String(within)}
        options={WITHIN.map((w) => ({ value: String(w.value), label: w.label }))}
        onChange={(value) => onWithin(Number(value) as 0 | 7 | 30 | 90)} />
      <ChipSelect label={tt.statusLabel} prefix={tt.filterStatus} size={touch ? 'm' : 'sm'} custom={status !== 'all'} value={status}
        options={[
          { value: 'all', label: tt.statusOptions.all },
          { value: 'notMade', label: tt.statusOptions.notMade },
          { value: 'made', label: tt.statusOptions.made },
        ]}
        onChange={(value) => onStatus(value as StatusFilter)} />
      <span className={`vx-grow ${stylex.props(exploreStyles.grow).className}`} />
      <span className={`vx-count num ${stylex.props(exploreStyles.count).className}`}>{count}</span>
    </div>
  )
}
