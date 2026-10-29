import { useEffect, useRef, useState } from 'react'
import { confirmImport, getImport, previewImport, startImport } from '../../../api/client'
import type { ImportPreview, Job, JobEvent } from '../../../api/types'
import { keys, setData, subscribeEvents, useAction, useResource } from '../../../data'
import { jobError } from '../../../shell/jobs/exportFlow'
import { Icon } from '@noverse/ui'
import { RadioCard, Switch } from '@noverse/ui'
import { formatInt, formatSize } from '../logic'
import { defaultContinue, importCommand, importPath, importSteps, longChapterWarning, longChapters, otherImportWarnings, previewRows, splitSummary, stepCount } from './logic'
import { Actions, Button, Chk, CmdSheet, Failure, Label, Loading, Tick, useCmdToast } from './shared'
import { ProgressRing } from '@noverse/ui'
import { t } from '../../../i18n/vi'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

const GUIDE_SUGGESTIONS = ['Chương 13 thực ra là hai chương', 'Tách ở dòng ***', 'Bỏ phần lời tựa đầu tệp']

export function Import({ project, story, command, job, event: detailEvent, onJob, onBack, onRead }: { project: string; story: string; command: string; job?: Job; event: JobEvent | null; onJob: (id: string) => void; onBack: () => void; onRead: (chapter?: number) => void }) {
  const resume = useResource(keys.story(project, story, 'snapshot', 'import'), ({ signal }) => getImport(project, story, signal), { staleTime: 0, enabled: !job || job.status === 'waiting' || job.status === 'running' })
  const [path, setPath] = useState(() => importPath(command))
  const [previewPath, setPreviewPath] = useState(() => importPath(command))
  const [status, setStatus] = useState<'open' | 'closed'>(command.includes('--story=closed') ? 'closed' : 'open')
  const [continueAfter, setContinueAfter] = useState(() => defaultContinue(command))
  const requestedGuide = command.match(/(?:^|\s)--guide=(.*)$/)?.[1]?.trim() ?? ''
  const [guide, setGuide] = useState(requestedGuide)
  const [appliedGuide, setAppliedGuide] = useState(requestedGuide)
  const [guideOpen, setGuideOpen] = useState(false)
  const [all, setAll] = useState(false)
  const [started, setStarted] = useState(false)
  const initialized = useRef(false)
  const notify = useCmdToast()
  const previewKey = ['story', project, story, 'import', 'preview', previewPath, requestedGuide] as const
  const previewResource = useResource(previewKey, () => previewImport(project, story, previewPath, requestedGuide || undefined), { enabled: !!previewPath && !job, staleTime: Infinity })
  const event = detailEvent ?? (job ? resume.data?.job : null) ?? null
  const preview = previewResource.data ?? event?.import?.preview ?? resume.data?.preview
  const awaiting = !!event?.awaiting
  const running = started || !!job && (!awaiting || job.status === 'done' || job.status === 'failed')
  useEffect(() => subscribeEvents(['job.created', 'job.progress', 'job.waiting', 'job.failed', 'job.done'], () => { void resume.refresh() }), [resume.refresh])
  useEffect(() => {
    if (!job || !resume.data || initialized.current) return
    initialized.current = true
    setPath(resume.data.path || '')
    setStatus(resume.data.options?.storyStatus === 'closed' ? 'closed' : 'open')
    setContinueAfter(resume.data.options?.continueAfter ?? false)
    setGuide(resume.data.options?.guide ?? '')
    setAppliedGuide(resume.data.options?.guide ?? '')
  }, [job, resume.data])
  const split = useAction(async () => {
    setGuideOpen(false)
    const next = await previewImport(project, story, '', guide.trim())
    setData<ImportPreview>(previewKey, next)
    setAppliedGuide(guide.trim())
    setAll(false)
    notify(`Đã chia lại · ${next.chapters.length} chương`)
  }, { error: false })
  const accept = useAction(async () => {
    if (job && awaiting) await confirmImport(project, story, true, status, { continueAfter: status === 'open' && continueAfter, guide: appliedGuide })
    else {
      const result = await startImport(project, story, { path: previewPath || path, storyStatus: status, continueAfter: status === 'open' && continueAfter, guide: appliedGuide || undefined, autoConfirm: true })
      onJob(result.job)
    }
    setStarted(true)
    await resume.refresh()
  }, { error: false, invalidate: [keys.jobs(), keys.story(project, story), keys.project(project)] })
  const acceptRef = useRef(() => {})
  acceptRef.current = () => { if (preview && !accept.busy && !split.busy) void accept.run() }
  useEffect(() => {
    if (running) return
    const key = (event: KeyboardEvent) => { if (event.key.toLowerCase() === 'y' && !event.metaKey && !event.ctrlKey && !event.altKey && !(event.target as HTMLElement)?.closest('input,textarea,select,[contenteditable]')) { event.preventDefault(); acceptRef.current() } }
    window.addEventListener('keydown', key)
    return () => window.removeEventListener('keydown', key)
  }, [running])
  if (running) return <ImportRun job={job} event={event} chapters={preview?.chapters.length ?? event?.total ?? 0} continueAfter={status === 'open' && continueAfter} onBack={onBack} onRead={onRead} />
  const average = preview ? preview.chapters.reduce((sum, chapter) => sum + chapter.words, 0) / Math.max(1, preview.chapters.length) : 0
  const commandLine = importCommand(previewPath || path || resume.data?.path || '', status, continueAfter, appliedGuide)
  const longRows = preview ? longChapters(preview.chapters) : []
  const rows = preview ? previewRows(preview.chapters, all) : []
  return <>
    <div className="mc-scroll mc-import">
      <PanelRefreshBar resourceKeys={[resume.key, previewResource.key]} />
      <Failure error={previewResource.error || resume.error} onRetry={() => { void previewResource.refresh(); void resume.refresh() }} />
      {!preview && (previewPath || job) && !previewResource.error && !resume.error ? <Loading text="Đang xem trước cách chia chương…" />
        : !preview ? <div className="mc-inset"><Label>TỆP TRUYỆN TRÊN MÁY CHỦ</Label><input className="kit-frame mc-field" value={path} onChange={(event) => setPath(event.target.value)} placeholder="Đường dẫn tệp .txt" aria-label="Đường dẫn tệp truyện" /><Button primary disabled={!path.trim()} onClick={() => setPreviewPath(path.trim())}>Xem trước cách chia</Button><p className="mc-note">Tệp phải nằm trong thư viện được phép trên máy chủ.</p></div>
        : <>
          <div className="mc-file"><span><Icon name="document-text" size={21} /></span><div><strong>{preview.filename || (previewPath || path).split('/').pop()}</strong><small>{preview.size === undefined ? 'Chưa rõ dung lượng' : formatSize(preview.size)} · {formatInt(preview.words ?? preview.chapters.reduce((sum, chapter) => sum + chapter.words, 0))} chữ</small></div></div>
          <div className="mc-import-count">
            {split.busy ? <span className="mc-resplitting"><span className="mc-spinner" aria-hidden="true" />Đang chia lại…</span> : <span className="mc-count mc-rise">{preview.chapters.length} chương</span>}
            <span>{splitSummary(preview, !!appliedGuide)}</span>
          </div>
          {!split.busy && longRows.map((chapter) => <div key={`long-${chapter.n}`} className="mc-warning"><Icon name="warning-2" size={18} className="mc-warning-icon" /><span>{longChapterWarning(chapter)} Bấm <b>Chia lại…</b> để gợi ý cách tách.</span></div>)}
          {otherImportWarnings(preview).map((warning, index) => <div key={`warn-${index}`} className="mc-warning"><Icon name="warning-2" size={18} className="mc-warning-icon" /><span>{warning}</span></div>)}
          <Label>TRẠNG THÁI TRUYỆN</Label>
          <div role="radiogroup" aria-label="Trạng thái truyện" className="mc-radio-group">{(['open', 'closed'] as const).map((value) => <RadioCard key={value} size="lg" checked={status === value} disabled={accept.busy} onChange={() => setStatus(value)} label={<>{value === 'open' ? 'Còn tiếp' : 'Đã hoàn'} <code>--story={value}</code></>} description={value === 'open' ? 'Nhập rồi viết tiếp các chương sau' : 'Chỉ lưu để đọc, nghe và xuất'} />)}</div>
          {status === 'open' && <div className="mc-group mc-continue"><div className="mc-row"><span>Viết tiếp ngay sau khi nhập<small>Người viết bắt đầu chương {preview.chapters.length + 1} <code>--continue</code></small></span><Switch size={[51, 31]} checked={continueAfter} label="Viết tiếp ngay sau khi nhập" disabled={accept.busy} onChange={setContinueAfter} /></div></div>}
          <Label split><span>XEM TRƯỚC CÁCH CHIA</span><span className="mc-num">{preview.chapters.length}</span></Label>
          <div className="mc-group">
            {rows.map((chapter) => { const warn = !!chapter.warning || chapter.words >= average * 2; return <div key={chapter.n} className={`mc-row mc-import-chapter ${warn ? 'mc-long-chapter' : ''}`}><code>{chapter.n}</code><span><strong className={chapter.title ? '' : 'mc-untitled'}>{chapter.title || '(không có tiêu đề)'}</strong><small>{formatInt(chapter.words)} chữ{warn ? ' · dài gấp đôi' : ''}</small></span>{warn && <Icon name="warning-2" size={17} label="Dài bất thường" className="mc-long-icon" />}{chapter.newlySplit && <em>mới tách</em>}</div> })}
            <button type="button" className="mc-row mc-expand" onClick={() => setAll(!all)}>{all ? 'Thu gọn' : `Xem cả ${preview.chapters.length} chương`}</button>
          </div>
          <Label>LỆNH SẼ GỬI</Label>
          <div className="mc-command-line">{commandLine}</div>
        </>}
      <Failure error={accept.error || split.error} />
    </div>
    <Actions>
      <Button className="mc-auto" disabled={!preview || accept.busy || split.busy} onClick={() => setGuideOpen(true)}><Icon name="scissor" size={17} />Chia lại…</Button>
      <Button primary disabled={!preview || accept.busy || split.busy} onClick={() => void accept.run()}>{accept.busy ? 'Đang bắt đầu…' : <>Đồng ý <code>(y)</code></>}</Button>
    </Actions>
    <CmdSheet open={guideOpen} onClose={() => setGuideOpen(false)} title="Chia lại theo gợi ý" lead="Mô tả bằng lời cách tệp đánh dấu chương. Noverse sẽ đọc lại và đưa bạn xem trước.">
      <textarea className="kit-frame mc-textarea" aria-label="Gợi ý cách chia" value={guide} onChange={(event) => setGuide(event.target.value)} placeholder="Ví dụ: chương 13 thực ra là hai chương, tách ở dòng ***" />
      <div className="mc-wrap">{GUIDE_SUGGESTIONS.map((text) => <button key={text} type="button" className="mc-chip" onClick={() => setGuide((value) => value.trim() ? `${value.trim()}; ${text.toLowerCase()}` : text)}>{text}</button>)}</div>
      <div className="mc-sheet-info"><Icon name="info-circle" size={15} /><span>Gợi ý được gửi kèm <code>--guide=</code> ở cuối lệnh nên có thể chứa dấu cách.</span></div>
      <div className="mc-two-buttons"><Button onClick={() => setGuideOpen(false)}>Huỷ</Button><Button primary disabled={!guide.trim() || split.busy} onClick={() => void split.run()}><Icon name="scissor" size={17} />Chia lại</Button></div>
    </CmdSheet>
  </>
}

function ImportRun({ job, event, chapters, continueAfter, onBack, onRead }: { job?: Job; event: JobEvent | null; chapters: number; continueAfter: boolean; onBack: () => void; onRead: (chapter?: number) => void }) {
  const done = job?.status === 'done' || !!event?.done && !event.error
  const error = (job ? jobError(job) : '') || event?.error || ''
  const detail = event?.import
  const steps = detail?.steps ?? importSteps(event)
  const summary = detail?.summary
  const percent = done ? 100 : job?.progress == null ? null : Math.round(job.progress * 100)
  const eta = job?.etaSec ?? detail?.etaSec
  const doneSub = summary ? `${summary.chapters} chương · ${summary.characters} nhân vật · ${summary.foreshadows} phục bút` : job?.result?.sub || `${chapters} chương`
  return <>
    <div className="mc-scroll mc-import-run">
      {done ? <>
        <span className="mc-done-circle mc-rise"><Tick size={44} /></span>
        <div className="mc-center mc-rise"><h2>Đã nhập xong</h2><p className="mc-note">{doneSub}{(detail?.continued ?? continueAfter) && chapters ? ` · đang viết chương ${chapters + 1}` : ''}</p></div>
      </> : <>
        <span className="mc-big-ring"><ProgressRing className="mc-ringm" size={148} stroke={7} percent={percent ?? 0} indeterminate={percent === null && !error}
          gradient={['var(--ring-import-start)', 'var(--ring-voice-start)']} track="var(--line-2)"><strong>{percent === null ? '…' : `${percent}%`}</strong><small>{eta ? t.audio.logic.conPh(Math.max(1, Math.round(eta / 60))) : 'Đang xử lý'}</small></ProgressRing></span>
        <div className="mc-center"><h2>{error ? 'Việc nhập đã dừng' : `Đang nhập ${chapters || ''} chương`}</h2><p className="mc-note">Kiến trúc sư đọc từng chương để dựng lại truyện</p></div>
      </>}
      <Failure error={error ? new Error(error) : null} />
      <ol className="mc-import-steps">{steps.map((step, index) => <li key={step.id} className={step.status === 'pending' ? 'mc-todo' : ''}><Chk state={step.status === 'done' ? 'done' : step.status === 'running' ? 'now' : 'todo'} /><span>{step.title}</span><code>{stepCount(step, index, chapters)}</code></li>)}</ol>
      {!done && <p className="mc-note mc-center mc-run-hint">Có thể rời màn này — việc nhập chạy nền và báo khi xong.</p>}
    </div>
    <Actions>{done ? <Button primary onClick={() => onRead(1)}><Icon name="book-1" size={18} />Mở truyện</Button> : <Button onClick={onBack}>Chạy nền</Button>}</Actions>
  </>
}
