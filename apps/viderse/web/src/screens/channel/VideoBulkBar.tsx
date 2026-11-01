// Thanh thao tác khi Chọn nhiều (artboard Channel › select, 04-components BulkBar):
// “Đã chọn N”, mỗi nút kèm số video hợp lệ trong số đã chọn (khoá khi 0, lý do ở
// tooltip), ⋯ cho Huỷ/Xoá, “Thoát chọn Esc”. Thay chỗ ô nhắn trợ lý ở dock.
import * as stylex from '@stylexjs/stylex'
import { ActionMenu, Icon, type ActionMenuItem } from '@noverse/ui'
import { t } from '../../i18n/vi'
import type { BulkCounts } from './videosLogic'
import { videoStyles as sx } from './videos.stylex'

const tt = t.channels.videos.bulk

type Props = {
  count: number
  counts: BulkCounts
  busy: boolean
  onRetry: () => void
  onApprove: () => void
  onBundle: () => void
  onMarkPosted: () => void
  onCancel: () => void
  onRemove: () => void
  onExit: () => void
}

export function VideoBulkBar({ count, counts, busy, onRetry, onApprove, onBundle, onMarkPosted, onCancel, onRemove, onExit }: Props) {
  const buttons = [
    { id: 'retry', icon: 'refresh', label: tt.retry, n: counts.retry, tip: tt.retryTip, run: onRetry },
    { id: 'approve', icon: 'tick-b', label: tt.approve, n: counts.approve, tip: tt.approveTip, run: onApprove },
    { id: 'bundle', icon: 'download', label: tt.bundle, n: counts.bundle, tip: tt.bundleTip, run: onBundle },
    { id: 'posted', icon: 'send', label: tt.markPosted, n: counts.markPosted, tip: tt.markPostedTip, run: onMarkPosted },
  ] as const
  const more: ActionMenuItem[] = [
    counts.cancel > 0
      ? { id: 'cancel', label: tt.cancel, meta: counts.cancel, onSelect: onCancel }
      : { id: 'cancel', label: tt.cancel, lockedReason: tt.cancelTip },
    counts.delete > 0
      ? { id: 'delete', label: tt.remove, meta: counts.delete, danger: true, onSelect: onRemove }
      : { id: 'delete', label: tt.remove, lockedReason: tt.removeTip },
  ]
  return (
    <div role="toolbar" aria-label={tt.toolbar} {...stylex.props(sx.bulk)}>
      <span className="num" {...stylex.props(sx.bulkCount)}>{tt.chosen(count)}</span>
      {buttons.map((button) => {
        const off = button.n === 0 || busy
        return (
          <button
            key={button.id}
            type="button"
            aria-disabled={off}
            title={button.tip}
            onClick={() => { if (!off) button.run() }}
            {...stylex.props(sx.bulkBtn, off && sx.bulkBtnOff)}
          >
            <Icon name={button.icon} size={15} />
            {button.label}
            <span className="num" {...stylex.props(sx.chipCount)}>{button.n}</span>
          </button>
        )
      })}
      <ActionMenu alwaysVisible label={tt.more} items={more} width={248} triggerStyle={sx.bulkMore} trigger={<Icon name="more" size={16} />} />
      <span {...stylex.props(sx.spacer)} />
      <button type="button" onClick={onExit} {...stylex.props(sx.bulkExit)}>
        {tt.exit}
        <kbd className="mono" {...stylex.props(sx.kbd)}>Esc</kbd>
      </button>
    </div>
  )
}
