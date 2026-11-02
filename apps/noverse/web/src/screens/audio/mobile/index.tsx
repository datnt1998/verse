import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { audioChapterURL, cancelAudioRender, enqueueAudioChapters, getAudioPeaks, getTtsStatus, installTts, pauseAudioRender, prioritizeAudioChapter, resumeAudioRender } from '../../../api/client'
import type { AudioMode } from '../../../router'
import { navigate, paths, useRoute } from '../../../router'
import { invalidate, keys, useAction, useConnection, useResource } from '../../../data'
import { BottomSheet, MobilePage, useMobileToast } from '../../../mobile/primitives'
import { useJobSelection } from '../../../shell/jobs/useJobs'
import { useStoryName } from '../../../shell/useNames'
import { Icon } from '@noverse/ui'
import { followIndex, followTimeline, peakBars, voiceName } from '../logic'
import { charInitials } from '../../../lib/cast'
import { loadAudioScript, useAudioAccess, useAudioData, usePlayer } from '../useAudio'
import { useMediaSession } from '../mediaSession'
import { audioAlbumTitle, chapterTrackTitle } from '../../../lib/audioName'
import { ExportSheet } from './ExportSheet'
import { ExportJobCard } from './ExportJobCard'
import { chapterEnded, formatMinutes, nextSleep, playerQueue, SLEEP_LABELS, sleepExpired, SPEEDS, startSleep, stepChapter } from './logic'
import { MachineCard } from './MachineCard'
import { machineView, type MachineActionKind, type MachineStatus } from '../machine'
import { Ring } from './Ring'
import { MobileWave } from './Wave'
import './mobile.css'
import { t } from '../../../i18n/vi'
import type { JobsState } from '../../../shell/jobs/store'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'
import { VoiceGenSheet } from './VoiceGenSheet'
import { genAction, genQueueInput, genRows, genStrip, genTag } from '../genWork'
import { VoiceSheet } from './voice'
import { VolumeSheet } from './VolumeSheet'
import { ChapterActions } from './ChapterActions'
import { chapterActions, type ChapterActionId } from './chapter-actions'

function Eq() { return <span className="ma-eq" aria-hidden="true"><i /><i /><i /></span> }

/** M3 owns only listening state. Shared audio resources/actions stay identical to desktop. */
export default function MobileAudio({ project, story, volume: volumeProp }: { project: string; story: string; volume: number | null; mode?: AudioMode }) {
  const requestedVolume = volumeProp ?? 1
  const data = useAudioData(project, story, requestedVolume)
  // keepPrevious retains the old chapters during a volume fetch. Keep their
  // volume identity too, so playback/export never address old chapters in the new volume.
  const previousVolume = useRef(requestedVolume)
  const volume = data.updatedAt === null ? previousVolume.current : requestedVolume
  useEffect(() => { if (data.updatedAt !== null) previousVolume.current = requestedVolume }, [data.updatedAt, requestedVolume])
  const storyTitle = useStoryName(project, story)
  const access = useAudioAccess(project, story)
  const connection = useConnection()
  // §AT — trạng thái máy đọc lấy từ GET /api/tts/status; không có nút “Kiểm tra lại”:
  // tự làm mới theo sự kiện và khi quay lại tab (AT-2).
  const tts = useResource([...keys.tts(), 'status'], ({ signal }) => getTtsStatus(signal))
  const retryMachine = useAction(() => installTts(), { invalidate: [keys.tts()] })
  const { search } = useRoute()
  const initialState = import.meta.env.MODE === 'mock' ? search.get('mstate') ?? search.get('maudio') : null
  const chapterQuery = Number(search.get('chapter'))
  const requestedChapter = Number.isSafeInteger(chapterQuery) && chapterQuery > 0 ? chapterQuery : null
  const playRequested = search.get('play') === '1' && requestedChapter !== null
  const [sheet, setSheet] = useState<'volume' | 'player' | 'voice' | 'export' | 'gen' | null>((initialState?.startsWith('gen') && initialState !== 'genEntry') || initialState?.startsWith('queue') ? 'gen' : initialState?.startsWith('genVoice') ? 'voice' : initialState === 'player' || initialState === 'voice' ? initialState : initialState?.startsWith('export') ? 'export' : null)
  /** > 0: sheet Ghép & xuất mở thẳng view “Đã xuất” (thẻ nổi §AE › Xem tệp). */
  const [exportDone, setExportDone] = useState(0)
  /** Menu ⋯ của một chương và chương đang được xuất/chia sẻ riêng. */
  const [menuChapter, setMenuChapter] = useState<number | null>(null)
  const [exportScope, setExportScope] = useState<number | null>(null)
  /** true: mở “Đã xuất” của đúng chương để chia sẻ tệp thật; false: mở form xuất riêng chương. */
  const [exportShare, setExportShare] = useState(false)
  const [voiceOpener, setVoiceOpener] = useState<'gen' | 'player'>('gen')
  const [pickRedo, setPickRedo] = useState<number[]>(initialState === 'genRedo' ? [3] : initialState?.startsWith('genPickRedo') ? [4] : [])
  const openGen = (redo: number[] = []) => { setPickRedo(redo); setSheet('gen') }
  const [selected, setSelected] = useState<number | null>(requestedChapter)
  const [autoplay, setAutoplay] = useState(playRequested)
  useEffect(() => {
    setSelected(requestedChapter)
    setAutoplay(playRequested)
    setMenuChapter(null)
    setExportScope(null)
    setExportShare(false)
    if (playRequested) setSheet('player')
  }, [project, story, volume, requestedChapter, playRequested])
  const [sleep, setSleep] = useState(() => startSleep('off', 0))
  const [slow, setSlow] = useState(false)
  useEffect(() => {
    setSlow(false)
    if (!data.loading) return
    const timer = setTimeout(() => setSlow(true), 8000)
    return () => clearTimeout(timer)
  }, [data.loading, project, story, volume])
  const notify = useMobileToast()
  const selectJobs = useCallback((state: JobsState) => state.jobs.filter((job) => job.project === project && job.story === story && job.kind === 'audio' && job.status === 'running' && job.id.includes(`/audio/${volume}/`) && !job.id.endsWith('/export')), [project, story, volume])
  const activeJobs = useJobSelection(selectJobs)
  const chapters = data.volume?.chapters ?? []
  const queue = useMemo(() => playerQueue(chapters), [data.volume])
  const chapter = selected ?? (import.meta.env.MODE === 'mock' && queue.includes(4) ? 4 : queue[0] ?? null)
  const current = chapters.find((item) => item.chapter === chapter)
  const playable = chapter !== null && queue.includes(chapter)
  const end = useCallback(() => {
    const result = chapterEnded(queue, chapter, sleep, Date.now())
    if (result.sleep) { setSleep(startSleep('off', 0)); notify('Đã tắt trình phát theo hẹn giờ') }
    setAutoplay(result.next !== null)
    if (result.next !== null) setSelected(result.next)
  }, [queue, chapter, sleep, notify])
  const chapterURL = (number: number) => audioChapterURL(project, story, volume, number, chapters.find((item) => item.chapter === number)?.audio?.version)
  const player = usePlayer(playable ? chapterURL(chapter!) : null, { onEnded: end, autoplay })
  const script = useResource(keys.story(project, story, 'audio', 'script', volume, chapter), ({ signal }) => loadAudioScript(project, story, volume, chapter!, signal), { enabled: chapter !== null, keepPrevious: true })
  const peaks = useResource(keys.story(project, story, 'audio', 'peaks', volume, chapter), ({ signal }) => getAudioPeaks(project, story, volume, chapter!, signal), { enabled: chapter !== null && queue.includes(chapter), keepPrevious: true })
  const timeline = useMemo(() => followTimeline(script.data?.segments ?? [], data.cast), [script.data, data.cast])
  const entry = timeline[followIndex(timeline, player.time)]
  const words = entry?.segment.text.split(/\s+/) ?? []
  const wordIndex = entry ? Math.min(words.length - 1, Math.floor((player.time - entry.startSec) / Math.max(.01, entry.endSec - entry.startSec) * words.length)) : -1
  const bars = useMemo(() => peakBars(peaks.data?.peaks ?? [], 48), [peaks.data])
  const duration = player.duration || peaks.data?.durationSec || current?.audio?.durationSec || current?.durationSec || 0
  const fraction = duration ? Math.min(1, player.time / duration) : 0
  const cast = data.cast
  const narrator = cast?.speakers.find((speaker) => speaker.id === 'dan') ?? cast?.speakers[0]
  const voiceId = cast?.mode === 'single' ? cast.single.voiceId : narrator?.voiceId
  const voice = voiceName(data.voices, voiceId ?? null) || 'Chưa chọn giọng'
  /** Trình phát thu nhỏ ghi người đọc thật (MAudio §AE), không dùng tên giọng mẫu. */
  const readerName = narrator?.name || 'Dẫn truyện'
  const initials = narrator?.ini ?? charInitials(voice)
  // Dùng chung bộ trạng thái với ngăn Tạo giọng (genWork.ts).
  const genRowList = useMemo(() => genRows(chapters, data.queue), [chapters, data.queue])
  /** Hàng đang mở menu ⋯; dữ liệu đổi (job xong, chương biến mất) thì menu tự đóng theo. */
  const menuRow = menuChapter === null ? null : genRowList.find((row) => row.chapter.chapter === menuChapter) ?? null
  useEffect(() => {
    if (menuChapter !== null && !genRowList.some((row) => row.chapter.chapter === menuChapter)) setMenuChapter(null)
  }, [genRowList, menuChapter])
  const genStripData = useMemo(() => genStrip(genRowList), [genRowList])
  const genPending = genRowList.filter((row) => row.state === 'stale' || row.state === 'new')
  const holdCount = genRowList.filter((row) => row.state === 'hold').length
  // §AT — cùng bộ chuyển trạng thái với máy tính (../machine), chỉ vẽ lại cho khổ điện thoại.
  const machine = useMemo(() => machineView(tts.data, { waitN: Math.max((tts.data as MachineStatus | null)?.queueWaiting ?? 0, holdCount) }), [tts.data, holdCount])
  const machineHold = machine.kind === 'provisioning'
    ? { spinning: true, note: machine.progress?.percent != null ? t.audio.generation.holdPrepNote(Math.round(machine.progress.percent)) : machine.sentence }
    : machine.kind === 'offline'
      ? { spinning: false, note: machine.lastSeenLabel ? t.audio.generation.holdWaitingNote(machine.lastSeenLabel) : machine.sentence }
      : null
  // §8.3 — điện thoại chỉ có 4 biến thể; `ready` không vẽ thẻ, `missingFfmpeg` chỉ khoá nút Ghép & xuất.
  const showMachine = machine.kind === 'noMachine' || machine.kind === 'offline' || machine.kind === 'provisioning' || machine.kind === 'provisionFailed'
  const entryAction = genAction(genPending.filter((row) => row.state === 'stale').length, genPending.filter((row) => row.state === 'new').length, 0)
  // Đổi chương bằng tap: play() chạy đồng bộ trong cử chỉ (iOS mở khoá audio),
  // effect của usePlayer thấy src đã nạp trên cùng một thẻ nên không nạp lại.
  const step = useCallback((direction: 1 | -1) => {
    const next = stepChapter(queue, chapter, direction)
    if (next !== null) {
      if (player.playing) void player.play(chapterURL(next)).catch(() => {})
      setAutoplay(player.playing); setSelected(next)
    }
  }, [queue, chapter, player.playing, player.play, project, story, volume, data.volume])
  useEffect(() => {
    if (sleep.deadline === null) return
    const check = () => {
      if (!sleepExpired(sleep, Date.now())) return
      player.pause(); setAutoplay(false); setSleep(startSleep('off', 0)); notify('Đã tắt trình phát theo hẹn giờ')
    }
    const timer = setInterval(check, 1000)
    window.addEventListener('focus', check)
    document.addEventListener('visibilitychange', check)
    check()
    return () => { clearInterval(timer); window.removeEventListener('focus', check); document.removeEventListener('visibilitychange', check) }
  }, [sleep, player.pause, notify])
  // Media Session (A7): artist luôn "Noverse Audio", bìa truyện — không trạng
  // thái giọng, không chữ viết tắt; ⏮ ⏭ = chương trước/sau.
  useMediaSession(
    current && playable ? {
      project, story, volume,
      title: chapterTrackTitle(current.chapter, current.title),
      album: audioAlbumTitle(storyTitle || story, volume), duration,
    } : null,
    player, { onPrevious: () => step(-1), onNext: () => step(1) })
  const close = useCallback(() => setSheet(null), [])
  const openVoice = (opener: 'gen' | 'player') => { setVoiceOpener(opener); setSheet('voice') }
  const openExport = useCallback(() => { setExportDone(0); setExportScope(null); setExportShare(false); setSheet('export') }, [])
  /** “Xuất riêng chương này” mở phạm vi Từng chương với đúng chương đã chọn. */
  const openChapterExport = (number: number) => { setExportDone(0); setExportScope(number); setExportShare(false); setSheet('export') }
  /** “Chia sẻ tệp audio” mở view Đã xuất, lọc tệp của chương; chưa xuất thì dẫn sang xuất chương. */
  const openChapterShare = (number: number) => { setExportDone(0); setExportScope(number); setExportShare(true); setSheet('export') }
  // §AT — Thử lại (và Chuẩn bị bộ đọc ở bản cá nhân) gọi cùng một lượt cài; các lối khác mở màn Thêm
  // (điện thoại chưa có trang Giọng đọc riêng).
  const machineAction = (kind: MachineActionKind) => {
    if (kind === 'retry' || kind === 'prepare') { void retryMachine.run(); return }
    navigate(paths.settings('mobile'))
  }
  const openExportDone = useCallback(() => { setExportScope(null); setExportShare(false); setExportDone((count) => count + 1); setSheet('export') }, [])
  const playChapter = (number: number) => {
    setSheet('player')
    // play() đồng bộ trong cử chỉ chạm — iOS mở khoá thẻ audio ngay cả khi đổi chương.
    void player.play(chapterURL(number)).catch(() => {})
    if (chapter !== number) { setAutoplay(true); setSelected(number) }
  }
  /**
   * Thao tác luôn khoanh vào đúng chương đang mở.
   * Lỗi được ném lại để tấm menu hiện ngay tại chỗ, không đóng mất ngữ cảnh.
   */
  const runChapterAction = async (id: ChapterActionId) => {
    if (!menuRow) return
    const n = menuRow.chapter.chapter
    const quality = data.settings?.quality ?? 'standard'
    const voiceMode = data.cast?.voiceMode ?? data.cast?.mode ?? 'cast'
    if (id === 'play') { playChapter(n); setMenuChapter(null); return }
    if (id === 'pausePlayback') { player.pause(); setMenuChapter(null); return }
    if (id === 'export') { openChapterExport(n); setMenuChapter(null); return }
    if (id === 'share') { openChapterShare(n); setMenuChapter(null); return }
    if (id === 'prioritize') {
      await prioritizeAudioChapter(project, story, volume, n)
      void invalidate(keys.story(project, story, 'audio'))
      void invalidate(keys.jobs())
      notify(t.audio.chapterActions.prioritized(n))
      setMenuChapter(null)
      return
    }
    if (id === 'update' || id === 'create' || id === 'redo') {
      const result = await enqueueAudioChapters(project, story, volume, id === 'redo'
        ? { chapters: [], redo: [n], quality, voiceMode, lock: false }
        : genQueueInput([menuRow], quality, voiceMode))
      void invalidate(keys.story(project, story, 'audio'))
      void invalidate(keys.jobs())
      notify(result.chapters.includes(n)
        ? id === 'update' ? t.audio.generation.updQueuedToast : t.audio.generation.queuedToast(1)
        : t.audio.chapterActions.current)
      setMenuChapter(null)
      return
    }
    const job = menuRow.job
    if (!job) throw new Error(t.audio.chapterActions.missingJob)
    if (id === 'retry' || id === 'resumeJob') await resumeAudioRender(project, story, job.jobId)
    else if (id === 'pauseJob') await pauseAudioRender(project, story, job.jobId)
    else await cancelAudioRender(project, story, job.jobId)
    void invalidate(keys.story(project, story, 'audio'))
    void invalidate(keys.jobs())
    notify(id === 'retry' ? t.audio.chapterActions.retried(n)
      : id === 'resumeJob' ? t.audio.chapterActions.resumed(n)
      : id === 'pauseJob' ? t.audio.chapterActions.paused(n)
      : t.audio.chapterActions.cancelled(n))
    setMenuChapter(null)
  }
  const selectVolume = (next: number) => {
    close()
    if (next === volume) return
    player.pause()
    setAutoplay(false)
    setSelected(null)
    setPickRedo([])
    navigate(paths.audio(project, story, next, 'listen'))
  }
  const playButton = (large = false) => <button className={large ? 'ma-play-main' : 'ma-icon'} aria-label={player.playing ? 'Tạm dừng' : 'Phát'} disabled={!playable} onClick={player.toggle}><Icon name={player.playing ? 'pause' : 'play'} size={large ? 32 : 24} /></button>
  // Tiêu đề và dòng phụ luôn theo chương đang mở.
  const menuQuality = data.settings?.quality ?? 'standard'
  const menuItems = menuRow ? chapterActions(menuRow, { ready: queue.includes(menuRow.chapter.chapter), playing: chapter === menuRow.chapter.chapter && player.playing, writable: access.writable, quality: menuQuality }) : []
  const menuTitle = menuRow ? t.audio.chapterActions.heading(menuRow.chapter.chapter, menuRow.chapter.title) : t.audio.chapterActions.title
  const menuDuration = menuRow ? menuRow.chapter.audio?.durationSec ?? menuRow.chapter.durationSec ?? 0 : 0
  const menuVoice = data.cast?.mode === 'single' ? t.audio.generation.single
    : data.cast?.voiceMode === 'pack' ? t.audio.generation.pack
      : data.cast?.speakers.length ? `${t.audio.generation.cast} · ${data.cast.speakers.length} ${t.audio.generation.voiceCount}`
        : t.audio.generation.cast
  const menuMeta = menuRow ? [menuDuration > 0 ? formatMinutes(menuDuration) : genTag(menuRow).text, t.audio.generation[menuQuality], menuVoice].filter(Boolean).join(' · ') : ''
  return <div className="ma-root"><MobilePage title={<button className="ma-heading" aria-haspopup="dialog" aria-expanded={sheet === 'volume'} onClick={() => setSheet('volume')}><span className="ma-heading-text"><small>Sách nói</small><span title={`Tập ${volume} · ${storyTitle}`}>Tập {volume} · {storyTitle}</span></span><Icon name="arrow-down-2" size={15} style={{ color: '#7E7E8A' }} /></button>} actions={<button className="ma-export-button" aria-label="Ghép và xuất tập" onClick={openExport}><Icon name="export-curve" size={16} />Xuất</button>}>
    <PanelRefreshBar resourceKeys={[keys.story(project, story, 'audio')]} updatedAt={data.updatedAt} />
    {data.conflict && <div className="ma-warning" role="alert"><span>Đã có thay đổi từ nơi khác.</span><button onClick={data.viewCurrent}>Xem bản mới</button><button disabled={!connection.writable || data.savePending} onClick={() => void data.overwrite()}>Ghi đè</button></div>}
    {data.error && <div className="m-error" role="alert">{data.volume ? 'Không tải lại được · ' : ''}{data.error}<button onClick={data.refresh}>Thử lại</button></div>}
    {player.error && <div className="m-error" role="alert">{player.error}<button onClick={() => void player.play().catch(() => {})}>Thử lại</button></div>}
    {slow && data.loading && <p className="ma-note" role="status">Đang chậm hơn bình thường…</p>}
    {data.loading && !data.volume ? <div className="ma-skeleton" aria-label="Đang tải sách nói"><div />{Array.from({ length: 8 }, (_, i) => <div key={i} />)}</div> : <>
      {showMachine && <MachineCard view={machine} onAction={machineAction} />}
      {/* Gói AK — thẻ “Tạo giọng · Tập N” dùng chung nhãn nút chính với chân ngăn và thanh Phòng thu. */}
      <section className="ma-gencard" aria-label={`${t.audio.generation.title} · Tập ${volume}`}>
        <button type="button" className="ma-gencard-head" onClick={() => openGen()} title={data.updatedAt ? `Cập nhật lúc ${new Date(data.updatedAt).toLocaleTimeString('vi-VN')}` : undefined}>
          <Icon name="microphone-2" size={22} style={{ color: 'var(--color-accent)', flexShrink: 0 }} />
          <span className="ma-gencard-text">
            <b>{t.audio.generation.title} · Tập {volume}</b>
            <span>{genStripData.text}</span>
          </span>
          <Icon name="arrow-right-2" size={16} style={{ color: '#7E7E8A', flexShrink: 0 }} />
        </button>
        <div className="ma-ov" aria-hidden="true">{genStripData.cells.map((cell) => <span key={cell.key} className={cell.cls}>{cell.cls === 'run' && <i style={{ width: `${cell.pct}%` }} />}</span>)}</div>
        {entryAction && <button type="button" className={`ma-gencard-cta${entryAction.tone === 'update' ? ' warn' : ''}`} onClick={() => openGen()}>
          <Icon name={entryAction.icon} size={18} />{entryAction.label}
        </button>}
      </section>
      <div className="ma-chapters-label">Chương</div>
      <div className="ma-chapters" aria-label="Danh sách chương">{chapters.map((item) => {
        const ready = queue.includes(item.chapter)
        const on = chapter === item.chapter
        const job = activeJobs.find((task) => task.id.endsWith(`/render/${item.chapter}`) || task.id.endsWith(`/chapters/${item.chapter}`))
        const row = genRowList.find((entry) => entry.chapter.chapter === item.chapter)
        const queued = data.queue.find((task) => task.chapter === item.chapter)
        const running = item.audio?.rendering || queued?.state === 'rendering' || !!job
        const percent = job?.progress ?? (queued?.percent ?? item.audio?.percent ?? 0) / 100
        const tag = row ? genTag(row) : null
        const label = tag?.text || (ready ? formatMinutes(item.audio?.durationSec ?? item.durationSec ?? 0) : t.audio.volumePicker.waiting)
        // Menu ⋯ chỉ hiện khi chương có thao tác thật (chờ máy đọc / đang soạn thì hàng tự nói trạng thái).
        const actions = row ? chapterActions(row, { ready, playing: on && player.playing, writable: access.writable, quality: data.settings?.quality ?? 'standard' }) : []
        return <div className={`ma-chapter${on ? ' current' : ''}${!ready && !running ? ' waiting' : ''}`} key={item.chapter}>
          {ready ? <button className="ma-chapter-play" aria-label={`Nghe chương ${item.chapter}`} onClick={() => playChapter(item.chapter)}>{on && player.playing ? <Eq /> : <Icon name="play" size={15} />}</button>
            : running ? <Ring percent={percent}>{Math.round(percent * 100)}%</Ring>
              : <span className="ma-wait-ring" />}
          <span className="ma-chapter-text">
            <span title={item.title}>{item.chapter}. {item.title}</span>
            <small className="ma-ch-meta">
              {label && <span className={`ma-tag${tag?.tone ? ` ${tag.tone}` : ''}`} title={item.staleReason || label}>{label}</span>}
            </small>
          </span>
          {actions.length > 0 && <button type="button" className="ma-chapter-menu" aria-label={`Thao tác chương ${item.chapter}`} aria-haspopup="dialog" aria-expanded={menuChapter === item.chapter} onClick={(event) => { event.stopPropagation(); setMenuChapter(item.chapter) }}><Icon name="more" size={18} /></button>}
        </div>
      })}</div>
      {!chapters.length && <p className="ma-note">Chưa có chương trong tập này.</p>}
      <p className="ma-note">Duyệt và sửa bản đọc chi tiết nên làm trên máy tính. Ở đây bạn nghe, theo dõi tiến độ và xuất tập.</p>
    </>}
  </MobilePage>
  <VolumeSheet project={project} story={story} volume={volume} open={sheet === 'volume'} onClose={close} onSelect={selectVolume}/>
  {current && queue.includes(current.chapter) && <div className="ma-mini"><i style={{ width: `${fraction * 100}%` }} /><button aria-label="Mở trình phát" onClick={() => setSheet('player')}><span className="ma-avatar">{initials}</span><span><strong>Ch. {chapter} · {current.title}</strong><small>{readerName} · {formatMinutes(player.time)}</small></span></button>{playButton()}</div>}
  <BottomSheet open={sheet === 'player'} onClose={close} height={800} label="Trình phát" style={{ background: 'radial-gradient(120% 60% at 50% 0%,var(--kit-run-14),transparent 60%),var(--color-card)', paddingBottom: 30 }}>
    <div className="ma-player"><header><button className="ma-icon" onClick={close} aria-label="Thu nhỏ trình phát"><Icon name="arrow-down-2" size={22} /></button><span>Đang nghe · Tập {volume}</span><span /></header><div className="ma-now"><button className="ma-portrait" aria-label="Chọn giọng đọc" onClick={() => openVoice('player')}>{player.playing && <><i /><i /></>}<span>{initials}</span></button><div><strong>{current?.title ?? 'Chưa có chương'}</strong><span>Chương {chapter} · {playable ? `${voice} đang đọc` : 'Chưa có giọng đọc'}</span></div></div>
      {(current?.chapterChanged || current?.scriptChanged) && <div className="ma-warning" role="status">{current.staleReason || t.audio.generation.edited}</div>}
      <p className="ma-karaoke">{!data.loading && !current ? `Không tìm thấy chương ${chapter} trong tập này.` : !data.loading && !playable ? `Chương ${chapter} chưa có giọng. Bạn có thể nghe khi việc tạo giọng hoàn tất.` : words.length ? words.map((word, index) => <span key={`${entry?.segment.id}-${index}`} className={index === wordIndex ? 'active' : index < wordIndex ? 'read' : ''}>{word} </span>) : script.error ? 'Không tải được lời đọc.' : 'Đang tải lời đọc…'}</p>
      {Boolean(script.error || peaks.error) && <button className="ma-retry" onClick={() => { void script.refresh(); void peaks.refresh() }}>Không tải lại được · Thử lại</button>}
      <MobileWave bars={bars} fraction={fraction} time={player.time} duration={duration} disabled={!playable} onSeek={player.seek} /><div className="ma-times"><span>{formatMinutes(player.time)}</span><span>-{formatMinutes(Math.max(0, duration - player.time))}</span></div>
      <div className="ma-controls"><button className="ma-icon step" aria-label="Chương trước" disabled={stepChapter(queue, chapter, -1) === null} onClick={() => step(-1)}><Icon name="previous" size={24} /></button><button className="ma-icon skip" aria-label="Lùi 10 giây" disabled={!playable} onClick={() => player.skip(-10)}><Icon name="backward-10seconds" size={28} /></button>{playButton(true)}<button className="ma-icon skip" aria-label="Tới 10 giây" disabled={!playable} onClick={() => player.skip(10)}><Icon name="forward-10seconds" size={28} /></button><button className="ma-icon step" aria-label="Chương sau" disabled={stepChapter(queue, chapter, 1) === null} onClick={() => step(1)}><Icon name="next" size={24} /></button></div>
      <div className="ma-chips"><button className="m-chip" aria-label="Tốc độ đọc" onClick={() => player.setRate(SPEEDS[(SPEEDS.findIndex((value) => value === player.rate) + 1) % SPEEDS.length])}><Icon name="speedometer" size={15} /><span className="mono num">{player.rate.toFixed(player.rate === 1 || player.rate === .8 || player.rate === 1.5 ? 1 : 2)}×</span></button><button className={`m-chip${sleep.mode !== 'off' ? ' on' : ''}`} onClick={() => setSleep(nextSleep(sleep, Date.now()))}><Icon name="timer-1" size={15} />{SLEEP_LABELS[sleep.mode]}</button><button className="m-chip" onClick={close}><Icon name="note-text" size={15} />Chương</button></div>
      {player.error && <p className="m-error" role="alert">{player.error}</p>}
    </div>
  </BottomSheet>
  <VoiceGenSheet project={project} story={story} volume={volume} data={data} open={sheet==='gen'} onClose={close} onVoice={()=>openVoice('gen')} onExport={openExport} writable={access.writable} initialRedo={pickRedo} machineHold={machineHold}/>
  <VoiceSheet open={sheet==='voice'} onClose={()=>setSheet(voiceOpener)}/>
  <ChapterActions open={menuRow !== null} title={menuTitle} meta={menuMeta} items={menuItems} onClose={() => setMenuChapter(null)} onAction={runChapterAction}/>
  <ExportSheet project={project} story={story} volume={volume} open={sheet === 'export'} onClose={close} readyChapters={queue} total={chapters.length} queue={data.queue} writable={access.writable} initialDone={initialState === 'exportDone'} initialView={exportScope !== null ? (exportShare ? 'exportChDone' : 'exportCh') : initialState} trialChapter={exportScope ?? chapter ?? chapters[0]?.chapter ?? 1} focusChapter={exportScope ?? undefined} openDone={exportDone}/>
  <ExportJobCard project={project} story={story} volume={volume} onViewFiles={openExportDone}/>
  </div>
}
