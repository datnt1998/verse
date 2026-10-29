// Dock việc chạy nền — 14-jobs-dock.md §1 · Jobs.dc.html. Gắn một lần ở AppShell (ngoài router).
// Bốn trạng thái: ẩn · chấm (52) · pill (352×52) · bảng (392); neo right 16 / bottom 16.
import { useEffect, useMemo, useState } from 'react'
import './jobs.css'
import { useSession } from '../../api/session'
import { paths } from '../../router'
import { Icon } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { DoneRing, JobRing } from './JobRing'
import { IconButton, JobRow } from './JobRow'
import { JobToast } from './JobToast'
import { aggregateProgress, groupJobs, hiddenOnWorkspace, panelSubtitle, pillLines, showsAsDot } from './logic'
import { clearDone, dispatchDock, refreshJobsNow, setHoverToasts, toggleNotify } from './store'
import { useJobs } from './useJobs'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

const IDLE_MS = 4200
const LEAVE_MS = 380
const LINE_MS = 4200
const DOT_SPACE = '--dock-dot-space'
/** Màn có hàng thao tác sát đáy tự khai báo chiều cao để dock không đè lên nút chính. */
const DOCK_BOTTOM = '--dock-bottom'

const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)

export function JobsDock() {
  const { user } = useSession()
  const { state: s, where, jobs: visible, active } = useJobs()
  const sorted = active // useJobs đã sắp theo ưu tiên
  const workspace = where.workspace
  const lastDone = useMemo(() => visible
    .filter((job) => job.status === 'done')
    .reduce<typeof visible[number] | undefined>((latest, job) => (!latest || (job.finishedAt ?? '') > (latest.finishedAt ?? '') ? job : latest), undefined), [visible])
  const hiddenNote = s.jobs.some((job) => hiddenOnWorkspace(job, where))
  const { mode, panelAnim, leaving, enterN } = s.dock
  const panelOpen = mode === 'panel'
  const dot = showsAsDot(mode, workspace)
  const showDock = mode !== 'hidden' && (panelOpen || !(workspace && active.length === 0 && !lastDone && s.toasts.length === 0))

  // Một việc agent bị ẩn ở Workspace phải hiện lại khi chuyển sang màn khác.
  useEffect(() => {
    if (s.loaded && active.length > 0 && mode === 'hidden') dispatchDock({ type: 'jobAdded' })
  }, [s.loaded, active.length, mode])

  const [slow, setSlow] = useState(false)
  useEffect(() => {
    if (s.loaded || s.error) { setSlow(false); return }
    const timer = setTimeout(() => setSlow(true), 8000)
    return () => clearTimeout(timer)
  }, [s.loaded, s.error])

  // Dòng chữ trên pill trượt 36 px mỗi ~4,2 s giữa các việc.
  const lines = useMemo(() => pillLines(active, lastDone), [active, lastDone])
  const [tick, setTick] = useState(0)
  useEffect(() => {
    if (lines.length < 2) return
    const timer = setInterval(() => setTick((value) => value + 1), LINE_MS)
    return () => clearInterval(timer)
  }, [lines.length])
  const tickI = lines.length > 1 ? tick % lines.length : 0

  // Xong hết + 4,2 s → dock thu về góc (380 ms) → ẩn, dọn mục xong.
  const idle = active.length === 0 && s.toasts.length === 0 && mode !== 'hidden' && mode !== 'panel'
  useEffect(() => {
    if (!idle) return
    let hideTimer: ReturnType<typeof setTimeout> | undefined
    const timer = setTimeout(() => {
      dispatchDock({ type: 'idleOut' })
      hideTimer = setTimeout(() => { dispatchDock({ type: 'hide' }); clearDone() }, LEAVE_MS)
    }, IDLE_MS)
    return () => { clearTimeout(timer); clearTimeout(hideTimer) }
  }, [idle])

  // ⌥J / Alt+J bật/tắt bảng · Esc khi bảng mở → thu về pill.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.altKey && !event.metaKey && !event.ctrlKey && event.code === 'KeyJ') {
        event.preventDefault()
        dispatchDock({ type: 'toggle' })
        return
      }
      if (event.key === 'Escape' && mode === 'panel' && !event.defaultPrevented && !document.querySelector('[aria-modal="true"]')) {
        dispatchDock({ type: 'close' })
      }
    }
    addEventListener('keydown', onKey)
    return () => removeEventListener('keydown', onKey)
  }, [mode])

  // Chừa chỗ cho pill/chấm của dock (52 px + 16 px lề): Workspace cho cột agent, Trình đọc cho
  // cột thẩm duyệt; màn khác không đọc biến này. Bảng mở thì như hộp nổi, không chừa.
  const reserveDock = showDock && !panelOpen
  useEffect(() => {
    if (!reserveDock) return
    document.documentElement.style.setProperty(DOT_SPACE, '68px')
    return () => { document.documentElement.style.removeProperty(DOT_SPACE) }
  }, [reserveDock])

  if (!user) return null

  const multi = active.length > 1 && !dot
  const rings = sorted.slice(0, dot ? 1 : 3)
  const focusRing = tickI % Math.max(1, Math.min(3, sorted.length))
  const agg = aggregateProgress(active)
  const groups = groupJobs(visible)
  const toasts = s.toasts.slice(-2).reverse()
  const activityTarget = where.project && where.story ? where : sorted[0] ?? lastDone
  const activityHref = activityTarget?.project && activityTarget.story ? `#${paths.activity(activityTarget.project, activityTarget.story)}` : undefined
  const pillClick = () => dispatchDock({ type: 'clickPill', workspace })

  return (
    <section aria-label="Việc chạy nền" style={{ position: 'fixed', right: 16, bottom: `calc(16px + var(${DOCK_BOTTOM}, 0px))`, zIndex: 35, width: 392, pointerEvents: 'none' }}>
      {/* toasts — chỉ khi bảng đóng; mới nhất sát pill, cũ lùi lên */}
      {!panelOpen && toasts.length > 0 && (
        <div role="status" aria-live="polite"
          onMouseEnter={() => setHoverToasts(true)} onMouseLeave={() => setHoverToasts(false)}
          style={{ position: 'absolute', right: 0, bottom: 56, width: 352, display: 'flex', flexDirection: 'column-reverse', pointerEvents: 'auto' }}>
          {toasts.map((item, i) => <JobToast key={item.id} toast={item} front={i === 0} hover={s.hoverToasts} />)}
        </div>
      )}

      {/* pill / chấm */}
      {showDock && !panelOpen && (
        <div key={`dock-${enterN}`} className={leaving ? 'jd-out' : enterN % 2 ? 'jd-in-b' : 'jd-in-a'}
          style={{ position: 'absolute', right: 0, bottom: 0, pointerEvents: 'auto' }}>
          {s.bumpN > 0 && <span key={s.bumpN} aria-hidden="true" className={s.bumpN % 2 ? 'jd-glow-a' : 'jd-glow-b'} style={{ borderRadius: 26 }} />}
          <div className={`jd-glass jd-pill${s.bumpN ? (s.bumpN % 2 ? ' jd-bump-a' : ' jd-bump-b') : ''}`}
            role="button" tabIndex={0} aria-expanded="false"
            aria-label={`Việc chạy nền: ${active.length} việc. Bấm để mở`}
            onClick={pillClick}
            onKeyDown={(event) => { if (event.target === event.currentTarget && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); pillClick() } }}
            style={{
              position: 'relative', width: dot ? 52 : 352, height: 52, borderRadius: 26, display: 'flex', alignItems: 'center',
              gap: 10, padding: `0 ${dot ? 0 : 8}px 0 9px`, cursor: 'pointer', overflow: 'hidden',
            }}>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center', flex: '0 0 auto' }}>
              {rings.length ? rings.map((job, i) => (
                <JobRing key={job.id} job={job} variant="pill" style={{
                  marginLeft: i ? -12 : 0, zIndex: 5 - i, transition: 'opacity .4s',
                  opacity: sorted.length > 1 && i !== focusRing && !dot ? 0.5 : 1,
                }} />
              )) : <DoneRing />}
              {!dot && sorted.length > 3 && (
                <span className="jd-rg num" style={{ width: 34, height: 34, marginLeft: -12, fontSize: 11, color: '#C9C9D1' }}>+{sorted.length - 3}</span>
              )}
              {active.some((job) => job.status === 'waiting') && (
                <span className="jd-ping" style={{ top: -1, left: 26, width: 9, height: 9, boxShadow: '0 0 0 2px #1A1A1C' }} />
              )}
            </div>
            <div className="jd-txtfade" style={{ flex: '1 1 auto', minWidth: 0, height: 36, overflow: 'hidden', opacity: dot ? 0 : 1 }}>
              <div className="jd-tick-col" style={{ transform: `translateY(${-36 * tickI}px)` }}>
                {lines.map((line, i) => (
                  <div key={i} style={{ height: 36, display: 'flex', flexDirection: 'column', justifyContent: 'center', minWidth: 0 }}>
                    <span style={{ fontSize: 13, fontWeight: 500, color: line.fg, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{line.a}</span>
                    <span className="num" style={{ fontSize: 11.5, color: '#8E8E99', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{line.b}</span>
                  </div>
                ))}
              </div>
            </div>
            <div className="jd-txtfade" inert={dot} style={{ display: 'flex', alignItems: 'center', gap: 2, flex: '0 0 auto', opacity: dot ? 0 : 1 }}>
              <IconButton icon="minus" label="Thu nhỏ thành chấm" onClick={() => dispatchDock({ type: 'toMini' })} className="jd-hov" style={{ width: 28 }} />
              {multi && (
                <span className="num" style={{
                  minWidth: 22, height: 22, padding: '0 6px', borderRadius: 999, background: 'rgba(255,255,255,0.08)', color: '#EDEDEF',
                  fontSize: 11.5, fontWeight: 600, display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>{active.length}</span>
              )}
              <Icon name="arrow-up-2" size={16} style={{ color: '#A0A0AB', margin: '0 4px' }} />
            </div>
            <span aria-hidden="true" style={{
              position: 'absolute', left: 18, right: 18, bottom: 0, height: 2, borderRadius: 1, background: 'rgba(255,255,255,0.05)',
              overflow: 'hidden', opacity: dot ? 0 : 1,
            }}>
              <span className={active.length ? 'jd-agg' : undefined} style={{
                display: 'block', height: '100%', width: `${((agg ?? 1) * 100).toFixed(1)}%`,
                background: active.length ? undefined : 'rgba(166,237,193,0.55)',
              }} />
            </span>
          </div>
        </div>
      )}

      {/* bảng — morph clip-path từ đúng khung pill */}
      {showDock && panelOpen && (
        <div className={`jd-glass${panelAnim === 'in' ? ' jd-morph-in' : panelAnim === 'out' ? ' jd-morph-out' : ''}`}
          role="dialog" aria-label="Việc chạy nền"
          style={{ position: 'absolute', right: 0, bottom: 0, width: 392, borderRadius: 20, display: 'flex', flexDirection: 'column', overflow: 'hidden', pointerEvents: 'auto' }}>
          <PanelRefreshBar resourceKeys={[[ 'jobs' ]]} updatedAt={s.updatedAt} />
          <div aria-hidden="true" style={{ position: 'absolute', inset: '0 0 auto 0', height: 120, background: 'radial-gradient(90% 100% at 20% 0%, rgba(154,99,227,0.12), transparent 70%)', pointerEvents: 'none' }} />
          <div className="jd-stag" style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 10, padding: '14px 12px 10px 16px', animationDelay: '.1s' }}>
            <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
              <span style={{ fontSize: 14.5, fontWeight: 600 }}>Việc chạy nền</span>
              <span title={s.updatedAt ? `Cập nhật lúc ${new Date(s.updatedAt).toLocaleTimeString('vi-VN')}` : 'Vẫn chạy khi bạn chuyển sang màn khác'} style={{ fontSize: 12, color: '#8E8E99', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{panelSubtitle(active)}</span>
            </span>
            <button type="button" className="jd-ibtn" onClick={() => void toggleNotify()} aria-pressed={s.notify} title="Báo khi xong"
              style={{
                height: 28, padding: '0 8px', borderRadius: 8, border: 'none', background: 'transparent',
                color: s.notify ? '#C9A6F5' : '#7E7E8A', fontSize: 11.5, display: 'flex', alignItems: 'center', gap: 5, whiteSpace: 'nowrap',
              }}>
              <Icon name="notification" size={15} />{s.notify ? 'Báo khi xong' : 'Không báo'}
            </button>
            <IconButton icon="minus" label="Thu nhỏ thành chấm" onClick={() => dispatchDock({ type: 'toMini' })} />
          </div>
          <StaleBanner error={s.error} onRetry={refreshJobsNow} />
          <div className="scroll" style={{ position: 'relative', flex: '1 1 auto', minHeight: 0, maxHeight: 470, overflowY: 'auto', padding: '0 8px 6px' }}>
            {!s.loaded && !s.error && groups.length === 0 && <div aria-label="Đang tải việc chạy nền" style={{ display: 'grid', gap: 18, padding: '16px 8px' }}>{[0, 1, 2].map((i) => <div key={i} style={{ display: 'flex', gap: 12 }}><Skeleton width={36} height={36} radius={18} /><div style={{ flex: 1, display: 'grid', gap: 8 }}><Skeleton width="70%" /><Skeleton width="90%" height={10} /></div></div>)}{slow && <span style={{ fontSize: 12, color: '#8E8E99' }}>Đang chậm hơn bình thường…</span>}</div>}
            {groups.map((group, gi) => (
              <div key={group.id}>
                <div className="jd-stag" style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '8px 8px 4px', animationDelay: `${(0.1 + gi * 0.13).toFixed(2)}s` }}>
                  <span className="lbl" style={{ fontSize: 10.5, color: group.color }}>{group.label}</span>
                  <span className="num" style={{ fontSize: 10.5, color: '#5E5E68' }}>{group.jobs.length}</span>
                </div>
                {group.jobs.map((job, ri) => (
                  <JobRow key={job.id} job={job} out={Boolean(s.out[job.id])} flash={s.flash[job.id] ?? 0}
                    delay={`${(0.12 + (gi * 3 + ri) * 0.045).toFixed(3)}s`} />
                ))}
              </div>
            ))}
            {groups.length === 0 && (s.loaded || !!s.error) && (
              <div style={{ padding: '22px 12px', textAlign: 'center', fontSize: 12.5, color: '#7E7E8A' }}>{s.error && !s.loaded ? 'Không tải được việc chạy nền.' : 'Không còn việc nào chạy nền.'}</div>
            )}
            {hiddenNote && (
              <div style={{ margin: '4px 8px 6px', padding: '8px 10px', borderRadius: 10, background: 'rgba(255,255,255,0.03)', fontSize: 11.5, color: '#8E8E99', display: 'flex', gap: 8, alignItems: 'center' }}>
                <span className="jd-orb" aria-hidden="true" style={{ width: 12, height: 12, background: 'var(--orb-writer)', boxShadow: 'var(--orb-shade)' }} />
                Việc của các trợ lý đang hiện ngay trên Bàn viết nên không nhắc lại ở đây.
              </div>
            )}
          </div>
          {/* chân bảng = cùng khung pill để morph liền mạch */}
          <div style={{ position: 'relative', height: 52, display: 'flex', alignItems: 'center', gap: 10, padding: '0 10px 0 16px', borderTop: '1px solid rgba(255,255,255,0.06)' }}>
            <Icon name="activity" size={16} style={{ color: '#A0A0AB' }} />
            {activityHref
              ? <a href={activityHref} style={{ flex: '1 1 auto', minWidth: 0, fontSize: 12.5, color: '#C9C9D1', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>Xem nhật ký đầy đủ</a>
              : <span style={{ flex: '1 1 auto', minWidth: 0, fontSize: 12.5, color: '#7E7E8A', whiteSpace: 'nowrap' }}>Xem nhật ký đầy đủ</span>}
            <span style={{ fontSize: 11, color: '#5E5E68', display: 'flex', gap: 4, alignItems: 'center', whiteSpace: 'nowrap' }}>
              <span className="jd-kbd">{isMac ? '⌥' : 'Alt'}</span><span className="jd-kbd">J</span>
            </span>
            <button type="button" className="jd-ibtn" onClick={() => dispatchDock({ type: 'close' })} aria-label="Thu gọn"
              style={{
                height: 32, padding: '0 10px', borderRadius: 9, border: 'none', background: 'rgba(255,255,255,0.06)', color: '#EDEDEF',
                fontSize: 12, display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
              }}>
              Thu gọn<Icon name="arrow-down-2" size={14} />
            </button>
          </div>
        </div>
      )}
    </section>
  )
}
