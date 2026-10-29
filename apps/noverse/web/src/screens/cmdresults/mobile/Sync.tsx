import { useEffect, useRef, useState } from 'react'
import { checkSync, getSyncDiff, runSync, sendSyncFix } from '../../../api/client'
import type { Job, SyncResult } from '../../../api/types'
import { invalidate, keys, useAction, useResource } from '../../../data'
import { jobError } from '../../../shell/jobs/exportFlow'
import { Icon } from '@noverse/ui'
import { formatEditedAt, formatWordDelta, paragraphDiff, syncSummary } from '../logic'
import { changedLines } from './logic'
import { Actions, Button, Failure, Loading, Tick, useCmdToast } from './shared'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

function Difference({ project, story, chapter }: { project: string; story: string; chapter: number }) {
  const resource = useResource(['story', project, story, 'sync', 'diff', chapter], ({ signal }) => getSyncDiff(project, story, chapter, signal), { staleTime: 0 })
  return <div className="mc-diff"><Failure error={resource.error} onRetry={() => void resource.refresh()} />{!resource.data && !resource.error && <span className="mc-note">Đang tải khác biệt…</span>}{resource.data && changedLines(paragraphDiff(resource.data.before, resource.data.after)).map((line, index) => <div key={index} className={`mc-diff-line mc-diff-${line.kind}`}>{line.text}</div>)}</div>
}
export function Sync({ project, story, job, command, onJob, onRead, onBack, onWrite }: { project: string; story: string; job?: Job; command: string; onJob: (id: string) => void; onRead: (chapter?: number) => void; onBack: () => void; onWrite: () => void }) {
  const autoStart = !command.includes('--check')
  const launched = useRef(false)
  const resource = useResource(['story', project, story, 'sync', 'check'], () => checkSync(project, story), { enabled: !job && !autoStart, staleTime: 0 })
  const [open, setOpen] = useState<Set<number>>(new Set())
  const [sent, setSent] = useState<Set<string>>(new Set())
  const [sending, setSending] = useState('')
  const notify = useCmdToast()
  const changed = resource.data?.changed ?? []
  const result = job?.result?.data as SyncResult | undefined
  const done = job?.status === 'done'
  const failed = job?.status === 'failed'
  const apply = useAction(async () => { const result = await runSync(project, story); onJob(result.job) }, { error: false })
  const fix = useAction(async (chapter: number, issueIndex: number, target?: number) => {
    const key = `${chapter}:${issueIndex}`
    setSending(key)
    try { await sendSyncFix(project, story, chapter, issueIndex, job?.id.slice(`${project}/${story}/`.length)); setSent((previous) => new Set(previous).add(key)); notify(target ? `Đã nhờ Người viết sửa chương ${target}` : 'Đã nhờ Người viết sửa mâu thuẫn', onWrite); await invalidate(keys.story(project, story, 'snapshot')) } finally { setSending('') }
  }, { error: false })
  const summary = result ? syncSummary(result.analyses) : null
  useEffect(() => {
    if (autoStart && !job && !launched.current) { launched.current = true; void apply.run() }
  }, [autoStart, job, apply.run])
  if (autoStart && !job) return <><div className="mc-scroll">{apply.error ? <Failure error={apply.error} onRetry={() => void apply.run()} /> : <Loading text="Đang tiếp nhận các chương sửa ngoài…" />}</div><Actions><Button onClick={onBack}>{apply.error ? 'Quay lại' : 'Chạy nền'}</Button></Actions></>
  return <>
    <div className="mc-scroll mc-sync">
      <PanelRefreshBar resourceKeys={[resource.key]} />
      <Failure error={resource.error || apply.error || (failed ? new Error((job && jobError(job)) || 'Đồng bộ chưa hoàn tất. Phần sửa bên ngoài vẫn được giữ.') : null)} onRetry={!job ? () => void resource.refresh() : undefined} />
      {!job && !resource.data && !resource.error ? <Loading text="Đang kiểm tra các chương sửa ngoài…" />
        : !job ? <>
          <div className="mc-intro"><h2>{changed.length} chương đã được sửa bên ngoài Noverse</h2><p>Chế độ kiểm tra <code>--check</code> — chưa tiếp nhận gì. Chạm một chương để xem khác biệt.</p></div>
          <div className="mc-group">{changed.map((chapter, index) => { const expanded = open.has(chapter.chapter) || index === 0 && open.size === 0; return <section key={chapter.chapter} className="mc-sync-chapter">
            <button type="button" className="mc-row" aria-expanded={expanded} onClick={() => setOpen((previous) => { const next = new Set(previous); if (expanded) next.delete(chapter.chapter); else next.add(chapter.chapter); if (!next.size && expanded) next.add(-1); return next })}>
              <span className="mc-chtag">Chương {chapter.chapter}</span>
              <span><strong>{chapter.title}</strong><small><span className="mc-good-text">{formatWordDelta(chapter.added, '+')}</span> <span className="mc-del-count">{formatWordDelta(chapter.removed, '-')}</span> · {formatEditedAt(chapter.modifiedAt)}</small></span>
              <Icon name="arrow-down-2" size={16} className={`mc-chev ${expanded ? 'mc-up' : ''}`} />
            </button>
            {expanded && <Difference project={project} story={story} chapter={chapter.chapter} />}
          </section> })}
          {changed.length === 0 && <p className="mc-note mc-inset">Không có chương nào sửa bên ngoài cần tiếp nhận.</p>}</div>
          <div className="mc-sync-note"><Icon name="info-circle" size={16} /><span>Khi tiếp nhận, Noverse đọc lại các chương này, cập nhật tóm tắt và báo nếu chỗ sửa mâu thuẫn với chương phía sau.</span></div>
        </>
        : !done ? <div className="mc-run-status"><span className="mc-spinner" /><h2>{failed ? 'Đồng bộ đã dừng' : 'Đang tiếp nhận chương'}</h2><p>{job.step || job.title}</p>{job.progress !== null && <progress max={1} value={job.progress} />}<p className="mc-note">Có thể rời màn này — việc đồng bộ vẫn chạy nền.</p></div>
        : <>
          <div className="mc-sync-complete mc-rise"><span><Tick size={26} /></span><div><h2>Đã tiếp nhận {result?.applied.length ?? 0} chương</h2><p>Tóm tắt đã cập nhật{summary?.issues ? ` · ${summary.issues} chỗ cần bạn xem` : ''}</p></div></div>
          <div className="mc-applied">{result?.analyses.map((analysis, index) => <section className={`mc-applied-card mc-rise ${analysis.downstreamIssues.length ? 'mc-warning-border' : ''}`} key={analysis.chapter} style={{ animationDelay: `${(index * 0.06).toFixed(2)}s` }}>
            <div className="mc-applied-title"><span className="mc-chtag">Chương {analysis.chapter}</span><span>{analysis.changeSummary}</span></div>
            <div className="mc-sync-chips">
              <span className={analysis.storyChanged ? 'mc-chip-story' : ''}><Icon name={analysis.storyChanged ? 'flag' : 'tick-circle'} size={12} />{analysis.storyChanged ? 'Sự thật cốt truyện cập nhật' : 'Chỉ đổi lời văn'}</span>
              {analysis.downstreamIssues.length > 0 && <span className="mc-chip-conflict"><Icon name="warning-2" size={12} />{analysis.downstreamIssues.length} mâu thuẫn phía sau</span>}
            </div>
            {analysis.downstreamIssues.map((issue, issueAt) => { const key = `${analysis.chapter}:${issueAt}`; const sentAlready = issue.sent || sent.has(key); return <div className="mc-conflict" key={issueAt}><p>{issue.text}</p><div className="mc-two-buttons">{issue.chapter && <Button onClick={() => onRead(issue.chapter!)}><Icon name="book-1" size={16} />Mở chương {issue.chapter}</Button>}<Button primary={!sentAlready} className={sentAlready ? 'mc-sent' : ''} disabled={sentAlready || fix.busy} onClick={() => void fix.run(analysis.chapter, issue.issueIndex ?? issueAt, issue.chapter ?? undefined)}><Icon name={sentAlready ? 'tick-circle' : 'magic-star'} size={16} />{sentAlready ? 'Đã gửi' : sending === key ? 'Đang gửi…' : 'Nhờ sửa'}</Button></div></div> })}
          </section>)}</div>
          <Failure error={fix.error} />
        </>}
    </div>
    <Actions>{!job ? <><Button className="mc-auto mc-later" onClick={onBack}>Để sau</Button><Button primary disabled={!changed.length || apply.busy || !!resource.error} onClick={() => void apply.run()}>{apply.busy ? 'Đang tiếp nhận…' : <><Icon name="tick-circle" size={18} />Tiếp nhận {changed.length} chương</>}</Button></> : <Button primary={done} onClick={onBack}>{done ? 'Xong' : 'Chạy nền'}</Button>}</Actions>
  </>
}
