// Logic thuần của nhóm “Truy cập & khôi phục” (Tuỳ chỉnh › Nâng cao).
import { t } from '../../../i18n/vi'
import { formatContextK } from '../../../lib/modelContext'

const pad = (n: number) => String(n).padStart(2, '0')

/** ISO → `dd/mm` theo giờ máy (artboard: “Đã tạo 24/10”). Không đọc được → rỗng. */
export function shortDate(iso: string): string {
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? '' : `${pad(date.getDate())}/${pad(date.getMonth() + 1)}`
}

export type RecoveryStatus = { createdAt: string | null; used: boolean; /** mã che giữa do máy chủ trả (`TRAX-••••-••••-41ZN`); thiếu = che cả mã */ mask?: string }

/** Giá trị hàng “Mã khôi phục quản trị”: “Đã tạo dd/mm · chưa dùng”. */
export function recoveryValue(status: RecoveryStatus): string {
  const date = status.createdAt ? shortDate(status.createdAt) : ''
  return date ? t.adv.access.recoveryCreated(date, status.used) : t.adv.access.recoveryNone
}

export type LanView = {
  value: string
  action: string
  /** Lý do khoá nút; rỗng = bấm được. */
  disabledReason: string
}

/** Hàng “Cho phép thiết bị cùng Wi-Fi”: chỉ quản trị ngay trên máy chủ (canReveal) mới đổi được. */
export function lanView(lan: boolean, canToggle: boolean): LanView {
  return {
    value: lan ? t.adv.access.lanOn : t.adv.access.lanOff,
    action: lan ? t.adv.access.lanTurnOff : t.adv.access.lanTurnOn,
    disabledReason: canToggle ? '' : t.adv.access.lanNeedServer,
  }
}

export const recoveryFile = (code: string) => ({ name: t.adv.recovery.fileName, text: t.adv.recovery.fileText(code) })

// ─── Nhóm “Tinh chỉnh model” & “Tên thư mục & tên tệp xuất” ──────────────────────────────────────

const DURATION = /^(?:\d+(?:h|m|s))+$/

/** Thời lượng kiểu Go (“900s”, “15m”, “1h30m”); rỗng hợp lệ = dùng mặc định. */
export const isDuration = (text: string): boolean => !text.trim() || DURATION.test(text.trim())

/** “90s” → “90 giây”, “5m” → “5 phút”, “1h” → “1 giờ”; không đọc được thì giữ nguyên. */
export function humanDuration(text: string): string {
  const value = text.trim()
  if (!DURATION.test(value)) return value
  let total = 0
  for (const [, amount, unit] of value.matchAll(/(\d+)([hms])/g)) total += Number(amount) * (unit === 'h' ? 3600 : unit === 'm' ? 60 : 1)
  if (total % 3600 === 0 && total > 0) return `${total / 3600} giờ`
  if (total % 60 === 0 && total > 0) return `${total / 60} phút`
  return `${total} giây`
}

/** Mặc định của máy chủ khi `stream_idle_timeout` để trống. */
export const DEFAULT_IDLE = '5m'

/** Giá trị hàng “Ngắt khi model im lặng”: một giá trị chung, mặc định, hoặc khác nhau theo nhà cung cấp. */
export function idleSummary(values: (string | undefined)[]): string {
  const distinct = Array.from(new Set(values.map((value) => (value ?? '').trim())))
  if (distinct.length === 0 || (distinct.length === 1 && distinct[0] === '')) return `Mặc định (${humanDuration(DEFAULT_IDLE)})`
  if (distinct.length === 1) return humanDuration(distinct[0])
  return 'Khác nhau theo nhà cung cấp'
}

/** Compact context capacity; an unset override uses the server default. */
export function contextWindowValue(own: number | undefined, serverDefault: number | undefined): string {
  if (own && own > 0) return `${formatContextK(own)} token`
  return serverDefault && serverDefault > 0 ? `${formatContextK(serverDefault)} token (mặc định)` : 'Mặc định của máy chủ'
}

/** Ô nhập số token: rỗng → `undefined` (xoá khoá); số nguyên dương → số; còn lại → `null` (không hợp lệ). */
export function parseContextWindow(text: string): number | undefined | null {
  const value = text.trim()
  if (!value) return undefined
  return /^\d+$/.test(value) && Number(value) > 0 ? Number(value) : null
}

export const IMPORT_ROLES = [
  { key: 'import_segment', name: 'IMPORT_SEGMENT', desc: 'Chia bản thảo nhập thành đoạn theo ngữ nghĩa' },
  { key: 'import_analyze', name: 'IMPORT_ANALYZE', desc: 'Phân tích từng đoạn: nhân vật, sự kiện, giọng văn' },
  { key: 'import_synthesize', name: 'IMPORT_SYNTHESIZE', desc: 'Tổng hợp thành thiết lập, đại cương và trạng thái truyện' },
] as const

type RoleRefs = Record<string, { provider?: string; model?: string } | undefined> | undefined

/** Vai trò nhập truyện có model riêng khi có đủ provider + model; thiếu một trong hai = “Theo Mặc định”. */
export const importRoleModel = (roles: RoleRefs, key: string): string => {
  const role = roles?.[key]
  return role?.provider && role?.model ? `${role.provider}/${role.model}` : ''
}

/** Giá trị hàng “Model khi nhập truyện”. */
export function importModelsValue(roles: RoleRefs): string {
  const own = IMPORT_ROLES.filter((role) => importRoleModel(roles, role.key)).length
  return own === 0 ? `Theo Mặc định (${IMPORT_ROLES.length} bước)` : `${own}/${IMPORT_ROLES.length} bước đặt riêng`
}

/** “y-tuong · tai-san · kich_ban · xuat-ban” (Ý tưởng · Tư liệu · Truyện · Bản xuất). */
export const dirsValue = (dirs: { ideas: string; assets: string; stories: string; exports: string }): string =>
  [dirs.ideas, dirs.assets, dirs.stories, dirs.exports].join(' · ')

export const slugWordsValue = (words: number): string => `${words} từ đầu, bỏ dấu`

/** Lệnh báo động: rỗng → câu mô tả mặc định của artboard. */
export const commandValue = (command: string | undefined): string =>
  command?.trim() ? command.trim() : 'chưa đặt — dùng thông báo hệ thống'

type PathScope = 'global' | 'project' | 'story'
const HOME = /^(?:\/Users\/[^/]+|\/home\/[^/]+|[A-Za-z]:\\Users\\[^\\]+)(?=[/\\]|$)/
const parts = (path: string): string[] => path.split(/[/\\]+/).filter(Boolean)

/** Đường dẫn tệp cấu hình theo artboard: “~/.ainovel/config.json” · “<dự án>/.ainovel/config.json” · “<thư mục truyện>/<truyện>/.ainovel/config.json”. Đầy đủ nằm ở tooltip. */
export function configPathValue(path: string, scope: PathScope): string {
  if (scope === 'global') return path.replace(HOME, '~')
  return parts(path).slice(scope === 'project' ? -3 : -4).join('/')
}

/** Thư mục quy tắc theo artboard: “~/.ainovel/rules/” ở toàn cục, “…/.ainovel/rules/” ở dự án/truyện. */
export function rulesPathValue(path: string, scope: PathScope): string {
  if (scope !== 'global') return '…/.ainovel/rules/'
  const short = path.replace(HOME, '~')
  return /[/\\]$/.test(short) ? short : `${short}/`
}
