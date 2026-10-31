import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Dialog, Icon, Spinner, type IconName } from '@noverse/ui'
import { Toaster } from 'sonner'
import { executeCommands, getCommandExecutor, useCommandHistory } from '@editor/commands'
import { useProjectStore } from '@editor/features/projects/stores/project-store'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useSelectionStore } from '@editor/shared/state/selection'
import { buildTimelineFromStores } from '@editor/features/timeline/stores/timeline-persistence'
import { getProjectRevision, loadProject } from '@editor/storage/project-persistence'
import { getEditorStorage } from '@editor/storage/storage-backend'
import { mutate, request } from '../../api/request'
import { navigate, useRoute } from '../../router'
import { ShortcutProvider, useShortcutAction } from '../ui/shortcuts'
import { Shortcuts } from './Shortcuts'
import { useProjectSession } from '../ui/shell/use-project-session'
import { MediaMetadataSync } from '../ui/shell/media-sync'
import { captionSignature, canExport, fetchQuality, qualityRanges, rangeItem, type EditorQuality } from './quality'
import { FixPanel, rangeTime } from './FixPanel'
import { Preview } from './Preview'
import { Timeline } from './Timeline'
import { Inspector } from './Inspector'
import { ContentPanels } from './ContentPanels'
import { sx } from './editor.stylex'

type Panel = 'fix' | 'scenes' | 'subs' | 'bg' | 'audio' | 'media'
const panels: { id: Panel; label: string; icon: IconName }[] = [
  { id: 'fix', label: 'Cần sửa', icon: 'danger' }, { id: 'scenes', label: 'Cảnh', icon: 'gallery' },
  { id: 'subs', label: 'Phụ đề', icon: 'subtitle' }, { id: 'bg', label: 'Nền', icon: 'video-play' },
  { id: 'audio', label: 'Âm thanh', icon: 'volume' }, { id: 'media', label: 'Media', icon: 'folder' },
]
type TaskProgress = { phase?: string; framesDone?: number; framesTotal?: number; bytesSent?: number; bytesTotal?: number }
type TaskResult = { request: { id: string; status: string; progress?: TaskProgress; error?: string | { message?: string } }; export?: { id: string; size?: number }; score?: unknown }
type TaskAccepted = { requestId: string; stateUrl: string; request: TaskResult['request'] }

export function Editor({ projectId }: { projectId: string }) {
  return <ShortcutProvider><EditorBody key={projectId} projectId={projectId} /></ShortcutProvider>
}
function EditorBody({ projectId }: { projectId: string }) {
  const [aligning, setAligning] = useState(false)
  const [autosave, setAutosave] = useState(true)
  const session = useProjectSession(projectId, autosave && !aligning)
  const project = useProjectStore((state) => state.currentProject)
  const items = useItemsStore((state) => state.items)
  const tracks = useItemsStore((state) => state.tracks)
  const canUndo = useCommandHistory((state) => state.canUndo)
  const canRedo = useCommandHistory((state) => state.canRedo)
  const [panel, setPanel] = useState<Panel>('fix')
  const [quality, setQuality] = useState<EditorQuality>()
  const [error, setError] = useState<string>()
  const [selectedRange, setSelectedRange] = useState(0)
  const [alignedSnapshots, setAlignedSnapshots] = useState(new Map<string, string>())
  const [dialog, setDialog] = useState<'gate' | 'exporting' | 'done' | 'shortcuts' | null>(null)
  const [dismissedConflict, setDismissedConflict] = useState<number>()
  const [copying, setCopying] = useState(false)
  const [task, setTask] = useState<TaskAccepted>()
  const [taskResult, setTaskResult] = useState<TaskResult>()
  const [measureTask, setMeasureTask] = useState<TaskAccepted>()
  const { search } = useRoute()
  const openedAt = useRef(false)
  const fps = project?.metadata.fps ?? 30
  const ranges = useMemo(() => quality ? qualityRanges(quality) : [], [quality])
  const fixedIds = useMemo(() => new Set(ranges.filter((range) => alignedSnapshots.get(range.cueId ?? `${range.start}`) === captionSignature(range, items, fps) && alignedSnapshots.has(range.cueId ?? `${range.start}`)).map((range) => range.cueId ?? `${range.start}`)), [ranges, items, fps, alignedSnapshots])
  const visibleRanges = ranges.map((range) => fixedIds.has(range.cueId ?? `${range.start}`) ? { ...range, status: 'aligned' } : range)
  const left = ranges.filter((range) => range.status !== 'pass' && !fixedIds.has(range.cueId ?? `${range.start}`)).length
  const displayQuality = quality && (fixedIds.size || session.dirty) ? { ...quality, changedSinceMeasure: Math.max(1, quality.changedSinceMeasure) } : quality
  const passed = displayQuality ? canExport(displayQuality) : false
  const selectRange = useCallback((index: number) => {
    const range = ranges[index]
    if (!range) return
    setSelectedRange(index)
    const item = rangeItem(range, useItemsStore.getState().items, fps)
    useSelectionStore.getState().selectItems(item ? [item.id] : [])
    usePlaybackStore.getState().pause()
    usePlaybackStore.getState().setCurrentFrame(Math.round(range.start * fps))
  }, [ranges, fps])
  const refreshQuality = useCallback(async () => {
    try { setQuality(await fetchQuality(projectId)); setError(undefined) }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Chưa đọc được độ hoàn thiện') }
  }, [projectId])
  useEffect(() => {
    if (session.opening !== 'ready') return
    const controller = new AbortController()
    void fetchQuality(projectId, controller.signal).then(setQuality).catch((cause: unknown) => { if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : 'Chưa đọc được độ hoàn thiện') })
    return () => controller.abort()
  }, [session.opening, projectId])
  useEffect(() => {
    if (!ranges.length || openedAt.current || session.opening !== 'ready') return
    openedAt.current = true
    const requested = search.get('cue')
    const index = requested ? ranges.findIndex((range) => range.cueId === requested) : 0
    selectRange(Math.max(0, index)); setPanel('fix')
  }, [ranges, search, selectRange, session.opening])
  useEffect(() => {
    if (!measureTask) return
    let stopped = false
    let timer: number | undefined
    const update = async () => {
      try {
        const result = await request<TaskResult>(measureTask.stateUrl)
        if (stopped) return
        if (['done', 'completed', 'succeeded'].includes(result.request.status)) { setMeasureTask(undefined); setAlignedSnapshots(new Map()); await refreshQuality(); return }
        if (['failed', 'cancelled'].includes(result.request.status)) { setMeasureTask(undefined); setError(typeof result.request.error === 'string' ? result.request.error : result.request.error?.message ?? 'Đo lại chưa hoàn tất'); await refreshQuality(); return }
        timer = window.setTimeout(update, 2000)
      } catch (cause) { if (!stopped) setError(cause instanceof Error ? cause.message : 'Chưa đọc được tiến độ đo') }
    }
    void update()
    return () => { stopped = true; clearTimeout(timer) }
  }, [measureTask, refreshQuality])
  useEffect(() => {
    if (!task || dialog !== 'exporting') return
    let stopped = false
    let timer: number | undefined
    const update = async () => {
      try {
        const result = await request<TaskResult>(task.stateUrl)
        if (stopped) return
        setTaskResult(result)
        if (['done', 'completed', 'succeeded'].includes(result.request.status)) { setDialog('done'); return }
        if (['failed', 'cancelled'].includes(result.request.status)) { setDialog(null); setError(typeof result.request.error === 'string' ? result.request.error : result.request.error?.message ?? 'Xuất video chưa hoàn tất'); return }
        timer = window.setTimeout(update, 2000)
      } catch (cause) { if (!stopped) setError(cause instanceof Error ? cause.message : 'Chưa đọc được tiến độ xuất') }
    }
    void update()
    return () => { stopped = true; clearTimeout(timer) }
  }, [task, dialog])
  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => { if (session.dirty || session.saving || session.conflict) { event.preventDefault(); event.returnValue = '' } }
    window.addEventListener('beforeunload', beforeUnload)
    return () => window.removeEventListener('beforeunload', beforeUnload)
  }, [session.dirty, session.saving, session.conflict])
  useEffect(() => { if (project) document.title = `${project.name} · Editor` }, [project?.name])
  const undo = () => { getCommandExecutor().undo() }
  const redo = () => { getCommandExecutor().redo() }
  useShortcutAction({ id: 'undo', label: 'Hoàn tác', scope: 'global', enabled: canUndo && !aligning && dialog !== 'exporting', run: undo })
  useShortcutAction({ id: 'redo', label: 'Làm lại', scope: 'global', enabled: canRedo && !aligning && dialog !== 'exporting', run: redo })
  const align = async (all: boolean) => {
    if (!quality || aligning || !autosave || session.conflict) return
    setAligning(true); setError(undefined)
    try {
      if (!await session.save()) throw new Error('Hãy lưu thay đổi trước khi khớp phụ đề.')
      const targets = all ? ranges.filter((range) => !fixedIds.has(range.cueId ?? `${range.start}`)) : [ranges[selectedRange]].filter(Boolean)
      const cueIds = targets.map((range) => range.cueId).filter((id): id is string => !!id)
      if (cueIds.length !== targets.length) throw new Error('Dữ liệu đo chưa có mã đoạn phụ đề. Đo lại rồi thử lại.')
      await request(`/api/editor/${encodeURIComponent(projectId)}/captions/align`, { body: { cueIds: all ? 'all-drifting' : cueIds, revision: getProjectRevision(projectId) } })
      // The server persists the edit; adopt its CAS revision without clearing the engine's history.
      const aligned = await loadProject(projectId)
      if (!aligned?.timeline) throw new Error('Chưa đọc được bản phụ đề vừa khớp.')
      const result = executeCommands([{ type: 'project.patch', params: { patch: { items: aligned.timeline.items } } }])
      if (!result.ok) throw new Error(result.issues.map((issue) => issue.message).join(' '))
      const next = new Map(alignedSnapshots)
      for (const range of targets) next.set(range.cueId ?? `${range.start}`, captionSignature(range, useItemsStore.getState().items, fps))
      setAlignedSnapshots(next)
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Chưa khớp được theo giọng') }
    finally { setAligning(false) }
  }
  const measure = async () => {
    if (!quality?.jobId || measureTask) return
    try {
      if (!await session.save()) throw new Error('Hãy lưu thay đổi trước khi đo lại.')
      const accepted = await request<TaskAccepted>(`/api/clone/${encodeURIComponent(quality.jobId)}/measure`, { body: {} })
      setMeasureTask(accepted); setQuality({ ...quality, measuring: true }); setError(undefined)
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Chưa bắt đầu đo lại được') }
  }
  const startExport = async (accept = false) => {
    if (!quality?.jobId) { setError('Video chưa liên kết với lượt làm video để xuất.'); return }
    if (!passed && !accept) { setDialog('gate'); return }
    try {
      if (!await session.save()) throw new Error('Hãy lưu thay đổi trước khi xuất.')
      const accepted = await request<TaskAccepted>(`/api/clone/${encodeURIComponent(quality.jobId)}/export`, { body: accept ? { accept: true } : {} })
      setTask(accepted); setTaskResult(undefined); setDialog('exporting'); setError(undefined)
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Chưa bắt đầu xuất được') }
  }
  useShortcutAction({ id: 'export', label: 'Xuất video', scope: 'global', enabled: session.opening === 'ready', run: () => { void startExport() } })
  const goBack = async () => { if (!session.dirty || await session.save()) navigate('/') }
  const saveCopy = async () => {
    if (!project || copying) return
    setCopying(true)
    const storage = getEditorStorage()
    let copyId: string | undefined
    try {
      const copied = await storage.duplicateProject(projectId, `${project.name} — Bản sao`)
      copyId = copied.data.id
      await storage.saveProject({ ...project, id: copied.data.id, name: copied.data.name, createdAt: copied.data.createdAt, timeline: buildTimelineFromStores() }, copied.revision)
      copyId = undefined
      navigate(`/editor/${encodeURIComponent(copied.data.id)}`)
    } catch (cause) {
      if (copyId) { try { await storage.deleteProject(copyId) } catch { /* Keep the completed source and report the copy failure. */ } }
      setError(cause instanceof Error ? cause.message : 'Chưa lưu được bản sao')
    } finally { setCopying(false) }
  }
  const railKey = (event: KeyboardEvent, index: number) => {
    const next = event.key === 'ArrowDown' ? (index + 1) % panels.length : event.key === 'ArrowUp' ? (index + panels.length - 1) % panels.length : event.key === 'Home' ? 0 : event.key === 'End' ? panels.length - 1 : -1
    if (next < 0) return
    event.preventDefault(); setPanel(panels[next].id); document.getElementById(`editor-tab-${panels[next].id}`)?.focus()
  }
  const seconds = items.reduce((end, item) => Math.max(end, item.from + item.durationInFrames), 0) / fps
  const sceneCount = items.filter((item) => tracks.find((track) => track.id === item.trackId)?.templateSlot === 'scenes').length
  const progress = taskResult?.request.progress
  const percent = progress?.framesTotal ? Math.round((progress.framesDone ?? 0) / progress.framesTotal * 100) : progress?.bytesTotal ? Math.round((progress.bytesSent ?? 0) / progress.bytesTotal * 100) : null
  const download = taskResult?.export ? `/api/editor/projects/${encodeURIComponent(projectId)}/exports/${encodeURIComponent(taskResult.export.id)}/file` : undefined
  const conflictOpen = session.conflict && dismissedConflict !== session.conflict.timestamp
  return <main data-editor-root aria-label="Trình chỉnh sửa video" {...stylex.props(sx.root)}>
    <div {...stylex.props(sx.narrow)}><header {...stylex.props(sx.narrowHead)}><button type="button" {...stylex.props(sx.button, sx.quiet)} onClick={() => { void goBack() }}>← Bàn dựng</button></header><div {...stylex.props(sx.centered)}><div role="status" {...stylex.props(sx.narrowMessage)}><Icon name="workbench" size={30} /><span {...stylex.props(sx.narrowTitle)}>Cần màn hình rộng ≥ 1024px</span><span {...stylex.props(sx.narrowCopy)}>Editor cần chỗ cho khung xem, timeline và bảng thuộc tính. Kéo rộng cửa sổ hoặc mở trên máy tính. Duyệt ảnh và theo dõi video vẫn làm được ở Bàn dựng.</span><button type="button" {...stylex.props(sx.button, sx.narrowAction)} onClick={() => { void goBack() }}>Quay lại Bàn dựng</button></div></div></div>
    <div {...stylex.props(sx.wide)}>
      {session.opening !== 'ready' || !project ? <div {...stylex.props(sx.centered)} role="status">{session.opening === 'loading' ? <Spinner size={18} /> : <div {...stylex.props(sx.stack)}><h1 {...stylex.props(sx.dialogTitle)}>{session.opening === 'not-found' ? 'Không tìm thấy video này' : 'Chưa mở được video'}</h1><button {...stylex.props(sx.button)} onClick={session.reloadProject}>Thử lại</button><button {...stylex.props(sx.button)} onClick={() => navigate('/')}>Bàn dựng</button></div>}</div> : <>
        <header {...stylex.props(sx.header)}><button type="button" {...stylex.props(sx.button, sx.compact, sx.quiet)} title="Quay lại Bàn dựng" onClick={() => { void goBack() }}>← Bàn dựng</button><span aria-hidden="true" {...stylex.props(sx.separator)} /><div {...stylex.props(sx.heading)}><h1 title={project.name} {...stylex.props(sx.title)}>{project.name}</h1><span {...stylex.props(sx.meta)}>{[quality?.channelName, rangeTime(seconds), `${sceneCount} cảnh`, quality?.templateName].filter(Boolean).join(' · ')}</span></div><span role="status" {...stylex.props(sx.saved)}><Icon name={session.saveError || session.conflict ? 'warning' : 'tick-b'} size={14} />{!autosave ? 'Đã tắt tự lưu' : session.conflict ? 'Có bản mới hơn' : session.saving ? 'Đang lưu…' : session.saveError ? 'Chưa lưu được' : session.dirty ? 'Đang chờ lưu…' : 'Đã lưu'}</span><button type="button" disabled={!canUndo || aligning} aria-label="Hoàn tác" title="Hoàn tác (⌘Z)" {...stylex.props(sx.button, sx.compact, sx.quiet, sx.iconButton)} onClick={undo}><Icon name="undo" size={16} /></button><button type="button" disabled={!canRedo || aligning} aria-label="Làm lại" title="Làm lại (⌘⇧Z)" {...stylex.props(sx.button, sx.compact, sx.quiet, sx.iconButton)} onClick={redo}><Icon name="redo" size={16} /></button><Shortcuts /><span aria-hidden="true" {...stylex.props(sx.separator)} /><button type="button" aria-controls="ed-left" aria-label={`Độ hoàn thiện ${quality?.total ?? 'chưa đo'}/100, mở danh sách cần sửa`} {...stylex.props(sx.button, sx.compact, sx.scoreChip(passed))} onClick={() => setPanel('fix')}><Icon name={passed ? 'tick-b' : 'danger'} size={14} />Độ hoàn thiện <b>{quality?.total ?? '—'}/100</b></button><button type="button" title="Xuất MP4 (⌘E)" {...stylex.props(sx.button, passed && sx.primary)} onClick={() => { void startExport() }}><Icon name="export" size={15} />Xuất</button></header>
        {error && <div role="alert" {...stylex.props(sx.notice, sx.danger)}>{error}<button type="button" {...stylex.props(sx.button, sx.compact, sx.quiet)} onClick={() => { void refreshQuality() }}>Thử lại</button></div>}
        <div {...stylex.props(sx.body)} inert={dialog === 'exporting' ? true : undefined}>
          <nav role="tablist" aria-label="Bảng bên trái" aria-orientation="vertical" {...stylex.props(sx.rail)}>{panels.map((entry, index) => <button key={entry.id} id={`editor-tab-${entry.id}`} type="button" role="tab" aria-selected={entry.id === panel} aria-controls="ed-left" tabIndex={entry.id === panel ? 0 : -1} {...stylex.props(sx.railButton, entry.id === panel && sx.railSelected)} onKeyDown={(event) => railKey(event, index)} onClick={() => setPanel(entry.id)}><span {...stylex.props(sx.railIcon)}><Icon name={entry.id === 'fix' && passed ? 'tick-b' : entry.icon} size={19} />{entry.id === 'fix' && left > 0 && !passed && <span aria-label={`${left} việc`} {...stylex.props(sx.badge)}>{left}</span>}</span><span {...stylex.props(sx.railLabel)}>{entry.label}</span></button>)}</nav>
          <aside id="ed-left" role="tabpanel" aria-labelledby={`editor-tab-${panel}`} {...stylex.props(sx.left)}><div {...stylex.props(sx.panelHead)}><><h2 {...stylex.props(sx.panelTitle)}>{panel === 'bg' ? 'Nền & lớp phủ' : panels.find((entry) => entry.id === panel)?.label}</h2><span id="editor-panel-actions" /></><span {...stylex.props(sx.panelMeta)}>{panel === 'fix' ? passed ? 'đạt' : left ? `${left} đoạn` : 'đo lại' : ''}</span></div>{panel === 'fix' ? displayQuality ? <FixPanel quality={displayQuality} ranges={ranges} selectedRange={selectedRange} fixedIds={fixedIds} onSelectRange={selectRange} onAlignAll={() => align(true)} onMeasure={measure} aligning={aligning} /> : <div {...stylex.props(sx.fixBody)}><Spinner size={16} /></div> : <ContentPanels panel={panel} jobId={quality?.jobId} beforeMutation={session.save} onProjectChanged={session.reloadProject} measurement={{evidence:displayQuality?.items.find(item=>item.id==='loudness')?.evidence,measuredAt:displayQuality?.measuredAt,stale:displayQuality?.items.find(item=>item.id==='loudness')?.stale}} />}</aside>
          <div data-shell-preview data-shortcut-scope="preview" tabIndex={-1} {...stylex.props(sx.spread, sx.stack)}><Preview /></div>
          <Inspector ranges={visibleRanges} selectedRange={selectedRange} onSelectRange={selectRange} onAlign={() => align(false)} aligning={aligning} jobId={quality?.jobId} beforeMutation={session.save} onProjectChanged={session.reloadProject} />
        </div><div data-shortcut-scope="timeline" inert={dialog === 'exporting' ? true : undefined}><Timeline ranges={visibleRanges} selectedRange={selectedRange} onSelectRange={selectRange} /></div><MediaMetadataSync />
      </>}
    </div>
    {dialog === 'gate' && <Dialog label="Xuất khi chưa đạt độ hoàn thiện?" alertDialog width={460} onClose={() => setDialog(null)}><div {...stylex.props(sx.dialogBody)}><div {...stylex.props(sx.row)}><Icon name="warning" size={20} /><h2 {...stylex.props(sx.dialogTitle)}>Xuất khi chưa đạt độ hoàn thiện?</h2></div><p {...stylex.props(sx.dialogCopy)}>{quality?.total ?? 'Chưa đo'}/100, cần 90. Còn {left} đoạn phụ đề lệch tiếng; khớp theo giọng mất khoảng 10 giây.</p><ul {...stylex.props(sx.dialogCopy)}><li>Phụ đề khớp tiếng: {ranges.filter((range) => !fixedIds.has(range.cueId ?? `${range.start}`)).map((range) => rangeTime(range.start)).join(', ')}</li><li>Vẫn xuất: video chuyển sang Sẵn sàng đăng, Kiểm định ghi bạn đã chọn bỏ qua mục này</li></ul><div {...stylex.props(sx.dialogActions)}><button type="button" {...stylex.props(sx.button)} onClick={() => { void startExport(true) }}>Vẫn xuất</button><button type="button" {...stylex.props(sx.button, sx.primary)} onClick={() => { setDialog(null); setPanel('fix'); selectRange(selectedRange) }}>Sửa tiếp</button></div></div></Dialog>}
    {dialog === 'exporting' && <Dialog label="Đang xuất video" width={520}><div {...stylex.props(sx.dialogBody)}><div {...stylex.props(sx.row)}><Spinner size={22} /><h2 {...stylex.props(sx.dialogTitle, sx.spread)}>Đang xuất video</h2><span>{percent == null ? 'Đang chuẩn bị…' : `${percent}%`}</span></div><span {...stylex.props(sx.secondary)}>1080p · 30 fps · toàn bộ {rangeTime(seconds)} · nén sau khi xuất</span><div role="progressbar" aria-label="Tiến độ xuất" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent ?? undefined} {...stylex.props(sx.scoreTrack)}><span {...stylex.props(sx.scoreFill(percent ?? 0, true))} /></div><div {...stylex.props(sx.stack)}><span {...stylex.props(sx.row)}><Icon name="tick-b" size={16} />Dựng khung hình</span><span {...stylex.props(sx.row)}><Spinner size={14} />{progress?.phase === 'uploading' ? 'Lưu lên máy và kiểm tra tệp' : 'Mã hoá MP4'}</span></div><div {...stylex.props(sx.notice)}>Giữ tab này mở cho tới khi xong: video được dựng ngay trong trình duyệt. Bạn vẫn xem được timeline nhưng không sửa được.</div><div {...stylex.props(sx.dialogActions)}><button type="button" {...stylex.props(sx.button, sx.danger)} onClick={() => { if (task) void mutate('DELETE', task.stateUrl).then(() => setDialog(null)).catch((cause: unknown) => setError(cause instanceof Error ? cause.message : 'Chưa huỷ được xuất')) }}>Huỷ xuất</button></div></div></Dialog>}
    {dialog === 'done' && <Dialog label="Đã xuất MP4" width={520} onClose={() => setDialog(null)}><div {...stylex.props(sx.dialogBody)}><div {...stylex.props(sx.row)}><Icon name="tick-b" size={22} /><h2 {...stylex.props(sx.dialogTitle, sx.spread)}>Đã xuất MP4</h2><span {...stylex.props(sx.small)}>{rangeTime(seconds)}{taskResult?.export?.size != null ? ` · ${(taskResult.export.size / 1024 ** 3).toLocaleString('vi-VN', { maximumFractionDigits: 1 })} GB` : ''}</span></div>{download && <video controls src={download} aria-label="Phát bản vừa xuất" {...stylex.props(sx.previewVideo)} />}<div {...stylex.props(sx.notice)}>Quay lại Bàn dựng, Kiểm định sẽ đo lại độ hoàn thiện cho bản này. Đạt từ 90 thì video chuyển sang Sẵn sàng.</div><div {...stylex.props(sx.dialogActions)}>{download && <a href={download} download {...stylex.props(sx.button)}><Icon name="download" size={15} />Tải MP4</a>}<button type="button" {...stylex.props(sx.button, sx.brandPrimary)} onClick={() => navigate('/')}>Quay lại Bàn dựng</button></div></div></Dialog>}
    {conflictOpen && <Dialog label="Dự án vừa được lưu ở nơi khác" alertDialog width={520}><div {...stylex.props(sx.dialogBody)}><div {...stylex.props(sx.row)}><Icon name="warning" size={20} /><h2 {...stylex.props(sx.dialogTitle)}>Dự án vừa được lưu ở nơi khác</h2></div><p {...stylex.props(sx.dialogCopy)}>Dự án đã có bản mới hơn. Thay đổi bạn làm ở đây chưa được lưu. Chọn cách giữ lại thay đổi; không tự ghi đè bản mới.</p><div {...stylex.props(sx.stack)}><button type="button" {...stylex.props(sx.dialogChoice)} onClick={() => { setAutosave(true); session.reloadProject() }}><span>Tải lại bản mới</span><span {...stylex.props(sx.small)}>Bỏ thay đổi chưa lưu của bạn</span></button><button type="button" {...stylex.props(sx.dialogChoice)} onClick={() => { setAutosave(false); setDismissedConflict(session.conflict?.timestamp) }}><span>Tiếp tục ở đây, tạm tắt tự lưu</span><span {...stylex.props(sx.small)}>Bạn tự quyết sau; không ghi đè bản mới</span></button></div><div {...stylex.props(sx.dialogActions)}><button type="button" disabled={copying} {...stylex.props(sx.button, sx.brandPrimary)} onClick={() => { void saveCopy() }}><Icon name="copy" size={15} />Lưu thay đổi thành bản sao</button></div></div></Dialog>}
    <Toaster theme="dark" toastOptions={{ unstyled: true, classNames: { toast: stylex.props(sx.toast).className } }} />
  </main>
}
