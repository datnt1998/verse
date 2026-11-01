// Mock Audio (GAP-17…21) — dữ liệu bản đọc/phân vai/hàng đợi tạo giọng + xuất tập.
// Chạy trong mock dev server: trạng thái nằm trong bộ nhớ, hàng đợi tạo giọng tiến theo
// timer để mọi trạng thái của Transcript.dc.html / AudioExport.dc.html đều xem được.
//
// Hai truyện mẫu (mock/server.ts seed):
//   dit-dang-audio/01-tra-xanh-cuop-kich-ban  → review · locked · drafting · stale, tạo giọng chạy nền
//   dit-dang-audio/03-tuyen-bo-binh-dang      → allDone · rendered · listen · exported
//
// Biến môi trường: AINOVEL_MOCK_TTS=missing|installing|error (bộ đọc trên máy)
//                  AINOVEL_MOCK_FFMPEG=0 (thiếu ffmpeg)
import type {
  AudioCast, AudioChapter, AudioExportOptions, AudioExportState, AudioFlag, AudioJobState, AudioNormalize, AudioQueue, AudioQueueJob,
  AudioScript, AudioScriptStatus, AudioSegment, AudioSettings, AudioVolume, AudioVoice, ExportFile, ExportOnExists, Job,
} from '../src/api/types'
import { CHAPTER_TITLES } from './data'
import { bumpMockVersion, emitGlobalEvent, mockVersion } from './events'
import { mirrorAudioJob, removeAudioJob } from './jobs'
import { audioChapterFileName, audioVolumeFileName, audioZipFileName, shortStoryTitle } from '../src/lib/audioName'
import { ttsStatus } from './tts'

export type MockAudioExportOptions = {
  format?: 'mp3' | 'm4b' | 'wav'
  intro?: boolean
  chapterTitles?: boolean
  loudness?: boolean
  gapSec?: number
  chapters?: number[]
  trial?: boolean
  whenReady?: boolean
  chapterMarks?: boolean
  subtitles?: boolean
  split?: boolean
  pack?: 'zip' | 'files'
  onExists?: 'replace' | 'keep' | 'fail' | 'warn'
  /** chỉ mock: ép lỗi ngay để thử nhánh lỗi ('intro' = thiếu nhạc mở đầu). */
  demoFail?: 'intro' | 'blocked'
}
export class MockAudioExportConflict extends Error {
  constructor(message: string, readonly jobId?: string) { super(message) }
}
type MockAudioExportResult = Pick<AudioExportState, 'files' | 'chapterMarks' | 'durationSec' | 'chapterFiles' | 'split' | 'zipUrl'> & { pending?: boolean }
export type PendingAudioExport = {
  jobId: string
  options: MockAudioExportOptions
  paused: boolean
  /** chương đang ghép (0-bắt đầu) — retry giữ nguyên → chạy tiếp từ chương lỗi */
  chapterIndex: number
  /** lỗi cuối (§AE: {message, chapter, cause}) */
  error?: { message: string; chapter?: number; retryable?: boolean; cause?: string }
  result?: MockAudioExportResult
}


const iso = (ms: number) => new Date(ms).toISOString()
const NOW = Date.now()

/** Đoạn mẫu — đúng chữ của Transcript.dc.html (bản đọc chương 3). */
const baseSegments = (): AudioSegment[] => {
  const seg = (
    id: string, speaker: string, text: string, source: string,
    opts: Partial<AudioSegment> = {},
  ): AudioSegment => ({
    id, speaker, text, source, pauseMs: 400, tone: '', ...opts,
  })
  return [
    seg('s0001', 'th', 'Tôi giữ hồ sơ để có ngày xin xem lại căn cứ,', 'Tôi giữ hồ sơ để có ngày xin xem lại căn cứ,',
      { pauseMs: 200, tone: 'Bình thản' }),
    seg('s0002', 'dan', 'tôi nói, cất nó vào túi.', 'tôi nói, cất nó vào túi.', { pauseMs: 300 }),
    seg('s0003', 'th', 'Học phí trường đời hơi đắt, lại không được trả góp.', 'Học phí trường đời hơi đắt, lại không được trả góp.',
      { pauseMs: 800, tone: 'Mỉa mai nhẹ' }),
    seg('s0004', 'unk', 'Nếu cần nguồn thuộc đoàn, gửi yêu cầu cụ thể. Phần kho phát triển vẫn đang chờ hãng duyệt trích.',
      '“Nếu cần nguồn thuộc đoàn, gửi yêu cầu cụ thể. Phần kho phát triển vẫn đang chờ hãng duyệt trích.”',
      { pauseMs: 600, flags: ['unknown_speaker'], speakerGuess: [{ who: 'hc', p: 0.62 }, { who: 'dm', p: 0.3 }] }),
    seg('s0005', 'dan', 'Cuối buổi, Nhược Vy đề nghị dành ngày lai cuối cho trò đọc lời thoại bất kỳ.',
      'Cuối buổi, Nhược Vy đề nghị dành ngày live cuối cho trò đọc lời thoại bất kỳ.',
      { pauseMs: 400, normalized: [{ from: 'live', to: 'lai', kind: 'english' }], flags: ['english'] }),
    seg('s0006', 'nv', 'Chị muốn mọi người cảm nhận trực tiếp công sức viết. Luật có sẵn rồi, mình cứ chơi cho vui.',
      '“Chị muốn mọi người cảm nhận trực tiếp công sức viết. Luật có sẵn rồi, mình cứ chơi cho vui.”',
      { pauseMs: 600, tone: 'Ngọt, giả lả' }),
    seg('s0007', 'dan', 'Đỗ Mẫn cho tổ nội dung đọc lại phụ lục: tổng phần trích từ phim không quá chín mươi giây, gồm kho phát triển đã được cấp phép; chỉ đọc, không phát tệp.',
      'Đỗ Mẫn cho tổ nội dung đọc lại phụ lục: tổng phần trích từ phim không quá 90 giây, gồm kho phát triển đã được cấp phép; chỉ đọc, không phát tệp.',
      { pauseMs: 500, flags: ['long_sentence'], normalized: [{ from: '90 giây', to: 'chín mươi giây', kind: 'number' }] }),
    seg('s0008', 'dan', 'Thẻ được xáo và bốc trước máy, bản chuẩn đoàn phim niêm phong, mở sau lượt đọc.',
      'Thẻ được xáo và bốc trước máy, bản chuẩn đoàn phim niêm phong, mở sau lượt đọc.', { pauseMs: 600 }),
    seg('s0009', 'th', 'Tôi xin giữ máy toàn từ lúc xáo thẻ,', 'Tôi xin giữ máy toàn từ lúc xáo thẻ,',
      { pauseMs: 200, tone: 'Dứt khoát' }),
    seg('s0010', 'dan', 'tôi nói.', 'tôi nói.', { pauseMs: 200 }),
    seg('s0011', 'th', 'Và ghi thời lượng từng lượt.', 'Và ghi thời lượng từng lượt.', { pauseMs: 800 }),
    seg('s0012', 'dan', 'Nhược Vy mỉm cười đồng ý.', 'Nhược Vy mỉm cười đồng ý.', { pauseMs: 1000 }),
    seg('s0013', 'dan', 'Tôi vặn nắp chai nước. Ngày mai, tôi phải dùng cho đáng chín mươi giây ấy.',
      'Tôi vặn nắp chai nước. Ngày mai, tôi phải dùng cho đáng 90 giây ấy.',
      { pauseMs: 2000, tone: 'Chậm, lắng', normalized: [{ from: '90 giây', to: 'chín mươi giây', kind: 'number' }] }),
    seg('s0014', 'nv', 'Vậy em không khách sáo nữa.', 'Vậy em không khách sáo nữa.', { pauseMs: 300 }),
  ]
}

export const VOICES: AudioVoice[] = [
  { id: 'giong-tri-ha', name: 'Mẫu giọng Tri Hạ', file: 'giong/giong-tri-ha.wav', durationSec: 18, gender: 'nu', builtIn: false },
  { id: 'giong-nhuoc-vy', name: 'Mẫu giọng Nhược Vy', file: 'giong/giong-nhuoc-vy.wav', durationSec: 22, gender: 'nu', builtIn: false },
  { id: 'giong-ky-duong', name: 'Mẫu giọng Kỳ Dương', file: 'giong/giong-ky-duong.wav', durationSec: 15, gender: 'nam', builtIn: false },
  { id: 'ngoc-lan', name: 'Ngọc Lan', file: 'giong/ngoc-lan.wav', durationSec: 20, gender: 'nu', builtIn: false },
  { id: 'nam-tram', name: 'Nam trầm', file: '', durationSec: 0, gender: 'nam', builtIn: true },
  { id: 'nu-tre-am', name: 'Nữ trẻ, ấm', file: '', durationSec: 0, gender: 'nu', builtIn: true },
  { id: 'nu-trung-nien', name: 'Nữ trung niên', file: '', durationSec: 0, gender: 'nu', builtIn: true },
  { id: 'nam-tre-sang', name: 'Nam trẻ, sáng', durationSec: 0, gender: 'nam', builtIn: true },
  { id: 'giong-ke-lanh', name: 'Giọng kể lạnh', durationSec: 0, gender: 'nu', builtIn: true },
  { id: 'theo-gioi-tinh', name: 'Theo giới tính', file: '', durationSec: 0, gender: 'khac', builtIn: true },
]

/** Giọng có sẵn của bộ đọc (A1Voice /api/tts/voices) — dùng chung cho Cài đặt › Giọng đọc. */
export const BUILTIN_VOICES: AudioVoice[] = VOICES.filter((voice) => voice.builtIn)

/** Mẫu giọng của dự án (tai-san/giong) — mock trả cho mọi dự án. */
export const PROJECT_VOICES: AudioVoice[] = VOICES.filter((voice) => !voice.builtIn)

/** Từ điển cách đọc mặc định của dự án. */
export const DEFAULT_PRONUNCIATIONS = [
  { written: 'live', spoken: 'lai' },
  { written: 'show', spoken: 'sô' },
  { written: 'livestream', spoken: 'lai xtrim' },
  { written: 'Tinh Mang', spoken: 'Tinh Mang' },
  { written: 'MC', spoken: 'em-xi' },
]

const SPEAKERS: AudioCast['speakers'] = [
  { id: 'dan', name: 'Dẫn truyện', ini: 'TH', role: 'Ngôi thứ nhất — giọng Lâm Tri Hạ', voiceId: 'giong-tri-ha', sameAs: 'th' },
  { id: 'th', name: 'Lâm Tri Hạ', role: 'Nữ chính · lời thoại', voiceId: 'giong-tri-ha' },
  { id: 'nv', name: 'Tống Nhược Vy', role: 'Phản diện chính', voiceId: 'giong-nhuoc-vy' },
  { id: 'hc', name: 'Hứa Hoài Châu', role: 'Nam chính phụ trợ', voiceId: 'nam-tram' },
  { id: 'dm', name: 'Đỗ Mẫn', role: 'Tổng đạo diễn show', voiceId: 'nu-trung-nien' },
  { id: 'kd', name: 'Kỳ Dương', role: 'Đồng minh hài · xuất hiện từ chương 5', voiceId: null },
  { id: 'phu', name: 'Nhân vật phụ', role: 'Người không có giọng riêng', voiceId: 'theo-gioi-tinh' },
]

const NORMALIZE: AudioNormalize = {
  numbers: true, dates: true, money: true, symbols: true, abbreviations: true, chapterTitles: true, dropBrackets: true,
}

/** Thời lượng từng chương (giây) — dùng cho dòng thời gian và mốc chương. */
const DURATIONS = [1002, 1085, 1032, 1171, 1100, 1068, 1116, 1077]

type ChapterSeed = {
  durationSec?: number
  status: AudioScriptStatus
  totalSegments: number
  audioDone: number
  changedSegments?: number
  scriptChanged?: boolean
  issues?: number
  draftDone?: number
  draftTotal?: number
}

export type MockAudioState = {
  project: string
  story: string
  title: string
  chapterTitles: Record<number, string>
  /** §AF — tên ngắn cho tệp tải về; thiếu thì suy ra từ title (ranh giới từ ≤32). */
  shortTitle?: string
  scenario: string | null
  engineMissing: boolean
  composeTimers: Set<NodeJS.Timeout>
  startedAt: string
  /** §AE — ghép xuất là việc nền: {jobId} ngay, tiến độ qua job.*; retry giữ chapterIndex. */
  pendingExport: PendingAudioExport | null
  /** số lần ghép đã xong trong phiên — lần sau nhanh hơn (cache ffmpeg ấm). */
  exportsDone: number
  exportOptions: MockAudioExportOptions | null
  exportTimer: NodeJS.Timeout | null
  volume: number
  chapters: Record<number, ChapterSeed>
  scripts: Map<number, AudioSegment[]>
  cast: AudioCast
  settings: AudioSettings
  jobs: Map<number, AudioQueueJob>
  exported: (AudioExportState & { options?: MockAudioExportOptions }) | null
  exportArtifacts: Map<string, { durationSec: number; text?: string }>
  flags: AudioFlag[]
  failureUsed: boolean
  voices: AudioVoice[]
  pronunciations: { written: string; spoken: string }[]
  /** phát sự kiện SSE của truyện (server truyền vào) */
  emit: (event: string, data: unknown) => void
  storyEmit: (event: string, data: unknown) => void
  /** client SSE của riêng S/audio/events */
  clients: Set<{ res: { write(chunk: string): unknown } }>
  timer: NodeJS.Timeout | null
  locks: Map<number, string>
}

const FILLER = [
  'Tôi đặt tập hồ sơ xuống bàn, ngón tay vẫn giữ mép giấy.',
  'Ngoài hành lang, tiếng giày ai đó dừng lại rồi đi tiếp.',
  '“Mình cứ làm đúng luật,” Đỗ Mẫn nói, mắt không rời màn hình.',
  'Màn hình lớn hiện dòng chữ nhỏ ở góc phải: 73 giây.',
  'Nhược Vy cười, nụ cười rất đẹp và rất khó đoán.',
  'Tôi nhớ lại buổi chiều hôm ấy, khi mọi thứ còn chưa bắt đầu.',
  'Hứa Hoài Châu đẩy tập giấy về phía tôi, không nói gì.',
  'Chiếc máy quay thứ hai vẫn đỏ đèn, nhưng không ai nhìn nó.',
  'Tôi viết tên mình lên tờ khai, chậm và rõ từng nét.',
  '“Cậu chắc chứ?” ai đó hỏi, và tôi gật đầu.',
  'Bên ngoài, trời đã tối từ lúc nào.',
  'Kỳ Dương gõ hai ngón tay lên mặt bàn, đợi.',
]
const FILLER_ORDER = ['th', 'dan', 'nv', 'th', 'hc', 'th', 'dan', 'dm', 'th', 'nv']

/** Bản đọc đầy đủ: 14 đoạn đầu theo đúng artboard, phần còn lại lấp đủ tổng số đoạn. */
const fullScript = (chapter: number, total: number): AudioSegment[] => {
  const segments = baseSegments()
  for (let index = segments.length; index < total; index++) {
    const speaker = FILLER_ORDER[(index - segments.length) % FILLER_ORDER.length]
    const text = FILLER[(index - segments.length) % FILLER.length]
    segments.push({
      id: `s${String(index + 1).padStart(4, '0')}`,
      speaker,
      text,
      source: text,
      pauseMs: 300 + (index % 4) * 100,
      tone: index % 9 === 0 ? 'Chậm, lắng' : '',
      normalized: [],
      flags: index % 11 === 0 ? ['english'] : [],
      audio: null,
    })
  }
  return segments
}

const scriptOf = (chapter: number, seed: ChapterSeed): AudioSegment[] => {
  const total = seed.draftTotal ?? seed.totalSegments
  const all = fullScript(chapter, total)
  if (seed.status === 'draft') {
    const produced = seed.draftDone ?? 3
    return all.map((segment, index) => {
      if (index >= produced) return { ...segment, text: '…', source: '', pending: true, audio: null }
      return { ...segment, audio: { url: `/api/segments/${chapter}-${segment.id}/audio?volume=1`, state: 'ready', durationSec: 4.1 } }
    })
  }
  return all.map((segment, index) => {
    const done = index < Math.min(all.length, Math.round((seed.audioDone / seed.totalSegments) * all.length))
    const changed = seed.status === 'stale' && (index === 5 || index === 7)
    return {
      ...segment,
      changed,
      audio: done || changed
        ? {
          url: `/api/segments/${chapter}-${segment.id}/audio?volume=1`,
          state: changed ? 'stale' : 'ready',
          durationSec: 4.1,
        }
        : null,
    }
  })
}

const chapterSeed = (status: AudioScriptStatus, extra: Partial<ChapterSeed> = {}): ChapterSeed => ({
  status, totalSegments: 46, audioDone: 0, ...extra,
})

const mixedState = (emit: MockAudioState['emit']): MockAudioState => {
  const chapters: Record<number, ChapterSeed> = {
    1: chapterSeed('locked', { audioDone: 46, issues: 0 }),
    2: chapterSeed('locked', { audioDone: 46, issues: 0 }),
    3: chapterSeed('review', { audioDone: 12, issues: 4 }),
    4: chapterSeed('review', { totalSegments: 51, issues: 1 }),
    5: chapterSeed('draft', { totalSegments: 52, draftDone: 14, draftTotal: 52, issues: 1 }),
    6: chapterSeed('todo'),
    7: chapterSeed('todo'),
    8: chapterSeed('todo'),
  }
  const scripts = new Map<number, AudioSegment[]>()
  for (const [chapter, seed] of Object.entries(chapters)) scripts.set(Number(chapter), scriptOf(Number(chapter), seed))
  return {
    project: '', story: '', title: '', chapterTitles: {}, startedAt: iso(NOW),
    scenario: null, engineMissing: false, composeTimers: new Set(),
    pendingExport: null, exportsDone: 0, exportOptions: null, exportTimer: null,
    exportArtifacts: new Map(),
    volume: 1,
    chapters,
    scripts,
    cast: {
      mode: 'cast',
      single: { voiceId: 'giong-tri-ha', dialogueShift: true, keepTones: true, dialoguePauseMs: 0 },
      speakers: SPEAKERS.map((speaker) => ({ ...speaker })),
    },
    settings: { quality: 'standard', autoOnLock: true, normalize: { ...NORMALIZE } },
    jobs: new Map(),
    exported: null,
    flags: [
      { id: 'f1', chapter: 3, segmentId: '3-s0004', atSec: 354, reason: 'Đọc hơi nhanh', speaker: 'Hứa Hoài Châu', durationSec: 7 },
      { id: 'f2', chapter: 3, segmentId: '3-s0007', atSec: 378, reason: 'Ngắt hơi sai chỗ', speaker: 'Dẫn truyện', durationSec: 8 },
      { id: 'f3', chapter: 7, segmentId: '7-s0009', atSec: 662, reason: 'Đọc sai tên “Tinh Mang”', speaker: 'Kỳ Dương', durationSec: 6 },
    ],
    failureUsed: false,
    voices: VOICES.map((voice) => ({ ...voice })),
    pronunciations: DEFAULT_PRONUNCIATIONS.map((entry) => ({ ...entry })),
    emit,
    storyEmit: emit,
    clients: new Set(),
    timer: null,
    locks: new Map([[1, iso(NOW - 3 * 3600000)], [2, iso(NOW - 2 * 3600000)]]),
  }
}

const doneState = (emit: MockAudioState['emit']): MockAudioState => {
  const chapters: Record<number, ChapterSeed> = {}
  for (let chapter = 1; chapter <= 8; chapter++) chapters[chapter] = chapterSeed('locked', { audioDone: 46 })
  const scripts = new Map<number, AudioSegment[]>()
  for (const [chapter, seed] of Object.entries(chapters)) scripts.set(Number(chapter), scriptOf(Number(chapter), seed))
  return {
    project: '', story: '', title: '', chapterTitles: {}, startedAt: iso(NOW),
    scenario: null, engineMissing: false, composeTimers: new Set(),
    pendingExport: null, exportsDone: 0, exportOptions: null, exportTimer: null,
    exportArtifacts: new Map(),
    volume: 1,
    chapters,
    scripts,
    cast: {
      mode: 'cast',
      single: { voiceId: 'giong-tri-ha', dialogueShift: true, keepTones: true, dialoguePauseMs: 0 },
      speakers: SPEAKERS.map((speaker) => ({ ...speaker })),
    },
    settings: { quality: 'standard', autoOnLock: true, normalize: { ...NORMALIZE } },
    jobs: new Map(),
    exported: null,
    flags: [
      { id: 'f1', chapter: 3, segmentId: '3-s0004', atSec: 354, reason: 'Đọc hơi nhanh', speaker: 'Hứa Hoài Châu', durationSec: 7 },
      { id: 'f2', chapter: 3, segmentId: '3-s0007', atSec: 378, reason: 'Ngắt hơi sai chỗ', speaker: 'Dẫn truyện', durationSec: 8 },
    ],
    failureUsed: true,
    voices: VOICES.map((voice) => ({ ...voice })),
    pronunciations: DEFAULT_PRONUNCIATIONS.slice(0, 3).map((entry) => ({ ...entry })),
    emit,
    storyEmit: emit,
    clients: new Set(),
    timer: null,
    locks: new Map(Array.from({ length: 8 }, (_, index) => [index + 1, iso(NOW - (8 - index) * 1800000)])),
  }
}

const states = new Map<string, MockAudioState>()
const volumeStateKey = (p: string, s: string, volume: number) => `${p}/${s}${volume === 1 ? '' : `/volume/${volume}`}`

/** Trạng thái audio của một truyện — seed theo id (03 = đã tạo xong, còn lại = đang duyệt). */
export const audioStateOf = (p: string, s: string, storyEmit: MockAudioState['storyEmit'], metadata?: { title: string; shortTitle?: string; chapterTitles: Record<number, string>; volume?: number; totalChapters?: number }): MockAudioState => {
  const volume = metadata?.volume ?? 1
  const key = volumeStateKey(p, s, volume)
  const existing = states.get(key)
  const state = existing ?? (s === '03-tuyen-bo-binh-dang' ? doneState(storyEmit) : mixedState(storyEmit))
  if (!existing && volume > 1) {
    state.volume = volume
    state.chapters = {}
    state.scripts.clear()
    state.locks.clear()
    state.flags = []
    for (let chapter = (volume - 1) * 40 + 1; chapter <= Math.min(volume * 40, metadata?.totalChapters ?? 0); chapter++) {
      state.chapters[chapter] = chapterSeed('todo', { totalSegments: 0 })
    }
  }
  if (volume > 1) {
    const shared = audioStateOf(p, s, storyEmit, { ...metadata!, volume: 1 })
    state.cast = shared.cast
    state.settings = shared.settings
    state.voices = shared.voices
    state.pronunciations = shared.pronunciations
    state.engineMissing = shared.engineMissing
  }
  state.project = p
  state.story = s
  state.title = metadata?.title ?? s.replace(/^\d+-/, '').replaceAll('-', ' ')
  state.shortTitle = metadata?.shortTitle
  if (!state.scenario?.startsWith('mobile')) state.chapterTitles = metadata?.chapterTitles ?? {}
  state.storyEmit = storyEmit
  state.emit = (event, data) => {
    for (const client of state.clients) {
      try { client.res.write(`event: ${event}\ndata: ${JSON.stringify({ type: event, category: 'AUDIO', data })}\n\n`) } catch { /* đóng */ }
    }
    state.storyEmit(event, data)
  }
  if (!existing) {
    states.set(key, state)
    if (volume === 1 && s === '03-tuyen-bo-binh-dang') completeAudioExport(state, defaultExportOptions)
  }
  return state
}

/** Dòng chương cho GET /audio/volumes/:v — trạng thái suy ra từ seed + hàng đợi. */
export const chapterRows = (state: MockAudioState): AudioChapter[] =>
  Object.entries(state.chapters).map(([key, seed]) => {
    const chapter = Number(key)
    const job = state.jobs.get(chapter)
    const rendering = job?.state === 'rendering' || job?.state === 'paused'
    const done = job?.segmentsDone ?? seed.audioDone
    const total = job?.segments ?? seed.totalSegments
    return {
      chapter,
      title: state.chapterTitles[chapter] ?? CHAPTER_TITLES[chapter] ?? `Chương ${chapter}`,
      status: seed.status,
      issues: seed.issues ?? 0,
      durationSec: done > 0 && done >= total ? seed.durationSec ?? DURATIONS[chapter - 1] ?? null : null,
      lockedAt: state.locks.get(chapter) ?? null,
      segments: seed.status === 'todo' ? 0 : seed.draftTotal ?? seed.totalSegments,
      speakers: seed.status === 'todo' ? 0 : 5,
      changedSegments: seed.changedSegments ?? 0,
      chapterChanged: seed.status === 'stale' && !seed.scriptChanged,
      scriptChanged: seed.scriptChanged ?? false,
      staleReason: seed.scriptChanged ? 'Bản đọc đã sửa sau khi tạo. Tạo lại để cập nhật giọng đọc.' : seed.status === 'stale' ? 'Chương này đã được sửa sau khi chốt. Tạo lại để cập nhật bản đọc và giọng đọc.' : '',
      // §AK — ETA chỉ đọc lại đoạn đổi; chỉ có khi bộ đọc đã đo được tốc độ (chưa cài → chưa đo).
      updateEtaSec: (seed.changedSegments ?? 0) > 0 && (seed.status === 'stale' || seed.scriptChanged) && ttsStatus().rtf
        ? Math.max(1, Math.round((seed.changedSegments ?? 0) * 30 * (ttsStatus().rtf ?? 1)))
        : undefined,
      audio: seed.status === 'todo' || done === 0
        ? null
        : {
          segmentsDone: done,
          segments: total,
          durationSec: done >= total ? seed.durationSec ?? DURATIONS[chapter - 1] ?? null : null,
          rendering: Boolean(rendering),
          percent: Math.round((done / total) * 100),
        },
    }
  })

export const volumeOf = (state: MockAudioState): AudioVolume => ({
  chapters: chapterRows(state),
})

export const scriptFor = (state: MockAudioState, chapter: number): (AudioScript & { version: string }) | null => {
  const seed = state.chapters[chapter]
  if (!seed || (seed.status === 'todo' && !state.scripts.has(chapter))) return null
  const status = seed.status === 'locked' || seed.status === 'stale' ? 'locked' : 'review'
  return {
    chapter,
    version: audioVersion(state, `script:${chapter}`),
    volume: state.volume,
    status,
    lockedAt: state.locks.get(chapter) ?? null,
    sourceHash: `hash-${chapter}-${seed.status}`,
    segments: state.scripts.get(chapter) ?? [],
    chapterChanged: seed.status === 'stale' && !seed.scriptChanged,
    scriptChanged: seed.scriptChanged ?? false,
    staleReason: seed.scriptChanged ? 'Bản đọc đã sửa sau khi tạo. Tạo lại để cập nhật giọng đọc.' : seed.status === 'stale' ? 'Chương này đã được sửa sau khi chốt. Tạo lại để cập nhật bản đọc và giọng đọc.' : '',
  }
}

/** §AK — thứ tự 1…n trong các job `queued` (1 = “Tiếp theo”), ghi thẳng lên job như server. */
export const queueOf = (state: MockAudioState): AudioQueueJob[] => {
  let position = 0
  for (const job of state.jobs.values()) {
    if (job.state === 'queued') job.position = ++position
    else delete job.position
  }
  return [...state.jobs.values()]
}

/** Studio timeline data mirrors 06-data-contract §V (durations from the artboard fixture). */
export const queueTimelineOf = (state: MockAudioState): AudioQueue => {
  const rows = chapterRows(state)
  const chapters = rows.map(row => {
    const seed = state.chapters[row.chapter]
    const job = state.jobs.get(row.chapter)
    const estSec = seed.durationSec ?? DURATIONS[row.chapter - 1] ?? 960 + (((row.chapter - 1) * 7919) % 9) * 45 + (((row.chapter - 1) * 31) % 40)
    const complete = seed.audioDone >= seed.totalSegments && seed.status === 'locked'
    return {
      chapter: row.chapter, title: row.title,
      state: complete ? 'done' as const : job?.state==='error' ? 'error' as const : job && (job.state === 'rendering' || (job.state === 'paused' && job.percent > 0)) ? 'rendering' as const : seed.status === 'locked' ? 'queued' as const : 'unlocked' as const,
      sec: complete ? estSec : null, estSec,
      pct: complete ? 100 : job?.percent ?? 0,
      segment: job?.segmentsDone??seed.audioDone,segments:seed.totalSegments,etaSec:job?.etaSec??0,quality:job?.quality??state.settings.quality,reason:job?.error,at:job?.errorAt,
      chapterChanged: row.chapterChanged, scriptChanged: row.scriptChanged, staleReason: row.staleReason,
    }
  })
  return {
    jobs: queueOf(state), chapters,
    qualityEtaSec: (state.scenario?.startsWith('mobile') || state.scenario?.startsWith('gen')) && state.scenario !== 'mobileUnprepared' ? {fast:240,standard:780,high:1500} : undefined,
    madeSec: chapters.reduce((sum, chapter) => sum + (chapter.sec ?? chapter.estSec * ((state.jobs.get(chapter.chapter)?.segmentsDone ?? state.chapters[chapter.chapter].audioDone) / Math.max(1, state.chapters[chapter.chapter].totalSegments))), 0),
    estTotalSec: chapters.reduce((sum, chapter) => sum + (chapter.sec ?? chapter.estSec), 0),
  }
}
export const audioVersionKey = (state: MockAudioState, resource: string) => `audio:${state.project}/${state.story}:${resource}`
export const audioVersion = (state: MockAudioState, resource: string) => mockVersion(audioVersionKey(state, resource))
export const audioChanged = (state: MockAudioState, resource?: string) => {
  if (resource) bumpMockVersion(audioVersionKey(state, resource))
  const change = { project: state.project, story: state.story, parts: ['audio'] }
  state.emit('story.changed', change)
  emitGlobalEvent('story.changed', change)
}

const audioJobId = (state: MockAudioState) => `${state.project}/${state.story}/audio/${state.volume}`
const audioHref = (state: MockAudioState) => `/p/${encodeURIComponent(state.project)}/s/${encodeURIComponent(state.story)}/audio`
const jobBase = (state: MockAudioState) => ({
  project: state.project, story: state.story, href: audioHref(state), startedAt: state.startedAt,
})

/** Dock controls operate on the same queue as /audio/renders, never a second simulation. */
const controlRenders = (state: MockAudioState, action: 'pause' | 'resume' | 'cancel' | 'retry', chapter?: number) => {
  const jobs = queueOf(state).filter((job) => chapter === undefined || job.chapter === chapter)
  const applicable = jobs.filter((job) => action === 'pause'
    ? job.state === 'rendering' || job.state === 'queued'
    : action === 'resume' ? job.state === 'paused'
      : action === 'retry' ? job.state === 'error' : job.state !== 'done')
  if (applicable.length === 0) throw new Error('Không có chương phù hợp để thực hiện thao tác này')
  for (const job of applicable) {
    if (action === 'cancel') {
      if (state.chapters[job.chapter]) state.chapters[job.chapter].audioDone = job.segmentsDone
      state.jobs.delete(job.chapter)
      removeAudioJob(`${audioJobId(state)}/${job.chapter}`)
    }
    else {
      job.state = action === 'pause' ? 'paused' : 'queued'
      delete job.error
      delete job.errorAt
      removeAudioJob(`${audioJobId(state)}/${job.chapter}`)
    }
  }
  if (action === 'resume' || action === 'retry') ensureTimer(state)
  emitQueue(state)
}

const mirrorRenderJobs = (state: MockAudioState) => {
  const jobs = queueOf(state)
  if (jobs.length === 0) {
    removeAudioJob(audioJobId(state))
    return
  }
  const active = jobs.filter((job) => job.state !== 'done')
  const failed = active.filter((job) => job.state === 'error')
  const running = active.some((job) => job.state === 'rendering' || job.state === 'queued')
  const total = jobs.reduce((sum, job) => sum + job.segments, 0)
  const done = jobs.reduce((sum, job) => sum + job.segmentsDone, 0)
  const status: Job['status'] = active.length === 0 ? 'done' : running ? 'running' : active.some((job) => job.state === 'paused') ? 'paused' : 'failed'
  mirrorAudioJob({
    ...jobBase(state), id: audioJobId(state), kind: 'audio', title: `Tạo giọng · Tập ${state.volume}`,
    status, progress: total ? done / total : 0,
    unit: { done, total, label: 'đoạn' }, etaSec: active.reduce((sum, job) => sum + job.etaSec, 0),
    step: running ? `Chương ${active.find((job) => job.state === 'rendering' || job.state === 'queued')?.chapter}` : status === 'done' ? 'Đã tạo xong giọng' : status === 'paused' ? 'Đã tạm dừng' : 'Cần thử lại chương lỗi',
    ...(status === 'failed' ? { error: failed[0]?.error } : {}),
    ...(status === 'done' ? { finishedAt: iso(Date.now()), result: {
      title: 'Đã tạo xong giọng', sub: `${jobs.length} chương`,
      actions: [{ label: 'Nghe & duyệt', kind: 'open' as const, href: audioHref(state), primary: true }],
    } } : {}),
  }, (action) => controlRenders(state, action))
  for (const job of jobs) {
    if (job.state !== 'error') continue
    mirrorAudioJob({
      ...jobBase(state), id: `${audioJobId(state)}/${job.chapter}`, kind: 'audio',
      title: `Tạo giọng · Chương ${job.chapter}`, status: 'failed', progress: job.percent / 100,
      error: job.error, unit: { done: job.segmentsDone, total: job.segments, label: 'đoạn' },
    }, (action) => controlRenders(state, action, job.chapter))
  }
}

export const defaultExportOptions: MockAudioExportOptions = {
  format: 'mp3', chapterMarks: true, subtitles: true, whenReady: false,
  intro: true, chapterTitles: true, loudness: true, gapSec: 2,
}

const exportRows = (state: MockAudioState, chapters?: number[]) =>
  chapterRows(state).filter((row) => chapters === undefined || chapters.includes(row.chapter))

export const readyToExport = (state: MockAudioState, chapters?: number[], requireLocked = false) => {
  const rows = exportRows(state, chapters)
  return rows.length > 0 && rows.every((row) => {
    const segments = state.scripts.get(row.chapter) ?? []
    return (!requireLocked || row.status === 'locked') && segments.length > 0
      && segments.every((segment) => (segment.audio?.durationSec ?? 0) > 0)
  })
}

const exportTiming = (state: MockAudioState, options: MockAudioExportOptions) => {
  let at = options.intro ? 12 : 0
  const marks = options.intro ? [{ atSec: 0, label: 'Mở đầu' }] : []
  const rows = exportRows(state, options.chapters)
  rows.forEach((row, index) => {
    marks.push({ atSec: at, label: `Chương ${row.chapter} · ${row.title}` })
    at += (row.durationSec ?? chapterDuration(row.chapter)) + (options.chapterTitles ? 2 : 0)
    if (index < rows.length - 1) at += options.gapSec ?? 0
  })
  return { marks, durationSec: at }
}

/** Mock export manifest retains every option; WAV fixture playback is served by server.ts. */
const completeAudioExport = (state: MockAudioState, options: MockAudioExportOptions) => {
  options = { ...options, chapterMarks: options.chapterMarks && options.format === 'm4b' && !options.split, ...(options.split ? { intro: false, gapSec: 0 } : {}) }
  const rows = exportRows(state, options.chapters)
  const { marks, durationSec } = exportTiming(state, options)
  const dir = `tap-${String(state.volume).padStart(2, '0')}`
  const ranges: string[] = []
  if (rows.length !== chapterRows(state).length) {
    for (let index = 0; index < rows.length; index++) {
      const from = rows[index].chapter
      let to = from
      while (rows[index + 1]?.chapter === to + 1) to = rows[++index].chapter
      ranges.push(from === to ? String(from) : `${from}-${to}`)
    }
  }
  // §AF/A7 — tên tệp: "{Tên ngắn} · Tập N[ · Chương NN · {tên chương}]( (bản thử)).ext";
  // hậu tố -chN của xuất một phần vẫn nối sau tập; keep-suffix trước nhãn bản thử.
  const short = shortStoryTitle(state.title, state.shortTitle)
  const suffix = options.trial ? '' : ranges.length > 0 ? `-ch${ranges.join('_')}` : ''
  const trialTag = options.trial ? ' (bản thử)' : ''
  const stem = options.trial
    ? audioChapterFileName(short, state.volume, rows[0].chapter, rows[0].title, '', true).replace(/\.$/, '').replace(/ \(bản thử\)$/, '')
    : audioVolumeFileName(short, state.volume, '').replace(/\.$/, '')
  let keepSuffix = ''
  if (options.onExists && options.onExists !== 'replace' && state.exportArtifacts.has(`${dir}/${stem}${trialTag}.${options.format}`)) {
    if (options.onExists === 'fail') throw new MockAudioExportConflict(`Tệp ${stem}${trialTag}.${options.format} đã có. Chọn giữ cả hai hoặc thay thế.`)
    const now = new Date()
    const pad = (value: number) => String(value).padStart(2, '0')
    keepSuffix = `-${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
    if (state.exportArtifacts.has(`${dir}/${stem}${keepSuffix}${trialTag}.${options.format}`)) {
      throw new MockAudioExportConflict(`Tệp ${stem}${keepSuffix}${trialTag}.${options.format} đã có. Hãy thử lại sau một giây.`)
    }
  }
  const companion = `Tập ${state.volume}${suffix}${keepSuffix}`
  const updatedAt = iso(Date.now())
  const file = (name: string, size: number, format: ExportFile['format'] = 'audio', folder = ''): ExportFile => ({
    path: `${dir}/${folder}${name}`, name, size, format, updatedAt, volume: state.volume,
  })
  const bytesPerSec = options.format === 'wav' ? 48000 : 16000
  const chapterFormat = options.split ? (options.format ?? 'mp3') : 'mp3'
  const chapterBytesPerSec = chapterFormat === 'wav' ? 48000 : 16000
  const chapterFiles = rows.map((row) => ({
    ...file(audioChapterFileName(short, state.volume, row.chapter, row.title, chapterFormat), Math.round((row.durationSec ?? 0) * chapterBytesPerSec), 'audio', `chuong${suffix}${keepSuffix}/`),
    path: `${dir}/chuong${suffix}${keepSuffix}/tap-${state.volume}-chuong-${String(row.chapter).padStart(2, '0')}.${chapterFormat}`,
    chapter: row.chapter, durationSec: (row.durationSec ?? 0) + (options.chapterTitles ? 2 : 0),
  }))
  const files: ExportFile[] = options.split ? [...chapterFiles] : [
    file(`${stem}${keepSuffix}${trialTag}.${options.format}`, Math.round(durationSec * bytesPerSec)),
    ...chapterFiles,
    file(`${companion}.ban-doc.txt`, Buffer.byteLength(rows.map((row) => (state.scripts.get(row.chapter) ?? []).map((segment) => segment.text).join('\n')).join('\n\n')), 'other'),
  ]
  if (options.subtitles) {
    if (options.split) rows.forEach((row) => files.push(file(`tap-${state.volume}-chuong-${String(row.chapter).padStart(2, '0')}.srt`, 1024, 'other', `chuong${suffix}${keepSuffix}/`)))
    else files.push(file(`${companion}.srt`, 86_016, 'other'))
  }
  if (options.chapterMarks) files.push(file(`${companion}.moc-chuong.txt`, Buffer.byteLength(marks.map((mark) => `${mark.atSec}\t${mark.label}`).join('\n')), 'other'))
  const zipFile = options.split && options.pack === 'zip' ? file(audioZipFileName(short, state.volume, rows.length, suffix + keepSuffix), files.reduce((sum, item) => sum + item.size, 0), 'other') : undefined
  if (zipFile) files.push(zipFile)
  state.exportOptions = { ...options }
  const result = {
    volume: state.volume, files, chapterMarks: options.chapterMarks ? marks : [], exportedAt: updatedAt,
    durationSec, voices: state.cast.mode === 'single' ? 1 : state.cast.speakers.length,
    chapterCount: rows.length, dir: `xuat-ban › ${dir}`, totalBytes: files.reduce((sum, item) => sum + item.size, 0),
    options: { ...options },
    chapterFiles, split: options.split === true, zipUrl: zipFile?.path,
  }
  if (!options.trial) state.exported = result
  for (const item of files) {
    const chapter = chapterFiles.find((file) => file.path === item.path)?.chapter ?? null
    const text = item.format === 'other' ? renderAudioExportText(state, item.path) ?? undefined : undefined
    state.exportArtifacts.set(item.path, { durationSec: chapter === null ? durationSec : chapterDuration(chapter, state), text })
  }
  return result
}

/** §AE — phản chiếu việc ghép sang hệ jobs (/api/jobs + SSE job.*): tiến độ thật, lỗi kèm chương. */
const mirrorExportJob = (state: MockAudioState, status: Job['status'], body: {
  progress?: number | null; step?: string; etaSec?: number | null
  unit?: { done: number; total: number; label: string }
  result?: Job['result']; error?: PendingAudioExport['error']
} = {}) => {
  const pending = state.pendingExport
  const options = pending?.options ?? {}
  const total = pending
    ? (options.trial ? 1 : options.split ? (options.chapters?.length ?? 1) : Math.max(1, exportRows(state, options.chapters).length))
    : 0
  const waitingStep = options.chapters ? 'Chờ các chương đã chọn có đủ giọng' : 'Chờ tất cả chương có đủ giọng'
  mirrorAudioJob({
    ...jobBase(state), id: `${audioJobId(state)}/export`, kind: 'export',
    title: options.trial ? `Xuất thử chương ${options.chapters?.[0] ?? 1}` : options.split ? `Xuất ${options.chapters?.length ?? 1} chương` : `Ghép Tập ${state.volume}`,
    status,
    progress: body.progress ?? (status === 'done' ? 1 : status === 'waiting' || status === 'paused' ? null : pending && total > 0 ? pending.chapterIndex / total : 0),
    step: body.step ?? (status === 'waiting' ? waitingStep : status === 'paused' ? 'Đã tạm dừng' : status === 'done' ? 'Đã xuất xong' : status === 'failed' ? 'Không ghép được' : 'Đang ghép'),
    etaSec: body.etaSec ?? undefined,
    unit: body.unit ?? (total > 0 && status === 'running' ? { done: pending!.chapterIndex, total, label: 'Chương' } : undefined),
    ...(status === 'failed' ? { error: body.error ?? pending?.error, finishedAt: iso(Date.now()) } : {}),
    ...(status === 'done' ? { finishedAt: iso(Date.now()), result: body.result ?? (pending?.result ? exportJobResult(state, pending) : undefined) } : {}),
  }, async (action, ctlBody) => {
    const current = state.pendingExport
    if (!current) throw new Error('Việc xuất không còn chờ thực hiện')
    if (action === 'retry') {
      if (!current.error) throw new Error('Chỉ thử lại được việc xuất bị lỗi')
      current.error = undefined
      if (ctlBody?.intro === false) current.options = { ...current.options, intro: false }
      current.paused = false
      mirrorExportJob(state, 'running', { step: `Ghép lại từ chương ${Math.max(1, current.chapterIndex + 1)}` })
      maybeExport(state)
      audioChanged(state)
      return
    }
    if (state.exportTimer) { clearTimeout(state.exportTimer); state.exportTimer = null }
    if (action === 'cancel') state.pendingExport = null
    else {
      current.paused = action === 'pause'
      mirrorExportJob(state, current.paused ? 'paused' : 'waiting')
      maybeExport(state)
    }
    audioChanged(state)
  })
}

/** Kết quả §AE của việc ghép: Tải xuống + Xem bản xuất; data.files kèm url/sizeBytes để UI hiển thị. */
const exportJobResult = (state: MockAudioState, pending: PendingAudioExport): NonNullable<Job['result']> => {
  const result = pending.result!
  const base = `/api/projects/${encodeURIComponent(state.project)}/stories/${encodeURIComponent(state.story)}`
  const files = result.files.map((file) => ({
    name: file.name, path: file.path,
    url: `${base}/exports/file?path=${encodeURIComponent(file.path)}`,
    sizeBytes: Math.max(1, Math.round((file.durationSec ?? 0) * 1.15)),
    durationSec: file.durationSec,
    ...(file.chapter !== undefined ? { chapter: file.chapter } : {}),
  }))
  const zip = result.zipUrl
  const zipHref = zip ? (zip.startsWith('http') || zip.includes('?path=') ? zip : `${base}/exports/file?path=${encodeURIComponent(zip)}`) : undefined
  return {
    title: `Đã ${pending.options.split ? 'xuất' : 'ghép'} xong`,
    sub: zip ? zip.split('/').pop() ?? 'gói.zip' : `${files.length} tệp`,
    actions: [
      { kind: 'download', label: 'Tải xuống', primary: true, href: zipHref ?? files[0]?.url },
      { kind: 'open', label: 'Xem bản xuất', href: `/p/${encodeURIComponent(state.project)}/s/${encodeURIComponent(state.story)}/audio/${state.volume}?xem=da-xuat` },
    ],
    files,
    ...(zipHref ? { zipUrl: zipHref } : {}),
    data: { files, ...(zipHref ? { zipUrl: zipHref } : {}) },
  }
}

/** Nhịp ghép của mock: lần đầu chậm hơn (cache ffmpeg nguội), từ lần sau nhanh. */
const exportTickMs = (state: MockAudioState) => (state.exportsDone > 0 ? 260 : 560)
const maybeExport = (state: MockAudioState) => {
  const pending = state.pendingExport
  if (!pending || pending.paused || pending.error || state.exportTimer || !readyToExport(state, pending.options.chapters, pending.options.whenReady)) return
  state.exportTimer = setTimeout(() => {
    state.exportTimer = null
    if (state.pendingExport !== pending || pending.paused || pending.error || !readyToExport(state, pending.options.chapters, pending.options.whenReady)) {
      if (state.pendingExport === pending) mirrorExportJob(state, pending.paused ? 'paused' : 'waiting')
      return
    }
    const total = pending.options.trial ? 1 : pending.options.split ? (pending.options.chapters?.length ?? 1) : Math.max(1, exportRows(state, pending.options.chapters).length)
    try {
      // Mô phỏng lỗi thiếu nhạc mở đầu: dừng ở chương đang ghép, retry chạy tiếp từ đó.
      if (pending.options.demoFail === 'intro' && pending.options.intro !== false) {
        pending.options = { ...pending.options, demoFail: undefined }
        pending.error = { message: 'Nhạc mở đầu đã bị xoá khỏi Tài sản', chapter: Math.max(1, pending.chapterIndex + 1), retryable: true, cause: 'intro_missing' }
        mirrorExportJob(state, 'failed')
        audioChanged(state)
        return
      }
      pending.chapterIndex += 1
      if (pending.chapterIndex >= total) {
        pending.result = completeAudioExport(state, pending.options)
        state.exportsDone += 1
        mirrorExportJob(state, 'done')
        state.pendingExport = null
        audioChanged(state)
        return
      }
      mirrorExportJob(state, 'running', {
        progress: pending.chapterIndex / total,
        step: `Đang ghép chương ${pending.chapterIndex + 1}/${total}`,
        etaSec: Math.max(1, Math.round(((total - pending.chapterIndex) * exportTickMs(state)) / 1000)),
        unit: { done: pending.chapterIndex, total, label: 'Chương' },
      })
      maybeExport(state)
    } catch (error) {
      pending.error = { message: error instanceof Error ? error.message : 'Không ghép được audio', chapter: Math.max(1, pending.chapterIndex + 1), retryable: true }
      mirrorExportJob(state, 'failed')
      audioChanged(state)
    }
  }, exportTickMs(state))
}

/**
 * POST …/audio/volumes/:v/exports (§AE): luôn trả {jobId}; 409 khi cùng tập đang ghép.
 * Chưa đủ giọng → job 'waiting' tự ghép khi đủ. Tiến độ qua SSE job.*.
 */
export const requestAudioExport = (state: MockAudioState, options: MockAudioExportOptions): { jobId: string } => {
  const jobId = `${state.project}/${state.story}/audio/${state.volume}/export`
  const active = state.pendingExport
  if (active && !active.error) throw new MockAudioExportConflict('Tập này đang có một việc ghép đang chạy.', jobId)
  if (state.exportTimer) { clearTimeout(state.exportTimer); state.exportTimer = null }
  state.exportOptions = { ...options }
  state.pendingExport = {
    jobId, options: { ...options }, paused: false,
    chapterIndex: active?.error ? active.chapterIndex : 0,
  }
  if (!readyToExport(state, options.chapters, options.whenReady)) {
    mirrorExportJob(state, 'waiting')
    audioChanged(state)
    return { jobId }
  }
  const total = options.trial ? 1 : options.split ? (options.chapters?.length ?? 1) : Math.max(1, exportRows(state, options.chapters).length)
  mirrorExportJob(state, 'running', { progress: 0, step: `Đang ghép chương 1/${total}`, unit: { done: 0, total, label: 'Chương' } })
  maybeExport(state)
  return { jobId }
}

const renderAudioExportText = (state: MockAudioState, path: string): string | null => {
  if (!state.exported?.files.some((file) => file.path === path && file.format === 'other') || path.endsWith('.zip')) return null
  const options = state.exportOptions ?? defaultExportOptions
  const { marks } = exportTiming(state, options)
  if (path.endsWith('.moc-chuong.txt')) return marks.map((mark) => `${mark.atSec}\t${mark.label}`).join('\n')
  const chapter = options.split ? Number(path.match(/chuong-(\d+)\.srt$/)?.[1]) : null
  const rows = exportRows(state, chapter ? [chapter] : options.chapters)
  if (path.endsWith('.srt')) {
    const stamp = (seconds: number) => {
      const ms = Math.round(seconds * 1000)
      return `${String(Math.floor(ms / 3600000)).padStart(2, '0')}:${String(Math.floor(ms / 60000) % 60).padStart(2, '0')}:${String(Math.floor(ms / 1000) % 60).padStart(2, '0')},${String(ms % 1000).padStart(3, '0')}`
    }
    let cue = 0
    return rows.flatMap((row, index) => {
      const segments = state.scripts.get(row.chapter) ?? []
      const start = (options.split ? 0 : marks[index + (options.intro ? 1 : 0)].atSec) + (options.chapterTitles ? 2 : 0)
      const duration = (row.durationSec ?? chapterDuration(row.chapter)) / Math.max(1, segments.length)
      return segments.map((segment, i) => `${++cue}\n${stamp(start + i * duration)} --> ${stamp(start + (i + 1) * duration)}\n${segment.text}\n`)
    }).join('\n')
  }
  return rows.map((row) => `${row.title}\n\n${(state.scripts.get(row.chapter) ?? []).map((segment) => segment.text).join('\n')}`).join('\n\n')
}

export const audioExportText = (state: MockAudioState, path: string): string | null =>
  state.exportArtifacts.get(path)?.text ?? renderAudioExportText(state, path)

const emitQueue = (state: MockAudioState) => {
  state.emit('audio.queue', { jobs: queueOf(state) })
  mirrorRenderJobs(state)
  audioChanged(state)
  maybeExport(state)
}

/** Tiến hàng đợi: mỗi 420ms một bước; chương 4 lỗi một lần ở đoạn 23 (trạng thái renderError). */
const tick = (state: MockAudioState) => {
  const jobs = [...state.jobs.values()]
  if (jobs.length === 0) return
  let running = jobs.find((job) => job.state === 'rendering')
  if (!running) {
    const next = jobs.find((job) => job.state === 'queued')
    if (!next) return
    next.state = 'rendering'
    emitQueue(state)
    running = next
  }
  if (running.state !== 'rendering') return
  if (running.needsScript) {
    const seed = state.chapters[running.chapter]
    if (seed.status === 'todo') {
      seed.status = 'draft'
      running.step = 'compose'
      emitQueue(state)
      return
    }
    seed.totalSegments = 46
    seed.status = 'review'
    state.scripts.set(running.chapter, scriptOf(running.chapter, seed))
    lockChapter(state, running.chapter, false)
    running.needsScript = false
    running.step = 'render'
    running.segments = seed.totalSegments
    emitQueue(state)
    return
  }
  if (!state.failureUsed && running.chapter === 4 && running.segmentsDone >= 23) {
    state.failureUsed = true
    running.state = 'error'
    running.error = 'Máy hết bộ nhớ đồ hoạ khi đọc đoạn này'
    running.errorAt = iso(Date.now())
    state.emit('audio.render', {
      jobId: running.jobId, chapter: running.chapter, segment: running.segmentsDone,
      segments: running.segments, speaker: running.speaker, percent: running.percent,
      etaSec: running.etaSec, state: 'error',
    })
    emitQueue(state)
    return
  }
  const perStep = 0.4
  const segmentPercent = Math.min(1, (running.segmentPercent ?? 0) + perStep)
  if (segmentPercent >= 1) {
    running.segmentsDone = Math.min(running.segments, running.segmentsDone + 1)
    running.segmentPercent = 0
    if (running.segmentsDone >= running.segments) {
      running.state = 'done'
      running.percent = 100
      running.etaSec = 0
      const seed = state.chapters[running.chapter]
      if (seed) {
        seed.status = 'locked'
        if (running.kind === 'update') {
          // §AK — chỉ đọc lại đoạn đổi: chương vẫn đủ đoạn như trước, không co lại.
          seed.audioDone = seed.totalSegments
        } else {
          seed.audioDone = running.segments
          seed.totalSegments = running.segments
        }
        seed.scriptChanged = false
        seed.changedSegments = 0
        state.locks.set(running.chapter, state.locks.get(running.chapter) ?? iso(Date.now()))
        state.scripts.set(running.chapter, (state.scripts.get(running.chapter) ?? []).map((segment) => ({
          ...segment, changed: false,
          audio: { url: `/api/projects/${encodeURIComponent(state.project)}/stories/${encodeURIComponent(state.story)}/audio/segments/${running.chapter}-${segment.id}/audio?volume=${state.volume}`, state: 'ready', durationSec: 4.1 },
        })))
        audioChanged(state, `script:${running.chapter}`)
      }
      state.emit('audio.render', {
        jobId: running.jobId, chapter: running.chapter, segment: running.segments,
        segments: running.segments, speaker: running.speaker, percent: 100, etaSec: 0, state: 'done',
      })
      emitQueue(state)
      return
    }
  } else {
    running.segmentPercent = segmentPercent
  }
  running.percent = Math.min(99, Math.round(((running.segmentsDone + (running.segmentPercent ?? 0)) / running.segments) * 100))
  running.etaSec = Math.max(1, Math.round((running.segments - running.segmentsDone) * 0.9))
  state.emit('audio.render', {
    jobId: running.jobId, chapter: running.chapter, segment: running.segmentsDone + 1,
    segmentId: `${running.chapter}-${state.scripts.get(running.chapter)?.[0]?.id ?? 's0001'}`,
    segments: running.segments, speaker: running.speaker, percent: running.percent,
    etaSec: running.etaSec, state: 'rendering',
    text: running.currentText, segmentPercent: running.segmentPercent,
  })
  mirrorRenderJobs(state)
}

const ensureTimer = (state: MockAudioState) => {
  if (state.timer) return
  state.timer = setInterval(() => {
    tick(state)
    const active = [...state.jobs.values()].some((job) => job.state === 'rendering' || job.state === 'queued' || job.state === 'paused')
    if (!active && state.timer) { clearInterval(state.timer); state.timer = null }
  }, 420)
}

export const startRender = (state: MockAudioState, chapter: number, segments?: number, kind?: 'create' | 'update' | 'redo'): AudioQueueJob => {
  const seed = state.chapters[chapter]
  // §AK — update chỉ đọc lại đoạn đổi; redo đọc lại cả chương; create đọc chương chưa có giọng.
  const jobKind = kind ?? ((seed?.changedSegments ?? 0) > 0 || seed?.scriptChanged ? 'update' : 'create')
  const total = segments ?? (jobKind === 'update' ? seed?.changedSegments ?? 0 : seed?.totalSegments ?? 46)
  const existing = state.jobs.get(chapter)
  if (existing && existing.state !== 'done' && existing.state !== 'error') return existing
  if (existing?.state === 'error') removeAudioJob(`${audioJobId(state)}/${chapter}`)
  const speaker = 'Dẫn truyện'
  const doneBefore = jobKind === 'redo' ? 0 : existing?.state === 'error' ? existing.segmentsDone : jobKind === 'update' ? 0 : seed?.audioDone ?? 0
  const job: AudioQueueJob = {
    jobId: `${audioJobId(state)}/${chapter}`,
    chapter,
    kind: jobKind,
    state: 'queued',
    segmentsDone: doneBefore,
    segments: total,
    etaSec: Math.max(1, Math.round(total * 0.9)),
    speaker,
    currentText: 'Đồng hồ trên màn hình lớn bắt đầu đếm ngược từ bảy mươi ba giây.',
    segmentPercent: 0,
    percent: existing?.state === 'error' ? existing.percent : total > 0 ? Math.round((doneBefore / total) * 100) : 0,
  }
  state.jobs.set(chapter, job)
  ensureTimer(state)
  emitQueue(state)
  return job
}

export const setJobState = (state: MockAudioState, jobId: string, next: AudioJobState) => {
  const job = [...state.jobs.values()].find((item) => item.jobId === jobId)
  if (!job) return false
  if (next === 'rendering' || next === 'queued') {
    removeAudioJob(`${audioJobId(state)}/${job.chapter}`)
    delete job.error
    delete job.errorAt
  }
  job.state = next
  if (next === 'rendering' || next === 'queued') ensureTimer(state)
  state.emit('audio.render', {
    jobId: job.jobId, chapter: job.chapter, segment: job.segmentsDone + 1, segments: job.segments,
    speaker: job.speaker, percent: job.percent, etaSec: job.etaSec, state: next,
    segmentPercent: job.segmentPercent,
  })
  emitQueue(state)
  return true
}

export const cancelJob = (state: MockAudioState, jobId: string) => {
  const job = [...state.jobs.values()].find((item) => item.jobId === jobId)
  if (!job) return false
  if (state.chapters[job.chapter]) state.chapters[job.chapter].audioDone = job.segmentsDone
  state.jobs.delete(job.chapter)
  removeAudioJob(`${audioJobId(state)}/${job.chapter}`)
  emitQueue(state)
  return true
}


export const lockChapter = (state: MockAudioState, chapter: number, auto: boolean) => {
  const seed = state.chapters[chapter]
  if (!seed) return
  seed.status = 'locked'
  state.locks.set(chapter, iso(Date.now()))
  audioChanged(state, `script:${chapter}`)
  if (auto && seed.audioDone < seed.totalSegments) startRender(state, chapter, seed.totalSegments)
  maybeExport(state)
}

export const unlockChapter = (state: MockAudioState, chapter: number) => {
  const seed = state.chapters[chapter]
  if (!seed) return
  seed.status = 'review'
  state.locks.delete(chapter)
  audioChanged(state, `script:${chapter}`)
}

export const markStale = (state: MockAudioState, chapter: number, changed: number) => {
  const seed = state.chapters[chapter]
  if (!seed) return
  seed.scriptChanged = true
  seed.changedSegments = changed
  audioChanged(state, `script:${chapter}`)
}

export const refreshChapter = (state: MockAudioState, chapter: number) => {
  const seed = state.chapters[chapter]
  if (!seed) return
  seed.status = 'locked'
  seed.changedSegments = 0
  state.scripts.set(chapter, (state.scripts.get(chapter) ?? []).map((segment) => ({ ...segment, changed: false })))
  audioChanged(state, `script:${chapter}`)
  maybeExport(state)
}

export const composeChapters = (state: MockAudioState, chapters: number[]) => {
  for (const chapter of chapters) {
    const seed = state.chapters[chapter]
    if (!seed) continue
    seed.status = 'draft'
    seed.draftDone = 0
    seed.draftTotal = seed.totalSegments
    state.scripts.set(chapter, scriptOf(chapter, seed))
    audioChanged(state, `script:${chapter}`)
  }
  let index = 0
  const timer = setInterval(() => {
    const chapter = chapters[index]
    const seed = state.chapters[chapter]
    if (!seed) { clearInterval(timer); state.composeTimers.delete(timer); return }
    seed.draftDone = Math.min(seed.draftTotal ?? seed.totalSegments, (seed.draftDone ?? 0) + 6)
    state.scripts.set(chapter, scriptOf(chapter, seed))
    audioChanged(state, `script:${chapter}`)
    state.emit('audio.compose', {
      chapter, state: 'drafting', segments: seed.totalSegments,
      percent: Math.round(((seed.draftDone ?? 0) / (seed.draftTotal ?? seed.totalSegments)) * 100),
    })
    if ((seed.draftDone ?? 0) >= (seed.draftTotal ?? seed.totalSegments)) {
      seed.status = 'review'
      seed.draftDone = undefined
      state.scripts.set(chapter, scriptOf(chapter, seed))
      state.emit('audio.compose', { chapter, state: 'done', segments: seed.totalSegments, percent: 100 })
      audioChanged(state, `script:${chapter}`)
      index++
      if (index >= chapters.length) { clearInterval(timer); state.composeTimers.delete(timer) }
    }
  }, 900)
  state.composeTimers.add(timer)
}

/** POST audio/demo {scenario,live?:boolean}; no disk or real TTS is touched.
 * Desktop: review|locked|drafting|stale|allDone|confirm|rendering|paused|
 * renderError|background|rendered|listen|exported|noEngine|ffmpegMissing.
 * Mobile: mobile|mobileReady|mobileNoEngine|mobileExported.
 * Mobile §AT: mobileMachineNone|mobileMachineOffline|mobileMachinePrep|mobileMachineFail (thẻ máy đọc),
 * mobileWait|mobileWaitPrep (MVoiceGen chờ máy đọc).
 * almostDone leaves one segment for deferred export.
 * Seeds are frozen by default; live:true starts the normal renderer/composer.
 */
export const activateAudioScenario = (state: MockAudioState, scenario: string, live = false, len?: number, doneChapters?: number) => {
  const scenarios = ['mixed', 'review', 'locked', 'drafting', 'stale', 'allDone', 'confirm', 'rendering', 'paused', 'renderError', 'background', 'rendered', 'listen', 'exported', 'almostDone', 'noEngine', 'ffmpegMissing', 'mobile', 'mobileReady', 'mobileNoEngine', 'mobileExported']
  scenarios.push('mobileQueue','mobileQueuePaused','mobileQueueError','mobilePick','mobilePickRedo','mobileUnprepared')
  scenarios.push('genStale','genReady','genRunning','genError','genAllDone')
  scenarios.push('mobileMachineNone','mobileMachineOffline','mobileMachinePrep','mobileMachineFail','mobileWait','mobileWaitPrep')
  if (!scenarios.includes(scenario)) return false
  clearInterval(state.timer ?? undefined)
  clearTimeout(state.exportTimer ?? undefined)
  for (const timer of state.composeTimers) clearInterval(timer)
  for (const chapter of Object.keys(state.chapters)) removeAudioJob(`${audioJobId(state)}/${chapter}`)
  removeAudioJob(audioJobId(state))
  removeAudioJob(`${audioJobId(state)}/export`)
  const done = ['allDone', 'confirm', 'rendered', 'listen', 'exported', 'almostDone'].includes(scenario)
  const seed = done ? doneState(state.emit) : mixedState(state.emit)
  state.scenario = scenario
  state.engineMissing = scenario === 'mobileNoEngine'
  state.chapters = seed.chapters
  state.scripts = seed.scripts
  state.cast = seed.cast
  state.settings = seed.settings
  state.jobs = seed.jobs
  state.exported = null
  state.exportOptions = null
  state.pendingExport = null
  state.exportTimer = null
  state.timer = null
  state.composeTimers.clear()
  state.flags = seed.flags
  state.failureUsed = seed.failureUsed
  state.locks = seed.locks
  state.startedAt = iso(Date.now())
  if (scenario === 'stale') {
    state.chapters[3] = chapterSeed('stale', { audioDone: 46, changedSegments: 2 })
    state.scripts.set(3, scriptOf(3, state.chapters[3]))
  }
  if (scenario.startsWith('gen')) {
    // MVoiceGen.dc.html — 8 chương: 1 · 4 đã sửa, 7 chưa chốt, 8 chưa soạn bản đọc.
    const titles = ['Vai của tôi là đồ canh', 'Chiếc cặp ở giữa bàn', 'Hai chiếc bát trên bàn', 'Bảy mươi ba giây', 'Tôi cướp vai cái nồi à?', 'Chiếc vỏ ổ cứng', 'Dòng tên bị gạch', 'Tên tôi trên màn hình']
    const durations = [1002, 1085, 1032, 1170, 1100, 1068, 1116, 1077]
    const delayed = scenario !== 'genStale' && scenario !== 'genAllDone'
    const allDoneSeed = scenario === 'genAllDone'
    state.chapterTitles = Object.fromEntries(titles.map((title, index) => [index + 1, title]))
    state.chapters = {}
    state.scripts.clear()
    state.locks.clear()
    state.failureUsed = true
    titles.forEach((_, index) => {
      const chapter = index + 1
      const status: AudioScriptStatus = chapter === 8 && delayed ? 'todo' : chapter === 7 && delayed ? 'review' : (chapter === 1 || chapter === 4) && !allDoneSeed ? 'stale' : 'locked'
      state.chapters[chapter] = chapterSeed(status, {
        durationSec: status === 'todo' ? undefined : durations[index],
        audioDone: status === 'todo' || status === 'review' ? 0 : 46,
        totalSegments: status === 'todo' ? 0 : 46,
        issues: chapter === 7 && delayed ? 2 : 0,
        changedSegments: allDoneSeed ? 0 : chapter === 1 ? 3 : chapter === 4 ? 2 : 0,
        scriptChanged: !allDoneSeed && chapter === 4,
        draftDone: chapter === 8 && delayed ? 3 : undefined,
        draftTotal: chapter === 8 && delayed ? 46 : undefined,
      })
      if (status !== 'todo') {
        state.locks.set(chapter, iso(NOW - chapter * 60000))
        state.scripts.set(chapter, scriptOf(chapter, state.chapters[chapter]))
      }
    })
    if (scenario === 'genRunning' || scenario === 'genError') {
      const errors = scenario === 'genError'
      if (errors) {
        // ch1 đã cập nhật xong; ch4 dừng do lỗi; ch7 đang tạo; ch8 chờ soạn bản đọc.
        state.chapters[1].changedSegments = 0
        state.chapters[1].status = 'locked'
        const four = startRender(state, 4, 2, 'update')
        four.state = 'error'; four.segmentsDone = 1; four.percent = 50; four.error = 'Dừng ở đoạn 1/2 · bộ đọc không phản hồi'; four.errorAt = iso(NOW)
        const seven = startRender(state, 7, 46, 'create')
        seven.state = 'rendering'; seven.segmentsDone = 6; seven.percent = 12; seven.etaSec = 660; seven.segmentPercent = .26
      } else {
        const one = startRender(state, 1, 3, 'update')
        one.state = 'rendering'; one.segmentsDone = 2; one.percent = 60; one.etaSec = 60; one.segmentPercent = .2
        const four = startRender(state, 4, 2, 'update')
        four.state = 'queued'
        const seven = startRender(state, 7, 46, 'create')
        seven.state = 'queued'
      }
      const eight = startRender(state, 8, 0, 'create')
      eight.state = 'queued'; eight.needsScript = true; eight.step = 'compose'; eight.etaSec = 0
      emitQueue(state)
    }
    for (const chapter of Object.keys(state.chapters)) audioChanged(state, `script:${chapter}`)
    audioChanged(state, 'cast')
    audioChanged(state, 'settings')
    emitQueue(state)
    // Seed đóng băng: chỉ `live:true` mới cho tiến trình chạy tiếp.
    if (!live) {
      if (state.timer) { clearInterval(state.timer); state.timer = null }
      for (const timer of state.composeTimers) clearInterval(timer)
      state.composeTimers.clear()
    }
    return true
  }
  if (scenario.startsWith('mobile')) {
    const titles = [
      'Vai của tôi là đồ canh', 'Chiếc cặp ở giữa bàn', 'Hai chiếc bát trên bàn', 'Bảy mươi ba giây',
      'Tôi cướp vai cái nồi à?', 'Chiếc vỏ ổ cứng', 'Dòng tên bị gạch', 'Tên tôi trên màn hình',
      'Máy quay số ba', 'Cốc trà của đạo diễn', 'Bản gốc', 'Đêm mất điện',
    ]
    const durations = [822, 845, 778, 800, 791, 870, 767, 816, 782, 810, 800, 820]
    const allReady = scenario === 'mobileReady' || scenario === 'mobileExported'
    state.chapterTitles = Object.fromEntries(titles.map((title, index) => [index + 1, title]))
    state.chapters = {}
    state.scripts.clear()
    state.locks.clear()
    state.failureUsed = true
    state.cast.mode = 'cast'
    state.cast.single.voiceId = 'ngoc-lan'
    state.cast.speakers = [
      { id: 'dan', name: 'Dẫn truyện', ini: 'TH', role: 'Dẫn truyện', voiceId: 'giong-tri-ha', sameAs: 'th' },
      { id: 'th', name: 'Lâm Tri Hạ', role: 'Nữ chính', voiceId: 'giong-tri-ha' },
      { id: 'nv', name: 'Tống Nhược Vy', role: 'Phản diện chính', voiceId: 'giong-nhuoc-vy' },
      { id: 'hc', name: 'Hứa Hoài Châu', role: 'Nam chính', voiceId: 'nam-tram' },
      { id: 'dm', name: 'Đỗ Mẫn', role: 'Tổng đạo diễn', voiceId: 'nu-trung-nien' },
    ]
    titles.forEach((_, index) => {
      const chapter = index + 1
      state.chapters[chapter] = chapterSeed('locked', { durationSec: durations[index], audioDone: allReady || chapter <= 9 ? 46 : 0 })
      state.locks.set(chapter, iso(NOW - chapter * 60000))
      state.scripts.set(chapter, scriptOf(chapter, state.chapters[chapter]))
    })
    const line = 'Bảy mươi ba giây. Đó là thời gian tôi có, trước khi đèn đỏ trên máy quay bật lên và mọi người nhìn thấy tôi.'
    state.scripts.set(4, [{
      id: 's0001', speaker: 'dan', text: line, source: line, pauseMs: 0, tone: '', normalized: [], flags: [],
      audio: { url: `/api/projects/${encodeURIComponent(state.project)}/stories/${encodeURIComponent(state.story)}/audio/segments/4-s0001/audio?volume=1`, state: 'ready', durationSec: 800 },
    }])
    if (!allReady && !state.engineMissing) {
      for (const chapter of [10, 11, 12]) {
        const job = startRender(state, chapter)
        if (chapter === 10) {
          job.state = 'rendering'
          job.segmentsDone = 20
          job.percent = 44
          job.segmentPercent = 0.24
        }
      }
    }
    // §AT — thẻ Máy đọc: mọi job còn sống chuyển thành xếp hàng chờ máy đọc.
    if (scenario.startsWith('mobileMachine')) {
      for (const job of state.jobs.values()) {
        job.state = 'queued'
        job.waitingMachine = true
        job.percent = 0
        job.segmentsDone = 0
      }
    }
    // §AT — MVoiceGen gk=waiting|waitPrep: ba chương cuối chờ máy đọc, giữ đúng thứ tự.
    if (scenario === 'mobileWait' || scenario === 'mobileWaitPrep') {
      for (const chapter of [10, 11, 12]) {
        const job = state.jobs.get(chapter) ?? startRender(state, chapter)
        job.state = 'queued'
        job.waitingMachine = true
        job.percent = 0
        job.segmentsDone = 0
      }
    }
    if (['mobileQueue','mobileQueuePaused','mobileQueueError','mobilePick','mobilePickRedo'].includes(scenario)) {
      for (const n of [11,12]) {
        state.jobs.delete(n);state.chapters[n].status='review';state.chapters[n].issues=n===11?2:0;state.locks.delete(n)
      }
      const current=state.jobs.get(10)
      if(current){current.segments=51;current.segmentsDone=23;current.percent=46;current.etaSec=300;current.speaker='Dẫn truyện';current.state=scenario==='mobileQueuePaused'?'paused':scenario==='mobileQueueError'?'error':'rendering';if(current.state==='error'){current.percent=44;current.error='Máy chủ hết bộ nhớ đồ hoạ khi đọc một đoạn dài. Phần đã tạo vẫn giữ.';current.errorAt=iso(NOW)}}
    }
    if (scenario === 'mobilePickRedo') state.chapters[4].scriptChanged = true
    if (scenario === 'mobileUnprepared') {
      state.jobs.clear()
      state.chapters = {}
      state.scripts.clear()
      state.locks.clear()
      state.chapterTitles = {}
      for (let chapter = 1; chapter <= 32; chapter++) {
        state.chapterTitles[chapter] = `Chương ${chapter}`
        state.chapters[chapter] = chapterSeed(chapter <= 3 ? 'locked' : 'todo', { totalSegments: chapter <= 3 ? 110 : 0, audioDone: chapter <= 3 ? 110 : 0, durationSec: 432 })
        if (chapter <= 3) {
          state.scripts.set(chapter, scriptOf(chapter, state.chapters[chapter]))
          state.locks.set(chapter, iso(NOW - chapter * 60000))
        }
      }
    }
    if (scenario === 'mobileExported') completeAudioExport(state, { ...defaultExportOptions, format: 'm4b', subtitles: false })
  } else if (['rendering', 'paused', 'renderError', 'background', 'locked', 'almostDone'].includes(scenario)) {
    const chapter = scenario === 'locked' ? 3 : scenario === 'almostDone' ? 8 : 4
    if (chapter === 4) {
      for (let number = 1; number <= 6; number++) {
        state.chapters[number] = chapterSeed('locked', { totalSegments: number === 4 ? 51 : 46, audioDone: number <= 3 ? 46 : 0 })
        state.locks.set(number, iso(NOW - number * 60000))
        state.scripts.set(number, scriptOf(number, state.chapters[number]))
      }
    } else state.chapters[chapter].status = 'locked'
    const job = startRender(state, chapter)
    job.segmentsDone = scenario === 'almostDone' ? job.segments - 1 : scenario === 'locked' ? 17 : 23
    if (scenario === 'almostDone') {
      const segments = state.scripts.get(chapter) ?? []
      state.scripts.set(chapter, segments.map((segment, index) => index === segments.length - 1 ? { ...segment, audio: null } : segment))
    }
    job.percent = Math.round(job.segmentsDone / job.segments * 100)
    job.state = scenario === 'paused' ? 'paused' : scenario === 'renderError' ? 'error' : 'rendering'
    if (scenario === 'renderError') {
      state.failureUsed = true
      job.error = 'Máy hết bộ nhớ đồ hoạ khi đọc đoạn này'
      job.errorAt = iso(Date.now())
    }
    if (chapter === 4) {
      startRender(state, 5)
      startRender(state, 6)
      if (scenario === 'paused') for (const queued of state.jobs.values()) queued.state = 'paused'
    }
  }
  if (len !== undefined && [8, 16, 40, 120].includes(len)) {
    state.chapters = {}
    state.scripts.clear()
    state.jobs.clear()
    state.locks.clear()
    const finished = done ? len : doneChapters ?? (len === 8 ? 3 : Math.max(1, Math.round(len * .3)))
    const locked = done ? len : Math.ceil(len * .75)
    for (let chapter = 1; chapter <= len; chapter++) {
      const durationSec = DURATIONS[chapter - 1] ?? 960 + (((chapter - 1) * 7919) % 9) * 45 + (((chapter - 1) * 31) % 40)
      state.chapters[chapter] = chapterSeed(chapter <= locked ? 'locked' : 'todo', { durationSec, audioDone: chapter <= finished ? 46 : 0 })
      state.scripts.set(chapter, scriptOf(chapter, state.chapters[chapter]))
      if (chapter <= locked) state.locks.set(chapter, iso(NOW - chapter * 60000))
      if (chapter > finished && chapter <= locked) {
        const job = startRender(state, chapter)
        if (chapter === finished + 1) {
          job.state = 'rendering'
          job.segmentsDone = 20
          job.percent = Math.round(20 / 46 * 100)
        }
      }
    }
    if (scenario === 'paused') for (const job of state.jobs.values()) job.state = 'paused'
  }
  if (scenario === 'exported') completeAudioExport(state, defaultExportOptions)
  // startRender seeds via the same path as user actions; freeze only its timer.
  if (!live && state.timer) { clearInterval(state.timer); state.timer = null }
  if (live && scenario === 'drafting') composeChapters(state, [5])
  for (const chapter of Object.keys(state.chapters)) audioChanged(state, `script:${chapter}`)
  audioChanged(state, 'cast')
  audioChanged(state, 'settings')
  emitQueue(state)
  return true
}

export const addFlag = (state: MockAudioState, input: { chapter: number; segmentId: string; atSec: number; reason: string }) => {
  const flag: AudioFlag = {
    id: `f${state.flags.length + 1}`,
    chapter: input.chapter,
    segmentId: input.segmentId,
    atSec: input.atSec,
    reason: input.reason,
    speaker: state.cast.speakers.find((speaker) => input.segmentId.endsWith(speaker.id))?.name ?? 'Dẫn truyện',
    durationSec: 6,
  }
  state.flags.push(flag)
  return flag
}

/** Sóng âm tính sẵn (≈ 2 giá trị/giây) — cùng công thức với artboard. */
export const peaksFor = (durationSec: number, seed = 3): number[] => {
  const count = Math.max(24, Math.round(durationSec * 2))
  return Array.from({ length: count }, (_, index) => {
    const value = Math.abs(Math.sin(index * 0.37 + seed) * 0.6 + Math.sin(index * 1.7 + seed * 2) * 0.3 + Math.sin(index * 0.08) * 0.25)
    return Math.max(0.08, Math.min(1, value))
  })
}

/** WAV 8 kHz mono — đủ để HTMLAudioElement phát thật và tua bằng Range. */
const wavCache = new Map<string, Buffer>()
export const wavFor = (durationSec: number, rate = 8000): Buffer => {
  const rounded = Math.max(1, Math.round(durationSec))
  const key = `${rounded}@${rate}`
  const cached = wavCache.get(key)
  if (cached) return cached
  const samples = rounded * rate
  const buffer = Buffer.alloc(44 + samples * 2)
  buffer.write('RIFF', 0)
  buffer.writeUInt32LE(36 + samples * 2, 4)
  buffer.write('WAVEfmt ', 8)
  buffer.writeUInt32LE(16, 16)
  buffer.writeUInt16LE(1, 20)
  buffer.writeUInt16LE(1, 22)
  buffer.writeUInt32LE(rate, 24)
  buffer.writeUInt32LE(rate * 2, 28)
  buffer.writeUInt16LE(2, 32)
  buffer.writeUInt16LE(16, 34)
  buffer.write('data', 36)
  buffer.writeUInt32LE(samples * 2, 40)
  for (let index = 0; index < samples; index++) {
    const t = index / rate
    const envelope = 0.35 + 0.25 * Math.sin(t * 0.7)
    const value = Math.sin(2 * Math.PI * 220 * t) * 0.18 * envelope + Math.sin(2 * Math.PI * 330 * t) * 0.08
    buffer.writeInt16LE(Math.round(value * 32767), 44 + index * 2)
  }
  wavCache.set(key, buffer)
  return buffer
}

/** Đọc Range của trình duyệt (HTMLAudioElement) — trả phần cắt + header 206. */
export const rangeOf = (buffer: Buffer, header: string | undefined): { body: Buffer; start: number; end: number; total: number } | null => {
  const total = buffer.length
  if (!header) return { body: buffer, start: 0, end: total - 1, total }
  const match = /^bytes=(\d*)-(\d*)$/.exec(header.trim())
  if (!match) return null
  const start = match[1] ? Number.parseInt(match[1], 10) : 0
  const end = match[2] ? Math.min(Number.parseInt(match[2], 10), total - 1) : total - 1
  if (!Number.isFinite(start) || start > end) return null
  return { body: buffer.subarray(start, end + 1), start, end, total }
}

/** Thời lượng bản ghép cả tập (giây) — dùng cho tệp audio đã xuất trong mock. */
export const audioExportDuration = (p: string, s: string, path?: string) => {
  const volume = Number(path?.match(/tap-(\d+)/)?.[1] ?? 1)
  const state = states.get(volumeStateKey(p, s, volume))
  if (path && state?.exportArtifacts.has(path)) return state.exportArtifacts.get(path)!.durationSec
  if (state && path?.includes('/chuong/')) {
    const chapter = Number.parseInt(path.split('/').pop() ?? '', 10)
    return chapterDuration(chapter, state)
  }
  if (state?.exported) return state.exported.durationSec
  return DURATIONS.reduce((sum, value) => sum + value, 0) + 14
}

/** Thời lượng audio một chương (giây) — dùng cho mp3/peaks/sóng âm của mock. */
export const chapterDuration = (chapter: number, state?: MockAudioState) => state?.chapters[chapter]?.durationSec ?? DURATIONS[chapter - 1] ?? 1000

export const resetMockAudio = () => {
  // Stops all audio-owned timers and removes only this renderer's dock entries.
  for (const state of states.values()) {
    clearInterval(state.timer ?? undefined)
    clearTimeout(state.exportTimer ?? undefined)
    for (const timer of state.composeTimers) clearInterval(timer)
    removeAudioJob(audioJobId(state))
    removeAudioJob(`${audioJobId(state)}/export`)
    for (const chapter of Object.keys(state.chapters)) removeAudioJob(`${audioJobId(state)}/${chapter}`)
  }
  states.clear()
  wavCache.clear()
}

/** Dừng toàn bộ audio của riêng truyện bị xoá, không ảnh hưởng truyện khác. */
export function deleteAudioState(project: string, story: string) {
  for (const [key, state] of states) {
    if (state.project !== project || state.story !== story) continue
    clearInterval(state.timer ?? undefined)
    clearTimeout(state.exportTimer ?? undefined)
    for (const timer of state.composeTimers) clearInterval(timer)
    state.pendingExport = null
    removeAudioJob(audioJobId(state))
    removeAudioJob(`${audioJobId(state)}/export`)
    for (const chapter of Object.keys(state.chapters)) removeAudioJob(`${audioJobId(state)}/${chapter}`)
    for (const client of state.clients) {
      const response = client.res as typeof client.res & { end?: () => void }
      response.end?.()
    }
    state.clients.clear()
    states.delete(key)
  }
}
