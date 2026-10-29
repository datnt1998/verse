import { useEffect, useState } from 'react'
import { getJob } from '../../../api/client'
import { subscribeEvents, useResource } from '../../../data'
import { Icon } from '@noverse/ui'
import { Cocreate } from './Cocreate'
import { Diag } from './Diag'
import { Import } from './Import'
import { Reopen } from './Reopen'
import { Sync } from './Sync'
import { qualifyJobId, resultKind } from './logic'
import { CmdToastProvider, Failure, Loading } from './shared'
import './mobile.css'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

export type MobileCommandResultsProps = {
  project: string; story: string; job?: string; command?: string
  onBack: () => void; onGuide?: (command: string) => void
  onRead?: (chapter?: number) => void; onWrite?: () => void
  onJob?: (id: string) => void
}
const TITLES = { cocreate: 'Bàn hướng đi với Kiến trúc sư', import: 'Nhập truyện có sẵn', diag: 'Chẩn đoán truyện', sync: 'Đồng bộ chương sửa ngoài', reopen: 'Mở lại truyện đã hoàn' }

/** Push page, shared by M1 launches and /cmd/:job; the shell owns navigation only. */
export function MobileCommandResults({ project, story, job, command = '', onBack, onGuide, onRead = onBack, onWrite = onBack, onJob }: MobileCommandResultsProps) {
  const [activeJob, setActiveJob] = useState(job && job !== 'new' ? job : '')
  useEffect(() => setActiveJob(job && job !== 'new' ? job : ''), [job])
  const detail = useResource(['job', activeJob], ({ signal }) => getJob(activeJob, signal), { enabled: !!activeJob, staleTime: 0 })
  useEffect(() => subscribeEvents(['job.created', 'job.progress', 'job.waiting', 'job.failed', 'job.done'], () => { if (activeJob) void detail.refresh() }), [activeJob, detail.refresh])
  const kind = resultKind(command, detail.data?.job.kind)
  const p = detail.data?.project || project
  const s = detail.data?.story || story
  const line = kind === 'sync' && detail.data?.job ? '/sync' : command ? (command.startsWith('/') ? command : `/${command}`) : kind ? `/${kind}` : '/…'
  const selectedJob = detail.data?.job
  const trackJob = (id: string) => {
    const fullId = qualifyJobId(p, s, id)
    setActiveJob(fullId)
    onJob?.(fullId)
  }
  return <CmdToastProvider><section className="mc-page" aria-label="Kết quả lệnh">
    <div className="mc-safe-top" />
    <header className="mc-header">
      <button className="mc-icon-button" aria-label="Quay lại Viết" onClick={onBack}><Icon name="arrow-left-2" size={22} /></button>
      <div><code>{line}</code><h1>{kind ? TITLES[kind] : 'Kết quả lệnh'}</h1></div>
      <button className="mc-icon-button" aria-label="Hướng dẫn lệnh này" disabled={!kind || !onGuide} onClick={() => kind && onGuide?.(kind)}><Icon name="message-question" size={21} /></button>
    </header>
    <PanelRefreshBar resourceKeys={[detail.key]} />
    {detail.error ? <div className="mc-scroll"><Failure error={detail.error} onRetry={() => void detail.refresh()} /></div>
      : activeJob && !selectedJob ? <div className="mc-scroll"><Loading text="Đang mở kết quả việc nền…" /></div>
      : !kind ? <div className="mc-scroll"><Failure error={new Error('Lệnh này chưa có màn kết quả trên điện thoại.')} /></div>
      : kind === 'cocreate' ? <Cocreate project={p} story={s} onWrite={onWrite} />
      : kind === 'import' ? <Import project={p} story={s} command={line} job={selectedJob} event={detail.data?.event ?? null} onJob={trackJob} onBack={onBack} onRead={onRead} />
      : kind === 'diag' ? <Diag project={p} story={s} existing={!!activeJob} onWrite={onWrite} />
      : kind === 'sync' ? <Sync project={p} story={s} command={line} job={selectedJob} onJob={trackJob} onRead={onRead} onBack={onBack} onWrite={onWrite} />
      : <Reopen project={p} story={s} onRead={onRead} onWrite={onWrite} />}
  </section></CmdToastProvider>
}
