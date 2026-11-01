// Bốn trạng thái việc nền dùng dữ liệu thật. Máy tính: viên chữ 30px.
// Điện thoại (r36): nút tròn 36px — vòng tiến độ + số việc đang chạy, rảnh chỉ
// icon (04-components › Đầu trang điện thoại › AppBar), chạm 44px.
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
  /** có thông báo cần bạn chưa đọc → chấm vàng trên nút; số thì câu aria ghi rõ số */
  unread?: boolean | number
  compact?: boolean
  /** điện thoại: nút tròn 36px thay viên chữ */
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
  /* Vòng tiến độ = trung bình phần đã xong của các việc đang chạy. Mỗi video có
     9 bước (0–8) nên bước cuối là 100%. Chưa có dữ liệu hàng chờ thì giữ vòng
     rỗng thay vì vẽ một tiến độ không có thật. */
  const steps = (queue.data?.running ?? []).filter((job) => job.stepIndex >= 0)
  const progress = steps.length ? steps.reduce((sum, job) => sum + Math.min(1, (job.stepIndex + 1) / 9), 0) / steps.length : 0
  /* Ẩn khi mất kết nối: chưa đọc được thông báo mới (components › AppBar). */
  const showDot = !!unread && kind !== 'offline'
  /* Câu aria của nút tròn: copy 06 › Thanh trên điện thoại và Thông báo. */
  const unreadNote = typeof unread === 'number' ? (unread > 0 ? t.shell.jobsUnreadCount(unread) : '') : t.shell.jobsUnreadHint

  if (mobileBar) {
    const aria = kind === 'offline' ? t.shell.jobsMobileOffline
      : kind === 'paused' ? t.shell.jobsMobilePaused(queued, time)
      : kind === 'idle' ? t.shell.jobsMobileIdle
      : t.shell.jobsMobileRunning(running, queued)
    const icon = kind === 'offline' ? 'danger' : kind === 'paused' ? 'pause' : 'activity'
    return (
      <a
        href={`#${paths.queue()}`}
        aria-label={showDot ? `${aria}, ${unreadNote}` : aria}
        aria-haspopup={onOpen ? 'dialog' : undefined}
        aria-expanded={onOpen ? expanded : undefined}
        data-jobs={kind === 'run' ? 'running' : kind}
        className={`vv-jobs vv-jobsbtn vv-jobsbtn--${kind === 'run' ? 'running' : kind} ${stylex.props(styles.round, kind === 'paused' && styles.roundPaused, kind === 'offline' && styles.roundOffline).className}`}
        onClick={(e) => { e.preventDefault(); if (onOpen) onOpen(); else navigate(paths.queue()) }}
      >
        {(kind === 'run' || kind === 'paused') && (
          <svg aria-hidden="true" viewBox="0 0 36 36" className={`vv-jobsbtn__ring ${stylex.props(styles.ring).className}`}>
            <circle cx="18" cy="18" r="17" className={`vv-jobsbtn__trk ${stylex.props(styles.ringTrack).className}`} />
            {/* 106,8 = chu vi r 17; tạm dừng thì vòng xám đầy. */}
            <circle cx="18" cy="18" r="17" strokeDashoffset={kind === 'paused' ? 0 : Number((106.8 * (1 - progress)).toFixed(1))}
              className={`vv-jobsbtn__arc ${stylex.props(styles.ringArc, kind === 'paused' && styles.ringArcPaused).className}`} />
          </svg>
        )}
        {kind === 'run'
          ? <span className={`vv-jobsbtn__n ${stylex.props(styles.roundNumber).className}`}>{running}</span>
          : <Icon aria-hidden name={icon} size={kind === 'paused' ? 15 : 18}
              xstyle={kind === 'idle' ? styles.roundIdleIcon : kind === 'offline' ? styles.offlineIcon : styles.pausedIcon} />}
        {showDot && <span aria-hidden="true" className={`vv-notice-dot ${stylex.props(styles.roundDot).className}`} />}
        <span aria-hidden="true" {...stylex.props(styles.roundTouch)} />
      </a>
    )
  }

  return (
    <a
      href={`#${paths.queue()}`}
      aria-label={`${t.shell.jobsAriaLabel}: ${text}`}
      aria-haspopup={onOpen ? 'dialog' : undefined}
      aria-expanded={onOpen ? expanded : undefined}
      title={tooltip}
      data-jobs={kind === 'run' ? 'running' : kind}
      className={`vv-jobs vv-jobs--${kind} ${stylex.props(styles.root, kind === 'idle' && styles.idle, kind === 'offline' && styles.offline).className}`}
      style={style}
      onClick={(e) => { e.preventDefault(); if (onOpen) onOpen(); else navigate(paths.queue()) }}
    >
      {kind === 'paused' || kind === 'offline'
        ? <Icon aria-hidden name={kind === 'paused' ? 'pause' : 'danger'} size={compact ? 13 : 14} xstyle={kind === 'paused' ? styles.pausedIcon : styles.offlineIcon} />
        : <span aria-hidden="true" className={`vv-jobs__dot ${stylex.props(styles.dot, kind === 'run' ? styles.runningDot : styles.idleDot, spinning && styles.rec).className}`} />}
      <span className="num">{visible}</span>
      {showDot && <span role="img" aria-label={unreadNote || t.shell.jobsUnread} className={`vv-jobs__unread ${stylex.props(styles.unread).className}`} />}
    </a>
  )
}

const recPulse = stylex.keyframes({ '50%': { opacity: '.35' } })
const TOUCH = '@media (max-width: 767px) and (pointer: coarse)'
const styles = stylex.create({
  root: { position: 'relative', height: 30, padding: `0 ${s['--vv-s-10']}`, display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], borderRadius: radius['--radius-control'], borderWidth: 0, borderStyle: "none", borderColor: { default: null, ':hover': line['--line-1'] }, backgroundColor: { default: "transparent", ':hover': color['--color-surface-1'] }, color: color['--color-fg-3'], fontSize: 12.5, whiteSpace: 'nowrap', textDecoration: 'none' },
  /* Viên co lại được nhưng chấm thì không: flex-shrink mặc định bóp chấm về 0px
     khi topbar hẹp, mất hẳn dấu trạng thái. */
  dot: { width: 8, height: 8, borderRadius: '50%', flexShrink: 0 },
  /* Chấm đỏ nhấp khi có job chạy (trước ở .vv-rec/@keyframes vv-rec). */
  rec: { animationName: recPulse, animationDuration: '1.6s', animationTimingFunction: 'ease-in-out', animationIterationCount: 'infinite' },
  runningDot: { backgroundColor: "var(--vv-red)", boxShadow: '0 0 0 3px var(--vv-red-18)' },
  idleDot: { backgroundColor: color['--ring-paused'], boxShadow: `0 0 0 3px ${line['--line-3']}` },
  idle: { color: 'var(--vv-fg-note)' },
  offline: { color: 'var(--vv-warn-ink)' },
  pausedIcon: { color: 'var(--vv-st-neutral)' },
  offlineIcon: { color: color['--color-err'] },
  unread: { position: 'absolute', top: s['--vv-s-3'], right: s['--vv-s-3'], width: 7, height: 7, borderRadius: '50%', backgroundColor: color['--color-warn'], boxShadow: `0 0 0 2px ${color['--color-bg']}`, flexShrink: 0, zIndex: 1 },

  /* Điện thoại — nút tròn 36px, cùng nền/viền với bộ đổi kênh và avatar. */
  round: { position: 'relative', flex: '0 0 auto', width: 36, height: 36, boxSizing: 'border-box', padding: 0, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', borderRadius: '50%', borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'], backgroundColor: color['--color-surface-1'], color: color['--color-fg'], textDecoration: 'none' },
  roundPaused: { color: 'var(--vv-st-neutral)' },
  roundOffline: { borderColor: visualRoles.jobsOfflineBorder, color: color['--color-err'] },
  roundIdleIcon: { color: color['--color-fg-4'] },
  /* Vòng 2px nằm đúng mép nút: inset -1px để phủ lên viền 1px. */
  ring: { position: 'absolute', inset: -1, width: 36, height: 36, transform: 'rotate(-90deg)', pointerEvents: 'none' },
  ringTrack: { fill: 'none', stroke: line['--line-2'], strokeWidth: 2 },
  ringArc: { fill: 'none', stroke: 'var(--vv-red)', strokeWidth: 2, strokeLinecap: 'round', strokeDasharray: 106.8, transitionProperty: 'stroke-dashoffset', transitionDuration: '.3s', transitionTimingFunction: 'linear' },
  ringArcPaused: { stroke: 'var(--vv-st-neutral)', transitionProperty: 'none' },
  roundNumber: { position: 'relative', fontSize: 13, fontWeight: 600, fontVariantNumeric: 'tabular-nums', color: color['--color-fg'] },
  /* Chấm 10px, viền 2px màu nền, tâm nằm trên mép tròn góc trên phải. */
  roundDot: { position: 'absolute', top: -1, right: -1, width: 10, height: 10, borderRadius: '50%', backgroundColor: color['--color-warn'], boxShadow: `0 0 0 2px ${color['--color-bg']}`, zIndex: 1, pointerEvents: 'none' },
  /* Vùng chạm 44px: con cuối nên phải xuyên pointer, nếu không nó nuốt cú bấm. */
  roundTouch: { display: { default: 'none', [TOUCH]: 'block' }, position: 'absolute', inset: `calc((${s['--vv-s-44']} - 100%) / -2)`, zIndex: 0, pointerEvents: 'none' },
})
