import { forwardRef } from 'react'
import type { Job } from '../api/types'
import { paths } from '../router'
import { pct, unitText, kindIcon, DONE_COLOR, isIndeterminate, ringColor } from '../shell/jobs/logic'
import { jobError } from '../shell/jobs/exportFlow'
import { Icon } from '@noverse/ui'
import { ProgressRing } from '@noverse/ui'

// Icon loại việc: artboard chỉ định export → export-curve, audio → audio-square; loại khác giữ icon của dock việc nền.
const rowIcon = (job: Job) => (job.kind === 'export' ? 'export-curve' : job.kind === 'audio' ? 'audio-square' : kindIcon(job))

type TriggerProps = { active: readonly Job[]; open: boolean; onClick: () => void }
export const JobsTrigger = forwardRef<HTMLButtonElement, TriggerProps>(function JobsTrigger({ active, open, onClick }, ref) {
  const first = active[0]
  return <button ref={ref} type="button" className={`tablet-jobs-trigger${open ? ' open' : ''}`} aria-expanded={open} aria-label={`${active.length} việc nền đang chạy`} onClick={onClick}>
    {first
      ? <ProgressRing className="tablet-ring" size={34} stroke={3} percent={['done', 'waiting', 'failed'].includes(first.status) ? 100 : (first.progress ?? 0) * 100} color={ringColor(first)} indeterminate={isIndeterminate(first) && first.status === 'running'}><span className="tablet-spinner" aria-hidden="true" /></ProgressRing>
      : <ProgressRing className="tablet-ring" size={34} stroke={3} percent={100} color={DONE_COLOR}><Icon name="tick-circle" size={15} style={{ color: DONE_COLOR }} /></ProgressRing>}
    {active.length} việc
  </button>
})

type PopoverProps = { jobs: readonly Job[]; loaded: boolean; story: { project: string; story: string } | null; onOpen: (job: Job) => void; onLog: () => void }
export const JobsPopover = forwardRef<HTMLDivElement, PopoverProps>(function JobsPopover({ jobs, loaded, story, onOpen, onLog }, ref) {
  return <div id="tablet-jobs" className="tablet-pop tablet-jobs" ref={ref} role="dialog" aria-label="Việc nền">
    <div className="tablet-jobs-title">VIỆC NỀN</div>
    {!loaded && <p role="status">Đang tải việc nền…</p>}
    {loaded && jobs.length === 0 && <p>Không có việc chạy nền.</p>}
    {jobs.map((job) => <button type="button" className="tablet-prow tablet-job" key={job.id} onClick={() => onOpen(job)}>
      <ProgressRing className="tablet-ring" size={40} stroke={3} percent={['done', 'waiting', 'failed'].includes(job.status) ? 100 : (job.progress ?? 0) * 100} color={ringColor(job)} indeterminate={isIndeterminate(job) && job.status === 'running'}><Icon name={rowIcon(job)} size={17} style={{ color: ringColor(job) }} /></ProgressRing>
      <span className="tablet-job-copy"><span>{job.result?.title ?? job.title}</span><small>{jobError(job) || job.result?.sub || job.step || unitText(job)}</small></span>
      <span className="tablet-job-pct">{pct(job)}</span>
    </button>)}
    {story && <a className="tablet-log-link" href={`#${paths.activity(story.project, story.story)}`} onClick={onLog}>Mở nhật ký hoạt động</a>}
  </div>
})
