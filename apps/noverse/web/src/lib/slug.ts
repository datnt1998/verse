// Quy tắc slug/thư mục dùng chung — chép đúng 11-api-contract.md §1, §3 và internal/library.Slugify.
// Dùng cho: xem trước tên thư mục truyện, xem trước mẫu tên tệp xuất bản, kiểm tra tên thư mục.

export const SLUG_MIN_WORDS = 1
export const SLUG_MAX_WORDS = 12
export const DEFAULT_SLUG_WORDS = 5
export const DIR_NAME_MAX = 100

/** `^[a-z0-9][a-z0-9_-]*$` */
export const DIR_NAME_PATTERN = /^[a-z0-9][a-z0-9_-]*$/

/** Tên không được dùng cho thư mục dự án/truyện/dirs.* (11-api-contract.md §1) */
export const RESERVED_DIR_NAMES = ['output', '.ainovel', 'project.json', 'story.json'] as const

export function clampSlugWords(value: number): number {
  if (!Number.isFinite(value)) return DEFAULT_SLUG_WORDS
  return Math.min(SLUG_MAX_WORDS, Math.max(SLUG_MIN_WORDS, Math.trunc(value)))
}

/**
 * `slugify` của backend (internal/library.Slugify): NFD + bỏ dấu kết hợp, đ→d, chữ thường,
 * ký tự khác [a-z0-9] thành khoảng trắng, lấy `words` từ đầu (bỏ trống/0 = tất cả), nối bằng '-'.
 * Rỗng → "truyen-moi".
 */
export function slugifyOrEmpty(text: string, words?: number): string {
  const parts = text
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/đ/g, 'd')
    .replace(/Đ/g, 'D')
    .toLowerCase()
    .replace(/[^a-z0-9\s]/g, ' ')
    .trim()
    .split(/\s+/)
    .filter(Boolean)
  const kept = words && words > 0 ? parts.slice(0, words) : parts
  return kept.join('-')
}

/** Như `slugify` nhưng giữ nguyên chuỗi rỗng khi tên không còn ký tự dùng được (để báo lỗi nhập). */
export function slugify(text: string, words?: number): string {
  return slugifyOrEmpty(text, words) || 'truyen-moi'
}

/** Thư mục truyện: `NN-<slug>` (NN = số lớn nhất hiện có + 1, 2 chữ số). */
export function storyFolderName(index: number, title: string, words: number): string {
  const nn = String(Math.max(1, Math.trunc(index))).padStart(2, '0')
  return `${nn}-${slugify(title, words)}`
}

export type DirNameState = 'standard' | 'valid' | 'invalid'

/** standard = đúng tên mặc định; valid = đổi nhưng hợp lệ; invalid = sai chuẩn. */
export function dirNameState(value: string, standard: string): DirNameState {
  if (value === standard) return 'standard'
  if (value.length === 0 || value.length > DIR_NAME_MAX) return 'invalid'
  if (!DIR_NAME_PATTERN.test(value)) return 'invalid'
  if ((RESERVED_DIR_NAMES as readonly string[]).includes(value)) return 'invalid'
  return 'valid'
}

export type PatternVars = {
  /** tên thư mục truyện, vd "01-tra-xanh" — biến {truyen} */
  storyFolder: string
  tap?: string
  from?: string
  to?: string
  date?: string
  formats: string[]
}

/** Mẫu tên tệp: {truyen} {tap} {tu} {den} {ngay} {ext} — 11-api-contract.md §3. */
export function renderPattern(pattern: string, vars: PatternVars): string {
  const ext = vars.formats.includes('epub') && !vars.formats.includes('txt') ? 'epub' : 'txt'
  return pattern
    .replace(/\{truyen\}/g, vars.storyFolder)
    .replace(/\{tap\}/g, vars.tap ?? '01')
    .replace(/\{tu\}/g, vars.from ?? '1')
    .replace(/\{den\}/g, vars.to ?? '8')
    .replace(/\{ngay\}/g, vars.date ?? 'YYYYMMDD')
    .replace(/\{ext\}/g, ext)
}

/** `onExists: 'keep'` thêm `-{ngay}` (YYYYMMDD-HHmmss) trước đuôi tệp. */
export function withKeepSuffix(path: string): string {
  const slash = path.lastIndexOf('/')
  const dot = path.lastIndexOf('.')
  if (dot <= slash) return `${path}-YYYYMMDD-HHmmss`
  return `${path.slice(0, dot)}-YYYYMMDD-HHmmss${path.slice(dot)}`
}
