// Logic thuần cho CmdResults.dc.html (14-jobs-dock §2): lưới xuất, số chữ ±, báo cáo /diag,
// bảng /model, hồ sơ văn phong. Không chạm React/API để test được trực tiếp.
import type {
  DiagFinding, DiagSeverity, DiagStats, ModelCatalogProvider, ModelRole, ModelRoleKey,
  SimulationSynthesis, SyncAnalysis,
} from '../../api/types'
import { formatContextK } from '../../lib/modelContext'
import { vi } from '../../i18n/vi'

// ─── Số & thời gian (vi-VN) ────────────────────────────────────────────────

const intFmt = new Intl.NumberFormat('vi-VN')
const dec1Fmt = new Intl.NumberFormat('vi-VN', { minimumFractionDigits: 1, maximumFractionDigits: 1 })

export const formatInt = (value: number) => intFmt.format(Math.round(value))

/** “+212” / “−48” — dấu trừ U+2212 như artboard; 0 vẫn có dấu để cột thẳng hàng. */
export function formatWordDelta(value: number, sign: '+' | '-'): string {
  return `${sign === '+' ? '+' : '−'}${formatInt(Math.abs(value))}`
}

/** “1,2 MB” — thập phân dấu phẩy theo vi-VN. */
export function formatSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  if (bytes < 1024) return `${Math.round(bytes)} B`
  if (bytes < 1024 * 1024) return `${dec1Fmt.format(bytes / 1024)} KB`
  return `${dec1Fmt.format(bytes / (1024 * 1024))} MB`
}

const pad = (n: number) => String(n).padStart(2, '0')
const DAY = 86_400_000
const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()

/** Giờ sửa của chương (thẻ /sync --check): “14:02 hôm nay” · “hôm qua” · “12/10”. */
export function formatEditedAt(iso: string | null, now = new Date()): string {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  const days = Math.round((startOfDay(now) - startOfDay(date)) / DAY)
  if (days <= 0) return `${pad(date.getHours())}:${pad(date.getMinutes())} hôm nay`
  if (days === 1) return 'hôm qua'
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)}`
}

/** “2 ngày trước” — bản đang có ở thẻ tệp trùng. */
export function formatAgo(iso: string | null, now = new Date()): string {
  if (!iso) return '—'
  const ms = now.getTime() - new Date(iso).getTime()
  if (Number.isNaN(ms)) return '—'
  if (ms < 60_000) return 'vừa xong'
  const minutes = Math.floor(ms / 60_000)
  if (minutes < 60) return `${minutes} phút trước`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} giờ trước`
  return `${Math.floor(hours / 24)} ngày trước`
}

/** Dải chương gọn: [25,26,27,28,29,30] → “25–30”; [3,5,6] → “3, 5–6”. */
export function formatChapterRanges(chapters: number[]): string {
  const sorted = [...new Set(chapters)].sort((a, b) => a - b)
  const parts: string[] = []
  for (let i = 0; i < sorted.length; i++) {
    const start = sorted[i]
    while (i + 1 < sorted.length && sorted[i + 1] === sorted[i] + 1) i++
    parts.push(start === sorted[i] ? String(start) : `${start}–${sorted[i]}`)
  }
  return parts.join(', ')
}

/** “3, 7, 12” — tiêu đề thẻ đã tiếp nhận. */
export const formatChapterList = (chapters: number[]) => [...chapters].sort((a, b) => a - b).join(', ')

// ─── /export — lưới 24 ô sáng dần ─────────────────────────────────────────

export type GridCell = { label: string; state: 'done' | 'now' | 'todo' }
export const EXPORT_GRID_CELLS = 24

/**
 * Chia tiến trình vào 24 ô. Truyện dài gom nhiều chương vào một ô; truyện ngắn có
 * nhiều ô cho mỗi chương. Ô “done” khi mọi chương của nó đã ghép; ô kế tiếp là “now”.
 */
export function exportGrid(current: number, total: number, done = false): { cells: GridCell[]; percent: number } {
  const count = EXPORT_GRID_CELLS
  const merged = done ? Math.max(total, 0) : Math.max(0, Math.min(current, total))
  const cells: GridCell[] = []
  let marked = false
  for (let i = 0; i < count; i++) {
    const first = total > 0 ? Math.floor((i * total) / count) + 1 : i + 1
    const last = total > 0 ? Math.max(first, Math.ceil(((i + 1) * total) / count)) : i + 1
    const label = first === last ? `Chương ${first}` : `Chương ${first}–${last}`
    let state: GridCell['state'] = 'todo'
    if (total > 0 && last <= merged) state = 'done'
    else if (!marked && !done && total > 0) { state = 'now'; marked = true }
    cells.push({ label, state })
  }
  const percent = done ? 100 : total > 0 ? Math.round((merged / total) * 100) : 0
  return { cells, percent }
}

// ─── /diag ────────────────────────────────────────────────────────────────

export type SeverityFilter = 'all' | DiagSeverity
const SEVERITY_RANK: Record<DiagSeverity, number> = { critical: 0, warning: 1, info: 2 }

/** Nghiêm trọng → Cảnh báo → Gợi ý; cùng mức giữ thứ tự engine (ổn định). */
export function sortFindings(findings: DiagFinding[]): DiagFinding[] {
  return findings
    .map((finding, index) => ({ finding, index }))
    .sort((a, b) => (SEVERITY_RANK[a.finding.severity] ?? 3) - (SEVERITY_RANK[b.finding.severity] ?? 3) || a.index - b.index)
    .map(({ finding }) => finding)
}

export function filterFindings(findings: DiagFinding[], filter: SeverityFilter): DiagFinding[] {
  const sorted = sortFindings(findings)
  return filter === 'all' ? sorted : sorted.filter((finding) => finding.severity === filter)
}

export function severityCounts(findings: DiagFinding[]): Record<SeverityFilter, number> {
  const counts: Record<SeverityFilter, number> = { all: findings.length, critical: 0, warning: 0, info: 0 }
  for (const finding of findings) if (finding.severity in counts) counts[finding.severity] += 1
  return counts
}

export const SEVERITY_META: Record<DiagSeverity, { label: string; color: string; halo: string }> = {
  critical: { label: 'Nghiêm trọng', color: '#FF9FA8', halo: 'rgba(255,159,168,0.12)' },
  warning: { label: 'Cảnh báo', color: '#F6CB8C', halo: 'rgba(246,203,140,0.12)' },
  info: { label: 'Gợi ý', color: '#9DB0F7', halo: 'rgba(157,176,247,0.12)' },
}
export const CATEGORY_LABEL: Record<string, string> = {
  flow: 'Quy trình', quality: 'Chất lượng', planning: 'Đại cương', context: 'Bối cảnh',
}
export const CONFIDENCE_LABEL: Record<string, string> = { high: 'Tin cậy cao', medium: 'Tin cậy vừa', low: 'Tham khảo' }

const PLANNING_TIER_LABEL: Record<string, string> = { short: 'Truyện ngắn', mid: 'Truyện vừa', long: 'Truyện dài' }

export type StatTile = { key: string; value: string; sub: string; warn: boolean }

/** 8 ô tổng quan theo `diag.Stats` (thứ tự & nhãn như artboard). */
export function diagTiles(stats: DiagStats, findings: DiagFinding[], phaseLabel: (phase: string) => string, now = new Date()): StatTile[] {
  const counts = severityCounts(findings)
  const started = stats.startedAt ? new Date(stats.startedAt) : null
  const startedOk = started && !Number.isNaN(started.getTime())
  const days = startedOk ? Math.max(0, Math.round((startOfDay(now) - startOfDay(started)) / DAY)) : 0
  const volumes = stats.totalVolumes > 0
  return [
    { key: 'Chương', value: `${stats.completedChapters}/${stats.totalChapters || '?'}`, sub: `Giai đoạn: ${phaseLabel(stats.phase) || '—'}`, warn: false },
    { key: 'Số chữ', value: formatInt(stats.totalWords), sub: `${formatInt(stats.avgWordsPerChapter)} chữ / chương`, warn: false },
    stats.avgReviewScore > 0
      ? { key: 'Điểm trung bình', value: dec1Fmt.format(stats.avgReviewScore), sub: 'Biên tập viên chấm', warn: false }
      : { key: 'Điểm trung bình', value: '—', sub: 'Chưa có điểm', warn: false },
    { key: 'Thẩm duyệt', value: `${formatInt(stats.reviewCount)} lần`, sub: `Viết lại ${formatInt(stats.rewriteCount)} lần`, warn: false },
    { key: 'Phục bút đang mở', value: formatInt(stats.foreshadowOpen), sub: `${formatInt(stats.foreshadowStale)} đình trệ`, warn: stats.foreshadowStale > 0 },
    volumes
      ? { key: 'Quy hoạch', value: 'Theo tập', sub: `Tập ${stats.currentVolume || 1}/${stats.totalVolumes}`, warn: false }
      : { key: 'Quy hoạch', value: PLANNING_TIER_LABEL[stats.planningTier] ?? '—', sub: stats.planningTier ? 'Không chia tập' : 'Chưa quy hoạch', warn: false },
    startedOk
      ? { key: 'Bắt đầu', value: `${pad(started.getDate())}/${pad(started.getMonth() + 1)}`, sub: `đã viết ${days} ngày`, warn: false }
      : { key: 'Bắt đầu', value: '—', sub: 'Chưa rõ', warn: false },
    { key: 'Phát hiện', value: formatInt(findings.length), sub: `${counts.critical} nghiêm trọng`, warn: false },
  ]
}

// ─── /model ───────────────────────────────────────────────────────────────

export const THINKING_LABEL: Record<string, string> = vi.thinkingLevel
export const thinkingLabel = (level: string) => THINKING_LABEL[level] ?? level

export type RolePick = { provider: string; model: string; thinking: string }

export const pickOf = (role: ModelRole): RolePick => ({ provider: role.provider, model: role.model, thinking: role.thinking })
export const samePick = (a: RolePick, b: RolePick) => a.provider === b.provider && a.model === b.model && a.thinking === b.thinking

/** Mức suy luận model hỗ trợ; model lạ (chưa có trong catalog) → chỉ “Mặc định”. */
export function thinkingFor(providers: ModelCatalogProvider[], provider: string, model: string): string[] {
  const found = providers.find((p) => p.provider === provider)?.models.find((m) => m.name === model)
  return found && found.thinking.length > 0 ? found.thinking : ['']
}

/** Đổi provider/model giữ mức đang chọn nếu model mới hỗ trợ, ngược lại về “Mặc định”. */
export function clampThinking(providers: ModelCatalogProvider[], pick: RolePick): RolePick {
  const levels = thinkingFor(providers, pick.provider, pick.model)
  return levels.includes(pick.thinking) ? pick : { ...pick, thinking: '' }
}

export const ROLE_META: Record<ModelRoleKey, { label: string; job: string; orb: 'architect' | 'writer' | 'editor' | null }> = {
  default: { label: 'Mặc định', job: 'Dùng cho mọi trợ lý chưa chọn riêng.', orb: null },
  architect: { label: 'Kiến trúc sư', job: 'Lên dàn ý, quy hoạch tập và giữ mạch truyện.', orb: 'architect' },
  writer: { label: 'Người viết', job: 'Viết từng chương — ảnh hưởng lớn nhất tới giọng văn.', orb: 'writer' },
  editor: { label: 'Biên tập viên', job: 'Soát nhất quán và chấm điểm chương.', orb: 'editor' },
}

/** Context chip shared by desktop and mobile model lists. */
export function formatContext(tokens: number): string {
  const value = formatContextK(tokens)
  return value ? `ngữ cảnh ${value}` : ''
}

/** “Đã lưu: Người viết → gpt-6-luna, Biên tập viên → deepseek-chat”. */
export function savedSummary(changed: { role: ModelRoleKey; pick: RolePick }[]): string {
  return `Đã lưu: ${changed.map(({ role, pick }) => `${ROLE_META[role].label} → ${pick.model}`).join(', ')}`
}

// ─── /simulate — 4 đặc điểm + câu tiêu biểu từ SimulationSynthesis ───────

export function simulationTraits(synthesis: SimulationSynthesis | undefined): { key: string; value: string }[] {
  const first = (values: string[] | undefined) => values?.find((v) => v.trim() !== '')?.trim() ?? '—'
  const style = synthesis?.style
  return [
    { key: 'Câu', value: first(style?.sentence_rhythm) },
    { key: 'Xưng hô', value: first(style?.perspective) },
    { key: 'Thoại', value: first(synthesis?.pacing_density?.dialogue_action_ratio) },
    { key: 'Giọng', value: first(style?.narrative_voice ?? style?.mood) },
  ]
}

export const signatureQuote = (synthesis: SimulationSynthesis | undefined) =>
  synthesis?.lexicon?.signature_phrases?.find((v) => v.trim() !== '')?.trim() ?? ''

// ─── /sync — nhãn từng chương đã tiếp nhận ───────────────────────────────

export function syncSummary(analyses: SyncAnalysis[]): { issues: number; storyChanged: number } {
  let issues = 0
  let storyChanged = 0
  for (const a of analyses) {
    issues += a.downstreamIssues.length
    if (a.storyChanged) storyChanged += 1
  }
  return { issues, storyChanged }
}

// ─── /sync --check — “Xem khác biệt”: so đoạn văn trước/sau ─────────────

export type DiffLine = { kind: 'same' | 'add' | 'del'; text: string }

/**
 * Diff theo đoạn (LCS) giữa bản đã tiếp nhận và bản sửa ngoài app; đoạn giống nhau dài được
 * gộp thành một dòng “same” rút gọn để chỉ còn ngữ cảnh quanh chỗ sửa.
 */
export function paragraphDiff(before: string, after: string, context = 1): DiffLine[] {
  const a = before.split(/\n+/).map((s) => s.trim()).filter(Boolean)
  const b = after.split(/\n+/).map((s) => s.trim()).filter(Boolean)
  const lcs: number[][] = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0))
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1])
    }
  }
  const raw: DiffLine[] = []
  let i = 0
  let j = 0
  while (i < a.length || j < b.length) {
    if (i < a.length && j < b.length && a[i] === b[j]) { raw.push({ kind: 'same', text: a[i] }); i++; j++ }
    else if (j < b.length && (i >= a.length || lcs[i][j + 1] >= lcs[i + 1][j])) { raw.push({ kind: 'add', text: b[j] }); j++ }
    else { raw.push({ kind: 'del', text: a[i] }); i++ }
  }
  const near = raw.map((_, k) => raw.slice(Math.max(0, k - context), k + context + 1).some((line) => line.kind !== 'same'))
  const out: DiffLine[] = []
  let skipped = 0
  raw.forEach((line, k) => {
    if (line.kind !== 'same' || near[k]) {
      if (skipped > 0) out.push({ kind: 'same', text: `… ${skipped} đoạn không đổi …` })
      skipped = 0
      out.push(line)
    } else skipped++
  })
  if (skipped > 0) out.push({ kind: 'same', text: `… ${skipped} đoạn không đổi …` })
  return out
}

// ─── /export — dòng lệnh khi chọn cách xử lý tệp trùng ───────────────────

/** Bỏ cờ xử lý-trùng cũ rồi gắn `--overwrite` (Thay bản cũ) hoặc `--keep` (Giữ cả hai). */
export function withExportFlag(line: string, mode: 'replace' | 'keep'): string {
  const tokens = line.trim().split(/\s+/).filter((token) => token !== '--overwrite' && token !== '--keep')
  return [...tokens, mode === 'replace' ? '--overwrite' : '--keep'].join(' ')
}

/** Đường dẫn người dùng gõ sau `/export` (đối số đầu không phải cờ / from= / to=); '' nếu bỏ trống. */
export function exportPathArg(line: string): string {
  const [, ...args] = line.trim().split(/\s+/)
  return args.find((arg) => !arg.startsWith('--') && !/^(from|to)=/.test(arg)) ?? ''
}

/**
 * “Xuất thêm định dạng” (CmdResults exportDone): giữ from=/to= và các cờ, bỏ đối số đường dẫn
 * cùng `format=…` cũ rồi ghi từ khoá định dạng — đúng cách `/export txt|epub|narration` chọn định
 * dạng mà không cần đặt tên tệp (13-commands.md §/export).
 */
export function withExportFormat(line: string, format: 'txt' | 'epub' | 'narration'): string {
  const [name, ...args] = line.trim().split(/\s+/)
  const path = exportPathArg(line)
  // `doc-so` là công tắc không có gạch đầu dòng — đừng nhầm nó với đối số đường dẫn.
  const kept = args.filter((arg) => !/^format=/.test(arg) && !/^(txt|epub|narration)$/.test(arg) && !(arg === path && arg !== 'doc-so'))
  return [name, format, ...kept].join(' ')
}

/** Tên lệnh (không có `/`, chữ thường) của dòng gửi lên; '' nếu không phải lệnh. */
export const commandName = (line: string) => (/^\/(\S+)/.exec(line.trim())?.[1] ?? '').toLowerCase()
