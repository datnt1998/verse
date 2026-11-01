// Bốn trạng thái việc nền dùng dữ liệu thật; điện thoại mở sheet Hàng chờ.
import type { CSSProperties } from 'react'
import { navigate, paths } from '../router'
import { t } from '../i18n/vi'
import type { Status } from '../api/status'
import { Icon } from '@noverse/ui'
import { getQueue } from '../api/clone'
import { useResource } from '../data/useResource'
import { keys } from '../data/keys'
import * as stylex from '@stylexjs/stylex'
import { color, line, radius } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r, visualRoles } from '../styles/viderse.stylex'

export function JobsPill({ status, online, unread = false, compact = false, mobileBar = false, style, onOpen, expanded }: {
  status: Status | undefined
  online: boolean
  /** có thông báo cần bạn chưa đọc → chấm vàng trên viên */
  unread?: boolean
  compact?: boolean
  mobileBar?: boolean
  style?: CSSProperties
  onOpen?: () => void
  expanded?: boolean
}) {
  const queue = useResource(keys.queue(), ({ signal }) => getQueue(signal))
  const running = status?.running ?? 0
  const queued = status?.queued ?? 0
  const paused = status?.paused ?? false

  const pausedAt = queue.data?.pausedAt
  const time = pausedAt ? new Date(pausedAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit', hour12: false }) : undefined
  const kind = !online ? 'offline' : paused ? 'paused' : running > 0 || queued > 0 ? 'run' : 'idle'
  const text = kind === 'offline' ? t.shell.jobsOffline
    : kind === 'paused' ? t.shell.jobsPaused(queued, time)
    : kind === 'idle' ? t.shell.jobsIdle(0) : t.shell.jobsRunning(running, queued)
  const visible = !compact ? text : kind === 'offline' ? t.shell.jobsOfflineCompact
    : kind === 'paused' ? String(queued) : kind === 'idle' ? t.shell.jobsIdleCompact : t.shell.jobsCompact(running, queued)
  const tooltip = kind === 'offline' ? t.shell.jobsOfflineTip
    : kind === 'paused' ? t.shell.jobsPausedTip(time)
    : kind === 'idle' ? t.shell.jobsIdleTip
    : t.shell.jobsTip(running, queue.data?.workers.total ?? running, queued)
  const spinning = kind === 'run' && running > 0

  return (
    <a
      href={`#${paths.queue()}`}
      aria-label={`${t.shell.jobsAriaLabel}: ${text}`}
      aria-haspopup={onOpen ? 'dialog' : undefined}
      aria-expanded={onOpen ? expanded : undefined}
      title={tooltip}
      data-jobs={kind === 'run' ? 'running' : kind}
      className={`vv-jobs vv-jobs--${kind} ${stylex.props(styles.root, mobileBar && styles.mobile, kind === 'idle' && styles.idle, kind === 'offline' && styles.offline, mobileBar && kind === 'offline' && styles.mobileOffline).className}`}
      style={style}
      onClick={(e) => { e.preventDefault(); if (onOpen) onOpen(); else navigate(paths.queue()) }}
    >
      {kind === 'paused' || kind === 'offline'
        ? <Icon aria-hidden name={kind === 'paused' ? 'pause' : 'danger'} size={compact ? 13 : 14} xstyle={kind === 'paused' ? styles.pausedIcon : styles.offlineIcon} />
        : <span aria-hidden="true" className="vv-jobs__dot" {...stylex.props(styles.dot, kind === 'run' ? styles.runningDot : styles.idleDot, mobileBar && styles.mobileDot, spinning && styles.rec)} />}
      <span className={`num ${stylex.props(mobileBar && styles.mobileText).className ?? ''}`}>{visible}</span>
      {unread && <span role="img" aria-label={t.shell.jobsUnread} className={`vv-jobs__unread ${stylex.props(styles.unread).className}`} />}
      {mobileBar && <span aria-hidden="true" {...stylex.props(styles.touchHalo)} />}
    </a>
  )
}

const recPulse = stylex.keyframes({ '50%': { opacity: '.35' } })
const MOBILE = '@media (max-width: 767px)'
const TOUCH = '@media (max-width: 767px) and (pointer: coarse)'
const styles = stylex.create({
  root: { position: 'relative', height: 30, padding: `0 ${s['--vv-s-10']}`, display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], borderRadius: radius['--radius-control'], borderWidth: 0, borderStyle: "none", borderColor: { default: null, ':hover': line['--line-1'] }, backgroundColor: { default: "transparent", ':hover': color['--color-surface-1'] }, color: color['--color-fg-3'], fontSize: 12.5, whiteSpace: 'nowrap', textDecoration: 'none' },
  dot: { width: 8, height: 8, borderRadius: '50%' },
  /* Chấm đỏ nhấp khi có job chạy (trước ở .vv-rec/@keyframes vv-rec). */
  rec: { animationName: recPulse, animationDuration: '1.6s', animationTimingFunction: 'ease-in-out', animationIterationCount: 'infinite' },
  runningDot: { backgroundColor: "var(--vv-red)", boxShadow: '0 0 0 3px var(--vv-red-18)' },
  idleDot: { backgroundColor: color['--ring-paused'], boxShadow: `0 0 0 3px ${line['--line-3']}` },
  idle: { color: 'var(--vv-fg-note)' },
  offline: { color: 'var(--vv-warn-ink)' },
  mobileOffline: { borderColor: visualRoles.jobsOfflineBorder },
  pausedIcon: { color: 'var(--vv-st-neutral)' },
  offlineIcon: { color: color['--color-err'] },
  unread: { position: 'absolute', top: s['--vv-s-3'], right: s['--vv-s-3'], width: 7, height: 7, borderRadius: '50%', backgroundColor: color['--color-warn'], boxShadow: `0 0 0 2px ${color['--color-bg']}` },
  mobile: { height: 36, padding: `0 ${s['--vv-s-11']}`, gap: s['--vv-s-7'], borderRadius: r['--vv-r-pill'], borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-4'], backgroundColor: color['--color-surface-1'], color: color['--color-fg-2'], flex: { default: null, [MOBILE]: '0 1 auto' }, minWidth: { default: null, [MOBILE]: 0 }, overflow: { default: null, [MOBILE]: 'hidden', [TOUCH]: 'visible' } },
  mobileDot: { width: 7, height: 7 },
  mobileText: { minWidth: { default: null, [MOBILE]: 0 }, overflow: { default: null, [MOBILE]: 'hidden' }, textOverflow: { default: null, [MOBILE]: 'ellipsis' }, whiteSpace: { default: null, [MOBILE]: 'nowrap' } },
  touchHalo: { display: { default: 'none', [TOUCH]: 'block' }, position: 'absolute', inset: `calc((${s['--vv-s-44']} - 100%) / -2) 0` },
})
