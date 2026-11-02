// Dữ liệu + phát audio cho màn Audio. Mọi thay đổi đi qua API thật (mock ở dev),
// SSE `S/audio/events` đẩy tiến độ tạo giọng; audio phát bằng HTMLAudioElement (tự gửi Range).
import { useCallback, useEffect, useRef, useState } from 'react'
import {
  ApiError, ConflictError, addAudioFlag, deleteAudioFlag, getAudioCast, getAudioFlags, getAudioQueue,
  getAudioReady, getAudioScript, getAudioSettings, getAudioVolume, getProjectVoices, putAudioCast, putAudioSettings,
} from '../../api/client'
import { useSession } from '../../api/session'
import { canWriteStory } from '../../api/client'
import type {
  AudioCast, AudioFlag, AudioQueue, AudioQueueJob, AudioReady, AudioRenderEvent, AudioSettings,
  AudioVolume, AudioVoice, TtsStatus,
} from '../../api/types'
import { getAudioExportState, getTtsStatus } from '../../api/client'
import { t } from '../../i18n/vi'
import { invalidate, keys, setData, subscribeEvents, useAction, useConnection, useResource } from '../../data'
import { toast } from '@noverse/ui'

export const errText = (error: unknown) => {
  if (typeof error === 'string') return error
  if (error instanceof ApiError) return error.message
  return error instanceof Error ? error.message : t.audio.data.khongKetNoiDuoc
}

function playbackSource(source: string): string {
  const url = new URL(source, window.location.href)
  if (url.origin !== window.location.origin || !url.pathname.startsWith('/api/')) return source
  url.searchParams.set('inline', '1')
  return url.href
}

/** WAV im lặng 2 ms — phát ngay trong cử chỉ để iOS mở khoá thẻ audio. */
const SILENT_WAV = 'data:audio/wav;base64,UklGRlYAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgATElTVBoAAABJTkZPSVNGVA0AAABMYXZmNjMuMS4xMDIAAGRhdGEQAAAAgICAgICAgICAgICAgICAgA=='

/** iOS: play() gọi sau await bị NotAllowedError. Gọi hàm này ĐỒNG BỘ trong
 * handler chạm để thẻ audio được người dùng cho phép; lượt play() thật sau đó
 * (khi URL đã về) dùng lại đúng thẻ này nên không cần cử chỉ nữa. */
export function unlockMedia(element: HTMLMediaElement) {
  element.src = SILENT_WAV
  element.play().catch(() => {})
}

/**
 * Các thẻ audio của màn Audio. Chúng được tạo bằng `new Audio()` nên không nằm
 * trong DOM: chỉ danh sách này biết thẻ nào đang phát. Nhờ đó nghe thử giọng
 * tạm dừng chương (và ngược lại) — hai tiếng không bao giờ chồng nhau, đúng
 * như một trình phát duy nhất.
 */
const mediaElements = new Set<HTMLMediaElement>()

function trackMedia(element: HTMLMediaElement) {
  mediaElements.add(element)
  return element
}

function releaseMedia(element: HTMLMediaElement | null) {
  if (element) mediaElements.delete(element)
}

function pauseOthers(element: HTMLMediaElement) {
  for (const other of mediaElements) if (other !== element) other.pause()
}

/**
 * Hai mốc "ý định phát" để tệp về muộn không cướp lượt phát: một lượt nghe thử
 * chen vào lúc chương đang chờ, hoặc người dùng bấm phát chương lúc nghe thử
 * đang chờ máy đọc. Lượt cũ tự bỏ — và không pause lượt mới.
 */
let chapterStarts = 0
let previewStarts = 0


/**
 * Lỗi phát audio → tiếng Việt. DOMException của HTMLMediaElement.play() luôn
 * tiếng Anh (iOS: "The request is not allowed by the user agent…") nên không
 * bao giờ hiện nguyên văn. AbortError = lượt phát bị thao tác khác cắt ngang:
 * trả '' để caller im lặng, không phải lỗi.
 */
export function playError(error: unknown): string {
  if (error instanceof DOMException) {
    switch (error.name) {
      case 'AbortError': return ''
      case 'NotAllowedError': return 'Chạm nút phát để nghe.'
      case 'NotSupportedError': return 'Không phát được tệp âm thanh này.'
      default: return 'Không phát được tệp âm thanh.'
    }
  }
  return errText(error)
}

/** `media.error` của thẻ audio (sự kiện `error`, không phải promise của play()). */
export function mediaErrorText(media: HTMLMediaElement | null): string {
  const code = media?.error?.code
  if (code === 1) return '' // MEDIA_ERR_ABORTED — bị src mới/người dùng cắt
  if (code === 2) return 'Không tải được tệp âm thanh. Kiểm tra kết nối rồi thử lại.'
  return 'Không phát được tệp âm thanh. Thử tải lại hoặc chọn chương khác.'
}

/** 404 là chương chưa soạn theo contract audio, không phải lỗi tải bản đọc. */
export async function loadAudioScript(project: string, story: string, volume: number, chapter: number, signal?: AbortSignal) {
  try { return await getAudioScript(project, story, volume, chapter, signal) }
  catch (error) {
    if (error instanceof ApiError && error.status === 404) return null
    throw error
  }
}

type AudioEventHandlers = {
  onRender?: (event: AudioRenderEvent) => void
  onQueue?: (jobs: AudioQueueJob[]) => void
  onCompose?: (payload: { chapter: number; state: string; error?: string }) => void
  onExport?: (payload: { volume: number; percent: number; state: string }) => void
}

/** Quyền ghi của truyện: reader chỉ nghe/tải, truyện đang mở bởi tiến trình khác thì khoá. */
export function useAudioAccess(project: string, story: string) {
  const { user } = useSession()
  const { writable } = useConnection()
  return { writable: writable && !!user && canWriteStory(user, `${project}/${story}`) }
}

/** Nghe sự kiện audio trên luồng SSE chung; handler giữ trong ref để không nối lại mỗi lần render. */
export function useAudioEvents(project: string, story: string, handlers: AudioEventHandlers, enabled = true) {
  const ref = useRef(handlers)
  ref.current = handlers
  useEffect(() => {
    if (!enabled) return
    // Một kết nối SSE cho mỗi tab: máy chủ phát sự kiện audio thành khung `live`
    // (`kind = "audio."+loại`) trên /api/events. Không mở EventSource riêng nữa —
    // mỗi màn một kết nối sẽ đụng trần 6 kết nối/origin của HTTP/1.1.
    return subscribeEvents('live', (raw) => {
      const frame = liveFrameOf(raw)
      if (!frame || frame.project !== project || frame.story !== story) return
      // Khung `live.data` là nguyên khung cũ `{type, category, data}`, nên dữ liệu
      // thật nằm ở `.data.data` (xem internal/audio/queue.go type Event).
      const payload = recordOf(recordOf(frame.data)?.data)
      if (!payload) return
      switch (frame.kind) {
        case 'audio.render':
          if (typeof payload.jobId === 'string') ref.current.onRender?.(payload as unknown as AudioRenderEvent)
          break
        case 'audio.queue':
          if (Array.isArray(payload.jobs)) ref.current.onQueue?.(payload.jobs as AudioQueueJob[])
          break
        case 'audio.compose':
          if (typeof payload.chapter === 'number' && typeof payload.state === 'string') {
            ref.current.onCompose?.(payload as unknown as { chapter: number; state: string; error?: string })
          }
          break
        case 'audio.export':
          if (typeof payload.volume === 'number' && typeof payload.percent === 'number' && typeof payload.state === 'string') {
            ref.current.onExport?.(payload as unknown as { volume: number; percent: number; state: string })
          }
          break
      }
    })
  }, [project, story, enabled])
}

/** `live` frame từ /api/events: {project, story, kind, data}. Khung lạ bị bỏ. */
export function liveFrameOf(raw: unknown): { project: string; story: string; kind: string; data: unknown } | null {
  const frame = recordOf(raw)
  if (!frame) return null
  const { project, story, kind } = frame
  if (typeof project !== 'string' || typeof story !== 'string' || typeof kind !== 'string') return null
  return { project, story, kind, data: frame.data }
}

/** Dữ liệu từ ngoài (JSON SSE): chỉ nhận object, còn lại bỏ. */
function recordOf(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' ? value as Record<string, unknown> : null
}

export type AudioConflict =
  | { kind: 'cast'; attempted: AudioCast; current: AudioCast | null }
  | { kind: 'settings'; attempted: AudioSettings; current: AudioSettings | null }

/**
 * Áp tiến độ một đoạn vào hàng đợi tại chỗ. Sự kiện `audio.render` đến mỗi đoạn;
 * tải lại cả màn theo từng sự kiện làm màn nháy và mất vị trí cuộn, nên chip
 * "Đang tạo giọng…" chỉ đổi từ dữ liệu đã có. Trả nguyên mảng khi không có job
 * tương ứng (hàng đợi chưa tải xong) để không dựng job giả.
 */
export function applyRenderToQueue(jobs: AudioQueueJob[], event: AudioRenderEvent): AudioQueueJob[] {
  let hit = false
  const next = jobs.map((job) => {
    if (job.jobId !== event.jobId) return job
    hit = true
    return {
      ...job,
      chapter: event.chapter,
      state: event.state,
      segmentsDone: event.segment,
      segments: event.segments || job.segments,
      percent: event.percent,
      etaSec: event.etaSec,
      speaker: event.speaker || job.speaker,
      ...(event.text !== undefined ? { currentText: event.text } : {}),
      ...(event.segmentPercent !== undefined ? { segmentPercent: event.segmentPercent } : {}),
    }
  })
  return hit ? next : jobs
}

/**
 * Áp tiến độ vào chương đang tạo trong `audio/volumes`: vòng phần trăm và số đoạn
 * của tập đổi ngay mà không phải hỏi máy chủ. Xong/hỏng thì thôi hiện "đang tạo".
 */
export function applyRenderToVolume(volume: AudioVolume, event: AudioRenderEvent): AudioVolume {
  const index = volume.chapters.findIndex((item) => item.chapter === event.chapter)
  if (index < 0) return volume
  const chapter = volume.chapters[index]
  const total = event.segments || chapter.audio?.segments || chapter.segments || 0
  const done = event.state === 'done'
    ? (total || event.segment)
    : Math.max(chapter.audio?.segmentsDone ?? 0, Math.min(event.segment, total || event.segment))
  const audio = {
    segmentsDone: done,
    segments: total || chapter.audio?.segments || 0,
    durationSec: chapter.audio?.durationSec ?? null,
    rendering: event.state === 'rendering',
    percent: total > 0 ? Math.min(100, (done / total) * 100) : event.percent,
  }
  const chapters = volume.chapters.slice()
  chapters[index] = { ...chapter, audio }
  return { chapters }
}

export type AudioData = {
  volume: AudioVolume | null
  cast: AudioCast | null
  voices: AudioVoice[]
  settings: AudioSettings | null
  queue: AudioQueueJob[]
  timeline?: AudioQueue
  ready: AudioReady | null
  flags: AudioFlag[]
  live: AudioRenderEvent | null
  loading: boolean
  refreshing: boolean
  updatedAt: number | null
  error: string
  conflict: AudioConflict | null
  composeError: string
  savePending: boolean
  saveSuccess: boolean
  viewCurrent: () => void
  overwrite: () => Promise<void>
  refresh: () => void
  reloadVoices: () => Promise<void>
  saveCast: (cast: AudioCast) => Promise<boolean>
  saveSettings: (settings: AudioSettings) => Promise<boolean>
  reloadQueue: () => Promise<void>
  addFlag: (input: { chapter: number; segmentId: string; atSec: number; reason: string }) => Promise<void>
  removeFlag: (id: string) => Promise<void>
}

export function useAudioData(project: string, story: string, volume: number): AudioData {
  const prefix = keys.story(project, story, 'audio')
  const volumeKey = keys.story(project, story, 'audio', 'volume', volume)
  const castKey = keys.story(project, story, 'audio', 'cast')
  const settingsKey = keys.story(project, story, 'audio', 'settings')
  const queueKey = keys.story(project, story, 'audio', 'queue', volume)
  const flagsKey = keys.story(project, story, 'audio', 'flags', volume)
  const volumeResource = useResource(volumeKey, ({ signal }) => getAudioVolume(project, story, volume, signal), { keepPrevious: true })
  const castResource = useResource(castKey, ({ signal }) => getAudioCast(project, story, signal))
  const settingsResource = useResource(settingsKey, ({ signal }) => getAudioSettings(project, story, signal))
  const readyResource = useResource(keys.story(project, story, 'audio', 'ready'), ({ signal }) => getAudioReady(project, story, signal))
  const queueResource = useResource(queueKey, ({ signal }) => getAudioQueue(project, story, signal, volume))
  const flagsResource = useResource(flagsKey, ({ signal }) => getAudioFlags(project, story, volume, signal))
  // GET /projects/:p/voices đã gộp giọng dự án với giọng có sẵn của bộ đọc (builtIn);
  // khi bộ đọc chưa cài thì chỉ trả giọng dự án, nên không gọi thêm /tts/voices.
  const projectVoices = useResource(keys.project(project, 'voices'), ({ signal }) => getProjectVoices(project, signal))
  // Cài xong bộ đọc thì danh sách mới có giọng có sẵn: tải lại đúng lúc chuyển sang sẵn sàng.
  const engineReady = readyResource.data?.engine.ready
  const engineWasReady = useRef(engineReady)
  useEffect(() => {
    if (engineReady && engineWasReady.current === false) void invalidate(keys.project(project, 'voices'))
    engineWasReady.current = engineReady
  }, [engineReady, project])
  // Bộ đọc đã cài nhưng tiến trình chưa lên (bị tắt sau 10 phút rảnh, hoặc máy chủ
  // vừa khởi động): màn đọc không chờ nạp mô hình nữa, nên hỏi lại trạng thái thưa
  // cho tới khi máy đọc sẵn sàng — danh sách giọng có sẵn sẽ tự xuất hiện (effect
  // trên tải lại khi `engineReady` chuyển false → true).
  const engineWarming = !!readyResource.data?.ready && !engineReady
  const refreshReady = readyResource.refresh
  useEffect(() => {
    if (!engineWarming) return
    const timer = window.setInterval(() => { void refreshReady() }, 2000)
    return () => window.clearInterval(timer)
  }, [engineWarming, refreshReady])
  const [live, setLive] = useState<AudioRenderEvent | null>(null)
  const [conflict, setConflict] = useState<AudioConflict | null>(null)
  const [composeError, setComposeError] = useState('')
  useEffect(() => { setComposeError('') }, [project, story, volume])
  const refresh = useCallback(() => { void invalidate(keys.story(project, story, 'audio')) }, [project, story])
  const reloadQueue = async () => { await queueResource.refresh() }

  useAudioEvents(project, story, {
    onRender: (event) => {
      setLive(event)
      // Tiến độ: chỉ đổi chip + vòng phần trăm + dòng đang đọc, tại chỗ. Không
      // invalidate cả nhóm audio (mỗi đoạn một lần sẽ nháy màn và tải lại 7 tài
      // nguyên, kéo theo skeleton/giật).
      if (queueResource.data) setData(queueKey, { ...queueResource.data, jobs: applyRenderToQueue(queueResource.data.jobs, event) })
      if (volumeResource.data) setData(volumeKey, applyRenderToVolume(volumeResource.data, event))
      // Tổng giây thật chỉ thay đổi khi một đoạn xong: hỏi lại riêng hàng đợi,
      // không tải lại màn hay các tài nguyên giọng/bản đọc ở từng nhịp phần trăm.
      const previous = queueResource.data?.jobs.find(job => job.jobId === event.jobId)
      if (event.state === 'rendering' && previous && event.segment !== previous.segmentsDone) void queueResource.refresh()
      // Xong/hỏng một chương thì chỉ làm mới bản đọc của chương đó (URL audio từng
      // đoạn, trạng thái chốt) và số liệu tập; phân vai/cách đọc/giọng giữ nguyên.
      if (event.state === 'done' || event.state === 'error') {
        void invalidate(keys.story(project, story, 'audio', 'script', volume, event.chapter))
        if (volumeResource.data) void volumeResource.refresh()
        void queueResource.refresh()
      }
    },
    onQueue: (jobs) => {
      const chapters = queueResource.data?.chapters ?? volumeResource.data?.chapters
      if (chapters && volumeResource.updatedAt !== null) {
        const numbers = new Set(chapters.map(chapter => chapter.chapter))
        setData(queueKey, { ...queueResource.data, jobs: jobs.filter(job => numbers.has(job.chapter)) })
      }
      void queueResource.refresh()
    },
    onCompose: (payload) => {
      if (payload.state === 'error') setComposeError(payload.error || 'Không soạn được bản đọc. Kiểm tra nhà cung cấp AI rồi thử lại.')
      else if (payload.state === 'drafting') setComposeError('')
      if (payload.state === 'done' || payload.state === 'error') {
        void invalidate(keys.story(project, story, 'audio', 'script', volume, payload.chapter))
        void invalidate(volumeKey)
      }
    },
    onExport: (payload) => {
      if (payload.state !== 'done' && payload.state !== 'error') return
      void invalidate(keys.story(project, story, 'audio', 'exports', payload.volume))
      void invalidate(keys.story(project, story, 'exports'))
    },
  })

  const castAction = useAction(async (cast: AudioCast, force = false) => {
    try {
      const saved = await putAudioCast(project, story, cast, force ? '' : cast.version)
      setData(castKey, saved)
      setConflict(null)
      return saved
    } catch (error) {
      if (error instanceof ConflictError) setConflict({ kind: 'cast', attempted: cast, current: error.current as AudioCast | null })
      throw error
    }
  }, {
    optimistic: (cast, _force = false) => {
      const before = castResource.data
      setData(castKey, cast)
      return () => { if (before) setData(castKey, before) }
    },
    invalidate: [prefix],
  })
  const settingsAction = useAction(async (settings: AudioSettings, force = false) => {
    try {
      const saved = await putAudioSettings(project, story, settings, force ? '' : settings.version)
      setData(settingsKey, saved)
      setConflict(null)
      return saved
    } catch (error) {
      if (error instanceof ConflictError) setConflict({ kind: 'settings', attempted: settings, current: error.current as AudioSettings | null })
      throw error
    }
  }, {
    optimistic: (settings, _force = false) => {
      const before = settingsResource.data
      setData(settingsKey, settings)
      return () => { if (before) setData(settingsKey, before) }
    },
    invalidate: [prefix],
  })
  const addFlagAction = useAction(async (input: { chapter: number; segmentId: string; atSec: number; reason: string }) => {
    const flag = await addAudioFlag(project, story, input)
    setData<{ flags: AudioFlag[] }>(flagsKey, (before) => ({ flags: [...(before?.flags ?? []), flag] }))
    return flag
  }, { success: t.audio.listen.daDanhDauCho, invalidate: [flagsKey] })
  const removeFlagAction = useAction(async (flag: AudioFlag) => {
    await deleteAudioFlag(project, story, flag.id)
    return flag
  }, {
    success: 'Đã xoá đánh dấu',
    optimistic: (flag) => {
      const before = flagsResource.data
      setData(flagsKey, { flags: (before?.flags ?? []).filter((item) => item.id !== flag.id) })
      return () => { if (before) setData(flagsKey, before) }
    },
    undo: async (flag) => {
      await addAudioFlag(project, story, { chapter: flag.chapter, segmentId: flag.segmentId ?? '', atSec: flag.atSec, reason: flag.reason })
    },
    invalidate: [flagsKey],
  })
  const resources = [volumeResource, castResource, settingsResource, readyResource, queueResource, flagsResource, projectVoices]
  const failure = resources.find((resource) => resource.error)?.error
  return {
    volume: volumeResource.data ?? null,
    cast: castResource.data ?? null,
    settings: settingsResource.data ?? null,
    ready: readyResource.data ?? null,
    voices: projectVoices.data ?? [],
    flags: flagsResource.data?.flags ?? [],
    queue: queueResource.data?.jobs ?? [],
    timeline: queueResource.data,
    live, conflict, composeError,
    loading: volumeResource.initialLoading,
    refreshing: resources.some((resource) => resource.refreshing),
    updatedAt: volumeResource.updatedAt,
    error: failure ? errText(failure) : '',
    savePending: castAction.pending || settingsAction.pending,
    saveSuccess: castAction.success || settingsAction.success,
    viewCurrent: () => {
      if (conflict?.current) setData(conflict.kind === 'cast' ? castKey : settingsKey, conflict.current)
      setConflict(null)
      refresh()
    },
    overwrite: async () => {
      if (conflict?.kind === 'cast') await castAction.run(conflict.attempted, true)
      else if (conflict?.kind === 'settings') await settingsAction.run(conflict.attempted, true)
    },
    refresh,
    reloadVoices: async () => { await projectVoices.refresh() },
    saveCast: async (cast) => !!await castAction.run(cast),
    saveSettings: async (settings) => !!await settingsAction.run(settings),
    reloadQueue,
    addFlag: async (input) => { await addFlagAction.run(input) },
    removeFlag: async (id) => {
      const flag = flagsResource.data?.flags.find((item) => item.id === id)
      if (flag) await removeFlagAction.run(flag)
    },
  }
}

/** Thành phẩm được làm mới cùng các khoá audio khi job hoặc story.changed đến. */
export function useAudioExport(project: string, story: string, volume: number) {
  const exported = useResource(keys.story(project, story, 'audio', 'exports', volume),
    ({ signal }) => getAudioExportState(project, story, volume, signal))
  const tts = useResource<TtsStatus>(['tts', 'status'], ({ signal }) => getTtsStatus(signal))
  return {
    state: exported.data ?? null, tts: tts.data ?? null,
    loaded: !exported.initialLoading, error: exported.error, refreshing: exported.refreshing,
    updatedAt: exported.updatedAt, refresh: exported.refresh,
  }
}

export type PlayerOptions = { onEnded?: () => void; autoplay?: boolean }

/** Trình phát một tệp audio (HTMLAudioElement, Range do trình duyệt tự gửi). */
export type Player = {
  playing: boolean
  time: number
  duration: number
  rate: number
  setRate: (rate: number) => void
  /** `next` = phát URL này ngay (đổi chương trong cử chỉ chạm). */
  play: (next?: string) => Promise<void>
  error: string
  toggle: () => void
  pause: () => void
  seek: (seconds: number) => void
  skip: (delta: number) => void
}

export function usePlayer(src: string | null, options: PlayerOptions = {}): Player {
  const [playing, setPlaying] = useState(false)
  const [time, setTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [rate, setRate] = useState(1)
  const [error, setError] = useState('')
  // iOS chỉ mở khoá phát cho thẻ được play() trong một cử chỉ; đổi src trên cùng
  // thẻ (không tạo thẻ mới mỗi chương) để chuyển chương tự động không cần chạm.
  const element = useRef<HTMLAudioElement | null>(null)
  const loaded = useRef<string | null>(null)
  const latest = useRef(options)
  const rateRef = useRef(rate)
  const frame = useRef<number | null>(null)
  const loadedAt = useRef(0)
  const retrying = useRef(false)
  const resumeAt = useRef<number | null>(null)
  const wantsPlay = useRef(false)
  const refreshSource = (audio: HTMLAudioElement, at = audio.currentTime) => {
    const source = loaded.current
    if (!source) return false
    const url = new URL(source, window.location.href)
    // Always return through same-origin authorization, never reuse a signed URL.
    if (url.origin !== window.location.origin || !url.pathname.startsWith('/api/')) return false
    url.searchParams.set('inline', '1')
    url.searchParams.set('_media', String(Date.now()))
    resumeAt.current = at
    loadedAt.current = Date.now()
    audio.src = url.href
    audio.load()
    return true
  }
  const stopClock = () => {
    if (frame.current !== null) cancelAnimationFrame(frame.current)
    frame.current = null
  }
  latest.current = options
  rateRef.current = rate
  const make = () => {
    const audio = trackMedia(new Audio())
    audio.preload = 'metadata'
    audio.setAttribute('playsinline', '')
    audio.playbackRate = rateRef.current
    const onTime = () => setTime(audio.currentTime)
    const onMeta = () => {
      setDuration(Number.isFinite(audio.duration) ? audio.duration : 0)
      if (resumeAt.current !== null) {
        audio.currentTime = resumeAt.current
        resumeAt.current = null
        if (wantsPlay.current) void audio.play().catch((cause) => setError(playError(cause)))
      }
    }
    // rAF chỉ lấy mẫu đồng hồ media, không cộng thời gian thực: đứng tiếng,
    // tải chậm và đổi tốc độ đều phải giữ nguyên vị trí của tiếng đang phát.
    const sampleClock = () => {
      onTime()
      frame.current = requestAnimationFrame(sampleClock)
    }
    const onPlay = () => {
      setPlaying(true)
      stopClock()
      sampleClock()
    }
    const onPause = () => { stopClock(); onTime(); setPlaying(false) }
    const onEnd = () => { wantsPlay.current = false; onPause(); latest.current.onEnded?.() }
    const onError = () => {
      // HTMLMediaElement hides HTTP status (including an expired S3 403).
      // One fresh authenticated request recovers; a second failure is visible.
      if (!retrying.current && audio.error?.code !== 1) {
        retrying.current = true
        if (refreshSource(audio)) return
      }
      onPause()
      setError(mediaErrorText(audio))
    }
    audio.addEventListener('canplay', () => { retrying.current = false; setError('') })
    audio.addEventListener('timeupdate', onTime)
    audio.addEventListener('loadedmetadata', onMeta)
    audio.addEventListener('durationchange', onMeta)
    audio.addEventListener('play', onPlay)
    audio.addEventListener('pause', onPause)
    audio.addEventListener('ended', onEnd)
    audio.addEventListener('error', onError)
    element.current = audio
    return audio
  }
  const applySrc = (audio: HTMLAudioElement, next: string, keepIntent = false) => {
    if (loaded.current === next) return
    if (!keepIntent) wantsPlay.current = false
    loaded.current = next
    loadedAt.current = Date.now()
    retrying.current = false
    resumeAt.current = null
    audio.src = playbackSource(next)
    audio.load()
  }
  // play() chạy đồng bộ trong handler chạm: KHÔNG await trước audio.play(), src
  // gắn trước khi play. `next` cho phép caller phát chương sắp chọn ngay trong cử
  // chỉ (đổi chương bằng tap); effect đồng bộ src sau đó thấy đã nạp rồi bỏ qua.
  const play = useCallback(async (next?: string) => {
    const audio = element.current ?? make()
    const url = next ?? loaded.current
    if (!url) throw new Error('Chương này chưa có tệp audio để nghe.')
    const previews = previewStarts
    chapterStarts += 1
    pauseOthers(audio)
    wantsPlay.current = true
    applySrc(audio, url, true)
    if (!next && Date.now() - loadedAt.current > 4 * 60_000) refreshSource(audio)
    try {
      await audio.play()
      // Một lượt nghe thử chen vào sau đó: lượt phát chương này không được
      // tiếp tục chồng lên tiếng mẫu.
      if (previewStarts !== previews) { audio.pause(); return }
      setError('')
    }
    catch (cause) { setError(playError(cause)); throw cause }
  }, [])
  useEffect(() => {
    const audio = element.current ?? make()
    // Activity có thể ẩn rồi dựng lại effect trong lúc media còn đang phát.
    // Không giữ callback của thẻ cũ sau khi màn bị ẩn/gỡ.
    return () => {
      audio.pause()
      stopClock()
      audio.removeAttribute('src')
      audio.load()
      releaseMedia(audio)
      loaded.current = null
      element.current = null
    }
  }, [])
  useEffect(() => {
    setPlaying(false)
    setTime(0)
    setDuration(0)
    setError('')
    const audio = element.current
    if (!audio) return
    if (src) applySrc(audio, src, !!latest.current.autoplay)
    else if (loaded.current !== null) { stopClock(); loaded.current = null; audio.removeAttribute('src'); audio.load() }
    if (src && latest.current.autoplay && audio.paused) void play().catch(() => {})
  }, [src, play])
  useEffect(() => { if (element.current) element.current.playbackRate = rate }, [rate])
  const toggle = useCallback(() => {
    const audio = element.current
    if (!audio || !loaded.current) return
    if (audio.paused) void play().catch((cause) => {
      const text = playError(cause)
      if (text) toast(text, 'err', { label: 'Thử lại', onClick: () => { void play().catch((error) => { const retry = playError(error); if (retry) toast.err(retry) }) } })
    })
    else { wantsPlay.current = false; audio.pause() }
  }, [play])
  const pause = useCallback(() => { wantsPlay.current = false; element.current?.pause() }, [])
  const seek = useCallback((seconds: number) => {
    const audio = element.current
    if (!audio) return
    const target = Math.max(0, Math.min(audio.duration || seconds, seconds))
    if (Date.now() - loadedAt.current > 4 * 60_000 && refreshSource(audio, target)) return
    audio.currentTime = target
    setTime(audio.currentTime)
  }, [])
  const skip = useCallback((delta: number) => {
    const audio = element.current
    if (!audio) return
    const target = Math.max(0, Math.min(audio.duration || 0, audio.currentTime + delta))
    if (Date.now() - loadedAt.current > 4 * 60_000 && refreshSource(audio, target)) return
    audio.currentTime = target
    setTime(audio.currentTime)
  }, [])
  return { playing, time, duration, rate, setRate, play, error, toggle, pause, seek, skip }
}

/** Một URL phát trực tiếp, hoặc hàm nạp URL (đoạn cần gọi API tạo blob). */
export type PreviewSource = string | (() => Promise<string>)

export type PreviewAudio = {
  playingId: string | null
  loadingId: string | null
  /** Lỗi phát của thẻ audio (HTTP 5xx, tệp hỏng) — rỗng khi không có. */
  error: string
  play: (id: string, source: PreviewSource) => Promise<void>
  stop: () => void
}

/**
 * Nghe thử đoạn/giọng: phát bằng một thẻ audio dùng lại. Truyền thẳng URL thì
 * `play()` thật được gọi ngay trong cử chỉ chạm; truyền hàm nạp (đoạn cần gọi
 * API tạo blob) thì thẻ đã được mở khoá đồng bộ trước khi chờ.
 */
export function usePreviewAudio(): PreviewAudio {
  const [loadingId, setLoadingId] = useState<string | null>(null)
  const [playingId, setPlayingId] = useState<string | null>(null)
  const [error, setError] = useState('')
  // Thẻ audio duy nhất, mở khoá bằng play() đồng bộ ngay trong cử chỉ đầu tiên
  // (iOS chặn play() sau await). Giữ thẻ sống để các lượt nghe thử sau phát được.
  const audio = useRef<HTMLAudioElement | null>(null)
  const urls = useRef<string[]>([])
  // Mỗi lượt play/stop tăng số này. Một lượt chờ máy đọc lâu có thể về sau khi
  // người dùng đã bấm dừng hoặc chọn giọng khác: kết quả cũ không được phát.
  const request = useRef(0)

  const stop = useCallback(() => {
    request.current += 1
    const element = audio.current
    if (element) {
      // Bỏ src để đóng luôn request đang chờ máy đọc: máy chủ thấy kết nối
      // đóng và bỏ lượt nghe thử thay vì đọc tiếp cho một người đã rời đi.
      element.pause()
      element.removeAttribute('src')
      element.load()
    }
    setPlayingId(null)
    setLoadingId(null)
    setError('')
  }, [])

  const play = useCallback(async (id: string, source: PreviewSource) => {
    if (playingId === id || loadingId === id) { stop(); return }
    stop()
    const ticket = request.current
    const chapters = chapterStarts
    previewStarts += 1
    setLoadingId(id)
    if (!audio.current) {
      const element = trackMedia(new Audio())
      element.setAttribute('playsinline', '')
      element.addEventListener('ended', () => setPlayingId(null))
      // Bị dừng từ bên ngoài (chương bắt đầu phát, đổi tab): nút phải trở lại
      // trạng thái phát, không kẹt ở “đang phát”.
      element.addEventListener('pause', () => setPlayingId(null))
      // HTTP 5xx của một URL media chỉ hiện qua `error` của thẻ, không qua
      // promise play(); không nghe sự kiện này thì người dùng chỉ thấy im lặng.
      element.addEventListener('error', () => {
        const text = mediaErrorText(element)
        setPlayingId(null)
        setLoadingId(null)
        if (text) setError(text)
      })
      audio.current = element
    }
    const element = audio.current
    // Phát WAV im lặng đồng bộ trong cử chỉ để iOS cho phép thẻ; tệp thật phát
    // trên cùng thẻ.
    pauseOthers(element)
    unlockMedia(element)
    const directURL = typeof source === 'string' ? source : ''
    const loadURL = typeof source === 'string' ? null : source
    try {
      // URL trực tiếp: không await gì trước khi gọi play(), nên nó nằm đúng
      // trong cử chỉ chạm. Hàm nạp bất đồng bộ (đoạn cần tạo blob) mới chờ.
      const url = loadURL ? await loadURL() : directURL
      if (loadURL) {
        // Đã dừng, chọn giọng khác, hoặc chương đã bắt đầu phát trong lúc chờ:
        // tệp về muộn không được cướp lượt phát.
        if (ticket !== request.current || chapterStarts !== chapters) {
          if (url.startsWith('blob:')) URL.revokeObjectURL(url)
          return
        }
        if (url.startsWith('blob:')) urls.current.push(url)
      }
      element.src = playbackSource(url)
      await element.play()
      // Một lượt mới đã chiếm thẻ này: không được pause nó. Chỉ dừng khi chương
      // đã bắt đầu phát trong lúc chờ.
      if (ticket !== request.current) return
      if (chapterStarts !== chapters) { element.pause(); return }
      setPlayingId(id)
    } catch (cause) {
      if (ticket !== request.current) return
      const text = playError(cause)
      if (!text) return
      throw new Error(text, { cause })
    } finally {
      if (ticket === request.current) setLoadingId(null)
    }
  }, [playingId, loadingId, stop])

  useEffect(() => () => {
    // Rời màn khi đang chờ máy đọc: đóng request (bỏ src + load) để máy chủ bỏ
    // lượt, và kết quả về sau không được phát. Không setState khi unmount.
    request.current += 1
    const element = audio.current
    if (element) {
      element.pause()
      element.removeAttribute('src')
      element.load()
      releaseMedia(element)
    }
    audio.current = null
    for (const url of urls.current) URL.revokeObjectURL(url)
    urls.current = []
  }, [])

  return { playingId, loadingId, error, play, stop }
}

/** Đồng hồ đếm giây cho “Đã chạy” của phòng thu. */
export function useElapsed(active: boolean, resetKey: string) {
  const [seconds, setSeconds] = useState(0)
  useEffect(() => {
    setSeconds(0)
    if (!active) return
    const startedAt = Date.now()
    const timer = window.setInterval(() => setSeconds(Math.round((Date.now() - startedAt) / 1000)), 1000)
    return () => window.clearInterval(timer)
  }, [active, resetKey])
  return seconds
}
