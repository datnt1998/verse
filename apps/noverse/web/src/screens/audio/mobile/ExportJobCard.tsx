// Gói AE (MAudio.dc.html › states bgRun · bgDone · bgManual · bgBlocked · bgFail) —
// thẻ việc nổi trên trình phát thu nhỏ: thay cho `exportRun` trong sheet.
// Nội dung chữ và nút lấy từ `exportLook(job, true)` của shell/jobs/exportFlow (một nguồn
// chung với toast máy tính); ở đây chỉ dựng hình theo artboard `#mjob`.
import { useCallback, useReducer } from 'react'
import type { Job } from '../../../api/types'
import { useMobileToast } from '../../../mobile/primitives'
import { useToastBottom } from '../../../mobile/toastBottom'
import { useJobSelection } from '../../../shell/jobs/useJobs'
import {
  exportLook, isAudioExportJob, type ExportActionId, type ExportLook,
} from '../../../shell/jobs/exportFlow'
import {
  cancelJob, downloadExportManual, retryExport, shareExportJob,
} from '../../../shell/jobs/store'
import type { JobsState } from '../../../shell/jobs/store'
import { Icon } from '@noverse/ui'
import { ProgressRing } from '@noverse/ui'
import { audioExportCopy as x } from '../../../i18n/vi'
import { errText } from '../useAudio'

/** Đã đóng thẻ cho đúng lượt chạy này (id + finishedAt) — sống qua lần mount lại của màn. */
const dismissed = new Set<string>()
const cardKey = (job: Job) => `${job.id}/${job.finishedAt ?? ''}`
const isRunning = (job: Job) => job.status !== 'done' && job.status !== 'failed'

const TONE: Record<ExportLook['tone'], { ring: string; tint: string; note: string }> = {
  ok: { ring: 'var(--color-ok)', tint: 'var(--kit-ok-16)', note: 'var(--kit-warn-body)' },
  warn: { ring: 'var(--color-warn)', tint: 'var(--kit-warn-12)', note: 'var(--kit-warn-body)' },
  err: { ring: 'var(--job-failed)', tint: 'var(--kit-errf-12)', note: 'var(--kit-err-text)' },
}

const ACTIONS: Record<ExportActionId, { icon: string; label: string; primary?: boolean }> = {
  download: { icon: 'document-download', label: 'Tải xuống', primary: true },
  share: { icon: 'link-2', label: x.share },
  openFile: { icon: 'music', label: x.viewFile },
  openExport: { icon: 'folder-open', label: 'Xem bản xuất' },
  retry: { icon: 'refresh-2', label: x.retry, primary: true },
  retryNoMusic: { icon: 'music', label: x.retryNoMusic },
}

/**
 * Thẻ việc ghép của một tập. Tự ẩn khi không có việc, khi đã đóng, và tự đăng ký
 * khoảng cách toast theo artboard (`236px` khi chạy/tải · `300px` khi có nút).
 */
export function ExportJobCard({ project, story, volume, onViewFiles }: {
  project: string; story: string; volume: number
  /** “Xem tệp” — mở sheet Ghép & xuất ở view “Đã xuất”. */
  onViewFiles: () => void
}) {
  const notify = useMobileToast()
  const [, bump] = useReducer((value: number) => value + 1, 0)
  const select = useCallback((state: JobsState) => state.jobs.find((job) =>
    job.project === project && job.story === story && isAudioExportJob(job)
    && job.id === `${project}/${story}/audio/${volume}/export`), [project, story, volume])
  const job = useJobSelection(select)
  const look = job ? exportLook(job, true) : null
  const open = !!job && !!look && !dismissed.has(cardKey(job))
  useToastBottom(open ? (look!.phase === 'run' || look!.phase === 'dl' ? 246 : 310) : null, 1)

  const close = useCallback((target: Job) => {
    dismissed.add(cardKey(target))
    bump()
    if (!isRunning(target)) return
    void cancelJob(target).catch((error: unknown) => notify(errText(error)))
  }, [notify])

  if (!job || !look || !open) return null
  const tone = TONE[look.tone]
  const pct = look.phase === 'dl' ? look.bar : job.progress === null ? null : Math.round(job.progress * 100)
  const value = Math.max(0, Math.min(100, pct ?? (look.phase === 'run' ? 0 : 100)))
  const act = (id: ExportActionId) => {
    if (id === 'openFile' || id === 'openExport') { dismissed.add(cardKey(job)); bump(); onViewFiles(); return }
    if (id === 'download') { void downloadExportManual(job).catch((error: unknown) => notify(errText(error))); return }
    if (id === 'share') { void shareExportJob(job).catch((error: unknown) => notify(errText(error))); return }
    void retryExport(job, id === 'retryNoMusic').catch((error: unknown) => notify(errText(error)))
  }

  return <div className="mjob" role="status" aria-live="polite" style={{ background: `radial-gradient(120% 90% at 0% 0%, ${tone.tint}, transparent 60%), var(--glass-bg)` }}>
    <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 12, padding: '12px 10px 12px 12px' }}>
      <ProgressRing size={40} stroke={7} percent={value} color={tone.ring} track="var(--line-3)">
        {look.phase === 'run' || look.phase === 'dl'
          ? <span className="num" style={{ position: 'relative', fontSize: 11.5, fontWeight: 600 }}>{pct === null ? '' : `${Math.round(value)}%`}</span>
          : look.tone === 'err'
            ? <Icon name="warning-2" size={17} style={{ position: 'relative', color: tone.ring }} />
            : look.tone === 'warn'
              ? <Icon name="info-circle" size={17} style={{ position: 'relative', color: tone.ring }} />
              : look.phase === 'downloaded' || look.phase === 'manual'
                ? <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true" style={{ position: 'relative' }}><path d="M6 12.5l4 4 8-9" stroke="var(--color-ok)" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round" /></svg>
                : <Icon name="document-download" size={17} style={{ position: 'relative', color: tone.ring }} />}
      </ProgressRing>
      <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 }}>
        <span style={{ fontSize: 15, fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }} title={look.title}>{look.title}</span>
        <span className="num" style={{ fontSize: 12.5, color: 'var(--color-fg-3)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }} title={look.sub}>{look.sub}</span>
      </span>
      <button className="tap" type="button" onClick={() => close(job)} aria-label={isRunning(job) ? x.cancelMerge : x.dismissCard}
        style={{ width: 44, height: 44, border: 'none', borderRadius: 12, background: 'transparent', color: 'var(--color-fg-4)', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto' }}>
        <Icon name="add" size={20} style={{ transform: 'rotate(45deg)' }} />
      </button>
    </div>
    {look.note && <p style={{ position: 'relative', margin: '-4px 14px 10px 64px', fontSize: 13, lineHeight: 1.45, color: tone.note }}>{look.note}</p>}
    {look.actions.length > 0 && <div style={{ position: 'relative', display: 'grid', gridTemplateColumns: `repeat(${Math.min(2, look.actions.length)}, minmax(0, 1fr))`, gap: 8, padding: '0 12px 12px' }}>
      {look.actions.map((id) => {
        const action = ACTIONS[id]
        return <button key={id} className={`mjbtn${action.primary ? ' primary' : ''}`} type="button" onClick={() => act(id)}>
          <Icon name={action.icon} size={17} />{action.label}
        </button>
      })}
    </div>}
    <span aria-hidden="true" style={{ position: 'absolute', left: 14, right: 14, bottom: 0, height: 2, background: 'var(--line-1)' }}>
      <span style={{ display: 'block', height: '100%', width: `${value}%`, background: tone.ring, transition: 'width .3s linear' }} />
    </span>
  </div>
}
