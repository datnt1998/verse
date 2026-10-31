// JobsPill — viên đếm việc nền ở đầu trang (04-components › JobsPill):
// “N đang chạy · M chờ lượt” chấm đỏ khi N > 0 · “Đã tạm dừng” chấm xám ·
// “Máy rảnh” khi rảnh · “Mất kết nối” chấm cam khi offline. Bấm mở Hàng chờ.
import type { CSSProperties } from 'react'
import { navigate, paths } from '../router'
import { t } from '../i18n/vi'
import type { Status } from '../api/status'
import * as stylex from '@stylexjs/stylex'
import { color, line, radius } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../styles/viderse.stylex'

export function JobsPill({ status, online, unread = false, compact = false, mobileBar = false, style }: {
  status: Status | undefined
  online: boolean
  /** có thông báo cần bạn chưa đọc → chấm vàng trên viên */
  unread?: boolean
  compact?: boolean
  mobileBar?: boolean
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
      className={`vv-jobs vv-jobs--${kind} ${stylex.props(styles.root, mobileBar && styles.mobile).className}`}
      style={style}
      onClick={(e) => { e.preventDefault(); navigate(paths.queue()) }}
    >
      <span aria-hidden="true" className={`${spinning ? 'vv-jobs__dot vv-rec' : 'vv-jobs__dot'} ${stylex.props(styles.dot, kind === 'run' ? styles.runningDot : kind === 'offline' ? styles.offlineDot : styles.idleDot, mobileBar && styles.mobileDot).className}`} />
      <span className={`num ${stylex.props(mobileBar && styles.mobileText).className ?? ''}`}>{compact && online && !paused ? t.shell.jobsCompact(running, queued) : text}</span>
      {unread && <span role="img" aria-label={t.shell.jobsUnread} className={`vv-jobs__unread ${stylex.props(styles.unread).className}`} />}
      {mobileBar && <span aria-hidden="true" {...stylex.props(styles.touchHalo)} />}
    </a>
  )
}

const MOBILE = '@media (max-width: 767px)'
const TOUCH = '@media (max-width: 767px) and (pointer: coarse)'
const styles = stylex.create({
  root: { position: 'relative', height: 30, padding: `0 ${s['--vv-s-10']}`, display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], borderRadius: radius['--radius-control'], borderWidth: 0, borderStyle: "none", borderColor: { default: null, ':hover': line['--line-1'] }, backgroundColor: { default: "transparent", ':hover': color['--color-surface-1'] }, color: color['--color-fg-3'], fontSize: 12.5, whiteSpace: 'nowrap', textDecoration: 'none' },
  dot: { width: 8, height: 8, borderRadius: '50%' },
  runningDot: { backgroundColor: "var(--vv-red)", boxShadow: '0 0 0 3px var(--vv-red-18)' },
  idleDot: { backgroundColor: "var(--vv-st-neutral)", boxShadow: '0 0 0 3px var(--vv-st-neutral-15)' },
  offlineDot: { backgroundColor: color['--color-err'], boxShadow: '0 0 0 3px var(--vv-err-18)' },
  unread: { position: 'absolute', top: s['--vv-s-3'], right: s['--vv-s-3'], width: 7, height: 7, borderRadius: '50%', backgroundColor: color['--color-warn'], boxShadow: `0 0 0 2px ${color['--color-bg']}` },
  mobile: { height: 36, padding: `0 ${s['--vv-s-11']}`, gap: s['--vv-s-7'], borderRadius: r['--vv-r-pill'], borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-4'], backgroundColor: color['--color-surface-1'], color: color['--color-fg-2'], flex: { default: null, [MOBILE]: '0 1 auto' }, minWidth: { default: null, [MOBILE]: 0 }, overflow: { default: null, [MOBILE]: 'hidden', [TOUCH]: 'visible' } },
  mobileDot: { width: 7, height: 7 },
  mobileText: { minWidth: { default: null, [MOBILE]: 0 }, overflow: { default: null, [MOBILE]: 'hidden' }, textOverflow: { default: null, [MOBILE]: 'ellipsis' }, whiteSpace: { default: null, [MOBILE]: 'nowrap' } },
  touchHalo: { display: { default: 'none', [TOUCH]: 'block' }, position: 'absolute', inset: `calc((${s['--vv-s-44']} - 100%) / -2) 0` },
})
