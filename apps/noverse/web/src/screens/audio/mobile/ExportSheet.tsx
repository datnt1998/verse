import { useCallback, useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { ApiError, audioExportFileURL, createAudioExport, downloadExportFile, downloadJobFile, getAudioExportState, getAudioVolume } from '../../../api/client'
import type { AudioQueueJob, ExportFile } from '../../../api/types'
import { keys, useAction, useConnection, useResource } from '../../../data'
import { invalidate } from '@noverse/ui/data'
import { BottomSheet, useMobileToast } from '../../../mobile/primitives'
import { useJobSelection } from '../../../shell/jobs/useJobs'
import { exportData, exportStartError, registerExportWatch, useAutoDownload } from '../../../shell/jobs/exportFlow'
import { Icon, Switch, Segmented } from '@noverse/ui'
import { formatBytes, formatDuration } from '../logic'
import { errText } from '../useAudio'
import { pendingNotice } from './logic'
import type { JobsState } from '../../../shell/jobs/store'
import { t, audioExportCopy as x } from '../../../i18n/vi'
import { usePlayer } from '../useAudio'
import { useExportForm } from '../useExportForm'
import { audioExportOptions } from '../export'
import { exportStyles as sx } from '../export.stylex'
import { useStoryShortTitle } from '../../../shell/useNames'
import { audioChapterFileName } from '../../../lib/audioName'

export function ExportSheet({ project, story, volume, open, onClose, readyChapters, total, queue, writable, initialDone = false, initialView, trialChapter, openDone = 0 }: {
  project: string; story: string; volume: number; open: boolean; onClose: () => void
  readyChapters: number[]; total: number; queue: readonly AudioQueueJob[]; writable: boolean; initialDone?: boolean; initialView?: string | null; trialChapter: number
  /** > 0 = mở thẳng view “Đã xuất” (thẻ nổi “Xem tệp”); đổi số mỗi lần yêu cầu. */
  openDone?: number
}) {
  const ready = readyChapters.length
  const notify = useMobileToast()
  const connection = useConnection()
  const selectJob = useCallback((state: JobsState) => state.jobs.find((item) => item.project === project && item.story === story && item.id === `${project}/${story}/audio/${volume}/export`), [project, story, volume])
  const job = useJobSelection(selectJob)
  const exported = useResource(keys.story(project, story, 'audio', 'exports', volume), ({ signal }) => getAudioExportState(project, story, volume, signal))
  const volumeData = useResource(keys.story(project, story, 'audio', 'volume', volume), ({ signal }) => getAudioVolume(project, story, volume, signal))
  const shared = useExportForm(project, story, open, { split: initialView === 'exportCh' || initialView === 'exportChDone', chapters: [trialChapter] })
  const { form, update, visible, setForm } = shared
  useEffect(() => {
    if (open) setForm((previous) => ({ ...previous, split: initialView === 'exportCh' || initialView === 'exportChDone', chapters: [trialChapter] }))
  }, [open, trialChapter, initialView, setForm])
  const format = form.format
  const [expanded, setExpanded] = useState(false)
  const selected = (form.chapters ?? []).filter((chapter) => readyChapters.includes(chapter))
  const selectedSeconds = (volumeData.data?.chapters ?? []).filter((chapter) => selected.includes(chapter.chapter)).reduce((sum, chapter) => sum + (chapter.durationSec ?? 0), 0)
  const selectedMB = Math.round(selectedSeconds / 60 * ({ mp3: 1.18, m4b: .95, wav: 10.1 })[format])
  const shortTitle = useStoryShortTitle(project, story)
  const [view, setView] = useState<'form' | 'done'>(initialDone || initialView === 'exportTrial' || initialView === 'exportChDone' ? 'done' : 'form')
  const [trial, setTrial] = useState(initialView === 'exportTrial')
  const c = t.audio.generation
  const [postError, setPostError] = useState<unknown>(null)
  const [autoDl, setAutoDl] = useAutoDownload()
  // Thẻ nổi §AE: “Xem tệp” mở lại sheet ở view done (openDone = số lần được yêu cầu).
  useEffect(() => {
    if (!open) return
    setView(openDone > 0 || initialDone ? 'done' : 'form')
    if (openDone > 0) setTrial(false)
  }, [open, openDone, initialDone])
  useEffect(() => {
    if (job?.status === 'done') void exported.refresh()
  }, [job?.status, job?.finishedAt, exported.refresh])
  const download = useAction((path: string) => /[?]path=|^https?:/.test(path)
    ? downloadJobFile(path, decodeURIComponent(path.split('path=').at(-1)?.split('&')[0] ?? '') || 'gói.zip')
    : downloadExportFile(project, story, path), { success: 'Đã tải sách nói' })
  const share = useAction(async (path?: string) => {
    if (!navigator.share) throw new Error('Trình duyệt chưa hỗ trợ chia sẻ. Hãy dùng Tải xuống.')
    const choices = files.filter((item) => path ? item.path === path : /\.(m4b|mp3|wav|srt)$/i.test(item.name))
    if (!choices.length) throw new Error('Chưa có tệp sách nói để chia sẻ')
    const sharedFiles = await Promise.all(choices.map(async (item) => {
      const response = await fetch(item.url ?? audioExportFileURL(project, story, item.path))
      if (!response.ok) throw new Error('Không tải được tệp để chia sẻ')
      // A7 — tệp chia sẻ đúng tên người dùng thấy ({Tên ngắn} · Tập N · …), không slug đĩa.
      return new File([await response.blob()], item.name || item.path.split('/').at(-1) || 'sach-noi', { type: response.headers.get('content-type') ?? 'application/octet-stream' })
    }))
    if (!navigator.canShare?.({ files: sharedFiles })) throw new Error('Thiết bị chưa hỗ trợ chia sẻ tệp. Hãy dùng Tải xuống.')
    await navigator.share({ title: `Sách nói · Tập ${volume}`, files: sharedFiles })
  })
  /** §AE — đóng sheet ngay rồi mới gửi lệnh: ghép chạy nền, tiến độ ở thẻ nổi trên trình phát thu nhỏ. */
  const start = (whenReady: boolean, isTrial = false) => {
    setTrial(isTrial)
    setPostError(null)
    const scope = isTrial ? `bản thử chương ${trialChapter}`
      : form.split ? (selected.length === 1 ? `chương ${selected[0]}` : `${selected.length} chương`) : `Tập ${volume}`
    registerExportWatch(project, story, volume, {
      what: scope,
      verb: form.split && !isTrial ? 'xuất' : 'ghép',
      auto: autoDl,
      all: !isTrial && form.split === true && (form.pack ?? 'files') === 'files',
    })
    onClose()
    void (async () => {
      try {
        await shared.save()
        await createAudioExport(project, story, volume, audioExportOptions({ ...form, whenReady, ...(isTrial ? { chapters: [trialChapter], trial: true, split: false, intro: false, gap: false, chapterMarks: false } : form.split ? { chapters: selected, split: true } : !whenReady ? { chapters: readyChapters } : { chapters: undefined }) }))
        void invalidate(keys.story(project, story, 'audio'))
      } catch (cause) {
        const notice = exportStartError(cause instanceof ApiError ? cause : { message: errText(cause) })
        notify(notice?.text ?? errText(cause))
      }
    })()
  }
  const jobData = job ? exportData(job) : undefined
  const jobFiles = jobData?.files ?? []
  const files: ExportFile[] = jobFiles.length
    ? jobFiles.map((item) => ({
      path: item.path ?? '', name: item.name, format: /\.(m4b|mp3|wav)$/i.test(item.name) ? 'audio' : 'other',
      size: item.sizeBytes ?? 0, updatedAt: null, volume, durationSec: item.durationSec, chapter: item.chapter,
      sizeBytes: item.sizeBytes, url: item.url,
    }))
    : exported.data?.files ?? []
  // A7 — bản thử theo quy tắc tên mới: "{Tên ngắn} · Tập N · Chương NN · {tên} (bản thử).ext".
  const trialName = audioChapterFileName(shortTitle, volume, trialChapter, volumeData.data?.chapters.find((item) => item.chapter === trialChapter)?.title ?? '', format, true)
  const file = files.find((item) => trial ? item.name === trialName || / \(bản thử\)\.(m4b|mp3|wav)$/i.test(item.name) : /\.(m4b|mp3|wav)$/i.test(item.name))
  const duration = exported.data?.durationSec ?? 0
  const splitDone = !trial && (exported.data?.split ?? form.split)
  const chapterFiles = exported.data?.chapterFiles ?? files.filter((item) => item.format === 'audio' && /chuong/.test(item.path))
  const zipPath = jobData?.zipUrl ?? exported.data?.zipUrl
  const trialPlayer = usePlayer(open && trial && view === 'done' && file ? audioExportFileURL(project, story, file.path) : null)
  const busy = download.busy || share.busy || shared.loading || !!shared.error
  const canWrite = writable && connection.writable
  return <BottomSheet open={open} onClose={onClose} height="auto" style={{ maxHeight: 'calc(100dvh - 44px)', overflowY: 'auto' }} title={view === 'form' ? form.split ? x.title(volume) : `Ghép & xuất Tập ${volume}` : undefined}>
    <div className={`ma-export${view === 'form' ? '' : ' bare'} ${stylex.props(sx.sheet).className}`}>
      {view === 'form' && <>
        <Segmented label={x.scope} value={form.split ? 'chapters' : 'whole'} onChange={(value) => update('split', value === 'chapters')} columns={2} size={50} radius={11} style={{ display: 'grid', gridTemplateColumns: 'repeat(2,minmax(0,1fr))', gap: 4, padding: 4, borderRadius: 14, background: 'var(--color-surface-0)', border: 0 }} options={[{ id: 'whole', label: <span>{x.whole}<small style={{ display: 'block', fontSize: 11.5, color: 'var(--kit-fg-icon)' }}>{x.wholeShort}</small></span> }, { id: 'chapters', label: <span>{x.each}<small style={{ display: 'block', fontSize: 11.5, color: 'var(--kit-fg-icon)' }}>{x.splitShort}</small></span> }]} />
        {form.split && <>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}><span style={{ flex: 1, fontSize: 13, color: 'var(--color-fg-3)' }}>{x.select(selected.length, ready)}</span><button className="gchip" onClick={() => update('chapters', selected.length === ready ? [] : readyChapters)}>{selected.length === ready ? x.clear : x.all(ready)}</button></div>
          <div {...stylex.props(sx.grid, sx.mobileGrid)} role="group" aria-label={x.chapters}>{readyChapters.map((chapter) => <button key={chapter} {...stylex.props(sx.chapter, sx.mobileChapter, selected.includes(chapter) && sx.selected)} type="button" role="checkbox" aria-checked={selected.includes(chapter)} onClick={() => update('chapters', selected.includes(chapter) ? selected.filter((n) => n !== chapter) : [...selected, chapter].sort((a, b) => a - b))}>{chapter}</button>)}</div>
        </>}
        <Segmented label={x.format} value={format} onChange={(value) => update('format', value as typeof format)} columns={3} size={54} radius={11} thumbStyle={{ boxShadow: 'none' }} style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 4, padding: 4, borderRadius: 14, background: 'var(--color-surface-0)', border: 'none' }} options={(['mp3', 'm4b', 'wav'] as const).map((value) => ({ id: value, label: <span style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', lineHeight: 1.3 }}><span style={{ fontSize: 15, fontWeight: 600 }}>{value.toUpperCase()}</span><span style={{ fontSize: 11, color: 'var(--kit-fg-icon)' }}>{x.formatShort[value]}</span></span> }))} />
        <div className="ma-options">
          <button type="button" aria-expanded={expanded} onClick={() => setExpanded(!expanded)} style={{ width: '100%', display: 'flex', alignItems: 'center', textAlign: 'left', color: 'var(--color-fg)', background: 'transparent', border: 0, minHeight: 62, padding: '0 16px', fontSize: 14.5 }}><span style={{ flex: 1 }}>{x.audio}<small style={{ display: 'block', fontSize: 12, color: 'var(--color-fg-3)' }}>{x.enabled([...(visible.intro ? [form.intro] : []), form.chapterTitles, form.loudness, ...(visible.gap ? [form.gap] : [])].filter(Boolean).length, form.split ? 2 : 4, !!form.split)}</small></span><Icon name="arrow-down-2" size={16} style={{ transform: expanded ? 'rotate(180deg)' : undefined }} /></button>
          {expanded && <>
            {visible.intro && <Option title={x.intro} note={x.introHint} checked={form.intro} onChange={(value) => update('intro', value)} />}
            <Option title={x.titles} note={x.titlesHint} checked={form.chapterTitles} onChange={(value) => update('chapterTitles', value)} />
            <Option title={x.loudness} note={x.loudnessHint} checked={form.loudness} onChange={(value) => update('loudness', value)} />
            {visible.gap && <Option title={x.gap} note={x.gapHint} checked={form.gap} onChange={(value) => update('gap', value)} />}
          </>}
          {visible.chapterMarks && <Option title={x.marks} note={x.marksHint} checked={form.chapterMarks} onChange={(value) => update('chapterMarks', value)} />}
          <Option title={x.srt} note={x.srtShort} checked={form.subtitles} onChange={(value) => update('subtitles', value)} />
          <Option title={x.autoDownloadShort} note={x.autoDownloadHint} checked={autoDl} onChange={setAutoDl} />
        </div>
        {visible.pack && <Segmented label={x.receive} value={form.pack ?? 'files'} onChange={(value) => update('pack', value as 'files' | 'zip')} columns={2} size={44} options={[{ id: 'files', label: x.separate }, { id: 'zip', label: x.zip }]} />}
        {form.split && <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)', textAlign: 'center' }}>{selected.length ? x.summary(selected.length, Math.floor(selectedSeconds / 60), selectedMB, form.subtitles) : x.selectMinimum}</span>}
        {!form.split && ready < total && <div className="ma-warning"><Icon name="info-circle" size={17} /><span>{pendingNotice(total - ready, ready, queue)}</span></div>}
        {!canWrite && <p className="ma-note">{writable ? 'Đang mất kết nối. Có thể nghe và tải tệp đã xuất.' : 'Bạn có quyền nghe và tải sách nói. Cần quyền biên tập để xuất.'}</p>}
        <div className="ma-export-actions" style={form.split || ready === total ? { gridTemplateColumns: 'minmax(0,1fr)' } : undefined}><button className={`m-button${form.split ? '' : ' secondary'}`} disabled={!canWrite || busy || (form.split ? selected.length === 0 : ready === 0)} aria-busy={shared.loading} onClick={() => start(false)}>{form.split ? x.export(selected) : `Xuất ${ready} chương`}</button>{!form.split && ready < total && <button className="m-button" disabled={!canWrite || busy || total === 0} onClick={() => start(true)}>Xuất khi xong</button>}</div>
        {!form.split && <button style={{height:44,border:0,background:'transparent',color:'var(--color-accent)',fontSize:14.5,display:'flex',alignItems:'center',justifyContent:'center',gap:8}} disabled={!canWrite||busy||!readyChapters.includes(trialChapter)} onClick={()=>start(false,true)}><Icon name="play" size={15}/>{c.trial} {trialChapter} {c.trialBefore}</button>}
      </>}
      {view === 'done' && <>
        {file ? <>
          <div className="ma-export-status done">
            <span className="ma-export-tick"><svg width="30" height="30" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M6 12.5l4 4 8-9" /></svg></span>
            <strong>{trial?`Bản thử chương ${trialChapter} ${c.trialReady}`:splitDone ? x.exported(chapterFiles.length, volume) : t.audio.listen.daXuatTap(volume)}</strong>
            {trial&&<p style={{fontSize:13.5,color:'var(--color-fg-3)',maxWidth:300,margin:0}}>{c.trialNote}</p>}
            <span title={file.name} style={{ maxWidth: '100%', whiteSpace: 'normal', overflowWrap: 'anywhere', fontSize: 14, color: 'var(--color-fg-3)' }}>{splitDone ? `${chapterFiles.length} tệp .${format}${files.some((item) => item.name.endsWith('.srt')) ? ' + .srt' : ''}` : `${file.name} · ${trial ? 1 : exported.data?.chapterCount ?? ready} chương · ${formatDuration(Math.round(duration / 60) * 60)} · ${formatBytes(file.size)}${files.some((item) => item.name.endsWith('.srt')) ? ' + .srt' : ''}`}</span>
          </div>
          {splitDone && <div className="ma-options" style={{ maxHeight: 230, overflowY: 'auto' }}>{chapterFiles.map((item) => <div key={item.path} style={{ minHeight: 56, paddingRight: 8 }}><Icon name="music" size={18} style={{ color: 'var(--color-ok)', flex: '0 0 auto' }} /><span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 }}><span title={item.name} style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', fontSize: 14 }}>{item.name}</span><small style={{ fontSize: 12 }}>{formatDuration(item.durationSec ?? 0)} · {formatBytes(item.size)}</small></span><button type="button" aria-label={`${x.share} ${item.name}`} style={{ width: 38, height: 38, borderRadius: '50%', background: 'var(--line-2)', display: 'flex', alignItems: 'center', justifyContent: 'center' }} onClick={() => void share.run(item.path)}><Icon name="export-curve" size={16} /></button></div>)}</div>}
          <div className="ma-export-actions">
            {trial ? <>
              <button className="m-button secondary" onClick={()=>trialPlayer.toggle()}><Icon name={trialPlayer.playing?'pause':'play'} size={17}/>{c.listenTrial}</button>
              <button className="m-button" disabled={!canWrite||busy} onClick={()=>{ update('split', false); start(ready<total) }}>{c.fullExport}</button>
            </> : <>
              <button className="m-button secondary" disabled={share.busy} aria-busy={share.pending} onClick={() => void share.run()}><Icon name="link-2" size={17} />{splitDone ? x.shareAll : x.share}</button>
              <button className="m-button" disabled={download.busy} aria-busy={download.pending} onClick={() => { if (splitDone) { if (zipPath) void download.run(zipPath); else void (async () => { for (const item of files) await download.run(item.path) })() } else void (async () => { await download.run(file.path); for (const item of files.filter((item) => item.name.endsWith('.srt'))) await download.run(item.path) })() }}><Icon name="document-download" size={17} />{splitDone ? zipPath ? x.downloadZip : x.downloadAll(chapterFiles.length) : 'Tải xuống'}</button>
            </>}
          </div>
        </> : <div className="ma-export-status"><p>{exported.initialLoading ? 'Đang tải bản xuất…' : 'Chưa có sách nói đã xuất'}</p><button className="m-button" onClick={() => setView('form')}>Ghép & xuất</button></div>}
      </>}
      {Boolean(postError || exported.error) && <p className="m-error" role="alert">{errText(postError || exported.error)}</p>}
      {shared.error != null && <p className="m-error" role="alert">{errText(shared.error)}</p>}
    </div>
  </BottomSheet>
}

function Option({ title, note, checked, onChange }: { title: string; note: string; checked: boolean; onChange: (value: boolean) => void }) {
  return <div><span>{title}<small>{note}</small></span><Switch size={[51, 31]} checked={checked} label={title} onChange={onChange} /></div>
}
