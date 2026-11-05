import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Dialog, Icon, Spinner, type IconName } from '@noverse/ui'
import { executeCommands, getCommandExecutor, useCommandHistory } from '@editor/commands'
import { useProjectStore } from '@editor/features/projects/stores/project-store'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useSelectionStore } from '@editor/shared/state/selection'
import { buildTimelineFromStores } from '@editor/features/timeline/stores/timeline-persistence'
import { getProjectRevision, loadProject } from '@editor/storage/project-persistence'
import { getEditorStorage } from '@editor/storage/storage-backend'
import { mutate, request } from '../../api/request'
import { subscribeEvents } from '../../data/events'
import { navigate, useRoute } from '../../router'
import { ShortcutProvider, useShortcutAction } from '../ui/shortcuts'
import { documentTitle } from '../../shell/labels'
import { Shortcuts } from './Shortcuts'
import { useProjectSession } from '../ui/shell/use-project-session'
import { MediaMetadataSync } from '../ui/shell/media-sync'
import { captionSignature, canExport, extraBlockers, fetchQuality, qualityRanges, rangeItem, type EditorQuality } from './quality'
import { FixPanel, rangeTime } from './FixPanel'
import { ExportGateDialog } from './ExportGateDialog'
import { ExportDoneDialog, ExportingDialog, type ExportProgress } from './ExportDialogs'
import { Preview } from './Preview'
import { Timeline } from './Timeline'
import { Inspector } from './Inspector'
import { ContentPanels, isCaption, matchesCloneEvent } from './ContentPanels'
import { clipIdFor, drawingVersionOf, sceneNumber, undoSceneSwap, useSceneRedraws, useSceneSwapStore } from './scene-images'
import { sx } from './editor.stylex'
import '../../styles/editor-utilities.css'

type Panel = 'fix' | 'scenes' | 'subs' | 'bg' | 'audio' | 'media'
const panels: { id: Panel; label: string; icon: IconName }[] = [
  { id: 'fix', label: 'Cần sửa', icon: 'danger' }, { id: 'scenes', label: 'Cảnh', icon: 'gallery' },
  { id: 'subs', label: 'Phụ đề', icon: 'subtitle' }, { id: 'bg', label: 'Nền', icon: 'video-play' },
  { id: 'audio', label: 'Âm thanh', icon: 'volume' }, { id: 'media', label: 'Media', icon: 'folder' },
]
type TaskResult = { request: { id: string; status: string; progress?: ExportProgress; error?: string | { message?: string } }; export?: { id: string; size?: number }; score?: unknown }
type TaskAccepted = { requestId: string; stateUrl: string; request: TaskResult['request'] }

function clockLabel(value: number): string {
  return new Date(value).toLocaleTimeString('vi', { hour: '2-digit', minute: '2-digit' })
}
function decimalSeconds(value: number): string {
  return Math.abs(value).toLocaleString('vi', { maximumFractionDigits: 1 })
}

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
  const selectedItemId = useSelectionStore((state) => state.selectedItemIds[0])
  const [panel, setPanel] = useState<Panel>('fix')
  const [quality, setQuality] = useState<EditorQuality>()
  const [error, setError] = useState<string>()
  const [selectedRange, setSelectedRange] = useState(0)
  const [alignedSnapshots, setAlignedSnapshots] = useState(new Map<string, string>())
  const [dialog, setDialog] = useState<'gate' | 'exporting' | 'done' | 'shortcuts' | null>(null)
  const locked = dialog === 'exporting'
  const [dismissedConflict, setDismissedConflict] = useState<number>()
  const [copying, setCopying] = useState(false)
  const [task, setTask] = useState<TaskAccepted>()
  const [taskResult, setTaskResult] = useState<TaskResult>()
  const [measureTask, setMeasureTask] = useState<TaskAccepted>()
  const redraws = useSceneRedraws(quality?.jobId)
  const swapNotice = useSceneSwapStore(state => state.notices[0])
  const swapError = useSceneSwapStore(state => state.error)
  const { search } = useRoute()
  const openedAt = useRef(false)
  const fps = project?.metadata.fps ?? 30
  const ranges = useMemo(() => quality ? qualityRanges(quality) : [], [quality])
  const fixedIds = useMemo(() => new Set(ranges.filter((range) => alignedSnapshots.get(range.cueId ?? `${range.start}`) === captionSignature(range, items, fps) && alignedSnapshots.has(range.cueId ?? `${range.start}`)).map((range) => range.cueId ?? `${range.start}`)), [ranges, items, fps, alignedSnapshots])
  const visibleRanges = ranges.map((range) => fixedIds.has(range.cueId ?? `${range.start}`) ? { ...range, status: 'aligned' } : range)
  const left = ranges.filter((range) => range.status !== 'pass' && !fixedIds.has(range.cueId ?? `${range.start}`)).length
  const blockers = quality ? extraBlockers(quality) : []
  const issueCount = left + blockers.length + (quality?.items.filter(item => item.status === 'fail' && !ranges.some(range => range.checkId === item.id)).length ?? 0)
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
    if (!quality?.jobId) return
    const jobId = quality.jobId
    return subscribeEvents(['clone.job.updated', 'reset'], data => {
      if (matchesCloneEvent(data, jobId)) void refreshQuality()
    })
  }, [quality?.jobId, refreshQuality])
  useEffect(() => {
    if (session.savedAt && !aligning) void refreshQuality()
  }, [session.savedAt, aligning, refreshQuality])
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
        if (result.request.status === 'measured') { setMeasureTask(undefined); setAlignedSnapshots(new Map()); await refreshQuality(); return }
        if (['failed', 'cancelled', 'stale'].includes(result.request.status)) { setMeasureTask(undefined); await refreshQuality(); setError(typeof result.request.error === 'string' ? result.request.error : result.request.error?.message ?? 'Đo lại chưa hoàn tất'); return }
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
        if (result.request.status === 'done') { setDialog('done'); return }
        if (['failed', 'cancelled', 'stale'].includes(result.request.status)) { setDialog(null); setError(typeof result.request.error === 'string' ? result.request.error : result.request.error?.message ?? 'Xuất video chưa hoàn tất'); return }
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
  useEffect(() => { if (project) document.title = documentTitle({ name: 'editor', projectId }, { project: project.name }) }, [project?.name, projectId])
  useEffect(() => {
    if (!swapNotice) return
    const timer = window.setTimeout(() => useSceneSwapStore.getState().dismiss(swapNotice.id), 4000)
    return () => window.clearTimeout(timer)
  }, [swapNotice])
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
      const response = await request<{ cues: { cueId: string }[] }>(`/api/editor/${encodeURIComponent(projectId)}/captions/align`, { body: { cueIds: all ? 'all-drifting' : cueIds, revision: getProjectRevision(projectId) } })
      if (!response.cues.length) throw new Error('Chưa khớp được theo giọng')
      // The server persists the edit; adopt its CAS revision without clearing the engine's history.
      const aligned = await loadProject(projectId)
      if (!aligned?.timeline) throw new Error('Chưa đọc được bản phụ đề vừa khớp.')
      const result = executeCommands([{ type: 'project.patch', params: { patch: { items: aligned.timeline.items } } }])
      if (!result.ok) throw new Error(result.issues.map((issue) => issue.message).join(' '))
      const next = new Map(alignedSnapshots)
      for (const range of targets) if (response.cues.some(cue => cue.cueId === range.cueId)) next.set(range.cueId ?? `${range.start}`, captionSignature(range, useItemsStore.getState().items, fps))
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
    if (locked) return
    if (!quality?.jobId) { setError('Video chưa liên kết với lượt làm video để xuất.'); return }
    if (!passed && !accept) { setDialog('gate'); return }
    try {
      if (!await session.save()) throw new Error('Hãy lưu thay đổi trước khi xuất.')
      const accepted = await request<TaskAccepted>(`/api/clone/${encodeURIComponent(quality.jobId)}/export`, { body: accept ? { accept: true } : {} })
      setTask(accepted); setTaskResult(undefined); setDialog('exporting'); setError(undefined)
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Chưa bắt đầu xuất được') }
  }
  useShortcutAction({ id: 'export', label: 'Xuất video', scope: 'global', enabled: session.opening === 'ready' && !locked, run: () => { void startExport() } })
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
  const backgroundCount = items.filter((item) => item.type === 'video' && tracks.some((track) => track.id === item.trackId && track.templateSlot === 'background-video')).length
  const captionCount = items.reduce((count, item) => count + (isCaption(item) ? item.type === 'subtitle' ? item.cues.length : 1 : 0), 0)
  const cutout = tracks.find((track) => track.templateSlot === 'scenes')?.templateScenes?.style
  const panelMeta = panel === 'fix'
    ? passed ? 'đạt' : left ? `${left} đoạn` : issueCount ? `${issueCount} mục` : 'đo lại'
    : panel === 'scenes' ? `${sceneCount} cảnh`
      : panel === 'subs' ? `${captionCount} đoạn`
        : panel === 'bg' ? `${backgroundCount} clip` : ''
  const selectedItem = items.find((item) => item.id === selectedItemId)
  const selectedCaptionRange = visibleRanges[selectedRange]
  const captionSelected = Boolean(selectedItem && isCaption(selectedItem))
  const cueStatus = captionSelected && selectedCaptionRange
    ? {
        ok: fixedIds.has(selectedCaptionRange.cueId ?? `${selectedCaptionRange.start}`) || ['pass', 'passed', 'aligned', 'ok'].includes(selectedCaptionRange.status),
        text: selectedCaptionRange.offsetSec != null
          ? `Chữ ${selectedCaptionRange.offsetSec > 0 ? 'muộn' : 'sớm'} ${decimalSeconds(selectedCaptionRange.offsetSec)} giây so với tiếng`
          : 'Chữ lệch tiếng',
      }
    : undefined
  const drawingImage = redraws.images.find(image => drawingVersionOf(image))
  const drawingScene = drawingImage ? sceneNumber(clipIdFor(projectId, drawingImage) ?? '') ?? undefined : undefined
  const drawingClipId = drawingImage ? clipIdFor(projectId, drawingImage) ?? undefined : undefined
  const savedText = !autosave ? 'Đã tắt tự lưu'
    : session.conflict ? 'Chưa lưu · có bản mới hơn'
      : locked ? 'Đã lưu · khoá khi đang xuất'
        : session.saveError ? 'Chưa lưu được'
          : session.saving ? 'Đang lưu…'
            : session.dirty ? 'Đang chờ lưu…'
              : session.savedAt ? `Đã lưu · ${clockLabel(session.savedAt)}` : 'Đã lưu'
  const progress = taskResult?.request.progress
  const download = taskResult?.export ? getEditorStorage().exportUrl(projectId, taskResult.export.id) : undefined
  const conflictOpen = session.conflict && dismissedConflict !== session.conflict.timestamp
  return <main data-editor-root aria-label="Trình chỉnh sửa video" {...stylex.props(sx.root)}>
    <div {...stylex.props(sx.narrow)}><header {...stylex.props(sx.narrowHead)}><Button variant="quiet" size="lg" onClick={() => { void goBack() }} icon={<Icon name="arrow-r" size={15} {...stylex.props(sx.backArrow)} />}>Bàn dựng</Button></header><div {...stylex.props(sx.centered)}><div role="status" {...stylex.props(sx.narrowMessage)}><Icon name="workbench" size={30} /><span {...stylex.props(sx.narrowTitle)}>Cần màn hình rộng ≥ 1024px</span><span {...stylex.props(sx.narrowCopy)}>Biên tập cần chỗ cho khung xem, dòng thời gian và bảng thuộc tính. Kéo rộng cửa sổ hoặc mở trên máy tính. Duyệt ảnh và theo dõi video vẫn làm được ở Bàn dựng.</span><Button size="lg" onClick={() => { void goBack() }}>Quay lại Bàn dựng</Button></div></div></div>
    <div {...stylex.props(sx.wide)}>
      {session.opening !== 'ready' || !project ? <div {...stylex.props(sx.centered)} role="status">{session.opening === 'loading' ? <Spinner size={18} /> : <div {...stylex.props(sx.stack)}><h1 {...stylex.props(sx.dialogTitle)}>{session.opening === 'not-found' ? 'Không tìm thấy video này' : 'Chưa mở được video'}</h1><Button onClick={session.reloadProject}>Thử lại</Button><Button onClick={() => navigate('/')}>Bàn dựng</Button></div>}</div> : <>
        <header {...stylex.props(sx.header)} inert={locked || undefined}>
          <Button size="sm" variant="quiet" title="Quay lại Bàn dựng" onClick={() => { void goBack() }} icon={<Icon name="arrow-r" size={14} {...stylex.props(sx.backArrow)} />}>Bàn dựng</Button>
          <span aria-hidden="true" {...stylex.props(sx.separator)} />
          <div {...stylex.props(sx.heading)}><h1 title={project.name} {...stylex.props(sx.title)}>{project.name}</h1><span {...stylex.props(sx.meta)}>{[quality?.channelName, rangeTime(seconds), `${sceneCount} cảnh`, quality?.templateName, cutout && cutout !== 'full' ? 'Tách nền' : null].filter(Boolean).join(' · ')}</span></div>
          <span role="status" aria-live="polite" {...stylex.props(sx.saved)}><Icon name={session.saveError || session.conflict ? 'warning' : 'tick-b'} size={14} />{savedText}</span>
          <Button size="sm" variant="icon" disabled={!canUndo || aligning || locked} aria-label="Hoàn tác" title={!canUndo ? 'Không có thay đổi để hoàn tác' : locked ? 'Không thể hoàn tác khi đang xuất' : 'Hoàn tác (⌘Z)'} onClick={undo}><Icon name="undo" size={16} /></Button>
          <Button size="sm" variant="icon" disabled={!canRedo || aligning || locked} aria-label="Làm lại" title={!canRedo ? 'Không có thay đổi để làm lại' : locked ? 'Không thể làm lại khi đang xuất' : 'Làm lại (⌘⇧Z)'} onClick={redo}><Icon name="redo" size={16} /></Button>
          <Shortcuts />
          <span aria-hidden="true" {...stylex.props(sx.separator)} />
          <Button size="sm" variant={passed ? 'ghost' : 'dsoft'} aria-controls="ed-left" aria-label={`Độ hoàn thiện ${quality?.total ?? 'chưa đo'}/100, mở danh sách cần sửa`} title={quality ? `Độ hoàn thiện ${quality.total ?? '—'}/100 · mở Cần sửa` : 'Chưa đọc được độ hoàn thiện · mở Cần sửa'} onClick={() => setPanel('fix')} icon={<Icon name={passed ? 'tick-b' : 'danger'} size={14} />}>Độ hoàn thiện <b>{quality?.total ?? '—'}/100</b></Button>
          <Button variant={passed ? 'primary' : 'ghost'} title={!quality?.jobId ? 'Video chưa liên kết với lượt làm video nên chưa xuất được' : locked ? 'Đang xuất video' : 'Xuất MP4 (⌘E)'} aria-disabled={!quality?.jobId || locked} onClick={() => { void startExport() }} icon={<Icon name="export" size={15} />}>Xuất</Button>
        </header>
        {error && <div role="alert" {...stylex.props(sx.notice, sx.danger)}>{error}<Button size="sm" variant="quiet" onClick={() => { void refreshQuality() }}>Thử lại</Button></div>}
        <div {...stylex.props(sx.body)} inert={dialog === 'exporting' ? true : undefined}>
          <nav role="tablist" aria-label="Bảng bên trái" aria-orientation="vertical" {...stylex.props(sx.rail)}>{panels.map((entry, index) => <button key={entry.id} id={`editor-tab-${entry.id}`} type="button" role="tab" aria-selected={entry.id === panel} aria-controls="ed-left" tabIndex={entry.id === panel ? 0 : -1} {...stylex.props(sx.railButton, entry.id === panel && sx.railSelected)} onKeyDown={(event) => railKey(event, index)} onClick={() => setPanel(entry.id)}><span {...stylex.props(sx.railIcon)}><Icon name={entry.id === 'fix' && passed ? 'tick-b' : entry.icon} size={19} label={undefined} xstyle={[entry.id === 'fix' && (passed ? sx.railFixPass : sx.railFixFail), entry.id !== 'fix' && entry.id === panel && sx.railSelectedInk]} />{entry.id === 'fix' && issueCount > 0 && !passed && <span aria-label={`${issueCount} việc`} {...stylex.props(sx.badge)}>{issueCount}</span>}</span><span {...stylex.props(sx.railLabel)}>{entry.label}</span></button>)}</nav>
          <aside id="ed-left" role="tabpanel" aria-labelledby={`editor-tab-${panel}`} {...stylex.props(sx.left)}><div {...stylex.props(sx.panelHead)}><><h2 {...stylex.props(sx.panelTitle)}>{panel === 'bg' ? 'Nền & lớp phủ' : panels.find((entry) => entry.id === panel)?.label}</h2><span id="editor-panel-actions" /></><span {...stylex.props(sx.panelMeta)}>{panelMeta}</span></div>{panel === 'fix' ? displayQuality ? <FixPanel quality={displayQuality} ranges={ranges} selectedRange={selectedRange} fixedIds={fixedIds} onSelectRange={selectRange} onAlignAll={() => align(true)} onMeasure={measure} aligning={aligning} /> : <div {...stylex.props(sx.fixBody)}><Spinner size={16} /></div> : <ContentPanels panel={panel} jobId={quality?.jobId} beforeMutation={session.save} onProjectChanged={session.reloadProject} ranges={visibleRanges} selectedRange={selectedRange} onSelectRange={selectRange} measurement={{evidence:displayQuality?.items.find(item=>item.id==='loudness')?.evidence,measuredAt:displayQuality?.measuredAt,stale:displayQuality?.items.find(item=>item.id==='loudness')?.stale}} />}</aside>
          <div data-shell-preview data-shortcut-scope="preview" tabIndex={-1} {...stylex.props(sx.spread, sx.stack)}>
            <Preview overlay={<>
              {drawingScene && <span role="status" {...stylex.props(sx.drawBadge)}><Icon name="magicpen" size={13} />Đang vẽ lại ảnh cảnh {drawingScene} · bạn chỉnh tiếp được</span>}
              {cueStatus && <span role="status" {...stylex.props(sx.cueOverlay, cueStatus.ok ? sx.cueOverlayOk : sx.cueOverlayWarn)}><span aria-hidden="true" {...stylex.props(sx.cueDot, cueStatus.ok ? sx.cueDotOk : sx.cueDotWarn)} />{cueStatus.text}</span>}
              {swapNotice && <div role="status" aria-live="polite" {...stylex.props(sx.swapToast)}>
                <Icon name="tick-b" size={14} {...stylex.props(sx.swapToastOk)} />
                <span>{swapNotice.label} đã dùng bản {swapNotice.n} · chuyển động, điểm mốc giữ nguyên</span>
                <Button size="sm" variant="quiet" onClick={() => undoSceneSwap(swapNotice)}>Hoàn tác <kbd {...stylex.props(sx.kbd)}>⌘Z</kbd></Button>
                <button type="button" aria-label="Đóng thông báo" {...stylex.props(sx.toastClose)} onClick={() => useSceneSwapStore.getState().dismiss(swapNotice.id)}><Icon name="close" size={13} /></button>
              </div>}
              {swapError && <div role="alert" {...stylex.props(sx.swapError)}>{swapError}</div>}
            </>} />
          </div>
          <Inspector ranges={visibleRanges} selectedRange={selectedRange} onSelectRange={selectRange} onAlign={() => align(false)} aligning={aligning} jobId={quality?.jobId} images={redraws.images} imageProgress={redraws.progress} imageQuota={redraws.quota} onRedrawn={redraws.reload} captionAlignmentPassed={!session.dirty && quality?.items.some(item => item.id === 'captionAlignment' && item.status === 'pass' && !item.stale)} beforeMutation={session.save} onProjectChanged={session.reloadProject} />
        </div><div data-shortcut-scope="timeline" inert={dialog === 'exporting' ? true : undefined}><Timeline ranges={visibleRanges} selectedRange={selectedRange} onSelectRange={selectRange} drawingClipId={drawingClipId} /></div><MediaMetadataSync />
      </>}
    </div>
    {dialog === 'gate' && <ExportGateDialog quality={displayQuality} ranges={ranges.filter(range => range.status !== 'pass' && !fixedIds.has(range.cueId ?? `${range.start}`))} onAccept={() => { void startExport(true) }} onContinue={() => { setDialog(null); setPanel('fix'); selectRange(selectedRange) }} onClose={() => setDialog(null)} />}
    {dialog === 'exporting' && <ExportingDialog seconds={seconds} fps={fps} progress={progress} onCancel={() => { if (task) void mutate('DELETE', task.stateUrl).then(() => setDialog(null)).catch((cause: unknown) => setError(cause instanceof Error ? cause.message : 'Chưa huỷ được xuất')) }} />}
    {dialog === 'done' && <ExportDoneDialog seconds={seconds} bytes={taskResult?.export?.size} download={download} onClose={() => setDialog(null)} onBack={() => navigate('/')} />}
    {conflictOpen && <Dialog label="Dự án đã thay đổi trên máy chủ" alertDialog width={520} padBody={false}><div {...stylex.props(sx.dialogBody)}><div {...stylex.props(sx.row)}><Icon name="warning" size={20} {...stylex.props(sx.warning)} /><h2 {...stylex.props(sx.dialogTitle)}>Dự án đã thay đổi trên máy chủ</h2></div><p {...stylex.props(sx.dialogCopy)}>Phiên bản trên máy chủ không còn khớp với bản đã mở ở đây. Thay đổi bạn làm ở đây chưa được lưu. Chọn cách giữ lại thay đổi; không tự ghi đè bản mới.</p><div {...stylex.props(sx.stack)}>
      <div {...stylex.props(sx.stack)}><Button block aria-describedby="editor-reload-help" onClick={() => { setAutosave(true); session.reloadProject() }}>Tải lại bản mới</Button><p id="editor-reload-help" {...stylex.props(sx.dialogCopy)}>Bỏ thay đổi chưa lưu của bạn</p></div>
      <div {...stylex.props(sx.stack)}><Button block aria-describedby="editor-continue-help" onClick={() => { setAutosave(false); setDismissedConflict(session.conflict?.timestamp) }}>Tiếp tục ở đây, tạm tắt tự lưu</Button><p id="editor-continue-help" {...stylex.props(sx.dialogCopy)}>Bạn tự quyết sau; không ghi đè bản mới</p></div>
    </div><div {...stylex.props(sx.dialogActions)}><Button variant="primary" pending={copying} onClick={() => { void saveCopy() }} icon={<Icon name="copy" size={15} />}>Lưu thay đổi thành bản sao</Button></div></div></Dialog>}

  </main>
}
