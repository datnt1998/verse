// Thanh chọn nhiều của Khám phá — sticky đáy cột giữa (artboard .card region
// "Thao tác với video đã chọn"): "Đã chọn N" · Top 10/20 · Bỏ chọn ·
// Sao chép link · nút chính "Làm N video" (khoá: chưa có kênh của bạn) · nút ✕.
import { Icon, toast } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'
import { ActionButton } from '../../components'
import type { Video } from '../../api/rivals'
import { copyText } from './VideoGrid'

export function BulkBar({ count, selected, onTop, onClear, onExit }: {
  count: number
  /** video đang được tích — "Sao chép link" gom URL của chúng */
  selected: readonly Video[]
  onTop: (n: number) => void
  onClear: () => void
  onExit: () => void
}) {
  const tt = t.screens.explore
  const copyLinks = async () => {
    const urls = selected.map((v) => v.url)
    const ok = await copyText(urls.join('\n'))
    if (ok) toast.ok(tt.copied)
    else toast.err(tt.copyFailed)
  }
  return (
    <div className="vx-bulk">
      <div className="vx-bulk__bar" role="region" aria-label={tt.bulkRegion}>
        <span className="vx-bulk__count num">{tt.selected(count)}</span>
        <span className="vx-bulk__quick">
          <button type="button" onClick={() => onTop(10)}>{tt.top10}</button>
          <button type="button" onClick={() => onTop(20)}>{tt.top20}</button>
          <button type="button" onClick={onClear}>{tt.deselect}</button>
        </span>
        <ActionButton size="md" icon={<Icon name="copy" size={15} />} onClick={copyLinks}>{tt.copyLink}</ActionButton>
        <ActionButton
          variant="primary"
          size="md"
          disabled
          disabledReason={t.screens.explore.lockedReason}
          icon={<Icon name="clock" size={15} />}
        >
          {tt.bulkMake(count)}
          <Icon name="arrow-d" size={13} aria-hidden />
        </ActionButton>
        <button type="button" className="vx-iconbtn" aria-label={tt.exitSelect} onClick={onExit}
          style={{ width: 'var(--vv-s-34)', height: 'var(--vv-s-34)' }}>
          <Icon name="close" size={16} />
        </button>
      </div>
    </div>
  )
}
