// Quy tắc viết — logic thuần (không React): đọc/ghi `quy-tac.md`, chip “Noverse hiểu là”, nhóm theo nguồn,
// bảng “Noverse hiểu thế nào?”, câu “khi nào có hiệu lực”. Spec: 20-settings-rules.md §3.
import type {
  ConfigScope, RuleChip, RuleChipKind, RuleItem, RuleStatus, RulesTiming, RuntimeRule,
} from '../../../../api/types'

/** Tệp duy nhất UI quản lý ở mỗi cấp (20 §3.3). */
export const RULES_FILE = 'quy-tac.md'

// ─── Tệp quy-tac.md: mỗi quy tắc một dòng `- …` ─────────────────────────

/** Mỗi dòng không trống / không bắt đầu bằng `#` là một quy tắc; tiền tố `- ` / `* ` được cắt (như backend). */
export function parseRuleLines(text: string): string[] {
  const out: string[] = []
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim()
    if (!line || line.startsWith('#')) continue
    out.push(line.replace(/^[-*]\s+/, '').trim())
  }
  return out.filter(Boolean)
}

/** Một quy tắc = một dòng: xuống dòng trong ô nhập gộp thành khoảng trắng. */
export const oneLine = (text: string): string => text.replace(/\s*\r?\n\s*/g, ' ').trim()

export const serializeRuleLines = (lines: string[]): string => lines.length ? lines.map((line) => `- ${oneLine(line)}`).join('\n') + '\n' : ''

// ─── Chip “Noverse hiểu là” ──────────────────────────────────────────────

export type ChipStyle = { icon: string; bg: string; fg: string }
/** Màu & icon: Settings.dc.html `K` (đỏ nhạt · cam · xanh · tím). Icon = tệp resolve từ `.i-*` của chính artboard. */
export const CHIP_STYLE: Record<RuleChipKind, ChipStyle> = {
  ban: { icon: 'close-circle-bulk', bg: 'rgba(255,159,168,0.1)', fg: '#FFC4CA' },
  chr: { icon: 'close-circle-bulk', bg: 'rgba(255,159,168,0.1)', fg: '#FFC4CA' },
  max: { icon: 'flag', bg: 'rgba(246,203,140,0.1)', fg: '#F3D9AE' },
  genre: { icon: 'tag', bg: 'rgba(157,176,247,0.12)', fg: '#C8D3F8' },
  pref: { icon: 'messages-2', bg: 'rgba(201,166,245,0.1)', fg: '#DCC8F8' },
}

const quote = (text: string) => `“${text}”`

/** Nhãn của chip, ví dụ `Không dùng “bỗng nhiên”`, `Tối đa “thở dài” · 2 lần/chương`. */
export function chipText(chip: RuleChip, degraded = false): string {
  switch (chip.kind) {
    case 'ban': return `Không dùng ${quote(chip.text)}`
    case 'chr': return `Không dùng ký tự ${quote(chip.text)}`
    case 'max': return `Tối đa ${quote(chip.text)} · ${chip.limit ?? 1} lần/chương`
    case 'genre': return chip.text
    case 'pref': return degraded ? 'Lời dặn' : chip.text
  }
}

/** Chip khi chuẩn hoá lỗi: quy tắc vẫn lưu, hiện như “Lời dặn” (lời dặn cho trợ lý, không kiểm máy). */
export const degradedChips = (text: string): RuleChip[] => [{ kind: 'pref', text }]

// ─── Bản ghi một quy tắc trên màn hình ───────────────────────────────────

export type RuleEntry = {
  key: string
  text: string
  chips: RuleChip[]
  /** máy chủ đánh dấu chưa chắc (Câu 26a: `items[].uncertain`); false = rõ ràng */
  uncertain: boolean
  /** câu lý do chưa chắc (`uncertainNote`); rỗng = không có gì để hỏi */
  note: string
  /** 0–3 cách hiểu viết lại cho khung cam */
  options: string[]
  status: RuleStatus
  /** thẻ mới thêm trong phiên này → loé tím */
  fresh?: boolean
  /** đã bấm “Giữ như lời dặn”: bỏ khung cam */
  acked?: boolean
  /** luật Trọng tài đã chuyển thành quy tắc của truyện */
  fromRuntime?: string
}

let seq = 0
export const nextKey = (prefix: string) => `${prefix}:${++seq}`

export function entryFromItem(item: RuleItem, key: string): RuleEntry {
  return {
    key, text: item.text, chips: item.chips ?? [],
    uncertain: item.uncertain === true, note: item.uncertainNote ?? '',
    options: item.options ?? [], status: item.status,
  }
}

export function entryFromRuntime(rule: RuntimeRule): RuleEntry {
  const reasons = rule.uncertain ?? []
  return {
    key: `rt:${rule.id}`, text: rule.text, chips: rule.chips ?? [],
    uncertain: reasons.length > 0, note: rule.uncertainNote || reasons[0] || '',
    options: rule.options ?? [], status: rule.status, fromRuntime: rule.id,
  }
}

/** Khung cam “chưa chắc”: máy chủ đánh dấu, có câu lý do và thẻ chưa bị đóng. Thẻ degraded luôn hiện “Lời dặn”. */
export const isUncertain = (entry: RuleEntry): boolean =>
  entry.status === 'ready' && entry.uncertain && entry.note !== '' && !entry.acked

// ─── Trạng thái chỉnh sửa (chưa Áp dụng) ─────────────────────────────────

export type Level = 'story' | 'learned' | 'project' | 'global'

export type EditState = {
  /** danh sách đầy đủ của cấp đã bị sửa (cấp chưa đụng tới không có khoá) */
  lists: Partial<Record<ConfigScope, RuleEntry[]>>
  /** id luật Trọng tài sẽ “Giữ” / “Bỏ” khi Áp dụng */
  kept: string[]
  dropped: string[]
  /** số thao tác (bảng đếm “N thay đổi”) */
  ops: number
}

export const emptyEdits = (): EditState => ({ lists: {}, kept: [], dropped: [], ops: 0 })

export type Baseline = Record<ConfigScope, RuleEntry[]> & { runtime: RuleEntry[] }

const sameTexts = (a: RuleEntry[], b: RuleEntry[]) => a.length === b.length && a.every((entry, index) => entry.text === b[index].text)

/** Số thay đổi hiển thị; 0 khi mọi thứ đã trở về đúng như trên máy chủ (thêm rồi xoá…). */
export function changeCount(base: Baseline, edits: EditState): number {
  const scopes = Object.keys(edits.lists) as ConfigScope[]
  const changed = scopes.some((scope) => !sameTexts(edits.lists[scope] ?? [], base[scope])) || edits.kept.length > 0 || edits.dropped.length > 0
  return changed ? Math.max(1, edits.ops) : 0
}

export const listOf = (base: Baseline, edits: EditState, scope: ConfigScope): RuleEntry[] => edits.lists[scope] ?? base[scope]

/** Luật Trọng tài còn hiện trong nhóm “Học từ góp ý” (chưa Giữ/Bỏ). */
export const visibleRuntime = (base: Baseline, edits: EditState): RuleEntry[] =>
  base.runtime.filter((entry) => !edits.kept.includes(entry.fromRuntime ?? '') && !edits.dropped.includes(entry.fromRuntime ?? ''))

// ─── Nhóm theo nguồn ─────────────────────────────────────────────────────

export const GROUP_ORDER: Record<ConfigScope, Level[]> = {
  story: ['story', 'learned', 'project', 'global'],
  project: ['project', 'global'],
  global: ['global'],
}

export type GroupMeta = { label: string; icon: string; color: string }
/** Icon = tệp: i-book→book-1 · i-messages→messages-2 · i-folder→folder-open · i-global→global. */
export function groupMeta(level: Level, projectName?: string): GroupMeta {
  switch (level) {
    case 'story': return { label: 'Riêng truyện này', icon: 'book-1', color: '#C9A6F5' }
    case 'learned': return { label: 'Học từ góp ý của bạn', icon: 'messages-2', color: '#A6EDC1' }
    case 'project': return { label: projectName ? `Từ dự án “${projectName}”` : 'Từ dự án', icon: 'folder-open', color: '#9DB0F7' }
    case 'global': return { label: 'Từ “Mọi truyện”', icon: 'global', color: '#9DB0F7' }
  }
}

/** Cấp đang sửa được: chính cấp đang chọn, và ở cấp truyện còn nhóm “Học từ góp ý”. */
export const isOwnLevel = (level: Level, scope: ConfigScope): boolean => level === scope || (scope === 'story' && level === 'learned')

export const groupSub = (count: number, level: Level, scope: ConfigScope): string =>
  `${count} quy tắc${isOwnLevel(level, scope) ? '' : ' · kế thừa'}${level === 'learned' ? ' · Trọng tài ghi lại khi bạn dặn trong chat' : ''}`

/** Ngày của dòng “Từ góp ý của bạn ở chương 12 · 21/10”: hôm nay / hôm qua / dd/MM. */
export function shortDate(iso: string, now = new Date()): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  const day = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const diff = Math.round((day(now) - day(date)) / 86_400_000)
  if (diff === 0) return 'hôm nay'
  if (diff === 1) return 'hôm qua'
  return `${date.getDate()}/${date.getMonth() + 1}`
}

/** Dòng meta: máy tính “Từ góp ý của bạn ở chương 12 · 21/10”, điện thoại “Từ góp ý ở chương 12”. */
export const learnedMeta = (rule: Pick<RuntimeRule, 'chapter' | 'createdAt'>, mobile = false): string => {
  if (mobile) return `Từ góp ý ở chương ${rule.chapter}`
  const date = shortDate(rule.createdAt)
  return `Từ góp ý của bạn ở chương ${rule.chapter}${date ? ` · ${date}` : ''}`
}

export const moveLabel = (scope: ConfigScope): string => scope === 'story' ? 'Chuyển lên dự án (mọi truyện trong dự án)' : 'Chuyển lên “Mọi truyện”'
export const moveTarget = (scope: ConfigScope): ConfigScope | null => scope === 'story' ? 'project' : scope === 'project' ? 'global' : null

// ─── Mặc định của Noverse ────────────────────────────────────────────────

/** Gương quy tắc mặc định của engine: 4 cụm không dùng và 16 từ dễ lặp. */
export const DEFAULT_BAN = ['theo một nghĩa nào đó', 'đáng chú ý là', 'không rõ vì sao', 'không nói thành lời']
/** Giữ đủ 16 từ thật; artboard Gói AC chỉ vẽ 12 (chờ design chốt). */
export const DEFAULT_FATIGUE: [string, number][] = [
  ['bỗng nhiên', 1], ['dường như', 2], ['tuy nhiên', 2], ['một chút', 2], ['tựa như', 1], ['như một', 3], ['im lặng', 2], ['một thoáng', 3],
  ['chốc lát', 3], ['trong chớp mắt', 2], ['không kìm được', 1], ['không khỏi', 1],
  ['chẳng biết từ đâu', 1], ['một vệt', 2], ['một sợi', 2], ['không nói gì', 2],
]

// ─── “Noverse hiểu thế nào?” ─────────────────────────────────────────────

export type SummaryRow = { kind: RuleChipKind; label: string; icon: string; items: string[] }

const maxLabel = (chip: RuleChip) => `${quote(chip.text)} · ${chip.limit ?? 1} lần/chương`

/** Gộp mọi chip đang hiệu lực + phần mặc định còn lại (người dùng nới/đè một từ thì bớt khỏi “+ N … mặc định”). */
export function summarize(entries: RuleEntry[]): SummaryRow[] {
  const chips = entries.flatMap((entry) => entry.status === 'degraded' ? [] : entry.chips)
  const prefs = entries.flatMap((entry) => entry.status === 'degraded' ? [entry.text] : entry.chips.filter((c) => c.kind === 'pref').map((c) => c.text))
  const ofKind = (...kinds: RuleChipKind[]) => chips.filter((chip) => kinds.includes(chip.kind))
  const banned = ofKind('ban')
  const maxed = ofKind('max')
  const overlapBan = DEFAULT_BAN.filter((word) => banned.some((chip) => chip.text === word)).length
  const overlapFat = DEFAULT_FATIGUE.filter(([word]) => maxed.some((chip) => chip.text === word)).length
  const rows: SummaryRow[] = [
    { kind: 'genre', label: 'Thể loại', icon: 'tag', items: ofKind('genre').map((chip) => chip.text) },
    { kind: 'ban', label: 'Không dùng', icon: 'close-circle-bulk', items: [...banned.map((chip) => quote(chip.text)), ...ofKind('chr').map((chip) => quote(chip.text)), `+ ${DEFAULT_BAN.length - overlapBan} cụm mặc định`] },
    { kind: 'max', label: 'Giới hạn lặp', icon: 'flag', items: [...maxed.map(maxLabel), `+ ${DEFAULT_FATIGUE.length - overlapFat} từ mặc định`] },
    { kind: 'pref', label: 'Lời dặn', icon: 'messages-2', items: prefs },
  ]
  return rows.filter((row) => row.items.length > 0)
}

// ─── Khi nào có hiệu lực ─────────────────────────────────────────────────

/** Cấp truyện: từ chương n+1; chương đang viết dở giữ luật cũ; chương đã viết không bị sửa. */
export function whenText(scope: ConfigScope, timing: RulesTiming | undefined): string {
  if (scope !== 'story') return `Có hiệu lực cho truyện mới, và cho các truyện đang viết trong ${scope === 'project' ? 'dự án' : 'thư viện'} từ chương kế tiếp của từng truyện.`
  if (!timing) return 'Quy tắc mới có hiệu lực từ chương kế tiếp. Chương đã viết không bị sửa.'
  const clauses = []
  if (timing.inProgress > 0) clauses.push(`Chương ${timing.inProgress} đang viết dở giữ quy tắc cũ`)
  if (timing.completed > 0) clauses.push(`${timing.completed} chương đã viết không bị sửa`)
  return `Quy tắc mới có hiệu lực từ chương ${timing.effectiveFrom}.${clauses.length ? ` ${clauses.join('; ')}.` : ''}`
}

/** “có hiệu lực từ chương 26” cho thanh nổi đáy (chỉ cấp truyện). */
export const effectiveHint = (timing: RulesTiming | undefined): string | undefined => timing ? `có hiệu lực từ chương ${timing.effectiveFrom}` : undefined

export function appliedToast(scope: ConfigScope, object: string, effectiveFrom: number | undefined, recheck: boolean, completed: number): string {
  if (scope !== 'story') return `Đã áp dụng cho ${object}`
  return `Đã áp dụng · từ chương ${effectiveFrom ?? '?'}` + (recheck && completed > 0 ? ` · đang kiểm tra ${completed} chương cũ` : '')
}

/** Chip của thẻ “lưu nguyên văn”: máy chủ bận, Noverse tự thử hiểu lại khi ổn (Settings.dc.html › rules=retry). */
export const RAW_SAVED_CHIP: RuleChip = { kind: 'pref', text: 'Lưu nguyên văn · sẽ thử hiểu lại khi máy chủ ổn' }

/** Thử lại tối đa 3 lần trong khoảng 20 giây (brief 25/10 › câu hỏi treo “lỗi 500 phải chờ tới 2 phút”). */
export const RULES_RETRY_ATTEMPTS = 3
export const RULES_RETRY_INTERVAL_MS = 10_000

/** Thẻ chờ thử lại: `attempt` là số lần thử kế tiếp (2 → “Noverse thử lại lần 2/3”). */
export type RuleRetry = { text: string; attempt: number; total: number }

/** Câu của thẻ chờ, đúng artboard. */
export const retryLine = (retry: RuleRetry): string =>
  `Máy chủ đang bận, Noverse thử lại lần ${retry.attempt}/${retry.total}. Nếu vẫn lỗi, quy tắc được lưu nguyên văn sau khoảng 20 giây.`
