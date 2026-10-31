// JobsPill — viên đếm việc nền ở đầu trang (04-components › JobsPill):
// “N đang chạy · M chờ lượt” chấm đỏ khi N > 0 · “Đã tạm dừng” chấm xám ·
// “Máy rảnh” khi rảnh · “Mất kết nối” chấm cam khi offline. Bấm mở Hàng chờ.
// Kiểu: styles/components.css .vv-jobs (--idle/--run/--paused/--offline).
import type { CSSProperties } from 'react'
import { navigate, paths } from '../router'
import { t } from '../i18n/vi'
import type { Status } from '../api/status'

export function JobsPill({ status, online, unread = false, compact = false, style }: {
  status: Status | undefined
  online: boolean
  /** có thông báo cần bạn chưa đọc → chấm vàng trên viên */
  unread?: boolean
  compact?: boolean
  style?: CSSProperties
}) {
  const running = status?.running ?? 0
  const queued = status?.queued ?? 0
  const paused = status?.paused ?? false

  const text = !online
    ? t.shell.jobsOffline
    : paused
      ? t.shell.jobsPaused(queued)
      : running > 0
        ? t.shell.jobsRunning(running, queued)
        : t.shell.jobsIdle(queued)

  const kind = !online ? 'offline' : paused ? 'paused' : running > 0 ? 'run' : 'idle'
  const spinning = kind === 'run'

  return (
    <a
      href={`#${paths.queue()}`}
      aria-label={`${t.shell.jobsAriaLabel}: ${text}`}
      title={t.shell.jobsAriaLabel}
      className={`vv-jobs vv-jobs--${kind}`}
      style={style}
      onClick={(e) => { e.preventDefault(); navigate(paths.queue()) }}
    >
      <span aria-hidden="true" className={spinning ? 'vv-jobs__dot vv-rec' : 'vv-jobs__dot'} />
      <span className="num">{compact && online && !paused ? t.shell.jobsCompact(running, queued) : text}</span>
      {unread && <span role="img" aria-label={t.shell.jobsUnread} className="vv-jobs__unread" />}
    </a>
  )
}
