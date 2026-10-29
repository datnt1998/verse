// StatusLabel — chấm 7px (spinner 9px khi Đang làm) + tên trạng thái đúng màu
// của nó, kèm dòng phụ 12px xám (04-components · kit.css .vv-status).
// Tên và màu theo ia-flows §3.1 — không đặt tên mới.
// Kiểu: styles/components.css .vv-status*.
import type { CSSProperties } from 'react'
import { t } from '../i18n/vi'

export type VideoStatus = 'queued' | 'working' | 'review' | 'issue' | 'paused' | 'ready' | 'posted'

const STATE_COLOR: Record<VideoStatus, string> = {
  queued: 'var(--vv-st-neutral)',
  working: 'var(--color-info)',
  review: 'var(--color-warn)',
  issue: 'var(--color-err)',
  paused: 'var(--vv-st-neutral)',
  ready: 'var(--color-ok)',
  posted: 'var(--vv-st-neutral)',
}

export const statusName = (status: VideoStatus) => t.states[status]
export const statusColor = (status: VideoStatus) => STATE_COLOR[status]

export function StatusLabel({ status, hint, style }: {
  status: VideoStatus
  /** dòng phụ 12px xám (vd “Hoạ sĩ đang vẽ cảnh 5/6”) */
  hint?: string
  style?: CSSProperties
}) {
  const color = STATE_COLOR[status]
  return (
    <span className="vv-status" style={style}>
      <span className="vv-status__row" style={{ color }}>
        <span aria-hidden="true" className={status === 'working' ? 'vv-status__spin' : 'vv-status__dot'} />
        {statusName(status)}
      </span>
      {hint && <span className="vv-status__hint">{hint}</span>}
    </span>
  )
}
