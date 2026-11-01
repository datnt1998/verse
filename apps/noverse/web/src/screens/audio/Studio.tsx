// Phòng thu (#/p/:p/s/:s/audio/:v/make) — AudioExport.dc.html (rendering · paused · renderError · rendered).
// Hero đang đọc (vòng tiến độ, câu đang đọc, sóng, 4 bước của đoạn), thẻ lỗi, thẻ xong, dòng thời gian 8 chương,
// theo giọng, nhật ký, nghe thử chương đã xong, thông số máy. Trạng thái “background” do dock việc chạy nền đảm nhận.
import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { navigate, paths } from '../../router'
import {
  audioChapterURL, cancelAudioRender, enqueueAudioChapters, getAudioPeaks, getAudioScript, pauseAudioRender, renderAudioChapter,
  resumeAudioRender,
} from '../../api/client'
import type { AudioCast, AudioChapter, AudioQueueJob, CharacterKind } from '../../api/types'
import { Icon, Button, ConfirmDialog, toast } from '@noverse/ui'
import { keys, useAction, useResource } from '../../data'
import { toggleNotify, useJobsState } from '../../shell/jobs/store'
import { AudioFeedback, AudioSkeleton } from './AudioFeedback'
import { ConfirmExportDialog } from './ConfirmDialog'
import { MachineCard, useMachineCardActions, useMachineStatus } from './MachineCard'
import { PipelineBar } from './PipelineBar'
import {
  activeJob, allDone, clockOf, displayPercent, estimateSegmentSeconds, formatClock,
  formatDuration, formatEta, generationTone, overallPercent, renderSpeedLabel,
} from './logic'
import { charInitials } from '../../lib/cast'
import { KIND_META } from '../../lib/charKind'
import { CharAvatar } from '../../ui/CharAvatar'
import { playError, useAudioAccess, useAudioData, useAudioExport, useElapsed, usePlayer } from './useAudio'
import { usePackVoice } from './packVoice'
import { Lbl, Ring, Waveform, kindRing } from './ui'
import { genRows } from './genWork'
import { ProgressRing } from '@noverse/ui'
import { useQualityChange } from './dialogs'
import { ChapterTimeline } from './ChapterTimeline'
import './audio.css'
import { t } from '../../i18n/vi'

type LogEntry = { id: string; at: Date; text: string; meta: string; icon: 'tick-circle-bulk' | 'refresh-2' | 'flag'; tone: 'ok' | 'warn' }

const QUALITY_NAME = { fast: t.audio.pipeline.ngheThuNhanh, standard: t.audio.pipeline.chuan, high: t.audio.pipeline.caoNhat } as const
/** Bốn bước của một đoạn theo AudioExport.dc.html (36 nhịp: 0–5 · 5–27 · 27–32 · 32–36). */
const STEP_SPANS: [string, number, number][] = [['Chuẩn bị câu', 0, 5], ['Tạo giọng', 5, 27], ['Làm sạch âm', 27, 32], ['Ghép vào chương', 32, 36]]
const MAX_LANES = 4

const decimal = (value: number) => value.toFixed(1).replace('.', ',')

type Lane = { key: string; ini: string; name: string; kind?: CharacterKind; done: number; total: number; active: boolean }

/** Số đoạn đã đọc theo từng người nói — đọc bản đọc của các chương đã chốt (đoạn được tạo lần lượt từ đầu chương). */
function useVoiceLanes(project: string, story: string, volume: number, chapters: AudioChapter[], cast: AudioCast | null, job: AudioQueueJob | null, singleName: string, singleLabel: string, forceSingle: boolean) {
  const [orders, setOrders] = useState<Map<number, string[]>>(new Map())
  const single = forceSingle || cast?.mode === 'single'
  const signature = `${single}|${chapters.filter((item) => item.segments > 0).map((item) => `${item.chapter}:${item.segments}`).join(',')}`
  useEffect(() => {
    if (single) return
    const controller = new AbortController()
    void Promise.all(chapters.filter((item) => item.segments > 0).map(async (item) => {
      const script = await getAudioScript(project, story, volume, item.chapter, controller.signal)
      return [item.chapter, script.segments.map((segment) => segment.speaker)] as const
    })).then((entries) => { if (!controller.signal.aborted) setOrders(new Map(entries)) }, () => undefined)
    return () => controller.abort()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [project, story, volume, signature])

  return useMemo<Lane[]>(() => {
    const doneOf = (item: AudioChapter) => job && job.chapter === item.chapter ? job.segmentsDone : item.audio?.segmentsDone ?? 0
    if (single) {
      const total = chapters.reduce((sum, item) => sum + item.segments, 0)
      return [{ key: 'single', ini: charInitials(singleName), name: singleLabel, kind: 'lead' as const, done: chapters.reduce((sum, item) => sum + Math.min(item.segments, doneOf(item)), 0), total, active: true }]
    }
    const totals = new Map<string, { total: number; done: number }>()
    for (const item of chapters) {
      const order = orders.get(item.chapter) ?? []
      const done = doneOf(item)
      order.forEach((speaker, index) => {
        const entry = totals.get(speaker) ?? { total: 0, done: 0 }
        entry.total += 1
        if (index < done) entry.done += 1
        totals.set(speaker, entry)
      })
    }
    const speakers = (cast?.speakers ?? []).filter((speaker) => (totals.get(speaker.id)?.total ?? 0) > 0)
    const isActive = (id: string, name: string) => job !== null && (job.speaker === id || job.speaker === name)
    const lanes: Lane[] = speakers.slice(0, MAX_LANES).map((speaker) => ({
      key: speaker.id, ini: charInitials(speaker.name, speaker.ini), name: speaker.name, kind: speaker.kind,
      done: totals.get(speaker.id)!.done, total: totals.get(speaker.id)!.total, active: isActive(speaker.id, speaker.name),
    }))
    const known = new Set(speakers.slice(0, MAX_LANES).map((speaker) => speaker.id))
    const rest = [...totals.entries()].filter(([id]) => !known.has(id))
    if (rest.length > 0) {
      lanes.push({
        key: 'other', ini: '…', name: 'Nhân vật phụ',
        done: rest.reduce((sum, [, entry]) => sum + entry.done, 0), total: rest.reduce((sum, [, entry]) => sum + entry.total, 0),
        active: job !== null && !lanes.some((lane) => lane.active),
      })
    }
    return lanes
  }, [orders, chapters, cast, job, single, singleName, singleLabel])
}

export function StudioScreen({ project, story, volume }: { project: string; story: string; volume: number }) {
  const data = useAudioData(project, story, volume)
  const pack = usePackVoice(project, story)
  const { writable } = useAudioAccess(project, story)
  const exportState = useAudioExport(project, story, volume)
  const qualityChange = useQualityChange(data)
  const chapters = data.volume?.chapters ?? []
  const job = activeJob(data.queue)
  const single = pack !== null || data.cast?.mode === 'single'
  const speaker = data.cast?.speakers.find((item) => item.id === job?.speaker || item.name === job?.speaker)
  const singleName = data.voices.find((voice) => voice.id === data.cast?.single.voiceId)?.name ?? 'Giọng đọc'
  const avatarName = single
    ? singleName
    : data.cast?.speakers.find((item) => item.id === speaker?.sameAs)?.name ?? speaker?.name ?? job?.speaker ?? ''
  const speakerName = single ? (pack ? t.audio.pack.giongKeCuaGoi : 'Giọng đọc chung') : speaker?.name ?? job?.speaker ?? ''
  const heroRing = kindRing(speaker?.kind)
  const heroMeta = KIND_META[speaker?.kind ?? 'support']
  const done = allDone(chapters)
  const percent = job ? displayPercent(job.percent) : overallPercent(chapters)
  const running = job?.state === 'rendering'
  const paused = job?.state === 'paused'
  const failed = job?.state === 'error'
  // Chương đã xếp hàng nhưng máy đọc chưa sẵn sàng: khung “n chương đang chờ máy đọc” thay khung đang đọc.
  const queuedChapters = data.queue.filter((item) => item.state === 'queued' && item.chapter)
  const machine = useMachineStatus(queuedChapters.length)
  const machineActions = useMachineCardActions()
  const waitingMachine = !machine.view.ready && queuedChapters.length > 0
  const { notify } = useJobsState()
  const [log, setLog] = useState<LogEntry[]>([])
  const [confirmCancel, setConfirmCancel] = useState(false)
  const [showExport, setShowExport] = useState(false)
  const [techOpen, setTechOpen] = useState(false)
  const elapsed = useElapsed(Boolean(running), job?.jobId ?? '')
  const previous = useRef<{ chapter: number; segment: number; speaker: string; at: number } | null>(null)

  const currentChapter = chapters.find((item) => item.chapter === job?.chapter) ?? null
  const [listenChapter, setListenChapter] = useState<number | null>(null)
  const completeChapter = chapters.find((item) => item.chapter === listenChapter && item.status === 'locked' && item.audio && item.audio.segmentsDone >= item.audio.segments)
    ?? chapters.find((item) => item.status === 'locked' && item.audio && item.audio.segmentsDone >= item.audio.segments) ?? null
  const peaks = useChapterPeaks(project, story, volume, completeChapter?.chapter ?? null, Boolean(completeChapter))
  const player = usePlayer(completeChapter ? audioChapterURL(project, story, volume, completeChapter.chapter, completeChapter.audio?.version) : null, { autoplay: listenChapter !== null })
  const lanes = useVoiceLanes(project, story, volume, chapters, data.cast, job, singleName, pack ? t.audio.pack.giongKeCuaGoi : 'Giọng đọc chung', pack !== null)

  useEffect(() => {
    const live = data.live
    if (!live) return
    const at = new Date()
    if (live.state === 'done') {
      const finished = chapters.find((item) => item.chapter === live.chapter)
      setLog((before) => [{
        id: `d-${at.getTime()}`, at, text: `Xong chương ${live.chapter}${finished ? ` · ${finished.title}` : ''}`,
        meta: finished?.durationSec ? formatClock(finished.durationSec) : '', icon: 'tick-circle-bulk' as const, tone: 'ok' as const,
      }, ...before].slice(0, 6))
      previous.current = null
      return
    }
    if (live.state !== 'rendering') return
    const last = previous.current
    if (last && last.chapter === live.chapter && last.segment !== live.segment) {
      setLog((before) => [{
        id: `${at.getTime()}-${last.segment}`, at, text: t.audio.studio.doan2(last.segment, last.speaker),
        meta: `${decimal((at.getTime() - last.at) / 1000)} s`, icon: 'tick-circle-bulk' as const, tone: 'ok' as const,
      }, ...before].slice(0, 6))
    }
    if (!last || last.segment !== live.segment || last.chapter !== live.chapter) {
      previous.current = { chapter: live.chapter, segment: live.segment, speaker: live.speaker, at: at.getTime() }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data.live])

  const startPending = useAction(async () => {
    const pending = chapters.filter((item) => item.status === 'locked' && (!item.audio || item.audio.segmentsDone < item.audio.segments))
    for (const item of pending) await renderAudioChapter(project, story, volume, item.chapter)
    return pending.length
  }, { success: 'Đã xếp chương vào hàng đợi tạo giọng', invalidate: [keys.story(project, story, 'audio'), keys.jobs()] })
  const control = useAction(async (operation: 'pause' | 'resume' | 'cancel' | 'retry' | 'fast') => {
    if (!job) return
    if (operation === 'pause') await pauseAudioRender(project, story, job.jobId)
    else if (operation === 'cancel') await cancelAudioRender(project, story, job.jobId)
    else {
      if (operation === 'fast' && data.settings && !await data.saveSettings({ ...data.settings, quality: 'fast' })) return
      await resumeAudioRender(project, story, job.jobId)
    }
    await data.reloadQueue()
  }, { invalidate: [keys.story(project, story, 'audio'), keys.jobs()] })

  const updateChapters = useAction(async (list: number[]) => {
    const picked = chapters.filter((item) => list.includes(item.chapter))
    if (picked.length === 0) return 0
    await enqueueAudioChapters(project, story, volume, {
      chapters: list,
      quality: data.settings?.quality ?? qualityChange.quality,
      voiceMode: data.cast?.voiceMode ?? data.cast?.mode ?? 'cast',
      lock: false,
    })
    return picked.length
  }, { success: t.audio.generation.updQueuedToast, invalidate: [keys.story(project, story, 'audio'), keys.jobs()] })

  const totalAudioBytes = chapters.reduce((sum, item) => sum + (item.durationSec ?? 0) * 24_000, 0)
  const usedBytes = Math.round((totalAudioBytes * (done ? 100 : percent)) / 100)
  const activeChapters = chapters.filter((item) => item.status === 'locked')
  // Gói AK — dải chương Phòng thu đánh dấu ô “đã sửa” và thanh Cập nhật từ cùng bộ trạng thái với ngăn Tạo giọng.
  const updateRows = useMemo(() => genRows(chapters, data.queue), [chapters, data.queue])
  const totalSec = data.timeline?.estTotalSec ?? chapters.reduce((sum, item) => sum + (item.durationSec ?? 0), 0)
  const queueEta = data.queue.filter((item) => item.state === 'rendering' || item.state === 'queued').reduce((sum, item) => sum + item.etaSec, 0)
  const etaMin = Math.max(1, Math.round(queueEta / 60))
  const sp = job?.segmentPercent ?? 0
  const phase = sp * 36
  const genP = Math.max(0, Math.min(1, (phase - 5) / 22))
  const segNo = job ? Math.min(job.segments, job.segmentsDone + 1) : 0
  const lineWords = (job?.currentText ?? '').split(' ').filter(Boolean)
  const spoken = Math.floor(genP * lineWords.length)
  const filled = Math.floor(genP * 96)

  if (data.loading && !data.volume) {
    return <AudioSkeleton sidebar={false} />
  }
  if (!data.volume) {
    return <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 13, color: 'var(--color-fg-3)' }}>
      {t.audio.studio.khongMoDuocPhong}<Button size="sm" variant="ghost" onClick={data.refresh}>{t.audio.studio.thuLai}</Button>
    </div>
  }

  const voiceLabel = single ? '1 giọng' : `${new Set(data.cast?.speakers.map((item) => item.voiceId).filter(Boolean)).size} giọng`
  const heroTitle = paused
    ? `Đã tạm dừng · tiếp tục từ đoạn ${segNo}`
    : single ? `${speakerName} · đang đọc lời ${speaker?.name ?? job?.speaker ?? ''}` : `${speakerName} đang đọc`
  // Cột phải “Máy đọc”: dòng đầu tên máy đọc hoặc trạng thái; chờ máy đọc thì số liệu chưa có.
  const machineRows: [string, string, string][] = [
    ['cpu', 'Máy đọc', machine.view.ready ? machine.view.machineName : machine.view.statusLabel],
    ['speedometer', 'Tốc độ tạo', machine.view.speedLabel ?? renderSpeedLabel(exportState.tts?.rtf ?? null)],
    ['timer-1', 'Đã chạy', waitingMachine ? '—' : formatDuration(elapsed)],
    ['box', 'Dung lượng đã dùng', `${formatGb(usedBytes)} / ${formatGb(totalAudioBytes)}`],
  ]

  return (
    <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      <PipelineBar project={project} story={story} volume={volume} mode="make" chapters={chapters} jobs={data.queue}
        quality={qualityChange.quality} onQuality={qualityChange.ask}
        topMeta={t.audio.pipeline.tuTaoKhiChot(QUALITY_NAME[qualityChange.quality])}
        audioRunning={running} />
      <AudioFeedback data={data} project={project} story={story} />
      {player.error && <div role="alert" style={{ padding: '7px 16px', color: 'var(--color-err)', fontSize: 12 }}>{player.error}</div>}

      <div className={paused ? 'audio-frozen' : undefined} style={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <main className="scroll" style={{ flex: '999 1 640px', minWidth: 0, padding: '18px 24px 22px', display: 'flex', flexDirection: 'column', gap: 14 }}>
          <MachineCard view={machine.view} onAction={machineActions.onAction} />
          {waitingMachine && (
            <section aria-label="Chương đang chờ máy đọc" role="status" className="card" style={{
              borderRadius: 20, padding: '18px 20px', display: 'flex', flexDirection: 'column', gap: 14,
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                <span aria-hidden="true" style={{
                  flex: '0 0 auto', width: 44, height: 44, borderRadius: '50%', border: '1.5px dashed var(--line-5)',
                  color: 'var(--color-fg-3)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}><Icon name="clock" size={20} /></span>
                <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <span style={{ fontSize: 17, fontWeight: 600, letterSpacing: '-.01em' }}>{queuedChapters.length} chương đang chờ máy đọc</span>
                  <span style={{ fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.5 }}>{machine.view.waitSub}</span>
                </div>
                <button type="button" className="hov-ghost" onClick={() => void toggleNotify()} aria-pressed={notify} style={{
                  flex: '0 0 auto', height: 32, padding: '0 12px', borderRadius: 8,
                  border: `1px solid ${notify ? 'var(--kit-acc-40)' : 'var(--line-4)'}`,
                  background: notify ? 'var(--kit-acc-12)' : 'transparent', color: 'var(--color-fg)', fontSize: 12.5,
                  display: 'flex', alignItems: 'center', gap: 6,
                }}><Icon name="notification" size={14} />Báo khi bắt đầu</button>
              </div>
              <ol aria-label="Thứ tự chờ" style={{
                listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column',
                borderRadius: 12, border: '1px solid var(--line-1)', overflow: 'hidden',
              }}>
                {queuedChapters.map((item, index) => (
                  <li key={item.jobId} style={{
                    display: 'flex', alignItems: 'center', gap: 12, minHeight: 42, padding: '0 12px',
                    borderTop: index === 0 ? 'none' : '1px solid var(--line-1)', background: 'var(--color-surface-2)',
                  }}>
                    <span className="num" style={{ width: 18, fontSize: 12, color: 'var(--color-fg-4)' }}>{index + 1}</span>
                    <span style={{ flex: '1 1 auto', minWidth: 0, fontSize: 13.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {chapters.find((chapter) => chapter.chapter === item.chapter)?.title ?? `Chương ${item.chapter}`}
                    </span>
                    <span style={{
                      height: 22, padding: '0 8px', borderRadius: 7, background: 'var(--line-1)', color: 'var(--color-fg-2)',
                      fontSize: 12, display: 'inline-flex', alignItems: 'center', gap: 5, whiteSpace: 'nowrap',
                    }}><Icon name="clock" size={12} style={{ color: 'var(--color-fg-4)' }} />Đang chờ máy đọc</span>
                  </li>
                ))}
              </ol>
              <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Không phải lỗi: chương giữ nguyên thứ tự và tự tạo giọng khi máy đọc sẵn sàng. Đóng app vẫn được.</span>
            </section>
          )}
          {!waitingMachine && (running || paused) && job && currentChapter && (
            <section aria-label={t.audio.studio.dangTaoGiong} className="card" style={{
              position: 'relative', overflow: 'hidden', borderRadius: 20, padding: '20px 22px', display: 'flex', flexDirection: 'column', gap: 16,
              borderColor: paused ? 'rgba(246,203,140,.35)' : 'rgba(201,166,245,.22)',
            }}>
              <div aria-hidden="true" className="audio-glowdrift" style={{
                position: 'absolute', right: -120, top: -160, width: 460, height: 460, borderRadius: '50%',
                background: `radial-gradient(closest-side, ${paused ? 'rgba(246,203,140,.08)' : 'rgba(154,99,227,.16)'}, transparent 70%)`,
                filter: 'blur(12px)', pointerEvents: 'none',
              }} />
              <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 18 }}>
                <div style={{ position: 'relative', width: 60, height: 60, flex: '0 0 auto', '--audio-ring-color': heroRing } as CSSProperties}>
                  {running && <><span className="audio-vring" aria-hidden="true" /><span className="audio-vring r2" aria-hidden="true" /></>}
                  {/* Gói S: màu avatar theo loại vai của người đang đọc. */}
                  <span aria-hidden="true" className={running ? 'audio-talk' : undefined} style={{
                    position: 'absolute', inset: 0, borderRadius: '50%', background: heroMeta.background, color: heroMeta.fg,
                    boxShadow: `0 0 0 2.5px ${heroRing}, 0 8px 24px -8px rgba(0,0,0,.8)`,
                    display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 17, fontWeight: 600,
                  }}>{charInitials(avatarName)}</span>
                  {running && (
                    <span className="audio-eq" aria-hidden="true" style={{
                      position: 'absolute', right: -6, bottom: -4, height: 22, padding: '0 6px', borderRadius: 999,
                      background: 'var(--color-surface-3)', border: '1px solid rgba(255,255,255,.12)',
                      display: 'flex', alignItems: 'center', gap: 2, color: 'var(--color-accent)',
                    }}><span /><span /><span /><span /></span>
                  )}
                </div>
                <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <span role="status" aria-live="polite" style={{ fontSize: 17, fontWeight: 600, letterSpacing: '-.01em' }}>{heroTitle}</span>
                  <span className="num" style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>
                    Chương {job.chapter} · {currentChapter.title} · {paused ? 'phần đã tạo được giữ nguyên' : `còn khoảng ${etaMin} phút cho các chương đã chốt`}
                  </span>
                </div>
                <Ring percent={percent} size={96} stroke={7.5} running={running} from={paused ? 'var(--color-warn)' : undefined} to={paused ? 'var(--color-warn)' : undefined} label={t.audio.studio.daTao(percent)}>
                  <span className="num" style={{ fontSize: 22, fontWeight: 700 }}>{percent}%</span>
                  <span className="num" style={{ fontSize: 10.5, color: 'var(--color-fg-4)' }}>{paused ? 'tạm dừng' : formatEta(queueEta)}</span>
                </Ring>
              </div>

              <p style={{ position: 'relative', margin: 0, minHeight: 66, fontFamily: "'Literata', Georgia, serif", fontSize: 19, lineHeight: 1.7, letterSpacing: '.002em' }}>
                <span style={{ fontFamily: 'var(--font-sans)', fontSize: 12, color: 'var(--color-fg-4)', display: 'block', lineHeight: 1.4, marginBottom: 2 }}>
                  Đoạn {segNo} · {single ? `lời ${speaker?.name ?? job.speaker}` : speakerName}
                </span>
                {lineWords.map((word, index) => (
                  <span key={index} className={running && index === spoken && genP < 1 ? 'audio-wnow' : undefined} style={{
                    color: index < spoken || genP >= 1 ? 'var(--color-fg)' : index === spoken ? 'var(--color-accent)' : '#4A4A52', transition: 'color .25s',
                  }}>{word} </span>
                ))}
              </p>

              <div style={{ position: 'relative' }}>
                <div aria-hidden="true" style={{ height: 72, display: 'flex', alignItems: 'center', gap: 2 }}>
                  {Array.from({ length: 96 }, (_, index) => {
                    const value = Math.abs(Math.sin(index * 0.41 + segNo) * 0.55 + Math.sin(index * 1.9 + segNo * 3) * 0.32 + Math.sin(index * 0.11) * 0.2)
                    const height = Math.max(12, Math.min(100, Math.round(value * 100)))
                    const tone = generationTone(index, filled)
                    if (tone === 'pending') return <span key={index} style={{ flex: 1, borderRadius: 999, minHeight: 4, height: 4, background: 'var(--wave-unplayed)' }} />
                    const newest = index === filled - 1 && running
                    return <span key={index} className={newest ? `${filled % 2 ? 'audio-wa' : 'audio-wb'} audio-pen` : undefined}
                      style={{ flex: 1, borderRadius: 999, minHeight: 4, height: `${height}%`, background: tone === 'newest' ? 'var(--wave-newest)' : 'var(--wave-generated)' }} />
                  })}
                </div>
                <div aria-hidden="true" style={{ position: 'absolute', left: 0, right: 0, bottom: -2, height: 1, background: 'linear-gradient(90deg, transparent, rgba(255,255,255,.08), transparent)' }} />
              </div>

              <ol aria-label="Các bước của đoạn đang tạo" style={{ position: 'relative', listStyle: 'none', margin: 0, padding: 0, display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                {STEP_SPANS.map(([label, from, to]) => {
                  const stepDone = phase >= to
                  const now = phase >= from && phase < to
                  const spinning = now && running
                  return (
                    <li key={label} style={{
                      height: 30, padding: '0 12px 0 8px', display: 'flex', alignItems: 'center', gap: 7, borderRadius: 999,
                      background: now ? 'rgba(201,166,245,.08)' : 'transparent',
                      border: `1px solid ${now ? 'rgba(201,166,245,.35)' : 'rgba(255,255,255,.08)'}`,
                      fontSize: 12.5, color: stepDone ? 'var(--color-fg-4)' : now ? 'var(--color-fg)' : 'var(--color-fg-4)',
                    }}>
                      {stepDone && <Icon name="tick-circle-bulk" size={15} style={{ color: 'var(--color-ok)' }} />}
                      {spinning && <span aria-hidden="true" className="audio-spin" style={{ width: 15, height: 15, borderRadius: '50%', border: '2px solid rgba(255,255,255,.16)', borderTopColor: 'var(--color-accent)', flex: '0 0 auto' }} />}
                      {!stepDone && !spinning && <span aria-hidden="true" style={{ width: 15, height: 15, borderRadius: '50%', border: '1.5px dashed rgba(255,255,255,.25)', flex: '0 0 auto' }} />}
                      <span className={spinning ? 'audio-shim' : undefined} style={{ textDecoration: stepDone ? 'line-through' : 'none' }}>{label}</span>
                    </li>
                  )
                })}
                <li style={{ flex: 1 }} />
                <li className="num" style={{ height: 30, display: 'flex', alignItems: 'center', fontSize: 12, color: 'var(--color-fg-4)' }}>
                  đoạn {segNo}/{job.segments}{job.currentText ? ` · ${decimal(estimateSegmentSeconds(job.currentText, 0))} giây` : ''}
                </li>
              </ol>

              <div style={{ position: 'relative', display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap', paddingTop: 14, borderTop: '1px solid var(--line-1)' }}>
                {paused
                  ? <HeroButton icon="play" disabled={!writable || control.busy} onClick={() => void control.run('resume')}>{t.audio.studio.tiepTuc}</HeroButton>
                  : <HeroButton icon="pause" disabled={!writable || control.busy} onClick={() => void control.run('pause')}>{t.audio.studio.tamDung}</HeroButton>}
                <HeroButton icon="arrow-down-2" onClick={() => navigate(paths.workspace(project, story))}>{t.audio.studio.chayNen}</HeroButton>
                <button type="button" className="hov-ghost" disabled={!writable || control.busy} onClick={() => setConfirmCancel(true)}
                  style={{ height: 34, padding: '0 12px', borderRadius: 9, border: 'none', background: 'transparent', color: 'var(--color-fg-3)', fontSize: 13 }}>Huỷ, giữ phần đã xong</button>
                <span style={{ flex: 1 }} />
                <button type="button" aria-pressed={notify} onClick={() => void toggleNotify()} style={{
                  display: 'flex', alignItems: 'center', gap: 6, padding: 0, border: 'none', background: 'transparent', fontSize: 12.5,
                  color: notify ? 'var(--color-fg-2)' : 'var(--color-fg-4)',
                }}><Icon name="notification" size={14} />{t.audio.studio.baoKhiXong}</button>
              </div>
            </section>
          )}

          {failed && job && (
            <section aria-label={t.audio.studio.loiTaoGiong} role="alert" className="card" style={{
              borderRadius: 20, padding: '20px 22px', display: 'flex', flexDirection: 'column', gap: 14, borderColor: 'rgba(246,203,140,.35)',
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
                <span className="audio-pop" aria-hidden="true" style={{
                  width: 56, height: 56, borderRadius: '50%', background: 'rgba(246,203,140,.1)', color: 'var(--color-warn)',
                  boxShadow: '0 0 0 1px rgba(246,203,140,.35), 0 0 30px -8px rgba(246,203,140,.5)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}><Icon name="warning-2" size={26} /></span>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <span style={{ display: 'block', fontSize: 17, fontWeight: 600 }}>Bộ đọc trên máy đã dừng giữa chừng</span>
                  <span className="num" style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>
                    Chương {job.chapter} · đoạn {segNo}/{job.segments} · lúc {clockOf(job.errorAt ?? null)} · đã xong {percent}%
                  </span>
                </div>
              </div>
              <div style={{ padding: '12px 14px', borderRadius: 14, background: 'var(--color-surface-0)', border: '1px solid var(--line-1)', display: 'flex', flexDirection: 'column', gap: 4 }}>
                <span style={{ fontSize: 13.5, fontWeight: 500 }}>{job.error || t.audio.studio.khongTaoDuocDoan}</span>
                <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>
                  {t.audio.studio.thuongXayRa(chapters.filter((item) => item.audio && item.audio.segmentsDone >= item.audio.segments).length, job.chapter, job.segmentsDone)}
                </span>
              </div>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                <button type="button" className="hov-primary" disabled={!writable || control.busy} onClick={() => void control.run('retry')} style={{
                  height: 36, padding: '0 14px 0 12px', borderRadius: 9, border: 'none', background: 'var(--color-fg)', color: 'var(--color-bg)',
                  fontSize: 13, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8,
                }}><Icon name="refresh-2" size={15} />{t.audio.studio.thuLaiTuDoan(segNo)}</button>
                <button type="button" className="hov-ghost" disabled={!writable || control.busy} onClick={() => void control.run('fast')}
                  style={{ height: 36, padding: '0 12px', borderRadius: 9, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 13 }}>
                  {t.audio.studio.phanConLaiDung}</button>
                <button type="button" className="hov-ghost" onClick={() => navigate(paths.settings('voice'))}
                  style={{ height: 36, padding: '0 12px', borderRadius: 9, border: 'none', background: 'transparent', color: 'var(--color-fg-3)', fontSize: 13 }}>
                  {t.audio.studio.moCaiDat}</button>
              </div>
              <button type="button" aria-expanded={techOpen} onClick={() => setTechOpen((open) => !open)} style={{
                alignSelf: 'flex-start', display: 'flex', alignItems: 'center', gap: 6, padding: 0, border: 'none', background: 'transparent', color: 'var(--color-fg-4)', fontSize: 12.5,
              }}><Icon name={techOpen ? 'arrow-down-2' : 'arrow-right-2'} size={14} />Chi tiết kỹ thuật</button>
              {techOpen && (
                <div className="mono" style={{ padding: '10px 12px', borderRadius: 10, background: 'var(--color-bg)', border: '1px solid var(--line-2)', fontSize: 11.5, lineHeight: 1.8, color: 'var(--color-fg-3)', overflowWrap: 'anywhere' }}>
                  {job.error}<br />đoạn {segNo} · giọng {job.speaker} · chất lượng {data.settings ? QUALITY_NAME[data.settings.quality] : ''}
                </div>
              )}
            </section>
          )}

          {!running && !paused && !failed && done && (
            <section aria-label={t.audio.studio.taoGiongXong2} role="status" className="card" style={{
              position: 'relative', overflow: 'hidden', borderRadius: 20, padding: '24px 22px', display: 'flex', alignItems: 'center', gap: 22,
              borderColor: 'rgba(166,237,193,.3)',
            }}>
              <div aria-hidden="true" style={{
                position: 'absolute', left: -80, top: -140, width: 420, height: 420, borderRadius: '50%',
                background: 'radial-gradient(closest-side, rgba(166,237,193,.12), transparent 70%)', filter: 'blur(10px)',
              }} />
              <div style={{ position: 'relative', width: 96, height: 96, flex: '0 0 auto' }}>
                <span className="audio-burst" aria-hidden="true" style={{ position: 'absolute', inset: 0, borderRadius: '50%', border: '2px solid rgba(166,237,193,.6)' }} />
                <span className="audio-burst" aria-hidden="true" style={{ position: 'absolute', inset: 0, borderRadius: '50%', border: '2px solid rgba(201,166,245,.5)', animationDelay: '.25s' }} />
                <ProgressRing className="audio-fillring" size={96} stroke={7.5} percent={100} gradient={['var(--color-ok)', 'var(--color-user)', 'var(--color-ok)']} />
                <svg aria-hidden="true" width="96" height="96" viewBox="0 0 24 24" fill="none" style={{ position: 'absolute', inset: 0, padding: 26 }}>
                  <path className="audio-tick" d="M6 12.5l4 4 8-9" stroke="#A6EDC1" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </div>
              <div style={{ position: 'relative', flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
                <span style={{ fontSize: 20, fontWeight: 600, letterSpacing: '-.01em' }}>Đã tạo xong audio Tập {volume}</span>
                <span className="num" style={{ fontSize: 13.5, color: 'var(--color-fg-3)' }}>
                  {chapters.length} chương · {formatDuration(totalSec)} audio · {voiceLabel}
                </span>
                <div style={{ display: 'flex', gap: 8, marginTop: 8, flexWrap: 'wrap' }}>
                  <button type="button" className="hov-primary" onClick={() => navigate(paths.audio(project, story, volume, 'listen'))} style={{
                    height: 38, padding: '0 16px 0 12px', borderRadius: 10, border: 'none', background: 'var(--color-fg)', color: 'var(--color-bg)',
                    fontSize: 13.5, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8,
                  }}><Icon name="headphone" size={16} />Nghe & duyệt</button>
                  <button type="button" className="hov-ghost" disabled={machine.view.kind === 'missingFfmpeg'}
                    title={machine.view.kind === 'missingFfmpeg' ? 'Máy chủ đang thiếu công cụ ghép âm thanh' : undefined}
                    onClick={() => setShowExport(true)} style={{
                      height: 38, padding: '0 14px', borderRadius: 10, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 13.5,
                      opacity: machine.view.kind === 'missingFfmpeg' ? 0.5 : 1, display: 'flex', alignItems: 'center', gap: 6,
                    }}>{machine.view.kind === 'missingFfmpeg' && <Icon name="lock" size={13} />}Ghép & xuất ngay</button>
                </div>
              </div>
            </section>
          )}

          {!running && !paused && !failed && !done && (
            <section aria-label={t.audio.studio.chuaCoGiDang} className="card" style={{
              borderRadius: 20, padding: '16px 18px', display: 'flex', gap: 14, alignItems: 'center', flexWrap: 'wrap',
            }}>
              <Icon name="voice-square-bulk" size={26} style={{ color: 'var(--color-fg-4)' }} />
              <div style={{ flex: 1, minWidth: 240, display: 'flex', flexDirection: 'column', gap: 3 }}>
                <h2 style={{ margin: 0, fontSize: 15.5, fontWeight: 600 }}>{t.audio.studio.chuaCoGiDang}</h2>
                <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>
                  {t.audio.studio.chuongDaChotDang(activeChapters.filter((item) => !item.audio || item.audio.segmentsDone < item.audio.segments).length, data.settings?.autoOnLock === false ? t.audio.studio.tat : t.audio.studio.bat)}</span>
              </div>
              <Button size="sm" variant="primary" disabled={!writable || startPending.busy} pending={startPending.pending} success={startPending.success} onClick={() => void startPending.run()}>
                {t.audio.studio.taoGiongChoCac}
              </Button>
            </section>
          )}

          <ChapterTimeline volume={volume} chapters={(data.timeline?.chapters ?? []).map(chapter => {
            const live = job?.chapter === chapter.chapter && ['rendering', 'paused', 'error'].includes(job.state) ? job : null
            return live && chapter.state !== 'done' ? { ...chapter, state: 'rendering' as const, pct: live.percent } : chapter
          })} madeSec={data.timeline?.madeSec ?? 0} estTotalSec={data.timeline?.estTotalSec} paused={paused}
            updateRows={updateRows} canUpdate={writable} qualityEtaSec={data.timeline?.qualityEtaSec?.[data.settings?.quality ?? qualityChange.quality]}
            onUpdate={(list) => void updateChapters.run(list)}
            onListen={chapter => {
              setListenChapter(chapter)
              void player.play(audioChapterURL(project, story, volume, chapter, chapters.find((item) => item.chapter === chapter)?.audio?.version)).catch(cause => {
                const message = playError(cause)
                if (message) toast.err(message)
              })
            }} />

          <div style={{ display: 'flex', gap: 14, flexWrap: 'wrap' }}>
            <section aria-label="Theo giọng" className="card" style={{ flex: '1 1 300px', borderRadius: 16, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 10 }}>
              <span style={{ fontSize: 13.5, fontWeight: 600 }}>{single ? 'Giọng đọc' : 'Theo giọng · đoạn đã đọc'}</span>
              {lanes.map((lane) => {
                const live = lane.active && running
                const value = done && !running && !paused ? lane.total : lane.done
                return (
                  <div key={lane.key} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <CharAvatar name={lane.name} ini={lane.ini} kind={lane.kind} size="sm" />
                    <span style={{ flex: '0 0 108px', fontSize: 12.5, color: live ? 'var(--color-fg)' : 'var(--color-fg-3)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{lane.name}</span>
                    <span aria-hidden="true" style={{ flex: 1, height: 5, borderRadius: 5, background: 'var(--color-surface-5)', overflow: 'hidden' }}>
                      <span className={live ? 'audio-run' : undefined} style={{
                        display: 'block', width: `${lane.total > 0 ? Math.round((value / lane.total) * 100) : 0}%`, height: '100%', borderRadius: 5, transition: 'width .3s',
                        background: done && !running && !paused ? 'var(--color-ok)' : live ? undefined : 'var(--color-fg-4)',
                      }} />
                    </span>
                    <span className="num" style={{ flex: '0 0 62px', textAlign: 'right', fontSize: 11.5, color: 'var(--color-fg-4)' }}>{value}/{lane.total}</span>
                  </div>
                )
              })}
            </section>
            <section aria-label={t.audio.studio.nhatKyTaoGiong} className="card" style={{ flex: '1 1 300px', borderRadius: 16, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
                <span style={{ fontSize: 13.5, fontWeight: 600 }}>Nhật ký tạo</span>
                <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>mới nhất trên cùng</span>
              </div>
              <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
                {log.length === 0 && <li style={{ fontSize: 12.5, color: 'var(--color-fg-4)', padding: '6px 0' }}>{t.audio.studio.chuaCoGiTrong}</li>}
                {log.map((entry, index) => (
                  <li key={entry.id} className={index === 0 && running ? (entry.at.getSeconds() % 2 ? 'audio-fa' : 'audio-fb') : undefined} style={{
                    display: 'flex', alignItems: 'center', gap: 10, padding: '6px 0', borderTop: index ? '1px solid rgba(255,255,255,.05)' : 'none',
                    fontSize: 12.5, opacity: Math.max(0.35, 1 - index * 0.12),
                  }}>
                    <Icon name={entry.icon} size={14} style={{ color: entry.tone === 'warn' ? 'var(--color-warn)' : 'var(--color-ok)' }} />
                    <span style={{ flex: 1, minWidth: 0, color: 'var(--color-fg-2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{entry.text}</span>
                    <span className="num mono" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{entry.meta}</span>
                  </li>
                ))}
              </ol>
            </section>
          </div>
        </main>

        <aside className="scroll" style={{ flex: '0 0 340px', padding: '18px 16px', borderLeft: '1px solid var(--line-1)', display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Lbl>Nghe trong lúc chờ</Lbl>
          <div className="card" style={{ borderRadius: 16, padding: 14, display: 'flex', flexDirection: 'column', gap: 12 }}>
            {completeChapter ? (
              <>
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                  <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Chương {completeChapter.chapter}</span>
                  <span style={{ fontSize: 14, fontWeight: 600 }}>{completeChapter.title}</span>
                </div>
                <Waveform peaks={peaks?.peaks ?? []} bars={48} height={44}
                  played={player.duration > 0 ? player.time / player.duration : 0}
                  onSeek={(fraction) => player.seek(fraction * (player.duration || 0))}
                  label={t.audio.studio.songAmChuong(completeChapter.chapter)} />
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <button type="button" className="hov-primary" aria-label={player.playing ? t.audio.studio.tamDung : t.audio.studio.phatChuongDaXong} onClick={() => player.toggle()} style={{
                    width: 38, height: 38, borderRadius: '50%', border: 'none', background: 'var(--color-fg)', color: 'var(--color-bg)',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                  }}><Icon name={player.playing ? 'pause' : 'play'} size={16} /></button>
                  <span className="num mono" style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>{formatClock(player.time)} / {formatClock(player.duration || completeChapter.durationSec || 0)}</span>
                  <span style={{ flex: 1 }} />
                  <button type="button" onClick={() => navigate(paths.audio(project, story, volume, 'listen'))} style={{ padding: 0, fontSize: 12.5, color: 'var(--color-fg-3)' }}>Mở trình nghe</button>
                </div>
              </>
            ) : (
              <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{t.audio.studio.chuaCoChuongNao}</span>
            )}
          </div>

          <Lbl style={{ paddingTop: 6 }}>Máy đọc</Lbl>
          <div className="card" style={{ borderRadius: 14, padding: '4px 14px' }}>
            {machineRows.map(([icon, label, value], index) => (
              <div key={label} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '9px 0', borderTop: index ? '1px solid var(--line-1)' : 'none' }}>
                <Icon name={icon} size={15} style={{ color: 'var(--color-fg-4)' }} />
                <span style={{ flex: 1, fontSize: 12.5, color: 'var(--color-fg-3)' }}>{label}</span>
                <span className="num" style={{ fontSize: 12.5, color: 'var(--color-fg)' }}>{value}</span>
              </div>
            ))}
          </div>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Ước tính tự cập nhật theo tốc độ thật của máy. Có thể rời trang — Noverse báo khi xong.</span>
        </aside>
      </div>
      {qualityChange.dialog}
      {confirmCancel && <ConfirmDialog title="Dừng tạo giọng tập này?" tone="warn"
        body="Các đoạn đã tạo được giữ nguyên. Những đoạn còn lại sẽ dừng và chỉ tiếp tục khi bạn xếp hàng lại."
        confirmLabel="Dừng tạo giọng" pendingLabel="Đang dừng…" onClose={() => setConfirmCancel(false)}
        onConfirm={async () => { await control.run('cancel') }} />}
      {showExport && (
        <ConfirmExportDialog project={project} story={story} volume={volume} chapters={chapters} cast={data.cast} voices={data.voices} pack={pack}
          machine={machine.view}
          onClose={() => setShowExport(false)} />
      )}
    </div>
  )
}

function HeroButton({ icon, onClick, disabled, children }: { icon: string; onClick: () => void; disabled?: boolean; children: ReactNode }) {
  return (
    <button type="button" className="hov-ghost" disabled={disabled} onClick={onClick} style={{
      height: 34, padding: '0 12px', borderRadius: 9, border: '1px solid rgba(255,255,255,.12)', background: 'transparent', color: 'var(--color-fg)',
      fontSize: 13, display: 'flex', alignItems: 'center', gap: 6,
    }}><Icon name={icon} size={14} />{children}</button>
  )
}

/** Đỉnh sóng của chương đã xong — phục vụ nghe thử ở cột phải. */
function useChapterPeaks(project: string, story: string, volume: number, chapter: number | null, ready: boolean) {
  const peaks = useResource(keys.story(project, story, 'audio', 'peaks', volume, chapter),
    ({ signal }) => getAudioPeaks(project, story, volume, chapter!, signal), { enabled: chapter !== null && ready })
  return peaks.data ?? null
}

const formatGb = (bytes: number) => `${(bytes / 1_073_741_824).toFixed(1).replace('.', ',')} GB`
