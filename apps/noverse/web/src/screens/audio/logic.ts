// Logic thuần của màn Audio — thời gian, sóng âm, dòng chạy theo, cờ & chỗ cần xem,
// phân vai, hàng đợi tạo giọng, mốc chương (10-audio.md §2–§5).
import type {
  AudioCast, AudioCastSpeaker, AudioChapter, AudioFlag, AudioQueueJob, AudioSegment, AudioVoice,
} from '../../api/types'
import { t } from '../../i18n/vi'

// ─── Thời gian ───────────────────────────────────────────────────────────

/** `06:12` (dưới 1 giờ) hoặc `2:24:31` — không làm tròn lên trước tiếng đang nghe. */
export function formatClock(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const rest = total % 60
  const pad = (value: number) => String(value).padStart(2, '0')
  return hours > 0 ? `${hours}:${pad(minutes)}:${pad(rest)}` : `${pad(minutes)}:${pad(rest)}`
}

/** `2 giờ 24 phút 31 giây` · `17 phút` · `42 giây` — câu tiếng Việt, không rút gọn số 0. */
export function formatDuration(seconds: number): string {
  const total = Math.max(0, Math.round(seconds))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const rest = total % 60
  const parts: string[] = []
  if (hours > 0) parts.push(t.audio.logic.gio(hours))
  if (minutes > 0) parts.push(t.audio.logic.phut(minutes))
  if (rest > 0 || parts.length === 0) parts.push(t.audio.logic.giay(rest))
  return parts.join(' ')
}

/** Ước tính luôn có “≈” (10-audio.md §6). */
export const approxDuration = (seconds: number) => `≈ ${formatDuration(seconds)}`

/** Remaining time in the centre of the studio ring. */
export function formatEta(seconds: number): string {
  if (seconds <= 0) return t.audio.logic.sapXong
  if (seconds < 60) return t.audio.logic.conGiay(Math.max(1, Math.round(seconds)))
  const minutes = Math.round(seconds / 60)
  return minutes < 60 ? t.audio.logic.conPh(minutes) : t.audio.logic.conGioPh(Math.floor(minutes / 60), minutes % 60)
}

/** `198 MB` · `1,4 GB` — dung lượng tệp. */
export function formatBytes(bytes: number): string {
  if (bytes >= 1_073_741_824) return `${(bytes / 1_073_741_824).toFixed(1).replace('.', ',')} GB`
  if (bytes >= 1_048_576) return `${Math.round(bytes / 1_048_576)} MB`
  if (bytes >= 1024) return `${Math.round(bytes / 1024)} KB`
  return `${bytes} B`
}

/** `xong lúc 23:18` — giờ địa phương từ RFC3339. */
export function clockOf(iso: string | null): string {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

/** Ước tính thời lượng đọc của một đoạn trước khi tạo (số từ / tốc độ + ngừng). */
export function estimateSegmentSeconds(text: string, pauseMs: number): number {
  const words = text.trim().split(/\s+/).filter(Boolean).length
  return Math.max(0.6, words / 2.6) + Math.max(0, pauseMs) / 1000
}

// ─── Sóng âm ─────────────────────────────────────────────────────────────

/** Thu/giãn đỉnh sóng server (≈ 2 giá trị/giây) về đúng số vạch cần vẽ. */
export function peakBars(peaks: number[], count: number): number[] {
  const bars = Math.max(1, Math.round(count))
  if (peaks.length === 0) return Array.from({ length: bars }, () => 0.18)
  return Array.from({ length: bars }, (_, index) => {
    const start = Math.floor((index * peaks.length) / bars)
    const end = Math.max(start + 1, Math.floor(((index + 1) * peaks.length) / bars))
    let max = 0
    for (let cursor = start; cursor < end && cursor < peaks.length; cursor++) max = Math.max(max, peaks[cursor])
    return Math.min(1, Math.max(0.06, max))
  })
}

/** Vạch sóng cao `height%`, tối thiểu 12% để luôn thấy đường nền. */
export const barHeight = (value: number) => `${Math.max(6, Math.round(value * 100))}%`

/** Số cột cuối sóng tạo giọng mang màu “mới nhất” (22-brief Câu 18: 3 cột cuối). */
export const NEWEST_BARS = 3

export type GenerationTone = 'pending' | 'newest' | 'done'

/** Màu từng vạch sóng tạo giọng (Studio): chưa tới = `pending`, đúng `NEWEST_BARS` cột cuối đã tạo = `newest`, còn lại = `done`. */
export function generationTone(index: number, filled: number): GenerationTone {
  if (index >= filled) return 'pending'
  return index >= filled - NEWEST_BARS ? 'newest' : 'done'
}

/** Phần đã phát của sóng (0…1) — tô sáng bên trái kim. */
export function playedFraction(timeSec: number, durationSec: number): number {
  if (durationSec <= 0) return 0
  return Math.min(1, Math.max(0, timeSec / durationSec))
}

// ─── Dòng chạy theo (FollowAlongScript) ──────────────────────────────────

export type FollowEntry = { segment: AudioSegment; startSec: number; endSec: number }

/** Cùng thứ tự ghép ở internal/audio/chapter.go: giọng thật rồi khoảng lặng sau đoạn. */
export function followTimeline(segments: AudioSegment[], cast?: AudioCast | null): FollowEntry[] {
  let cursor = 0
  return segments.map((segment) => {
    const dialoguePause = cast?.mode === 'single' && segment.speaker !== 'dan'
      ? Math.max(0, cast.single.dialoguePauseMs) : 0
    const pauseMs = Math.max(0, segment.pauseMs + dialoguePause)
    const seconds = segment.audio && segment.audio.durationSec > 0
      ? segment.audio.durationSec + pauseMs / 1000
      : estimateSegmentSeconds(segment.text, pauseMs)
    const entry = { segment, startSec: cursor, endSec: cursor + seconds }
    cursor += seconds
    return entry
  })
}

/** Đoạn đang đọc tại `timeSec`; trước đoạn đầu → 0, sau đoạn cuối → cuối. */
export function followIndex(timeline: FollowEntry[], timeSec: number): number {
  if (timeline.length === 0) return -1
  for (let index = 0; index < timeline.length; index++) {
    if (timeSec < timeline[index].endSec) return index
  }
  return timeline.length - 1
}

// ─── Cờ cần nghe lại & chỗ cần xem ───────────────────────────────────────

/** Cờ sắp theo vị trí nghe (chương rồi thời điểm). */
export const sortFlags = (flags: AudioFlag[]) =>
  [...flags].sort((a, b) => (a.chapter - b.chapter) || (a.atSec - b.atSec))

/** Vị trí cờ trên sóng (0…1). */
export function flagPosition(flag: AudioFlag, durationSec: number): number {
  if (durationSec <= 0) return 0
  return Math.min(1, Math.max(0, flag.atSec / durationSec))
}

/** Cờ kế/trước trong danh sách “Cần nghe lại” (có quay vòng). */
export function stepFlagIndex(flags: AudioFlag[], currentId: string | null, direction: 1 | -1): number {
  if (flags.length === 0) return -1
  const current = flags.findIndex((flag) => flag.id === currentId)
  if (current < 0) return direction > 0 ? 0 : flags.length - 1
  return (current + direction + flags.length) % flags.length
}

/** Đoạn có cờ cần xem; chế độ một giọng bỏ cờ về người nói/vai chưa có giọng. */
export function issueSegments(segments: AudioSegment[], mode: AudioCast['mode'] = 'cast'): AudioSegment[] {
  return segments.filter((segment) => segment.flags?.some((flag) =>
    mode === 'single' ? flag !== 'unknown_speaker' && flag !== 'no_voice' : true))
}

/** Chỗ cần xem kế/trước trong danh sách đoạn (J/K) — quay vòng, `-1` khi không có. */
export function stepIssueIndex(segments: AudioSegment[], from: number, direction: 1 | -1, mode: AudioCast['mode'] = 'cast'): number {
  const indexes = segments
    .map((segment, index) => ({ segment, index }))
    .filter(({ segment }) => issueSegments([segment], mode).length > 0)
    .map(({ index }) => index)
  if (indexes.length === 0) return -1
  if (direction > 0) return indexes.find((index) => index > from) ?? indexes[0]
  const before = indexes.filter((index) => index < from)
  return before.length > 0 ? before[before.length - 1] : indexes[indexes.length - 1]
}

// ─── Bản đọc: chuẩn hoá, lọc, trạng thái chương ──────────────────────────

export type SegmentPart = { text: string; normalized?: NonNullable<AudioSegment['normalized']>[number] }

/** Tách câu theo chỗ đã chuẩn hoá — phần chuẩn hoá gạch chân chấm xanh, hover xem gốc. */
export function segmentParts(segment: AudioSegment): SegmentPart[] {
  const text = segment.text
  const entries = (segment.normalized ?? []).filter((entry) => entry.from && entry.to)
  if (entries.length === 0) return [{ text }]
  const parts: SegmentPart[] = []
  let cursor = 0
  while (cursor < text.length) {
    let next: { at: number; entry: NonNullable<AudioSegment['normalized']>[number] } | null = null
    for (const entry of entries) {
      const at = text.indexOf(entry.to, cursor)
      if (at >= 0 && (next === null || at < next.at)) next = { at, entry }
    }
    if (!next) {
      parts.push({ text: text.slice(cursor) })
      break
    }
    if (next.at > cursor) parts.push({ text: text.slice(cursor, next.at) })
    parts.push({ text: next.entry.to, normalized: next.entry })
    cursor = next.at + next.entry.to.length
  }
  return parts.filter((part) => part.text.length > 0)
}

export type SegmentFilter = 'all' | 'issues' | 'other' | `spk:${string}`

export function filterSegments(segments: AudioSegment[], filter: SegmentFilter, mode: AudioCast['mode'] = 'cast'): AudioSegment[] {
  if (filter === 'all') return segments
  if (filter === 'issues') return issueSegments(segments, mode)
  if (filter === 'other') return segments.filter((segment) => segment.speaker !== 'th' && segment.speaker !== 'nv' && segment.speaker !== 'dan')
  const speaker = filter.slice(4)
  return segments.filter((segment) => segment.speaker === speaker)
}

/** Chip lọc theo người nói: hai vai nhiều đoạn nhất + “Người khác”. */
export function speakerFilters(segments: AudioSegment[], cast: AudioCast | null): { id: SegmentFilter; label: string; count: number }[] {
  const counts = new Map<string, number>()
  for (const segment of segments) counts.set(segment.speaker, (counts.get(segment.speaker) ?? 0) + 1)
  const named = [...counts.entries()]
    .filter(([speaker]) => speaker !== 'dan' && speaker !== 'unk')
    .sort((a, b) => b[1] - a[1])
    .slice(0, 2)
  const chips = named.map(([speaker, count]) => ({
    id: `spk:${speaker}` as SegmentFilter,
    label: speakerName(cast, speaker),
    count,
  }))
  const other = segments.filter((segment) => !named.some(([speaker]) => speaker === segment.speaker) && segment.speaker !== 'dan').length
  if (other > 0) chips.push({ id: 'other', label: t.audio.logic.nguoiKhac, count: other })
  return chips
}

export function speakerName(cast: AudioCast | null, id: string): string {
  if (id === 'unk') return t.audio.logic.chuaRoNguoiNoi
  return cast?.speakers.find((speaker) => speaker.id === id)?.name ?? id
}

export function speakerOf(cast: AudioCast | null, id: string): AudioCastSpeaker | null {
  return cast?.speakers.find((speaker) => speaker.id === id) ?? null
}

export const voiceName = (voices: AudioVoice[], voiceId: string | null) =>
  voices.find((voice) => voice.id === voiceId)?.name ?? ''

/** Máy chủ gửi % thô (vd 3/7 đoạn = 42.857…); mọi chỗ hiển thị dùng số nguyên 0…100. */
export const displayPercent = (value: number) => Math.min(100, Math.max(0, Math.round(value)))

/** `xong · 16:40` · `đang tạo 38%` · `12/46 đoạn có giọng` · `2 đoạn cần đọc lại` — dòng audio cột trái. */
export function chapterAudioLine(chapter: AudioChapter): { text: string; tone: 'ok' | 'accent' | 'muted' | 'warn'; running: boolean; percent: number } | null {
  if (chapter.status === 'todo' || chapter.status === 'draft') return null
  if (chapter.chapterChanged || chapter.scriptChanged) return { text: chapter.staleReason || t.audio.generation.edited, tone: 'warn', running: false, percent: 0 }
  if (chapter.status === 'stale') return { text: t.audio.logic.doanCanDocLai(chapter.changedSegments), tone: 'warn', running: false, percent: 0 }
  const audio = chapter.audio
  if (!audio) return null
  if (audio.segmentsDone >= audio.segments) {
    return { text: `xong · ${formatClock(audio.durationSec ?? chapter.durationSec ?? 0)}`, tone: 'ok', running: false, percent: 100 }
  }
  const percent = displayPercent(audio.percent)
  if (audio.rendering) {
    return { text: t.audio.logic.dangTao(percent), tone: 'accent', running: true, percent }
  }
  return { text: t.audio.logic.doanCoGiong(audio.segmentsDone, audio.segments), tone: 'muted', running: false, percent }
}

/** Nhãn trạng thái bản đọc dưới tên chương. */
export function chapterStatusLabel(chapter: AudioChapter, mode: AudioCast['mode'] = 'cast'): string {
  switch (chapter.status) {
    case 'todo': return t.audio.logic.chuaSoan
    case 'draft': return t.audio.logic.aiDangSoan
    case 'review': {
      const issues = mode === 'single' ? Math.max(0, chapter.issues - 2) : chapter.issues
      return issues > 0 ? t.audio.logic.choDuyetCanXem(issues) : t.audio.logic.choDuyet
    }
    case 'locked': return t.audio.logic.daChot
    case 'stale': return t.audio.logic.suaSauKhiChot
  }
}

export const lockedCount = (chapters: AudioChapter[]) => chapters.filter((chapter) => chapter.status === 'locked').length

export const todoChapters = (chapters: AudioChapter[]) => chapters.filter((chapter) => chapter.status === 'todo')

/** 8/8 chương đã chốt và có đủ giọng → mở “Nghe & xuất”. */
export const allDone = (chapters: AudioChapter[]) =>
  chapters.length > 0 && chapters.every((chapter) =>
    chapter.status === 'locked' && chapter.audio !== null && chapter.audio.segmentsDone >= chapter.audio.segments)

/** 8/8 đã chốt (đủ để nghe cả tập) — khác `allDone` ở chỗ chưa cần xong audio. */
export const allLocked = (chapters: AudioChapter[]) =>
  chapters.length > 0 && chapters.every((chapter) => chapter.status === 'locked')

export function volumeTotals(chapters: AudioChapter[]): { segments: number; minutes: number; speakers: number } {
  return {
    segments: chapters.reduce((sum, chapter) => sum + chapter.segments, 0),
    minutes: Math.round(chapters.reduce((sum, chapter) => sum + (chapter.durationSec ?? 0), 0) / 60),
    speakers: Math.max(0, ...chapters.map((chapter) => chapter.speakers)),
  }
}

/** Tổng % của cả tập theo số đoạn đã có giọng (vòng tiến độ + thẻ nổi). */
export function overallPercent(chapters: AudioChapter[]): number {
  const total = chapters.reduce((sum, chapter) => sum + (chapter.audio?.segments ?? 0), 0)
  const done = chapters.reduce((sum, chapter) => sum + (chapter.audio?.segmentsDone ?? 0), 0)
  if (total === 0) return 0
  return Math.min(100, Math.round((done / total) * 100))
}

// ─── Phòng thu: hàng đợi, 4 bước, tốc độ máy ─────────────────────────────

/** Job đang chạy của một chương (hoặc job đang chạy bất kỳ khi không truyền chương). */
export function activeJob(jobs: AudioQueueJob[], chapter?: number): AudioQueueJob | null {
  const relevant = jobs.filter((job) => job.state === 'rendering' || job.state === 'paused' || job.state === 'error' || job.state === 'queued')
  if (chapter !== undefined) return relevant.find((job) => job.chapter === chapter) ?? null
  return relevant.find((job) => job.state === 'rendering')
    ?? relevant.find((job) => job.state === 'paused')
    ?? relevant.find((job) => job.state === 'error')
    ?? relevant[0]
    ?? null
}

/** 4 bước của đoạn: Chuẩn bị câu → Tạo giọng → Làm sạch âm → Ghép vào chương. */
export function renderStep(segmentPercent: number): number {
  const percent = Math.min(1, Math.max(0, segmentPercent))
  if (percent < 0.14) return 0
  if (percent < 0.75) return 1
  if (percent < 0.9) return 2
  return 3
}

export const RENDER_STEPS = [t.audio.logic.chuanBiCau, t.audio.logic.taoGiong, t.audio.logic.lamSachAm, t.audio.logic.ghepVaoChuong] as const

/** Tốc độ tạo của máy: `1 phút audio ≈ 41 giây` (rtf = thời gian tạo / thời lượng audio). */
export function renderSpeedLabel(rtf: number | null): string {
  if (rtf === null || rtf <= 0) return t.audio.logic.dangDoTocDo
  return t.audio.logic.n1PhutAudioGiay(Math.max(1, Math.round(rtf * 60)))
}

// ─── Ghép & xuất ─────────────────────────────────────────────────────────

/** Dòng mốc chương để dán vào mô tả video. */
export const chapterMarkLines = (marks: { atSec: number; label: string }[]) =>
  marks.map((mark) => `${formatClock(mark.atSec)} ${mark.label}`)

/** Mốc chương dạng văn bản (nút Sao chép). */
export const chapterMarksText = (marks: { atSec: number; label: string }[]) => chapterMarkLines(marks).join('\n')

/** Dung lượng cần cho bản ghép: tệp chính + từng chương + 15% tạm. */
export function exportDiskNeed(durationSec: number, format: 'm4b' | 'mp3' | 'wav'): number {
  const bytesPerSecond = format === 'wav' ? 192_000 : format === 'mp3' ? 24_000 : 16_000
  return Math.round(durationSec * bytesPerSecond * 2 * 1.15)
}

/** Tên tệp chính của bản ghép (từ danh sách tệp xuất). */
export const mainExportFile = <T extends { name: string; format: string }>(files: T[]) =>
  files.find((file) => file.format === 'audio' && file.name.endsWith('.mp3')) ?? files.find((file) => file.format === 'audio') ?? null
