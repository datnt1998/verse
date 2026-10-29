// JobProgress — tiến trình việc dài (import · sync · simulate · export) từ SSE `job`
// (06-data-contract.md › SSE /events). Hiện thẻ gọn phía trên 2 panel của Workspace.
import type { JobEvent } from '../../api/types'
import { t } from '../../i18n/vi'
import { translateHost } from '../../i18n/hostStrings'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { ProgressBar } from '@noverse/ui'
import { useNow } from './hooks'

const JOB_ICON: Record<string, string> = {
  import: 'document-text',
  simulate: 'magic-star',
  importsim: 'data',
  sync: 'refresh-2',
  export: 'export-curve',
}

export function JobProgress({ jobs, trackedJobs, onConfirm, canConfirm = true, busy = false, pending = false, success = false }: {
  jobs: Map<string, JobEvent>; trackedJobs?: ReadonlySet<string>; onConfirm: () => void
  canConfirm?: boolean; busy?: boolean; pending?: boolean; success?: boolean
}) {
  const now = useNow(1000)
  const active = [...jobs.values()].filter((job) => !trackedJobs?.has(job.job) && (!job.done || job.error !== ''))
  if (active.length === 0) return null
  return (
    <div style={{ flex: '0 0 auto', display: 'flex', flexDirection: 'column', gap: 8 }}>
      {active.map((job) => {
        const total = job.total > 0 ? job.total : 0
        const percent = total > 0 ? (job.current / total) * 100 : 0
        const retryIn = job.retryAt ? Math.max(0, Math.ceil((Date.parse(job.retryAt) - now) / 1000)) : 0
        return (
          <div key={job.job} className="card" role="status" aria-label={t.workspace.jobLabel[job.kind] ?? t.status.running} style={{
            borderRadius: 12, padding: '10px 12px', display: 'flex', flexDirection: 'column', gap: 8,
            borderColor: job.error ? 'rgba(255,154,162,0.35)' : job.awaiting ? 'rgba(246,203,140,0.35)' : undefined,
          }}>
            <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 10 }}>
              <Icon name={JOB_ICON[job.kind] ?? 'activity'} size={16} style={{ color: 'var(--color-fg-3)' }} />
              <span style={{ fontSize: 13, fontWeight: 500 }}>{t.workspace.jobLabel[job.kind] ?? t.status.running}</span>
              <span style={{ flex: 1, minWidth: 0, fontSize: 12.5, color: job.error ? 'var(--color-err)' : 'var(--color-fg-3)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                {job.error ? translateHost(job.error) : translateHost(job.message)}
              </span>
              {total > 0 && <span className="num mono" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{t.workspace.jobProgress(job.current, total)}</span>}
              {retryIn > 0 && <span className="num" style={{ fontSize: 12, fontStyle: 'italic', color: 'var(--color-fg-4)' }}>{t.workspace.retryIn(retryIn)}</span>}
              {canConfirm && job.awaiting === 'confirm' && (
                <Button variant="primary" size="sm" onClick={onConfirm} disabled={busy} pending={pending} success={success}>{t.workspace.jobConfirm}</Button>
              )}
            </div>
            {total > 0 && <ProgressBar value={percent} tone={job.error ? 'err' : job.awaiting ? 'warn' : 'writer'} />}
          </div>
        )
      })}
    </div>
  )
}
