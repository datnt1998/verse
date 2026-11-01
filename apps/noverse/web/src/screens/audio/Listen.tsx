// Nghe & duyệt (#/p/:p/s/:s/audio/:v/listen) — AudioExport.dc.html 07 (listen · exported).
// Sóng âm thật (Range), chạy theo bản đọc, tua bằng bàn phím, đánh dấu chỗ cần nghe lại,
// đọc lại A/B một đoạn, rồi ghép & xuất tập.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { navigate, paths } from '../../router'
import {
  acceptAudioCandidate, audioChapterURL, audioExportFileURL, downloadExportFile,
  getAudioPeaks, getAudioScript, rejectAudioCandidate, rerenderAudioSegment, revealFolder,
} from '../../api/client'
import type { AudioCandidate, AudioFlag, AudioSegment, ExportFile } from '../../api/types'
import { Icon, Button, toast, EmptyState, Skeleton, Spinner } from '@noverse/ui'
import { MachineCard, useMachineCardActions, useMachineStatus } from './MachineCard'
import { PipelineBar } from './PipelineBar'
import { ConfirmExportDialog } from './ConfirmDialog'
import { useQualityChange } from './dialogs'
import { AudioFeedback, AudioSkeleton } from './AudioFeedback'
import { keys, useAction, useResource } from '../../data'
import { KitSelect } from '@noverse/ui'
import type { JobsState } from '../../shell/jobs/store'
import { useJobSelection } from '../../shell/jobs/useJobs'
import {
  allDone, chapterMarkLines, clockOf, followIndex, followTimeline, formatBytes, formatClock,
  formatDuration, flagPosition, sortFlags, stepFlagIndex,
} from './logic'
import {
  errText, useAudioAccess, useAudioData, useAudioExport, usePlayer, usePreviewAudio, type PreviewAudio,
} from './useAudio'
import { usePackVoice } from './packVoice'
import { useStoryName } from '../../shell/useNames'
import { Eq, Lbl, Note, Waveform, speakerTint } from './ui'
import { useMediaSession } from './mediaSession'
import { audioAlbumTitle, chapterTrackTitle, volumeTrackTitle } from '../../lib/audioName'
import { useStoryShortTitle } from '../../shell/useNames'
import './audio.css'
import { t, audioExportCopy as exportCopy } from '../../i18n/vi'
import { copyText } from '../../lib/clipboard'

/** Màn “Đã xuất” là trạng thái truy vấn trên route nghe để tải lại trang vẫn giữ nguyên. */
export const EXPORTED_VIEW = 'da-xuat'
export const exportedPath = (project: string, story: string, volume: number) =>
  `${paths.audio(project, story, volume, 'listen')}?xem=${EXPORTED_VIEW}`

export function ListenScreen({ project, story, volume }: { project: string; story: string; volume: number }) {
  const data = useAudioData(project, story, volume)
  const pack = usePackVoice(project, story)
  const storyTitle = useStoryName(project, story)
  const { writable } = useAudioAccess(project, story)
  const machine = useMachineStatus()
  const machineActions = useMachineCardActions()
  const chapters = data.volume?.chapters ?? []
  const playable = chapters.filter((item) => item.audio && item.audio.segmentsDone >= item.audio.segments)
  const [chapter, setChapter] = useState<number | null>(null)
  const scriptResource = useResource(keys.story(project, story, 'audio', 'script', volume, chapter),
    ({ signal }) => getAudioScript(project, story, volume, chapter!, signal), { enabled: chapter !== null })
  const peaksResource = useResource(keys.story(project, story, 'audio', 'peaks', volume, chapter),
    ({ signal }) => getAudioPeaks(project, story, volume, chapter!, signal), { enabled: chapter !== null && playable.some((item) => item.chapter === chapter) })
  const script = scriptResource.data ?? null
  const peaks = peaksResource.data ?? null
  const [showExport, setShowExport] = useState(false)
  const [exportChapters, setExportChapters] = useState<number[] | undefined>(undefined)
  // §AE — hàng thao tác sát đáy: nhấc dock việc nền lên trên (bottom = 60 + 12).
  useEffect(() => {
    const root = document.documentElement
    root.style.setProperty('--dock-bottom', '72px')
    return () => { root.style.removeProperty('--dock-bottom') }
  }, [])
  // Việc ghép tập này đang chạy → nút chính đổi “Đang ghép · N%”, không mở hộp nữa.
  const mergingJob = useJobSelection(useCallback((state: JobsState) =>
    state.jobs.find((item) => item.kind === 'export' && item.id === `${project}/${story}/audio/${volume}/export` && item.status !== 'done' && item.status !== 'failed')
    ?? null, [project, story, volume]))
  const qualityChange = useQualityChange(data)
  const [abSegment, setAbSegment] = useState<{ segment: AudioSegment; atSec: number } | null>(null)
  const [activeFlag, setActiveFlag] = useState<string | null>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const preview = usePreviewAudio()
  const chapterUrl = chapter !== null && playable.some((item) => item.chapter === chapter)
    ? audioChapterURL(project, story, volume, chapter, chapters.find((item) => item.chapter === chapter)?.audio?.version) : null
  const player = usePlayer(chapterUrl)
  const currentChapter = chapters.find((item) => item.chapter === chapter) ?? null
  const flags = useMemo(() => sortFlags(data.flags.filter((flag) => flag.chapter === chapter)), [data.flags, chapter])
  const timeline = useMemo(() => followTimeline(script?.segments ?? [], data.cast), [script, data.cast])
  const duration = player.duration || peaks?.durationSec || currentChapter?.durationSec || 0
  const currentIndex = followIndex(timeline, player.time)

  useEffect(() => {
    if (chapter !== null || chapters.length === 0) return
    setChapter((playable[0] ?? chapters[0]).chapter)
  }, [chapters, chapter, playable])

  // Media Session (A7): điều khiển trên màn khoá/thông báo/xe — artist luôn
  // Noverse Audio, bìa truyện từ GET cover, ⏮ ⏭ đổi chương.
  const goPrevious = useCallback(() => {
    const previous = previousChapter(chapters, chapter)
    if (previous) setChapter(previous)
  }, [chapters, chapter])
  const goNext = useCallback(() => {
    const next = nextPlayable(chapters, chapter)
    if (next) setChapter(next)
  }, [chapters, chapter])
  useMediaSession(
    currentChapter && chapterUrl ? {
      project, story, volume,
      title: chapterTrackTitle(currentChapter.chapter, currentChapter.title),
      album: audioAlbumTitle(storyTitle || story, volume), duration,
    } : null,
    player, { onPrevious: goPrevious, onNext: goNext })


  useEffect(() => { setAbSegment(null); setActiveFlag(null) }, [chapter])

  // Cuộn câu đang đọc vào giữa danh sách.
  useEffect(() => {
    if (currentIndex < 0) return
    const node = listRef.current?.querySelector(`[data-follow="${currentIndex}"]`)
    node?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }, [currentIndex])

  const flagHere = useCallback(async () => {
    if (chapter === null || !writable) return
    const segment = timeline[currentIndex]?.segment
    try {
      await data.addFlag({
        chapter,
        segmentId: `${chapter}-${segment?.id ?? 's0001'}`,
        atSec: Math.round(player.time),
        reason: t.audio.listen.canNgheLaiCho,
      })
    } catch (cause) { toast.err(errText(cause)) }
  }, [chapter, writable, timeline, currentIndex, data, player.time])

  const stepFlag = (direction: 1 | -1) => {
    const index = stepFlagIndex(flags, activeFlag, direction)
    if (index < 0) return
    const flag = flags[index]
    setActiveFlag(flag.id)
    player.seek(flag.atSec)
  }

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT' || target.isContentEditable)) return
      if (event.metaKey || event.ctrlKey) return
      if (event.code === 'Space') { event.preventDefault(); player.toggle(); return }
      if (event.key === 'ArrowLeft') { event.preventDefault(); player.skip(event.shiftKey ? -15 : -5); return }
      if (event.key === 'ArrowRight') { event.preventDefault(); player.skip(event.shiftKey ? 15 : 5); return }
      if (event.key.toLowerCase() === 'm') { event.preventDefault(); void flagHere(); return }
      if (event.key === '[') { event.preventDefault(); stepFlag(-1); return }
      if (event.key === ']') { event.preventDefault(); stepFlag(1) }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  if (data.loading && !data.volume) {
    return <AudioSkeleton />
  }
  if (!data.volume) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <EmptyState title={t.audio.listen.khongMoDuocBan} hint={data.error} action={<Button onClick={data.refresh}>{t.audio.listen.thuLai}</Button>} />
      </div>
    )
  }

  const done = allDone(chapters)
  const marks = flags.filter((flag) => flag.chapter === chapter).map((flag) => ({ at: flagPosition(flag, duration || 1), label: `${flag.reason} · ${formatClock(flag.atSec)}`, id: flag.id }))
  const flagsOf = (index: number) => flags.filter((flag) => flag.chapter === index)
  const speakerName = (key: string | undefined) => data.cast?.speakers.find((item) => item.id === key || item.name === key)?.name
    ?? (data.cast?.mode === 'single' ? 'Giọng đọc chung' : key || t.audio.listen.danTruyen)
  const openFlag = (flag: AudioFlag) => {
    setAbSegment(null)
    setActiveFlag(flag.id)
    if (flag.chapter !== chapter) setChapter(flag.chapter)
    else player.seek(flag.atSec)
  }
  const segmentOfFlag = (flag: AudioFlag) => flag.chapter === chapter
    ? timeline.find((entry) => `${chapter}-${entry.segment.id}` === flag.segmentId)?.segment ?? null
    : null
  const cycleRate = () => player.setRate(PLAY_RATES[(PLAY_RATES.indexOf(player.rate) + 1) % PLAY_RATES.length] ?? 1)
  const rateLabel = `${String(player.rate).replace('.', ',')}×`
  const totalSec = chapters.reduce((sum, item) => sum + (item.durationSec ?? 0), 0)
  const footColor = flags.length > 0 ? 'var(--color-warn)' : 'var(--color-fg-3)'
  // Dòng tóm tắt của thanh quy trình (AudioExport.dc.html `topMeta`): “2:24:31 · 8 chương”,
  // khi mở hộp thoại ghép thì “2:24:31 · 6 giọng · chỉ ghép, không đọc lại”.
  const summaryVoices = data.cast
    ? data.cast.mode === 'single' || pack !== null
      ? t.audio.pipeline.motGiong
      : t.audio.pipeline.soGiong(new Set(data.cast.speakers.map((item) => item.voiceId).filter(Boolean)).size)
    : null
  const topMeta = showExport && summaryVoices
    ? t.audio.pipeline.tomTatChiGhep(formatClock(totalSec), summaryVoices)
    : t.audio.pipeline.tomTatTap(formatClock(totalSec), chapters.length)

  return (
    <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      <PipelineBar project={project} story={story} volume={volume} mode="listen" chapters={chapters} jobs={data.queue}
        quality={qualityChange.quality} onQuality={qualityChange.ask} topMeta={topMeta} />
      <AudioFeedback data={data} project={project} story={story} />
      {(scriptResource.error || peaksResource.error) != null && <div role="alert" style={{ padding: '7px 16px', fontSize: 12, color: 'var(--color-warn)' }}>Không tải lại được bản đọc hoặc sóng âm. <Button size="sm" onClick={() => { void scriptResource.refresh(); void peaksResource.refresh() }}>Thử lại</Button></div>}
      {player.error && <div role="alert" style={{ padding: '7px 16px', color: 'var(--color-err)', fontSize: 12 }}>{player.error}</div>}
      <MachineCard view={machine.view} onAction={machineActions.onAction} style={{ margin: '12px 24px 0' }} />

      <div style={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <nav aria-label="Chương" className="scroll" style={{
          flex: '0 0 248px', padding: '14px 10px', borderRight: '1px solid var(--line-1)', display: 'flex', flexDirection: 'column', gap: 10,
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 8px' }}>
            <span style={{ fontSize: 13, fontWeight: 600 }}>Tập {volume}</span>
            <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{formatClock(totalSec)}</span>
          </div>
          <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 1 }}>
            {chapters.map((item) => {
              const on = item.chapter === chapter
              const count = flagsOf(item.chapter).length
              return (
                <li key={item.chapter} className="audio-seg" style={{ position: 'relative' }}>
                  <button type="button" className="hov-row" aria-current={on ? 'page' : 'false'}
                    onClick={() => setChapter(item.chapter)}
                    style={{
                      width: '100%', display: 'flex', alignItems: 'center', gap: 10, padding: '8px 36px 8px 8px', borderRadius: 9, border: 'none',
                      background: on ? 'var(--color-surface-5)' : 'transparent', color: 'var(--color-fg)', textAlign: 'left',
                    }}>
                    <span className="num mono" style={{ flex: '0 0 18px', fontSize: 12, color: 'var(--color-fg-4)' }}>{item.chapter}</span>
                    <span style={{ flex: 1, minWidth: 0, fontSize: 13, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.title}</span>
                    {count > 0 && (
                      <span className="num" style={{
                        height: 18, padding: '0 5px', borderRadius: 5, background: 'rgba(246,203,140,.14)', color: 'var(--color-warn)',
                        fontSize: 11, display: 'flex', alignItems: 'center', gap: 3,
                      }}><Icon name="flag" size={11} />{count}</span>
                    )}
                    <span className="num" style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{item.durationSec ? formatClock(item.durationSec) : '—'}</span>
                    {(item.chapterChanged || item.scriptChanged) && <span title={item.staleReason || t.audio.generation.edited}><Icon name="warning-2" size={14} label={item.staleReason || t.audio.generation.edited} style={{color:'var(--color-warn)',flexShrink:0}} /></span>}
                  </button>
                  <button type="button" className="audio-segact hov-ghost" aria-label={exportCopy.export([item.chapter])} title={exportCopy.export([item.chapter])}
                    disabled={!writable || !playable.includes(item)}
                    onClick={() => { setExportChapters([item.chapter]); setShowExport(true) }}
                    style={{ position: 'absolute', right: 4, top: '50%', transform: 'translateY(-50%)', width: 26, height: 26, display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 7, color: 'var(--color-fg-3)', ...(on ? { opacity: 1 } : {}) }}><Icon name="export-curve" size={13} /></button>
                </li>
              )
            })}
          </ol>
        </nav>

        <main style={{ flex: '999 1 560px', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          <div style={{ flex: '0 0 auto', padding: '16px 24px 14px', display: 'flex', flexDirection: 'column', gap: 12, borderBottom: '1px solid var(--line-1)' }}>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
              <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Chương {chapter ?? '—'}</span>
              <h2 style={{ margin: 0, fontSize: 19, fontWeight: 600 }}>{currentChapter?.title ?? t.audio.listen.chuaCoChuong}</h2>
            </div>
            {(currentChapter?.chapterChanged || currentChapter?.scriptChanged) && <Note tone="warn">{currentChapter.staleReason || t.audio.generation.edited}</Note>}
            <Waveform peaks={peaks?.peaks ?? []} bars={120} height={64}
              played={duration > 0 ? player.time / duration : 0}
              marks={marks} playhead={duration > 0 ? player.time / duration : null}
              onSeek={(fraction) => player.seek(fraction * duration)}
              label={t.audio.listen.songAmChuong(chapter ?? 1)} />
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <span className="num mono" style={{ fontSize: 12.5, color: 'var(--color-fg-2)', flex: '0 0 110px' }}>{formatClock(player.time)} / {formatClock(duration)}</span>
              <span style={{ flex: 1 }} />
              <RoundBtn label={t.audio.listen.chuongTruoc} icon="previous" size={34} iconSize={16} disabled={!previousChapter(chapters, chapter)} onClick={() => {
                const prev = previousChapter(chapters, chapter)
                if (prev) setChapter(prev)
              }} />
              <RoundBtn label={t.audio.listen.lui10Giay} icon="backward-10seconds" size={34} iconSize={20} onClick={() => player.skip(-10)} />
              <button type="button" className="hov-primary" disabled={!chapterUrl} aria-label={player.playing ? 'Tạm dừng' : 'Phát'} onClick={() => player.toggle()} style={{
                width: 46, height: 46, borderRadius: '50%', border: 'none', background: 'var(--color-fg)', color: 'var(--color-bg)',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}><Icon name={player.playing ? 'pause' : 'play'} size={18} /></button>
              <RoundBtn label={t.audio.listen.toi10Giay} icon="forward-10seconds" size={34} iconSize={20} onClick={() => player.skip(10)} />
              <RoundBtn label={t.audio.listen.chuongSau} icon="next" size={34} iconSize={16} disabled={!nextPlayable(chapters, chapter)} onClick={goNext} />
              <span style={{ flex: 1 }} />
              <div style={{ flex: '0 0 110px', display: 'flex', justifyContent: 'flex-end', gap: 6 }}>
                <button type="button" className="hov-ghost" aria-label={t.audio.listen.tocDoPhat} onClick={cycleRate} style={{
                  height: 30, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent',
                  color: 'var(--color-fg)', fontSize: 12.5, fontVariantNumeric: 'tabular-nums',
                }}>{rateLabel}</button>
                <button type="button" className="hov-ghost" aria-label="Đánh dấu chỗ cần sửa" title="Đánh dấu chỗ cần sửa (M)"
                  disabled={!writable || !chapterUrl} onClick={() => void flagHere()} style={{
                    width: 30, height: 30, borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent',
                    color: 'var(--color-warn)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                  }}><Icon name="flag" size={15} /></button>
              </div>
            </div>
          </div>

          <div ref={listRef} className="scroll" style={{ flex: 1, minHeight: 0, padding: '12px 16px' }}>
            {!chapterUrl && <Note tone="warn">{t.audio.listen.chuongNayChuaCo}</Note>}
            <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
              {timeline.map((entry, index) => {
                const current = index === currentIndex
                const past = index < currentIndex
                const segmentFlag = flags.find((flag) => flag.segmentId === `${chapter}-${entry.segment.id}`)
                return (
                  <li key={entry.segment.id} data-follow={index} className="audio-seg" style={{
                    display: 'flex', gap: 12, padding: '10px 12px', borderRadius: 12,
                    background: current ? 'rgba(201,166,245,.07)' : 'transparent',
                    border: `1px solid ${current ? 'rgba(201,166,245,.3)' : 'transparent'}`,
                  }}>
                    <button type="button" className="num mono" onClick={() => player.seek(entry.startSec)} aria-label={t.audio.listen.tuaToi(formatClock(entry.startSec))}
                      style={{ flex: '0 0 44px', padding: '3px 0 0', fontSize: 11.5, color: 'var(--color-fg-4)', textAlign: 'left' }}>
                      {formatClock(entry.startSec)}
                    </button>
                    <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <span style={{ fontSize: 12, fontWeight: 500, color: speakerTint(data.cast, entry.segment.speaker) ?? 'var(--color-fg-2)' }}>
                          {speakerName(entry.segment.speaker)}
                        </span>
                        {current && player.playing && <Eq color="var(--color-accent)" height={12} bars={4} />}
                        {segmentFlag && (
                          <span style={{
                            height: 20, padding: '0 7px', borderRadius: 6, background: 'rgba(246,203,140,.12)', color: 'var(--color-warn)',
                            fontSize: 11, display: 'inline-flex', alignItems: 'center', gap: 4,
                          }}><Icon name="flag" size={11} />{segmentFlag.reason}</span>
                        )}
                        <span style={{ flex: 1 }} />
                        <span className="audio-segact" style={{ display: 'flex', gap: 4 }}>
                          <button type="button" className="hov-ghost" disabled={!writable}
                            onClick={() => { setActiveFlag(null); setAbSegment({ segment: entry.segment, atSec: entry.startSec }) }}
                            style={{
                              height: 26, padding: '0 8px', borderRadius: 7, border: '1px solid var(--line-4)', background: 'transparent',
                              color: 'var(--color-fg-2)', fontSize: 11.5, display: 'flex', alignItems: 'center', gap: 5,
                            }}><Icon name="refresh-2" size={12} />{t.audio.listen.docLaiDoanNay}</button>
                          <button type="button" className="hov-ghost" disabled={!writable} aria-label="Đánh dấu"
                            onClick={() => {
                              if (chapter === null) return
                              void data.addFlag({
                                chapter, segmentId: `${chapter}-${entry.segment.id}`,
                                atSec: Math.round(entry.startSec), reason: t.audio.listen.canNgheLaiCho,
                              }).catch((cause) => toast.err(errText(cause)))
                            }}
                            style={{
                              width: 26, height: 26, borderRadius: 7, border: 'none', background: 'transparent', color: 'var(--color-fg-3)',
                              display: 'flex', alignItems: 'center', justifyContent: 'center',
                            }}><Icon name="flag" size={13} /></button>
                        </span>
                      </div>
                      <p style={{
                        margin: 0, fontFamily: "'Literata', Georgia, serif", fontSize: 15.5, lineHeight: 1.75,
                        color: past ? 'var(--color-fg-4)' : current ? 'var(--color-fg)' : 'var(--color-fg-2)',
                      }}>{entry.segment.text}</p>
                    </div>
                  </li>
                )
              })}
            </ol>
            {timeline.length === 0 && (
              <div role="status" aria-label={t.audio.listen.dangMoBanDoc} style={{ padding: 16, display: 'grid', gap: 14 }}>
                <Skeleton height={44} /><Skeleton height={44} /><Skeleton height={44} /></div>
            )}
          </div>

          <div style={{ flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 12, padding: '12px 24px', borderTop: '1px solid var(--line-1)' }}>
            <span style={{
              flex: '1 1 auto', minWidth: 0, fontSize: 13, color: footColor, display: 'flex', alignItems: 'center', gap: 8,
              whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
            }}>
              <Icon name={flags.length > 0 ? 'flag' : 'tick-circle-bulk'} size={16} />
              {flags.length > 0 ? `${flags.length} chỗ cần nghe lại · vẫn xuất được nếu bạn thấy ổn` : t.audio.listen.chuaDanhDauCho}
            </span>
            {!done && <span style={{ fontSize: 12, color: 'var(--color-warn)' }}>{t.audio.listen.conChuongChuaCo}</span>}
            <Button icon={<Icon name="music" size={15} />} disabled={!writable || playable.length === 0} onClick={() => { setExportChapters([]); setShowExport(true) }}>{exportCopy.exportEach}</Button>
            <button type="button" className="hov-primary" disabled={!writable || machine.view.kind === 'missingFfmpeg' || mergingJob !== null} onClick={() => { setExportChapters(undefined); setShowExport(true) }} style={{
              flex: '0 0 auto', whiteSpace: 'nowrap', height: 36, padding: '0 14px 0 12px', borderRadius: 9, border: 'none',
              background: 'var(--color-fg)', color: 'var(--color-bg)', fontSize: 13, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8,
              opacity: !writable || machine.view.kind === 'missingFfmpeg' ? 0.5 : 1,
            }}>{machine.view.kind === 'missingFfmpeg' ? <Icon name="lock" size={15} /> : <Icon name="export-curve" size={15} />}{mergingJob ? `Đang ghép · ${Math.round((mergingJob.progress ?? 0) * 100)}%` : done ? t.audio.listen.ghepXuatTap(volume) : t.audio.confirm.xuatKhiXong}</button>
          </div>
        </main>

        <aside className="scroll" style={{ flex: '0 0 340px', padding: '16px 14px', borderLeft: '1px solid var(--line-1)', display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
            <Lbl>{t.audio.listen.canNgheLai}</Lbl>
            <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{flags.length}</span>
          </div>
          {abSegment && !activeFlag && (
            <RereadCard key={abSegment.segment.id} project={project} story={story} volume={volume} chapter={chapter ?? 1}
              title={t.audio.listen.docLaiDoanNay} where={`Chương ${chapter ?? 1} · ${formatClock(abSegment.atSec)}`}
              who={speakerName(abSegment.segment.speaker)} flagged={false}
              segment={abSegment.segment} voices={data.voices} preview={preview} writable={writable}
              onClose={() => setAbSegment(null)} />
          )}
          {flags.map((flag) => {
            const open = flag.id === activeFlag
            return (
              <RereadCard key={flag.id} project={project} story={story} volume={volume} chapter={flag.chapter}
                title={flag.reason} where={`Chương ${flag.chapter} · ${formatClock(flag.atSec)}`} who={speakerName(flag.speaker)} flagged
                open={open} segment={segmentOfFlag(flag)} voices={data.voices} preview={preview} writable={writable}
                onOpen={() => openFlag(flag)} onClose={() => setActiveFlag(null)} />
            )
          })}
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Đọc lại chỉ tạo lại đúng đoạn đó (vài giây), rồi ghép vào chương — không phải tạo lại cả chương.</span>
        </aside>
      </div>

      {showExport && (
        <ConfirmExportDialog project={project} story={story} volume={volume} chapters={chapters} cast={data.cast} voices={data.voices} pack={pack}
          machine={machine.view}
          initialChapters={exportChapters}
          onClose={() => setShowExport(false)} />
      )}
      {qualityChange.dialog}
    </div>
  )
}

function previousChapter(chapters: { chapter: number }[], chapter: number | null) {
  const index = chapters.findIndex((item) => item.chapter === chapter)
  return index > 0 ? chapters[index - 1].chapter : null
}
function nextPlayable(chapters: { chapter: number; audio: { segmentsDone: number; segments: number } | null }[], chapter: number | null) {
  const index = chapters.findIndex((item) => item.chapter === chapter)
  for (let cursor = index + 1; cursor < chapters.length; cursor++) {
    const item = chapters[cursor]
    if (item.audio && item.audio.segmentsDone >= item.audio.segments) return item.chapter
  }
  return null
}

const PLAY_RATES = [1, 1.25, 1.5, 0.75]
const REREAD_SPEEDS = [0.8, 0.85, 0.9, 0.95, 1, 1.05, 1.1, 1.15, 1.2, 1.25, 1.3]
const speedLabel = (value: number) => `${String(value).replace('.', ',')}×${value < 1 ? ' (chậm hơn)' : value > 1 ? ' (nhanh hơn)' : ''}`
const minSec = (seconds: number) => `${Math.floor(seconds / 60)}:${String(Math.round(seconds % 60)).padStart(2, '0')}`

function RoundBtn({ label, icon, onClick, disabled, size, iconSize }: {
  label: string; icon: string; onClick: () => void; disabled?: boolean; size: number; iconSize: number
}) {
  return (
    <button type="button" className="hov-ghost" aria-label={label} title={label} onClick={onClick} disabled={disabled} style={{
      width: size, height: size, borderRadius: '50%', border: 'none', background: 'transparent',
      color: disabled ? 'var(--color-fg-5)' : 'var(--color-fg-2)', cursor: disabled ? 'not-allowed' : 'pointer',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
    }}><Icon name={icon} size={iconSize} /></button>
  )
}

/** Thẻ “Cần nghe lại” (A5): mở ra để đọc lại đúng đoạn và so Bản cũ / Bản mới trước khi dùng. */
function RereadCard({ project, story, volume, chapter, title, where, who, flagged, open = true, segment, voices, preview, writable, onOpen, onClose }: {
  project: string; story: string; volume: number; chapter: number
  title: string; where: string; who: string; flagged: boolean; open?: boolean
  segment: AudioSegment | null; voices: { id: string; name: string }[]; writable: boolean
  preview: PreviewAudio; onOpen?: () => void; onClose: () => void
}) {
  const [voiceId, setVoiceId] = useState('')
  const [speed, setSpeed] = useState(1)
  const [candidate, setCandidate] = useState<AudioCandidate | null>(null)
  const render = useAction(async () => {
    if (!segment) return
    const next = await rerenderAudioSegment(project, story, volume, chapter, segment.id, { voiceId: voiceId || undefined, speed })
    setCandidate(next)
    return next
  })
  const choose = useAction(async (accept: boolean) => {
    if (!segment || !candidate) return
    if (accept) await acceptAudioCandidate(project, story, volume, chapter, segment.id, candidate.candidateId)
    else await rejectAudioCandidate(project, story, volume, chapter, segment.id, candidate.candidateId)
    toast.ok(accept ? t.audio.listen.daDungBanMoi : t.audio.listen.giuBanCu2)
    setCandidate(null)
    onClose()
  }, { invalidate: [keys.story(project, story, 'audio')] })

  return (
    <div className="card" style={{
      borderRadius: 14, padding: 12, display: 'flex', flexDirection: 'column', gap: 10,
      borderColor: open ? 'rgba(246,203,140,.35)' : 'var(--line-2)',
    }}>
      <div style={{ display: 'flex', gap: 10 }}>
        <Icon name={flagged ? 'flag' : 'refresh-2'} size={15} style={{ color: flagged ? 'var(--color-warn)' : 'var(--color-fg-3)', marginTop: 2 }} />
        <span style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2 }}>
          <span style={{ fontSize: 13, fontWeight: 500 }}>{title}</span>
          <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>{where} · {who}</span>
        </span>
      </div>
      {open ? (
        <>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 8 }}>
            <label style={{ display: 'flex', flexDirection: 'column', gap: 4, minWidth: 0 }}>
              <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>Giọng</span>
              <KitSelect value={voiceId} onChange={setVoiceId} aria-label={t.audio.listen.giongDocLai}
                options={[{ value: '', label: t.audio.listen.giuGiongHienTai }, ...voices.map((voice) => ({ value: voice.id, label: voice.name }))]} />
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: 4, minWidth: 0 }}>
              <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>Tốc độ</span>
              <KitSelect value={String(speed)} onChange={(value) => setSpeed(Number(value))} aria-label={t.audio.listen.tocDoDocLai}
                options={REREAD_SPEEDS.map((value) => ({ value: String(value), label: speedLabel(value) }))} />
            </label>
          </div>
          {candidate ? (
            <>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: 8, borderRadius: 10, background: 'var(--color-surface-0)' }}>
                  <RoundBtnPlay label={t.audio.listen.ngheBanCu} onClick={() => void preview.play('old', async () => candidate.previousUrl ?? candidate.url).catch((cause) => toast.err(errText(cause)))} />
                  <span style={{ flex: 1, fontSize: 12.5, color: 'var(--color-fg-3)' }}>Bản cũ</span>
                  <span className="num mono" style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{minSec(candidate.previousDurationSec ?? 0)}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: 8, borderRadius: 10, background: 'rgba(166,237,193,.06)', border: '1px solid rgba(166,237,193,.25)' }}>
                  <RoundBtnPlay label={t.audio.listen.ngheBanMoi} onClick={() => void preview.play('new', async () => candidate.url).catch((cause) => toast.err(errText(cause)))} />
                  <span style={{ flex: 1, fontSize: 12.5 }}>Bản mới</span>
                  <span className="num mono" style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{minSec(candidate.durationSec)}</span>
                </div>
              </div>
              <div style={{ display: 'flex', gap: 6 }}>
                <button type="button" className="hov-primary" disabled={!writable || choose.busy} onClick={() => void choose.run(true)} style={{
                  flex: 1, height: 32, borderRadius: 8, border: 'none', background: 'var(--color-fg)', color: 'var(--color-bg)', fontSize: 12.5, fontWeight: 600,
                }}>Dùng bản mới</button>
                <button type="button" className="hov-ghost" disabled={!writable || choose.busy} onClick={() => void choose.run(false)} style={{
                  height: 32, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 12.5,
                }}>Giữ bản cũ</button>
              </div>
            </>
          ) : (
            <button type="button" className="hov-ghost" disabled={!segment || !writable || render.busy} onClick={() => void render.run()} style={{
              height: 30, borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 12.5,
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
            }}>{render.pending ? <Spinner size={13} /> : <Icon name="refresh-2" size={13} />}{render.pending ? t.audio.listen.dangDoc : t.audio.listen.docLaiDoanNay}</button>
          )}
        </>
      ) : (
        <button type="button" className="hov-ghost" disabled={!writable} onClick={onOpen} style={{
          height: 30, borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 12.5,
          display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
        }}><Icon name="refresh-2" size={13} />{t.audio.listen.docLaiDoanNay}</button>
      )}
    </div>
  )
}

function RoundBtnPlay({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button type="button" className="hov-ghost" aria-label={label} title={label} onClick={onClick} style={{
      width: 28, height: 28, borderRadius: '50%', border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
    }}><Icon name="play" size={11} /></button>
  )
}

// ─── Đã xuất (AudioExport.dc.html state exported) ────────────────────────

export function ExportedScreen({ project, story, volume }: { project: string; story: string; volume: number }) {
  const data = useAudioData(project, story, volume)
  const exportState = useAudioExport(project, story, volume)
  const qualityChange = useQualityChange(data)
  const files = exportState.state?.files ?? []
  const state = exportState.state
  const chapterFiles = state?.chapterFiles ?? files.filter((file) => file.format === 'audio' && /(^|\/)chuong[^/]*\//.test(file.path))
  const main = files.find((file) => file.format === 'audio' && !chapterFiles.some((chapter) => chapter.path === file.path)) ?? files[0]
  const subtitle = files.find((file) => file.name.endsWith('.srt'))
  const script = files.find((file) => file.format === 'other' && file.name.endsWith('.ban-doc.txt'))
  const player = usePlayer(main ? audioExportFileURL(project, story, main.path) : null)
  const duration = state?.durationSec ?? 0
  const marks = state?.chapterMarks ?? []
  const storyTitle = useStoryName(project, story)
  const shortTitle = useStoryShortTitle(project, story)
  // A7 — tệp cả tập trên màn khoá đổi tên theo mốc chương đang phát;
  // ⏮ ⏭ nhảy theo mốc (nơi OS cho hiện nút).
  const activeMark = marks.reduce((mark, candidate) => candidate.atSec <= player.time + 0.5 ? candidate : mark, marks[0])
  const markBack = useCallback(() => {
    const mark = marks.reduce((found, candidate) => candidate.atSec < player.time - 3 ? candidate : found, marks[0])
    if (mark) player.seek(mark.atSec)
  }, [marks, player.time, player.seek])
  const markForward = useCallback(() => {
    const mark = marks.find((candidate) => candidate.atSec > player.time + 0.5)
    if (mark) player.seek(mark.atSec)
  }, [marks, player.time, player.seek])
  useMediaSession(
    main ? {
      project, story, volume,
      title: activeMark?.label ?? volumeTrackTitle(shortTitle, volume),
      album: audioAlbumTitle(storyTitle || story, volume),
      duration: player.duration || duration,
    } : null,
    player, { onPrevious: marks.length ? markBack : null, onNext: marks.length ? markForward : null })
  const voiceCount = state?.voices ?? 0
  const onBack = () => navigate(paths.audio(project, story, volume, 'listen'))

  const copyMarks = useAction(async () => {
    try { await copyText(chapterMarkLines(marks).join('\n')) }
    catch { throw new Error(t.audio.listen.trinhDuyetKhongCho) }
  }, { success: t.audio.listen.daChepMocChuong })
  const download = useAction(async () => {
    if (!main) return
    const done = await downloadExportFile(project, story, main.path)
    toast.ok(t.audio.listen.daTai(done.file))
    return done
  })
  const reveal = useAction(() => revealFolder({ target: 'exports', project, story }), { success: 'Đã mở thư mục' })
  const [showChapterFiles, setShowChapterFiles] = useState(false)
  const downloadAll = useAction(async () => {
    if (state?.zipUrl) return downloadExportFile(project, story, state.zipUrl)
    for (const file of files.filter((file) => chapterFiles.some((chapter) => chapter.path === file.path) || file.name.endsWith('.srt'))) await downloadExportFile(project, story, file.path)
  })

  if (!exportState.loaded) {
    return <AudioSkeleton sidebar={false} />
  }
  if (!state || files.length === 0) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <EmptyState title={t.audio.listen.tapChuaXuat(volume)} hint={t.audio.listen.ngheHetCacChuong}
          action={<Button onClick={onBack}>{t.audio.listen.veManNghe}</Button>} />
      </div>
    )
  }

  const outFiles: { icon: string; name: string; desc: string; size: number }[] = []
  if (main) outFiles.push({ icon: 'audio-square', name: main.name, desc: 'Cả tập, có nhạc mở đầu & kết thúc', size: main.size })
  if (chapterFiles.length > 0) outFiles.push({ icon: 'folder-open', name: `Từng chương (${chapterFiles.length} tệp)`, desc: 'Để đăng lẻ hoặc dựng video', size: chapterFiles.reduce((sum, file) => sum + file.size, 0) })
  if (subtitle) outFiles.push({ icon: 'document-text', name: `Phụ đề Tập ${volume}.srt`, desc: 'Khớp thời gian từng câu — tải lên cùng video', size: subtitle.size })
  if (script) outFiles.push({ icon: 'note-text', name: `Bản đọc Tập ${volume}.txt`, desc: 'Bản đã chốt, dùng làm lời mô tả hoặc lưu trữ', size: script.size })
  const totalBytes = main?.size ?? 0

  return (
    <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      <PipelineBar project={project} story={story} volume={volume} mode="listen" chapters={data.volume?.chapters ?? []}
        jobs={data.queue} quality={qualityChange.quality} onQuality={qualityChange.ask} exported
        topMeta={t.audio.pipeline.tomTatTap(formatClock(duration), state.chapterCount || data.volume?.chapters.length || 0)} />
      <AudioFeedback data={data} project={project} story={story} />
      {exportState.error != null && <div role="alert" style={{ padding: '7px 16px', fontSize: 12, color: 'var(--color-warn)' }}>{errText(exportState.error)} <Button size="sm" onClick={() => void exportState.refresh()}>Thử lại</Button></div>}
      {state.split ? <div className="scroll" style={{ flex: 1, padding: '28px 16px' }}><div style={{ maxWidth: 880, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div className="card" style={{ borderRadius: 20, padding: 22, display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}><Icon name="tick-circle-bulk" size={56} style={{ color: 'var(--color-ok)' }} /><div><h2 style={{ margin: 0, fontSize: 21, fontWeight: 600 }}>{exportCopy.exported(chapterFiles.length, volume)}</h2><p style={{ margin: '4px 0 0', fontSize: 13.5, color: 'var(--color-fg-3)' }}>{chapterFiles.length} tệp · {formatDuration(duration)} · {formatBytes(state.totalBytes)}</p></div></div>
          <ChapterExportFiles project={project} story={story} files={chapterFiles} />
          <div style={{ display: 'flex', gap: 8 }}><Button variant="primary" icon={<Icon name="folder-open" size={15} />} onClick={() => void reveal.run()}>Mở thư mục</Button><Button pending={downloadAll.pending} disabled={downloadAll.busy} onClick={() => void downloadAll.run()}>{state.zipUrl ? exportCopy.downloadZip : exportCopy.downloadAll(chapterFiles.length)}</Button><Button variant="ghost" onClick={onBack}>Sửa & xuất lại</Button></div>
        </div>
      </div></div> :
      <div className="scroll" style={{ flex: 1, minHeight: 0, padding: '28px 16px' }}>
        <div style={{ maxWidth: 880, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card" style={{ borderRadius: 20, padding: 22, display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
              <span className="audio-pop" aria-hidden="true" style={{
                width: 56, height: 56, borderRadius: '50%', background: 'rgba(166,237,193,.1)', color: 'var(--color-ok)',
                boxShadow: '0 0 0 1px rgba(166,237,193,.3), 0 0 36px -6px rgba(166,237,193,.45)', display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}><Icon name="tick-circle-bulk" size={30} /></span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <h2 style={{ margin: 0, fontSize: 21, fontWeight: 600 }}>{t.audio.listen.daXuatTap(volume)}</h2>
                <p className="num" style={{ margin: '4px 0 0', fontSize: 13.5, color: 'var(--color-fg-3)' }}>
                  {state.chapterCount || data.volume?.chapters.length || 0} chương · {formatDuration(duration)} · {voiceCount} giọng đọc{state.exportedAt ? ` · xong lúc ${clockOf(state.exportedAt)}` : ''}
                </p>
              </div>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px', borderRadius: 14, background: 'var(--color-surface-0)', border: '1px solid var(--line-1)' }}>
              <button type="button" className="hov-primary" aria-label={t.audio.listen.phatBanDaXuat} onClick={() => player.toggle()} style={{
                width: 42, height: 42, borderRadius: '50%', border: 'none', background: 'var(--color-fg)', color: 'var(--color-bg)', flex: '0 0 auto',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}><Icon name={player.playing ? 'pause' : 'play'} size={16} /></button>
              <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
                <span style={{ fontSize: 13.5, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{main?.name ?? t.audio.listen.tap(volume)}</span>
                <Waveform peaks={state.peaks ?? []} bars={96} height={26}
                  played={player.duration > 0 ? player.time / player.duration : 0}
                  onSeek={(fraction) => player.seek(fraction * (player.duration || 0))}
                  label={t.audio.listen.songAmBanDa} />
              </div>
              <span className="num mono" style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>
                {formatClock(player.playing || player.time > 0 ? player.time : duration)} · {formatBytes(totalBytes || state.totalBytes)}
              </span>
            </div>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              <button type="button" className="hov-primary" disabled={reveal.busy} onClick={() => void reveal.run()} style={{
                height: 36, padding: '0 14px 0 12px', borderRadius: 9, border: 'none', background: 'var(--color-fg)', color: 'var(--color-bg)',
                fontSize: 13, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8,
              }}><Icon name="folder-open" size={15} />Mở thư mục</button>
              <button type="button" className="hov-ghost" disabled={!main || download.busy} onClick={() => void download.run()} style={{
                height: 36, padding: '0 14px 0 12px', borderRadius: 9, border: '1px solid var(--line-4)', background: 'transparent',
                color: 'var(--color-fg)', fontSize: 13, display: 'flex', alignItems: 'center', gap: 8,
              }}>{download.pending ? <Spinner size={15} /> : <Icon name="export-curve" size={15} />}Tải xuống</button>
              <button type="button" className="hov-ghost" onClick={() => navigate(paths.audio(project, story, volume, 'review'))} style={{
                height: 36, padding: '0 14px', borderRadius: 9, border: 'none', background: 'transparent', color: 'var(--color-fg-3)', fontSize: 13,
                display: 'flex', alignItems: 'center',
              }}>Sửa & xuất lại</button>
            </div>
          </div>

          <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
            <div className="card" style={{ flex: '1 1 400px', borderRadius: 16, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 10 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ flex: 1, fontSize: 14, fontWeight: 600 }}>Mốc chương</span>
                <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>dán vào mô tả video</span>
                <button type="button" className="hov-ghost" disabled={copyMarks.busy || marks.length === 0} onClick={() => void copyMarks.run()} style={{
                  height: 28, padding: '0 10px', borderRadius: 7, border: '1px solid var(--line-4)', background: 'transparent',
                  color: 'var(--color-fg)', fontSize: 12, display: 'flex', alignItems: 'center', gap: 6,
                }}><Icon name="copy" size={13} />Sao chép</button>
              </div>
              <div style={{
                padding: '10px 12px', borderRadius: 10, background: 'var(--color-bg)', border: '1px solid var(--line-1)',
                fontSize: 12, lineHeight: 1.9, color: 'var(--color-fg-2)',
              }}>
                {marks.map((mark, index) => <div key={index}><span className="num mono" style={{ color: 'var(--color-ok)' }}>{formatClock(mark.atSec)}</span> {mark.label}</div>)}
              </div>
            </div>
            <div className="card" style={{ flex: '1 1 300px', borderRadius: 16, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 4 }}>
              <span style={{ fontSize: 14, fontWeight: 600, paddingBottom: 6 }}>Trong Bản xuất · Tập {volume}</span>
              {outFiles.map((file, index) => (
                <div key={file.name} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 0', borderTop: index ? '1px solid var(--line-1)' : 'none' }}>
                  <Icon name={file.icon} size={16} style={{ color: 'var(--color-fg-3)' }} />
                  <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                    <span title={file.name} style={{ fontSize: 13, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{file.name}</span>
                    <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{file.desc}</span>
                  </span>
                  <span className="num" style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{formatBytes(file.size)}</span>
                  {file.icon === 'folder-open' && <Button size="sm" onClick={() => setShowChapterFiles(!showChapterFiles)}>{showChapterFiles ? exportCopy.collapse : exportCopy.viewFiles(chapterFiles.length)}</Button>}
                </div>
              ))}
              {showChapterFiles && <ChapterExportFiles project={project} story={story} files={chapterFiles} />}
            </div>
          </div>
        </div>
      </div>}
      {qualityChange.dialog}
    </div>
  )
}

function ChapterExportFiles({ project, story, files }: { project: string; story: string; files: ExportFile[] }) {
  const [playing, setPlaying] = useState<string | null>(null)
  const player = usePlayer(playing ? audioExportFileURL(project, story, playing) : null, { autoplay: true })
  const download = useAction((path: string) => downloadExportFile(project, story, path))
  return <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>{files.map((file) => <div key={file.path} style={{ padding: '12px 14px', borderRadius: 12, border: '1px solid var(--line-1)', background: 'var(--color-surface-0)', display: 'flex', alignItems: 'center', gap: 12 }}>
    <button type="button" aria-label={`Nghe ${file.name}`} onClick={() => { if (playing === file.path) player.toggle(); else setPlaying(file.path) }}><Icon name={playing === file.path && player.playing ? 'pause' : 'play'} size={16} /></button>
    <span title={file.name} style={{ flex: 1, minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', fontSize: 13.5 }}>{file.name}<small style={{ display: 'block', color: 'var(--color-fg-4)', fontSize: 12 }}>{formatDuration(file.durationSec ?? 0)} · {formatBytes(file.size)}</small></span>
    <Button size="sm" disabled={download.busy} onClick={() => void download.run(file.path)}>{exportCopy.download}</Button>
  </div>)}</div>
}
