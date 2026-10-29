// D13 · Chất lượng truyện (Quality.dc.html, GAP-42) — phần tính toán thuần.
// Máy chủ trả mục thẩm duyệt kèm trạng thái qua các lần chấm; mọi tổng hàng/cột, bộ lọc mức
// và phạm vi, ba nhóm “Việc cần làm” và dải số đều tính ở client (22-brief-2510.md › D13).
import type { Quality, QualityChapterStatus, QualityIssue } from '../../api/types'
import { reviewLevel } from '../../ui/ReviewIssue'

export type QualityView = 'todo' | 'matrix'
export type QualityLevelFilter = 'all' | 'error' | 'warning'
export type QualityScope = 'all' | 'open'
/** Ô = ref + ch · cột = chỉ ch · hàng = chỉ ref (artboard `sel`: cell · chapter · row). */
export type QualityPick = { ref?: string; ch?: number }

/** Đếm của một tập mục: e/w = Bắt sửa/Cảnh báo còn mở, f = đã sửa, k = giữ nguyên. */
export type Tally = { e: number; w: number; f: number; k: number }

export const tally = (issues: readonly QualityIssue[]): Tally => {
  const out: Tally = { e: 0, w: 0, f: 0, k: 0 }
  for (const issue of issues) {
    if (issue.state === 'fixed') out.f++
    else if (issue.state === 'kept') out.k++
    else if (reviewLevel(issue) === 'error') out.e++
    else out.w++
  }
  return out
}

/** Bộ lọc phía client: mức (Tất cả · Bắt sửa · Cảnh báo) và phạm vi (Mọi lần chấm · Chỉ còn mở). */
export const filterIssues = (issues: readonly QualityIssue[], level: QualityLevelFilter, scope: QualityScope): QualityIssue[] =>
  issues.filter((issue) => (level === 'all' || reviewLevel(issue) === level) && (scope === 'all' || issue.state === 'open'))

/** Nguồn của mục luật → màu chip (artboard SRC: M Master · A bộ của gói · X Xưng hô · U Quy tắc của bạn). */
export type RefSource = 'master' | 'pack' | 'address' | 'user'
export function refSource(ref: string): RefSource {
  if (ref === 'Xưng hô') return 'address'
  if (ref === 'Quy tắc của bạn') return 'user'
  if (ref.startsWith('Master')) return 'master'
  return /§/.test(ref) ? 'pack' : 'master'
}

/** Hàng ma trận: mục luật có ít nhất một mục ở bất kỳ lần chấm nào, giữ thứ tự gói của máy chủ. */
export const matrixRows = (data: Quality) =>
  data.sections.filter((section) => data.issues.some((issue) => issue.ref === section.ref))

/** Truyện dài: artboard có đúng hai bố cục — 8 cột co giãn và cuộn ngang ô 38 px (`len=40`). */
export const isLong = (data: Quality) => data.chapters.length > 8

/** Chương “Bản trước” hoặc “Đang viết lại”: cảnh báo ở đó sẽ tự xử lý khi viết lại. */
export const isPending = (status: QualityChapterStatus | undefined) => status === 'previous' || status === 'rewriting'

/** Ô trong ma trận theo bộ lọc. Ô trống (không mục nào) = Đạt. */
export function cellIssues(issues: readonly QualityIssue[], ref: string, ch: number) {
  return issues.filter((issue) => issue.ref === ref && issue.ch === ch)
}

/**
 * Tổng của hàng: còn mở (e + w) nếu có; không thì “✓N” đã sửa (chỉ khi xem mọi lần chấm); không thì “—”.
 * `tone`: err · warn · ok · muted (artboard totFg).
 */
export function rowTotal(t: Tally, scope: QualityScope): { text: string; tone: 'err' | 'warn' | 'ok' | 'muted' } {
  const open = t.e + t.w
  const text = open ? String(open) : t.f && scope === 'all' ? `✓${t.f}` : '—'
  return { text, tone: t.e ? 'err' : t.w ? 'warn' : t.f ? 'ok' : 'muted' }
}

/** Tổng của cột: số mục còn mở (0 khi ô trống), màu theo mục nặng nhất. */
export function colTotal(t: Tally, hasIssues: boolean): { text: string; tone: 'err' | 'warn' | 'muted' } {
  return { text: hasIssues ? String(t.e + t.w) : '0', tone: t.e ? 'err' : t.w ? 'warn' : 'muted' }
}

/**
 * Chiều cao hai vạch ở đầu cột (px): tỉ lệ theo chương có nhiều mục còn mở nhất (không lọc),
 * cộng 2 px khi khác 0 — đúng công thức artboard (`eh`/`wh`).
 */
export function barHeights(t: Tally, maxOpen: number): { e: number; w: number } {
  const max = Math.max(1, maxOpen)
  return { e: Math.round((t.e / max) * 20 + (t.e ? 2 : 0)), w: Math.round((t.w / max) * 20 + (t.w ? 2 : 0)) }
}

export const maxOpenPerChapter = (data: Quality) =>
  Math.max(1, ...data.chapters.map((chapter) => data.issues.filter((issue) => issue.ch === chapter.n && issue.state === 'open').length))

/** “1–3, 5” — dãy chương liền nhau gộp bằng gạch nối. */
export function chapterRanges(chapters: readonly number[]): string {
  const sorted = [...new Set(chapters)].sort((a, b) => a - b)
  const parts: string[] = []
  for (let i = 0; i < sorted.length; i++) {
    const start = sorted[i]
    while (i + 1 < sorted.length && sorted[i + 1] === sorted[i] + 1) i++
    parts.push(start === sorted[i] ? String(start) : `${start}–${sorted[i]}`)
  }
  return parts.join(', ')
}

/** Mục luật bị vi phạm nhiều nhất (mọi trạng thái), tối đa 3, kèm bề rộng thanh so với mục đầu. */
export function topViolated(data: Quality): { ref: string; refName: string; n: number; width: number }[] {
  const rows = data.sections
    .map((section) => ({ ...section, n: data.issues.filter((issue) => issue.ref === section.ref).length }))
    .filter((row) => row.n > 0)
    .sort((a, b) => b.n - a.n)
    .slice(0, 3)
  const max = rows[0]?.n ?? 1
  return rows.map((row) => ({ ref: row.ref, refName: row.refName, n: row.n, width: Math.round((row.n / max) * 100) }))
}

/**
 * Ba nhóm “Việc cần làm” (chỉ mục còn mở, theo bộ lọc mức, xếp theo chương):
 * Đang chặn chương = Bắt sửa · Cần bạn xem = Cảnh báo ở chương đã viết xong ·
 * Sẽ tự sửa khi viết lại = Cảnh báo ở chương đang hoặc sắp viết lại.
 * `done` = mục vừa nhờ sửa trong phiên này (rời danh sách như artboard).
 */
export type TodoGroups = { block: QualityIssue[]; look: QualityIssue[]; auto: QualityIssue[] }
export function todoGroups(data: Quality, level: QualityLevelFilter, done: ReadonlySet<string>): TodoGroups {
  const status = new Map(data.chapters.map((chapter) => [chapter.n, chapter.status]))
  const open = data.issues
    .filter((issue) => issue.state === 'open' && !done.has(issue.id) && (level === 'all' || reviewLevel(issue) === level))
    .sort((a, b) => a.ch - b.ch)
  const block = open.filter((issue) => reviewLevel(issue) === 'error')
  const look = open.filter((issue) => reviewLevel(issue) === 'warning' && !isPending(status.get(issue.ch)))
  const auto = open.filter((issue) => reviewLevel(issue) === 'warning' && isPending(status.get(issue.ch)))
  return { block, look, auto }
}

/** Dòng phụ của nhóm gập: “chương 5–8” (quá 3 chương) hoặc “chương 5, 6”. */
export function autoChapters(auto: readonly QualityIssue[]): string {
  const chapters = [...new Set(auto.map((issue) => issue.ch))].sort((a, b) => a - b)
  if (!chapters.length) return ''
  return chapters.length > 3 ? `${chapters[0]}–${chapters[chapters.length - 1]}` : chapters.join(', ')
}

/** Chương đầu đang hoặc sắp viết lại và chương cuối — vế “ở chương 5–8” của đoạn tóm tắt. */
export function pendingSpan(data: Quality): { from: number; to: number } | null {
  const first = data.chapters.find((chapter) => isPending(chapter.status))
  const last = data.chapters[data.chapters.length - 1]
  return first && last ? { from: first.n, to: last.n } : null
}

/** Số việc ở tab “Việc cần làm” = Đang chặn chương + Cần bạn xem (cũng là “N việc cần xem” ở Trình đọc). */
export function todoCount(data: Quality, level: QualityLevelFilter = 'all', done: ReadonlySet<string> = new Set()): number {
  const { block, look } = todoGroups(data, level, done)
  return block.length + look.length
}

/** Chương (không lặp, tăng dần) của một tập mục, nối bằng “, ” như artboard. */
export const chapterList = (issues: readonly QualityIssue[]) =>
  [...new Set(issues.map((issue) => issue.ch))].sort((a, b) => a - b).join(', ')

/** Năm ô số (không theo bộ lọc): Bắt sửa còn mở · Cảnh báo còn mở · Đã sửa · Giữ nguyên · Chương đạt. */
export function kpis(data: Quality) {
  const all = tally(data.issues)
  const openErrors = data.issues.filter((issue) => issue.state === 'open' && reviewLevel(issue) === 'error')
  const passed = data.chapters.filter((chapter) => !isPending(chapter.status)).map((chapter) => chapter.n)
  return { all, errorChapters: chapterList(openErrors), passed: passed.length, passedRange: chapterRanges(passed), total: data.chapters.length }
}

/** Đoạn “Tóm tắt” ở panel phải của Việc cần làm (artboard `summary`). */
export type SummaryParts = { block: number; blockChapters: string; look: number; auto: number; span: { from: number; to: number } | null }
export function summaryParts(data: Quality, groups: TodoGroups): SummaryParts {
  return {
    block: groups.block.length, blockChapters: chapterList(groups.block),
    look: groups.look.length, auto: groups.auto.length, span: pendingSpan(data),
  }
}

/** Artboard chỉ dựng tối đa 6 thẻ Bắt sửa và 8 thẻ Cảnh báo; số đếm ở tiêu đề nhóm vẫn là tổng. */
export const GROUP_LIMIT = { block: 6, look: 8 } as const

/** Ô chọn mặc định: ô đầu tiên (theo thứ tự hàng rồi chương) còn mục mở; không có thì ô đầu có mục. */
export function initialPick(data: Quality): QualityPick {
  const rows = matrixRows(data)
  for (const state of ['open', null] as const) {
    for (const row of rows) {
      for (const chapter of data.chapters) {
        if (data.issues.some((issue) => issue.ref === row.ref && issue.ch === chapter.n && (state === null || issue.state === state))) {
          return { ref: row.ref, ch: chapter.n }
        }
      }
    }
  }
  return data.chapters[0] ? { ch: data.chapters[0].n } : {}
}

/** Mục của vùng đang chọn trong panel phải (theo bộ lọc). */
export function pickedIssues(issues: readonly QualityIssue[], pick: QualityPick): QualityIssue[] {
  return issues.filter((issue) => (pick.ref === undefined || issue.ref === pick.ref) && (pick.ch === undefined || issue.ch === pick.ch))
}

/** Lớp luật có mặt (“Master”, “Bộ A”) cho phụ đề “Chấm theo gói … (Master + Bộ A)”. */
export function ruleLayers(data: Quality): string[] {
  const layers: string[] = []
  for (const section of data.sections) {
    const match = /^(.+?)\s*§/.exec(section.ref)
    const layer = match?.[1]?.trim()
    if (layer && !layers.includes(layer)) layers.push(layer)
  }
  return layers
}
