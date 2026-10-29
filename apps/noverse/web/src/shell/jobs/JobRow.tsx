// Dòng việc trong bảng — 14-jobs-dock.md §1.5.
import type { CSSProperties } from 'react'
import type { Job, JobAction } from '../../api/types'
import { agentBar } from '../../ui/agentPalette'
import { useAction } from '../../data/useAction'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { JobRing } from './JobRing'
import { agentLine, formatEta, isIndeterminate, kindColor, pct, unitText } from './logic'
import { isAudioExportJob, jobError, musicFailure } from './exportFlow'
import { cancelJob, openHref, pauseJob, resumeJob, retryExport, retryJob, runAction } from './store'

export type Cta = { icon: string; label: string; primary?: boolean; run: () => void | Promise<void> }

export const actionIcon = (action: JobAction) =>
  action.icon ?? (action.kind === 'download' ? 'document-download' : action.kind === 'reveal' ? 'folder-open' : 'arrow-right-2')

export function CtaButton({ cta }: { cta: Cta }) {
  const action = useAction(async () => { await cta.run() })
  return (
    <Button type="button" size="sm" className={`jd-tbtn${cta.primary ? ' jd-primary' : ''}`}
      onClick={() => void action.run()} disabled={action.busy} pending={action.pending} success={action.success} successLabel="" style={{
      height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)',
      background: cta.primary ? '#EDEDEF' : 'transparent', color: cta.primary ? '#0B0B0D' : '#EDEDEF',
      fontSize: 12, fontWeight: 500, display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
    }}>
      <Icon name={cta.icon} size={14} />{cta.label}
    </Button>
  )
}

export function IconButton({ icon, label, onClick, rotate, size = 28, iconSize = 15, className, style }: {
  icon: string; label: string; onClick: () => void | Promise<void>; rotate?: boolean; size?: number; iconSize?: number; className?: string; style?: CSSProperties
}) {
  const action = useAction(async () => { await onClick() })
  return (
    <Button type="button" variant="icon" className={className ? `jd-ibtn ${className}` : 'jd-ibtn'} aria-label={label} title={label}
      disabled={action.busy} pending={action.pending} success={action.success} successLabel=""
      onClick={(event) => { event.stopPropagation(); void action.run() }} style={{
        width: size, height: size, borderRadius: size > 26 ? 8 : 7, border: 'none', background: 'transparent',
        color: '#8E8E99', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto', ...style,
      }}>
      <Icon name={icon} size={iconSize} style={rotate ? { transform: 'rotate(45deg)' } : undefined} />
    </Button>
  )
}

export function resultCtas(job: Job, after?: () => void): Cta[] {
  const actions: JobAction[] = job.result?.actions?.length ? job.result.actions : job.status === 'waiting'
    ? [{ kind: 'open', label: 'Xem & xác nhận', primary: true, href: job.href }]
    : []
  return actions.map((action) => ({
    icon: actionIcon(action), label: action.label, primary: action.primary,
    run: async () => { await runAction(job, action); after?.() },
  }))
}

/** Nút của việc lỗi trong bảng: Thử lại · Ghép không nhạc (§AE, lỗi do nhạc) · Bỏ qua. */
const failCtas = (job: Job): Cta[] => isAudioExportJob(job)
  ? [
    { icon: 'refresh-2', label: 'Thử lại', primary: true, run: () => retryExport(job) },
    ...(musicFailure(job) ? [{ icon: 'music', label: 'Ghép không nhạc', run: () => retryExport(job, true) }] : []),
    { icon: 'add', label: 'Bỏ qua', run: () => cancelJob(job) },
  ]
  : [
    { icon: 'refresh-2', label: 'Thử lại', primary: true, run: () => retryJob(job) },
    { icon: 'add', label: job.kind === 'audio' ? 'Bỏ qua chương này' : 'Bỏ qua', run: () => cancelJob(job) },
  ]

export function JobRow({ job, delay, out, flash }: { job: Job; delay: string; out: boolean; flash: number }) {
  const { status } = job
  const runLike = status === 'running' || status === 'paused'
  const icons: { icon: string; label: string; run: () => void | Promise<void>; rotate?: boolean }[] = []
  if (status === 'running') icons.push({ icon: 'pause', label: job.agent ? 'Tạm dừng sau bước này' : 'Tạm dừng', run: () => pauseJob(job) })
  if (status === 'paused') icons.push({ icon: 'play', label: 'Chạy tiếp', run: () => resumeJob(job) })
  if (job.href && (status === 'running' || status === 'paused')) icons.push({ icon: 'arrow-right-2', label: 'Mở', run: () => openHref(job.href) })
  if (status !== 'waiting') icons.push({ icon: 'add', label: status === 'done' ? 'Bỏ khỏi danh sách' : 'Huỷ', rotate: true, run: () => cancelJob(job) })

  let sub = ''
  let subFg = '#8E8E99'
  let meta = ''
  let barBg = ''
  let ctas: Cta[] = []
  if (runLike) {
    if (job.agent) { sub = agentLine(job); barBg = agentBar(job.agent); meta = isIndeterminate(job) ? formatEta(job.etaSec) : pct(job) }
    else if (isIndeterminate(job)) { sub = job.step ?? ''; barBg = kindColor(job); meta = formatEta(job.etaSec) }
    else { sub = [unitText(job), job.step].filter(Boolean).join(' · '); barBg = kindColor(job); meta = [pct(job), formatEta(job.etaSec)].filter(Boolean).join(' · ') }
    if (status === 'paused') { sub = `Đã tạm dừng · ${unitText(job) || pct(job)}`; barBg = '#5E5E68'; meta = isIndeterminate(job) ? '' : pct(job) }
  }
  if (status === 'waiting') { sub = job.step ?? job.result?.sub ?? ''; subFg = '#E9D9BD'; ctas = resultCtas(job) }
  if (status === 'failed') { sub = jobError(job); subFg = '#FFC9CE'; ctas = failCtas(job) }
  if (status === 'done') { sub = job.result?.sub ?? ''; ctas = resultCtas(job) }
  const title = status === 'done' ? (job.result?.title ?? job.title) : job.title

  return (
    <div className={`jd-row jd-stag${out ? ' jd-row-out' : ''}`}
      style={{ maxHeight: 120, display: 'flex', gap: 12, padding: '10px 8px', borderRadius: 12, animationDelay: delay }}>
      {flash > 0 && <span key={flash} className="jd-flash" aria-hidden="true" />}
      <JobRing job={job} variant="row" />
      <div style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 3 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
          <span style={{ flex: '1 1 auto', minWidth: 0, fontSize: 13.5, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{title}</span>
          {job.kind === 'audio' && status === 'running' && <span className="jd-eq" aria-hidden="true"><span /><span /><span /></span>}
          <span className="jd-act" style={{ display: 'flex', gap: 2, flex: '0 0 auto' }}>
            {icons.map((item) => (
              <IconButton key={item.label} icon={item.icon} label={item.label} onClick={item.run} rotate={item.rotate}
                size={26} iconSize={14} style={{ color: '#A0A0AB' }} />
            ))}
          </span>
        </div>
        <span style={{ fontSize: 12, color: subFg, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{sub}</span>
        {runLike && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 3 }}>
            <div className={`jd-bar${isIndeterminate(job) && status === 'running' ? ' jd-indet' : ''}`} style={{ flex: '1 1 auto' }}>
              <i style={{ width: `${((job.progress ?? 0) * 100).toFixed(1)}%`, background: barBg }} />
            </div>
            {meta && <span className="num" style={{ fontSize: 11.5, color: '#8E8E99', whiteSpace: 'nowrap', flex: '0 0 auto' }}>{meta}</span>}
          </div>
        )}
        {ctas.length > 0 && (
          <div style={{ display: 'flex', gap: 6, marginTop: 6 }}>
            {ctas.map((cta) => <CtaButton key={cta.label} cta={cta} />)}
          </div>
        )}
      </div>
    </div>
  )
}
