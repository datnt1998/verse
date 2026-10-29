import type { DiagFinding, DiagSeverity, DiagStats, ImportPreview, ImportPreviewChapter, JobEvent } from '../../../api/types'
import { commandName, diagTiles, formatInt, type DiffLine, type StatTile } from '../logic'

export type MobileResultKind = 'cocreate' | 'import' | 'diag' | 'sync' | 'reopen'
export function resultKind(command: string, jobKind?: string): MobileResultKind | null {
  const name = commandName(command.startsWith('/') ? command : `/${command}`) || jobKind
  return name === 'cocreate' || name === 'import' || name === 'diag' || name === 'sync' || name === 'reopen' ? name : null
}

/** --guide consumes the rest of the line; never append a flag after it. */
export function importCommand(path: string, status: 'open' | 'closed', continueAfter: boolean, guide: string): string {
  const file = /\s/.test(path) ? JSON.stringify(path) : path
  return [`/import ${file}`, `--story=${status}`, ...(status === 'open' && continueAfter ? ['--continue'] : []), '--yes', ...(guide.trim() ? [`--guide=${guide.trim()}`] : [])].join(' ')
}

export function importPath(command: string): string {
  const value = command.replace(/^\/?import\s*/i, '').trimStart()
  const match = /^("(?:\\.|[^"\\])*"|'[^']*'|[^\s]+)/.exec(value)?.[1] ?? ''
  if (match.startsWith('--')) return ''
  if (match.startsWith('"')) { try { return JSON.parse(match) as string } catch { return '' } }
  return match.replace(/^'|'$/g, '')
}

/** Six rows: opening context and the unusually long / newly split chapters. */
export function previewRows<T extends ImportPreviewChapter & { newlySplit?: boolean; warning?: string }>(chapters: T[], all: boolean): T[] {
  if (all || chapters.length <= 6) return chapters
  const average = chapters.reduce((sum, row) => sum + row.words, 0) / chapters.length
  const focus = chapters.findIndex((row) => row.newlySplit || row.warning || row.words >= average * 2)
  const indices = new Set([0, 1, 2])
  const start = focus < 0 ? 3 : Math.max(3, Math.min(focus - 1, chapters.length - 3))
  for (let index = start; index < start + 3; index++) indices.add(index)
  return [...indices].sort((a, b) => a - b).map((index) => chapters[index])
}

export type ImportStep = { id: string; title: string; current: number; total: number; status: 'done' | 'running' | 'pending' }
const STAGES = [
  { id: 'ingesting', title: 'Đọc tệp' },
  { id: 'segmenting', title: 'Chia chương' },
  { id: 'summarizing', title: 'Tóm tắt từng chương' },
  { id: 'extracting', title: 'Dựng nhân vật & bối cảnh' },
  { id: 'planning', title: 'Lập dàn ý & phục bút' },
]
export function importSteps(job: JobEvent | null): ImportStep[] {
  const stage = job?.stage ?? ''
  const index = /complete|done|finished/.test(stage) ? 5 : /plan|outline|foreshadow|finaliz|persist|validat|publish|awaiting_story_status/.test(stage) ? 4 : /extract|character|world|context|synthesiz/.test(stage) ? 3 : /summari|analyz/.test(stage) ? 2 : /segment|split|confirm|status/.test(stage) ? 1 : 0
  return STAGES.map((step, at) => ({ ...step, current: at === 2 ? job?.current ?? 0 : 0, total: at === 2 ? job?.total ?? 0 : 0, status: job?.done && !job.error || at < index ? 'done' : at === index ? 'running' : 'pending' }))
}

/** Cột số bên phải mỗi bước nhập (MCmd runSteps): bước 2 hiện số chương khi xong, bước 3 hiện tiến độ khi đang chạy/đã xong, còn lại để trống. */
export function stepCount(step: ImportStep, index: number, chapters: number): string {
  if (index === 1) return step.status === 'done' && chapters > 0 ? String(chapters) : ''
  if (index === 2) return step.status !== 'pending' && step.total > 0 ? `${step.current}/${step.total}` : ''
  return ''
}

/** Worker resets may clear the accumulated stream between two replies. */
export function replyDelta(stream: string, previous: string): string {
  return stream.startsWith(previous) ? stream.slice(previous.length) : stream
}

/** Chương dài bất thường: backend đã gắn `warning`, hoặc dài từ gấp đôi trung bình. */
export function longChapters<T extends ImportPreviewChapter>(chapters: T[]): T[] {
  if (chapters.length === 0) return []
  const average = chapters.reduce((sum, row) => sum + row.words, 0) / chapters.length
  return chapters.filter((row) => !!row.warning || row.words >= average * 2)
}

/** Câu cảnh báo chương dài đúng chữ của artboard MCmd (một khung cho mỗi chương). */
export function longChapterWarning(chapter: ImportPreviewChapter): string {
  return `Chương ${chapter.n} dài ${formatInt(chapter.words)} chữ — gấp đôi trung bình, có thể đang dính hai chương.`
}

/** Cảnh báo khác của backend (mã hoá, ghi chú tách…) — bỏ các câu chỉ nói chương dài vì đã dựng ở trên. */
export function otherImportWarnings(preview: Pick<ImportPreview, 'chapters' | 'warnings'>): string[] {
  const own = new Set(preview.chapters.map((row) => row.warning).filter(Boolean))
  return preview.warnings.filter((text) => !own.has(text) && !/gấp đôi/.test(text))
}

/** Dòng phụ dưới số chương: “Chia theo dòng «Chương N» · trung bình 7.760 chữ/chương” hoặc theo gợi ý. */
export function splitSummary(preview: Pick<ImportPreview, 'chapters' | 'splitHow' | 'words'>, guided: boolean): string {
  const words = preview.words ?? preview.chapters.reduce((sum, row) => sum + row.words, 0)
  if (guided) return `Chia theo gợi ý của bạn · ${formatInt(words)} chữ`
  const average = preview.chapters.length ? words / preview.chapters.length : 0
  const how = (preview.splitHow ?? '').trim()
  const lead = /^theo\s/i.test(how) ? `Chia ${how.charAt(0).toLowerCase()}${how.slice(1)}` : how
  return [lead, `trung bình ${formatInt(average)} chữ/chương`].filter(Boolean).join(' · ')
}

/** Mặc định “Viết tiếp ngay sau khi nhập”: bật như artboard khi lệnh không nêu trạng thái truyện. */
export function defaultContinue(command: string): boolean {
  return /(?:^|\s)--story=/.test(command) ? /(?:^|\s)--continue(?:\s|$)/.test(command) : true
}

export type ImpactKind = { icon: string; color: string }
/** Chip tác động của đề xuất: sửa dàn ý (xanh) · giữ nguyên (lục) · phục bút (hổ phách). */
export function impactKind(text: string): ImpactKind {
  if (/phục bút/i.test(text)) return { icon: 'flag', color: '#F6CB8C' }
  if (/giữ nguyên|không (?:đổi|sửa|ảnh hưởng)/i.test(text)) return { icon: 'lock', color: '#A6EDC1' }
  return { icon: 'edit-2', color: '#9DB0F7' }
}

/** “25–27” từ các chương đề xuất; một chương thì chỉ số đó. */
export function chapterRange(chapters: { chapter: number }[]): string {
  if (chapters.length === 0) return ''
  const numbers = chapters.map((row) => row.chapter)
  const first = Math.min(...numbers), last = Math.max(...numbers)
  return first === last ? String(first) : `${first}–${last}`
}

export const MOBILE_SEVERITY_META: Record<DiagSeverity, { label: string; color: string; halo: string }> = {
  critical: { label: 'Nghiêm trọng', color: '#FF9FA8', halo: 'rgba(255,159,168,0.14)' },
  warning: { label: 'Cảnh báo', color: '#F6CB8C', halo: 'rgba(246,203,140,0.14)' },
  info: { label: 'Gợi ý', color: '#9DB0F7', halo: 'rgba(157,176,247,0.14)' },
}

/** Nhãn ô tổng quan của MCmd khác bản máy tính một vài chữ (“Phục bút mở”, “Đã viết …”). */
export function mobileDiagTiles(stats: DiagStats, findings: DiagFinding[], phaseLabel: (phase: string) => string, now = new Date()): StatTile[] {
  return diagTiles(stats, findings, phaseLabel, now).map((tile) => ({
    ...tile,
    key: tile.key === 'Phục bút đang mở' ? 'Phục bút mở' : tile.key,
    sub: tile.sub.replace(/^Giai đoạn: (.)/, (_, first: string) => `Giai đoạn ${first.toLowerCase()}`).replace(/^đã viết /, 'Đã viết '),
  }))
}

/** Dòng cảnh báo trên nút “Mở lại truyện” khi bị chặn: “Cần duyệt 2 chương trước khi mở lại”. */
export function blockedNote(checks: { id: string; ok: boolean; note: string; chapters: number[] }[]): string {
  const failing = checks.find((check) => !check.ok)
  if (failing?.chapters.length) return `Cần duyệt ${failing.chapters.length} chương trước khi mở lại`
  return failing?.note || 'Cần hoàn tất các điều kiện trước khi mở lại'
}

/** MCmd chỉ vẽ phần sửa: bỏ đoạn không đổi, mỗi cụm sửa liền nhau hiện đoạn cũ (gạch) trước rồi đoạn mới. */
export function changedLines(lines: DiffLine[]): DiffLine[] {
  const out: DiffLine[] = []
  let removed: DiffLine[] = []
  let added: DiffLine[] = []
  const flush = () => { out.push(...removed, ...added); removed = []; added = [] }
  for (const line of lines) {
    if (line.kind === 'same') flush()
    else if (line.kind === 'del') removed.push(line)
    else added.push(line)
  }
  flush()
  return out
}

/** Id việc nền đầy đủ như backend `qualifyJob`: `<dự án>/<truyện>/<loại>/<số>`; id đã đủ tiền tố thì giữ nguyên. */
export function qualifyJobId(project: string, story: string, id: string): string {
  const prefix = `${project}/${story}/`
  return id.startsWith(prefix) ? id : `${prefix}${id}`
}
