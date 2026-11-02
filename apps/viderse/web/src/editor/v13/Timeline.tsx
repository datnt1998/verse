import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type PointerEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon, type IconName } from '@noverse/ui'
import { toast } from '@/editor/ui/toast'
import { executeCommands } from '@editor/commands'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import { useZoomStore } from '@editor/features/timeline/stores/zoom-store'
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
import { splitItem, toggleSnap } from '@editor/features/timeline/stores/timeline-actions'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useSelectionStore } from '@editor/shared/state/selection'
import { useEditorStore } from '@editor/shared/state/editor'
import { expandSelectionWithLinkedItems } from '@editor/features/timeline/utils/linked-items'
import type { TimelineItem, TimelineTrack } from '@editor/types/timeline'
import { waveformCache, type CachedWaveform } from '@editor/features/timeline/services/waveform-cache'
import { resolveMediaUrl } from '@editor/features/media-library/utils/media-resolver'
import { useShortcutAction } from '../ui/shortcuts'
import { clampFrame, fitZoomLevel, indexItems, rulerKeyFrame, rulerStep, snapFrame, timecode, visibleItems } from '../ui/timeline/geometry'
import { timelineStyles as s } from './timeline.stylex'

export interface TimelineRange {
  start: number
  end: number
  checkId: string
  status: string
  cueId?: string
  itemId?: string
}
export interface TimelineProps {
  ranges: readonly TimelineRange[]
  selectedRange: number
  onSelectRange: (index: number) => void
  drawingClipId?: string
}
type Kind = 'caption' | 'scene' | 'overlay' | 'video' | 'audio'
interface Clip {
  id: string
  item: TimelineItem
  cueId?: string
  from: number
  durationInFrames: number
  trackId: string
  label: string
  kind: Kind
}
const HEADER = 168
const MAX_ZOOM = 2
const icons: Record<Kind, IconName> = { caption: 'subtitle', scene: 'gallery', overlay: 'magic', video: 'video-play', audio: 'volume' }
const iconStyles = { caption: s.captionIcon, scene: s.sceneIcon, overlay: s.overlayIcon, video: s.videoIcon, audio: s.audioIcon }
function itemKind(item?: TimelineItem): Kind {
  if (!item) return 'overlay'
  if (item.type === 'subtitle' || (item.type === 'text' && item.textRole === 'caption')) return 'caption'
  if (item.type === 'image') return 'scene'
  if (item.type === 'video') return 'video'
  if (item.type === 'audio') return 'audio'
  return 'overlay'
}
function AudioWaveform({ item, fps, scale, left, width }: { item: TimelineItem; fps: number; scale: number; left: number; width: number }) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const [waveform, setWaveform] = useState<CachedWaveform | null>(null)
  const [error, setError] = useState(false)
  const mediaId = item.mediaId
  const src = 'src' in item ? item.src : undefined
  useEffect(() => {
    if (!mediaId || !src) return
    let active = true
    setWaveform(null); setError(false)
    const unsubscribe = waveformCache.subscribe(mediaId, value => { if (active) setWaveform(value) })
    const load = async () => {
      try {
        const url = await resolveMediaUrl(mediaId)
        const value = await waveformCache.getWaveform(mediaId, url || src)
        if (active) setWaveform(value)
      } catch { if (active) setError(true) }
    }
    void load()
    return () => { active = false; unsubscribe() }
  }, [mediaId, src])
  useEffect(() => {
    const node = canvas.current
    if (!node || !waveform || width <= 0) return
    const ratio = window.devicePixelRatio || 1
    node.width = Math.ceil(width * ratio); node.height = 20 * ratio
    const context = node.getContext('2d')
    if (!context) return
    context.scale(ratio, ratio)
    context.strokeStyle = getComputedStyle(node).color
    context.lineWidth = 1
    const channels = waveform.stereo ? 2 : 1
    const sourceStart = (item.sourceStart ?? item.trimStart ?? 0) / (item.sourceFps ?? fps)
    const speed = item.speed ?? 1
    context.beginPath()
    for (let x = 0; x < width; x += 2) {
      const time = sourceStart + (left + x) / scale / fps * speed
      const index = Math.floor(time * waveform.sampleRate) * channels
      const end = Math.min(waveform.peaks.length, index + Math.max(channels, Math.ceil(2 / scale / fps * speed * waveform.sampleRate) * channels))
      let peak = 0
      for (let n = Math.max(0, index); n < end; n++) peak = Math.max(peak, Math.abs(waveform.peaks[n]))
      const amplitude = Math.min(1, peak / Math.max(waveform.maxPeak, 0.01)) * 9
      context.moveTo(x, 10 - amplitude); context.lineTo(x, 10 + amplitude)
    }
    context.stroke()
  }, [waveform, fps, scale, left, width, item.sourceStart, item.trimStart, item.sourceFps, item.speed])
  return error ? <span title="Chưa đọc được sóng âm." {...stylex.props(s.waveformError)}>Chưa đọc được sóng âm.</span> : <canvas ref={canvas} aria-hidden {...stylex.props(s.waveform(left, width))} />
}
function rulerLabel(frame: number, fps: number): string {
  const seconds = Math.max(0, Math.floor(frame / fps))
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const rest = String(seconds % 60).padStart(2, '0')
  return hours ? `${hours}:${String(minutes).padStart(2, '0')}:${rest}` : `${minutes}:${rest}`
}
function Playhead({ scale }: { scale: number }) {
  const frame = usePlaybackStore(state => state.previewFrame ?? state.currentFrame)
  return <div aria-hidden {...stylex.props(s.playhead(HEADER + frame * scale))}><span {...stylex.props(s.playheadCap)} /></div>
}
function Ruler({ duration, fps, scale, left, width, markers, onPointerDown, onPointerMove, onPointerUp }: {
  duration: number; fps: number; scale: number; left: number; width: number; markers: readonly number[]
  onPointerDown: (event: PointerEvent<HTMLDivElement>) => void
  onPointerMove: (event: PointerEvent<HTMLDivElement>) => void
  onPointerUp: (event: PointerEvent<HTMLDivElement>) => void
}) {
  const frame = usePlaybackStore(state => state.currentFrame)
  const step = rulerStep(fps, scale)
  const ticks = []
  for (let tick = Math.floor(left / scale / step) * step; tick < Math.min(duration, (left + width) / scale); tick += step) ticks.push(tick)
  const minorStep = step >= 5 ? Math.max(1, Math.round(step / 5)) : 0
  const minorTicks = []
  if (minorStep) for (let tick = Math.floor(left / scale / minorStep) * minorStep; tick < Math.min(duration, (left + width) / scale); tick += minorStep) minorTicks.push(tick)
  return <div role="slider" tabIndex={0} aria-label="Vị trí phát" aria-valuemin={0} aria-valuemax={Math.max(0, duration - 1)} aria-valuenow={Math.min(frame, duration - 1)} aria-valuetext={timecode(frame, fps)} {...stylex.props(s.ruler)} onPointerDown={onPointerDown} onPointerMove={onPointerMove} onPointerUp={onPointerUp} onPointerCancel={onPointerUp} onKeyDown={event => {
    if (event.metaKey || event.ctrlKey || event.altKey) return
    const next = rulerKeyFrame(event.key, frame, fps, duration, event.shiftKey)
    if (next === null) return
    event.preventDefault(); event.stopPropagation()
    const playback = usePlaybackStore.getState()
    playback.pause(); playback.finishScrub(next)
  }}>{minorTicks.map(tick => <span key={`m${tick}`} aria-hidden {...stylex.props(s.tickMinor(tick * scale))} />)}{ticks.map(tick => <span key={tick} {...stylex.props(s.tick(tick * scale))}><span {...stylex.props(s.tickLabel)}>{rulerLabel(tick, fps)}</span></span>)}{markers.map((marker, index) => <span key={index} aria-hidden {...stylex.props(s.rangeMarker(marker * fps * scale))} />)}<span aria-hidden {...stylex.props(s.playhead(frame * scale))}><span {...stylex.props(s.playheadCap)} /></span></div>
}

export function Timeline({ ranges, selectedRange, onSelectRange, drawingClipId }: TimelineProps) {
  const items = useItemsStore(state => state.items)
  const tracks = useItemsStore(state => state.tracks)
  const fps = useTimelineSettingsStore(state => state.fps)
  const snapEnabled = useTimelineSettingsStore(state => state.snapEnabled)
  const selectedIds = useSelectionStore(state => state.selectedItemIds)
  const zoom = useZoomStore(state => state.level)
  const scroll = useRef<HTMLDivElement>(null)
  const initialized = useRef(false)
  const zoomScroll = useRef<number | null>(null)
  const dragging = useRef<number | null>(null)
  const [viewport, setViewport] = useState({ width: 0, left: 0 })
  const orderedTracks = useMemo(() => [...tracks].sort((a, b) => a.order - b.order), [tracks])
  const duration = useMemo(() => items.reduce((end, item) => Math.max(end, item.from + item.durationInFrames), 1), [items])
  const clips = useMemo(() => items.flatMap((item): Clip[] => {
    const kind = itemKind(item)
    if (item.type !== 'subtitle') return [{ id: item.id, item, from: item.from, durationInFrames: item.durationInFrames, trackId: item.trackId, label: item.label, kind }]
    return item.cues.flatMap(cue => {
      const from = Math.max(item.from, item.from + Math.round(cue.startSeconds * fps))
      const end = Math.min(item.from + item.durationInFrames, item.from + Math.round(cue.endSeconds * fps))
      return end > from ? [{ id: `${item.id}:${cue.id}`, item, cueId: cue.id, from, durationInFrames: end - from, trackId: item.trackId, label: cue.text, kind }] : []
    })
  }), [items, fps])
  const indexes = useMemo(() => indexItems(clips), [clips])
  const selected = useMemo(() => new Set(selectedIds), [selectedIds])
  const selectedCueClips = useMemo(() => clips.filter(clip => clip.cueId && selected.has(clip.item.id)), [clips, selected])
  const activeCueId = usePlaybackStore(state => selectedCueClips.find(clip => state.currentFrame >= clip.from && state.currentFrame < clip.from + clip.durationInFrames)?.id)
  const keyframes = useKeyframesStore(state => state.keyframes)
  const selectedScene = items.find(item => item.type === 'image' && selected.has(item.id))
  const keyframeFrames = useMemo(() => {
    if (!selectedScene) return []
    const entry = keyframes.find(record => record.itemId === selectedScene.id)
    const frames = new Set<number>()
    for (const property of entry?.properties ?? []) for (const keyframe of property.keyframes) frames.add(keyframe.frame)
    for (const property of entry?.vectorProperties ?? []) for (const keyframe of property.keyframes) frames.add(keyframe.frame)
    return [...frames].filter(frame => frame > 0 && frame < selectedScene.durationInFrames).toSorted((a, b) => a - b)
  }, [keyframes, selectedScene])
  const failures = useMemo(() => ranges.flatMap((range, index) => Number.isFinite(range.start) && Number.isFinite(range.end) && range.end > range.start && !['pass', 'passed', 'aligned'].includes(range.status) ? [{ range, index }] : []), [ranges])
  const rangeMarkers = useMemo(() => failures.map(({ range }) => range.start), [failures])
  const targets = useMemo(() => [0, ...clips.flatMap(clip => [clip.from, clip.from + clip.durationInFrames])], [clips])
  const available = Math.max(1, viewport.width - HEADER)
  const fitLevel = fitZoomLevel(available, duration, fps, MAX_ZOOM)
  const minimumZoom = Math.min(0.01, fitLevel)
  const scale = Math.max(minimumZoom, zoom) * 100 / fps
  const contentWidth = Math.max(viewport.width, HEADER + duration * scale)
  useLayoutEffect(() => {
    if (zoomScroll.current === null || !scroll.current) return
    scroll.current.scrollLeft = zoomScroll.current
    zoomScroll.current = null
  }, [contentWidth, scale])
  const position = failures.findIndex(entry => entry.index === selectedRange)
  const syncViewport = useCallback(() => {
    const node = scroll.current
    if (!node) return
    setViewport(current => current.width === node.clientWidth && current.left === node.scrollLeft ? current : { width: node.clientWidth, left: node.scrollLeft })
  }, [])
  useEffect(() => {
    const node = scroll.current
    if (!node) return
    syncViewport()
    const observer = new ResizeObserver(syncViewport)
    observer.observe(node)
    return () => observer.disconnect()
  }, [syncViewport])
  useEffect(() => {
    if (initialized.current || viewport.width <= HEADER) return
    initialized.current = true
    useZoomStore.getState().setZoomLevelSynchronized(fitLevel)
  }, [fitLevel, viewport.width])
  const seek = (frame: number) => {
    const playback = usePlaybackStore.getState()
    playback.pause(); playback.finishScrub(clampFrame(frame, duration - 1))
  }
  const reveal = (frame: number) => {
    const node = scroll.current
    if (node && (frame * scale < node.scrollLeft || frame * scale > node.scrollLeft + available)) node.scrollLeft = Math.max(0, frame * scale - available / 2)
  }
  useEffect(() => {
    const range = ranges[selectedRange]
    if (range && Number.isFinite(range.start)) reveal(range.start * fps)
    // Keep a selected range visible when the user zooms or switches failures.
  }, [ranges, selectedRange, scale, available, fps])
  const selectRange = (index: number) => {
    const range = ranges[index]
    if (!range) return
    onSelectRange(index)
    seek(range.start * fps); reveal(range.start * fps)
    const clip = clips.find(entry => (!range.itemId || range.itemId === entry.item.id) && (!range.cueId || range.cueId === entry.cueId) && entry.from / fps < range.end && (entry.from + entry.durationInFrames) / fps > range.start && (range.itemId || range.cueId || entry.kind === 'caption'))
    if (clip) useSelectionStore.getState().selectItems([clip.item.id])
  }
  const navigate = (direction: number) => {
    if (!failures.length) return
    const next = position < 0 ? direction > 0 ? 0 : failures.length - 1 : (position + direction + failures.length) % failures.length
    selectRange(failures[next].index)
  }
  const editableIds = () => {
    const ids = useSelectionStore.getState().selectedItemIds
    const expanded = useEditorStore.getState().linkedSelectionEnabled ? expandSelectionWithLinkedItems(items, ids) : ids
    if (items.some(item => expanded.includes(item.id) && tracks.find(track => track.id === item.trackId)?.locked)) {
      toast.error('Rãnh đang khoá. Mở khoá rãnh rồi thử lại.'); return []
    }
    return expanded
  }
  const split = () => {
    const frame = usePlaybackStore.getState().currentFrame
    const ids = editableIds()
    const target = items.find(item => ids.includes(item.id) && item.from < frame && item.from + item.durationInFrames > frame)
    if (!target) { toast('Chọn một phần có vị trí phát nằm bên trong để tách.'); return }
    const result = splitItem(target.id, frame)
    if (!result) { toast.error('Không tách được phần đang chọn.'); return }
    useSelectionStore.getState().selectItems([result.rightItem.id])
  }
  const remove = () => {
    const ids = editableIds()
    if (!ids.length) return
    const result = executeCommands([{ type: 'item.remove', params: { itemIds: ids, linkedSelection: useEditorStore.getState().linkedSelectionEnabled } }])
    if (!result.ok) { toast.error('Không xoá được phần đang chọn.'); return }
    useSelectionStore.getState().clearItemSelection()
  }
  const snap = () => {
    try { toggleSnap() } catch { toast.error('Không đổi được chế độ bắt dính.') }
  }
  const changeTrack = (track: TimelineTrack, updates: Partial<TimelineTrack>) => {
    const result = executeCommands([{ type: 'track.update', params: { id: track.id, updates } }])
    if (!result.ok) toast.error('Không đổi được rãnh này.')
  }
  const setZoom = (level: number) => {
    if (!Number.isFinite(level)) return
    const frame = (viewport.left + available / 2) / scale
    const next = Math.max(minimumZoom, Math.min(MAX_ZOOM, level))
    zoomScroll.current = Math.max(0, frame * next * 100 / fps - available / 2)
    useZoomStore.getState().setZoomLevelSynchronized(next)
  }
  const fit = () => {
    zoomScroll.current = 0
    useZoomStore.getState().setZoomLevelSynchronized(fitLevel)
    if (scroll.current) scroll.current.scrollLeft = 0
  }
  useShortcutAction({ id: 'split', label: 'Tách tại vị trí phát', scope: 'timeline', run: split })
  useShortcutAction({ id: 'delete', label: 'Xoá phần đang chọn', scope: 'timeline', run: remove })
  useShortcutAction({ id: 'snap', label: 'Bắt dính', scope: 'timeline', run: snap })
  useShortcutAction({ id: 'zoom-in', label: 'Phóng to dòng thời gian', scope: 'timeline', run: () => setZoom(zoom * 1.25) })
  useShortcutAction({ id: 'zoom-out', label: 'Thu nhỏ dòng thời gian', scope: 'timeline', run: () => setZoom(zoom / 1.25) })
  useShortcutAction({ id: 'zoom-fit', label: 'Vừa cả video', scope: 'timeline', run: fit })
  useShortcutAction({ id: 'range-next', label: 'Đoạn lệch sau', scope: 'timeline', bindings: ['KeyF'], run: () => navigate(1) })
  useShortcutAction({ id: 'range-previous', label: 'Đoạn lệch trước', scope: 'timeline', bindings: ['shift+KeyF'], run: () => navigate(-1) })
  const pointerFrame = (event: PointerEvent<HTMLDivElement>) => {
    const frame = (event.clientX - event.currentTarget.getBoundingClientRect().left) / scale
    return clampFrame(snapEnabled && !event.altKey ? snapFrame(frame, targets, scale).frame : frame, duration - 1)
  }
  const startScrub = (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 || (event.target instanceof Element && event.target.closest('button'))) return
    event.preventDefault(); event.currentTarget.focus(); event.currentTarget.setPointerCapture(event.pointerId)
    dragging.current = event.pointerId
    const playback = usePlaybackStore.getState()
    playback.pause(); playback.setScrubFrame(pointerFrame(event))
  }
  const moveScrub = (event: PointerEvent<HTMLDivElement>) => {
    if (dragging.current === event.pointerId) usePlaybackStore.getState().setScrubFrame(pointerFrame(event))
  }
  const endScrub = (event: PointerEvent<HTMLDivElement>) => {
    if (dragging.current !== event.pointerId) return
    usePlaybackStore.getState().finishScrub(pointerFrame(event)); dragging.current = null
    if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId)
  }
  const matchRange = (clip: Clip) => {
    const matches = ({ range }: (typeof failures)[number]) => (!range.itemId || range.itemId === clip.item.id) && (!range.cueId || range.cueId === clip.cueId) && (range.itemId || range.cueId || clip.kind === 'caption') && range.start * fps < clip.from + clip.durationInFrames && range.end * fps > clip.from
    return failures.find(entry => entry.index === selectedRange && matches(entry)) ?? failures.find(matches)
  }
  return <section aria-label="Dòng thời gian" data-shortcut-scope="timeline" {...stylex.props(s.root)}>
    <div {...stylex.props(s.toolbar)}>
      <button type="button" title="Tách tại vị trí phát (S)" disabled={!selectedIds.length} onClick={split} {...stylex.props(s.button)}><Icon name="scissor" size={14} />Tách <kbd {...stylex.props(s.shortcut)}>S</kbd></button>
      <button type="button" title="Xoá phần đang chọn (⌫)" disabled={!selectedIds.length} onClick={remove} {...stylex.props(s.button)}><Icon name="trash" size={14} />Xoá</button>
      <button type="button" aria-pressed={snapEnabled} title="Bắt dính vào mép cảnh và phụ đề (N)" onClick={snap} {...stylex.props(s.button, snapEnabled && s.pressed)}><Icon name="tick" size={13} />Bắt dính</button>
      {failures.length > 0 && <><span aria-hidden {...stylex.props(s.divider)} /><span {...stylex.props(s.chip)}><span>Đoạn lệch {position + 1}/{failures.length}</span><button type="button" aria-label="Đoạn lệch trước" title="Đoạn lệch trước (⇧F)" onClick={() => navigate(-1)} {...stylex.props(s.button, s.quiet, s.iconButton, s.chipButton)}><Icon name="arrow-left-2" size={13} /></button><button type="button" aria-label="Đoạn lệch sau" title="Đoạn lệch sau (F)" onClick={() => navigate(1)} {...stylex.props(s.button, s.quiet, s.iconButton, s.chipButton)}><Icon name="arrow-r" size={13} /></button></span></>}
      <span {...stylex.props(s.spacer)} />
      <button type="button" aria-label="Thu nhỏ dòng thời gian" title="Thu nhỏ (−)" onClick={() => setZoom(zoom / 1.25)} {...stylex.props(s.button, s.iconButton)}><Icon name="minus" size={14} /></button>
      <input type="range" aria-label="Thu phóng dòng thời gian" min={minimumZoom} max={MAX_ZOOM} step="any" value={Math.max(minimumZoom, Math.min(MAX_ZOOM, zoom))} onChange={event => setZoom(Number(event.target.value))} {...stylex.props(s.zoom)} />
      <button type="button" aria-label="Phóng to dòng thời gian" title="Phóng to (=)" onClick={() => setZoom(zoom * 1.25)} {...stylex.props(s.button, s.iconButton)}><Icon name="add" size={14} /></button>
      <button type="button" title="Vừa cả video (\)" onClick={fit} {...stylex.props(s.button, s.quiet)}>Vừa</button>
    </div>
    <div ref={scroll} onScroll={syncViewport} {...stylex.props(s.scroll)}>
      <div {...stylex.props(s.content(contentWidth))}>
        <div {...stylex.props(s.rulerRow)}><div {...stylex.props(s.header, s.rulerHeader)} /><Ruler duration={duration} fps={fps} scale={scale} left={viewport.left} width={available} markers={rangeMarkers} onPointerDown={startScrub} onPointerMove={moveScrub} onPointerUp={endScrub} /></div>
        {orderedTracks.map(track => {
          const kind: Kind = track.templateSlot === 'captions' ? 'caption' : track.templateSlot === 'scenes' ? 'scene' : track.templateSlot === 'source-audio' || track.kind === 'audio' ? 'audio' : track.templateSlot === 'background-video' ? 'video' : itemKind(items.find(item => item.trackId === track.id))
          const count = clips.filter(clip => clip.trackId === track.id).length
          const label = kind === 'video' && count > 1 ? `${track.name} · ${count}` : track.name
          return <div key={track.id} role="group" aria-label={`Rãnh ${track.name}`} {...stylex.props(s.row)}>
            <div {...stylex.props(s.header)}><Icon name={icons[kind]} size={14} xstyle={iconStyles[kind]} /><span title={label} {...stylex.props(s.trackLabel)}>{label}</span>{(track.kind === 'audio' || kind === 'audio') && <button type="button" aria-label={`${track.muted ? 'Bật tiếng' : 'Tắt tiếng'} ${track.name}`} aria-pressed={track.muted} title={track.muted ? 'Bật tiếng' : 'Tắt tiếng'} onClick={() => changeTrack(track, { muted: !track.muted })} {...stylex.props(s.trackButton)}><Icon name="volume" size={13} xstyle={track.muted ? s.mutedIcon : undefined} /></button>}<button type="button" aria-label={`${track.visible ? 'Ẩn' : 'Hiện'} rãnh ${track.name}`} aria-pressed={track.visible} title={track.visible ? 'Ẩn rãnh' : 'Hiện rãnh'} onClick={() => changeTrack(track, { visible: !track.visible })} {...stylex.props(s.trackButton)}><Icon name={track.visible ? 'eye' : 'eye-slash'} size={13} /></button></div>
            <div tabIndex={-1} {...stylex.props(s.lane, !track.visible && s.hidden)} onPointerDown={startScrub} onPointerMove={moveScrub} onPointerUp={endScrub} onPointerCancel={endScrub}>
              {visibleItems(indexes.get(track.id), Math.max(0, viewport.left / scale - 1), (viewport.left + available) / scale + 1).map(clip => {
                const failure = matchRange(clip)
                const selectedClip = failure ? failure.index === selectedRange : selected.has(clip.item.id) && (!clip.cueId || clip.id === activeCueId)
                return <button key={clip.id} type="button" aria-label={`${clip.label}, ${timecode(clip.from, fps)}${failure ? ', Đoạn lệch' : ''}`} aria-pressed={selectedClip} title={clip.label} {...stylex.props(s.clip(clip.from * scale, clip.durationInFrames * scale), s[clip.kind], failure && s.failing, selectedClip && s.selected)} onClick={event => {
                  if (failure) selectRange(failure.index)
                  else {
                    const ids = useSelectionStore.getState().selectedItemIds
                    useSelectionStore.getState().selectItems(event.shiftKey ? selected.has(clip.item.id) ? ids.filter(id => id !== clip.item.id) : [...ids, clip.item.id] : [clip.item.id])
                    seek(clip.from)
                  }
                }}
                >
                  {clip.kind === 'audio' && <AudioWaveform item={clip.item} fps={fps} scale={scale} left={Math.max(0, viewport.left - clip.from * scale)} width={Math.max(0, Math.min(clip.durationInFrames * scale, viewport.left + available - clip.from * scale) - Math.max(0, viewport.left - clip.from * scale))} />}
                  {clip.item.id === drawingClipId && <span aria-label="Đang vẽ lại ảnh cảnh" title="Đang vẽ lại ảnh cảnh" {...stylex.props(s.drawingRing)} />}
                  <span {...stylex.props(s.clipLabel, clip.kind === 'audio' && s.audioLabel)}>{clip.label}</span>
                </button>
              })}
              {kind === 'scene' && selectedScene?.trackId === track.id && keyframeFrames.map(frame => <span key={frame} aria-hidden {...stylex.props(s.keyframe((selectedScene.from + frame) * scale))} />)}
            </div>
          </div>
        })}
        {!orderedTracks.length && <p {...stylex.props(s.empty)}>Chưa có nội dung trên dòng thời gian.</p>}
        <Playhead scale={scale} />
      </div>
    </div>
  </section>
}
