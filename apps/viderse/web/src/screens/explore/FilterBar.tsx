import * as stylex from '@stylexjs/stylex'
import { text } from '../../styles/shared.stylex'
import { exploreStyles } from './explore.stylex'
// Hàng lọc của lưới Khám phá: radiogroup Sắp xếp (Nổi trội · Mới nhất · Lượt
// xem) rồi ba chip — "Dài ≥ 8 phút" (toggle long=1), "Đăng trong N ngày"
// (listbox 0/7/30/90 → within), "Trạng thái" khoá ở "Tất cả" (lọc theo kênh của
// bạn cần kênh — gói sau). ChipSelect của kit sở hữu listbox và bàn phím;
// chip khoá là nút aria-disabled vẫn nhận focus, lý do ở tooltip +
// aria-describedby. Dòng đếm: "24 / 1.284 video", khi tìm "2 video có “floor”
// · trong 1.284 đã quét" (1.284 = số video đã quét của phạm vi, không lọc).
import { useId } from 'react'
import { Button, ChipSelect, Icon, Tooltip } from '@noverse/ui'
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
      <Button variant="ghost" size={touch ? 'lg' : 'sm'} aria-pressed={long} active={long} onClick={() => onLong(!long)} icon={<Icon name="clock" size={14} />}>
        {tt.filterLong} {tt.filterLongValue}
      </Button>
      <ChipSelect label={tt.withinLabel} prefix={tt.filterWithin} size={touch ? 'm' : 'sm'} custom={within !== 30} value={String(within)}
        options={WITHIN.map((w) => ({ value: String(w.value), label: w.label }))}
        onChange={(value) => onWithin(Number(value) as 0 | 7 | 30 | 90)} />
      {/* Khoá: chỉ "Tất cả" có nghĩa khi chưa có kênh của bạn. Nút vẫn nhận focus,
          bấm không mở gì; lý do đọc được bằng trình đọc màn hình. */}
      <Tooltip content={lockReason} asChild>
        <Button variant="ghost" size={touch ? 'lg' : 'sm'} aria-disabled="true" aria-describedby={lockId} icon={<Icon name="filter" size={14} />}>
          {tt.filterStatus} {tt.statusAll}
        </Button>
      </Tooltip>
      <span id={lockId} {...stylex.props(text.srOnly)}>{lockReason}</span>
      <span className={`vx-grow ${stylex.props(exploreStyles.grow).className}`} />
      <span className={`vx-count num ${stylex.props(exploreStyles.count).className}`}>{count}</span>
    </div>
  )
}
