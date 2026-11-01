// StatusLabel — chấm 7px (spinner 9px khi Đang làm) + tên trạng thái đúng màu
// của nó, kèm dòng phụ 12px xám (04-components · kit.css .vv-status).
// Tên và màu theo ia-flows §3.1 — không đặt tên mới.
// Kiểu: stylex.create bên dưới (trước ở styles/components.css .vv-status*).
import type { CSSProperties } from 'react'
import * as stylex from '@stylexjs/stylex'
import { t } from '../i18n/vi'

export type VideoStatus = 'queued' | 'working' | 'review' | 'issue' | 'paused' | 'ready' | 'posted'

const STATE_COLOR: Record<VideoStatus, string> = {
  queued: 'var(--vv-st-neutral)',
  working: 'var(--color-info)',
  review: 'var(--color-warn)',
  issue: 'var(--color-err)',
  paused: 'var(--vv-st-neutral)',
  ready: 'var(--color-ok)',
  posted: 'var(--vv-fg-note)', // artboard: Đã đăng dùng màu ghi chú, khác màu Chờ lượt
}

const spinFrames = stylex.keyframes({ to: { transform: 'rotate(360deg)' } })

const sx = stylex.create({
  root: { display: 'inline-flex', flexDirection: 'column', gap: 'var(--vv-s-2)', minWidth: 0 },
  row: { display: 'inline-flex', alignItems: 'center', gap: 'var(--vv-s-6)', fontSize: 12, whiteSpace: 'nowrap' },
  dot: { width: 7, height: 7, borderRadius: '50%', backgroundColor: 'currentColor', flex: '0 0 auto' },
  hint: { fontSize: 12, color: 'var(--color-fg-4)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  spin: {
    width: 9,
    height: 9,
    borderRadius: '50%',
    flex: '0 0 auto',
    borderWidth: 1.5,
    borderStyle: 'solid',
    borderColor: 'var(--vv-spinner-line)',
    borderTopColor: 'currentColor',
    animationName: spinFrames,
    animationDuration: '.9s',
    animationTimingFunction: 'linear',
    animationIterationCount: 'infinite',
  },
})

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
    <span {...stylex.props(sx.root)} style={style}>
      <span {...stylex.props(sx.row)} style={{ color }}>
        <span aria-hidden="true" {...stylex.props(status === 'working' ? sx.spin : sx.dot)} />
        {statusName(status)}
      </span>
      {hint && <span {...stylex.props(sx.hint)}>{hint}</span>}
    </span>
  )
}
