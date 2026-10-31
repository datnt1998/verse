import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { ArrowLeft, ArrowRight, Eye, Pause, Play, Volume2, VolumeX } from 'lucide-react'
import { HeadlessPlayer, type PlayerRef } from '@editor/runtime/player/Player'
import { MainComposition } from '@editor/runtime/composition-runtime/compositions/main-composition'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useProjectStore } from '@editor/features/projects/stores/project-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { CanvasPreview } from '../ui/preview/canvas-preview'
import { usePlayerStoreSync, usePreviewComposition } from '../ui/preview/use-preview-composition'
import { frameTimecode } from '../ui/preview/timecode'
import { useShortcutAction, type ShortcutScope } from '../ui/shortcuts'
import { previewStyles as s } from './preview.stylex'

function TransportShortcuts({ scope, fps, duration }: { scope: ShortcutScope; fps: number; duration: number }) {
  const jump = (delta: number) => {
    const state = usePlaybackStore.getState()
    state.pause(); state.finishScrub(Math.max(0, Math.min(duration - 1, state.currentFrame + delta)))
  }
  useShortcutAction({ id: 'play', label: 'Phát / tạm dừng', scope, bindings: ['Space'], run: event => { if (!event.repeat) usePlaybackStore.getState().togglePlayPause() } })
  useShortcutAction({ id: 'frame-back', label: 'Lùi một khung', scope, run: () => jump(-1) })
  useShortcutAction({ id: 'frame-forward', label: 'Tiến một khung', scope, run: () => jump(1) })
  useShortcutAction({ id: 'second-back', label: 'Lùi một giây', scope, run: () => jump(-fps) })
  useShortcutAction({ id: 'second-forward', label: 'Tiến một giây', scope, run: () => jump(fps) })
  useShortcutAction({ id: 'start', label: 'Về đầu', scope, run: () => jump(-duration) })
  useShortcutAction({ id: 'end', label: 'Tới cuối', scope, run: () => jump(duration) })
  return null
}

function Transport({ fps, duration, safe, toggleSafe }: { fps: number; duration: number; safe: boolean; toggleSafe: () => void }) {
  const frame = usePlaybackStore(state => state.currentFrame)
  const playing = usePlaybackStore(state => state.isPlaying)
  const volume = usePlaybackStore(state => state.volume)
  const muted = usePlaybackStore(state => state.muted)
  const quality = usePlaybackStore(state => state.previewQuality)
  const [volumeOpen, setVolumeOpen] = useState(false)
  const volumeRoot = useRef<HTMLDivElement>(null)
  const volumeButton = useRef<HTMLButtonElement>(null)
  const volumeInput = useRef<HTMLInputElement>(null)
  useEffect(() => {
    if (!volumeOpen) return
    volumeInput.current?.focus()
    const outside = (event: globalThis.PointerEvent) => {
      if (event.target instanceof Node && !volumeRoot.current?.contains(event.target)) setVolumeOpen(false)
    }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [volumeOpen])
  const seek = (target: number) => {
    const state = usePlaybackStore.getState()
    state.pause(); state.finishScrub(Math.max(0, Math.min(duration - 1, target)))
  }
  return <div {...stylex.props(s.transport)}>
    <button type="button" aria-label="Về đầu" title="Về đầu (Home)" onClick={() => seek(0)} {...stylex.props(s.button)}><ArrowLeft size={14} aria-hidden /></button>
    <button type="button" aria-label={playing ? 'Tạm dừng' : 'Phát'} title="Phát / tạm dừng (Space)" onClick={() => usePlaybackStore.getState().togglePlayPause()} {...stylex.props(s.button, s.play)}>{playing ? <Pause size={16} aria-hidden /> : <Play size={16} aria-hidden />}</button>
    <button type="button" aria-label="Tới cuối" title="Tới cuối (End)" onClick={() => seek(duration - 1)} {...stylex.props(s.button)}><ArrowRight size={14} aria-hidden /></button>
    <span role="timer" aria-label={`Thời gian ${frameTimecode(frame, fps)}`} {...stylex.props(s.timer)}>{frameTimecode(frame, fps)}<span {...stylex.props(s.muted)}> / {frameTimecode(duration, fps)}</span></span>
    <span {...stylex.props(s.spacer)} />
    <button type="button" aria-pressed={safe} title="Khung an toàn" onClick={toggleSafe} {...stylex.props(s.button, s.safeButton, safe && s.safeActive)}><Eye size={13} aria-hidden /><span {...stylex.props(s.safeLabel)}>Khung an toàn</span></button>
    <button type="button" aria-label="Chất lượng xem trước" title="Chất lượng xem trước" onClick={() => usePlaybackStore.getState().setPreviewQuality(quality === 1 ? 0.5 : quality === 0.5 ? 0.25 : 1)} {...stylex.props(s.button, s.field, s.quality)}><span {...stylex.props(s.muted)}>Xem</span>{quality === 1 ? '1/1' : quality === 0.5 ? '1/2' : quality === 0.33 ? '1/3' : '1/4'}</button>
    <span title="Thu phóng khung xem" {...stylex.props(s.field, s.zoom)}><span {...stylex.props(s.muted)}>Zoom</span>Vừa khung</span>
    <div ref={volumeRoot} {...stylex.props(s.volume)} onKeyDown={event => {
      if (event.key === 'Escape' && volumeOpen) { event.preventDefault(); event.stopPropagation(); setVolumeOpen(false); volumeButton.current?.focus() }
    }}>
      <button ref={volumeButton} type="button" aria-label="Âm lượng xem trước" title="Âm lượng xem trước" aria-expanded={volumeOpen} aria-controls="editor-preview-volume" onClick={() => setVolumeOpen(open => !open)} {...stylex.props(s.button)}>{muted || volume === 0 ? <VolumeX size={15} aria-hidden /> : <Volume2 size={15} aria-hidden />}</button>
      {volumeOpen && <div id="editor-preview-volume" role="group" aria-label="Âm lượng xem trước" {...stylex.props(s.volumePanel)}>
        <button type="button" aria-label={muted ? 'Bật tiếng xem trước' : 'Tắt tiếng xem trước'} aria-pressed={muted} onClick={() => usePlaybackStore.getState().toggleMute()} {...stylex.props(s.button)}>{muted ? <VolumeX size={15} aria-hidden /> : <Volume2 size={15} aria-hidden />}</button>
        <input ref={volumeInput} type="range" min={0} max={1} step={0.01} value={volume} aria-label="Âm lượng xem trước" onChange={event => usePlaybackStore.getState().setVolume(Number(event.target.value))} {...stylex.props(s.volumeRange)} />
        <span {...stylex.props(s.volumeValue)}>{Math.round(volume * 100)}%</span>
      </div>}
    </div>
  </div>
}

export function Preview() {
  const { items, fps, width, height, duration, resolved, transitions, keyframes, backgroundColor, composition } = usePreviewComposition()
  const quality = usePlaybackStore(state => state.previewQuality)
  const loop = usePlaybackStore(state => state.loop)
  const name = useProjectStore(state => state.currentProject?.name)
  const player = useRef<PlayerRef>(null)
  const [safe, setSafe] = useState(true)
  usePlayerStoreSync(player)
  return <main aria-label="Xem trước" data-shortcut-scope="preview" {...stylex.props(s.root)}>
    <TransportShortcuts scope="preview" fps={fps} duration={duration} />
    <TransportShortcuts scope="timeline" fps={fps} duration={duration} />
    <div {...stylex.props(s.stage)}>
      <div role="region" tabIndex={0} aria-label={`Khung xem${name ? `: ${name}` : ''}`} {...stylex.props(s.frame(width / height))}>
        <HeadlessPlayer ref={player} fps={fps} durationInFrames={duration} width={Math.round(width * quality)} height={Math.round(height * quality)} initialFrame={usePlaybackStore.getState().currentFrame} loop={loop} {...stylex.props(s.player)} onFrameChange={frame => usePlaybackStore.getState().setCurrentFrame(frame)} onPlayStateChange={playing => {
          const state = usePlaybackStore.getState()
          if (playing !== state.isPlaying) { if (playing) state.play(); else state.pause() }
        }}>
          <MainComposition fps={fps} tracks={resolved} transitions={transitions} keyframes={keyframes} width={width} height={height} backgroundColor={backgroundColor} liveItemTransformSource={useItemsStore} />
        </HeadlessPlayer>
        <CanvasPreview composition={composition} quality={quality} />
        {!items.length && <div {...stylex.props(s.empty)}>Nhập media, rồi thêm vào timeline.</div>}
        {safe && <><span aria-hidden {...stylex.props(s.outerSafe)} /><span aria-hidden {...stylex.props(s.innerSafe)} /></>}
      </div>
    </div>
    <Transport fps={fps} duration={duration} safe={safe} toggleSafe={() => setSafe(value => !value)} />
  </main>
}
