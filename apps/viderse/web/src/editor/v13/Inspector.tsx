import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Dialog, Icon } from '@noverse/ui'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import { useTransitionsStore } from '@editor/features/timeline/stores/transitions-store'
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
import { useProjectStore } from '@editor/features/projects/stores/project-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useSelectionStore } from '@editor/shared/state/selection'
import { getWaveformMeta, getWaveformRecord } from '@editor/infrastructure/storage'
import { computeInitialTransform } from '@editor/features/timeline/utils/transform-init'
import type { CaptionWord, ImageItem, SubtitleSegmentItem, TextItem, TimelineItem, VideoItem } from '@editor/types/timeline'
import type { SerializedCommand } from '@editor/commands'
import type { TransformProperties } from '@editor/types/transform'
import type { ImageProgress, ImageQuota, JobImage } from '../../api/images'
import { Segmented } from '../../components/Segmented'
import { useShortcutAction } from '../ui/shortcuts'
import { MediaAsset, backdropItemFor, editorCommand, isCaption, panelError, panelTime, useBackdropActions, type BackdropActions } from './ContentPanels'
import { sceneImageFor } from './scene-images'
import { SceneImagePanel } from './SceneImagePanel'
import { inspectorStyles as sx } from './Inspector.stylex'

export interface InspectorRange {
  start: number
  end: number
  checkId: string
  status: string
  cueId?: string
  itemId?: string
  offsetSec?: number
  text?: string
}
export interface InspectorProps {
  ranges: readonly InspectorRange[]
  selectedRange: number | null
  onSelectRange: (index: number) => void
  onAlign: () => Promise<void>
  aligning: boolean
  jobId?: string
  /** Ảnh + phiên bản của video (§26), đọc một lần ở Editor và truyền xuống. */
  images?: readonly JobImage[]
  imageProgress?: Record<string, ImageProgress>
  imageQuota?: ImageQuota
  captionAlignmentPassed?: boolean
  beforeMutation?: () => Promise<boolean>
  onProjectChanged?: () => void | Promise<void>
  /** Vẽ lại xong: đọc lại danh sách ảnh + bản đang dùng. */
  onRedrawn?: () => void | Promise<void>
}

type CaptionItem = TextItem | SubtitleSegmentItem

function preciseTime(seconds: number): string {
  const value = Math.max(0, seconds)
  const minutes = Math.floor(value / 60)
  return `${minutes}:${(value - minutes * 60).toFixed(2).replace('.', ',').padStart(5, '0')}`
}

function CaptionTiming({ item, startFrame, fps, aligned }: { item: CaptionItem; startFrame: number; fps: number; aligned: boolean }) {
  const items = useItemsStore(state => state.items)
  const tracks = useItemsStore(state => state.tracks)
  const sourceId = item.type === 'text' ? item.captionSource?.clipId : 'clipId' in item.source ? item.source.clipId : undefined
  const audio = items.find(entry => entry.id === sourceId && (entry.type === 'audio' || entry.type === 'video'))
    ?? items.find(entry => (entry.type === 'audio' || entry.type === 'video') && entry.from <= startFrame && entry.from + entry.durationInFrames > startFrame && tracks.some(track => track.id === entry.trackId && track.templateSlot === 'source-audio'))
  const windowStart = Math.max(0, startFrame / fps - 1)
  const [waveform, setWaveform] = useState<{ key: string; peaks: number[]; failed: boolean } | null>(null)
  const key = `${audio?.id ?? ''}:${audio?.mediaId ?? ''}:${audio?.sourceStart ?? 0}:${audio?.from ?? 0}:${audio?.speed ?? 1}:${windowStart}:${fps}`
  useEffect(() => {
    let active = true
    const load = async () => {
      try {
        const mediaId = audio?.mediaId
        if (!audio || !mediaId) { if (active) setWaveform({ key, peaks: [], failed: false }); return }
        const meta = await getWaveformMeta(mediaId)
        const sourceStart = (audio.sourceStart ?? 0) / (audio.sourceFps || fps)
        const sourceAt = (seconds: number) => sourceStart + (seconds - audio.from / fps) * (audio.speed ?? 1)
        if (!meta) {
          const data = audio.type === 'audio' ? audio.waveformData : undefined
          const sourceDuration = (audio.sourceDuration ?? 0) / (audio.sourceFps || fps)
          const peaks = data?.length && sourceDuration > 0 ? Array.from({ length: 80 }, (_, index) => {
            const time = windowStart + index / 80 * 3
            if (time < audio.from / fps || time >= (audio.from + audio.durationInFrames) / fps) return 0
            return Math.abs(data[Math.floor(sourceAt(time) / sourceDuration * data.length)] ?? 0)
          }) : []
          if (active) setWaveform({ key, peaks, failed: false })
          return
        }
        const indices = new Set<number>()
        for (let index = 0; index < 80; index++) {
          const source = sourceAt(windowStart + index / 80 * 3)
          if (source >= 0 && source < meta.duration) indices.add(Math.floor(source / meta.binDurationSec))
        }
        const records = new Map(await Promise.all([...indices].map(async index => [index, await getWaveformRecord(`${mediaId}:bin:${index}`)] as const)))
        const peaks = Array.from({ length: 80 }, (_, index) => {
          const time = windowStart + index / 80 * 3
          const source = sourceAt(time)
          if (source < 0 || source >= meta.duration || time < audio.from / fps || time >= (audio.from + audio.durationInFrames) / fps) return 0
          const binIndex = Math.floor(source / meta.binDurationSec)
          const record = records.get(binIndex)
          if (!record || !('kind' in record) || record.kind !== 'bin') return 0
          const values = new Float32Array(record.peaks)
          const at = Math.floor((source - binIndex * meta.binDurationSec) * meta.sampleRate) * (meta.stereo ? 2 : 1)
          return Math.max(Math.abs(values[at] ?? 0), meta.stereo ? Math.abs(values[at + 1] ?? 0) : 0)
        })
        if (active) setWaveform({ key, peaks: records.size && [...records.values()].some(Boolean) ? peaks : [], failed: false })
      } catch { if (active) setWaveform({ key, peaks: [], failed: true }) }
    }
    void load()
    return () => { active = false }
  }, [key, audio, fps, windowStart])
  const peaks = waveform?.key === key ? waveform.peaks : []
  const words = (item.words ?? []).filter(word => (item.from + word.endFrame) / fps > windowStart && (item.from + word.startFrame) / fps < windowStart + 3)
  return <div {...stylex.props(sx.strip)} aria-label="Chữ so với tiếng trong 3 giây quanh đoạn này">
    <div {...stylex.props(sx.row)}><span {...stylex.props(sx.stripLabel)}>Tiếng</span><div {...stylex.props(sx.voice)}>{peaks.length ? peaks.map((peak, index) => <span key={index} aria-hidden="true" {...stylex.props(sx.peak(index / peaks.length * 100, Math.min(28, Math.max(1, peak * 28))))} />) : <span {...stylex.props(sx.help)}>{waveform?.failed ? 'Không đọc được dữ liệu tiếng.' : 'Chưa có dữ liệu tiếng.'}</span>}</div></div>
    <div {...stylex.props(sx.row)}><span {...stylex.props(sx.stripLabel)}>Chữ</span><div {...stylex.props(sx.words)}>{words.map((word, index) => {
      const start = Math.max(0, ((item.from + word.startFrame) / fps - windowStart) / 3 * 100)
      const end = Math.min(100, ((item.from + word.endFrame) / fps - windowStart) / 3 * 100)
      return <button type="button" key={`${word.startFrame}:${index}`} title={`${word.text} · ${((item.from + word.startFrame) / fps).toFixed(2)}–${((item.from + word.endFrame) / fps).toFixed(2)} s`} aria-label={`Nghe từ ${word.text}`} {...stylex.props(sx.word(start, Math.max(0, end - start), aligned))} onClick={() => { const playback = usePlaybackStore.getState(); playback.pause(); playback.setCurrentFrame(item.from + word.startFrame) }} />
    })}{!words.length && <span {...stylex.props(sx.help)}>Chưa có mốc từng từ.</span>}</div></div>
    <div {...stylex.props(sx.ticks)}>{[0, 1, 2, 3].map(offset => <span key={offset}>{panelTime(windowStart + offset)}</span>)}</div>
  </div>
}

function CaptionInspector({ item, ranges, selectedRange, onSelectRange, onAlign, aligning, captionAlignmentPassed }: InspectorProps & { item: CaptionItem }) {
  const fps = useTimelineSettingsStore(state => state.fps)
  const activeCue = usePlaybackStore(state => item.type === 'subtitle'
    ? item.cues.find(entry => (state.currentFrame - item.from) / fps >= entry.startSeconds && (state.currentFrame - item.from) / fps < entry.endSeconds)
    : undefined)
  const selected = selectedRange === null ? undefined : ranges[selectedRange]
  const cue = item.type === 'subtitle' ? activeCue
    ?? item.cues.find(entry => entry.id === selected?.cueId)
    ?? item.cues[0] : undefined
  const start = item.from + Math.round((cue?.startSeconds ?? 0) * fps)
  const end = cue ? item.from + Math.round(cue.endSeconds * fps) : item.from + item.durationInFrames
  const text = item.type === 'text' ? item.text : cue?.text ?? ''
  const [draft, setDraft] = useState<string | null>(null)
  const [error, setError] = useState('')
  const cueKey = `${item.id}:${cue?.id ?? ''}`
  const [draftKey, setDraftKey] = useState(cueKey)
  if (draftKey !== cueKey) { setDraftKey(cueKey); setDraft(null); setError('') }
  const stopReplay = useRef<(() => void) | null>(null)
  useEffect(() => () => {
    if (stopReplay.current) {
      stopReplay.current()
      stopReplay.current = null
      usePlaybackStore.getState().pause()
    }
  }, [cueKey])
  const drift = ranges.map((range, index) => ({ range, index })).filter(({ range }) => !['pass', 'passed', 'ok', 'aligned'].includes(range.status) && (/caption|subtitle|karaoke|phu_de/iu.test(range.checkId) || Boolean(range.cueId) || range.itemId === item.id))
  const driftIndex = drift.findIndex(entry => entry.index === selectedRange)
  const matching = ranges.find(range => range.cueId ? range.cueId === (cue?.id ?? item.id) : range.itemId ? range.itemId === item.id : range.start < end / fps && range.end > start / fps && /caption|subtitle|karaoke/iu.test(range.checkId))
  const passed = captionAlignmentPassed || Boolean(matching && ['pass', 'passed', 'ok', 'aligned'].includes(matching.status))
  const perform = (commands: SerializedCommand[]) => { try { editorCommand(commands); setError('') } catch (error) { setError(panelError(error)) } }
  const navigate = (direction: number) => {
    if (!drift.length) return
    const at = driftIndex < 0 ? direction > 0 ? 0 : drift.length - 1 : (driftIndex + direction + drift.length) % drift.length
    const next = drift[at]
    if (next) onSelectRange(next.index)
  }
  const shift = (direction: number) => {
    const delta = direction * Math.max(1, Math.round(fps / 10))
    if (!cue || item.type !== 'subtitle') {
      if (item.from + delta < 0) { setError('Không thể dời đoạn trước đầu video.'); return }
      perform([{ type: 'item.move', params: { id: item.id, from: item.from + delta, trackId: item.trackId } }])
      return
    }
    const nextStart = cue.startSeconds + delta / fps
    const nextEnd = cue.endSeconds + delta / fps
    if (nextStart < 0 || nextEnd * fps > item.durationInFrames) { setError('Mốc phụ đề phải nằm trong thời lượng đoạn.'); return }
    const localStart = Math.round(cue.startSeconds * fps)
    const localEnd = Math.round(cue.endSeconds * fps)
    const words = (item.words ?? []).map(word => word.startFrame >= localStart && word.startFrame < localEnd ? { ...word, startFrame: word.startFrame + delta, endFrame: word.endFrame + delta } : word).toSorted((a, b) => a.startFrame - b.startFrame)
    perform([{ type: 'item.update', params: { id: item.id, updates: { cues: item.cues.map(entry => entry.id === cue.id ? { ...entry, startSeconds: nextStart, endSeconds: nextEnd } : entry).toSorted((a, b) => a.startSeconds - b.startSeconds) } } }, { type: 'caption.setWords', params: { itemId: item.id, words, estimated: item.wordsEstimated ?? false } }])
  }
  const replay = () => {
    stopReplay.current?.()
    const playback = usePlaybackStore.getState()
    playback.pause(); playback.setCurrentFrame(start)
    stopReplay.current = usePlaybackStore.subscribe(state => {
      if (state.currentFrame >= end || !state.isPlaying) {
        stopReplay.current?.(); stopReplay.current = null
        if (state.currentFrame >= end) { state.pause(); state.setCurrentFrame(Math.max(start, end - 1)) }
      }
    })
    playback.play()
  }
  const align = async () => { if (aligning || passed) return; setError(''); try { await onAlign() } catch (error) { setError(panelError(error)) } }
  const commit = () => {
    if (draft === null || draft === text) { setDraft(null); return }
    const next = draft.trim()
    if (!next) { setError('Chữ của đoạn không được để trống.'); return }
    const oldWords = item.words ?? []
    const tokens = next.split(/\s+/u)
    const firstFrame = cue ? Math.round(cue.startSeconds * fps) : 0
    const lastFrame = cue ? Math.round(cue.endSeconds * fps) : item.durationInFrames
    const selectedWords = oldWords.filter(word => word.startFrame >= firstFrame && word.startFrame < lastFrame)
    let at = 0
    // Keep measured timing only when each edited word has a real corresponding timing.
    // Otherwise discard the stale cue timings; alignment must measure the new words.
    const words: CaptionWord[] = oldWords.flatMap(word => {
      if (word.startFrame < firstFrame || word.startFrame >= lastFrame) return [word]
      return selectedWords.length === tokens.length ? [{ ...word, text: tokens[at++]! }] : []
    })
    const commands: SerializedCommand[] = [{ type: 'item.update', params: { id: item.id, updates: item.type === 'text' ? { text: next } : { cues: item.cues.map(entry => entry.id === cue?.id ? { ...entry, text: next } : entry) }, ...(item.type === 'text' ? { clearFields: ['textSpans', 'textLayoutDrafts'] } : {}) } }, { type: 'caption.setWords', params: { itemId: item.id, words, estimated: item.wordsEstimated ?? false } }]
    try { editorCommand(commands); setDraft(null); setError('') } catch (error) { setError(panelError(error)) }
  }
  useShortcutAction({ id: 'v13-caption-previous', label: 'Đoạn lệch trước', scope: 'global', bindings: ['shift+KeyF'], run: () => navigate(-1) })
  useShortcutAction({ id: 'v13-caption-next', label: 'Đoạn lệch sau', scope: 'global', bindings: ['KeyF'], run: () => navigate(1) })
  useShortcutAction({ id: 'v13-caption-earlier', label: 'Sớm hơn 0,1 giây', scope: 'global', bindings: ['Comma'], run: () => shift(-1) })
  useShortcutAction({ id: 'v13-caption-later', label: 'Muộn hơn 0,1 giây', scope: 'global', bindings: ['Period'], run: () => shift(1) })
  useShortcutAction({ id: 'v13-caption-replay', label: 'Nghe lại đoạn này', scope: 'global', bindings: ['KeyL'], run: replay })
  return <>
    <section {...stylex.props(sx.section, sx.spacious)}>
      <div {...stylex.props(sx.row)}>
        <h2 {...stylex.props(sx.heading)}><span {...stylex.props(sx.captionIcon)}><Icon name="subtitle" size={15} /></span>Phụ đề</h2>
        {driftIndex >= 0 && <><span {...stylex.props(sx.meta)}>Đoạn lệch {driftIndex + 1}/{drift.length}</span>
        <Button variant="icon" aria-label="Đoạn lệch trước" title="Đoạn lệch trước (⇧F)" onClick={() => navigate(-1)} icon={<Icon name="arrow-left-2" size={14} />} />
        <Button variant="icon" aria-label="Đoạn lệch sau" title="Đoạn lệch sau (F)" onClick={() => navigate(1)} icon={<Icon name="arrow-r" size={14} />} /></>}
      </div>
      <span {...stylex.props(sx.meta)}>{panelTime(start / fps)} · đoạn {preciseTime(start / fps)}–{preciseTime(end / fps)}</span>
      <div role="status" {...stylex.props(sx.status, passed && sx.statusOk)}><Icon name={passed ? 'tick-circle' : 'info-circle'} size={14} />{aligning ? 'Đang khớp theo giọng…' : passed ? 'Đã khớp tiếng · lệch không đáng kể' : matching?.offsetSec != null ? `Chữ hiện ${matching.offsetSec > 0 ? 'muộn' : 'sớm'} ${Math.abs(matching.offsetSec).toLocaleString('vi', { maximumFractionDigits: 1 })} giây so với tiếng` : matching ? 'Phụ đề lệch với giọng' : item.words?.length ? item.wordsEstimated ? 'Mốc từng từ đang được ước lượng' : 'Đã có mốc từng từ' : 'Chưa có mốc từng từ'}</div>
      <CaptionTiming item={item} startFrame={start} fps={fps} aligned={passed} />
      <Button block variant={passed ? 'ghost' : 'primary'} aria-disabled={passed || aligning} title={passed ? 'Đoạn này đã khớp theo giọng. Sửa chữ hoặc thời lượng sẽ cần khớp lại.' : aligning ? 'Đang khớp theo giọng; chờ xong.' : 'Khớp mốc phụ đề theo tiếng'} pending={aligning} pendingLabel="Đang khớp theo giọng…" onClick={() => { void align() }} icon={<Icon name="magic" size={15} />}>{passed ? 'Đã khớp theo giọng' : 'Khớp theo giọng'}</Button>
      <div {...stylex.props(sx.row)}><span {...stylex.props(sx.grow, sx.muted)}>Dời cả đoạn</span><Button size="sm" title="Sớm hơn 0,1 giây (,)" onClick={() => shift(-1)}>−0,1 s</Button><Button size="sm" title="Muộn hơn 0,1 giây (.)" onClick={() => shift(1)}>+0,1 s</Button></div>
      <Button variant="quiet" size="sm" xstyle={sx.quietButton} title="Nghe lại đoạn này (L)" onClick={replay} icon={<Icon name="play" size={14} />}>Nghe lại đoạn này</Button>
      {error && <p role="alert" {...stylex.props(sx.error)}>{error}</p>}
    </section>
    <section {...stylex.props(sx.section)}><label htmlFor="v13-caption-text" {...stylex.props(sx.muted)}>Chữ của đoạn</label><textarea id="v13-caption-text" {...stylex.props(sx.input)} rows={2} value={draft ?? text} onChange={event => setDraft(event.target.value)} onBlur={commit} onKeyDown={event => { if (event.key === 'Escape') { setDraft(null); event.currentTarget.blur() } if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) { event.preventDefault(); commit() } }} /><p {...stylex.props(sx.help)}>Sửa chữ không đổi tiếng. Chữ karaoke tô theo mốc từng từ; cỡ chữ và màu theo mẫu video.</p></section>
  </>
}

function SceneInspector({ item, jobId, images, imageProgress, imageQuota, beforeMutation, onRedrawn }: Pick<InspectorProps, 'jobId' | 'images' | 'imageProgress' | 'imageQuota' | 'beforeMutation' | 'onProjectChanged'> & { item: ImageItem; onRedrawn?: () => void | Promise<void> }) {
  const items = useItemsStore(state => state.items)
  const fps = useTimelineSettingsStore(state => state.fps)
  const metadata = useProjectStore(state => state.currentProject?.metadata)
  const projectId = useProjectStore(state => state.currentProject?.id)
  const transitions = useTransitionsStore(state => state.transitions)
  const keyframes = useKeyframesStore(state => state.keyframes.find(entry => entry.itemId === item.id))
  const [advanced, setAdvanced] = useState(false)
  const [error, setError] = useState('')
  const image = images ? sceneImageFor(images, item, projectId) : undefined
  const sceneItems = items.filter(entry => entry.trackId === item.trackId && entry.type === 'image').toSorted((a, b) => a.from - b.from)
  const number = sceneItems.findIndex(entry => entry.id === item.id) + 1
  const next = sceneItems.find(entry => entry.from === item.from + item.durationInFrames)
  const transition = transitions.find(entry => entry.leftClipId === item.id && entry.rightClipId === next?.id)
  const fitted = computeInitialTransform(item.sourceWidth || metadata?.width || 1920, item.sourceHeight || metadata?.height || 1080, metadata?.width || 1920, metadata?.height || 1080)
  const transform = { ...fitted, ...item.transform }
  const modifiers = item.motionModifiers?.filter(entry => entry.enabled) ?? []
  const animated = keyframes?.properties.filter(entry => entry.keyframes.length > 1 && ['x', 'y', 'width', 'height', 'rotation'].includes(entry.property)) ?? []
  const motion = modifiers.some(entry => entry.type === 'float-drift') && modifiers.some(entry => entry.type === 'breath-pulse')
    ? 'drift'
    : !modifiers.length && animated.some(entry => entry.property === 'width') && animated.some(entry => entry.property === 'height') && animated.every(entry => entry.property === 'width' || entry.property === 'height')
      ? 'zoom'
      : !modifiers.length && !animated.length && !item.motionLayers?.some(entry => entry.enabled) && !keyframes?.vectorProperties?.some(lane => lane.keyframes.length > 1)
        ? 'none' : 'custom'
  const transitionMode = !transition ? 'cut' : ['fade', 'dissolve'].includes(transition.presentation) ? 'fade' : transition.presentation === 'slide' ? 'slide' : 'custom'
  const run = (commands: SerializedCommand[]) => { try { editorCommand(commands); setError('') } catch (error) { setError(panelError(error)) } }
  const setMotion = (value: 'none' | 'zoom' | 'drift') => {
    const commands: SerializedCommand[] = [{ type: 'item.update', params: { id: item.id, updates: { motionModifiers: [], motionLayers: [] } } }, ...(['x', 'y', 'width', 'height', 'rotation'] as const).map(property => ({ type: 'keyframe.removeForProperty', params: { itemId: item.id, property } }))]
    for (const lane of keyframes?.vectorProperties ?? []) {
      if (lane.property === 'position' || lane.property === 'scale') {
        for (const keyframe of lane.keyframes) commands.push({ type: 'keyframe.vectorRemove', params: { itemId: item.id, property: lane.property, keyframeId: keyframe.id } })
      }
    }
    if (value === 'zoom') {
      if (item.durationInFrames < 2) { setError('Cảnh quá ngắn để tạo chuyển động.'); return }
      commands.push({ type: 'keyframe.addMany', params: { payloads: (['width', 'height'] as const).flatMap(property => [
        { itemId: item.id, property, frame: 0, value: transform[property] ?? fitted[property] ?? 1, easing: 'linear' },
        { itemId: item.id, property, frame: item.durationInFrames - 1, value: (transform[property] ?? fitted[property] ?? 1) * 1.1, easing: 'linear' },
      ]) } })
    }
    if (value === 'drift') commands.push({ type: 'item.update', params: { id: item.id, updates: { motionModifiers: [{ id: crypto.randomUUID(), type: 'float-drift', enabled: true, amplitude: 0.3, frequency: 0.08, phaseFrames: 0, seed: 1 }, { id: crypto.randomUUID(), type: 'breath-pulse', enabled: true, amplitude: 0.3, frequency: 0.08, phaseFrames: 0, seed: 1 }] } } })
    run(commands)
  }
  const setTransition = (value: 'cut' | 'fade' | 'slide') => {
    if (!next) { setError('Cần hai cảnh liền kề để đổi chuyển cảnh.'); return }
    if (value === 'cut') { if (transition) run([{ type: 'transition.remove', params: { id: transition.id } }]); return }
    const durationInFrames = Math.min(Math.round(1.2 * fps), item.durationInFrames - 1, next.durationInFrames - 1)
    if (durationInFrames < 1) { setError('Cảnh quá ngắn để chuyển cảnh.'); return }
    run([transition ? { type: 'transition.update', params: { id: transition.id, updates: { presentation: value, durationInFrames, ...(value === 'slide' ? { direction: 'from-left' } : {}) } } } : { type: 'transition.add', params: { leftClipId: item.id, rightClipId: next.id, type: 'crossfade', presentation: value, durationInFrames, ...(value === 'slide' ? { direction: 'from-left' } : {}) } }])
  }
  const updateTransform = (patch: Partial<TransformProperties>) => { if (Object.values(patch).some(value => typeof value === 'number' && !Number.isFinite(value))) { setError('Giá trị phải là số hợp lệ.'); return }; run([{ type: 'item.updateTransform', params: { id: item.id, transform: patch } }]) }
  const scalarKeys = keyframes?.properties.flatMap(property => property.keyframes.map(keyframe => ({ property: property.property, ...keyframe }))) ?? []
  return <>
    <SceneImagePanel
      item={item}
      number={number}
      jobId={jobId}
      image={image}
      images={images}
      imageProgress={imageProgress}
      progress={image ? imageProgress?.[image.id] : undefined}
      quota={imageQuota}
      beforeMutation={beforeMutation}
      onRedrawn={onRedrawn}
    />
    <section {...stylex.props(sx.section)}>
      <h3 {...stylex.props(sx.subheading)}>Chuyển động</h3>
      <Segmented size="sm" label="Chuyển động của ảnh" value={motion}
        options={[
          { id: 'none', label: 'Đứng yên' },
          { id: 'zoom', label: 'Zoom chậm' },
          { id: 'drift', label: 'Trôi + co giãn' },
        ]}
        onChange={value => { if (value === 'none' || value === 'zoom' || value === 'drift') setMotion(value) }} />
    </section>
    {next && <section {...stylex.props(sx.section)}>
      <h3 {...stylex.props(sx.subheading)}>Sang cảnh {number + 1}</h3>
      <Segmented size="sm" label={`Chuyển sang cảnh ${number + 1}`} value={transitionMode}
        options={[
          { id: 'cut', label: 'Cắt' },
          { id: 'fade', label: `Mờ ${transition && ['fade', 'dissolve'].includes(transition.presentation) ? (transition.durationInFrames / fps).toLocaleString('vi', { maximumFractionDigits: 1 }) : '1,2'} s` },
          { id: 'slide', label: 'Trượt' },
        ]}
        onChange={value => { if (value === 'cut' || value === 'fade' || value === 'slide') setTransition(value) }} />
      <p {...stylex.props(sx.help)}>Theo mẫu video. Đổi ở đây chỉ cho chỗ này.</p>
    </section>}
    <section {...stylex.props(sx.section, sx.noBorder)}>
      <Button variant="quiet" size="sm" xstyle={sx.quietButton} aria-expanded={advanced} aria-describedby="v13-scene-advanced-help"
        onClick={() => setAdvanced(!advanced)} icon={<Icon name={advanced ? 'arrow-up-2' : 'arrow-d'} size={13} />}>
        Nâng cao
      </Button>
      <p id="v13-scene-advanced-help" {...stylex.props(sx.help)}>· vị trí, kích thước, độ trong suốt, điểm mốc</p>
      {advanced && <>
        <div {...stylex.props(sx.twoColumns)}>
          {([['x', 'X'], ['y', 'Y'], ['rotation', 'Xoay']] as const).map(([property, label]) => <label key={property} {...stylex.props(sx.field)}>
            {label}<input type="number" {...stylex.props(sx.numeric)} aria-label={label} value={transform[property] ?? 0}
              onChange={event => { if (event.currentTarget.value !== '') updateTransform({ [property]: event.currentTarget.valueAsNumber }) }} />
          </label>)}
          <label {...stylex.props(sx.field)}>Tỉ lệ<input type="number" min={1} step={1} {...stylex.props(sx.numeric)} aria-label="Tỉ lệ ảnh"
            value={Math.round((transform.width ?? fitted.width ?? 1) / (fitted.width || 1) * 100)}
            onChange={event => {
              const percent = event.currentTarget.valueAsNumber
              if (Number.isFinite(percent) && percent > 0) updateTransform({ width: (fitted.width || 1) * percent / 100, height: (fitted.height || 1) * percent / 100 })
            }} /></label>
        </div>
        <label {...stylex.props(sx.field)}>Độ trong suốt
          <input type="range" {...stylex.props(sx.slider)} min={0} max={100} value={Math.round((transform.opacity ?? 1) * 100)}
            aria-label={`Độ trong suốt ảnh cảnh ${number}`} onChange={event => updateTransform({ opacity: Number(event.target.value) / 100 })} />
          <span>{Math.round((transform.opacity ?? 1) * 100)}%</span>
        </label>
        <span {...stylex.props(sx.meta)}>Điểm mốc {scalarKeys.length + (keyframes?.vectorProperties?.reduce((sum, property) => sum + property.keyframes.length, 0) ?? 0)}</span>
        {scalarKeys.map(keyframe => <label key={keyframe.id} {...stylex.props(sx.field)}>
          {panelTime((item.from + keyframe.frame) / fps)} · {keyframe.property}
          <input type="number" aria-label={`Điểm mốc ${keyframe.property} tại ${keyframe.frame}`} {...stylex.props(sx.numeric)} value={keyframe.value}
            onChange={event => {
              if (Number.isFinite(event.currentTarget.valueAsNumber)) run([{ type: 'keyframe.update', params: { itemId: item.id, property: keyframe.property, keyframeId: keyframe.id, updates: { value: event.currentTarget.valueAsNumber } } }])
            }} />
        </label>)}
      </>}
    </section>
  </>
}

function BackgroundInspector({ item, actions, jobId }: { item: VideoItem; actions: BackdropActions; jobId?: string }) {
  const fps = useTimelineSettingsStore(state => state.fps)
  const [error, setError] = useState('')
  const [choosing, setChoosing] = useState(false)
  const sourceFps = item.sourceFps || fps
  const source = backdropItemFor(item, actions.backdrop)
  const start = source?.startSec ?? (item.sourceStart ?? 0) / sourceFps
  const length = source ? source.endSec - source.startSec : item.durationInFrames / fps * (item.speed ?? 1)
  const [sourceStart, setSourceStart] = useState(start)
  const remove = async () => {
    if (jobId) {
      if (!source) { setError('Chưa tìm thấy clip trong nguồn nền của video.'); return }
      await actions.mutateBackdrop({ op: 'remove', itemId: source.id })
      return
    }
    try {
      editorCommand([{ type: 'item.remove', params: { itemIds: [item.id], linkedSelection: false } }])
      setError('')
    } catch (error) { setError(panelError(error)) }
  }
  const choose = async () => {
    const duration = source?.videoDurationSec ?? (item.sourceDuration === undefined ? undefined : item.sourceDuration / sourceFps)
    if (!Number.isFinite(sourceStart) || sourceStart < 0 || duration !== undefined && duration !== null && sourceStart + length > duration) {
      setError('Đoạn chọn phải nằm trong thời lượng clip nguồn.')
      return
    }
    if (jobId) {
      if (!source) { setError('Chưa tìm thấy clip trong nguồn nền của video.'); return }
      await actions.mutateBackdrop({ op: 'swap', itemId: source.id, channelId: source.channelId, videoId: source.videoId, startSec: sourceStart, endSec: sourceStart + length })
      return
    }
    try {
      editorCommand([{ type: 'item.update', params: { id: item.id, updates: { sourceStart: Math.round(sourceStart * sourceFps), sourceEnd: Math.round((sourceStart + length) * sourceFps) }, clearFields: ['trimStart', 'trimEnd'] } }])
      setChoosing(false); setError('')
    } catch (error) { setError(panelError(error)) }
  }
  return <section {...stylex.props(sx.section, sx.spacious)}>
    <h2 {...stylex.props(sx.heading)}>
      <span {...stylex.props(sx.bgIcon)}><Icon name="video-play" size={15} /></span>
      <span {...stylex.props(sx.headingText)} title={item.label}>Clip nền · {item.label}</span>
    </h2>
    <span {...stylex.props(sx.meta)}>
      {source && `${source.kind} · `}trong video {panelTime(item.from / fps)}–{panelTime((item.from + item.durationInFrames) / fps)} · lấy đoạn {panelTime(start)}–{panelTime(start + length)}
    </span>
    <div {...stylex.props(sx.thumbnail)}><MediaAsset mediaId={item.mediaId} src={item.thumbnailUrl} label={item.label} /></div>
    <div {...stylex.props(sx.commandGrid)}>
      <Button disabled={!source || !actions.backdrop?.randomPicks} title={!source ? 'Chưa tìm thấy clip nguồn của cảnh nền này.' : !actions.backdrop?.randomPicks ? 'Video đang tắt đổi ngẫu nhiên trong Mẫu video › Nền & hiệu ứng.' : 'Đổi clip nền ngẫu nhiên cho riêng video này'} pending={actions.pending} onClick={() => { if (source) void actions.mutateBackdrop({ op: 'reroll', itemId: source.id }) }} icon={<Icon name="refresh" size={15} />}>Đổi ngẫu nhiên</Button>
      <Button disabled={actions.pending || Boolean(jobId && !source)} title={actions.pending ? 'Đang dựng lại nền; chờ xong.' : jobId && !source ? 'Chưa tìm thấy clip nguồn của cảnh nền này.' : 'Chọn đoạn khác từ clip nguồn'} onClick={() => { setSourceStart(start); setError(''); setChoosing(true) }} icon={<Icon name="scissor" size={15} />}>Chọn đoạn khác</Button>
    </div>
    <Button variant="quiet" size="sm" disabled={actions.pending || Boolean(jobId && !source)} xstyle={sx.quietButton} onClick={() => { void remove() }} icon={<Icon name="trash" size={14} />}>Bỏ clip này</Button>
    <p {...stylex.props(sx.help)}>Chỉ đổi cho video này. Bỏ clip thì nền lặp lại 1:00 ở cuối — Kiểm định sẽ báo nếu nền không phủ kín.</p>
    {actions.pending && <p role="status" {...stylex.props(sx.meta)}>Đang dựng lại nền…</p>}
    {error && <p role="alert" {...stylex.props(sx.error)}>{error}</p>}
    {choosing && <Dialog title="Chọn đoạn khác" padBody={false} onClose={actions.pending ? undefined : () => setChoosing(false)} footer={<>
      <Button disabled={actions.pending} onClick={() => setChoosing(false)}>Đóng</Button>
      <Button variant="primary" pending={actions.pending} onClick={() => { void choose() }}>Dùng đoạn này</Button>
    </>}>
      <div {...stylex.props(sx.dialog)}>
        <label {...stylex.props(sx.field)}>Bắt đầu (giây)<input {...stylex.props(sx.numeric)} type="number" min={0} step={0.1} value={Number.isFinite(sourceStart) ? sourceStart : ''} onChange={event => setSourceStart(event.currentTarget.valueAsNumber)} /></label>
        <span {...stylex.props(sx.meta)}>Dài {length.toLocaleString('vi')} giây</span>
        {error && <p role="alert" {...stylex.props(sx.error)}>{error}</p>}
        {actions.error && <p role="alert" {...stylex.props(sx.error)}>{actions.error}</p>}
      </div>
    </Dialog>}
  </section>
}

export function Inspector(props: InspectorProps) {
  const backdropActions = useBackdropActions(props)
  const selectedId = useSelectionStore(state => state.selectedItemIds[0])
  const item = useItemsStore(state => selectedId ? state.itemById[selectedId] : undefined)
  const tracks = useItemsStore(state => state.tracks)
  const items = useItemsStore(state => state.items)
  const project = useProjectStore(state => state.currentProject)
  const fps = useTimelineSettingsStore(state => state.fps)
  const background = item?.type === 'video' && tracks.some(track => track.id === item.trackId && track.templateSlot === 'background-video')
  const duration = items.reduce((end, entry) => Math.max(end, entry.from + entry.durationInFrames), 0)
  const captionKey = item && isCaption(item) ? `${item.id}:${item.type === 'subtitle' ? (props.selectedRange === null ? '' : props.ranges[props.selectedRange]?.cueId ?? '') : ''}` : ''
  const inspectorElement = useRef<HTMLElement>(null)
  useEffect(() => {
    if (inspectorElement.current) inspectorElement.current.scrollTop = 0
  }, [selectedId, props.selectedRange])
  return <aside ref={inspectorElement} {...stylex.props(sx.panel)} aria-label={item && isCaption(item) ? 'Phụ đề' : item?.type === 'image' ? 'Cảnh' : background ? 'Clip nền' : 'Video'} data-shortcut-scope="inspector">
    {backdropActions.error && <p role="alert" {...stylex.props(sx.error)}>{backdropActions.error}</p>}
    {item && isCaption(item) ? <CaptionInspector key={captionKey} {...props} item={item} /> : item?.type === 'image' ? <SceneInspector key={item.id} item={item} jobId={props.jobId} images={props.images} imageProgress={props.imageProgress} beforeMutation={props.beforeMutation} onProjectChanged={props.onProjectChanged} onRedrawn={props.onRedrawn} /> : item?.type === 'video' && background ? <BackgroundInspector key={item.id} item={item} jobId={props.jobId} actions={backdropActions} /> : <><section {...stylex.props(sx.section)}><h2 {...stylex.props(sx.heading)}>Video</h2><p {...stylex.props(sx.muted)}>Chọn một cảnh, đoạn phụ đề hay clip nền trên dòng thời gian để sửa. Bấm Cần sửa ở bên trái để xem việc còn lại.</p></section><section {...stylex.props(sx.section)}><h3 {...stylex.props(sx.subheading)}>Khung hình</h3><span {...stylex.props(sx.meta)}>{project?.metadata.width ?? 1920}×{project?.metadata.height ?? 1080} · {fps} fps · cả video {panelTime(duration / fps)}</span><p {...stylex.props(sx.help)}>Cố định cho video kênh: Kiểm định chỉ nhận bản xuất 1080p30 đủ thời lượng.</p></section></>}
  </aside>
}
