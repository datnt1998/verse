import { useState } from 'react'
import { BottomSheet } from '../../../mobile/primitives'
import { jobActions as actions, useJobs } from '../../../shell/jobs/useJobs'
import { groupJobs, kindColor, pct, unitText } from '../../../shell/jobs/logic'
import { jobError } from '../../../shell/jobs/exportFlow'
import { JobRing } from '../../../shell/jobs/JobRing'
import { useAction } from '../../../data'
import { navigate } from '../../../router'
import { Icon } from '@noverse/ui'
import type { Job } from '../../../api/types'

function MobileJob({ job, onClose }: { job: Job; onClose: () => void }) {
  const [expanded, setExpanded] = useState(false)
  const action = useAction(async (run: () => Promise<void>) => { await run() }, { error: 'Không thực hiện được việc nền' })
  const run = (fn: () => Promise<void>) => { void action.run(fn) }
  return <div className="mw-job">
    <button className="mw-job-row" onClick={() => setExpanded(!expanded)} aria-expanded={expanded}>
      <JobRing job={job} variant="mobile" /><span className="m-row-copy"><span className="mw-between"><strong>{job.result?.title ?? job.title}</strong><small className="num">{job.progress !== null ? pct(job) : job.status === 'done' ? 'Đã xong' : job.status === 'paused' ? 'Tạm dừng' : job.status === 'failed' ? 'Lỗi' : job.status === 'running' ? 'Đang chạy' : 'Đang chờ'}</small></span><small>{jobError(job) || job.result?.sub || job.step || unitText(job)}</small><span className="m-progress"><span style={{ width: `${Math.round((job.progress ?? (job.status === 'done' ? 1 : 0)) * 100)}%`, background: kindColor(job) }} /></span></span>
    </button>
    {expanded && <div className="mw-job-actions">
      {job.status === 'running' && <button className="m-chip" disabled={action.busy} onClick={() => run(() => actions.pause(job))}><Icon name="pause" size={14} />Tạm dừng</button>}
      {job.status === 'paused' && <button className="m-chip" disabled={action.busy} onClick={() => run(() => actions.resume(job))}><Icon name="play" size={14} />Chạy tiếp</button>}
      {job.status === 'failed' && <button className="m-chip" disabled={action.busy} onClick={() => run(() => actions.retry(job))}>Thử lại</button>}
      {job.result?.actions.map((cta) => <button key={cta.label} className="m-chip" disabled={action.busy} onClick={() => run(() => actions.runAction(job, cta))}>{cta.label}</button>)}
      {job.status === 'waiting' && !job.result?.actions.length && <button className="m-chip" onClick={() => { onClose(); actions.openHref(job.href) }}>Xem & xác nhận</button>}
      {job.status !== 'waiting' && <button className="m-chip" disabled={action.busy} onClick={() => run(() => actions.cancel(job))}>{job.status === 'done' ? 'Bỏ khỏi danh sách' : 'Huỷ'}</button>}
      {action.pending && <span className="m-muted" role="status">Đang thực hiện…</span>}{action.success && <span className="m-muted" role="status">Đã thực hiện</span>}{action.error != null && <span className="m-error" role="alert">{action.error instanceof Error ? action.error.message : 'Không thực hiện được'}</span>}
    </div>}
  </div>
}
export function JobsSheet({ open, onClose, logHref }: { open: boolean; onClose: () => void; logHref?: string }) {
  const { jobs, state } = useJobs(open)
  return <BottomSheet open={open} onClose={onClose} label="Việc chạy nền" height="auto">
    <div className="mw-jobs-head"><h2>Việc chạy nền</h2><span>vẫn chạy khi khoá màn hình</span></div>
    {!state.loaded && <p className="m-muted" style={{ padding: 16 }} role="status">Đang tải việc nền…</p>}
    {state.loaded && jobs.length === 0 && <div className="m-desktop-note"><Icon name="tick-circle" size={32} /><p>Không có việc chạy nền.</p></div>}
    {groupJobs(jobs).flatMap((group) => group.jobs).map((job) => <MobileJob key={job.id} job={job} onClose={onClose} />)}
    {logHref && <button type="button" className="m-button secondary" style={{ margin: '12px 16px 0' }} onClick={() => { onClose(); navigate(logHref) }}>Xem nhật ký đầy đủ</button>}
  </BottomSheet>
}
